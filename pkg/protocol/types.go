package protocol

type ChatCompletionRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Stream      bool      `json:"stream,omitempty"`
	Temperature *float64  `json:"temperature,omitempty"`
	MaxTokens   *int      `json:"max_tokens,omitempty"`
	TopP        *float64  `json:"top_p,omitempty"`
	Stop        []string  `json:"stop,omitempty"`
	User        string    `json:"user,omitempty"`
	Logprobs    *bool     `json:"logprobs,omitempty"`
	TopLogprobs *int      `json:"top_logprobs,omitempty"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   *Usage   `json:"usage,omitempty"`
}

type Choice struct {
	Index        int             `json:"index"`
	Message      *Message        `json:"message,omitempty"`
	Delta        *Delta          `json:"delta,omitempty"`
	FinishReason *string         `json:"finish_reason,omitempty"`
	Logprobs     *ChoiceLogprobs `json:"logprobs,omitempty"`
}

type ChoiceLogprobs struct {
	Content []TokenLogprob `json:"content"`
}

type TokenLogprob struct {
	Token       string            `json:"token"`
	Logprob     float64           `json:"logprob"`
	TopLogprobs []TopLogprobEntry `json:"top_logprobs"`
}

type TopLogprobEntry struct {
	Token   string  `json:"token"`
	Logprob float64 `json:"logprob"`
}

type Delta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type StreamChunk struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
}

type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
	Message string  `json:"message"`
	Type    string  `json:"type"`
	Code    *string `json:"code,omitempty"`
}
