package core

import (
	"strings"
	"testing"
)

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

func TestReplaceMediaPlaceholdersReplacesSeedPlaceholder(t *testing.T) {
	// 1. Explicit seed passed via constraints
	workflow := map[string]any{
		"prompt": "{{prompt}}",
		"sampler": map[string]any{
			"seed": "{{seed}}",
			"info": "seed={{seed}}",
		},
	}
	explicitSeed := int64(123456789)
	replaced, err := replaceMediaPlaceholders(workflow, "a portrait of {{seed}}", map[string]any{"seed": explicitSeed})
	if err != nil {
		t.Fatal(err)
	}
	if replaced["prompt"] != "a portrait of 123456789" {
		t.Fatalf("prompt seed replacement failed: %v", replaced["prompt"])
	}
	sampler := mapValue(replaced["sampler"])
	if sampler["seed"] != explicitSeed {
		t.Fatalf("standalone seed was %v (type %T), expected %v (int64)", sampler["seed"], sampler["seed"], explicitSeed)
	}
	if sampler["info"] != "seed=123456789" {
		t.Fatalf("embedded seed was %v, expected seed=123456789", sampler["info"])
	}

	// 2. Random seed generated when not specified
	workflowRandom := map[string]any{
		"sampler1": map[string]any{"seed": "{{seed}}"},
		"sampler2": map[string]any{"seed": "{{seed}}"},
	}
	run1, err := replaceMediaPlaceholders(workflowRandom, "cat", nil)
	if err != nil {
		t.Fatal(err)
	}
	run2, err := replaceMediaPlaceholders(workflowRandom, "cat", nil)
	if err != nil {
		t.Fatal(err)
	}
	seed1_1 := mapValue(run1["sampler1"])["seed"].(int64)
	seed1_2 := mapValue(run1["sampler2"])["seed"].(int64)
	seed2_1 := mapValue(run2["sampler1"])["seed"].(int64)
	if seed1_1 <= 0 || seed1_1 > 9007199254740991 {
		t.Fatalf("seed1_1 out of safe range: %d", seed1_1)
	}
	if seed1_1 != seed1_2 {
		t.Fatalf("expected samplers in the same workflow to share seed, got %d and %d", seed1_1, seed1_2)
	}
	if seed1_1 == seed2_1 {
		t.Fatalf("expected different random seeds across runs, got both %d", seed1_1)
	}
}

func TestMediaRendererConstraintsUsesCognitionContextBinding(t *testing.T) {
	constraints := mediaRendererConstraints(map[string]any{
		"context_binding": map[string]any{
			"visual_identity": map[string]any{
				"renderer_constraints": map[string]any{"chest_cup": "B", "chest_lora_weight": -3.0},
			},
		},
	})
	if constraints["chest_cup"] != "B" || constraints["chest_lora_weight"] != -3.0 {
		t.Fatalf("nested renderer constraints = %#v", constraints)
	}
}

func TestMediaRendererConstraintsRootValuesWinOverContextBinding(t *testing.T) {
	constraints := mediaRendererConstraints(map[string]any{
		"renderer_constraints": map[string]any{"chest_lora_weight": -5.0},
		"context_binding": map[string]any{
			"visual_identity": map[string]any{
				"renderer_constraints": map[string]any{"chest_lora_weight": -3.0, "chest_cup": "B"},
			},
		},
	})
	if constraints["chest_lora_weight"] != -5.0 || constraints["chest_cup"] != "B" {
		t.Fatalf("merged renderer constraints = %#v", constraints)
	}
}

func TestVisualIdentityReferenceImagePlaceholderUsesUploadedFilename(t *testing.T) {
	workflow := map[string]any{
		"load_image": map[string]any{"inputs": map[string]any{"image": "{{visual_identity_reference_image}}"}},
	}
	replaced, err := replaceMediaPlaceholdersWithReference(workflow, "portrait", nil, "visual_identity_reference_asset.png")
	if err != nil {
		t.Fatal(err)
	}
	inputs := mapValue(mapValue(replaced["load_image"])["inputs"])
	if inputs["image"] != "visual_identity_reference_asset.png" {
		t.Fatalf("reference image placeholder = %#v", inputs["image"])
	}
	if _, err := replaceMediaPlaceholders(workflow, "portrait", nil); err == nil {
		t.Fatal("missing reference filename should fail")
	}
}

func TestVisualIdentityReferenceAssetPrefersCharacterSheet(t *testing.T) {
	concept := map[string]any{
		"context_binding": map[string]any{
			"visual_identity": map[string]any{
				"character_sheet_asset_id": "asset_character_sheet",
				"canonical_asset_id":       "asset_canonical",
			},
		},
	}
	if got := visualIdentityReferenceAssetID(concept); got != "asset_character_sheet" {
		t.Fatalf("reference asset = %q", got)
	}
}

