package sweep

import (
	"github.com/trnahnh/draft-thinker/benchmarks/internal/collect"
)

type CategoryStats struct {
	Total          int
	Escalated      int
	Accepted       int
	DraftAccuracy  float64
	EscalationRate float64
}

type ThresholdStats struct {
	Method         Method
	Threshold      float64
	EscalationRate float64
	DraftAccuracy  float64
	OverallAccuracy float64
	Precision      float64
	Recall         float64
	F1             float64
	TotalPrompts   int
	EscalatedCount int
	AcceptedCount  int
	TP             int
	FP             int
	TN             int
	FN             int
	EstimatedCost  float64
	BaselineCostTotal float64
	CostReduction  float64
	ByCategory     map[string]*CategoryStats
}

// ComputeStats replays the entropy router at the given threshold. It is kept
// as a thin wrapper around ComputeStatsWithPolicy for backward compatibility.
func ComputeStats(records []*collect.Record, threshold float64, replayCfg ReplayConfig, pricing Pricing) ThresholdStats {
	stats := ComputeStatsWithPolicy(records, MethodEntropyRouter, threshold, EntropyPolicy(threshold, replayCfg), pricing)
	return stats
}

// ComputeStatsWithPolicy computes routing/cost/accuracy stats for an arbitrary
// escalation policy, so entropy routing, confidence-threshold routing, and
// always-heavyweight can all be evaluated against the same recorded traces.
func ComputeStatsWithPolicy(records []*collect.Record, method Method, threshold float64, policy EscalationPolicy, pricing Pricing) ThresholdStats {
	stats := ThresholdStats{
		Method:     method,
		Threshold:  threshold,
		ByCategory: make(map[string]*CategoryStats),
	}

	totalEstimated := 0.0
	totalBaseline := 0.0

	for _, r := range records {
		if r.Error != "" {
			continue
		}

		stats.TotalPrompts++

		wouldEscalate := policy(r.Tokens)
		acceptable := r.Judge.Acceptable

		if wouldEscalate {
			stats.EscalatedCount++
		} else {
			stats.AcceptedCount++
		}

		switch {
		case !acceptable && wouldEscalate:
			stats.TP++
		case acceptable && wouldEscalate:
			stats.FP++
		case acceptable && !wouldEscalate:
			stats.TN++
		case !acceptable && !wouldEscalate:
			stats.FN++
		}

		totalEstimated += estimateCostForMethod(r, method, wouldEscalate, pricing)
		totalBaseline += BaselineCost(r, pricing)

		cat := r.Category
		cs, ok := stats.ByCategory[cat]
		if !ok {
			cs = &CategoryStats{}
			stats.ByCategory[cat] = cs
		}
		cs.Total++
		if wouldEscalate {
			cs.Escalated++
		} else {
			cs.Accepted++
			if acceptable {
				cs.DraftAccuracy++
			}
		}
	}

	if stats.TotalPrompts > 0 {
		stats.EscalationRate = float64(stats.EscalatedCount) / float64(stats.TotalPrompts)
	}

	accepted := stats.TN + stats.FN
	if accepted > 0 {
		stats.DraftAccuracy = float64(stats.TN) / float64(accepted)
	}

	// Overall accuracy = fraction of ALL served responses judged acceptable.
	// Escalated requests are treated as acceptable since they return the
	// heavyweight response, which is itself the judge's reference answer.
	// The only way to serve a bad response is FN: accepted-but-unacceptable.
	if stats.TotalPrompts > 0 {
		stats.OverallAccuracy = 1 - float64(stats.FN)/float64(stats.TotalPrompts)
	}

	if stats.TP+stats.FP > 0 {
		stats.Precision = float64(stats.TP) / float64(stats.TP+stats.FP)
	}
	if stats.TP+stats.FN > 0 {
		stats.Recall = float64(stats.TP) / float64(stats.TP+stats.FN)
	}
	if stats.Precision+stats.Recall > 0 {
		stats.F1 = 2 * stats.Precision * stats.Recall / (stats.Precision + stats.Recall)
	}

	stats.EstimatedCost = totalEstimated
	stats.BaselineCostTotal = totalBaseline
	if totalBaseline > 0 {
		stats.CostReduction = 1 - totalEstimated/totalBaseline
	}

	for _, cs := range stats.ByCategory {
		if cs.Accepted > 0 {
			cs.DraftAccuracy = cs.DraftAccuracy / float64(cs.Accepted)
		}
		if cs.Total > 0 {
			cs.EscalationRate = float64(cs.Escalated) / float64(cs.Total)
		}
	}

	return stats
}
