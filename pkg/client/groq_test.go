package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/trnahnh/draft-thinker/pkg/protocol"
)

func TestGroqClient_Complete(t *testing.T) {
	expected := protocol.ChatCompletionResponse{
		ID:      "chatcmpl-test",
		Object:  "chat.completion",
		Created: 1700000000,
		Model:   "llama3-8b-8192",
		Choices: []protocol.Choice{
			{
				Index:   0,
				Message: &protocol.Message{Role: "assistant", Content: "4"},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method: got %s, want POST", r.Method)
		}
		if r.URL.Path != "/chat/completions" {
			t.Errorf("path: got %s, want /chat/completions", r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer test-key" {
			t.Errorf("auth: got %q, want %q", auth, "Bearer test-key")
		}

		var req protocol.ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Stream {
			t.Error("stream should be false for Complete")
		}
		if req.Model != "llama3-8b-8192" {
			t.Errorf("model: got %q, want %q", req.Model, "llama3-8b-8192")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expected)
	}))
	defer server.Close()

	client := NewGroqClient(server.URL, "test-key", "llama3-8b-8192", 10*time.Second)

	resp, err := client.Complete(context.Background(), &protocol.ChatCompletionRequest{
		Model:    "auto",
		Messages: []protocol.Message{{Role: "user", Content: "2+2?"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.ID != "chatcmpl-test" {
		t.Errorf("id: got %q, want %q", resp.ID, "chatcmpl-test")
	}
	if resp.Choices[0].Message.Content != "4" {
		t.Errorf("content: got %q, want %q", resp.Choices[0].Message.Content, "4")
	}
}

func TestGroqClient_Complete_UpstreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error": {"message": "rate limited"}}`))
	}))
	defer server.Close()

	client := NewGroqClient(server.URL, "test-key", "llama3-8b-8192", 10*time.Second)

	_, err := client.Complete(context.Background(), &protocol.ChatCompletionRequest{
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
	if upErr.StatusCode != http.StatusTooManyRequests {
		t.Errorf("status: got %d, want %d", upErr.StatusCode, http.StatusTooManyRequests)
	}
}

func TestGroqClient_Stream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req protocol.ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if !req.Stream {
			t.Error("stream should be true")
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("expected Flusher")
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		chunks := []string{"Hello", " world", "!"}
		for i, text := range chunks {
			chunk := protocol.StreamChunk{
				ID:      "chatcmpl-stream",
				Object:  "chat.completion.chunk",
				Created: 1700000000,
				Model:   "llama3-8b-8192",
				Choices: []protocol.Choice{
					{
						Index: 0,
						Delta: &protocol.Delta{Content: text},
					},
				},
			}
			data, _ := json.Marshal(chunk)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
			_ = i
		}
		fmt.Fprint(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer server.Close()

	client := NewGroqClient(server.URL, "test-key", "llama3-8b-8192", 10*time.Second)

	ch, err := client.Stream(context.Background(), &protocol.ChatCompletionRequest{
		Model:    "auto",
		Messages: []protocol.Message{{Role: "user", Content: "Hello"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var collected string
	for sc := range ch {
		if sc.Err != nil {
			t.Fatalf("stream error: %v", sc.Err)
		}
		if len(sc.Chunk.Choices) > 0 && sc.Chunk.Choices[0].Delta != nil {
			collected += sc.Chunk.Choices[0].Delta.Content
		}
	}

	if collected != "Hello world!" {
		t.Errorf("collected: got %q, want %q", collected, "Hello world!")
	}
}

func TestGroqClient_Stream_UpstreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": {"message": "internal error"}}`))
	}))
	defer server.Close()

	client := NewGroqClient(server.URL, "test-key", "llama3-8b-8192", 10*time.Second)

	_, err := client.Stream(context.Background(), &protocol.ChatCompletionRequest{
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
	if upErr.StatusCode != http.StatusInternalServerError {
		t.Errorf("status: got %d, want %d", upErr.StatusCode, http.StatusInternalServerError)
	}
}

func TestGroqClient_Complete_AutoModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req protocol.ChatCompletionRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.Model != "llama3-8b-8192" {
			t.Errorf("expected model replacement, got %q", req.Model)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(protocol.ChatCompletionResponse{
			ID:     "test",
			Object: "chat.completion",
		})
	}))
	defer server.Close()

	client := NewGroqClient(server.URL, "test-key", "llama3-8b-8192", 10*time.Second)
	_, err := client.Complete(context.Background(), &protocol.ChatCompletionRequest{
		Model:    "auto",
		Messages: []protocol.Message{{Role: "user", Content: "Hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
