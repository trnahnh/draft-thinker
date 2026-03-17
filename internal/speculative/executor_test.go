package speculative

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/trnahnh/draft-thinker/internal/entropy"
	"github.com/trnahnh/draft-thinker/internal/metrics"
	"github.com/trnahnh/draft-thinker/pkg/client"
	"github.com/trnahnh/draft-thinker/pkg/protocol"
)

func confidentChunk(token string) protocol.StreamChunk {
	return protocol.StreamChunk{
		ID: "chunk-c", Object: "chat.completion.chunk", Model: "gpt-4.1-nano",
		Choices: []protocol.Choice{
			{
				Index: 0,
				Delta: &protocol.Delta{Content: token},
				Logprobs: &protocol.ChoiceLogprobs{
					Content: []protocol.TokenLogprob{
						{
							Token:   token,
							Logprob: math.Log(0.95),
							TopLogprobs: []protocol.TopLogprobEntry{
								{Token: token, Logprob: math.Log(0.95)},
								{Token: "alt1", Logprob: math.Log(0.03)},
								{Token: "alt2", Logprob: math.Log(0.02)},
							},
						},
					},
				},
			},
		},
	}
}

func uncertainChunk(token string) protocol.StreamChunk {
	return protocol.StreamChunk{
		ID: "chunk-u", Object: "chat.completion.chunk", Model: "gpt-4.1-nano",
		Choices: []protocol.Choice{
			{
				Index: 0,
				Delta: &protocol.Delta{Content: token},
				Logprobs: &protocol.ChoiceLogprobs{
					Content: []protocol.TokenLogprob{
						{
							Token:   token,
							Logprob: math.Log(0.34),
							TopLogprobs: []protocol.TopLogprobEntry{
								{Token: token, Logprob: math.Log(0.34)},
								{Token: "alt1", Logprob: math.Log(0.33)},
								{Token: "alt2", Logprob: math.Log(0.33)},
							},
						},
					},
				},
			},
		},
	}
}

// midEntropyChunk generates a chunk with entropy above softThreshold (0.8) but below hardThreshold (1.0).
// Probs 0.80/0.12/0.08 give Shannon entropy ~0.92 bits.
func midEntropyChunk(token string) protocol.StreamChunk {
	return protocol.StreamChunk{
		ID: "chunk-m", Object: "chat.completion.chunk", Model: "gpt-4.1-nano",
		Choices: []protocol.Choice{
			{
				Index: 0,
				Delta: &protocol.Delta{Content: token},
				Logprobs: &protocol.ChoiceLogprobs{
					Content: []protocol.TokenLogprob{
						{
							Token:   token,
							Logprob: math.Log(0.80),
							TopLogprobs: []protocol.TopLogprobEntry{
								{Token: token, Logprob: math.Log(0.80)},
								{Token: "alt1", Logprob: math.Log(0.12)},
								{Token: "alt2", Logprob: math.Log(0.08)},
							},
						},
					},
				},
			},
		},
	}
}

type mockHeavyweight struct {
	streamChunks []protocol.StreamChunk
	streamErr    error
	streamCalled bool
}

func (m *mockHeavyweight) Complete(_ context.Context, _ *protocol.ChatCompletionRequest) (*protocol.ChatCompletionResponse, error) {
	return nil, nil
}

func (m *mockHeavyweight) Stream(_ context.Context, _ *protocol.ChatCompletionRequest) (<-chan client.StreamChunk, error) {
	m.streamCalled = true
	if m.streamErr != nil {
		return nil, m.streamErr
	}
	ch := make(chan client.StreamChunk, len(m.streamChunks))
	for i := range m.streamChunks {
		ch <- client.StreamChunk{Chunk: &m.streamChunks[i]}
	}
	close(ch)
	return ch, nil
}

func defaultCfg() ExecutorConfig {
	return ExecutorConfig{
		WindowCfg: entropy.WindowConfig{
			Size:           5,
			Threshold:      1.0,
			EarlyExitCount: 3,
		},
		SoftThresholdMult: 0.8,
	}
}

func sendChunks(chunks []protocol.StreamChunk) <-chan client.StreamChunk {
	ch := make(chan client.StreamChunk, len(chunks))
	for i := range chunks {
		ch <- client.StreamChunk{Chunk: &chunks[i]}
	}
	close(ch)
	return ch
}

func testReq() *protocol.ChatCompletionRequest {
	return &protocol.ChatCompletionRequest{
		Model:    "auto",
		Messages: []protocol.Message{{Role: "user", Content: "test"}},
	}
}

