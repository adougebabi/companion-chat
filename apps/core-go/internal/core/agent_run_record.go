package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
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
	var previousDigest, status, failure string
	var encoded []byte
	var startedAt time.Time
	if err := a.DB.Pool().QueryRow(ctx, `SELECT input_digest,status,error_detail,result,started_at FROM public.agent_runs WHERE fluctlight_id=$1 AND agent_id=$2 AND run_id=$3`, record.FluctlightID, record.AgentID, record.RunID).Scan(&previousDigest, &status, &failure, &encoded, &startedAt); err != nil {
		return nil, nil, err
	}
	result := &ADKStructuredTaskResult{Trace: &ADKCapabilityTrace{}}
	if len(encoded) > 0 && string(encoded) != "{}" {
		if err := json.Unmarshal(encoded, result); err != nil {
			return nil, result, fmt.Errorf("decode Agent result: %w", err)
		}
	}
	if previousDigest != digest {
		return nil, result, newCapabilityError("agent_run_input_conflict", false, ErrConflict)
	}
	if status == "running" {
		if time.Since(startedAt) > 5*time.Minute {
			_, _ = a.DB.Pool().Exec(ctx, `UPDATE public.agent_runs SET status='failed',error_detail='stale_run_timeout',failure_stage='timeout',failure_code='request_timeout',finished_at=now() WHERE fluctlight_id=$1 AND agent_id=$2 AND run_id=$3 AND status='running'`, record.FluctlightID, record.AgentID, record.RunID)
			return nil, result, errors.New("agent_run_failed: stale_run_timeout")
		}
		rows, err := a.DB.Pool().Query(ctx, `SELECT invocation,result FROM public.tool_executions WHERE fluctlight_id=$1 AND agent_id=$2 AND run_id=$3 ORDER BY committed_at,operation_id`, record.FluctlightID, record.AgentID, record.RunID)
		if err != nil {
			return nil, result, err
		}
		defer rows.Close()
		for rows.Next() {
			var invocationJSON, resultJSON []byte
			if err := rows.Scan(&invocationJSON, &resultJSON); err != nil {
				return nil, result, err
			}
			var invocation CapabilityInvocation
			var receipt CapabilityResult
			if err := json.Unmarshal(invocationJSON, &invocation); err != nil {
				return nil, result, err
			}
			if err := json.Unmarshal(resultJSON, &receipt); err != nil {
				return nil, result, err
			}
			result.Trace.Append(invocation, receipt)
		}
		if err := rows.Err(); err != nil {
			return nil, result, err
		}
		return nil, result, errors.New("agent_run_incomplete: inspect committed results before starting a new business run")
	}
	if status == "failed" {
		return nil, result, fmt.Errorf("agent_run_failed: %s", failure)
	}
	if status != "completed" {
		return nil, result, errors.New("agent_run_status_invalid")
	}
	return nil, result, nil
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
		failure = strings.TrimSpace(runErr.Error())
		if len(failure) > 4096 {
			failure = failure[:4096]
		}
	}
	tag, err := a.DB.Pool().Exec(ctx, `UPDATE public.agent_runs SET status=$4,error_detail=$5,failure_stage=$6,failure_code=$7,result=$8,finished_at=now() WHERE fluctlight_id=$1 AND agent_id=$2 AND run_id=$3 AND status='running'`, record.FluctlightID, record.AgentID, record.RunID, status, failure, failureStage, failureCode, jsonBytes(result))
	if err != nil {
		return fmt.Errorf("record Agent result: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return ErrConflict
	}
	return nil
}
