// Package selfcert — yerel yuz icin self-signed TLS cert'i URET/YUKLE (§17.9). LAN'da token'i
// sifreler (duz-HTTP dinleme kapanir). Cert kutuda kalir, yeniden-kullanilir; tarayici "guvenilmez"
// uyarisi verir (self-signed) — beklenen, operator tek-seferlik gecer. Buluta giden yol AYRI + mTLS (§12).
package selfcert

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"
)

// LoadOrCreate — dir'de cert/key varsa yukler; yoksa hosts (IP+DNS SAN) icin uretir + yazar (0600).
// Deterministik dosya adlari → yeniden-baslatmada AYNI cert (tarayici bir kez guvenir, tekrar sormaz).
func LoadOrCreate(dir string, hosts []string) (tls.Certificate, error) {
	certPath := filepath.Join(dir, "web-cert.pem")
	keyPath := filepath.Join(dir, "web-key.pem")
	if fileExists(certPath) && fileExists(keyPath) {
		return tls.LoadX509KeyPair(certPath, keyPath)
	}
	certPEM, keyPEM, err := generate(hosts)
	if err != nil {
		return tls.Certificate{}, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return tls.Certificate{}, err
	}
	if err := os.WriteFile(certPath, certPEM, 0o600); err != nil {
		return tls.Certificate{}, err
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		return tls.Certificate{}, err
	}
	return tls.X509KeyPair(certPEM, keyPEM)
}

// generate — ECDSA P-256 self-signed cert (10 yil). hosts: IP olanlar IPAddresses, digerleri DNSNames.
func generate(hosts []string) (certPEM, keyPEM []byte, err error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, err
	}
	tmpl := x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "masha-agent yerel yuz"},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
		} else if h != "" {
			tmpl.DNSNames = append(tmpl.DNSNames, h)
		}
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &priv.PublicKey, priv)
	if err != nil {
		return nil, nil, err
	}
	keyDER, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return nil, nil, err
	}
	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	return certPEM, keyPEM, nil
}

// LocalHosts — cert SAN'lari: loopback + host'un tum loopback-disi IP'leri (LAN erisimi calissin).
func LocalHosts() []string {
	hosts := []string{"127.0.0.1", "::1", "localhost"}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return hosts
	}
	seen := map[string]bool{"127.0.0.1": true, "::1": true}
	for _, a := range addrs {
		var ip net.IP
		if n, ok := a.(*net.IPNet); ok {
			ip = n.IP
		}
		// link-local (fe80::) + coklu-yayin = SAN gurultu; global/LAN IP'leri tut.
		if ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || seen[ip.String()] {
			continue
		}
		seen[ip.String()] = true
		hosts = append(hosts, ip.String())
	}
	return hosts
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// FormatHosts — log icin kisa ozet.
func FormatHosts(h []string) string {
	if len(h) == 0 {
		return "(yok)"
	}
	return fmt.Sprintf("%v", h)
}
