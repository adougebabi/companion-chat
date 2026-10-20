package core

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func withoutUpstreamRetryDelay(t *testing.T) {
	t.Helper()
	previous := upstreamRetrySleep
	upstreamRetrySleep = func(context.Context, time.Duration) error { return nil }
	t.Cleanup(func() { upstreamRetrySleep = previous })
}

func TestUpstreamRetryBackoffStaysInsideTheCap(t *testing.T) {
	for range 30 {
		if delay := upstreamRetryBackoff(1); delay < 0 || delay > upstreamRetryBaseDelay {
			t.Fatalf("first retry delay %s", delay)
		}
		if delay := upstreamRetryBackoff(2); delay < 0 || delay > upstreamRetryBaseDelay*2 {
			t.Fatalf("second retry delay %s", delay)
		}
		if delay := upstreamRetryBackoff(8); delay < 0 || delay > upstreamRetryMaxDelay {
			t.Fatalf("capped delay %s", delay)
		}
	}
}

func TestUpstreamModelCallRetries500AndGetChannelFailed(t *testing.T) {
	withoutUpstreamRetryDelay(t)
	for _, tc := range []struct {
		name, scenario string
		status         int
		body           string
		ownerRetry     bool
	}{
		{name: "500", scenario: "initialization", status: 500, body: `{"error":{"message":"temporary"}}`},
		{name: "get_channel_failed", scenario: "persona_compilation", status: 503, body: `{"error":{"message":"get_channel_failed"}}`},
		{name: "studio_text", scenario: "studio_text_preview", status: 500, body: `{"error":{"message":"temporary"}}`},
		{name: "studio_assistant", scenario: "studio_assistant", status: 502, body: `{"error":{"message":"bad gateway"}}`},
		{name: "owner_request", scenario: "native_cognition", status: 500, body: `{"error":{"message":"get_channel_failed"}}`, ownerRetry: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls, keys := 0, []string{}
			var mu sync.Mutex
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				calls++
				seen := calls
				keys = append(keys, r.Header.Get("Idempotency-Key"))
				mu.Unlock()
				w.Header().Set("Content-Type", "application/json")
				if seen == 1 {
					w.WriteHeader(tc.status)
					_, _ = w.Write([]byte(tc.body))
					return
				}
				_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"重试后的正文"}}]}`))
			}))
			defer server.Close()
			ctx := context.Background()
			if tc.ownerRetry {
				ctx = withUpstreamModelRetry(ctx)
			}
			ctx = WithADKCapabilityInvoker(ctx, &adkFakeInvoker{}, &ADKCapabilityTrace{})
			_, err := (&ProviderClient{HTTP: server.Client()}).generateWithEino(ctx, upstreamRetryCall(server.URL, tc.scenario))
			mu.Lock()
			defer mu.Unlock()
			if err != nil || calls != 2 || keys[0] == "" || keys[0] != keys[1] {
				t.Fatalf("calls=%d keys=%v err=%v", calls, keys, err)
			}
		})
	}
}

func TestUpstreamModelCallExhaustionReturnsTheProviderError(t *testing.T) {
	withoutUpstreamRetryDelay(t)
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(500)
		_, _ = w.Write([]byte(`{"error":{"message":"still down"}}`))
	}))
	defer server.Close()
	ctx := WithADKCapabilityInvoker(context.Background(), &adkFakeInvoker{}, &ADKCapabilityTrace{})
	response, err := (&ProviderClient{HTTP: server.Client()}).generateWithEino(ctx, upstreamRetryCall(server.URL, "cognitive_assessment"))
	if err == nil || calls != upstreamProviderMaxRetries+1 || response.Message != nil && strings.Contains(response.Message.Content, "重试后的正文") {
		t.Fatalf("calls=%d response=%#v err=%v", calls, response.Message, err)
	}
	if !strings.Contains(err.Error(), "provider HTTP 500") {
		t.Fatalf("error lost the upstream failure: %v", err)
	}
}

func TestUpstreamModelCallDoesNotRetryClientOrQuotaErrors(t *testing.T) {
	withoutUpstreamRetryDelay(t)
	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{
		{name: "400", status: 400, body: `{"error":{"message":"bad request"}}`},
		{name: "429", status: 429, body: `{"error":{"message":"rate limit"}}`},
		{name: "quota", status: 500, body: `{"error":{"message":"insufficient_user_quota"}}`},
		{name: "oom", status: 500, body: `{"error":{"message":"out of memory"}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			ctx := WithADKCapabilityInvoker(context.Background(), &adkFakeInvoker{}, &ADKCapabilityTrace{})
			_, err := (&ProviderClient{HTTP: server.Client()}).generateWithEino(ctx, upstreamRetryCall(server.URL, "initialization"))
			if err == nil || calls != 1 {
				t.Fatalf("calls=%d err=%v", calls, err)
			}
		})
	}
}

