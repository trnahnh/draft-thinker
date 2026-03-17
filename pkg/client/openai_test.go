package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/trnahnh/draft-thinker/pkg/protocol"
)

func TestOpenAIClient_ProviderInErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error": {"message": "rate limited"}}`))
	}))
	defer server.Close()

	c := NewOpenAIClient(server.URL, "test-key", "gpt-4o", 10*time.Second)

	_, err := c.Complete(context.Background(), &protocol.ChatCompletionRequest{
		Model:    "auto",
		Messages: []protocol.Message{{Role: "user", Content: "Hi"}},
	})
	if err == nil {
		t.Fatal("expected error")
	}

	upErr, ok := err.(*UpstreamError)
	if !ok {
		t.Fatalf("expected UpstreamError, got %T", err)
	}
	if upErr.Provider != "openai" {
		t.Errorf("provider: got %q, want %q", upErr.Provider, "openai")
	}
	if upErr.StatusCode != http.StatusTooManyRequests {
		t.Errorf("status: got %d, want %d", upErr.StatusCode, http.StatusTooManyRequests)
	}
}

func TestOpenAIClient_Complete(t *testing.T) {
	expected := protocol.ChatCompletionResponse{
		ID:      "chatcmpl-openai",
		Object:  "chat.completion",
		Created: 1700000000,
		Model:   "gpt-4o",
		Choices: []protocol.Choice{
			{
				Index:   0,
				Message: &protocol.Message{Role: "assistant", Content: "Hello from OpenAI"},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Get("Authorization"); auth != "Bearer openai-key" {
			t.Errorf("auth: got %q, want %q", auth, "Bearer openai-key")
		}

		var req protocol.ChatCompletionRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.Model != "gpt-4o" {
			t.Errorf("model: got %q, want %q", req.Model, "gpt-4o")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expected)
	}))
	defer server.Close()

	c := NewOpenAIClient(server.URL, "openai-key", "gpt-4o", 10*time.Second)

	resp, err := c.Complete(context.Background(), &protocol.ChatCompletionRequest{
		Model:    "auto",
		Messages: []protocol.Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.ID != "chatcmpl-openai" {
		t.Errorf("id: got %q, want %q", resp.ID, "chatcmpl-openai")
	}
	if resp.Choices[0].Message.Content != "Hello from OpenAI" {
		t.Errorf("content: got %q", resp.Choices[0].Message.Content)
	}
}
