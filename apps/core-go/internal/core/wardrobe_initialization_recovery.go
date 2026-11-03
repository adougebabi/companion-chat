package core

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

// RecoverInitialWardrobe rearms the current initializer under its stable ID.
// Committed inventory and the preparation checkpoint remain authoritative.
func (a *App) RecoverInitialWardrobe(ctx context.Context, owner, fid string, body map[string]any) (map[string]any, error) {
	if !eventKeys(body, "reason", "idempotency_key") || strings.TrimSpace(stringValue(body["reason"])) == "" || len([]rune(stringValue(body["reason"]))) > 500 || strings.TrimSpace(stringValue(body["idempotency_key"])) == "" || len([]rune(stringValue(body["idempotency_key"]))) > 256 {
		return nil, ErrInvalidArguments
	}
	tx, ok := SurfaceDBTx(ctx)
	if !ok {
		return nil, ErrInvalidArguments
	}
	if err := lockLifeContextTx(ctx, tx, fid); err != nil {
		return nil, err
	}
	if err := requireOwnerMutableTx(ctx, tx, owner, fid); err != nil {
		return nil, err
	}
	var lifecycleStatus string
	if err := tx.QueryRow(ctx, `SELECT status FROM public.fluctlights WHERE id=$1 AND created_by_actor_id=$2 FOR SHARE`, fid, owner).Scan(&lifecycleStatus); err != nil {
		return nil, err
	}
	if lifecycleStatus != "active" {
		return nil, &SurfaceError{Status: 409, Code: "wardrobe_recovery_unavailable", Message: "角色当前未处于活动状态，无法恢复衣橱初始化"}
	}
	var preparation, id, workflowID, status, preparationStatus string
	var cause *string
	err := tx.QueryRow(ctx, `SELECT p.id,w.intent_id,w.workflow_id,w.status,p.status,w.last_error
		FROM public.fluctlight_wardrobe_preparations p JOIN public.platform_workflow_intents w ON w.intent_id=p.intent_id
		WHERE p.fluctlight_id=$1 AND w.intent_type='wardrobe.initialize' AND w.payload->>'fluctlight_id'=$1 FOR UPDATE OF p,w`, fid).Scan(&preparation, &id, &workflowID, &status, &preparationStatus, &cause)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	result := map[string]any{"intent_id": id, "workflow_id": workflowID, "status": status, "restarted": false}
	if preparationStatus == "completed" {
		return result, nil
	}
	if status == "failed" || status == "dead_letter" {
		if _, err := tx.Exec(ctx, `UPDATE public.platform_workflow_intents SET status='pending',attempt_count=0,next_attempt_at=now(),started_at=NULL,completed_at=NULL,last_error=NULL WHERE intent_id=$1`, id); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_wardrobe_preparations SET status=CASE WHEN result->'inventory_committed'='true'::jsonb THEN 'awaiting_wear' ELSE 'queued' END,result=result-'failure_reason',error_code=NULL,updated_at=now() WHERE id=$1`, preparation); err != nil {
			return nil, err
		}
		result["status"], result["restarted"] = "pending", true
		if cause != nil {
			_, reason := wardrobePreparationFailure(errors.New(*cause))
			result["previous_error"] = reason
		}
	} else if status != "pending" && status != "retry" && status != "started" && status != "running" && status != "completed" {
		return nil, &SurfaceError{Status: 409, Code: "wardrobe_recovery_unavailable", Message: "原衣橱初始化任务当前不可恢复，请核对任务状态"}
	}
	return result, nil
}
