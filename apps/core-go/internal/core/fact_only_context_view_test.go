package core

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
)

func morningReplyFixture(t *testing.T) ReplyContextInput {
	t.Helper()
	manifestData, err := os.ReadFile("testdata/fact_only_context/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Hashes map[string]string `json:"sha256"`
	}
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		t.Fatal(err)
	}
	for name, hash := range manifest.Hashes {
		data, err := os.ReadFile("testdata/fact_only_context/" + name)
		if err != nil {
			t.Fatal(err)
		}
		if fmt.Sprintf("%x", sha256.Sum256(data)) != hash {
			t.Fatalf("frozen synthetic fixture changed: %s", name)
		}
	}
	data, err := os.ReadFile("testdata/fact_only_context/reported_morning.synthetic.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Persona      map[string]any    `json:"working_persona"`
		CurrentInput string            `json:"current_input"`
		Projection   ContextProjection `json:"projection"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	return ReplyContextInput{WorkingPersona: fixture.Persona, CurrentInput: fixture.CurrentInput, Projection: fixture.Projection,
		CapabilityFacts: map[string]any{"image_generation_registered": true, "vision_understanding": "disabled"}}
}

func TestFactOnlyViewsPreserveSourceSemanticsAndScope(t *testing.T) {
	input := morningReplyFixture(t)
	before := jsonString(input)
	view := factOnlyMemoryCandidates(input.Projection, nil, nil, false)
	if len(view.Input.RetrievedMemories) != 1 || intValue(view.Diagnostics["deduplicated_count"]) != 1 {
		t.Fatal("duplicate source inflated candidates", view.Diagnostics)
	}
	value := mapValue(view.Input.RetrievedMemories[0].Content)
	if value["epistemic_kind"] != "inference" || value["provenance_status"] != "verified" {
		t.Fatal("provenance and epistemic kind conflated", value)
	}
	if sources := jsonString(view.Diagnostics["sources"]); !strings.Contains(sources, "test-memory-1") || !strings.Contains(sources, "test-memory-2") {
		t.Fatal("source pointers lost", sources)
	}
	for _, change := range []map[string]any{{"occurred_at": "2026-10-12T10:00:00.000+08:00"}, {"actor_refs": []any{"another-actor"}}, {"epistemic_kind": "user_statement"}} {
		copyProjection := input.Projection
		row := cloneMap(copyProjection.Memories[0])
		for key, value := range change {
			row[key] = value
		}
		copyProjection.Memories = append(append([]map[string]any(nil), copyProjection.Memories...), row)
		if other := factOnlyMemoryCandidates(copyProjection, nil, nil, false); len(other.Input.RetrievedMemories) != 2 {
			t.Fatal("different event, Actor or kind merged", change)
		}
	}
	row := cloneMap(input.Projection.Memories[0])
	row["content"] = "同一来源的另一种说法，不能裁定哪一个成立。"
	input.Projection.Memories = append(input.Projection.Memories, row)
	conflicting := factOnlyMemoryCandidates(input.Projection, nil, nil, false)
	if len(conflicting.Conflicts) != 1 || len(conflicting.Input.RetrievedMemories) != 2 {
		t.Fatal("shared source swallowed distinct claim", conflicting)
	}
	input.Projection.Memories = input.Projection.Memories[:2]
	if before != jsonString(input) {
		t.Fatal("projection mutated")
	}
}

func TestFactOnlyStateExpiryIsFieldLocal(t *testing.T) {
	input := morningReplyFixture(t)
	input.Projection.EffectiveAppearance = map[string]any{"body_fields": map[string]any{"unknown": map[string]any{"status": "unknown"}, "cleared": map[string]any{"status": "cleared"}, "flag": map[string]any{"status": "known", "value": false}}}
	input.Projection.ActiveActivities = []map[string]any{{"status": "in_progress", "kind": "reading", "active_until": "2026-10-14T12:00:00.000+08:00"}}
	for _, cognition := range []bool{false, true} {
		morning := factOnlyCurrentState(input.Projection, cognition)
		if !strings.Contains(jsonString(morning), "上午阅读") || len(arrayValue(mapValue(morning["data"])["active_activities"])) != 1 {
			t.Fatal("morning validity lost", morning)
		}
		later := input.Projection
		later.AsOf = "2026-10-14T12:00:00.000+08:00"
		state := factOnlyCurrentState(later, cognition)
		data := mapValue(state["data"])
		if strings.Contains(jsonString(state), "上午阅读") || len(arrayValue(data["active_activities"])) != 0 {
			t.Fatal("expired activity still current", state)
		}
		fields := mapValue(mapValue(data["appearance"])["body_fields"])
		if mapValue(fields["unknown"])["status"] != "unknown" || mapValue(fields["cleared"])["status"] != "cleared" || mapValue(fields["flag"])["value"] != false {
			t.Fatal("expiry erased unrelated state/false", fields)
		}
	}
}

func TestFactOnlyGoalsKeepOriginalConditionsAndRealDependencies(t *testing.T) {
	input := morningReplyFixture(t)
	input.Projection.Goals[0]["execution"].(map[string]any)["dependency_snapshot"] = map[string]any{"prerequisite_goals_completed": false, "context_review_required": true}
	input.Projection.Goals[0]["execution"].(map[string]any)["last_result"] = map[string]any{"status": "failed", "capability": "wardrobe.wear", "error_code": "item_not_owned", "reason": "LEAK_REASON"}
	input.Projection.Goals = append(input.Projection.Goals, map[string]any{"description": "shared", "status": "active", "success_criteria": []any{"SHARED_ORIGINAL"}}, map[string]any{"description": "finished", "status": "completed", "execution": map[string]any{"next_step": "LEAK_COMPLETED"}})
	before := jsonString(input)
	for _, cognition := range []bool{false, true} {
		view := factOnlyContextFacts(input.Projection, cognition)
		wire := jsonString(view.Facts)
		for _, text := range []string{"双方明确表达理解", "SHARED_ORIGINAL", "item_not_owned", "prerequisite_goals_completed", "finished"} {
			if !strings.Contains(wire, text) {
				t.Errorf("required condition lost: %s", text)
			}
		}
		if strings.Contains(wire, "LEAK_") || strings.Contains(wire, "HIDDEN_OTHER_PROFILE") {
			t.Fatal("strategy/profile boundary failed", wire)
		}
		if intValue(view.Selection["goal_visible_active_count"]) != 2 {
			t.Fatal("visible/global counts confused", view.Selection)
		}
	}
	if before != jsonString(input) {
		t.Fatal("Goal source mutated")
	}
}

func TestFactOnlyHistoryMetadataMatchesSelectedRoles(t *testing.T) {
	input := morningReplyFixture(t)
	prompt, err := buildFactOnlyReplyContext(input, providerAssignment{TokenBudget: 1024})
	if err != nil {
		t.Fatal(err)
	}
	history := prompt.Messages[2 : len(prompt.Messages)-1]
	if len(history) != 3 || history[0]["role"] != "user" || history[1]["role"] != "assistant" || history[2]["role"] != "assistant" {
		t.Fatal("real history roles lost", history)
	}
	for index, message := range history {
		if message["metadata"] != nil || strings.HasPrefix(stringValue(message["content"]), "[sender=") {
			t.Fatal("metadata became original text", message)
		}
		if stringValue(message["content"]) != stringValue(input.Projection.RecentMessages[index]["text"]) {
			t.Fatal("original message rewritten")
		}
	}
	if strings.Contains(jsonString(prompt.Messages), "display_name: actor_user") {
		t.Fatal("internal alias used as nickname")
	}
}

func TestFactOnlyScheduleUsesSnapshotClockAndMinimalWindow(t *testing.T) {
	input := morningReplyFixture(t)
	input.Projection.Schedule = map[string]any{"timezone": "Asia/Shanghai", "items": []any{
		map[string]any{"start_at": "2026-10-14T09:00:00+08:00", "end_at": "2026-10-14T12:00:00+08:00", "activity": "CURRENT_CONFIRMED_PLAN"},
		map[string]any{"start_at": "2026-10-14T12:00:00+08:00", "end_at": "2026-10-14T13:00:00+08:00", "activity": "NEXT_CONFIRMED_PLAN"},
		map[string]any{"start_at": "2026-10-14T18:00:00+08:00", "end_at": "2026-10-14T19:00:00+08:00", "activity": "UNNEEDED_FULL_DAY"},
	}}
	value := factOnlySchedule(input.Projection, false)
	wire := jsonString(value)
	if !strings.Contains(wire, "CURRENT_CONFIRMED_PLAN") || !strings.Contains(wire, "NEXT_CONFIRMED_PLAN") || strings.Contains(wire, "UNNEEDED_FULL_DAY") {
		t.Fatal("schedule window invalid", wire)
	}
}

func TestFactOnlyNativeArgumentsAndServerPurpose(t *testing.T) {
	definition := (factOnlyReplyCapability{}).Definition()
	for _, args := range []map[string]any{{"purpose": "先道歉"}, {"reply_intent": "暂不推进"}, {"direction": "先道歉"}, {"evidence_refs": []any{map[string]any{"tone": "温柔"}}}, {"draft_ref": map[string]any{"strategy": "等待"}}} {
		if err := validateCapabilitySchemaValue(args, definition.InputSchema); err == nil {
			t.Fatalf("strategy parameters accepted: %v", args)
		}
	}
	for _, args := range []map[string]any{{}, {"topic_key": "relationship", "evidence_refs": []any{}, "draft_ref": "draft", "action_result_refs": []any{"result"}}} {
		if err := validateCapabilitySchemaValue(args, definition.InputSchema); err != nil {
			t.Fatal(err)
		}
	}
	legacy := conversationReplyCapabilityDefinition()
	if err := validateCapabilitySchemaValue(map[string]any{"text": "legacy text", "purpose": "legacy purpose"}, legacy.InputSchema); err != nil {
		t.Fatal("legacy changed", err)
	}
	run := &privateReplyRun{}
	metadata := replyExecutionScope(run, CapabilityInvocation{Arguments: jsonBytes(map[string]any{"topic_key": "topic", "purpose": "LEAK_PURPOSE"})})
	if metadata["purpose"] != factOnlyReplyPublicationPurpose || strings.Contains(jsonString(metadata), "LEAK_PURPOSE") {
		t.Fatal("model strategy used as service purpose", metadata)
	}
}

func TestFactOnlyPhaseProtocolsDoNotMutateMessages(t *testing.T) {
	input := []*schema.Message{schema.SystemMessage(factOnlyCognitionProtocol + "\n" + factOnlyConversationPolicy + "\n# 人格设定\nPERSONA_MUST_SURVIVE"), schema.UserMessage("CURRENT_INPUT")}
	before := jsonString(input)
	trace := &ADKCapabilityTrace{Results: []CapabilityResult{{CapabilityName: conversationReplyCapabilityName, Status: "accepted", Output: map[string]any{"reply_status": "waiting", "draft_ref": "draft"}}}}
	ctx := withADKCapabilityContext(context.Background(), conversationPublicationTestInvoker{trace: trace}, trace, nil)
	if factOnlyRequestPhase(ctx) != "cognition" {
		t.Fatal("waiting media misclassified as published")
	}
	trace.Results = []CapabilityResult{{CapabilityName: conversationReplyCapabilityName, Status: "completed", Output: map[string]any{"target_kind": "conversation_message", "target_ref": "committed", "text": "REAL_COMMITTED"}}}
	for _, phaseCtx := range []context.Context{ctx, finalRepairContext(ctx)} {
		messages := factOnlyPhaseInput(phaseCtx, input)
		if strings.Contains(messages[0].Content, "普通私聊必须真实调用") || !strings.Contains(messages[0].Content, "PERSONA_MUST_SURVIVE") || messages[1].Content != "CURRENT_INPUT" {
			t.Fatal("phase protocol/context lost", messages)
		}
	}
	if before != jsonString(input) {
		t.Fatal("original Eino history mutated")
	}
}

func TestFactOnlyFixtureReplayEstimates(t *testing.T) {
	input := morningReplyFixture(t)
	after, err := buildFactOnlyReplyContext(input, providerAssignment{TokenBudget: 1024})
	if err != nil {
		t.Fatal(err)
	}
	working := factOnlyCognitionMemoryInput(input.Projection, nil, nil)
	wm, err := ResolveWorkingMemory(working, DefaultWorkingMemoryPolicy())
	if err != nil {
		t.Fatal(err)
	}
	cognition, err := AssemblePromptContext(PromptAssemblyInput{Surface: ProviderContextSurfaceFactOnlyCognition, Role: "cognitive_assessment", OperationRules: []string{factOnlyExecutionBoundary}, CorePersona: input.WorkingPersona, CurrentInput: input.CurrentInput, WorkingMemory: wm, Tools: RenderCapabilityTools([]CapabilityDefinition{(factOnlyReplyCapability{}).Definition()}), ResponseFormat: providerResponseFormatForSchema("cognitive_assessment", "conversation_turn_response", cognitiveTurnResponseSchema()), Policy: DefaultPromptBudgetPolicy(1024)})
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, baseline, output string
		after                  PromptAssemblyResult
	}{
		{"Writer", "before.synthetic.json", "reply-after.synthetic.json", after},
		{"Cognition", "cognition-before-context.synthetic.json", "cognition-after.synthetic.json", cognition},
	} {
		data, err := os.ReadFile("testdata/fact_only_context/" + test.baseline)
		if err != nil {
			t.Fatal(err)
		}
		var before PromptAssemblyResult
		if err := json.Unmarshal(data, &before); err != nil {
			t.Fatal(err)
		}
		t.Logf("synthetic %s context-only comparison: estimated tokens %d -> %d; wire chars %d -> %d; provider actual usage and latency NOT_RUN", test.name, before.Trace.EstimatedInputTokens, test.after.Trace.EstimatedInputTokens, before.Trace.WireChars, test.after.Trace.WireChars)
		if directory := os.Getenv("FLUCTLIGHT_CONTEXT_REPLAY_OUTPUT"); directory != "" {
			if err := os.MkdirAll(directory, 0755); err != nil {
				t.Fatal(err)
			}
			data, err := json.MarshalIndent(test.after, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(directory+"/"+test.output, append(data, '\n'), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestFactOnlyCognitionViewIsIndependentFromLegacy(t *testing.T) {
	input := morningReplyFixture(t)
	before := jsonString(input.Projection)
	view := workingMemoryInputFromProjectionForSurface(input.Projection, ProviderContextSurfaceFactOnlyCognition, nil, nil)
	wire := jsonString(view)
	if strings.Contains(wire, "LEAK_") || strings.Contains(wire, "HIDDEN_OTHER_PROFILE") || strings.Contains(wire, "[sender=") {
		t.Fatal("cognition kept strategy or rendered metadata", wire)
	}
	for _, required := range []string{"双方明确表达理解", "inference", "verified", "09:10:17.690", "09:29:22.688"} {
		if !strings.Contains(wire, required) {
			t.Error("cognition lost required semantics", required)
		}
	}
	legacy := workingMemoryInputFromProjectionForSurface(input.Projection, ProviderContextSurfaceConversationMain, nil, nil)
	if !strings.Contains(jsonString(legacy), "LEAK_NEXT_STEP") {
		t.Fatal("new projection changed legacy assembly")
	}
	if before != jsonString(input.Projection) {
		t.Fatal("shared projection mutated")
	}
}

func TestFactOnlyRequiredHistoryOverflowIsExplicit(t *testing.T) {
	input := morningReplyFixture(t)
	input.Projection.RecentMessages[0]["text"] = strings.Repeat("原始触发内容不可静默截断", 5000)
	result, err := buildFactOnlyReplyContext(input, providerAssignment{TokenBudget: 1024, ContextWindowTokens: 16384, MaxInputTokens: 8192, PromptBudgetPolicyVersion: promptBudgetPolicyVersionV1})
	if err != nil || !strings.Contains(jsonString(result.Messages), strings.Repeat("原始触发内容不可静默截断", 5000)) {
		t.Fatal("protected triggering user silently truncated", err)
	}
}

func TestPhysicalRequestConfigurationExcludesPrivatePayload(t *testing.T) {
	config := physicalRequestConfiguration(map[string]any{"model": "controlled", "temperature": 0.7, "top_p": 0.9, "tools": []any{map[string]any{"function": map[string]any{"name": "reply"}}}, "response_format": map[string]any{"type": "json_schema", "json_schema": map[string]any{"name": "PRIVATE_SCHEMA_TEXT"}}, "messages": []any{"PRIVATE_MESSAGE"}, "api_key": "PRIVATE_SECRET", "reasoning_content": "PRIVATE_REASONING"})
	if strings.Contains(jsonString(config), "PRIVATE_") || intValue(config["tool_count"]) != 1 || config["response_format_present"] != true || config["temperature"] != 0.7 {
		t.Fatal("effective config either leaks input or misses actual parameters", config)
	}
	if physicalRequestModuleEstimates(nil)["available"] != false {
		t.Fatal("missing estimates reported as measured zero")
	}
}

func TestFactOnlyEvidenceSnapshotsCannotReintroduceStrategy(t *testing.T) {
	input := morningReplyFixture(t)
	entry := ContextReference{Kind: ContextReferenceGoal, Snapshot: jsonBytes(input.Projection.Goals[0])}
	if factOnlyReplyEvidenceFact(entry, input.Projection) != nil {
		t.Fatal("Goal snapshot bypassed the dedicated Goal view")
	}
	input.ToolFacts = []map[string]any{{"kind": "goal", "value": input.Projection.Goals[0]}, {"kind": "renamed_social_advice", "value": map[string]any{"apparently_factual_next_action": "LEAK_RENAMED"}}}
	prompt, err := buildFactOnlyReplyContext(input, providerAssignment{TokenBudget: 1024})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(jsonString(prompt.Messages), "LEAK_") {
		t.Fatal("extra reference/snapshot path bypassed Writer projection")
	}
	entry = ContextReference{Kind: ContextReferenceOutcome, Snapshot: jsonBytes(map[string]any{"status": "failed", "capability_name": "media.image.generate", "error_code": "tool_not_selected", "renamed_guidance": "LEAK_RENAMED"})}
	fact := factOnlyReplyEvidenceFact(entry, input.Projection)
	if !strings.Contains(jsonString(fact), "tool_not_selected") || strings.Contains(jsonString(fact), "LEAK_") {
		t.Fatal("actual Tool status lost or freeform advice retained", fact)
	}
}

func TestFactOnlyValidityDoesNotPromoteFutureOrInactiveMemory(t *testing.T) {
	input := morningReplyFixture(t)
	for _, change := range []map[string]any{{"status": "superseded"}, {"valid_until": "2026-10-14T09:00:00+08:00"}, {"valid_from": "2026-10-15T09:00:00+08:00"}} {
		row := cloneMap(input.Projection.Memories[0])
		for key, value := range change {
			row[key] = value
		}
		projection := input.Projection
		projection.Memories = []map[string]any{row}
		if view := factOnlyMemoryCandidates(projection, nil, nil, false); len(view.Input.RetrievedMemories) != 0 {
			t.Fatal("inactive/future/expired candidate retained", change)
		}
	}
	input.Projection.Presence = map[string]any{"current_task": "INDEPENDENT_VALID_PRESENCE", "expires_at": "2026-10-14T15:00:00+08:00"}
	input.Projection.AsOf = "2026-10-14T13:00:00+08:00"
	if state := factOnlyCurrentState(input.Projection, false); !strings.Contains(jsonString(state), "INDEPENDENT_VALID_PRESENCE") || strings.Contains(jsonString(state), "上午阅读") {
		t.Fatal("one source expiry erased independent current presence", state)
	}
}

func TestFactOnlySenderTimestampKeepsItsOriginalZone(t *testing.T) {
	input := morningReplyFixture(t)
	message := input.Projection.RecentMessages[0]
	message["sender_timezone"] = "Europe/Paris"
	message["sender_utc_offset_minutes"] = 120
	message["sender_sent_at"] = "2026-10-14T03:10:00+02:00"
	prompt, err := buildFactOnlyReplyContext(input, providerAssignment{TokenBudget: 1024})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(jsonString(prompt.Messages), "2026-10-14T03:10:00+02:00") {
		t.Fatal("sender snapshot relabeled into self display timezone")
	}
}

func TestFactOnlyMorningContextRegression(t *testing.T) {
	input := morningReplyFixture(t)
	before := jsonString(input)
	prompt, err := buildFactOnlyReplyContext(input, providerAssignment{TokenBudget: 1024})
	if err != nil {
		t.Fatal(err)
	}

	wire := jsonString(prompt.Messages)
	for _, text := range []string{"LEAK_NEXT_STEP", "LEAK_STRATEGY", "LEAK_WAIT", "LEAK_EVALUATION", "HIDDEN_OTHER_PROFILE"} {
		if strings.Contains(wire, text) {
			t.Errorf("strategy or unauthorized Goal leaked: %s", text)
		}
	}
	for _, text := range []string{"上午阅读", "双方明确表达理解", "09:10:17.690", "09:29:22.688", "# 2026-10-14 23:58", "inference"} {
		if !strings.Contains(wire, text) {
			t.Errorf("required context missing: %s", text)
		}
	}
	if strings.Count(wire, input.CurrentInput) != 1 || before != jsonString(input) {
		t.Fatal("current input/source immutability lost")
	}
}

func TestFactOnlyProductionActorShapeKeepsRecipientRelationship(t *testing.T) {
	input := morningReplyFixture(t)
	if input.Projection.CurrentSpeaker["actor_id"] != "test-user" || input.Projection.CurrentSpeaker["id"] != nil {
		t.Fatal("fixture must match buildActorProjection")
	}
	for _, cognition := range []bool{false, true} {
		view := factOnlyContextFacts(input.Projection, cognition)
		wire := jsonString(view.Facts)
		if !strings.Contains(wire, "CURRENT_RECIPIENT_RELATIONSHIP") || strings.Contains(wire, "HIDDEN_OTHER_RECIPIENT") {
			t.Fatal("recipient relationship dropped/misattributed", wire)
		}
	}
}

func TestFactOnlyConsecutiveRepliesKeepTheirTrigger(t *testing.T) {
	input := morningReplyFixture(t)
	input.Projection.RecentMessages[0]["text"] = strings.Repeat("真实触发用户内容", 300)
	prompt, err := buildFactOnlyReplyContext(input, providerAssignment{TokenBudget: 1024})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(jsonString(prompt.Messages), "真实触发用户内容") {
		t.Fatal("protected assistant tail lost its triggering user")
	}
}
