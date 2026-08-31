package core

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

type testCapabilityExecutor struct{}

func (testCapabilityExecutor) Manifest() CapabilityManifest {
	return CapabilityManifest{Name: "search.lookup", Version: "v1", Description: "Look up a bounded fact.", SideEffectClass: "read_only", ConcurrencyClass: "parallel"}
}

func (testCapabilityExecutor) Execute(_ context.Context, _, _, _ string, call ToolCallV1) (ToolResultV1, error) {
	return ToolResultV1{ToolCallID: call.ID, Name: call.Name, Status: "completed", Output: map[string]any{"value": "ok"}, SchemaVersion: ToolResultSchemaVersion}, nil
}

func TestNormalizeProviderToolCallsAcceptsNativeAndSidecarShapes(t *testing.T) {
	calls, err := NormalizeProviderToolCalls([]any{
		map[string]any{
			"id": "call_native",
			"function": map[string]any{
				"name":      "media.image.generate",
				"arguments": `{"concept":{"subject":"a cat"}}`,
			},
		},
		map[string]any{
			"id":        "call_sidecar",
			"name":      "media.image.generate",
			"arguments": map[string]any{"concept": map[string]any{"subject": "a dog"}},
		},
	}, "fact-1", "provider-1")
	if err != nil {
		t.Fatalf("NormalizeProviderToolCalls() error = %v", err)
	}
	if len(calls) != 2 || calls[0].Sequence != 0 || calls[1].Sequence != 1 {
		t.Fatalf("normalized calls = %#v", calls)
	}
	if calls[0].SchemaVersion != ToolCallSchemaVersion || calls[0].SourceFactID != "fact-1" {
		t.Fatalf("normalized metadata = %#v", calls[0])
	}
	var firstArgs map[string]any
	if err := json.Unmarshal(calls[0].Arguments, &firstArgs); err != nil {
		t.Fatalf("first arguments are not JSON = %v", err)
	}
	if firstArgs["concept"] == nil {
		t.Fatalf("first arguments = %#v", firstArgs)
	}
}

func TestNormalizeProviderToolCallsRejectsMalformedOrDuplicateCalls(t *testing.T) {
	cases := []struct {
		name  string
		value any
	}{
		{name: "missing id", value: []any{map[string]any{"name": "media.image.generate", "arguments": `{}`}}},
		{name: "invalid name", value: []any{map[string]any{"id": "call", "name": "media image", "arguments": `{}`}}},
		{name: "non object arguments", value: []any{map[string]any{"id": "call", "name": "media.image.generate", "arguments": `[]`}}},
		{name: "duplicate id", value: []any{
			map[string]any{"id": "call", "name": "media.image.generate", "arguments": `{}`},
			map[string]any{"id": "call", "name": "media.image.generate", "arguments": `{}`},
		}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := NormalizeProviderToolCalls(testCase.value, "fact", "provider"); err == nil {
				t.Fatal("expected normalization error")
			}
		})
	}

	tooLarge := strings.Repeat("x", maxToolArgumentsBytes)
	if _, err := NormalizeProviderToolCalls([]any{map[string]any{"id": "call", "name": "media.image.generate", "arguments": `{"value":"` + tooLarge + `"}`}}, "fact", "provider"); err == nil {
		t.Fatal("expected oversized arguments error")
	}
}

func TestToolCallValidateRequiresRegisteredCapability(t *testing.T) {
	manifests := toolManifestMap(ExternalCapabilityManifests())
	calls, err := NormalizeProviderToolCalls([]any{map[string]any{
		"id":   "call",
		"name": "media.image.generate",
		"arguments": map[string]any{
			"concept": map[string]any{"subject": "a cat"},
		},
	}}, "fact", "provider")
	if err != nil {
		t.Fatalf("normalize = %v", err)
	}
	if err := calls[0].Validate(manifests); err != nil {
		t.Fatalf("registered capability rejected = %v", err)
	}
	calls[0].Name = "media.video.generate"
	if err := calls[0].Validate(manifests); err == nil {
		t.Fatal("expected unavailable capability error")
	}
}

func TestToolCallPayloadKeepsProviderSchemaAtBoundary(t *testing.T) {
	payload := ToolCallPayload(ExternalCapabilityManifests())
	if len(payload) != 1 {
		t.Fatalf("tool payload = %#v", payload)
	}
	function, ok := payload[0]["function"].(map[string]any)
	if !ok || function["name"] != "media.image.generate" {
		t.Fatalf("function payload = %#v", payload[0])
	}
	parameters, ok := function["parameters"].(map[string]any)
	if !ok || parameters["type"] != "object" {
		t.Fatalf("parameters payload = %#v", function)
	}
}

func TestCapabilityRegistryIsAnExtensibleSlot(t *testing.T) {
	registry := NewCapabilityRegistry(testCapabilityExecutor{})
	manifests := registry.Manifests()
	if len(manifests) != 1 || manifests[0].Name != "search.lookup" {
		t.Fatalf("manifests = %#v", manifests)
	}
	calls, err := NormalizeProviderToolCalls([]any{map[string]any{"id": "call", "name": "search.lookup", "arguments": `{ "query": "fluctlight" }`}}, "fact", "provider")
	if err != nil {
		t.Fatalf("normalize = %v", err)
	}
	if err := calls[0].Validate(toolManifestMap(manifests)); err != nil {
		t.Fatalf("call validation = %v", err)
	}
	result, ok := registry.Lookup("search.lookup")
	if !ok || result == nil {
		t.Fatalf("lookup = %#v, ok=%v", result, ok)
	}
	output, err := result.Execute(context.Background(), "fl", "conv", "fact", calls[0])
	if err != nil {
		t.Fatalf("execute = %v", err)
	}
	if err := output.Validate(calls[0]); err != nil {
		t.Fatalf("result validation = %v", err)
	}
}

func TestProviderChatPayloadUsesToolsInsteadOfProseControl(t *testing.T) {
	messages := []map[string]any{{"role": "user", "content": "draw a cat"}}
	payload := providerChatPayload("model", messages, 512, false, ExternalCapabilityManifests())
	if _, ok := payload["response_format"]; ok {
		t.Fatal("tool-call payload must not force JSON response_format")
	}
	if payload["tool_choice"] != "auto" {
		t.Fatalf("tool_choice = %#v", payload["tool_choice"])
	}
	tools, ok := payload["tools"].([]map[string]any)
	if !ok || len(tools) != 1 {
		t.Fatalf("tools = %#v", payload["tools"])
	}
	if payload["max_tokens"] != 512 {
		t.Fatalf("max_tokens = %#v", payload["max_tokens"])
	}

	sidecar := providerChatPayload("model", messages, 0, true, nil)
	if _, ok := sidecar["tools"]; ok {
		t.Fatal("sidecar payload must not advertise absent tools")
	}
	if _, ok := sidecar["response_format"]; !ok {
		t.Fatal("structured sidecar payload must request JSON mode")
	}
}
