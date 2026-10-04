package core

import (
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
	if _, err := renderCurrentCapturePrompt(concept, plan); err == nil {
		t.Fatal("explicit framing silently replaced")
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
	for _, scenario := range []string{"valid-frozen", "model-clothing", "workflow-clothing"} {
		t.Run(scenario, func(t *testing.T) {
			f := seedWardrobeToolFixture(t)
			seedCognitiveProviderRole(t, f.ctx, f.repository, "capture-worker-provider-"+f.suffix)
			workflow := map[string]any{"1": map[string]any{"class_type": "CLIPTextEncode", "inputs": map[string]any{"text": "{{prompt}}"}}, "2": map[string]any{"class_type": "KSampler", "inputs": map[string]any{"positive": []any{"1", 0}}}}
			if scenario == "workflow-clothing" {
				mapValue(mapValue(workflow["1"])["inputs"])["text"] = "{{prompt}} wearing nonexistent boots"
			}
			if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.runtime_settings(key,value_json) VALUES('media.comfyui',$1)`, jsonString(map[string]any{"baseUrl": "http://capture-comfy.invalid", "workflow": workflow})); err != nil {
				t.Fatal(err)
			}
			submitted := 0
			modelCalls := 0
			var final map[string]any
			f.app.Provider.HTTP = &http.Client{Transport: projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
				body, _ := io.ReadAll(request.Body)
				if request.URL.Host == "capture-comfy.invalid" {
					if request.URL.Path != "/prompt" {
						t.Fatal("unexpected renderer request", request.URL.Path)
					}
					submitted++
					final = decodeObject(body)
					return embeddingHTTPResponse(request, http.StatusServiceUnavailable, `{}`), nil
				}
				modelCalls++
				plan := captureStyleForTest()
				if scenario == "model-clothing" {
					plan["clothing"] = "nonexistent boots"
				}
				response := map[string]any{"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": jsonString(plan)}}}}
				return embeddingHTTPResponse(request, http.StatusOK, jsonString(response)), nil
			})}
			before, _, _, err := f.app.readEffectiveLifeSnapshot(f.ctx, f.fluctlightID, f.app.now())
			if err != nil {
				t.Fatal(err)
			}
			request := f.request("media.image.generate", "current-worker", map[string]any{"intent": "穿上刚买的黑色短靴拍现在的照片"})
			request.TargetKind = "conversation"
			request.TargetRef = f.conversationID
			accepted, err := f.app.ExecuteTool(f.ctx, request)
			if err != nil {
				t.Fatal(err)
			}
			intentID := stringValue(mapValue(accepted.Result.Output)["media_intent_id"])
			if _, err := f.app.ProcessMediaIntent(f.ctx, intentID); err == nil {
				t.Fatal("renderer failure/override was reported successful")
			}
			if scenario == "valid-frozen" {
				wire := jsonString(final)
				if submitted != 1 || !strings.Contains(wire, "白衬衫") || strings.Contains(wire, "短靴") || strings.Contains(wire, "nonexistent boots") {
					t.Fatalf("final renderer lost authority: %s submits=%d", wire, submitted)
				}
				t.Logf("FINAL_COMFY_INPUT=%s", wire)
			} else if submitted != 0 {
				t.Fatal("override reached final renderer", final)
			}
			if modelCalls < 1 {
				t.Fatal("formal MediaPrompt was bypassed")
			}
			after, _, _, err := f.app.readEffectiveLifeSnapshot(f.ctx, f.fluctlightID, f.app.now())
			if err != nil || appearanceSnapshotIdentity(before) != appearanceSnapshotIdentity(after) {
				t.Fatalf("media wrote inventory/body/use %#v %v", after, err)
			}
		})
	}
}
