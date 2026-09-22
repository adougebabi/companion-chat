package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type toolExecutionReader interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

// ToolAuthorityRevisions records the authoritative Core revisions observed in
// the same transaction that committed a Tool mutation. Agent settlement uses
// these values to distinguish this run's own writes from unrelated concurrent
// updates; reading whatever revision happens to be current at settlement time
// would silently disable the stale-snapshot guard.
type ToolAuthorityRevisions struct {
	Before       *ToolAuthorityRevisions `json:"before,omitempty"`
	Foundation   int                     `json:"foundation_revision"`
	CurrentState int                     `json:"current_state_revision"`
	LifeContext  string                  `json:"life_context_revision"`
}

func toolExecutionDigest(request ToolExecutionRequest) string {
	var arguments map[string]any
	_ = json.Unmarshal(request.Arguments, &arguments)
	return stableDigest(jsonString(map[string]any{
		"authorization_actor_id": request.AuthorizationActorID, "subject_actor_id": request.SubjectActorID, "conversation_id": request.ConversationID,
		"target_kind": request.TargetKind, "target_ref": request.TargetRef,
		"evidence_id": request.EvidenceID, "working_profile_id": request.WorkingProfileID, "arguments": arguments,
	}))
}

func readToolExecution(ctx context.Context, reader toolExecutionReader, request ToolExecutionRequest) (CapabilityResult, ToolAuthorityRevisions, bool, error) {
	var digest string
	var encoded, invocationEncoded []byte
	err := reader.QueryRow(ctx, `SELECT request_digest,result,invocation FROM public.tool_executions WHERE fluctlight_id=$1 AND capability_name=$2 AND operation_id=$3`, request.FluctlightID, request.CapabilityName, request.OperationID).Scan(&digest, &encoded, &invocationEncoded)
	if errors.Is(err, pgx.ErrNoRows) {
		return CapabilityResult{}, ToolAuthorityRevisions{}, false, nil
	}
	if err != nil {
		return CapabilityResult{}, ToolAuthorityRevisions{}, false, err
	}
	if digest != toolExecutionDigest(request) {
		return CapabilityResult{}, ToolAuthorityRevisions{}, false, newCapabilityError("operation_id_conflict", false, ErrConflict)
	}
	var result CapabilityResult
	if err := json.Unmarshal(encoded, &result); err != nil {
		return result, ToolAuthorityRevisions{}, false, fmt.Errorf("decode committed tool result: %w", err)
	}
	var invocationRecord struct {
		AuthorityRevisions ToolAuthorityRevisions `json:"authority_revisions"`
	}
	if err := json.Unmarshal(invocationEncoded, &invocationRecord); err != nil {
		return result, ToolAuthorityRevisions{}, false, fmt.Errorf("decode committed tool invocation: %w", err)
	}
	if output, ok := result.Output.(map[string]any); ok {
		if _, exists := output["replayed"]; exists {
			output["replayed"] = true
		}
	}
	return result, invocationRecord.AuthorityRevisions, true, nil
}

// Business state and the receipt share one commit, so a later model error can
// neither erase a committed result nor require replaying its side effect.
func (a *App) executeToolMutation(ctx context.Context, request ToolExecutionRequest, execute func(pgx.Tx) (CapabilityResult, error)) (CapabilityResult, ToolAuthorityRevisions, bool, error) {
	var result CapabilityResult
	var authority ToolAuthorityRevisions
	var replayed bool
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		// One snapshot separates this transaction's writes from concurrent commits.
		if _, err := tx.Exec(ctx, `SET TRANSACTION ISOLATION LEVEL REPEATABLE READ`); err != nil {
			return err
		}
		key := request.FluctlightID + "\x1f" + request.CapabilityName + "\x1f" + request.OperationID
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, key); err != nil {
			return err
		}
		var err error
		result, authority, replayed, err = readToolExecution(ctx, tx, request)
		if err != nil || replayed {
			return err
		}
		allowed, policyReason, err := a.authorizeToolPolicyTx(ctx, tx, request)
		if err != nil {
			return err
		}
		beforeFoundation, beforeState, beforeLife, err := readCognitionAuthorityRevisionsWith(ctx, tx, request.FluctlightID, time.Now().UTC())
		if err != nil {
			return err
		}
		before := &ToolAuthorityRevisions{Foundation: beforeFoundation, CurrentState: beforeState, LifeContext: beforeLife}
		if allowed {
			result, err = execute(tx)
		} else {
			result = CapabilityResult{CallID: firstString(request.NativeToolCallID, "direct_call_"+stableDigest(request.FluctlightID + "\x1f" + request.CapabilityName + "\x1f" + request.OperationID)[:32]), CapabilityName: request.CapabilityName, Status: "rejected", ErrorCode: "policy_" + policyReason, Output: map[string]any{"reason": policyReason}, ProviderRequestID: request.ProviderRequestID}
		}
		if err != nil {
			return err
		}
		result.ActingProfileID = request.WorkingProfileID
		if result.Status != "completed" && result.Status != "accepted" && result.Status != "rejected" && result.Status != "failed" {
			return fmt.Errorf("tool returned nonterminal execution status %q", result.Status)
		}
		foundation, currentState, lifeContext, err := readCognitionAuthorityRevisionsWith(ctx, tx, request.FluctlightID, time.Now().UTC())
		if err != nil {
			return err
		}
		authority = ToolAuthorityRevisions{Before: before, Foundation: foundation, CurrentState: currentState, LifeContext: lifeContext}
		invocation := CapabilityInvocation{CallID: result.CallID, CapabilityName: request.CapabilityName, Arguments: request.Arguments, SourceFactID: request.EvidenceID, ProviderRequestID: result.ProviderRequestID, SchemaVersion: CapabilityInvocationSchemaVersion, Metadata: InvocationMetadata{OperationID: request.OperationID, AuthorizationActorID: request.AuthorizationActorID, SubjectActorID: request.SubjectActorID, WorkingProfileID: request.WorkingProfileID, FluctlightID: request.FluctlightID, ConversationID: request.ConversationID}}
		if request.NativeToolCallID != "" {
			invocation.Metadata.Source = "model_tool"
		} else {
			invocation.Metadata.Source = "direct"
		}
		invocationRecord := mapValue(decodeJSONValue(jsonBytes(invocation)))
		invocationRecord["authority_revisions"] = authority
		_, err = tx.Exec(ctx, `INSERT INTO public.tool_executions(fluctlight_id,capability_name,operation_id,authorization_actor_id,request_digest,result,agent_id,run_id,invocation) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, request.FluctlightID, request.CapabilityName, request.OperationID, request.AuthorizationActorID, toolExecutionDigest(request), jsonBytes(result), request.AgentID, request.RunID, jsonBytes(invocationRecord))
		return err
	})
	return result, authority, replayed, err
}
