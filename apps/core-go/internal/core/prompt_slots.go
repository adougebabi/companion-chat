package core

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/cloudwego/eino/schema"
)

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

type PromptCompositionInput struct {
	System         string
	CurrentInput   string
	Slots          []PromptSlot
	Tools          []map[string]any
	ResponseFormat map[string]any
}

type PromptCompositionResult struct {
	Messages []*schema.Message
	Trace    PromptAssemblyTrace
}

// PromptComposer is a pure renderer. It accepts already-resolved business
// facts and never reaches into App, repositories, Redis or domain state.
type PromptComposer struct {
	Policy PromptBudgetPolicy
}

func NewPromptComposer(policy PromptBudgetPolicy) (*PromptComposer, error) {
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	return &PromptComposer{Policy: policy}, nil
}

// ComposeAssembly is the production bridge for the established Core
// projection assembler. It keeps one budget/selection owner while making the
// Eino-facing Composer explicit at every Provider egress.
func (c *PromptComposer) ComposeAssembly(input PromptAssemblyInput) (PromptAssemblyResult, error) {
	if c == nil {
		return PromptAssemblyResult{}, errors.New("prompt_composer_unavailable")
	}
	if input.Policy != c.Policy {
		return PromptAssemblyResult{}, errors.New("prompt_composer_policy_mismatch")
	}
	return AssemblePromptContext(input)
}

// ComposeTaskMessages is the operation-task surface for non-assembled calls
// (initialization/text/media). It keeps the legacy wire-shaping behavior under
// the same Composer owner while those tasks are incrementally converted to
// explicit slots.
func (c *PromptComposer) ComposeTaskMessages(role string, messages []map[string]any) []map[string]any {
	return composeTaskMessages(role, messages)
}

func (c *PromptComposer) Compose(ctx context.Context, input PromptCompositionInput) (PromptCompositionResult, error) {
	if c == nil {
		return PromptCompositionResult{}, errors.New("prompt_composer_unavailable")
	}
	if err := ctx.Err(); err != nil {
		return PromptCompositionResult{}, err
	}
	if strings.TrimSpace(input.System) == "" || strings.TrimSpace(input.CurrentInput) == "" {
		return PromptCompositionResult{}, errors.New("prompt_composer_required_input")
	}
	slots := append([]PromptSlot(nil), input.Slots...)
	sort.SliceStable(slots, func(i, j int) bool {
		if slots[i].Order != slots[j].Order {
			return slots[i].Order < slots[j].Order
		}
		return slots[i].ID < slots[j].ID
	})
	var working WorkingMemory
	for _, slot := range slots {
		if slot.Required && len(slot.Fragments) == 0 && slot.ID != PromptSlotCurrent && slot.ID != PromptSlotToolCatalog && slot.ID != PromptSlotSchema {
			return PromptCompositionResult{}, errors.New("prompt_required_slot_empty")
		}
		fragments := append([]PromptFragment(nil), slot.Fragments...)
		if slot.BudgetTokens > 0 {
			used := 0
			bounded := make([]PromptFragment, 0, len(fragments))
			for _, candidate := range fragments {
				cost := candidate.EstimatedTokens
				if cost <= 0 {
					cost = EstimatePromptTokens(candidate.Content) + 4
				}
				if used+cost > slot.BudgetTokens {
					if slot.Required || candidate.Required {
						return PromptCompositionResult{}, ErrPromptRequiredBudgetExceeded
					}
					continue
				}
				used += cost
				bounded = append(bounded, candidate)
			}
			fragments = bounded
		}
		for _, fragment := range fragments {
			fragment.SlotID = slot.ID
			fragment.Position = slot.Position
			fragment.Order = slot.Order
			if fragment.BudgetTokens <= 0 {
				fragment.BudgetTokens = slot.BudgetTokens
			}
			switch fragment.Kind {
			case PromptFragmentRuntimeFact:
				working.RuntimeFacts = append(working.RuntimeFacts, fragment)
			case PromptFragmentActiveMemory:
				working.Active = append(working.Active, fragment)
			case PromptFragmentRecentMessage:
				working.Recent = append(working.Recent, fragment)
			case PromptFragmentRetrievedMemory:
				working.Retrieved = append(working.Retrieved, fragment)
			case PromptFragmentSummary:
				working.Summaries = append(working.Summaries, fragment)
			default:
				if slot.Required {
					return PromptCompositionResult{}, errors.New("prompt_slot_fragment_kind_invalid")
				}
			}
		}
	}
	// Reuse the established whole-turn/budget algorithm as the single
	// selection authority, then convert the result to Eino messages exactly
	// once. The legacy map-shaped result is not sent to a provider.
	assembled, err := AssemblePromptContext(PromptAssemblyInput{
		OperationRules: []string{input.System},
		WorkingMemory:  working,
		CurrentInput:   input.CurrentInput,
		Tools:          input.Tools,
		ResponseFormat: input.ResponseFormat,
		Policy:         c.Policy,
	})
	if err != nil {
		return PromptCompositionResult{}, err
	}
	messages, err := providerMessagesToEino(assembled.Messages)
	if err != nil {
		return PromptCompositionResult{}, err
	}
	return PromptCompositionResult{Messages: messages, Trace: assembled.Trace}, nil
}
