// Package expression — per-alan donusum (maske/format), ON-PREM, tunelden ONCE uygulanir.
//
// docs/masha-plan.md §17.7/§17.8: alan degeri buluta gitmeden once donusturulur. Maskeleme
// at-source ve DETERMINISTIK (regex pii-sanitizer'dan daha kesin: kolonun TC oldugu BILINIR).
// SQL-expression (bir kolonda tutulan SQL) ERTELENDI — burada YOK.
package expression

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

var reTCKN = regexp.MustCompile(`(tckn|tcno|tc_no|tc_kimlik|kimlikno|kimlik_no|vkn|vergino|vergi_no)`)

// MaskForField — alan/kolon ADINDAN PII maske ifadesi ("mask:email" vb.) ya da "". Heuristik (KESİN DEĞİL) —
// onboard (DB manifest önerisi) + erpnext (generic doküman maskeleme) ortak kullanır (drift önlenir).
func MaskForField(name string) string {
	n := strings.ToLower(name)
	switch {
	case containsAny(n, "email", "eposta", "e_posta", "mail"):
		return "mask:email"
	case reTCKN.MatchString(n):
		return "mask:tckn"
	case strings.Contains(n, "iban"):
		return "mask:iban"
	case containsAny(n, "phone", "telefon", "gsm", "cep", "mobile", "_tel", "tel_"):
		return "mask:phone"
	case containsAny(n, "kredikart", "creditcard", "kartno", "cardno", "card_no", "cardnumber"):
		return "mask:card"
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

// Apply "kind:arg" ifadesini value'ya uygular. Bos ifade => value aynen.
// Bilinmeyen ifade => guvenli taraf: maske:default (fail-safe, veri sizdirma).
func Apply(expr string, value any) any {
	if expr == "" || value == nil {
		return value
	}
	kind, arg, _ := strings.Cut(expr, ":")
	switch kind {
	case "mask":
		return mask(arg, toString(value))
	case "format":
		return format(arg, value)
	case "":
		return value
	default:
		// tanimsiz ifade => sessizce degeri gecirmek yerine maskele (fail-safe).
		return mask("default", toString(value))
	}
}

func toString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", t)
	}
}

// mask — deterministik maskeleme. Bas/son birkac karakteri korur, ortayi gizler.
func mask(kind, s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	switch kind {
	case "tckn", "id": // 11 hane TC / kimlik: ilk 3 + son 2 goster
		return keepEnds(digitsOnly(s), 3, 2)
	case "iban":
		return keepEnds(strings.ReplaceAll(s, " ", ""), 4, 4)
	case "phone":
		return keepEnds(s, 0, 4)
	case "email":
		at := strings.LastIndex(s, "@")
		if at <= 0 {
			return keepEnds(s, 1, 0)
		}
		return keepEnds(s[:at], 1, 0) + s[at:]
	case "card":
		return keepEnds(digitsOnly(s), 0, 4)
	default: // "default" / bilinmeyen => sadece ilk karakter
		return keepEnds(s, 1, 0)
	}
}

func digitsOnly(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// keepEnds head+tail karakter gosterir, arayi '*' yapar. Cok kisa string => tamamen maskeler.
func keepEnds(s string, head, tail int) string {
	r := []rune(s)
	if len(r) <= head+tail {
		if len(r) == 0 {
			return ""
		}
		return strings.Repeat("*", len(r))
	}
	mid := len(r) - head - tail
	return string(r[:head]) + strings.Repeat("*", mid) + string(r[len(r)-tail:])
}

// format — sunum bicimi (PII degil; gorunum). Belirsizse degeri aynen dondurur.
func format(kind string, value any) any {
	switch kind {
	case "date":
		if t, ok := asTime(value); ok {
			return t.Format("2006-01-02")
		}
	case "datetime":
		if t, ok := asTime(value); ok {
			return t.Format("2006-01-02 15:04")
		}
	case "money":
		if f, ok := asFloat(value); ok {
			return fmt.Sprintf("%.2f", f)
		}
	}
	return value
}

func asTime(v any) (time.Time, bool) {
	switch t := v.(type) {
	case time.Time:
		return t, true
	case string:
		for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02"} {
			if p, err := time.Parse(layout, t); err == nil {
				return p, true
			}
		}
	}
	return time.Time{}, false
}

func asFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int64:
		return float64(t), true
	case int:
		return float64(t), true
	}
	return 0, false
}
