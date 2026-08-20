package collect

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/trnahnh/draft-thinker/benchmarks/internal/dataset"
	"github.com/trnahnh/draft-thinker/internal/entropy"
	"github.com/trnahnh/draft-thinker/pkg/client"
	"github.com/trnahnh/draft-thinker/pkg/protocol"
)

type CollectorConfig struct {
	TopLogprobs int
	MaxTokens   int
}

type Collector struct {
	drafter     client.LLMClient
	heavyweight client.LLMClient
	judge       *Judge
	cfg         CollectorConfig
}

func NewCollector(drafter, heavyweight client.LLMClient, judge *Judge, cfg CollectorConfig) *Collector {
	return &Collector{
		drafter:     drafter,
		heavyweight: heavyweight,
		judge:       judge,
		cfg:         cfg,
	}
}

func (c *Collector) Collect(ctx context.Context, prompt dataset.Prompt) (*Record, error) {
	record := &Record{
		PromptID:   prompt.ID,
		Category:   string(prompt.Category),
		PromptText: prompt.Text,
	}

	messages := []protocol.Message{
		{Role: "user", Content: prompt.Text},
	}

	logprobs := true
	topLogprobs := c.cfg.TopLogprobs
	maxTokens := c.cfg.MaxTokens

	draftReq := &protocol.ChatCompletionRequest{
		Messages:    messages,
		Logprobs:    &logprobs,
		TopLogprobs: &topLogprobs,
		MaxTokens:   &maxTokens,
	}

	draftStart := time.Now()
	stream, err := c.drafter.Stream(ctx, draftReq)
	if err != nil {
		record.Error = fmt.Sprintf("drafter stream: %v", err)
		record.CollectedAt = time.Now().UTC().Format(time.RFC3339)
		return record, nil
	}

	var draftContent strings.Builder
	tokenIndex := 0

	for sc := range stream {
		if sc.Err != nil {
			record.Error = fmt.Sprintf("drafter chunk: %v", sc.Err)
			record.CollectedAt = time.Now().UTC().Format(time.RFC3339)
			return record, nil
		}

		if sc.Chunk == nil {
			continue
		}

		for _, choice := range sc.Chunk.Choices {
			if choice.Delta != nil {
				draftContent.WriteString(choice.Delta.Content)
			}
		}

		tokenEntropies := entropy.ExtractTokenEntropy(sc.Chunk, tokenIndex)
		for _, te := range tokenEntropies {
			record.Tokens = append(record.Tokens, TokenRecord{
				Token:   te.Token,
				Entropy: te.Entropy,
				Logprob: te.Logprob,
				Index:   te.Index,
			})
			tokenIndex++
		}
	}
	record.DraftLatencyMS = time.Since(draftStart).Milliseconds()
	record.DraftResponse = draftContent.String()
	record.DraftTokensUsed = tokenIndex
	record.TokenCount = len(record.Tokens)

	if record.TokenCount > 0 {
		sum := 0.0
		maxE := 0.0
		for _, t := range record.Tokens {
			sum += t.Entropy
			if t.Entropy > maxE {
				maxE = t.Entropy
			}
		}
		record.MeanEntropy = sum / float64(record.TokenCount)
		record.MaxEntropy = maxE
	}

	heavyReq := &protocol.ChatCompletionRequest{
		Messages:  messages,
		MaxTokens: &maxTokens,
	}

	heavyStart := time.Now()
	heavyResp, err := c.heavyweight.Complete(ctx, heavyReq)
	if err != nil {
		record.Error = fmt.Sprintf("heavyweight: %v", err)
		record.CollectedAt = time.Now().UTC().Format(time.RFC3339)
		return record, nil
	}
	record.HeavyLatencyMS = time.Since(heavyStart).Milliseconds()

	if len(heavyResp.Choices) > 0 && heavyResp.Choices[0].Message != nil {
		record.HeavyResponse = heavyResp.Choices[0].Message.Content
	}
	if heavyResp.Usage != nil {
		record.HeavyTokensUsed = heavyResp.Usage.CompletionTokens
	}

	record.DraftModel = "drafter"
	record.HeavyModel = "heavyweight"

	judgeResult, err := c.judge.Evaluate(ctx, prompt.Text, record.DraftResponse, record.HeavyResponse)
	if err != nil {
		record.Error = fmt.Sprintf("judge: %v", err)
		record.CollectedAt = time.Now().UTC().Format(time.RFC3339)
		return record, nil
	}
	record.Judge = *judgeResult

	record.CollectedAt = time.Now().UTC().Format(time.RFC3339)
	return record, nil
}
