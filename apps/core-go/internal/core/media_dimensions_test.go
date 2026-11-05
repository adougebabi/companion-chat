package core

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestImageDimensionsFreezeRequestedRatioAndReplay(t *testing.T) {
	for _, test := range []struct {
		ratio      string
		resolution string
		width      int
		height     int
	}{
		{"", "", 432, 768}, {"9:16", "", 432, 768}, {"16:9", "", 768, 432}, {"1:1", "", 768, 768}, {"3:2", "", 768, 512}, {"2:3", "", 512, 768}, {"16:9", "2048", 2048, 1152},
	} {
		t.Run("ratio_"+test.ratio+"_"+test.resolution, func(t *testing.T) {
			args := map[string]any{"intent": "一只小猫", "media_plan": mediaTestPlan("animal", "none", "no", "unknown")}
			if test.ratio != "" {
				args["aspect_ratio"] = test.ratio
			}
			expectedResolution := defaultImageResolution
			if test.resolution != "" {
				args["resolution"] = test.resolution
				expectedResolution = test.resolution
			}
			if err := validateCapabilitySchemaValue(args, imageCapabilityInputSchema()); err != nil {
				t.Fatal(err)
			}
			call := CapabilityInvocation{Arguments: jsonBytes(args)}
			prepared, err := (imageGenerateCapability{}).Prepare(context.Background(), call, CapabilityContext{})
			if err != nil {
				t.Fatal(err)
			}
			raw, found, err := capabilityPreparedData(prepared, "media_concept")
			if err != nil || !found {
				t.Fatalf("missing prepared concept: %v", err)
			}
			concept := mapValue(raw)
			dimensions, err := mediaImageDimensions(concept)
			width, _ := imageDimensionInt(dimensions["width"])
			height, _ := imageDimensionInt(dimensions["height"])
			if err != nil || width != int64(test.width) || height != int64(test.height) || dimensions["resolution"] != expectedResolution {
				t.Fatalf("incorrect frozen dimensions: %#v %v", dimensions, err)
			}
			before := jsonString(concept)
			replayed, err := (imageGenerateCapability{}).Prepare(context.Background(), prepared, CapabilityContext{})
			if err != nil {
				t.Fatal(err)
			}
			replayedConcept, _, _ := capabilityPreparedData(replayed, "media_concept")
			if jsonString(replayedConcept) != before {
				t.Fatal("replay changed the frozen image dimensions")
			}
			packet, err := mediaNormalizationInput(concept)
			modelDimensions := mapValue(packet["image_dimensions"])
			if err != nil || modelDimensions["aspect_ratio"] != dimensions["aspect_ratio"] || modelDimensions["resolution"] != dimensions["resolution"] || len(modelDimensions) != 2 {
				t.Fatalf("normalizer did not receive the frozen dimensions: %#v %v", packet, err)
			}
		})
	}
	for _, raw := range []any{"4:3", "", 42, nil} {
		args := map[string]any{"intent": "猫", "aspect_ratio": raw}
		if _, err := requestedImageDimensions(args); err == nil {
			t.Fatalf("accepted invalid ratio %#v", raw)
		}
		if err := validateCapabilitySchemaValue(args, imageCapabilityInputSchema()); err == nil {
			t.Fatalf("tool schema accepted invalid ratio %#v", raw)
		}
	}
	for _, level := range []string{"512", "768", "1024", "1280", "1536", "2048"} {
		args := map[string]any{"intent": "猫", "aspect_ratio": "3:2", "resolution": level}
		if err := validateCapabilitySchemaValue(args, imageCapabilityInputSchema()); err != nil {
			t.Fatalf("resolution %s was rejected: %v", level, err)
		}
		dimensions, err := requestedImageDimensions(args)
		if err != nil || dimensions["resolution"] != level {
			t.Fatalf("resolution %s was not preserved: %#v %v", level, dimensions, err)
		}
		if _, err := mediaImageDimensions(map[string]any{"image_dimensions": dimensions}); err != nil {
			t.Fatalf("resolution %s did not produce valid numeric dimensions: %v", level, err)
		}
	}
	for _, raw := range []any{"789", "720x1280", 768, nil} {
		args := map[string]any{"intent": "猫", "resolution": raw}
		if _, err := requestedImageDimensions(args); err == nil {
			t.Fatalf("accepted invalid resolution %#v", raw)
		}
		if err := validateCapabilitySchemaValue(args, imageCapabilityInputSchema()); err == nil {
			t.Fatalf("tool schema accepted invalid resolution %#v", raw)
		}
	}
}

