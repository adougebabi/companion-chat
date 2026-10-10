package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestAgentRunRecordRetainsCommittedToolAfterCancellationAndPreventsReplay(t *testing.T) {
	ctx, repo, app, owner, fluctlight, conversation := setupDirectPublicationToolTest(t, "run-record")
	ctx = WithProviderCorrelation(ctx, "turn:run-record")
	definition, _ := FormalAgentDefinitionByID(FormalAgentConversationCognition)
	input := ADKStructuredTaskInput{AgentID: definition.ID, Prompt: PromptAssemblyResult{Messages: []map[string]any{{"role": "user", "content": "one immutable business request"}}}, Capability: &ADKCapabilityRequest{AuthorizationActorID: owner, FluctlightID: fluctlight, OperationID: "run-with-cancelled-result"}}
	record, previous, err := app.admitFormalRun(ctx, definition, input)
	if err != nil || record == nil || previous != nil {
		t.Fatalf("admit: record=%v previous=%v err=%v", record, previous, err)
	}
	receipt, err := app.ExecuteTool(ctx, ToolExecutionRequest{AgentID: definition.ID, RunID: record.RunID, CapabilityName: "conversation.reply", OperationID: "committed-reply", AuthorizationActorID: owner, FluctlightID: fluctlight, ConversationID: conversation, Arguments: json.RawMessage(`{"text":"已真实提交"}`)})
	if err != nil || receipt.Result.Status != "completed" {
		t.Fatalf("commit: %#v %v", receipt, err)
	}
	_, partial, err := app.admitFormalRun(ctx, definition, input)
	if err == nil || !strings.Contains(err.Error(), "agent_run_incomplete") || partial == nil || len(partial.Trace.Results) != 1 {
		t.Fatalf("interrupted admission lost committed fact: %#v %v", partial, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := app.finishFormalRun(cancelled, record, *partial, context.Canceled); err != nil {
		t.Fatalf("cancelled run fact recording: %v", err)
	}
	_, failed, replayErr := app.admitFormalRun(ctx, definition, input)
	if replayErr == nil || !strings.Contains(replayErr.Error(), "context canceled") || failed == nil || len(failed.Trace.Results) != 1 {
		t.Fatalf("failed run admission: %#v %v", failed, replayErr)
	}
	var status, correlationID, stage, code string
	var count int
	if err := repo.Pool().QueryRow(ctx, `SELECT status,correlation_id,failure_stage,failure_code FROM public.agent_runs WHERE run_id=$1`, record.RunID).Scan(&status, &correlationID, &stage, &code); err != nil || status != "failed" || correlationID != "turn:run-record" || stage != "cancellation" || code != "request_cancelled" {
		t.Fatalf("run diagnostic status=%q correlation=%q stage=%q code=%q: %v", status, correlationID, stage, code, err)
	}
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant'`, conversation).Scan(&count); err != nil || count != 1 {
		t.Fatalf("committed message count %d: %v", count, err)
	}
	changed := input
	changed.Prompt.Messages = []map[string]any{{"role": "user", "content": "different business input"}}
	if _, _, err := app.admitFormalRun(ctx, definition, changed); !errors.Is(err, ErrConflict) {
		t.Fatalf("reused run identity must conflict: %v", err)
	}
}

func TestClassifyAgentRunFailureUsesTypedBoundaries(t *testing.T) {
	for _, testCase := range []struct {
		name, stage, code string
		err               error
	}{
		{name: "tool", stage: "tool", code: "capability_prepare_failed", err: fmt.Errorf("adk_run: %w", &agentRunFailure{stage: "tool", code: "capability_prepare_failed", cause: errors.New("invalid arguments")})},
		{name: "cancelled", stage: "cancellation", code: "request_cancelled", err: context.Canceled},
		{name: "deadline", stage: "model", code: "request_timeout", err: context.DeadlineExceeded},
		{name: "provider", stage: "model", code: "provider_request_failed", err: fmt.Errorf("adk_run: %w", errProviderRequestFailed)},
		{name: "malformed provider tool call", stage: "model", code: "tool_call_invalid", err: fmt.Errorf("%w: %w", errProviderRequestFailed, newProviderToolCallNormalizationError(0, "id_required", errors.New("raw model call id")))},
		{name: "unsafe tool code", stage: "tool", code: "tool_execution_failed", err: &agentRunFailure{stage: "tool", code: "token=private", cause: errors.New("tool failure")}},
		{name: "agent", stage: "agent", code: "agent_run_failed", err: errors.New("invalid final DTO")},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			stage, code := classifyAgentRunFailure(testCase.err)
			if stage != testCase.stage || code != testCase.code {
				t.Fatalf("classification = %q/%q, want %q/%q", stage, code, testCase.stage, testCase.code)
			}
		})
	}
}

func TestAgentFailureCodeRejectsUnboundedOrSecretBearingValues(t *testing.T) {
	for _, unsafe := range []string{"token=private", strings.Repeat("x", 129), "UPPER_CASE", "foo bar"} {
		if got := safeAgentFailureCode(unsafe, "tool_execution_failed"); got != "tool_execution_failed" {
			t.Fatalf("unsafe code %q rendered as %q", unsafe, got)
		}
	}
}

func TestConversationFinalSchemaHasNoPublicationTextProtocol(t *testing.T) {
	properties := mapValue(cognitiveTurnResponseSchema()["properties"])
	if _, exists := properties["visible_text"]; exists {
		t.Fatalf("conversation final schema still exposes visible_text: %#v", properties)
	}
	if _, exists := mapValue(mapValue(properties["response_plan"])["properties"])["visible_text"]; exists {
		t.Fatalf("conversation response_plan still exposes visible_text: %#v", properties["response_plan"])
	}
}
