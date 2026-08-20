package sweep

import "testing"

func TestBootstrapIntervalContainsPointEstimate(t *testing.T) {
	records := makeTestRecords()
	pricing := DefaultPricing()
	cfg := ReplayConfig{WindowSize: 10, EarlyExitCount: 10}

	result := Bootstrap(records, MethodEntropyRouter, 1.0, EntropyPolicy(1.0, cfg), pricing, 500, 42)

	if result.CostReductionLower > result.CostReductionPoint || result.CostReductionPoint > result.CostReductionUpper {
		t.Errorf("point estimate %f not within [%f, %f]", result.CostReductionPoint, result.CostReductionLower, result.CostReductionUpper)
	}
	if result.AccuracyLower > result.AccuracyPoint || result.AccuracyPoint > result.AccuracyUpper {
		t.Errorf("accuracy point estimate %f not within [%f, %f]", result.AccuracyPoint, result.AccuracyLower, result.AccuracyUpper)
	}
	if result.Iterations != 500 {
		t.Errorf("expected 500 iterations, got %d", result.Iterations)
	}
}

func TestBootstrapDeterministicWithSeed(t *testing.T) {
	records := makeTestRecords()
	pricing := DefaultPricing()
	cfg := ReplayConfig{WindowSize: 10, EarlyExitCount: 10}

	a := Bootstrap(records, MethodEntropyRouter, 1.0, EntropyPolicy(1.0, cfg), pricing, 200, 7)
	b := Bootstrap(records, MethodEntropyRouter, 1.0, EntropyPolicy(1.0, cfg), pricing, 200, 7)

	if a.CostReductionLower != b.CostReductionLower || a.CostReductionUpper != b.CostReductionUpper {
		t.Error("expected identical bootstrap results for the same seed")
	}
}

func TestPercentile(t *testing.T) {
	sorted := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	if p := percentile(sorted, 0); p != 1 {
		t.Errorf("expected p0=1, got %f", p)
	}
	if p := percentile(sorted, 100); p != 10 {
		t.Errorf("expected p100=10, got %f", p)
	}
}
