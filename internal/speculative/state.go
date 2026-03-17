package speculative

import (
	"context"

	"github.com/trnahnh/draft-thinker/internal/entropy"
	"github.com/trnahnh/draft-thinker/pkg/client"
	"github.com/trnahnh/draft-thinker/pkg/protocol"
)

type State int

const (
	Drafting State = iota
	Speculating
	Escalated
	DraftAccepted
)

func (s State) String() string {
	switch s {
	case Drafting:
		return "drafting"
	case Speculating:
		return "speculating"
	case Escalated:
		return "escalated"
	case DraftAccepted:
		return "draft_accepted"
	default:
		return "unknown"
	}
}

type Result struct {
	State       State
	DraftChunks []protocol.StreamChunk
	TokenCount  int
	Entropies   []entropy.TokenEntropy
	HeavyCh     <-chan client.StreamChunk
	HeavyCancel context.CancelFunc
}