func TestExecute_DraftAccepted_NoSoftThreshold(t *testing.T) {
	hw := &mockHeavyweight{}
	exec := NewExecutor(defaultCfg(), hw, &metrics.NoopRecorder{})

	chunks := []protocol.StreamChunk{
		confidentChunk("Hello"),
		confidentChunk(" world"),
	}
	drafterCh := sendChunks(chunks)

	result, err := exec.Execute(context.Background(), drafterCh, testReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.State != DraftAccepted {
		t.Errorf("state: got %v, want DraftAccepted", result.State)
	}
	if len(result.DraftChunks) != 2 {
		t.Errorf("chunks: got %d, want 2", len(result.DraftChunks))
	}
	if hw.streamCalled {
		t.Error("heavyweight should not have been called")
	}
	if result.HeavyCh != nil {
		t.Error("HeavyCh should be nil")
	}
}

func TestExecute_DraftAccepted_SoftTriggeredThenRecovered(t *testing.T) {
	hw := &mockHeavyweight{
		streamChunks: []protocol.StreamChunk{
			{ID: "hw-1", Object: "chat.completion.chunk", Model: "gpt-4.1",
				Choices: []protocol.Choice{{Index: 0, Delta: &protocol.Delta{Content: "Heavy"}}}},
		},
	}
	exec := NewExecutor(defaultCfg(), hw, &metrics.NoopRecorder{})

	chunks := []protocol.StreamChunk{
		confidentChunk("Hello"),
		midEntropyChunk("maybe"),
		confidentChunk(" world"),
		confidentChunk(" again"),
		confidentChunk(" more"),
		confidentChunk(" text"),
	}
	drafterCh := sendChunks(chunks)

	result, err := exec.Execute(context.Background(), drafterCh, testReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.State != DraftAccepted {
		t.Errorf("state: got %v, want DraftAccepted", result.State)
	}
	if !hw.streamCalled {
		t.Error("heavyweight should have been called speculatively")
	}
	if result.HeavyCh != nil {
		t.Error("HeavyCh should be nil after cancellation")
	}
}

func TestExecute_Escalated_WithSpeculation(t *testing.T) {
	hw := &mockHeavyweight{
		streamChunks: []protocol.StreamChunk{
			{ID: "hw-1", Object: "chat.completion.chunk", Model: "gpt-4.1",
				Choices: []protocol.Choice{{Index: 0, Delta: &protocol.Delta{Content: "Heavy"}}}},
		},
	}
	exec := NewExecutor(defaultCfg(), hw, &metrics.NoopRecorder{})

	chunks := []protocol.StreamChunk{
		midEntropyChunk("hmm"),
		uncertainChunk("well"),
		uncertainChunk("uh"),
	}
	drafterCh := sendChunks(chunks)

	result, err := exec.Execute(context.Background(), drafterCh, testReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.State != Escalated {
		t.Errorf("state: got %v, want Escalated", result.State)
	}
	if !hw.streamCalled {
		t.Error("heavyweight should have been called")
	}
	if result.HeavyCh == nil {
		t.Error("HeavyCh should be non-nil for speculative escalation")
	}
	if result.HeavyCancel == nil {
		t.Error("HeavyCancel should be non-nil")
	}
	if result.HeavyCancel != nil {
		result.HeavyCancel()
	}
}

func TestExecute_Escalated_WithoutSpeculation(t *testing.T) {
	hw := &mockHeavyweight{}
	exec := NewExecutor(defaultCfg(), hw, &metrics.NoopRecorder{})

	// All uncertain tokens trigger early exit before soft threshold fires
	// (early exit checks individual token entropy > threshold, which happens
	// on the first token before soft threshold logic runs)
	chunks := []protocol.StreamChunk{
		uncertainChunk("well"),
		uncertainChunk("uh"),
		uncertainChunk("hmm"),
	}
	drafterCh := sendChunks(chunks)

	result, err := exec.Execute(context.Background(), drafterCh, testReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.State != Escalated {
		t.Errorf("state: got %v, want Escalated", result.State)
	}
	if result.HeavyCh != nil {
		t.Error("HeavyCh should be nil for early-exit escalation")
	}
}

func TestExecute_HeavyweightStartError(t *testing.T) {
	hw := &mockHeavyweight{
		streamErr: errors.New("connection refused"),
	}
	exec := NewExecutor(defaultCfg(), hw, &metrics.NoopRecorder{})

	// midEntropy triggers soft threshold, but heavyweight fails to start.
	// Subsequent confident tokens let drafter complete.
	chunks := []protocol.StreamChunk{
		midEntropyChunk("hmm"),
		confidentChunk("ok"),
		confidentChunk("fine"),
		confidentChunk("sure"),
		confidentChunk("yes"),
		confidentChunk("done"),
	}
	drafterCh := sendChunks(chunks)

	result, err := exec.Execute(context.Background(), drafterCh, testReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.State != DraftAccepted {
		t.Errorf("state: got %v, want DraftAccepted (graceful fallback)", result.State)
	}
}

func TestExecute_DrafterStreamError(t *testing.T) {
	hw := &mockHeavyweight{}
	exec := NewExecutor(defaultCfg(), hw, &metrics.NoopRecorder{})

	ch := make(chan client.StreamChunk, 3)
	c := confidentChunk("ok")
	ch <- client.StreamChunk{Chunk: &c}
	ch <- client.StreamChunk{Err: errors.New("stream broke")}
	close(ch)

	_, err := exec.Execute(context.Background(), ch, testReq())
	if err == nil {
		t.Fatal("expected error from drafter stream")
	}
}

func TestExecute_DrafterStreamError_WhileSpeculating(t *testing.T) {
	hw := &mockHeavyweight{
		streamChunks: []protocol.StreamChunk{
			{ID: "hw-1", Object: "chat.completion.chunk", Model: "gpt-4.1",
				Choices: []protocol.Choice{{Index: 0, Delta: &protocol.Delta{Content: "Heavy"}}}},
		},
	}
	exec := NewExecutor(defaultCfg(), hw, &metrics.NoopRecorder{})

	ch := make(chan client.StreamChunk, 3)
	c := midEntropyChunk("hmm")
	ch <- client.StreamChunk{Chunk: &c}
	ch <- client.StreamChunk{Err: errors.New("stream broke")}
	close(ch)

	_, err := exec.Execute(context.Background(), ch, testReq())
	if err == nil {
		t.Fatal("expected error from drafter stream")
	}
}

func TestExecute_ContextCancellation(t *testing.T) {
	hw := &mockHeavyweight{}
	exec := NewExecutor(defaultCfg(), hw, &metrics.NoopRecorder{})

	ch := make(chan client.StreamChunk)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	_, err := exec.Execute(ctx, ch, testReq())
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
}
