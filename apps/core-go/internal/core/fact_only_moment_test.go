package core

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestFactOnlyMomentUsesSameFrozenWriterWithFeedScene(t *testing.T) {
	input := replyContextFixture()
	input.Scene = replySceneMoment
	input.CurrentInput = "为本次已授权的动态发布生成面向参与者的正文。"
	definition := (factOnlyMomentCapability{}).Definition()
	if err := validateNativeArguments(json.RawMessage(`{"text":"父模型代写正文"}`), definition); err == nil {
		t.Fatal("parent text accepted")
	}
	if err := validateNativeArguments(json.RawMessage(`{}`), definition); err != nil {
		t.Fatal(err)
	}
	if len(replyWriterDefinitions(input)) != 0 {
		t.Fatal("feed Writer inherited private image handoff")
	}
	calls := 0
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		calls++
		wire := jsonString(payload["messages"])
		if payload["model"] != "shared-persona-writer" || len(arrayValue(payload["tools"])) != 0 || !strings.Contains(wire, "面向动态参与者") || strings.Contains(wire, "一次真实私聊回复") || strings.Contains(wire, "父模型隐含计划") {
			t.Errorf("incorrect Writer binding/scene: %s", wire)
		}
		return fakeProviderResult{Text: "今天读到一个值得记下的段落。\n\n\n留作稍后的讨论。"}
	})
	app := &App{Provider: &ProviderClient{HTTP: &http.Client{Transport: router}}}
	ctx := WithPromptDiagnostics(context.Background(), map[string]any{"response_plan": "父模型隐含计划"})
	draft, err := app.RunConversationReplyTask(ctx, ConversationReplyTaskInput{Context: input, Assignment: providerAssignment{Role: conversationReplyModelRole, BaseURL: "http://controlled.invalid", ModelID: "shared-persona-writer", Timeout: time.Second, TokenBudget: 1024}, CorrelationID: "controlled-moment", Concurrency: 1})
	if err != nil || calls != 1 || draft.Text != "今天读到一个值得记下的段落。\n\n留作稍后的讨论。" {
		t.Fatalf("feed Writer failed: %+v err=%v calls=%d", draft, err, calls)
	}
}

func TestMomentWriterPreservesGoalConditionsWithoutPreviousStrategy(t *testing.T) {
	input := replyContextFixture()
	input.Scene = replySceneMoment
	criteria := []any{"原始完成条件\n第二行", map[string]any{"id": "business-id", "source_ref": "business-ref", "valid": false, "ratio": 0.123456, "changed_at": "2026-10-10T01:02:03Z", "schema": map[string]any{"properties": map[string]any{"item_ids": map[string]any{"type": "array"}}}}}
	input.Projection.Goals = []map[string]any{{
		"id": "private-goal", "description": "阅读目标", "status": "active", "success_criteria": criteria,
		"execution": map[string]any{"next_step": "old-next-step", "current_stage": map[string]any{"strategy": "old-strategy"}, "last_evaluation": map[string]any{"impact": "old-evaluation"}},
	}}
	input.Projection.GoalPolicy = map[string]any{"active_count": 1, "ordering_mode": "manual"}
	before := jsonString(input)
	var request map[string]any
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		request = payload
		return fakeProviderResult{Text: "今天读到一段有趣的文字。"}
	})
	app := &App{Provider: &ProviderClient{HTTP: &http.Client{Transport: router}}}
	_, err := app.RunConversationReplyTask(context.Background(), ConversationReplyTaskInput{Context: input, Assignment: providerAssignment{Role: conversationReplyModelRole, BaseURL: "http://controlled.invalid", ModelID: "fixture", Timeout: time.Second}, Concurrency: 1})
	if err != nil {
		t.Fatal(err)
	}
	var runtime map[string]any
	for _, raw := range arrayValue(request["messages"]) {
		content := stringValue(mapValue(raw)["content"])
		if strings.HasPrefix(content, "[RUNTIME CONTEXT]\n") {
			text := strings.TrimSuffix(strings.TrimPrefix(content, "[RUNTIME CONTEXT]\n"), "\n[/RUNTIME CONTEXT]")
			if err := json.Unmarshal([]byte(text), &runtime); err != nil {
				t.Fatal(err)
			}
		}
	}
	goals := arrayValue(runtime["goals"])
	if len(goals) != 1 || jsonString(mapValue(goals[0])["success_criteria"]) != jsonString(criteria) {
		t.Fatalf("Moment request changed original Goal conditions: %#v", goals)
	}
	wire := jsonString(runtime)
	for _, forbidden := range []string{"private-goal", "old-next-step", "old-strategy", "old-evaluation", "goal_policy"} {
		if strings.Contains(wire, forbidden) {
			t.Fatalf("Moment request retained %q: %s", forbidden, wire)
		}
	}
	if jsonString(input) != before {
		t.Fatal("Moment projection mutated its source")
	}
}

