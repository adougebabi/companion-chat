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

type conversationPublicationTestInvoker struct {
	trace *ADKCapabilityTrace
}

func conversationWireForcesReply(payload map[string]any) bool {
	choice := payload["tool_choice"]
	if stringValue(choice) == "required" {
		return true
	}
	function := mapValue(mapValue(choice)["function"])
	return stringValue(function["name"]) == conversationReplyCapabilityName
}

func (i conversationPublicationTestInvoker) Execute(ctx context.Context, name, arguments string) (string, error) {
	return i.ExecuteWithID(ctx, "conversation-publication-call", name, arguments)
}

func (i conversationPublicationTestInvoker) ExecuteWithID(ctx context.Context, callID, name, arguments string) (string, error) {
	i.trace.Invocations = append(i.trace.Invocations, CapabilityInvocation{CallID: callID, CapabilityName: name, Arguments: json.RawMessage(arguments), SchemaVersion: CapabilityInvocationSchemaVersion, SourceFactID: "source", ProviderRequestID: "provider:" + callID})
	i.trace.Results = append(i.trace.Results, CapabilityResult{CallID: callID, CapabilityName: name, Status: "completed", Output: map[string]any{"target_kind": "conversation_message", "target_ref": "message-native"}})
	return `{"status":"completed","output":{"target_kind":"conversation_message"}}`, nil
}

func TestConversationPublicationPolicyRespectsCommittedOutputs(t *testing.T) {
	for _, result := range []CapabilityResult{
		{CapabilityName: "conversation.reply", Status: "completed", Output: map[string]any{"target_kind": "conversation_message", "target_ref": "message"}},
	} {
		trace := &ADKCapabilityTrace{Results: []CapabilityResult{result}}
		ctx := withADKCapabilityContext(context.Background(), conversationPublicationTestInvoker{trace: trace}, trace, nil)
		decision, err := (conversationPublicationPhysicalRequestPolicy{}).DecidePhysicalModelRequest(ctx)
		if err != nil || decision.ToolChoice != schema.ToolChoiceAllowed || decision.OmitResponseFormat {
			t.Fatal("already committed output requires another publication", result, decision, err)
		}
	}
	for _, result := range []CapabilityResult{
		{CapabilityName: "conversation.reply", Status: "failed"},
		{CapabilityName: "media.image.generate", Status: "accepted", Output: map[string]any{"media_intent_id": "image"}},
		{CapabilityName: "wardrobe.inspect", Status: "completed"},
	} {
		trace := &ADKCapabilityTrace{Results: []CapabilityResult{result}}
		ctx := withADKCapabilityContext(context.Background(), conversationPublicationTestInvoker{trace: trace}, trace, nil)
		decision, err := (conversationPublicationPhysicalRequestPolicy{}).DecidePhysicalModelRequest(ctx)
		if err != nil || decision.ToolChoice != schema.ToolChoiceForced || !decision.OmitResponseFormat {
			t.Fatal("non-output result permitted silent conversation completion", result, decision, err)
		}
	}
}

