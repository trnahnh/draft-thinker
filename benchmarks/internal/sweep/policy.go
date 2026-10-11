package sweep

import (
	"hash/fnv"
	"math/rand"

	"github.com/trnahnh/draft-thinker/benchmarks/internal/collect"
)

type Method string

const (
	MethodEntropyRouter       Method = "entropy_router"
	MethodConfidenceThreshold Method = "confidence_threshold"
	MethodAlwaysHeavyweight   Method = "always_heavyweight"
	MethodAlwaysDrafter       Method = "always_drafter"
	MethodRandom              Method = "random_matched_rate"
	MethodOracle              Method = "oracle"
	MethodMeanEntropy         Method = "mean_entropy_router"
)

// Decision is the outcome of replaying a policy over a drafter trace.
// TokensConsumed is how many drafter tokens were generated before the
// policy could decide, which drives the latency and cost estimates.
type Decision struct {
	Escalate       bool
	TokensConsumed int
}

// EscalationPolicy decides, given a full record of the drafter's run,
// whether the request would have been escalated to the heavyweight model.
type EscalationPolicy func(r *collect.Record) Decision

// EntropyPolicy replays the windowed Shannon-entropy router used in production.
func EntropyPolicy(threshold float64, cfg ReplayConfig) EscalationPolicy {
	return func(r *collect.Record) Decision {
		res := Replay(r.Tokens, threshold, cfg)
		return Decision{Escalate: res.WouldEscalate, TokensConsumed: res.TokensConsumed}
	}
}

// ConfidencePolicy escalates when the single lowest sampled-token logprob
// in the drafter's response falls below threshold. No entropy, no window,
// no early exit: the whole response is generated first, then checked once.
func ConfidencePolicy(threshold float64) EscalationPolicy {
	return func(r *collect.Record) Decision {
		tokens := r.Tokens
		if len(tokens) == 0 {
			return Decision{}
		}
		minLP := tokens[0].Logprob
		for _, t := range tokens {
			if t.Logprob < minLP {
				minLP = t.Logprob
			}
		}
		return Decision{Escalate: minLP < threshold, TokensConsumed: len(tokens)}
	}
}

// AlwaysHeavyweightPolicy sends every request straight to the heavyweight
// model. This is the cost ceiling the other methods are compared against.
func AlwaysHeavyweightPolicy() EscalationPolicy {
	return func(r *collect.Record) Decision {
		return Decision{Escalate: true}
	}
}

// AlwaysDrafterPolicy never escalates. It is the cost floor: any router
// must beat its accuracy to justify the heavyweight spend it adds.
func AlwaysDrafterPolicy() EscalationPolicy {
	return func(r *collect.Record) Decision {
		return Decision{Escalate: false, TokensConsumed: len(r.Tokens)}
	}
}

// RandomPolicy escalates a fixed fraction of requests chosen without
// looking at the trace. Matching its rate to the entropy router's isolates
// whether the entropy signal selects worse-than-average drafts or merely
// spends heavyweight budget at random. The choice is a deterministic hash
// of the prompt ID so results are reproducible and stable under bootstrap
// resampling.
func RandomPolicy(rate float64, seed int64) EscalationPolicy {
	return func(r *collect.Record) Decision {
		h := fnv.New64a()
		h.Write([]byte(r.PromptID))
		rng := rand.New(rand.NewSource(int64(h.Sum64()) ^ seed))
		return Decision{Escalate: rng.Float64() < rate, TokensConsumed: len(r.Tokens)}
	}
}

// OraclePolicy escalates exactly the drafts that were judged unacceptable.
// It is unattainable in production (it needs the label) but bounds the
// best cost reduction any router could reach at 100% draft accuracy.
func OraclePolicy() EscalationPolicy {
	return func(r *collect.Record) Decision {
		return Decision{Escalate: !r.Judge.Acceptable, TokensConsumed: len(r.Tokens)}
	}
}

// MeanEntropyPolicy escalates when the trace-wide mean per-token entropy
// exceeds threshold. Unlike the windowed router it needs the complete
// draft before deciding, so it cannot early-exit or speculate, but it is
// insensitive to a single uncertain window.
func MeanEntropyPolicy(threshold float64) EscalationPolicy {
	return func(r *collect.Record) Decision {
		return Decision{Escalate: r.MeanEntropy > threshold, TokensConsumed: len(r.Tokens)}
	}
}

// ScoreThresholdPolicy turns any ScoreFunc into a policy that escalates
// when score > threshold after reading the whole draft.
func ScoreThresholdPolicy(score ScoreFunc, threshold float64) EscalationPolicy {
	return func(r *collect.Record) Decision {
		return Decision{Escalate: score(r) > threshold, TokensConsumed: len(r.Tokens)}
	}
}
