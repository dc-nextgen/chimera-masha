package expression

import (
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
