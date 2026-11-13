package core

import (
	"strings"
	"testing"
)

func TestExpressionContextKeepsCurrentProfileRelationshipAddressing(t *testing.T) {
	input := morningReplyFixture(t)
	input.Projection.PersonalityRuntime["active_profile_id"] = "guarded"
	input.Projection.Relationships = []map[string]any{
		{"target_actor_id": "test-user", "role": map[string]any{"label": "friend", "addressing": map[string]any{"preferred": "SHARED_ADDRESS"}}},
		{"target_actor_id": "test-user", "profile_id": "other", "role": map[string]any{"label": "partner", "addressing": map[string]any{"preferred": "OTHER_PROFILE_ADDRESS"}}},
		{"target_actor_id": "test-user", "profile_id": "guarded", "role": map[string]any{"label": "friend", "addressing": map[string]any{"preferred": "CURRENT_PROFILE_ADDRESS", "self_reference": "CURRENT_SELF_REFERENCE"}}},
		{"target_actor_id": "other-user", "profile_id": "guarded", "role": map[string]any{"addressing": map[string]any{"preferred": "OTHER_ACTOR_ADDRESS"}}},
	}
	before := jsonString(input.Projection.Relationships)
	for _, cognition := range []bool{true, false} {
		wire := jsonString(factOnlyContextFacts(input.Projection, cognition).Facts)
		for _, required := range []string{"CURRENT_PROFILE_ADDRESS", "CURRENT_SELF_REFERENCE"} {
			if !strings.Contains(wire, required) {
				t.Errorf("cognition=%t missing authoritative addressing %s", cognition, required)
			}
		}
		for _, forbidden := range []string{"SHARED_ADDRESS", "OTHER_PROFILE_ADDRESS", "OTHER_ACTOR_ADDRESS"} {
			if strings.Contains(wire, forbidden) {
				t.Errorf("cognition=%t leaked addressing %s", cognition, forbidden)
			}
		}
	}
	if jsonString(input.Projection.Relationships) != before {
		t.Fatal("rendering changed relationship facts")
	}
}
