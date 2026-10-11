package config

type Config struct {
	Server      ServerConfig      `yaml:"server"`
	Drafter     DrafterConfig     `yaml:"drafter"`
	Heavyweight HeavyweightConfig `yaml:"heavyweight"`
	Entropy     EntropyConfig     `yaml:"entropy"`
	Speculative SpeculativeConfig `yaml:"speculative"`
	Cache       CacheConfig       `yaml:"cache"`
	Fallback    FallbackConfig    `yaml:"fallback"`
	Metrics     MetricsConfig     `yaml:"metrics"`
}

// FallbackConfig controls graceful degradation when a provider call fails.
// Both options default to enabled when omitted.
type FallbackConfig struct {
	// OnDrafterError sends the request straight to the heavyweight when the
	// drafter call errors or times out before a routing decision is made.
	OnDrafterError *bool `yaml:"on_drafter_error"`
	// ServeDraftOnHeavyError re-runs the drafter to completion and serves
	// its answer (uncached, flagged via X-Routing-Decision: fallback_draft)
	// when the heavyweight call fails after an escalation decision.
	ServeDraftOnHeavyError *bool `yaml:"serve_draft_on_heavy_error"`
}

func (f FallbackConfig) OnDrafterErrorEnabled() bool {
	if f.OnDrafterError == nil {
		return true
	}
	return *f.OnDrafterError
}

func (f FallbackConfig) ServeDraftOnHeavyErrorEnabled() bool {
	if f.ServeDraftOnHeavyError == nil {
		return true
	}
	return *f.ServeDraftOnHeavyError
}

type CacheConfig struct {
	Enabled             *bool   `yaml:"enabled"`
	SimilarityThreshold float64 `yaml:"similarity_threshold"`
	TTLSeconds          int     `yaml:"ttl_seconds"`
	EmbeddingModel      string  `yaml:"embedding_model"`
	EmbeddingDimensions int     `yaml:"embedding_dimensions"`
	QdrantCollection    string  `yaml:"qdrant_collection"`
}

func (c CacheConfig) IsEnabled() bool {
	if c.Enabled == nil {
		return false
	}
	return *c.Enabled
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
