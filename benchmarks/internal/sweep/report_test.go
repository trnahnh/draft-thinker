package sweep

import (
	"bytes"
	"encoding/csv"
	"strings"
	"testing"
)

func TestWriteCSV(t *testing.T) {
	stats := []ThresholdStats{
		{
			Threshold:      0.5,
			EscalationRate: 0.78,
			DraftAccuracy:  0.99,
			Precision:      0.85,
			Recall:         0.90,
			F1:             0.87,
			TotalPrompts:   100,
			EscalatedCount: 78,
			AcceptedCount:  22,
			TP:             20,
			FP:             58,
			TN:             21,
			FN:             1,
			EstimatedCost:  0.05,
			BaselineCostTotal: 0.10,
			CostReduction:  0.50,
		},
	}

	var buf bytes.Buffer
	if err := WriteCSV(&buf, stats); err != nil {
		t.Fatalf("WriteCSV: %v", err)
	}

	reader := csv.NewReader(&buf)
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("reading CSV: %v", err)
	}

	if len(records) != 2 {
		t.Fatalf("expected 2 rows (header + 1 data), got %d", len(records))
	}

	header := records[0]
	if header[0] != "threshold" {
		t.Errorf("expected first column 'threshold', got %q", header[0])
	}

	if records[1][0] != "0.50" {
		t.Errorf("expected threshold 0.50, got %s", records[1][0])
	}
}

func TestWriteSummary(t *testing.T) {
	stats := []ThresholdStats{
		{Threshold: 0.5, EscalationRate: 0.78, DraftAccuracy: 0.99, CostReduction: 0.22, F1: 0.92},
		{Threshold: 1.0, EscalationRate: 0.35, DraftAccuracy: 0.96, CostReduction: 0.65, F1: 0.83},
		{Threshold: 1.5, EscalationRate: 0.12, DraftAccuracy: 0.87, CostReduction: 0.88, F1: 0.65},
	}

	var buf bytes.Buffer
	if err := WriteSummary(&buf, stats); err != nil {
		t.Fatalf("WriteSummary: %v", err)
	}

	output := buf.String()

	if !strings.Contains(output, "Threshold") {
		t.Error("summary missing header")
	}
	if !strings.Contains(output, "0.50") {
		t.Error("summary missing threshold 0.50")
	}
	if !strings.Contains(output, "Selected: T=0.50") {
		t.Errorf("expected selected T=0.50 (best F1 among accuracy>=95%%), output:\n%s", output)
	}
}

func TestSelectBestThreshold(t *testing.T) {
	stats := []ThresholdStats{
		{Threshold: 0.5, DraftAccuracy: 0.99, F1: 0.70},
		{Threshold: 1.0, DraftAccuracy: 0.96, F1: 0.85},
		{Threshold: 1.5, DraftAccuracy: 0.87, F1: 0.90},
	}

	best, found := SelectBestThreshold(stats)
	if !found {
		t.Fatal("expected to find a best threshold")
	}
	if best.Threshold != 1.0 {
		t.Errorf("expected threshold 1.0, got %f", best.Threshold)
	}
}

func TestSelectBestThresholdNoneQualify(t *testing.T) {
	stats := []ThresholdStats{
		{Threshold: 1.5, DraftAccuracy: 0.80, F1: 0.90},
		{Threshold: 2.0, DraftAccuracy: 0.70, F1: 0.85},
	}

	_, found := SelectBestThreshold(stats)
	if found {
		t.Error("expected no threshold to qualify")
	}
}

func TestWriteCSVRoundTrip(t *testing.T) {
	stats := []ThresholdStats{
		{Threshold: 0.75, EscalationRate: 0.5, DraftAccuracy: 0.97, F1: 0.80,
			TotalPrompts: 50, EscalatedCount: 25, AcceptedCount: 25,
			TP: 10, FP: 15, TN: 24, FN: 1,
			EstimatedCost: 0.03, BaselineCostTotal: 0.06, CostReduction: 0.50, Precision: 0.4, Recall: 0.91},
		{Threshold: 1.25, EscalationRate: 0.3, DraftAccuracy: 0.95, F1: 0.75,
			TotalPrompts: 50, EscalatedCount: 15, AcceptedCount: 35,
			TP: 8, FP: 7, TN: 33, FN: 2,
			EstimatedCost: 0.02, BaselineCostTotal: 0.06, CostReduction: 0.67, Precision: 0.53, Recall: 0.80},
	}

	var buf bytes.Buffer
	if err := WriteCSV(&buf, stats); err != nil {
		t.Fatalf("WriteCSV: %v", err)
	}

	reader := csv.NewReader(&buf)
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("reading: %v", err)
	}

	if len(records) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(records))
	}

	if len(records[0]) != 16 {
		t.Errorf("expected 16 columns, got %d", len(records[0]))
	}
}
