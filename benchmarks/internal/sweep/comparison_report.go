package sweep

import (
	"encoding/csv"
	"fmt"
	"io"
	"sort"
	"strconv"
)

// WriteComparisonCSV writes stats from multiple methods into a single CSV
// with a method column so they can be plotted and compared directly.
func WriteComparisonCSV(w io.Writer, stats []ThresholdStats) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()

	header := []string{
		"method", "threshold", "escalation_rate", "draft_accuracy", "overall_accuracy",
		"precision", "recall", "f1",
		"total", "escalated", "accepted", "tp", "fp", "tn", "fn", "served_wrong",
		"estimated_cost", "baseline_cost", "cost_reduction",
		"cost_per_1k_usd", "baseline_cost_per_1k_usd",
		"latency_p50_ms", "latency_p99_ms",
	}
	if err := cw.Write(header); err != nil {
		return fmt.Errorf("writing CSV header: %w", err)
	}

	for _, s := range stats {
		row := []string{
			string(s.Method),
			strconv.FormatFloat(s.Threshold, 'f', 4, 64),
			strconv.FormatFloat(s.EscalationRate, 'f', 4, 64),
			strconv.FormatFloat(s.DraftAccuracy, 'f', 4, 64),
			strconv.FormatFloat(s.OverallAccuracy, 'f', 4, 64),
			strconv.FormatFloat(s.Precision, 'f', 4, 64),
			strconv.FormatFloat(s.Recall, 'f', 4, 64),
			strconv.FormatFloat(s.F1, 'f', 4, 64),
			strconv.Itoa(s.TotalPrompts),
			strconv.Itoa(s.EscalatedCount),
			strconv.Itoa(s.AcceptedCount),
			strconv.Itoa(s.TP),
			strconv.Itoa(s.FP),
			strconv.Itoa(s.TN),
			strconv.Itoa(s.FN),
			strconv.Itoa(s.ServedWrong),
			strconv.FormatFloat(s.EstimatedCost, 'f', 6, 64),
			strconv.FormatFloat(s.BaselineCostTotal, 'f', 6, 64),
			strconv.FormatFloat(s.CostReduction, 'f', 4, 64),
			strconv.FormatFloat(s.CostPer1K, 'f', 4, 64),
			strconv.FormatFloat(s.BaselineCostPer1K, 'f', 4, 64),
			strconv.FormatFloat(s.LatencyP50MS, 'f', 0, 64),
			strconv.FormatFloat(s.LatencyP99MS, 'f', 0, 64),
		}
		if err := cw.Write(row); err != nil {
			return fmt.Errorf("writing CSV row: %w", err)
		}
	}

	return nil
}

// WriteCategoryCSV writes one row per (method, threshold, category).
func WriteCategoryCSV(w io.Writer, stats []ThresholdStats) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()

	header := []string{"method", "threshold", "category", "total", "escalated", "escalation_rate", "draft_accuracy", "overall_accuracy", "cost_reduction"}
	if err := cw.Write(header); err != nil {
		return fmt.Errorf("writing CSV header: %w", err)
	}

	for _, s := range stats {
		cats := make([]string, 0, len(s.ByCategory))
		for c := range s.ByCategory {
			cats = append(cats, c)
		}
		sort.Strings(cats)
		for _, c := range cats {
			cs := s.ByCategory[c]
			row := []string{
				string(s.Method),
				strconv.FormatFloat(s.Threshold, 'f', 4, 64),
				c,
				strconv.Itoa(cs.Total),
				strconv.Itoa(cs.Escalated),
				strconv.FormatFloat(cs.EscalationRate, 'f', 4, 64),
				strconv.FormatFloat(cs.DraftAccuracy, 'f', 4, 64),
				strconv.FormatFloat(cs.OverallAccuracy, 'f', 4, 64),
				strconv.FormatFloat(cs.CostReduction, 'f', 4, 64),
			}
			if err := cw.Write(row); err != nil {
				return fmt.Errorf("writing CSV row: %w", err)
			}
		}
	}
	return nil
}

func WriteComparisonSummary(w io.Writer, stats []ThresholdStats) error {
	fmt.Fprintf(w, "%-22s | %-9s | %-10s | %-10s | %-10s | %-10s | %-9s | %-9s\n",
		"Method", "Threshold", "Escalate %", "Accuracy %", "Cost Red %", "USD/1k req", "p50 ms", "p99 ms")
	fmt.Fprintf(w, "%-22s-+-%-9s-+-%-10s-+-%-10s-+-%-10s-+-%-10s-+-%-9s-+-%-9s\n",
		"----------------------", "---------", "----------", "----------", "----------", "----------", "---------", "---------")

	for _, s := range stats {
		fmt.Fprintf(w, "%-22s | %-9.2f | %-10.1f | %-10.1f | %-10.1f | %-10.3f | %-9.0f | %-9.0f\n",
			s.Method, s.Threshold, s.EscalationRate*100, s.OverallAccuracy*100, s.CostReduction*100,
			s.CostPer1K, s.LatencyP50MS, s.LatencyP99MS)
	}

	return nil
}
