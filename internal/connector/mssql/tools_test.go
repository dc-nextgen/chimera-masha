package mssql

import (
	"testing"

	"github.com/mehmetor/chimera-ai/stack/masha/agent/internal/live"
	"github.com/mehmetor/chimera-ai/stack/masha/agent/internal/manifest"
)

func toolsMan() *manifest.Manifest {
	return &manifest.Manifest{
		Name: "t", Label: "T", ERPKind: "mssql-generic",
		DB:       manifest.DBConfig{Driver: "sqlserver", ReadOnly: true},
		Entities: map[string]manifest.Entity{"e": {Table: "dbo.E", Fields: map[string]manifest.Field{"id": {Column: "Id"}}}},
		Tools: []manifest.Tool{
			{Name: "count_e", Kind: "count", Entity: "e"},
			{Name: "delete_e", Kind: "count", Entity: "e"}, // yazma-fiili
		},
	}
}

func TestAllowToolWriteGuard(t *testing.T) {
	c := &Connector{man: live.New(toolsMan())}
	if !c.AllowTool("count_e") {
		t.Error("count_e izinli olmali")
	}
	if c.AllowTool("delete_e") {
		t.Error("delete_e yazma-fiili → reddedilmeli")
	}
	if c.AllowTool("bilinmeyen") {
		t.Error("manifest-disi tool reddedilmeli")
	}
}

func TestAllowToolNilManifest(t *testing.T) {
	c := &Connector{man: live.New(nil)}
	if c.AllowTool("count_e") {
		t.Error("manifest yoksa hicbir tool izinli olmamali (fail-closed)")
	}
}

func TestOpenAPINilManifest(t *testing.T) {
	c := &Connector{man: live.New(nil)}
	spec := c.OpenAPI("masha-x")
	if spec["openapi"] != "3.1.0" {
		t.Error("manifest yoksa bile gecerli bos spec donmeli")
	}
}
