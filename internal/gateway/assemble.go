package gateway

import (
	"sort"
	"strings"
	"time"

	"github.com/trnahnh/draft-thinker/pkg/protocol"
)

func assembleResponse(chunks []protocol.StreamChunk) *protocol.ChatCompletionResponse {
	if len(chunks) == 0 {
		return &protocol.ChatCompletionResponse{}
	}

	first := chunks[0]
	resp := &protocol.ChatCompletionResponse{
		ID:      first.ID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   first.Model,
	}

	choiceContents := make(map[int]*strings.Builder)
	var finishReason *string

	for _, chunk := range chunks {
		for _, choice := range chunk.Choices {
			if _, ok := choiceContents[choice.Index]; !ok {
				choiceContents[choice.Index] = &strings.Builder{}
			}
			if choice.Delta != nil {
				choiceContents[choice.Index].WriteString(choice.Delta.Content)
			}
			if choice.FinishReason != nil {
				fr := *choice.FinishReason
				finishReason = &fr
			}
		}
	}

	indices := make([]int, 0, len(choiceContents))
	for idx := range choiceContents {
		indices = append(indices, idx)
	}
	sort.Ints(indices)

	for _, idx := range indices {
		resp.Choices = append(resp.Choices, protocol.Choice{
			Index: idx,
			Message: &protocol.Message{
				Role:    "assistant",
				Content: choiceContents[idx].String(),
			},
			FinishReason: finishReason,
		})
	}

	return resp
}