func TestMomentActorEvidencePreservesBusinessValueWithoutCoreEnvelope(t *testing.T) {
	input := replyContextFixture()
	input.Scene = replySceneMoment
	input.Projection.Actors = []map[string]any{{"actor_id": "private-actor", "ref": "actor_user", "type": "human"}}
	value := map[string]any{"id": "business-id", "source_ref": "business-ref", "revision": 7, "valid": false, "ratio": 0.625, "schema": map[string]any{"properties": map[string]any{"item_ids": map[string]any{"type": "array"}}}}
	entry := ContextReference{Kind: ContextReferenceActorFact, Snapshot: jsonBytes(map[string]any{
		"fact_id": "private-fact", "actor_id": "private-actor", "revision": 9, "attribute": "preferences", "value": value, "epistemic_kind": "self_reported",
	})}
	fact := momentReplyEvidenceFact(entry, input.Projection)
	if fact == nil || jsonString(mapValue(fact["value"])["value"]) != jsonString(value) {
		t.Fatalf("Actor evidence was changed: %#v", fact)
	}
	if momentReplyEvidenceFact(ContextReference{Kind: ContextReferenceActorFact, Snapshot: jsonBytes(map[string]any{"epistemic_kind": "inference", "value": "inferred"})}, input.Projection) != nil {
		t.Fatal("inferred Actor fact became authority")
	}
	if momentReplyEvidenceFact(ContextReference{Kind: ContextReferencePreference, Snapshot: jsonBytes(map[string]any{"assertion_type": "inference", "value": "inferred"})}, input.Projection) != nil {
		t.Fatal("inferred preference became authority")
	}
	if momentReplyEvidenceFact(ContextReference{Kind: ContextReferenceGoal, Snapshot: jsonBytes(input.Projection.Goals[0])}, input.Projection) != nil {
		t.Fatal("Goal snapshot bypassed the independent Goal view")
	}
	input.ToolFacts = []map[string]any{fact}
	for _, kind := range []ContextReferenceKind{ContextReferenceDrive, ContextReferencePreference, ContextReferenceTrigger} {
		slot := ContextReference{Kind: kind, Snapshot: jsonBytes(map[string]any{"id": "private-slot", "key": "quiet", "value": value, "confidence": 0.8, "update_policy": map[string]any{"private_policy": "secret-policy"}})}
		input.ToolFacts = append(input.ToolFacts, momentReplyEvidenceFact(slot, input.Projection))
	}
	input.ToolFacts = append(input.ToolFacts, map[string]any{"kind": "goal", "value": map[string]any{"next_step": "old-snapshot-strategy"}})
	before := jsonString(input)
	var request map[string]any
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		request = payload
		return fakeProviderResult{Text: "今天想分享一些阅读感受。"}
	})
	app := &App{Provider: &ProviderClient{HTTP: &http.Client{Transport: router}}}
	_, err := app.RunConversationReplyTask(context.Background(), ConversationReplyTaskInput{Context: input, Assignment: providerAssignment{Role: conversationReplyModelRole, BaseURL: "http://controlled.invalid", ModelID: "fixture", Timeout: time.Second}, Concurrency: 1})
	if err != nil {
		t.Fatal(err)
	}
	var runtime map[string]any
	for _, raw := range arrayValue(request["messages"]) {
		content := stringValue(mapValue(raw)["content"])
		if strings.HasPrefix(content, "[RUNTIME CONTEXT]\n") {
			text := strings.TrimSuffix(strings.TrimPrefix(content, "[RUNTIME CONTEXT]\n"), "\n[/RUNTIME CONTEXT]")
			if err := json.Unmarshal([]byte(text), &runtime); err != nil {
				t.Fatal(err)
			}
		}
	}
	facts := arrayValue(runtime["tool_facts"])
	if len(facts) != 4 {
		t.Fatalf("physical Moment request changed evidence count: %#v", facts)
	}
	for _, fact := range facts {
		if jsonString(mapValue(mapValue(fact)["value"])["value"]) != jsonString(value) {
			t.Fatalf("physical Moment request changed Actor evidence: %#v", facts)
		}
	}
	wire := jsonString(runtime)
	for _, forbidden := range []string{"private-fact", "private-actor", "private-slot", "secret-policy", "old-snapshot-strategy"} {
		if strings.Contains(wire, forbidden) {
			t.Fatalf("physical Moment request included %q: %s", forbidden, wire)
		}
	}
	if jsonString(input) != before {
		t.Fatal("Moment projection mutated evidence")
	}
}

