package core

// StructuredAssembledWithToolsSchema remains the canonical assembled Eino task boundary.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	defaultWakeUpIntervalSeconds = 30 * 60
	minWakeUpIntervalSeconds     = 5 * 60
	maxWakeUpIntervalSeconds     = 24 * 60 * 60
	wakeUpFirstIdleDelay         = 10 * time.Minute
	wakeUpSecondIdleDelay        = 30 * time.Minute
)

type wakeUpIdleClock struct {
	Epoch string
	Since time.Time
	Slot  int
}

type wakeUpIdleEpochContextKey struct{}
type wakeUpCycleContextKey struct{}

func WithWakeUpIdleEpoch(ctx context.Context, epoch string) context.Context {
	return context.WithValue(ctx, wakeUpIdleEpochContextKey{}, strings.TrimSpace(epoch))
}

func WithWakeUpCycle(ctx context.Context, cycle int) context.Context {
	return context.WithValue(ctx, wakeUpCycleContextKey{}, cycle)
}

func expectedWakeUpCycle(ctx context.Context) int {
	cycle, _ := ctx.Value(wakeUpCycleContextKey{}).(int)
	return cycle
}

func expectedWakeUpIdleEpoch(ctx context.Context) string {
	epoch, _ := ctx.Value(wakeUpIdleEpochContextKey{}).(string)
	return strings.TrimSpace(epoch)
}

func wakeUpExecutionMatches(status string, payload map[string]any, cycle int, expectedEpoch string) bool {
	switch status {
	case "pending", "retry", "started", "running":
	default:
		return false
	}
	if intValue(payload["cycle"]) != cycle {
		return false
	}
	return stringValue(payload["idle_epoch"]) == expectedEpoch
}

func (a *App) wakeUpExecutionCurrent(ctx context.Context, cycle int) (bool, error) {
	intentID := lifecycleIntentID(ctx)
	if intentID == "" {
		return true, nil
	}
	var status string
	var raw []byte
	if err := a.DB.Pool().QueryRow(ctx, `SELECT status,payload FROM public.platform_workflow_intents WHERE intent_id=$1 AND intent_type='wake_up.current'`, intentID).Scan(&status, &raw); err != nil {
		return false, err
	}
	return wakeUpExecutionMatches(status, decodeObject(raw), cycle, expectedWakeUpIdleEpoch(ctx)), nil
}

func wakeUpExecutionCurrentTx(ctx context.Context, tx pgx.Tx, cycle int) (bool, error) {
	intentID := lifecycleIntentID(ctx)
	if intentID == "" {
		return true, nil
	}
	var status string
	var raw []byte
	if err := tx.QueryRow(ctx, `SELECT status,payload FROM public.platform_workflow_intents WHERE intent_id=$1 AND intent_type='wake_up.current' FOR UPDATE`, intentID).Scan(&status, &raw); err != nil {
		return false, err
	}
	return wakeUpExecutionMatches(status, decodeObject(raw), cycle, expectedWakeUpIdleEpoch(ctx)), nil
}

func wakeUpIdlePhase(slot int) string {
	switch slot {
	case 0:
		return "first_10m"
	case 1:
		return "second_30m"
	default:
		return "recurring"
	}
}

func wakeUpIdleClockFromPayload(payload map[string]any) (wakeUpIdleClock, bool) {
	if intValue(payload["idle_version"]) != 1 || strings.TrimSpace(stringValue(payload["idle_epoch"])) == "" {
		return wakeUpIdleClock{}, false
	}
	since, err := time.Parse(time.RFC3339Nano, stringValue(payload["idle_since"]))
	if err != nil || intValue(payload["idle_slot"]) < 0 {
		return wakeUpIdleClock{}, false
	}
	return wakeUpIdleClock{Epoch: stringValue(payload["idle_epoch"]), Since: since.UTC(), Slot: intValue(payload["idle_slot"])}, true
}

