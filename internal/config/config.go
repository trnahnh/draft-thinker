package config

type Config struct {
	Server      ServerConfig      `yaml:"server"`
	Drafter     DrafterConfig     `yaml:"drafter"`
	Heavyweight HeavyweightConfig `yaml:"heavyweight"`
	Entropy     EntropyConfig     `yaml:"entropy"`
	Speculative SpeculativeConfig `yaml:"speculative"`
	Metrics     MetricsConfig     `yaml:"metrics"`
}

type SpeculativeConfig struct {
	Enabled           *bool   `yaml:"enabled"`
	SoftThresholdMult float64 `yaml:"soft_threshold_mult"`
}

func (s SpeculativeConfig) IsEnabled() bool {
	if s.Enabled == nil {
		return true
	}
	return *s.Enabled
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

type HeavyweightConfig struct {
	Provider string `yaml:"provider"`
	BaseURL  string `yaml:"base_url"`
	Model    string `yaml:"model"`
	Timeout  int    `yaml:"timeout"`
}

type EntropyConfig struct {
	Threshold      float64 `yaml:"threshold"`
	WindowSize     int     `yaml:"window_size"`
	EarlyExitCount int     `yaml:"early_exit_count"`
	TopLogprobs    int     `yaml:"top_logprobs"`
}

type MetricsConfig struct {
	Enabled bool   `yaml:"enabled"`
	Path    string `yaml:"path"`
}
