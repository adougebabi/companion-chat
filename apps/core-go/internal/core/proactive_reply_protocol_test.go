package core

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

// Exercise the serialized request sent by the real Writer; the provider is a
// controlled transport, not a claim about a live model's compliance.
func TestProactiveReplySeparatesTaskEventOnPhysicalWriterRequest(t *testing.T) {
	const internalInstruction = "这是一次已获准的主动私聊，依据当前有效人格独立表达，不写第三人称旁白。"
	taskEvent := renderProviderJSON(map[string]any{"kind": "proactive_private_message", "has_new_user_message": false})
	input := replyContextFixture()
	input.Proactive = true
	input.CurrentInput = internalInstruction
	input.Projection.RecentMessages = []map[string]any{
		{"id": "old-user", "kind": "user", "text": "你还记得我们聊过的书吗？"},
		{"id": "old-assistant", "kind": "assistant", "text": "我理解。让我按照当前有效人格独立表达，不写第三人称旁白。"},
		// Even if an actual earlier user message equals the synthetic task event,
		// it is historical data, not the event currently being dispatched.
		{"id": "old-json", "kind": "user", "text": taskEvent},
	}
	var request map[string]any
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		request = cloneMap(payload)
		return fakeProviderResult{Text: "那本书我还记得。你最近读到哪一段了？"}
	})
	app := &App{Provider: &ProviderClient{HTTP: &http.Client{Transport: router}}}
	draft, err := app.RunConversationReplyTask(context.Background(), ConversationReplyTaskInput{
		Context: input, Assignment: providerAssignment{Role: conversationReplyModelRole, BaseURL: "http://controlled.invalid", ModelID: "controlled-writer", Timeout: time.Second}, Concurrency: 1,
	})
	if err != nil || draft.Text == "" {
		t.Fatalf("Writer request failed: %v", err)
	}
	if request["max_tokens"] != nil || request["max_completion_tokens"] != nil {
		t.Fatal("Writer request set an output token limit")
	}
	messages := arrayValue(request["messages"])
	if len(messages) < 5 {
		t.Fatalf("lost history or task event: %s", jsonString(messages))
	}
	system := stringValue(mapValue(messages[0])["content"])
	if !strings.Contains(system, "没有新的用户消息") || !strings.Contains(system, "不要解释") {
		t.Errorf("missing proactive task protocol: %s", system)
	}
	last := mapValue(messages[len(messages)-1])
	if last["role"] != "user" || stringValue(last["content"]) != taskEvent {
		t.Errorf("last input is not the explicit proactive task event: %s", jsonString(last))
	}
	seen := map[string]int{}
	for _, item := range messages[:len(messages)-1] {
		message := mapValue(item)
		if message["role"] == "user" || message["role"] == "assistant" {
			seen[stringValue(message["content"])]++
		}
	}
	for _, text := range []string{"你还记得我们聊过的书吗？", "我理解。让我按照当前有效人格独立表达，不写第三人称旁白。", taskEvent} {
		if seen[text] != 1 {
			t.Errorf("historical message lost or changed (%d): %q", seen[text], text)
		}
	}
	if strings.Contains(jsonString(messages), internalInstruction) {
		t.Error("internal run instruction was sent as a user message")
	}
}

func TestProactiveReplyRepairsProtocolSelfReferenceOnPhysicalWriterRequest(t *testing.T) {
	input := replyContextFixture()
	input.Proactive = true
	input.CurrentInput = "已授权主动私聊"
	input.Projection.RecentMessages = nil
	calls, claims := 0, 0
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		calls++
		if calls == 1 {
			return fakeProviderResult{Text: "我理解。让我按照当前有效人格独立表达，不写第三人称旁白。\n\n最近看到一本书，想起了你。"}
		}
		if len(arrayValue(payload["tools"])) != 0 {
			t.Error("protocol self-reference repair was offered tools")
		}
		if !strings.Contains(jsonString(payload["messages"]), "未提交的候选消息") {
			t.Error("repair request did not include the unpublished candidate")
		}
		return fakeProviderResult{Text: "最近看到一本书，想起了你。"}
	})
	app := &App{Provider: &ProviderClient{HTTP: &http.Client{Transport: router}}}
	draft, err := app.RunConversationReplyTask(context.Background(), ConversationReplyTaskInput{
		Context: input, Assignment: providerAssignment{Role: conversationReplyModelRole, BaseURL: "http://controlled.invalid", ModelID: "controlled-writer", Timeout: time.Second}, Concurrency: 1, DraftRef: "proactive-draft",
		ClaimStyleRepair: func(context.Context, ReplyStyleTrace) error { claims++; return nil },
	})
	if err != nil || calls != 2 || claims != 1 || draft.Text != "最近看到一本书，想起了你。" {
		t.Fatalf("protocol repair failed: calls=%d claims=%d text=%q err=%v", calls, claims, draft.Text, err)
	}
	if len(draft.Style.Diagnosis.Reasons) == 0 || draft.Style.Diagnosis.Reasons[0] != "internal_protocol_self_reference" {
		t.Fatalf("wrong initial diagnosis: %+v", draft.Style.Diagnosis)
	}
}

