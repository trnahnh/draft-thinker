package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTestConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing test config: %v", err)
	}
	return path
}

func TestLoad_ValidConfig(t *testing.T) {
	path := writeTestConfig(t, `
server:
  port: 9090
  write_timeout: 60
drafter:
  provider: groq
  base_url: https://api.groq.com/openai/v1
  model: llama3-70b-8192
metrics:
  enabled: true
  path: /metrics
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Server.Port != 9090 {
		t.Errorf("port: got %d, want 9090", cfg.Server.Port)
	}
	if cfg.Server.WriteTimeout != 60 {
		t.Errorf("write_timeout: got %d, want 60", cfg.Server.WriteTimeout)
	}
	if cfg.Drafter.Model != "llama3-70b-8192" {
		t.Errorf("model: got %q, want %q", cfg.Drafter.Model, "llama3-70b-8192")
	}
	if !cfg.Metrics.Enabled {
		t.Error("metrics.enabled should be true")
	}
}

func TestLoad_Defaults(t *testing.T) {
	path := writeTestConfig(t, `{}`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Server.Port != 8080 {
		t.Errorf("default port: got %d, want 8080", cfg.Server.Port)
	}
	if cfg.Server.ReadTimeout != 30 {
		t.Errorf("default read_timeout: got %d, want 30", cfg.Server.ReadTimeout)
	}
	if cfg.Server.WriteTimeout != 120 {
		t.Errorf("default write_timeout: got %d, want 120", cfg.Server.WriteTimeout)
	}
	if cfg.Drafter.Provider != "groq" {
		t.Errorf("default provider: got %q, want %q", cfg.Drafter.Provider, "groq")
	}
	if cfg.Drafter.BaseURL != "https://api.groq.com/openai/v1" {
		t.Errorf("default base_url: got %q", cfg.Drafter.BaseURL)
	}
	if cfg.Drafter.Model != "llama3-8b-8192" {
		t.Errorf("default model: got %q, want %q", cfg.Drafter.Model, "llama3-8b-8192")
	}
	if cfg.Metrics.Path != "/metrics" {
		t.Errorf("default metrics path: got %q, want %q", cfg.Metrics.Path, "/metrics")
	}
}

func TestLoad_InvalidPort(t *testing.T) {
	path := writeTestConfig(t, `
server:
  port: 99999
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid port")
	}
}

func TestLoad_MissingFile(t *testing.T) {
	_, err := Load("/nonexistent/config.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	path := writeTestConfig(t, `{{{invalid`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}
