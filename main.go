// Command masha-agent — on-prem DB connector ajani (Go tek-binary, arka-plan servis).
//
// Zincir (docs/masha-plan.md §17): bulut OWUI → tunel → [bu agent: toolserver → mssql connector] → SQL Server.
// MCP yurutmesi BURADA (kutuda); tanim (manifest) bulutta uretilir, iner. Ham satir kutuda kalir.
//
// Komutlar:
//
//	serve       (default) tool-server + yerel web yuz + (ops) frpc tunel
//	introspect  sema YAPISINI (tablo/kolon) JSON bas — onboarding/test (satir okumaz)
//	service     install|uninstall|start|stop|restart|status — reboot-proof OS servisi (kardianos;
//	            launchd/systemd/Windows Service; MASHA_SERVICE_USER=1 → per-user servis)
//	tray        sistem tepsisi ikonu + menu (Aç·Restart·Ayarlar·Çıkış); `serve`'i COCUK SUREC olarak
//	            gozetir (login-item/GUI-oturum yuzu; headless kurulumda 'service' kullanilir)
//	version
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

	"github.com/mehmetor/chimera-ai/stack/masha/agent/internal/audit"
	"github.com/mehmetor/chimera-ai/stack/masha/agent/internal/config"
	"github.com/mehmetor/chimera-ai/stack/masha/agent/internal/connector"
	"github.com/mehmetor/chimera-ai/stack/masha/agent/internal/connector/documents"
	"github.com/mehmetor/chimera-ai/stack/masha/agent/internal/connector/erpnext"
	"github.com/mehmetor/chimera-ai/stack/masha/agent/internal/connector/mssql"
	"github.com/mehmetor/chimera-ai/stack/masha/agent/internal/creds"
	"github.com/mehmetor/chimera-ai/stack/masha/agent/internal/live"
	"github.com/mehmetor/chimera-ai/stack/masha/agent/internal/manifest"
	"github.com/mehmetor/chimera-ai/stack/masha/agent/internal/onboard"
	"github.com/mehmetor/chimera-ai/stack/masha/agent/internal/registry"
	"github.com/mehmetor/chimera-ai/stack/masha/agent/internal/selfcert"
	"github.com/mehmetor/chimera-ai/stack/masha/agent/internal/toolserver"
	"github.com/mehmetor/chimera-ai/stack/masha/agent/internal/tunnel"
	"github.com/mehmetor/chimera-ai/stack/masha/agent/internal/webui"
)

// Sürüm: SemVer ön-sürüm (0.x = stabil değil + 'beta' = müşteri erken-erişim). Her build ayırt edilebilir → takip.
// var (const değil) — build-binaries.sh ileride `-ldflags -X main.version=$(git describe)` ile damgalar (tag varsa);
// tag yoksa bu default kalır. Stabilleşince 0.1.0, üretim-hazır olunca 1.0.0.
var version = "0.1.0-beta.3"

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
	case "service":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "kullanim: masha-agent service install|uninstall|start|stop|restart|status")
			os.Exit(2)
		}
		runServiceCmd(os.Args[2])
	case "tray":
		runTray()
	default:
		fmt.Fprintf(os.Stderr, "bilinmeyen komut: %s (serve|introspect|service|tray|version)\n", cmd)
		os.Exit(2)
	}
}

