package expression

import (
	"strings"
	"testing"
	"time"
)

func TestMaskTCKN(t *testing.T) {
	got := Apply("mask:tckn", "12345678901").(string)
	if got != "123******01" {
		t.Fatalf("tckn maske yanlis: %q", got)
	}
}

func TestMaskEmail(t *testing.T) {
	got := Apply("mask:email", "alice@example.com").(string)
	if got != "a****@example.com" {
		t.Fatalf("email maske yanlis: %q", got)
	}
}

func TestMaskCard(t *testing.T) {
	got := Apply("mask:card", "4111 1111 1111 1234").(string)
	if got != "************1234" {
		t.Fatalf("card maske yanlis: %q", got)
	}
}

func TestFormatDate(t *testing.T) {
	tm := time.Date(2026, 7, 6, 13, 30, 0, 0, time.UTC)
	if got := Apply("format:date", tm).(string); got != "2026-07-06" {
		t.Fatalf("date format yanlis: %q", got)
	}
}

func TestFormatMoney(t *testing.T) {
	if got := Apply("format:money", 1234.5).(string); got != "1234.50" {
		t.Fatalf("money format yanlis: %q", got)
	}
}

func TestUnknownFailsafe(t *testing.T) {
	// tanimsiz ifade => degeri OLDUGU GIBI gecirmemeli (fail-safe maske).
	got := Apply("weird:thing", "secret").(string)
	if got == "secret" {
		t.Fatal("bilinmeyen ifade degeri sizdirdi (fail-safe olmali)")
	}
}

func TestEmptyExprPassthrough(t *testing.T) {
	if got := Apply("", "x"); got != "x" {
		t.Fatalf("bos ifade degeri degistirdi: %v", got)
	}
}

func TestNilPassthrough(t *testing.T) {
	if got := Apply("mask:tckn", nil); got != nil {
		t.Fatalf("nil deger maske ile bozuldu: %v", got)
	}
}

func TestMaskTextEmail(t *testing.T) {
	got := MaskText("iletisim: alice@example.com lutfen yazin")
	if strings.Contains(got, "alice@example.com") {
		t.Fatalf("email maskelenmedi: %q", got)
	}
	if !strings.Contains(got, "@example.com") {
		t.Fatalf("domain korunmali: %q", got)
	}
}

func TestMaskTextIBAN(t *testing.T) {
	got := MaskText("hesap: TR330006100519786457841326 tesekkurler")
	if strings.Contains(got, "TR330006100519786457841326") {
		t.Fatalf("iban maskelenmedi: %q", got)
	}
}

func TestMaskTextPhone(t *testing.T) {
	got := MaskText("beni ara: 0532 123 45 67")
	if strings.Contains(got, "532 123 45 67") {
		t.Fatalf("telefon maskelenmedi: %q", got)
	}
}

func TestMaskTextTCKN(t *testing.T) {
	got := MaskText("tc kimlik no 12345678901 kayitli")
	if strings.Contains(got, "12345678901") {
		t.Fatalf("tckn maskelenmedi: %q", got)
	}
}

func TestMaskTextCard(t *testing.T) {
	// gecerli Luhn: 4539 1488 0343 6467
	got := MaskText("kart: 4539 1488 0343 6467 son")
	if strings.Contains(got, "4539 1488 0343 6467") {
		t.Fatalf("kart maskelenmedi: %q", got)
	}
}

func TestMaskTextPlainSentenceUntouched(t *testing.T) {
	s := "Bu cumlede hicbir kisisel veri yok, sadece duz metin."
	if got := MaskText(s); got != s {
		t.Fatalf("PII olmayan metin degistirildi: %q", got)
	}
}

func TestMaskTextNonPIINumberUntouched(t *testing.T) {
	// 4 haneli siparis no gibi kisa bir sayi PII sayilmamali.
	s := "siparis no 4521 onaylandi"
	if got := MaskText(s); got != s {
		t.Fatalf("PII olmayan sayi maskelendi: %q", got)
	}
}

func TestMaskTextInvalidCardNotMangled(t *testing.T) {
	// Luhn gecmeyen 16 haneli rastgele sayi kart sanilmamali (asiri-maskeleme onlenir).
	s := "referans kodu 1234567890123456 kaydedildi"
	got := MaskText(s)
	if strings.Contains(got, "*") {
		// Not: bu 16 hane hem Luhn'u gecmez hem 11-hane blogu icermez, degismemeli.
		t.Fatalf("Luhn gecmeyen sayi yanlislikla maskelendi: %q", got)
	}
}
