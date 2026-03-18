package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/trnahnh/draft-thinker/internal/metrics"
	"github.com/trnahnh/draft-thinker/pkg/protocol"
)

type mockEmbedder struct {
	vector []float64
	err    error
}

func (m *mockEmbedder) Embed(_ context.Context, _ string) ([]float64, error) {
	return m.vector, m.err
}

type mockVectorIndex struct {
	searchResult *SearchResult
	searchErr    error
	upsertErr    error
	deleteErr    error
	ensureErr    error
	upserted     []qdrantPoint
	deleted      []string
}

func (m *mockVectorIndex) Search(_ context.Context, _ []float64, _ float64) (*SearchResult, error) {
	return m.searchResult, m.searchErr
}

func (m *mockVectorIndex) Upsert(_ context.Context, id string, vector []float64, payload map[string]string) error {
	m.upserted = append(m.upserted, qdrantPoint{ID: id, Vector: vector, Payload: payload})
	return m.upsertErr
}

func (m *mockVectorIndex) Delete(_ context.Context, ids []string) error {
	m.deleted = append(m.deleted, ids...)
	return m.deleteErr
}

func (m *mockVectorIndex) EnsureCollection(_ context.Context, _ int) error {
	return m.ensureErr
}

func testMessages() []protocol.Message {
	return []protocol.Message{
		{Role: "user", Content: "What is 2+2?"},
	}
}

func testResponse() *protocol.ChatCompletionResponse {
	return &protocol.ChatCompletionResponse{
		ID:      "resp-1",
		Object:  "chat.completion",
		Created: 1700000000,
		Model:   "gpt-4.1-nano",
		Choices: []protocol.Choice{
			{Index: 0, Message: &protocol.Message{Role: "assistant", Content: "4"}},
		},
	}
}

func TestVectorStore_LookupHit(t *testing.T) {
	resp := testResponse()
	respData, _ := json.Marshal(resp)

	kv := newMockKVStore()
	kv.Set(context.Background(), "cache:abc-123", respData)

	store := NewVectorStore(
		&mockEmbedder{vector: []float64{0.1, 0.2}},
		&mockVectorIndex{searchResult: &SearchResult{
			ID:      "abc-123",
			Score:   0.97,
			Payload: map[string]string{"redis_key": "cache:abc-123"},
		}},
		kv,
		0.95,
		&metrics.NoopRecorder{},
	)

	result, err := store.Lookup(context.Background(), testMessages())
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if result == nil {
		t.Fatal("expected cache hit")
	}
	if result.ID != "resp-1" {
		t.Errorf("id: got %q", result.ID)
	}
	if result.Choices[0].Message.Content != "4" {
		t.Errorf("content: got %q", result.Choices[0].Message.Content)
	}
}

func TestVectorStore_LookupMiss(t *testing.T) {
	store := NewVectorStore(
		&mockEmbedder{vector: []float64{0.1, 0.2}},
		&mockVectorIndex{searchResult: nil},
		newMockKVStore(),
		0.95,
		&metrics.NoopRecorder{},
	)

	result, err := store.Lookup(context.Background(), testMessages())
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if result != nil {
		t.Error("expected nil for cache miss")
	}
}

func TestVectorStore_LookupLazyCleanup(t *testing.T) {
	idx := &mockVectorIndex{
		searchResult: &SearchResult{
			ID:      "orphan-123",
			Score:   0.98,
			Payload: map[string]string{"redis_key": "cache:orphan-123"},
		},
	}

	store := NewVectorStore(
		&mockEmbedder{vector: []float64{0.1, 0.2}},
		idx,
		newMockKVStore(),
		0.95,
		&metrics.NoopRecorder{},
	)

	result, err := store.Lookup(context.Background(), testMessages())
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if result != nil {
		t.Error("expected nil for expired redis key")
	}
	if len(idx.deleted) != 1 || idx.deleted[0] != "orphan-123" {
		t.Errorf("expected lazy cleanup of orphaned point, got deleted=%v", idx.deleted)
	}
}

func TestVectorStore_Insert(t *testing.T) {
	kv := newMockKVStore()
	idx := &mockVectorIndex{}

	store := NewVectorStore(
		&mockEmbedder{vector: []float64{0.1, 0.2}},
		idx,
		kv,
		0.95,
		&metrics.NoopRecorder{},
	)

	err := store.Insert(context.Background(), testMessages(), testResponse())
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	if len(idx.upserted) != 1 {
		t.Fatalf("expected 1 upserted point, got %d", len(idx.upserted))
	}

	redisKey := idx.upserted[0].Payload["redis_key"]
	if redisKey == "" {
		t.Fatal("expected redis_key in payload")
	}

	data, ok, err := kv.Get(context.Background(), redisKey)
	if err != nil {
		t.Fatalf("get from kv: %v", err)
	}
	if !ok {
		t.Fatal("expected data in kv store")
	}

	var cached protocol.ChatCompletionResponse
	if err := json.Unmarshal(data, &cached); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cached.ID != "resp-1" {
		t.Errorf("cached id: got %q", cached.ID)
	}
}

func TestVectorStore_Evict(t *testing.T) {
	kv := newMockKVStore()
	kv.Set(context.Background(), "cache:evict-me", []byte("data"))

	idx := &mockVectorIndex{}

	store := NewVectorStore(
		&mockEmbedder{vector: []float64{0.1}},
		idx,
		kv,
		0.95,
		&metrics.NoopRecorder{},
	)

	err := store.Evict(context.Background(), "evict-me")
	if err != nil {
		t.Fatalf("evict: %v", err)
	}

	if len(idx.deleted) != 1 || idx.deleted[0] != "evict-me" {
		t.Errorf("expected vector delete, got %v", idx.deleted)
	}

	_, ok, _ := kv.Get(context.Background(), "cache:evict-me")
	if ok {
		t.Error("expected kv entry to be deleted")
	}
}

func TestVectorStore_LookupEmbedError(t *testing.T) {
	store := NewVectorStore(
		&mockEmbedder{err: fmt.Errorf("embed failed")},
		&mockVectorIndex{},
		newMockKVStore(),
		0.95,
		&metrics.NoopRecorder{},
	)

	_, err := store.Lookup(context.Background(), testMessages())
	if err == nil {
		t.Fatal("expected error from embed failure")
	}
}

func TestVectorStore_InsertEmbedError(t *testing.T) {
	store := NewVectorStore(
		&mockEmbedder{err: fmt.Errorf("embed failed")},
		&mockVectorIndex{},
		newMockKVStore(),
		0.95,
		&metrics.NoopRecorder{},
	)

	err := store.Insert(context.Background(), testMessages(), testResponse())
	if err == nil {
		t.Fatal("expected error from embed failure")
	}
}

func TestBuildKeyText(t *testing.T) {
	messages := []protocol.Message{
		{Role: "system", Content: "You are helpful"},
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there"},
		{Role: "user", Content: "How are you?"},
	}

	result := buildKeyText(messages)
	expected := "system: You are helpful\nuser: Hello\nuser: How are you?\n"
	if result != expected {
		t.Errorf("key text: got %q, want %q", result, expected)
	}
}
