package sweep

import "github.com/trnahnh/draft-thinker/benchmarks/internal/collect"

type Method string

const (
	MethodEntropyRouter      Method = "entropy_router"
	MethodConfidenceThreshold Method = "confidence_threshold"
	MethodAlwaysHeavyweight  Method = "always_heavyweight"
)

// EscalationPolicy decides, given a drafter's recorded token trace, whether
// the request would have been escalated to the heavyweight model.
type EscalationPolicy func(tokens []collect.TokenRecord) bool

// EntropyPolicy replays the windowed Shannon-entropy router used in production.
func EntropyPolicy(threshold float64, cfg ReplayConfig) EscalationPolicy {
	return func(tokens []collect.TokenRecord) bool {
		return Replay(tokens, threshold, cfg).WouldEscalate
	}
}

// ConfidencePolicy escalates when the single lowest top-1 token logprob in
// the drafter's response falls below threshold. No entropy, no window, no
// early exit: the whole response is generated first, then checked once.
func ConfidencePolicy(threshold float64) EscalationPolicy {
	return func(tokens []collect.TokenRecord) bool {
		if len(tokens) == 0 {
			return false
		}
		min := tokens[0].Logprob
		for _, t := range tokens {
			if t.Logprob < min {
				min = t.Logprob
			}
		}
		return min < threshold
	}
}

// AlwaysHeavyweightPolicy sends every request straight to the heavyweight
// model. This is the cost/accuracy ceiling both other methods are compared against.
func AlwaysHeavyweightPolicy() EscalationPolicy {
	return func(tokens []collect.TokenRecord) bool {
		return true
	}
}
