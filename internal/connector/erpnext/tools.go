package erpnext

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/mehmetor/chimera-ai/stack/masha/agent/internal/connector"
	"github.com/mehmetor/chimera-ai/stack/masha/agent/internal/expression"
	"github.com/mehmetor/chimera-ai/stack/masha/agent/internal/manifest"
)

// maskAny — ErpNext dokümanının PII alanlarını (ad heuristigi) YERİNDE maskele; nested (child tablo)
// ve dizilere recurse. Tünelden ÖNCE (§3). Heuristik → KESİN DEĞİL (dürüst sınır; en-az-yetki login öneri).
func maskAny(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			if s, ok := val.(string); ok {
				if e := expression.MaskForField(k); e != "" && s != "" {
					t[k] = expression.Apply(e, s)
				}
			} else {
				t[k] = maskAny(val)
			}
		}
		return t
	case []any:
		for i := range t {
			t[i] = maskAny(t[i])
		}
		return t
	}
	return v
}

// readTools — SUNULAN salt-okuma araçları (erpnext-mcp'nin write araçları HARİÇ). AllowTool tabanı.
var readTools = map[string]bool{
	"get_count": true, "get_documents": true, "get_document": true,
	"get_doctypes": true, "get_doctype_fields": true, "run_report": true,
}

const maxDocLimit = 200

// writeTools — insan-onayli YAZMA araclari (Faz1: yalniz create). submit/update/delete/cancel/
// call_method BILEREK YOK = verb-tavani KOD'da (prompt/manifest guvenine dayanmaz; decision E).
// Bu araclar OpenAPI'ye GIRMEZ (LLM gormez) — yalniz M2M/onay-akisi cagirir.
var writeTools = map[string]bool{"create_document": true}

// docstatusDraft — create yalniz TASLAK (docstatus=0) uretir; submit ASLA. Bu alanlar govdeden
// atilir (istemci 1 gondererek submit'e yukseltemez) + name/owner gibi sistem alanlari da.
var strippedWriteFields = map[string]bool{
	"docstatus": true, "__islocal": true, "__unsaved": true, "name": true,
	"owner": true, "modified_by": true, "creation": true, "modified": true, "idx": true,
}

// AllowTool — salt-okuma HER ZAMAN; create_document YALNIZ writeEnabled ise (SetWrite ile acilir).
// Doctype beyaz-listesi burada DEGIL Call'da (fail-closed) — AllowTool tool-adi gorur, doctype gormez.
func (c *Connector) AllowTool(tool string) bool {
	if readTools[tool] {
		return true
	}
	return c.writeEnabled && writeTools[tool]
}