func TestConversationPhysicalRequestsForceNativePublicationGenerateAndStream(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		t.Run(fmt.Sprint(streaming), func(t *testing.T) {
			requests := make([]map[string]any, 0, 2)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var payload map[string]any
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Error(err)
					return
				}
				requests = append(requests, payload)
				message := map[string]any{"role": "assistant", "content": jsonString(map[string]any{"action_type": "reply", "response_intent": "published natively", "influences": []any{}})}
				finish := "stop"
				if len(requests) == 1 {
					message = map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"index": 0, "id": "native-reply", "type": "function", "function": map[string]any{"name": conversationReplyCapabilityName, "arguments": `{"text":"真实原生回复"}`}}}}
					finish = "tool_calls"
				}
				if payload["stream"] == true {
					w.Header().Set("Content-Type", "text/event-stream")
					fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", jsonString(map[string]any{"id": "response", "model": "test", "choices": []any{map[string]any{"index": 0, "delta": message, "finish_reason": finish}}}))
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"id": "response", "model": "test", "choices": []any{map[string]any{"index": 0, "message": message, "finish_reason": finish}}})
			}))
			defer server.Close()

			trace := &ADKCapabilityTrace{}
			ctx := withADKCapabilityContext(context.Background(), conversationPublicationTestInvoker{trace: trace}, trace, nil)
			ctx = withPhysicalModelRequestPolicy(ctx, conversationPublicationPhysicalRequestPolicy{})
			p := &ProviderClient{HTTP: server.Client()}
			response, err := p.generateWithEino(ctx, EinoModelCall{
				Assignment: providerAssignment{Role: "cognitive_assessment", BaseURL: server.URL, ModelID: "fake", Timeout: time.Second, TokenBudget: 4096, ContextWindowTokens: 131072, MaxInputTokens: 98304, PromptBudgetPolicyVersion: "prompt-budget.v1"},
				Role:       "cognitive_assessment", Scenario: "cognitive_assessment", Messages: []map[string]any{{"role": "user", "content": "reply please"}}, Definitions: []CapabilityDefinition{conversationReplyCapabilityDefinition()},
				JSONMode: true, SchemaName: "conversation_turn_response", ResponseSchema: cognitiveTurnResponseSchema(), EnableStreaming: streaming, ProviderRequestID: "native-output", CorrelationID: "turn:native-output",
			})
			if err != nil || response.Message == nil || len(requests) != 2 || len(trace.Results) != 1 {
				t.Fatalf("native publication loop failed: requests=%d results=%d response=%v err=%v", len(requests), len(trace.Results), response.Message, err)
			}
			if !conversationWireForcesReply(requests[0]) || requests[0]["response_format"] != nil {
				t.Fatalf("first request was not forced native execution: %#v", requests[0])
			}
			if requests[1]["tool_choice"] != "auto" || requests[1]["response_format"] == nil {
				t.Fatalf("final request did not restore final contract: %#v", requests[1])
			}
		})
	}
}

func TestConversationFinalOnlyResponseNeverPublishesOrRepairs(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		t.Run(fmt.Sprint(streaming), func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				var payload map[string]any
				_ = json.NewDecoder(r.Body).Decode(&payload)
				if !conversationWireForcesReply(payload) || payload["response_format"] != nil {
					t.Errorf("missing native publication was not forced: %#v", payload)
				}
				message := map[string]any{"role": "assistant", "content": jsonString(map[string]any{"action_type": "reply", "response_intent": "final-only text cannot publish", "influences": []any{}})}
				if payload["stream"] == true {
					w.Header().Set("Content-Type", "text/event-stream")
					fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", jsonString(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": message, "finish_reason": "stop"}}}))
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": message, "finish_reason": "stop"}}})
			}))
			defer server.Close()
			trace := &ADKCapabilityTrace{}
			ctx := withADKCapabilityContext(context.Background(), conversationPublicationTestInvoker{trace: trace}, trace, nil)
			ctx = withPhysicalModelRequestPolicy(ctx, conversationPublicationPhysicalRequestPolicy{})
			p := &ProviderClient{HTTP: server.Client()}
			_, err := p.generateWithEino(ctx, EinoModelCall{Assignment: providerAssignment{Role: "cognitive_assessment", BaseURL: server.URL, ModelID: "fake", Timeout: time.Second, TokenBudget: 4096, ContextWindowTokens: 131072, MaxInputTokens: 98304, PromptBudgetPolicyVersion: "prompt-budget.v1"}, Role: "cognitive_assessment", Messages: []map[string]any{{"role": "user", "content": "reply"}}, Definitions: []CapabilityDefinition{conversationReplyCapabilityDefinition()}, JSONMode: true, SchemaName: "conversation_turn_response", ResponseSchema: cognitiveTurnResponseSchema(), EnableStreaming: streaming})
			if err == nil || !strings.Contains(err.Error(), "conversation_native_output_missing") || calls != 1 || len(trace.Results) != 0 {
				t.Fatalf("final-only response became output or repair: calls=%d results=%d err=%v", calls, len(trace.Results), err)
			}
		})
	}
}