func TestMergeVisualIdentityRendererConstraintsUpdatesOnlyExplicitConcepts(t *testing.T) {
	prompt, err := mergeVisualIdentityRendererConstraints(`{"purpose":"visual_identity","stage":"seed","renderer_constraints":{"schema_version":"old"}}`, map[string]any{"chest_cup": "A", "chest_lora_weight": -5.0})
	if err != nil || !strings.Contains(prompt, `"chest_cup":"A"`) || !strings.Contains(prompt, `"chest_lora_weight":-5`) {
		t.Fatalf("merged prompt = %q, %v", prompt, err)
	}
	unchanged, err := mergeVisualIdentityRendererConstraints(`{"purpose":"scene","prompt":"x"}`, map[string]any{"chest_lora_weight": -5.0})
	if err != nil || unchanged != `{"purpose":"scene","prompt":"x"}` {
		t.Fatalf("non visual prompt changed: %q, %v", unchanged, err)
	}
}

func TestVisualIdentityMediaPromptInstructionRequiresThreePanelCharacterSheet(t *testing.T) {
	messages := addVisualIdentityMediaPromptInstruction("media_prompt", []map[string]any{
		{"role": "system", "content": "generic"},
		{"role": "user", "content": `{"purpose":"visual_identity","stage":"seed","render_intent":"character_design_sheet"}`},
	})
	if len(messages) != 2 || !strings.Contains(stringValue(messages[0]["content"]), "CHARACTER PROFILE") || !strings.Contains(stringValue(messages[0]["content"]), "Do not render any text") || !strings.Contains(stringValue(messages[0]["content"]), "consistent real human face") {
		t.Fatalf("visual identity prompt instruction = %#v", messages)
	}
}

func TestVisualIdentityValidationInstructionsDoNotRequireRenderedText(t *testing.T) {
	for name, instruction := range map[string]string{
		"vision": visualIdentityVisionTaskInstruction,
		"patch":  visualIdentityPatchTaskInstruction,
	} {
		t.Run(name, func(t *testing.T) {
			for _, section := range visualIdentityRequiredCardSections {
				if !strings.Contains(instruction, section) {
					t.Errorf("validation instruction missing visual section %q", section)
				}
			}
			for _, requirement := range []string{"text-free", "Do not require", "incidental unreadable marks"} {
				if !strings.Contains(instruction, requirement) {
					t.Errorf("validation instruction missing text-free rule %q", requirement)
				}
			}
		})
	}
}

func TestMediaComfyPromptSubmissionDiagnosticIncludesFinalRequestPayload(t *testing.T) {
	intent := mediaIntent{ID: "media-1", ProviderRequestID: "request-1", WorkflowID: "workflow-1"}
	payload := mediaComfyPromptSubmissionDiagnostic(intent, map[string]any{"stage": "seed"}, "portrait", map[string]any{
		"3": map[string]any{"inputs": map[string]any{"text": "portrait", "strength_model": -3.0}},
	})
	if payload["provider_prompt"] != "portrait" || payload["prompt"] != "portrait" {
		t.Fatalf("provider prompt fields = %#v", payload)
	}
	requestPayload := mapValue(payload["request_payload"])
	if mapValue(requestPayload["prompt"]) == nil || mapValue(mapValue(requestPayload["prompt"])["3"])["inputs"] == nil {
		t.Fatalf("final ComfyUI request payload = %#v", payload["request_payload"])
	}
}

func TestCleanGeneratedMediaPrompt(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "already clean",
			input:    "一位年轻东方女性，微卷长发，在阳光明媚的窗边拍摄写真。",
			expected: "一位年轻东方女性，微卷长发，在阳光明媚的窗边拍摄写真。",
		},
		{
			name:     "markdown fences",
			input:    "```markdown\n一位年轻东方女性，微卷长发，在阳光明媚的窗边拍摄写真。\n```",
			expected: "一位年轻东方女性，微卷长发，在阳光明媚的窗边拍摄写真。",
		},
		{
			name:     "boilerplate preamble with intent and context_binding",
			input:    "这是一条基于你提供的 `intent` 和 `context_binding` 优化后的完整图像生成提示词：\n\n一位年轻东方女性，微卷长发，在阳光明媚的窗边拍摄写真。",
			expected: "一位年轻东方女性，微卷长发，在阳光明媚的窗边拍摄写真。",
		},
		{
			name:     "boilerplate preamble with 好的以下是",
			input:    "好的，以下是为您生成的写真提示词：\n一位年轻东方女性，微卷长发，在阳光明媚的窗边拍摄写真。",
			expected: "一位年轻东方女性，微卷长发，在阳光明媚的窗边拍摄写真。",
		},
		{
			name:     "prompt label prefix",
			input:    "提示词：一位年轻东方女性，微卷长发，在阳光明媚的窗边拍摄写真。",
			expected: "一位年轻东方女性，微卷长发，在阳光明媚的窗边拍摄写真。",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanGeneratedMediaPrompt(tt.input)
			if got != tt.expected {
				t.Fatalf("cleanGeneratedMediaPrompt() = %q, want %q", got, tt.expected)
			}
		})
	}
}
