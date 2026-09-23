package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	capabilitycontract "github.com/fluctlight/local-ai-companion/apps/core-go/internal/capability"
)

func TestInitializationProviderErrorsDistinguishTimeoutAndCancellation(t *testing.T) {
	if got := initializationProviderErrorCode(fmt.Errorf("provider request failed: %w", context.DeadlineExceeded)); got != "initialization_provider_timeout" {
		t.Fatalf("deadline error code = %q", got)
	}
	if got := initializationProviderErrorCode(fmt.Errorf("provider request failed: %w", context.Canceled)); got != "initialization_provider_cancelled" {
		t.Fatalf("cancellation error code = %q", got)
	}
	if got := initializationProviderErrorCode(errors.New("connection refused")); got != "initialization_provider_unavailable" {
		t.Fatalf("generic Provider error code = %q", got)
	}
}

func TestInitializationProviderFailureLogKeepsBoundedCause(t *testing.T) {
	source, err := os.ReadFile("app.go")
	if err != nil {
		t.Fatal(err)
	}
	body := sourceBetween(t, string(source), "func (a *App) AnalyzeDescription", "func validInitializationDescription")
	for _, required := range []string{
		`"provider_error_code", initializationProviderCauseCode(err)`,
		`"retryable", failure.Retryable`,
		`"error_type", fmt.Sprintf("%T", err)`,
		`"safe_cause", boundedLifecycleCause(err.Error())`,
	} {
		if !strings.Contains(body, required) {
			t.Fatalf("initialization Provider failure log missing %q", required)
		}
	}
}

func TestInitializationProviderCauseCodePreservesStructuredParseFailure(t *testing.T) {
	if got := initializationProviderCauseCode(errors.New("initialization_response_invalid_json")); got != "initialization_response_invalid_json" {
		t.Fatalf("structured parse cause code = %q", got)
	}
	if got := initializationProviderCauseCode(fmt.Errorf("provider request failed: %w", context.DeadlineExceeded)); got != "request_timeout" {
		t.Fatalf("timeout cause code = %q", got)
	}
}

func TestInitializationStructuredCandidateRejectsEmbeddedJSONFence(t *testing.T) {
	payload := map[string]any{
		"core_persona": map[string]any{
			"identity": map[string]any{"name": "岚音", "notes": strings.Repeat("完整设定", 3100)},
		},
		"initial_relationships": []any{},
		"initial_goals":         []any{},
		"initial_intentions":    []any{},
	}
	content := "以下是完整初始化对象：\n```json\n" + jsonString(payload) + "\n```\n以上内容已经按要求整理。"
	if len([]rune(content)) <= 12000 {
		t.Fatalf("fixture is too short: %d", len([]rune(content)))
	}
	candidates := providerStructuredCandidates(map[string]any{"content": content})
	if _, ok := parseStructuredCandidates(candidates); ok {
		t.Fatal("Markdown fenced initialization content must not be treated as formal Content JSON")
	}
	truncated := "前置说明\n```json\n" + jsonString(payload)[:6000]
	if _, ok := parseStructuredCandidates(providerStructuredCandidates(map[string]any{"content": truncated})); ok {
		t.Fatal("truncated initialization JSON was accepted")
	}
}

