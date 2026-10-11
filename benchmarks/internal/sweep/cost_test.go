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
	p := DefaultPricing().OutputOnly()

	cost := EstimateCost(r, false, p)
	expected := 100.0 * 0.80 / 1_000_000
	if math.Abs(cost-expected) > 1e-12 {
		t.Errorf("expected %e, got %e", expected, cost)
	}
}

func TestEstimateCostWithEscalation(t *testing.T) {
	r := &collect.Record{DraftTokensUsed: 100, HeavyTokensUsed: 200}
	p := DefaultPricing().OutputOnly()

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
	p := DefaultPricing().OutputOnly()

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

func TestInputTokensArePriced(t *testing.T) {
	r := &collect.Record{PromptTokens: 1000, DraftTokensUsed: 100, HeavyTokensUsed: 200}
	p := DefaultPricing()

	draft := EstimateCost(r, false, p)
	expectedDraft := (1000.0*0.20 + 100.0*0.80) / 1_000_000
	if math.Abs(draft-expectedDraft) > 1e-12 {
		t.Errorf("draft path: expected %e, got %e", expectedDraft, draft)
	}

	base := BaselineCost(r, p)
	expectedBase := (1000.0*2.50 + 200.0*10.00) / 1_000_000
	if math.Abs(base-expectedBase) > 1e-12 {
		t.Errorf("baseline: expected %e, got %e", expectedBase, base)
	}

	escalated := EstimateCost(r, true, p)
	if math.Abs(escalated-(draft+base)) > 1e-12 {
		t.Errorf("escalation should pay both calls: expected %e, got %e", draft+base, escalated)
	}
}

func TestOutputOnlyPricingIgnoresPromptTokens(t *testing.T) {
	r := &collect.Record{PromptTokens: 5000, HeavyTokensUsed: 100}
	cost := BaselineCost(r, DefaultPricing().OutputOnly())
	if math.Abs(cost-100.0*10.00/1_000_000) > 1e-12 {
		t.Errorf("output-only pricing must ignore prompt tokens, got %e", cost)
	}
}
