// Yerel yuz kimlik dogrulama (§17.9): LAN'a acilinca (0.0.0.0) /try + /onboard/* korumasiz kalmasin.
// Parola (MASHA_WEB_PASSWORD) POST /auth/login'de sabit-zamanli dogrulanir → rastgele OTURUM token'i
// verilir (parola her istekte GITMEZ). Token in-memory, sureli. Parola bos → auth KAPALI (loopback'te guvenli).
//
// DURUST SINIR: TLS YOK → duz-HTTP LAN'da login/token dinlenebilir. Parola, gucluluk degil ERISIM
// kapisi (guvenilir LAN icin). Gercek cozum = self-signed TLS (§17.9, sonraki adim).
package webui

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"
)

const sessionTTL = 12 * time.Hour

type auth struct {
	password []byte
	mu       sync.Mutex
	sessions map[string]time.Time // token → son gecerlilik
}

// newAuth — parola bos ise nil (auth KAPALI).
func newAuth(password string) *auth {
	if strings.TrimSpace(password) == "" {
		return nil
	}
	return &auth{password: []byte(password), sessions: map[string]time.Time{}}
}

// login — parola dogruysa yeni oturum token'i. Sabit-zamanli karsilastirma.
func (a *auth) login(password string) (string, bool) {
	if subtle.ConstantTimeCompare([]byte(password), a.password) != 1 {
		return "", false
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", false
	}
	tok := hex.EncodeToString(b)
	a.mu.Lock()
	a.sessions[tok] = time.Now().Add(sessionTTL)
	a.gcLocked()
	a.mu.Unlock()
	return tok, true
}

// valid — token gecerli + suresi dolmamis mi.
func (a *auth) valid(tok string) bool {
	if tok == "" {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	exp, ok := a.sessions[tok]
	if !ok {
		return false
	}
	if time.Now().After(exp) {
		delete(a.sessions, tok)
		return false
	}
	return true
}

// gcLocked — suresi dolmus oturumlari temizle (mu tutulurken cagrilir).
func (a *auth) gcLocked() {
	now := time.Now()
	for t, exp := range a.sessions {
		if now.After(exp) {
			delete(a.sessions, t)
		}
	}
}

// check — istekteki Bearer token gecerli mi.
func (a *auth) check(r *http.Request) bool {
	hdr := r.Header.Get("Authorization")
	if len(hdr) < 7 || !strings.EqualFold(hdr[:7], "bearer ") {
		return false
	}
	return a.valid(strings.TrimSpace(hdr[7:]))
}

// gated — auth aciksa Bearer token ister; kapaliysa (nil) dogrudan gecirir.
func (u *UI) gated(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if u.auth != nil && !u.auth.check(r) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "kimlik gerekli (login)"})
			return
		}
		h(w, r)
	}
}

// authStatus — SPA'nin login gerekli mi bilmesi icin (acik uc).
func (u *UI) authStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"auth_required": u.auth != nil})
}

// authLogin — parola → oturum token'i (acik uc). Kapaliysa 400.
func (u *UI) authLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "POST"})
		return
	}
	if u.auth == nil {
		writeJSON(w, 400, map[string]string{"error": "auth kapali"})
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "gecersiz istek"})
		return
	}
	tok, ok := u.auth.login(req.Password)
	if !ok {
		if u.aud != nil {
			u.aud.Record(map[string]any{"decision": "deny", "source": "web-login", "reason": "parola"})
		}
		writeJSON(w, 401, map[string]string{"error": "parola yanlis"})
		return
	}
	writeJSON(w, 200, map[string]string{"token": tok})
}
