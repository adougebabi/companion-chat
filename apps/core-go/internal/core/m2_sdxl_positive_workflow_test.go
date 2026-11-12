package core

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// The database and ProcessMediaIntent path are real; the model and renderer
// responses are controlled and do not claim real image generation.
func TestSDXLPositiveTextCannotOverrideNoPersonImage(t *testing.T) {
	for _, sample := range []struct {
		name          string
		global, local any
		root          string
		reject        bool
		primitive     string
	}{
		{"fixed-global", "人物穿红衣", "{{prompt}}", "positive", true, ""},
		{"fixed-local", "{{prompt}}", "人物穿红衣", "positive", true, ""},
		{"fixed-via-positive-prompt-edge", "{{prompt}}", "人物穿红衣", "positive_prompt", true, ""},
		{"fixed-primitive", []any{"value", 0}, "{{prompt}}", "positive", true, "人物穿红衣"},
		{"both-frozen", "{{prompt}}", "{{prompt}}", "positive", false, ""},
		{"empty-local", "{{prompt}}", "", "positive", false, ""},
	} {
		t.Run(sample.name, func(t *testing.T) {
			ctx, repo, app, owner, fid, conversation := setupDirectPublicationToolTest(t, "m2-sdxl-"+sample.name)
			seedCognitiveProviderRole(t, ctx, repo, "m2-sdxl-provider-"+sample.name)
			var concept map[string]any
			var rendererCalls int
			router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
				if strings.Contains(jsonString(payload["messages"]), "blue coat") {
					t.Error("no-person normalizer request included the character's clothing")
				}
				return fakeProviderResult{Structured: mediaTestOutput(concept)}
			})
			app.Provider.HTTP = &http.Client{Transport: projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
				if request.URL.Path != "/prompt" {
					return router.RoundTrip(request)
				}
				rendererCalls++
				var payload map[string]any
				if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
					t.Fatal(err)
				}
				graph := mapValue(payload["prompt"])
				positive := mapValue(mapValue(graph["positive"])["inputs"])
				negative := mapValue(mapValue(graph["negative"])["inputs"])
				if strings.Contains(jsonString(positive), "人物穿红衣") {
					t.Error("renderer received fixed person/clothing text")
				}
				if !strings.HasPrefix(stringValue(negative["text_g"]), "low quality") {
					t.Error("negative quality text changed")
				}
				return embeddingHTTPResponse(request, http.StatusBadRequest, `{"error":{"type":"prompt_outputs_failed_validation"},"node_errors":{}}`), nil
			})}
			graph := map[string]any{
				"positive": map[string]any{"class_type": "CLIPTextEncodeSDXL", "inputs": map[string]any{"text_g": sample.global, "text_l": sample.local}},
				"negative": map[string]any{"class_type": "CLIPTextEncodeSDXL", "inputs": map[string]any{"text_g": "low quality", "text_l": ""}},
				"sampler":  map[string]any{"class_type": "KSampler", "inputs": map[string]any{sample.root: []any{"positive", 0}, "negative": []any{"negative", 0}}},
			}
			if sample.primitive != "" {
				graph["value"] = map[string]any{"class_type": "PrimitiveString", "inputs": map[string]any{"value": sample.primitive}}
			}
			if _, err := repo.Pool().Exec(ctx, `UPDATE public.runtime_settings SET value_json=$1 WHERE key='media.comfyui'`, jsonString(map[string]any{"baseUrl": "http://renderer.invalid", "workflow": graph})); err != nil {
				t.Fatal(err)
			}
			request := ToolExecutionRequest{CapabilityName: "media.image.generate", OperationID: "m2-sdxl-" + sample.name, AuthorizationActorID: owner, FluctlightID: fid, ConversationID: conversation, TargetKind: "conversation", TargetRef: conversation, Arguments: jsonBytes(map[string]any{"intent": "窗外风景", "media_plan": mediaTestPlan("landscape", "none", "no", "unknown")})}
			receipt, err := app.ExecuteTool(ctx, request)
			if err != nil || receipt.Result.Status != "accepted" {
				t.Fatalf("Image admission failed: %#v err=%v", receipt, err)
			}
			intentID := stringValue(mapValue(receipt.Result.Output)["media_intent_id"])
			var raw []byte
			if err := repo.Pool().QueryRow(ctx, `SELECT prompt FROM public.media_intents WHERE id=$1`, intentID).Scan(&raw); err != nil {
				t.Fatal(err)
			}
			concept = decodeObject(raw)
			_, err = app.ProcessMediaIntent(ctx, intentID)
			if sample.reject {
				if err == nil || !strings.Contains(err.Error(), "current_capture_workflow_positive_override") || rendererCalls != 0 {
					t.Fatalf("fixed positive text reached renderer: calls=%d err=%v", rendererCalls, err)
				}
			} else if err == nil || !strings.Contains(err.Error(), "HTTP 400") || rendererCalls != 1 {
				t.Fatalf("valid SDXL workflow did not reach controlled renderer: calls=%d err=%v", rendererCalls, err)
			}
		})
	}
}

func TestDirectPositivePromptCannotBypassFrozenText(t *testing.T) {
	for _, sample := range []struct {
		name, value string
		reject      bool
	}{{"fixed", "人物穿红衣", true}, {"frozen", "{{prompt}}", false}} {
		t.Run(sample.name, func(t *testing.T) {
			graph := map[string]any{"unused": map[string]any{"class_type": "CLIPTextEncode", "inputs": map[string]any{"text": "{{prompt}}"}}, "sampler": map[string]any{"class_type": "CustomSampler", "inputs": map[string]any{"positive_prompt": sample.value}}}
			err := validateCurrentCaptureWorkflow(graph)
			if sample.reject != (err != nil) {
				t.Fatalf("direct positive prompt accepted=%t err=%v", err == nil, err)
			}
		})
	}
}

func TestSDXLPositiveTextRejectsFixedPrimitiveString(t *testing.T) {
	for _, sample := range []struct {
		name, value string
		reject      bool
	}{{"fixed", "人物穿红衣", true}, {"frozen", "{{prompt}}", false}, {"empty", "", false}} {
		t.Run(sample.name, func(t *testing.T) {
			graph := map[string]any{
				"value":    map[string]any{"class_type": "PrimitiveString", "inputs": map[string]any{"value": sample.value}},
				"positive": map[string]any{"class_type": "CLIPTextEncodeSDXL", "inputs": map[string]any{"text_g": []any{"value", 0}, "text_l": "{{prompt}}"}},
				"sampler":  map[string]any{"class_type": "KSampler", "inputs": map[string]any{"positive": []any{"positive", 0}}},
			}
			err := validateCurrentCaptureWorkflow(graph)
			if sample.reject != (err != nil) {
				t.Fatalf("primitive positive text accepted=%t err=%v", err == nil, err)
			}
		})
	}
}
