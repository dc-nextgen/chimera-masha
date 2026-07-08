// Package erpnext — Go-native ErpNext REST connector (§19.1 gömülü sınıf, ama Node/mcpo YOK →
// tek-binary korunur). erpnext-mcp-server'ın (rakeshgangwar fork) SALT-OKUMA araçlarını Go'da
// yeniden yazar: ince REST GET sarmalayıcısı. On-prem ErpNext'e yerelden erer (bulut ERP'de VPS
// tercih; kod aynı). Kimlik (api_key:api_secret) YERELDE (§3). Yazma araçları YOK (salt-okuma).
package erpnext

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
)

type cfg struct {
	baseURL string // "https://erp... " (sonda / yok)
	apiKey  string
	apiSec  string
}

// Connector — ErpNext REST connector. conf ATOMIK → ekrandan-yeniden-yapılandır (Connect) swap eder.
// mask=true → doküman PII alanları (email/telefon/tckn...) tünelden ÖNCE maskelenir (heuristik; §3).
type Connector struct {
	conf atomic.Pointer[cfg]
	hc   *http.Client
	mask bool
}

// Fields — ekrandan girilen ErpNext bağlantısı (kimlik YERELDE, §3).
type Fields struct {
	URL       string `json:"url"`
	APIKey    string `json:"api_key"`
	APISecret string `json:"api_secret"`
}

// BuildDSN — Fields → opak DSN (JSON). connector.Connect(dsn) bunu bekler (mssql.BuildDSN muadili).
func BuildDSN(f Fields) string {
	b, _ := json.Marshal(f)
	return string(b)
}

// Open — baseURL boş ise BAĞLANTISIZ başlar (ekrandan Connect bekler). mask=doküman PII maskele.
func Open(baseURL, apiKey, apiSecret string, mask bool) *Connector {
	c := &Connector{hc: &http.Client{Timeout: 30 * time.Second}, mask: mask}
	if strings.TrimSpace(baseURL) != "" {
		c.conf.Store(&cfg{strings.TrimRight(baseURL, "/"), apiKey, apiSecret})
	}
	return c
}

func (c *Connector) Connected() bool { return c.conf.Load() != nil }
func (c *Connector) Close() error    { return nil }

// Connect — dsn (JSON Fields) ile yeniden-yapılandır + doğrula (giriş yapan kullanıcı sorgusu). Swap atomik.
func (c *Connector) Connect(ctx context.Context, dsn string) error {
	var f Fields
	if err := json.Unmarshal([]byte(dsn), &f); err != nil {
		return fmt.Errorf("erpnext config JSON gecersiz: %w", err)
	}
	if strings.TrimSpace(f.URL) == "" {
		return fmt.Errorf("erpnext URL bos")
	}
	nc := &cfg{strings.TrimRight(strings.TrimSpace(f.URL), "/"), f.APIKey, f.APISecret}
	vctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	if _, err := c.getWith(vctx, nc, "/api/method/frappe.auth.get_logged_user", nil); err != nil {
		return fmt.Errorf("erpnext dogrulama basarisiz: %w", err)
	}
	c.conf.Store(nc)
	return nil
}

// Health — giriş yapan kullanıcıyı sorgula (auth + erişilebilirlik).
func (c *Connector) Health(ctx context.Context) error {
	cf := c.conf.Load()
	if cf == nil {
		return fmt.Errorf("ErpNext bagli degil (ekrandan baglan)")
	}
	_, err := c.getWith(ctx, cf, "/api/method/frappe.auth.get_logged_user", nil)
	return err
}

// getWith — verilen cfg ile kimlik-doğrulamalı GET; JSON gövdeyi döndürür.
func (c *Connector) getWith(ctx context.Context, cf *cfg, path string, params url.Values) (map[string]any, error) {
	u := cf.baseURL + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	if cf.apiKey != "" {
		req.Header.Set("Authorization", "token "+cf.apiKey+":"+cf.apiSec)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("ErpNext HTTP %d: %s", resp.StatusCode, truncate(string(raw), 160))
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("ErpNext yanit JSON degil: %s", truncate(string(raw), 120))
	}
	return out, nil
}

func (c *Connector) get(ctx context.Context, path string, params url.Values) (map[string]any, error) {
	cf := c.conf.Load()
	if cf == nil {
		return nil, fmt.Errorf("ErpNext bagli degil")
	}
	return c.getWith(ctx, cf, path, params)
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