func TestPostRunFailureRetainsOuterBoundaryAndSpecificCauses(t *testing.T) {
	for _, c := range []struct {
		stage, code         string
		cause               error
		wantStage, wantCode string
	}{
		{"output_publication", "agent_output_publication_failed", errors.New("cognition_visible_text_missing"), "output_publication", "agent_output_publication_failed"},
		{"settlement", "agent_cognition_settlement_failed", ErrCurrentFactsStale, "settlement", "agent_cognition_settlement_failed"},
		{"output_publication", "agent_output_publication_failed", errProviderRequestFailed, "model", "provider_request_failed"},
		{"settlement", "agent_cognition_settlement_failed", context.Canceled, "cancellation", "request_cancelled"},
	} {
		stage, code := classifyAgentPostRunFailure(c.stage, c.code, c.cause)
		if stage != c.wantStage || code != c.wantCode {
			t.Fatal("failure boundary lost", stage, code, c.wantStage, c.wantCode)
		}
	}
}

func TestCurrentFactsMismatchPreservesStaleContract(t *testing.T) {
	err := &currentFactsMismatch{Expected: "facts_gen_1", Actual: "facts_gen_2", Boundary: "settlement"}
	if !errors.Is(err, ErrCurrentFactsStale) || err.Error() != "current_facts_stale" {
		t.Fatal("diagnostic mismatch relaxed or changed the stale contract", err)
	}
	stage, code := classifyAgentPostRunFailure("settlement", "agent_cognition_settlement_failed", err)
	if stage != "settlement" || code != "agent_cognition_settlement_failed" {
		t.Fatal("fact mismatch lost settlement boundary", stage, code)
	}
}

func TestCurrentFactsGenerationIntervalBoundsBothDirections(t *testing.T) {
	for _, test := range []struct {
		expected, actual string
		lower, upper     int64
	}{
		{expected: "facts_gen_12", actual: "facts_gen_19", lower: 12, upper: 19},
		{expected: "facts_gen_19", actual: "facts_gen_12", lower: 12, upper: 19},
		{expected: " facts_gen_4 ", actual: "facts_gen_4", lower: 4, upper: 4},
	} {
		lower, upper, err := currentFactsGenerationIntervalBounds(test.expected, test.actual)
		if err != nil || lower != test.lower || upper != test.upper {
			t.Fatalf("bounds(%q,%q)=(%d,%d,%v), want (%d,%d)", test.expected, test.actual, lower, upper, err, test.lower, test.upper)
		}
	}
	for _, revision := range []string{"", "facts_1", "facts_gen_-1", "facts_gen_bad"} {
		if _, err := currentFactsGenerationNumber(revision); err == nil {
			t.Fatalf("invalid revision %q was accepted", revision)
		}
	}
	for _, test := range []struct {
		lower, upper, observed, minimum, maximum int64
		want                                     bool
	}{
		{lower: 12, upper: 19, observed: 7, minimum: 13, maximum: 19, want: true},
		{lower: 12, upper: 19, observed: 6, minimum: 14, maximum: 19, want: false},
		{lower: 19, upper: 19, observed: 0, minimum: 0, maximum: 0, want: true},
	} {
		if got := currentFactsGenerationCoverageComplete(test.lower, test.upper, test.observed, test.minimum, test.maximum); got != test.want {
			t.Fatalf("coverage(%d,%d,%d,%d,%d)=%v, want %v", test.lower, test.upper, test.observed, test.minimum, test.maximum, got, test.want)
		}
	}
}