// Call — tool'u ErpNext REST'e cevirir (yalniz GET; yazma yok). Sonuc JSON-serializable.
func (c *Connector) Call(ctx context.Context, tool string, args map[string]any) (any, error) {
	switch tool {
	case "get_count":
		dt := strArg(args, "doctype")
		if dt == "" {
			return nil, fmt.Errorf("doctype zorunlu")
		}
		p := url.Values{"doctype": {dt}}
		if f := jsonArg(args, "filters"); f != "" {
			p.Set("filters", f)
		}
		r, err := c.get(ctx, "/api/method/frappe.client.get_count", p)
		if err != nil {
			return nil, err
		}
		return map[string]any{"count": r["message"]}, nil

	case "get_documents":
		dt := strArg(args, "doctype")
		if dt == "" {
			return nil, fmt.Errorf("doctype zorunlu")
		}
		p := url.Values{}
		if f := jsonArg(args, "fields"); f != "" {
			p.Set("fields", f)
		}
		if f := jsonArg(args, "filters"); f != "" {
			p.Set("filters", f)
		}
		lim := intArg(args, "limit")
		if lim <= 0 {
			lim = 20 // varsayilan: token patlamasini + ham-dump'i sinirla
		}
		if lim > maxDocLimit {
			lim = maxDocLimit
		}
		p.Set("limit_page_length", strconv.Itoa(lim))
		r, err := c.get(ctx, "/api/resource/"+url.PathEscape(dt), p)
		if err != nil {
			return nil, err
		}
		data := r["data"]
		if c.mask {
			data = maskAny(data)
		}
		return map[string]any{"documents": data, "count": lenOf(data)}, nil

	case "get_document":
		dt, nm := strArg(args, "doctype"), strArg(args, "name")
		if dt == "" || nm == "" {
			return nil, fmt.Errorf("doctype + name zorunlu")
		}
		r, err := c.get(ctx, "/api/resource/"+url.PathEscape(dt)+"/"+url.PathEscape(nm), nil)
		if err != nil {
			return nil, err
		}
		d := r["data"]
		if c.mask {
			d = maskAny(d)
		}
		return d, nil

	case "get_doctypes":
		r, err := c.get(ctx, "/api/resource/DocType", url.Values{
			"fields": {`["name"]`}, "limit_page_length": {"500"},
		})
		if err != nil {
			return nil, err
		}
		return map[string]any{"doctypes": names(r["data"])}, nil

	case "get_doctype_fields":
		dt := strArg(args, "doctype")
		if dt == "" {
			return nil, fmt.Errorf("doctype zorunlu")
		}
		r, err := c.get(ctx, "/api/resource/DocType/"+url.PathEscape(dt), nil)
		if err != nil {
			return nil, err
		}
		if d, ok := r["data"].(map[string]any); ok {
			return map[string]any{"fields": d["fields"]}, nil
		}
		return map[string]any{"fields": nil}, nil

	case "run_report":
		rn := strArg(args, "report_name")
		if rn == "" {
			return nil, fmt.Errorf("report_name zorunlu")
		}
		p := url.Values{"report_name": {rn}}
		if f := jsonArg(args, "filters"); f != "" {
			p.Set("filters", f)
		}
		r, err := c.get(ctx, "/api/method/frappe.desk.query_report.run", p)
		if err != nil {
			return nil, err
		}
		msg := r["message"]
		if c.mask {
			msg = maskAny(msg)
		}
		return msg, nil

	case "create_document":
		// YAZMA (Faz1 create-only). Cok-katli kapi: (1) writeEnabled, (2) doctype beyaz-liste
		// (fail-closed), (3) verb-tavani = yalniz create (submit yok; docstatus stripped → taslak),
		// (4) tehlikeli/sistem alan reddi. Insan-onayi AKIS katmaninda (AP Telegram); burasi KAPI.
		if !c.writeEnabled {
			return nil, fmt.Errorf("yazma kapali (MASHA_ERPNEXT_WRITE)")
		}
		dt := strArg(args, "doctype")
		if dt == "" {
			return nil, fmt.Errorf("doctype zorunlu")
		}
		if !c.writeAllow[dt] { // fail-closed beyaz-liste (decision G)
			return nil, fmt.Errorf("doctype yazma izinli degil: %q", dt)
		}
		fields, ok := args["fields"].(map[string]any)
		if !ok || len(fields) == 0 {
			return nil, fmt.Errorf("fields (nesne) zorunlu")
		}
		body := map[string]any{"doctype": dt}
		for k, v := range fields {
			if strippedWriteFields[k] { // submit-yukseltme + sistem alanlari reddi
				continue
			}
			body[k] = v
		}
		r, err := c.post(ctx, "/api/resource/"+url.PathEscape(dt), body)
		if err != nil {
			return nil, err
		}
		d, _ := r["data"].(map[string]any)
		out := map[string]any{"created": true, "doctype": dt}
		if d != nil {
			out["name"] = d["name"]
			out["docstatus"] = d["docstatus"] // 0 = taslak (dogrulama: submit olmadi)
		}
		return out, nil
	}
	return nil, fmt.Errorf("bilinmeyen tool: %q", tool)
}

