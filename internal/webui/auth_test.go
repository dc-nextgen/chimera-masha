package webui

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewAuthNilWhenEmpty(t *testing.T) {
	if newAuth("") != nil || newAuth("   ") != nil {
		t.Error("bos parola → nil (auth kapali) bekleniyordu")
	}
	if newAuth("s3cret") == nil {
		t.Error("dolu parola → auth bekleniyordu")
	}
}

func TestLoginAndValid(t *testing.T) {
	a := newAuth("s3cret")
	if _, ok := a.login("yanlis"); ok {
		t.Error("yanlis parola login olmamali")
	}
	tok, ok := a.login("s3cret")
	if !ok || tok == "" {
		t.Fatal("dogru parola token vermeli")
	}
	if !a.valid(tok) {
		t.Error("taze token gecerli olmali")
	}
	if a.valid("bogus") || a.valid("") {
		t.Error("gecersiz/bos token reddedilmeli")
	}
}

func TestSessionExpiry(t *testing.T) {
	a := newAuth("s3cret")
	tok, _ := a.login("s3cret")
	a.mu.Lock()
	a.sessions[tok] = time.Now().Add(-time.Minute) // suresi gecmis
	a.mu.Unlock()
	if a.valid(tok) {
		t.Error("suresi dolmus token reddedilmeli")
	}
}

func TestGatedMiddleware(t *testing.T) {
	called := false
	h := func(w http.ResponseWriter, r *http.Request) { called = true; w.WriteHeader(200) }

	// auth kapali → gecer.
	uOpen := &UI{auth: nil}
	rec := httptest.NewRecorder()
	uOpen.gated(h)(rec, httptest.NewRequest("GET", "/x", nil))
	if !called || rec.Code != 200 {
		t.Error("auth kapali iken gecmeliydi")
	}

	// auth acik, token yok → 401.
	uAuth := &UI{auth: newAuth("s3cret")}
	called = false
	rec = httptest.NewRecorder()
	uAuth.gated(h)(rec, httptest.NewRequest("GET", "/x", nil))
	if called || rec.Code != 401 {
		t.Errorf("token yoksa 401 bekleniyordu, goren %d", rec.Code)
	}

	// auth acik, gecerli token → gecer.
	tok, _ := uAuth.auth.login("s3cret")
	called = false
	rec = httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	uAuth.gated(h)(rec, req)
	if !called || rec.Code != 200 {
		t.Errorf("gecerli token gecmeliydi, goren %d", rec.Code)
	}
}
