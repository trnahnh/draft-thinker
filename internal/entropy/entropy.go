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
	Index   int
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
				Index:   index,
			})
			index++
		}
	}

	return results
}
