// Package mssql — SQL Server connector: manifest'ten PARAMETRELI SELECT uretir + calistirir.
//
// GUVENLIK: tanimlayici (tablo/kolon) YALNIZ manifest'ten gelir (operator-authored, kullanici
// girdisi DEGIL) ve yine de bracket-quote edilir. KULLANICI degeri ASLA SQL'e gomulmez — daima
// sql.Named parametresi olarak baglanir. Agent serbest SQL calistirmaz (§17.7: ham execute_sql yok).
package mssql

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/dc-nextgen/chimera-masha/internal/manifest"
)

// OutCol — sonuc sekillendirme + expression icin cikti kolonu tanimi.
type OutCol struct {
	Field string // mantiksal alan adi (JSON anahtari)
	Expr  string // manifest field.Expression (maske/format)
}

const defaultLimit = 100
const maxLimit = 1000

// BuildQuery — tool + args'tan (SQL metni, parametreler, cikti kolonlari) uretir.
func BuildQuery(m *manifest.Manifest, t *manifest.Tool, args map[string]any) (string, []any, []OutCol, error) {
	ent, ok := m.Entities[t.Entity]
	if !ok {
		return "", nil, nil, fmt.Errorf("entity yok: %q", t.Entity)
	}
	var params []any
	pIdx := 0
	nextParam := func(v any) string {
		name := fmt.Sprintf("p%d", pIdx)
		pIdx++
		params = append(params, sql.Named(name, v))
		return "@" + name
	}

	// WHERE — yalniz saglanan (ya da required) filtreler.
	var where []string
	for _, f := range t.Filters {
		v, present := args[f.Name]
		if !present || v == nil || v == "" {
			if f.Required {
				return "", nil, nil, fmt.Errorf("zorunlu filtre eksik: %q", f.Name)
			}
			continue
		}
		fld := ent.Fields[f.Field] // Validate garanti eder var
		ph := nextParam(v)
		where = append(where, fmt.Sprintf("%s %s %s", quoteIdent(fld.Column), sqlOp(f.Op), ph))
	}
	whereSQL := ""
	if len(where) > 0 {
		whereSQL = " WHERE " + strings.Join(where, " AND ")
	}
	table := quoteTable(ent.Table)

	switch t.Kind {
	case "count":
		q := fmt.Sprintf("SELECT COUNT(*) AS [n] FROM %s%s", table, whereSQL)
		return q, params, []OutCol{{Field: "count"}}, nil

	case "query":
		limit := t.Limit
		if limit <= 0 {
			limit = defaultLimit
		}
		if limit > maxLimit {
			limit = maxLimit
		}
		limPh := nextParam(limit)
		var cols []string
		var out []OutCol
		for _, sf := range t.Select {
			fld := ent.Fields[sf]
			cols = append(cols, fmt.Sprintf("%s AS %s", quoteIdent(fld.Column), quoteIdent(sf)))
			out = append(out, OutCol{Field: sf, Expr: fld.Expression})
		}
		q := fmt.Sprintf("SELECT TOP (%s) %s FROM %s%s", limPh, strings.Join(cols, ", "), table, whereSQL)
		return q, params, out, nil

	default:
		return "", nil, nil, fmt.Errorf("desteklenmeyen tool kind: %q", t.Kind)
	}
}

// sqlOp — manifest op'unu SQL'e cevirir. Validate zaten allowlist uyguladi.
func sqlOp(op string) string {
	if op == "like" {
		return "LIKE"
	}
	return op
}

// quoteTable — "dbo.Faturalar" -> "[dbo].[Faturalar]". Nokta ile bolunur, her parca quote.
func quoteTable(t string) string {
	parts := strings.Split(t, ".")
	for i, p := range parts {
		parts[i] = quoteIdent(p)
	}
	return strings.Join(parts, ".")
}

// quoteIdent — MSSQL bracket-quote; ']' -> ']]' escape (identifier injection tabanina karsi).
func quoteIdent(s string) string {
	return "[" + strings.ReplaceAll(s, "]", "]]") + "]"
}
