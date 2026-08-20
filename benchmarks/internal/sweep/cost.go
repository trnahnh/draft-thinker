package sweep

import "github.com/trnahnh/draft-thinker/benchmarks/internal/collect"

type Pricing struct {
	DrafterInputPer1M  float64
	DrafterOutputPer1M float64
	HeavyInputPer1M    float64
	HeavyOutputPer1M   float64
}

func DefaultPricing() Pricing {
	return Pricing{
		DrafterInputPer1M:  0.20,
		DrafterOutputPer1M: 0.80,
		HeavyInputPer1M:    2.50,
		HeavyOutputPer1M:   10.00,
	}
}

func EstimateCost(r *collect.Record, wouldEscalate bool, p Pricing) float64 {
	drafterCost := float64(r.DraftTokensUsed) * p.DrafterOutputPer1M / 1_000_000

	if wouldEscalate {
		heavyCost := float64(r.HeavyTokensUsed) * p.HeavyOutputPer1M / 1_000_000
		return drafterCost + heavyCost
	}

	return drafterCost
}

func BaselineCost(r *collect.Record, p Pricing) float64 {
	return float64(r.HeavyTokensUsed) * p.HeavyOutputPer1M / 1_000_000
}

// estimateCostForMethod accounts for the fact that entropy and confidence
// routing must run the drafter to obtain their signal before deciding
// whether to escalate, while always-heavyweight never calls the drafter at
// all and so pays only the heavyweight cost.
func estimateCostForMethod(r *collect.Record, method Method, wouldEscalate bool, p Pricing) float64 {
	if method == MethodAlwaysHeavyweight {
		return BaselineCost(r, p)
	}
	return EstimateCost(r, wouldEscalate, p)
}
