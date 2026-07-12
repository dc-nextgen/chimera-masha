// Package creds — kimlik (DB/ErpNext parola vb.) saklama katmani. §3: kimlik YERELDE kalir,
// buluta ASLA gitmez. Faz 3 (§18): OS keychain (macOS Keychain / Windows Credential Manager /
// Linux Secret Service) tercih edilir; headless Linux'ta (Secret Service yok) ZARIFCE 0600
// dosyaya duser (crash YOK — probeKeyring ile tespit edilir).
package creds

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/zalando/go-keyring"
)

// ErrNotFound — anahtar depoda yok (dosya da yoksa, keychain girisi de yoksa).
var ErrNotFound = errors.New("creds: bulunamadi")

// Store — tek bir kimlik deposu soyutlamasi (keychain veya dosya).
type Store interface {
	Get(key string) (string, error)
	Set(key, val string) error
	Delete(key string) error
}

// ---------------------------------------------------------------------------
// KeyringStore — OS keychain sarmalayici (go-keyring; macOS=security, Windows=wincred,
// Linux=D-Bus Secret Service). Pure-Go, CGO GEREKMEZ (cross-compile CGO_ENABLED=0 calisir).
// ---------------------------------------------------------------------------

type KeyringStore struct {
	Service string
}

func (k KeyringStore) service() string {
	if k.Service == "" {
		return "masha-agent"
	}
	return k.Service
}

func (k KeyringStore) Get(key string) (string, error) {
	v, err := keyring.Get(k.service(), key)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", ErrNotFound
	}
	return v, err
}

func (k KeyringStore) Set(key, val string) error {
	return keyring.Set(k.service(), key, val)
}

func (k KeyringStore) Delete(key string) error {
	err := keyring.Delete(k.service(), key)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil // yok sayilir — idempotent
	}
	return err
}

// ---------------------------------------------------------------------------
// FileStore — 0600 JSON dosya, anahtar-basina bir dosya. Geriye-donuk uyumluluk icin cagiran
// taraf (CredManager) legacy dosya yolunu Dir+anahtar yerine DOGRUDAN gecebilir (bkz. Resolve
// dokumantasyonu + CredManager.Load) — bugunku .masha-db.json / .masha-erpnext.json davranisi
// birebir korunur.
// ---------------------------------------------------------------------------

type FileStore struct {
	Dir string // bos ise mevcut dizin
}

func (f FileStore) path(key string) string {
	name := key + ".json"
	if f.Dir == "" {
		return name
	}
	return filepath.Join(f.Dir, name)
}

func (f FileStore) Get(key string) (string, error) {
	return readFile(f.path(key))
}

func (f FileStore) Set(key, val string) error {
	return writeFileAtomic(f.path(key), val)
}

func (f FileStore) Delete(key string) error {
	err := os.Remove(f.path(key))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// readFile — verilen YOLU (tam dosya yolu, key-turetilmis DEGIL) okur. FileStore.Get ve
// CredManager.Load'un legacy-yol okumasi ortak kullanir.
func readFile(path string) (string, error) {
	if path == "" {
		return "", ErrNotFound
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", ErrNotFound
		}
		return "", err
	}
	return string(b), nil
}

