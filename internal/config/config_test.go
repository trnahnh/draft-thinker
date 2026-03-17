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
	if cfg.Drafter.Provider != "openai" {
		t.Errorf("default provider: got %q, want %q", cfg.Drafter.Provider, "openai")
	}
	if cfg.Drafter.BaseURL != "https://api.openai.com/v1" {
		t.Errorf("default base_url: got %q", cfg.Drafter.BaseURL)
	}
	if cfg.Drafter.Model != "gpt-4.1-nano" {
		t.Errorf("default model: got %q, want %q", cfg.Drafter.Model, "gpt-4.1-nano")
	}
	if cfg.Metrics.Path != "/metrics" {
		t.Errorf("default metrics path: got %q, want %q", cfg.Metrics.Path, "/metrics")
	}
	if cfg.Heavyweight.Provider != "openai" {
		t.Errorf("default heavyweight provider: got %q, want %q", cfg.Heavyweight.Provider, "openai")
	}
	if cfg.Heavyweight.BaseURL != "https://api.openai.com/v1" {
		t.Errorf("default heavyweight base_url: got %q", cfg.Heavyweight.BaseURL)
	}
	if cfg.Heavyweight.Model != "gpt-4.1" {
		t.Errorf("default heavyweight model: got %q, want %q", cfg.Heavyweight.Model, "gpt-4.1")
	}
	if cfg.Heavyweight.Timeout != 60 {
		t.Errorf("default heavyweight timeout: got %d, want 60", cfg.Heavyweight.Timeout)
	}
	if cfg.Entropy.Threshold != 2.0 {
		t.Errorf("default entropy threshold: got %f, want 2.0", cfg.Entropy.Threshold)
	}
	if cfg.Entropy.WindowSize != 10 {
		t.Errorf("default entropy window_size: got %d, want 10", cfg.Entropy.WindowSize)
	}
	if cfg.Entropy.EarlyExitCount != 10 {
		t.Errorf("default entropy early_exit_count: got %d, want 10", cfg.Entropy.EarlyExitCount)
	}
	if cfg.Entropy.TopLogprobs != 5 {
		t.Errorf("default entropy top_logprobs: got %d, want 5", cfg.Entropy.TopLogprobs)
	}
	if !cfg.Speculative.IsEnabled() {
		t.Error("default speculative should be enabled when not set")
	}
	if cfg.Speculative.Enabled != nil {
		t.Error("default speculative.enabled should be nil (unset)")
	}
	if cfg.Speculative.SoftThresholdMult != 0.8 {
		t.Errorf("default speculative.soft_threshold_mult: got %f, want 0.8", cfg.Speculative.SoftThresholdMult)
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

func TestLoad_InvalidEntropyThreshold(t *testing.T) {
	path := writeTestConfig(t, `
entropy:
  threshold: -1.0
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for negative entropy threshold")
	}
}

func TestLoad_InvalidTopLogprobs(t *testing.T) {
	path := writeTestConfig(t, `
entropy:
  top_logprobs: 25
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for top_logprobs > 20")
	}
}

func TestLoad_SpeculativeDisabled(t *testing.T) {
	path := writeTestConfig(t, `
speculative:
  enabled: false
  soft_threshold_mult: 0.5
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Speculative.IsEnabled() {
		t.Error("speculative should be disabled when enabled: false")
	}
	if cfg.Speculative.SoftThresholdMult != 0.5 {
		t.Errorf("soft_threshold_mult: got %f, want 0.5", cfg.Speculative.SoftThresholdMult)
	}
}

func TestLoad_SpeculativeDisabledWithoutMult(t *testing.T) {
	path := writeTestConfig(t, `
speculative:
  enabled: false
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Speculative.IsEnabled() {
		t.Error("speculative should be disabled when explicitly set to false")
	}
}

func TestLoad_InvalidSoftThresholdMult_TooHigh(t *testing.T) {
	path := writeTestConfig(t, `
speculative:
  soft_threshold_mult: 1.0
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for soft_threshold_mult = 1.0")
	}
}

func TestLoad_InvalidSoftThresholdMult_Negative(t *testing.T) {
	path := writeTestConfig(t, `
speculative:
  soft_threshold_mult: -0.5
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for negative soft_threshold_mult")
	}
}

func TestLoad_InvalidWindowSize(t *testing.T) {
	path := writeTestConfig(t, `
entropy:
  window_size: -1
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for window_size < 1")
	}
}
