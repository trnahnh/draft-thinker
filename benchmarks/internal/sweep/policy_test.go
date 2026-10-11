package sweep

import (
	"testing"

	"github.com/trnahnh/draft-thinker/benchmarks/internal/collect"
)

func recordWithLogprobs(lps ...float64) *collect.Record {
	tokens := make([]collect.TokenRecord, len(lps))
	for i, lp := range lps {
		tokens[i] = collect.TokenRecord{Token: "t", Logprob: lp, Index: i}
	}
	return &collect.Record{PromptID: "p", Tokens: tokens, TokenCount: len(tokens)}
}

func TestConfidencePolicyEscalatesOnLowLogprob(t *testing.T) {
	r := recordWithLogprobs(-0.1, -0.2, -6.5, -0.05)
	d := ConfidencePolicy(-3.0)(r)
	if !d.Escalate {
		t.Error("expected escalation: min logprob -6.5 is below threshold -3.0")
	}
	if d.TokensConsumed != 4 {
		t.Errorf("confidence policy reads the whole response, got %d tokens consumed", d.TokensConsumed)
	}
}

func TestConfidencePolicyNoEscalation(t *testing.T) {
	r := recordWithLogprobs(-0.1, -0.2, -1.5)
	if ConfidencePolicy(-3.0)(r).Escalate {
		t.Error("expected no escalation: all logprobs above threshold -3.0")
	}
}

func TestConfidencePolicyEmptyTokens(t *testing.T) {
	if ConfidencePolicy(-3.0)(&collect.Record{}).Escalate {
		t.Error("expected no escalation for empty tokens")
	}
}

func TestConfidencePolicySentinelAlwaysEscalates(t *testing.T) {
	r := recordWithLogprobs(-0.1, -9999.0, -0.2)
	if !ConfidencePolicy(-40.0)(r).Escalate {
		t.Error("a -9999 sentinel should escalate even at a very lenient threshold")
	}
}

func TestAlwaysPolicies(t *testing.T) {
	r := recordWithLogprobs(-0.1)
	if !AlwaysHeavyweightPolicy()(r).Escalate {
		t.Error("always-heavyweight must escalate")
	}
	if AlwaysDrafterPolicy()(r).Escalate {
		t.Error("always-drafter must never escalate")
	}
}

func TestOraclePolicyFollowsLabel(t *testing.T) {
	good := &collect.Record{PromptID: "g", Judge: collect.JudgeResult{Acceptable: true}}
	bad := &collect.Record{PromptID: "b", Judge: collect.JudgeResult{Acceptable: false}}
	if OraclePolicy()(good).Escalate {
		t.Error("oracle must accept acceptable drafts")
	}
	if !OraclePolicy()(bad).Escalate {
		t.Error("oracle must escalate unacceptable drafts")
	}
}

func TestRandomPolicyIsDeterministicAndRateMatched(t *testing.T) {
	records := make([]*collect.Record, 2000)
	for i := range records {
		records[i] = &collect.Record{PromptID: "prompt_" + string(rune('a'+i%26)) + string(rune('a'+(i/26)%26)) + string(rune('a'+(i/676)%26))}
	}
	policy := RandomPolicy(0.25, 42)
	escalated := 0
	for _, r := range records {
		if policy(r).Escalate {
			escalated++
		}
	}
	rate := float64(escalated) / float64(len(records))
	if rate < 0.20 || rate > 0.30 {
		t.Errorf("expected escalation rate near 0.25, got %.3f", rate)
	}
	again := RandomPolicy(0.25, 42)
	for _, r := range records {
		if policy(r).Escalate != again(r).Escalate {
			t.Fatal("random policy must be deterministic for a fixed seed and prompt ID")
		}
	}
}

func TestEntropyPolicyReportsEscalationPoint(t *testing.T) {
	tokens := make([]collect.TokenRecord, 30)
	for i := range tokens {
		e := 0.2
		if i == 20 {
			e = 2.3
		}
		tokens[i] = collect.TokenRecord{Token: "t", Entropy: e, Index: i}
	}
	r := &collect.Record{Tokens: tokens, TokenCount: 30}
	cfg := ReplayConfig{WindowSize: 10, EarlyExitCount: 10}

	d := EntropyPolicy(0.4, cfg)(r)
	if !d.Escalate {
		t.Fatal("expected escalation when the window average exceeds the threshold")
	}
	if d.TokensConsumed != 21 {
		t.Errorf("expected escalation right after token 20, got %d tokens consumed", d.TokensConsumed)
	}
	if EntropyPolicy(2.4, cfg)(r).Escalate {
		t.Error("threshold above the maximum possible entropy must never escalate")
	}
}
