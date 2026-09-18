package core

import (
	"context"
	"errors"
	"strings"
	"time"
)

// The Main generation and the takeover reply must be normalized by exactly the
// same code. Keeping one function is what makes "B passed the same validation
// chain as A" a structural fact instead of a promise (design.md 4.8, R02/R04).
//
// The single visible-text resolution (R05/F02) also lives here: it is the only
// place that decides which candidate text is authoritative, and it freezes the
// answer on the decision so the candidate preview, the takeover Judge and the
// settlement INSERT all read the same string.

// turnDecisionNormalizationInput is everything the shared normalizer needs.
type turnDecisionNormalizationInput struct {
	InboxID        string
	FluctlightID   string
	ConversationID string
	TurnID         string
	Projection     ContextProjection
	Grant          persistentSwitchGrant
	Decision       map[string]any
	Invocations    []CapabilityInvocation
	Definitions    []CapabilityDefinition
	// StructuredFallback reports that the Provider answered in the structured
	// fallback shape, which changes how an empty reply is interpreted.
	StructuredFallback bool
	// ContinuationBaseMessages is frozen onto a query_continuation decision so
	// the continuation worker can rebuild its request.
	ContinuationBaseMessages []map[string]any
	// ForbidQueryContinuation marks the takeover reply. A second generation may
	// not spend a third call on a result-dependent continuation (R06/F06).
	ForbidQueryContinuation bool
}

// turnDecisionNormalization is the normalized candidate.
type turnDecisionNormalization struct {
	Decision                 map[string]any
	Invocations              []CapabilityInvocation
	ResponsePlan             map[string]any
	ResponseMode             string
	Action                   string
	Composite                CompositeActionV1
	PersonalityPlan          *personalityDecisionPlan
	Canonical                canonicalVisibleReply
	ToolOnlyNoReply          bool
	ContinuationBaseMessages []map[string]any
}

// takeoverReplyBudgetExhaustedCode is the stable failure code recorded on the
// frozen turn. It is also the error text so the code a test asserts is the same
// string an operator greps for (F06).
const takeoverReplyBudgetExhaustedCode = "takeover_reply_budget_exhausted"

// errTakeoverReplyBudgetExhausted is the controlled failure for a takeover
// reply that asks for yet another query continuation. It never fabricates an
// answer and never spends a third main generation.
var errTakeoverReplyBudgetExhausted = errors.New(takeoverReplyBudgetExhaustedCode)

