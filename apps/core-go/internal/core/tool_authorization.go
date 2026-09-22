package core

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// AuthorizationPolicy is an explicit business delegation, not an Agent name,
// source prefix, surface or phase. Owner commands need no autonomous grant;
// delegated external effects must satisfy the Owner's actual policy.
func (a *App) authorizeToolPolicyTx(ctx context.Context, tx pgx.Tx, request ToolExecutionRequest) (bool, string, error) {
	if request.AuthorizationPolicy == "" {
		return true, "", nil
	}
	if request.AuthorizationPolicy != "autonomy" {
		return false, "unknown_authorization_policy", ErrUnauthorized
	}
	definition, ok := a.capabilityRegistry().Definition(request.CapabilityName)
	if !ok {
		return false, "capability_not_found", ErrCapabilityNotFound
	}
	actionType := "capability"
	switch definition.OutputRole {
	case "conversation_message":
		actionType = "proactive_message"
	case "moment":
		actionType = "moment"
	default:
		if definition.SideEffectClass != "external_async" && definition.CompletionBoundary == "" {
			return true, "", nil
		}
	}
	// One policy lock serializes permission/budget admission for this resource.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "tool-policy:"+request.FluctlightID); err != nil {
		return false, "", err
	}
	reservationID := request.CapabilityName + ":" + request.OperationID
	if request.RunID != "" {
		reservationID = string(request.AgentID) + ":" + request.RunID
	}
	var reserved bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.tool_policy_reservations WHERE fluctlight_id=$1 AND reservation_id=$2)`, request.FluctlightID, reservationID).Scan(&reserved); err != nil {
		return false, "", err
	}
	policy, err := a.evaluateAutonomyPolicyWithReader(ctx, tx, request.FluctlightID, actionType, a.now().UTC(), "", reserved)
	if err != nil {
		return false, "", err
	}
	if !policy.Allowed {
		return false, policy.Reason, nil
	}
	if !reserved {
		var concurrent int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM public.tool_policy_reservations p JOIN public.agent_runs r ON r.fluctlight_id=p.fluctlight_id AND r.agent_id=p.agent_id AND r.run_id=p.run_id WHERE p.fluctlight_id=$1 AND p.reservation_id<>$2 AND r.status='running'`, request.FluctlightID, reservationID).Scan(&concurrent); err != nil {
			return false, "", err
		}
		limit := intValue(policy.Snapshot["concurrency_limit"])
		if limit < 1 {
			limit = 1
		}
		if concurrent >= limit {
			return false, "concurrency_limit", nil
		}
		if err := reserveAutonomyBudgetTx(ctx, tx, request.FluctlightID); err != nil {
			return false, "", fmt.Errorf("reserve delegated Tool budget: %w", err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.tool_policy_reservations(fluctlight_id,reservation_id,agent_id,run_id) VALUES($1,$2,$3,$4)`, request.FluctlightID, reservationID, request.AgentID, request.RunID); err != nil {
			return false, "", err
		}
	}
	return true, "", nil
}
