package core

import (
	"slices"
	"testing"
)

func sanitizeTestProjection(t *testing.T) (ContextProjection, string) {
	t.Helper()
	projection := ContextProjection{
		SchemaVersion: "fluctlight.context.v3", FluctlightID: "sanitize-fluctlight", OwnerActorID: "sanitize-owner",
		ConversationID: "sanitize-conversation", SourceFactID: "sanitize-fact", CurrentSpeaker: map[string]any{"actor_id": "sanitize-owner"},
		Relationships: []map[string]any{{"id": "sanitize-relationship", "target_actor_id": "sanitize-owner", "revision": 1, "role": map[string]any{"label": "朋友"}}},
	}
	if err := buildContextReferenceIndex(&projection); err != nil {
		t.Fatal(err)
	}
	ref := stringValue(projection.Relationships[0]["ref"])
	if ref == "" {
		t.Fatal("fixture relationship has no reference")
	}
	return projection, ref
}

func TestSanitizeDirectReplyDecisionKeepsValidDataAndDropsTheRest(t *testing.T) {
	projection, ref := sanitizeTestProjection(t)
	raw := map[string]any{
		"action_type": "reply", "response_intent": "回应",
		"influences": []any{
			map[string]any{"ref": ref, "role": "grounds", "confidence": 0.9, "note": "关系"},
			map[string]any{"ref": "relationship:ctx_00000000000000000000000000000000", "role": "grounds", "confidence": 0.9, "note": "伪造"},
		},
		"goal_event_candidates":   []any{map[string]any{"goal_ref": "goal:missing", "reason": "不存在"}},
		"claims":                  "不是数组",
		"thought":                 "想认真回应",
		"invented_field":          true,
		"context_reference_index": map[string]any{"forged": true},
	}
	decision, dropped := sanitizeDirectReplyDecision(raw, projection)
	for _, key := range []string{"goal_event_candidates", "claims", "invented_field", "context_reference_index", "influences"} {
		if !slices.Contains(dropped, key) {
			t.Errorf("%s was not reported as dropped: %v", key, dropped)
		}
	}
	if decision["thought"] != "想认真回应" || decision["claims"] != nil || decision["goal_event_candidates"] != nil {
		t.Fatalf("valid data lost or invalid data kept: %#v", decision)
	}
	influences := arrayValue(decision["influences"])
	if len(influences) != 1 || stringValue(mapValue(influences[0])["ref"]) != ref {
		t.Fatalf("influences were not filtered to the valid reference: %#v", influences)
	}
	if _, err := freezeDecisionInfluences(decision, projection, false); err != nil {
		t.Fatalf("sanitized decision does not freeze: %v", err)
	}
}

func TestSanitizeDirectReplyDecisionFillsAMissingDecision(t *testing.T) {
	projection, _ := sanitizeTestProjection(t)
	for _, raw := range []map[string]any{nil, {}, {"action_type": "wave", "influences": "none"}} {
		decision, _ := sanitizeDirectReplyDecision(raw, projection)
		if decision["action_type"] != "reply" || stringValue(decision["response_intent"]) == "" || len(arrayValue(decision["influences"])) != 0 {
			t.Fatalf("missing decision was not defaulted: %#v", decision)
		}
		if _, err := freezeDecisionInfluences(decision, projection, false); err != nil {
			t.Fatalf("default decision does not freeze: %v", err)
		}
	}
}
