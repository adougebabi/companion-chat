package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
)

func TestInitializationLengthIsTypedAndDoesNotRepeatGeneration(t *testing.T) {
	for _, stream := range []bool{false, true} {
		for _, item := range []struct {
			name, content, finish string
			wantCalls             int
		}{
			{"partial", `{"ok":`, "length", 1},
			{"valid-but-truncated", `{"ok":true}`, "length", 1},
			{"ordinary-repair", `{"ok":`, "stop", 2},
		} {
			t.Run(fmt.Sprint(stream)+item.name, func(t *testing.T) {
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					var body map[string]any
					_ = json.NewDecoder(r.Body).Decode(&body)
					if body["max_tokens"] != nil || body["max_completion_tokens"] != nil {
						t.Error("application token cap restored")
					}
					content, finish := item.content, item.finish
					if calls > 1 {
						content, finish = `{"ok":true}`, "stop"
					}
					message := map[string]any{"role": "assistant", "content": content}
					choice := map[string]any{"index": 0, "finish_reason": finish}
					if stream {
						choice["delta"] = message
						w.Header().Set("Content-Type", "text/event-stream")
						fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", jsonString(map[string]any{"choices": []any{choice}, "usage": map[string]any{"completion_tokens": 8192}}))
						return
					}
					choice["message"] = message
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{choice}, "usage": map[string]any{"completion_tokens": 8192}})
				}))
				defer server.Close()
				p := &ProviderClient{HTTP: server.Client()}
				trace := &ADKCapabilityTrace{}
				ctx := withADKCapabilityContext(context.Background(), conversationPublicationTestInvoker{trace: trace}, trace, nil)
				_, err := p.generateWithEino(ctx, EinoModelCall{Assignment: providerAssignment{Role: "initialization", BaseURL: server.URL, ModelID: "test", Timeout: 3 * time.Second},
					Role: "initialization", Scenario: "initialization", Messages: []map[string]any{{"role": "user", "content": "原始人物描述"}}, JSONMode: true, SchemaName: "initialization_response", ResponseSchema: openObjectSchema(), EnableStreaming: stream, ProviderRequestID: "length-check", CorrelationID: "length-check"})
				if calls != item.wantCalls {
					t.Fatalf("calls=%d want=%d err=%v", calls, item.wantCalls, err)
				}
				if item.finish == "length" {
					var truncated *structuredResponseTruncatedError
					if !errors.As(err, &truncated) || initializationProviderErrorCode(err) != "initialization_response_truncated" {
						t.Fatalf("untyped truncation: %v", err)
					}
					if truncated.completionTokens != 8192 {
						t.Fatalf("usage lost: %#v", truncated)
					}
				} else if err != nil {
					t.Fatal(err)
				}
			})
		}
	}
	wrapped := fmt.Errorf("outer: %w: %w", errors.New("contract failed"), errors.New("initialization_response_invalid_json"))
	if initializationProviderErrorCode(wrapped) != "initialization_response_invalid_json" {
		t.Fatal("wrapped parse error classified as unavailable")
	}
	cancelled := fmt.Errorf("%w: %w", context.Canceled, &structuredResponseTruncatedError{role: "initialization"})
	if initializationProviderErrorCode(cancelled) != "initialization_provider_cancelled" {
		t.Fatal("cancellation lost precedence")
	}
}

type replyProgressTestInvoker struct {
	trace  *ADKCapabilityTrace
	states []string
	calls  int
}

func (i *replyProgressTestInvoker) Execute(ctx context.Context, name, args string) (string, error) {
	return "", errors.New("native identity required")
}
func (i *replyProgressTestInvoker) ExecuteWithID(ctx context.Context, id, name, args string) (string, error) {
	index := i.calls
	i.calls++
	state := i.states[index]
	status := "requested"
	if state == "waiting" {
		status = "pending"
	}
	if state == "changed" {
		status = "failed"
		state = "needs_action"
	}
	output := map[string]any{"reply_status": state, "draft_ref": "same-draft", "media_requests": []any{map[string]any{"media_request_ref": "same-action", "status": status}}}
	i.trace.Append(CapabilityInvocation{CallID: id, CapabilityName: name}, CapabilityResult{CallID: id, CapabilityName: name, Status: "accepted", Output: output})
	return jsonString(output), nil
}
func TestPrivateReplyMediaProgressBoundsNativeLoop(t *testing.T) {
	for _, stream := range []bool{false, true} {
		for _, item := range []struct {
			name    string
			states  []string
			wantErr bool
		}{
			{"waiting", []string{"waiting"}, false},
			{"stalled", []string{"needs_action", "needs_action"}, true},
			{"changed", []string{"needs_action", "changed", "waiting"}, false},
		} {
			t.Run(fmt.Sprint(stream)+item.name, func(t *testing.T) {
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					var payload map[string]any
					_ = json.NewDecoder(r.Body).Decode(&payload)
					if calls > len(item.states) {
						t.Error("unexpected no-progress model call")
						http.Error(w, "unexpected call", 400)
						return
					}
					message := map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"index": 0, "id": fmt.Sprintf("fresh-%d", calls), "type": "function", "function": map[string]any{"name": conversationReplyCapabilityName, "arguments": "{}"}}}}
					choice := map[string]any{"index": 0, "finish_reason": "tool_calls"}
					if stream {
						choice["delta"] = message
						w.Header().Set("Content-Type", "text/event-stream")
						fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", jsonString(map[string]any{"choices": []any{choice}}))
						return
					}
					choice["message"] = message
					_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{choice}})
				}))
				defer server.Close()
				trace := &ADKCapabilityTrace{}
				invoker := &replyProgressTestInvoker{trace: trace, states: item.states}
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				ctx = withPhysicalModelRequestPolicy(withADKCapabilityContext(ctx, invoker, trace, nil), conversationPublicationPhysicalRequestPolicy{})
				p := &ProviderClient{HTTP: server.Client()}
				response, err := p.generateWithEino(ctx, EinoModelCall{Assignment: providerAssignment{Role: "cognitive_assessment", BaseURL: server.URL, ModelID: "test", Timeout: time.Second},
					Role: "cognitive_assessment", Messages: []map[string]any{{"role": "user", "content": "发张图片"}}, Definitions: []CapabilityDefinition{(factOnlyReplyCapability{}).Definition()},
					JSONMode: true, SchemaName: "conversation_turn_response", ResponseSchema: cognitiveTurnResponseSchema(), EnableStreaming: stream, ProviderRequestID: "progress-check", CorrelationID: "progress-check"})
				if calls != len(item.states) || invoker.calls != len(item.states) {
					t.Fatalf("requests=%d tools=%d err=%v", calls, invoker.calls, err)
				}
				if item.wantErr {
					if err == nil || !strings.Contains(err.Error(), errReplyMediaNoProgress.Error()) {
						t.Fatalf("missing progress guard: %v", err)
					}
				} else {
					if err != nil || response.Message == nil {
						t.Fatal(err)
					}
					if strings.Contains(response.Message.Content, "committed") {
						t.Fatal("waiting media claimed visible publication")
					}
				}
			})
		}
	}
	// A real failed action changes the receipt; it must not be mistaken for an unchanged wait.
	message := schema.AssistantMessage("{}", nil)
	if structuredTerminationError(message, "initialization") != nil {
		t.Fatal("normal response misclassified")
	}
}
