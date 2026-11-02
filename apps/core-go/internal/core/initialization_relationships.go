package core

import (
	"fmt"
	"strings"
)

// Activation prefers initial_relationships, but falls back to life-profile
// seeds when that collection is empty. Validate both persisted collections so
// neither an empty nor a populated primary collection can hide invalid seeds.
func initializationRelationshipValidationDiagnostic(value map[string]any) (string, string, bool) {
	persona := mapValue(value["core_persona"])
	profiles := personalityProfileIDs(persona)
	for _, collection := range []struct {
		path string
		raw  any
	}{
		{"core_persona.life_profile.relationship_seeds", mapValue(persona["life_profile"])["relationship_seeds"]},
		{"initial_relationships", value["initial_relationships"]},
	} {
		items, ok := collection.raw.([]any)
		if !ok {
			return "type_invalid", collection.path, false
		}
		for index, raw := range items {
			path := fmt.Sprintf("%s[%d]", collection.path, index)
			item, ok := raw.(map[string]any)
			if !ok || item == nil {
				return "type_invalid", path, false
			}
			target, ok := item["target_actor_id"].(string)
			if !ok || strings.TrimSpace(target) == "" {
				return "required_value_missing", path + ".target_actor_id", false
			}
			// Reusable initialization definitions refer to the Owner through an
			// alias, never an arbitrary Actor or the character itself.
			if resolveInitializationActorRef(target, "actor_user", "actor_self") != "actor_user" {
				return "reference_invalid", path + ".target_actor_id", false
			}
			if rawProfile, present := item["profile_id"]; present && rawProfile != nil {
				if _, ok := rawProfile.(string); !ok {
					return "type_invalid", path + ".profile_id", false
				}
			}
			if _, err := initializationScopeProfileID(item, "initial_relationship_profile_invalid", profiles); err != nil {
				return "reference_invalid", path + ".profile_id", false
			}
		}
	}
	return "", "", true
}
