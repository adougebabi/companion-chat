package core

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Replay-only query adapter: returns explicitly reconstructed fixture facts,
// never production state. The formal model/tool wire is real; publication is
// absent. Canonical query authorization is tested separately with PostgreSQL.
type expressionReplayInvoker struct {
	input ReplyContextInput
	image replyImageRequestInvoker
}

func (i *expressionReplayInvoker) ExecuteWithID(ctx context.Context, id, name, args string) (string, error) {
	if name == replyImageRequestName {
		return i.image.ExecuteWithID(ctx, id, name, args)
	}
	if name != personaDetailCapabilityName {
		return "", ErrUnauthorized
	}
	arguments := decodeObject([]byte(args))
	section := stringValue(arguments["section_id"])
	if section == "" {
		return jsonString(CapabilityResult{CallID: id, CapabilityName: name, Status: "completed", Output: map[string]any{"profile_id": "fixture-profile", "source_revision": 1, "overlay_revision": 0, "time_semantics": "current", "sections": []any{map[string]any{"section_id": "current_appearance", "description": "重建fixture当前外貌"}}}}), nil
	}
	if section != "current_appearance" {
		return jsonString(handledCapabilityResult(CapabilityInvocation{CallID: id, CapabilityName: name}, "section_not_in_fixture", "no_results", nil)), nil
	}
	return jsonString(CapabilityResult{CallID: id, CapabilityName: name, Status: "completed", Output: map[string]any{"profile_id": "fixture-profile", "source_revision": 1, "overlay_revision": 0, "time_semantics": "current", "section_id": section, "cursor": 0, "content": jsonString(i.input.Projection.EffectiveAppearance), "has_more": false, "next_cursor": 0}}), nil
}
func (i *expressionReplayInvoker) Execute(ctx context.Context, name, args string) (string, error) {
	return i.ExecuteWithID(ctx, "fixture-query", name, args)
}

func expressionReplayInput(t *testing.T, group string, dirty bool) ReplyContextInput {
	input := privateChatExpressionFixture(t, dirty)
	switch group {
	case "A":
		input.ExpressionVersion = factOnlyReplyLegacyPromptVersion
	case "B":
		input.ViewVersion = "private-reply-view.legacy"
	case "C":
		input.CapabilityFacts["persona_detail_registered"] = true
		input.HistoryViewVersion = "private-reply-view.legacy"
	case "D":
		input.CapabilityFacts["persona_detail_registered"] = true
	default:
		t.Fatal("unknown replay group", group)
	}
	return input
}

func runExpressionReplay(t *testing.T, app *App, input ReplyContextInput, assignment providerAssignment, group string) (ReplyDraft, PromptAssemblyResult, error) {
	t.Helper()
	prompt, err := buildFactOnlyReplyContext(input, assignment)
	if err != nil {
		return ReplyDraft{}, prompt, err
	}
	image := &replyImageRequestTool{}
	registry, err := NewCapabilityRegistry(image, personaDetailCapability{service: newPersonaDetailService(app)})
	if err != nil {
		return ReplyDraft{}, prompt, err
	}
	ctx := withCapabilityRunRegistry(context.Background(), registry)
	ctx = withFrozenProviderAssignment(ctx, conversationReplyModelRole, assignment)
	ctx = WithProviderCorrelation(ctx, "reconstructed-ablation-"+group)
	result, err := app.RunFormalAgent(ctx, FormalAgentConversationReply, FormalAgentRunInput{Prompt: prompt, SchemaName: "conversation_reply_text", Definitions: replyWriterDefinitions(input), Invoker: &expressionReplayInvoker{input: input, image: replyImageRequestInvoker{tool: image}}})
	if err != nil {
		return ReplyDraft{}, prompt, err
	}
	draft, err := decodeReplyCompletion(result.Completion, input, image.requests, assignment.Secret)
	if err != nil {
		return draft, prompt, err
	}
	draft.Expression, _ = replyExpressionContract(input.ExpressionVersion)
	draft.ExpressionVersion = draft.Expression.Version
	draft.Mode, draft.ModelRef, draft.ConfigVersion = "fact_only", assignment.ModelID, 1
	if group == "D" {
		claimed := false
		draft, err = app.validateAndRepairReplyStyle(ctx, ConversationReplyTaskInput{Context: input, Assignment: assignment, DraftRef: "reconstructed-same-draft", CorrelationID: "reconstructed-ablation-D", Concurrency: 1, ClaimStyleRepair: func(context.Context, ReplyStyleTrace) error {
			if claimed {
				return ErrConflict
			}
			claimed = true
			return nil
		}}, draft, result.Completion.Text)
	}
	return draft, prompt, err
}

