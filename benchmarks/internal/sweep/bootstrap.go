package sweep

import (
	"math/rand"
	"sort"

	"github.com/trnahnh/draft-thinker/benchmarks/internal/collect"
)

type BootstrapResult struct {
	Method             Method
	Threshold          float64
	Iterations         int
	CostReductionPoint float64
	CostReductionLower float64
	CostReductionUpper float64
	AccuracyPoint      float64
	AccuracyLower      float64
	AccuracyUpper      float64
}

// Bootstrap resamples records with replacement to put a 95% confidence
// interval on cost reduction and overall accuracy. The point estimate comes
// from the full (unresampled) dataset; the interval comes from the
// resampled distribution.
func Bootstrap(records []*collect.Record, method Method, threshold float64, policy EscalationPolicy, pricing Pricing, iterations int, seed int64) BootstrapResult {
	point := ComputeStatsWithPolicy(records, method, threshold, policy, pricing)

	rng := rand.New(rand.NewSource(seed))
	n := len(records)
	costs := make([]float64, iterations)
	accs := make([]float64, iterations)

	sample := make([]*collect.Record, n)
	for i := range iterations {
		for j := range n {
			sample[j] = records[rng.Intn(n)]
		}
		stats := ComputeStatsWithPolicy(sample, method, threshold, policy, pricing)
		costs[i] = stats.CostReduction
		accs[i] = stats.OverallAccuracy
	}

	sort.Float64s(costs)
	sort.Float64s(accs)

	return BootstrapResult{
		Method:             method,
		Threshold:          threshold,
		Iterations:         iterations,
		CostReductionPoint: point.CostReduction,
		CostReductionLower: percentile(costs, 2.5),
		CostReductionUpper: percentile(costs, 97.5),
		AccuracyPoint:      point.OverallAccuracy,
		AccuracyLower:      percentile(accs, 2.5),
		AccuracyUpper:      percentile(accs, 97.5),
	}
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := max(int(p/100*float64(len(sorted)-1)), 0)
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}
