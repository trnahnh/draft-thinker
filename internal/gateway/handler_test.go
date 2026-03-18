package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/trnahnh/draft-thinker/internal/cache"
	"github.com/trnahnh/draft-thinker/internal/config"
	"github.com/trnahnh/draft-thinker/internal/entropy"
	"github.com/trnahnh/draft-thinker/internal/metrics"
	"github.com/trnahnh/draft-thinker/internal/router"
	"github.com/trnahnh/draft-thinker/internal/speculative"
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
	disabled := false
	specCfg := config.SpeculativeConfig{Enabled: &disabled}
	return newChatHandler(drafter, heavyweight, rtr, nil, &metrics.NoopRecorder{}, cfg, specCfg, nil)
}

func newTestHandlerWithSpec(drafter, heavyweight *mockLLMClient) *chatHandler {
	cfg := defaultEntropyCfg()
	rtr := router.NewRouter(entropy.WindowConfig{
		Size:           cfg.WindowSize,
		Threshold:      cfg.Threshold,
		EarlyExitCount: cfg.EarlyExitCount,
	}, &metrics.NoopRecorder{})
	enabled := true
	specCfg := config.SpeculativeConfig{Enabled: &enabled, SoftThresholdMult: 0.8}
	exec := speculative.NewExecutor(speculative.ExecutorConfig{
		WindowCfg: entropy.WindowConfig{
			Size:           cfg.WindowSize,
			Threshold:      cfg.Threshold,
			EarlyExitCount: cfg.EarlyExitCount,
		},
		SoftThresholdMult: specCfg.SoftThresholdMult,
	}, heavyweight, &metrics.NoopRecorder{})
	return newChatHandler(drafter, heavyweight, rtr, exec, &metrics.NoopRecorder{}, cfg, specCfg, nil)
}

