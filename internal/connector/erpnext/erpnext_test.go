package erpnext

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAllowToolReadOnly(t *testing.T) {
	c := Open("http://x", "k", "s", false)
	for _, ok := range []string{"get_count", "get_documents", "get_document", "get_doctypes", "get_doctype_fields", "run_report"} {
		if !c.AllowTool(ok) {
			t.Errorf("%s izinli olmali", ok)
		}
	}
	for _, no := range []string{"create_document", "update_document", "delete_document", "submit_document", "call_method"} {
		if c.AllowTool(no) {
			t.Errorf("%s (yazma) reddedilmeli", no)
		}
	}
}

func TestOpenAPIShape(t *testing.T) {
	c := Open("http://x", "k", "s", false)
	spec := c.OpenAPI("erpnext")
	paths, _ := spec["paths"].(map[string]any)
	if len(paths) != 6 {
		t.Errorf("6 path bekleniyordu, %d", len(paths))
	}
	if _, ok := paths["/get_count"]; !ok {
		t.Error("/get_count eksik")
	}
}

func TestConnectedAndOpenEmpty(t *testing.T) {
	if Open("", "", "", false).Connected() {
		t.Error("URL bos → Connected false olmali")
	}
	if !Open("http://x", "k", "s", false).Connected() {
		t.Error("URL dolu → Connected true olmali")
	}
}

func TestCallGetCountAgainstMock(t *testing.T) {
	var gotPath, gotQuery, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotAuth = r.Header.Get("Authorization")
		json.NewEncoder(w).Encode(map[string]any{"message": 43})
	}))
	defer srv.Close()

	c := Open(srv.URL, "mykey", "mysecret", false)
	res, err := c.Call(context.Background(), "get_count", map[string]any{"doctype": "Lead"})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	m := res.(map[string]any)
	if m["count"] != float64(43) {
		t.Errorf("count 43 bekleniyordu, %v", m["count"])
	}
	if gotPath != "/api/method/frappe.client.get_count" {
		t.Errorf("path yanlis: %s", gotPath)
	}
	if gotQuery != "doctype=Lead" {
		t.Errorf("query yanlis: %s", gotQuery)
	}
	if gotAuth != "token mykey:mysecret" {
		t.Errorf("auth basligi yanlis: %s", gotAuth)
	}
}

func TestCallGetDocumentsFiltersEncoded(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]any{"name": "L-1"}}})
	}))
	defer srv.Close()
	c := Open(srv.URL, "k", "s", false)
	res, err := c.Call(context.Background(), "get_documents", map[string]any{
		"doctype": "Lead", "filters": map[string]any{"status": "Open"}, "limit": 5,
	})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if res.(map[string]any)["count"] != 1 {
		t.Errorf("count 1 bekleniyordu")
	}
	// filters JSON-encode + limit gecmis olmali.
	if !contains(gotQuery, "filters=") || !contains(gotQuery, "limit_page_length=5") {
		t.Errorf("query filters/limit icermeli: %s", gotQuery)
	}
}

func TestWriteHasNoHTTPMethod(t *testing.T) {
	// Connector yalniz GET yapar; bilinmeyen/yazma tool → hata (REST'e write gitmez).
	c := Open("http://x", "k", "s", false)
	if _, err := c.Call(context.Background(), "create_document", map[string]any{"doctype": "Lead"}); err == nil {
		t.Error("create_document Call'da hata dondurmeli (dispatch'te yok)")
	}
}

func TestMaskingAppliedToDocuments(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": []any{
			map[string]any{"name": "L-1", "email_id": "ali@firma.com", "phone": "05321234567", "company": "Acme"},
		}})
	}))
	defer srv.Close()
	c := Open(srv.URL, "k", "s", true) // maske AÇIK
	res, err := c.Call(context.Background(), "get_documents", map[string]any{"doctype": "Lead"})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	d := res.(map[string]any)["documents"].([]any)[0].(map[string]any)
	if d["email_id"] == "ali@firma.com" {
		t.Error("email_id maskelenmedi")
	}
	if d["phone"] == "05321234567" {
		t.Error("phone maskelenmedi")
	}
	if d["company"] != "Acme" {
		t.Errorf("PII olmayan alan (company) değişmemeli, goren %v", d["company"])
	}
}

func TestNoMaskingWhenOff(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]any{"email_id": "ali@firma.com"}}})
	}))
	defer srv.Close()
	c := Open(srv.URL, "k", "s", false) // maske KAPALI
	res, _ := c.Call(context.Background(), "get_documents", map[string]any{"doctype": "Lead"})
	d := res.(map[string]any)["documents"].([]any)[0].(map[string]any)
	if d["email_id"] != "ali@firma.com" {
		t.Error("maske kapalıyken email değişmemeli")
	}
}

