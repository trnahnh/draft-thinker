package client

import "time"

func NewGroqClient(baseURL, apiKey, model string, timeout time.Duration) *OpenAICompatibleClient {
	return NewOpenAICompatibleClient(baseURL, apiKey, model, "groq", timeout)
}
