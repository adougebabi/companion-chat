package core

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5"
)

const fluctlightPausedStopReason = "fluctlight_paused"

var errAutonomousLifecycleStopped = errors.New("autonomous_lifecycle_stopped")

func autonomousStopReason(status string) string {
	if strings.TrimSpace(status) == "paused" {
		return fluctlightPausedStopReason
	}
	return "fluctlight_not_active"
}

func (a *App) autonomousActivityBlocked(ctx context.Context, fluctlightID string) (bool, string, error) {
	if a == nil || a.DB == nil || a.DB.Pool() == nil || strings.TrimSpace(fluctlightID) == "" {
		return false, "", nil
	}
	var status string
	err := a.DB.Pool().QueryRow(ctx, `SELECT status FROM public.fluctlights WHERE id=$1`, fluctlightID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return true, "", ErrNotFound
	}
	if err != nil {
		return false, "", err
	}
	return status != "active", status, nil
}

func autonomousActivityBlockedTx(ctx context.Context, tx pgx.Tx, fluctlightID string) (bool, string, error) {
	var status string
	err := tx.QueryRow(ctx, `SELECT status FROM public.fluctlights WHERE id=$1`, fluctlightID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return true, "", ErrNotFound
	}
	if err != nil {
		return false, "", err
	}
	return status != "active", status, nil
}

// autonomousEnqueueBlocked skips scheduling when the Fluctlight is not active.
// A missing row is not a failure of the caller's own write: there is no live
// character to enqueue work for.
func autonomousEnqueueBlocked(ctx context.Context, tx pgx.Tx, fluctlightID string) (bool, error) {
	blocked, _, err := autonomousActivityBlockedTx(ctx, tx, fluctlightID)
	if errors.Is(err, ErrNotFound) {
		return true, nil
	}
	return blocked, err
}

func cancelCurrentDayScheduleIntentTx(ctx context.Context, tx pgx.Tx, fluctlightID, status string) error {
	reason := autonomousStopReason(status)
	_, err := tx.Exec(ctx, `
		UPDATE public.platform_workflow_intents
		SET status='cancelled', completed_at=COALESCE(completed_at, now()), last_error=$2
		WHERE intent_type='schedule.current_day'
		  AND (payload->>'fluctlight_id'=$1 OR intent_id='schedule_intent:'||$1)
		  AND status IN ('pending','retry','started','running','cancel_requested')`, fluctlightID, reason)
	return err
}

func reopenPausedCurrentDayScheduleIntentTx(ctx context.Context, tx pgx.Tx, fluctlightID string) error {
	_, err := tx.Exec(ctx, `
		UPDATE public.platform_workflow_intents
		SET status='pending', started_at=NULL, completed_at=NULL, last_error=NULL, next_attempt_at=now()
		WHERE intent_type='schedule.current_day'
		  AND (payload->>'fluctlight_id'=$1 OR intent_id='schedule_intent:'||$1)
		  AND status='cancelled'
		  AND last_error=$2`, fluctlightID, fluctlightPausedStopReason)
	return err
}

func (a *App) requestCurrentDayScheduleCancel(ctx context.Context, fluctlightID, status string) {
	if a == nil || a.Workflows == nil || strings.TrimSpace(fluctlightID) == "" {
		return
	}
	workflowID := normalizedLifecycleWorkflowID("schedule:" + fluctlightID)
	if err := a.Workflows.Cancel(ctx, workflowID, "", autonomousStopReason(status)+":"+fluctlightID); err != nil && !errors.Is(err, pgx.ErrNoRows) && !errors.Is(err, ErrNotFound) {
		slog.Warn("Go Core could not cancel the current-day schedule workflow",
			"fluctlight_id", fluctlightID,
			"workflow_id", workflowID,
			"error_type", err.Error(),
		)
	}
}
