package collect

type TokenRecord struct {
	Token   string  `json:"token"`
	Entropy float64 `json:"entropy"`
	Logprob float64 `json:"logprob"`
	Index   int     `json:"index"`
}

type JudgeResult struct {
	Acceptable bool   `json:"acceptable"`
	Score      int    `json:"score"`
	Reasoning  string `json:"reasoning"`
}

type Record struct {
	PromptID       string        `json:"prompt_id"`
	Category       string        `json:"category"`
	PromptText     string        `json:"prompt_text"`
	DraftResponse  string        `json:"draft_response"`
	HeavyResponse  string        `json:"heavy_response"`
	Tokens         []TokenRecord `json:"tokens"`
	TokenCount     int           `json:"token_count"`
	MeanEntropy    float64       `json:"mean_entropy"`
	MaxEntropy     float64       `json:"max_entropy"`
	Judge          JudgeResult   `json:"judge"`
	DraftModel     string        `json:"draft_model"`
	HeavyModel     string        `json:"heavy_model"`
	DraftLatencyMS int64         `json:"draft_latency_ms"`
	HeavyLatencyMS int64         `json:"heavy_latency_ms"`
	DraftTokensUsed  int         `json:"draft_tokens_used"`
	HeavyTokensUsed  int         `json:"heavy_tokens_used"`
	Error          string        `json:"error,omitempty"`
	CollectedAt    string        `json:"collected_at"`
}
