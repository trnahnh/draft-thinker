package entropy

import (
	"math"
	"testing"

	"github.com/trnahnh/draft-thinker/pkg/protocol"
)

func TestComputeEntropy_Uniform(t *testing.T) {
	k := 5
	logprob := math.Log(1.0 / float64(k))
	logprobs := make([]float64, k)
	for i := range logprobs {
		logprobs[i] = logprob
	}

	got := ComputeEntropy(logprobs)
	want := math.Log2(float64(k))

	if math.Abs(got-want) > 1e-10 {
		t.Errorf("uniform k=%d: got %f, want %f", k, got, want)
	}
}

func TestComputeEntropy_NearCertain(t *testing.T) {
	logprobs := []float64{
		math.Log(0.999),
		math.Log(0.0005),
		math.Log(0.0005),
	}

	got := ComputeEntropy(logprobs)
	if got > 0.05 {
		t.Errorf("near-certain: got %f, want near 0", got)
	}
}

func TestComputeEntropy_Empty(t *testing.T) {
	got := ComputeEntropy(nil)
	if got != 0 {
		t.Errorf("empty: got %f, want 0", got)
	}
}

func TestComputeEntropy_SingleToken(t *testing.T) {
	got := ComputeEntropy([]float64{math.Log(1.0)})
	if math.Abs(got) > 1e-10 {
		t.Errorf("single token: got %f, want 0", got)
	}
}

func TestComputeEntropy_RealisticLogprobs(t *testing.T) {
	logprobs := []float64{-0.5, -2.3, -3.0, -4.0, -5.0}

	got := ComputeEntropy(logprobs)
	if got <= 0 || got >= math.Log2(5) {
		t.Errorf("realistic: got %f, expected between 0 and %f", got, math.Log2(5))
	}
}

func TestExtractTokenEntropy_WithLogprobs(t *testing.T) {
	chunk := &protocol.StreamChunk{
		ID:    "test",
		Model: "llama3-8b-8192",
		Choices: []protocol.Choice{
			{
				Index: 0,
				Delta: &protocol.Delta{Content: "Hello"},
				Logprobs: &protocol.ChoiceLogprobs{
					Content: []protocol.TokenLogprob{
						{
							Token:   "Hello",
							Logprob: -0.001,
							TopLogprobs: []protocol.TopLogprobEntry{
								{Token: "Hello", Logprob: -0.001},
								{Token: "Hi", Logprob: -7.0},
								{Token: "Hey", Logprob: -8.0},
							},
						},
					},
				},
			},
		},
	}

	results := ExtractTokenEntropy(chunk, 0)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].Token != "Hello" {
		t.Errorf("token: got %q, want %q", results[0].Token, "Hello")
	}
	if results[0].Entropy > 0.1 {
		t.Errorf("entropy should be near 0 for confident token, got %f", results[0].Entropy)
	}
	if results[0].Index != 0 {
		t.Errorf("index: got %d, want 0", results[0].Index)
	}
}

func TestExtractTokenEntropy_NoLogprobs(t *testing.T) {
	chunk := &protocol.StreamChunk{
		ID:    "test",
		Model: "llama3-8b-8192",
		Choices: []protocol.Choice{
			{
				Index: 0,
				Delta: &protocol.Delta{Content: "Hello"},
			},
		},
	}

	results := ExtractTokenEntropy(chunk, 0)
	if len(results) != 0 {
		t.Errorf("expected 0 results for no logprobs, got %d", len(results))
	}
}

func TestExtractTokenEntropy_NilChunk(t *testing.T) {
	results := ExtractTokenEntropy(nil, 0)
	if results != nil {
		t.Errorf("expected nil for nil chunk, got %v", results)
	}
}