func (a *App) normalizeTurnDecision(ctx context.Context, input turnDecisionNormalizationInput) (turnDecisionNormalization, error) {
	decision := input.Decision
	if decision == nil {
		decision = map[string]any{}
	}
	invocations := append([]CapabilityInvocation(nil), input.Invocations...)
	for index := range invocations {
		invocations[index] = normalizeCapabilityInvocationMetadata(invocations[index], input.FluctlightID, input.ConversationID, input.InboxID, input.InboxID, index)
	}
	result := turnDecisionNormalization{}
	// Appraisal is optional in the closed conversation schema. Absence means
	// "not proposed"; a present but malformed appraisal still fails closed.
	result.ToolOnlyNoReply = input.StructuredFallback && len(invocations) > 0 && !hasConversationReplyCapability(invocations, a.capabilityRegistry()) && !hasDeferredOutputCapabilities(invocations, a.capabilityRegistry())
	skipCognitiveStateTransition := len(mapValue(decision["appraisal"])) == 0
	if result.ToolOnlyNoReply {
		decision["action_type"] = "no_op"
		decision["response_intent"] = ""
	}
	// Influences are validated against exactly the Core-owned projection the
	// Provider saw, before any state transition can refresh execution context.
	if _, err := freezeDecisionInfluences(decision, input.Projection, false); err != nil {
		return result, err
	}
	if skipCognitiveStateTransition {
		decision["cognitive_state_transition"] = "not_proposed"
	}
	// E1: an unauthorized generation loses every persistent-switch field before
	// the proposal is prepared, so a takeover reply can never reach the durable
	// decision. The drop is recorded on the decision itself.
	decision, personaSwitchDiagnostics := applyPersistentSwitchGrant(decision, input.Grant)
	if len(personaSwitchDiagnostics) > 0 {
		recorded := make([]any, 0, len(personaSwitchDiagnostics))
		for _, diagnostic := range personaSwitchDiagnostics {
			recorded = append(recorded, diagnostic.asMap())
		}
		decision["persona_switch_diagnostics"] = recorded
	}
	if personalityDecision := mapValue(decision["personality_decision"]); len(personalityDecision) > 0 {
		plan, err := a.preparePersonalityDecision(ctx, input.FluctlightID, personalityDecision)
		if err != nil {
			return result, err
		}
		if plan != nil {
			decision["personality_transition"] = plan
			result.PersonalityPlan = plan
		}
	}
	// Normalize the root sidecar once; the response plan never receives a
	// second nested tool_calls copy.
	if len(invocations) > 0 {
		decision["capability_invocations"] = invocations
	}
	responsePlan, err := normalizeResponsePlan(decision, input.InboxID, input.Projection)
	if err != nil {
		return result, err
	}
	// Root tool_calls is the sole provider codec sidecar. Keep it on the frozen
	// decision; response_plan is a visible-plan projection only.
	decision["response_plan"] = responsePlan
	decision["context_projection"] = input.Projection
	// R05/F02: the visible text is resolved exactly once here and then frozen.
	canonical, visibleTextDiagnostics := resolveCanonicalVisibleReply(responsePlan, decision, invocations, a.capabilityRegistry())
	if len(visibleTextDiagnostics) > 0 {
		recorded := make([]any, 0, len(visibleTextDiagnostics))
		for _, diagnostic := range visibleTextDiagnostics {
			recorded = append(recorded, diagnostic.asMap())
		}
		decision["visible_text_diagnostics"] = recorded
		a.recordDiagnosticEvent(ctx, "cognition.visible_text.source_conflict", "warning", input.FluctlightID, input.InboxID, "turn:"+input.TurnID, recorded)
	}
	// F-01 path b: a conflicting reply argument means the model proposed two
	// texts. conversation.reply is a Core-derived execution record, not a model
	// proposal source, so the disagreement must fail closed instead of silently
	// choosing the root precedence.
	if canonical.Conflict {
		return result, errors.New("visible_text_source_conflict")
	}
	decision["visible_text_source"] = canonical.Source
	decision["visible_text_source_conflict"] = canonical.Conflict
	decision["visible_text_digest"] = canonical.Digest
	visibleCandidate := canonical.Text
	responseMode := normalizeConversationResponseMode(stringValue(decision["response_mode"]), input.StructuredFallback, visibleCandidate, invocations, a.capabilityRegistry())
	responsePlan["response_mode"] = responseMode
	var action string
	if len(invocations) > 0 {
		definitionMap := make(map[string]CapabilityDefinition, len(input.Definitions))
		for _, definition := range input.Definitions {
			definitionMap[definition.Name] = definition
		}
		action, err = resolveCapabilityAction(invocations, definitionMap)
		if err != nil {
			return result, err
		}
		if result.ToolOnlyNoReply {
			action = "no_op"
		}
	} else {
		action = normalizeConversationActionType(stringValue(decision["action_type"]))
	}
	// A direct user turn has one terminal product contract: visible text from the
	// same Main cognition, or an explicit tool-only completion when at least one
	// Capability invocation is present. A completely empty turn still fails
	// explicitly and remains retryable.
	if responseMode == "query_continuation" {
		if input.ForbidQueryContinuation {
			return result, errTakeoverReplyBudgetExhausted
		}
		if visibleCandidate != "" || validatePureQueryContinuation(invocations, a.capabilityRegistry()) != nil {
			return result, errors.New("query_continuation_contract_invalid")
		}
		decision["continuation_base_messages"] = input.ContinuationBaseMessages
		delete(responsePlan, "visible_text")
		delete(decision, "visible_text")
	} else {
		if responseMode != "final" {
			return result, errors.New("response_mode_invalid")
		}
		if visibleCandidate == "" {
			if len(invocations) == 0 {
				return result, errors.New("cognition_visible_text_missing")
			}
			// Tool calls are independent event-driven effects. A direct turn may
			// complete without assistant prose when it contains valid capability
			// invocations; each invocation is settled on its own target below.
			// `tool_only` is frozen so the execution path does not mistake this
			// valid silent turn for a failed no-op.
			result.ToolOnlyNoReply = true
			decision["tool_only"] = true
			decision["action_type"] = "no_op"
			responsePlan["action_type"] = "no_op"
			delete(responsePlan, "visible_text")
			delete(decision, "visible_text")
		} else {
			// Freeze the single authority into both carriers so every consumer
			// reads the same text.
			responsePlan["visible_text"] = visibleCandidate
			decision["visible_text"] = visibleCandidate
		}
	}
	if result.ToolOnlyNoReply {
		action = "no_op"
		decision["action_type"] = "no_op"
	} else {
		action = "reply"
		decision["action_type"] = "reply"
	}
	if preferenceDecision := mapValue(responsePlan["output_preference_decision"]); len(preferenceDecision) > 0 {
		responsePlan["output_preference_decision"] = evaluateOutputPreferenceAction(preferenceDecision, action, invocations, a.capabilityRegistry())
	}
	composite, err := normalizeCompositeAction(decision, invocations, input.InboxID, action)
	if err != nil {
		return result, err
	}
	decision["composite_action"] = composite
	if action != "reply" && action != "no_op" {
		return result, errors.New("decision_effect_invalid")
	}
	result.Decision = decision
	result.Invocations = invocations
	result.ResponsePlan = responsePlan
	result.ResponseMode = responseMode
	result.Action = action
	result.Composite = composite
	result.Canonical = canonical
	result.ContinuationBaseMessages = input.ContinuationBaseMessages
	return result, nil
}

