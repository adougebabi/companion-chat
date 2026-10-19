package core

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func captureStyleForTest() map[string]any {
	return map[string]any{"framing": "upper_body", "pose": "seated", "expression": "smiling", "lighting": "soft", "style": "photographic"}
}
func TestCurrentCaptureRejectsClothingFreeTextAndKeepsActualWearing(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	appearance, _, _, err := f.app.readEffectiveLifeSnapshot(f.ctx, f.fluctlightID, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	concept := map[string]any{"intent": "穿刚买的黑色短靴拍现在的照片", "context_binding": map[string]any{"appearance": appearance, "current_life": map[string]any{"scene": "书房"}}}
	plan := captureStyleForTest()
	prompt, err := renderCurrentCapturePrompt(concept, plan)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(prompt, "白衬衫") || strings.Contains(prompt, "短靴") {
		t.Fatalf("wish replaced actual wearing: %s", prompt)
	}
	malicious := cloneMap(plan)
	malicious["clothing"] = "黑色短靴"
	if _, err := renderCurrentCapturePrompt(concept, malicious); err == nil {
		t.Fatal("free clothing passed closed final renderer plan")
	}
	malicious = cloneMap(plan)
	malicious["pose"] = "seated wearing black boots"
	if _, err := renderCurrentCapturePrompt(concept, malicious); err == nil {
		t.Fatal("pose bypassed clothing authority")
	}
	visual := map[string]any{"identity_snapshot": map[string]any{"source": "effective_life", "body_revision": appearance["body_revision"], "wardrobe_revision": appearance["wardrobe_revision"]}}
	mapValue(concept["context_binding"])["visual_identity"] = visual
	if !currentCaptureReferenceCompatible(concept) {
		t.Fatal("sourced matching reference rejected")
	}
	mapValue(visual["identity_snapshot"])["wardrobe_revision"] = 999
	if currentCaptureReferenceCompatible(concept) {
		t.Fatal("reference clothes from another revision accepted")
	}
	mapValue(appearance)["wearing_state"] = "unknown"
	if _, err := renderCurrentCapturePrompt(concept, plan); err == nil {
		t.Fatal("missing outfit silently became nude/default outfit")
	}
}
func TestCurrentCapturePlanCannotUseAnUnheldObject(t *testing.T) {
	concept := map[string]any{"context_binding": map[string]any{"appearance": map[string]any{"body_revision": 0, "wardrobe_revision": 0, "wearing_state": "known", "worn_items": []any{}, "used_items": []any{}}}}
	plan := captureStyleForTest()
	plan["pose"] = "holding_used_item"
	if _, err := renderCurrentCapturePrompt(concept, plan); err == nil {
		t.Fatal("imaginary prop accepted")
	}
}

func TestCurrentCapturePreservesCameraAndRejectsFinalTemplateOverrides(t *testing.T) {
	concept := map[string]any{"capture": map[string]any{"mode": "mirror_selfie", "framing": "upper body", "camera": "rear", "device_visibility": "visible"}, "context_binding": map[string]any{"appearance": map[string]any{"body_revision": 0, "wardrobe_revision": 0, "wearing_state": "known", "worn_items": []any{}}}}
	plan := captureStyleForTest()
	prompt, err := renderCurrentCapturePrompt(concept, plan)
	if err != nil || !strings.Contains(prompt, "through a mirror") || !strings.Contains(prompt, "rear camera") {
		t.Fatalf("capture lost %s %v", prompt, err)
	}
	plan["framing"] = "full_body"
	if prompt, err := renderCurrentCapturePrompt(concept, plan); err != nil || !strings.Contains(prompt, "full-body composition") || !strings.Contains(prompt, "through a mirror") {
		t.Fatalf("standardized framing was compared literally to upstream hint: %s %v", prompt, err)
	}
	valid := map[string]any{"1": map[string]any{"class_type": "CLIPTextEncode", "inputs": map[string]any{"text": "{{prompt}}"}}, "2": map[string]any{"class_type": "KSampler", "inputs": map[string]any{"positive": []any{"1", 0}}}}
	if err := validateCurrentCaptureWorkflow(valid); err != nil {
		t.Fatal(err)
	}
	for _, workflow := range []map[string]any{
		{"prompt": "{{prompt}}, wearing imaginary boots"},
		{"prompt": "{{prompt}}", "1": map[string]any{"class_type": "CLIPTextEncode", "inputs": map[string]any{"text": "wearing imaginary boots"}}, "2": map[string]any{"class_type": "KSampler", "inputs": map[string]any{"positive": []any{"1", 0}}}},
		{"prompt": "{{prompt}}", "1": map[string]any{"class_type": "LoadImage", "inputs": map[string]any{"image": "unverified-outfit.png"}}},
	} {
		if err := validateCurrentCaptureWorkflow(workflow); err == nil {
			t.Fatal("final template override accepted", workflow)
		}
	}
}

func TestCurrentCaptureFinalMediaWorkerSubmitsOnlyFrozenFactsAndRejectsOverrides(t *testing.T) {
	for _, scenario := range []string{"valid-frozen", "enum-rejected", "explicit-selfie", "model-clothing", "workflow-clothing"} {
		t.Run(scenario, func(t *testing.T) {
			f := seedWardrobeToolFixture(t)
			seedCognitiveProviderRole(t, f.ctx, f.repository, "capture-worker-provider-"+f.suffix)
			// A controlled active identity is declared setup data, not an image result.
			if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.fluctlight_visual_identities(id,fluctlight_id,status,identity_snapshot) VALUES($1,$2,'active','{"identity":{"name":"Fixture identity"}}') ON CONFLICT(fluctlight_id) DO UPDATE SET status='active',identity_snapshot=EXCLUDED.identity_snapshot`, "capture-visual-"+f.suffix, f.fluctlightID); err != nil {
				t.Fatal(err)
			}
			workflow := map[string]any{"1": map[string]any{"class_type": "CLIPTextEncode", "inputs": map[string]any{"text": "{{prompt}}"}}, "2": map[string]any{"class_type": "KSampler", "inputs": map[string]any{"positive": []any{"1", 0}}}}
			if scenario == "workflow-clothing" {
				mapValue(mapValue(workflow["1"])["inputs"])["text"] = "{{prompt}} wearing nonexistent boots"
			}
			if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.runtime_settings(key,value_json) VALUES('media.comfyui',$1) ON CONFLICT(key) DO UPDATE SET value_json=EXCLUDED.value_json`, jsonString(map[string]any{"baseUrl": "http://capture-comfy.invalid", "workflow": workflow})); err != nil {
				t.Fatal(err)
			}
			plan := mediaTestPlan("person", "present", "yes", "full")
			plan["capture_method"], plan["viewpoint"], plan["framing"], plan["camera_angle"] = "external_photo", "observer", "half_body", "eye_level"
			if scenario == "explicit-selfie" {
				plan["capture_method"], plan["viewpoint"], plan["framing"] = "handheld_selfie", "subject_facing", "full_body"
			}
			submitted, modelCalls := 0, 0
			var final map[string]any
			f.app.Provider.HTTP = &http.Client{Transport: projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
				raw, err := io.ReadAll(request.Body)
				if err != nil {
					return nil, err
				}
				if request.URL.Host == "capture-comfy.invalid" {
					if request.URL.Path != "/prompt" {
						return nil, fmt.Errorf("unexpected renderer route %s", request.URL.Path)
					}
					submitted++
					final = decodeObject(raw)
					return embeddingHTTPResponse(request, http.StatusServiceUnavailable, `{}`), nil
				}
				modelCalls++
				var output map[string]any
				if strings.Contains(string(raw), "解析本次原始图像意图") {
					output = cloneMap(plan)
					if scenario == "enum-rejected" {
						output["framing"] = "unsupported_medium_long_shot"
					}
				} else if strings.Contains(string(raw), "你负责把本次媒体意图编译") {
					if !strings.Contains(string(raw), "白衬衫") {
						return nil, errors.New("normalizer did not receive actual frozen clothing")
					}
					output = map[string]any{"prompt": "人物穿当前实际白衬衫，佩戴银项链。", "declared_human_presence": "present", "self_in_frame": "yes", "capture_method": plan["capture_method"], "used_subject_ids": []any{"self"}, "used_reference_ids": []any{}, "declared_human_count": nil}
					if scenario == "explicit-selfie" {
						output["prompt"] = "人物穿当前实际白衬衫，手持手机全身自拍。"
					}
					if scenario == "model-clothing" {
						output["clothing"] = "nonexistent boots"
					}
				} else {
					return nil, fmt.Errorf("unexpected model task: %s", string(raw))
				}
				response := map[string]any{"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": jsonString(output)}}}}
				return embeddingHTTPResponse(request, http.StatusOK, jsonString(response)), nil
			})}
			before, _, _, err := f.app.readEffectiveLifeSnapshot(f.ctx, f.fluctlightID, f.app.now())
			if err != nil {
				t.Fatal(err)
			}
			args := map[string]any{"intent": "穿上刚买的黑色短靴拍现在的照片", "media_plan": plan}
			if scenario == "enum-rejected" {
				delete(args, "media_plan")
			}
			if scenario == "explicit-selfie" {
				args["intent"] = "拍现在的全身自拍"
				args["capture"] = map[string]any{"mode": "selfie", "framing": "full_body", "camera": "front"}
			}
			request := f.request("media.image.generate", "current-worker", args)
			request.TargetKind, request.TargetRef = "conversation", f.conversationID
			accepted, err := f.app.ExecuteTool(f.ctx, request)
			if scenario == "enum-rejected" {
				if err == nil || !strings.Contains(err.Error(), "media_subject_parse_invalid") || submitted != 0 || modelCalls != 1 {
					t.Fatal("invalid enum was silently accepted or retried", accepted, err, submitted, modelCalls)
				}
			} else {
				if err != nil {
					t.Fatal("image admission", err)
				}
				intentID := stringValue(mapValue(accepted.Result.Output)["media_intent_id"])
				if _, err = f.app.ProcessMediaIntent(f.ctx, intentID); err == nil {
					t.Fatal("renderer failure/override was reported successful")
				}
				if scenario == "valid-frozen" || scenario == "explicit-selfie" {
					wire := jsonString(final)
					if submitted != 1 || !strings.Contains(wire, "白衬衫") || strings.Contains(wire, "短靴") || strings.Contains(wire, "nonexistent boots") {
						t.Fatalf("renderer lost actual clothing: %s submits=%d", wire, submitted)
					}
					if scenario == "explicit-selfie" {
						stored, err := f.app.readMediaIntent(f.ctx, intentID)
						if err != nil {
							t.Fatal(err)
						}
						frozen := mapValue(decodeObject([]byte(stored.Prompt))["media_capture_plan"])
						if frozen["capture_method"] != "handheld_selfie" || frozen["framing"] != "full_body" {
							t.Fatal("normalization rewrote explicit camera", frozen)
						}
					}
				} else if submitted != 0 {
					t.Fatal("override reached renderer", final)
				}
				if scenario != "workflow-clothing" && modelCalls < 1 {
					t.Fatal("formal media task was bypassed")
				}
			}
			after, _, _, err := f.app.readEffectiveLifeSnapshot(f.ctx, f.fluctlightID, f.app.now())
			if err != nil || appearanceSnapshotIdentity(before) != appearanceSnapshotIdentity(after) {
				t.Fatal("media wrote inventory/body/use", after, err)
			}
		})
	}
}

