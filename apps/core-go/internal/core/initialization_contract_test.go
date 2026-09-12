package core

import (
	"errors"
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

func TestPrepareInitializationResponseFillsMissingPersonaFieldsWithDefaults(t *testing.T) {
	value := map[string]any{"core_persona": defaultCorePersona("", "影者")}
	delete(mapValue(mapValue(value["core_persona"])["personality"]), "openness")
	delete(mapValue(mapValue(value["core_persona"])["behavioral_policy"]), "response_style")
	prepared, err := prepareInitializationResponse(value)
	if err != nil {
		t.Fatalf("missing persona fields were not defaulted: %v", err)
	}
	persona := mapValue(prepared["core_persona"])
	if mapValue(persona["personality"])["openness"] != defaultPersonality()["openness"] || mapValue(persona["behavioral_policy"])["response_style"] != defaultPolicy()["response_style"] {
		t.Fatalf("persona defaults were not applied: %#v", persona)
	}
}

func TestPrepareInitializationResponseStillRejectsExplicitInvalidValues(t *testing.T) {
	value := map[string]any{"core_persona": defaultCorePersona("", "影者")}
	mapValue(mapValue(value["core_persona"])["identity"])["timezone"] = "Mars/Olympus"
	_, err := prepareInitializationResponse(value)
	var failure *initializationAnalysisError
	if err == nil || err.Error() != "initialization_persona_invalid" || !errors.As(err, &failure) {
		t.Fatalf("explicit invalid timezone err=%v", err)
	}
	details := failure.PublicDetails()
	validation := mapValue(details["validation_error"])
	if stringValue(validation["type"]) != "value_invalid" || stringValue(validation["path"]) != "core_persona.identity.timezone" {
		t.Fatalf("invalid timezone details=%#v", details)
	}
}

func TestInitializationErrorAddsCorrelationWithoutExposingPayload(t *testing.T) {
	failure := initializationErrorWithCorrelation(&initializationAnalysisError{Code: "initialization_persona_invalid", ValidationType: "reference_invalid", Path: "initial_intentions[0].goal_index"}, "initialization-analysis:test")
	details := failure.PublicDetails()
	validation := mapValue(details["validation_error"])
	if stringValue(details["correlation_id"]) != "initialization-analysis:test" || stringValue(validation["path"]) != "initial_intentions[0].goal_index" || strings.Contains(jsonString(details), "core_persona") {
		t.Fatalf("public initialization details=%#v", details)
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

func TestInitializationStructuredFallbackUsesSafeDefaults(t *testing.T) {
	value, err := structuredResultForRole("initialization", ProviderCompletion{Structured: map[string]any{}, StructuredFallback: true})
	if err != nil {
		t.Fatalf("normal initialization response with missing fields was rejected: %v", err)
	}
	prepared, err := prepareInitializationResponse(value)
	if err != nil || !validInitialization(prepared) {
		t.Fatalf("initialization fallback was not safely completed: value=%#v prepared=%#v err=%v", value, prepared, err)
	}
}

func TestInitializationAnalysisCorrelationIsUniquePerUserAttempt(t *testing.T) {
	first, second := initializationAnalysisCorrelation(), initializationAnalysisCorrelation()
	if first == second || !strings.HasPrefix(first, "initialization-analysis:") || !strings.HasPrefix(second, "initialization-analysis:") {
		t.Fatalf("analysis correlations are not unique: %q %q", first, second)
	}
}

func TestInitializationUsesJSONObjResponseFormatAndCanonicalSkeleton(t *testing.T) {
	format := providerResponseFormatForSchema("initialization", "initialization_response", initializationResponseSchema())
	if stringValue(format["type"]) != "json_object" || format["json_schema"] != nil {
		t.Fatalf("initialization response format=%#v", format)
	}
	if !strings.Contains(initializationResponseShapeInstruction, `"core_persona"`) || !strings.Contains(initializationResponseShapeInstruction, `"personality_system"`) || !strings.Contains(initializationResponseShapeInstruction, `"profiles"`) || !strings.Contains(initializationResponseShapeInstruction, "never replace them with actor_self") {
		t.Fatalf("canonical initialization skeleton is incomplete: %s", initializationResponseShapeInstruction)
	}
	cognitive := providerResponseFormatForSchema("cognitive_assessment", "conversation_turn_response", cognitiveTurnResponseSchema())
	if stringValue(cognitive["type"]) != "json_schema" || mapValue(cognitive["json_schema"])["strict"] != true {
		t.Fatalf("non-initialization strict schema changed: %#v", cognitive)
	}
}

func TestPrepareInitializationResponseNormalizesCommonLLMAliases(t *testing.T) {
	value := map[string]any{
		"core_persona": map[string]any{
			"identity": map[string]any{"name": "岚音", "profession": "天文摄影师", "location": "上海", "values": []any{"诚实", "独立"}},
			"personality_system": map[string]any{"mode": "multiple", "profiles": []any{
				map[string]any{"id": "profile_jinghai", "name": "静海", "voice": map[string]any{"speed": "slow"}},
				map[string]any{"id": "profile_liuhuo", "name": "流火", "voice": map[string]any{"speed": "fast"}},
			}},
		},
		"initial_goals": []any{
			map[string]any{"id": "goal_archive", "description": "完成档案"},
			map[string]any{"id": "goal_meteor", "description": "观测流星雨"},
		},
		"initial_intentions": []any{map[string]any{"description": "推进档案", "linked_goal_id": "goal_archive"}},
		"relationships":      []any{map[string]any{"actor": "actor_user", "type": "长期搭档", "intimacy": "亲密朋友"}},
	}
	prepared, err := prepareInitializationResponse(value)
	if err != nil {
		t.Fatalf("common LLM aliases were rejected: %v", err)
	}
	identity := mapValue(mapValue(prepared["core_persona"])["identity"])
	intention := mapValue(arrayValue(prepared["initial_intentions"])[0])
	relationship := mapValue(arrayValue(prepared["initial_relationships"])[0])
	if stringValue(identity["occupation"]) != "天文摄影师" || stringValue(identity["residence"]) != "上海" || len(arrayValue(identity["core_values"])) != 2 || stringValue(intention["action"]) != "推进档案" || intValue(intention["goal_index"]) != 0 || stringValue(relationship["target_actor_id"]) != "actor_user" || stringValue(mapValue(relationship["role"])["label"]) == "" {
		t.Fatalf("LLM aliases were not normalized: identity=%#v intention=%#v relationship=%#v", identity, intention, relationship)
	}
}

func TestPrepareInitializationResponseDropsOrRepairsInvalidOptionalCandidates(t *testing.T) {
	value := map[string]any{
		"core_persona": defaultCorePersona("", "岚音"),
		"developing_self": map[string]any{"claims": []any{
			map[string]any{"category": "relationship_state", "claim": "未分类候选"},
			map[string]any{"category": "interest", "claim": "喜欢天文", "confidence": 2.0},
		}},
		"initial_goals":         []any{map[string]any{"description": "完成档案", "importance": 4.0, "urgency": "high"}},
		"initial_intentions":    []any{map[string]any{"action": "推进档案", "goal_index": 99, "confidence": -1.0}},
		"initial_relationships": []any{map[string]any{"target_actor_id": "actor_user", "role": map[string]any{"label": "搭档"}, "metrics": map[string]any{"trust": "high"}, "trend": "warming"}},
	}
	prepared, err := prepareInitializationResponse(value)
	if err != nil {
		t.Fatalf("invalid optional candidates rejected the whole initialization: %v", err)
	}
	claims := arrayValue(mapValue(prepared["developing_self"])["claims"])
	goal := mapValue(arrayValue(prepared["initial_goals"])[0])
	intention := mapValue(arrayValue(prepared["initial_intentions"])[0])
	relationship := mapValue(arrayValue(prepared["initial_relationships"])[0])
	claimConfidence, _ := numberFloat(mapValue(claims[0])["confidence"])
	goalImportance, _ := numberFloat(goal["importance"])
	goalUrgency, _ := numberFloat(goal["urgency"])
	intentionConfidence, _ := numberFloat(intention["confidence"])
	if len(claims) != 1 || stringValue(mapValue(claims[0])["category"]) != "interest" || claimConfidence != 0.5 || goalImportance != 1 || goalUrgency != 0.5 || intention["goal_index"] != nil || intentionConfidence != 0.5 || len(mapValue(relationship["metrics"])) != 0 || stringValue(relationship["trend"]) != "stable" {
		t.Fatalf("optional candidates were not normalized: claims=%#v goal=%#v intention=%#v relationship=%#v", claims, goal, intention, relationship)
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

func TestInitializationCompletesMissingProfileFieldsButPreservesProfileIdentity(t *testing.T) {
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
	prepared, err := prepareInitializationResponse(value)
	if err != nil || !isObjectValue(mapValue(arrayValue(mapValue(mapValue(prepared["core_persona"])["personality_system"])["profiles"])[0])["voice"]) {
		t.Fatalf("missing personality voice should normalize to an empty object: prepared=%#v err=%v", prepared, err)
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