func TestPrivateChatExpressionAblationRequests(t *testing.T) {
	reports := []map[string]any{}
	for _, dirty := range []bool{false, true} {
		history := "clean"
		if dirty {
			history = "dirty"
		}
		for _, group := range []string{"A", "B", "C", "D"} {
			for sample := 0; sample < 3; sample++ {
				t.Run(fmt.Sprintf("%s_%s_%d", group, history, sample), func(t *testing.T) {
					input := expressionReplayInput(t, group, dirty)
					before := jsonString(input)
					calls := 0
					router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
						calls++
						if group == "C" && calls == 1 {
							return fakeProviderResult{ToolCalls: []map[string]any{{"id": "fixture-appearance-query", "type": "function", "function": map[string]any{"name": personaDetailCapabilityName, "arguments": `{"operation":"read","section_id":"current_appearance"}`}}}}
						}
						if group == "D" && dirty && calls == 1 {
							return fakeProviderResult{Text: stringValue(privateChatExpressionFixture(t, true).Projection.RecentMessages[1]["text"])}
						}
						return fakeProviderResult{Text: "受控脚本输出，不能作为真实模型语义表现。"}
					})
					capture := captureProviderWirePayload(router)
					app := &App{Provider: &ProviderClient{HTTP: &http.Client{Transport: capture}}}
					temperature, topP := 0.8, 0.9
					assignment := providerAssignment{Role: conversationReplyModelRole, BaseURL: "http://controlled.invalid", ModelID: "controlled-cydonia", Timeout: time.Second, TokenBudget: 1024, MaxInputTokens: 8192, ContextWindowTokens: 16384, PromptBudgetPolicyVersion: promptBudgetPolicyVersionV1, Temperature: &temperature, TopP: &topP}
					start := time.Now()
					draft, prompt, err := runExpressionReplay(t, app, input, assignment, group)
					elapsed := time.Since(start)
					if err != nil {
						t.Fatal(err)
					}
					if before != jsonString(input) {
						t.Fatal("replay mutated original persona/history")
					}
					expected := 1
					if group == "C" || group == "D" && dirty {
						expected = 2
					}
					if calls != expected {
						t.Fatal("unexpected calls", group, history, calls)
					}
					if group == "C" && !strings.Contains(jsonString(capture.decoded), "current_appearance") {
						t.Fatal("actual on-demand Tool/result absent")
					}
					report := map[string]any{"group": group, "history": history, "sample": sample, "classification": "controlled_model_and_fixture_query_no_publication", "physical_calls": calls, "estimated_input_tokens": prompt.Trace.EstimatedInputTokens, "controlled_duration_ms": float64(elapsed.Microseconds()) / 1000, "provider_usage": nil, "real_model_validation": "NOT_RUN", "messages": capture.decoded, "draft": draft, "diagnostics": prompt.Diagnostics}
					reports = append(reports, report)
					if dir := os.Getenv("FLUCTLIGHT_PRIVATE_REPLY_REPLAY_OUTPUT"); dir != "" {
						path := filepath.Join(dir, fmt.Sprintf("%s-%s-%d.replay.json", group, history, sample))
						if err := os.WriteFile(path, jsonBytes(report), 0644); err != nil {
							t.Fatal(err)
						}
					}
				})
			}
		}
	}
	if dir := os.Getenv("FLUCTLIGHT_PRIVATE_REPLY_REPLAY_OUTPUT"); dir != "" {
		if err := os.WriteFile(filepath.Join(dir, "ablation-controlled-summary.json"), jsonBytes(reports), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

// Optional real Cydonia + reconstructed facts, never a publication or domain
// mutation. It deliberately preserves every first draft and failure sample.
func TestLivePrivateChatExpressionAblation(t *testing.T) {
	if os.Getenv("FLUCTLIGHT_LIVE_PROVIDER_TEST") != "1" {
		t.Skip("real model replay not opted in")
	}
	base, model := os.Getenv("FLUCTLIGHT_LIVE_REPLY_PROVIDER_BASE_URL"), os.Getenv("FLUCTLIGHT_LIVE_REPLY_PROVIDER_MODEL")
	if base == "" || model == "" {
		t.Skip("Cydonia replay endpoint/model not configured")
	}
	dir := os.Getenv("FLUCTLIGHT_PRIVATE_REPLY_REPLAY_OUTPUT")
	if dir == "" {
		t.Fatal("output directory required to retain failures")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	temperature, topP := 0.8, 0.9 // same frozen reconstructed settings for every group
	assignment := providerAssignment{Role: conversationReplyModelRole, BaseURL: strings.TrimRight(base, "/"), ModelID: model, Secret: os.Getenv("FLUCTLIGHT_LIVE_REPLY_PROVIDER_API_KEY"), Timeout: 5 * time.Minute, TokenBudget: 1024, MaxInputTokens: 8192, ContextWindowTokens: 16384, PromptBudgetPolicyVersion: promptBudgetPolicyVersionV1, Temperature: &temperature, TopP: &topP}
	for _, dirty := range []bool{false, true} {
		history := "clean"
		if dirty {
			history = "dirty"
		}
		for _, group := range []string{"A", "B", "C", "D"} {
			for sample := 0; sample < 3; sample++ {
				t.Run(fmt.Sprintf("%s_%s_%d", group, history, sample), func(t *testing.T) {
					capture := captureProviderWirePayload(http.DefaultTransport)
					app := &App{Provider: &ProviderClient{HTTP: &http.Client{Transport: capture}}}
					start := time.Now()
					draft, prompt, err := runExpressionReplay(t, app, expressionReplayInput(t, group, dirty), assignment, group)
					errorCode := ""
					if err != nil {
						errorCode = replyErrorCode(err)
					}
					report := map[string]any{"group": group, "history": history, "sample": sample, "classification": "real_cydonia_reconstructed_facts_fixture_queries_no_publication", "model": model, "draft": draft, "requests": capture.decoded, "estimated_input_tokens": prompt.Trace.EstimatedInputTokens, "duration_ms": float64(time.Since(start).Microseconds()) / 1000, "error_code": errorCode, "human_personality_judgment": "PENDING"}
					if writeErr := os.WriteFile(filepath.Join(dir, fmt.Sprintf("live-%s-%s-%d.json", group, history, sample)), jsonBytes(report), 0600); writeErr != nil {
						t.Fatal(writeErr)
					}
					if err != nil {
						t.Error(errorCode)
					}
				})
			}
		}
	}
}

func TestLivePrivateChatMultiRoundAndVoices(t *testing.T) {
	if os.Getenv("FLUCTLIGHT_LIVE_PROVIDER_TEST") != "1" {
		t.Skip("real model replay not opted in")
	}
	base, model := os.Getenv("FLUCTLIGHT_LIVE_REPLY_PROVIDER_BASE_URL"), os.Getenv("FLUCTLIGHT_LIVE_REPLY_PROVIDER_MODEL")
	if base == "" || model == "" {
		t.Skip("Cydonia replay endpoint/model not configured")
	}
	dir := os.Getenv("FLUCTLIGHT_PRIVATE_REPLY_REPLAY_OUTPUT")
	if dir == "" {
		t.Fatal("output directory required to retain failures")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	temperature, topP := 0.8, 0.9
	assignment := providerAssignment{Role: conversationReplyModelRole, BaseURL: strings.TrimRight(base, "/"), ModelID: model, Secret: os.Getenv("FLUCTLIGHT_LIVE_REPLY_PROVIDER_API_KEY"), Timeout: 5 * time.Minute, TokenBudget: 1024, MaxInputTokens: 8192, ContextWindowTokens: 16384, PromptBudgetPolicyVersion: promptBudgetPolicyVersionV1, Temperature: &temperature, TopP: &topP}
	voices := []struct {
		name, portrait string
		turns          []string
	}{
		{"wenzhiyi_reconstructed", "", []string{"嗨，在干嘛呢？", "随手找找你呗，你忙的话我就不打扰你了。", "其实我想和你聊会儿。", "你挑书时最看重什么？", "我觉得实用的书就够了，不太需要文学，你怎么看？", "有空的话，周六一起聊新书吧。", "你现在发型是什么样的？", "那个抓夹还是上次说的样子吗？", "想看一张你愿意分享的书店画面。"}},
		{"terse_synthetic", "寡言直接，保留独立判断，不以客套和道歉替代拒绝。", []string{"在干嘛？", "我不同意你刚才的看法。", "我愿意听你详细说。"}},
		{"analytical_synthetic", "严谨，善于长篇分析，主动列出证据与不确定性，允许不同意。", []string{"在干嘛？", "讲讲你看问题的依据。", "我愿意继续讨论，也可以约时间。"}},
		{"lively_synthetic", "活泼跳跃，喜欢玩梗和俏皮自称，认真时明确表达边界。", []string{"嗨，找你聊会儿。", "隔空抱抱，要是在你那儿就好了。", "周六一起聊聊新书？"}},
	}
	for _, voice := range voices {
		t.Run(voice.name, func(t *testing.T) {
			input := expressionReplayInput(t, "D", true)
			classification := "reconstructed_task_appendix"
			if voice.portrait != "" {
				classification = "synthetic_voice_fixture"
				mapValue(input.WorkingPersona["working_persona"])["portrait_text"] = voice.portrait
			}
			reports := []map[string]any{}
			for turn, user := range voice.turns {
				input.CurrentInput = user
				input.Projection.CurrentUserText = user
				capture := captureProviderWirePayload(http.DefaultTransport)
				app := &App{Provider: &ProviderClient{HTTP: &http.Client{Transport: capture}}}
				start := time.Now()
				draft, prompt, err := runExpressionReplay(t, app, input, assignment, "D")
				errorCode := ""
				if err != nil {
					errorCode = replyErrorCode(err)
				}
				reports = append(reports, map[string]any{"turn": turn, "user": user, "classification": classification, "draft": draft, "requests": capture.decoded, "duration_ms": float64(time.Since(start).Microseconds()) / 1000, "estimated_input_tokens": prompt.Trace.EstimatedInputTokens, "error_code": errorCode, "publication": "simulated_only", "media_generation": "NOT_RUN; native requests remain private, no ready asset synthesized", "goal_completion_rate": "not_defined; original empty criteria preserved", "human_personality_judgment": "PENDING"})
				if err == nil && len(draft.MediaRequests) == 0 {
					input.Projection.RecentMessages = append(input.Projection.RecentMessages, map[string]any{"kind": "user", "text": user, "author_actor_id": "fixture-user"}, map[string]any{"kind": "assistant", "text": draft.Text, "author_actor_id": "fixture-self"})
				}
				if writeErr := os.WriteFile(filepath.Join(dir, "live-multiround-"+voice.name+".json"), jsonBytes(reports), 0600); writeErr != nil {
					t.Fatal(writeErr)
				}
				if err != nil {
					t.Error(turn, errorCode)
				}
			}
		})
	}
}
