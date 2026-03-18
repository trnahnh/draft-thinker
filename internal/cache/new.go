package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/trnahnh/draft-thinker/internal/metrics"
	"github.com/trnahnh/draft-thinker/pkg/client"
)

func New(
	openaiBaseURL, apiKey, embeddingModel string,
	embeddingDimensions int,
	qdrantURL, qdrantCollection string,
	redisAddr string,
	ttlSeconds int,
	similarityThreshold float64,
	rec metrics.Recorder,
) (*VectorStore, error) {
	emb := client.NewEmbeddingClient(openaiBaseURL, apiKey, embeddingModel, embeddingDimensions, 10*time.Second)

	idx := newQdrantClient(qdrantURL, qdrantCollection, 10*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := idx.EnsureCollection(ctx, embeddingDimensions); err != nil {
		return nil, fmt.Errorf("ensuring qdrant collection: %w", err)
	}

	kv, err := newRedisClient(redisAddr, ttlSeconds)
	if err != nil {
		return nil, fmt.Errorf("connecting to redis: %w", err)
	}

	return NewVectorStore(emb, idx, kv, similarityThreshold, rec), nil
}
