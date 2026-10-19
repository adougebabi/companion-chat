package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	capabilitycontract "github.com/fluctlight/local-ai-companion/apps/core-go/internal/capability"
)

func TestReplyProviderUsesFrozenModelSamplingAndIsolatedMessages(t *testing.T) {
	for _, finish := range []string{"stop", "length"} {
		t.Run(finish, func(t *testing.T) {
			var received map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
					t.Error(err)
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"finish_reason": finish, "message": map[string]any{"role": "assistant", "content": "那就约定周六一起讨论。🙂"}}}, "usage": map[string]any{"prompt_tokens": 51, "completion_tokens": 12, "total_tokens": 63}})
			}))
			defer server.Close()
			app := &App{Provider: &ProviderClient{HTTP: server.Client()}}
			temp, top := 0.37, 0.83
			assignment := providerAssignment{Role: conversationReplyModelRole, BaseURL: server.URL, ModelID: "cydonia-frozen", Timeout: 3 * time.Second, TokenBudget: 333, ContextWindowTokens: 16384, MaxInputTokens: 8192, PromptBudgetPolicyVersion: promptBudgetPolicyVersionV1, Temperature: &temp, TopP: &top}
			ctx := withPhysicalModelRequestPolicy(context.Background(), conversationPublicationPhysicalRequestPolicy{})
			ctx = WithPromptDiagnostics(ctx, map[string]any{"response_plan": "用户冷淡所以暂时不推进"})
			draft, err := app.RunConversationReplyTask(ctx, ConversationReplyTaskInput{Context: replyContextFixture(), Assignment: assignment, ConfigVersion: 7, CorrelationID: "reply-controlled", ParentRunID: "parent-controlled", Concurrency: 1})
			if finish == "length" {
				if err == nil || !strings.Contains(err.Error(), "incomplete") {
					t.Fatalf("truncated text accepted: %#v %v", draft, err)
				}
			} else if err != nil || draft.Text != "那就约定周六一起讨论。🙂" || draft.Mode != "fact_only" {
				t.Fatalf("reply failed: %#v %v", draft, err)
			}
			if received["model"] != "cydonia-frozen" || received["temperature"] != temp || received["top_p"] != top || received["max_completion_tokens"] != float64(333) {
				t.Fatalf("frozen transport lost: %#v", received)
			}
			if len(arrayValue(received["tools"])) != 1 || !capabilitycontract.CapabilityNamesMatch(stringValue(mapValue(mapValue(arrayValue(received["tools"])[0])["function"])["name"]), replyImageRequestName) || received["tool_choice"] != "auto" || received["response_format"] != nil {
				t.Fatalf("parent tool/schema policy leaked: %#v", received)
			}
			if strings.Contains(jsonString(received["messages"]), "用户冷淡所以暂时不推进") {
				t.Fatal("parent social plan leaked")
			}
		})
	}
}

func TestReplyDraftMediaParsingNeverPublishesBrokenEnvelopes(t *testing.T) {
	for _, text := range []string{`{"text":"稍等","media_requests":[{"kind":"image","intent":"当前状态的照片"}]}`, `{"text":"🙂"}`} {
		if _, err := parseReplyDraft(text); err != nil {
			t.Fatalf("valid draft rejected: %v", err)
		}
	}
	for _, text := range []string{`{ "text": "断掉了"`, `{"text":"照片发了","direction":"先安慰"}`, `{"text":"照片发了","media_requests":[{"kind":"image","intent":""}]}`} {
		if _, err := parseReplyDraft(text); err == nil {
			t.Errorf("invalid envelope accepted: %s", text)
		}
	}
	draft, err := parseReplyDraft("这个设备型号是 e6，🙂。")
	if err != nil || draft.Text != "这个设备型号是 e6，🙂。" {
		t.Fatal("normal model/code/emoji erased")
	}
}

