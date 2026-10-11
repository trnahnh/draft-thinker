package sweep

import "github.com/trnahnh/draft-thinker/benchmarks/internal/collect"

type Pricing struct {
	DrafterInputPer1M  float64
	DrafterOutputPer1M float64
	HeavyInputPer1M    float64
	HeavyOutputPer1M   float64
}

// DefaultPricing is OpenAI list pricing for gpt-4.1-nano (drafter) and
// gpt-4.1 (heavyweight) in USD per 1M tokens.
func DefaultPricing() Pricing {
	return Pricing{
		DrafterInputPer1M:  0.20,
		DrafterOutputPer1M: 0.80,
		HeavyInputPer1M:    2.50,
		HeavyOutputPer1M:   10.00,
	}
}

// OutputOnly returns a copy of the pricing with input tokens priced at
// zero. It reproduces the cost model used by the original calibration,
// which ignored prompt tokens entirely.
func (p Pricing) OutputOnly() Pricing {
	p.DrafterInputPer1M = 0
	p.HeavyInputPer1M = 0
	return p
}

func drafterCost(r *collect.Record, outputTokens int, p Pricing) float64 {
	in := float64(r.EstimatedPromptTokens()) * p.DrafterInputPer1M
	out := float64(outputTokens) * p.DrafterOutputPer1M
	return (in + out) / 1_000_000
}

func heavyCost(r *collect.Record, p Pricing) float64 {
	in := float64(r.EstimatedPromptTokens()) * p.HeavyInputPer1M
	out := float64(r.HeavyTokensUsed) * p.HeavyOutputPer1M
	return (in + out) / 1_000_000
}

// EstimateCost prices a request routed through the drafter. On escalation
// the drafter's full output is still charged: the production gateway
// aborts the drafter stream at the escalation point, but providers bill
// tokens already generated and the abort is not instantaneous, so the
// full draft is the conservative choice.
func EstimateCost(r *collect.Record, wouldEscalate bool, p Pricing) float64 {
	cost := drafterCost(r, r.DraftTokensUsed, p)
	if wouldEscalate {
		cost += heavyCost(r, p)
	}
	return cost
}

// BaselineCost is what the request costs when sent straight to the
// heavyweight model with no drafter call at all.
func BaselineCost(r *collect.Record, p Pricing) float64 {
	return heavyCost(r, p)
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
