package core

import (
	"context"
	"testing"
)

func requireTaskSourceSlots(t *testing.T, ctx context.Context, repo *PostgresRepository, scenario string, ids ...string) {
	t.Helper()
	expected := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		expected = append(expected, map[string]any{"slot_id": id})
	}
	var found bool
	if err := repo.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.diagnostic_model_runs WHERE scenario=$1 AND metrics->'prompt_slots' @> $2::jsonb)`, scenario, jsonBytes(expected)).Scan(&found); err != nil || !found {
		t.Fatalf("physical source slots missing for %s: %v", scenario, err)
	}
}

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

func TestTypedTaskRepairSlotsKeepTheirMessagePositions(t *testing.T) {
	messages := []map[string]any{{"role": "system", "content": "task"}, {"role": "user", "content": "original source"}, {"role": "assistant", "content": "previous candidate"}, {"role": "user", "content": "validation feedback"}}
	ctx := withTaskPromptSlots(context.Background(),
		PromptSlotTrace{SlotID: "persona_compilation_source", Source: "selected_persona_profile", Reason: "compile_authorized_profile"},
		PromptSlotTrace{SlotID: "persona_previous_output", Position: PromptSlotRecent, MessageIndex: 2, Source: "previous_model_output", Reason: "validation_repair"},
		PromptSlotTrace{SlotID: "persona_validation_feedback", MessageIndex: 3, Source: "core_validation", Reason: "validation_repair"},
	)
	assembly := formalPromptAssembly(FormalAgentPersonaCompilation, taskPromptAssembly(ctx, messages))
	positions := map[PromptSlotID]int{}
	for _, slot := range assembly.Trace.Slots {
		positions[slot.SlotID] = slot.MessageIndex
	}
	if positions["persona_compilation_source"] != 1 || positions["persona_previous_output"] != 2 || positions["persona_validation_feedback"] != 3 || jsonString(assembly.Messages[1:]) != jsonString(messages[1:]) {
		t.Fatalf("repair slot positions or messages changed: %#v", assembly.Trace.Slots)
	}
}
