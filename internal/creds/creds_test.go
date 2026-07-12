package creds

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// memStore — testte gercek OS keychain'e DOKUNMAYAN sahte Store (bellek-ici map).
type memStore struct {
	m map[string]string
}

func newMemStore() *memStore { return &memStore{m: map[string]string{}} }

func (s *memStore) Get(key string) (string, error) {
	v, ok := s.m[key]
	if !ok {
		return "", ErrNotFound
	}
	return v, nil
}

func (s *memStore) Set(key, val string) error {
	s.m[key] = val
	return nil
}

func (s *memStore) Delete(key string) error {
	delete(s.m, key)
	return nil
}

func TestFileStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	fs := FileStore{Dir: dir}

	if _, err := fs.Get("missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("beklenen ErrNotFound, alinan: %v", err)
	}

	if err := fs.Set("k1", `{"user":"x"}`); err != nil {
		t.Fatalf("Set: %v", err)
	}
	v, err := fs.Get("k1")
	if err != nil || v != `{"user":"x"}` {
		t.Fatalf("Get sonrasi = %q, %v", v, err)
	}

	info, err := os.Stat(filepath.Join(dir, "k1.json"))
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("beklenen 0600, alinan %v", info.Mode().Perm())
	}

	if err := fs.Delete("k1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := fs.Get("k1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("silme sonrasi beklenen ErrNotFound, alinan: %v", err)
	}

	// Delete idempotent olmali (var olmayan anahtar hata vermez).
	if err := fs.Delete("k1"); err != nil {
		t.Fatalf("ikinci Delete hata verdi: %v", err)
	}
}

func TestResolveWithoutKeyringReturnsFileStore(t *testing.T) {
	dir := t.TempDir()
	store, label := Resolve(false, "masha-agent-test", dir)
	if label != "dosya (0600)" {
		t.Fatalf("beklenen 'dosya (0600)', alinan %q", label)
	}
	if _, ok := store.(FileStore); !ok {
		t.Fatalf("beklenen FileStore, alinan %T", store)
	}
}

// TestCredManager_Migration — Store olarak sahte memStore kullanan bir CredManager, mevcut
// legacy dosyadaki kimligi Store'a TASIR + dosyayi SILER (tek seferlik migrasyon). Gercek
// OS keychain'e DOKUNMAZ (hermetik).
func TestCredManager_Migration(t *testing.T) {
	dir := t.TempDir()
	legacyPath := filepath.Join(dir, ".masha-db.json")
	if err := os.WriteFile(legacyPath, []byte(`{"user":"eski"}`), 0o600); err != nil {
		t.Fatalf("legacy dosya yazilamadi: %v", err)
	}

	ms := newMemStore()
	cm := NewCredManager(ms, "keychain (test)")

	v, err := cm.Load("db", legacyPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if v != `{"user":"eski"}` {
		t.Fatalf("beklenen legacy icerik, alinan %q", v)
	}

	// migrasyon: Store'da artik var, dosya silinmis olmali.
	if _, ok := ms.m["db"]; !ok {
		t.Fatalf("migrasyon sonrasi Store'da 'db' anahtari yok")
	}
	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Fatalf("migrasyon sonrasi legacy dosya hala duruyor (err=%v)", err)
	}

	// ikinci Load — dosya artik yok, Store'dan okunmali (hala ayni deger).
	v2, err := cm.Load("db", legacyPath)
	if err != nil {
		t.Fatalf("ikinci Load: %v", err)
	}
	if v2 != v {
		t.Fatalf("ikinci Load farkli deger dondurdu: %q != %q", v2, v)
	}
}

// TestCredManager_FileModeUsesLegacyPathDirectly — Store bir FileStore ise (keychain YOK
// senaryosu) Load/Save DOGRUDAN legacyPath'e okur/yazar; bugunku .masha-db.json davranisi
// birebir korunur (anahtar-turetilmis "db.json" adi KULLANILMAZ).
func TestCredManager_FileModeUsesLegacyPathDirectly(t *testing.T) {
	dir := t.TempDir()
	legacyPath := filepath.Join(dir, ".masha-db.json")

	store, label := Resolve(false, "masha-agent-test", dir)
	cm := NewCredManager(store, label)

	if err := cm.Save("db", legacyPath, `{"user":"y"}`); err != nil {
		t.Fatalf("Save: %v", err)
	}
	// dogrudan legacyPath'te olmali (key-turetilmis "db.json" DEGIL).
	b, err := os.ReadFile(legacyPath)
	if err != nil {
		t.Fatalf("legacyPath okunamadi: %v", err)
	}
	if string(b) != `{"user":"y"}` {
		t.Fatalf("beklenmeyen icerik: %q", b)
	}
	if _, err := os.Stat(filepath.Join(dir, "db.json")); !os.IsNotExist(err) {
		t.Fatalf("beklenmeyen key-turetilmis dosya olusmus")
	}

	v, err := cm.Load("db", legacyPath)
	if err != nil || v != `{"user":"y"}` {
		t.Fatalf("Load = %q, %v", v, err)
	}
}

// TestKeyringIntegration — GERCEK OS keychain'e dokunur; varsayilan CI/headless kosumda
// ATLANIR (yalniz MASHA_TEST_KEYRING=1 ile calisir; ör. gelistirici Mac'inde elle).
func TestKeyringIntegration(t *testing.T) {
	if os.Getenv("MASHA_TEST_KEYRING") != "1" {
		t.Skip("MASHA_TEST_KEYRING=1 degil — gercek OS keychain'e dokunulmuyor")
	}
	ks := KeyringStore{Service: "masha-agent-test"}
	if err := ks.Set("itest", "deger"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	defer ks.Delete("itest")
	v, err := ks.Get("itest")
	if err != nil || v != "deger" {
		t.Fatalf("Get = %q, %v", v, err)
	}
}