func TestNormalizeCurrentCapturePlanHandlesSpacedFramingAndWrappers(t *testing.T) {
	concept := map[string]any{
		"capture": map[string]any{"mode": "mirror_selfie", "framing": "full body", "camera": "rear", "device_visibility": "visible"},
		"context_binding": map[string]any{
			"appearance": map[string]any{"body_revision": 0, "wardrobe_revision": 0, "wearing_state": "known", "worn_items": []any{}},
		},
	}

	testCases := []struct {
		name     string
		plan     map[string]any
		expected string
	}{
		{
			name:     "spaced-framing-full-body",
			plan:     map[string]any{"framing": "full body", "pose": "standing", "expression": "smiling", "lighting": "soft", "style": "photographic"},
			expected: "A full-body composition.",
		},
		{
			name: "wrapped-in-current_capture_plan",
			plan: map[string]any{
				"current_capture_plan": map[string]any{
					"framing": "full body", "pose": "standing", "expression": "neutral", "lighting": "daylight", "style": "photographic",
				},
			},
			expected: "A full-body composition.",
		},
		{
			name: "wrapped-in-capture_plan",
			plan: map[string]any{
				"capture_plan": map[string]any{
					"framing": "full_body", "pose": "seated", "expression": "smiling", "lighting": "ambient", "style": "photographic",
				},
			},
			expected: "A full-body composition.",
		},
		{
			name: "capitalized-and-spaced-keys",
			plan: map[string]any{
				"Framing": "full body", "Pose": "standing", "Expression": "smiling", "Lighting": "soft", "Style": "photographic",
			},
			expected: "A full-body composition.",
		},
		{
			name: "missing-framing-inherits-from-concept",
			plan: map[string]any{
				"pose": "standing", "expression": "smiling", "lighting": "soft", "style": "photographic",
			},
			expected: "A full-body composition.",
		},
		{
			name: "spaced-pose-and-lighting-aliases",
			plan: map[string]any{
				"framing": "upper body", "pose": "raising hand", "expression": "smiling", "lighting": "soft light", "style": "photo",
			},
			expected: "An upper-body composition.",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			normalized := normalizeCurrentCapturePlan(concept, tc.plan)
			if err := validateCapabilitySchemaValue(normalized, currentCapturePlanSchema()); err != nil {
				t.Fatalf("validation failed: %v", err)
			}
			prompt, err := renderCurrentCapturePrompt(concept, tc.plan)

			if err != nil {
				t.Fatalf("render failed: %v", err)
			}
			if !strings.Contains(prompt, tc.expected) {
				t.Fatalf("prompt missing expected %q: %s", tc.expected, prompt)
			}
		})
	}
}

