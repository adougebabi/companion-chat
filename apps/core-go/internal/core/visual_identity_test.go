package core

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizeChestCupAndAdapter(t *testing.T) {
	for _, test := range []struct {
		input  string
		cup    string
		weight float64
	}{
		{input: " a ", cup: "A", weight: -5},
		{input: "B", cup: "B", weight: -3},
		{input: "c", cup: "C", weight: -1},
		{input: "D", cup: "D", weight: 1},
	} {
		cup, err := NormalizeChestCup(test.input)
		if err != nil || cup != test.cup {
			t.Fatalf("NormalizeChestCup(%q) = %q, %v", test.input, cup, err)
		}
		weight, version, err := chestCupToLoRAWeight(test.input)
		if err != nil || weight != test.weight || version != visualIdentityAdapterVersion {
			t.Fatalf("chestCupToLoRAWeight(%q) = %v, %q, %v", test.input, weight, version, err)
		}
	}
	for _, input := range []string{"", "AA", "E", "unknown"} {
		if _, _, err := chestCupToLoRAWeight(input); err == nil {
			t.Fatalf("chestCupToLoRAWeight(%q) accepted unsupported value", input)
		}
	}
}

func TestChestRendererConstraintsPreserveSemanticAndResolvedValues(t *testing.T) {
	constraints, err := chestRendererConstraints(map[string]any{"appearance": map[string]any{"chest_cup": "B"}})
	if err != nil {
		t.Fatal(err)
	}
	if constraints["chest_cup"] != "B" || constraints["chest_lora_weight"] != float64(-3) {
		t.Fatalf("constraints = %#v", constraints)
	}
	if _, err := chestRendererConstraints(map[string]any{"appearance": map[string]any{"chest_cup": "E"}}); err == nil {
		t.Fatal("unsupported cup should fail")
	}
}

func TestRendererConstraintsReadIdentityAppearanceAndHandleMale(t *testing.T) {
	constraints, err := rendererConstraintsForCorePersona(map[string]any{
		"identity":     map[string]any{"gender": "female", "appearance": map[string]any{"bust": "B cup"}},
		"life_profile": map[string]any{"appearance": map[string]any{}},
	})
	if err != nil || constraints["chest_cup"] != "B" || constraints["chest_lora_weight"] != float64(-3) {
		t.Fatalf("identity appearance constraints = %#v, %v", constraints, err)
	}
	male, err := rendererConstraintsForCorePersona(map[string]any{
		"identity":     map[string]any{"gender": "男"},
		"life_profile": map[string]any{"appearance": map[string]any{"chest_cup": "B"}},
	})
	if err != nil || male["chest_cup"] != "not_applicable" || male["chest_lora_weight"] != float64(0) || male["chest_lora_applicable"] != false {
		t.Fatalf("male constraints = %#v, %v", male, err)
	}
	bodyType, err := rendererConstraintsForCorePersona(map[string]any{
		"identity":     map[string]any{"gender": "female", "body_type": "A cup"},
		"life_profile": map[string]any{"appearance": map[string]any{}},
	})
	if err != nil || bodyType["chest_cup"] != "A" || bodyType["chest_lora_weight"] != float64(-5) {
		t.Fatalf("body_type constraints = %#v, %v", bodyType, err)
	}
	chest, err := rendererConstraintsForCorePersona(map[string]any{
		"identity":     map[string]any{"gender": "female", "chest": "A cup"},
		"life_profile": map[string]any{"appearance": map[string]any{}},
	})
	if err != nil || chest["chest_cup"] != "A" || chest["chest_lora_weight"] != float64(-5) {
		t.Fatalf("chest constraints = %#v, %v", chest, err)
	}
}

func TestNormalizeVisualIdentityFoundationUsesCanonicalAppearancePath(t *testing.T) {
	persona := map[string]any{
		"identity": map[string]any{
			"gender":     "女性",
			"appearance": map[string]any{"chest": "A cup"},
		},
		"life_profile": map[string]any{
			"physical_traits": map[string]any{"bust_size": "B cup"},
		},
	}
	normalizeVisualIdentityFoundation(persona)
	appearance := mapValue(mapValue(persona["life_profile"])["appearance"])
	if appearance["chest_cup"] != "B" {
		t.Fatalf("canonical chest_cup = %#v", appearance["chest_cup"])
	}
	constraints, err := rendererConstraintsForCorePersona(persona)
	if err != nil || constraints["chest_cup"] != "B" || constraints["chest_lora_weight"] != float64(-3) {
		t.Fatalf("canonical renderer constraints = %#v, %v", constraints, err)
	}
}