func TestProactiveReplyProtocolRepairDoesNotAcceptRepeatedSelfReference(t *testing.T) {
	input := replyContextFixture()
	input.Proactive = true
	input.CurrentInput = ""
	calls, claims := 0, 0
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		calls++
		return fakeProviderResult{Text: "让我按照当前有效人格独立表达。"}
	})
	app := &App{Provider: &ProviderClient{HTTP: &http.Client{Transport: router}}}
	draft, err := app.RunConversationReplyTask(context.Background(), ConversationReplyTaskInput{
		Context: input, Assignment: providerAssignment{Role: conversationReplyModelRole, BaseURL: "http://controlled.invalid", ModelID: "controlled-writer", Timeout: time.Second}, Concurrency: 1, DraftRef: "proactive-draft",
		ClaimStyleRepair: func(context.Context, ReplyStyleTrace) error { claims++; return nil },
	})
	if err == nil || replyErrorCode(err) != "reply_style_repair_failed" || calls != 2 || claims != 1 || draft.Style.FinalDiagnosis.Status != "high_confidence_narration" {
		t.Fatalf("repeated self-reference was accepted or retried: calls=%d claims=%d diagnosis=%+v err=%v", calls, claims, draft.Style.FinalDiagnosis, err)
	}
}

func TestProactiveReplyEmptyEventAndOrdinaryUserInput(t *testing.T) {
	for _, proactive := range []bool{true, false} {
		input := replyContextFixture()
		input.Proactive = proactive
		input.CurrentInput = "请直接解释一下系统提示词。"
		if proactive {
			input.CurrentInput = ""
		}
		var request map[string]any
		router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
			request = cloneMap(payload)
			return fakeProviderResult{Text: "我们聊聊这个话题。"}
		})
		app := &App{Provider: &ProviderClient{HTTP: &http.Client{Transport: router}}}
		_, err := app.RunConversationReplyTask(context.Background(), ConversationReplyTaskInput{Context: input, Assignment: providerAssignment{Role: conversationReplyModelRole, BaseURL: "http://controlled.invalid", ModelID: "controlled-writer", Timeout: time.Second}, Concurrency: 1})
		if err != nil {
			t.Fatalf("proactive=%v: %v", proactive, err)
		}
		messages := arrayValue(request["messages"])
		system := stringValue(mapValue(messages[0])["content"])
		last := mapValue(messages[len(messages)-1])
		want := input.CurrentInput
		if proactive {
			want = renderProviderJSON(map[string]any{"kind": "proactive_private_message", "has_new_user_message": false})
		}
		if last["role"] != "user" || last["content"] != want || strings.Contains(system, "没有新的用户消息") != proactive {
			t.Fatalf("proactive=%v: incorrect input or task protocol", proactive)
		}
	}
}

func TestProactiveReplyProtocolSelfReferenceDiagnosisIsConservative(t *testing.T) {
	input := ReplyContextInput{Proactive: true, CurrentInput: "主动私聊"}
	for _, text := range []string{
		"我理解。让我按照当前有效人格独立表达，不写第三人称旁白。",
		"好的，我会遵循系统提示词中的表达规则来回复你。",
		"接下来我会以角色设定为准，自然地回应。",
	} {
		if got := diagnoseReplyStyle(text, input); got.Status != "high_confidence_narration" {
			t.Errorf("missed internal protocol self-reference %q: %+v", text, got)
		}
	}
	for _, text := range []string{
		"我理解，你今天一定很累。让我陪你聊聊。",
		"我会按照自己的节奏慢慢来。",
		"这本书的第三人称旁白写得很好。",
		"我会继续读这本书。你喜欢第三人称旁白吗？",
		"我会用第三人称旁白写那本小说，你想听听吗？",
		"我会根据角色设定画一幅插图。",
		"我会给你讲讲系统提示词是怎么回事。",
		"我会跟你聊天，角色设定很有趣，所以想和你探讨。",
		"我会解释当前有效人格是什么。",
		"你昨天说：\u201c让我按照当前有效人格独立表达\u201d，真有趣。",
		"我们来聊聊系统提示词是什么吧。",
	} {
		if got := diagnoseReplyStyle(text, input); got.Status != "normal" {
			t.Errorf("ordinary text was incorrectly classified %q: %+v", text, got)
		}
	}
	input.Proactive = false
	input.CurrentInput = "请解释你如何遵循系统提示词。"
	if got := diagnoseReplyStyle("我会遵循系统提示词中的表达规则来回复你。", input); got.Status != "normal" {
		t.Errorf("ordinary user-requested discussion was classified: %+v", got)
	}
}