func TestCurrentCaptureInvalidEnumFallsBackToFirstPersonFullBodyAndKeepsFrozenFacts(t *testing.T) {
	concept := map[string]any{"capture": map[string]any{"mode": "mirror_selfie", "framing": "upper body", "camera": "front", "mirror": true, "device_visibility": "visible"}, "context_binding": map[string]any{"appearance": map[string]any{"body_revision": 1, "wardrobe_revision": 2, "wearing_state": "known", "worn_items": []any{map[string]any{"id": "held-white-shirt", "slot": "top", "description": "白衬衫", "availability": "available", "source_verified": true}}}}}
	before := jsonString(concept)
	for _, value := range []any{"medium_long_shot", "first_person_full_body", nil, 42} {
		raw := captureStyleForTest()
		raw["framing"] = value
		plan, fallback, err := resolveCurrentCapturePlan(concept, raw)
		if err != nil || plan["framing"] != "full_body" || fallback["mode"] != "first_person" {
			t.Fatalf("fallback %#v %#v %v", plan, fallback, err)
		}
		prepared := cloneMap(concept)
		prepared["capture_plan"] = plan
		prepared["capture_plan_fallback"] = fallback
		prompt, err := renderCurrentCapturePrompt(prepared, plan)
		if err != nil || !strings.Contains(prompt, "full-body composition") || !strings.Contains(prompt, "first-person view") || !strings.Contains(prompt, "白衬衫") || strings.Contains(prompt, "through a mirror") {
			t.Fatalf("fallback prompt=%s err=%v", prompt, err)
		}
		reloaded := decodeObject(jsonBytes(prepared))
		again, err := renderCurrentCapturePrompt(reloaded, mapValue(reloaded["capture_plan"]))
		if err != nil || again != prompt {
			t.Fatalf("persisted fallback changed %s %v", again, err)
		}
		provider, ok := compactMediaConceptObjectForProvider(jsonString(prepared))
		if !ok || mapValue(provider["capture"])["mode"] != "first_person" || mapValue(provider["capture"])["framing"] != "full_body" {
			t.Fatalf("quality/prompt view lost effective fallback %#v", provider)
		}
		if jsonString(concept) != before {
			t.Fatal("fallback rewrote original snapshot/capture")
		}
	}
}

