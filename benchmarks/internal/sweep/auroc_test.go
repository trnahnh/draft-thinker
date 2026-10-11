package sweep

import (
	"math"
	"testing"

	"github.com/trnahnh/draft-thinker/benchmarks/internal/collect"
)

func labeled(acceptable bool, mean float64, entropies ...float64) *collect.Record {
	tokens := make([]collect.TokenRecord, len(entropies))
	for i, e := range entropies {
		tokens[i] = collect.TokenRecord{Token: "t", Entropy: e, Index: i}
	}
	return &collect.Record{
		Tokens:      tokens,
		TokenCount:  len(tokens),
		MeanEntropy: mean,
		Judge:       collect.JudgeResult{Acceptable: acceptable},
	}
}

func TestAUROCPerfectSeparation(t *testing.T) {
	records := []*collect.Record{
		labeled(true, 0.1), labeled(true, 0.2), labeled(true, 0.3),
		labeled(false, 0.8), labeled(false, 0.9),
	}
	auc, ok := AUROC(records, MeanEntropyScore)
	if !ok || math.Abs(auc-1.0) > 1e-9 {
		t.Fatalf("expected AUROC 1.0 for perfectly separated scores, got %f (ok=%v)", auc, ok)
	}
}

func TestAUROCInvertedAndTies(t *testing.T) {
	inverted := []*collect.Record{
		labeled(true, 0.9), labeled(true, 0.8),
		labeled(false, 0.1), labeled(false, 0.2),
	}
	if auc, _ := AUROC(inverted, MeanEntropyScore); math.Abs(auc) > 1e-9 {
		t.Errorf("expected AUROC 0 for inverted scores, got %f", auc)
	}
	tied := []*collect.Record{
		labeled(true, 0.5), labeled(true, 0.5),
		labeled(false, 0.5), labeled(false, 0.5),
	}
	if auc, _ := AUROC(tied, MeanEntropyScore); math.Abs(auc-0.5) > 1e-9 {
		t.Errorf("expected AUROC 0.5 for all-tied scores, got %f", auc)
	}
}

func TestAUROCNeedsBothClasses(t *testing.T) {
	if _, ok := AUROC([]*collect.Record{labeled(true, 0.1), labeled(true, 0.2)}, MeanEntropyScore); ok {
		t.Error("AUROC must be undefined without any positive record")
	}
}

func TestEntropyScoreMatchesReplayDecision(t *testing.T) {
	cfg := ReplayConfig{WindowSize: 5, EarlyExitCount: 3}
	traces := [][]float64{
		{0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1},
		{0.1, 2.2, 0.1, 0.1, 0.1, 0.1, 0.1, 0.1},
		{0.1, 0.1, 0.1, 0.1, 1.5, 1.6, 1.7, 1.8, 1.9, 2.0},
		{0.1, 0.1, 0.1, 2.3, 0.1, 0.1, 0.1},
	}
	score := EntropyScore(cfg)
	for _, tr := range traces {
		r := labeled(true, 0, tr...)
		peak := score(r)
		for _, threshold := range []float64{0.5, 1.0, 1.5, 2.0, 2.1} {
			escalates := Replay(r.Tokens, threshold, cfg).WouldEscalate
			if escalates != (peak > threshold) {
				t.Errorf("trace %v T=%.1f: replay escalates=%v but peak=%.3f", tr, threshold, escalates, peak)
			}
		}
	}
}

func TestHighEntropyFraction(t *testing.T) {
	r := labeled(true, 0, 0.2, 1.5, 0.3, 2.0)
	if got := HighEntropyFraction(1.0)(r); math.Abs(got-0.5) > 1e-9 {
		t.Errorf("expected half the tokens above 1 bit, got %f", got)
	}
	if got := HighEntropyFraction(1.0)(labeled(true, 0)); got != 0 {
		t.Errorf("empty trace must score 0, got %f", got)
	}
}

func TestThresholdForRateMatchesEscalationRate(t *testing.T) {
	records := make([]*collect.Record, 100)
	for i := range records {
		records[i] = labeled(true, float64(i)/100)
	}
	threshold := ThresholdForRate(records, MeanEntropyScore, 0.10)
	escalated := 0
	for _, r := range records {
		if MeanEntropyPolicy(threshold)(r).Escalate {
			escalated++
		}
	}
	if escalated != 10 {
		t.Errorf("expected 10 of 100 records to escalate at the matched threshold, got %d", escalated)
	}
}

func TestBootstrapAUROCIntervalContainsPoint(t *testing.T) {
	records := make([]*collect.Record, 0, 60)
	for i := range 50 {
		records = append(records, labeled(true, 0.1+float64(i%7)*0.05))
	}
	for i := range 10 {
		records = append(records, labeled(false, 0.3+float64(i%5)*0.1))
	}
	res := BootstrapAUROC(records, "mean", MeanEntropyScore, 300, 7)
	if !res.OK {
		t.Fatal("expected a defined AUROC")
	}
	if res.Lower > res.Point || res.Point > res.Upper {
		t.Errorf("point %f outside interval [%f, %f]", res.Point, res.Lower, res.Upper)
	}
	if res.Positives != 10 || res.Negatives != 50 {
		t.Errorf("class counts = %d/%d, want 10/50", res.Positives, res.Negatives)
	}
}
