package core

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestImageSubmissionForbidsTextInPositiveAndNegativeProviderInputs(t *testing.T) {
	for _, purpose := range []string{"scene", "visual_identity", "studio_definition"} {
		t.Run(purpose, func(t *testing.T) {
			prompt := "保留原始人物与场景事实"
			if purpose == "visual_identity" {
				prompt = visualIdentityPromptFromConcept(map[string]any{"visual_identity": map[string]any{"identity_snapshot": map[string]any{"identity": map[string]any{"name": "青岚"}}}})
			}
			const seed int64 = 9007199254740993
			workflow := map[string]any{
				"positive": map[string]any{"class_type": "CLIPTextEncode", "inputs": map[string]any{"text": prompt}},
				"negative": map[string]any{"class_type": "CLIPTextEncodeSDXL", "inputs": map[string]any{"text_g": "low quality", "text_l": ""}},
				"combine":  map[string]any{"class_type": "ConditioningCombine", "inputs": map[string]any{"conditioning_1": []any{"negative", 0}, "conditioning_2": []any{"negative", 0}}},
				"sampler":  map[string]any{"class_type": "KSampler", "inputs": map[string]any{"positive": []any{"positive", 0}, "negative": []any{"combine", 0}, "seed": seed}},
				"custom":   map[string]any{"inputs": map[string]any{"prompt": prompt, "negative_prompt": "blur"}},
				"image":    map[string]any{"class_type": "LoadImage", "inputs": map[string]any{"image": "original-reference.png"}},
			}
			before := jsonString(workflow)
			calls := 0
			app := &App{Provider: &ProviderClient{HTTP: &http.Client{Transport: projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
				calls++
				if request.URL.Path != "/prompt" {
					t.Fatalf("wrong physical boundary: %s", request.URL.Path)
				}
				var payload map[string]any
				decoder := json.NewDecoder(request.Body)
				decoder.UseNumber()
				if err := decoder.Decode(&payload); err != nil {
					t.Fatal(err)
				}
				actual := mapValue(payload["prompt"])
				for _, path := range []struct{ node, key, original, rule string }{
					{"positive", "text", prompt, imageNoTextInstruction},
					{"negative", "text_g", "low quality", imageNoTextNegativePrompt},
					{"negative", "text_l", "", imageNoTextNegativePrompt},
					{"custom", "prompt", prompt, imageNoTextInstruction},
					{"custom", "negative_prompt", "blur", imageNoTextNegativePrompt},
				} {
					value := stringValue(mapValue(mapValue(actual[path.node])["inputs"])[path.key])
					if !strings.HasPrefix(value, path.original) || !strings.Contains(value, path.rule) {
						t.Errorf("physical %s.%s lost source or text prohibition: %q", path.node, path.key, value)
					}
				}
				if mapValue(mapValue(actual["sampler"])["inputs"])["seed"] != json.Number("9007199254740993") || mapValue(mapValue(actual["image"])["inputs"])["image"] != "original-reference.png" {
					t.Fatal("text policy changed native numeric controls or reference image")
				}
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"prompt_id":"text-free"}`)), Header: make(http.Header)}, nil
			})}}}
			response, err := app.submitComfyPrompt(context.Background(), mediaIntent{}, map[string]any{"purpose": purpose}, workflow, prompt, "http://controlled.invalid")
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if calls != 1 || before != jsonString(workflow) {
				t.Fatal("submission changed frozen graph or did not send one request")
			}
			first := imageWorkflowWithoutText(workflow)
			if jsonString(imageWorkflowWithoutText(first)) != jsonString(first) {
				t.Fatal("text policy duplicated constraints on replay")
			}
		})
	}
}

func TestImageNegativePromptPlaceholderResolvesWithoutLosingOriginalExclusions(t *testing.T) {
	workflow, err := replaceMediaPlaceholders(map[string]any{"node": map[string]any{"inputs": map[string]any{"negative_prompt": "blur, {{negative_prompt}}"}}}, "portrait", nil)
	if err != nil {
		t.Fatal(err)
	}
	negative := stringValue(mapValue(mapValue(workflow["node"])["inputs"])["negative_prompt"])
	if negative != "blur, "+imageNoTextNegativePrompt {
		t.Fatal(negative)
	}
}

func TestImageSubmissionPreservesPositiveTextWhenFluxSharesNegativeConditioning(t *testing.T) {
	const original = "青岚站在树林中，保留原始场景"
	workflow := map[string]any{
		"encode": map[string]any{"class_type": "CLIPTextEncode", "inputs": map[string]any{"text": original}},
		"zero":   map[string]any{"class_type": "ConditioningZeroOut", "inputs": map[string]any{"conditioning": []any{"encode", 0}}},
		"sample": map[string]any{"class_type": "KSampler", "inputs": map[string]any{"positive": []any{"encode", 0}, "negative": []any{"zero", 0}}},
	}
	before := jsonString(workflow)
	app := &App{Provider: &ProviderClient{HTTP: &http.Client{Transport: projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		var payload map[string]any
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		actual := mapValue(payload["prompt"])
		positive := stringValue(mapValue(mapValue(actual["encode"])["inputs"])["text"])
		if positive != original+"\n\n"+imageNoTextInstruction || strings.Contains(positive, imageNoTextNegativePrompt) {
			t.Fatalf("shared positive encoder received negative exclusion tokens: %q", positive)
		}
		for _, node := range []string{"zero", "sample"} {
			if jsonString(actual[node]) != jsonString(workflow[node]) {
				t.Fatalf("conditioning graph changed for %s", node)
			}
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"prompt_id":"flux-text-free"}`)), Header: make(http.Header)}, nil
	})}}}
	response, err := app.submitComfyPrompt(context.Background(), mediaIntent{}, nil, workflow, original, "http://controlled.invalid")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if before != jsonString(workflow) {
		t.Fatal("submission changed frozen workflow")
	}
	first := imageWorkflowWithoutText(workflow)
	if jsonString(imageWorkflowWithoutText(first)) != jsonString(first) {
		t.Fatal("shared conditioning duplicated the no-text rule on replay")
	}
}