func TestUpstreamModelCallRetryDoesNotRepeatToolSideEffects(t *testing.T) {
	withoutUpstreamRetryDelay(t)
	for _, kind := range []string{"retry_before_tool", "retry_after_tool"} {
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
				w.Header().Set("Content-Type", "application/json")
				fail := seen == 1
				if kind == "retry_after_tool" {
					fail = seen == 2
				}
				if fail {
					w.WriteHeader(500)
					_, _ = w.Write([]byte(`{"error":{"message":"temporary"}}`))
					return
				}
				if (kind == "retry_before_tool" && seen == 2) || (kind == "retry_after_tool" && seen == 1) {
					_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","content":"","tool_calls":[{"id":"provider-call-1","type":"function","function":{"name":"memory.recall","arguments":"{\"intent\":\"recent\"}"}}]}}]}`))
					return
				}
				_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"{\"visible_text\":\"found\",\"response_mode\":\"final\",\"capability_invocations\":[]}"}}]}`))
			}))
			defer server.Close()
			invoker := &adkFakeInvoker{}
			trace := &ADKCapabilityTrace{}
			ctx := WithADKCapabilityInvoker(context.Background(), invoker, trace)
			response, err := (&ProviderClient{HTTP: server.Client()}).generateWithEino(ctx, EinoModelCall{
				Assignment:        providerAssignment{Role: "cognitive_assessment", BaseURL: server.URL, ModelID: "fake", Timeout: 10 * time.Second, TokenBudget: 128},
				Role:              "cognitive_assessment",
				Scenario:          "cognitive_assessment",
				Messages:          []map[string]any{{"role": "system", "content": "protocol"}, {"role": "user", "content": "find"}},
				Definitions:       []CapabilityDefinition{{Name: "memory.recall", Description: "Recall", InputSchema: objectSchema(map[string]any{"intent": stringSchema()}, []string{"intent"}, false)}},
				JSONMode:          true,
				SchemaName:        "conversation_turn_response",
				ResponseSchema:    map[string]any{"type": "object"},
				EnableThinking:    true,
				ProviderRequestID: "provider-request-1",
				CorrelationID:     "turn:upstream-retry-" + kind,
			})
			mu.Lock()
			defer mu.Unlock()
			if err != nil || response.Message == nil || len(invoker.calls) != 1 || calls != 3 {
				t.Fatalf("calls=%d tools=%d err=%v message=%#v", calls, len(invoker.calls), err, response.Message)
			}
			if kind == "retry_before_tool" && (keys[0] == "" || keys[0] != keys[1] || keys[1] == keys[2]) {
				t.Fatalf("idempotency keys = %#v", keys)
			}
			if kind == "retry_after_tool" && (keys[1] == "" || keys[1] != keys[2] || keys[0] == keys[1]) {
				t.Fatalf("idempotency keys = %#v", keys)
			}
		})
	}
}

func TestUpstreamModelCallDoesNotRetryWakeUpOrPlainNativeCognition(t *testing.T) {
	withoutUpstreamRetryDelay(t)
	for _, scenario := range []string{"wake_up", "reflection", "native_cognition", "daily_review"} {
		t.Run(scenario, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if calls == 1 {
					w.WriteHeader(500)
					_, _ = w.Write([]byte(`{"error":{"message":"get_channel_failed"}}`))
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"role": "assistant", "content": "不应该重试成功"}}}})
			}))
			defer server.Close()
			ctx := WithADKCapabilityInvoker(context.Background(), &adkFakeInvoker{}, &ADKCapabilityTrace{})
			_, err := (&ProviderClient{HTTP: server.Client()}).generateWithEino(ctx, upstreamRetryCall(server.URL, scenario))
			if err == nil || calls != 1 || strings.Contains(err.Error(), "不应该重试成功") {
				t.Fatalf("scenario %s calls=%d err=%v", scenario, calls, err)
			}
		})
	}
}

func upstreamRetryCall(baseURL, scenario string) EinoModelCall {
	schemaName := "studio_preview_text"
	role := conversationReplyModelRole
	switch scenario {
	case "initialization":
		schemaName = "initialization_response"
		role = "initialization"
	case "persona_compilation":
		schemaName = "persona_compilation_response"
		role = "initialization"
	case "studio_assistant", "cognitive_assessment", "wake_up", "reflection", "native_cognition", "daily_review":
		schemaName = "studio_assistant_v1"
		role = "cognitive_assessment"
		if scenario == "reflection" {
			role = "reflection"
		}
	}
	return EinoModelCall{
		Assignment:        providerAssignment{Role: role, BaseURL: baseURL, ModelID: "fake", Timeout: 10 * time.Second, TokenBudget: 128},
		Role:              role,
		Scenario:          scenario,
		Messages:          []map[string]any{{"role": "system", "content": "protocol"}, {"role": "user", "content": "task"}},
		JSONMode:          false,
		SchemaName:        schemaName,
		ProviderRequestID: "upstream-request",
		CorrelationID:     "upstream:" + scenario,
	}
}
