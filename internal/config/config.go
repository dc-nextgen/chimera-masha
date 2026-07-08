// Package config — agent yapilandirmasi (env'den). Sir (DB kimligi) YERELDE kalir; Faz 3'te
// keychain'e tasinir. Bulut yalniz manifest (tanim) gonderir, kimlik ASLA.
package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	AppToken     string // MASHA_APP_TOKEN — bearer (OWUI tool-server key ile ayni). ZORUNLU.
	ServerLabel  string // MASHA_SERVER_LABEL — "server" segmenti / OWUI tool_ids oneki. default masha-db
	ManifestPath string // MASHA_MANIFEST — connector manifest.json. ZORUNLU (serve).
	DSN          string // MASHA_DB_DSN — sqlserver DSN. Yerel; bos ise ekrandan-baglan/dosya beklenir.
	DBCredFile   string // MASHA_DB_CRED_FILE — ekrandan girilen kimligin YEREL kalici konumu (0600).
	//                     default .masha-db.json (gitignored). Faz 3'te keychain'e tasinir (§18).
	UpstreamAddr string // MASHA_UPSTREAM_ADDR — tunelin forward ettigi dinleme adresi. default 127.0.0.1:9800
	WebAddr      string // MASHA_WEB_ADDR — yerel web yuz. default 127.0.0.1:8787 (yalniz kutudan)
	WebPassword  string // MASHA_WEB_PASSWORD — yerel yuz parolasi. bos + loopback = auth YOK (kutuda guvenli).
	//                     bos + LAN (0.0.0.0) = UYARI (auth onerilir; §17.9). Doluysa login zorunlu.
	WebTLS     bool   // MASHA_WEB_TLS — yerel yuz HTTPS (self-signed, oto-uretilir). LAN'da token'i sifreler.
	WebTLSDir  string // MASHA_WEB_TLS_DIR — cert/key konumu (yeniden-kullanilir). default .masha-webtls
	AuditFile  string // MASHA_AUDIT_LOG — hash-zincir audit dosyasi. bos => yalniz stdout
	Tenant     string // MASHA_TENANT — audit/telemetri etiketi
	FrpcBin    string // MASHA_FRPC_BIN — frpc ikili yolu. default "frpc"
	FrpcConfig string // MASHA_FRPC_CONFIG — frpc.toml. bos => tunel KAPALI (yalniz-yerel)
	// Onboarding LLM danismani (§5 sozlesme: base_url+api_key+model). Bos => LLM oneri KAPALI
	// (heuristik yeter). Semaya (yapisi; satir yok) dayanir → tablo siniflandirma + PII/isim onerisi.
	LLMBaseURL string // MASHA_LLM_BASE_URL — OpenAI-uyumlu uc (or. http://localhost:11434/v1)
	LLMAPIKey  string // MASHA_LLM_API_KEY
	LLMModel   string // MASHA_LLM_MODEL
	// ErpNext 2. baglanti (§19.1 Go-native connector). URL bos ise erpnext KAYITLANMAZ.
	// Kimlik YERELDE (buluta gitmez §3). MASHA_ERPNEXT_* > ERPNEXT_* (stack/.env uyumu).
	ERPNextURL      string
	ERPNextKey      string
	ERPNextSecret   string
	ERPNextLabel    string // server_label + OWUI tool_id. default "erpnext"
	ERPNextCredFile string // ekrandan girilen ErpNext kimliginin YEREL konumu. default .masha-erpnext.json
	ERPNextMask     bool   // MASHA_ERPNEXT_MASK — dokuman PII maskele (default ACIK; on-prem ERP privacy). 0=kapat
	// Plan / deneme (satis yuzeyi) — yerel yuz "Plan/Yukselt" ekrani. Bos plan = normal (ucretli/kurulu).
	// "trial" = deneme rozeti + limit gosterimi. Talep = musteri yukselt/iletisim (operatore ulasir).
	Plan          string // MASHA_PLAN — "" (normal) | "trial" (deneme)
	TrialLimitUSD string // MASHA_TRIAL_LIMIT_USD — deneme token limiti gosterimi (default "10")
	ContactEmail  string // MASHA_CONTACT_EMAIL — "Talep ilet" mailto hedefi (bos ise mailto gizli)
	RequestURL    string // MASHA_REQUEST_URL — opsiyonel bulut ucu (talep POST); bos ise yalniz mailto
}