func TestMediaPromptPhysicalRequestsRequireTextFreeImages(t *testing.T) {
	for _, stage := range []string{"legacy", "retry", "normalized"} {
		t.Run(stage, func(t *testing.T) {
			instruction := mediaPromptSystemInstruction(mediaIntent{})
			if stage == "retry" {
				instruction = mediaPromptSystemInstruction(mediaIntent{QualityRetryCount: 1})
			}
			if stage == "normalized" {
				instruction = mediaNormalizationInstruction
			}
			router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
				wire := jsonString(payload["messages"])
				for _, required := range []string{"禁止文字", "伪文字", "字母", "数字", "签名", "水印", "原始图片需求"} {
					if !strings.Contains(wire, required) {
						t.Errorf("physical prompt lost %q: %s", required, wire)
					}
				}
				for _, key := range []string{"max_tokens", "max_completion_tokens"} {
					if _, exists := payload[key]; exists {
						t.Error("unexpected output token limit", key)
					}
				}
				return fakeProviderResult{Text: "生成正文"}
			})
			messages := formatProviderMessagesForRole([]map[string]any{{"role": "system", "content": instruction}, {"role": "user", "content": jsonString(map[string]any{"intent": "原始图片需求"})}}, "media_prompt")
			_, err := (&ProviderClient{HTTP: &http.Client{Transport: router}}).generateWithEino(context.Background(), EinoModelCall{
				Assignment: providerAssignment{Role: "generic_llm", BaseURL: "http://controlled.invalid", ModelID: "fake", Timeout: time.Second, TokenBudget: 1},
				Role:       "media_prompt", Messages: formalPromptMessages(FormalAgentMediaPrompt, messages),
			})
			if err != nil || router.totalRequests() != 1 {
				t.Fatalf("physical prompt request err=%v calls=%d", err, router.totalRequests())
			}
		})
	}
}
