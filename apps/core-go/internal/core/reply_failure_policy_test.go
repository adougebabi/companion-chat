package core

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestReplyTransientRetryIsBoundedButOOMAndTruncationDoNotRetry(t *testing.T) {
	for _, kind := range []string{"transient", "oom", "truncated"} {
		t.Run(kind, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if kind == "oom" {
					w.WriteHeader(500)
					_, _ = w.Write([]byte(`{"error":{"message":"out of memory"}}`))
					return
				}
				if kind == "transient" && calls == 1 {
					w.WriteHeader(503)
					return
				}
				finish := "stop"
				if kind == "truncated" {
					finish = "length"
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"finish_reason": finish, "message": map[string]any{"role": "assistant", "content": "真实回复"}}}})
			}))
			defer server.Close()
			app := &App{Provider: &ProviderClient{HTTP: server.Client()}}
			config := defaultConversationReplySettings()
			config.Mode = "fact_only"
			config.RetryLimit = 1
			assignment := providerAssignment{Role: conversationReplyModelRole, BaseURL: server.URL, ModelID: "writer", Timeout: time.Second, TokenBudget: 1024, ContextWindowTokens: 16384, MaxInputTokens: 8192, PromptBudgetPolicyVersion: promptBudgetPolicyVersionV1}
			_, err := app.runReplyWithFailurePolicy(context.Background(), ConversationReplyTaskInput{Context: replyContextFixture(), Assignment: assignment, CorrelationID: "controlled-retry"}, &privateReplyRun{Settings: config})
			if kind == "transient" {
				if err != nil || calls != 2 {
					t.Fatalf("transient: calls=%d error=%v", calls, err)
				}
			} else if err == nil || calls != 1 {
				t.Fatalf("unsafe retry/success: calls=%d error=%v", calls, err)
			}
		})
	}
}

func TestReply500AndGetChannelFailedRetryThenExhaustHonestly(t *testing.T) {
	withoutUpstreamRetryDelay(t)
	for _, kind := range []string{"http_500", "get_channel_failed", "exhausted", "client_error", "quota"} {
		t.Run(kind, func(t *testing.T) {
			var mu sync.Mutex
			calls := 0
			keys := []string{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				calls++
				seen := calls
				keys = append(keys, r.Header.Get("Idempotency-Key"))
				mu.Unlock()
				switch kind {
				case "exhausted":
					w.WriteHeader(500)
					_, _ = w.Write([]byte(`{"error":{"message":"upstream failed"}}`))
					return
				case "client_error":
					w.WriteHeader(400)
					_, _ = w.Write([]byte(`{"error":{"message":"bad request"}}`))
					return
				case "quota":
					w.WriteHeader(429)
					_, _ = w.Write([]byte(`{"error":{"message":"insufficient_user_quota"}}`))
					return
				}
				if seen == 1 {
					if kind == "get_channel_failed" {
						w.WriteHeader(503)
						_, _ = w.Write([]byte(`{"error":{"message":"get_channel_failed"}}`))
						return
					}
					w.WriteHeader(500)
					_, _ = w.Write([]byte(`{"error":{"message":"temporary"}}`))
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": "真实回复"}}}})
			}))
			defer server.Close()
			app := &App{Provider: &ProviderClient{HTTP: server.Client()}}
			config := defaultConversationReplySettings()
			config.Mode = "fact_only"
			config.RetryLimit = 1
			assignment := providerAssignment{Role: conversationReplyModelRole, BaseURL: server.URL, ModelID: "writer", Timeout: time.Second, TokenBudget: 1024, ContextWindowTokens: 16384, MaxInputTokens: 8192, PromptBudgetPolicyVersion: promptBudgetPolicyVersionV1}
			draft, err := app.runReplyWithFailurePolicy(context.Background(), ConversationReplyTaskInput{Context: replyContextFixture(), Assignment: assignment, CorrelationID: "upstream-retry-" + kind}, &privateReplyRun{Settings: config})
			mu.Lock()
			defer mu.Unlock()
			switch kind {
			case "http_500", "get_channel_failed":
				if err != nil || calls != 2 || draft.Text != "真实回复" || keys[0] == "" || keys[0] != keys[1] {
					t.Fatalf("calls=%d keys=%v draft=%q err=%v", calls, keys, draft.Text, err)
				}
			case "exhausted":
				if err == nil || calls != upstreamProviderMaxRetries+1 || draft.Text == "真实回复" {
					t.Fatalf("exhausted calls=%d draft=%q err=%v", calls, draft.Text, err)
				}
			default:
				if err == nil || calls != 1 || draft.Text == "真实回复" {
					t.Fatalf("terminal calls=%d draft=%q err=%v", calls, draft.Text, err)
				}
			}
		})
	}
}

func TestReplyFallbackRequiresExplicitPolicyAndRecordsActualMode(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": "明确降级后的回复"}}}})
	}))
	defer server.Close()
	assignment := providerAssignment{Role: "generic_llm", BaseURL: server.URL, ModelID: "qwen-fallback", EndpointID: "cognition", Timeout: time.Second, TokenBudget: 1024, ContextWindowTokens: 16384, MaxInputTokens: 8192, PromptBudgetPolicyVersion: promptBudgetPolicyVersionV1}
	ctx := withFrozenProviderAssignment(context.Background(), "generic_llm", assignment)
	app := &App{Provider: &ProviderClient{HTTP: server.Client()}}
	config := defaultConversationReplySettings()
	config.Mode = "fact_only"
	run := &privateReplyRun{Settings: config, AssignmentError: errors.New("reply_model_unconfigured")}
	input := ConversationReplyTaskInput{Context: replyContextFixture(), CorrelationID: "fallback-local"}
	if _, err := app.runReplyWithFailurePolicy(ctx, input, run); err == nil || calls != 0 {
		t.Fatal("silent fallback occurred")
	}
	run.Settings.FailurePolicy = "legacy_fallback"
	draft, err := app.runReplyWithFailurePolicy(ctx, input, run)
	if err != nil || calls != 1 || draft.Mode != "legacy_fallback" || draft.ModelRef != "qwen-fallback" {
		t.Fatalf("unmarked fallback: %#v %v", draft, err)
	}
	changed := assignment
	changed.ModelID = "new-binding"
	changedCtx := withFrozenProviderAssignment(context.Background(), "generic_llm", changed)
	for _, resumeRun := range []*privateReplyRun{run, {Settings: run.Settings, Assignment: *run.EffectiveAssignment, EffectiveAssignment: run.EffectiveAssignment, EffectiveMode: run.EffectiveMode}} {
		next, err := app.runReplyWithFailurePolicy(changedCtx, input, resumeRun)
		if err != nil || next.ModelRef != "qwen-fallback" || next.Mode != "legacy_fallback" {
			t.Fatalf("media adjustment changed captured fallback: %#v %v", next, err)
		}
	}
	if calls != 3 {
		t.Fatalf("unexpected fallback requests=%d", calls)
	}
}
