package mssql

import (
	"strings"

	"github.com/mehmetor/chimera-ai/stack/masha/agent/internal/manifest"
)

// writeVerbs — belt-and-suspenders: tool adinda bu fiiller varsa ASLA gecmez (savunma tabani;
// manifest zaten yalniz count/query uretir, ama isim-bazli guard ek katman).
var writeVerbs = []string{"create", "update", "delete", "submit", "cancel", "insert",
	"drop", "alter", "exec", "call_method", "write"}

// OpenAPI — canli manifest'ten tool yuzeyi. Manifest yoksa bos-path spec (OWUI bos tool listesi gorur).
func (c *Connector) OpenAPI(serverLabel string) map[string]any {
	m := c.man.Get()
	if m == nil {
		return map[string]any{
			"openapi": "3.1.0",
			"info":    map[string]any{"title": serverLabel, "version": manifest.AgentVersion},
			"servers": []any{map[string]any{"url": "/" + serverLabel}},
			"paths":   map[string]any{},
		}
	}
	return m.OpenAPI(serverLabel)
}

// AllowTool — tool manifest'te var + yazma-fiili degil (fail-closed).
func (c *Connector) AllowTool(tool string) bool {
	m := c.man.Get()
	if m == nil {
		return false
	}
	if _, ok := m.Tool(tool); !ok {
		return false
	}
	lower := strings.ToLower(tool)
	for _, v := range writeVerbs {
		if strings.Contains(lower, v) {
			return false
		}
	}
	return true
}
