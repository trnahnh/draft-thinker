package gateway

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/trnahnh/draft-thinker/internal/config"
	"github.com/trnahnh/draft-thinker/internal/entropy"
	"github.com/trnahnh/draft-thinker/internal/metrics"
	"github.com/trnahnh/draft-thinker/internal/router"
	"github.com/trnahnh/draft-thinker/internal/speculative"
	"github.com/trnahnh/draft-thinker/pkg/client"
	"github.com/trnahnh/draft-thinker/pkg/protocol"
)

func newFallbackHandler(drafter, heavyweight *mockLLMClient, speculativeOn bool) *chatHandler {
	cfg := defaultEntropyCfg()
	windowCfg := entropy.WindowConfig{Size: cfg.WindowSize, Threshold: cfg.Threshold, EarlyExitCount: cfg.EarlyExitCount}
	rtr := router.NewRouter(windowCfg, &metrics.NoopRecorder{})
	specCfg := config.SpeculativeConfig{Enabled: &speculativeOn, SoftThresholdMult: 0.8}
	var exec *speculative.Executor
	if speculativeOn {
		exec = speculative.NewExecutor(speculative.ExecutorConfig{WindowCfg: windowCfg, SoftThresholdMult: 0.8}, heavyweight, &metrics.NoopRecorder{})
	}
	return newChatHandler(drafter, heavyweight, rtr, exec, &metrics.NoopRecorder{}, cfg, specCfg, config.FallbackConfig{}, nil)
}

func heavyweightResponse(content string) *protocol.ChatCompletionResponse {
	return &protocol.ChatCompletionResponse{
		ID: "hw-1", Object: "chat.completion", Model: "gpt-4.1",
		Choices: []protocol.Choice{{Index: 0, Message: &protocol.Message{Role: "assistant", Content: content}}},
	}
}

func uncertainDraft() []protocol.StreamChunk {
	return []protocol.StreamChunk{uncertainChunk("Well"), uncertainChunk(","), uncertainChunk(" maybe")}
}

func post(handler http.Handler, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w
}

func TestFallback_DrafterErrorRoutesToHeavyweight(t *testing.T) {
	drafter := &mockLLMClient{streamErr: &client.UpstreamError{StatusCode: 500, Body: "boom", Provider: "openai"}}
	heavyweight := &mockLLMClient{completeResp: heavyweightResponse("heavy answer")}

	w := post(newFallbackHandler(drafter, heavyweight, false), `{"model":"auto","messages":[{"role":"user","content":"Hi"}]}`)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (body %s)", w.Code, w.Body.String())
	}
	if got := w.Header().Get("X-Routing-Decision"); got != "fallback_heavyweight" {
		t.Errorf("X-Routing-Decision = %q, want fallback_heavyweight", got)
	}
	if got := w.Header().Get("X-Fallback-Reason"); got != "drafter_error" {
		t.Errorf("X-Fallback-Reason = %q, want drafter_error", got)
	}
	var resp protocol.ChatCompletionResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Choices[0].Message.Content != "heavy answer" {
		t.Errorf("content = %q, want heavyweight answer", resp.Choices[0].Message.Content)
	}
}

func TestFallback_DrafterTimeoutRoutesToHeavyweight(t *testing.T) {
	drafter := &mockLLMClient{streamErr: &client.UpstreamTimeoutError{Provider: "openai"}}
	heavyweight := &mockLLMClient{completeResp: heavyweightResponse("heavy answer")}

	w := post(newFallbackHandler(drafter, heavyweight, true), `{"model":"auto","messages":[{"role":"user","content":"Hi"}]}`)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}
	if got := w.Header().Get("X-Routing-Decision"); got != "fallback_heavyweight" {
		t.Errorf("X-Routing-Decision = %q, want fallback_heavyweight", got)
	}
}

func TestFallback_DrafterErrorStreamRoutesToHeavyweight(t *testing.T) {
	drafter := &mockLLMClient{streamErr: &client.UpstreamError{StatusCode: 503, Body: "down", Provider: "openai"}}
	heavyweight := &mockLLMClient{streamChunks: []protocol.StreamChunk{confidentChunk("heavy"), confidentChunk(" stream")}}

	w := post(newFallbackHandler(drafter, heavyweight, false), `{"model":"auto","messages":[{"role":"user","content":"Hi"}],"stream":true}`)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}
	if got := w.Header().Get("X-Routing-Decision"); got != "fallback_heavyweight" {
		t.Errorf("X-Routing-Decision = %q, want fallback_heavyweight", got)
	}
	if !strings.Contains(w.Body.String(), "data: [DONE]") {
		t.Error("expected SSE terminator in fallback stream")
	}
}

