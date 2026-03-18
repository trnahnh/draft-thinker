package cache

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestQdrantClient_EnsureCollection(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method: got %s, want PUT", r.Method)
		}
		if r.URL.Path != "/collections/test_cache" {
			t.Errorf("path: got %s", r.URL.Path)
		}

		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)

		vectors, ok := body["vectors"].(map[string]any)
		if !ok {
			t.Fatal("expected vectors in body")
		}
		if vectors["distance"] != "Cosine" {
			t.Errorf("distance: got %v", vectors["distance"])
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"result": true}`))
	}))
	defer srv.Close()

	q := newQdrantClient(srv.URL, "test_cache", 5*time.Second)
	err := q.EnsureCollection(context.Background(), 1536)
	if err != nil {
		t.Fatalf("ensure collection: %v", err)
	}
}

func TestQdrantClient_Search_Hit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method: got %s, want POST", r.Method)
		}

		resp := qdrantSearchResponse{
			Result: []qdrantSearchHit{
				{ID: "abc-123", Score: 0.97, Payload: map[string]string{"redis_key": "cache:abc-123"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	q := newQdrantClient(srv.URL, "test_cache", 5*time.Second)
	result, err := q.Search(context.Background(), []float64{0.1, 0.2}, 0.95)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if result == nil {
		t.Fatal("expected search result")
	}
	if result.ID != "abc-123" {
		t.Errorf("id: got %q", result.ID)
	}
	if result.Score != 0.97 {
		t.Errorf("score: got %f", result.Score)
	}
	if result.Payload["redis_key"] != "cache:abc-123" {
		t.Errorf("payload: got %v", result.Payload)
	}
}

func TestQdrantClient_Search_Miss(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := qdrantSearchResponse{Result: []qdrantSearchHit{}}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	q := newQdrantClient(srv.URL, "test_cache", 5*time.Second)
	result, err := q.Search(context.Background(), []float64{0.1, 0.2}, 0.95)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if result != nil {
		t.Error("expected nil result for miss")
	}
}

func TestQdrantClient_Upsert(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method: got %s, want PUT", r.Method)
		}

		var body qdrantUpsertRequest
		json.NewDecoder(r.Body).Decode(&body)

		if len(body.Points) != 1 {
			t.Fatalf("points: got %d, want 1", len(body.Points))
		}
		if body.Points[0].ID != "test-id" {
			t.Errorf("id: got %q", body.Points[0].ID)
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"result": {"status": "completed"}}`))
	}))
	defer srv.Close()

	q := newQdrantClient(srv.URL, "test_cache", 5*time.Second)
	err := q.Upsert(context.Background(), "test-id", []float64{0.1, 0.2}, map[string]string{"redis_key": "cache:test-id"})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
}

func TestQdrantClient_Delete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method: got %s, want POST", r.Method)
		}
		if r.URL.Path != "/collections/test_cache/points/delete" {
			t.Errorf("path: got %s", r.URL.Path)
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"result": {"status": "completed"}}`))
	}))
	defer srv.Close()

	q := newQdrantClient(srv.URL, "test_cache", 5*time.Second)
	err := q.Delete(context.Background(), []string{"id-1", "id-2"})
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestQdrantClient_SearchError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer srv.Close()

	q := newQdrantClient(srv.URL, "test_cache", 5*time.Second)
	_, err := q.Search(context.Background(), []float64{0.1}, 0.95)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}