func wakeUpIdleDue(clock wakeUpIdleClock, intervalSeconds int) time.Time {
	if intervalSeconds <= 0 {
		intervalSeconds = defaultWakeUpIntervalSeconds
	}
	if clock.Slot <= 0 {
		return clock.Since.Add(wakeUpFirstIdleDelay)
	}
	if clock.Slot == 1 {
		return clock.Since.Add(wakeUpSecondIdleDelay)
	}
	return clock.Since.Add(wakeUpSecondIdleDelay + time.Duration(clock.Slot-1)*time.Duration(intervalSeconds)*time.Second)
}

func nextWakeUpIdleClock(clock wakeUpIdleClock, now time.Time, intervalSeconds int) wakeUpIdleClock {
	if intervalSeconds <= 0 {
		intervalSeconds = defaultWakeUpIntervalSeconds
	}
	next := clock
	next.Slot++
	if clock.Slot >= 1 {
		interval := time.Duration(intervalSeconds) * time.Second
		elapsed := now.UTC().Sub(clock.Since.Add(wakeUpSecondIdleDelay))
		if elapsed >= 0 {
			futureSlot := int((elapsed+interval-time.Nanosecond)/interval) + 1
			if futureSlot < 2 {
				futureSlot = 2
			}
			if futureSlot > next.Slot {
				next.Slot = futureSlot
			}
		}
	}
	return next
}

func setWakeUpIdleEpochTx(ctx context.Context, tx pgx.Tx, fluctlightID, messageID string, acceptedAt time.Time) error {
	if strings.TrimSpace(messageID) == "" {
		return errors.New("wake_up_idle_message_required")
	}
	intentID := "wake_up_intent:" + fluctlightID
	var raw []byte
	err := tx.QueryRow(ctx, `SELECT payload FROM public.platform_workflow_intents WHERE intent_id=$1 AND intent_type='wake_up.current' FOR UPDATE`, intentID).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		payload := map[string]any{"fluctlight_id": fluctlightID, "cycle": 0, "idle_version": 1, "idle_epoch": messageID, "idle_since": acceptedAt.UTC().Format(time.RFC3339Nano), "idle_slot": 0, "idle_phase": wakeUpIdlePhase(0)}
		inserted, insertErr := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload,status,next_attempt_at) VALUES($1,$2,'lifecycle','wake_up.current',$3,'completed',$4) ON CONFLICT (intent_id) DO NOTHING`, intentID, "wake_up:"+fluctlightID, jsonBytes(payload), acceptedAt.UTC().Add(wakeUpFirstIdleDelay))
		if insertErr != nil || inserted.RowsAffected() == 1 {
			return insertErr
		}
		err = tx.QueryRow(ctx, `SELECT payload FROM public.platform_workflow_intents WHERE intent_id=$1 AND intent_type='wake_up.current' FOR UPDATE`, intentID).Scan(&raw)
	}
	if err != nil {
		return err
	}
	payload := decodeObject(raw)
	payload["idle_version"] = 1
	payload["idle_epoch"] = messageID
	payload["idle_since"] = acceptedAt.UTC().Format(time.RFC3339Nano)
	payload["idle_slot"] = 0
	payload["idle_phase"] = wakeUpIdlePhase(0)
	delete(payload, "idle_last_settled_cycle")
	_, err = tx.Exec(ctx, `UPDATE public.platform_workflow_intents SET payload=$2,next_attempt_at=$3,status=CASE WHEN status IN ('superseded','cancelled','failed','dead_letter') THEN 'completed' ELSE status END WHERE intent_id=$1 AND intent_type='wake_up.current'`, intentID, jsonBytes(payload), acceptedAt.UTC().Add(wakeUpFirstIdleDelay))
	return err
}

// WakeUpSettings controls the durable internal-life timer. The setting is
// intentionally small and owner-editable through product settings so a local
// deployment can trade model cost for more or less frequent self-reflection.
// The workflow still clamps the interval so an accidental value cannot create
// a tight provider loop or make the persona effectively dormant.
type WakeUpSettings struct {
	Enabled         bool `json:"enabled"`
	IntervalSeconds int  `json:"interval_seconds"`
}

func defaultWakeUpSettings() WakeUpSettings {
	return WakeUpSettings{Enabled: true, IntervalSeconds: defaultWakeUpIntervalSeconds}
}

func nextWakeUpDue(previousDue, now time.Time, intervalSeconds int) time.Time {
	if intervalSeconds <= 0 {
		intervalSeconds = defaultWakeUpIntervalSeconds
	}
	now = now.UTC()
	interval := time.Duration(intervalSeconds) * time.Second
	if previousDue.IsZero() {
		return now.Add(interval)
	}
	previousDue = previousDue.UTC()
	if previousDue.After(now) {
		return previousDue
	}
	steps := now.Sub(previousDue)/interval + 1
	return previousDue.Add(steps * interval)
}

func (a *App) ensureWakeUpNextDue(ctx context.Context, fluctlightID string, cycle, intervalSeconds int, reason string) (time.Time, error) {
	var nextDue time.Time
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if lifecycleIntentID(ctx) != "" {
			if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('fluctlight_lifecycle:' || $1))`, fluctlightID); err != nil {
				return err
			}
			current, err := wakeUpExecutionCurrentTx(ctx, tx, cycle)
			if err != nil {
				return err
			}
			if !current {
				return errLifecycleSupersededByCognition
			}
		}
		var err error
		nextDue, err = updateWakeUpNextDueTx(ctx, tx, fluctlightID, cycle, intervalSeconds, a.now().UTC())
		return err
	})
	if err != nil {
		return time.Time{}, err
	}
	correlationID := wakeUpCycleCorrelation(fluctlightID, cycle)
	a.RecordLifecycleDiagnosticBestEffort(ctx, LifecycleDiagnostic{
		Surface: "wake_up", Transition: LifecycleTransitionNextCycleScheduled,
		FluctlightID: fluctlightID, CorrelationID: correlationID,
		IntentID: "wake_up_intent:" + fluctlightID, WorkflowID: "wake_up:" + fluctlightID,
		Stage: "clock", Status: "scheduled", ReasonCode: reason,
		Attempt: cycle, NextDueAt: nextDue,
	})
	return nextDue.UTC(), nil
}

