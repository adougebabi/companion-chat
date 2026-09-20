package prompt

// PromptSlotID is a task-owned prompt slot identifier. It is intentionally a
// different type from CapabilityContextSlot and the durable evolution slots.
type PromptSlotID string

type PromptSlotPosition string

const (
	PromptSlotSystem         PromptSlotPosition = "system"
	PromptSlotRuntime        PromptSlotPosition = "runtime"
	PromptSlotRecent         PromptSlotPosition = "recent"
	PromptSlotCurrentInput   PromptSlotPosition = "current_input"
	PromptSlotTools          PromptSlotPosition = "tools"
	PromptSlotResponseSchema PromptSlotPosition = "response_schema"
)

const (
	PromptSlotCorePersona PromptSlotID = "core_persona"
	PromptSlotOperation   PromptSlotID = "operation"
	PromptSlotRuntimeFact PromptSlotID = "runtime_fact"
	PromptSlotMemory      PromptSlotID = "memory"
	PromptSlotRecentTurns PromptSlotID = "recent_turns"
	PromptSlotCurrent     PromptSlotID = "current_input"
	PromptSlotToolCatalog PromptSlotID = "tool_catalog"
	PromptSlotSchema      PromptSlotID = "response_schema"
)

type PromptFragmentKind string

const (
	PromptFragmentRuntimeFact     PromptFragmentKind = "runtime_fact"
	PromptFragmentActiveMemory    PromptFragmentKind = "active_memory"
	PromptFragmentRecentMessage   PromptFragmentKind = "recent_message"
	PromptFragmentRetrievedMemory PromptFragmentKind = "retrieved_memory"
	PromptFragmentSummary         PromptFragmentKind = "conversation_summary"
)

type PromptFragment struct {
	Kind            PromptFragmentKind `json:"kind"`
	SlotID          PromptSlotID       `json:"slot_id,omitempty"`
	Position        PromptSlotPosition `json:"position,omitempty"`
	Order           int                `json:"order,omitempty"`
	BudgetTokens    int                `json:"budget_tokens,omitempty"`
	Priority        int                `json:"priority"`
	Required        bool               `json:"required"`
	Content         any                `json:"content"`
	EstimatedTokens int                `json:"estimated_tokens"`
	SourceRefs      []string           `json:"source_refs"`
	GroupKey        string             `json:"group_key,omitempty"`
}

// PromptSlot declares one independently selectable region. A task includes
// only the slots it needs; adding a new slot therefore cannot affect another
// task's prompt by accident.
type PromptSlot struct {
	ID           PromptSlotID
	Position     PromptSlotPosition
	Order        int
	Required     bool
	BudgetTokens int
	Fragments    []PromptFragment
}