func TestReplyTextNormalizesParagraphsAndPreservesCode(t *testing.T) {
	for _, sample := range []struct{ input, want string }{
		{"\n\n（看向你）\n\n\n\n第一段\n\n\n第二段\n", "（看向你）\n\n第一段\n\n第二段"},
		{`\n\n第一段\n\n\n\n第二段\n`, "第一段\n\n第二段"},
		{"第一段\r\n \r\n\r\n第二段", "第一段\n\n第二段"},
		{"示例：`\\n`\n\n```go\n\n\nconst x = `\\n\\n`\n```", "示例：`\\n`\n\n```go\n\n\nconst x = `\\n\\n`\n```"},
		{"第一段\\n第二段\n`\\n\\n`\n```text\n\\n\n```", "第一段\n第二段\n`\\n\\n`\n```text\n\\n\n```"},
	} {
		draft, err := parseReplyDraft(sample.input)
		if err != nil || draft.Text != sample.want {
			t.Fatalf("normalized %q, want %q: %v", draft.Text, sample.want, err)
		}
	}
}

func TestReplyCorrectionKeepsCurrentMessageAndProtectedTurns(t *testing.T) {
	input := replyContextFixture()
	input.Projection.RecentMessages = []map[string]any{
		{"id": "a-u", "turn_id": "a", "kind": "user", "text": "第一完整轮的限制"}, {"id": "a-a", "turn_id": "a", "kind": "assistant", "text": "第一完整轮的确认"},
		{"id": "b-u", "turn_id": "b", "kind": "user", "text": "第二完整轮的限制"}, {"id": "b-a", "turn_id": "b", "kind": "assistant", "text": "第二完整轮的确认"},
		{"id": "current", "turn_id": "current", "kind": "user", "text": input.CurrentInput},
	}
	calls := 0
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		calls++
		wire := jsonString(payload["messages"])
		if strings.Count(wire, input.CurrentInput) != 1 || !strings.Contains(wire, "第一完整轮的限制") || !strings.Contains(wire, "第二完整轮的确认") {
			t.Errorf("correction duplicated current message or lost whole turns: %s", wire)
		}
		if calls == 1 {
			return fakeProviderResult{Text: `{"text":"","media_requests":[]}`}
		}
		if !strings.Contains(wire, "回复格式纠错") {
			t.Error("output correction rule absent")
		}
		return fakeProviderResult{Text: "格式已按原始消息修正。"}
	})
	app := &App{Provider: &ProviderClient{HTTP: &http.Client{Transport: router}}}
	settings := defaultConversationReplySettings()
	settings.RetryLimit = 0
	run := &privateReplyRun{Settings: settings}
	draft, err := app.runReplyWithFailurePolicy(context.Background(), ConversationReplyTaskInput{Context: input, Assignment: providerAssignment{Role: conversationReplyModelRole, BaseURL: "http://controlled.invalid", ModelID: "controlled-writer", Timeout: time.Second, TokenBudget: 1024}, CorrelationID: "correction", Concurrency: 1}, run)
	if err != nil || calls != 2 || draft.Text != "格式已按原始消息修正。" {
		t.Fatalf("bounded correction failed: calls=%d draft=%+v err=%v", calls, draft, err)
	}
}

func TestReplyFreshJSONImageRequestRequiresNativeReceipt(t *testing.T) {
	router := newFakeProviderRouter().otherwise(func(map[string]any) fakeProviderResult {
		return fakeProviderResult{Text: `{"text":"稍等","media_requests":[{"kind":"image","intent":"夜景"}]}`}
	})
	app := &App{Provider: &ProviderClient{HTTP: &http.Client{Transport: router}}}
	_, err := app.RunConversationReplyTask(context.Background(), ConversationReplyTaskInput{Context: replyContextFixture(), Assignment: providerAssignment{Role: conversationReplyModelRole, BaseURL: "http://controlled.invalid", ModelID: "controlled-writer", Timeout: time.Second, TokenBudget: 1024}, CorrelationID: "no-native-image", Concurrency: 1})
	if err == nil || err.Error() != "reply_media_request_invalid" {
		t.Fatalf("JSON bypassed native image tool: %v", err)
	}
}

