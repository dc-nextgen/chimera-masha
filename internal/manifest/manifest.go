// Package manifest — MCP TANIMI (bulutta operator-eli uretilir, kutuya iner).
//
// setup/connectors/<name>/manifest.json desenini DB connector icin genisletir:
// entities (tablo->is-varligi eslemesi) + tools (tipli arac yuzu) + per-alan expression.
// Bu dosya connector'un TEK "ne yapabilir" kaynagidir; agent asla serbest SQL calistirmaz,
// yalniz burada tanimli araclardan PARAMETRELI SELECT uretir.
package manifest

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// izinli filtre operatorleri (fail-closed: liste disi = red). Hepsi salt-okuma.
var allowedOps = map[string]bool{
	"=": true, "!=": true, ">": true, ">=": true, "<": true, "<=": true, "like": true,
}

// izinli arac turleri.
var allowedKinds = map[string]bool{"count": true, "query": true}

type Manifest struct {
	Name     string            `json:"name"`
	Label    string            `json:"label"`
	ERPKind  string            `json:"erp_kind"` // "mssql-generic" | "ibs" | ...
	Prompt   string            `json:"prompt,omitempty"`
	DB       DBConfig          `json:"db"`
	Entities map[string]Entity `json:"entities"`
	Tools    []Tool            `json:"tools"`
}

type DBConfig struct {
	Driver   string `json:"driver"`    // "sqlserver"
	ReadOnly bool   `json:"read_only"` // her zaman true olmali (asagida dogrulanir)
}

// Entity = bir is-varliginin (Fatura, Cari...) tabloya + alan eslemesine baglanmasi.
type Entity struct {
	Table  string           `json:"table"` // "dbo.Faturalar" — operator-authored, kullanici girdisi DEGIL
	Fields map[string]Field `json:"fields"`
}

// Field = mantiksal alan adi -> gercek kolon + opsiyonel donusum (maske/format).
type Field struct {
	Column     string `json:"column"`
	Expression string `json:"expression,omitempty"` // "mask:tckn" | "format:date" | "format:money" | ...
}

// Tool = buluta (OWUI'ye) sunulan TIPLI arac. kind=count|query.
type Tool struct {
	Name        string   `json:"name"`
	Kind        string   `json:"kind"`
	Entity      string   `json:"entity"`
	Description string   `json:"description"`
	Select      []string `json:"select,omitempty"` // query: dondurulecek alan adlari
	Filters     []Filter `json:"filters,omitempty"`
	Limit       int      `json:"limit,omitempty"` // query: ust sinir (0 => default 100)
}

// Filter = arac parametresi -> alan + operator. Deger PARAMETRE olarak baglanir (SQL'e gomulmez).
type Filter struct {
	Name     string `json:"name"`  // parametre adi (OWUI'nin gonderdigi)
	Field    string `json:"field"` // entity field adi
	Op       string `json:"op"`
	Required bool   `json:"required,omitempty"`
}

// Load bir manifest.json okur ve dogrular.
func Load(path string) (*Manifest, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("manifest okunamadi: %w", err)
	}
	var m Manifest
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("manifest JSON gecersiz: %w", err)
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

// Empty — taze kurulum icin BOS baslangic manifest'i (0 arac). Validate'den GECMEZ (>=1 tool ister)
// ama STARTUP icin gecerli: web yuz + Bağlan + wizard calisir, tool-server 0 arac sunar. Musteri
// wizard'la DB baglar → ilk apply gecerli manifest'i (>=1 tool) YAZAR (Validate orada uygulanir).
func Empty(label string) *Manifest {
	if label == "" {
		label = "connector"
	}
	return &Manifest{
		Name:     label,
		Label:    label,
		DB:       DBConfig{Driver: "sqlserver", ReadOnly: true},
		Entities: map[string]Entity{},
		Tools:    []Tool{},
	}
}

// LoadOrEmpty — dosya VARSA Load (Validate'li); YOKSA bos-baslangic manifest'i (taze kurulum/deneme).
// Deneme onboarding'in tavuk-yumurta'sini cozer: agent manifest olmadan acilir, wizard doldurur.
func LoadOrEmpty(path, label string) (*Manifest, error) {
	if path == "" {
		return Empty(label), nil
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return Empty(label), nil
	}
	return Load(path)
}

// Validate — connector'un guvenlik/dogruluk on-kosulu. Basarisizsa agent araci sunmaz
// (fail-closed): eksik/yanlis eslemede uydurma alan/tablo riski (docs §17.7).
func (m *Manifest) Validate() error {
	if m.Name == "" {
		return fmt.Errorf("manifest: name bos")
	}
	if m.DB.Driver != "sqlserver" {
		return fmt.Errorf("manifest: yalniz driver=sqlserver destekli (goren: %q)", m.DB.Driver)
	}
	if !m.DB.ReadOnly {
		return fmt.Errorf("manifest: db.read_only=false — salt-okunur zorunlu (§4)")
	}
	if len(m.Tools) == 0 {
		return fmt.Errorf("manifest: hic tool yok")
	}
	seen := map[string]bool{}
	for i := range m.Tools {
		t := &m.Tools[i]
		if t.Name == "" {
			return fmt.Errorf("manifest: tool[%d] name bos", i)
		}
		if seen[t.Name] {
			return fmt.Errorf("manifest: tool adi tekrar: %q", t.Name)
		}
		seen[t.Name] = true
		if !allowedKinds[t.Kind] {
			return fmt.Errorf("manifest: tool %q kind gecersiz: %q (count|query)", t.Name, t.Kind)
		}
		ent, ok := m.Entities[t.Entity]
		if !ok {
			return fmt.Errorf("manifest: tool %q bilinmeyen entity: %q", t.Name, t.Entity)
		}
		if ent.Table == "" {
			return fmt.Errorf("manifest: entity %q table bos", t.Entity)
		}
		for _, sf := range t.Select {
			if _, ok := ent.Fields[sf]; !ok {
				return fmt.Errorf("manifest: tool %q select alani %q entity'de yok", t.Name, sf)
			}
		}
		if t.Kind == "query" && len(t.Select) == 0 {
			return fmt.Errorf("manifest: query tool %q select bos", t.Name)
		}
		for _, f := range t.Filters {
			if f.Name == "" {
				return fmt.Errorf("manifest: tool %q filtre adi bos", t.Name)
			}
			if !allowedOps[f.Op] {
				return fmt.Errorf("manifest: tool %q filtre %q op gecersiz: %q", t.Name, f.Name, f.Op)
			}
			if _, ok := ent.Fields[f.Field]; !ok {
				return fmt.Errorf("manifest: tool %q filtre %q alani %q entity'de yok", t.Name, f.Name, f.Field)
			}
		}
	}
	return nil
}

// Tool adiyla bul.
func (m *Manifest) Tool(name string) (*Tool, bool) {
	for i := range m.Tools {
		if m.Tools[i].Name == name {
			return &m.Tools[i], true
		}
	}
	return nil, false
}

// ToolNames — allowlist icin.
func (m *Manifest) ToolNames() []string {
	out := make([]string, 0, len(m.Tools))
	for _, t := range m.Tools {
		out = append(out, t.Name)
	}
	return out
}
