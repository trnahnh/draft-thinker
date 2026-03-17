package gateway

import (
	"testing"

	"github.com/trnahnh/draft-thinker/pkg/protocol"
)

func TestAssembleResponse_Basic(t *testing.T) {
	finish := "stop"
	chunks := []protocol.StreamChunk{
		{
			ID: "chunk-1", Object: "chat.completion.chunk", Model: "llama3-8b-8192",
			Choices: []protocol.Choice{{Index: 0, Delta: &protocol.Delta{Content: "Hello"}}},
		},
		{
			ID: "chunk-2", Object: "chat.completion.chunk", Model: "llama3-8b-8192",
			Choices: []protocol.Choice{{Index: 0, Delta: &protocol.Delta{Content: " world"}}},
		},
		{
			ID: "chunk-3", Object: "chat.completion.chunk", Model: "llama3-8b-8192",
			Choices: []protocol.Choice{{Index: 0, Delta: &protocol.Delta{Content: "!"}, FinishReason: &finish}},
		},
	}

	resp := assembleResponse(chunks)

	if resp.ID != "chunk-1" {
		t.Errorf("id: got %q, want %q", resp.ID, "chunk-1")
	}
	if resp.Model != "llama3-8b-8192" {
		t.Errorf("model: got %q, want %q", resp.Model, "llama3-8b-8192")
	}
	if resp.Object != "chat.completion" {
		t.Errorf("object: got %q, want %q", resp.Object, "chat.completion")
	}
	if len(resp.Choices) != 1 {
		t.Fatalf("choices count: got %d, want 1", len(resp.Choices))
	}
	if resp.Choices[0].Message.Content != "Hello world!" {
		t.Errorf("content: got %q, want %q", resp.Choices[0].Message.Content, "Hello world!")
	}
	if resp.Choices[0].Message.Role != "assistant" {
		t.Errorf("role: got %q, want %q", resp.Choices[0].Message.Role, "assistant")
	}
	if resp.Choices[0].FinishReason == nil || *resp.Choices[0].FinishReason != "stop" {
		t.Error("finish_reason should be 'stop'")
	}
}

func TestAssembleResponse_Empty(t *testing.T) {
	resp := assembleResponse(nil)
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if len(resp.Choices) != 0 {
		t.Errorf("expected 0 choices, got %d", len(resp.Choices))
	}
}

func TestAssembleResponse_NilDelta(t *testing.T) {
	chunks := []protocol.StreamChunk{
		{
			ID: "chunk-1", Object: "chat.completion.chunk", Model: "llama3-8b-8192",
			Choices: []protocol.Choice{{Index: 0, Delta: &protocol.Delta{Role: "assistant"}}},
		},
		{
			ID: "chunk-2", Object: "chat.completion.chunk", Model: "llama3-8b-8192",
			Choices: []protocol.Choice{{Index: 0, Delta: &protocol.Delta{Content: "Hi"}}},
		},
	}

	resp := assembleResponse(chunks)
	if len(resp.Choices) != 1 {
		t.Fatalf("choices count: got %d, want 1", len(resp.Choices))
	}
	if resp.Choices[0].Message.Content != "Hi" {
		t.Errorf("content: got %q, want %q", resp.Choices[0].Message.Content, "Hi")
	}
}
