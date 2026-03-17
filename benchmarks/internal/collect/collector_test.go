package collect

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/trnahnh/draft-thinker/benchmarks/internal/dataset"
	"github.com/trnahnh/draft-thinker/pkg/client"
	"github.com/trnahnh/draft-thinker/pkg/protocol"
)

func TestCollectorCollect(t *testing.T) {
	drafterSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("expected flusher")
		}

		chunks := []protocol.StreamChunk{
			{
				Choices: []protocol.Choice{
					{
						Delta: &protocol.Delta{Content: "Hello"},
						Logprobs: &protocol.ChoiceLogprobs{
							Content: []protocol.TokenLogprob{
								{
									Token:   "Hello",
									Logprob: -0.1,
									TopLogprobs: []protocol.TopLogprobEntry{
										{Token: "Hello", Logprob: -0.1},
										{Token: "Hi", Logprob: -2.3},
										{Token: "Hey", Logprob: -3.5},
									},
								},
							},
						},
					},
				},
			},
			{
				Choices: []protocol.Choice{
					{
						Delta: &protocol.Delta{Content: " world"},
						Logprobs: &protocol.ChoiceLogprobs{
							Content: []protocol.TokenLogprob{
								{
									Token:   " world",
									Logprob: -0.05,
									TopLogprobs: []protocol.TopLogprobEntry{
										{Token: " world", Logprob: -0.05},
										{Token: " there", Logprob: -3.0},
									},
								},
							},
						},
					},
				},
			},
		}

		for _, chunk := range chunks {
			data, _ := json.Marshal(chunk)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
		fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer drafterSrv.Close()

	heavySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req protocol.ChatCompletionRequest
		json.NewDecoder(r.Body).Decode(&req)

		if req.Stream {
			t.Fatal("heavyweight should not be called with stream=true in collector")
		}

		resp := protocol.ChatCompletionResponse{
			Choices: []protocol.Choice{
				{Message: &protocol.Message{Content: "Hello world!"}},
			},
			Usage: &protocol.Usage{
				CompletionTokens: 3,
			},
		}

		isJudge := false
		for _, m := range req.Messages {
			if strings.Contains(m.Content, "evaluation judge") {
				isJudge = true
				break
			}
		}

		if isJudge {
			resp = protocol.ChatCompletionResponse{
				Choices: []protocol.Choice{
					{Message: &protocol.Message{Content: `{"acceptable": true, "score": 4, "reasoning": "good match"}`}},
				},
			}
		}

		json.NewEncoder(w).Encode(resp)
	}))
	defer heavySrv.Close()

	drafter := client.NewOpenAICompatibleClient(drafterSrv.URL, "key", "model", "test-drafter", 10*time.Second)
	heavy := client.NewOpenAICompatibleClient(heavySrv.URL, "key", "model", "test-heavy", 10*time.Second)
	judge := NewJudge(heavy)

	collector := NewCollector(drafter, heavy, judge, CollectorConfig{
		TopLogprobs: 5,
		MaxTokens:   100,
	})

	prompt := dataset.Prompt{
		ID:       "test_001",
		Category: "simple_factual",
		Text:     "Say hello",
	}

	record, err := collector.Collect(context.Background(), prompt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if record.Error != "" {
		t.Fatalf("unexpected record error: %s", record.Error)
	}

	if record.PromptID != "test_001" {
		t.Errorf("expected prompt ID test_001, got %s", record.PromptID)
	}

	if record.DraftResponse != "Hello world" {
		t.Errorf("expected draft response 'Hello world', got %q", record.DraftResponse)
	}

	if record.HeavyResponse != "Hello world!" {
		t.Errorf("expected heavy response 'Hello world!', got %q", record.HeavyResponse)
	}

	if len(record.Tokens) != 2 {
		t.Errorf("expected 2 token records, got %d", len(record.Tokens))
	}

	if record.TokenCount != 2 {
		t.Errorf("expected token count 2, got %d", record.TokenCount)
	}

	if record.Judge.Score != 4 {
		t.Errorf("expected judge score 4, got %d", record.Judge.Score)
	}

	if !record.Judge.Acceptable {
		t.Error("expected judge acceptable=true")
	}

	if record.MeanEntropy <= 0 {
		t.Error("expected positive mean entropy")
	}

	if record.HeavyTokensUsed != 3 {
		t.Errorf("expected 3 heavy tokens used, got %d", record.HeavyTokensUsed)
	}

	if record.CollectedAt == "" {
		t.Error("expected non-empty collected_at timestamp")
	}
}
