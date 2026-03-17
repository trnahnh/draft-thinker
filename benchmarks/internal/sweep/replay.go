package sweep

import (
	"github.com/trnahnh/draft-thinker/benchmarks/internal/collect"
	"github.com/trnahnh/draft-thinker/internal/entropy"
)

type ReplayConfig struct {
	WindowSize     int
	EarlyExitCount int
}

type ReplayResult struct {
	WouldEscalate  bool
	EscalateAtToken int
	TokensConsumed int
}

func Replay(tokens []collect.TokenRecord, threshold float64, cfg ReplayConfig) ReplayResult {
	window := entropy.NewWindow(entropy.WindowConfig{
		Size:           cfg.WindowSize,
		Threshold:      threshold,
		EarlyExitCount: cfg.EarlyExitCount,
	})

	for i, t := range tokens {
		decision := window.Add(t.Entropy)
		if decision == entropy.Escalate {
			return ReplayResult{
				WouldEscalate:   true,
				EscalateAtToken: i,
				TokensConsumed:  i + 1,
			}
		}
	}

	return ReplayResult{
		WouldEscalate:  false,
		TokensConsumed: len(tokens),
	}
}
