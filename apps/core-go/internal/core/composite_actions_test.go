package core

import (
	"encoding/json"
	"testing"
)

func TestNormalizeCompositeActionUsesCanonicalMomentInvocation(t *testing.T) {
	invocations := testInvocations([]ToolCallV1{{ID: "image-1", Name: "media.image.generate", Arguments: json.RawMessage(`{"intent":"雨后的窗边，一张低饱和照片"}`), SourceFactID: "fact-1", ProviderRequestID: "provider-1", SchemaVersion: ToolCallSchemaVersion}})
	action, err := normalizeCompositeAction(map[string]any{
		"action_type":     "moment",
		"response_intent": "记录刚才的安静片刻",
	}, invocations, "fact-1", "moment")
	if err != nil {
		t.Fatal(err)
	}
	if action.SchemaVersion != compositeActionSchemaVersion || action.Kind != "moment" || action.ActionType != "moment" {
		t.Fatalf("action = %#v", action)
	}
	if len(action.ToolCalls) != 1 || action.ToolCalls[0].CapabilityName != "media.image.generate" {
		t.Fatalf("tool calls = %#v", action.ToolCalls)
	}
	var arguments map[string]any
	if err := json.Unmarshal(action.ToolCalls[0].Arguments, &arguments); err != nil {
		t.Fatal(err)
	}
	if stringValue(arguments["intent"]) != "雨后的窗边，一张低饱和照片" {
		t.Fatalf("media intent = %#v", arguments)
	}
	if len(action.OutputBindings) != 1 || action.OutputBindings[0].TargetKind != "moment" {
		t.Fatalf("bindings = %#v", action.OutputBindings)
	}
}

func TestNormalizeCompositeActionUsesConversationTargetForProactiveMessage(t *testing.T) {
	invocations := testInvocations([]ToolCallV1{{ID: "reply-1", Name: "conversation.reply", Arguments: json.RawMessage(`{"text":"告诉 Owner 一件事"}`), SourceFactID: "fact-2", ProviderRequestID: "provider-2", SchemaVersion: ToolCallSchemaVersion}})
	action, err := normalizeCompositeAction(map[string]any{
		"action_type":     "proactive_message",
		"response_intent": "告诉 Owner 一件事",
	}, invocations, "fact-2", "proactive_message")
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

func TestNormalizeCompositeActionMapsRespondToReply(t *testing.T) {
	action, err := normalizeCompositeAction(map[string]any{
		"action_type":  "respond",
		"visible_text": "已经整理好了。",
	}, nil, "fact-respond", "reply")
	if err != nil {
		t.Fatal(err)
	}
	if action.ActionType != "reply" || action.Kind != "conversation_turn" {
		t.Fatalf("respond action = %#v", action)
	}
}

func TestCompositeOutputValidationUsesTypedTargetKinds(t *testing.T) {
	registry, err := NewCapabilityRegistry(testCapabilityWithDefinition{definition: CapabilityDefinition{
		Name:             "calendar.event.create",
		Version:          "v1",
		Type:             CapabilityTypeAction,
		Description:      "Create a calendar event.",
		InputSchema:      map[string]any{"type": "object"},
		FailurePolicy:    FailurePolicyRequiredForVisibleClaim,
		TargetKinds:      []string{"conversation_message"},
		SideEffectClass:  "external_async",
		ConcurrencyClass: "exclusive",
	}})
	if err != nil {
		t.Fatal(err)
	}
	call := ToolCallV1{ID: "call-1", Name: "calendar.event.create", Arguments: json.RawMessage(`{"title":"demo"}`), SourceFactID: "fact-1", ProviderRequestID: "provider-1", SchemaVersion: ToolCallSchemaVersion}
	if err := validateCompositeOutputCapabilities(testInvocations([]ToolCallV1{call}), "conversation_message", registry); err != nil {
		t.Fatalf("valid typed output slot rejected: %v", err)
	}
	if err := validateCompositeOutputCapabilities(testInvocations([]ToolCallV1{call}), "moment", registry); err == nil {
		t.Fatal("expected unsupported target kind")
	}
}

func TestCompositeOutputValidationAllowsOptionalNativeCallsAlongsideOutput(t *testing.T) {
	registry, err := NewCapabilityRegistry(
		testCapabilityWithDefinition{definition: CapabilityDefinition{
			Name: "calendar.event.create", Version: "v1", Type: CapabilityTypeAction, Description: "Create a calendar event.",
			InputSchema: map[string]any{"type": "object"}, FailurePolicy: FailurePolicyRequiredForVisibleClaim, TargetKinds: []string{"conversation_message"},
			SideEffectClass: "external_async", ConcurrencyClass: "exclusive",
		}},
		testCapabilityWithDefinition{definition: CapabilityDefinition{
			Name: "affect_event", Version: "v1", Type: CapabilityTypeAction, Description: "Record an affect event.",
			InputSchema: map[string]any{"type": "object"}, FailurePolicy: FailurePolicyOptionalInternal, SideEffectClass: "state", ConcurrencyClass: "shared",
		}},
	)
	if err != nil {
		t.Fatal(err)
	}
	calls := []ToolCallV1{
		{ID: "call-output", Name: "calendar.event.create", Arguments: json.RawMessage(`{"title":"demo"}`), SourceFactID: "fact-1", ProviderRequestID: "provider-1", SchemaVersion: ToolCallSchemaVersion},
		{ID: "call-native", Name: "affect_event", Arguments: json.RawMessage(`{"event":{"type":"happy"}}`), SourceFactID: "fact-1", ProviderRequestID: "provider-2", SchemaVersion: ToolCallSchemaVersion},
	}
	if err := validateCompositeOutputCapabilities(testInvocations(calls), "conversation_message", registry); err != nil {
		t.Fatalf("optional native call rejected beside output call: %v", err)
	}
}

func TestNormalizeCompositeActionDoesNotDuplicateCanonicalMediaCall(t *testing.T) {
	action, err := normalizeCompositeAction(map[string]any{
		"action_type":          "moment",
		"moment_media_request": map[string]any{"scene": "legacy"},
	}, testInvocations([]ToolCallV1{{ID: "call-1", Name: "media.image.generate", Arguments: json.RawMessage(`{"intent":"canonical"}`), SourceFactID: "fact-3", ProviderRequestID: "provider-3", SchemaVersion: ToolCallSchemaVersion}}), "fact-3", "no_op")
	if err != nil {
		t.Fatal(err)
	}
	if len(action.ToolCalls) != 1 || action.ToolCalls[0].CallID != "call-1" {
		t.Fatalf("tool calls = %#v", action.ToolCalls)
	}
}

func TestNormalizeCompositeActionAllowsCapabilityActionWithoutOutputTarget(t *testing.T) {
	action, err := normalizeCompositeAction(map[string]any{
		"action_type": "media.image.generate",
	}, testInvocations([]ToolCallV1{{ID: "call-1", Name: "media.image.generate", Arguments: json.RawMessage(`{"intent":"a cat"}`), SourceFactID: "fact-4", ProviderRequestID: "provider-4", SchemaVersion: ToolCallSchemaVersion}}), "fact-4", "no_op")
	if err != nil {
		t.Fatal(err)
	}
	if action.Kind != "none" || action.ActionType != "media.image.generate" || len(action.OutputBindings) != 0 {
		t.Fatalf("action = %#v", action)
	}
}
