package webui

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

// TestSettingsHandler — /settings enjekte edilen GERCEK config'i doner (§4: uydurma yok).
func TestSettingsHandler(t *testing.T) {
	u := &UI{
		settings: SettingsInfo{
			Version: "9.9.9", WebAddr: "127.0.0.1:8787", WebTLS: true,
			AuthEnabled: true, TunnelMode: "embed", CredStore: "keychain",
			LLMEnabled: true, ERPNextMask: true, Plan: "trial",
		},
		tunnelStatus: func() (string, string) { return "connected", "" },
	}
	rec := httptest.NewRecorder()
	u.settingsHandler(rec, httptest.NewRequest("GET", "/settings", nil))
	if rec.Code != 200 {
		t.Fatalf("200 bekleniyordu, goren %d", rec.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["version"] != "9.9.9" || resp["web_addr"] != "127.0.0.1:8787" {
		t.Errorf("beklenmeyen icerik: %v", resp)
	}
	if resp["web_tls"] != true || resp["auth_enabled"] != true {
		t.Errorf("bool alanlar yanlis: %v", resp)
	}
	if resp["tunnel_mode"] != "embed" || resp["cred_store"] != "keychain" {
		t.Errorf("mod/kimlik alanlari yanlis: %v", resp)
	}
	if resp["plan"] != "trial" {
		t.Errorf("plan yanlis: %v", resp)
	}
	if resp["tunnel_state"] != "connected" {
		t.Errorf("canli tunel durumu eklenmemis: %v", resp)
	}
	if _, ok := resp["tunnel_msg"]; ok {
		t.Errorf("bos tunnel_msg eklenmemeliydi: %v", resp)
	}
}
