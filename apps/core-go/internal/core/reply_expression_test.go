package core

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func TestPrivateChatExpressionSettingsAndFrozenContract(t *testing.T) {
	for _, version := range []string{factOnlyReplyLegacyPromptVersion, factOnlyReplyPromptVersion} {
		settings, err := decodeConversationReplySettings(jsonBytes(map[string]any{"mode": "fact_only", "expression_version": version}))
		if err != nil || settings.ExpressionVersion != version {
			t.Fatal(settings, err)
		}
		contract, err := replyExpressionContract(version)
		if err != nil {
			t.Fatal(err)
		}
		restored, err := storedReplyExpressionContract(decodeObject(jsonBytes(map[string]any{"reply_contract": contract})))
		if err != nil || restored != contract {
			t.Fatal(restored, err)
		}
	}
	for _, raw := range []string{`{"expression_version":"unknown"}`, `{"expression_version":null}`, `{"expression_version":""}`} {
		if _, err := decodeConversationReplySettings([]byte(raw)); err == nil {
			t.Fatal("invalid expression version accepted", raw)
		}
	}
	old, err := decodeConversationReplySettings([]byte(`{"mode":"fact_only"}`))
	if err != nil || old.ExpressionVersion != factOnlyReplyPromptVersion {
		t.Fatal(old, err)
	}
	stored, err := storedReplyExpressionContract(map[string]any{})
	if err != nil || stored.Version != factOnlyReplyLegacyPromptVersion {
		t.Fatal(stored, err)
	}
	bad := decodeObject(jsonBytes(map[string]any{"reply_contract": stored}))
	mapValue(bad["reply_contract"])["protocol_hash"] = "changed"
	if _, err := storedReplyExpressionContract(bad); err == nil {
		t.Fatal("changed historical contract accepted")
	}
}

func TestPrivateChatRestoredBindingIgnoresHotUpdates(t *testing.T) {
	settings := defaultConversationReplySettings()
	settings.Mode = "fact_only"
	settings.ExpressionVersion = factOnlyReplyLegacyPromptVersion
	settings.Temperature = 0.37
	temperature := 0.37
	assignment := providerAssignment{Role: conversationReplyModelRole, ModelID: "original-frozen-model", BaseURL: "http://controlled.invalid", Temperature: &temperature, TokenBudget: 777}
	contract, _ := replyExpressionContract(settings.ExpressionVersion)
	binding := decodeObject(jsonBytes(map[string]any{"reply_contract": contract, "assignment": assignment, "style_repair_claimed": true}))
	run := &privateReplyRun{Settings: defaultConversationReplySettings(), ConfigVersion: 99, Assignment: providerAssignment{ModelID: "new-model"}, Scope: ToolExecutionRequest{FluctlightID: "original-self", AuthorizationActorID: "original-owner"}}
	if err := (&App{}).restoreReplyRunBinding(context.Background(), run, jsonBytes(settings), 7, binding); err != nil {
		t.Fatal(err)
	}
	if run.Assignment.ModelID != assignment.ModelID || run.ConfigVersion != 7 || run.Settings.Temperature != 0.37 || run.Expression.Version != factOnlyReplyLegacyPromptVersion || run.Scope.AuthorizationActorID != "original-owner" {
		t.Fatal("hot config/model changed durable draft", run)
	}
	delete(binding, "reply_contract")
	if err := (&App{}).restoreReplyRunBinding(context.Background(), run, jsonBytes(settings), 7, binding); err != nil || run.Expression.Version != factOnlyReplyLegacyPromptVersion {
		t.Fatal("old draft used latest expression", run, err)
	}
}