func TestInitializationNonEmptyParseFailureIsTypedAndMetadataOnly(t *testing.T) {
	const privateCanary = "PRIVATE_INITIALIZATION_CARD_CANARY"
	truncatedCandidate := `{"core_persona":{"identity":{"name":"` + privateCanary
	candidates := []string{truncatedCandidate}
	if _, ok, err := parseStructuredCandidatesForRole("initialization", candidates); ok || err == nil || err.Error() != "initialization_response_invalid_json" {
		t.Fatalf("non-empty parse failure = ok=%v err=%v", ok, err)
	}
	diagnostic := providerResponseDiagnostic(map[string]any{"content": truncatedCandidate}, candidates, 0)
	addStructuredParseFailureDiagnostic(diagnostic, candidates, "length")
	if diagnostic["parse_error"] != "structured_response_truncated" || diagnostic["finish_reason"] != "length" || diagnostic["delimiters_balanced"] != false {
		t.Fatalf("truncated diagnostic = %#v", diagnostic)
	}
	metadata := providerDiagnosticResponse("initialization", diagnostic)
	encoded := jsonString(metadata)
	if strings.Contains(encoded, privateCanary) || !strings.Contains(encoded, "structured_response_truncated") || !strings.Contains(encoded, "candidate_lengths") {
		t.Fatalf("initialization parse metadata leaked or lost structure: %s", encoded)
	}

	invalidCandidate := `{"core_persona":,}`
	invalidDiagnostic := providerResponseDiagnostic(map[string]any{"content": invalidCandidate}, []string{invalidCandidate}, 0)
	addStructuredParseFailureDiagnostic(invalidDiagnostic, []string{invalidCandidate}, "stop")
	if invalidDiagnostic["parse_error"] != "structured_response_invalid_json" || intValue(invalidDiagnostic["syntax_offset"]) <= 0 {
		t.Fatalf("invalid JSON diagnostic = %#v", invalidDiagnostic)
	}
	if _, ok, err := parseStructuredCandidatesForRole("cognitive_assessment", candidates); ok || err == nil || err.Error() != "structured_response_invalid_json" {
		t.Fatalf("non-initialization malformed Content must fail explicitly: ok=%v err=%v", ok, err)
	}
}

func TestInitializationCanonicalOwnersIncludeDenseCharacterFields(t *testing.T) {
	persona := defaultCorePersona("fl-1", "岚音")
	identity := mapValue(persona["identity"])
	for _, key := range []string{"nickname", "height", "height_cm", "blood_type", "birthplace", "background_story"} {
		if _, ok := identity[key]; !ok {
			t.Fatalf("default identity missing %q", key)
		}
	}
	life := mapValue(persona["life_profile"])
	appearance := mapValue(life["appearance"])
	for _, key := range []string{"description", "physical_features", "daily_outfit_preferences", "style_preferences"} {
		if _, ok := appearance[key]; !ok {
			t.Fatalf("default appearance missing %q", key)
		}
	}
	if _, ok := life["media_preferences"]; !ok {
		t.Fatal("default life profile missing media_preferences")
	}
	system := mapValue(persona["personality_system"])
	for _, key := range []string{"core_relationship", "core_conflict", "forced_activation"} {
		if _, ok := system[key]; !ok {
			t.Fatalf("default personality system missing %q", key)
		}
	}
	if _, ok := persona["extensions"]; !ok {
		t.Fatal("Core Persona missing runtime semantic extensions")
	}
	schema := initializationResponseSchema()
	coreProperties := mapValue(mapValue(schema["properties"])["core_persona"])
	coreProperties = mapValue(coreProperties["properties"])
	identityProperties := mapValue(mapValue(coreProperties["identity"])["properties"])
	lifeProperties := mapValue(mapValue(coreProperties["life_profile"])["properties"])
	systemProperties := mapValue(mapValue(coreProperties["personality_system"])["properties"])
	for _, key := range []string{"nickname", "height", "height_cm", "blood_type", "birthplace", "background_story"} {
		if identityProperties[key] == nil {
			t.Fatalf("initialization schema identity missing %q", key)
		}
	}
	if lifeProperties["media_preferences"] == nil || systemProperties["core_relationship"] == nil || systemProperties["core_conflict"] == nil || systemProperties["forced_activation"] == nil || coreProperties["extensions"] == nil {
		t.Fatalf("initialization schema missing dense semantic owners: %#v", coreProperties)
	}
}