func TestCurrentCaptureFallbackRejectsPhysicalOverridesAndBadSnapshot(t *testing.T) {
	concept := map[string]any{"context_binding": map[string]any{"appearance": map[string]any{"body_revision": 0, "wardrobe_revision": 0, "wearing_state": "known", "worn_items": []any{}}}}
	for _, key := range []string{"clothing", "body", "objects", "reference_images", "prompt"} {
		plan := captureStyleForTest()
		plan["framing"] = "invalid-frame"
		plan[key] = "invented boots"
		if _, err := renderCurrentCapturePrompt(concept, plan); err == nil {
			t.Fatalf("fallback accepted physical/extra field %s", key)
		}
	}
	plan := captureStyleForTest()
	plan["framing"] = "invalid-frame"
	mapValue(mapValue(concept["context_binding"])["appearance"])["wearing_state"] = "unknown"
	if _, err := renderCurrentCapturePrompt(concept, plan); err == nil {
		t.Fatal("fallback bypassed unknown wearing state")
	}
}

func TestCurrentCaptureEnumInstructionMatchesSchemaAndMissingFramingDefault(t *testing.T) {
	instruction := currentCaptureEnumInstruction()
	for key, value := range mapValue(currentCapturePlanSchema()["properties"]) {
		if !strings.Contains(instruction, key) {
			t.Fatalf("field missing %s", key)
		}
		for _, choice := range arrayValue(mapValue(value)["enum"]) {
			if !strings.Contains(instruction, stringValue(choice)) {
				t.Fatalf("choice missing %v", choice)
			}
		}
	}
	plan, fallback, err := resolveCurrentCapturePlan(map[string]any{}, map[string]any{})
	if err != nil || plan["framing"] != "full_body" || fallback["mode"] != "first_person" {
		t.Fatalf("missing framing default %#v %#v %v", plan, fallback, err)
	}
}