func TestPrivateChatMediumUsesCurrentSources(t *testing.T) {
	for _, sample := range []struct {
		name  string
		facts []map[string]any
		want  string
	}{
		{"empty", nil, "unknown"},
		{"relationship_background", []map[string]any{{"actor_id": "fixture-user", "attribute": "meeting_confirmed", "value": true}, {"actor_id": "fixture-user", "attribute": "relationship_distance", "value": "same_building"}}, "unknown"},
		{"same_place", []map[string]any{{"actor_id": "fixture-user", "attribute": "co_located", "value": true, "source_ref": "message:explicit", "epistemic_kind": "user_statement", "status": "active", "effective_at": "2026-10-14T17:40:00+08:00"}}, "same_place"},
		{"different_place", []map[string]any{{"actor_id": "fixture-user", "attribute": "co_located", "value": false, "source_ref": "owner:current", "epistemic_kind": "owner_assertion"}}, "different_place"},
		{"expired", []map[string]any{{"actor_id": "fixture-user", "attribute": "co_located", "value": true, "source_ref": "message:old", "valid_until": "2026-10-13T17:00:00+08:00"}}, "unknown"},
		{"future", []map[string]any{{"actor_id": "fixture-user", "attribute": "co_located", "value": true, "source_ref": "plan:later", "effective_at": "2026-10-15T17:00:00+08:00"}}, "unknown"},
		{"inference", []map[string]any{{"actor_id": "fixture-user", "attribute": "co_located", "value": true, "source_ref": "inference", "epistemic_kind": "inference"}}, "unknown"},
		{"foreign", []map[string]any{{"actor_id": "foreign", "attribute": "co_located", "value": true, "source_ref": "message:foreign"}}, "unknown"},
	} {
		t.Run(sample.name, func(t *testing.T) {
			input := privateChatExpressionFixture(t, true)
			input.Projection.ActorFacts = sample.facts
			before := jsonString(input.Projection)
			view := privateChatCommunicationView(input.Projection)
			if view["channel"] != "text_chat" || view["co_presence"] != sample.want || mapValue(view["recipient_location"])["status"] != "unknown" {
				t.Fatal(view)
			}
			for _, cognition := range []bool{false, true} {
				projected := privateTextChatProjection(input.Projection)
				facts := factOnlyContextFacts(projected, cognition)
				if jsonString(facts.Facts["communication_state"]) != jsonString(view) {
					t.Fatal("two paths disagree")
				}
			}
			if before != jsonString(input.Projection) {
				t.Fatal("source mutated")
			}
		})
	}
}

func TestPrivateChatPersonaAppearanceHistoryFinalSelection(t *testing.T) {
	input := privateChatExpressionFixture(t, true)
	input.CapabilityFacts["persona_detail_registered"] = true
	before := jsonString(input)
	prompt, err := buildFactOnlyReplyContext(input, providerAssignment{TokenBudget: 1024})
	if err != nil {
		t.Fatal(err)
	}
	wire := jsonString(prompt.Messages)
	for _, required := range []string{"极度通透敏锐", "刻意示弱", "情绪留白", "乱了阵脚", "原文完整保留", "挑选新书与调试咖啡豆", "灵魂共谋者", "success_criteria: []", "text_chat", "past_message_verbatim_not_current_event_proof", "在干嘛呢？"} {
		if !strings.Contains(wire, required) {
			t.Errorf("required semantic/source missing: %s", required)
		}
	}
	runtime := stringValue(prompt.Messages[1]["content"])
	if strings.Contains(runtime, "chest_cup") || strings.Contains(runtime, "乌黑微卷长发") {
		t.Fatal("unrelated appearance expanded", runtime)
	}
	if len(replyWriterDefinitions(input)) != 2 {
		t.Fatal("scoped query/image tools missing")
	}
	if prompt.Messages[2]["role"] != "user" || prompt.Messages[3]["role"] != "assistant" || prompt.Messages[3]["content"] != input.Projection.RecentMessages[1]["text"] {
		t.Fatal("dirty history altered")
	}
	if before != jsonString(input) {
		t.Fatal("persona/history source mutated")
	}
	// Required detail cannot silently vanish if no scoped reader is installed.
	delete(input.CapabilityFacts, "persona_detail_registered")
	fallback, err := buildFactOnlyReplyContext(input, providerAssignment{TokenBudget: 1024})
	if err != nil || !strings.Contains(jsonString(fallback.Messages), "chest_cup") {
		t.Fatal("unavailable-query fallback lost state", err)
	}
	input.Scene = replySceneMoment
	moment, err := buildFactOnlyReplyContext(input, providerAssignment{TokenBudget: 1024})
	if err != nil || strings.Contains(stringValue(moment.Messages[0]["content"]), "不是描写该人物的作者") {
		t.Fatal("private rule leaked into Moment", err)
	}
}

