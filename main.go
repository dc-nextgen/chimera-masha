// Command masha-agent — on-prem DB connector ajani (Go tek-binary, arka-plan servis).
//
// Zincir: bulut OWUI → tunel → [bu agent: toolserver → mssql connector] → SQL Server.
// MCP yurutmesi BURADA (kutuda); tanim (manifest) bulutta uretilir, iner. Ham satir kutuda kalir.
//
// Komutlar:
//
//	serve       (default) tool-server + yerel web yuz + (ops) frpc tunel
//	introspect  sema YAPISINI (tablo/kolon) JSON bas — onboarding/test (satir okumaz)
//	version
//
// kardianos service (install/start/stop) = Faz 3.
package main

import (
	"context"
	"crypto/tls"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dc-nextgen/chimera-masha/internal/audit"
	"github.com/dc-nextgen/chimera-masha/internal/config"
	"github.com/dc-nextgen/chimera-masha/internal/connector"
	"github.com/dc-nextgen/chimera-masha/internal/connector/erpnext"
	"github.com/dc-nextgen/chimera-masha/internal/connector/mssql"
	"github.com/dc-nextgen/chimera-masha/internal/live"
	"github.com/dc-nextgen/chimera-masha/internal/manifest"
	"github.com/dc-nextgen/chimera-masha/internal/onboard"
	"github.com/dc-nextgen/chimera-masha/internal/registry"
	"github.com/dc-nextgen/chimera-masha/internal/selfcert"
	"github.com/dc-nextgen/chimera-masha/internal/toolserver"
	"github.com/dc-nextgen/chimera-masha/internal/tunnel"
	"github.com/dc-nextgen/chimera-masha/internal/webui"
)

const version = "0.1.0-faz1"

// Yerel web yuz (React+shadcn, web/) binary'ye GOMULU → tek-binary korunur, ayri sunucu yok.
// `make build` (veya npm run build) web/dist'i uretir; go build gomer.
//
//go:embed all:web/dist
var webDistFS embed.FS

// webUIFS — dist alt-agacini dondurur (yoksa nil → yalniz JSON API, UI'siz).
func webUIFS() fs.FS {
	sub, err := fs.Sub(webDistFS, "web/dist")
	if err != nil {
		return nil
	}
	if _, err := fs.Stat(sub, "index.html"); err != nil {
		return nil // dist bos/derlenmemis
	}
	return sub
}

func main() {
	cmd := "serve"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	switch cmd {
	case "version", "-v", "--version":
		fmt.Println("masha-agent", version)
	case "introspect":
		runIntrospect()
	case "serve":
		runServe()
	default:
		fmt.Fprintf(os.Stderr, "bilinmeyen komut: %s (serve|introspect|version)\n", cmd)
		os.Exit(2)
	}
}

// openConnector — manifest + DSN'den mssql connector. DSN kaynak sirasi: env MASHA_DB_DSN >
// yerel cred dosyasi (ekrandan-baglan) > BOS (ekrandan baglanmayi bekle, fatal DEGIL).
// Manifest CANLI holder'a konur (onboarding apply hot-swap eder).
func openConnector(cfg *config.Config) (*mssql.Connector, *live.Manifest) {
	// Taze kurulum/deneme: manifest dosyasi YOKSA bos-baslangicla ac (web+wizard calisir; ilk apply doldurur).
	man, err := manifest.LoadOrEmpty(cfg.ManifestPath, cfg.ServerLabel)
	if err != nil {
		log.Fatalf("FATAL manifest: %v", err) // dosya VAR ama gecersiz → durur (fail-closed)
	}
	if len(man.Tools) == 0 {
		log.Printf("manifest BOS (taze kurulum) — 0 arac; DB baglayip wizard'la tanimla (yerel yuz → Bağlantılar)")
	}
	lm := live.New(man)
	dsn := cfg.DSN
	if dsn == "" {
		if f, err := loadCreds(cfg.DBCredFile); err == nil && f != nil {
			dsn = mssql.BuildDSN(*f)
			log.Printf("DB kimligi yerel dosyadan yuklendi (%s@%s)", f.User, f.Host)
		}
	}
	conn, err := mssql.Open(dsn, lm)
	if err != nil {
		log.Fatalf("FATAL connector: %v", err)
	}
	if dsn == "" {
		log.Printf("DB kimligi YOK — ekrandan baglan (yerel yuz → Bağlantı)")
	}
	return conn, lm
}

