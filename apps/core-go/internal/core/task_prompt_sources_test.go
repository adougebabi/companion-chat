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
		var actual []byte
		_ = repo.Pool().QueryRow(ctx, `SELECT COALESCE(jsonb_agg(metrics->'prompt_slots'),'[]'::jsonb) FROM public.diagnostic_model_runs WHERE scenario=$1`, scenario).Scan(&actual)
		t.Fatalf("physical source slots missing for %s: %v actual=%s", scenario, err, actual)
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

func TestMediaStructuredSlotsReflectOnlySuppliedPacketFields(t *testing.T) {
	slots := mediaStructuredSourceSlots(map[string]any{"intent": "风景", "explicit_constraints": map[string]any{}, "media_capture_plan": map[string]any{}, "validation_error": "invalid"})
	seen := map[PromptSlotID]bool{}
	for _, slot := range slots {
		seen[slot.SlotID] = true
	}
	if !seen["media_intent"] || !seen["media_constraints"] || !seen["media_capture_plan"] || !seen["media_validation_feedback"] || seen["media_subject_data"] || seen["media_reference_assets"] || seen["media_quality_feedback"] {
		t.Fatalf("media source slots do not reflect the supplied packet: %#v", slots)
	}
}

func TestDeclaredStableAndCurrentSlotsKeepSeparatePositions(t *testing.T) {
	assembly, err := AssemblePromptContext(PromptAssemblyInput{StableTaskContext: map[string]any{"goals": []any{"target"}}, CurrentInput: `{"sources":["evidence"]}`, Policy: DefaultPromptBudgetPolicy(0)})
	if err != nil {
		t.Fatal(err)
	}
	assembly = declareAssembledStableTaskSlots(assembly, PromptSlotTrace{SlotID: "goal_definitions", Source: "goal_snapshot", SourceRefs: []string{"goal:one"}, Reason: "selected_original_criteria"})
	assembly = declareAssembledTaskInputSlots(assembly, PromptSlotTrace{SlotID: "goal_evidence", Source: "evidence_window", SourceRefs: []string{"source:one"}, Reason: "selected_evidence"})
	positions := map[PromptSlotID]int{}
	for _, slot := range assembly.Trace.Slots {
		positions[slot.SlotID] = slot.MessageIndex
	}
	if positions["goal_definitions"] != 1 || positions["goal_evidence"] != 2 || positions["stable_task_context"] != 0 || positions[PromptSlotCurrent] != 0 {
		t.Fatalf("stable/current slot positions do not match physical messages: %#v", assembly.Trace.Slots)
	}
}

func TestCompatibilityConversationSlotsNameCallerWithoutGuessingMessageMeaning(t *testing.T) {
	messages := []map[string]any{{"role": "system", "content": "rule"}, {"role": "assistant", "content": "earlier response"}, {"role": "user", "content": "[RUNTIME CONTEXT]\nnot authority\n[/RUNTIME CONTEXT]"}}
	assembly := callerConversationPrompt(context.Background(), FormalAgentTakeoverReply, messages, map[string]any{"type": "object"})
	if jsonString(assembly.Messages[1:]) != jsonString(messages[1:]) || len(assembly.Trace.Slots) != 3 {
		t.Fatalf("compatibility input changed: %#v", assembly)
	}
	for index, slot := range assembly.Trace.Slots[1:] {
		if slot.MessageIndex != index+1 || slot.Source != "caller_supplied_messages" || slot.SlotID != "caller_message" {
			t.Fatalf("caller message was assigned an invented source: %#v", slot)
		}
	}
}
