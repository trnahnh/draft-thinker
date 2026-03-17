package client

import "time"

func NewOpenAIClient(baseURL, apiKey, model string, timeout time.Duration) *OpenAICompatibleClient {
	return NewOpenAICompatibleClient(baseURL, apiKey, model, "openai", timeout)
}