func newTestHandlerWithCache(drafter, heavyweight *mockLLMClient, cs cache.Store) *chatHandler {
	cfg := defaultEntropyCfg()
	rtr := router.NewRouter(entropy.WindowConfig{
		Size:           cfg.WindowSize,
		Threshold:      cfg.Threshold,
		EarlyExitCount: cfg.EarlyExitCount,
	}, &metrics.NoopRecorder{})
	disabled := false
	specCfg := config.SpeculativeConfig{Enabled: &disabled}
	return newChatHandler(drafter, heavyweight, rtr, nil, &metrics.NoopRecorder{}, cfg, specCfg, cs)
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
	if got := w.Header().Get("X-Routing-Decision"); got != "accept" {
		t.Errorf("X-Routing-Decision: got %q, want %q", got, "accept")
	}
	if got := w.Header().Get("X-Request-Duration-Ms"); got == "" {
		t.Error("X-Request-Duration-Ms header missing")
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
	if got := w.Header().Get("X-Routing-Decision"); got != "escalate" {
		t.Errorf("X-Routing-Decision: got %q, want %q", got, "escalate")
	}
	if got := w.Header().Get("X-Request-Duration-Ms"); got == "" {
		t.Error("X-Request-Duration-Ms header missing")
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

// midEntropyChunk generates entropy above softThreshold but below hardThreshold.
// Probs 0.80/0.12/0.08 give Shannon entropy ~0.92 bits (> 0.8 soft, < 1.0 hard with threshold=1.0).
func midEntropyChunk(token string) protocol.StreamChunk {
	return protocol.StreamChunk{
		ID: "chunk-m", Object: "chat.completion.chunk", Model: "llama3-8b-8192",
		Choices: []protocol.Choice{
			{
				Index: 0,
				Delta: &protocol.Delta{Content: token},
				Logprobs: &protocol.ChoiceLogprobs{
					Content: []protocol.TokenLogprob{
						{
							Token:   token,
							Logprob: math.Log(0.80),
							TopLogprobs: []protocol.TopLogprobEntry{
								{Token: token, Logprob: math.Log(0.80)},
								{Token: "alt1", Logprob: math.Log(0.12)},
								{Token: "alt2", Logprob: math.Log(0.08)},
							},
						},
					},
				},
			},
		},
	}
}

func TestHandler_SpeculativeAccept(t *testing.T) {
	drafter := &mockLLMClient{
		streamChunks: []protocol.StreamChunk{
			confidentChunk("Hello"),
			confidentChunk(" world"),
		},
	}
	heavyweight := &mockLLMClient{}

	handler := newTestHandlerWithSpec(drafter, heavyweight)

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
	if resp.Choices[0].Message.Content != "Hello world" {
		t.Errorf("content: got %q, want %q", resp.Choices[0].Message.Content, "Hello world")
	}
}

func TestHandler_SpeculativeEscalation(t *testing.T) {
	drafter := &mockLLMClient{
		streamChunks: []protocol.StreamChunk{
			midEntropyChunk("hmm"),
			uncertainChunk("well"),
			uncertainChunk("uh"),
		},
	}
	heavyweight := &mockLLMClient{
		completeResp: &protocol.ChatCompletionResponse{
			ID: "hw-resp", Object: "chat.completion", Model: "gpt-4o",
			Choices: []protocol.Choice{
				{Index: 0, Message: &protocol.Message{Role: "assistant", Content: "Heavyweight response"}},
			},
		},
		streamChunks: []protocol.StreamChunk{
			{ID: "hw-1", Object: "chat.completion.chunk", Model: "gpt-4o",
				Choices: []protocol.Choice{{Index: 0, Delta: &protocol.Delta{Content: "Speculative"}}}},
			{ID: "hw-2", Object: "chat.completion.chunk", Model: "gpt-4o",
				Choices: []protocol.Choice{{Index: 0, Delta: &protocol.Delta{Content: " heavy"}}}},
		},
	}

	handler := newTestHandlerWithSpec(drafter, heavyweight)

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
	if resp.Choices[0].Message.Content != "Speculative heavy" {
		t.Errorf("content: got %q, want %q", resp.Choices[0].Message.Content, "Speculative heavy")
	}
}

func TestHandler_SpeculativeCancellation(t *testing.T) {
	drafter := &mockLLMClient{
		streamChunks: []protocol.StreamChunk{
			confidentChunk("Hello"),
			midEntropyChunk("maybe"),
			confidentChunk(" world"),
			confidentChunk(" again"),
			confidentChunk(" more"),
			confidentChunk(" text"),
		},
	}
	heavyweight := &mockLLMClient{
		streamChunks: []protocol.StreamChunk{
			{ID: "hw-1", Object: "chat.completion.chunk", Model: "gpt-4o",
				Choices: []protocol.Choice{{Index: 0, Delta: &protocol.Delta{Content: "Heavy"}}}},
		},
	}

	handler := newTestHandlerWithSpec(drafter, heavyweight)

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
	if resp.Choices[0].Message.Content != "Hellomaybe world again more text" {
		t.Errorf("content: got %q", resp.Choices[0].Message.Content)
	}
}

func TestHandler_SpeculativeStreamAccept(t *testing.T) {
	drafter := &mockLLMClient{
		streamChunks: []protocol.StreamChunk{
			confidentChunk("Hello"),
			confidentChunk(" world"),
		},
	}
	heavyweight := &mockLLMClient{}

	handler := newTestHandlerWithSpec(drafter, heavyweight)

	body := `{"model":"auto","messages":[{"role":"user","content":"Hi"}],"stream":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", w.Code, http.StatusOK)
	}
	result := w.Body.String()
	if !strings.Contains(result, "Hello") {
		t.Error("response should contain draft content")
	}
	if !strings.Contains(result, "data: [DONE]") {
		t.Error("response should end with [DONE]")
	}
}

func TestHandler_SpeculativeStreamEscalation(t *testing.T) {
	drafter := &mockLLMClient{
		streamChunks: []protocol.StreamChunk{
			midEntropyChunk("hmm"),
			uncertainChunk("well"),
			uncertainChunk("uh"),
		},
	}
	heavyweight := &mockLLMClient{
		streamChunks: []protocol.StreamChunk{
			{ID: "hw-1", Object: "chat.completion.chunk", Model: "gpt-4o",
				Choices: []protocol.Choice{{Index: 0, Delta: &protocol.Delta{Content: "Heavy"}}}},
			{ID: "hw-2", Object: "chat.completion.chunk", Model: "gpt-4o",
				Choices: []protocol.Choice{{Index: 0, Delta: &protocol.Delta{Content: " stream"}}}},
		},
	}

	handler := newTestHandlerWithSpec(drafter, heavyweight)

	body := `{"model":"auto","messages":[{"role":"user","content":"Explain CRDTs"}],"stream":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", w.Code, http.StatusOK)
	}
	result := w.Body.String()
	if !strings.Contains(result, "Heavy") {
		t.Error("response should contain speculative heavyweight content")
	}
	if !strings.Contains(result, "data: [DONE]") {
		t.Error("response should end with [DONE]")
	}
}

func TestHandler_HardEscalationNoSoftThreshold(t *testing.T) {
	drafter := &mockLLMClient{
		streamChunks: []protocol.StreamChunk{
			uncertainChunk("Well"),
			uncertainChunk(","),
			uncertainChunk(" it"),
		},
	}
	heavyweight := &mockLLMClient{
		completeResp: &protocol.ChatCompletionResponse{
			ID: "hw-resp", Object: "chat.completion", Model: "gpt-4o",
			Choices: []protocol.Choice{
				{Index: 0, Message: &protocol.Message{Role: "assistant", Content: "Heavyweight response"}},
			},
		},
	}

	handler := newTestHandlerWithSpec(drafter, heavyweight)

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

type mockCacheStore struct {
	lookupResp *protocol.ChatCompletionResponse
	lookupErr  error
	inserted   bool
	insertErr  error
	evicted    bool
	evictErr   error
}

func (m *mockCacheStore) Lookup(_ context.Context, _ []protocol.Message) (*protocol.ChatCompletionResponse, error) {
	return m.lookupResp, m.lookupErr
}

func (m *mockCacheStore) Insert(_ context.Context, _ []protocol.Message, _ *protocol.ChatCompletionResponse) error {
	m.inserted = true
	return m.insertErr
}

func (m *mockCacheStore) Evict(_ context.Context, _ string) error {
	m.evicted = true
	return m.evictErr
}

func TestHandler_CacheHit(t *testing.T) {
	cachedResp := &protocol.ChatCompletionResponse{
		ID: "cached-1", Object: "chat.completion", Model: "gpt-4.1-nano",
		Choices: []protocol.Choice{
			{Index: 0, Message: &protocol.Message{Role: "assistant", Content: "cached answer"}},
		},
	}
	cs := &mockCacheStore{lookupResp: cachedResp}
	handler := newTestHandlerWithCache(&mockLLMClient{}, &mockLLMClient{}, cs)

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
	if resp.Choices[0].Message.Content != "cached answer" {
		t.Errorf("content: got %q, want %q", resp.Choices[0].Message.Content, "cached answer")
	}
	if got := w.Header().Get("X-Routing-Decision"); got != "cache_hit" {
		t.Errorf("X-Routing-Decision: got %q, want %q", got, "cache_hit")
	}
	if got := w.Header().Get("X-Request-Duration-Ms"); got == "" {
		t.Error("X-Request-Duration-Ms header missing")
	}
}

func TestHandler_CacheHitStream(t *testing.T) {
	cachedResp := &protocol.ChatCompletionResponse{
		ID: "cached-1", Object: "chat.completion", Model: "gpt-4.1-nano",
		Choices: []protocol.Choice{
			{Index: 0, Message: &protocol.Message{Role: "assistant", Content: "cached stream"}},
		},
	}
	cs := &mockCacheStore{lookupResp: cachedResp}
	handler := newTestHandlerWithCache(&mockLLMClient{}, &mockLLMClient{}, cs)

	body := `{"model":"auto","messages":[{"role":"user","content":"Hi"}],"stream":true}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", w.Code, http.StatusOK)
	}

	result := w.Body.String()
	if !strings.Contains(result, "cached stream") {
		t.Error("response should contain cached content")
	}
	if !strings.Contains(result, "data: [DONE]") {
		t.Error("response should end with [DONE]")
	}
}

func TestHandler_CacheMiss(t *testing.T) {
	cs := &mockCacheStore{lookupResp: nil}
	drafter := &mockLLMClient{
		streamChunks: []protocol.StreamChunk{
			confidentChunk("Hello"),
			confidentChunk(" world"),
		},
	}
	handler := newTestHandlerWithCache(drafter, &mockLLMClient{}, cs)

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
	if resp.Choices[0].Message.Content != "Hello world" {
		t.Errorf("content: got %q, want draft content", resp.Choices[0].Message.Content)
	}
}

func TestHandler_CacheError(t *testing.T) {
	cs := &mockCacheStore{lookupErr: fmt.Errorf("cache unavailable")}
	drafter := &mockLLMClient{
		streamChunks: []protocol.StreamChunk{
			confidentChunk("Hello"),
		},
	}
	handler := newTestHandlerWithCache(drafter, &mockLLMClient{}, cs)

	body := `{"model":"auto","messages":[{"role":"user","content":"Hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d (should fall through on cache error)", w.Code, http.StatusOK)
	}
}

func TestHandler_NoCacheInsertOnEscalate(t *testing.T) {
	cs := &mockCacheStore{lookupResp: nil}
	drafter := &mockLLMClient{
		streamChunks: []protocol.StreamChunk{
			uncertainChunk("Well"),
			uncertainChunk(","),
			uncertainChunk(" it"),
		},
	}
	heavyweight := &mockLLMClient{
		completeResp: &protocol.ChatCompletionResponse{
			ID: "hw-resp", Object: "chat.completion", Model: "gpt-4o",
			Choices: []protocol.Choice{
				{Index: 0, Message: &protocol.Message{Role: "assistant", Content: "Heavyweight response"}},
			},
		},
	}
	handler := newTestHandlerWithCache(drafter, heavyweight, cs)

	body := `{"model":"auto","messages":[{"role":"user","content":"Explain CRDTs"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", w.Code, http.StatusOK)
	}
	if cs.inserted {
		t.Error("cache insert should not be called on escalated responses")
	}
}
