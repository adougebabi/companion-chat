package core

import (
	"encoding/json"
	"testing"
)

func TestNormalizeCompositeActionAdaptsLegacyMomentMedia(t *testing.T) {
	action, err := normalizeCompositeAction(map[string]any{
		"action_type":          "moment",
		"response_intent":      "记录刚才的安静片刻",
		"moment_media_request": map[string]any{"scene": "雨后的窗边", "style": "低饱和"},
	}, nil, "fact-1", "no_op")
	if err != nil {
		t.Fatal(err)
	}
	if action.SchemaVersion != compositeActionSchemaVersion || action.Kind != "moment" || action.ActionType != "moment" {
		t.Fatalf("action = %#v", action)
	}
	if len(action.ToolCalls) != 1 || action.ToolCalls[0].Name != "media.image.generate" {
		t.Fatalf("tool calls = %#v", action.ToolCalls)
	}
	var arguments map[string]any
	if err := json.Unmarshal(action.ToolCalls[0].Arguments, &arguments); err != nil {
		t.Fatal(err)
	}
	if media := mapValue(arguments["concept"]); media["scene"] != "雨后的窗边" {
		t.Fatalf("media concept = %#v", media)
	}
	if len(action.OutputBindings) != 1 || action.OutputBindings[0].TargetKind != "moment" {
		t.Fatalf("bindings = %#v", action.OutputBindings)
	}
}

func TestNormalizeCompositeActionUsesConversationTargetForProactiveMessage(t *testing.T) {
	action, err := normalizeCompositeAction(map[string]any{
		"action_type":          "proactive_message",
		"response_intent":      "告诉 Owner 一件事",
		"moment_media_request": map[string]any{"subject": "一张照片"},
	}, nil, "fact-2", "no_op")
	if err != nil {
		t.Fatal(err)
	}
	if action.Kind != "conversation_turn" || len(action.ToolCalls) != 1 || len(action.OutputBindings) != 1 {
		t.Fatalf("action = %#v", action)
	}
	if action.OutputBindings[0].TargetKind != "conversation_message" {
		t.Fatalf("binding = %#v", action.OutputBindings[0])
	}
}

func TestCompositeOutputValidationUsesTypedTargetKinds(t *testing.T) {
	registry := NewCapabilityRegistry(testManifestExecutor{manifest: CapabilityManifest{
		Name:             "calendar.event.create",
		Version:          "v1",
		Description:      "Create a calendar event.",
		Parameters:       map[string]any{"type": "object"},
		TargetKinds:      []string{"conversation_message"},
		SideEffectClass:  "external_async",
		ConcurrencyClass: "exclusive",
	}})
	call := ToolCallV1{ID: "call-1", Name: "calendar.event.create", Arguments: json.RawMessage(`{"title":"demo"}`), SourceFactID: "fact-1", ProviderRequestID: "provider-1", SchemaVersion: ToolCallSchemaVersion}
	if err := validateCompositeOutputCalls([]ToolCallV1{call}, "conversation_message", registry); err != nil {
		t.Fatalf("valid typed output slot rejected: %v", err)
	}
	if err := validateCompositeOutputCalls([]ToolCallV1{call}, "moment", registry); err == nil {
		t.Fatal("expected unsupported target kind")
	}
}

func TestNormalizeCompositeActionDoesNotDuplicateCanonicalMediaCall(t *testing.T) {
	action, err := normalizeCompositeAction(map[string]any{
		"action_type":          "moment",
		"moment_media_request": map[string]any{"scene": "legacy"},
	}, []ToolCallV1{{ID: "call-1", Name: "media.image.generate", Arguments: json.RawMessage(`{"concept":{"scene":"canonical"}}`)}}, "fact-3", "no_op")
	if err != nil {
		t.Fatal(err)
	}
	if len(action.ToolCalls) != 1 || action.ToolCalls[0].ID != "call-1" {
		t.Fatalf("tool calls = %#v", action.ToolCalls)
	}
}

func TestNormalizeCompositeActionAllowsCapabilityActionWithoutOutputTarget(t *testing.T) {
	action, err := normalizeCompositeAction(map[string]any{
		"action_type": "media.image.generate",
	}, []ToolCallV1{{ID: "call-1", Name: "media.image.generate", Arguments: json.RawMessage(`{"concept":{"subject":"a cat"}}`)}}, "fact-4", "no_op")
	if err != nil {
		t.Fatal(err)
	}
	if action.Kind != "none" || action.ActionType != "media.image.generate" || len(action.OutputBindings) != 0 {
		t.Fatalf("action = %#v", action)
	}
}
