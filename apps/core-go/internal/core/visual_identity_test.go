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
