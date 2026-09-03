package core

import "testing"

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
