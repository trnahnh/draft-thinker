package router

import (
	"context"
	"fmt"

	"github.com/trnahnh/draft-thinker/internal/entropy"
	"github.com/trnahnh/draft-thinker/internal/metrics"
	"github.com/trnahnh/draft-thinker/pkg/client"
	"github.com/trnahnh/draft-thinker/pkg/protocol"
)

type RoutingResult struct {
	Decision    entropy.Decision
	DraftChunks []protocol.StreamChunk
	TokenCount  int
	Entropies   []entropy.TokenEntropy
}

type Router struct {
	windowCfg entropy.WindowConfig
	recorder  metrics.Recorder
}

func NewRouter(cfg entropy.WindowConfig, rec metrics.Recorder) *Router {
	return &Router{
		windowCfg: cfg,
		recorder:  rec,
	}
}

func (r *Router) Route(ctx context.Context, chunks <-chan client.StreamChunk) (*RoutingResult, error) {
	window := entropy.NewWindow(r.windowCfg)

	result := &RoutingResult{
		Decision: entropy.Continue,
	}

	tokenIndex := 0

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case sc, ok := <-chunks:
			if !ok {
				result.Decision = entropy.Continue
				return result, nil
			}

			if sc.Err != nil {
				return nil, fmt.Errorf("drafter stream error: %w", sc.Err)
			}

			if sc.Chunk == nil {
				continue
			}

			result.DraftChunks = append(result.DraftChunks, *sc.Chunk)

			tokenEntropies := entropy.ExtractTokenEntropy(sc.Chunk, tokenIndex)
			for _, te := range tokenEntropies {
				r.recorder.RecordEntropy(te.Entropy)
				result.Entropies = append(result.Entropies, te)
				result.TokenCount++
				tokenIndex++

				decision := window.Add(te.Entropy)
				if decision == entropy.Escalate {
					result.Decision = entropy.Escalate
					go func() {
						for range chunks {
						}
					}()
					return result, nil
				}
			}

		}
	}
}
