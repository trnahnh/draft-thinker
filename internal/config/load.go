package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

func applyDefaults(cfg *Config) {
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.Server.ReadTimeout == 0 {
		cfg.Server.ReadTimeout = 30
	}
	if cfg.Server.WriteTimeout == 0 {
		cfg.Server.WriteTimeout = 120
	}
	if cfg.Server.IdleTimeout == 0 {
		cfg.Server.IdleTimeout = 60
	}
	if cfg.Drafter.Provider == "" {
		cfg.Drafter.Provider = "groq"
	}
	if cfg.Drafter.BaseURL == "" {
		cfg.Drafter.BaseURL = "https://api.groq.com/openai/v1"
	}
	if cfg.Drafter.Model == "" {
		cfg.Drafter.Model = "llama3-8b-8192"
	}
	if cfg.Drafter.Timeout == 0 {
		cfg.Drafter.Timeout = 30
	}
	if cfg.Metrics.Path == "" {
		cfg.Metrics.Path = "/metrics"
	}
}

func validate(cfg *Config) error {
	if cfg.Server.Port < 1 || cfg.Server.Port > 65535 {
		return fmt.Errorf("server.port must be between 1 and 65535, got %d", cfg.Server.Port)
	}
	if cfg.Drafter.BaseURL == "" {
		return fmt.Errorf("drafter.base_url is required")
	}
	if cfg.Drafter.Model == "" {
		return fmt.Errorf("drafter.model is required")
	}
	return nil
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	applyDefaults(&cfg)

	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}

	return &cfg, nil
}
