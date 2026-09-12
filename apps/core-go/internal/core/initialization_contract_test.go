package core

import (
	"strings"
	"testing"
)

func TestPrepareInitializationResponseNormalizesSafeContainersBeforeValidation(t *testing.T) {
	value := map[string]any{
		"core_persona": defaultCorePersona("", "影者"),
	}
	prepared, err := prepareInitializationResponse(value)
	if err != nil {
		t.Fatalf("safe structural omissions rejected as initialization_persona_invalid: %v", err)
	}
	if !hasInitializationEnvelope(prepared) || !validInitialization(prepared) {
		t.Fatalf("prepared initialization remains invalid: %#v", prepared)
	}
	if len(arrayValue(prepared["initial_relationships"])) != 0 || len(arrayValue(prepared["initial_goals"])) != 0 || len(arrayValue(prepared["initial_intentions"])) != 0 || len(arrayValue(mapValue(prepared["developing_self"])["claims"])) != 0 || !isObjectValue(prepared["extensions"]) {
		t.Fatalf("safe containers were not normalized: %#v", prepared)
	}
}

func TestPrepareInitializationResponseStillRejectsMissingPersonaSemantics(t *testing.T) {
	value := map[string]any{"core_persona": defaultCorePersona("", "影者")}
	delete(mapValue(mapValue(value["core_persona"])["personality"]), "openness")
	if _, err := prepareInitializationResponse(value); err == nil || err.Error() != "initialization_persona_invalid" {
		t.Fatalf("missing personality semantics err=%v", err)
	}
}

func TestPrepareInitializationResponseFillsOnlyNonSemanticPersonaStructure(t *testing.T) {
	value := map[string]any{
		"core_persona": map[string]any{
			"identity":          map[string]any{"name": "影者"},
			"personality":       defaultPersonality(),
			"behavioral_policy": defaultPolicy(),
			"life_profile":      map[string]any{},
		},
	}
	prepared, err := prepareInitializationResponse(value)
	if err != nil {
		t.Fatalf("non-semantic persona scaffolding rejected: %v", err)
	}
	persona := mapValue(prepared["core_persona"])
	identity := mapValue(persona["identity"])
	if _, ok := identity["notes"]; !ok || identity["name"] != "影者" || identity["timezone"] != nil {
		t.Fatalf("identity placeholders were not safely completed: %#v", identity)
	}
	if len(mapValue(persona["personality_system"])) == 0 || !hasInitializationKeys(mapValue(persona["life_profile"]), []string{"appearance", "social_background", "preferences", "life_habits", "recurring_commitments", "relationship_seeds", "character_constraints"}) {
		t.Fatalf("persona scaffolding was not completed: %#v", persona)
	}
	if !validInitialization(prepared) {
		t.Fatalf("safely completed persona remains invalid: %#v", prepared)
	}
}

func TestInitializationStructuredFallbackIsInvalidJSON(t *testing.T) {
	if _, err := structuredResultForRole("initialization", ProviderCompletion{Structured: map[string]any{}, StructuredFallback: true}); err == nil || err.Error() != "initialization_response_invalid_json" {
		t.Fatalf("initialization fallback err=%v", err)
	}
	value, err := structuredResultForRole("reflection", ProviderCompletion{Structured: map[string]any{}, StructuredFallback: true})
	if err != nil || value == nil {
		t.Fatalf("non-initialization fallback changed: value=%#v err=%v", value, err)
	}
}

func TestInitializationAnalysisCorrelationIsUniquePerUserAttempt(t *testing.T) {
	first, second := initializationAnalysisCorrelation(), initializationAnalysisCorrelation()
	if first == second || !strings.HasPrefix(first, "initialization-analysis:") || !strings.HasPrefix(second, "initialization-analysis:") {
		t.Fatalf("analysis correlations are not unique: %q %q", first, second)
	}
}

