package core

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestFactOnlyRegistryViewIsFixedWithoutMutatingLegacy(t *testing.T) {
	base, err := NewCapabilityRegistry(builtinCapabilities(&App{})...)
	if err != nil {
		t.Fatal(err)
	}
	view, err := capabilityRegistryView(base, factOnlyReplyCapability{})
	if err != nil {
		t.Fatal(err)
	}
	legacy, _ := base.Definition(conversationReplyCapabilityName)
	fact, _ := view.Definition(conversationReplyCapabilityName)
	if !strings.Contains(jsonString(legacy.InputSchema), `"text"`) || strings.Contains(jsonString(fact.InputSchema), `"text"`) {
		t.Fatal("legacy/fact-only schemas conflated")
	}
	ctx := withCapabilityRunRegistry(context.Background(), view)
	app := &App{Capabilities: base}
	if err := validateADKCapabilityDefinitionsForContext(ctx, app, []CapabilityDefinition{fact}, CapabilitySurfaceConversation); err != nil {
		t.Fatal(err)
	}
	if err := validateADKCapabilityDefinitions(app, []CapabilityDefinition{fact}, CapabilitySurfaceConversation); err == nil {
		t.Fatal("foreign run contract accepted by legacy registry")
	}
	for _, args := range []string{`{"text":"Qwen正文"}`, `{"direction":"等待用户主动"}`, `{"tone":"温柔"}`, `{"constraints":"暂时不推进目标"}`} {
		if err := validateNativeArguments(json.RawMessage(args), fact); err == nil {
			t.Errorf("social instruction accepted: %s", args)
		}
	}
	if err := validateNativeArguments(json.RawMessage(`{}`), fact); err != nil {
		t.Fatal(err)
	}
	result := CapabilityResult{Status: "completed", Output: map[string]any{"text": "Cydonia的实际正文", "target_ref": "private-message-id", "target_kind": "conversation_message", "reply_mode": "fact_only", "reply_status": "committed"}}
	visible := modelFacingToolResult(ToolExecutionReceipt{Result: result}, fact)
	if !strings.Contains(jsonString(visible), "Cydonia的实际正文") || strings.Contains(jsonString(visible), "private-message-id") {
		t.Fatalf("real text feedback/privacy lost: %#v", visible)
	}
	catalog := capabilityCatalogCapability{catalog: base.Catalog}
	receipt, err := catalog.Execute(ctx, CapabilityInvocation{CallID: "catalog-test", CapabilityName: capabilityCatalogName, Metadata: InvocationMetadata{Surface: CapabilitySurfaceConversation}}, CapabilityContext{})
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range arrayValue(mapValue(receipt.Output)["items"]) {
		item := mapValue(raw)
		if item["name"] == conversationReplyCapabilityName && strings.Contains(jsonString(item["parameters"]), `"text"`) {
			t.Fatal("discovery returned the other run's legacy schema")
		}
	}
}

func TestReplyLogicalIdentityUsesFixedRunWhenNoInboundFact(t *testing.T) {
	run := &privateReplyRun{Scope: ToolExecutionRequest{RunID: "fixed-independent-run"}}
	ctx := context.WithValue(context.Background(), privateReplyRunKey{}, run)
	inv := CapabilityInvocation{Metadata: InvocationMetadata{FluctlightID: "self", ConversationID: "direct", AuthorizationActorID: "owner", OperationID: "native-position-one"}}
	first := privateReplyLogicalKey(ctx, inv)
	inv.Metadata.OperationID = "native-position-two"
	if privateReplyLogicalKey(ctx, inv) != first {
		t.Fatal("same run received a different publication identity")
	}
	inv.SourceFactID = "inbound-one"
	one := privateReplyLogicalKey(ctx, inv)
	inv.SourceFactID = "inbound-two"
	if privateReplyLogicalKey(ctx, inv) == one {
		t.Fatal("different inbound facts shared a reply identity")
	}
}
