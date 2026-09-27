package core

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// A run can be current only while its starting Event is authoritative and its
// frozen boundary has not passed. Legacy runs have no Event link but still get
// a bounded active_until during migration.
func activityAuthorityActiveWith(ctx context.Context, query lifeContextQuerier, activityID, fluctlightID string, at time.Time) (bool, error) {
	var active bool
	err := query.QueryRow(ctx, `SELECT active_until>$3 AND (authority_event_id IS NULL OR EXISTS(
		SELECT 1 FROM public.life_events e WHERE e.id=r.authority_event_id AND e.fluctlight_id=r.fluctlight_id
		AND e.status IN ('confirmed','inferred') AND e.start_at<=$3 AND e.end_at>$3 AND (e.expires_at IS NULL OR e.expires_at>$3)
	)) FROM public.fluctlight_life_activity_runs r WHERE r.id=$1 AND r.fluctlight_id=$2`, activityID, fluctlightID, at).Scan(&active)
	return active, err
}

func (a *App) closeExpiredActivityRunsTx(ctx context.Context, tx pgx.Tx, fluctlightID string, at time.Time) error {
	rows, err := tx.Query(ctx, `UPDATE public.fluctlight_life_activity_runs r SET status='cancelled',resolved_at=$2,
		result_json=jsonb_set(r.result_json,'{result}',jsonb_build_object('status','cancelled','reason','event_ended'),true),revision=r.revision+1
		WHERE r.fluctlight_id=$1 AND r.status IN ('scheduled','in_progress','deferred')
		AND (r.active_until IS NULL OR r.active_until<=$2 OR (r.authority_event_id IS NOT NULL AND NOT EXISTS(
			SELECT 1 FROM public.life_events e WHERE e.id=r.authority_event_id AND e.fluctlight_id=r.fluctlight_id
			AND e.status IN ('confirmed','inferred') AND e.start_at<=$2 AND e.end_at>$2 AND (e.expires_at IS NULL OR e.expires_at>$2)))) RETURNING r.id`, fluctlightID, at)
	if err != nil {
		return err
	}
	return a.settleClosedActivityOutcomesTx(ctx, tx, rows)
}

func (a *App) closeActivityRunsForEventTx(ctx context.Context, tx pgx.Tx, fluctlightID, eventID string, at time.Time) error {
	rows, err := tx.Query(ctx, `UPDATE public.fluctlight_life_activity_runs SET status='cancelled',resolved_at=$3,
		result_json=jsonb_set(result_json,'{result}',jsonb_build_object('status','cancelled','reason','event_ended'),true),revision=revision+1
		WHERE fluctlight_id=$1 AND authority_event_id=$2 AND status IN ('scheduled','in_progress','deferred') RETURNING id`, fluctlightID, eventID, at)
	if err != nil {
		return err
	}
	return a.settleClosedActivityOutcomesTx(ctx, tx, rows)
}

func (a *App) settleClosedActivityOutcomesTx(ctx context.Context, tx pgx.Tx, rows pgx.Rows) error {
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err := rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range ids {
		var pending bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.cognition_action_outcomes WHERE external_ref=$1 AND status IN ('pending','unknown'))`, id).Scan(&pending); err != nil {
			return err
		}
		if pending {
			if _, err := a.settleActionOutcomeByExternalRefTx(ctx, tx, id, ActionOutcomeCancelled, map[string]any{"reason_code": "event_ended"}, "event_ended"); err != nil {
				return err
			}
		}
	}
	return nil
}

func (a *App) cancelActivityForEndedEventTx(ctx context.Context, tx pgx.Tx, invocation CapabilityInvocation, activityID, fluctlightID, intentionID string, revision int, resultRaw []byte) (CapabilityResult, error) {
	now := time.Now().UTC()
	result := decodeObject(resultRaw)
	result["result"] = map[string]any{"status": "cancelled", "reason": "event_ended"}
	command, err := tx.Exec(ctx, `UPDATE public.fluctlight_life_activity_runs SET status='cancelled',resolved_at=$3,result_json=$4,revision=revision+1 WHERE id=$1 AND fluctlight_id=$2 AND revision=$5 AND status IN ('scheduled','in_progress','deferred')`, activityID, fluctlightID, now, jsonBytes(result), revision)
	if err != nil || command.RowsAffected() != 1 {
		if err == nil {
			err = ErrConflict
		}
		return failedCapabilityResult(invocation, "activity_authority_close_failed", true), err
	}
	var pending bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.cognition_action_outcomes WHERE external_ref=$1 AND status IN ('pending','unknown'))`, activityID).Scan(&pending); err != nil {
		return failedCapabilityResult(invocation, "activity_outcome_read_failed", true), err
	}
	if pending {
		if _, err := a.settleActionOutcomeByExternalRefTx(ctx, tx, activityID, ActionOutcomeCancelled, map[string]any{"reason_code": "event_ended"}, "event_ended"); err != nil {
			return failedCapabilityResult(invocation, "activity_outcome_close_failed", true), err
		}
	}
	return CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "completed",
		Output:            map[string]any{"activity_id": activityID, "status": "cancelled", "intention_id": intentionID, "reason": "event_ended"},
		ProviderRequestID: invocation.ProviderRequestID, CorrelationID: "activity:" + activityID}, nil
}

