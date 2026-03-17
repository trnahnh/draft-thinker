package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/trnahnh/draft-thinker/internal/config"
	"github.com/trnahnh/draft-thinker/internal/entropy"
	"github.com/trnahnh/draft-thinker/internal/metrics"
	"github.com/trnahnh/draft-thinker/internal/router"
	"github.com/trnahnh/draft-thinker/pkg/client"
	"github.com/trnahnh/draft-thinker/pkg/protocol"
)

type mockLLMClient struct {
	completeResp *protocol.ChatCompletionResponse
	completeErr  error
	streamChunks []protocol.StreamChunk
	streamErr    error
}

func (m *mockLLMClient) Complete(_ context.Context, _ *protocol.ChatCompletionRequest) (*protocol.ChatCompletionResponse, error) {
	return m.completeResp, m.completeErr
}

func (m *mockLLMClient) Stream(_ context.Context, _ *protocol.ChatCompletionRequest) (<-chan client.StreamChunk, error) {
	if m.streamErr != nil {
		return nil, m.streamErr
	}
	ch := make(chan client.StreamChunk, len(m.streamChunks))
	for i := range m.streamChunks {
		ch <- client.StreamChunk{Chunk: &m.streamChunks[i]}
	}
	close(ch)
	return ch, nil
}

func confidentChunk(token string) protocol.StreamChunk {
	return protocol.StreamChunk{
		ID: "chunk-1", Object: "chat.completion.chunk", Model: "llama3-8b-8192",
		Choices: []protocol.Choice{
			{
				Index: 0,
				Delta: &protocol.Delta{Content: token},
				Logprobs: &protocol.ChoiceLogprobs{
					Content: []protocol.TokenLogprob{
						{
							Token:   token,
							Logprob: math.Log(0.95),
							TopLogprobs: []protocol.TopLogprobEntry{
								{Token: token, Logprob: math.Log(0.95)},
								{Token: "alt1", Logprob: math.Log(0.03)},
								{Token: "alt2", Logprob: math.Log(0.02)},
							},
						},
					},
				},
			},
		},
	}
}

func uncertainChunk(token string) protocol.StreamChunk {
	return protocol.StreamChunk{
		ID: "chunk-u", Object: "chat.completion.chunk", Model: "llama3-8b-8192",
		Choices: []protocol.Choice{
			{
				Index: 0,
				Delta: &protocol.Delta{Content: token},
				Logprobs: &protocol.ChoiceLogprobs{
					Content: []protocol.TokenLogprob{
						{
							Token:   token,
							Logprob: math.Log(0.34),
							TopLogprobs: []protocol.TopLogprobEntry{
								{Token: token, Logprob: math.Log(0.34)},
								{Token: "alt1", Logprob: math.Log(0.33)},
								{Token: "alt2", Logprob: math.Log(0.33)},
							},
						},
					},
				},
			},
		},
	}
}

func defaultEntropyCfg() config.EntropyConfig {
	return config.EntropyConfig{
		Threshold:      1.0,
		WindowSize:     5,
		EarlyExitCount: 3,
		TopLogprobs:    5,
	}
}

func newTestHandler(drafter, heavyweight *mockLLMClient) *chatHandler {
	cfg := defaultEntropyCfg()
	rtr := router.NewRouter(entropy.WindowConfig{
		Size:           cfg.WindowSize,
		Threshold:      cfg.Threshold,
		EarlyExitCount: cfg.EarlyExitCount,
	}, &metrics.NoopRecorder{})
	return newChatHandler(drafter, heavyweight, rtr, &metrics.NoopRecorder{}, cfg)
}

