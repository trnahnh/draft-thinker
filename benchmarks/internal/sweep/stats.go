package sweep

import (
	"sort"

	"github.com/trnahnh/draft-thinker/benchmarks/internal/collect"
)

type CategoryStats struct {
	Total           int
	Escalated       int
	Accepted        int
	Served          int
	ServedCorrect   int
	DraftAccuracy   float64
	OverallAccuracy float64
	EscalationRate  float64
	EstimatedCost   float64
	BaselineCost    float64
	CostReduction   float64
}

type ThresholdStats struct {
	Method            Method
	Threshold         float64
	EscalationRate    float64
	DraftAccuracy     float64
	OverallAccuracy   float64
	Precision         float64
	Recall            float64
	F1                float64
	TotalPrompts      int
	EscalatedCount    int
	AcceptedCount     int
	TP                int
	FP                int
	TN                int
	FN                int
	ServedWrong       int
	EstimatedCost     float64
	BaselineCostTotal float64
	CostReduction     float64
	CostPer1K         float64
	BaselineCostPer1K float64
	LatencyP50MS      float64
	LatencyP99MS      float64
	ByCategory        map[string]*CategoryStats
}

// ComputeStats replays the entropy router at the given threshold. It is kept
// as a thin wrapper around ComputeStatsWithPolicy for backward compatibility.
func ComputeStats(records []*collect.Record, threshold float64, replayCfg ReplayConfig, pricing Pricing) ThresholdStats {
	return ComputeStatsWithPolicy(records, MethodEntropyRouter, threshold, EntropyPolicy(threshold, replayCfg), pricing)
}

// servedCorrect reports whether the response the user actually received
// was correct. With an LLM judge the heavyweight response is the reference
// answer and is treated as correct by construction. With ground-truth
// grading both models are scored against the same answer key, so an
// escalation to a heavyweight that also gets it wrong still counts as a
// wrong answer served.
func servedCorrect(r *collect.Record, escalated bool) bool {
	if r.HasGroundTruth() {
		if escalated {
			return *r.HeavyCorrect
		}
		return *r.DraftCorrect
	}
	if escalated {
		return true
	}
	return r.Judge.Acceptable
}

// servedLatencyMS estimates end-to-end latency for the serial gateway
// path from the recorded per-call latencies. An accepted draft costs the
// full drafter call. An escalation costs the fraction of the drafter call
// spent before the decision plus the full heavyweight call. The
// always-heavyweight baseline pays only the heavyweight call.
func servedLatencyMS(r *collect.Record, method Method, d Decision) float64 {
	if method == MethodAlwaysHeavyweight {
		return float64(r.HeavyLatencyMS)
	}
	draft := float64(r.DraftLatencyMS)
	if !d.Escalate {
		return draft
	}
	frac := 1.0
	if r.TokenCount > 0 && d.TokensConsumed > 0 && d.TokensConsumed < r.TokenCount {
		frac = float64(d.TokensConsumed) / float64(r.TokenCount)
	}
	return draft*frac + float64(r.HeavyLatencyMS)
}

// ComputeStatsWithPolicy computes routing/cost/accuracy stats for an arbitrary
// escalation policy, so every method can be evaluated against the same
// recorded traces.
func ComputeStatsWithPolicy(records []*collect.Record, method Method, threshold float64, policy EscalationPolicy, pricing Pricing) ThresholdStats {
	stats := ThresholdStats{
		Method:     method,
		Threshold:  threshold,
		ByCategory: make(map[string]*CategoryStats),
	}

	totalEstimated := 0.0
	totalBaseline := 0.0
	latencies := make([]float64, 0, len(records))

	for _, r := range records {
		if r.Error != "" {
			continue
		}

		stats.TotalPrompts++

		decision := policy(r)
		wouldEscalate := decision.Escalate
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

		correct := servedCorrect(r, wouldEscalate)
		if !correct {
			stats.ServedWrong++
		}

		est := estimateCostForMethod(r, method, wouldEscalate, pricing)
		base := BaselineCost(r, pricing)
		totalEstimated += est
		totalBaseline += base
		latencies = append(latencies, servedLatencyMS(r, method, decision))

		cs, ok := stats.ByCategory[r.Category]
		if !ok {
			cs = &CategoryStats{}
			stats.ByCategory[r.Category] = cs
		}
		cs.Total++
		cs.Served++
		cs.EstimatedCost += est
		cs.BaselineCost += base
		if correct {
			cs.ServedCorrect++
		}
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
		stats.OverallAccuracy = 1 - float64(stats.ServedWrong)/float64(stats.TotalPrompts)
		stats.CostPer1K = totalEstimated / float64(stats.TotalPrompts) * 1000
		stats.BaselineCostPer1K = totalBaseline / float64(stats.TotalPrompts) * 1000
	}

	accepted := stats.TN + stats.FN
	if accepted > 0 {
		stats.DraftAccuracy = float64(stats.TN) / float64(accepted)
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

	if len(latencies) > 0 {
		sort.Float64s(latencies)
		stats.LatencyP50MS = percentile(latencies, 50)
		stats.LatencyP99MS = percentile(latencies, 99)
	}

	for _, cs := range stats.ByCategory {
		if cs.Accepted > 0 {
			cs.DraftAccuracy = cs.DraftAccuracy / float64(cs.Accepted)
		}
		if cs.Total > 0 {
			cs.EscalationRate = float64(cs.Escalated) / float64(cs.Total)
			cs.OverallAccuracy = float64(cs.ServedCorrect) / float64(cs.Total)
		}
		if cs.BaselineCost > 0 {
			cs.CostReduction = 1 - cs.EstimatedCost/cs.BaselineCost
		}
	}

	return stats
}