func TestNormalizeVisualIdentityFoundationNormalizesCanonicalDecoratedCup(t *testing.T) {
	persona := map[string]any{
		"identity":     map[string]any{"gender": "female"},
		"life_profile": map[string]any{"appearance": map[string]any{"chest_cup": "A cup"}},
	}
	normalizeVisualIdentityFoundation(persona)
	if got := mapValue(mapValue(persona["life_profile"])["appearance"])["chest_cup"]; got != "A" {
		t.Fatalf("normalized canonical chest_cup = %#v", got)
	}
}

func TestRendererConstraintsPreferCanonicalAppearanceOverLegacyAliases(t *testing.T) {
	constraints, err := rendererConstraintsForCorePersona(map[string]any{
		"identity": map[string]any{
			"gender":     "female",
			"appearance": map[string]any{"chest": "A cup"},
		},
		"life_profile": map[string]any{
			"appearance":      map[string]any{"chest_cup": "B"},
			"physical_traits": map[string]any{"bust_size": "C cup"},
		},
	})
	if err != nil || constraints["chest_cup"] != "B" || constraints["chest_lora_weight"] != float64(-3) {
		t.Fatalf("canonical precedence constraints = %#v, %v", constraints, err)
	}
}

func TestInitializationSchemaDeclaresCanonicalChestCupPath(t *testing.T) {
	schema := initializationResponseSchema()
	corePersona := mapValue(mapValue(schema["properties"])["core_persona"])
	lifeProfile := mapValue(mapValue(corePersona["properties"])["life_profile"])
	appearance := mapValue(mapValue(lifeProfile["properties"])["appearance"])
	chestCup := mapValue(mapValue(appearance["properties"])["chest_cup"])
	if chestCup["type"] != "string" || len(arrayValue(chestCup["enum"])) != 4 {
		t.Fatalf("canonical chest_cup schema = %#v", chestCup)
	}
}

func TestInitializationSchemaAllowsSparsePersonalityProfile(t *testing.T) {
	schema := initializationResponseSchema()
	rootRequired := arrayValue(schema["required"])
	if len(rootRequired) != 1 || stringValue(rootRequired[0]) != "core_persona" {
		t.Fatalf("initialization root should require only core_persona: %#v", rootRequired)
	}
	corePersona := mapValue(mapValue(schema["properties"])["core_persona"])
	system := mapValue(mapValue(corePersona["properties"])["personality_system"])
	profiles := mapValue(mapValue(system["properties"])["profiles"])
	profile := mapValue(profiles["items"])
	required := arrayValue(profile["required"])
	if len(required) != 1 || stringValue(required[0]) != "id" {
		t.Fatalf("personality profile should require only stable id: %#v", required)
	}
	if profile["additionalProperties"] != false {
		t.Fatalf("personality profile must reserve unknown fields under extensions: %#v", profile)
	}
}

func TestVisualIdentityStageOrder(t *testing.T) {
	if visualIdentityStageOrder(visualIdentityStageSeedReady) >= visualIdentityStageOrder(visualIdentityStageImageRequested) {
		t.Fatal("seed_ready must sort before image_requested")
	}
	if visualIdentityStageOrder(visualIdentityStageVisionReady) >= visualIdentityStageOrder(visualIdentityStagePatchRequested) {
		t.Fatal("vision_ready must sort before patch_requested")
	}
}

func TestVisualIdentityJSONEmptyTreatsDatabaseDefaultAsEmpty(t *testing.T) {
	for _, raw := range []string{"", "{}", " {} ", "null"} {
		if !visualIdentityJSONEmpty([]byte(raw)) {
			t.Fatalf("visualIdentityJSONEmpty(%q) = false", raw)
		}
	}
	if visualIdentityJSONEmpty([]byte(`{"summary":"ok"}`)) {
		t.Fatal("non-empty vision result was treated as empty")
	}
}

func TestVisualIdentityProviderPendingRecognizesCurrentGenericBinding(t *testing.T) {
	for _, message := range []string{
		"provider role generic_llm unavailable: no rows in result set",
		"provider role visual_identity_vision unavailable: no rows in result set",
	} {
		if !visualIdentityProviderPending(errors.New(message)) {
			t.Fatalf("provider pending error was not recognized: %s", message)
		}
	}
	if visualIdentityProviderPending(errors.New("provider request failed: timeout")) {
		t.Fatal("runtime Provider failure was incorrectly downgraded to configuration pending")
	}
}

