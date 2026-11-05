package core

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

func assertReplyIdentityProtocol(t *testing.T, system string) {
	t.Helper()
	for _, constraint := range []string{"你就是当前角色本身，是一个具体的人", "不得自称 AI、助手、模型或人工智能", "同样适用于 blank_slate（白板）角色", "人格资料为空", "未知的个人经历保持未知"} {
		if !strings.Contains(system, constraint) {
			t.Errorf("identity constraint missing from system protocol: %s", constraint)
		}
	}
}

func TestReplyIdentityProtocolForBlankSlateAndDefinedPersonas(t *testing.T) {
	baseline, err := synthesizeBaselineWorkingPersona(PersonaCompilationInput{ProfileID: "default", CorePersona: map[string]any{"identity": map[string]any{}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, sample := range []struct {
		name    string
		persona map[string]any
	}{
		{"blank-slate", map[string]any{workingPersonaBodyKey: map[string]any{"portrait_text": baseline.PortraitText}}},
		{"empty-portrait", map[string]any{workingPersonaBodyKey: map[string]any{"portrait_text": ""}}},
		{"identity-only", map[string]any{"identity": map[string]any{"name": "白板"}}},
		{"defined-persona", replyContextFixture().WorkingPersona},
	} {
		t.Run(sample.name, func(t *testing.T) {
			input := replyContextFixture()
			input.WorkingPersona = sample.persona
			before := jsonString(input)
			assembled, err := buildFactOnlyReplyContext(input, providerAssignment{})
			if err != nil {
				t.Fatal(err)
			}
			assertReplyIdentityProtocol(t, stringValue(assembled.Messages[0]["content"]))
			if before != jsonString(input) || strings.Count(jsonString(assembled.Messages), input.CurrentInput) != 1 {
				t.Fatal("identity rule changed source data or duplicated current input")
			}
		})
	}
}

func TestReplyIdentityAndMarkdownPolicyReachWriterTransport(t *testing.T) {
	input := replyContextFixture()
	input.WorkingPersona = map[string]any{workingPersonaBodyKey: map[string]any{"portrait_text": "default"}}
	calls := 0
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		calls++
		messages := arrayValue(payload["messages"])
		assertReplyIdentityProtocol(t, stringValue(mapValue(messages[0])["content"]))
		if payload["max_tokens"] != nil || payload["max_completion_tokens"] != nil {
			t.Error("identity rule introduced an output token limit")
		}
		return fakeProviderResult{Text: "# 协议测试\n\n我想再了解你一点。\n```python\n# 注释原样保留\n```"}
	})
	app := &App{Provider: &ProviderClient{HTTP: &http.Client{Transport: router}}}
	draft, err := app.RunConversationReplyTask(context.Background(), ConversationReplyTaskInput{Context: input, Assignment: providerAssignment{Role: conversationReplyModelRole, BaseURL: "http://controlled.invalid", ModelID: "controlled-reply", Timeout: time.Second}, Concurrency: 1})
	want := "协议测试\n\n我想再了解你一点。\n```python\n# 注释原样保留\n```"
	if err != nil || calls != 1 || draft.Text != want || !strings.HasPrefix(draft.RawCandidate, "# 协议测试") {
		t.Fatalf("writer output = %q, calls=%d, error=%v", draft.Text, calls, err)
	}
}

func TestReplyExpressionProtocolsRemainFrozen(t *testing.T) {
	for _, sample := range []struct{ version, hash string }{
		{factOnlyReplyLegacyPromptVersion, "a8df314d706cb6b1786e677181bb4665"},
		{factOnlyReplyV5PromptVersion, "87fe7f55fcaa4684ca9356a2493a4442"},
		{"fact-only-reply.v6", "3bfa91b565ec90cdd8450bd305a21cde"},
		{factOnlyReplyV7PromptVersion, "7708ed509ac8db36dda9fa8147abf094"},
	} {
		t.Run(sample.version, func(t *testing.T) {
			stored := map[string]any{"reply_contract": map[string]any{"version": sample.version, "protocol_hash": sample.hash, "view_version": replyViewVersion, "medium_version": replyMediumVersion}}
			if sample.version == factOnlyReplyLegacyPromptVersion {
				mapValue(stored["reply_contract"])["view_version"] = "private-reply-view.legacy"
				mapValue(stored["reply_contract"])["medium_version"] = "unspecified.legacy"
			}
			contract, err := storedReplyExpressionContract(stored)
			if err != nil || contract.Version != sample.version {
				t.Fatalf("frozen contract changed: %#v, %v", contract, err)
			}
			if _, err := decodeConversationReplySettings(jsonBytes(map[string]any{"expression_version": sample.version})); err != nil {
				t.Fatal(err)
			}
		})
	}
	if defaultConversationReplySettings().ExpressionVersion != "fact-only-reply.v7" {
		t.Fatal("new replies do not default to identity-constrained protocol")
	}
}
