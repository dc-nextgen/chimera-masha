package onboard

import (
	"testing"

	"github.com/mehmetor/chimera-ai/stack/masha/agent/internal/connector"
	"github.com/mehmetor/chimera-ai/stack/masha/agent/internal/manifest"
)

func keys(m map[string]manifest.Entity) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func sampleSchema() *connector.Schema {
	return &connector.Schema{Tables: []connector.Table{
		{Schema: "dbo", Name: "Participants", Columns: []connector.Column{
			{Name: "Id", Type: "int"},
			{Name: "FullName", Type: "nvarchar"},
			{Name: "Email", Type: "nvarchar"},
			{Name: "Telefon", Type: "varchar"},
			{Name: "TCKN", Type: "char"},
			{Name: "KayitTarihi", Type: "datetime"},
		}},
		{Schema: "dbo", Name: "Invoices", Columns: []connector.Column{
			{Name: "Id", Type: "int"},
			{Name: "Tutar", Type: "decimal"},
			{Name: "Tarih", Type: "date"},
			{Name: "IBAN", Type: "varchar"},
		}},
		{Schema: "dbo", Name: "Empty", Columns: nil}, // kolonsuz → atlanmali
	}}
}

func TestSuggestValidates(t *testing.T) {
	m := Suggest(sampleSchema(), Selection{Name: "Demo DB", Label: "Demo"})
	if err := m.Validate(); err != nil {
		t.Fatalf("uretilen manifest Validate gecmedi: %v", err)
	}
	if m.Name != "demo_db" {
		t.Errorf("name slug beklenen demo_db, goren %q", m.Name)
	}
	if m.DB.Driver != "sqlserver" || !m.DB.ReadOnly {
		t.Errorf("db config yanlis: %+v", m.DB)
	}
	// Empty tablo atlandi → 2 entity (participants, invoices).
	if len(m.Entities) != 2 {
		t.Errorf("2 entity bekleniyordu, goren %d: %v", len(m.Entities), keys(m.Entities))
	}
	// entity basi count+list → 4 tool.
	if len(m.Tools) != 4 {
		t.Errorf("4 tool bekleniyordu, goren %d", len(m.Tools))
	}
}

func TestPIIandFormatHeuristics(t *testing.T) {
	m := Suggest(sampleSchema(), Selection{Name: "x"})
	p, ok := m.Entities["participants"]
	if !ok {
		t.Fatalf("participants entity yok: %v", keys(m.Entities))
	}
	want := map[string]string{
		"email":       "mask:email",
		"telefon":     "mask:phone",
		"tckn":        "mask:tckn",
		"kayittarihi": "format:datetime",
		"id":          "",
		"fullname":    "",
	}
	for f, exp := range want {
		got := p.Fields[f].Expression
		if got != exp {
			t.Errorf("field %q expression: beklenen %q, goren %q", f, exp, got)
		}
	}
	inv := m.Entities["invoices"]
	if inv.Fields["tutar"].Expression != "format:money" {
		t.Errorf("tutar → format:money bekleniyordu, goren %q", inv.Fields["tutar"].Expression)
	}
	if inv.Fields["tarih"].Expression != "format:date" {
		t.Errorf("tarih (date) → format:date bekleniyordu, goren %q", inv.Fields["tarih"].Expression)
	}
	if inv.Fields["iban"].Expression != "mask:iban" {
		t.Errorf("iban → mask:iban bekleniyordu, goren %q", inv.Fields["iban"].Expression)
	}
}

func TestSelectionFiltersTables(t *testing.T) {
	m := Suggest(sampleSchema(), Selection{Name: "x", Tables: []string{"dbo.Invoices"}})
	if len(m.Entities) != 1 {
		t.Fatalf("secim ile 1 entity bekleniyordu, goren %d: %v", len(m.Entities), keys(m.Entities))
	}
	if _, ok := m.Entities["invoices"]; !ok {
		t.Errorf("invoices bekleniyordu, goren %v", keys(m.Entities))
	}
	// list tool select = tum alanlar (4 kolon).
	for _, tl := range m.Tools {
		if tl.Kind == "query" && len(tl.Select) != 4 {
			t.Errorf("list select 4 alan bekliyordu, goren %d", len(tl.Select))
		}
	}
}

func TestListSelectNonEmptyForValidate(t *testing.T) {
	// Query tool select bos olamaz (Validate). Tek-kolonlu tablo bile gecmeli.
	sc := &connector.Schema{Tables: []connector.Table{
		{Schema: "dbo", Name: "One", Columns: []connector.Column{{Name: "OnlyCol", Type: "int"}}},
	}}
	m := Suggest(sc, Selection{Name: "x"})
	if err := m.Validate(); err != nil {
		t.Fatalf("tek-kolon manifest Validate gecmedi: %v", err)
	}
}