func TestHandler_AcceptDraft(t *testing.T) {
	drafter := &mockLLMClient{
		streamChunks: []protocol.StreamChunk{
			confidentChunk("Hello"),
			confidentChunk(" world"),
		},
	}
	heavyweight := &mockLLMClient{}

	handler := newTestHandler(drafter, heavyweight)

	body := `{"model":"auto","messages":[{"role":"user","content":"2+2?"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", w.Code, http.StatusOK)
	}

	var resp protocol.ChatCompletionResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp.Model != "llama3-8b-8192" {
		t.Errorf("model: got %q, want drafter model", resp.Model)
	}
	if len(resp.Choices) == 0 || resp.Choices[0].Message == nil {
		t.Fatal("expected assembled response with message")
	}
	if resp.Choices[0].Message.Content != "Hello world" {
		t.Errorf("content: got %q, want %q", resp.Choices[0].Message.Content, "Hello world")
	}
}

func TestHandler_EscalateToHeavyweight(t *testing.T) {
	drafter := &mockLLMClient{
		streamChunks: []protocol.StreamChunk{
			uncertainChunk("Well"),
			uncertainChunk(","),
			uncertainChunk(" it"),
		},
	}
	heavyweight := &mockLLMClient{
		completeResp: &protocol.ChatCompletionResponse{
			ID:      "hw-resp",
			Object:  "chat.completion",
			Created: 1700000000,
			Model:   "gpt-4o",
			Choices: []protocol.Choice{
				{
					Index:   0,
					Message: &protocol.Message{Role: "assistant", Content: "Heavyweight response"},
				},
			},
		},
	}

	handler := newTestHandler(drafter, heavyweight)

	body := `{"model":"auto","messages":[{"role":"user","content":"Explain CRDTs"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", w.Code, http.StatusOK)
	}

	var resp protocol.ChatCompletionResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if resp.Model != "gpt-4o" {
		t.Errorf("model: got %q, want heavyweight model", resp.Model)
	}
	if resp.Choices[0].Message.Content != "Heavyweight response" {
		t.Errorf("content: got %q", resp.Choices[0].Message.Content)
	}
}

func TestHandler_StreamAccept(t *testing.T) {
	drafter := &mockLLMClient{
		streamChunks: []protocol.StreamChunk{
			confidentChunk("Hello"),
			confidentChunk(" world"),
		},
	}
	heavyweight := &mockLLMClient{}

	handler := newTestHandler(drafter, heavyweight)

	body := `{"model":"auto","messages":[{"role":"user","content":"Hi"}],"stream":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", w.Code, http.StatusOK)
	}

	result := w.Body.String()
	if !strings.Contains(result, "data: ") {
		t.Error("response should contain SSE data lines")
	}
	if !strings.Contains(result, "data: [DONE]") {
		t.Error("response should end with [DONE]")
	}
	if !strings.Contains(result, "Hello") {
		t.Error("response should contain draft content 'Hello'")
	}
}

func TestHandler_StreamEscalate(t *testing.T) {
	drafter := &mockLLMClient{
		streamChunks: []protocol.StreamChunk{
			uncertainChunk("Well"),
			uncertainChunk(","),
			uncertainChunk(" it"),
		},
	}
	heavyweight := &mockLLMClient{
		streamChunks: []protocol.StreamChunk{
			{
				ID: "hw-1", Object: "chat.completion.chunk", Model: "gpt-4o",
				Choices: []protocol.Choice{{Index: 0, Delta: &protocol.Delta{Content: "Heavyweight"}}},
			},
			{
				ID: "hw-2", Object: "chat.completion.chunk", Model: "gpt-4o",
				Choices: []protocol.Choice{{Index: 0, Delta: &protocol.Delta{Content: " stream"}}},
			},
		},
	}

	handler := newTestHandler(drafter, heavyweight)

	body := `{"model":"auto","messages":[{"role":"user","content":"Explain CRDTs"}],"stream":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", w.Code, http.StatusOK)
	}

	result := w.Body.String()
	if !strings.Contains(result, "data: [DONE]") {
		t.Error("response should end with [DONE]")
	}
	if !strings.Contains(result, "Heavyweight") {
		t.Error("response should contain heavyweight content")
	}
	if strings.Contains(result, "Well") {
		t.Error("response should not contain drafter content after escalation")
	}
}

func TestHandler_DrafterError(t *testing.T) {
	drafter := &mockLLMClient{
		streamErr: &client.UpstreamError{
			StatusCode: 500,
			Body:       "internal error",
			Provider:   "groq",
		},
	}
	heavyweight := &mockLLMClient{}

	handler := newTestHandler(drafter, heavyweight)

	body := `{"model":"auto","messages":[{"role":"user","content":"Hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != 500 {
		t.Fatalf("status: got %d, want 500", w.Code)
	}
}

func TestHandler_HeavyweightError(t *testing.T) {
	drafter := &mockLLMClient{
		streamChunks: []protocol.StreamChunk{
			uncertainChunk("Well"),
			uncertainChunk(","),
			uncertainChunk(" it"),
		},
	}
	heavyweight := &mockLLMClient{
		completeErr: &client.UpstreamError{
			StatusCode: 429,
			Body:       "rate limited",
			Provider:   "openai",
		},
	}

	handler := newTestHandler(drafter, heavyweight)

	body := `{"model":"auto","messages":[{"role":"user","content":"Explain CRDTs"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != 429 {
		t.Fatalf("status: got %d, want 429", w.Code)
	}
}

func TestHandler_InvalidJSON(t *testing.T) {
	handler := newTestHandler(&mockLLMClient{}, &mockLLMClient{})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{invalid`))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandler_EmptyMessages(t *testing.T) {
	handler := newTestHandler(&mockLLMClient{}, &mockLLMClient{})

	body := `{"model":"auto","messages":[]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandler_WrongMethod(t *testing.T) {
	handler := newTestHandler(&mockLLMClient{}, &mockLLMClient{})

	req := httptest.NewRequest(http.MethodGet, "/v1/chat/completions", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandler_UpstreamTimeout(t *testing.T) {
	drafter := &mockLLMClient{
		streamErr: &client.UpstreamTimeoutError{Provider: "groq"},
	}
	heavyweight := &mockLLMClient{}

	handler := newTestHandler(drafter, heavyweight)

	body := `{"model":"auto","messages":[{"role":"user","content":"Hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusGatewayTimeout {
		t.Fatalf("status: got %d, want %d", w.Code, http.StatusGatewayTimeout)
	}
}
