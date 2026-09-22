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
	tag, err := a.DB.Pool().Exec(ctx, `INSERT INTO public.agent_runs(fluctlight_id,agent_id,run_id,input_digest,status) VALUES($1,$2,$3,$4,'running') ON CONFLICT DO NOTHING`, record.FluctlightID, record.AgentID, record.RunID, digest)
	if err != nil {
		return nil, nil, fmt.Errorf("record Agent admission: %w", err)
	}
	if tag.RowsAffected() == 1 {
		return record, nil, nil
	}
	var previousDigest, status, failure string
	var encoded []byte
	if err := a.DB.Pool().QueryRow(ctx, `SELECT input_digest,status,error_detail,result FROM public.agent_runs WHERE fluctlight_id=$1 AND agent_id=$2 AND run_id=$3`, record.FluctlightID, record.AgentID, record.RunID).Scan(&previousDigest, &status, &failure, &encoded); err != nil {
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
	if runErr != nil {
		status = "failed"
		failure = strings.TrimSpace(runErr.Error())
		if len(failure) > 4096 {
			failure = failure[:4096]
		}
	}
	tag, err := a.DB.Pool().Exec(ctx, `UPDATE public.agent_runs SET status=$4,error_detail=$5,result=$6,finished_at=now() WHERE fluctlight_id=$1 AND agent_id=$2 AND run_id=$3 AND status='running'`, record.FluctlightID, record.AgentID, record.RunID, status, failure, jsonBytes(result))
	if err != nil {
		return fmt.Errorf("record Agent result: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return ErrConflict
	}
	return nil
}
