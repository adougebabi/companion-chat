package core

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func multiLoraTestGraph() map[string]any {
	return map[string]any{
		"positive": map[string]any{"class_type": "CLIPTextEncode", "inputs": map[string]any{"text": "{{prompt}}"}},
		"loras":    map[string]any{"class_type": "Krea2LoRALoader", "inputs": map[string]any{"lora_1": "art.safetensors", "strength_1": 1.0, "lora_2": "color.safetensors", "strength_2": 0.5, "lora_3": "none", "strength_3": 1.0}},
	}
}

func TestMediaMultiLoraClassifiesEverySlot(t *testing.T) {
	none := mediaPreparedConcept(t, "风景", mediaTestPlan("landscape", "none", "no", "unknown"), CapabilityContext{})
	config := map[string]any{"style_loras": []any{"art.safetensors", "color.safetensors", ""}, "face_identity_loras": []any{"face.safetensors"}}
	for _, sample := range []struct {
		name     string
		value    any
		strength any
		reject   bool
	}{
		{"styles", "color.safetensors", 0.5, false}, {"empty", "", 1, false}, {"none", "none", 1, false},
		{"unknown", "unknown.safetensors", 1, true}, {"unknown-zero-still-resolved", "unknown.safetensors", 0, true},
		{"identity", "face.safetensors", 1, true}, {"linked-name", []any{"name", 0}, 1, true}, {"invalid-name", false, 1, true},
	} {
		t.Run(sample.name, func(t *testing.T) {
			graph := multiLoraTestGraph()
			inputs := mapValue(mapValue(graph["loras"])["inputs"])
			inputs["lora_2"], inputs["strength_2"] = sample.value, sample.strength
			before := jsonString(graph)
			err := validateMediaWorkflowPolicy(config, none, graph)
			if (err != nil) != sample.reject || before != jsonString(graph) {
				t.Fatalf("rejected=%t expected=%t err=%v", err != nil, sample.reject, err)
			}
		})
	}
	state := mediaTestContext("CURRENT")
	mapValue(state.Outfit.Data["body_fields"])["chest_cup"] = map[string]any{"status": "known", "value": "B"}
	self := mediaPreparedConcept(t, "本人全身", mediaTestPlan("person", "present", "yes", "full"), state)
	graph := multiLoraTestGraph()
	slots := mapValue(mapValue(graph["loras"])["inputs"])
	slots["lora_1"], slots["strength_1"] = "body.safetensors", "{{chest_lora_weight}}"
	if err := validateMediaWorkflowPolicy(config, self, graph); err != nil {
		t.Fatal(err)
	}
	slots["lora_2"] = "unclassified.safetensors"
	if err := validateMediaWorkflowPolicy(config, self, graph); err == nil {
		t.Fatal("second slot borrowed first slot body proof")
	}
	slots["lora_2"] = "face.safetensors"
	if err := validateMediaWorkflowPolicy(config, self, graph); err != nil {
		t.Fatal(err)
	}
}

// Real database, formal tool admission and media processing. Model and physical
// renderer responses are controlled; no actual image is generated.
func TestMediaMultiLoraFormalImageBoundary(t *testing.T) {
	for _, reject := range []bool{false, true} {
		t.Run(map[bool]string{false: "style", true: "identity-rejected"}[reject], func(t *testing.T) {
			ctx, repo, app, owner, fid, conversation := setupDirectPublicationToolTest(t, "multi-lora")
			seedCognitiveProviderRole(t, ctx, repo, "multi-lora-provider")
			graph := multiLoraTestGraph()
			if reject {
				mapValue(mapValue(graph["loras"])["inputs"])["lora_2"] = "face.safetensors"
			}
			config := map[string]any{"baseUrl": "http://controlled-renderer.invalid", "workflow": graph, "style_loras": []any{"art.safetensors", "color.safetensors"}, "face_identity_loras": []any{"face.safetensors"}}
			if _, err := repo.Pool().Exec(ctx, `UPDATE public.runtime_settings SET value_json=$1 WHERE key='media.comfyui'`, jsonString(config)); err != nil {
				t.Fatal(err)
			}
			var concept map[string]any
			posts := 0
			router := newFakeProviderRouter().otherwise(func(map[string]any) fakeProviderResult {
				return fakeProviderResult{Structured: mediaTestOutput(concept)}
			})
			app.Provider.HTTP = &http.Client{Transport: projectHealthRoundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path != "/prompt" {
					return router.RoundTrip(r)
				}
				posts++
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				if jsonString(mapValue(body["prompt"])["loras"]) != jsonString(graph["loras"]) {
					t.Fatal("physical adapter graph changed")
				}
				return embeddingHTTPResponse(r, http.StatusBadRequest, `{"error":{"type":"prompt_outputs_failed_validation"},"node_errors":{}}`), nil
			})}
			receipt, err := app.ExecuteTool(ctx, ToolExecutionRequest{CapabilityName: "media.image.generate", OperationID: "multi-lora", AuthorizationActorID: owner, FluctlightID: fid, ConversationID: conversation, TargetKind: "conversation", TargetRef: conversation, Arguments: jsonBytes(map[string]any{"intent": "窗外风景", "media_plan": mediaTestPlan("landscape", "none", "no", "unknown")})})
			if err != nil || receipt.Result.Status != "accepted" {
				t.Fatalf("admission: %#v %v", receipt, err)
			}
			id := stringValue(mapValue(receipt.Result.Output)["media_intent_id"])
			var raw []byte
			if err := repo.Pool().QueryRow(ctx, `SELECT prompt FROM public.media_intents WHERE id=$1`, id).Scan(&raw); err != nil {
				t.Fatal(err)
			}
			concept = decodeObject(raw)
			_, err = app.ProcessMediaIntent(ctx, id)
			if reject {
				if err == nil || !strings.Contains(err.Error(), "media_workflow_unclassified_lora_forbidden") || posts != 0 {
					t.Fatalf("unsafe graph reached renderer: posts=%d err=%v", posts, err)
				}
			} else if err == nil || !strings.Contains(err.Error(), "HTTP 400") || posts != 1 {
				t.Fatalf("classified graph did not reach renderer: posts=%d err=%v", posts, err)
			}
		})
	}
}
