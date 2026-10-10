package core

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestConversationFinalWithoutOutputGetsOneToolFreeRepair(t *testing.T) {
	for _, missing := range []string{"omitted", "empty", "whitespace"} {
		t.Run(missing, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				var payload map[string]any
				_ = json.NewDecoder(r.Body).Decode(&payload)
				output := map[string]any{"action_type": "reply", "response_intent": "reply", "influences": []any{}}
				if missing == "empty" {
					output["visible_text"] = ""
				} else if missing == "whitespace" {
					output["visible_text"] = " \n\t"
				}
				if calls == 2 {
					if len(arrayValue(payload["tools"])) != 0 || !strings.Contains(jsonString(payload["messages"]), "nonempty visible_text") {
						t.Error("repair may execute tools or lacks visible-output feedback")
					}
					output["visible_text"] = "这是实际要发送的回复。"
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": jsonString(output)}}}})
			}))
			defer server.Close()
			trace := &ADKCapabilityTrace{}
			ctx := WithADKCapabilityInvoker(context.Background(), adkTraceInvoker{trace: trace}, trace)
			p := &ProviderClient{HTTP: server.Client()}
			response, err := p.generateWithEino(ctx, EinoModelCall{Assignment: providerAssignment{Role: "cognitive_assessment", BaseURL: server.URL, ModelID: "fake", Timeout: time.Second, TokenBudget: 4096}, Role: "cognitive_assessment", Scenario: "cognitive_assessment", Messages: []map[string]any{{"role": "user", "content": "reply please"}}, JSONMode: true, SchemaName: "conversation_turn_response", ResponseSchema: cognitiveTurnResponseSchema(), ProviderRequestID: "missing-output", CorrelationID: "turn:missing-output"})
			if err != nil || calls != 2 || response.Message == nil || !strings.Contains(response.Message.Content, "这是实际要发送的回复") {
				t.Fatalf("reply omission was not repaired: calls=%d response=%v err=%v", calls, response.Message, err)
			}
		})
	}
}

func TestConversationVisibleFinalRequirementRespectsCommittedOutputs(t *testing.T) {
	call := EinoModelCall{SchemaName: "conversation_turn_response", ResponseSchema: cognitiveTurnResponseSchema()}
	for _, result := range []CapabilityResult{
		{CapabilityName: "conversation.reply", Status: "completed", Output: map[string]any{"target_kind": "conversation_message", "target_ref": "message"}},
		{CapabilityName: "media.image.generate", Status: "accepted", Output: map[string]any{"media_intent_id": "image"}},
	} {
		if conversationNeedsVisibleFinal(call, &ADKCapabilityTrace{Results: []CapabilityResult{result}}) {
			t.Fatal("already committed output requires another reply", result)
		}
	}
	for _, result := range []CapabilityResult{
		{CapabilityName: "conversation.reply", Status: "failed"},
		{CapabilityName: "wardrobe.inspect", Status: "completed"},
	} {
		if !conversationNeedsVisibleFinal(call, &ADKCapabilityTrace{Results: []CapabilityResult{result}}) {
			t.Fatal("non-output result permitted silent conversation completion", result)
		}
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
