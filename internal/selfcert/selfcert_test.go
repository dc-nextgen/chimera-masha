package selfcert

import (
	"crypto/x509"
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadOrCreateGeneratesAndReuses(t *testing.T) {
	dir := t.TempDir()
	hosts := []string{"127.0.0.1", "localhost", "192.168.1.50"}

	c1, err := LoadOrCreate(dir, hosts)
	if err != nil {
		t.Fatalf("ilk uretim: %v", err)
	}
	// dosyalar yazildi mi
	for _, f := range []string{"web-cert.pem", "web-key.pem"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Errorf("%s yazilmadi: %v", f, err)
		}
	}
	// SAN'lar: IP + DNS dogru ayristi mi
	leaf, err := x509.ParseCertificate(c1.Certificate[0])
	if err != nil {
		t.Fatalf("cert parse: %v", err)
	}
	if err := leaf.VerifyHostname("192.168.1.50"); err != nil {
		t.Errorf("LAN IP SAN eksik: %v", err)
	}
	if err := leaf.VerifyHostname("localhost"); err != nil {
		t.Errorf("localhost DNS SAN eksik: %v", err)
	}
	foundIP := false
	for _, ip := range leaf.IPAddresses {
		if ip.Equal(net.ParseIP("127.0.0.1")) {
			foundIP = true
		}
	}
	if !foundIP {
		t.Error("127.0.0.1 IP SAN eksik")
	}

	// ikinci cagri = AYNI cert'i yuklemeli (yeniden-uretmemeli)
	c2, err := LoadOrCreate(dir, hosts)
	if err != nil {
		t.Fatalf("ikinci yukleme: %v", err)
	}
	if string(c1.Certificate[0]) != string(c2.Certificate[0]) {
		t.Error("yeniden-baslatmada cert degisti (yeniden-uretildi) — tarayici her sefer sorar")
	}
}

func TestLocalHostsIncludesLoopback(t *testing.T) {
	h := LocalHosts()
	var hasLo bool
	for _, x := range h {
		if x == "127.0.0.1" {
			hasLo = true
		}
	}
	if !hasLo {
		t.Error("LocalHosts 127.0.0.1 icermeli")
	}
}
