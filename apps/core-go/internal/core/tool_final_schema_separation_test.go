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

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// Controlled transport regression. Native calls execute the test invoker and
// return its receipts through Eino; no database or production publication occurs.
func TestToolFinalSchemaSeparationPhysicalRequests(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		for _, policy := range []bool{false, true} {
			for _, outcome := range []string{"valid", "repair", "invalid_repair", "malformed", "malformed_invalid_repair"} {
				t.Run(fmt.Sprintf("stream=%t/policy=%t/%s", streaming, policy, outcome), func(t *testing.T) {
					var requests []map[string]any
					seen := map[string]bool{}
					server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						var payload map[string]any
						if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
							t.Error(err)
							return
						}
						requests = append(requests, payload)
						id := r.Header.Get("X-Fluctlight-Provider-Request-Id")
						if id == "" || id != r.Header.Get("Idempotency-Key") || seen[id] {
							t.Error("missing/reused physical identity", id)
						}
						seen[id] = true
						if boolValue(payload["stream"]) != streaming {
							t.Error("wrong streaming wire")
						}
						tools := arrayValue(payload["tools"])
						if len(tools) > 0 && payload["tool_choice"] == "auto" && payload["response_format"] != nil {
							w.WriteHeader(http.StatusBadRequest)
							_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"message": "Cannot combine structured output constraints with lazy grammar"}})
							return
						}
						n := len(requests)
						message := map[string]any{"role": "assistant", "content": `{"action_type":"reply","response_intent":"published","influences":[]}`}
						finish := "stop"
						if n <= 2 {
							name, callID, args := conversationReplyCapabilityName, "native-reply", `{"text":"one published reply"}`
							if n == 2 {
								name, callID, args = "memory.recall", "optional-read", `{"query":"after publication"}`
							}
							message = map[string]any{"role": "assistant", "tool_calls": fakeProviderNativeToolCalls([]map[string]any{{"call_id": callID, "capability_name": name, "arguments": decodeObject([]byte(args))}})}
							finish = "tool_calls"
						}
						if n == 3 && outcome != "valid" {
							message["content"] = `{"action_type":42}`
						}
						if n == 3 && strings.HasPrefix(outcome, "malformed") {
							message["content"] = `{"action_type":`
						}
						if n == 4 && strings.HasSuffix(outcome, "invalid_repair") {
							message["content"] = `{"action_type":42}`
						}
						if n <= 3 {
							if len(tools) != 2 || payload["response_format"] != nil {
								t.Error("native catalog/final schema separation lost")
							}
							if policy && ((n == 1 && payload["tool_choice"] != "required") || (n > 1 && payload["tool_choice"] != "auto")) {
								t.Error("task-owned choice changed", payload["tool_choice"])
							}
							if payload["enable_thinking"] != true {
								t.Error("business thinking policy changed")
							}
						} else {
							if n != 4 || len(tools) != 0 || payload["tool_choice"] != nil || payload["parallel_tool_calls"] != nil || payload["response_format"] == nil || payload["enable_thinking"] != false {
								t.Error("repair must be isolated with original schema", payload)
							}
						}
						if n >= 2 {
							system := stringValue(mapValue(arrayValue(payload["messages"])[0])["content"])
							wantProtocol := factOnlySettlementProtocol
							if n == 4 {
								wantProtocol = factOnlyFinalRepairProtocol
							}
							if !strings.Contains(system, wantProtocol) || strings.Contains(system, "普通私聊必须真实调用") || !strings.Contains(system, "KEEP_TEST_PERSONA") {
								t.Error("physical phase protocol/persona incorrect")
							}
							receipt, found := formalAdapterResultFromPayload(payload, "native-reply")
							if !found || receipt["status"] != "completed" {
								t.Error("real paired reply receipt missing", receipt)
							}
						}
						if n >= 3 {
							receipt, found := formalAdapterResultFromPayload(payload, "optional-read")
							if !found || receipt["status"] != "completed" {
								t.Error("optional read receipt missing", receipt)
							}
						}
						if streaming {
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
					observations := []map[string]any{}
					trace := &ADKCapabilityTrace{}
					ctx := withADKCapabilityContext(context.Background(), conversationPublicationTestInvoker{trace: trace}, trace, nil)
					ctx = context.WithValue(ctx, physicalRequestObservationKey{}, physicalRequestObservation(func(_ context.Context, config map[string]any) { observations = append(observations, cloneMap(config)) }))
					if policy {
						ctx = withPhysicalModelRequestPolicy(ctx, conversationPublicationPhysicalRequestPolicy{})
					}
					result, err := (&ProviderClient{HTTP: server.Client()}).generateWithEino(ctx, EinoModelCall{Assignment: providerAssignment{Role: "cognitive_assessment", BaseURL: server.URL, ModelID: "fake", Timeout: time.Second, TokenBudget: 4096}, Role: "cognitive_assessment", Messages: []map[string]any{{"role": "system", "content": factOnlyCognitionProtocol + "\n" + factOnlyConversationPolicy + "\n# 人格设定\nKEEP_TEST_PERSONA"}, {"role": "user", "content": "reply then optionally read"}}, Definitions: []CapabilityDefinition{conversationReplyCapabilityDefinition(), {Name: "memory.recall", Description: "Read memory", InputSchema: objectSchema(map[string]any{"query": stringSchema()}, []string{"query"}, false)}}, JSONMode: true, SchemaName: "conversation_turn_response", ResponseSchema: cognitiveTurnResponseSchema(), EnableThinking: true, EnableStreaming: streaming, CorrelationID: "schema-separation"})
					wantRequests := 3
					if outcome != "valid" {
						wantRequests = 4
					}
					if strings.HasSuffix(outcome, "invalid_repair") {
						if err == nil || !strings.Contains(err.Error(), "adk_final_repair_invalid") {
							t.Fatalf("strict invalid repair accepted: %v", err)
						}
					} else if err != nil || result.Message == nil || len(result.ExecutedToolCalls) != 2 {
						t.Fatalf("native loop failed: %v", err)
					}
					if len(requests) != wantRequests || len(trace.Results) != 2 || len(trace.Invocations) != 2 {
						t.Fatalf("effect replay or premature final: requests=%d results=%d invocations=%d", len(requests), len(trace.Results), len(trace.Invocations))
					}
					if len(observations) != len(requests) {
						t.Fatalf("physical configuration missing: %d/%d", len(observations), len(requests))
					}
					for index, observed := range observations {
						if jsonString(observed) != jsonString(physicalRequestConfiguration(requests[index])) {
							t.Fatalf("observed configuration differs from wire request %d", index)
						}
						if observed["messages"] != nil || observed["response_format"] != nil {
							t.Fatal("private payload copied into config observation")
						}
					}
					replies := 0
					for _, invocation := range trace.Invocations {
						if invocation.CapabilityName == conversationReplyCapabilityName {
							replies++
						}
						identity, ok := trace.ModelIdentity(invocation.CallID)
						if !ok || !seen[identity.ProviderRequestID] {
							t.Error("native physical provenance lost", invocation.CallID, identity)
						}
					}
					if replies != 1 {
						t.Fatal("reply effect count", replies)
					}
				})
			}
		}
	}
}