func (a *App) scheduleWakeUpHint(ctx context.Context, fluctlightID string, cycle int, nextDue time.Time) {
	delay := time.Until(nextDue)
	seconds := int((delay + time.Second - 1) / time.Second)
	if seconds < 1 {
		seconds = 1
	}
	if err := a.scheduleWakeUpTrigger(ctx, fluctlightID, seconds); err != nil {
		correlationID := wakeUpCycleCorrelation(fluctlightID, cycle)
		slog.Warn("Go Core WakeUp Redis hint failed",
			"fluctlight_id", fluctlightID,
			"correlation_id", correlationID,
			"error_type", fmt.Sprintf("%T", err),
		)
		a.RecordLifecycleDiagnosticBestEffort(ctx, LifecycleDiagnostic{
			Surface: "wake_up", Transition: LifecycleTransitionFailed, Severity: "warn",
			FluctlightID: fluctlightID, CorrelationID: correlationID,
			IntentID: "wake_up_intent:" + fluctlightID, WorkflowID: "wake_up:" + fluctlightID,
			Stage: "redis_hint", Status: "degraded", ReasonCode: "redis_hint_failed",
			ErrorCategory: "transport", ErrorCode: "redis_hint_failed", Retryable: true,
			Attempt: cycle, NextDueAt: nextDue,
		})
	}
}

func normalizeWakeUpSettings(value map[string]any) WakeUpSettings {
	settings := defaultWakeUpSettings()
	if enabled, ok := value["enabled"].(bool); ok {
		settings.Enabled = enabled
	}
	if raw, ok := value["interval_seconds"]; ok {
		settings.IntervalSeconds = int(numberOrDefault(raw, float64(settings.IntervalSeconds)))
	}
	if settings.IntervalSeconds < minWakeUpIntervalSeconds {
		settings.IntervalSeconds = minWakeUpIntervalSeconds
	}
	if settings.IntervalSeconds > maxWakeUpIntervalSeconds {
		settings.IntervalSeconds = maxWakeUpIntervalSeconds
	}
	return settings
}