// loadCreds — ekrandan girilen DB kimligini yerel dosyadan oku (0600). Yoksa (nil,nil).
func loadCreds(path string) (*connector.DBFields, error) {
	if path == "" {
		return nil, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err // yok/okunamaz → dosya yok say (ekrandan baglanilir)
	}
	var f connector.DBFields
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, err
	}
	return &f, nil
}

// saveCreds — DB kimligini YERELE kalici yaz (0600; §3 buluta ASLA gitmez). Faz 3: keychain.
func saveCreds(path string, f connector.DBFields) error {
	if path == "" {
		return fmt.Errorf("MASHA_DB_CRED_FILE bos — kimlik kalici yazilamaz")
	}
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// loadErpFields — ekrandan girilen ErpNext kimligini yerel dosyadan oku (0600). Yoksa (nil,nil).
func loadErpFields(path string) (*erpnext.Fields, error) {
	if path == "" {
		return nil, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f erpnext.Fields
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, err
	}
	return &f, nil
}

// saveErpFields — ErpNext kimligini YERELE kalici yaz (0600; §3 buluta gitmez). Faz 3: keychain.
func saveErpFields(path string, f erpnext.Fields) error {
	if path == "" {
		return fmt.Errorf("MASHA_ERPNEXT_CRED_FILE bos")
	}
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// writeManifestAtomic — manifest'i KALICI yaz (temp + rename; yarim dosya birakmaz). Onboarding apply.
func writeManifestAtomic(path string, m *manifest.Manifest) error {
	if path == "" {
		return fmt.Errorf("MASHA_MANIFEST yolu bos — manifest kalici yazilamaz")
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func runIntrospect() {
	cfg, _ := config.Load()
	// introspect MANIFEST istemez — sema, manifest'i YAZMADAN once taranir (onboarding mantigi).
	if cfg.DSN == "" {
		log.Fatal("FATAL: MASHA_DB_DSN gerekli")
	}
	conn, err := mssql.Open(cfg.DSN, nil)
	if err != nil {
		log.Fatalf("FATAL connector: %v", err)
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	sc, err := conn.Introspect(ctx)
	if err != nil {
		log.Fatalf("FATAL introspeksiyon: %v", err)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(sc)
}

func runServe() {
	cfg, _ := config.Load()
	if err := cfg.RequireServe(); err != nil {
		log.Fatalf("FATAL: %v", err) // kimlik-doğrulamasiz/manifest'siz acilmaz
	}
	conn, lm := openConnector(cfg)
	defer conn.Close()
	man := lm.Get()

	aud, err := audit.New(cfg.AuditFile)
	if err != nil {
		log.Fatalf("FATAL audit: %v", err)
	}
	defer aud.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// DB canliligi — uyari, fatal degil (ag/DB sonra ayaga kalkabilir).
	{
		c2, cancel := context.WithTimeout(ctx, 5*time.Second)
		if err := conn.Health(c2); err != nil {
			log.Printf("UYARI: DB'ye baglanilamadi (%v) — sunucu yine de aciliyor, DB gelince duzelir", err)
		} else {
			log.Printf("DB baglantisi OK (%s)", man.ERPKind)
		}
		cancel()
	}

	// apply — onboarding "uygula": manifest'i KALICI yaz + CANLI swap (restart YOK). webui cagirir.
	apply := func(m *manifest.Manifest) error {
		if err := writeManifestAtomic(cfg.ManifestPath, m); err != nil {
			return err
		}
		lm.Set(m)
		log.Printf("onboarding apply: manifest guncellendi (%s, %d arac) — hot-reload", m.Name, len(m.Tools))
		return nil
	}

	// dbConnect — ekrandan DB kimligi: DSN kur + Connect (ping) + YERELE kalici yaz (§3, buluta gitmez).
	dbConnect := func(f connector.DBFields) error {
		if err := conn.Connect(context.Background(), mssql.BuildDSN(f)); err != nil {
			return err
		}
		if err := saveCreds(cfg.DBCredFile, f); err != nil {
			log.Printf("UYARI: DB baglandi ama yerel kayit basarisiz (%v) — restart'ta unutulur", err)
		}
		log.Printf("DB baglandi (ekrandan): %s@%s/%s", f.User, f.Host, f.Database)
		return nil
	}

	// LLM danismani (opsiyonel §5): yoksa nil → onboarding heuristik + /onboard/advise 501.
	adviser := onboard.NewSuggester(cfg.LLMBaseURL, cfg.LLMAPIKey, cfg.LLMModel)
	if adviser != nil {
		log.Printf("LLM danismani AKTIF (%s)", cfg.LLMModel)
	}

	// Registry (§19.2 çok-bağlantı): DB (auroville/mssql) + opsiyonel ErpNext (Go-native, §19.1).
	reg := registry.New()
	reg.Add(&registry.Connection{
		Name: cfg.ServerLabel, Label: man.Label, Kind: "mssql", ServerLabel: cfg.ServerLabel, Conn: conn,
	})
	// ErpNext kimligi: env > yerel dosya (ekrandan-baglan). Varsa 2. baglanti olarak kaydet.
	erpFields := erpnext.Fields{URL: cfg.ERPNextURL, APIKey: cfg.ERPNextKey, APISecret: cfg.ERPNextSecret}
	if erpFields.URL == "" {
		if f, err := loadErpFields(cfg.ERPNextCredFile); err == nil && f != nil {
			erpFields = *f
			log.Printf("ErpNext kimligi yerel dosyadan yuklendi (%s)", f.URL)
		}
	}
	var erpConn *erpnext.Connector
	if erpFields.URL != "" {
		erpConn = erpnext.Open(erpFields.URL, erpFields.APIKey, erpFields.APISecret, cfg.ERPNextMask)
		reg.Add(&registry.Connection{
			Name: cfg.ERPNextLabel, Label: "ErpNext (salt-okunur)", Kind: "erpnext",
			ServerLabel: cfg.ERPNextLabel, Conn: erpConn,
		})
		log.Printf("2. baglanti kayitli: ErpNext (%s) → server=%q (maske=%v)", erpFields.URL, cfg.ERPNextLabel, cfg.ERPNextMask)
	}
	// erpnextConnect — ekrandan ErpNext bağlan: yoksa oluştur+kaydet, Connect (doğrula), YERELE yaz (§3).
	erpnextConnect := func(f erpnext.Fields) error {
		if erpConn == nil {
			erpConn = erpnext.Open("", "", "", cfg.ERPNextMask)
			reg.Add(&registry.Connection{
				Name: cfg.ERPNextLabel, Label: "ErpNext (salt-okunur)", Kind: "erpnext",
				ServerLabel: cfg.ERPNextLabel, Conn: erpConn,
			})
		}
		if err := erpConn.Connect(context.Background(), erpnext.BuildDSN(f)); err != nil {
			return err
		}
		if err := saveErpFields(cfg.ERPNextCredFile, f); err != nil {
			log.Printf("UYARI: ErpNext baglandi ama yerel kayit basarisiz (%v)", err)
		}
		log.Printf("ErpNext baglandi (ekrandan): %s", f.URL)
		return nil
	}

	ts := toolserver.New(cfg.AppToken, reg, aud)
	upstream := &http.Server{Addr: cfg.UpstreamAddr, Handler: ts, ReadHeaderTimeout: 10 * time.Second}
	web := &http.Server{Addr: cfg.WebAddr, Handler: webui.New(webui.Deps{
		Live: lm, Conn: conn, Aud: aud, Static: webUIFS(), Apply: apply,
		Adviser: adviser, Password: cfg.WebPassword, DBConnect: dbConnect, Registry: reg,
		ErpnextConnect: erpnextConnect, PrimaryLabel: cfg.ServerLabel,
		Plan: webui.Plan{Plan: cfg.Plan, TrialLimitUSD: cfg.TrialLimitUSD,
			ContactEmail: cfg.ContactEmail, RequestURL: cfg.RequestURL},
	}).Handler(), ReadHeaderTimeout: 10 * time.Second}
	tun := tunnel.NewSidecar(cfg.FrpcBin, cfg.FrpcConfig)

	// Yerel yuz TLS (self-signed, §17.9): LAN'da token'i sifreler. Ac ise cert URET/YUKLE.
	scheme := "http"
	if cfg.WebTLS {
		hosts := selfcert.LocalHosts()
		cert, err := selfcert.LoadOrCreate(cfg.WebTLSDir, hosts)
		if err != nil {
			log.Fatalf("FATAL web TLS: %v", err)
		}
		web.TLSConfig = &tls.Config{Certificates: []tls.Certificate{cert}}
		scheme = "https"
		log.Printf("yerel yuz TLS ACIK (self-signed; SAN=%s; dizin=%s)", selfcert.FormatHosts(hosts), cfg.WebTLSDir)
	}
	// LAN'a acik + (parolasiz VEYA TLS'siz) = token/veri korumasiz → UYAR (§17.9).
	if !isLoopback(cfg.WebAddr) {
		if cfg.WebPassword == "" {
			log.Printf("UYARI: yerel yuz %s (LAN) PAROLASIZ — /try + /onboard/apply korumasiz. MASHA_WEB_PASSWORD ayarla (§17.9).", cfg.WebAddr)
		}
		if !cfg.WebTLS {
			log.Printf("UYARI: yerel yuz %s (LAN) TLS'siz — token duz-HTTP'de gidiyor. MASHA_WEB_TLS=1 ayarla (§17.9).", cfg.WebAddr)
		}
	}

	log.Printf("yerel yuz erisim: %s://%s", scheme, cfg.WebAddr)
	go serve("tool-server (tunel upstream)", upstream, cfg.UpstreamAddr)
	go serve("yerel web yuz", web, cfg.WebAddr)
	if err := tun.Start(ctx); err != nil {
		log.Printf("UYARI: tunel baslamadi (%v) — yalniz-yerel modda devam", err)
	} else if cfg.FrpcConfig != "" {
		log.Printf("tunel (frpc sidecar) baslatildi")
	} else {
		log.Printf("tunel KAPALI (MASHA_FRPC_CONFIG bos) — yalniz-yerel mod")
	}

	log.Printf("masha-agent %s hazir · server=%q · araclar=%v", version, cfg.ServerLabel, man.ToolNames())
	<-ctx.Done()
	log.Printf("kapatiliyor...")

	sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	upstream.Shutdown(sctx)
	web.Shutdown(sctx)
	tun.Stop()
}

// isLoopback — WebAddr host'u yalniz-kutu mu (127.0.0.1/localhost/::1). LAN uyarisi icin.
func isLoopback(addr string) bool {
	host := addr
	if h, _, err := net.SplitHostPort(addr); err == nil {
		host = h
	}
	return host == "127.0.0.1" || host == "localhost" || host == "::1" || host == ""
}

func serve(name string, s *http.Server, addr string) {
	log.Printf("%s dinliyor: %s", name, addr)
	var err error
	if s.TLSConfig != nil {
		err = s.ListenAndServeTLS("", "") // cert TLSConfig.Certificates'ta
	} else {
		err = s.ListenAndServe()
	}
	if err != nil && err != http.ErrServerClosed {
		log.Fatalf("FATAL %s: %v", name, err)
	}
}