func TestPrivateChatStyleDiagnosisConservative(t *testing.T) {
	input := privateChatExpressionFixture(t, true)
	dirty := stringValue(input.Projection.RecentMessages[1]["text"])
	if got := diagnoseReplyStyle(dirty, input); got.Status != "high_confidence_narration" {
		t.Fatal("known dirty sample not diagnosed", got)
	}
	for _, text := range []string{"来抱一个，隔空的🙂", "要是在你店里就好了。", "温知意今天要认真讲讲这个问题。", "她说她不喜欢咖啡，我记得。", "（笑）还没忙到不能聊天。", "月亮像一枚放凉的硬币。", strings.Repeat("我不同意这个判断，因为证据还不够。", 60)} {
		if got := diagnoseReplyStyle(text, input); got.Status == "high_confidence_narration" {
			t.Fatal("normal style rejected", text, got)
		}
	}
	input.CurrentInput = "请写一段第三人称小说。"
	if got := diagnoseReplyStyle(dirty, input); got.Status != "requested_work" {
		t.Fatal(got)
	}
	input.CurrentInput = "不要写小说旁白，直接说话。"
	if got := diagnoseReplyStyle(dirty, input); got.Status != "high_confidence_narration" {
		t.Fatal(got)
	}
	input.CurrentInput = "下一段继续。"
	input.Projection.RecentMessages = []map[string]any{{"kind": "user", "text": "给我讲一个故事。"}, {"kind": "assistant", "text": dirty}}
	if got := diagnoseReplyStyle(dirty, input); got.Status != "requested_work" {
		t.Fatal("creative continuation rejected", got)
	}
}

func TestPrivateChatStyleDiagnosisUsesAuthoritativeIdentityWithCompiledPortrait(t *testing.T) {
	dirty := "暮光抬起头，看向窗外。\n\n她轻声说：“周六见你。”\n\n她笑着问：“一起聊新书？”\n\n她转向柜台，说：“我想见你。”\n\n她低声答：“我愿意。”"
	for _, carrier := range []string{"canonical", "shared"} {
		t.Run(carrier, func(t *testing.T) {
			input := ReplyContextInput{CurrentInput: "继续聊近况", WorkingPersona: map[string]any{workingPersonaBodyKey: map[string]any{"portrait_text": "稳定的已编译人物描述"}}, Projection: ContextProjection{SelfActor: map[string]any{"display_name": "不同的账户名"}}}
			if carrier == "canonical" {
				input.Projection.CorePersona = map[string]any{"identity": map[string]any{"name": "暮光"}}
			} else {
				input.WorkingPersona[workingPersonaSharedIdentityKey] = map[string]any{"identity": map[string]any{"name": "暮光"}}
			}
			if got := diagnoseReplyStyle(dirty, input); got.Status != "high_confidence_narration" {
				t.Fatal("authoritative name missed", carrier, got)
			}
			if got := diagnoseReplyStyle("暮光：周六一起聊聊新书？", input); got.Status != "normal" {
				t.Fatal("ordinary named message rejected", got)
			}
		})
	}
}

