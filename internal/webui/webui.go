// Package webui — YEREL yuz (durum + araclari DENE + sema + ONBOARDING), varsayilan 127.0.0.1'e
// bind (yalniz kutudan erisilir; plain-HTTP guvenli — §17.9). Ayri masaustu app / notarize YOK.
// UI = React+shadcn (web/), Go binary'ye EMBED edilir → ayni origin'de JSON API + statik SPA.
// /try connector'i DOGRUDAN cagirir (yerel/salt-okunur) + audit'ler. Bulut yolu ayri + mTLS.
//
// Onboarding (Faz 2, docs §17.3): /onboard/suggest sema→aday manifest; /onboard/apply
// duzenlenmis manifest'i dogrula+yaz+HOT-RELOAD (apply callback → live.Set; restart YOK).
package webui

import (
	"context"
	"encoding/json"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dc-nextgen/chimera-masha/internal/audit"
	"github.com/dc-nextgen/chimera-masha/internal/connector"
	"github.com/dc-nextgen/chimera-masha/internal/connector/erpnext"
	"github.com/dc-nextgen/chimera-masha/internal/live"
	"github.com/dc-nextgen/chimera-masha/internal/manifest"
	"github.com/dc-nextgen/chimera-masha/internal/onboard"
	"github.com/dc-nextgen/chimera-masha/internal/registry"
)

type UI struct {
	man    *live.Manifest // CANLI manifest (apply hot-swap eder)
	conn   connector.Connector
	aud    *audit.Auditor
	static fs.FS // gomulu web/dist (nil ise UI yok, yalniz JSON API)
	// apply — onboarding "uygula": manifest'i KALICI yaz + live.Set (hot-reload). main saglar.
	// nil ise onboarding salt-okunur (suggest calisir, apply 501 doner).
	apply func(*manifest.Manifest) error
	// adviser — LLM danismani (opsiyonel, §5). nil ise /onboard/advise 501 doner (heuristik yeter).
	adviser *onboard.Suggester
	// auth — yerel yuz parola korumasi (opsiyonel). nil ise auth KAPALI (loopback'te guvenli, §17.9).
	auth *auth
	// dbConnect — ekrandan DB kimligi: DSN kur + connector.Connect + YERELE kalici yaz. main saglar.
	// nil ise /db/connect 501 (yalnız env/dosya DSN). Kimlik ASLA buluta gitmez (§3).
	dbConnect func(connector.DBFields) error
	// reg — CANLI baglanti kaydi (§19.2). /connections listeler; nil ise tek-baglanti (geriye-donuk).
	reg *registry.Registry
	// erpnextConnect — ekrandan ErpNext bağlan (URL+api_key+secret → Connect+kayıt). nil ise 501.
	erpnextConnect func(erpnext.Fields) error
	// primaryLabel — ?conn verilmezse varsayılan bağlantı (mssql). Per-connection op scoping (§19.2).
	primaryLabel string
	// plan — satis yuzeyi (Plan/Yukselt ekrani): deneme rozeti + iletisim/talep. Bos = normal.
	plan Plan
}

// Plan — yerel yuz "Plan/Yukselt" ekrani icin config (deneme durumu + talep kanali).
type Plan struct {
	Plan          string `json:"plan"`            // "" (normal) | "trial"
	TrialLimitUSD string `json:"trial_limit_usd"` // deneme token limiti gosterimi
	ContactEmail  string `json:"contact_email"`   // "Talep ilet" mailto (bos ise gizli)
	RequestURL    string `json:"request_url"`     // opsiyonel bulut talep ucu (bos ise yalniz mailto)
}

// Deps — webui bagimliliklari (parametre patlamasi yerine struct; main doldurur).
type Deps struct {
	Live           *live.Manifest
	Conn           connector.Connector
	Aud            *audit.Auditor
	Static         fs.FS
	Apply          func(*manifest.Manifest) error
	Adviser        *onboard.Suggester
	Password       string
	DBConnect      func(connector.DBFields) error
	Registry       *registry.Registry
	ErpnextConnect func(erpnext.Fields) error
	PrimaryLabel   string
	Plan           Plan
}

