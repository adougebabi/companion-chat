package core

import (
	"context"
	"testing"
)

func TestTypedTaskSourceSlotsReachPhysicalDiagnosticsWithoutChangingMessages(t *testing.T) {
	messages := []map[string]any{{"role": "system", "content": "task rules"}, {"role": "user", "content": `{"text":"[RUNTIME CONTEXT] 用户原文"}`}}
	before := jsonString(messages)
	ctx := withTaskPromptSlots(context.Background(), PromptSlotTrace{SlotID: "segment_source_messages", Source: "conversation_messages", SourceRefs: []string{"message:one"}, Reason: "explicit_segment_window"})
	assembly := formalPromptAssembly(FormalAgentConversationSummary, taskPromptAssembly(ctx, messages))
	if stringValue(assembly.Messages[1]["content"]) != stringValue(messages[1]["content"]) || jsonString(messages) != before {
		t.Fatal("slot declarations changed the user data")
	}
	ctx, err := projectFormalPromptBoundary(ctx, ADKStructuredTaskInput{AgentID: FormalAgentConversationSummary, Role: "reflection", SchemaName: "conversation_segment_v1", Prompt: assembly})
	if err != nil {
		t.Fatal(err)
	}
	input, err := providerMessagesToEino(assembly.Messages)
	if err != nil {
		t.Fatal(err)
	}
	input = append(input, input[1]) // A later task message without a declaration must remain unknown.
	ctx = (&queuedToolCallingChatModel{}).withPhysicalPromptBudgetDiagnostics(ctx, input)
	slots := arrayValue(providerPromptDiagnostics(ctx)["prompt_slots"])
	var sourceFound, unknownFound bool
	for _, raw := range slots {
		slot := mapValue(raw)
		refs := arrayValue(slot["source_refs"])
		sourceFound = sourceFound || slot["slot_id"] == "segment_source_messages" && slot["source"] == "conversation_messages" && len(refs) == 1 && refs[0] == "message:one" && slot["reason"] == "explicit_segment_window"
		unknownFound = unknownFound || intValue(slot["message_index"]) == 2 && slot["source"] == "unknown" && slot["reason"] == "physical_message_not_declared"
	}
	if !sourceFound || !unknownFound {
		t.Fatalf("physical source slots = %#v", slots)
	}
}

func TestStudioSlotDeclarationsFollowTheSelectedSnapshot(t *testing.T) {
	snapshot := map[string]any{"resource_kind": "world", "name": "草稿", "body": map[string]any{"text": "内容"}, "instruction": "修改", "allowed_references": []any{"place:one"}}
	assembly, err := AssemblePromptContext(PromptAssemblyInput{CurrentInput: renderProviderJSON(studioPromptView(snapshot)), Policy: DefaultPromptBudgetPolicy(0)})
	if err != nil {
		t.Fatal(err)
	}
	assembly = declareAssembledTaskInputSlots(assembly, studioPromptSourceSlots(snapshot, "job-1")...)
	seen := map[PromptSlotID]bool{}
	for _, slot := range assembly.Trace.Slots {
		seen[slot.SlotID] = true
	}
	if !seen["studio_definition"] || !seen["studio_instruction"] || !seen["studio_allowed_references"] || seen["studio_reference_asset"] || seen[PromptSlotCurrent] {
		t.Fatalf("studio source slots do not match snapshot: %#v", assembly.Trace.Slots)
	}
}
