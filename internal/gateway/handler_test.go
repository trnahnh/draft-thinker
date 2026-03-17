package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/trnahnh/draft-thinker/internal/metrics"
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

func TestHandler_Complete(t *testing.T) {
	finish := "stop"
	mock := &mockLLMClient{
		completeResp: &protocol.ChatCompletionResponse{
			ID:      "test-123",
			Object:  "chat.completion",
			Created: 1700000000,
			Model:   "llama3-8b-8192",
			Choices: []protocol.Choice{
				{
					Index:        0,
					Message:      &protocol.Message{Role: "assistant", Content: "4"},
					FinishReason: &finish,
				},
			},
		},
	}

	handler := newChatHandler(mock, &metrics.NoopRecorder{})

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

	if resp.ID != "test-123" {
		t.Errorf("id: got %q, want %q", resp.ID, "test-123")
	}
	if resp.Choices[0].Message.Content != "4" {
		t.Errorf("content: got %q, want %q", resp.Choices[0].Message.Content, "4")
	}
}

func TestHandler_Stream(t *testing.T) {
	mock := &mockLLMClient{
		streamChunks: []protocol.StreamChunk{
			{
				ID: "chunk-1", Object: "chat.completion.chunk", Model: "llama3-8b-8192",
				Choices: []protocol.Choice{{Index: 0, Delta: &protocol.Delta{Content: "Hello"}}},
			},
			{
				ID: "chunk-2", Object: "chat.completion.chunk", Model: "llama3-8b-8192",
				Choices: []protocol.Choice{{Index: 0, Delta: &protocol.Delta{Content: " world"}}},
			},
		},
	}

	handler := newChatHandler(mock, &metrics.NoopRecorder{})

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
		t.Error("response should contain 'Hello'")
	}
}

func TestHandler_InvalidJSON(t *testing.T) {
	handler := newChatHandler(&mockLLMClient{}, &metrics.NoopRecorder{})

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{invalid`))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}

	var errResp protocol.ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if errResp.Error.Type != "invalid_request_error" {
		t.Errorf("type: got %q, want %q", errResp.Error.Type, "invalid_request_error")
	}
}

func TestHandler_EmptyMessages(t *testing.T) {
	handler := newChatHandler(&mockLLMClient{}, &metrics.NoopRecorder{})

	body := `{"model":"auto","messages":[]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandler_WrongMethod(t *testing.T) {
	handler := newChatHandler(&mockLLMClient{}, &metrics.NoopRecorder{})

	req := httptest.NewRequest(http.MethodGet, "/v1/chat/completions", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandler_UpstreamError(t *testing.T) {
	mock := &mockLLMClient{
		completeErr: &client.UpstreamError{
			StatusCode: 429,
			Body:       "rate limited",
			Provider:   "groq",
		},
	}

	handler := newChatHandler(mock, &metrics.NoopRecorder{})

	body := `{"model":"auto","messages":[{"role":"user","content":"Hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != 429 {
		t.Fatalf("status: got %d, want 429", w.Code)
	}
}

func TestHandler_UpstreamTimeout(t *testing.T) {
	mock := &mockLLMClient{
		completeErr: &client.UpstreamTimeoutError{Provider: "groq"},
	}

	handler := newChatHandler(mock, &metrics.NoopRecorder{})

	body := `{"model":"auto","messages":[{"role":"user","content":"Hi"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusGatewayTimeout {
		t.Fatalf("status: got %d, want %d", w.Code, http.StatusGatewayTimeout)
	}
}
