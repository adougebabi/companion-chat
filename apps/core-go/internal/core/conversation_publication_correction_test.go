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

	"github.com/cloudwego/eino/schema"
	capabilitycontract "github.com/fluctlight/local-ai-companion/apps/core-go/internal/capability"
)

func TestConversationNativePublicationCorrectionPhysicalRequests(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		for _, outcome := range []string{"reply", "ignored", "wrong_tool", "multiple_tools", "rejected_receipt", "corrected_receipt"} {
			t.Run(fmt.Sprintf("stream=%v/%s", streaming, outcome), func(t *testing.T) {
				var requests []map[string]any
				var identities []string
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var payload map[string]any
					if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
						t.Error(err)
						return
					}
					requests = append(requests, payload)
					id := r.Header.Get("X-Fluctlight-Provider-Request-Id")
					if id == "" || id != r.Header.Get("Idempotency-Key") {
						t.Errorf("physical headers missing: %v", r.Header)
					}
					identities = append(identities, id)
					// Match Providers which accept only string tool_choice values.
					// Reject the physical request before emitting any model response.
					choice, ok := payload["tool_choice"].(string)
					if !ok || (choice != "none" && choice != "auto" && choice != "required") {
						w.WriteHeader(http.StatusBadRequest)
						_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"message": "Invalid tool_choice type: 'object'. Supported values are 'none', 'auto', 'required'."}})
						return
					}
					message := map[string]any{"role": "assistant", "content": "Rejected private prose candidate"}
					finish := "stop"
					tool := func(id, name, args string) map[string]any {
						return map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"index": 0, "id": id, "type": "function", "function": map[string]any{"name": name, "arguments": args}}}}
					}
					switch len(requests) {
					case 1:
						message = tool("native-read", "memory.recall", `{"query":"read first"}`)
						finish = "tool_calls"
					case 3:
						if outcome == "reply" || outcome == "rejected_receipt" || outcome == "corrected_receipt" {
							message = tool("native-corrected-reply", conversationReplyCapabilityName, `{"text":"Model generated native reply"}`)
							finish = "tool_calls"
						}
						if outcome == "wrong_tool" {
							message = tool("native-wrong-read", "memory.recall", `{"query":"wrong"}`)
							finish = "tool_calls"
						}
						if outcome == "multiple_tools" {
							message = tool("native-first-reply", conversationReplyCapabilityName, `{"text":"First reply"}`)
							second := tool("native-second-reply", conversationReplyCapabilityName, `{"text":"Second reply"}`)
							secondCall := second["tool_calls"].([]any)[0].(map[string]any)
							secondCall["index"] = 1
							message["tool_calls"] = append(message["tool_calls"].([]any), secondCall)
							finish = "tool_calls"
						}
					case 4:
						message = map[string]any{"role": "assistant", "content": jsonString(map[string]any{"action_type": "reply", "response_intent": "published", "influences": []any{}})}
						if outcome == "corrected_receipt" {
							message = tool("native-argument-corrected-reply", conversationReplyCapabilityName, `{"text":"Corrected native reply arguments"}`)
							finish = "tool_calls"
						}
					case 5:
						message = map[string]any{"role": "assistant", "content": jsonString(map[string]any{"action_type": "reply", "response_intent": "published", "influences": []any{}})}
					}
					if len(requests) > 5 {
						t.Error("unbounded correction")
					}
					if payload["stream"] == true {
						w.Header().Set("Content-Type", "text/event-stream")
						fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", jsonString(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": message, "finish_reason": finish}}}))
					} else {
						_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": message, "finish_reason": finish}}})
					}
				}))
				defer server.Close()
				trace := &ADKCapabilityTrace{}
				ctx := withADKCapabilityContext(context.Background(), conversationPublicationTestInvoker{trace: trace, rejectReply: outcome == "rejected_receipt", rejectFirstReply: outcome == "corrected_receipt"}, trace, nil)
				ctx = withPhysicalModelRequestPolicy(ctx, conversationPublicationPhysicalRequestPolicy{})
				p := &ProviderClient{HTTP: server.Client()}
				response, err := p.generateWithEino(ctx, EinoModelCall{Assignment: providerAssignment{Role: "cognitive_assessment", BaseURL: server.URL, ModelID: "fake", Timeout: time.Second, TokenBudget: 4096, ContextWindowTokens: 131072, MaxInputTokens: 98304, PromptBudgetPolicyVersion: "prompt-budget.v1"}, Role: "cognitive_assessment", Messages: []map[string]any{{"role": "user", "content": "Read memory then reply"}}, Definitions: []CapabilityDefinition{conversationReplyCapabilityDefinition(), {Name: "memory.recall", Description: "Read memory", InputSchema: map[string]any{"type": "object", "properties": map[string]any{"query": map[string]any{"type": "string"}}}}}, JSONMode: true, SchemaName: "conversation_turn_response", ResponseSchema: cognitiveTurnResponseSchema(), EnableStreaming: streaming, EnableThinking: true, CorrelationID: "turn:correction"})
				wantCalls, wantResults := 3, 1
				if outcome == "rejected_receipt" {
					wantCalls, wantResults = 4, 2
				}
				if outcome == "reply" || outcome == "corrected_receipt" {
					wantCalls, wantResults = 3, 2
					if outcome == "corrected_receipt" {
						wantCalls, wantResults = 4, 3
					}
					if err != nil || len(response.ExecutedToolCalls) != wantResults || strings.Contains(response.Message.Content, "Rejected private prose") {
						t.Fatalf("native success=%#v err=%v", response, err)
					}
				} else if err == nil || !strings.Contains(err.Error(), "conversation_native_output_missing") {
					t.Fatalf("missing fail-closed error: %v", err)
				}
				if len(requests) != wantCalls || len(trace.Results) != wantResults {
					t.Fatalf("requests=%d results=%d", len(requests), len(trace.Results))
				}
				seen := map[string]bool{}
				for _, id := range identities {
					if seen[id] {
						t.Errorf("physical identity reused %q", id)
					}
					seen[id] = true
				}
				for _, request := range requests[:2] {
					if len(arrayValue(request["tools"])) != 2 || request["tool_choice"] != "required" || request["enable_thinking"] != true {
						t.Fatalf("initial catalog/gates changed: %#v", request)
					}
				}
				correction := requests[2]
				correctionTools := arrayValue(correction["tools"])
				if len(correctionTools) != 1 || !capabilitycontract.CapabilityNamesMatch(stringValue(mapValue(mapValue(correctionTools[0])["function"])["name"]), conversationReplyCapabilityName) || correction["tool_choice"] != "required" || correction["response_format"] != nil || correction["enable_thinking"] != false {
					t.Fatalf("correction wire=%#v", correction)
				}
				if strings.Contains(jsonString(correction["messages"]), "Rejected private prose candidate") || !strings.Contains(jsonString(correction["messages"]), "target_kind") {
					t.Fatalf("candidate leaked or native result missing: %#v", correction["messages"])
				}
				for _, invocation := range trace.Invocations {
					identity, ok := trace.ModelIdentity(invocation.CallID)
					if !ok || !seen[identity.ProviderRequestID] {
						t.Fatalf("native provenance lost %s: %#v", invocation.CallID, identity)
					}
				}
				if outcome == "corrected_receipt" {
					continuation := requests[3]
					if len(arrayValue(continuation["tools"])) != 2 || continuation["tool_choice"] != "required" || continuation["enable_thinking"] != true || !strings.Contains(jsonString(continuation["messages"]), "invalid_arguments") {
						t.Fatalf("native argument correction did not restore catalog and rejected receipt feedback: %#v", continuation)
					}
				}
				if outcome == "reply" || outcome == "corrected_receipt" {
					if len(requests) != wantCalls {
						t.Fatalf("committed reply continued the turn: requests=%d", len(requests))
					}
				}
			})
		}
	}
}

