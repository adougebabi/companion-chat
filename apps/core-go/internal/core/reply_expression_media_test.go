package core

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestPrivateChatStyleRepairPreservesNativeImageIntent(t *testing.T) {
	input := privateChatExpressionFixture(t, true)
	dirty := stringValue(input.Projection.RecentMessages[1]["text"])
	intent := "想象的书店画面，暖阳、微卷长发、杯沿细节；是画面创作，不是当下事实。"
	calls, claims := 0, 0
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		calls++
		if calls == 1 {
			return fakeProviderResult{ToolCalls: []map[string]any{{"id": "one-native-image", "type": "function", "function": map[string]any{"name": replyImageRequestName, "arguments": jsonString(map[string]any{"intent": intent})}}}}
		}
		if calls == 2 {
			if !strings.Contains(jsonString(payload["messages"]), "requested_not_generated") {
				t.Error("missing native receipt")
			}
			return fakeProviderResult{Text: dirty}
		}
		if len(arrayValue(payload["tools"])) != 0 {
			t.Error("repair regained media tools")
		}
		return fakeProviderResult{Text: "我想分享这个想象的画面，等实际结果。🙂"}
	})
	app := &App{Provider: &ProviderClient{HTTP: &http.Client{Transport: router}}}
	task := ConversationReplyTaskInput{Context: input, Assignment: providerAssignment{Role: conversationReplyModelRole, BaseURL: "http://controlled.invalid", ModelID: "same-cydonia", TokenBudget: 1024, Timeout: time.Second}, DraftRef: "same-image-draft", Concurrency: 1, ClaimStyleRepair: func(context.Context, ReplyStyleTrace) error { claims++; return nil }}
	draft, err := app.RunConversationReplyTask(context.Background(), task)
	if err != nil || calls != 3 || claims != 1 || len(draft.MediaRequests) != 1 || draft.MediaRequests[0].Intent != intent || draft.Style.RepairCount != 1 {
		t.Fatal("native intent/repair altered", draft, err, calls, claims)
	}
	task.Context.ToolFacts = []map[string]any{{"kind": "media_action_result", "value": map[string]any{"status": "ready", "intent": intent}}, {"kind": "private_candidate_text", "value": draft.Text}}
	task.Context.CapabilityFacts["persona_detail_registered"] = true
	if len(replyWriterDefinitions(task.Context)) != 0 {
		t.Fatal("media result adjustment regained query/image tools")
	}
	// The unavailable on-demand reader must not cause body facts to disappear in
	// a tool-free adjustment or repair.
	prompt, err := buildFactOnlyReplyContext(task.Context, task.Assignment)
	if err != nil || !strings.Contains(jsonString(prompt.Messages), "chest_cup") {
		t.Fatal("tool-free adjustment lost authority state", err)
	}
}

type privateReplyQuerySpy struct{ calls int }

func (s *privateReplyQuerySpy) Execute(context.Context, string, string) (string, error) {
	s.calls++
	return `{}`, nil
}
func (s *privateReplyQuerySpy) ExecuteWithID(context.Context, string, string, string) (string, error) {
	s.calls++
	return `{}`, nil
}

// This checks the child catalog/name boundary only. The canonical query's
// owner/profile/version authorization remains covered by disposable-PG tests.
func TestPrivateChatDetailBridgeIsScopedAndRepairIsToolFree(t *testing.T) {
	input := privateChatExpressionFixture(t, true)
	input.CapabilityFacts["persona_detail_registered"] = true
	spy := &privateReplyQuerySpy{}
	invoker := &replyWriterInvoker{app: &App{DB: &PostgresRepository{}}, input: ConversationReplyTaskInput{Context: input}, detail: spy}
	ctx := withADKCapabilityContext(context.Background(), spy, &ADKCapabilityTrace{}, nil)
	if _, err := invoker.ExecuteWithID(ctx, "read-current", personaDetailCapabilityName, `{"operation":"read","section_id":"current_appearance"}`); err != nil || spy.calls != 1 {
		t.Fatal(err, spy.calls)
	}
	for _, name := range []string{"persona.switch", "media.image.generate", conversationReplyCapabilityName} {
		if _, err := invoker.ExecuteWithID(ctx, "forbidden", name, `{}`); err != ErrUnauthorized {
			t.Fatal("parent/writer action exposed", name, err)
		}
	}
	invoker.input.Context.StyleRepair = true
	if _, err := invoker.ExecuteWithID(ctx, "repair-detail", personaDetailCapabilityName, `{}`); err != ErrUnauthorized || spy.calls != 1 {
		t.Fatal("repair executed query", err, spy.calls)
	}
}