// Introspect — doctype'lari "tablo" olarak dondurur (kolon yok). Sema gorunumu icin (§17 entities/resources).
func (c *Connector) Introspect(ctx context.Context) (*connector.Schema, error) {
	r, err := c.get(ctx, "/api/resource/DocType", url.Values{
		"fields": {`["name"]`}, "limit_page_length": {"500"},
	})
	if err != nil {
		return nil, err
	}
	sc := &connector.Schema{}
	for _, n := range names(r["data"]) {
		sc.Tables = append(sc.Tables, connector.Table{Schema: "erpnext", Name: n})
	}
	return sc, nil
}

// OpenAPI — SABIT tool yuzeyi (6 salt-okuma araci). mssql'in manifest-uretiminden farkli (erpnext generic).
func (c *Connector) OpenAPI(serverLabel string) map[string]any {
	str := map[string]any{"type": "string"}
	obj := map[string]any{"type": "object"}
	arrStr := map[string]any{"type": "array", "items": map[string]any{"type": "string"}}
	intT := map[string]any{"type": "integer"}

	op := func(id, desc string, props map[string]any, req []string) map[string]any {
		schema := map[string]any{"type": "object", "properties": props}
		if len(req) > 0 {
			schema["required"] = req
		}
		return map[string]any{"post": map[string]any{
			"operationId": id, "summary": desc, "description": desc + " (salt-okunur).",
			"requestBody": map[string]any{"required": len(req) > 0, "content": map[string]any{
				"application/json": map[string]any{"schema": schema}}},
			"responses": map[string]any{"200": map[string]any{"description": "sonuc",
				"content": map[string]any{"application/json": map[string]any{"schema": obj}}}},
		}}
	}

	paths := map[string]any{
		"/get_count": op("get_count",
			"Bir DocType icin TAM kayit sayisi (limit-bagimsiz, filtre-duyarli). 'kac X var' sorulari icin.",
			map[string]any{"doctype": str, "filters": obj}, []string{"doctype"}),
		"/get_documents": op("get_documents",
			"Bir DocType'in kayitlarini listeler (filtre/alan/limit).",
			map[string]any{"doctype": str, "filters": obj, "fields": arrStr, "limit": intT}, []string{"doctype"}),
		"/get_document": op("get_document", "Tek bir kaydi ad ile getirir.",
			map[string]any{"doctype": str, "name": str}, []string{"doctype", "name"}),
		"/get_doctypes": op("get_doctypes", "Kullanilabilir DocType (varlik turu) listesi.",
			map[string]any{}, nil),
		"/get_doctype_fields": op("get_doctype_fields", "Bir DocType'in alan listesi (sema).",
			map[string]any{"doctype": str}, []string{"doctype"}),
		"/run_report": op("run_report", "Bir raporu calistirir (filtreli).",
			map[string]any{"report_name": str, "filters": obj}, []string{"report_name"}),
	}
	return map[string]any{
		"openapi": "3.1.0",
		"info": map[string]any{"title": "Chimera Masha ErpNext (salt-okunur)", "version": manifest.AgentVersion,
			"description": "ErpNext REST connector (Go-native, salt-okunur)."},
		"servers": []any{map[string]any{"url": "/" + serverLabel}},
		"paths":   paths,
	}
}

// ── arg/yanit yardimcilari ────────────────────────────────────────────────

func strArg(args map[string]any, k string) string {
	if v, ok := args[k].(string); ok {
		return v
	}
	return ""
}

func intArg(args map[string]any, k string) int {
	switch v := args[k].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case string:
		n, _ := strconv.Atoi(v)
		return n
	}
	return 0
}

// jsonArg — args[k] varsa JSON string'e cevirir (ErpNext filters/fields JSON bekler). Yoksa "".
func jsonArg(args map[string]any, k string) string {
	v, ok := args[k]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok { // zaten JSON string ise oldugu gibi
		return s
	}
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}

func lenOf(v any) int {
	if a, ok := v.([]any); ok {
		return len(a)
	}
	return 0
}

// names — [{name: X}, ...] → [X, ...].
func names(v any) []string {
	var out []string
	if a, ok := v.([]any); ok {
		for _, it := range a {
			if m, ok := it.(map[string]any); ok {
				if n, ok := m["name"].(string); ok {
					out = append(out, n)
				}
			}
		}
	}
	return out
}
