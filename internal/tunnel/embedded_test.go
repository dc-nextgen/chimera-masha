package tunnel

import (
	"context"
	"testing"

	"github.com/fatedier/frp/client/proxy"
)

// TestEmbedded_ConfigBos_KAPALI — Config bos ise EmbeddedFrpc sidecar ile AYNI davransin:
// Status "off", Start no-op (nil), Stop nil.
func TestEmbedded_ConfigBos_KAPALI(t *testing.T) {
	e := NewEmbedded("")
	if state, msg := e.Status(); state != "off" || msg != "" {
		t.Fatalf("Status() = (%q, %q), beklenen (\"off\", \"\")", state, msg)
	}
	if err := e.Start(context.Background()); err != nil {
		t.Fatalf("Start() bos config ile hata dondurdu: %v", err)
	}
	if err := e.Stop(); err != nil {
		t.Fatalf("Stop() hata dondurdu: %v", err)
	}
	// Start sonrasi da hala "off" olmali (baslatilmadi).
	if state, _ := e.Status(); state != "off" {
		t.Fatalf("Start() sonrasi Status() = %q, beklenen \"off\"", state)
	}
}

// TestMapPhase — frp proxy.WorkingStatus.Phase → bizim durum sozlesmemiz eslesmesi. Canli frps
// GEREKMEZ (saf fonksiyon).
func TestMapPhase(t *testing.T) {
	cases := []struct {
		name      string
		phase     string
		errMsg    string
		wantState string
		wantMsg   string
	}{
		{"running", proxy.ProxyPhaseRunning, "", "connected", ""},
		{"new", proxy.ProxyPhaseNew, "", "connecting", ""},
		{"wait_start", proxy.ProxyPhaseWaitStart, "", "connecting", ""},
		{"start_err_generic", proxy.ProxyPhaseStartErr, "boom", "error", "boom"},
		{"start_err_already_exists", proxy.ProxyPhaseStartErr, "proxy [x] already exists", "conflict", conflictMsg},
		{"start_err_already_used", proxy.ProxyPhaseStartErr, "port already used", "conflict", conflictMsg},
		{"start_err_ALREADY_uppercase", proxy.ProxyPhaseStartErr, "ALREADY in use", "conflict", conflictMsg},
		{"check_failed", proxy.ProxyPhaseCheckFailed, "healthcheck down", "error", "healthcheck down"},
		{"check_failed_conflict", proxy.ProxyPhaseCheckFailed, "already exists somewhere", "conflict", conflictMsg},
		{"closed", proxy.ProxyPhaseClosed, "", "off", ""},
		{"unknown", "weird-phase", "", "connecting", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state, msg := mapPhase(tc.phase, tc.errMsg)
			if state != tc.wantState || msg != tc.wantMsg {
				t.Fatalf("mapPhase(%q,%q) = (%q,%q), beklenen (%q,%q)", tc.phase, tc.errMsg, state, msg, tc.wantState, tc.wantMsg)
			}
		})
	}
}