func extendLifeActivityTx(ctx context.Context, tx pgx.Tx, invocation CapabilityInvocation, activityID, fluctlightID string, revision int, decision map[string]any) (CapabilityResult, error) {
	minutes := intValue(decision["extend_minutes"])
	if minutes < 15 || minutes > 240 || stringValue(decision["reason"]) == "" {
		return failedCapabilityResult(invocation, "activity_extension_invalid", false), ErrInvalidArguments
	}
	var eventID string
	var previous time.Time
	if err := tx.QueryRow(ctx, `SELECT COALESCE(authority_event_id,''),active_until FROM public.fluctlight_life_activity_runs WHERE id=$1 AND fluctlight_id=$2 AND revision=$3 FOR UPDATE`, activityID, fluctlightID, revision).Scan(&eventID, &previous); err != nil {
		return failedCapabilityResult(invocation, "activity_extension_stale", true), err
	}
	if eventID == "" {
		return failedCapabilityResult(invocation, "activity_extension_legacy_run", false), ErrConflict
	}
	now := time.Now().UTC()
	if !now.Before(previous) {
		return failedCapabilityResult(invocation, "activity_window_ended", false), ErrConflict
	}
	newUntil := previous.Add(time.Duration(minutes) * time.Minute)
	command, err := tx.Exec(ctx, `UPDATE public.life_events SET end_at=$3,expires_at=$3,revision=revision+1,result=jsonb_set(result,'{revision}',to_jsonb(revision+1),true),updated_at=$4 WHERE id=$1 AND fluctlight_id=$2 AND status IN ('confirmed','inferred') AND end_at>$4 AND (expires_at IS NULL OR expires_at>$4)`, eventID, fluctlightID, newUntil, now)
	if err != nil || command.RowsAffected() != 1 {
		if err == nil {
			err = ErrConflict
		}
		return failedCapabilityResult(invocation, "activity_extension_event_failed", true), err
	}
	command, err = tx.Exec(ctx, `UPDATE public.fluctlight_life_activity_runs SET active_until=$3,revision=revision+1,result_json=jsonb_set(result_json,'{extension}', $4::jsonb,true) WHERE id=$1 AND fluctlight_id=$2 AND revision=$5 AND status IN ('in_progress','deferred')`, activityID, fluctlightID, newUntil, jsonBytes(map[string]any{"reason": decision["reason"], "until": newUntil.Format(time.RFC3339Nano)}), revision)
	if err != nil || command.RowsAffected() != 1 {
		if err == nil {
			err = ErrConflict
		}
		return failedCapabilityResult(invocation, "activity_extension_run_failed", true), err
	}
	return CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "completed",
		Output:            map[string]any{"activity_id": activityID, "status": "extended", "active_until": newUntil.Format(time.RFC3339Nano), "reason": decision["reason"]},
		ProviderRequestID: invocation.ProviderRequestID, CorrelationID: "activity:" + activityID}, nil
}