func Load() (*Config, error) {
	c := &Config{
		AppToken:        env("MASHA_APP_TOKEN", ""),
		ServerLabel:     env("MASHA_SERVER_LABEL", "masha-db"),
		ManifestPath:    env("MASHA_MANIFEST", ""),
		DSN:             env("MASHA_DB_DSN", ""),
		DBCredFile:      env("MASHA_DB_CRED_FILE", ".masha-db.json"),
		UpstreamAddr:    env("MASHA_UPSTREAM_ADDR", "127.0.0.1:9800"),
		WebAddr:         env("MASHA_WEB_ADDR", "127.0.0.1:8787"),
		WebPassword:     env("MASHA_WEB_PASSWORD", ""),
		WebTLS:          truthy(env("MASHA_WEB_TLS", "")),
		WebTLSDir:       env("MASHA_WEB_TLS_DIR", ".masha-webtls"),
		AuditFile:       env("MASHA_AUDIT_LOG", ""),
		Tenant:          env("MASHA_TENANT", ""),
		FrpcBin:         env("MASHA_FRPC_BIN", "frpc"),
		FrpcConfig:      env("MASHA_FRPC_CONFIG", ""),
		LLMBaseURL:      env("MASHA_LLM_BASE_URL", ""),
		LLMAPIKey:       env("MASHA_LLM_API_KEY", ""),
		LLMModel:        env("MASHA_LLM_MODEL", ""),
		ERPNextURL:      envOr("MASHA_ERPNEXT_URL", "ERPNEXT_URL"),
		ERPNextKey:      envOr("MASHA_ERPNEXT_API_KEY", "ERPNEXT_API_KEY"),
		ERPNextSecret:   envOr("MASHA_ERPNEXT_API_SECRET", "ERPNEXT_API_SECRET"),
		ERPNextLabel:    env("MASHA_ERPNEXT_LABEL", "erpnext"),
		ERPNextCredFile: env("MASHA_ERPNEXT_CRED_FILE", ".masha-erpnext.json"),
		ERPNextMask:     envBool("MASHA_ERPNEXT_MASK", true),
		Plan:            env("MASHA_PLAN", ""),
		TrialLimitUSD:   env("MASHA_TRIAL_LIMIT_USD", "10"),
		ContactEmail:    env("MASHA_CONTACT_EMAIL", ""),
		RequestURL:      env("MASHA_REQUEST_URL", ""),
	}
	return c, nil
}

// envBool — set edilmemiş/boş ise def; aksi halde truthy(v).
func envBool(k string, def bool) bool {
	v, ok := os.LookupEnv(k)
	if !ok || v == "" {
		return def
	}
	return truthy(v)
}

// envOr — ilk isim boşsa ikinciye düş (MASHA_ERPNEXT_* > ERPNEXT_*).
func envOr(primary, fallback string) string {
	if v := env(primary, ""); v != "" {
		return v
	}
	return env(fallback, "")
}

// RequireServe — serve icin zorunlu alanlar. Kimlik-doğrulamasiz acilmaz (app.py deseni).
func (c *Config) RequireServe() error {
	var miss []string
	if strings.TrimSpace(c.AppToken) == "" {
		miss = append(miss, "MASHA_APP_TOKEN")
	}
	if strings.TrimSpace(c.ManifestPath) == "" {
		miss = append(miss, "MASHA_MANIFEST")
	}
	if len(miss) > 0 {
		return fmt.Errorf("eksik zorunlu env: %s", strings.Join(miss, ", "))
	}
	return nil
}

func env(k, def string) string {
	if v, ok := os.LookupEnv(k); ok {
		return v
	}
	return def
}

// truthy — "1"/"true"/"yes"/"on" (buyuk-kucuk duyarsiz) → true.
func truthy(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}
