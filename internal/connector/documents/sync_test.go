package documents

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// mockOWUI — hermetik, bellek-ici bir OWUI sunucusu. Yuklenen govdeleri id'ye gore saklar ki
// testler "yuklenen metin gercekten maskelenmis mi" gibi seyleri dogrulayabilsin.
type mockOWUI struct {
	mu        sync.Mutex
	nextID    int64
	uploaded  map[string]string // fileID -> govde
	inKB      map[string]bool   // fileID -> knowledge'a eklendi mi
	deleted   []string
	srv       *httptest.Server
}

func newMockOWUI(t *testing.T) *mockOWUI {
	m := &mockOWUI{uploaded: map[string]string{}, inKB: map[string]bool{}}
	mux := http.NewServeMux()

	mux.HandleFunc("/api/v1/files", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		f, _, err := r.FormFile("file")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer f.Close()
		b, _ := io.ReadAll(f)

		id := atomic.AddInt64(&m.nextID, 1)
		fileID := "file-" + itoa(id)
		m.mu.Lock()
		m.uploaded[fileID] = string(b)
		m.mu.Unlock()

		resp, _ := json.Marshal(map[string]string{"id": fileID})
		w.WriteHeader(http.StatusOK)
		w.Write(resp)
	})

	mux.HandleFunc("/api/v1/knowledge/", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			FileID string `json:"file_id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)

		m.mu.Lock()
		switch {
		case strings.HasSuffix(r.URL.Path, "/file/add"):
			m.inKB[body.FileID] = true
		case strings.HasSuffix(r.URL.Path, "/file/remove"):
			m.inKB[body.FileID] = false
		}
		m.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("/api/v1/files/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			http.Error(w, "method", http.StatusMethodNotAllowed)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/api/v1/files/")
		m.mu.Lock()
		delete(m.uploaded, id)
		m.deleted = append(m.deleted, id)
		m.mu.Unlock()
		w.WriteHeader(http.StatusOK)
	})

	m.srv = httptest.NewServer(mux)
	t.Cleanup(m.srv.Close)
	return m
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

func (m *mockOWUI) bodyFor(fileID string) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.uploaded[fileID]
	return b, ok
}

func (m *mockOWUI) isInKB(fileID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.inKB[fileID]
}

func newSyncer(t *testing.T, dir string, mock *mockOWUI, mask bool) *Syncer {
	statePath := filepath.Join(t.TempDir(), "state.json")
	st, err := LoadState(statePath)
	if err != nil {
		t.Fatal(err)
	}
	return &Syncer{
		Dir:         dir,
		KnowledgeID: "kb-1",
		Mask:        mask,
		Client:      &OWUIClient{BaseURL: mock.srv.URL},
		State:       st,
		Logf:        func(string, ...any) {}, // testte sessiz.
	}
}

func TestSyncOnceAdd(t *testing.T) {
	dir := t.TempDir()
	mock := newMockOWUI(t)
	s := newSyncer(t, dir, mock, false)

	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("merhaba"), 0o644); err != nil {
		t.Fatal(err)
	}

	rep, err := s.SyncOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rep.Added != 1 || rep.Updated != 0 || rep.Removed != 0 {
		t.Fatalf("beklenmedik rapor: %+v", rep)
	}
	fs, ok := s.State.Files["a.txt"]
	if !ok {
		t.Fatal("state'e eklenmedi")
	}
	body, ok := mock.bodyFor(fs.FileID)
	if !ok || body != "merhaba" {
		t.Fatalf("yuklenen govde beklenmedik: %q ok=%v", body, ok)
	}
	if !mock.isInKB(fs.FileID) {
		t.Fatal("knowledge'a eklenmedi")
	}
}

func TestSyncOnceUnchangedSkipsReupload(t *testing.T) {
	dir := t.TempDir()
	mock := newMockOWUI(t)
	s := newSyncer(t, dir, mock, false)

	path := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(path, []byte("ayni icerik"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SyncOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	firstID := s.State.Files["a.txt"].FileID

	rep, err := s.SyncOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rep.Added != 0 || rep.Updated != 0 {
		t.Fatalf("degismemis dosya tekrar yuklenmemeli: %+v", rep)
	}
	if s.State.Files["a.txt"].FileID != firstID {
		t.Fatal("fileID degismemeli")
	}
}

func TestSyncOnceUpdate(t *testing.T) {
	dir := t.TempDir()
	mock := newMockOWUI(t)
	s := newSyncer(t, dir, mock, false)

	path := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(path, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SyncOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	oldID := s.State.Files["a.txt"].FileID

	if err := os.WriteFile(path, []byte("v2 degisti"), 0o644); err != nil {
		t.Fatal(err)
	}
	rep, err := s.SyncOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rep.Updated != 1 {
		t.Fatalf("update bekleniyordu: %+v", rep)
	}
	newID := s.State.Files["a.txt"].FileID
	if newID == oldID {
		t.Fatal("fileID degismeli")
	}
	if _, stillThere := mock.bodyFor(oldID); stillThere {
		t.Fatal("eski dosya OWUI'de silinmemis")
	}
	if body, _ := mock.bodyFor(newID); body != "v2 degisti" {
		t.Fatalf("yeni govde beklenmedik: %q", body)
	}
}

func TestSyncOnceRemove(t *testing.T) {
	dir := t.TempDir()
	mock := newMockOWUI(t)
	s := newSyncer(t, dir, mock, false)

	path := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(path, []byte("silinecek"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SyncOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	id := s.State.Files["a.txt"].FileID

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	rep, err := s.SyncOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rep.Removed != 1 {
		t.Fatalf("remove bekleniyordu: %+v", rep)
	}
	if _, ok := s.State.Files["a.txt"]; ok {
		t.Fatal("state'ten dusmedi")
	}
	if _, stillThere := mock.bodyFor(id); stillThere {
		t.Fatal("OWUI'de hala duruyor")
	}
}

func TestSyncOnceMasksPII(t *testing.T) {
	dir := t.TempDir()
	mock := newMockOWUI(t)
	s := newSyncer(t, dir, mock, true) // Mask=true

	path := filepath.Join(dir, "musteri.txt")
	if err := os.WriteFile(path, []byte("iletisim: alice@example.com"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SyncOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	fs := s.State.Files["musteri.txt"]
	body, _ := mock.bodyFor(fs.FileID)
	if strings.Contains(body, "alice@example.com") {
		t.Fatalf("PII maskelenmeden gitmis: %q", body)
	}
	if !strings.Contains(body, "@example.com") {
		t.Fatalf("beklenmedik govde: %q", body)
	}
}

func TestSyncOnceSkipsUnsupported(t *testing.T) {
	dir := t.TempDir()
	mock := newMockOWUI(t)
	s := newSyncer(t, dir, mock, false)

	if err := os.WriteFile(filepath.Join(dir, "eski.doc"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	rep, err := s.SyncOnce(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rep.Skipped != 1 || rep.Added != 0 {
		t.Fatalf("beklenmedik rapor: %+v", rep)
	}
}
