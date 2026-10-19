package core

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

const scheduledResolutionMaximumAttempts = 3
const scheduledResolutionRetryDelay = 5 * time.Minute

func knownInvalidScheduledResolution(result CapabilityResult) bool {
	out := mapValue(result.Output)
	return result.Status == "handled" && out["handled"] == true && out["effect_applied"] == false && out["mutation_committed"] == false && out["disposition"] == "awaiting_evidence" && containsString([]string{"activity_result_invalid", "virtual_activity_result_invalid"}, stringValue(out["warning_code"]))
}
func scheduledResolutionRetryView(id string, at time.Time, attempt int, code string) map[string]any {
	return map[string]any{"activity_id": id, "status": "pending", "not_before": at.UTC().Format(time.RFC3339Nano), "reason": "result_validation_retry", "warning_code": code, "resolution_attempt": attempt, "effect_applied": false}
}
func scheduledResolutionSourcesActiveWith(ctx context.Context, q lifeContextQuerier, id, fid, intention string, data map[string]any, at time.Time) (bool, error) {
	active, err := activityAuthorityActiveWith(ctx, q, id, fid, at)
	if err != nil || !active {
		return active, err
	}
	item := stringValue(mapValue(data["request"])["schedule_item_id"])
	// A withdrawn scheduled appointment cannot execute. An already started
	// native activity may still report a real late result while its Intention
	// is paused; that existing boundary must not be replaced by cancellation.
	if intention == "" || item == "" {
		return true, nil
	}
	var permitted bool
	err = q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.fluctlight_intentions i WHERE i.id=$1 AND i.fluctlight_id=$2 AND i.status='in_progress' AND i.expiration>$3 AND ($4::text='' OR EXISTS(SELECT 1 FROM public.life_schedule_items item JOIN public.life_schedules s ON s.id=item.schedule_id WHERE item.id=$4 AND item.intention_id=i.id AND s.fluctlight_id=i.fluctlight_id AND s.status='accepted')))`, intention, fid, at, item).Scan(&permitted)
	return permitted, err
}

// Rotate only a committed, explicitly no-effect preparation rejection. The
// original operation/receipt remains immutable; an unknown network/DB/effect
// outcome never reaches this path. Metadata describes recovery, not life facts.
func (a *App) recoverScheduledResolution(ctx context.Context, id, fid string, expected int, operation string) (map[string]any, error) {
	var view map[string]any
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('fluctlight_lifecycle:' || $1))`, fid); err != nil {
			return err
		}
		if err := lockLifeContextTx(ctx, tx, fid); err != nil {
			return err
		}
		var lifecycle string
		if err := tx.QueryRow(ctx, `SELECT status FROM public.fluctlights WHERE id=$1 FOR UPDATE`, fid).Scan(&lifecycle); err != nil {
			return err
		}
		if lifecycle != "active" {
			return newCapabilityError("fluctlight_not_active", true, ErrConflict)
		}

		var state, intention, profile string
		var revision int
		var raw []byte
		var until, notBefore time.Time
		var resolved *time.Time
		var event *string
		if err := tx.QueryRow(ctx, `SELECT status,COALESCE(intention_id,''),profile_id,revision,result_json,active_until,not_before,resolved_at,source_event_id FROM public.fluctlight_life_activity_runs WHERE id=$1 AND fluctlight_id=$2 FOR UPDATE`, id, fid).Scan(&state, &intention, &profile, &revision, &raw, &until, &notBefore, &resolved, &event); err != nil {
			return err
		}
		if state == "completed" || state == "failed" || state == "cancelled" {
			view = map[string]any{"activity_id": id, "status": state}
			return nil
		}
		data := decodeObject(raw)
		meta := mapValue(data["resolution_recovery"])
		attempts := 0
		if len(meta) > 0 {
			n, ok := parseRevision(meta["attempt_count"])
			if !ok || n < 1 || n > scheduledResolutionMaximumAttempts {
				return errors.New("scheduled_resolution_retry_metadata_invalid")
			}
			attempts = n
		}
		if revision != expected {
			at := notBefore
			if value, ok := meta["next_attempt_at"]; ok {
				parsed, e := time.Parse(time.RFC3339Nano, stringValue(value))
				if e != nil {
					return e
				}
				at = parsed
			}
			if !at.After(a.now()) {
				at = a.now().Add(scheduledResolutionRetryDelay)
			}
			view = scheduledResolutionRetryView(id, at, attempts, stringValue(meta["last_warning_code"]))
			return nil
		}
		if resolved != nil || event != nil {
			return ErrConflict
		}
		// Re-read exactly the persisted operation, not a caller-supplied boolean.
		var encoded []byte
		if err := tx.QueryRow(ctx, `SELECT result FROM public.tool_executions WHERE fluctlight_id=$1 AND capability_name=$2 AND operation_id=$3`, fid, lifeActivityAdvanceCapabilityName, operation).Scan(&encoded); err != nil {
			return err
		}
		var result CapabilityResult
		if err := json.Unmarshal(encoded, &result); err != nil {
			return err
		}
		if !knownInvalidScheduledResolution(result) {
			return ErrConflict
		}
		now := a.now().UTC()
		inv := CapabilityInvocation{CallID: result.CallID, CapabilityName: lifeActivityAdvanceCapabilityName, ProviderRequestID: result.ProviderRequestID, SourceFactID: "activity:" + id}
		authority, err := activityAuthorityActiveWith(ctx, tx, id, fid, now)
		if err != nil {
			return err
		}
		if !authority {
			closed, e := a.cancelActivityForEndedEventTx(ctx, tx, inv, id, fid, intention, revision, raw)
			view = mapValue(closed.Output)
			return e
		}
		active, err := scheduledResolutionSourcesActiveWith(ctx, tx, id, fid, intention, data, now)
		if err != nil {
			return err
		}
		if !active {
			closed, e := cancelUnsettledScheduledLifeActivityTx(ctx, tx, a, inv, id, fid, intention, revision, raw, now)
			view = mapValue(closed.Output)
			return e
		}
		if intention != "" && stringValue(mapValue(data["request"])["schedule_item_id"]) != "" {
			if err := tx.QueryRow(ctx, `SELECT LEAST($2::timestamptz,expiration) FROM public.fluctlight_intentions WHERE id=$1 AND fluctlight_id=$3`, intention, until, fid).Scan(&until); err != nil {
				return err
			}
		}
		attempts++
		code := stringValue(mapValue(result.Output)["warning_code"])
		next := now.Add(scheduledResolutionRetryDelay)
		meta = map[string]any{"attempt_count": attempts, "last_operation_id": operation, "last_warning_code": code, "last_failed_at": now.Format(time.RFC3339Nano)}
		reason := ""
		if attempts >= scheduledResolutionMaximumAttempts {
			reason = "result_validation_retry_exhausted"
		} else if !next.Before(until) {
			reason = "result_validation_retry_window_ended"
		}
		if reason != "" {
			meta["stopped_reason"] = reason
			data["resolution_recovery"] = meta
			if intention != "" {
				var currentStatus string
				if err := tx.QueryRow(ctx, `SELECT status FROM public.fluctlight_intentions WHERE id=$1 AND fluctlight_id=$2 FOR UPDATE`, intention, fid).Scan(&currentStatus); err != nil {
					return err
				}
				if currentStatus == string(IntentionInProgress) {
					if _, err := transitionLinkedIntentionTx(ctx, tx, fid, profile, intention, IntentionPause, "activity:"+id, reason, "resolution-retry-stop:"+operation, now); err != nil {
						return err
					}
				}
			}
			closed, e := cancelUnsettledLifeActivityWithReasonTx(ctx, tx, a, inv, id, fid, intention, revision, jsonBytes(data), now, reason)
			view = mapValue(closed.Output)
			return e
		}
		meta["next_attempt_at"] = next.Format(time.RFC3339Nano)
		data["resolution_recovery"] = meta
		tag, err := tx.Exec(ctx, `UPDATE public.fluctlight_life_activity_runs SET revision=revision+1,result_json=$3 WHERE id=$1 AND fluctlight_id=$2 AND revision=$4 AND resolved_at IS NULL AND source_event_id IS NULL`, id, fid, jsonBytes(data), revision)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return ErrConflict
		}
		view = scheduledResolutionRetryView(id, next, attempts, code)
		return nil
	})
	return view, err
}