// openConnector — manifest + DSN'den mssql connector. DSN kaynak sirasi: env MASHA_DB_DSN >
// yerel cred dosyasi (ekrandan-baglan) > BOS (ekrandan baglanmayi bekle, fatal DEGIL).
// Manifest CANLI holder'a konur (onboarding apply hot-swap eder).
func openConnector(cfg *config.Config, cm *creds.CredManager) (*mssql.Connector, *live.Manifest) {
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
		if f, err := loadCreds(cm, cfg.DBCredFile); err == nil && f != nil {
			dsn = mssql.BuildDSN(*f)
			log.Printf("DB kimligi %s'den yuklendi (%s@%s)", cm.Label, f.User, f.Host)
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

// loadCreds — ekrandan girilen DB kimligini kimlik deposundan (keychain veya 0600 dosya, cm.Label'a
// gore) oku. Yoksa (nil,nil). Keychain kullanilirken legacyPath'te eski-kurulum kalintisi varsa
// otomatik migrasyon uygulanir (cm.Load).
func loadCreds(cm *creds.CredManager, legacyPath string) (*connector.DBFields, error) {
	var f connector.DBFields
	ok, err := cm.LoadJSON("db", legacyPath, &f)
	if err != nil || !ok {
		return nil, err
	}
	return &f, nil
}

// saveCreds — DB kimligini kimlik deposuna kalici yaz (§3 buluta ASLA gitmez; Faz 3 keychain).
func saveCreds(cm *creds.CredManager, legacyPath string, f connector.DBFields) error {
	return cm.SaveJSON("db", legacyPath, f)
}

// loadErpFields — ekrandan girilen ErpNext kimligini kimlik deposundan oku. Yoksa (nil,nil).
func loadErpFields(cm *creds.CredManager, legacyPath string) (*erpnext.Fields, error) {
	var f erpnext.Fields
	ok, err := cm.LoadJSON("erpnext", legacyPath, &f)
	if err != nil || !ok {
		return nil, err
	}
	return &f, nil
}

// saveErpFields — ErpNext kimligini kimlik deposuna kalici yaz (§3 buluta gitmez; Faz 3 keychain).
func saveErpFields(cm *creds.CredManager, legacyPath string, f erpnext.Fields) error {
	return cm.SaveJSON("erpnext", legacyPath, f)
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

// tunnelIface — sidecar (SidecarFrpc) VE embed (EmbeddedFrpc) tunel'in ORTAK yuzu; runServe
// hangi implementasyonu kullandigini bilmez (MASHA_TUNNEL_MODE secer).
type tunnelIface interface {
	Start(ctx context.Context) error
	Stop() error
	Status() (state, msg string)
}

// newTunnel — cfg.TunnelMode'a gore sidecar (harici frpc ikilisi/Docker) veya embed (surec-ici
// frp client, Docker/harici frpc YOK) secer. Varsayilan "sidecar" — mevcut davranis DEGISMEZ.
func newTunnel(cfg *config.Config) tunnelIface {
	if cfg.TunnelMode == "embed" {
		return tunnel.NewEmbedded(cfg.FrpcConfig)
	}
	return tunnel.NewSidecar(cfg.FrpcBin, cfg.FrpcConfig)
}

func runServe() {
	manifest.SetVersion(version) // OpenAPI info.version + /healthz TEK kaynak (build-binaries.sh ldflags damgalarsa o)
	cfg, _ := config.Load()
	if err := cfg.RequireServe(); err != nil {
		log.Fatalf("FATAL: %v", err) // kimlik-doğrulamasiz/manifest'siz acilmaz
	}
	credStore, credLabel := creds.Resolve(cfg.Keyring, "masha-agent", "")
	cm := creds.NewCredManager(credStore, credLabel)
	log.Printf("kimlik saklama: %s", cm.Label)

	conn, lm := openConnector(cfg, cm)
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
		if err := saveCreds(cm, cfg.DBCredFile, f); err != nil {
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
		if f, err := loadErpFields(cm, cfg.ERPNextCredFile); err == nil && f != nil {
			erpFields = *f
			log.Printf("ErpNext kimligi %s'den yuklendi (%s)", cm.Label, f.URL)
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
		if err := saveErpFields(cm, cfg.ERPNextCredFile, f); err != nil {
			log.Printf("UYARI: ErpNext baglandi ama yerel kayit basarisiz (%v)", err)
		}
		log.Printf("ErpNext baglandi (ekrandan): %s", f.URL)
		return nil
	}

	ts := toolserver.New(cfg.AppToken, reg, aud)
	upstream := &http.Server{Addr: cfg.UpstreamAddr, Handler: ts, ReadHeaderTimeout: 10 * time.Second}
	tun := newTunnel(cfg)
	web := &http.Server{Addr: cfg.WebAddr, Handler: webui.New(webui.Deps{
		Live: lm, Conn: conn, Aud: aud, Static: webUIFS(), Apply: apply,
		Adviser: adviser, Password: cfg.WebPassword, DBConnect: dbConnect, Registry: reg,
		ErpnextConnect: erpnextConnect, PrimaryLabel: cfg.ServerLabel, Version: version,
		Plan: webui.Plan{Plan: cfg.Plan, TrialLimitUSD: cfg.TrialLimitUSD,
			ContactEmail: cfg.ContactEmail, RequestURL: cfg.RequestURL},
		TunnelStatus: tun.Status,
		Settings: webui.SettingsInfo{
			Version: version, WebAddr: cfg.WebAddr, WebTLS: cfg.WebTLS,
			AuthEnabled: cfg.WebPassword != "", TunnelMode: cfg.TunnelMode,
			CredStore: cm.Label, LLMEnabled: adviser != nil,
			ERPNextMask: cfg.ERPNextMask, Plan: cfg.Plan,
		},
	}).Handler(), ReadHeaderTimeout: 10 * time.Second}

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
		if cfg.TunnelMode == "embed" {
			log.Printf("tunel modu: embed (surec-ici frp) baslatildi")
		} else {
			log.Printf("tunel (frpc sidecar) baslatildi")
		}
	} else {
		log.Printf("tunel KAPALI (MASHA_FRPC_CONFIG bos) — yalniz-yerel mod")
	}

	// Dokuman/RAG sync (3. bilgi kaynagi): DocsDir set ise dizini izle → metni cikar → (maskele) →
	// OWUI knowledge'a push. Arka-plan; ctx ile durur. Kimlik/durum YEREL (§3). Eksik config = uyar, gecme.
	if cfg.DocsDir != "" {
		if cfg.DocsOWUIURL == "" || cfg.DocsOWUIKey == "" || cfg.DocsKnowledgeID == "" {
			log.Printf("UYARI: MASHA_DOCS_DIR set ama OWUI_URL/KEY/KNOWLEDGE_ID eksik — dokuman sync ATLANDI")
		} else {
			st, err := documents.LoadState(cfg.DocsStateFile)
			if err != nil {
				log.Printf("UYARI: dokuman sync durumu okunamadi (%v) — bos durumla devam", err)
				st = documents.NewState(cfg.DocsStateFile)
			}
			syncer := &documents.Syncer{
				Dir: cfg.DocsDir, KnowledgeID: cfg.DocsKnowledgeID, Mask: cfg.DocsMask,
				Client: &documents.OWUIClient{BaseURL: cfg.DocsOWUIURL, APIKey: cfg.DocsOWUIKey, HTTP: http.DefaultClient},
				State:  st,
				Logf:   func(f string, a ...any) { log.Printf("[dokuman] "+f, a...) },
			}
			watcher := &documents.Watcher{Syncer: syncer, Logf: func(f string, a ...any) { log.Printf("[dokuman] "+f, a...) }}
			go watcher.Run(ctx)
			log.Printf("dokuman sync AKIF: %s → OWUI knowledge %s (maske=%v)", cfg.DocsDir, cfg.DocsKnowledgeID, cfg.DocsMask)
		}
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
