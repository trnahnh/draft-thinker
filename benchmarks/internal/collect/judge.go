package collect

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/trnahnh/draft-thinker/pkg/client"
	"github.com/trnahnh/draft-thinker/pkg/protocol"
)

type Judge struct {
	client client.LLMClient
}

func NewJudge(c client.LLMClient) *Judge {
	return &Judge{client: c}
}

const judgePrompt = `You are an evaluation judge. Compare a draft response against a reference response for the given prompt.

Evaluate the draft on:
1. Factual accuracy compared to the reference
2. Completeness of the answer
3. Whether it would be acceptable to serve to a user

Respond with ONLY valid JSON, no markdown fences:
{"acceptable": true/false, "score": 1-5, "reasoning": "brief explanation"}

Score guide:
1 = completely wrong or incoherent
2 = partially correct but missing critical information
3 = mostly correct, acceptable quality
4 = good quality, minor differences from reference
5 = equivalent or better than reference

A score of 3 or above means acceptable=true.

Prompt: %s

Draft response: %s

Reference response: %s`

func (j *Judge) Evaluate(ctx context.Context, prompt, draftResponse, referenceResponse string) (*JudgeResult, error) {
	msg := fmt.Sprintf(judgePrompt, prompt, draftResponse, referenceResponse)

	req := &protocol.ChatCompletionRequest{
		Messages: []protocol.Message{
			{Role: "user", Content: msg},
		},
		MaxTokens: intPtr(256),
	}

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		resp, err := j.client.Complete(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("judge API call: %w", err)
		}

		if len(resp.Choices) == 0 {
			return nil, fmt.Errorf("judge returned no choices")
		}

		content := resp.Choices[0].Message.Content
		content = stripMarkdownFences(content)
		content = strings.TrimSpace(content)

		var result JudgeResult
		if err := json.Unmarshal([]byte(content), &result); err != nil {
			lastErr = fmt.Errorf("parsing judge response (attempt %d): %w, raw: %s", attempt+1, err, content)
			continue
		}

		result.Acceptable = result.Score >= 3
		return &result, nil
	}

	return nil, lastErr
}

func stripMarkdownFences(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```json") {
		s = strings.TrimPrefix(s, "```json")
	} else if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
	}
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

func intPtr(n int) *int {
	return &n
}
