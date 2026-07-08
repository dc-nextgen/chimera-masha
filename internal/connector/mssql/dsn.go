package mssql

import (
	"net"
	"net/url"
	"strings"

	"github.com/dc-nextgen/chimera-masha/internal/connector"
)

// BuildDSN — ekran alanlarindan go-mssqldb DSN'i (URL form, net/url ile DOGRU encode → parola
// ozel-karakterleri [@ / : ; ?] guvenli; elle string birlestirme bug'i YOK). Kimlik YERELDE kalir (§3).
func BuildDSN(f connector.DBFields) string {
	host := strings.TrimSpace(f.Host)
	if p := strings.TrimSpace(f.Port); p != "" {
		host = net.JoinHostPort(host, p)
	}
	q := url.Values{}
	if db := strings.TrimSpace(f.Database); db != "" {
		q.Set("database", db)
	}
	enc := strings.TrimSpace(f.Encrypt)
	if enc == "" {
		enc = "true"
	}
	q.Set("encrypt", enc)
	if f.TrustServerCert {
		q.Set("trustservercertificate", "true")
	}
	u := url.URL{
		Scheme:   "sqlserver",
		User:     url.UserPassword(f.User, f.Password),
		Host:     host,
		RawQuery: q.Encode(),
	}
	return u.String()
}
