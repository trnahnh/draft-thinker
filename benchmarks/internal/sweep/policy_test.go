package sweep

import (
	"testing"

	"github.com/trnahnh/draft-thinker/benchmarks/internal/collect"
)

func TestConfidencePolicyEscalatesOnLowLogprob(t *testing.T) {
	tokens := []collect.TokenRecord{
		{Token: "a", Logprob: -0.1},
		{Token: "b", Logprob: -0.2},
		{Token: "c", Logprob: -6.5},
		{Token: "d", Logprob: -0.05},
	}

	policy := ConfidencePolicy(-3.0)
	if !policy(tokens) {
		t.Error("expected escalation: min logprob -6.5 is below threshold -3.0")
	}
}

func TestConfidencePolicyNoEscalation(t *testing.T) {
	tokens := []collect.TokenRecord{
		{Token: "a", Logprob: -0.1},
		{Token: "b", Logprob: -0.2},
		{Token: "c", Logprob: -1.5},
	}

	policy := ConfidencePolicy(-3.0)
	if policy(tokens) {
		t.Error("expected no escalation: all logprobs above threshold -3.0")
	}
}

func TestConfidencePolicyEmptyTokens(t *testing.T) {
	policy := ConfidencePolicy(-3.0)
	if policy(nil) {
		t.Error("expected no escalation for empty tokens")
	}
}

func TestConfidencePolicyBoundary(t *testing.T) {
	tokens := []collect.TokenRecord{{Logprob: -3.0}}

	policy := ConfidencePolicy(-3.0)
	if policy(tokens) {
		t.Error("logprob equal to threshold should NOT escalate (strict <)")
	}

	tokensBelow := []collect.TokenRecord{{Logprob: -3.01}}
	if !policy(tokensBelow) {
		t.Error("logprob below threshold should escalate")
	}
}

func TestAlwaysHeavyweightPolicy(t *testing.T) {
	policy := AlwaysHeavyweightPolicy()
	if !policy(nil) {
		t.Error("expected always-heavyweight to escalate on empty tokens")
	}
	if !policy([]collect.TokenRecord{{Logprob: -0.01}}) {
		t.Error("expected always-heavyweight to escalate regardless of tokens")
	}
}

func TestComputeStatsWithPolicyOverallAccuracy(t *testing.T) {
	records := makeTestRecords()

	stats := ComputeStatsWithPolicy(records, MethodAlwaysHeavyweight, 0, AlwaysHeavyweightPolicy(), DefaultPricing())

	if stats.EscalationRate != 1.0 {
		t.Errorf("expected escalation rate 1.0, got %f", stats.EscalationRate)
	}
	if stats.OverallAccuracy != 1.0 {
		t.Errorf("expected overall accuracy 1.0 (always escalates, never serves a bad draft), got %f", stats.OverallAccuracy)
	}
	if stats.CostReduction != 0 {
		t.Errorf("expected zero cost reduction for always-heavyweight, got %f", stats.CostReduction)
	}
}