func TestInitializationSourceIsDedicatedAndExcludedFromOrdinaryPrompt(t *testing.T) {
	appSource, err := os.ReadFile("app.go")
	if err != nil {
		t.Fatal(err)
	}
	appText := string(appSource)
	for _, required := range []string{"persistInitializationAnalysisSource", "linkInitializationSourceTx", "fluctlight_initialization_sources", "fluctlight_initialization_source_links"} {
		if !strings.Contains(appText, required) {
			t.Fatalf("initialization source flow missing %q", required)
		}
	}
	digestIndex := strings.Index(appText, "initializationActivationDigest = stableDigest(jsonString")
	idMutationIndex := strings.Index(appText, `identity["id"] = id`)
	if digestIndex < 0 || idMutationIndex < 0 || digestIndex > idMutationIndex {
		t.Fatal("initialization projection digest is captured after activation mutates the accepted Persona")
	}
	promptSource, err := os.ReadFile("provider_prompt_composer.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(promptSource), "source_text") || strings.Contains(string(promptSource), "fluctlight_initialization_sources") {
		t.Fatal("immutable initialization source entered ordinary Prompt composition")
	}
	contextSource, err := os.ReadFile("app_capability_context.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(contextSource), "fluctlight_initialization_sources") || strings.Contains(string(contextSource), "source_text") {
		t.Fatal("immutable initialization source entered ordinary ContextProjection")
	}
	detailSource, err := os.ReadFile("detail.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(detailSource), "s.owner_actor_id=$2") || !strings.Contains(string(detailSource), "f.created_by_actor_id=$2") {
		t.Fatal("initialization source detail is not Owner scoped")
	}
}

func TestRuntimePersonaKeepsSemanticExtensionsWithoutSourceBookkeeping(t *testing.T) {
	filtered := filterCorePersona(map[string]any{
		"identity":   map[string]any{"name": "岚音"},
		"extensions": map[string]any{"special_ritual": "睡前整理画稿", "source_text": "PRIVATE_CARD", "source_digest": "secret-digest"},
	})
	extensions := mapValue(filtered["extensions"])
	if stringValue(extensions["special_ritual"]) == "" {
		t.Fatalf("semantic extension was dropped: %#v", filtered)
	}
	if extensions["source_text"] != nil || extensions["source_digest"] != nil {
		t.Fatalf("source bookkeeping entered runtime Persona: %#v", filtered)
	}
}