func TestMomentActiveMemoryEvidenceKeepsAssociationsWithoutCoreIDs(t *testing.T) {
	input := replyContextFixture()
	input.Scene = replySceneMoment
	input.Projection.Actors = []map[string]any{
		{"actor_id": "private-memory-actor", "ref": "actor_user", "type": "human"},
		{"actor_id": "private-third-actor", "ref": "actor_c", "type": "human"},
		{"actor_id": "private-extra-actor", "ref": "actor_{", "type": "human"},
	}
	row := map[string]any{
		"id": "private-active-memory", "owner_fluctlight_id": input.Projection.FluctlightID, "conversation_id": input.Projection.ConversationID,
		"actor_refs": []any{"private-memory-actor", "private-third-actor", "private-extra-actor", "private-unknown-actor"}, "source_fact_id": "private-memory-source", "revision": 1,
		"kind": "commitment", "content": "你明天要去看医生", "status": "active", "confidence": 0.75,
		"source_validity": "valid", "valid_until": "2027-01-01T00:00:00Z",
	}
	index := newContextReferenceIndex(input.Projection)
	if err := addActiveMemoryReferences(&index, []map[string]any{row}); err != nil {
		t.Fatal(err)
	}
	entry := index.ByRef[stringValue(row["ref"])]
	input.ToolFacts = []map[string]any{momentReplyEvidenceFact(entry, input.Projection)}
	before := jsonString(input)
	var request map[string]any
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		request = payload
		return fakeProviderResult{Text: "今天想记录一点见闻。"}
	})
	app := &App{Provider: &ProviderClient{HTTP: &http.Client{Transport: router}}}
	_, err := app.RunConversationReplyTask(context.Background(), ConversationReplyTaskInput{Context: input, Assignment: providerAssignment{Role: conversationReplyModelRole, BaseURL: "http://controlled.invalid", ModelID: "fixture", Timeout: time.Second}, Concurrency: 1})
	if err != nil {
		t.Fatal(err)
	}
	var runtime map[string]any
	for _, raw := range arrayValue(request["messages"]) {
		content := stringValue(mapValue(raw)["content"])
		if strings.HasPrefix(content, "[RUNTIME CONTEXT]\n") {
			text := strings.TrimSuffix(strings.TrimPrefix(content, "[RUNTIME CONTEXT]\n"), "\n[/RUNTIME CONTEXT]")
			if err := json.Unmarshal([]byte(text), &runtime); err != nil {
				t.Fatal(err)
			}
		}
	}
	facts := arrayValue(runtime["tool_facts"])
	if len(facts) != 1 {
		t.Fatalf("physical Moment request changed evidence count: %#v", facts)
	}
	fact := mapValue(facts[0])
	memory := mapValue(fact["value"])
	if memory["content"] != "你明天要去看医生" || memory["source_validity"] != "valid" || memory["valid_until"] != "2027-01-01T00:00:00Z" {
		t.Fatalf("physical Moment request lost memory semantics: %#v", memory)
	}
	associated := arrayValue(memory["associated_actors"])
	if len(associated) != 4 || mapValue(associated[0])["actor"] != "actor_user" || mapValue(associated[1])["actor"] != "actor_c" || mapValue(associated[2])["actor"] != "actor_{" || mapValue(associated[3])["actor"] != "unknown" {
		t.Errorf("physical Moment request lost associated actor identities: %#v", associated)
	}
	if _, exists := memory["subjects"]; exists || !strings.Contains(stringValue(fact["certainty"]), "关联人物不一定是内容主体") {
		t.Error("memory evidence did not distinguish associated actors from the content's subject")
	}
	wire := jsonString(runtime)
	for _, forbidden := range []string{"private-active-memory", "private-memory-actor", "private-third-actor", "private-extra-actor", "private-unknown-actor", "private-memory-source", "actor_refs"} {
		if strings.Contains(wire, forbidden) {
			t.Errorf("physical Moment request exposed Core envelope %q", forbidden)
		}
	}
	if jsonString(input) != before {
		t.Fatal("Moment projection mutated evidence")
	}
}

