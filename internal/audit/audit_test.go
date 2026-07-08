package audit

import (
	"testing"
	"time"
)

func TestChainLinks(t *testing.T) {
	a, err := New("") // yalniz stdout
	if err != nil {
		t.Fatal(err)
	}
	a.nowFn = func() time.Time { return time.Unix(0, 0).UTC() } // deterministik

	if a.Head() != "genesis" {
		t.Fatalf("baslangic head genesis degil: %s", a.Head())
	}
	a.Record(map[string]any{"decision": "allow", "tool": "count"})
	h1 := a.Head()
	if h1 == "genesis" || len(h1) != 64 {
		t.Fatalf("ilk hash beklenmedik: %s", h1)
	}
	// AYNI icerik tekrar => prev degistigi icin FARKLI hash (zincir).
	a.Record(map[string]any{"decision": "allow", "tool": "count"})
	h2 := a.Head()
	if h2 == h1 {
		t.Fatal("ayni icerik ayni hash uretti (zincir prev'e baglanmiyor)")
	}
}

func TestRecentNewestFirstAndFilter(t *testing.T) {
	a, _ := New("")
	a.nowFn = func() time.Time { return time.Unix(0, 0).UTC() }
	a.Record(map[string]any{"decision": "allow", "server": "auroville", "tool": "count_x"})
	a.Record(map[string]any{"decision": "allow", "server": "erpnext", "tool": "get_count"})
	a.Record(map[string]any{"decision": "error", "server": "auroville", "tool": "list_x"})

	all := a.Recent(0, "")
	if len(all) != 3 {
		t.Fatalf("hepsi 3 bekleniyordu, %d", len(all))
	}
	// YENIDEN ESKIYE: en son kayit ilk sirada.
	if all[0]["tool"] != "list_x" {
		t.Fatalf("newest-first degil: %v", all[0]["tool"])
	}
	// server filtresi: yalniz auroville (2 kayit).
	av := a.Recent(0, "auroville")
	if len(av) != 2 {
		t.Fatalf("auroville 2 bekleniyordu, %d", len(av))
	}
	for _, e := range av {
		if e["server"] != "auroville" {
			t.Fatalf("filtre sizdirdi: %v", e["server"])
		}
	}
	// limit: yalniz en son 1.
	if l := a.Recent(1, ""); len(l) != 1 || l[0]["tool"] != "list_x" {
		t.Fatalf("limit=1 newest bekleniyordu, %v", l)
	}
}

func TestRecentRingCap(t *testing.T) {
	a, _ := New("")
	a.nowFn = func() time.Time { return time.Unix(0, 0).UTC() }
	for i := 0; i < recentCap+50; i++ {
		a.Record(map[string]any{"decision": "allow", "n": i})
	}
	if got := len(a.Recent(0, "")); got != recentCap {
		t.Fatalf("ring cap %d bekleniyordu, %d", recentCap, got)
	}
}
