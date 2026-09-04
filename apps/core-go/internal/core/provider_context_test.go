package core

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCompactCognitionContextKeepsOnlyCanonicalLayersAndNonEmptyEvidence(t *testing.T) {
	projection := ContextProjection{
		SchemaVersion:          "fluctlight.context.v2",
		FluctlightID:           "fl-1",
		ConversationID:         "conversation-1",
		SourceFactID:           "fact-1",
		CurrentUserText:        "hello",
		ContextRevision:        4,
		CorePersonaRevision:    2,
		DevelopingSelfRevision: 3,
		CurrentStateRevision:   5,
		CorePersona: map[string]any{
			"authority": "hard_constraint",
			"data": map[string]any{
				"identity":          map[string]any{"name": "影者"},
				"personality":       map[string]any{"temperament": "冷静"},
				"behavioral_policy": map[string]any{"directness": 0.8},
			},
		},
		DevelopingSelf: []map[string]any{{"claim": "喜欢安静", "confidence": 0.6}},
		CurrentState: map[string]any{
			"authority": "transient_state",
			"data": map[string]any{
				"inner_state": map[string]any{
					"mood":            map[string]any{"label": "平静", "intensity": 0.4, "started_at": "db-only", "expected_decay_at": "db-only"},
					"pad":             map[string]any{"arousal": 0.1, "pleasure": 0.2, "dominance": 0.3},
					"regulation":      map[string]any{"stress": 0.1, "stability": 0.9, "natural_decay_rate": 0.25},
					"revision":        9,
					"last_updated_at": "db-only",
				},
				"life_context": map[string]any{"scene": "图书馆"},
			},
		},
		// These fields intentionally duplicate the canonical layer data. They must
		// not appear in the Provider-facing DTO.
		Identity:           map[string]any{"name": "影者"},
		Personality:        map[string]any{"temperament": "冷静"},
		BehavioralPolicy:   map[string]any{"directness": 0.8},
		InnerState:         map[string]any{"mood": map[string]any{"intensity": 0.4}},
		LifeContext:        map[string]any{"scene": "图书馆"},
		Capabilities:       []map[string]any{{"name": "memory_event"}},
		RecentMessages:     nil,
		Memories:           nil,
		Relationships:      nil,
		Hypotheses:         nil,
		DriveSlots:         nil,
		PreferenceSlots:    nil,
		TriggerPreferences: nil,
	}

	compact := compactCognitionContext(projection)
	for _, key := range []string{"identity", "personality", "behavioral_policy", "inner_state", "life_context", "capabilities", "recent_messages", "memories", "relationships", "hypotheses", "drive_slots", "preference_slots", "trigger_preferences"} {
		if _, ok := compact[key]; ok {
			t.Fatalf("compact context contains duplicate/empty field %q: %#v", key, compact)
		}
	}
	for _, key := range []string{"current_user_text", "core_persona", "developing_self", "current_state"} {
		if _, ok := compact[key]; !ok {
			t.Fatalf("compact context is missing canonical field %q: %#v", key, compact)
		}
	}
	for _, key := range []string{"fluctlight_id", "conversation_id", "source_fact_id", "context_revision", "core_persona_revision", "developing_self_revision", "current_state_revision"} {
		if _, ok := compact[key]; ok {
			t.Fatalf("database/coordination field leaked into compact context: %q", key)
		}
	}
	state := mapValue(compact["current_state"])
	if mapValue(compact["core_persona"])["authority"] != "hard_constraint" || state["authority"] != "transient_state" {
		t.Fatalf("semantic authority labels were removed: %#v", compact)
	}
	if _, ok := mapValue(state["data"])["inner_state"]; !ok {
		t.Fatalf("current state lost inner_state: %#v", state)
	}
	if _, ok := mapValue(state["data"])["life_context"]; !ok {
		t.Fatalf("current state lost life_context: %#v", state)
	}
	inner := mapValue(mapValue(state["data"])["inner_state"])
	for _, key := range []string{"revision", "last_updated_at"} {
		if _, ok := inner[key]; ok {
			t.Fatalf("database inner-state field leaked: %q: %#v", key, inner)
		}
	}
	if _, ok := mapValue(inner["mood"])["started_at"]; ok {
		t.Fatal("mood persistence timestamp leaked into compact state")
	}
	if _, ok := mapValue(inner["regulation"])["natural_decay_rate"]; ok {
		t.Fatal("regulation control parameter leaked into compact state")
	}
	if _, ok := mapValue(mapValue(compact["core_persona"])["data"])["identity"].(map[string]any)["id"]; ok {
		t.Fatal("database identity id leaked into compact core persona")
	}
	encoded, err := json.Marshal(compact)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "schema_version") {
		t.Fatal("schema_version leaked into provider context")
	}
	if !strings.Contains(string(encoded), "hard_constraint") {
		t.Fatal("semantic authority label was lost")
	}
	if strings.Contains(string(encoded), "memory_event") {
		t.Fatal("native capability manifest leaked into compact user context")
	}
}

