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

func TestReplyRefusalAndFailedMediaClaimNeverInvokeFallbackModel(t *testing.T) {
	for _, tc := range []struct {
		name, text string
		wantError  bool
		wantCalls  int
	}{
		{"refusal", "我不想这样做。", false, 1},
		{"unfulfilled_media_claim", "（发送图片）", true, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writerCalls, fallbackCalls := 0, 0
			router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
				if payload["model"] == "writer" {
					writerCalls++
					return fakeProviderResult{Text: tc.text}
				}
				fallbackCalls++
				return fakeProviderResult{Text: "不应由另一个模型重新决定"}
			})
			app := &App{Provider: &ProviderClient{HTTP: &http.Client{Transport: router}}}
			config := defaultConversationReplySettings()
			config.FailurePolicy = "legacy_fallback"
			writer := providerAssignment{Role: conversationReplyModelRole, EndpointID: "writer-endpoint", BaseURL: "http://controlled.invalid", ModelID: "writer", Timeout: time.Second}
			fallback := providerAssignment{Role: "generic_llm", EndpointID: "fallback-endpoint", BaseURL: "http://controlled.invalid", ModelID: "fallback", Timeout: time.Second}
			ctx := withFrozenProviderAssignment(context.Background(), "generic_llm", fallback)
			draft, err := app.runReplyWithFailurePolicy(ctx, ConversationReplyTaskInput{Context: replyContextFixture(), Assignment: writer, CorrelationID: "fallback-" + tc.name}, &privateReplyRun{Settings: config})
			if (err != nil) != tc.wantError || writerCalls != tc.wantCalls || fallbackCalls != 0 {
				t.Fatalf("draft=%q err=%v writer calls=%d fallback calls=%d", draft.Text, err, writerCalls, fallbackCalls)
			}
			if !tc.wantError && draft.Text != tc.text {
				t.Fatalf("character refusal changed: %#v", draft)
			}
		})
	}
}

func TestReplyConsistencyCorrectionTransportFailureDoesNotChangeModels(t *testing.T) {
	withoutUpstreamRetryDelay(t)
	writerCalls, fallbackCalls := 0, 0
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		if payload["model"] == "writer" {
			writerCalls++
			if writerCalls == 1 {
				return fakeProviderResult{Text: "（发送图片）"}
			}
			return fakeProviderResult{Status: http.StatusServiceUnavailable}
		}
		fallbackCalls++
		return fakeProviderResult{Text: "不应由另一个模型重新生成"}
	})
	app := &App{Provider: &ProviderClient{HTTP: &http.Client{Transport: router}}}
	config := defaultConversationReplySettings()
	config.FailurePolicy = "legacy_fallback"
	run := &privateReplyRun{Settings: config}
	writer := providerAssignment{Role: conversationReplyModelRole, EndpointID: "writer-endpoint", BaseURL: "http://controlled.invalid", ModelID: "writer", Timeout: time.Second}
	fallback := providerAssignment{Role: "generic_llm", EndpointID: "fallback-endpoint", BaseURL: "http://controlled.invalid", ModelID: "fallback", Timeout: time.Second}
	ctx := withFrozenProviderAssignment(context.Background(), "generic_llm", fallback)
	draft, err := app.runReplyWithFailurePolicy(ctx, ConversationReplyTaskInput{Context: replyContextFixture(), Assignment: writer, CorrelationID: "failed-consistency-correction"}, run)
	code, retryable := capabilityErrorInfo(err, "", true)
	if err == nil || code != "reply_media_placeholder_invalid" || retryable || writerCalls != 1+upstreamProviderMaxRetries+1 || fallbackCalls != 0 || run.EffectiveAssignment != nil || run.EffectiveMode == "legacy_fallback" {
		t.Fatalf("output consistency failure triggered fallback: draft=%q err=%v writer=%d fallback=%d mode=%q", draft.Text, err, writerCalls, fallbackCalls, run.EffectiveMode)
	}
}