func TestConversationPublicationCorrectionEligibility(t *testing.T) {
	for _, test := range []struct {
		content, reasoning, finish string
		want                       bool
	}{
		{"draft", "", "stop", true}, {"draft", "", "", true}, {"draft", "", "unknown", true},
		{"draft", "", "length", false}, {"draft", "", "content_filter", false}, {"draft", "", "tool_calls", false},
		{"", "reasoning", "stop", false}, {"", "reasoning", "length", false},
	} {
		message := &schema.Message{Content: test.content, ReasoningContent: test.reasoning, ResponseMeta: &schema.ResponseMeta{FinishReason: test.finish}}
		if got := conversationPublicationOmission(message); got != test.want {
			t.Errorf("%#v eligible=%v", test, got)
		}
	}
	if conversationPublicationOmission(&schema.Message{Content: "draft", ToolCalls: []schema.ToolCall{{ID: "real-call"}}}) {
		t.Fatal("real native call became correction")
	}
}

func TestConversationNativePublicationDoesNotCorrectNoneligibleFinals(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		for _, test := range []struct{ content, reasoning, finish string }{
			{"partial prose", "", "length"}, {"filtered", "", "content_filter"}, {"", "reasoning only", "stop"}, {"", "reasoning only", "length"}, {"", "reasoning only", ""},
		} {
			t.Run(fmt.Sprintf("stream=%v/%s/%s", streaming, test.finish, test.content), func(t *testing.T) {
				calls := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					var payload map[string]any
					_ = json.NewDecoder(r.Body).Decode(&payload)
					message := map[string]any{"role": "assistant", "content": test.content, "reasoning_content": test.reasoning}
					if payload["stream"] == true {
						w.Header().Set("Content-Type", "text/event-stream")
						fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", jsonString(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": message, "finish_reason": test.finish}}}))
					} else {
						_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": message, "finish_reason": test.finish}}})
					}
				}))
				defer server.Close()
				trace := &ADKCapabilityTrace{}
				ctx := withPhysicalModelRequestPolicy(withADKCapabilityContext(context.Background(), conversationPublicationTestInvoker{trace: trace}, trace, nil), conversationPublicationPhysicalRequestPolicy{})
				p := &ProviderClient{HTTP: server.Client()}
				_, err := p.generateWithEino(ctx, EinoModelCall{Assignment: providerAssignment{Role: "cognitive_assessment", BaseURL: server.URL, ModelID: "fake", Timeout: time.Second, TokenBudget: 4096, ContextWindowTokens: 131072, MaxInputTokens: 98304, PromptBudgetPolicyVersion: "prompt-budget.v1"}, Role: "cognitive_assessment", Messages: []map[string]any{{"role": "user", "content": "reply"}}, Definitions: []CapabilityDefinition{conversationReplyCapabilityDefinition()}, JSONMode: true, SchemaName: "conversation_turn_response", ResponseSchema: cognitiveTurnResponseSchema(), EnableStreaming: streaming})
				if err == nil || calls != 1 || len(trace.Results) != 0 {
					t.Fatalf("noneligible response retried or published: calls=%d results=%d err=%v", calls, len(trace.Results), err)
				}
			})
		}
	}
}
