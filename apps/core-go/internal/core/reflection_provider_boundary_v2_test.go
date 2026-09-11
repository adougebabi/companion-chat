package core

import (
	"strings"
	"testing"
)

func boolValueForTest(value any) bool {
	result, _ := value.(bool)
	return result
}

func TestReflectionV2ProviderSchemaOwnsOnlySemanticFields(t *testing.T) {
	schema := reflectionProposalV2ProviderSchema()
	if jsonString(providerSchemaForRole("reflection")) != jsonString(schema) {
		t.Fatal("generic reflection role did not resolve to the V2 schema")
	}
	forbidden := map[string]struct{}{}
	for _, key := range []string{
		"id", "memory_id", "profile_id", "goal_id", "intention_id", "target_actor_id", "owner_actor_id", "entity_id", "fluctlight_id",
		"revision", "expected_revision", "idempotency_key", "provider_request_id", "workflow_id", "provenance", "context_reference_index",
		"prepared_payload", "requested_delta", "applied_delta", "raw_delta", "metrics",
	} {
		forbidden[key] = struct{}{}
	}
	var walk func(map[string]any, string)
	walk = func(node map[string]any, path string) {
		if len(node) == 0 {
			return
		}
		for key, child := range mapValue(node["properties"]) {
			if _, found := forbidden[key]; found {
				t.Fatalf("Reflection V2 schema exposes runtime-owned field %s%s", path, key)
			}
			walk(mapValue(child), path+key+".")
		}
		walk(mapValue(node["items"]), path+"[].")
		for _, raw := range arrayValue(node["anyOf"]) {
			walk(mapValue(raw), path+"anyOf.")
		}
	}
	walk(schema, "")
	for _, required := range []string{"relationship_observations", "emotional_summary", "affect_recalibration_candidates", "personality_evolution_candidates", "behavior_policy_evolution_candidates"} {
		if !containsSchemaRequired(schema, required) {
			t.Fatalf("Reflection V2 schema does not require %q", required)
		}
	}
	for _, forbiddenText := range []string{"relationship_candidates", "profile_id", "target_actor_id", "expected_revision", "完整复制"} {
		if strings.Contains(reflectionV2Instruction, forbiddenText) {
			t.Fatalf("Reflection V2 instruction asks for runtime-owned field %q: %s", forbiddenText, reflectionV2Instruction)
		}
	}
}

func TestCompactReflectionEvidenceV2RejectsNestedAndOversizedValues(t *testing.T) {
	oversized := strings.Repeat("界", 2500)
	compact := compactReflectionEvidenceV2([]map[string]any{{
		"sequence": 3, "event_type": "life.observation",
		"payload": map[string]any{
			"summary": map[string]any{"internal_id": "must-not-egress", "text": oversized},
			"scene":   oversized, "status": []any{"processed", "internal"}, "current_task": "阅读",
		},
		"appraisal": map[string]any{
			"direction": oversized, "confidence": 1.7, "relevance": 0.8,
			"drive_signals": []any{map[string]any{"ref": "db-id", "direction": "increase", "strength": 0.7, "confidence": 0.9, "provider_request_id": "secret"}},
		},
	}})
	if len(compact) != 1 {
		t.Fatalf("compact evidence=%#v", compact)
	}
	encoded := jsonString(compact)
	for _, leaked := range []string{"must-not-egress", "internal", "provider_request_id", "secret", "db-id"} {
		if strings.Contains(encoded, leaked) {
			t.Fatalf("nested/runtime value %q leaked: %s", leaked, encoded)
		}
	}
	observation := mapValue(compact[0]["observation"])
	if len([]rune(stringValue(observation["scene"]))) != 2000 || stringValue(observation["current_task"]) != "阅读" {
		t.Fatalf("bounded semantic observation=%#v", observation)
	}
	appraisal := mapValue(compact[0]["appraisal"])
	if appraisal["confidence"] != nil || numberOrZero(appraisal["relevance"]) != 0.8 || len([]rune(stringValue(appraisal["direction"]))) != 128 {
		t.Fatalf("bounded appraisal=%#v", appraisal)
	}
}

func TestActorRelationshipContextCompactsTargetedIntention(t *testing.T) {
	projection := ContextProjection{
		SelfActor:          map[string]any{"ref": "actor_self", "actor_id": "fl-1", "type": "fluctlight"},
		CurrentSpeaker:     map[string]any{"ref": "actor_user", "actor_id": "owner-1", "type": "human"},
		PersonalityRuntime: map[string]any{"active_profile_id": "default"},
		Relationships: []map[string]any{{
			"ref": "relationship:ctx_0123456789abcdef0123456789abcdef", "target_actor_id": "owner-1", "profile_id": "default",
			"role": map[string]any{"label": "friend"}, "trend": "stable", "revision": 2, "provenance": map[string]any{"source": "db-secret"},
		}},
		Intentions: []map[string]any{{
			"ref": "intention:ctx_0123456789abcdef0123456789abcdef", "target_actor_id": "owner-1", "profile_id": "default",
			"goal_id": "goal-db-id", "current_attempt_id": "attempt-db-id", "evidence_refs": []any{"fact-db-id"},
			"action_intent": "问候用户", "expected_outcome": "形成联结", "status": "qualified", "revision": 3,
		}},
	}
	encoded := jsonString(compactActorRelationshipContext(projection))
	for _, leaked := range []string{"goal-db-id", "attempt-db-id", "fact-db-id", "db-secret", "current_attempt_id", "profile_id", "goal_id", "evidence_refs", "provenance"} {
		if strings.Contains(encoded, leaked) {
			t.Fatalf("actor relationship context leaked %q: %s", leaked, encoded)
		}
	}
	for _, expected := range []string{"intention:ctx_0123456789abcdef0123456789abcdef", "问候用户", "形成联结", "expected_revision"} {
		if !strings.Contains(encoded, expected) {
			t.Fatalf("actor relationship context lost %q: %s", expected, encoded)
		}
	}
}
