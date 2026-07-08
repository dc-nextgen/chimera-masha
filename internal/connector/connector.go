// Package connector — DB connector soyutlamasi (parametrik; §1). Bugun mssql; yarin ibs/baska.
// Ust katman (toolserver) yalniz bu arayuze bakar.
package connector

import "context"

// Connector — manifest-tanimli tipli araclari calistiran + semayi introspekte eden arka-uc.
type Connector interface {
	// Call bir tool'u args ile calistirir; sonuc JSON-serializable (map/rows). Salt-okuma.
	Call(ctx context.Context, tool string, args map[string]any) (any, error)
	// Introspect sema YAPISINI dondurur (tablo/kolon/tip — SATIR YOK). Onboarding icin.
	Introspect(ctx context.Context) (*Schema, error)
	// Health baglanti canli mi.
	Health(ctx context.Context) error
	// Connected — DB kimligi ayarli mi (ucuz; ping YOK). Ekrandan-baglan akisi icin.
	Connected() bool
	// Connect — YENI kimlikle bagla (ping ile dogrula, basarilıysa swap). Ekrandan-baglan.
	Connect(ctx context.Context, dsn string) error
	// OpenAPI — bu connector'un tool yuzeyi (OWUI'nin okudugu spec). mssql=manifest'ten; erpnext-proxy=mcpo'dan.
	OpenAPI(serverLabel string) map[string]any
	// AllowTool — tool bu connector'da var + salt-okuma mi (fail-closed; yazma-fiili reddi). Registry dispatch'i icin.
	AllowTool(tool string) bool
	Close() error
}

// DBFields — ekrandan girilen DB baglanti bilgileri (kimlik YERELDE kalir, §3). DSN'e cevrilir.
type DBFields struct {
	Host            string `json:"host"`
	Port            string `json:"port,omitempty"` // bos => 1433
	Database        string `json:"database"`
	User            string `json:"user"`
	Password        string `json:"password"`
	Encrypt         string `json:"encrypt,omitempty"`           // "true"|"false"|"disable" (bos => true)
	TrustServerCert bool   `json:"trust_server_cert,omitempty"` // self-signed DB cert kabul
}

// Schema — introspeksiyon ciktisi (yalniz yapi; veri yok).
type Schema struct {
	Tables []Table `json:"tables"`
}

type Table struct {
	Schema  string   `json:"schema"`
	Name    string   `json:"name"`
	Columns []Column `json:"columns"`
}

type Column struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
}