func TestReplyWriterRejectsFabricatedImagePlaceholder(t *testing.T) {
	for _, text := range []string{"[path data for a fabricated image]", "（发送图片）这是新画的。"} {
		if err := validateReplyMediaProse(ReplyDraft{Text: text}, replyContextFixture()); err == nil {
			t.Fatal("fabricated image accepted", text)
		}
	}
	input := replyContextFixture()
	input.ToolFacts = []map[string]any{{"kind": "media_action_result", "value": map[string]any{"status": "ready"}}}
	if err := validateReplyMediaProse(ReplyDraft{Text: "（发送图片）"}, input); err != nil {
		t.Fatal("real ready attachment rejected", err)
	}
}

func TestReplyWriterNativeImageRequestOwnsPrivateHandoff(t *testing.T) {
	calls := 0
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		calls++
		tools := arrayValue(payload["tools"])
		if len(tools) != 1 || !capabilitycontract.CapabilityNamesMatch(stringValue(mapValue(mapValue(tools[0])["function"])["name"]), replyImageRequestName) {
			t.Errorf("Writer needs its sole private image tool: %v", tools)
		}
		if calls == 1 {
			return fakeProviderResult{ToolCalls: []map[string]any{{"id": "writer-image-1", "type": "function", "function": map[string]any{"name": replyImageRequestName, "arguments": `{"intent":"当前地点的夜景素描"}`}}}}
		}
		if wire := jsonString(payload["messages"]); !strings.Contains(wire, "requested_not_generated") || !strings.Contains(wire, `\"sent\":false`) {
			t.Errorf("missing truthful private receipt: %s", wire)
		}
		return fakeProviderResult{Text: "\n\n我想用夜景表达这个画面。\n\n\n\n等实际结果回来。"}
	})
	app := &App{Provider: &ProviderClient{HTTP: &http.Client{Transport: router}}}
	input := ConversationReplyTaskInput{Context: replyContextFixture(), Assignment: providerAssignment{Role: conversationReplyModelRole, BaseURL: "http://controlled.invalid", ModelID: "controlled-writer", Timeout: time.Second, TokenBudget: 1024}, CorrelationID: "private-image", Concurrency: 1}
	draft, err := app.RunConversationReplyTask(context.Background(), input)
	if err != nil || calls != 2 || len(draft.MediaRequests) != 1 || draft.MediaRequests[0].Intent != "当前地点的夜景素描" || strings.Contains(draft.Text, "\n\n\n") {
		t.Fatalf("native handoff failed: calls=%d draft=%+v err=%v", calls, draft, err)
	}
	// Adjustment is intentionally tool-free: terminal media results cannot start
	// a fresh generation and exceed the one draft's action budget.
	input.Context.ToolFacts = []map[string]any{{"kind": "media_action_result", "value": map[string]any{"status": "unavailable"}}}
	if len(replyWriterDefinitions(input.Context)) != 0 {
		t.Fatal("adjustment can request another image")
	}
}

type nestedReplyTransportInvoker struct {
	app        *App
	assignment providerAssignment
	trace      *ADKCapabilityTrace
	published  []string
}

func (i *nestedReplyTransportInvoker) Execute(ctx context.Context, name, args string) (string, error) {
	return i.ExecuteWithID(ctx, "nested-native", name, args)
}
func (i *nestedReplyTransportInvoker) ExecuteWithID(ctx context.Context, id, name, args string) (string, error) {
	definition := factOnlyReplyCapability{}.Definition()
	if err := validateNativeArguments(json.RawMessage(args), definition); err != nil {
		return "", err
	}
	draft, err := i.app.RunConversationReplyTask(ctx, ConversationReplyTaskInput{Context: replyContextFixture(), Assignment: i.assignment, CorrelationID: "nested-writer", Concurrency: 1})
	if err != nil {
		return "", err
	}
	i.published = append(i.published, draft.Text)
	i.trace.AppendInvocation(CapabilityInvocation{CallID: id, CapabilityName: name, Arguments: json.RawMessage(args)})
	result := CapabilityResult{CallID: id, CapabilityName: name, Status: "completed", Output: map[string]any{"text": draft.Text, "target_kind": "conversation_message", "target_ref": "mock-committed-message", "reply_status": "committed", "reply_mode": "fact_only"}}
	i.trace.AppendResult(result)
	return jsonString(modelFacingToolResult(ToolExecutionReceipt{Result: result}, definition)), nil
}

