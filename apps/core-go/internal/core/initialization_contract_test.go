package core

import "testing"

func TestValidInitializationAcceptsOpenRelationshipLabelAndActorUser(t *testing.T) {
	value := map[string]any{
		"core_persona": map[string]any{
			"identity":          map[string]any{"name": "影者"},
			"personality":       map[string]any{"curiosity": 0.8},
			"behavioral_policy": map[string]any{"response_style": "温和"},
			"life_profile":      map[string]any{"character_constraints": []any{}},
		},
		"developing_self":       map[string]any{"claims": []any{}},
		"initial_relationships": []any{map[string]any{"target_actor_id": "actor_user", "role": map[string]any{"label": "准恋人/暧昧对象", "addressing": map[string]any{"preferred": "你", "self_reference": "夏希/希希"}}}},
		"initial_goals":         []any{},
		"initial_intentions":    []any{},
		"extensions":            map[string]any{},
	}
	if !validInitialization(value) {
		t.Fatal("expected canonical actor_user initialization to be valid")
	}
}

func TestNormalizeInitializationResponseMovesUnknownFieldsToExtensions(t *testing.T) {
	value := normalizeInitializationResponse(map[string]any{
		"core_persona": map[string]any{
			"identity":          map[string]any{"name": "影者", "unclassified": "keep"},
			"personality":       map[string]any{},
			"behavioral_policy": map[string]any{},
			"life_profile":      map[string]any{"relationship_seeds": []any{}},
		},
		"developing_self": map[string]any{"claims": []any{}},
		"goals":           []any{},
	})
	if _, ok := value["initial_goals"]; !ok {
		t.Fatal("expected missing goals to normalize to an empty array")
	}
	extensions := mapValue(value["extensions"])
	if extensions["top_level.goals"] == nil || extensions["core_persona.identity.unclassified"] == nil {
		t.Fatalf("unknown fields were not preserved in extensions: %#v", extensions)
	}
	if len(arrayValue(value["initial_relationships"])) != 0 {
		t.Fatalf("expected empty initial relationships: %#v", value["initial_relationships"])
	}
}