func TestImageDimensionsReachComfyPhysicalRequest(t *testing.T) {
	for _, test := range []struct {
		name, ratio, resolution string
		concept                 map[string]any
		width                   string
		height                  string
	}{
		{"avatar", "1:1", "768", map[string]any{"purpose": "visual_identity", "stage": "avatar"}, "768", "768"},
		{"character sheet", "2:3", "768", map[string]any{"purpose": "visual_identity", "stage": "character_sheet"}, "512", "768"},
		{"ordinary portrait", "9:16", "768", map[string]any{}, "432", "768"},
		{"ordinary landscape", "16:9", "768", map[string]any{}, "768", "432"},
		{"ordinary 3:2", "3:2", "768", map[string]any{}, "768", "512"},
		{"explicit 2048", "16:9", "2048", map[string]any{}, "2048", "1152"},
	} {
		t.Run(test.name, func(t *testing.T) {
			frozen, err := imageDimensionsAtResolution(test.ratio, test.resolution)
			if err != nil {
				t.Fatal(err)
			}
			test.concept["image_dimensions"] = frozen
			dimensions, err := mediaImageDimensions(test.concept)
			if err != nil {
				t.Fatal(err)
			}
			graph := map[string]any{
				"positive": map[string]any{"class_type": "CLIPTextEncode", "inputs": map[string]any{"text": "{{prompt}}"}},
				"latent":   map[string]any{"class_type": "EmptyLatentImage", "inputs": map[string]any{"width": "{{width}}", "height": "{{height}}"}},
				"preset": map[string]any{"class_type": "RatioResolutionNode", "inputs": map[string]any{
					"ratio": "{{ratio}}", "resolution": "{{resolution}}", "label": "{{aspect_ratio}}/{{width}}x{{height}}",
				}},
			}
			before := jsonString(graph)
			workflow, err := replaceMediaPlaceholders(graph, "A cat in a garden", map[string]any{"image_dimensions": dimensions})
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			app := &App{Provider: &ProviderClient{HTTP: &http.Client{Transport: projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
				calls++
				var payload map[string]any
				decoder := json.NewDecoder(request.Body)
				decoder.UseNumber()
				if err := decoder.Decode(&payload); err != nil {
					t.Fatal(err)
				}
				submitted := mapValue(payload["prompt"])
				inputs := mapValue(mapValue(submitted["latent"])["inputs"])
				preset := mapValue(mapValue(submitted["preset"])["inputs"])
				if inputs["width"] != json.Number(test.width) || inputs["height"] != json.Number(test.height) || preset["ratio"] != test.ratio || preset["resolution"] != test.resolution || preset["label"] != test.ratio+"/"+test.width+"x"+test.height {
					t.Fatalf("incorrect physical ComfyUI dimensions: width/height=%#v preset=%#v", inputs, preset)
				}
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"prompt_id":"dimension-test"}`)), Header: make(http.Header)}, nil
			})}}}
			response, err := app.submitComfyPrompt(context.Background(), mediaIntent{}, test.concept, workflow, "A cat in a garden", "http://controlled.invalid")
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if calls != 1 || jsonString(graph) != before {
				t.Fatal("submission did not preserve the original workflow")
			}
		})
	}
}

func TestImageDimensionsKeepLegacyWorkflowsAndRejectInvalidFrozenSize(t *testing.T) {
	legacy := map[string]any{"node": map[string]any{"inputs": map[string]any{"width": 512, "height": 768, "text": "{{prompt}}"}}}
	frozen, _ := imageDimensions("1:1")
	replaced, err := replaceMediaPlaceholders(legacy, "radio", map[string]any{"image_dimensions": frozen})
	if err != nil || mapValue(mapValue(replaced["node"])["inputs"])["width"] != 512 || mapValue(mapValue(replaced["node"])["inputs"])["height"] != 768 {
		t.Fatalf("fixed workflow dimensions changed: %#v %v", replaced, err)
	}
	if _, err := replaceMediaPlaceholders(map[string]any{"width": "{{width}}"}, "", nil); err == nil {
		t.Fatal("unresolved dimensions should fail before submission")
	}
	for _, bad := range []map[string]any{
		{"aspect_ratio": "1:1", "width": 1024, "height": 512, "resolution": "1024x512"},
		{"aspect_ratio": "1:1", "width": 1024.5, "height": 1024, "resolution": "1024x1024"},
		{"aspect_ratio": "16:9", "width": 1280, "height": 720, "resolution": "wrong"},
	} {
		if _, err := mediaImageDimensions(map[string]any{"image_dimensions": bad}); err == nil {
			t.Fatalf("accepted invalid frozen dimensions: %#v", bad)
		}
	}
	landscape, _ := imageDimensions("16:9")
	if _, err := mediaImageDimensions(map[string]any{"purpose": "visual_identity", "stage": "avatar", "image_dimensions": landscape}); err == nil {
		t.Fatal("avatar accepted a non-square frozen size")
	}
	if dimensions, err := mediaImageDimensions(map[string]any{"purpose": "visual_identity", "stage": "avatar"}); err != nil || dimensions["aspect_ratio"] != "1:1" {
		t.Fatalf("legacy avatar fallback = %#v, %v", dimensions, err)
	}
	for _, ratio := range []string{"1:1", "3:4", "9:16"} {
		legacy := legacyImageDimensions(ratio)
		concept := map[string]any{"image_dimensions": legacy}
		if ratio == "3:4" {
			concept["purpose"], concept["stage"] = "visual_identity", "character_sheet"
		}
		recovered, err := mediaImageDimensions(concept)
		if err != nil || jsonString(recovered) != jsonString(legacy) {
			t.Fatalf("legacy %s dimensions were rewritten: %#v %v", ratio, recovered, err)
		}
		legacyGraph, err := replaceMediaPlaceholders(map[string]any{"ratio": "{{ratio}}", "resolution": "{{resolution}}"}, "", map[string]any{"image_dimensions": recovered})
		if err != nil || legacyGraph["ratio"] != ratio || legacyGraph["resolution"] != legacy["resolution"] {
			t.Fatalf("legacy workflow values were rewritten: %#v %v", legacyGraph, err)
		}
	}
}
