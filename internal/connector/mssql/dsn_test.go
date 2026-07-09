package mssql

import (
	"net/url"
	"strings"
	"testing"

	"github.com/mehmetor/chimera-ai/stack/masha/agent/internal/connector"
)

func TestBuildDSNEncodesSpecialPassword(t *testing.T) {
	// Ozel-karakterli parola (@ : / ; ?) elle-birlestirmeyi kirar; net/url dogru encode etmeli.
	f := connector.DBFields{
		Host: "db.example.com", Port: "1433", Database: "SalesDB",
		User: "svc", Password: "p@ss:w/o;rd?", Encrypt: "true", TrustServerCert: true,
	}
	dsn := BuildDSN(f)
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("uretilen DSN parse edilemedi: %v (%q)", err, dsn)
	}
	if u.Scheme != "sqlserver" {
		t.Errorf("scheme sqlserver bekleniyordu, %q", u.Scheme)
	}
	if u.Hostname() != "db.example.com" || u.Port() != "1433" {
		t.Errorf("host/port yanlis: %q:%q", u.Hostname(), u.Port())
	}
	pw, _ := u.User.Password()
	if pw != "p@ss:w/o;rd?" {
		t.Errorf("parola round-trip bozuldu: %q", pw)
	}
	q := u.Query()
	if q.Get("database") != "SalesDB" || q.Get("encrypt") != "true" || q.Get("trustservercertificate") != "true" {
		t.Errorf("query paramlari yanlis: %v", q)
	}
}

func TestBuildDSNNoPortNoTrust(t *testing.T) {
	f := connector.DBFields{Host: "h", Database: "d", User: "u", Password: "p"}
	dsn := BuildDSN(f)
	if strings.Contains(dsn, "trustservercertificate") {
		t.Error("trust set edilmediyse eklenmemeli")
	}
	if !strings.Contains(dsn, "encrypt=true") {
		t.Error("encrypt default true olmali")
	}
	u, _ := url.Parse(dsn)
	if u.Port() != "" {
		t.Errorf("port verilmediyse bos olmali, %q", u.Port())
	}
}
