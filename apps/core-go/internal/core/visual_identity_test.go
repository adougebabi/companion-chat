package core

import (
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

func TestEnforceVisualIdentityPromptRequiresThreePanelLayout(t *testing.T) {
	prompt := enforceVisualIdentityTurnaroundPrompt("character description", "seed")
	expected := "Character design sheet, three separate panels on a white background. Left: front close-up portrait of character description. Center: front full body standing straight. Right: back full body from behind. Symmetrical pose, no side view, high resolution concept art."
	if prompt != expected {
		t.Fatalf("three-panel prompt = %q, want %q", prompt, expected)
	}
	conceptPrompt := visualIdentityPromptFromConcept(map[string]any{
		"purpose":         "visual_identity",
		"visual_identity": map[string]any{"identity_snapshot": map[string]any{"identity": map[string]any{"visible_text": "一位20岁女性"}}},
	})
	if !strings.Contains(conceptPrompt, "一位20岁女性") || !strings.Contains(conceptPrompt, "no side view") || strings.Contains(conceptPrompt, "front view, side view") {
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
