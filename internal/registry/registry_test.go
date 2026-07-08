package registry

import "testing"

func TestRegistryAddGetListRemove(t *testing.T) {
	r := New()
	if r.Get("x") != nil {
		t.Error("bos registry'de Get nil olmali")
	}
	r.Add(&Connection{Name: "b", ServerLabel: "masha-b", Kind: "mssql"})
	r.Add(&Connection{Name: "a", ServerLabel: "masha-a", Kind: "erpnext"})
	if c := r.Get("masha-a"); c == nil || c.Kind != "erpnext" {
		t.Errorf("Get(masha-a) yanlis: %+v", c)
	}
	// List deterministik sirali (ServerLabel).
	l := r.List()
	if len(l) != 2 || l[0].ServerLabel != "masha-a" || l[1].ServerLabel != "masha-b" {
		t.Errorf("List sirasi yanlis: %v", []string{l[0].ServerLabel, l[1].ServerLabel})
	}
	// Add ayni label = guncelle (mukerrer degil).
	r.Add(&Connection{Name: "b2", ServerLabel: "masha-b", Kind: "mssql"})
	if len(r.List()) != 2 {
		t.Errorf("ayni label tekrar eklenince 2 kalmali, %d", len(r.List()))
	}
	if got := r.Remove("masha-a"); got == nil || got.Name != "a" {
		t.Errorf("Remove donen yanlis: %+v", got)
	}
	if r.Get("masha-a") != nil || len(r.List()) != 1 {
		t.Error("Remove sonrasi kalmamali")
	}
}
