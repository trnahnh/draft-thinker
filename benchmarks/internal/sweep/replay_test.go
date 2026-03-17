package sweep

import (
	"testing"

	"github.com/trnahnh/draft-thinker/benchmarks/internal/collect"
)

func TestReplayNoEscalation(t *testing.T) {
	tokens := []collect.TokenRecord{
		{Token: "a", Entropy: 0.1, Index: 0},
		{Token: "b", Entropy: 0.2, Index: 1},
		{Token: "c", Entropy: 0.15, Index: 2},
		{Token: "d", Entropy: 0.1, Index: 3},
		{Token: "e", Entropy: 0.2, Index: 4},
	}

	result := Replay(tokens, 1.5, ReplayConfig{WindowSize: 3, EarlyExitCount: 3})
	if result.WouldEscalate {
		t.Error("expected no escalation for low entropy tokens")
	}
	if result.TokensConsumed != 5 {
		t.Errorf("expected 5 tokens consumed, got %d", result.TokensConsumed)
	}
}

func TestReplayEarlyExit(t *testing.T) {
	tokens := []collect.TokenRecord{
		{Token: "a", Entropy: 0.1, Index: 0},
		{Token: "b", Entropy: 2.5, Index: 1},
		{Token: "c", Entropy: 0.1, Index: 2},
	}

	result := Replay(tokens, 1.5, ReplayConfig{WindowSize: 10, EarlyExitCount: 10})
	if !result.WouldEscalate {
		t.Error("expected escalation on early high-entropy token")
	}
	if result.EscalateAtToken != 1 {
		t.Errorf("expected escalation at token 1, got %d", result.EscalateAtToken)
	}
	if result.TokensConsumed != 2 {
		t.Errorf("expected 2 tokens consumed, got %d", result.TokensConsumed)
	}
}

func TestReplayWindowAverageEscalation(t *testing.T) {
	tokens := make([]collect.TokenRecord, 15)
	for i := 0; i < 10; i++ {
		tokens[i] = collect.TokenRecord{Token: "ok", Entropy: 0.5, Index: i}
	}
	for i := 10; i < 15; i++ {
		tokens[i] = collect.TokenRecord{Token: "bad", Entropy: 2.0, Index: i}
	}

	result := Replay(tokens, 1.0, ReplayConfig{WindowSize: 5, EarlyExitCount: 5})

	if !result.WouldEscalate {
		t.Error("expected escalation from high window average")
	}
}

func TestReplayEmptyTokens(t *testing.T) {
	result := Replay(nil, 1.0, ReplayConfig{WindowSize: 10, EarlyExitCount: 10})
	if result.WouldEscalate {
		t.Error("expected no escalation for empty tokens")
	}
	if result.TokensConsumed != 0 {
		t.Errorf("expected 0 tokens consumed, got %d", result.TokensConsumed)
	}
}

func TestReplayMatchesProductionWindow(t *testing.T) {
	tokens := []collect.TokenRecord{
		{Entropy: 0.3}, {Entropy: 0.4}, {Entropy: 0.5},
		{Entropy: 0.3}, {Entropy: 0.2}, {Entropy: 0.6},
		{Entropy: 0.4}, {Entropy: 0.3}, {Entropy: 0.5},
		{Entropy: 0.4},
		{Entropy: 1.8}, {Entropy: 1.9}, {Entropy: 2.0},
		{Entropy: 1.7}, {Entropy: 1.6},
	}

	cfg := ReplayConfig{WindowSize: 5, EarlyExitCount: 5}

	lowResult := Replay(tokens, 0.5, cfg)
	if !lowResult.WouldEscalate {
		t.Error("threshold 0.5: expected escalation")
	}

	highResult := Replay(tokens, 5.0, cfg)
	if highResult.WouldEscalate {
		t.Error("threshold 5.0: expected no escalation")
	}
}

func TestReplayThresholdBoundary(t *testing.T) {
	tokens := make([]collect.TokenRecord, 20)
	for i := range tokens {
		tokens[i] = collect.TokenRecord{Token: "t", Entropy: 1.0, Index: i}
	}

	exactly := Replay(tokens, 1.0, ReplayConfig{WindowSize: 5, EarlyExitCount: 3})
	if exactly.WouldEscalate {
		t.Error("entropy equal to threshold should NOT escalate (window uses strict >)")
	}

	justBelow := Replay(tokens, 0.99, ReplayConfig{WindowSize: 5, EarlyExitCount: 3})
	if !justBelow.WouldEscalate {
		t.Error("entropy above threshold should escalate")
	}
}