// Controlled HTTP/Eino/ADK transport and mock publication boundary. This is
// not PostgreSQL ExecuteTool or real-model/real-environment acceptance.
func TestNestedReplyAtConcurrencyOneFeedsActualTextBack(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprint(stream), func(t *testing.T) {
			parentCalls, writerCalls := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var payload map[string]any
				_ = json.NewDecoder(r.Body).Decode(&payload)
				message := map[string]any{"role": "assistant", "content": "周六一起讨论吧。"}
				finish := "stop"
				if payload["model"] == "qwen-parent" {
					parentCalls++
					if parentCalls == 1 {
						message = map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"id": "real-native-reply", "type": "function", "function": map[string]any{"name": conversationReplyCapabilityName, "arguments": "{}"}}}}
						finish = "tool_calls"
					} else {
						if !strings.Contains(jsonString(payload["messages"]), "周六一起讨论吧。") {
							t.Error("actual child text absent from parent tool result")
						}
						message["content"] = `{"action_type":"reply","response_intent":"已消费真实回复结果","influences":[]}`
					}
				} else {
					writerCalls++
					if tools := arrayValue(payload["tools"]); len(tools) != 1 || !capabilitycontract.CapabilityNamesMatch(stringValue(mapValue(mapValue(tools[0])["function"])["name"]), replyImageRequestName) {
						t.Error("writer inherited parent tools")
					}
				}
				if boolValue(payload["stream"]) {
					w.Header().Set("Content-Type", "text/event-stream")
					for index, call := range arrayValue(message["tool_calls"]) {
						mapValue(call)["index"] = index
					}
					fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", jsonString(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": message, "finish_reason": finish}}}))
				} else {
					_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": message, "finish_reason": finish}}})
				}
			}))
			defer server.Close()
			provider := &ProviderClient{HTTP: server.Client()}
			assignment := providerAssignment{Role: conversationReplyModelRole, BaseURL: server.URL, ModelID: "cydonia-child", Timeout: 4 * time.Second, TokenBudget: 1024, ContextWindowTokens: 16384, MaxInputTokens: 8192, PromptBudgetPolicyVersion: promptBudgetPolicyVersionV1}
			trace := &ADKCapabilityTrace{}
			invoker := &nestedReplyTransportInvoker{app: &App{Provider: provider}, assignment: assignment, trace: trace}
			ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
			defer cancel()
			ctx = WithADKCapabilityInvoker(ctx, invoker, trace)
			ctx = withPhysicalModelRequestPolicy(ctx, conversationPublicationPhysicalRequestPolicy{})
			parent := assignment
			parent.Role = "generic_llm"
			parent.ModelID = "qwen-parent"
			_, err := provider.generateWithEino(ctx, EinoModelCall{Assignment: parent, Role: "cognitive_assessment", Agent: formalAgentRegistry[FormalAgentConversationCognition], SchemaName: "conversation_turn_response", ResponseSchema: cognitiveTurnResponseSchema(), JSONMode: true, EnableStreaming: stream,
				Messages: []map[string]any{{"role": "user", "content": "我们继续聊。"}}, Definitions: []CapabilityDefinition{factOnlyReplyCapability{}.Definition()}})
			if err != nil {
				t.Fatal(err)
			}
			if parentCalls != 2 || writerCalls != 1 || len(invoker.published) != 1 {
				t.Fatalf("loop/effect counts: parent=%d writer=%d publication=%d", parentCalls, writerCalls, len(invoker.published))
			}
		})
	}
}