func New(d Deps) *UI {
	return &UI{man: d.Live, conn: d.Conn, aud: d.Aud, static: d.Static, apply: d.Apply,
		adviser: d.Adviser, auth: newAuth(d.Password), dbConnect: d.DBConnect, reg: d.Registry,
		erpnextConnect: d.ErpnextConnect, primaryLabel: d.PrimaryLabel, plan: d.Plan}
}

// connFor — istekteki ?conn=<label> bağlantısı (yoksa primary). Per-connection op scoping.
func (u *UI) connFor(r *http.Request) *registry.Connection {
	if u.reg == nil {
		return nil
	}
	label := r.URL.Query().Get("conn")
	if label == "" {
		label = u.primaryLabel
	}
	return u.reg.Get(label)
}

// toolsFromOpenAPI — connector'un OpenAPI'sinden birleşik araç metadata (mssql+erpnext aynı yol).
// Araçlar ekranı bunu kullanır: her tool = ad + açıklama + parametreler (ad/tip/zorunlu).
func toolsFromOpenAPI(spec map[string]any) []map[string]any {
	out := []map[string]any{}
	paths, _ := spec["paths"].(map[string]any)
	names := make([]string, 0, len(paths))
	for p := range paths {
		names = append(names, p)
	}
	sort.Strings(names)
	for _, p := range names {
		post, _ := (paths[p].(map[string]any))["post"].(map[string]any)
		if post == nil {
			continue
		}
		name := strOr(post["operationId"], strings.TrimPrefix(p, "/"))
		desc := strOr(post["summary"], name)
		var params []map[string]any
		req := map[string]bool{}
		if rb, _ := post["requestBody"].(map[string]any); rb != nil {
			if ct, _ := rb["content"].(map[string]any); ct != nil {
				if aj, _ := ct["application/json"].(map[string]any); aj != nil {
					if sc, _ := aj["schema"].(map[string]any); sc != nil {
						if rl, _ := sc["required"].([]any); rl != nil {
							for _, x := range rl {
								if s, ok := x.(string); ok {
									req[s] = true
								}
							}
						}
						if props, _ := sc["properties"].(map[string]any); props != nil {
							pn := make([]string, 0, len(props))
							for k := range props {
								pn = append(pn, k)
							}
							sort.Strings(pn)
							for _, k := range pn {
								typ := "string"
								if pm, _ := props[k].(map[string]any); pm != nil {
									typ = strOr(pm["type"], "string")
								}
								params = append(params, map[string]any{"name": k, "type": typ, "required": req[k]})
							}
						}
					}
				}
			}
		}
		out = append(out, map[string]any{"name": name, "description": desc, "params": params})
	}
	return out
}

func strOr(v any, def string) string {
	if s, ok := v.(string); ok && s != "" {
		return s
	}
	return def
}

func (u *UI) Handler() http.Handler {
	mux := http.NewServeMux()
	// Acik uclar: auth durumu + login (login gerekli mi ve parola girisi).
	mux.HandleFunc("/auth/status", u.authStatus)
	mux.HandleFunc("/auth/login", u.authLogin)
	// Korumali uclar (auth aciksa Bearer token ister): veri + yeniden-yapilandirma.
	mux.HandleFunc("/healthz", u.gated(u.healthz))
	mux.HandleFunc("/connections", u.gated(u.connections))
	mux.HandleFunc("/db/status", u.gated(u.dbStatus))
	mux.HandleFunc("/db/connect", u.gated(u.dbConnectHandler))
	mux.HandleFunc("/erpnext/connect", u.gated(u.erpnextConnectHandler))
	mux.HandleFunc("/tools", u.gated(u.tools))
	mux.HandleFunc("/try", u.gated(u.try))
	mux.HandleFunc("/schema", u.gated(u.schema))
	mux.HandleFunc("/onboard/suggest", u.gated(u.onboardSuggest))
	mux.HandleFunc("/onboard/advise", u.gated(u.onboardAdvise))
	mux.HandleFunc("/onboard/apply", u.gated(u.onboardApply))
	mux.HandleFunc("/onboard/export", u.gated(u.onboardExport))
	mux.HandleFunc("/audit/log", u.gated(u.auditLog))
	mux.HandleFunc("/plan", u.gated(u.planInfo))
	mux.HandleFunc("/", u.serveStatic) // SPA kabuk (acik — login ekrani yuklenebilsin)
	return mux
}

func (u *UI) dbOK() bool {
	if u.conn == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return u.conn.Health(ctx) == nil
}

