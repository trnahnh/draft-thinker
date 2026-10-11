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
	// DrafterOnly skips the heavyweight reference and the judge. Used for
	// logprob-artifact analysis against providers where only the drafter
	// trace matters.
	DrafterOnly bool
	DraftModel  string
	HeavyModel  string
}

type Collector struct {
	drafter     client.LLMClient
	heavyweight client.LLMClient
	judge       *Judge
	cfg         CollectorConfig
}

func NewCollector(drafter, heavyweight client.LLMClient, judge *Judge, cfg CollectorConfig) *Collector {
	if cfg.DraftModel == "" {
		cfg.DraftModel = "drafter"
	}
	if cfg.HeavyModel == "" {
		cfg.HeavyModel = "heavyweight"
	}
	return &Collector{
		drafter:     drafter,
		heavyweight: heavyweight,
		judge:       judge,
		cfg:         cfg,
	}
}

func (c *Collector) Collect(ctx context.Context, prompt dataset.Prompt) (*Record, error) {
	record := &Record{
		PromptID:    prompt.ID,
		Category:    string(prompt.Category),
		PromptText:  prompt.Text,
		DraftModel:  c.cfg.DraftModel,
		HeavyModel:  c.cfg.HeavyModel,
		Grading:     prompt.Grading,
		GroundTruth: prompt.Answer,
	}
	if record.Grading == "" {
		record.Grading = GradingJudge
	}
	exact := record.Grading != GradingJudge

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
		return c.fail(record, fmt.Sprintf("drafter stream: %v", err)), nil
	}

	var draftContent strings.Builder
	tokenIndex := 0

	for sc := range stream {
		if sc.Err != nil {
			return c.fail(record, fmt.Sprintf("drafter chunk: %v", sc.Err)), nil
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
				Token:       te.Token,
				Entropy:     te.Entropy,
				Logprob:     te.Logprob,
				Index:       te.Index,
				OutsideTopK: !te.InTopK,
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

	if exact {
		answer, ok := Grade(record.Grading, record.DraftResponse, record.GroundTruth)
		record.DraftAnswer = answer
		record.DraftCorrect = &ok
		record.Judge = JudgeResult{Acceptable: ok, Score: scoreFor(ok), Reasoning: "exact match against ground truth"}
	}

	if c.cfg.DrafterOnly {
		if !exact {
			record.Judge = JudgeResult{Reasoning: "drafter-only collection: not judged"}
		}
		record.CollectedAt = time.Now().UTC().Format(time.RFC3339)
		return record, nil
	}

	heavyReq := &protocol.ChatCompletionRequest{
		Messages:  messages,
		MaxTokens: &maxTokens,
	}

	heavyStart := time.Now()
	heavyResp, err := c.heavyweight.Complete(ctx, heavyReq)
	if err != nil {
		return c.fail(record, fmt.Sprintf("heavyweight: %v", err)), nil
	}
	record.HeavyLatencyMS = time.Since(heavyStart).Milliseconds()

	if len(heavyResp.Choices) > 0 && heavyResp.Choices[0].Message != nil {
		record.HeavyResponse = heavyResp.Choices[0].Message.Content
	}
	if heavyResp.Usage != nil {
		record.HeavyTokensUsed = heavyResp.Usage.CompletionTokens
		record.PromptTokens = heavyResp.Usage.PromptTokens
	}

	if exact {
		answer, ok := Grade(record.Grading, record.HeavyResponse, record.GroundTruth)
		record.HeavyAnswer = answer
		record.HeavyCorrect = &ok
		record.CollectedAt = time.Now().UTC().Format(time.RFC3339)
		return record, nil
	}

	judgeResult, err := c.judge.Evaluate(ctx, prompt.Text, record.DraftResponse, record.HeavyResponse)
	if err != nil {
		return c.fail(record, fmt.Sprintf("judge: %v", err)), nil
	}
	record.Judge = *judgeResult

	record.CollectedAt = time.Now().UTC().Format(time.RFC3339)
	return record, nil
}

func (c *Collector) fail(record *Record, msg string) *Record {
	record.Error = msg
	record.CollectedAt = time.Now().UTC().Format(time.RFC3339)
	return record
}

func scoreFor(correct bool) int {
	if correct {
		return 5
	}
	return 1
}