func TestPrivateChatStyleRepairUsesSameModelNoTools(t *testing.T) {
	for _, outcome := range []string{"success", "still_narration", "oom", "repair_503", "oom_503", "claim_database_error", "cancelled", "normal", "quota_spent"} {
		t.Run(outcome, func(t *testing.T) {
			input := privateChatExpressionFixture(t, true)
			dirty := stringValue(input.Projection.RecentMessages[1]["text"]) + "\n\n温知意轻声说：“我想周六见你，一起聊聊新书？”"
			calls, claims := 0, 0
			requests := []map[string]any{}
			router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
				calls++
				requests = append(requests, cloneMap(payload))
				if payload["model"] != "controlled-cydonia" {
					t.Error("model changed")
				}
				if calls == 1 {
					if outcome == "normal" {
						return fakeProviderResult{Text: "我想周六见你。"}
					}
					return fakeProviderResult{Text: dirty}
				}
				if len(arrayValue(payload["tools"])) != 0 || payload["tool_choice"] != nil || payload["parallel_tool_calls"] != nil {
					t.Error("repair can execute tools", payload)
				}
				if !strings.Contains(jsonString(payload["messages"]), "未提交的候选消息") || !strings.Contains(jsonString(payload["messages"]), "不要新增事实") {
					t.Error("repair boundary absent")
				}
				if outcome == "oom" {
					return fakeProviderResult{Status: 507, Text: "out of memory"}
				}
				if outcome == "repair_503" {
					return fakeProviderResult{Status: 503, Text: "unavailable"}
				}
				if outcome == "oom_503" {
					return fakeProviderResult{Status: 503, Text: "out of memory"}
				}
				if outcome == "still_narration" {
					return fakeProviderResult{Text: dirty}
				}
				return fakeProviderResult{Text: "我想周六见你，来聊聊这批新书？"}
			})
			app := &App{Provider: &ProviderClient{HTTP: &http.Client{Transport: router}}}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			settings := defaultConversationReplySettings()
			settings.Mode = "fact_only"
			settings.FailurePolicy = "legacy_fallback"
			run := &privateReplyRun{Settings: settings}
			task := ConversationReplyTaskInput{Context: input, Assignment: providerAssignment{Role: conversationReplyModelRole, BaseURL: "http://controlled.invalid", ModelID: "controlled-cydonia", Timeout: time.Second, TokenBudget: 1024, MaxInputTokens: 8192}, Concurrency: 1, DraftRef: "same-draft", CorrelationID: "controlled-style", ClaimStyleRepair: func(context.Context, ReplyStyleTrace) error {
				claims++
				if outcome == "claim_database_error" {
					return errors.New("controlled database disconnected")
				}
				if outcome == "quota_spent" {
					return newCapabilityError("reply_style_repair_unavailable", false, ErrConflict)
				}
				if outcome == "cancelled" {
					cancel()
				}
				return nil
			}}
			draft, err := app.runReplyWithFailurePolicy(ctx, task, run)
			if dir := os.Getenv("FLUCTLIGHT_PRIVATE_REPLY_REPLAY_OUTPUT"); dir != "" {
				errorCode := ""
				if err != nil {
					errorCode = replyErrorCode(err)
				}
				if writeErr := os.WriteFile(dir+"/style-"+outcome+"-controlled.json", jsonBytes(map[string]any{"classification": "controlled_script_not_real_model_validation", "scenario": outcome, "physical_calls": calls, "quota_claims": claims, "requests": requests, "draft": draft, "error_code": errorCode, "publication": "NOT_EXECUTED"}), 0644); writeErr != nil {
					t.Fatal(writeErr)
				}
			}
			if outcome == "success" {
				if err != nil || calls != 2 || claims != 1 || draft.Style.RepairCount != 1 || draft.Style.FirstText != dirty || draft.Text != "我想周六见你，来聊聊这批新书？" {
					t.Fatal(draft, err, calls, claims)
				}
			} else if outcome == "normal" {
				if err != nil || calls != 1 || claims != 0 {
					t.Fatal(draft, err, calls, claims)
				}
			} else if outcome == "repair_503" {
				if err == nil || calls != 1+upstreamProviderMaxRetries+1 || claims != 1 {
					t.Fatal("repair 503 was not limited to the model call", err, calls, claims)
				}
			} else {
				if err == nil || calls > 2 || claims != 1 {
					t.Fatal("unbounded/fallback repair", err, calls, claims)
				}
				if outcome == "cancelled" && !errors.Is(err, context.Canceled) {
					t.Fatal("cancellation lost", err)
				}
				if outcome == "oom_503" && calls != 2 {
					t.Fatal("repair transport retried whole generation", calls)
				}
				if outcome == "claim_database_error" && calls != 1 {
					t.Fatal("claim error invoked fallback", calls)
				}
			}
		})
	}
}

