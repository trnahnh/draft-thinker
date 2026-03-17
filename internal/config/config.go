package config

type Config struct {
	Server  ServerConfig  `yaml:"server"`
	Drafter DrafterConfig `yaml:"drafter"`
	Metrics MetricsConfig `yaml:"metrics"`
}

type ServerConfig struct {
	Port         int `yaml:"port"`
	ReadTimeout  int `yaml:"read_timeout"`
	WriteTimeout int `yaml:"write_timeout"`
	IdleTimeout  int `yaml:"idle_timeout"`
}

type DrafterConfig struct {
	Provider string `yaml:"provider"`
	BaseURL  string `yaml:"base_url"`
	Model    string `yaml:"model"`
	Timeout  int    `yaml:"timeout"`
}

type MetricsConfig struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"`
}
