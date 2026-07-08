package toolserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dc-nextgen/chimera-masha/internal/connector"
	"github.com/dc-nextgen/chimera-masha/internal/manifest"
	"github.com/dc-nextgen/chimera-masha/internal/registry"
)

type fakeConn struct {
	man    *manifest.Manifest
	called string
}

func (f *fakeConn) Call(ctx context.Context, tool string, args map[string]any) (any, error) {
	f.called = tool
	return map[string]any{"ok": true, "tool": tool}, nil
}
func (f *fakeConn) Introspect(ctx context.Context) (*connector.Schema, error) { return &connector.Schema{}, nil }
func (f *fakeConn) Health(ctx context.Context) error                          { return nil }
func (f *fakeConn) Connected() bool                                           { return true }
func (f *fakeConn) Connect(ctx context.Context, dsn string) error             { return nil }
func (f *fakeConn) OpenAPI(label string) map[string]any                       { return f.man.OpenAPI(label) }
func (f *fakeConn) AllowTool(tool string) bool {
	if _, ok := f.man.Tool(tool); !ok {
		return false
	}
	for _, v := range []string{"delete", "create", "update", "write", "drop"} {
		if strings.Contains(strings.ToLower(tool), v) {
			return false
		}
	}
	return true
}
func (f *fakeConn) Close() error { return nil }

func testMan() *manifest.Manifest {
	return &manifest.Manifest{
		Name: "t", Label: "T", ERPKind: "mssql-generic",
		DB: manifest.DBConfig{Driver: "sqlserver", ReadOnly: true},
		Entities: map[string]manifest.Entity{
			"invoice": {Table: "dbo.Invoices", Fields: map[string]manifest.Field{"id": {Column: "InvoiceID"}}},
		},
		Tools: []manifest.Tool{
			{Name: "count_invoices", Kind: "count", Entity: "invoice"},
			{Name: "delete_invoices", Kind: "count", Entity: "invoice"}, // yazma-fiili adi → guard reddetmeli
		},
	}
}

const token = "sekret-token"

func newSrv() (*Server, *fakeConn) {
	fc := &fakeConn{man: testMan()}
	reg := registry.New()
	reg.Add(&registry.Connection{Name: "t", Label: "T", Kind: "mssql", ServerLabel: "masha-db", Conn: fc})
	return New(token, reg, nil), fc
}

func do(s *Server, method, path, auth, body string) *httptest.ResponseRecorder {
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	if auth != "" {
		r.Header.Set("Authorization", auth)
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}

func TestNoAuth(t *testing.T) {
	s, _ := newSrv()
	if w := do(s, "POST", "/masha-db/count_invoices", "", "{}"); w.Code != 401 {
		t.Fatalf("auth'suz istek %d (401 bekleniyordu)", w.Code)
	}
}

func TestWrongToken(t *testing.T) {
	s, _ := newSrv()
	if w := do(s, "POST", "/masha-db/count_invoices", "Bearer yanlis", "{}"); w.Code != 401 {
		t.Fatalf("yanlis token %d (401 bekleniyordu)", w.Code)
	}
}

func TestOpenAPI(t *testing.T) {
	s, _ := newSrv()
	w := do(s, "GET", "/masha-db/openapi.json", "Bearer "+token, "")
	if w.Code != 200 {
		t.Fatalf("openapi %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "count_invoices") {
		t.Fatal("openapi tool'u icermiyor")
	}
}

func TestDispatch(t *testing.T) {
	s, fc := newSrv()
	w := do(s, "POST", "/masha-db/count_invoices", "Bearer "+token, `{"start_date":"2026-01-01"}`)
	if w.Code != 200 {
		t.Fatalf("dispatch %d", w.Code)
	}
	if fc.called != "count_invoices" {
		t.Fatalf("connector cagrilmadi: %q", fc.called)
	}
}

func TestUnknownTool(t *testing.T) {
	s, _ := newSrv()
	if w := do(s, "POST", "/masha-db/nope", "Bearer "+token, "{}"); w.Code != 403 {
		t.Fatalf("bilinmeyen tool %d (403 bekleniyordu)", w.Code)
	}
}

func TestWriteVerbGuard(t *testing.T) {
	s, _ := newSrv()
	// manifest'te olsa bile yazma-fiili adi (delete_) → fail-closed 403.
	if w := do(s, "POST", "/masha-db/delete_invoices", "Bearer "+token, "{}"); w.Code != 403 {
		t.Fatalf("yazma-fiili tool %d (403 bekleniyordu)", w.Code)
	}
}

func TestPathTraversalShape(t *testing.T) {
	s, _ := newSrv()
	if w := do(s, "POST", "/masha-db/a/b", "Bearer "+token, "{}"); w.Code != 403 {
		t.Fatalf("fazla segment %d (403 bekleniyordu)", w.Code)
	}
}

func TestWrongServerLabel(t *testing.T) {
	s, _ := newSrv()
	if w := do(s, "POST", "/baska/count_invoices", "Bearer "+token, "{}"); w.Code != 403 {
		t.Fatalf("yanlis server %d (403 bekleniyordu)", w.Code)
	}
}