func TestCompactCognitionContextRetainsNonEmptySemanticCollections(t *testing.T) {
	projection := ContextProjection{
		SchemaVersion:      "fluctlight.context.v2",
		CorePersona:        map[string]any{"authority": "hard_constraint"},
		CurrentState:       map[string]any{"authority": "transient_state", "data": map[string]any{}},
		RecentMessages:     []map[string]any{{"id": "message-1", "kind": "user", "text": "hello", "created_at": "2026-09-03T00:00:00Z"}},
		Memories:           []map[string]any{{"id": "memory-1", "content": "fact"}},
		Relationships:      []map[string]any{{"actor_id": "actor-1", "status": "active", "summary": "协作"}},
		Hypotheses:         []map[string]any{{"content": "hypothesis"}},
		DriveSlots:         []map[string]any{{"key": "focus"}},
		PreferenceSlots:    []map[string]any{{"key": "quiet"}},
		TriggerPreferences: []map[string]any{{"key": "morning"}},
	}
	compact := compactCognitionContext(projection)
	for _, key := range []string{"memories", "relationships", "hypotheses", "drive_slots", "preference_slots", "trigger_preferences"} {
		if len(arrayValue(compact[key])) != 1 {
			t.Fatalf("non-empty collection %q was not retained: %#v", key, compact)
		}
	}
	if !strings.Contains(stringValue(compact["recent_messages"]), "[09-03 00:00:00] user: hello") {
		t.Fatalf("compact recent messages = %#v", compact["recent_messages"])
	}
}

func TestCompactCognitionContextRemovesDatabaseMetadataFromEvidence(t *testing.T) {
	compact := compactCognitionContext(ContextProjection{
		SchemaVersion: "fluctlight.context.v2",
		CorePersona:   map[string]any{"authority": "hard_constraint", "data": map[string]any{}},
		CurrentState:  map[string]any{"authority": "transient_state", "data": map[string]any{}},
		RecentMessages: []map[string]any{{
			"id": "message-db-id", "sequence": 3, "author_actor_id": "actor-db-id", "kind": "user",
			"text": "hello", "attachment_refs": []any{}, "created_at": "2026-09-03T00:00:00Z", "source": "message:message-db-id",
		}},
		Memories: []map[string]any{{
			"id": "memory-db-id", "type": "episodic", "content": "fact", "confidence": 0.9,
			"importance": 0.4, "emotional_significance": 0.2, "created_at": "2026-09-03T00:00:00Z",
			"evidence_refs": []any{"fact-1"}, "source": "memory:memory-db-id", "status": "active", "revision": 2,
			"visibility": "private", "conversation_id": "conversation-db-id", "event_refs": []any{"event-1"},
		}},
		DevelopingSelf: []map[string]any{{
			"id": "claim-db-id", "category": "preference", "claim": "喜欢安静", "value": "quiet",
			"confidence": 0.6, "evidence_refs": []any{"fact-1"}, "provenance": map[string]any{"source": "owner_defined"},
			"status": "uncertain", "revision": 4, "updated_at": "2026-09-03T00:00:00Z", "fluctlight_id": "fl-db-id",
		}},
	})
	message := stringValue(compact["recent_messages"])
	if !strings.Contains(message, "[09-03 00:00:00] user: hello") {
		t.Fatalf("message semantics changed: %#v", message)
	}
	memory := mapValue(arrayValue(compact["memories"])[0])
	for _, key := range []string{"id", "source", "status", "revision", "visibility", "conversation_id", "event_refs"} {
		if _, ok := memory[key]; ok {
			t.Fatalf("memory database field leaked: %q: %#v", key, memory)
		}
	}
	for _, key := range []string{"type", "content", "confidence", "importance", "emotional_significance"} {
		if _, ok := memory[key]; !ok {
			t.Fatalf("memory semantic field missing: %q: %#v", key, memory)
		}
	}
	claim := mapValue(arrayValue(compact["developing_self"])[0])
	for _, key := range []string{"id", "revision", "updated_at", "fluctlight_id"} {
		if _, ok := claim[key]; ok {
			t.Fatalf("Developing Self database field leaked: %q: %#v", key, claim)
		}
	}
}

func TestCompactCognitionContextRebuildsMissingCorePersonaEnvelope(t *testing.T) {
	compact := compactCognitionContext(ContextProjection{
		SchemaVersion:    "fluctlight.context.v2",
		Identity:         map[string]any{"name": "影者"},
		Personality:      map[string]any{"temperament": "冷静"},
		BehavioralPolicy: map[string]any{"directness": 0.8},
		CurrentState:     map[string]any{},
	})
	core := mapValue(compact["core_persona"])
	if core["authority"] != "hard_constraint" {
		t.Fatalf("core persona authority was removed: %#v", core["authority"])
	}
	data := mapValue(core["data"])
	for _, key := range []string{"identity", "personality", "behavioral_policy"} {
		if _, ok := data[key]; !ok {
			t.Fatalf("legacy core persona reconstruction missing %q: %#v", key, data)
		}
	}
	for _, key := range []string{"identity", "personality", "behavioral_policy"} {
		if _, ok := compact[key]; ok {
			t.Fatalf("reconstructed field was duplicated at top level: %q", key)
		}
	}
}

func TestCompactMessageTimeKeepsDateAndSecondsWithoutSequence(t *testing.T) {
	if got := compactMessageTime("2026-09-03T05:27:14.105684Z"); got != "09-03 05:27:14" {
		t.Fatalf("compact message time = %q", got)
	}
}