func TestFallback_BothProvidersDownSurfacesHeavyweightError(t *testing.T) {
	drafter := &mockLLMClient{streamErr: &client.UpstreamError{StatusCode: 500, Body: "boom", Provider: "openai"}}
	heavyweight := &mockLLMClient{completeErr: &client.UpstreamError{StatusCode: 503, Body: "down", Provider: "openai"}}

	w := post(newFallbackHandler(drafter, heavyweight, false), `{"model":"auto","messages":[{"role":"user","content":"Hi"}]}`)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: got %d, want 503", w.Code)
	}
}

func TestFallback_HeavyweightErrorServesDraft(t *testing.T) {
	drafter := &mockLLMClient{streamChunks: uncertainDraft()}
	heavyweight := &mockLLMClient{completeErr: &client.UpstreamError{StatusCode: 429, Body: "rate limited", Provider: "openai"}}

	w := post(newFallbackHandler(drafter, heavyweight, false), `{"model":"auto","messages":[{"role":"user","content":"Explain CRDTs"}]}`)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (body %s)", w.Code, w.Body.String())
	}
	if got := w.Header().Get("X-Routing-Decision"); got != "fallback_draft" {
		t.Errorf("X-Routing-Decision = %q, want fallback_draft", got)
	}
	if got := w.Header().Get("X-Fallback-Reason"); got != "heavyweight_error" {
		t.Errorf("X-Fallback-Reason = %q, want heavyweight_error", got)
	}
	var resp protocol.ChatCompletionResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Choices[0].Message.Content != "Well, maybe" {
		t.Errorf("content = %q, want the assembled draft", resp.Choices[0].Message.Content)
	}
}

func TestFallback_HeavyweightStreamErrorServesDraftAsSSE(t *testing.T) {
	drafter := &mockLLMClient{streamChunks: uncertainDraft()}
	heavyweight := &mockLLMClient{streamErr: &client.UpstreamTimeoutError{Provider: "openai"}}

	w := post(newFallbackHandler(drafter, heavyweight, false), `{"model":"auto","messages":[{"role":"user","content":"Explain CRDTs"}],"stream":true}`)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}
	if got := w.Header().Get("X-Routing-Decision"); got != "fallback_draft" {
		t.Errorf("X-Routing-Decision = %q, want fallback_draft", got)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Well") || !strings.Contains(body, "data: [DONE]") {
		t.Errorf("expected draft tokens replayed as SSE, got %q", body)
	}
}

func TestFallback_SpeculativeHeavyweightStreamFailsBeforeFirstChunk(t *testing.T) {
	drafter := &mockLLMClient{streamChunks: uncertainDraft()}
	heavyweight := &mockLLMClient{streamErr: &client.UpstreamError{StatusCode: 502, Body: "bad gateway", Provider: "openai"}}

	w := post(newFallbackHandler(drafter, heavyweight, true), `{"model":"auto","messages":[{"role":"user","content":"Explain CRDTs"}],"stream":true}`)

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want 200 (body %s)", w.Code, w.Body.String())
	}
	if got := w.Header().Get("X-Routing-Decision"); got != "fallback_draft" {
		t.Errorf("X-Routing-Decision = %q, want fallback_draft", got)
	}
}

func TestFallback_DisabledPreservesUpstreamErrors(t *testing.T) {
	off := false
	cfg := defaultEntropyCfg()
	rtr := router.NewRouter(entropy.WindowConfig{Size: cfg.WindowSize, Threshold: cfg.Threshold, EarlyExitCount: cfg.EarlyExitCount}, &metrics.NoopRecorder{})
	specCfg := config.SpeculativeConfig{Enabled: &off}
	fb := config.FallbackConfig{OnDrafterError: &off, ServeDraftOnHeavyError: &off}

	drafter := &mockLLMClient{streamChunks: uncertainDraft()}
	heavyweight := &mockLLMClient{completeErr: &client.UpstreamError{StatusCode: 429, Body: "rate limited", Provider: "openai"}}
	h := newChatHandler(drafter, heavyweight, rtr, nil, &metrics.NoopRecorder{}, cfg, specCfg, fb, nil)

	w := post(h, `{"model":"auto","messages":[{"role":"user","content":"Explain CRDTs"}]}`)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("status: got %d, want 429 with fallback disabled", w.Code)
	}
}

func TestFallbackConfigDefaultsToEnabled(t *testing.T) {
	var fb config.FallbackConfig
	if !fb.OnDrafterErrorEnabled() || !fb.ServeDraftOnHeavyErrorEnabled() {
		t.Error("fallback options must default to enabled when omitted from config")
	}
}
