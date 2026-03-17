package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/trnahnh/draft-thinker/pkg/protocol"
)

type OpenAICompatibleClient struct {
	baseURL    string
	apiKey     string
	model      string
	provider   string
	httpClient *http.Client
}

func NewOpenAICompatibleClient(baseURL, apiKey, model, provider string, timeout time.Duration) *OpenAICompatibleClient {
	return &OpenAICompatibleClient{
		baseURL:  strings.TrimRight(baseURL, "/"),
		apiKey:   apiKey,
		model:    model,
		provider: provider,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *OpenAICompatibleClient) buildRequest(ctx context.Context, req *protocol.ChatCompletionRequest) (*http.Request, error) {
	if req.Model == "auto" || req.Model == "" {
		req.Model = c.model
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	return httpReq, nil
}

func (c *OpenAICompatibleClient) Complete(ctx context.Context, req *protocol.ChatCompletionRequest) (*protocol.ChatCompletionResponse, error) {
	req.Stream = false

	httpReq, err := c.buildRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return nil, &UpstreamTimeoutError{Provider: c.provider}
		}
		return nil, fmt.Errorf("%s request failed: %w", c.provider, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, &UpstreamError{
			StatusCode: resp.StatusCode,
			Body:       string(respBody),
			Provider:   c.provider,
		}
	}

	var result protocol.ChatCompletionResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}

	return &result, nil
}

func (c *OpenAICompatibleClient) Stream(ctx context.Context, req *protocol.ChatCompletionRequest) (<-chan StreamChunk, error) {
	req.Stream = true

	httpReq, err := c.buildRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return nil, &UpstreamTimeoutError{Provider: c.provider}
		}
		return nil, fmt.Errorf("%s stream request failed: %w", c.provider, err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, &UpstreamError{
			StatusCode: resp.StatusCode,
			Body:       string(body),
			Provider:   c.provider,
		}
	}

	ch := make(chan StreamChunk, 16)

	go func() {
		defer close(ch)
		defer resp.Body.Close()

		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()

			if line == "" || line == "\n" {
				continue
			}

			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")

			if data == "[DONE]" {
				return
			}

			var chunk protocol.StreamChunk
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				ch <- StreamChunk{Err: fmt.Errorf("parsing SSE chunk: %w", err)}
				return
			}

			select {
			case ch <- StreamChunk{Chunk: &chunk}:
			case <-ctx.Done():
				return
			}
		}

		if err := scanner.Err(); err != nil {
			ch <- StreamChunk{Err: fmt.Errorf("reading SSE stream: %w", err)}
		}
	}()

	return ch, nil
}
