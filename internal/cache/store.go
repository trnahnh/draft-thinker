package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/trnahnh/draft-thinker/internal/metrics"
	"github.com/trnahnh/draft-thinker/pkg/protocol"
)

type VectorStore struct {
	embedder  embedder
	index     vectorIndex
	kv        kvStore
	threshold float64
	recorder  metrics.Recorder
}

func NewVectorStore(emb embedder, idx vectorIndex, kv kvStore, threshold float64, rec metrics.Recorder) *VectorStore {
	return &VectorStore{
		embedder:  emb,
		index:     idx,
		kv:        kv,
		threshold: threshold,
		recorder:  rec,
	}
}

func buildKeyText(messages []protocol.Message) string {
	var b strings.Builder
	for _, msg := range messages {
		if msg.Role == "system" || msg.Role == "user" {
			b.WriteString(msg.Role)
			b.WriteString(": ")
			b.WriteString(msg.Content)
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (s *VectorStore) Lookup(ctx context.Context, messages []protocol.Message) (*protocol.ChatCompletionResponse, error) {
	start := time.Now()

	keyText := buildKeyText(messages)
	vector, err := s.embedder.Embed(ctx, keyText)
	if err != nil {
		return nil, fmt.Errorf("embedding for lookup: %w", err)
	}

	result, err := s.index.Search(ctx, vector, s.threshold)
	if err != nil {
		return nil, fmt.Errorf("vector search: %w", err)
	}

	if result == nil {
		s.recorder.RecordCacheMiss()
		s.recorder.RecordCacheLookupLatency(time.Since(start))
		return nil, nil
	}

	redisKey, ok := result.Payload["redis_key"]
	if !ok {
		s.recorder.RecordCacheMiss()
		s.recorder.RecordCacheLookupLatency(time.Since(start))
		return nil, nil
	}

	data, found, err := s.kv.Get(ctx, redisKey)
	if err != nil {
		return nil, fmt.Errorf("kv get: %w", err)
	}

	if !found {
		log.Printf("cache: lazy cleanup of orphaned qdrant point %s", result.ID)
		if delErr := s.index.Delete(ctx, []string{result.ID}); delErr != nil {
			log.Printf("cache: failed to delete orphaned point: %v", delErr)
		}
		s.recorder.RecordCacheMiss()
		s.recorder.RecordCacheLookupLatency(time.Since(start))
		return nil, nil
	}

	var resp protocol.ChatCompletionResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("unmarshaling cached response: %w", err)
	}

	s.recorder.RecordCacheHit()
	s.recorder.RecordCacheLookupLatency(time.Since(start))
	return &resp, nil
}

func (s *VectorStore) Insert(ctx context.Context, messages []protocol.Message, resp *protocol.ChatCompletionResponse) error {
	keyText := buildKeyText(messages)
	vector, err := s.embedder.Embed(ctx, keyText)
	if err != nil {
		return fmt.Errorf("embedding for insert: %w", err)
	}

	id := generateUUID()
	redisKey := "cache:" + id

	data, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("marshaling response for cache: %w", err)
	}

	if err := s.kv.Set(ctx, redisKey, data); err != nil {
		return fmt.Errorf("kv set: %w", err)
	}

	payload := map[string]string{"redis_key": redisKey}
	if err := s.index.Upsert(ctx, id, vector, payload); err != nil {
		s.kv.Del(ctx, redisKey)
		return fmt.Errorf("vector upsert: %w", err)
	}

	return nil
}

func (s *VectorStore) Evict(ctx context.Context, id string) error {
	if err := s.index.Delete(ctx, []string{id}); err != nil {
		return fmt.Errorf("vector delete: %w", err)
	}
	if err := s.kv.Del(ctx, "cache:"+id); err != nil {
		return fmt.Errorf("kv delete: %w", err)
	}
	return nil
}

func (s *VectorStore) Close() error {
	return s.kv.Close()
}
