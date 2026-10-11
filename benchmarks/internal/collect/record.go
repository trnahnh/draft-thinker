package collect

type TokenRecord struct {
	Token   string  `json:"token"`
	Entropy float64 `json:"entropy"`
	Logprob float64 `json:"logprob"`
	Index   int     `json:"index"`
	// OutsideTopK is true when the sampled token did not appear among the
	// requested top-k alternatives. OpenAI encodes this case by reporting
	// logprob -9999 for the sampled token; open-model servers report the
	// real logprob, so this flag is the provider-neutral form of the signal.
	OutsideTopK bool `json:"outside_top_k,omitempty"`
}

type JudgeResult struct {
	Acceptable bool   `json:"acceptable"`
	Score      int    `json:"score"`
	Reasoning  string `json:"reasoning"`
}

type Record struct {
	PromptID        string        `json:"prompt_id"`
	Category        string        `json:"category"`
	PromptText      string        `json:"prompt_text"`
	DraftResponse   string        `json:"draft_response"`
	HeavyResponse   string        `json:"heavy_response"`
	Tokens          []TokenRecord `json:"tokens"`
	TokenCount      int           `json:"token_count"`
	MeanEntropy     float64       `json:"mean_entropy"`
	MaxEntropy      float64       `json:"max_entropy"`
	Judge           JudgeResult   `json:"judge"`
	DraftModel      string        `json:"draft_model"`
	HeavyModel      string        `json:"heavy_model"`
	DraftLatencyMS  int64         `json:"draft_latency_ms"`
	HeavyLatencyMS  int64         `json:"heavy_latency_ms"`
	DraftTokensUsed int           `json:"draft_tokens_used"`
	HeavyTokensUsed int           `json:"heavy_tokens_used"`
	// PromptTokens is the provider-reported prompt token count. Zero on
	// records collected before this field existed; the cost model then
	// falls back to a character-based estimate.
	PromptTokens int `json:"prompt_tokens,omitempty"`
	// Grading is "judge" (LLM-as-judge against the heavyweight response) or
	// an exact-match mode ("numeric", "choice") against GroundTruth.
	Grading     string `json:"grading,omitempty"`
	GroundTruth string `json:"ground_truth,omitempty"`
	// DraftCorrect and HeavyCorrect are set only for exact-match grading,
	// where both models are scored against the same ground truth.
	DraftCorrect *bool  `json:"draft_correct,omitempty"`
	HeavyCorrect *bool  `json:"heavy_correct,omitempty"`
	DraftAnswer  string `json:"draft_answer,omitempty"`
	HeavyAnswer  string `json:"heavy_answer,omitempty"`
	Error        string `json:"error,omitempty"`
	CollectedAt  string `json:"collected_at"`
}

// SampledOutsideTopK reports whether this token hit the logprob artifact:
// either flagged at collection time or carrying OpenAI's -9999 sentinel
// (older traces predate the flag).
func (t TokenRecord) SampledOutsideTopK() bool {
	return t.OutsideTopK || t.Logprob <= -9999
}

// HasGroundTruth reports whether the record was graded against a known
// answer rather than by an LLM judge.
func (r *Record) HasGroundTruth() bool {
	return r.DraftCorrect != nil && r.HeavyCorrect != nil
}

// EstimatedPromptTokens returns the reported prompt token count, or a
// four-characters-per-token estimate for records collected before prompt
// usage was persisted.
func (r *Record) EstimatedPromptTokens() int {
	if r.PromptTokens > 0 {
		return r.PromptTokens
	}
	return max((len(r.PromptText)+3)/4, 1)
}
