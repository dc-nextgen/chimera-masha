package mssql

import (
	"database/sql"
	"strings"
	"testing"

	"github.com/mehmetor/chimera-ai/stack/masha/agent/internal/manifest"
)

func testManifest() *manifest.Manifest {
	return &manifest.Manifest{
		Name: "t", DB: manifest.DBConfig{Driver: "sqlserver", ReadOnly: true},
		Entities: map[string]manifest.Entity{
			"invoice": {Table: "dbo.Invoices", Fields: map[string]manifest.Field{
				"id":       {Column: "InvoiceID"},
				"date":     {Column: "InvoiceDate", Expression: "format:date"},
				"customer": {Column: "CustomerName"},
				"total":    {Column: "Total", Expression: "format:money"},
			}},
		},
		Tools: []manifest.Tool{
			{Name: "count_invoices", Kind: "count", Entity: "invoice", Filters: []manifest.Filter{
				{Name: "start_date", Field: "date", Op: ">="},
				{Name: "end_date", Field: "date", Op: "<="},
			}},
			{Name: "list_invoices", Kind: "query", Entity: "invoice",
				Select: []string{"id", "date", "customer"}, Limit: 50,
				Filters: []manifest.Filter{{Name: "customer", Field: "customer", Op: "like"}}},
			{Name: "req_tool", Kind: "count", Entity: "invoice", Filters: []manifest.Filter{
				{Name: "cust", Field: "customer", Op: "=", Required: true},
			}},
		},
	}
}

func named(params []any) map[string]any {
	out := map[string]any{}
	for _, p := range params {
		if n, ok := p.(sql.NamedArg); ok {
			out[n.Name] = n.Value
		}
	}
	return out
}

func TestBuildCount(t *testing.T) {
	m := testManifest()
	tool, _ := m.Tool("count_invoices")
	q, params, out, err := BuildQuery(m, tool, map[string]any{"start_date": "2026-01-01"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(q, "SELECT COUNT(*) AS [n] FROM [dbo].[Invoices]") {
		t.Fatalf("count SQL beklenmedik: %s", q)
	}
	if !strings.Contains(q, "[InvoiceDate] >= @p0") {
		t.Fatalf("filtre WHERE eksik: %s", q)
	}
	if strings.Contains(q, "end") || strings.Contains(q, "@p1") {
		t.Fatalf("saglanmayan filtre eklendi: %s", q)
	}
	if named(params)["p0"] != "2026-01-01" {
		t.Fatalf("parametre yanlis: %v", named(params))
	}
	if len(out) != 1 || out[0].Field != "count" {
		t.Fatalf("count cikti kolonu yanlis: %v", out)
	}
}

func TestBuildQueryCols(t *testing.T) {
	m := testManifest()
	tool, _ := m.Tool("list_invoices")
	q, params, out, err := BuildQuery(m, tool, map[string]any{"customer": "Acme%"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(q, "SELECT TOP (@p1)") { // p0=customer filtre, p1=limit
		t.Fatalf("TOP parametresi eksik: %s", q)
	}
	if !strings.Contains(q, "[InvoiceID] AS [id]") || !strings.Contains(q, "[CustomerName] AS [customer]") {
		t.Fatalf("kolon eslemesi yanlis: %s", q)
	}
	if !strings.Contains(q, "[CustomerName] LIKE @p0") {
		t.Fatalf("like filtresi yanlis: %s", q)
	}
	np := named(params)
	if np["p0"] != "Acme%" || np["p1"] != 50 {
		t.Fatalf("parametreler yanlis: %v", np)
	}
	if len(out) != 3 || out[1].Expr != "format:date" {
		t.Fatalf("cikti kolonlari/expr yanlis: %+v", out)
	}
}

func TestRequiredFilterMissing(t *testing.T) {
	m := testManifest()
	tool, _ := m.Tool("req_tool")
	if _, _, _, err := BuildQuery(m, tool, map[string]any{}); err == nil {
		t.Fatal("zorunlu filtre eksikken hata donmedi")
	}
}

func TestQuoteIdentEscaping(t *testing.T) {
	if got := quoteIdent("a]b"); got != "[a]]b]" {
		t.Fatalf("bracket escape yanlis: %s", got)
	}
	if got := quoteTable("dbo.Faturalar"); got != "[dbo].[Faturalar]" {
		t.Fatalf("quoteTable yanlis: %s", got)
	}
}

func TestLimitCap(t *testing.T) {
	m := testManifest()
	tool, _ := m.Tool("list_invoices")
	tool.Limit = 999999
	_, params, _, err := BuildQuery(m, tool, nil)
	if err != nil {
		t.Fatal(err)
	}
	if named(params)["p0"] != maxLimit {
		t.Fatalf("limit cap uygulanmadi: %v", named(params))
	}
}
