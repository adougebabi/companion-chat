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
				"inner_state":  map[string]any{"mood": map[string]any{"intensity": 0.4}},
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
	for _, key := range []string{"schema_version", "fluctlight_id", "conversation_id", "source_fact_id", "current_user_text", "context_revision", "core_persona_revision", "developing_self_revision", "current_state_revision", "core_persona", "developing_self", "current_state"} {
		if _, ok := compact[key]; !ok {
			t.Fatalf("compact context is missing canonical field %q: %#v", key, compact)
		}
	}
	state := mapValue(compact["current_state"])
	if _, ok := mapValue(state["data"])["inner_state"]; !ok {
		t.Fatalf("current state lost inner_state: %#v", state)
	}
	if _, ok := mapValue(state["data"])["life_context"]; !ok {
		t.Fatalf("current state lost life_context: %#v", state)
	}
	encoded, err := json.Marshal(compact)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "memory_event") {
		t.Fatal("native capability manifest leaked into compact user context")
	}
}

func TestCompactCognitionContextRetainsNonEmptyEvidenceCollections(t *testing.T) {
	projection := ContextProjection{
		SchemaVersion:      "fluctlight.context.v2",
		CorePersona:        map[string]any{"authority": "hard_constraint"},
		CurrentState:       map[string]any{"authority": "transient_state", "data": map[string]any{}},
		RecentMessages:     []map[string]any{{"id": "message-1", "text": "hello"}},
		Memories:           []map[string]any{{"id": "memory-1", "content": "fact"}},
		Relationships:      []map[string]any{{"actor_id": "actor-1", "status": "active"}},
		Hypotheses:         []map[string]any{{"content": "hypothesis"}},
		DriveSlots:         []map[string]any{{"key": "focus"}},
		PreferenceSlots:    []map[string]any{{"key": "quiet"}},
		TriggerPreferences: []map[string]any{{"key": "morning"}},
	}
	compact := compactCognitionContext(projection)
	for _, key := range []string{"recent_messages", "memories", "relationships", "hypotheses", "drive_slots", "preference_slots", "trigger_preferences"} {
		if len(arrayValue(compact[key])) != 1 {
			t.Fatalf("non-empty collection %q was not retained: %#v", key, compact)
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
		t.Fatalf("core persona authority = %#v", core["authority"])
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