func TestValidInitializationAcceptsOpenRelationshipLabelAndActorUser(t *testing.T) {
	value := map[string]any{
		"core_persona": map[string]any{
			"schema_version":     1,
			"identity":           defaultIdentity("", "影者"),
			"personality":        defaultPersonality(),
			"behavioral_policy":  defaultPolicy(),
			"life_profile":       defaultLifeProfile(),
			"personality_system": defaultPersonalitySystem(),
		},
		"schema_version":        1,
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

func TestValidInitializationRequiresCompleteDeclaredPersonalityProfile(t *testing.T) {
	persona := defaultCorePersona("", "影者")
	system := defaultPersonalitySystem()
	system["mode"] = "multiple"
	system["active_profile_id"] = "warm"
	system["profiles"] = []any{completeInitializationProfile("warm")}
	persona["personality_system"] = system
	value := map[string]any{
		"core_persona": persona, "schema_version": 1,
		"developing_self": map[string]any{"claims": []any{}}, "initial_relationships": []any{},
		"initial_goals": []any{}, "initial_intentions": []any{}, "extensions": map[string]any{},
	}
	if !validInitialization(value) {
		t.Fatal("complete personality profile should be valid")
	}
	profile := completeInitializationProfile("warm")
	delete(profile, "voice")
	system["profiles"] = []any{profile}
	if validInitialization(value) {
		t.Fatal("missing personality voice contract should be rejected")
	}
}

func completeInitializationProfile(id string) map[string]any {
	return map[string]any{
		"id": id, "name": id, "identity": map[string]any{}, "personality": map[string]any{},
		"behavioral_policy": map[string]any{}, "emotional_state": map[string]any{}, "voice": map[string]any{},
		"body_language": map[string]any{}, "behavior_state_machine": map[string]any{}, "behavior_loops": map[string]any{},
		"scenario_behavior": map[string]any{}, "secrets": map[string]any{}, "intimacy_progression": map[string]any{},
		"output_preferences": []any{}, "fears": []any{}, "desires": []any{}, "extensions": map[string]any{},
	}
}

func TestInitializationShapeDoesNotFillMissingPersonalityFields(t *testing.T) {
	value, changed := normalizeProviderStructured(map[string]any{"core_persona": map[string]any{}}, "initialization_response", initializationResponseSchema())
	if changed != nil || len(mapValue(value["core_persona"])) != 0 {
		t.Fatalf("initialization shape was silently repaired: value=%#v changed=%#v", value, changed)
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

func TestNormalizeInitializationProfilesKeepsKnownFieldsAndMovesUnknowns(t *testing.T) {
	value := normalizeInitializationResponse(map[string]any{
		"core_persona": map[string]any{
			"identity": map[string]any{"name": "影者"}, "personality": map[string]any{},
			"behavioral_policy": map[string]any{}, "life_profile": map[string]any{},
			"personality_system": map[string]any{
				"mode": "multiple", "active_profile_id": "warm", "profiles": []any{map[string]any{
					"id": "warm", "name": "温柔人格", "output_preferences": []any{map[string]any{"channel": "image"}}, "unclassified": "保留",
				}}, "switching": map[string]any{}, "influence": map[string]any{}, "conflict_resolution": map[string]any{}, "integration": map[string]any{}, "behavior_state_machine": map[string]any{},
			},
		},
		"developing_self": map[string]any{"claims": []any{}}, "initial_relationships": []any{}, "initial_goals": []any{}, "initial_intentions": []any{}, "extensions": map[string]any{},
	})
	profiles := arrayValue(mapValue(mapValue(value["core_persona"])["personality_system"])["profiles"])
	profile := mapValue(profiles[0])
	if stringValue(profile["id"]) != "warm" || len(arrayValue(profile["output_preferences"])) != 1 || stringValue(mapValue(profile["extensions"])["unclassified"]) != "保留" {
		t.Fatalf("profile normalization = %#v", profile)
	}
}