func (u *UI) auditHead() string {
	if u.aud == nil {
		return ""
	}
	return u.aud.Head()
}

func (u *UI) healthz(w http.ResponseWriter, r *http.Request) {
	m := u.man.Get()
	resp := map[string]any{"ok": true, "db": u.dbOK(), "audit_head": u.auditHead()}
	if m != nil {
		resp["connector"], resp["erp_kind"], resp["tools"] = m.Name, m.ERPKind, m.ToolNames()
	}
	writeJSON(w, 200, resp)
}

// tools — SEÇİLİ bağlantının araç metadata'sı (OpenAPI'den birleşik; §19.2 per-connection).
func (u *UI) tools(w http.ResponseWriter, r *http.Request) {
	c := u.connFor(r)
	if c == nil {
		writeJSON(w, 200, map[string]any{"connector": "", "kind": "", "tools": []any{}})
		return
	}
	writeJSON(w, 200, map[string]any{
		"connector": c.Name, "label": c.Label, "kind": c.Kind,
		"tools": toolsFromOpenAPI(c.Conn.OpenAPI(c.ServerLabel)),
	})
}

// try — SEÇİLİ bağlantıdan arac DENEME. Salt-okunur (connector.AllowTool doğrular).
func (u *UI) try(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "POST"})
		return
	}
	var req struct {
		Tool string         `json:"tool"`
		Args map[string]any `json:"args"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "gecersiz istek"})
		return
	}
	c := u.connFor(r)
	if c == nil {
		writeJSON(w, 404, map[string]string{"error": "bağlantı yok"})
		return
	}
	if !c.Conn.AllowTool(req.Tool) {
		writeJSON(w, 404, map[string]string{"error": "bilinmeyen/izinsiz tool"})
		return
	}
	if req.Args == nil {
		req.Args = map[string]any{}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	res, err := c.Conn.Call(ctx, req.Tool, req.Args)
	if u.aud != nil {
		rec := map[string]any{"decision": "allow", "source": "webui", "server": c.ServerLabel, "tool": req.Tool}
		if err != nil {
			rec["decision"] = "error"
			rec["err"] = err.Error()
		}
		u.aud.Record(rec)
	}
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, res)
}

// connections — kayıtlı tüm bağlantılar (§19.2). Bağlantılar yöneticisi bunu listeler.
// reg yoksa (geriye-dönük) tek primary bağlantıyı döndürür.
func (u *UI) connections(w http.ResponseWriter, r *http.Request) {
	type connInfo struct {
		Name        string `json:"name"`
		Label       string `json:"label"`
		Kind        string `json:"kind"`
		ServerLabel string `json:"server_label"`
		Connected   bool   `json:"connected"`
	}
	var out []connInfo
	if u.reg != nil {
		for _, c := range u.reg.List() {
			out = append(out, connInfo{c.Name, c.Label, c.Kind, c.ServerLabel, c.Conn.Connected()})
		}
	}
	writeJSON(w, 200, map[string]any{"connections": out})
}

// dbStatus — DB baglantisi kurulu mu (ucuz). Ekrandan-baglan akisi + Durum ekrani.
func (u *UI) dbStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"connected": u.conn.Connected(), "can_connect": u.dbConnect != nil})
}

// dbConnectHandler — ekrandan DB kimligi al → DSN kur + Connect (ping) + YERELE kalici yaz.
// Kimlik YERELDE kalir (§3); asla buluta/log'a gitmez. dbConnect yoksa 501 (env/dosya DSN modu).
func (u *UI) dbConnectHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "POST"})
		return
	}
	if u.dbConnect == nil {
		writeJSON(w, 501, map[string]string{"error": "ekrandan-baglan kapali (DSN env/dosyadan)"})
		return
	}
	var f connector.DBFields
	if err := json.NewDecoder(r.Body).Decode(&f); err != nil {
		writeJSON(w, 400, map[string]string{"error": "gecersiz istek"})
		return
	}
	if strings.TrimSpace(f.Host) == "" || strings.TrimSpace(f.Database) == "" || strings.TrimSpace(f.User) == "" {
		writeJSON(w, 400, map[string]string{"error": "host, database, user zorunlu"})
		return
	}
	if err := u.dbConnect(f); err != nil {
		if u.aud != nil { // parola LOG'lanmaz — yalniz host/db + hata.
			u.aud.Record(map[string]any{"decision": "error", "source": "db-connect",
				"host": f.Host, "database": f.Database, "err": err.Error()})
		}
		writeJSON(w, 502, map[string]string{"error": err.Error()})
		return
	}
	if u.aud != nil {
		u.aud.Record(map[string]any{"decision": "allow", "source": "db-connect",
			"host": f.Host, "database": f.Database})
	}
	writeJSON(w, 200, map[string]any{"ok": true, "connected": true})
}

// erpnextConnectHandler — ekrandan ErpNext bağlan (URL+api_key+api_secret → Connect + kayıt + YERELE yaz).
// Kimlik buluta/log'a GİTMEZ (§3). erpnextConnect yoksa 501.
func (u *UI) erpnextConnectHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "POST"})
		return
	}
	if u.erpnextConnect == nil {
		writeJSON(w, 501, map[string]string{"error": "erpnext ekrandan-baglan kapali"})
		return
	}
	var f erpnext.Fields
	if err := json.NewDecoder(r.Body).Decode(&f); err != nil {
		writeJSON(w, 400, map[string]string{"error": "gecersiz istek"})
		return
	}
	if strings.TrimSpace(f.URL) == "" {
		writeJSON(w, 400, map[string]string{"error": "url zorunlu"})
		return
	}
	if err := u.erpnextConnect(f); err != nil {
		if u.aud != nil { // sir LOG'lanmaz — yalniz url + hata.
			u.aud.Record(map[string]any{"decision": "error", "source": "erpnext-connect", "url": f.URL, "err": err.Error()})
		}
		writeJSON(w, 502, map[string]string{"error": err.Error()})
		return
	}
	if u.aud != nil {
		u.aud.Record(map[string]any{"decision": "allow", "source": "erpnext-connect", "url": f.URL})
	}
	writeJSON(w, 200, map[string]any{"ok": true, "connected": true})
}

// schema — SEÇİLİ bağlantının sema YAPISI (introspeksiyon; satir okumaz). mssql=tablo, erpnext=doctype.
func (u *UI) schema(w http.ResponseWriter, r *http.Request) {
	c := u.connFor(r)
	if c == nil {
		writeJSON(w, 404, map[string]string{"error": "bağlantı yok"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	sc, err := c.Conn.Introspect(ctx)
	if err != nil {
		writeJSON(w, 502, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, sc)
}

// onboardSuggest — sema YAPISINDAN aday manifest (satir okumaz). GET=tum tablolar; POST=Selection
// (name/label/tables). Cikti ADAYDIR — operator duzenler, sonra /onboard/apply'a POST'lar.
func (u *UI) onboardSuggest(w http.ResponseWriter, r *http.Request) {
	var sel onboard.Selection
	if r.Method == http.MethodPost && r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&sel); err != nil && err.Error() != "EOF" {
			writeJSON(w, 400, map[string]string{"error": "gecersiz Selection"})
			return
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	sc, err := u.conn.Introspect(ctx)
	if err != nil {
		writeJSON(w, 502, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, onboard.Suggest(sc, sel))
}

// onboardAdvise — LLM danismani (opsiyonel): sema YAPISINI siniflandirir (tablo tur + hassas-flag +
// PII/isim onerisi). Yalniz yapi gonderilir (satir yok). adviser yoksa 501 (heuristik yeter).
func (u *UI) onboardAdvise(w http.ResponseWriter, r *http.Request) {
	if u.adviser == nil {
		writeJSON(w, 501, map[string]string{"error": "LLM danismani yapilandirilmadi (MASHA_LLM_BASE_URL/MODEL)"})
		return
	}
	var sel onboard.Selection
	if r.Method == http.MethodPost && r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&sel); err != nil && err.Error() != "EOF" {
			writeJSON(w, 400, map[string]string{"error": "gecersiz Selection"})
			return
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 100*time.Second)
	defer cancel()
	sc, err := u.conn.Introspect(ctx)
	if err != nil {
		writeJSON(w, 502, map[string]string{"error": err.Error()})
		return
	}
	sc = filterSchema(sc, sel.Tables)
	adv, err := u.adviser.Advise(ctx, sc)
	if err != nil {
		writeJSON(w, 502, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"tables": adv})
}

// filterSchema — semayi secili "schema.table" listesine indirger (bos => hepsi). LLM prompt'unu sinirlar.
func filterSchema(sc *connector.Schema, tables []string) *connector.Schema {
	if len(tables) == 0 {
		return sc
	}
	want := map[string]bool{}
	for _, t := range tables {
		want[strings.ToLower(strings.TrimSpace(t))] = true
	}
	out := &connector.Schema{}
	for _, t := range sc.Tables {
		if want[strings.ToLower(t.Schema+"."+t.Name)] {
			out.Tables = append(out.Tables, t)
		}
	}
	return out
}

// onboardExport — yururlukteki manifest'i JSON dondurur (config TASIMA; §19.3). KİMLİK İÇERMEZ
// (creds ayri dosyada/keychain; manifest yalniz yapi/eslem) → danisman baska musteriye tasiyabilir.
func (u *UI) onboardExport(w http.ResponseWriter, r *http.Request) {
	m := u.man.Get()
	if m == nil {
		writeJSON(w, 404, map[string]string{"error": "manifest yok"})
		return
	}
	writeJSON(w, 200, m)
}

// onboardApply — duzenlenmis manifest'i dogrula + KALICI yaz + hot-reload (live.Set via apply).
// Bilinmeyen alan reddedilir (manifest.Load ile ayni katilik). apply yoksa 501.
func (u *UI) onboardApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "POST"})
		return
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	var m manifest.Manifest
	if err := dec.Decode(&m); err != nil {
		writeJSON(w, 400, map[string]string{"error": "manifest JSON gecersiz: " + err.Error()})
		return
	}
	if err := m.Validate(); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	if u.apply == nil {
		writeJSON(w, 501, map[string]string{"error": "apply kapali (salt-okunur onboarding)"})
		return
	}
	if err := u.apply(&m); err != nil {
		if u.aud != nil {
			u.aud.Record(map[string]any{"decision": "error", "source": "onboard-apply", "err": err.Error()})
		}
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if u.aud != nil {
		u.aud.Record(map[string]any{"decision": "allow", "source": "onboard-apply",
			"connector": m.Name, "tools": len(m.Tools)})
	}
	writeJSON(w, 200, map[string]any{"ok": true, "connector": m.Name, "tools": m.ToolNames()})
}

// auditLog — son denetim kayitlari (Etkinlik akisi + "son test" rozeti). ?conn=<label> verilirse
// yalniz o baglantinin server_label kayitlari; ?limit sinir (default 50). aud yoksa bos liste.
// Salt-OKUR (yerel yuz gosterimi); hash-zincir tam dosyada. §4: uydurma yok — gercek kayitlar.
func (u *UI) auditLog(w http.ResponseWriter, r *http.Request) {
	if u.aud == nil {
		writeJSON(w, 200, map[string]any{"records": []any{}})
		return
	}
	server := ""
	if label := r.URL.Query().Get("conn"); label != "" && u.reg != nil {
		if c := u.reg.Get(label); c != nil {
			server = c.ServerLabel
		} else {
			server = label // bilinmeyen label → yine de dene (server_label = label olabilir)
		}
	}
	limit := 50
	if s := r.URL.Query().Get("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 && n <= 200 {
			limit = n
		}
	}
	writeJSON(w, 200, map[string]any{"records": u.aud.Recent(limit, server)})
}

// planInfo — satis yuzeyi config'i (Plan/Yukselt ekrani): deneme durumu + talep kanali. Sir YOK.
func (u *UI) planInfo(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, u.plan)
}

// serveStatic — gomulu SPA. Dosya varsa onu, yoksa index.html (client-side routing) sunar.
func (u *UI) serveStatic(w http.ResponseWriter, r *http.Request) {
	if u.static == nil {
		writeJSON(w, 404, map[string]string{"error": "UI gomulu degil (yalniz JSON API)"})
		return
	}
	upath := strings.TrimPrefix(r.URL.Path, "/")
	if upath == "" {
		upath = "index.html"
	}
	b, err := fs.ReadFile(u.static, upath)
	if err != nil {
		b, err = fs.ReadFile(u.static, "index.html") // SPA fallback
		if err != nil {
			http.NotFound(w, r)
			return
		}
		upath = "index.html"
	}
	if ct := mime.TypeByExtension(path.Ext(upath)); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	w.Write(b)
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
