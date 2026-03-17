package client

import (
	"context"

	"github.com/trnahnh/draft-thinker/pkg/protocol"
)

type StreamChunk struct {
	Chunk *protocol.StreamChunk
	Err   error
}

type LLMClient interface {
	Complete(ctx context.Context, req *protocol.ChatCompletionRequest) (*protocol.ChatCompletionResponse, error)
	Stream(ctx context.Context, req *protocol.ChatCompletionRequest) (<-chan StreamChunk, error)
}
