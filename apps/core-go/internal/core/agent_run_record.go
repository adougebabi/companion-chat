package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	aiagent "github.com/fluctlight/local-ai-companion/apps/core-go/internal/ai/agent"
)

type formalRunRecord struct {
	FluctlightID string
	AgentID      FormalAgentID
	RunID        string
}

type agentRunFailure struct {
	stage string
	code  string
	cause error
}

func (failure *agentRunFailure) Error() string { return failure.cause.Error() }
func (failure *agentRunFailure) Unwrap() error { return failure.cause }

func safeAgentFailureCode(value, fallback string) string {
	value = strings.TrimSpace(value)
	if !validLifecycleToken(value, 128, true) {
		return fallback
	}
	return value
}

func classifyAgentPostRunFailure(stage, code string, cause error) (string, string) {
	failureStage, failureCode := classifyAgentRunFailure(cause)
	if failureCode == "agent_run_failed" {
		failureCode = safeAgentFailureCode(code, "agent_run_failed")
		if failureStage == "agent" {
			failureStage = stage
		}
	}
	return failureStage, failureCode
}

func classifyAgentRunFailure(err error) (string, string) {
	if err == nil {
		return "", ""
	}
	if errors.Is(err, context.Canceled) {
		return "cancellation", "request_cancelled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "model", "request_timeout"
	}
	var empty *aiagent.EmptyFinalMessageError
	if errors.As(err, &empty) {
		return "model_output", "adk_final_message_empty"
	}
	var typed *agentRunFailure
	if errors.As(err, &typed) {
		return typed.stage, safeAgentFailureCode(typed.code, "tool_execution_failed")
	}
	if strings.Contains(err.Error(), "working_memory_required_budget_exceeded") {
		return "model_input", "working_memory_required_budget_exceeded"
	}
	if strings.Contains(err.Error(), "decision_influence_") && strings.Contains(err.Error(), "_ref_unknown") {
		return "model_output", "decision_influence_ref_unknown"
	}
	if errors.Is(err, ErrPromptOutputReserveConflict) {
		return "model_input", "prompt_output_reserve_conflict"
	}
	if errors.Is(err, ErrPromptToolResultBudgetExceeded) {
		return "model_input", "prompt_tool_result_budget_exceeded"
	}
	if errors.Is(err, ErrPromptCurrentInputBudgetExceeded) {
		return "model_input", "prompt_current_input_budget_exceeded"
	}
	if errors.Is(err, ErrPromptRequiredBudgetExceeded) {
		return "model_input", "prompt_required_budget_exceeded"
	}
	if providerToolCallInvalidError(err) {
		return "model", "tool_call_invalid"
	}
	for _, code := range []string{"adk_structured_response_invalid", "adk_final_output_invalid", "provider_context_ref_alias_unknown"} {
		if strings.Contains(err.Error(), code) {
			return "model_output", code
		}
	}
	if errors.Is(err, errProviderPaused) {
		return "model", "provider_suppressed_fluctlight_paused"
	}
	if errors.Is(err, errProviderInactive) {
		return "model", "provider_suppressed_fluctlight_inactive"
	}
	if errors.Is(err, errProviderRequestFailed) {
		return "model", "provider_request_failed"
	}
	return "agent", "agent_run_failed"
}

// Admission is durable before any model/tool work. An interrupted run can be
// inspected, but cannot silently start its decision loop again with new calls.
func (a *App) admitFormalRun(ctx context.Context, definition FormalAgentDefinition, input ADKStructuredTaskInput) (*formalRunRecord, *ADKStructuredTaskResult, error) {
	if input.Capability == nil || a.DB == nil || a.DB.Pool() == nil {
		return nil, nil, nil
	}
	request := input.Capability
	runID := firstString(request.OperationID, firstString(request.ActionID, request.SourceFactID))
	if runID == "" || request.FluctlightID == "" {
		return nil, nil, nil
	}
	actorID := firstString(request.AuthorizationActorID, request.Projection.OwnerActorID)
	if _, err := a.DB.GetFluctlight(ctx, request.FluctlightID, actorID); err != nil {
		return nil, nil, err
	}
	digest := stableDigest(jsonString(map[string]any{
		"messages": input.Prompt.Messages, "schema": input.Prompt.ResponseFormat, "agent": definition.ID, "actor": actorID,
		"tools": input.Definitions, "role": input.Role, "schema_name": input.SchemaName,
		"subject": request.SubjectActorID, "conversation": request.ConversationID,
		"target_kind": request.TargetKind, "target_ref": request.TargetRef,
		"authorization_policy": request.AuthorizationPolicy, "source_fact": request.SourceFactID,
	}))
	record := &formalRunRecord{request.FluctlightID, definition.ID, runID}
	correlationID := firstString(providerCorrelation(ctx), request.CorrelationID)
	tag, err := a.DB.Pool().Exec(ctx, `INSERT INTO public.agent_runs(fluctlight_id,agent_id,run_id,input_digest,correlation_id,status) VALUES($1,$2,$3,$4,$5,'running') ON CONFLICT DO NOTHING`, record.FluctlightID, record.AgentID, record.RunID, digest, correlationID)
	if err != nil {
		return nil, nil, fmt.Errorf("record Agent admission: %w", err)
	}
	if tag.RowsAffected() == 1 {
		return record, nil, nil
	}
	return a.restoreRecordedAgentRun(ctx, record, digest)
}

func (a *App) finishFormalRun(ctx context.Context, record *formalRunRecord, result ADKStructuredTaskResult, runErr error) error {
	if record == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	status, failure := "completed", ""
	failureStage, failureCode := classifyAgentRunFailure(runErr)
	if runErr != nil {
		status = "failed"
		failure = boundedOperationalCause(runErr.Error())
	}
	safeResult := sanitizeAgentRunResult(result)
	tag, err := a.DB.Pool().Exec(ctx, `UPDATE public.agent_runs SET status=$4,error_detail=$5,failure_stage=$6,failure_code=$7,result=$8,finished_at=now() WHERE fluctlight_id=$1 AND agent_id=$2 AND run_id=$3 AND status='running'`, record.FluctlightID, record.AgentID, record.RunID, status, failure, failureStage, failureCode, jsonBytes(safeResult))
	if err != nil {
		return fmt.Errorf("record Agent result: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return ErrConflict
	}
	return nil
}

func sanitizeAgentRunResult(result ADKStructuredTaskResult) ADKStructuredTaskResult {
	safe := result
	safe.Trace = result.Trace.Clone()
	if safe.Trace == nil {
		return safe
	}
	for index := range safe.Trace.Results {
		item := &safe.Trace.Results[index]
		if item.Status == "failed" || item.Status == "rejected" || item.Status == "handled" {
			item.Output = redactDiagnosticFailureOutput(item.Output)
		}
	}
	return safe
}