func (a *App) readWakeUpSettings(ctx context.Context) (WakeUpSettings, error) {
	settings := defaultWakeUpSettings()
	var raw string
	err := a.DB.Pool().QueryRow(ctx, `SELECT value_json FROM public.runtime_settings WHERE key='product.wakeup'`).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return settings, nil
	}
	if err != nil {
		return settings, err
	}
	var value map[string]any
	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return settings, fmt.Errorf("product.wakeup setting is invalid: %w", err)
	}
	return normalizeWakeUpSettings(value), nil
}

// EnsureWakeUpIntents repairs the stable clock independently of Schedule.
// Schedule enriches WakeUp context but never owns autonomous-lifecycle liveness.
func (a *App) EnsureWakeUpIntents(ctx context.Context) (int64, error) {
	settings, err := a.readWakeUpSettings(ctx)
	if err != nil {
		return 0, err
	}
	var ensured int64
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		inserted, err := tx.Exec(ctx, `
		INSERT INTO public.platform_workflow_intents(
				intent_id,workflow_id,task_queue,intent_type,payload,status,next_attempt_at
			)
			SELECT
				'wake_up_intent:' || f.id,
				'wake_up:' || f.id,
				'lifecycle',
				'wake_up.current',
				jsonb_build_object('fluctlight_id', f.id, 'cycle', 0),
				'pending',
				now()+($1 * interval '1 second')
			FROM public.fluctlights AS f
			WHERE f.status IN ('active', 'paused')
			ON CONFLICT (intent_id) DO NOTHING`, settings.IntervalSeconds)
		if err != nil {
			return err
		}
		ensured += inserted.RowsAffected()
		requeued, err := tx.Exec(ctx, `
			UPDATE public.platform_workflow_intents AS i
			SET status='retry',next_attempt_at=now(),attempt_count=0,started_at=NULL,completed_at=NULL,last_error=NULL
			FROM public.fluctlights AS f
			WHERE i.intent_type='wake_up.current'
			  AND i.payload->>'fluctlight_id' = f.id
			  AND f.status IN ('active', 'paused')
			  AND (
			    i.status IN ('failed', 'dead_letter', 'cancelled')
			    OR (i.status = 'started' AND (i.started_at IS NULL OR i.started_at < now() - interval '5 minutes'))
			  )`)
		if err != nil {
			return err
		}
		ensured += requeued.RowsAffected()
		// A conversation can preempt WakeUp and then fail after a committed
		// reply. When no executable cognition intent remains, restore the
		// durable clock instead of leaving the one-shot intent superseded.
		repaired, err := tx.Exec(ctx, `
			UPDATE public.platform_workflow_intents AS i
			SET status='completed',next_attempt_at=CASE
				WHEN i.payload->>'idle_version'='1' AND i.payload ? 'idle_since' THEN
					GREATEST(now(), (i.payload->>'idle_since')::timestamptz +
					CASE WHEN COALESCE((i.payload->>'idle_slot')::integer,0) <= 0 THEN interval '10 minutes'
					     WHEN (i.payload->>'idle_slot')::integer = 1 THEN interval '30 minutes'
					     ELSE interval '30 minutes' + (((i.payload->>'idle_slot')::integer-1) * $1 * interval '1 second') END)
				ELSE now()+interval '10 minutes' END,
				started_at=NULL,completed_at=now(),last_error=NULL
			FROM public.fluctlights AS f
			WHERE i.intent_type='wake_up.current'
			  AND i.payload->>'fluctlight_id'=f.id
			  AND f.status IN ('active','paused')
			  AND i.status IN ('superseded', 'cancelled')
			  AND NOT EXISTS (
				SELECT 1 FROM public.cognition_inbox AS c
				JOIN public.platform_workflow_intents AS w ON w.intent_id='cognition_intent:'||c.id
				WHERE c.fluctlight_id=f.id AND c.status IN ('pending','claimed')
				  AND w.status IN ('pending','retry','started','running','cancel_requested')
			  )`, settings.IntervalSeconds)
		if err != nil {
			return err
		}
		ensured += repaired.RowsAffected()
		initialized, err := tx.Exec(ctx, `
			UPDATE public.platform_workflow_intents AS i
			SET next_attempt_at=now()+($1 * interval '1 second')
			FROM public.fluctlights AS f
			WHERE i.intent_type='wake_up.current'
			  AND i.payload->>'fluctlight_id'=f.id
			  AND f.status IN ('active','paused')
			  AND i.next_attempt_at IS NULL`, settings.IntervalSeconds)
		if err != nil {
			return err
		}
		ensured += initialized.RowsAffected()
		return nil
	})
	return ensured, err
}

