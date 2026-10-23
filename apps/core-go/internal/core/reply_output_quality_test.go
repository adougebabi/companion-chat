package core

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestUnexpectedScriptTailPreservesNamesQuotesEmojiAndLanguageRequests(t *testing.T) {
	if !unexpectedReplyScriptTail("今天读到一个很好的故事。ાન્ય", "") {
		t.Fatal("corrupt suffix passed")
	}
	for _, tc := range []struct{ text, source string }{
		{"今天读到一个很好的故事。✨", ""},
		{"书名是 અન્ય", "书名：અન્ય"},
		{"古吉拉特语例句：અન્ય", "请使用古吉拉特语写例句"},
		{"Hello, world!", ""},
	} {
		if unexpectedReplyScriptTail(tc.text, tc.source) {
			t.Fatal("authored multilingual text rejected", tc)
		}
	}
}
func TestMomentRepairsUnexpectedTailOnceWithoutTools(t *testing.T) {
	for _, staysBroken := range []bool{false, true} {
		input := replyContextFixture()
		input.Scene = replySceneMoment
		input.ToolFacts = append(input.ToolFacts, map[string]any{"kind": "private_candidate_text", "value": "older-unsent-candidate"})
		calls := 0
		router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
			calls++
			if calls > 1 && strings.Contains(jsonString(payload["messages"]), "older-unsent-candidate") {
				t.Error("obsolete candidate entered repair")
			}
			if len(arrayValue(payload["tools"])) != 0 {
				t.Error("repair gained tools")
			}
			if calls == 1 || staysBroken {
				return fakeProviderResult{Text: "今天读到一个很好的故事。ાન્ય"}
			}
			return fakeProviderResult{Text: "今天读到一个很好的故事。"}
		})
		app := &App{Provider: &ProviderClient{HTTP: &http.Client{Transport: router}}}
		draft, err := app.RunConversationReplyTask(context.Background(), ConversationReplyTaskInput{Context: input, Assignment: providerAssignment{Role: conversationReplyModelRole, BaseURL: "http://controlled.invalid", ModelID: "same-writer", Timeout: time.Second, TokenBudget: 1024}, Concurrency: 1})
		if calls != 2 || staysBroken != (err != nil) {
			t.Fatalf("calls=%d err=%v draft=%s", calls, err, draft.Text)
		}
	}
}

func TestPrivateQualityRepairKeepsOneMediaRequestAndOneCandidate(t *testing.T) {
	input := privateChatExpressionFixture(t, true)
	calls := 0
	intent := "一张想象的书店插画"
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		calls++
		if calls == 1 {
			return fakeProviderResult{ToolCalls: []map[string]any{{"id": "single-image-intent", "type": "function", "function": map[string]any{"name": replyImageRequestName, "arguments": jsonString(map[string]any{"intent": intent})}}}}
		}
		if calls == 2 {
			return fakeProviderResult{Text: "我想分享这幅想象的画面。ાન્ય"}
		}
		wire := jsonString(payload["messages"])
		if len(arrayValue(payload["tools"])) != 0 || strings.Contains(wire, "older-unsent-candidate") {
			t.Error("repair reused tools or an obsolete candidate")
		}
		return fakeProviderResult{Text: "我想分享这幅想象的画面，等等实际结果。"}
	})
	app := &App{Provider: &ProviderClient{HTTP: &http.Client{Transport: router}}}
	draft, err := app.RunConversationReplyTask(context.Background(), ConversationReplyTaskInput{Context: input, Assignment: providerAssignment{Role: conversationReplyModelRole, BaseURL: "http://controlled.invalid", ModelID: "same-writer", Timeout: time.Second, TokenBudget: 1024}, Concurrency: 1})
	if err != nil || calls != 3 || len(draft.MediaRequests) != 1 || draft.MediaRequests[0].Intent != intent {
		t.Fatal(draft, err, calls)
	}
}

func TestQualityRepairFailureNeverReopensToolsOrFallback(t *testing.T) {
	withoutUpstreamRetryDelay(t)
	for _, outcome := range []string{"invalid_envelope", "repeated_tail", "http_503", "format_then_invalid"} {
		t.Run(outcome, func(t *testing.T) {
			input := privateChatExpressionFixture(t, true)
			calls := 0
			intent := "一张想象的书店插画"
			router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
				calls++
				writerCall := calls
				if outcome == "format_then_invalid" {
					if calls == 1 {
						return fakeProviderResult{Text: `{"text":""}`}
					}
					writerCall--
				}
				if writerCall == 1 {
					return fakeProviderResult{ToolCalls: []map[string]any{{"id": "original-image-intent", "type": "function", "function": map[string]any{"name": replyImageRequestName, "arguments": jsonString(map[string]any{"intent": intent})}}}}
				}
				if writerCall == 2 {
					return fakeProviderResult{Text: "我想分享这幅想象的画面。ાન્ય"}
				}
				if len(arrayValue(payload["tools"])) != 0 {
					t.Error("quality failure reopened tools")
				}
				switch outcome {
				case "http_503":
					return fakeProviderResult{Status: 503, Text: "unavailable"}
				case "repeated_tail":
					return fakeProviderResult{Text: "我想分享这幅想象的画面。ાન્ય"}
				default:
					return fakeProviderResult{Text: `{"text":""}`}
				}
			})
			app := &App{Provider: &ProviderClient{HTTP: &http.Client{Transport: router}}}
			settings := defaultConversationReplySettings()
			settings.Mode, settings.FailurePolicy = "fact_only", "legacy_fallback"
			draft, err := app.runReplyWithFailurePolicy(context.Background(), ConversationReplyTaskInput{Context: input, Assignment: providerAssignment{Role: conversationReplyModelRole, BaseURL: "http://controlled.invalid", ModelID: "same-writer", Timeout: time.Second, TokenBudget: 1024}, Concurrency: 1}, &privateReplyRun{Settings: settings})
			wantCalls := 3
			if outcome == "format_then_invalid" {
				wantCalls++
			}
			if outcome == "http_503" {
				wantCalls = 2 + upstreamProviderMaxRetries + 1
			}
			if !errors.Is(err, errReplyQualityRepair) || replyErrorCode(err) != "reply_quality_repair_failed" || replyTransportRetryable(err) || calls != wantCalls || len(draft.MediaRequests) != 1 || draft.MediaRequests[0].Intent != intent {
				t.Fatal(draft, err, calls, wantCalls)
			}
		})
	}
}