func TestMomentPublicationResolvesActiveMemoryEvidenceWithoutCoreIDs(t *testing.T) {
	ctx, repo, app, owner, self, conversation := setupDirectPublicationToolTest(t, "moment-active-memory")
	config := defaultConversationReplySettings()
	config.Mode = "fact_only"
	config.RetryLimit = 0
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.runtime_settings(key,value_json) VALUES($1,$2) ON CONFLICT(key) DO UPDATE SET value_json=EXCLUDED.value_json`, conversationReplySettingKey, jsonString(config)); err != nil {
		t.Fatal(err)
	}
	created, err := app.ExecuteTool(ctx, ToolExecutionRequest{CapabilityName: "active_memory_event", OperationID: "moment-active-create",
		AuthorizationActorID: owner, FluctlightID: self, ConversationID: conversation, EvidenceID: "moment-active-source", Surface: CapabilitySurfaceNativeCognition,
		Arguments: jsonBytes(map[string]any{"operation": "create", "kind": "commitment", "content": "你明天要去看医生", "confidence": 0.9, "importance": 0.9, "original_time_expression": "", "time_precision": "unknown"})})
	if err != nil || created.Result.Status != "completed" {
		t.Fatalf("create Active Memory: %#v %v", created.Result, err)
	}
	projection, err := app.BuildContextProjectionFor(ctx, ContextProjectionRequest{AuthorizationActorID: owner, SpeakerActorID: owner, FluctlightID: self, ConversationID: conversation, MemoryOperation: MemoryForConversation, MemoryConversationMode: MemoryConversationExact})
	if err != nil {
		t.Fatal(err)
	}
	ref := ""
	for key, entry := range projection.ReferenceIndex.ByRef {
		if entry.Kind == ContextReferenceActiveMemory {
			ref = key
			break
		}
	}
	if ref == "" {
		t.Fatal("Active Memory was not offered in the scoped projection")
	}
	var request map[string]any
	app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		request = payload
		return fakeProviderResult{Text: "今天想记录一点见闻。"}
	})}
	ctx = withFrozenProviderAssignment(ctx, conversationReplyModelRole, providerAssignment{Role: conversationReplyModelRole, BaseURL: "http://controlled.invalid", ModelID: "fixture", Timeout: 10 * time.Second})
	ctx, err = app.withMomentWriterRun(ctx, ToolExecutionRequest{AuthorizationActorID: owner, FluctlightID: self, ConversationID: conversation})
	if err != nil {
		t.Fatal(err)
	}
	service := ctx.Value(momentWriterRunKey{}).(*factOnlyMomentService)
	invocation := CapabilityInvocation{CallID: "moment-active-evidence", CapabilityName: "moment.publish", Arguments: jsonBytes(map[string]any{"evidence_refs": []any{ref}}),
		Metadata:        InvocationMetadata{Source: "model_tool", AuthorizationActorID: owner, SubjectActorID: owner, FluctlightID: self, ConversationID: conversation, OperationID: "moment-active-evidence"},
		ContextSnapshot: map[string]any{"context_reference_index": projection.ReferenceIndex}}
	if _, err := service.prepare(ctx, invocation); err != nil {
		t.Fatal(err)
	}
	var runtime map[string]any
	for _, raw := range arrayValue(request["messages"]) {
		content := stringValue(mapValue(raw)["content"])
		if strings.HasPrefix(content, "[RUNTIME CONTEXT]\n") {
			text := strings.TrimSuffix(strings.TrimPrefix(content, "[RUNTIME CONTEXT]\n"), "\n[/RUNTIME CONTEXT]")
			if err := json.Unmarshal([]byte(text), &runtime); err != nil {
				t.Fatal(err)
			}
		}
	}
	facts := arrayValue(runtime["tool_facts"])
	if len(facts) != 1 || mapValue(mapValue(facts[0])["value"])["content"] != "你明天要去看医生" {
		t.Fatalf("selected Active Memory did not reach the Writer: %#v", facts)
	}
	associated := arrayValue(mapValue(mapValue(facts[0])["value"])["associated_actors"])
	if len(associated) != 1 || mapValue(associated[0])["actor"] != "actor_user" {
		t.Fatalf("selected Active Memory lost its associated actor: %#v", associated)
	}
	wire := jsonString(facts)
	for _, forbidden := range []string{owner, self, "moment-active-source", "actor_refs"} {
		if strings.Contains(wire, forbidden) {
			t.Errorf("selected Active Memory exposed Core envelope %q", forbidden)
		}
	}
}

func TestFactOnlyMomentPublicationPreparedCacheRejectsChangedSourceOrSpeaker(t *testing.T) {
	ctx, repo, app, owner, self, conversation := setupDirectPublicationToolTest(t, "moment-cache-identity")
	config := defaultConversationReplySettings()
	config.Mode = "fact_only"
	config.RetryLimit = 0
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.runtime_settings(key,value_json) VALUES($1,$2)`, conversationReplySettingKey, jsonString(config)); err != nil {
		t.Fatal(err)
	}
	first, err := app.AcceptTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": self, "text": "来源一", "turn_id": "moment-source-one", "idempotency_key": "moment-source-one", "attachment_refs": []any{}})
	if err != nil {
		t.Fatal(err)
	}
	second, err := app.AcceptTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": self, "text": "来源二", "turn_id": "moment-source-two", "idempotency_key": "moment-source-two", "attachment_refs": []any{}})
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().otherwise(func(map[string]any) fakeProviderResult {
		calls++
		return fakeProviderResult{Text: "属于原始来源的动态草稿。"}
	})}
	ctx = withFrozenProviderAssignment(ctx, conversationReplyModelRole, providerAssignment{Role: conversationReplyModelRole, BaseURL: "http://controlled.invalid", ModelID: "shared-writer", Timeout: 10 * time.Second, TokenBudget: 1024, ContextWindowTokens: 16384, MaxInputTokens: 8192, PromptBudgetPolicyVersion: promptBudgetPolicyVersionV1})
	ctx, err = app.withMomentWriterRun(ctx, ToolExecutionRequest{AuthorizationActorID: owner, FluctlightID: self, ConversationID: conversation})
	if err != nil {
		t.Fatal(err)
	}
	service := ctx.Value(momentWriterRunKey{}).(*factOnlyMomentService)
	inv := CapabilityInvocation{CallID: "candidate", CapabilityName: "moment.publish", SourceFactID: first.InboxID, Arguments: json.RawMessage(`{}`), Metadata: InvocationMetadata{AuthorizationActorID: owner, SubjectActorID: owner, FluctlightID: self, ConversationID: conversation, OperationID: "uncommitted-moment"}}
	if _, err := service.prepare(ctx, inv); err != nil {
		t.Fatal(err)
	}
	for _, change := range []string{"source", "speaker"} {
		changed := inv
		changed.CallID = "changed-physical-call"
		if change == "source" {
			changed.SourceFactID = second.InboxID
		} else {
			changed.Metadata.SubjectActorID = self
		}
		if _, err := service.prepare(ctx, changed); !errors.Is(err, ErrConflict) {
			t.Fatalf("changed %s reused old candidate: %v", change, err)
		}
	}
	if calls != 1 {
		t.Fatalf("identity conflict called Writer: %d", calls)
	}
}

