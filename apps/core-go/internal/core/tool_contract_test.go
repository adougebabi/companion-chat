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
	manifest := imageCapabilityManifest()
	if len(manifest.TargetKinds) != 2 || manifest.TargetKinds[0] != "conversation_message" || manifest.TargetKinds[1] != "moment" {
		t.Fatalf("media target kinds = %#v", manifest.TargetKinds)
	}
	if !manifest.IsDeferredOutput() || manifest.OutputSchema == nil {
		t.Fatalf("media manifest must be a typed deferred output slot: %#v", manifest)
	}
	if err := manifest.ValidateOutput(map[string]any{"media_intent_id": "intent"}); err == nil {
		t.Fatal("missing typed output target fields must be rejected")
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
	if err := registry.Register(testCapabilityExecutor{}); err == nil {
		t.Fatal("expected duplicate capability registration error")
	}
}

func TestCapabilityRequestIsAdvertisedAndKeepsReplyAction(t *testing.T) {
	manifest := capabilityRequestManifest()
	if manifest.Name != "capability.request" || manifest.Parameters == nil {
		t.Fatalf("capability request manifest = %#v", manifest)
	}
	call := ToolCallV1{ID: "need-1", Name: "capability.request", Arguments: json.RawMessage(`{"capability_key":"calendar.read","title":"读取日历","description":"需要知道日程安排","rationale":"帮助安排后续行动","desired_contract":{},"evidence_refs":["fact-1"]}`), SourceFactID: "fact-1", ProviderRequestID: "provider-1", SchemaVersion: ToolCallSchemaVersion}
	action, _, err := resolveToolCallAction([]ToolCallV1{call}, toolManifestMap([]CapabilityManifest{manifest}))
	if err != nil {
		t.Fatal(err)
	}
	if action != "reply" {
		t.Fatalf("capability request action = %q, want reply", action)
	}
}

func TestDefaultNativeCapabilitySlotsAreVersioned(t *testing.T) {
	registry := NewCapabilityRegistry(testCapabilityExecutor{})
	for _, manifest := range []CapabilityManifest{sceneCapabilityManifest(), presenceCapabilityManifest(), memoryCapabilityManifest()} {
		if err := registry.Register(testManifestExecutor{manifest: manifest}); err != nil {
			t.Fatalf("register %s = %v", manifest.Name, err)
		}
	}
	if got := len(registry.Manifests()); got != 4 {
		t.Fatalf("manifest count = %d", got)
	}
}

type testManifestExecutor struct{ manifest CapabilityManifest }

func (executor testManifestExecutor) Manifest() CapabilityManifest { return executor.manifest }
func (executor testManifestExecutor) Execute(_ context.Context, _, _, _ string, call ToolCallV1) (ToolResultV1, error) {
	return ToolResultV1{ToolCallID: call.ID, Name: call.Name, Status: "completed", SchemaVersion: ToolResultSchemaVersion}, nil
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

func TestStructuredProviderPayloadUsesJSONFormatAndCognitiveThinking(t *testing.T) {
	assessment := providerChatPayloadForRole("model", []map[string]any{{"role": "user", "content": "hello"}}, 512, true, ExternalCapabilityManifests(), "cognitive_assessment")
	if assessment["response_format"] == nil {
		t.Fatalf("cognitive assessment must request JSON output: %#v", assessment)
	}
	if assessment["enable_thinking"] != true {
		t.Fatalf("cognitive assessment must enable thinking: %#v", assessment)
	}
	reflection := providerChatPayloadForRole("model", nil, 512, true, nil, "reflection")
	if reflection["response_format"] == nil {
		t.Fatalf("structured provider must request JSON output: %#v", reflection)
	}
	if _, ok := reflection["enable_thinking"]; ok {
		t.Fatalf("non-cognitive provider must not enable thinking: %#v", reflection)
	}
	text := providerChatPayloadForRole("model", nil, 512, false, nil, "action_realization")
	if _, ok := text["response_format"]; ok {
		t.Fatalf("text realization must not force JSON output: %#v", text)
	}
}

func TestResolveToolCallActionSupportsNativeObservationSlots(t *testing.T) {
	manifests := toolManifestMap([]CapabilityManifest{sceneCapabilityManifest(), presenceCapabilityManifest(), memoryCapabilityManifest()})
	calls, err := NormalizeProviderToolCalls([]any{
		map[string]any{"id": "scene", "name": "scene_event", "arguments": map[string]any{"scene": "cafe", "activity": "read", "source_fact_id": "fact", "evidence_refs": []any{"fact"}, "confidence": 0.8}},
		map[string]any{"id": "presence", "name": "presence_event", "arguments": map[string]any{"current_task": "chat", "source_fact_id": "fact", "evidence_refs": []any{"fact"}, "confidence": 0.9}},
	}, "fact", "provider")
	if err != nil {
		t.Fatalf("normalize = %v", err)
	}
	action, concept, err := resolveToolCallAction(calls, manifests)
	if err != nil {
		t.Fatalf("resolve = %v", err)
	}
	if action != "reply" || len(concept) != 0 {
		t.Fatalf("action=%q concept=%#v", action, concept)
	}
}

func TestResolveToolCallActionKeepsMediaAsReplyComposite(t *testing.T) {
	call := ToolCallV1{
		ID: "media-1", Name: "media.image.generate",
		Arguments:    json.RawMessage(`{"concept":{"scene":"window"}}`),
		SourceFactID: "fact", ProviderRequestID: "provider", SchemaVersion: ToolCallSchemaVersion,
	}
	action, concept, err := resolveToolCallAction([]ToolCallV1{call}, toolManifestMap(ExternalCapabilityManifests()))
	if err != nil {
		t.Fatal(err)
	}
	if action != "reply" || len(concept) != 0 {
		t.Fatalf("action=%q concept=%#v; media must remain a reply composite", action, concept)
	}
}