// ── YAZMA (insan-onayli, Faz1 create-only) ────────────────────────────────

func TestWriteDisabledByDefault(t *testing.T) {
	c := Open("http://x", "k", "s", false) // SetWrite CAGRILMADI
	if c.AllowTool("create_document") {
		t.Error("yazma default KAPALI olmali (AllowTool false)")
	}
	if _, err := c.Call(context.Background(), "create_document",
		map[string]any{"doctype": "Lead", "fields": map[string]any{"lead_name": "X"}}); err == nil {
		t.Error("yazma kapaliyken create_document hata dondurmeli")
	}
}

func TestWriteEnabledCreatesDraft(t *testing.T) {
	var gotPath, gotAuth string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.EscapedPath(), r.Header.Get("Authorization")
		json.NewDecoder(r.Body).Decode(&gotBody)
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"name": "SAL-ORD-0001", "docstatus": float64(0)}})
	}))
	defer srv.Close()

	c := Open(srv.URL, "k", "s", false)
	c.SetWrite(true, []string{"Sales Order"})
	if !c.AllowTool("create_document") {
		t.Fatal("yazma acikken create_document izinli olmali")
	}
	res, err := c.Call(context.Background(), "create_document", map[string]any{
		"doctype": "Sales Order",
		"fields":  map[string]any{"customer": "Acme", "docstatus": float64(1), "name": "HACK"},
	})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	m := res.(map[string]any)
	if m["created"] != true || m["name"] != "SAL-ORD-0001" {
		t.Errorf("beklenen created+name, goren %v", m)
	}
	if gotPath != "/api/resource/Sales%20Order" {
		t.Errorf("path yanlis: %s", gotPath)
	}
	if gotAuth != "token k:s" {
		t.Errorf("auth yanlis: %s", gotAuth)
	}
	// VERB-TAVANI: docstatus + name govdeden ATILMIS olmali (submit-yukseltme + sistem alani reddi).
	if _, ok := gotBody["docstatus"]; ok {
		t.Error("docstatus govdeye SIZDI (submit-yukseltme riski)")
	}
	if _, ok := gotBody["name"]; ok {
		t.Error("name (sistem alani) govdeye sizdi")
	}
	if gotBody["customer"] != "Acme" {
		t.Errorf("mesru alan customer gitmedi: %v", gotBody["customer"])
	}
	if gotBody["doctype"] != "Sales Order" {
		t.Errorf("govdede doctype olmali: %v", gotBody["doctype"])
	}
}

func TestWriteDoctypeAllowlistFailClosed(t *testing.T) {
	c := Open("http://x", "k", "s", false)
	c.SetWrite(true, []string{"Lead"}) // yalniz Lead
	// Sales Order beyaz-listede DEGIL → HTTP'ye gitmeden reddedilmeli.
	if _, err := c.Call(context.Background(), "create_document", map[string]any{
		"doctype": "Sales Order", "fields": map[string]any{"customer": "X"},
	}); err == nil {
		t.Error("beyaz-liste disi doctype reddedilmeli (fail-closed)")
	}
	// Bos beyaz-liste => hicbir sey yazilamaz.
	c.SetWrite(true, nil)
	if _, err := c.Call(context.Background(), "create_document", map[string]any{
		"doctype": "Lead", "fields": map[string]any{"lead_name": "X"},
	}); err == nil {
		t.Error("bos beyaz-liste => yazma yok (fail-closed)")
	}
}

func TestWriteVerbCeiling(t *testing.T) {
	c := Open("http://x", "k", "s", false)
	c.SetWrite(true, []string{"Lead"})
	// create disinda YAZMA fiili yok (kod-tavani; AllowTool da reddeder).
	for _, no := range []string{"update_document", "submit_document", "delete_document", "cancel_document", "call_method"} {
		if c.AllowTool(no) {
			t.Errorf("%s yazma acikken bile reddedilmeli (verb-tavani)", no)
		}
	}
}

func TestOpenAPIExcludesWriteEvenWhenEnabled(t *testing.T) {
	c := Open("http://x", "k", "s", false)
	c.SetWrite(true, []string{"Sales Order"})
	paths := c.OpenAPI("erpnext")["paths"].(map[string]any)
	if len(paths) != 6 {
		t.Errorf("yazma acik olsa bile OpenAPI 6 salt-okuma path (LLM yazmayi GORMEZ), goren %d", len(paths))
	}
	if _, ok := paths["/create_document"]; ok {
		t.Error("create_document OpenAPI'ye SIZDI (LLM cagirabilir hale gelir)")
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
