package core

import "testing"

func TestParseRange(t *testing.T) {
	tests := []struct {
		name, value string
		s, e        int64
		partial     bool
		wantErr     bool
	}{
		{"full", "", 0, 99, false, false},
		{"bounded", "bytes=10-19", 10, 19, true, false},
		{"open", "bytes=90-", 90, 99, true, false},
		{"suffix", "bytes=-10", 90, 99, true, false},
		{"invalid", "bytes=100-101", 0, 0, false, true},
		{"multi", "bytes=1-2,4-5", 0, 0, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, e, partial, err := parseRange(tt.value, 100)
			if (err != nil) != tt.wantErr || s != tt.s || e != tt.e || partial != tt.partial {
				t.Fatalf("got (%d,%d,%v,%v), want (%d,%d,%v,%v)", s, e, partial, err, tt.s, tt.e, tt.partial, tt.wantErr)
			}
		})
	}
}

func TestStableFluctlightID(t *testing.T) {
	if StableFluctlightID("owner", "request") != StableFluctlightID("owner", "request") {
		t.Fatal("stable activation ID changed")
	}
	if StableFluctlightID("owner", "request") == StableFluctlightID("other", "request") {
		t.Fatal("different owners collided")
	}
}

func TestResolveDecisionActionAcceptsStructuredStringConcept(t *testing.T) {
	action, concept := resolveDecisionAction(map[string]any{
		"action_type":    "media_request",
		"visual_concept": "雨后的上海街角，北向窗与老相机",
	})
	if action != "media_request" {
		t.Fatalf("action = %q", action)
	}
	if concept["visual_concept"] != "雨后的上海街角，北向窗与老相机" {
		t.Fatalf("concept was not preserved: %#v", concept)
	}
}

func TestResolveDecisionActionDoesNotInventMissingConcept(t *testing.T) {
	action, concept := resolveDecisionAction(map[string]any{"action_type": "media_request"})
	if action != "media_request" {
		t.Fatalf("action = %q", action)
	}
	if len(concept) != 0 {
		t.Fatalf("unexpected invented concept: %#v", concept)
	}
}

func TestAppendUniqueAssetRefIsIdempotent(t *testing.T) {
	refs := []any{"asset_existing"}
	refs = appendUniqueAssetRef(refs, "asset_new")
	refs = appendUniqueAssetRef(refs, "asset_new")
	if len(refs) != 2 || stringValue(refs[1]) != "asset_new" {
		t.Fatalf("refs = %#v", refs)
	}
}

func TestSelectComfyWorkflowUsesExplicitVisualIdentityStage(t *testing.T) {
	fallback := map[string]any{"node": map[string]any{"text": "{{prompt}}"}}
	seed := map[string]any{"seed_node": map[string]any{"text": "{{prompt}}"}}
	config := map[string]any{"visual_identity_workflow": map[string]any{"seed": seed}}
	if got := selectComfyWorkflow(config, `{"purpose":"visual_identity","stage":"seed"}`, fallback); len(got) != 1 || mapValue(got["seed_node"])["text"] != "{{prompt}}" {
		t.Fatalf("seed workflow = %#v", got)
	}
	if got := selectComfyWorkflow(config, `{"purpose":"scene"}`, fallback); len(got) != 1 || got["node"] == nil {
		t.Fatalf("legacy workflow fallback = %#v", got)
	}
}

func TestReplaceMediaPlaceholdersKeepsPromptAndNumericLoRAType(t *testing.T) {
	workflow := map[string]any{"prompt": "{{prompt}}", "strength_model": "{{chest_lora_weight}}", "description": "weight={{chest_lora_weight}}"}
	replaced, err := replaceMediaPlaceholders(workflow, "portrait", map[string]any{"chest_lora_weight": -3.0})
	if err != nil {
		t.Fatal(err)
	}
	if replaced["prompt"] != "portrait" || replaced["strength_model"] != -3.0 || replaced["description"] != "weight=-3" {
		t.Fatalf("replaced workflow = %#v", replaced)
	}
	if _, err := replaceMediaPlaceholders(workflow, "portrait", nil); err == nil {
		t.Fatal("missing chest LoRA weight should fail instead of silently defaulting")
	}
}
