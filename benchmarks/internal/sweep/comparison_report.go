package sweep

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
)

// WriteComparisonCSV writes stats from multiple methods (entropy router,
// confidence threshold, always-heavyweight) into a single CSV with a method
// column so they can be plotted and compared directly.
func WriteComparisonCSV(w io.Writer, stats []ThresholdStats) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()

	header := []string{
		"method", "threshold", "escalation_rate", "draft_accuracy", "overall_accuracy",
		"precision", "recall", "f1",
		"total", "escalated", "accepted", "tp", "fp", "tn", "fn",
		"estimated_cost", "baseline_cost", "cost_reduction",
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

func WriteComparisonSummary(w io.Writer, stats []ThresholdStats) error {
	fmt.Fprintf(w, "%-22s | %-9s | %-16s | %-16s | %-15s\n",
		"Method", "Threshold", "Escalation Rate", "Overall Accuracy", "Cost Reduction")
	fmt.Fprintf(w, "%-22s-+-%-9s-+-%-16s-+-%-16s-+-%-15s\n",
		"----------------------", "---------", "----------------", "----------------", "---------------")

	for _, s := range stats {
		fmt.Fprintf(w, "%-22s | %-9.2f | %-16.1f | %-16.1f | %-15.1f\n",
			s.Method, s.Threshold, s.EscalationRate*100, s.OverallAccuracy*100, s.CostReduction*100)
	}

	return nil
}
