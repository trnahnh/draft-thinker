package sweep

import (
	"math"
	"testing"

	"github.com/trnahnh/draft-thinker/benchmarks/internal/collect"
)

func makeTestRecords() []*collect.Record {
	records := make([]*collect.Record, 10)

	for i := 0; i < 10; i++ {
		acceptable := i < 7
		var entropy float64
		if acceptable {
			entropy = 0.3
		} else {
			entropy = 2.0
		}

		tokens := make([]collect.TokenRecord, 20)
		for j := range tokens {
			tokens[j] = collect.TokenRecord{Token: "t", Entropy: entropy, Index: j}
		}

		score := 2
		if acceptable {
			score = 4
		}

		records[i] = &collect.Record{
			PromptID:        "p" + string(rune('0'+i)),
			Category:        "simple_factual",
			Tokens:          tokens,
			TokenCount:      20,
			DraftTokensUsed: 20,
			HeavyTokensUsed: 50,
			Judge: collect.JudgeResult{
				Acceptable: acceptable,
				Score:      score,
			},
		}
	}

	return records
}

func TestComputeStatsConfusionMatrix(t *testing.T) {
	records := makeTestRecords()
	cfg := ReplayConfig{WindowSize: 10, EarlyExitCount: 10}
	pricing := DefaultPricing()

	stats := ComputeStats(records, 1.0, cfg, pricing)

	if stats.TotalPrompts != 10 {
		t.Errorf("expected 10 total, got %d", stats.TotalPrompts)
	}

	total := stats.TP + stats.FP + stats.TN + stats.FN
	if total != 10 {
		t.Errorf("confusion matrix should sum to 10, got %d (TP=%d FP=%d TN=%d FN=%d)",
			total, stats.TP, stats.FP, stats.TN, stats.FN)
	}

	if stats.TP != 3 {
		t.Errorf("expected TP=3 (unacceptable and would escalate), got %d", stats.TP)
	}

	if stats.TN != 7 {
		t.Errorf("expected TN=7 (acceptable and would not escalate), got %d", stats.TN)
	}

	if stats.FN != 0 {
		t.Errorf("expected FN=0, got %d", stats.FN)
	}

	if stats.FP != 0 {
		t.Errorf("expected FP=0, got %d", stats.FP)
	}
}

func TestComputeStatsMetrics(t *testing.T) {
	records := makeTestRecords()
	cfg := ReplayConfig{WindowSize: 10, EarlyExitCount: 10}
	pricing := DefaultPricing()

	stats := ComputeStats(records, 1.0, cfg, pricing)

	if math.Abs(stats.EscalationRate-0.3) > 0.01 {
		t.Errorf("expected escalation rate 0.3, got %f", stats.EscalationRate)
	}

	if math.Abs(stats.DraftAccuracy-1.0) > 0.01 {
		t.Errorf("expected draft accuracy 1.0, got %f", stats.DraftAccuracy)
	}

	if stats.CostReduction <= 0 {
		t.Error("expected positive cost reduction")
	}

	if stats.F1 <= 0 {
		t.Error("expected positive F1")
	}
}

func TestComputeStatsErrorRecordsFiltered(t *testing.T) {
	records := []*collect.Record{
		{PromptID: "good", Category: "simple_factual", Tokens: []collect.TokenRecord{{Entropy: 0.1}},
			DraftTokensUsed: 10, HeavyTokensUsed: 20, Judge: collect.JudgeResult{Acceptable: true, Score: 4}},
		{PromptID: "bad", Category: "simple_factual", Error: "some error"},
	}

	cfg := ReplayConfig{WindowSize: 10, EarlyExitCount: 10}
	stats := ComputeStats(records, 1.0, cfg, DefaultPricing())

	if stats.TotalPrompts != 1 {
		t.Errorf("expected 1 (error record filtered), got %d", stats.TotalPrompts)
	}
}

func TestComputeStatsByCategory(t *testing.T) {
	records := []*collect.Record{
		{PromptID: "sf1", Category: "simple_factual",
			Tokens: []collect.TokenRecord{{Entropy: 0.1}}, DraftTokensUsed: 10, HeavyTokensUsed: 20,
			Judge: collect.JudgeResult{Acceptable: true, Score: 4}},
		{PromptID: "cg1", Category: "code_generation",
			Tokens: []collect.TokenRecord{{Entropy: 2.5}}, DraftTokensUsed: 10, HeavyTokensUsed: 20,
			Judge: collect.JudgeResult{Acceptable: false, Score: 2}},
	}

	cfg := ReplayConfig{WindowSize: 10, EarlyExitCount: 10}
	stats := ComputeStats(records, 1.0, cfg, DefaultPricing())

	if len(stats.ByCategory) != 2 {
		t.Errorf("expected 2 categories, got %d", len(stats.ByCategory))
	}

	sf, ok := stats.ByCategory["simple_factual"]
	if !ok {
		t.Fatal("missing simple_factual category")
	}
	if sf.Total != 1 || sf.Accepted != 1 {
		t.Errorf("simple_factual: expected total=1 accepted=1, got total=%d accepted=%d", sf.Total, sf.Accepted)
	}
}
