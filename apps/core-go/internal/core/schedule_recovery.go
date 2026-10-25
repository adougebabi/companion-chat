package core

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Recovery re-arms the original daily scheduler. Stable Workflow identity and
// failed-only Temporal reuse prevent a second scheduler or duplicate schedule.
func (a *App) RecoverInitialSchedule(ctx context.Context, owner, fid string, body map[string]any) (map[string]any, error) {
	for key := range body {
		if key != "reason" && key != "idempotency_key" {
			return nil, ErrInvalidArguments
		}
	}
	if strings.TrimSpace(stringValue(body["reason"])) == "" || len([]rune(stringValue(body["reason"]))) > 500 || strings.TrimSpace(stringValue(body["idempotency_key"])) == "" {
		return nil, ErrInvalidArguments
	}
	tx, ok := SurfaceDBTx(ctx)
	if !ok {
		return nil, ErrInvalidArguments
	}
	if err := requireOwnerMutableTx(ctx, tx, owner, fid); err != nil {
		return nil, err
	}
	if err := lockLifeContextTx(ctx, tx, fid); err != nil {
		return nil, err
	}
	{
		id := "schedule_intent:" + fid
		var workflow, status string
		var cause *string
		if err := tx.QueryRow(ctx, `SELECT workflow_id,status,last_error FROM public.platform_workflow_intents WHERE intent_id=$1 AND intent_type='schedule.current_day' AND payload->>'fluctlight_id'=$2 FOR UPDATE`, id, fid).Scan(&workflow, &status, &cause); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, ErrNotFound
			}
			return nil, err
		}
		result := map[string]any{"intent_id": id, "workflow_id": workflow, "status": status, "restarted": false}
		if status == "failed" || status == "dead_letter" {
			if _, err := tx.Exec(ctx, `UPDATE public.platform_workflow_intents SET status='pending',next_attempt_at=now(),started_at=NULL,completed_at=NULL,last_error=NULL WHERE intent_id=$1`, id); err != nil {
				return nil, err
			}
			result["status"], result["restarted"] = "pending", true
			if cause != nil {
				result["previous_error"] = boundedLifecycleCause(*cause)
			}
		} else if status != "pending" && status != "retry" && status != "started" && status != "running" && status != "completed" {
			return nil, scheduleFailure("schedule_recovery_unavailable", "原日程任务当前不可恢复，请核对任务状态")
		}
		return result, nil
	}
}
