package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestEmbeddingClient_Embed(t *testing.T) {
	expected := []float64{0.1, 0.2, 0.3, 0.4, 0.5}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method: got %s, want POST", r.Method)
		}
		if r.URL.Path != "/embeddings" {
			t.Errorf("path: got %s, want /embeddings", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("auth header: got %q", r.Header.Get("Authorization"))
		}

		var req embeddingRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Model != "text-embedding-3-small" {
			t.Errorf("model: got %q", req.Model)
		}
		if req.Dimensions != 1536 {
			t.Errorf("dimensions: got %d", req.Dimensions)
		}
		if req.Input != "hello world" {
			t.Errorf("input: got %q", req.Input)
		}

		resp := embeddingResponse{
			Data: []embeddingData{{Embedding: expected, Index: 0}},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := NewEmbeddingClient(srv.URL, "test-key", "text-embedding-3-small", 1536, 5*time.Second)

	vec, err := client.Embed(context.Background(), "hello world")
	if err != nil {
		t.Fatalf("embed: %v", err)
	}

	if len(vec) != len(expected) {
		t.Fatalf("vector length: got %d, want %d", len(vec), len(expected))
	}
	for i, v := range vec {
		if v != expected[i] {
			t.Errorf("vec[%d]: got %f, want %f", i, v, expected[i])
		}
	}
}

func TestEmbeddingClient_UpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error": "rate limited"}`))
	}))
	defer srv.Close()

	client := NewEmbeddingClient(srv.URL, "test-key", "text-embedding-3-small", 1536, 5*time.Second)

	_, err := client.Embed(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected error for 429 response")
	}

	upErr, ok := err.(*UpstreamError)
	if !ok {
		t.Fatalf("expected UpstreamError, got %T", err)
	}
	if upErr.StatusCode != 429 {
		t.Errorf("status: got %d, want 429", upErr.StatusCode)
	}
}

func TestEmbeddingClient_EmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := embeddingResponse{Data: []embeddingData{}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	client := NewEmbeddingClient(srv.URL, "test-key", "text-embedding-3-small", 1536, 5*time.Second)

	_, err := client.Embed(context.Background(), "hello")
	if err == nil {
		t.Fatal("expected error for empty embedding response")
	}
}