func TestVisualIdentityAgentCheckpointIdentityIsStableAndAdvancesWithDurableState(t *testing.T) {
	generate := visualIdentityAgentState{SessionID: "session-1", Attempt: 1, ActionRequired: visualIdentityGenerateCandidateCapabilityName}
	if first, replay := visualIdentityAgentCheckpointOperationID(generate), visualIdentityAgentCheckpointOperationID(generate); first == "" || first != replay {
		t.Fatalf("same durable checkpoint is not stable: first=%q replay=%q", first, replay)
	}
	review := generate
	review.ActionRequired = visualIdentityCommitReviewCapabilityName
	if visualIdentityAgentCheckpointOperationID(review) == visualIdentityAgentCheckpointOperationID(generate) {
		t.Fatal("candidate-ready review checkpoint reused the generation run identity")
	}
	nextAttempt := generate
	nextAttempt.Attempt = 2
	if visualIdentityAgentCheckpointOperationID(nextAttempt) == visualIdentityAgentCheckpointOperationID(generate) {
		t.Fatal("regenerated attempt reused the prior attempt run identity")
	}
	finalize := review
	finalize.ActionRequired = visualIdentityFinalizeCapabilityName
	if visualIdentityAgentCheckpointOperationID(finalize) == visualIdentityAgentCheckpointOperationID(review) {
		t.Fatal("character-sheet-ready checkpoint reused the review run identity")
	}
}

func TestEnforceVisualIdentityPromptRequiresThreePanelLayout(t *testing.T) {
	prompt := enforceVisualIdentityTurnaroundPrompt("character description", "seed")
	for _, required := range []string{"角色设定卡", "character description", "正面/侧面/背面三视图", "六种不同表情", "禁止生成任何文字", "非动漫、非Q版"} {
		if !strings.Contains(prompt, required) {
			t.Fatalf("three-panel prompt missing %q: %q", required, prompt)
		}
	}
	conceptPrompt := visualIdentityPromptFromConcept(map[string]any{
		"purpose":         "visual_identity",
		"visual_identity": map[string]any{"identity_snapshot": map[string]any{"identity": map[string]any{"visible_text": "一位20岁女性"}}},
	})
	if !strings.Contains(conceptPrompt, "一位20岁女性") || !strings.Contains(conceptPrompt, "正面/侧面/背面三视图") || strings.Contains(conceptPrompt, "校服") || strings.Contains(conceptPrompt, "BASIC INFORMATION｜") || !strings.Contains(conceptPrompt, "禁止生成任何文字") {
		t.Fatalf("concept prompt = %q", conceptPrompt)
	}
	if got := enforceVisualIdentityTurnaroundPrompt("front view", "review"); got != "front view" {
		t.Fatalf("review prompt should not receive seed layout constraint: %s", got)
	}
}

func TestVisualIdentitySchemasExposeDecisionAndVisionStages(t *testing.T) {
	vision := visualIdentityVisionResponseSchema()
	if len(arrayValue(vision["required"])) != 4 {
		t.Fatalf("vision schema required = %#v", vision["required"])
	}
	patch := visualIdentityPatchResponseSchema()
	properties := mapValue(patch["properties"])
	for _, key := range []string{"decision", "seed_prompt", "prompt_patch", "renderer_constraints"} {
		if _, ok := properties[key]; !ok {
			t.Fatalf("patch schema missing %q", key)
		}
	}
}

func TestDefaultCapabilityRegistryIncludesVisualIdentityInitializer(t *testing.T) {
	registry := (&App{}).capabilityRegistry()
	if _, ok := registry.Lookup("visual_identity.initialize"); !ok {
		t.Fatal("default capability registry does not expose visual_identity.initialize")
	}
}

func TestVisualIdentityProductionEntryUsesCompleteFormalAgent(t *testing.T) {
	source := readSourceFile(t, "visual_identity.go")
	body := sourceBetween(t, string(source), "func (a *App) ProcessVisualIdentity", "func visualIdentityJSONEmpty")
	if !strings.Contains(body, "RunVisualIdentityAgent") || !strings.Contains(body, "VisualIdentityAgentInput{SessionID: sessionID}") {
		t.Fatalf("production Visual Identity entry does not use the complete formal Agent: %s", body)
	}
	for _, forbidden := range []string{"RunVisualIdentityVisionTask", "RunVisualIdentityPatchTask", "promoteVisualIdentityCanonical("} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("production Visual Identity entry still chooses model/persistence stage %q", forbidden)
		}
	}
	creation := sourceBetween(t, string(source), "func (a *App) ensureVisualIdentityInitializationTx", "func appendVisualIdentityTimelineTx")
	if !strings.Contains(creation, `"correlation_id": "visual_identity:" + sessionID`) {
		t.Fatal("Visual Identity intent does not carry its stable lifecycle correlation")
	}
}
