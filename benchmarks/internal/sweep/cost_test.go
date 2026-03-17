package sweep

import (
	"math"
	"testing"

	"github.com/trnahnh/draft-thinker/benchmarks/internal/collect"
)

func TestDefaultPricing(t *testing.T) {
	p := DefaultPricing()
	if p.DrafterInputPer1M != 0.20 {
		t.Errorf("expected drafter input 0.20, got %f", p.DrafterInputPer1M)
	}
	if p.DrafterOutputPer1M != 0.80 {
		t.Errorf("expected drafter output 0.80, got %f", p.DrafterOutputPer1M)
	}
	if p.HeavyOutputPer1M != 10.00 {
		t.Errorf("expected heavy output 10.00, got %f", p.HeavyOutputPer1M)
	}
}

func TestEstimateCostDraftOnly(t *testing.T) {
	r := &collect.Record{DraftTokensUsed: 100, HeavyTokensUsed: 200}
	p := DefaultPricing()

	cost := EstimateCost(r, false, p)
	expected := 100.0 * 0.80 / 1_000_000
	if math.Abs(cost-expected) > 1e-12 {
		t.Errorf("expected %e, got %e", expected, cost)
	}
}

func TestEstimateCostWithEscalation(t *testing.T) {
	r := &collect.Record{DraftTokensUsed: 100, HeavyTokensUsed: 200}
	p := DefaultPricing()

	cost := EstimateCost(r, true, p)
	draftCost := 100.0 * 0.80 / 1_000_000
	heavyCost := 200.0 * 10.00 / 1_000_000
	expected := draftCost + heavyCost
	if math.Abs(cost-expected) > 1e-12 {
		t.Errorf("expected %e, got %e", expected, cost)
	}
}

func TestBaselineCost(t *testing.T) {
	r := &collect.Record{HeavyTokensUsed: 500}
	p := DefaultPricing()

	cost := BaselineCost(r, p)
	expected := 500.0 * 10.00 / 1_000_000
	if math.Abs(cost-expected) > 1e-12 {
		t.Errorf("expected %e, got %e", expected, cost)
	}
}

func TestCostReductionWhenDraftOnly(t *testing.T) {
	r := &collect.Record{DraftTokensUsed: 100, HeavyTokensUsed: 100}
	p := DefaultPricing()

	est := EstimateCost(r, false, p)
	base := BaselineCost(r, p)

	reduction := 1 - est/base
	if reduction < 0.90 {
		t.Errorf("expected >90%% cost reduction for draft-only, got %.2f%%", reduction*100)
	}
}
