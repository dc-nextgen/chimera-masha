package manifest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func valid() *Manifest {
	return &Manifest{
		Name:    "t",
		Label:   "T",
		ERPKind: "mssql-generic",
		DB:      DBConfig{Driver: "sqlserver", ReadOnly: true},
		Entities: map[string]Entity{
			"invoice": {Table: "dbo.Invoices", Fields: map[string]Field{
				"id":   {Column: "InvoiceID"},
				"date": {Column: "InvoiceDate", Expression: "format:date"},
			}},
		},
		Tools: []Tool{
			{Name: "count_invoices", Kind: "count", Entity: "invoice",
				Filters: []Filter{{Name: "start", Field: "date", Op: ">="}}},
			{Name: "list_invoices", Kind: "query", Entity: "invoice", Select: []string{"id", "date"}},
		},
	}
}

func TestValidateOK(t *testing.T) {
	if err := valid().Validate(); err != nil {
		t.Fatalf("gecerli manifest reddedildi: %v", err)
	}
}

func TestValidateReadOnlyRequired(t *testing.T) {
	m := valid()
	m.DB.ReadOnly = false
	if err := m.Validate(); err == nil {
		t.Fatal("read_only=false kabul edildi (salt-okunur zorunlu)")
	}
}

func TestValidateDriver(t *testing.T) {
	m := valid()
	m.DB.Driver = "postgres"
	if err := m.Validate(); err == nil {
		t.Fatal("driver=postgres kabul edildi")
	}
}

func TestValidateUnknownEntity(t *testing.T) {
	m := valid()
	m.Tools[0].Entity = "yok"
	if err := m.Validate(); err == nil {
		t.Fatal("bilinmeyen entity kabul edildi")
	}
}

func TestValidateBadOp(t *testing.T) {
	m := valid()
	m.Tools[0].Filters[0].Op = "; drop"
	if err := m.Validate(); err == nil {
		t.Fatal("gecersiz op kabul edildi")
	}
}

func TestValidateSelectFieldMissing(t *testing.T) {
	m := valid()
	m.Tools[1].Select = []string{"id", "yok_alan"}
	if err := m.Validate(); err == nil {
		t.Fatal("bilinmeyen select alani kabul edildi")
	}
}

func TestValidateFilterFieldMissing(t *testing.T) {
	m := valid()
	m.Tools[0].Filters[0].Field = "yok"
	if err := m.Validate(); err == nil {
		t.Fatal("bilinmeyen filtre alani kabul edildi")
	}
}

func TestValidateDupToolName(t *testing.T) {
	m := valid()
	m.Tools[1].Name = "count_invoices"
	if err := m.Validate(); err == nil {
		t.Fatal("tekrar eden tool adi kabul edildi")
	}
}

func TestLoadExample(t *testing.T) {
	m, err := Load("../../example/manifest.json")
	if err != nil {
		t.Fatalf("ornek manifest yuklenemedi: %v", err)
	}
	if len(m.Tools) != 2 {
		t.Fatalf("beklenen 2 tool, goren %d", len(m.Tools))
	}
	if _, ok := m.Tool("count_invoices"); !ok {
		t.Fatal("count_invoices tool bulunamadi")
	}
}

// Empty = taze kurulum baslangici: 0 arac, sqlserver+read_only; Validate'den GECMEZ (yayin degil).
func TestEmptyStartupManifest(t *testing.T) {
	m := Empty("musteri-db")
	if m.Name != "musteri-db" || len(m.Tools) != 0 || len(m.Entities) != 0 {
		t.Fatalf("bos manifest beklenmedik: %+v", m)
	}
	if m.DB.Driver != "sqlserver" || !m.DB.ReadOnly {
		t.Fatalf("bos manifest DB yanlis: %+v", m.DB)
	}
	if err := m.Validate(); err == nil {
		t.Fatal("bos manifest Validate'den GECMEMELI (>=1 tool); yalniz startup icin gecerli")
	}
	if Empty("").Name != "connector" {
		t.Fatal("label bos → 'connector' varsayilani")
	}
}

// LoadOrEmpty: dosya YOKSA bos (hatasiz); VARSA Load (Validate'li).
func TestLoadOrEmpty(t *testing.T) {
	// yok → bos, hata yok (tavuk-yumurta cozumu)
	m, err := LoadOrEmpty(filepath.Join(t.TempDir(), "yok.json"), "lbl")
	if err != nil || len(m.Tools) != 0 || m.Name != "lbl" {
		t.Fatalf("absent → bos(lbl) beklendi: %+v err=%v", m, err)
	}
	// path bos → bos
	if m2, err := LoadOrEmpty("", "x"); err != nil || len(m2.Tools) != 0 {
		t.Fatalf("bos path → bos beklendi: %+v err=%v", m2, err)
	}
	// var + gecerli → Load
	p := filepath.Join(t.TempDir(), "var.json")
	b, _ := json.Marshal(valid())
	if err := os.WriteFile(p, b, 0o600); err != nil {
		t.Fatal(err)
	}
	m3, err := LoadOrEmpty(p, "lbl")
	if err != nil || len(m3.Tools) != 2 {
		t.Fatalf("present+valid → Load(2 tool) beklendi: %+v err=%v", m3, err)
	}
}