func TestPrivateChatHistoryKeepsUnquotedPromisesAndRefusals(t *testing.T) {
	input := privateChatExpressionFixture(t, true)
	text := "温知意看向窗边。\n\n我愿意周六来，但今天不接受这件事。\n承诺：周五给你答复，没写引号也必须保留。\n\n“这是一句引用。”她说道。"
	input.Projection.RecentMessages = []map[string]any{{"id": "source-u", "kind": "user", "text": "你愿意周六来吗？", "created_at": "2026-10-14T09:00:00+08:00"}, {"id": "source-a", "kind": "assistant", "text": text, "created_at": "2026-10-14T09:01:00+08:00"}, {"id": "source-a2", "kind": "assistant", "text": "# 2026-10-14 23:59\n我不否认前面答应过的事。", "created_at": "2026-10-14T09:02:00+08:00"}, {"id": "source-current", "kind": "user", "text": input.CurrentInput}}
	before := jsonString(input)
	prompt, err := buildFactOnlyReplyContext(input, providerAssignment{TokenBudget: 1024})
	if err != nil {
		t.Fatal(err)
	}
	if prompt.Messages[2]["role"] != "user" || prompt.Messages[3]["role"] != "assistant" || prompt.Messages[3]["content"] != text || prompt.Messages[4]["role"] != "assistant" || prompt.Messages[4]["content"] != input.Projection.RecentMessages[2]["text"] {
		t.Fatal("unmarked mixed history/continuous roles altered")
	}
	if !strings.Contains(jsonString(prompt.Messages), "2026-10-14T17:40") || jsonString(input) != before {
		t.Fatal("body title overrode clock or source mutated")
	}
}

func TestPrivateChatExpressionProtocolOnlyWireComparison(t *testing.T) {
	// A is the pre-edit transport capture; B uses the exact same projection and
	// legacy view, changing only the stable protocol. It is not model acceptance.
	input := privateChatExpressionFixture(t, true)
	input.ViewVersion = "private-reply-view.legacy"
	var requests []map[string]any
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		requests = append(requests, cloneMap(payload))
		return fakeProviderResult{Text: "受控模型输出。"}
	})
	app := &App{Provider: &ProviderClient{HTTP: &http.Client{Transport: router}}}
	temperature, topP := 0.8, 0.9
	for _, version := range []string{factOnlyReplyLegacyPromptVersion, factOnlyReplyPromptVersion} {
		input.ExpressionVersion = version
		_, err := app.RunConversationReplyTask(context.Background(), ConversationReplyTaskInput{Context: input, Assignment: providerAssignment{Role: conversationReplyModelRole, BaseURL: "http://controlled.invalid", ModelID: "controlled-cydonia", Timeout: time.Second, TokenBudget: 1024, MaxInputTokens: 8192, ContextWindowTokens: 16384, PromptBudgetPolicyVersion: promptBudgetPolicyVersionV1, Temperature: &temperature, TopP: &topP}, Concurrency: 1})
		if err != nil {
			t.Fatal(err)
		}
	}
	before, after := arrayValue(requests[0]["messages"]), arrayValue(requests[1]["messages"])
	if jsonString(before[1:]) != jsonString(after[1:]) || jsonString(requests[0]["tools"]) != jsonString(requests[1]["tools"]) {
		t.Fatal("protocol-only experiment changed context/tools")
	}
	system := stringValue(mapValue(after[0])["content"])
	if strings.Count(system, "# 私聊运行协议") != 1 || strings.Contains(system, "言语交织神态") || strings.Contains(system, "自然输出人物正文") {
		t.Fatal("new/old protocol overlap")
	}
	if dir := os.Getenv("FLUCTLIGHT_PRIVATE_REPLY_REPLAY_OUTPUT"); dir != "" {
		if err := os.WriteFile(dir+"/B-dirty.wire.json", jsonBytes(requests[1]), 0644); err != nil {
			t.Fatal(err)
		}
	}
}
