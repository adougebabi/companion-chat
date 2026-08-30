package core

import "time"

type Fluctlight struct {
	ID               string         `json:"id"`
	Identity         map[string]any `json:"identity"`
	Personality      map[string]any `json:"personality"`
	BehavioralPolicy map[string]any `json:"behavioral_policy"`
	LifeProfile      map[string]any `json:"life_profile"`
	Provenance       map[string]any `json:"provenance"`
	Status           string         `json:"status"`
	CurrentRevision  int            `json:"current_revision"`
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
	ConversationID string    `json:"conversation_id"`
	ActorID        string    `json:"actor_id"`
	Role           string    `json:"role"`
	Status         string    `json:"status"`
	JoinedAt       time.Time `json:"joined_at"`
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
