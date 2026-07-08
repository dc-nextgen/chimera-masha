// Package onboard — "veritabanini anla → MCP uret".
//
// Sema YAPISINDAN (connector.Schema; SATIR YOK) bir ADAY manifest uretir: tablo→entity,
// kolon→field, count_/list_ tipli araclar + kolon-adi/tip heuristigiyle PII maske / tarih-para format.
// Bu ADAYDIR — operator konsolu duzenler/onaylar (§17.7.2 "LLM/heuristik onerir, operator onaylar").
// Ciktı manifest.Validate()'ten GECER (fail-closed uretim: gecersiz aday sunmayiz).
package onboard

import (
	"regexp"
	"strings"

	"github.com/dc-nextgen/chimera-masha/internal/connector"
	"github.com/dc-nextgen/chimera-masha/internal/expression"
	"github.com/dc-nextgen/chimera-masha/internal/manifest"
)

// Selection — operatorun onboarding secimi. Tables bos => tum tablolar.
type Selection struct {
	Name   string   // manifest slug (bos => "connector")
	Label  string   // gorunen etiket (bos => Name)
	Tables []string // dahil edilecek "schema.table" kimlikleri; bos => hepsi
}

// Suggest — semadan ADAY manifest. Deterministik (tablo/kolon sirasi semadan) → test edilebilir.
func Suggest(sc *connector.Schema, sel Selection) *manifest.Manifest {
	name := slug(sel.Name)
	if name == "" {
		name = "connector"
	}
	label := strings.TrimSpace(sel.Label)
	if label == "" {
		label = name + " (salt-okunur)"
	}
	include := map[string]bool{}
	for _, t := range sel.Tables {
		include[strings.ToLower(strings.TrimSpace(t))] = true
	}

	m := &manifest.Manifest{
		Name:     name,
		Label:    label,
		ERPKind:  "mssql-generic",
		DB:       manifest.DBConfig{Driver: "sqlserver", ReadOnly: true},
		Entities: map[string]manifest.Entity{},
		Tools:    []manifest.Tool{},
	}

	usedEntity := map[string]bool{}
	for _, tbl := range sc.Tables {
		qual := strings.ToLower(tbl.Schema + "." + tbl.Name)
		if len(include) > 0 && !include[qual] {
			continue
		}
		if len(tbl.Columns) == 0 {
			continue // secilemeyen alan yok => tool uretilemez
		}
		ent := uniqueKey(slug(tbl.Name), usedEntity)
		usedEntity[ent] = true

		fields := map[string]manifest.Field{}
		var order []string // list select icin deterministik alan sirasi
		usedField := map[string]bool{}
		for _, c := range tbl.Columns {
			fn := uniqueKey(slug(c.Name), usedField)
			usedField[fn] = true
			fields[fn] = manifest.Field{Column: c.Name, Expression: exprFor(c.Name, c.Type)}
			order = append(order, fn)
		}
		m.Entities[ent] = manifest.Entity{Table: tbl.Schema + "." + tbl.Name, Fields: fields}

		human := humanize(tbl.Name)
		m.Tools = append(m.Tools,
			manifest.Tool{Name: "count_" + ent, Kind: "count", Entity: ent,
				Description: human + " kayit sayisini dondurur."},
			manifest.Tool{Name: "list_" + ent, Kind: "query", Entity: ent,
				Description: human + " kayitlarini listeler.", Select: order, Limit: 100},
		)
	}
	return m
}

// ── heuristikler ──────────────────────────────────────────────────────────

// exprFor — kolon adi + tipinden opsiyonel expression (expression.Apply'in destekledigi degerler).
// Ad-tabanli PII maskesi tip-tabanli formattan ONCE (kolonun TC oldugunu bilmek deterministik, §17.7.3).
func exprFor(col, typ string) string {
	if m := expression.MaskForField(col); m != "" { // PII maske: ortak heuristik (expression paketi)
		return m
	}
	n := strings.ToLower(col)
	switch strings.ToLower(strings.TrimSpace(typ)) {
	case "date":
		return "format:date"
	case "datetime", "datetime2", "smalldatetime", "datetimeoffset":
		return "format:datetime"
	case "money", "smallmoney":
		return "format:money"
	case "decimal", "numeric", "float", "real":
		if containsAny(n, "tutar", "fiyat", "price", "amount", "bakiye", "odeme", "tahsilat", "borc", "alacak", "toplam", "total") {
			return "format:money"
		}
	}
	return ""
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

var reNonWord = regexp.MustCompile(`[^a-z0-9]+`)

// slug — kucuk harf, [a-z0-9] disi → "_", kenar "_" kirp. Bos girdi => "".
func slug(s string) string {
	s = reNonWord.ReplaceAllString(strings.ToLower(strings.TrimSpace(s)), "_")
	return strings.Trim(s, "_")
}

// humanize — tablo adindan okunabilir etiket (slug'i bosluklu, bas-harf buyuk).
func humanize(s string) string {
	parts := strings.Fields(strings.ReplaceAll(slug(s), "_", " "))
	for i, p := range parts {
		if p != "" {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	if len(parts) == 0 {
		return s
	}
	return strings.Join(parts, " ")
}

// uniqueKey — base zaten kullanildiysa _2, _3... ekle (entity/field carbismasi).
func uniqueKey(base string, used map[string]bool) string {
	if base == "" {
		base = "x"
	}
	if !used[base] {
		return base
	}
	for i := 2; ; i++ {
		cand := base + "_" + itoa(i)
		if !used[cand] {
			return cand
		}
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}