// ReconcileWakeUpIntents performs periodic (e.g. 30-minute) and startup supervision
// over all Fluctlight wake-up clocks. It repairs missing intents, resurrects
// dead_letter, failed, or stale started intents, releases due intents, and
// resynchronizes Redis trigger hints.
func (a *App) ReconcileWakeUpIntents(ctx context.Context) (int64, error) {
	ensured, err := a.EnsureWakeUpIntents(ctx)
	if err != nil {
		return 0, err
	}
	if scheduled, err := a.ScheduleWakeUpTriggers(ctx); err != nil {
		slog.Default().Warn("ReconcileWakeUpIntents schedule Redis triggers failed", "error", err)
	} else if scheduled > 0 {
		slog.Default().Info("ReconcileWakeUpIntents scheduled Redis wake-up hints", "count", scheduled)
	}
	if released, err := a.ReleaseDueWakeUpIntents(ctx, 50); err != nil {
		slog.Default().Warn("ReconcileWakeUpIntents release due intents failed", "error", err)
	} else if released > 0 {
		slog.Default().Info("ReconcileWakeUpIntents released due wake-up intents", "count", released)
	}
	return ensured, nil
}

// RepairWakeUpClocks repairs only missing or uninitialized durable clocks.
// It is safe to run periodically while the Worker is alive and deliberately
// does not requeue failed executions; retry policy remains owned by the
// dispatcher reconciliation path.
func (a *App) RepairWakeUpClocks(ctx context.Context) (int64, error) {
	settings, err := a.readWakeUpSettings(ctx)
	if err != nil {
		return 0, err
	}
	var repaired int64
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		inserted, err := tx.Exec(ctx, `
			INSERT INTO public.platform_workflow_intents(
				intent_id,workflow_id,task_queue,intent_type,payload,status,next_attempt_at
			)
			SELECT
				'wake_up_intent:' || f.id,
				'wake_up:' || f.id,
				'lifecycle',
				'wake_up.current',
				jsonb_build_object('fluctlight_id', f.id, 'cycle', 0),
				'pending',
				now()+($1 * interval '1 second')
			FROM public.fluctlights AS f
			WHERE f.status IN ('active','paused')
			ON CONFLICT (intent_id) DO NOTHING`, settings.IntervalSeconds)
		if err != nil {
			return err
		}
		repaired += inserted.RowsAffected()
		initialized, err := tx.Exec(ctx, `
			UPDATE public.platform_workflow_intents AS i
			SET next_attempt_at=now()+($1 * interval '1 second')
			FROM public.fluctlights AS f
			WHERE i.intent_type='wake_up.current'
			  AND i.payload->>'fluctlight_id'=f.id
			  AND f.status IN ('active','paused')
			  AND i.next_attempt_at IS NULL`, settings.IntervalSeconds)
		if err != nil {
			return err
		}
		repaired += initialized.RowsAffected()
		return nil
	})
	return repaired, err
}