func TestPrivateChatPersonaViewKeepsArbitraryVoiceAndProfile(t *testing.T) {
	for _, portrait := range []string{"寡言直接，不爱客套，明确拒绝时不道歉。", "毒舌，喜欢怪异比喻，被看穿时会沉默；亲密时主动试探。", "严谨分析，习惯长篇列出不确定性，不以安慰结尾。", "活泼跳跃，爱用自己的名字自称，也会突然认真。"} {
		persona := map[string]any{"identity": map[string]any{"name": "合成人格"}, "working_persona": map[string]any{"profile_id": "current-profile", "portrait_text": portrait}}
		before := jsonString(persona)
		view, trace := privateReplyPersonaView(persona, map[string]any{"profile_id": "current-profile", "source_kind": "synthetic_fixture"})
		if mapValue(view["working_persona"])["portrait_text"] != portrait || trace["raw_view_hash"] != stableDigest(before) || trace["profile_id"] != "current-profile" || jsonString(persona) != before {
			t.Fatal("voice/profile mutated", view, trace)
		}
	}
	raw := map[string]any{"working_persona": map[string]any{"custom_mechanism": []any{"倾向A但条件B时例外", "矛盾但有意保持"}}}
	view, trace := privateReplyPersonaView(raw, nil)
	if !strings.Contains(jsonString(view), "矛盾但有意保持") || !strings.Contains(stringValue(trace["mapping"]), "no inferred categories") {
		t.Fatal("unstructured fallback erased content")
	}
	legacy := map[string]any{"identity": map[string]any{"name": "合成人格"}, "personality": map[string]any{"traits": "严谨直白"}, "behavioral_policy": map[string]any{"conflict": "温和但不退让"}}
	projected, _ := privateReplyPersonaView(legacy, nil)
	if wire := jsonString(filterCorePersona(projected)); !strings.Contains(wire, "严谨直白") || !strings.Contains(wire, "温和但不退让") {
		t.Fatal("legacy structured persona was shadowed", wire)
	}
}

func TestPrivateChatCompletionValidationSharedWithReplay(t *testing.T) {
	input := privateChatExpressionFixture(t, true)
	for _, completion := range []ProviderCompletion{{Text: "截断的正文", FinishReason: "length"}, {Text: `{"text":"正文","media_requests":[{"kind":"image","intent":"画面"}]}`, FinishReason: "stop"}} {
		draft, err := decodeReplyCompletion(completion, input, nil, "")
		if err == nil || draft.RawCandidate != completion.Text || draft.FinishReason != completion.FinishReason {
			t.Fatal("replay accepted/lost invalid raw output", draft, err)
		}
	}
	if draft, err := decodeReplyCompletion(ProviderCompletion{Text: "private-test-secret", FinishReason: "length"}, input, nil, "private-test-secret"); err == nil || draft.RawCandidate != "" {
		t.Fatal("secret entered failure trace", draft, err)
	}
	if replyTransportRetryable(&providerHTTPResponseError{StatusCode: 503, Detail: "out of memory"}) {
		t.Fatal("OOM retried as transient 503")
	}
}

func TestPrivateChatProductionCommunicationShapeAndEffectiveBinding(t *testing.T) {
	input := privateChatExpressionFixture(t, true)
	input.Projection.ActorFacts = []map[string]any{{"fact_id": "location-live", "actor_id": "fixture-user", "attribute": "location", "value": "门口", "epistemic_kind": "user_statement", "effective_at": "2026-10-14T17:39:00+08:00", "status": "active", "source_kind": "conversation_message_assertion", "source_pointer": "actor_fact_source:traceable", "valid_until": "2026-10-14T18:00:00+08:00"}}
	view := privateChatCommunicationView(input.Projection)
	location := mapValue(view["recipient_location"])
	if location["effective_at"] != "2026-10-14T17:39:00+08:00" || location["source_pointer"] != "actor_fact_source:traceable" || location["valid_until"] == nil || view["co_presence"] != "unknown" {
		t.Fatal("location claim lost source or became co-presence", view)
	}
	assignment := providerAssignment{Role: "generic_llm", ModelID: "explicit-fallback", EndpointID: "frozen-endpoint", Secret: "private-test-secret"}
	run := &privateReplyRun{EffectiveAssignment: &assignment, EffectiveMode: "legacy_fallback"}
	patch := effectiveReplyBindingPatch(run)
	if strings.Contains(jsonString(patch), assignment.Secret) || patch["reply_mode"] != "legacy_fallback" || patch["model_id"] != assignment.ModelID {
		t.Fatal("effective binding patch incorrect", patch)
	}
	contract, _ := replyExpressionContract(factOnlyReplyPromptVersion)
	merged := map[string]any{"reply_contract": contract, "style_repair_claimed": true}
	for key, value := range patch {
		merged[key] = value
	}
	if merged["style_repair_claimed"] != true || merged["reply_contract"] != contract {
		t.Fatal("binding patch reset durable contract/quota")
	}
}
