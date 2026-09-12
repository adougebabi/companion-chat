package core

import (
	"context"
	"errors"
	"strings"
)

const queryContinuationVersion = "query-continuation.v1"

type QueryContinuationState struct {
	SchemaVersion string             `json:"schema_version"`
	Phase         string             `json:"phase"`
	RequestDigest string             `json:"request_digest"`
	BaseMessages  []map[string]any   `json:"base_messages"`
	Results       []CapabilityResult `json:"results,omitempty"`
	VisibleText   string             `json:"visible_text,omitempty"`
}

func validatePureQueryContinuation(invocations []CapabilityInvocation, registry *CapabilityRegistry) error {
	if registry == nil || len(invocations) < 1 || len(invocations) > 2 {
		return errors.New("query_continuation_invocations_invalid")
	}
	for _, invocation := range invocations {
		capability, ok := registry.Lookup(invocation.CapabilityName)
		if !ok {
			return errors.New("query_continuation_capability_unknown")
		}
		definition, _ := registry.Definition(invocation.CapabilityName)
		class, err := classifyCapabilityExecution(capability, definition)
		if err != nil || class != CapabilityExecutionPureQuery {
			return errors.New("query_continuation_requires_pure_query")
		}
	}
	return nil
}

func normalizeConversationResponseMode(explicit string, structuredFallback bool, visibleText string, invocations []CapabilityInvocation, registry *CapabilityRegistry) string {
	explicit = strings.TrimSpace(explicit)
	if explicit != "" {
		return explicit
	}
	if structuredFallback && strings.TrimSpace(visibleText) == "" && validatePureQueryContinuation(invocations, registry) == nil {
		return "query_continuation"
	}
	return "final"
}

func newQueryContinuationState(baseMessages []map[string]any, invocations []CapabilityInvocation) QueryContinuationState {
	digest := stableDigest(jsonString(map[string]any{"messages": baseMessages, "invocations": invocations}))
	return QueryContinuationState{SchemaVersion: queryContinuationVersion, Phase: "requested", RequestDigest: digest, BaseMessages: cloneMapSlice(baseMessages)}
}

func queryContinuationStateFromValue(value any) (QueryContinuationState, error) {
	var state QueryContinuationState
	if jsonUnmarshal(jsonBytes(value), &state) != nil || state.SchemaVersion != queryContinuationVersion || state.RequestDigest == "" || len(state.BaseMessages) < 2 {
		return QueryContinuationState{}, errors.New("query_continuation_state_invalid")
	}
	if !validAssembledProviderMessages(state.BaseMessages) {
		return QueryContinuationState{}, errors.New("query_continuation_base_messages_invalid")
	}
	switch state.Phase {
	case "requested":
	case "queries_completed":
		if len(state.Results) == 0 {
			return QueryContinuationState{}, errors.New("query_continuation_results_missing")
		}
	case "provider_completed":
		if len(state.Results) == 0 || strings.TrimSpace(state.VisibleText) == "" {
			return QueryContinuationState{}, errors.New("query_continuation_output_missing")
		}
	default:
		return QueryContinuationState{}, errors.New("query_continuation_phase_invalid")
	}
	return state, nil
}

func (a *App) persistQueryContinuationState(ctx context.Context, frozenID string, state QueryContinuationState) error {
	if _, err := queryContinuationStateFromValue(state); err != nil {
		return err
	}
	tag, err := a.DB.Pool().Exec(ctx, `UPDATE public.cognition_frozen_actions SET payload=jsonb_set(payload,'{query_continuation}',$2::jsonb,true) WHERE id=$1 AND status='frozen'`, frozenID, jsonBytes(state))
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrConflict
	}
	return nil
}

func queryContinuationMessages(state QueryContinuationState, invocations []CapabilityInvocation, results []CapabilityResult) ([]map[string]any, error) {
	if len(invocations) != len(results) || len(invocations) < 1 || len(invocations) > 2 {
		return nil, errors.New("query_continuation_result_identity_invalid")
	}
	messages := cloneMapSlice(state.BaseMessages)
	toolCalls := make([]any, 0, len(invocations))
	for index, invocation := range invocations {
		if results[index].CallID != invocation.CallID || results[index].CapabilityName != invocation.CapabilityName || results[index].Status != "completed" {
			return nil, errors.New("query_continuation_query_failed")
		}
		toolCalls = append(toolCalls, map[string]any{"id": invocation.CallID, "type": "function", "function": map[string]any{"name": invocation.CapabilityName, "arguments": string(invocation.Arguments)}})
	}
	messages = append(messages, map[string]any{"role": "assistant", "content": "", "tool_calls": toolCalls})
	for index, result := range results {
		messages = append(messages, map[string]any{"role": "tool", "tool_call_id": result.CallID, "name": invocations[index].CapabilityName, "content": jsonString(result.Output)})
	}
	return messages, nil
}

func continuationVisibleText(value map[string]any) (string, error) {
	if len(value) != 1 {
		return "", errors.New("query_continuation_response_invalid")
	}
	text := normalizeVisibleReply(stringValue(value["visible_text"]))
	if text == "" {
		return "", errors.New("query_continuation_visible_text_missing")
	}
	return text, nil
}

func cloneMapSliceFromAny(value any) []map[string]any {
	result := make([]map[string]any, 0)
	for _, raw := range arrayValue(value) {
		if item := mapValue(raw); len(item) > 0 {
			result = append(result, cloneMap(item))
		}
	}
	return result
}

func firstError(primary, fallback error) error {
	if primary != nil {
		return primary
	}
	return fallback
}