// The effective format feeds both prompt budgeting and physical diagnostics.
// Check catalog options rather than only task definitions or policy presence.
func TestToolFinalSchemaSeparationEffectiveFormat(t *testing.T) {
	format := map[string]any{"type": "json_schema"}
	tools := []*schema.ToolInfo{{Name: "read"}}
	for _, test := range []struct {
		name    string
		bound   []*schema.ToolInfo
		opts    []model.Option
		omitted bool
	}{
		{name: "bound_catalog", bound: tools, omitted: true},
		{name: "option_catalog", opts: []model.Option{model.WithTools(tools)}, omitted: true},
		{name: "empty_override", bound: tools, opts: []model.Option{model.WithTools([]*schema.ToolInfo{})}},
		{name: "tool_free"},
	} {
		t.Run(test.name, func(t *testing.T) {
			queued := &queuedToolCallingChatModel{toolInfos: test.bound, responseFormat: format}
			_, _, effective, err := queued.physicalRequestOptions(context.Background(), test.opts)
			if err != nil || (effective == nil) != test.omitted {
				t.Fatalf("effective format=%#v err=%v", effective, err)
			}
			if queued.responseFormat["type"] != "json_schema" {
				t.Fatal("original final schema mutated")
			}
		})
	}
	queued := &queuedToolCallingChatModel{toolInfos: tools, responseFormat: format}
	called := false
	ctx := withPhysicalModelRequestPolicy(context.Background(), physicalModelRequestPolicyFunc(func(context.Context) (physicalModelRequestDecision, error) {
		called = true
		return physicalModelRequestDecision{}, nil
	}))
	_, opts, effective, err := queued.physicalRequestOptions(finalRepairContext(ctx), []model.Option{model.WithTools(tools)})
	if err != nil || called || effective["type"] != "json_schema" || len(model.GetCommonOptions(nil, opts...).Tools) != 0 {
		t.Fatalf("repair format/tools/policy isolation lost: %#v %v", effective, err)
	}
}
