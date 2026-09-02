package core

import "time"

type Fluctlight struct {
	ID                 string         `json:"id"`
	CorePersona        map[string]any `json:"core_persona"`
	Identity           map[string]any `json:"identity"`
	Personality        map[string]any `json:"personality"`
	BehavioralPolicy   map[string]any `json:"behavioral_policy"`
	LifeProfile        map[string]any `json:"life_profile"`
	Provenance         map[string]any `json:"provenance"`
	Status             string         `json:"status"`
	CurrentRevision    int            `json:"current_revision"`
	UnreadCount        int            `json:"unread_count,omitempty"`
	LastConversationAt *time.Time     `json:"last_conversation_at,omitempty"`
}

// DevelopingSelfClaim is an evidence-backed, non-authoritative observation
// about what a Fluctlight is gradually learning about itself. It is
// intentionally separate from Fluctlight's CorePersona and from transient
// inner state.
type DevelopingSelfClaim struct {
	ID           string         `json:"id"`
	FluctlightID string         `json:"fluctlight_id"`
	Category     string         `json:"category"`
	Claim        string         `json:"claim"`
	Value        any            `json:"value"`
	Confidence   float64        `json:"confidence"`
	EvidenceRefs []string       `json:"evidence_refs"`
	Provenance   map[string]any `json:"provenance"`
	Status       string         `json:"status"`
	ExpiresAt    *time.Time     `json:"expires_at,omitempty"`
	Revision     int            `json:"revision"`
	SupersededBy *string        `json:"superseded_by,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}

type Conversation struct {
	ID             string    `json:"id"`
	CreatedByActor string    `json:"created_by_actor_id"`
	Title          *string   `json:"title"`
	Revision       int       `json:"revision"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type Participant struct {
	ConversationID string     `json:"conversation_id"`
	ActorID        string     `json:"actor_id"`
	Role           string     `json:"role"`
	Status         string     `json:"status"`
	JoinedAt       time.Time  `json:"joined_at"`
	LeftAt         *time.Time `json:"left_at"`
}

type Message struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversation_id"`
	Sequence       int       `json:"sequence"`
	AuthorActorID  string    `json:"author_actor_id"`
	Kind           string    `json:"kind"`
	Text           string    `json:"text"`
	AttachmentRefs []string  `json:"attachment_refs"`
	CreatedAt      time.Time `json:"created_at"`
}

type ConversationPage struct {
	Conversation Conversation  `json:"conversation"`
	Participants []Participant `json:"participants"`
	Messages     []Message     `json:"messages"`
	NextBefore   *int          `json:"next_before_sequence"`
}
