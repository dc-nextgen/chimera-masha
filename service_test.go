package main

import "testing"

func TestServiceUserModeEnv(t *testing.T) {
	cases := map[string]bool{
		"":      false,
		"0":     false,
		"false": false,
		"1":     true,
		"true":  true,
		"YES":   true,
		"on":    true,
	}
	for v, want := range cases {
		t.Setenv("MASHA_SERVICE_USER", v)
		if got := serviceUserMode(); got != want {
			t.Errorf("MASHA_SERVICE_USER=%q: got %v, want %v", v, got, want)
		}
	}
}

func TestBuildServiceConfig(t *testing.T) {
	env := map[string]string{"MASHA_APP_TOKEN": "tok", "MASHA_MANIFEST": "/etc/masha/manifest.json"}
	cfg := buildServiceConfig(env, true, "/opt/masha")

	if cfg.Name != "masha-agent" {
		t.Errorf("Name = %q", cfg.Name)
	}
	if cfg.WorkingDirectory != "/opt/masha" {
		t.Errorf("WorkingDirectory = %q, want /opt/masha", cfg.WorkingDirectory)
	}
	if len(cfg.Arguments) != 1 || cfg.Arguments[0] != "serve" {
		t.Errorf("Arguments = %v, want [serve]", cfg.Arguments)
	}
	if cfg.EnvVars["MASHA_APP_TOKEN"] != "tok" {
		t.Errorf("EnvVars not passed through: %v", cfg.EnvVars)
	}
	if v, _ := cfg.Option["UserService"].(bool); !v {
		t.Errorf("UserService option should be true")
	}
	if v, _ := cfg.Option["Restart"].(string); v != "always" {
		t.Errorf("Restart option = %v, want always", v)
	}
}

func TestCaptureMashaEnvFiltersPrefix(t *testing.T) {
	t.Setenv("MASHA_APP_TOKEN", "secret")
	t.Setenv("ERPNEXT_URL", "https://erp.example")
	t.Setenv("UNRELATED_VAR", "x")
	env := captureMashaEnv()
	if env["MASHA_APP_TOKEN"] != "secret" {
		t.Errorf("expected MASHA_APP_TOKEN captured, got %v", env)
	}
	if env["ERPNEXT_URL"] != "https://erp.example" {
		t.Errorf("expected ERPNEXT_URL (envOr fallback) captured, got %v", env)
	}
	if _, ok := env["UNRELATED_VAR"]; ok {
		t.Errorf("UNRELATED_VAR should not be captured")
	}
}