// writeFileAtomic — temp+rename, 0600.
func writeFileAtomic(path, val string) error {
	if path == "" {
		return fmt.Errorf("creds: bos dosya yolu — yazilamadi")
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(val), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ---------------------------------------------------------------------------
// probeKeyring — OS keychain'in GERCEKTEN calisip calismadigini bir sentinel anahtarla
// Set+Get+Delete deneyerek tespit eder (headless Linux'ta Secret Service D-Bus servisi
// olmayabilir → keyring.Set hata doner, crash ETMEZ, sadece false doner).
// ---------------------------------------------------------------------------

func probeKeyring(service string) bool {
	const sentinelKey = "__masha_probe__"
	const sentinelVal = "ok"
	ks := KeyringStore{Service: service}
	if err := ks.Set(sentinelKey, sentinelVal); err != nil {
		return false
	}
	v, err := ks.Get(sentinelKey)
	_ = ks.Delete(sentinelKey)
	return err == nil && v == sentinelVal
}

// Resolve — useKeyring istenmisse VE probeKeyring basariliysa KeyringStore, aksi halde
// fileDir'e bakan FileStore doner. label operator-log icin ("keychain" / "dosya (0600)").
func Resolve(useKeyring bool, service, fileDir string) (Store, string) {
	if useKeyring && probeKeyring(service) {
		return KeyringStore{Service: service}, "keychain"
	}
	return FileStore{Dir: fileDir}, "dosya (0600)"
}

// ---------------------------------------------------------------------------
// CredManager — mevcut main.go cagiranlarinin (loadCreds/saveCreds/loadErpFields/saveErpFields)
// yerini alir. Her kimlik icin bir "anahtar" (ör. "db", "erpnext") + o kimligin BUGUNKU legacy
// dosya yolu (.masha-db.json vb.) verilir; keychain KULLANILAMIYORSA ayni legacy dosyaya
// dogrudan okur/yazar (davranis BIREBIR korunur). Keychain KULLANILABILIYORSA once store'dan
// okur; yoksa VE legacy dosya varsa (eski kurulumdan kalma) OKUYUP KEYCHAIN'E TASIR + dosyayi
// siler (tek seferlik migrasyon).
// ---------------------------------------------------------------------------

type CredManager struct {
	Store Store
	Label string // "keychain" | "dosya (0600)" — log icin
	// legacyMode — Store bir FileStore ise VE legacy yola dogrudan yazmak istiyorsak true.
	// Resolve ile kurulan FileStore zaten Dir bos oldugunda key+".json" adini kullanir; bugunku
	// dosya adlariyla (.masha-db.json) BIREBIR ayni olmasi icin caller Load/Save'e legacy yolu
	// acikca verir ve biz o yolu FileStore key-tabanli yol yerine KULLANIRIZ.
}

// NewCredManager — Resolve sonucunu sarar.
func NewCredManager(store Store, label string) *CredManager {
	return &CredManager{Store: store, Label: label}
}

// Load — key altinda kayitli kimligi getirir. Keychain kullanilirken: once Store'dan bakar;
// yoksa legacy dosyayi okur (varsa), Store'a tasir (Set), dosyayi siler (best-effort), sonucu
// doner. Dosya-modunda (Store bir FileStore VE legacyPath verilmisse): DOGRUDAN legacyPath'i
// okur — bugunku .masha-db.json/.masha-erpnext.json davranisi degismez.
func (m *CredManager) Load(key, legacyPath string) (string, error) {
	if _, ok := m.Store.(FileStore); ok && legacyPath != "" {
		return readFile(legacyPath)
	}
	v, err := m.Store.Get(key)
	if err == nil {
		return v, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return "", err
	}
	// Store'da yok — legacy dosyaya bak (eski kurulumdan kalma olabilir), varsa migrasyon.
	if legacyPath == "" {
		return "", ErrNotFound
	}
	fv, ferr := readFile(legacyPath)
	if ferr != nil {
		return "", ferr // ErrNotFound veya baska bir dosya hatasi
	}
	if serr := m.Store.Set(key, fv); serr != nil {
		// Migrasyon basarisiz olsa da elimizdeki degeri donebiliriz; dosya SILINMEZ (veri kaybi olmasin).
		return fv, nil
	}
	_ = os.Remove(legacyPath) // best-effort — kimlik artik keychain'de
	return fv, nil
}

// Save — key altina val yazar. Dosya-modunda (Store FileStore VE legacyPath verilmis):
// dogrudan legacyPath'e yazar (0600, temp+rename) — bugunku davranisla birebir.
func (m *CredManager) Save(key, legacyPath, val string) error {
	if _, ok := m.Store.(FileStore); ok && legacyPath != "" {
		return writeFileAtomic(legacyPath, val)
	}
	return m.Store.Set(key, val)
}

// LoadJSON / SaveJSON — Load/Save'in JSON-marshal sarmalayicisi (DBFields/erpnext.Fields).
func (m *CredManager) LoadJSON(key, legacyPath string, out any) (bool, error) {
	v, err := m.Load(key, legacyPath)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return false, nil
		}
		return false, err
	}
	if err := json.Unmarshal([]byte(v), out); err != nil {
		return false, err
	}
	return true, nil
}

func (m *CredManager) SaveJSON(key, legacyPath string, in any) error {
	b, err := json.MarshalIndent(in, "", "  ")
	if err != nil {
		return err
	}
	return m.Save(key, legacyPath, string(b))
}
