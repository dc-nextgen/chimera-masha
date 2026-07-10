package tunnel

import "testing"

// TestStatusOff — Config bos ise tunel hic acilmamis ("off"), state hicbir zaman set edilmemis olsa bile.
func TestStatusOff(t *testing.T) {
	s := NewSidecar("frpc", "")
	state, msg := s.Status()
	if state != "off" || msg != "" {
		t.Fatalf("got (%q, %q), want (\"off\", \"\")", state, msg)
	}
}

// TestStatusInitialConnecting — Config doluysa ama henuz frpc satiri gelmediyse varsayilan "connecting".
func TestStatusInitialConnecting(t *testing.T) {
	s := NewSidecar("frpc", "x")
	state, _ := s.Status()
	if state != "connecting" {
		t.Fatalf("got %q, want \"connecting\"", state)
	}
}

// TestInspectLineConflict — "already exists" gibi frps ret satirlari catisma durumuna geciriyor.
func TestInspectLineConflict(t *testing.T) {
	s := NewSidecar("frpc", "x")
	s.inspectLine("[masha-x] start error: proxy [masha-x] already exists")
	state, msg := s.Status()
	if state != "conflict" {
		t.Fatalf("got state %q, want \"conflict\"", state)
	}
	if msg == "" {
		t.Fatal("conflict msg bos olmamali")
	}
}

// TestInspectLineConflictThenConnected — mesru ayni-makine restart: sonraki "start proxy success"
// catismayi TEMIZLER (yanlis-pozitif kalici kalmasin).
func TestInspectLineConflictThenConnected(t *testing.T) {
	s := NewSidecar("frpc", "x")
	s.inspectLine("[masha-x] start error: proxy [masha-x] already exists")
	s.inspectLine("[masha-x] start proxy success")
	state, msg := s.Status()
	if state != "connected" {
		t.Fatalf("got state %q, want \"connected\"", state)
	}
	if msg != "" {
		t.Fatalf("connected msg bos olmali, got %q", msg)
	}
}

// TestInspectLineCaseInsensitive — buyuk/kucuk harf farki catisma tespitini bozmamali.
func TestInspectLineCaseInsensitive(t *testing.T) {
	s := NewSidecar("frpc", "x")
	s.inspectLine("some prefix ... Port Already Used ... suffix")
	state, _ := s.Status()
	if state != "conflict" {
		t.Fatalf("got state %q, want \"conflict\"", state)
	}
}

// TestInspectLineOtherVariants — "already used" / "already in use" varyantlari da yakalanmali.
func TestInspectLineOtherVariants(t *testing.T) {
	for _, line := range []string{
		"proxy name [masha-x] is already used",
		"port [7000] already in use",
	} {
		s := NewSidecar("frpc", "x")
		s.inspectLine(line)
		state, _ := s.Status()
		if state != "conflict" {
			t.Fatalf("line %q: got state %q, want \"conflict\"", line, state)
		}
	}
}