// TriggerWakeUp releases one Fluctlight's durable WakeUp intent immediately.
// It only changes the intent clock; the normal Worker dispatcher remains the
// sole owner of Temporal workflow execution and provider work.
func (a *App) TriggerWakeUp(ctx context.Context, actorID, fluctlightID string) (map[string]any, error) {
	fluctlightID = strings.TrimSpace(fluctlightID)
	if fluctlightID == "" {
		return nil, ErrNotFound
	}
	var result map[string]any
	var release wakeUpRelease
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var owner, fluctlightStatus string
		if err := tx.QueryRow(ctx, `SELECT created_by_actor_id,status FROM public.fluctlights WHERE id=$1 FOR UPDATE`, fluctlightID).Scan(&owner, &fluctlightStatus); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if owner != actorID {
			return ErrUnauthorized
		}
		if fluctlightStatus != "active" {
			return errors.New("fluctlight_not_active")
		}

		intentID := "wake_up_intent:" + fluctlightID
		var intentStatus, workflowID string
		var payloadRaw []byte
		var nextDue time.Time
		var startedAt *time.Time
		if err := tx.QueryRow(ctx, `SELECT workflow_id,status,payload,COALESCE(next_attempt_at,now()),started_at FROM public.platform_workflow_intents WHERE intent_id=$1 AND intent_type='wake_up.current' FOR UPDATE`, intentID).Scan(&workflowID, &intentStatus, &payloadRaw, &nextDue, &startedAt); err != nil {
			if !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			workflowID = "wake_up:" + fluctlightID
			payloadRaw = jsonBytes(map[string]any{"fluctlight_id": fluctlightID, "cycle": 0})
			inserted, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload,status,next_attempt_at) VALUES($1,$2,'lifecycle','wake_up.current',$3,'pending',now()) ON CONFLICT (intent_id) DO NOTHING`, intentID, workflowID, payloadRaw)
			if err != nil {
				return err
			}
			if inserted.RowsAffected() == 0 {
				if err := tx.QueryRow(ctx, `SELECT workflow_id,status,payload,COALESCE(next_attempt_at,now()),started_at FROM public.platform_workflow_intents WHERE intent_id=$1 AND intent_type='wake_up.current' FOR UPDATE`, intentID).Scan(&workflowID, &intentStatus, &payloadRaw, &nextDue, &startedAt); err != nil {
					return err
				}
			} else {
				intentStatus = "pending"
				nextDue = time.Now().UTC()
			}
		}

		var payload map[string]any
		if err := json.Unmarshal(payloadRaw, &payload); err != nil {
			return fmt.Errorf("wake_up_intent_payload_invalid: %w", err)
		}
		cycle := intValue(payload["cycle"])
		release = wakeUpRelease{IntentID: intentID, WorkflowID: workflowID, FluctlightID: fluctlightID, Cycle: cycle, DueAt: nextDue}
		result = map[string]any{"id": fluctlightID, "intent_id": intentID, "workflow_id": workflowID}
		switch intentStatus {
		case "started", "running", "cancel_requested":
			if startedAt != nil && time.Since(*startedAt) < 2*time.Minute {
				result["status"] = "running"
				result["cycle"] = cycle
				return nil
			}
			cycle++
			payload["cycle"] = cycle
			payloadRaw = jsonBytes(payload)
		case "completed":
			cycle++
			payload["cycle"] = cycle
			payloadRaw = jsonBytes(payload)
		}

		if intentStatus == "pending" || intentStatus == "retry" {
			if _, err := tx.Exec(ctx, `UPDATE public.platform_workflow_intents SET next_attempt_at=now(),last_error=NULL WHERE intent_id=$1`, intentID); err != nil {
				return err
			}
		} else {
			if _, err := tx.Exec(ctx, `UPDATE public.platform_workflow_intents SET status='retry',next_attempt_at=now(),started_at=NULL,completed_at=NULL,last_error=NULL,payload=$2 WHERE intent_id=$1`, intentID, payloadRaw); err != nil {
				return err
			}
		}
		release.Cycle = cycle
		release.DueAt = time.Now().UTC()
		result["status"] = "queued"
		result["cycle"] = cycle
		return nil
	})
	if err != nil {
		return nil, err
	}
	if stringValue(result["status"]) == "queued" {
		a.recordWakeUpReleaseDiagnostics(ctx, release, "manual_wake_up")
		if hintErr := a.scheduleWakeUpTriggerWithDelay(ctx, fluctlightID, time.Millisecond); hintErr != nil {
			slog.Default().Warn("Go Core manual wake up Redis trigger hint failed", "fluctlight_id", fluctlightID, "error", hintErr)
		}
	}
	return result, nil
}

func numberOrDefault(value any, fallback float64) float64 {
	if parsed, ok := numberFloat(value); ok {
		return parsed
	}
	return fallback
}

func wakeUpValue(value any, field string) (any, error) {
	switch typed := value.(type) {
	case string:
		text := strings.TrimSpace(typed)
		if text == "" || len([]rune(text)) > 4000 {
			return nil, fmt.Errorf("wake_up_%s_invalid", field)
		}
		return text, nil
	case map[string]any:
		if len(typed) == 0 || len(jsonBytes(typed)) > 12000 {
			return nil, fmt.Errorf("wake_up_%s_invalid", field)
		}
		return typed, nil
	default:
		return nil, fmt.Errorf("wake_up_%s_invalid", field)
	}
}

func normalizeWakeUpAssessment(value map[string]any) (map[string]any, error) {
	if value == nil {
		return nil, errors.New("wake_up_assessment_invalid")
	}
	result := make(map[string]any, len(value)+1)
	for key, raw := range value {
		result[key] = raw
	}
	actionType := normalizeConversationActionType(stringValue(value["action_type"]))
	if actionType == "" || (actionType != "no_op" && !validateSlotKey(actionType)) {
		return nil, errors.New("wake_up_action_type_invalid")
	}
	result["action_type"] = actionType
	if refs, ok := value["evidence_refs"]; ok {
		var normalizedRefs []any
		switch refs.(type) {
		case []any, []string:
			normalizedRefs = arrayValue(refs)
		default:
			return nil, errors.New("wake_up_evidence_refs_invalid")
		}
		if len(normalizedRefs) > 20 {
			return nil, errors.New("wake_up_evidence_refs_invalid")
		}
		for _, ref := range normalizedRefs {
			if text := stringValue(ref); text == "" || len([]rune(text)) > 256 {
				return nil, errors.New("wake_up_evidence_refs_invalid")
			}
		}
		result["evidence_refs"] = normalizedRefs
	} else {
		result["evidence_refs"] = []any{}
	}
	if intent := stringValue(value["response_intent"]); intent != "" {
		if len([]rune(intent)) > 4000 {
			return nil, errors.New("wake_up_response_intent_invalid")
		}
		result["response_intent"] = intent
	}
	return result, nil
}

func wakeUpScheduleStatus(schedule map[string]any) string {
	if len(schedule) == 0 {
		return "missing"
	}
	return firstString(schedule["status"], "ready")
}

// ProcessWakeUp performs one bounded proactive-action assessment. Wake-up is
// not a second cognition/reflection pass: it decides whether a Moment, direct
// message, or installed capability should be proposed, records that decision,
// and schedules the existing reflection workflow against the resulting fact.
// External effects are frozen only after their capability contract and hard
// execution invariants pass; delivery itself remains owned by a Temporal
// action workflow.
func updateWakeUpNextDueTx(ctx context.Context, tx pgx.Tx, fluctlightID string, cycle, intervalSeconds int, now time.Time) (time.Time, error) {
	// Keep each user-idle phase anchored to the accepted message. Legacy
	// activation clocks without an idle epoch retain completion cadence.
	var payloadRaw []byte
	var existingDue *time.Time
	if err := tx.QueryRow(ctx, `
		SELECT payload,next_attempt_at
		FROM public.platform_workflow_intents
		WHERE intent_type='wake_up.current'
		  AND payload->>'fluctlight_id'=$1
		FOR UPDATE`, fluctlightID).Scan(&payloadRaw, &existingDue); errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, ErrNotFound
	} else if err != nil {
		return time.Time{}, err
	}
	if intervalSeconds <= 0 {
		intervalSeconds = defaultWakeUpIntervalSeconds
	}
	payload := decodeObject(payloadRaw)
	nextDue := now.UTC().Add(time.Duration(intervalSeconds) * time.Second)
	if clock, ok := wakeUpIdleClockFromPayload(payload); ok {
		if cycle > 0 && intValue(payload["idle_last_settled_cycle"]) == cycle && existingDue != nil {
			return existingDue.UTC(), nil
		}
		nextClock := nextWakeUpIdleClock(clock, now, intervalSeconds)
		nextDue = wakeUpIdleDue(nextClock, intervalSeconds)
		for !nextDue.After(now.UTC()) {
			nextClock.Slot++
			nextDue = wakeUpIdleDue(nextClock, intervalSeconds)
		}
		payload["idle_slot"] = nextClock.Slot
		payload["idle_phase"] = wakeUpIdlePhase(nextClock.Slot)
		payload["idle_last_settled_cycle"] = cycle
	} else {
		for !nextDue.After(now.UTC()) {
			nextDue = nextDue.Add(time.Duration(intervalSeconds) * time.Second)
		}
	}
	command, err := tx.Exec(ctx, `
		UPDATE public.platform_workflow_intents
		SET next_attempt_at=$2,attempt_count=0,payload=$3,status='completed',completed_at=now(),started_at=NULL,last_error=NULL
		WHERE intent_type='wake_up.current'
		  AND payload->>'fluctlight_id'=$1`, fluctlightID, nextDue, jsonBytes(payload))
	if err != nil {
		return time.Time{}, err
	}
	if command.RowsAffected() != 1 {
		return time.Time{}, errors.New("wake_up_clock_not_written")
	}
	return nextDue, nil
}

func insertReflectionIntentTx(ctx context.Context, tx pgx.Tx, intentID, workflowID string, payload map[string]any) error {
	return insertReflectionIntentWithDelayTx(ctx, tx, intentID, workflowID, payload, 0)
}

// insertReflectionIntentWithDelayTx keeps the historical configured interval
// for independent action/outcome producers while allowing the cognition and
// Wake-up chains to opt into their explicit ten-minute debounce contract.
// A zero delay means "use the product Wake-up setting" for those independent
// producers; a positive delay is measured from this LLM completion boundary.
func insertReflectionIntentWithDelayTx(ctx context.Context, tx pgx.Tx, intentID, workflowID string, payload map[string]any, delay time.Duration) error {
	if delay <= 0 {
		settings := defaultWakeUpSettings()
		var raw string
		err := tx.QueryRow(ctx, `SELECT value_json FROM public.runtime_settings WHERE key='product.wakeup'`).Scan(&raw)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if err == nil {
			var value map[string]any
			if json.Unmarshal([]byte(raw), &value) != nil {
				return errors.New("product_wakeup_setting_invalid")
			}
			settings = normalizeWakeUpSettings(value)
		}
		delay = time.Duration(settings.IntervalSeconds) * time.Second
	}
	nextReflectionAt := time.Now().UTC().Add(delay)
	command, err := tx.Exec(ctx, `
		INSERT INTO public.platform_workflow_intents(
			intent_id,workflow_id,task_queue,intent_type,payload,next_attempt_at
		) VALUES($1,$2,'lifecycle','reflection.run',$3,$4)
		ON CONFLICT(intent_id) DO UPDATE SET
			next_attempt_at=CASE
				WHEN public.platform_workflow_intents.status IN ('pending','retry')
				THEN GREATEST(public.platform_workflow_intents.next_attempt_at,excluded.next_attempt_at)
				ELSE public.platform_workflow_intents.next_attempt_at
			END`, intentID, workflowID, jsonBytes(payload), nextReflectionAt)
	if err != nil {
		return err
	}
	if command.RowsAffected() != 1 {
		return errors.New("reflection_intent_not_written")
	}
	return nil
}
