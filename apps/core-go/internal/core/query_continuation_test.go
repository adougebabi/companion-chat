package core

import (
	"strings"
	"testing"
)

func TestQueryContinuationAcceptsOnlyOneOrTwoGenericPureQueries(t *testing.T) {
	registry := mustCapabilityRegistry(memoryRecallCapability{}, relationshipLookupCapability{}, activeMemoryEventCapability{})
	pure := []CapabilityInvocation{{CallID: "q1", CapabilityName: "memory.recall", Arguments: jsonBytes(map[string]any{"intent": "fact"})}, {CallID: "q2", CapabilityName: "relationship.lookup", Arguments: jsonBytes(map[string]any{"target_actor_id": "actor"})}}
	if err := validatePureQueryContinuation(pure, registry); err != nil {
		t.Fatal(err)
	}
	for name, calls := range map[string][]CapabilityInvocation{
		"none":   []CapabilityInvocation{},
		"three":  append(append([]CapabilityInvocation{}, pure...), pure[0]),
		"action": []CapabilityInvocation{{CallID: "a1", CapabilityName: "active_memory_event"}},
		"mixed":  []CapabilityInvocation{pure[0], CapabilityInvocation{CallID: "a1", CapabilityName: "active_memory_event"}},
	} {
		if err := validatePureQueryContinuation(calls, registry); err == nil {
			t.Fatalf("%s continuation accepted", name)
		}
	}
}

func TestNativeToolOnlyFallbackNormalizesOnlyPureQueriesToContinuation(t *testing.T) {
	registry := mustCapabilityRegistry(memoryRecallCapability{}, relationshipLookupCapability{}, activeMemoryEventCapability{})
	pure := []CapabilityInvocation{{CallID: "q1", CapabilityName: "memory.recall", Arguments: jsonBytes(map[string]any{"intent": "fact"})}}
	action := []CapabilityInvocation{{CallID: "a1", CapabilityName: "active_memory_event", Arguments: jsonBytes(map[string]any{"operation": "create"})}}
	mixed := append(append([]CapabilityInvocation{}, pure...), action...)
	for name, testCase := range map[string]struct {
		explicit string
		fallback bool
		visible  string
		calls    []CapabilityInvocation
		want     string
	}{
		"native pure query": {fallback: true, calls: pure, want: "query_continuation"},
		"explicit final":    {explicit: "final", fallback: true, calls: pure, want: "final"},
		"explicit query":    {explicit: "query_continuation", calls: pure, want: "query_continuation"},
		"schema omission":   {calls: pure, want: "final"},
		"visible fallback":  {fallback: true, visible: "answer", calls: pure, want: "final"},
		"action fallback":   {fallback: true, calls: action, want: "final"},
		"mixed fallback":    {fallback: true, calls: mixed, want: "final"},
		"empty fallback":    {fallback: true, want: "final"},
	} {
		t.Run(name, func(t *testing.T) {
			if got := normalizeConversationResponseMode(testCase.explicit, testCase.fallback, testCase.visible, testCase.calls, registry); got != testCase.want {
				t.Fatalf("response mode=%q want=%q", got, testCase.want)
			}
		})
	}
}

func TestQueryContinuationStateAndToolMessagesAreReplayStable(t *testing.T) {
	base := []map[string]any{{"role": "system", "content": "stable"}, {"role": "user", "content": "current"}}
	invocations := []CapabilityInvocation{{CallID: "q1", CapabilityName: "memory.recall", Arguments: jsonBytes(map[string]any{"intent": "fact"})}}
	state := newQueryContinuationState(base, invocations)
	if state.Phase != "requested" || state.RequestDigest == "" {
		t.Fatalf("state = %#v", state)
	}
	results := []CapabilityResult{{
		CallID: "q1", CapabilityName: "memory.recall", Status: "completed",
		Output: map[string]any{"items": []any{map[string]any{"ref": "memory:ctx_0123456789abcdef0123456789abcdef", "content": "fact"}}},
	}}
	messages, err := queryContinuationMessages(state, invocations, results)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 4 || stringValue(messages[2]["role"]) != "assistant" || stringValue(messages[3]["role"]) != "tool" || !validQueryContinuationMessages(messages) {
		t.Fatalf("continuation messages = %#v", messages)
	}
	if strings.Contains(jsonString(messages), "visibility") || strings.Contains(jsonString(messages), "evidence_refs") {
		t.Fatalf("internal fields leaked: %#v", messages)
	}
	state.Phase, state.Results = "queries_completed", results
	if _, err := queryContinuationStateFromValue(state); err != nil {
		t.Fatal(err)
	}
	state.Phase, state.VisibleText = "provider_completed", "最终答案"
	decoded, err := queryContinuationStateFromValue(state)
	if err != nil || decoded.VisibleText != "最终答案" {
		t.Fatalf("provider-completed state=%#v err=%v", decoded, err)
	}
}

func TestQueryContinuationMessagesAllowBLayoutAssistantHistory(t *testing.T) {
	base := []map[string]any{
		{"role": "system", "content": "stable"},
		{"role": "user", "content": "[RUNTIME CONTEXT]\n{}\n[/RUNTIME CONTEXT]"},
		{"role": "user", "content": "earlier question"},
		{"role": "assistant", "content": "earlier answer"},
		{"role": "user", "content": "current question"},
	}
	invocations := []CapabilityInvocation{{CallID: "q1", CapabilityName: "memory.recall", Arguments: jsonBytes(map[string]any{"intent": "fact"})}}
	results := []CapabilityResult{{
		CallID: "q1", CapabilityName: "memory.recall", Status: "completed",
		Output: map[string]any{"items": []any{map[string]any{"ref": "memory:ctx_0123456789abcdef0123456789abcdef", "content": "fact"}}},
	}}
	messages, err := queryContinuationMessages(newQueryContinuationState(base, invocations), invocations, results)
	if err != nil {
		t.Fatal(err)
	}
	if !validQueryContinuationMessages(messages) {
		t.Fatalf("B-layout continuation messages rejected: %#v", messages)
	}
	messages[3]["content"] = ""
	if validQueryContinuationMessages(messages) {
		t.Fatal("empty ordinary assistant history accepted")
	}
}

func TestQueryContinuationResponseSchemaIsVisibleTextOnlyAndResponseModeRequired(t *testing.T) {
	schema := queryContinuationResponseSchema()
	properties := mapValue(schema["properties"])
	if schema["additionalProperties"] != false || len(properties) != 1 || properties["visible_text"] == nil || !containsSchemaRequired(schema, "visible_text") {
		t.Fatalf("continuation schema = %#v", schema)
	}
	main := cognitiveTurnResponseSchema()
	if !containsSchemaRequired(main, "response_mode") {
		t.Fatalf("Main schema does not require response_mode: %#v", main)
	}
	mode := mapValue(mapValue(main["properties"])["response_mode"])
	if values := arrayValue(mode["enum"]); len(values) != 2 || !containsStringValue(values, "final") || !containsStringValue(values, "query_continuation") {
		t.Fatalf("response_mode schema = %#v", mode)
	}
}

func TestContinuationVisibleTextRejectsMutationFields(t *testing.T) {
	if text, err := continuationVisibleText(map[string]any{"visible_text": "答案"}); err != nil || text != "答案" {
		t.Fatalf("visible text=%q err=%v", text, err)
	}
	if _, err := continuationVisibleText(map[string]any{"visible_text": "答案", "claims": []any{}}); err == nil {
		t.Fatal("continuation mutation fields accepted")
	}
}
