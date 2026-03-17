package collect

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/trnahnh/draft-thinker/pkg/client"
	"github.com/trnahnh/draft-thinker/pkg/protocol"
)

func TestJudgeEvaluate(t *testing.T) {
	judgeResp := map[string]any{
		"acceptable": true,
		"score":      4,
		"reasoning":  "Draft is accurate and complete",
	}
	respBody, _ := json.Marshal(judgeResp)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := protocol.ChatCompletionResponse{
			Choices: []protocol.Choice{
				{Message: &protocol.Message{Content: string(respBody)}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := client.NewOpenAICompatibleClient(srv.URL, "test-key", "test-model", "test", 10*time.Second)
	judge := NewJudge(c)

	result, err := judge.Evaluate(context.Background(), "What is 2+2?", "4", "4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Acceptable {
		t.Error("expected acceptable=true")
	}
	if result.Score != 4 {
		t.Errorf("expected score 4, got %d", result.Score)
	}
}

func TestJudgeMarkdownFences(t *testing.T) {
	judgeResp := "```json\n{\"acceptable\": true, \"score\": 3, \"reasoning\": \"ok\"}\n```"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := protocol.ChatCompletionResponse{
			Choices: []protocol.Choice{
				{Message: &protocol.Message{Content: judgeResp}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := client.NewOpenAICompatibleClient(srv.URL, "test-key", "test-model", "test", 10*time.Second)
	judge := NewJudge(c)

	result, err := judge.Evaluate(context.Background(), "prompt", "draft", "reference")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Score != 3 {
		t.Errorf("expected score 3, got %d", result.Score)
	}
}

func TestJudgeMalformedRetry(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var content string
		if calls == 1 {
			content = "not valid json at all"
		} else {
			content = `{"acceptable": true, "score": 5, "reasoning": "great"}`
		}
		resp := protocol.ChatCompletionResponse{
			Choices: []protocol.Choice{
				{Message: &protocol.Message{Content: content}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := client.NewOpenAICompatibleClient(srv.URL, "test-key", "test-model", "test", 10*time.Second)
	judge := NewJudge(c)

	result, err := judge.Evaluate(context.Background(), "prompt", "draft", "reference")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Score != 5 {
		t.Errorf("expected score 5, got %d", result.Score)
	}
	if calls != 2 {
		t.Errorf("expected 2 calls, got %d", calls)
	}
}

func TestJudgeScoreThreshold(t *testing.T) {
	tests := []struct {
		score      int
		acceptable bool
	}{
		{1, false},
		{2, false},
		{3, true},
		{4, true},
		{5, true},
	}

	for _, tt := range tests {
		judgeResp, _ := json.Marshal(map[string]any{
			"acceptable": !tt.acceptable, // intentionally wrong, should be overridden by score
			"score":      tt.score,
			"reasoning":  "test",
		})

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			resp := protocol.ChatCompletionResponse{
				Choices: []protocol.Choice{
					{Message: &protocol.Message{Content: string(judgeResp)}},
				},
			}
			json.NewEncoder(w).Encode(resp)
		}))

		c := client.NewOpenAICompatibleClient(srv.URL, "test-key", "test-model", "test", 10*time.Second)
		judge := NewJudge(c)

		result, err := judge.Evaluate(context.Background(), "prompt", "draft", "ref")
		srv.Close()
		if err != nil {
			t.Fatalf("score %d: unexpected error: %v", tt.score, err)
		}
		if result.Acceptable != tt.acceptable {
			t.Errorf("score %d: expected acceptable=%v, got %v", tt.score, tt.acceptable, result.Acceptable)
		}
	}
}