func TestInitializationRelationshipsAcceptOnlyActorUser(t *testing.T) {
	source, err := os.ReadFile("app.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(source)
	start := strings.Index(body, "func (a *App) insertRelationshipSeeds")
	if start < 0 || !strings.Contains(body[start:], "target != humanActorID") || !strings.Contains(body[start:], "initial_relationship_target_not_actor_user") {
		t.Fatal("initial Relationship persistence accepts a target other than actor_user")
	}
}

func TestInitializationRejectsSemanticEmptyProviderFallback(t *testing.T) {
	_, err := prepareInitializationResponse(map[string]any{})
	var failure *initializationAnalysisError
	if !errors.As(err, &failure) || failure.Code != "initialization_response_semantic_empty" || failure.ValidationType != "semantic_empty" || failure.Path != "provider_response" || !failure.Retryable || failure.PublicDetails()["retryable"] != true {
		t.Fatalf("semantic-empty fallback error = %#v, %v", failure, err)
	}
}

func TestInitializationPromptRestoresCompleteSemanticVocabulary(t *testing.T) {
	for _, required := range []string{
		"nickname", "height_cm", "blood_type", "birthplace", "background_story",
		"daily_outfit_preferences", "media_preferences", "actor_user only",
		`target_actor_id:"actor_user"`, "return an empty array instead of inventing one",
		"core_relationship", "core_conflict", "forced_activation", "integration/fusion",
		"behavior_loops.loops", "do not leave behavior_loops empty",
		"situational contrast", "one complex single personality", "Never invent unsupported",
	} {
		if !strings.Contains(initializationSemanticCoverageInstruction, required) {
			t.Fatalf("initialization semantic coverage prompt missing %q", required)
		}
	}
	for instruction, required := range map[string][]string{
		initializationMediaOwnershipInstruction:             {"life_profile.media_preferences", "profile's output_preferences", "global media_preferences object empty"},
		initializationPersonalitySystemOwnershipInstruction: {"core_relationship", "share memory", "core_conflict", "Do not place a conflict summary", "integration.stage", "integration.shared_memory_policy", "do not leave integration empty"},
		initializationProfileVoiceOwnershipInstruction:      {"voice.speech_patterns", "derived numeric speed", "behavior_loops.loops"},
		initializationConstraintOwnershipInstruction:        {"professional-impersonation prohibition", "fact-invention prohibition", "life_profile.character_constraints", "separate entry"},
	} {
		for _, fragment := range required {
			if !strings.Contains(instruction, fragment) {
				t.Fatalf("initialization owner instruction missing %q", fragment)
			}
		}
	}
	for _, required := range []string{"core_persona.life_profile.media_preferences", "profile's output_preferences", "do not justify leaving the global media_preferences object empty"} {
		if !strings.Contains(initializationMediaOwnershipInstruction, required) {
			t.Fatalf("initialization media ownership prompt missing %q", required)
		}
	}
	format := providerResponseFormatForSchema("initialization", "initialization_response", initializationResponseSchema())
	if stringValue(format["type"]) != "json_object" {
		t.Fatalf("initialization response format = %#v, want json_object", format)
	}
}

func TestInitializationModelRunDiagnosticsAreMetadataOnly(t *testing.T) {
	const canary = "PRIVATE_INITIALIZATION_CARD_CANARY"
	messages := providerDiagnosticMessages("initialization", []map[string]any{{"role": "user", "content": canary}})
	response := providerDiagnosticResponse("initialization", map[string]any{"core_persona": map[string]any{"notes": canary}})
	encoded := jsonString(map[string]any{"messages": messages, "response": response})
	if strings.Contains(encoded, canary) || !strings.Contains(encoded, "metadata_only") || !strings.Contains(encoded, "prompt_digest") || !strings.Contains(encoded, "response_digest") {
		t.Fatalf("initialization diagnostics leaked source/response: %s", encoded)
	}
}

func TestInitializationDescriptionLimitIsByteBased(t *testing.T) {
	if !validInitializationDescription(strings.Repeat("a", InitializationDescriptionMaxBytes)) {
		t.Fatal("exact initialization byte limit was rejected")
	}
	if validInitializationDescription(strings.Repeat("界", InitializationDescriptionMaxBytes/3+1)) {
		t.Fatal("multibyte initialization source exceeded the byte limit without rejection")
	}
}

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

func TestInitializationStructuredFallbackCannotReplaceNonEmptyCharacterCard(t *testing.T) {
	structured, err := structuredResultForRole("initialization", ProviderCompletion{Structured: map[string]any{}, StructuredFallback: true})
	if err == nil {
		_, err = prepareInitializationResponse(structured)
	}
	if err == nil {
		t.Fatal("semantic-empty initialization fallback was accepted as a default Persona")
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

// TestPrepareInitializationResponseNormalizesRealProviderAliases covers the
// shorthand field names emitted by the live mlx-serve model. These aliases are
// structurally equivalent to the canonical contract and must be normalized
// before validation/persistence rather than making an otherwise usable
// multi-profile foundation fail closed.
func TestPrepareInitializationResponseNormalizesRealProviderAliases(t *testing.T) {
	persona := defaultCorePersona("", "岚音")
	system := mapValue(persona["personality_system"])
	system["mode"] = "multiple"
	system["profiles"] = []any{
		completeInitializationProfile("profile_jinghai"),
		completeInitializationProfile("profile_liuhuo"),
	}
	value := map[string]any{
		"core_persona": persona,
		"initial_goals": []any{
			map[string]any{"id": "goal_archive", "content": "完成年度深空摄影档案", "associated_profile": "profile_jinghai"},
			map[string]any{"id": "goal_meteor", "description": "规划流星雨观测", "associated_profile": "profile_liuhuo"},
		},
		"initial_intentions": []any{
			map[string]any{"content": "协作完成年度深空摄影档案", "associated_profile": "profile_jinghai"},
			map[string]any{"content": "共同规划下一次流星雨观测", "associated_profile": "profile_liuhuo"},
		},
		"initial_relationships": []any{
			map[string]any{"target": "actor_user", "type": "长期搭档", "closeness": "亲密朋友"},
		},
	}

	prepared, err := prepareInitializationResponse(value)
	if err != nil {
		t.Fatalf("live-provider aliases were rejected: %v", err)
	}
	goals := arrayValue(prepared["initial_goals"])
	intentions := arrayValue(prepared["initial_intentions"])
	relationships := arrayValue(prepared["initial_relationships"])
	if len(goals) != 2 || stringValue(mapValue(goals[0])["description"]) != "完成年度深空摄影档案" || stringValue(mapValue(goals[0])["profile_id"]) != "profile_jinghai" || stringValue(mapValue(goals[1])["profile_id"]) != "profile_liuhuo" {
		t.Fatalf("goal profile aliases were not normalized: %#v", goals)
	}
	if len(intentions) != 2 || stringValue(mapValue(intentions[0])["action"]) != "协作完成年度深空摄影档案" || stringValue(mapValue(intentions[1])["action"]) != "共同规划下一次流星雨观测" || stringValue(mapValue(intentions[0])["profile_id"]) != "profile_jinghai" || stringValue(mapValue(intentions[1])["profile_id"]) != "profile_liuhuo" {
		t.Fatalf("intention aliases were not normalized: %#v", intentions)
	}
	if len(relationships) != 1 || stringValue(mapValue(relationships[0])["target_actor_id"]) != "actor_user" {
		t.Fatalf("relationship target alias was not normalized: %#v", relationships)
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
	relationship := mapValue(arrayValue(prepared["initial_relationships"])[0])
	claimConfidence, _ := numberFloat(mapValue(claims[0])["confidence"])
	goalImportance, _ := numberFloat(goal["importance"])
	goalUrgency, _ := numberFloat(goal["urgency"])
	if len(claims) != 1 || stringValue(mapValue(claims[0])["category"]) != "interest" || claimConfidence != 0.5 || goalImportance != 1 || goalUrgency != 0.5 || len(arrayValue(prepared["initial_intentions"])) != 0 || len(mapValue(relationship["metrics"])) != 0 || stringValue(relationship["trend"]) != "stable" {
		t.Fatalf("optional candidates were not normalized: claims=%#v goal=%#v intentions=%#v relationship=%#v", claims, goal, prepared["initial_intentions"], relationship)
	}
}

func TestPrepareInitializationResponseAssignsImplicitIntentionGoalByPosition(t *testing.T) {
	value := map[string]any{
		"core_persona": defaultCorePersona("", "岚音"),
		"initial_goals": []any{
			map[string]any{"description": "整理档案"},
			map[string]any{"description": "观测流星雨"},
		},
		"initial_intentions": []any{
			map[string]any{"action": "整理第一批照片"},
			map[string]any{"action": "检查观测设备"},
			map[string]any{"action": "没有对应目标的额外候选"},
		},
	}

	prepared, err := prepareInitializationResponse(value)
	if err != nil {
		t.Fatalf("implicit intention goal normalization failed: %v", err)
	}
	intentions := arrayValue(prepared["initial_intentions"])
	if len(intentions) != 2 || intValue(mapValue(intentions[0])["goal_index"]) != 0 || intValue(mapValue(intentions[1])["goal_index"]) != 1 {
		t.Fatalf("implicit intention goals were not normalized by position: %#v", intentions)
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
	system["profiles"] = []any{completeInitializationProfile("warm"), completeInitializationProfile("cool")}
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
	system["profiles"] = []any{profile, completeInitializationProfile("cool")}
	prepared, err := prepareInitializationResponse(value)
	if err != nil || !isObjectValue(mapValue(arrayValue(mapValue(mapValue(prepared["core_persona"])["personality_system"])["profiles"])[0])["voice"]) {
		t.Fatalf("missing personality voice should normalize to an empty object: prepared=%#v err=%v", prepared, err)
	}
}

func TestInitializationModeMatchesDeclaredProfileCardinality(t *testing.T) {
	multipleWithOne := defaultCorePersona("", "单人格")
	multipleSystem := defaultPersonalitySystem()
	multipleSystem["mode"] = "multiple"
	multipleSystem["profiles"] = []any{completeInitializationProfile("only")}
	multipleSystem["active_profile_id"] = "default"
	multipleWithOne["personality_system"] = multipleSystem
	if validInitialization(map[string]any{
		"schema_version": 2, "core_persona": multipleWithOne,
		"developing_self":       map[string]any{"claims": []any{}},
		"initial_relationships": []any{}, "initial_goals": []any{},
		"initial_intentions": []any{}, "extensions": map[string]any{},
	}) {
		t.Fatal("multiple mode accepted fewer than two declared profiles")
	}

	singleWithTwo := defaultCorePersona("", "复杂单人格")
	singleSystem := defaultPersonalitySystem()
	singleSystem["mode"] = "single"
	singleSystem["profiles"] = []any{completeInitializationProfile("situational"), completeInitializationProfile("private")}
	singleSystem["active_profile_id"] = "default"
	singleWithTwo["personality_system"] = singleSystem
	if validInitialization(map[string]any{
		"schema_version": 2, "core_persona": singleWithTwo,
		"developing_self":       map[string]any{"claims": []any{}},
		"initial_relationships": []any{}, "initial_goals": []any{},
		"initial_intentions": []any{}, "extensions": map[string]any{},
	}) {
		t.Fatal("single mode accepted multiple declared profiles")
	}
}

func TestInitializationPreservesDenseCharacterCardSemanticOwners(t *testing.T) {
	value := normalizeInitializationResponse(map[string]any{
		"core_persona": map[string]any{
			"identity": map[string]any{
				"name": "岚音", "nickname": "小岚", "height": "168cm",
				"blood_type": "A", "birthplace": "杭州",
			},
			"personality":       map[string]any{},
			"behavioral_policy": map[string]any{},
			"life_profile": map[string]any{
				"appearance": map[string]any{
					"description":              "深色长发，日常偏爱宽松针织衫",
					"daily_outfit_preferences": []any{"宽松针织衫", "长裙"},
				},
				"media_preferences": map[string]any{
					"channels":  []any{"moment", "selfie", "artwork"},
					"frequency": "frequent",
				},
			},
			"personality_system": map[string]any{
				"mode": "multiple",
				"profiles": []any{
					map[string]any{"id": "quiet", "name": "静海"},
					map[string]any{"id": "bright", "name": "流火"},
				},
				"active_profile_id": "default",
				"core_relationship": "两个人格共享记忆并相互保护",
				"core_conflict":     "安全感与主动表达之间长期冲突",
			},
		},
	})
	persona := mapValue(value["core_persona"])
	identity := mapValue(persona["identity"])
	lifeProfile := mapValue(persona["life_profile"])
	system := mapValue(persona["personality_system"])
	if stringValue(identity["nickname"]) != "小岚" ||
		stringValue(identity["height"]) != "168cm" ||
		stringValue(identity["blood_type"]) != "A" ||
		stringValue(identity["birthplace"]) != "杭州" ||
		len(mapValue(lifeProfile["media_preferences"])) == 0 ||
		stringValue(system["core_relationship"]) == "" ||
		stringValue(system["core_conflict"]) == "" {
		t.Fatalf("dense character-card semantics left canonical runtime owners: %#v", value)
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

func TestInitializationSchemaAllowsExtendedFieldsAndStringTypes(t *testing.T) {
	schema := initializationResponseSchema()
	candidate := map[string]any{
		"schema_version": 2,
		"core_persona": map[string]any{
			"schema_version": 1,
			"identity": map[string]any{
				"name": "摇光", "age": "24岁", "gender": "female",
				"mbti": "INFJ",
			},
			"personality": map[string]any{
				"openness":         0.8,
				"attachment_style": "secure",
			},
			"behavioral_policy": map[string]any{
				"response_style": "warm",
			},
			"life_profile": map[string]any{
				"appearance": map[string]any{"description": "清秀"},
			},
			"personality_system": map[string]any{
				"mode":              "single",
				"active_profile_id": "default",
				"profiles": []any{
					map[string]any{"id": "default", "name": "默认"},
				},
			},
		},
		"developing_self": map[string]any{
			"claims": []any{},
		},
		"initial_goals":      []any{},
		"initial_intentions": []any{},
		"initial_relationships": []any{
			map[string]any{
				"target_actor_id": "actor_user",
				"role":            "朋友",
				"trend":           "良好",
			},
		},
	}
	if err := capabilitycontract.ValidateCapabilitySchemaValue(candidate, schema); err != nil {
		t.Fatalf("ValidateCapabilitySchemaValue rejected candidate with relaxed fields: %v", err)
	}
	prepared, err := prepareInitializationResponse(candidate)
	if err != nil {
		t.Fatalf("prepareInitializationResponse failed: %v", err)
	}
	if prepared == nil {
		t.Fatal("expected prepared foundation to be non-nil")
	}
}

func TestInitializationDefensiveNormalizationHandlesUnusualModelOutputs(t *testing.T) {
	candidate := map[string]any{
		"schema_version": 2,
		"core_persona": map[string]any{
			"schema_version": 1,
			"name":           "stray_name",
			"description":    "stray_description",
			"identity": map[string]any{
				"name":     "测试角色",
				"timezone": "Asia/Beijing",
			},
			"personality": map[string]any{},
			"behavioral_policy": map[string]any{},
			"life_profile": map[string]any{},
			"personality_system": map[string]any{
				"mode": "multiple",
				"profiles": []any{
					map[string]any{"name": "主日常人格"},
					map[string]any{"name": "夜间人格"},
				},
				"takeover_rules": []any{
					map[string]any{"condition": "夜深时触发", "target_profile_id": "profile_2"},
				},
			},
		},
		"developing_self": map[string]any{
			"claims": []any{
				map[string]any{
					"category":      "interest",
					"claim":         "喜欢摄影",
					"confidence":    0.8,
					"extra_tag":     "photography",
					"evidence_refs": []any{"对话记录", ""},
				},
			},
		},
		"initial_relationships": []any{
			map[string]any{
				"role":  "好友",
				"trend": "良好",
			},
		},
		"initial_goals": []any{
			map[string]any{"description": "完成一次画展"},
		},
		"initial_intentions": []any{
			map[string]any{"action": "挑选画作", "goal_index": 0},
		},
		"extensions": map[string]any{},
	}

	prepared, err := prepareInitializationResponse(candidate)
	if err != nil {
		t.Fatalf("prepareInitializationResponse failed on unusual model outputs: %v", err)
	}
	if prepared == nil {
		t.Fatal("expected prepared foundation to be non-nil")
	}
	// Verify timezone was normalized
	persona := mapValue(prepared["core_persona"])
	identity := mapValue(persona["identity"])
	if stringValue(identity["timezone"]) != "Asia/Shanghai" {
		t.Fatalf("expected Asia/Shanghai timezone, got %v", identity["timezone"])
	}
	// Verify stray keys on persona were moved to extensions
	extensions := mapValue(prepared["extensions"])
	if extensions["core_persona.name"] != "stray_name" {
		t.Fatalf("expected core_persona.name in extensions, got %v", extensions)
	}
	// Verify profile IDs were generated
	system := mapValue(persona["personality_system"])
	profiles := arrayValue(system["profiles"])
	if len(profiles) != 2 || stringValue(mapValue(profiles[0])["id"]) == "" {
		t.Fatalf("expected 2 profiles with non-empty IDs, got %v", profiles)
	}
	// Verify takeover rules were normalized
	takeoverRules := arrayValue(system["takeover_rules"])
	if len(takeoverRules) != 1 {
		t.Fatalf("expected 1 takeover rule, got %v", takeoverRules)
	}
	rule := mapValue(takeoverRules[0])
	if stringValue(rule["kind"]) != "turn_takeover" || stringValue(rule["version"]) != "turn-takeover.v1" {
		t.Fatalf("expected turn_takeover kind and version, got %v", rule)
	}
	// Verify relationship target_actor_id was filled
	rels := arrayValue(prepared["initial_relationships"])
	if len(rels) != 1 || stringValue(mapValue(rels[0])["target_actor_id"]) == "" {
		t.Fatalf("expected relationship target_actor_id to be populated, got %v", rels)
	}
}