// hydratedFrozenTurn is a frozen turn decoded back into the local variables the
// turn chain works with. Crash recovery and the post-takeover reload share it,
// so a replay and a takeover cannot drift apart (F03).
type hydratedFrozenTurn struct {
	Decision                 map[string]any
	Action                   string
	Invocations              []CapabilityInvocation
	Results                  []CapabilityResult
	ResponsePlan             map[string]any
	Composite                CompositeActionV1
	PersonalityPlan          *personalityDecisionPlan
	ResponseMode             string
	ContinuationBaseMessages []map[string]any
	Projection               ContextProjection
	Stage                    string
}

func (a *App) hydrateFrozenTurn(frozen frozenTurn, fluctlightID string) (hydratedFrozenTurn, error) {
	result := hydratedFrozenTurn{Stage: turnStageOf(frozen.Payload), Action: frozen.ActionType}
	decision := mapValue(frozen.Payload["decision"])
	if err := validateFrozenDecisionInfluences(decision); err != nil {
		return result, err
	}
	if raw, exists := decision["personality_transition"]; exists {
		plan, err := personalityDecisionPlanFromValue(raw)
		if err != nil || plan == nil || plan.FluctlightID != fluctlightID {
			return result, errors.New("personality_decision_plan_invalid")
		}
		result.PersonalityPlan = plan
	}
	if savedProjection, ok := contextProjectionFromValue(decision["context_projection"]); ok {
		result.Projection = savedProjection
	}
	invocations, err := capabilityInvocationsFromValue(frozen.Payload["capability_invocations"])
	if err != nil {
		return result, err
	}
	loaded, ok := compositeActionFromValue(decision["composite_action"])
	if !ok {
		return result, errors.New("frozen_composite_action_missing")
	}
	composite := loaded
	composite.ToolCalls = invocations
	if len(composite.CapabilityCallIDs) == 0 {
		composite.CapabilityCallIDs = capabilityCallIDs(invocations)
	}
	results, err := capabilityResultsFromValue(frozen.Payload["capability_results"])
	if err != nil {
		return result, err
	}
	responsePlan := mapValue(decision["response_plan"])
	if len(responsePlan) == 0 {
		return result, errors.New("frozen_response_plan_missing")
	}
	result.Decision = decision
	result.Invocations = invocations
	result.Results = results
	result.ResponsePlan = responsePlan
	result.Composite = composite
	result.ResponseMode = firstString(responsePlan["response_mode"], firstString(decision["response_mode"], "final"))
	base, _ := decision["continuation_base_messages"].([]map[string]any)
	if base == nil {
		base = cloneMapSliceFromAny(decision["continuation_base_messages"])
	}
	result.ContinuationBaseMessages = base
	return result, nil
}

// frozenCanonicalVisibleReply reconstructs the frozen visible-text authority
// from a persisted decision. It never re-derives the precedence from the reply
// capability arguments (R05/F02).
func frozenCanonicalVisibleReply(decision map[string]any) canonicalVisibleReply {
	return canonicalVisibleReply{
		Text:     stringValue(decision["visible_text"]),
		Source:   stringValue(decision["visible_text_source"]),
		Conflict: decision["visible_text_source_conflict"] == true,
		Digest:   stringValue(decision["visible_text_digest"]),
	}
}

// projectionCooldownUntil reads the persistent switch cooldown the projection
// already carries. A missing or malformed value means "no cooldown", never a
// silent block.
func projectionCooldownUntil(projection ContextProjection) *time.Time {
	raw := strings.TrimSpace(stringValue(mapValue(projection.PersonalityRuntime)["cooldown_until"]))
	if raw == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return nil
	}
	return &parsed
}
