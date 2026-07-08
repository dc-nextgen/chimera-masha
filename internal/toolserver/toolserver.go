// Package toolserver — bulut-yuzey (tunelden gelen istekler). OWUI'nin cagirdigi OpenAPI
// tool-server yuzeyini SUNAR + GUVENLIK SINIRINI uygular (app.py'nin Go portu, docs §6/§12):
//
//  1. Bearer-auth (sabit-zamanli) — her istek.
//  2. Yol TAM [server_label, leaf] (traversal/injection kapali; yeniden-kurulmus yol).
//  3. server_label → REGISTRY dispatch (§19.2 cok-baglanti; tek tunel cok label). Bilinmeyen label = 403.
//  4. Tool allowlist + yazma-fiili reddi = connector.AllowTool (fail-closed).
//  5. Hash-zincirli audit (her karar).
//
// mcpo'ya forward YOK (mssql); erpnext-proxy connector kendi Call'inda forward eder. Serbest SQL yok.
package toolserver

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/dc-nextgen/chimera-masha/internal/audit"
	"github.com/dc-nextgen/chimera-masha/internal/registry"
)

type Server struct {
	token   []byte
	reg     *registry.Registry // CANLI baglanti kaydi (label → connector)
	aud     *audit.Auditor
	timeout time.Duration
}

func New(token string, reg *registry.Registry, aud *audit.Auditor) *Server {
	return &Server{token: []byte(token), reg: reg, aud: aud, timeout: 30 * time.Second}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	client := clientIP(r)
	parts := splitPath(r.URL.Path)

	// 1) Kimlik dogrulama (her yol, spec kesfi dahil).
	if !s.authed(r) {
		s.audit("deny", map[string]any{"reason": "auth", "path": r.URL.Path, "client": client})
		writeJSON(w, http.StatusUnauthorized, map[string]string{"detail": "unauthorized"})
		return
	}

	// 2) Yol TAM [server_label, leaf]. Fazla segment = traversal vektoru → fail-closed.
	if len(parts) != 2 {
		s.audit("deny", map[string]any{"reason": "shape", "path": r.URL.Path, "client": client})
		writeJSON(w, http.StatusForbidden, map[string]string{"detail": "forbidden"})
		return
	}
	label, leaf := parts[0], parts[1]

	// 3) server_label → registry dispatch. Bilinmeyen = 403 (fail-closed).
	conn := s.reg.Get(label)
	if conn == nil {
		s.audit("deny", map[string]any{"reason": "server", "server": label, "client": client})
		writeJSON(w, http.StatusForbidden, map[string]string{"detail": "forbidden"})
		return
	}

	// 4) Spec kesfi — CANLI (onboarding apply sonrasi aninda yeni spec).
	if r.Method == http.MethodGet && leaf == "openapi.json" {
		b, _ := json.Marshal(conn.Conn.OpenAPI(label))
		w.Header().Set("Content-Type", "application/json")
		w.Write(b)
		return
	}

	// 5) Arac cagrisi — POST + allowlist + yazma-fiili reddi (connector).
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"detail": "method not allowed"})
		return
	}
	tool := leaf
	if !conn.Conn.AllowTool(tool) {
		s.audit("deny", map[string]any{"reason": "tool", "server": label, "tool": tool, "client": client})
		writeJSON(w, http.StatusForbidden, map[string]string{"detail": "forbidden tool"})
		return
	}

	var args map[string]any
	if r.Body != nil {
		defer r.Body.Close()
		dec := json.NewDecoder(r.Body)
		if err := dec.Decode(&args); err != nil && err.Error() != "EOF" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"detail": "invalid json body"})
			return
		}
	}
	if args == nil {
		args = map[string]any{}
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.timeout)
	defer cancel()
	res, err := conn.Conn.Call(ctx, tool, args)
	if err != nil {
		s.audit("error", map[string]any{"server": label, "tool": tool, "err": truncate(err.Error(), 200)})
		writeJSON(w, http.StatusBadGateway, map[string]string{"detail": "connector error"})
		return
	}
	s.audit("allow", map[string]any{"server": label, "tool": tool, "status": 200})
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) authed(r *http.Request) bool {
	hdr := r.Header.Get("Authorization")
	if len(hdr) < 7 || !strings.EqualFold(hdr[:7], "bearer ") {
		return false
	}
	got := strings.TrimSpace(hdr[7:])
	return subtle.ConstantTimeCompare([]byte(got), s.token) == 1
}

func (s *Server) audit(decision string, fields map[string]any) {
	if s.aud == nil {
		return
	}
	fields["decision"] = decision
	s.aud.Record(fields)
}

func splitPath(p string) []string {
	var out []string
	for _, seg := range strings.Split(p, "/") {
		if seg != "" {
			out = append(out, seg)
		}
	}
	return out
}

func clientIP(r *http.Request) string {
	if h := r.RemoteAddr; h != "" {
		if i := strings.LastIndex(h, ":"); i > 0 {
			return h[:i]
		}
		return h
	}
	return "?"
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