func TestFactOnlyMomentPublicationUsesWriterWithoutConversationAndReplays(t *testing.T) {
	ctx, repo, app, owner, self, _ := setupDirectPublicationToolTest(t, "writer-moment")
	config := defaultConversationReplySettings()
	config.Mode = "fact_only"
	config.RetryLimit = 0
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.runtime_settings(key,value_json) VALUES($1,$2) ON CONFLICT(key) DO UPDATE SET value_json=EXCLUDED.value_json`, conversationReplySettingKey, jsonString(config)); err != nil {
		t.Fatal(err)
	}
	calls := 0
	app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		calls++
		if payload["model"] != "shared-persona-writer" || len(arrayValue(payload["tools"])) != 0 || !strings.Contains(jsonString(payload["messages"]), "面向动态参与者") {
			t.Error("moment did not use isolated shared Writer")
		}
		return fakeProviderResult{Text: "由独立人格模型生成的动态。"}
	})}
	assignment := providerAssignment{Role: conversationReplyModelRole, BaseURL: "http://controlled.invalid", ModelID: "shared-persona-writer", Timeout: time.Second, TokenBudget: 1024, ContextWindowTokens: 16384, MaxInputTokens: 8192, PromptBudgetPolicyVersion: promptBudgetPolicyVersionV1}
	ctx = withFrozenProviderAssignment(ctx, conversationReplyModelRole, assignment)
	request := ToolExecutionRequest{CapabilityName: "moment.publish", OperationID: "writer-moment-operation", AuthorizationActorID: owner, FluctlightID: self, Arguments: json.RawMessage(`{}`)}
	first, err := app.ExecuteTool(ctx, request)
	if err != nil || first.Result.Status != "completed" || calls != 1 || stringValue(mapValue(first.Result.Output)["text"]) != "由独立人格模型生成的动态。" {
		t.Fatalf("moment preparation/publication: %+v err=%v calls=%d", first, err, calls)
	}
	// A committed operation replays through the durable ledger before model I/O.
	app.Provider.HTTP = &http.Client{Transport: providerErrorTestTransport(func(*http.Request) (*http.Response, error) {
		t.Fatal("published replay called Writer")
		return nil, nil
	})}
	replayed, err := app.ExecuteTool(ctx, request)
	if err != nil || !toolBoolValue(mapValue(replayed.Result.Output)["replayed"]) {
		t.Fatalf("moment replay failed: %+v %v", replayed, err)
	}
	id := stringValue(mapValue(first.Result.Output)["target_ref"])
	var moments, outbox int
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.moments WHERE id=$1 AND owner_fluctlight_id=$2 AND text=$3`, id, self, "由独立人格模型生成的动态。").Scan(&moments); err != nil {
		t.Fatal(err)
	}
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.platform_outbox_events WHERE kind='moment.published' AND aggregate_id=$1`, id).Scan(&outbox); err != nil {
		t.Fatal(err)
	}
	if moments != 1 || outbox != 1 {
		t.Fatalf("duplicate Moment facts: moment=%d outbox=%d", moments, outbox)
	}
}

func TestFactOnlyMomentPublicationRejectsWriterFailureAndStaleLife(t *testing.T) {
	for _, scenario := range []string{"writer_failed", "life_changed"} {
		t.Run(scenario, func(t *testing.T) {
			ctx, repo, app, owner, self, _ := setupDirectPublicationToolTest(t, "moment-"+scenario)
			config := defaultConversationReplySettings()
			config.Mode = "fact_only"
			config.RetryLimit = 0
			if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.runtime_settings(key,value_json) VALUES($1,$2)`, conversationReplySettingKey, jsonString(config)); err != nil {
				t.Fatal(err)
			}
			calls := 0
			app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().otherwise(func(map[string]any) fakeProviderResult {
				calls++
				if scenario == "writer_failed" {
					return fakeProviderResult{Status: 500, Text: "controlled terminal failure"}
				}
				life := currentLifeForTest(t, ctx, app, self, app.now().UTC())
				if _, err := app.SetPresence(ctx, owner, self, map[string]any{"current_task": "模型生成期间的新事实", "user_presence": "online", "expires_at": app.now().UTC().Add(time.Hour).Format(time.RFC3339), "idempotency_key": "moment-life-change", "expected_life_context_revision": life["context_revision"]}); err != nil {
					t.Fatal(err)
				}
				return fakeProviderResult{Text: "依赖旧生活状态的动态。"}
			})}
			ctx = withFrozenProviderAssignment(ctx, conversationReplyModelRole, providerAssignment{Role: conversationReplyModelRole, BaseURL: "http://controlled.invalid", ModelID: "shared-persona-writer", Timeout: 10 * time.Second, TokenBudget: 1024, ContextWindowTokens: 16384, MaxInputTokens: 8192, PromptBudgetPolicyVersion: promptBudgetPolicyVersionV1})
			receipt, err := app.ExecuteTool(ctx, ToolExecutionRequest{CapabilityName: "moment.publish", OperationID: "blocked-moment", AuthorizationActorID: owner, FluctlightID: self, Arguments: json.RawMessage(`{}`)})
			if scenario == "writer_failed" {
				if err == nil || calls != upstreamProviderMaxRetries+1 {
					t.Fatalf("writer 500 hidden or unbounded: calls=%d err=%v", calls, err)
				}
			} else {
				requireHandledToolNoEffect(t, receipt, err, "moment_context_stale")
				if calls != 1 {
					t.Fatalf("stale draft retried: %d", calls)
				}
			}
			var count int
			if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.moments WHERE owner_fluctlight_id=$1`, self).Scan(&count); err != nil || count != 0 {
				t.Fatalf("failed draft produced visible Moment: count=%d err=%v", count, err)
			}
		})
	}
}
