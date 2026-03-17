package sweep

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
)

func WriteCSV(w io.Writer, stats []ThresholdStats) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()

	header := []string{
		"threshold", "escalation_rate", "draft_accuracy", "precision", "recall", "f1",
		"total", "escalated", "accepted", "tp", "fp", "tn", "fn",
		"estimated_cost", "baseline_cost", "cost_reduction",
	}
	if err := cw.Write(header); err != nil {
		return fmt.Errorf("writing CSV header: %w", err)
	}

	for _, s := range stats {
		row := []string{
			strconv.FormatFloat(s.Threshold, 'f', 2, 64),
			strconv.FormatFloat(s.EscalationRate, 'f', 4, 64),
			strconv.FormatFloat(s.DraftAccuracy, 'f', 4, 64),
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
			strconv.FormatFloat(s.EstimatedCost, 'f', 6, 64),
			strconv.FormatFloat(s.BaselineCostTotal, 'f', 6, 64),
			strconv.FormatFloat(s.CostReduction, 'f', 4, 64),
		}
		if err := cw.Write(row); err != nil {
			return fmt.Errorf("writing CSV row: %w", err)
		}
	}

	return nil
}

func WriteSummary(w io.Writer, stats []ThresholdStats) error {
	fmt.Fprintf(w, "%-10s | %-16s | %-15s | %-15s | %-6s\n",
		"Threshold", "Escalation Rate", "Draft Accuracy", "Cost Reduction", "F1")
	fmt.Fprintf(w, "%-10s-+-%-16s-+-%-15s-+-%-15s-+-%-6s\n",
		"----------", "----------------", "---------------", "---------------", "------")

	for _, s := range stats {
		fmt.Fprintf(w, "%-10.2f | %-16.1f | %-15.1f | %-15.1f | %-6.2f\n",
			s.Threshold,
			s.EscalationRate*100,
			s.DraftAccuracy*100,
			s.CostReduction*100,
			s.F1)
	}

	best, found := SelectBestThreshold(stats)
	fmt.Fprintln(w)
	if found {
		fmt.Fprintf(w, "Selected: T=%.2f (best F1 with draft accuracy > 95%%)\n", best.Threshold)
	} else {
		fmt.Fprintln(w, "No threshold found with draft accuracy > 95%")
	}

	return nil
}

func SelectBestThreshold(stats []ThresholdStats) (ThresholdStats, bool) {
	var best ThresholdStats
	found := false

	for _, s := range stats {
		if s.DraftAccuracy < 0.95 {
			continue
		}
		if !found || s.F1 > best.F1 {
			best = s
			found = true
		}
	}

	return best, found
}
