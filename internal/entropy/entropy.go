package entropy

import (
	"math"

	"github.com/trnahnh/draft-thinker/pkg/protocol"
)

func ComputeEntropy(topLogprobs []float64) float64 {
	if len(topLogprobs) == 0 {
		return 0
	}

	probs := make([]float64, len(topLogprobs))
	sum := 0.0
	for i, lp := range topLogprobs {
		p := math.Exp(lp)
		probs[i] = p
		sum += p
	}

	if sum == 0 {
		return 0
	}

	h := 0.0
	for _, p := range probs {
		p /= sum
		if p > 0 {
			h -= p * math.Log2(p)
		}
	}

	return h
}

type TokenEntropy struct {
	Token   string
	Entropy float64
	Logprob float64
	Index   int
	InTopK  bool
}

// sentinelLogprob is the value OpenAI reports for a sampled token that was
// not among the requested top-k alternatives.
const sentinelLogprob = -9999.0

func sampledInTopK(tl protocol.TokenLogprob) bool {
	if tl.Logprob <= sentinelLogprob {
		return false
	}
	if len(tl.TopLogprobs) == 0 {
		return true
	}
	for _, entry := range tl.TopLogprobs {
		if entry.Token == tl.Token {
			return true
		}
	}
	return false
}

func ExtractTokenEntropy(chunk *protocol.StreamChunk, index int) []TokenEntropy {
	if chunk == nil || len(chunk.Choices) == 0 {
		return nil
	}

	var results []TokenEntropy

	for _, choice := range chunk.Choices {
		if choice.Logprobs == nil {
			continue
		}

		for _, tl := range choice.Logprobs.Content {
			logprobs := make([]float64, len(tl.TopLogprobs))
			for i, entry := range tl.TopLogprobs {
				logprobs[i] = entry.Logprob
			}

			results = append(results, TokenEntropy{
				Token:   tl.Token,
				Entropy: ComputeEntropy(logprobs),
				Logprob: tl.Logprob,
				Index:   index,
				InTopK:  sampledInTopK(tl),
			})
			index++
		}
	}

	return results
}
