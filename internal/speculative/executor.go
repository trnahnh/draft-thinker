package speculative

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/trnahnh/draft-thinker/internal/entropy"
	"github.com/trnahnh/draft-thinker/internal/metrics"
	"github.com/trnahnh/draft-thinker/pkg/client"
	"github.com/trnahnh/draft-thinker/pkg/protocol"
)

type ExecutorConfig struct {
	WindowCfg         entropy.WindowConfig
	SoftThresholdMult float64
}

type Executor struct {
	cfg         ExecutorConfig
	heavyweight client.LLMClient
	recorder    metrics.Recorder
}

func NewExecutor(cfg ExecutorConfig, hw client.LLMClient, rec metrics.Recorder) *Executor {
	return &Executor{
		cfg:         cfg,
		heavyweight: hw,
		recorder:    rec,
	}
}

func (e *Executor) Execute(ctx context.Context, drafterCh <-chan client.StreamChunk, req *protocol.ChatCompletionRequest) (*Result, error) {
	window := entropy.NewWindow(e.cfg.WindowCfg)
	softThreshold := e.cfg.WindowCfg.Threshold * e.cfg.SoftThresholdMult

	result := &Result{
		State: Drafting,
	}

	var (
		heavyCh     <-chan client.StreamChunk
		heavyCancel context.CancelFunc
		softTriggered bool
		specStart   time.Time
		tokenIndex  int
	)

	for {
		select {
		case <-ctx.Done():
			if heavyCancel != nil {
				heavyCancel()
				drainChannel(heavyCh)
			}
			return nil, ctx.Err()

		case sc, ok := <-drafterCh:
			if !ok {
				if softTriggered && heavyCancel != nil {
					e.recorder.RecordSpeculativeCancellation()
					heavyCancel()
					drainChannel(heavyCh)
				}
				result.State = DraftAccepted
				return result, nil
			}

			if sc.Err != nil {
				if heavyCancel != nil {
					heavyCancel()
					drainChannel(heavyCh)
				}
				return nil, fmt.Errorf("drafter stream error: %w", sc.Err)
			}

			if sc.Chunk == nil {
				continue
			}

			result.DraftChunks = append(result.DraftChunks, *sc.Chunk)

			tokenEntropies := entropy.ExtractTokenEntropy(sc.Chunk, tokenIndex)
			for _, te := range tokenEntropies {
				e.recorder.RecordEntropy(te.Entropy)
				result.Entropies = append(result.Entropies, te)
				result.TokenCount++
				tokenIndex++

				decision := window.Add(te.Entropy)
				if decision == entropy.Escalate {
					result.State = Escalated
					if softTriggered && heavyCh != nil {
						result.HeavyCh = heavyCh
						result.HeavyCancel = heavyCancel
						e.recorder.RecordSpeculativeLatencySaved(time.Since(specStart))
					}
					go func() {
						for range drafterCh {
						}
					}()
					return result, nil
				}

				if !softTriggered && te.Entropy > softThreshold {
					softTriggered = true
					specStart = time.Now()
					e.recorder.RecordSpeculativeTrigger()

					hvCtx, cancel := context.WithCancel(ctx)
					ch, err := e.heavyweight.Stream(hvCtx, req)
					if err != nil {
						log.Printf("speculative heavyweight start failed: %v", err)
						cancel()
						softTriggered = false
						continue
					}
					heavyCh = ch
					heavyCancel = cancel
					result.State = Speculating
				}
			}
		}
	}
}

func drainChannel(ch <-chan client.StreamChunk) {
	if ch == nil {
		return
	}
	go func() {
		for range ch {
		}
	}()
}