func TestMediaPromptStandardCaptureSupersedesVagueUpstreamHints(t *testing.T) {
	concept := map[string]any{"capture": map[string]any{"mode": "selfie", "framing": "portrait crop", "camera": "front"}, "context_binding": map[string]any{"appearance": map[string]any{"body_revision": 0, "wardrobe_revision": 0, "wearing_state": "known", "worn_items": []any{}}}}
	original := jsonString(concept)
	plan := captureStyleForTest()
	plan["framing"] = "full_body"
	plan["capture"] = map[string]any{"mode": "mirror_selfie", "camera": "rear", "device_visibility": "visible", "mirror": true}
	prompt, err := renderCurrentCapturePrompt(concept, plan)
	if err != nil || !strings.Contains(prompt, "full-body composition") || !strings.Contains(prompt, "through a mirror") || !strings.Contains(prompt, "rear camera") {
		t.Fatalf("normalization failed %s %v", prompt, err)
	}
	if jsonString(concept) != original {
		t.Fatal("upstream audit or frozen facts rewritten")
	}
	prepared := cloneMap(concept)
	prepared["capture_plan"] = plan
	quality, ok := compactMediaConceptObjectForProvider(jsonString(prepared))
	if !ok || mapValue(quality["capture"])["mode"] != "mirror_selfie" || mapValue(quality["capture"])["framing"] != "full_body" {
		t.Fatalf("quality sees raw hints rather than standard result %#v", quality)
	}
}