func TestReplyPhysicalModelTimeoutMayUseExplicitFallback(t *testing.T) {
	withoutUpstreamRetryDelay(t)
	writerCalls, fallbackCalls := 0, 0
	client := &http.Client{Transport: projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		var payload map[string]any
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload["model"] == "writer" {
			writerCalls++
			<-request.Context().Done()
			return nil, request.Context().Err()
		}
		fallbackCalls++
		return embeddingHTTPResponse(request, http.StatusOK, `{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"连接恢复后继续。"}}]}`), nil
	})}
	app := &App{Provider: &ProviderClient{HTTP: client}}
	config := defaultConversationReplySettings()
	config.FailurePolicy = "legacy_fallback"
	config.TimeoutSeconds = 5
	run := &privateReplyRun{Settings: config}
	writer := providerAssignment{Role: conversationReplyModelRole, EndpointID: "writer-endpoint", BaseURL: "http://controlled.invalid", ModelID: "writer", Timeout: 30 * time.Millisecond}
	fallback := providerAssignment{Role: "generic_llm", EndpointID: "fallback-endpoint", BaseURL: "http://controlled.invalid", ModelID: "fallback", Timeout: time.Second}
	ctx := withFrozenProviderAssignment(context.Background(), "generic_llm", fallback)
	draft, err := app.runReplyWithFailurePolicy(ctx, ConversationReplyTaskInput{Context: replyContextFixture(), Assignment: writer, CorrelationID: "writer-physical-timeout"}, run)
	if err != nil || writerCalls == 0 || fallbackCalls != 1 || draft.Mode != "legacy_fallback" || run.EffectiveMode != "legacy_fallback" {
		t.Fatalf("physical model timeout did not use configured fallback: draft=%#v err=%v writer=%d fallback=%d", draft, err, writerCalls, fallbackCalls)
	}
}

func TestReplyOverallDeadlineNeverInvokesFallback(t *testing.T) {
	withoutUpstreamRetryDelay(t)
	writerCalls, fallbackCalls := 0, 0
	client := &http.Client{Transport: projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		var payload map[string]any
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if payload["model"] != "writer" {
			fallbackCalls++
			return embeddingHTTPResponse(request, http.StatusOK, `{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"unexpected"}}]}`), nil
		}
		writerCalls++
		<-request.Context().Done()
		return nil, request.Context().Err()
	})}
	app := &App{Provider: &ProviderClient{HTTP: client}}
	config := defaultConversationReplySettings()
	config.FailurePolicy = "legacy_fallback"
	config.TimeoutSeconds = 5
	run := &privateReplyRun{Settings: config}
	writer := providerAssignment{Role: conversationReplyModelRole, EndpointID: "writer-endpoint", BaseURL: "http://controlled.invalid", ModelID: "writer", Timeout: time.Second}
	fallback := providerAssignment{Role: "generic_llm", EndpointID: "fallback-endpoint", BaseURL: "http://controlled.invalid", ModelID: "fallback", Timeout: time.Second}
	deadline, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	ctx := withFrozenProviderAssignment(deadline, "generic_llm", fallback)
	_, err := app.runReplyWithFailurePolicy(ctx, ConversationReplyTaskInput{Context: replyContextFixture(), Assignment: writer, CorrelationID: "reply-overall-timeout"}, run)
	if !errors.Is(err, context.DeadlineExceeded) || writerCalls == 0 || fallbackCalls != 0 || run.EffectiveAssignment != nil {
		t.Fatalf("overall timeout triggered fallback: err=%v writer=%d fallback=%d", err, writerCalls, fallbackCalls)
	}
}

func TestReplyBusinessFailuresNeverQualifyForFallback(t *testing.T) {
	for _, cause := range []error{ErrUnauthorized, ErrInvalidArguments, ErrConflict, newCapabilityError("tool_condition_unmet", false, ErrConflict), errors.New("reply_media_request_invalid"), errors.New("reply_media_placeholder_invalid"), errors.New("reply_generation_incomplete"), &providerHTTPResponseError{StatusCode: 429, Detail: "insufficient_quota"}, &providerHTTPResponseError{StatusCode: 500, Detail: "out of memory"}} {
		if replyFailureAllowsLegacyFallback(cause) {
			t.Errorf("business or output failure was treated as model unavailability: %v", cause)
		}
	}
	for _, cause := range []error{errors.New("reply_model_unconfigured"), errors.New("reply_model_unavailable"), &providerHTTPResponseError{StatusCode: 503}} {
		if !replyFailureAllowsLegacyFallback(cause) {
			t.Errorf("explicit technical fallback unexpectedly unavailable: %v", cause)
		}
	}
}

func TestReplyToolDependencyFailureIsNotModelUnavailability(t *testing.T) {
	for _, status := range []int{503, 504} {
		err := &agentRunFailure{stage: "tool", code: "persona_detail_unavailable", cause: &providerHTTPResponseError{StatusCode: status}}
		if replyFailureAllowsLegacyFallback(err) || replyErrorCode(err) != "persona_detail_unavailable" {
			t.Fatalf("tool failure became model fallback: status=%d code=%s", status, replyErrorCode(err))
		}
	}
}
