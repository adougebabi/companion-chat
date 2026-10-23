package core

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
)

func TestADKCompletionTimeoutBelongsToEachPhysicalCall(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		select {
		case <-time.After(150 * time.Millisecond):
		case <-r.Context().Done():
			return
		}
		content := `{"influences":{}}`
		if n > 1 {
			content = `{"influences":[]}`
		}
		response := map[string]any{"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": content}}}}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()
	assignment := providerAssignment{Role: "cognitive_assessment", BaseURL: server.URL, ModelID: "fake", Timeout: 250 * time.Millisecond, TokenBudget: 128, ContextWindowTokens: 65536, MaxInputTokens: 49152, PromptBudgetPolicyVersion: promptBudgetPolicyVersionV1}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	ctx = withFrozenProviderAssignment(ctx, "cognitive_assessment", assignment)
	definition, _ := FormalAgentDefinitionByID(FormalAgentConversationCognition)
	ctx = withFormalAgentDefinition(ctx, definition)
	ctx = WithADKCapabilityInvoker(ctx, &adkFakeInvoker{}, &ADKCapabilityTrace{})
	output := objectSchema(map[string]any{"influences": map[string]any{"type": "array", "items": map[string]any{"type": "object"}}}, []string{"influences"}, false)
	completion, err := (&ProviderClient{HTTP: server.Client()}).AgentAssembledCompletion(ctx, "cognitive_assessment", []map[string]any{{"role": "system", "content": "Return the required JSON contract"}, {"role": "user", "content": "Return an empty evidence list"}}, nil, "timeout_recovery", output, true, false)
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 || completion.Structured == nil {
		t.Fatalf("calls=%d completion=%#v", calls.Load(), completion)
	}
}

func TestProviderCompletionContextPreservesCallerAndSingleCallDeadlines(t *testing.T) {
	for _, adk := range []bool{false, true} {
		parent, cancelParent := context.WithCancel(context.Background())
		ctx, cancel := providerCompletionContext(parent, time.Millisecond, adk)
		if _, has := ctx.Deadline(); has == adk {
			t.Fatalf("adk=%t has deadline=%t", adk, has)
		}
		cancelParent()
		if !errors.Is(ctx.Err(), context.Canceled) {
			t.Fatal(ctx.Err())
		}
		cancel()
	}
}

func TestADKFinalContractEmptyEvidencePreservesInvalidNonemptyValues(t *testing.T) {
	output := objectSchema(map[string]any{"influences": map[string]any{"type": "array", "items": map[string]any{"type": "object"}}}, []string{"influences"}, false)
	for _, value := range []string{`{}`, `{"influences":null}`, `{"influences":[]}`} {
		if err := validateADKFinalContract(schema.AssistantMessage(value, nil), "cognitive_assessment", output, nil); err != nil {
			t.Fatalf("%s: %v", value, err)
		}
	}
	for _, value := range []string{`{"influences":{}}`, `{"influences":"none"}`, `{"influences":[{"ref":"invented"}]}`} {
		if err := validateADKFinalContract(schema.AssistantMessage(value, nil), "cognitive_assessment", output, nil); err == nil {
			t.Fatalf("invalid evidence accepted: %s", value)
		}
	}
	other := objectSchema(map[string]any{"answer": stringSchema()}, []string{"answer"}, false)
	if err := validateADKFinalContract(schema.AssistantMessage(`{"answer":"ok"}`, nil), "cognitive_assessment", other, nil); err != nil {
		t.Fatal(err)
	}
}

func TestOwnerRequestFailureReasonAndNotificationDoNotExposeRawErrors(t *testing.T) {
	for _, err := range []error{context.DeadlineExceeded, context.Canceled, ErrConflict, errors.New("prompt_required_budget_exceeded estimate=53091 max=49152 secret-token"), errors.New("adk_final_output_invalid: influences must be an array secret-token"), errors.New("sql secret-token")} {
		reason := ownerRequestFailureReason(err)
		if reason == "" || strings.Contains(reason, "secret-token") {
			t.Fatal(reason)
		}
		title, body, _, handled := notificationContent(NotificationSource{Kind: "owner.request.settled", Payload: map[string]any{"status": "failed", "request_id": "req", "failure_reason": reason}})
		if handled || title != "" || body != "" {
			t.Fatal(title, body, handled)
		}
	}
}

func TestADKPhysicalTimeoutAndCallerDeadlineRemainEnforced(t *testing.T) {
	for _, tc := range []struct {
		name                    string
		delay, physical, parent time.Duration
		calls                   int32
	}{
		{"slow physical request", 200 * time.Millisecond, 80 * time.Millisecond, 2 * time.Second, 1},
		{"caller expires in repair", 150 * time.Millisecond, time.Second, 250 * time.Millisecond, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				n := calls.Add(1)
				select {
				case <-time.After(tc.delay):
				case <-r.Context().Done():
					return
				}
				content := `{"influences":{}}`
				if n > 1 {
					content = `{"influences":[]}`
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": content}}}})
			}))
			defer server.Close()
			ctx, cancel := context.WithTimeout(context.Background(), tc.parent)
			defer cancel()
			assignment := providerAssignment{Role: "cognitive_assessment", BaseURL: server.URL, ModelID: "fake", Timeout: tc.physical, TokenBudget: 128, ContextWindowTokens: 65536, MaxInputTokens: 49152, PromptBudgetPolicyVersion: promptBudgetPolicyVersionV1}
			ctx = withFrozenProviderAssignment(ctx, "cognitive_assessment", assignment)
			definition, _ := FormalAgentDefinitionByID(FormalAgentConversationCognition)
			ctx = withFormalAgentDefinition(ctx, definition)
			ctx = WithADKCapabilityInvoker(ctx, &adkFakeInvoker{}, &ADKCapabilityTrace{})
			output := objectSchema(map[string]any{"influences": map[string]any{"type": "array", "items": map[string]any{"type": "object"}}}, []string{"influences"}, false)
			_, err := (&ProviderClient{HTTP: server.Client()}).AgentAssembledCompletion(ctx, "cognitive_assessment", []map[string]any{{"role": "system", "content": "Return the required JSON contract"}, {"role": "user", "content": "Return an empty evidence list"}}, nil, "timeout_recovery", output, true, false)
			if err == nil || calls.Load() != tc.calls {
				t.Fatalf("calls=%d err=%v", calls.Load(), err)
			}
		})
	}
}

func TestToolNameCanonicalizationOwnsItsMessageAndCalls(t *testing.T) {
	original := schema.AssistantMessage("", []schema.ToolCall{{ID: "native", Function: schema.FunctionCall{Name: "memory__recall", Arguments: "{}"}}})
	before := original.ToolCalls[0].Function.Name
	canonical := canonicalizeModelToolNames(original)
	if canonical == original || &canonical.ToolCalls[0] == &original.ToolCalls[0] {
		t.Fatal("canonicalization retained mutable Provider ownership")
	}
	if original.ToolCalls[0].Function.Name != before {
		t.Fatal("Provider message was mutated")
	}
	canonical.ToolCalls[0].Function.Name = "other"
	if original.ToolCalls[0].Function.Name != before {
		t.Fatal("ToolCalls slice aliases the original")
	}
}
