package conversation

import (
	"context"
	"time"
)

type RawHistoryKind string

const (
	RawHistoryConversationMessage RawHistoryKind = "conversation_message"
	RawHistoryCognitionFact       RawHistoryKind = "cognition_fact"
	RawHistoryCapabilityOutcome   RawHistoryKind = "capability_outcome"

	DefaultRawHistoryLimit = 50
	MaxRawHistoryLimit     = 200
	MaxRawHistorySources   = 64
	MaxRawHistoryQueryRune = 2000
)

type RawHistoryEvent struct {
	SourceRef      string
	Kind           RawHistoryKind
	FluctlightID   string
	ConversationID string
	ActorID        string
	OccurredAt     time.Time
	Sequence       int64
	Content        map[string]any
	Authority      string
	Relevance      float64
}

type RawHistoryQuery struct {
	AuthorizationActorID string
	FluctlightID         string
	ConversationID       string
	BeforeOccurredAt     *time.Time
	BeforeSourceRef      string
	Limit                int
}

type RawHistorySearchQuery struct {
	AuthorizationActorID string
	FluctlightID         string
	ConversationID       string
	Query                string
	Limit                int
}

type RawHistorySourceQuery struct {
	AuthorizationActorID string
	FluctlightID         string
	ConversationID       string
	SourceRefs           []string
}

type RawHistoryReader interface {
	Recent(context.Context, RawHistoryQuery) ([]RawHistoryEvent, error)
	Search(context.Context, RawHistorySearchQuery) ([]RawHistoryEvent, error)
	ReadSources(context.Context, RawHistorySourceQuery) ([]RawHistoryEvent, error)
}

