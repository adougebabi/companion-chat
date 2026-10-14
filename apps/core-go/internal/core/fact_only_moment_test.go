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
				if err == nil {
					t.Fatal("real Writer failure hidden")
				}
			} else {
				requireHandledToolNoEffect(t, receipt, err, "moment_context_stale")
			}
			if calls != 1 {
				t.Fatalf("failed/stale draft retried: %d", calls)
			}
			var count int
			if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.moments WHERE owner_fluctlight_id=$1`, self).Scan(&count); err != nil || count != 0 {
				t.Fatalf("failed draft produced visible Moment: count=%d err=%v", count, err)
			}
		})
	}
}
