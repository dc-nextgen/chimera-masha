// LLM danismani (Faz 2.4) — sema YAPISINDAN (satir YOK) tablo siniflandirma + PII/isim onerisi.
// §5 sozlesme: {base_url, api_key, model} (OpenAI-uyumlu). Opsiyonel: yoksa heuristik yeter.
// Yetki-farkindaligi (kullanici 2026-07-06): user/permission/audit tablolarini FLAG'ler → operatore
// "bu tabloyu acmak erisim-kontrol/kimlik verisi acar" uyarisi. AI zorlamaz, DANISIR (§4).
package onboard

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/dc-nextgen/chimera-masha/internal/connector"
)

// FieldAdvice — kolon icin onerilen expression (maske/format; bos=donusum yok).
type FieldAdvice struct {
	Column     string `json:"column"`
	Expression string `json:"expression"`
}

// TableAdvice — bir tablo icin LLM degerlendirmesi (danisma amacli).
type TableAdvice struct {
	Table     string        `json:"table"` // "schema.name"
	Entity    string        `json:"entity"`
	Kind      string        `json:"kind"`      // business|user|permission|audit|lookup|other
	Sensitive bool          `json:"sensitive"` // PII veya erisim-kontrol verisi
	Note      string        `json:"note"`
	Fields    []FieldAdvice `json:"fields,omitempty"`
}

var adviceKinds = map[string]bool{
	"business": true, "user": true, "permission": true, "audit": true, "lookup": true, "other": true,
}

// validExpr — expression.Apply'in destekledigi degerler (llm.go bunlara kisitlar; digerleri duser).
var validExpr = map[string]bool{
	"mask:email": true, "mask:phone": true, "mask:tckn": true, "mask:iban": true, "mask:card": true,
	"format:date": true, "format:datetime": true, "format:money": true,
}

// Suggester — OpenAI-uyumlu LLM istemcisi (onboarding danismani).
type Suggester struct {
	baseURL string
	apiKey  string
	model   string
	hc      *http.Client
}

// NewSuggester — baseURL+model bos ise nil (LLM oneri kapali; cagiran 501 doner).
func NewSuggester(baseURL, apiKey, model string) *Suggester {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" || strings.TrimSpace(model) == "" {
		return nil
	}
	return &Suggester{baseURL: baseURL, apiKey: apiKey, model: strings.TrimSpace(model),
		hc: &http.Client{Timeout: 90 * time.Second}}
}

const (
	maxAdviseTables  = 80
	maxAdviseColumns = 40
)

// Advise — semayi LLM'e siniflandirtir. Sadece YAPI gonderilir (tablo/kolon/tip; satir YOK).
func (s *Suggester) Advise(ctx context.Context, sc *connector.Schema) ([]TableAdvice, error) {
	sys := "Sen bir SQL Server sema danismanisin. Sana bir veritabaninin YALNIZ YAPISI verilir " +
		"(tablo + kolon + tip; VERI/satir YOK). Her tabloyu degerlendirip YALNIZ JSON dondur."
	user := "Asagidaki sema icin her tabloyu siniflandir. Cikti tam olarak bu JSON:\n" +
		`{"tables":[{"table":"schema.name","entity":"kisa_slug","kind":"business|user|permission|audit|lookup|other",` +
		`"sensitive":true,"note":"kisa Turkce","fields":[{"column":"Ad","expression":"mask:email|mask:phone|mask:tckn|mask:iban|mask:card|format:date|format:datetime|format:money|"}]}]}` +
		"\nKurallar: YALNIZ verilen kolon adlarina dayan, UYDURMA. sensitive=true eger PII (kisisel veri) " +
		"VEYA erisim-kontrol/kimlik verisi iceriyorsa. kind: kullanici/kimlik=user, rol/izin/yetki=permission, " +
		"log/audit=audit, sabit-liste=lookup, ana is verisi=business. PII kolonu→uygun mask; tarih/para→format; " +
		"yoksa bos string. Aciklama YAZMA, yalniz JSON.\n\nSema:\n" + schemaText(sc)

	body, _ := json.Marshal(map[string]any{
		"model":       s.model,
		"temperature": 0,
		"stream":      false,
		"messages": []map[string]string{
			{"role": "system", "content": sys},
			{"role": "user", "content": user},
		},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if s.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.apiKey)
	}
	resp, err := s.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("LLM cagrisi basarisiz: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LLM HTTP %d: %s", resp.StatusCode, truncate(string(raw), 160))
	}
	var cr struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &cr); err != nil || len(cr.Choices) == 0 {
		return nil, fmt.Errorf("LLM yaniti okunamadi")
	}
	return parseAdvice(cr.Choices[0].Message.Content)
}

// parseAdvice — model ciktisindan JSON'i cikar + sanitize (kind/expression/entity dogrula).
func parseAdvice(content string) ([]TableAdvice, error) {
	js := extractJSON(content)
	if js == "" {
		return nil, fmt.Errorf("LLM JSON dondurmedi")
	}
	var out struct {
		Tables []TableAdvice `json:"tables"`
	}
	if err := json.Unmarshal([]byte(js), &out); err != nil {
		return nil, fmt.Errorf("LLM JSON gecersiz: %w", err)
	}
	for i := range out.Tables {
		t := &out.Tables[i]
		t.Entity = slug(t.Entity)
		if !adviceKinds[t.Kind] {
			t.Kind = "other"
		}
		kept := t.Fields[:0]
		for _, f := range t.Fields {
			if f.Expression != "" && !validExpr[f.Expression] {
				f.Expression = "" // desteklenmeyen expression → dusur (fail-safe)
			}
			if f.Column != "" {
				kept = append(kept, f)
			}
		}
		t.Fields = kept
	}
	return out.Tables, nil
}

// extractJSON — ```json cit'lerini kaldir + ilk '{' ... son '}' araligini al (gurbuz).
func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	i := strings.Index(s, "{")
	j := strings.LastIndex(s, "}")
	if i < 0 || j < i {
		return ""
	}
	return s[i : j+1]
}

// schemaText — kompakt sema temsili (tablo: kolon(tip), ...). Sinirli (prompt boyutu).
func schemaText(sc *connector.Schema) string {
	var b strings.Builder
	for ti, t := range sc.Tables {
		if ti >= maxAdviseTables {
			fmt.Fprintf(&b, "... (+%d tablo daha)\n", len(sc.Tables)-maxAdviseTables)
			break
		}
		fmt.Fprintf(&b, "%s.%s: ", t.Schema, t.Name)
		for ci, c := range t.Columns {
			if ci >= maxAdviseColumns {
				fmt.Fprintf(&b, "... (+%d kolon)", len(t.Columns)-maxAdviseColumns)
				break
			}
			if ci > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "%s(%s)", c.Name, c.Type)
		}
		b.WriteString("\n")
	}
	return b.String()
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
