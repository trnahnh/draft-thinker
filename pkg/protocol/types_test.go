package protocol

import (
	"encoding/json"
	"testing"
)

func TestChatCompletionRequest_RoundTrip(t *testing.T) {
	temp := 0.7
	maxTok := 100
	req := ChatCompletionRequest{
		Model: "llama3-8b-8192",
		Messages: []Message{
			{Role: "system", Content: "You are helpful."},
			{Role: "user", Content: "Hello"},
		},
		Stream:      true,
		Temperature: &temp,
		MaxTokens:   &maxTok,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got ChatCompletionRequest
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Model != req.Model {
		t.Errorf("model: got %q, want %q", got.Model, req.Model)
	}
	if len(got.Messages) != 2 {
		t.Fatalf("messages count: got %d, want 2", len(got.Messages))
	}
	if got.Messages[0].Role != "system" {
		t.Errorf("messages[0].role: got %q, want %q", got.Messages[0].Role, "system")
	}
	if !got.Stream {
		t.Error("stream should be true")
	}
	if *got.Temperature != 0.7 {
		t.Errorf("temperature: got %f, want 0.7", *got.Temperature)
	}
	if *got.MaxTokens != 100 {
		t.Errorf("max_tokens: got %d, want 100", *got.MaxTokens)
	}
}

func TestChatCompletionRequest_StreamOmitted(t *testing.T) {
	req := ChatCompletionRequest{
		Model:    "llama3-8b-8192",
		Messages: []Message{{Role: "user", Content: "Hi"}},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	raw := string(data)
	if json.Valid(data) {
		var m map[string]interface{}
		json.Unmarshal(data, &m)
		if _, ok := m["stream"]; ok {
			t.Errorf("stream should be omitted when false, got: %s", raw)
		}
	}
}

func TestChatCompletionResponse_RoundTrip(t *testing.T) {
	finish := "stop"
	resp := ChatCompletionResponse{
		ID:      "chatcmpl-123",
		Object:  "chat.completion",
		Created: 1700000000,
		Model:   "llama3-8b-8192",
		Choices: []Choice{
			{
				Index:        0,
				Message:      &Message{Role: "assistant", Content: "Hello!"},
				FinishReason: &finish,
			},
		},
		Usage: &Usage{
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got ChatCompletionResponse
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.ID != resp.ID {
		t.Errorf("id: got %q, want %q", got.ID, resp.ID)
	}
	if got.Choices[0].Message.Content != "Hello!" {
		t.Errorf("content: got %q, want %q", got.Choices[0].Message.Content, "Hello!")
	}
	if got.Usage.TotalTokens != 15 {
		t.Errorf("total_tokens: got %d, want 15", got.Usage.TotalTokens)
	}
}

func TestStreamChunk_RoundTrip(t *testing.T) {
	chunk := StreamChunk{
		ID:      "chatcmpl-123",
		Object:  "chat.completion.chunk",
		Created: 1700000000,
		Model:   "llama3-8b-8192",
		Choices: []Choice{
			{
				Index: 0,
				Delta: &Delta{Content: "Hello"},
			},
		},
	}

	data, err := json.Marshal(chunk)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got StreamChunk
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Choices[0].Delta.Content != "Hello" {
		t.Errorf("delta content: got %q, want %q", got.Choices[0].Delta.Content, "Hello")
	}
}

func TestChatCompletionRequest_LogprobsRoundTrip(t *testing.T) {
	logprobs := true
	topLogprobs := 5
	req := ChatCompletionRequest{
		Model:       "llama3-8b-8192",
		Messages:    []Message{{Role: "user", Content: "Hello"}},
		Logprobs:    &logprobs,
		TopLogprobs: &topLogprobs,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got ChatCompletionRequest
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Logprobs == nil || !*got.Logprobs {
		t.Error("logprobs should be true")
	}
	if got.TopLogprobs == nil || *got.TopLogprobs != 5 {
		t.Errorf("top_logprobs: got %v, want 5", got.TopLogprobs)
	}
}

func TestChoiceLogprobs_RoundTrip(t *testing.T) {
	finish := "stop"
	resp := ChatCompletionResponse{
		ID:      "chatcmpl-lp",
		Object:  "chat.completion",
		Created: 1700000000,
		Model:   "llama3-8b-8192",
		Choices: []Choice{
			{
				Index:        0,
				Message:      &Message{Role: "assistant", Content: "4"},
				FinishReason: &finish,
				Logprobs: &ChoiceLogprobs{
					Content: []TokenLogprob{
						{
							Token:   "4",
							Logprob: -0.0001,
							TopLogprobs: []TopLogprobEntry{
								{Token: "4", Logprob: -0.0001},
								{Token: "four", Logprob: -9.21},
								{Token: "Four", Logprob: -11.5},
							},
						},
					},
				},
			},
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got ChatCompletionResponse
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Choices[0].Logprobs == nil {
		t.Fatal("logprobs should not be nil")
	}
	lp := got.Choices[0].Logprobs.Content[0]
	if lp.Token != "4" {
		t.Errorf("token: got %q, want %q", lp.Token, "4")
	}
	if lp.Logprob != -0.0001 {
		t.Errorf("logprob: got %f, want -0.0001", lp.Logprob)
	}
	if len(lp.TopLogprobs) != 3 {
		t.Fatalf("top_logprobs count: got %d, want 3", len(lp.TopLogprobs))
	}
	if lp.TopLogprobs[1].Token != "four" {
		t.Errorf("top_logprobs[1].token: got %q, want %q", lp.TopLogprobs[1].Token, "four")
	}
}

func TestErrorResponse_JSON(t *testing.T) {
	code := "invalid_request_error"
	errResp := ErrorResponse{
		Error: ErrorDetail{
			Message: "Invalid model",
			Type:    "invalid_request_error",
			Code:    &code,
		},
	}

	data, err := json.Marshal(errResp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got ErrorResponse
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.Error.Message != "Invalid model" {
		t.Errorf("message: got %q, want %q", got.Error.Message, "Invalid model")
	}
	if *got.Error.Code != code {
		t.Errorf("code: got %q, want %q", *got.Error.Code, code)
	}
}
