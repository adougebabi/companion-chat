package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

const (
	reflectionTriggerPrefix = "fluctlight:reflection:due:"
	wakeUpTriggerPrefix     = "fluctlight:wakeup:due:"
	reflectionQuietPeriod   = 30 * time.Minute
	wakeUpDueSweepLimit     = 50
	wakeUpOverdueGrace      = 2 * time.Minute
	wakeUpOverdueEventType  = "lifecycle.wake_up.overdue"
)

type wakeUpRelease struct {
	IntentID     string
	WorkflowID   string
	FluctlightID string
	Cycle        int
	DueAt        time.Time
}

func wakeUpCycleCorrelation(fluctlightID string, cycle int) string {
	return fmt.Sprintf("wake_up:%s:cycle:%d", strings.TrimSpace(fluctlightID), cycle)
}

var ErrWakeUpClockUnavailable = errors.New("wake_up_clock_unavailable")

func (a *App) reflectionDelay(_ context.Context) time.Duration {
	return reflectionQuietPeriod
}

func (a *App) scheduleReflectionTrigger(ctx context.Context, fluctlightID string, delay time.Duration) error {
	if a == nil || a.Redis == nil || strings.TrimSpace(fluctlightID) == "" {
		return nil
	}
	fluctlightID = strings.TrimSpace(fluctlightID)
	if delay <= 0 {
		delay = reflectionQuietPeriod
	}
	return a.Redis.Set(ctx, reflectionTriggerPrefix+fluctlightID, fluctlightID, delay).Err()
}

func (a *App) scheduleWakeUpTrigger(ctx context.Context, fluctlightID string, intervalSeconds int) error {
	if intervalSeconds <= 0 {
		intervalSeconds = defaultWakeUpIntervalSeconds
	}
	return a.scheduleWakeUpTriggerWithDelay(ctx, fluctlightID, time.Duration(intervalSeconds)*time.Second)
}

func (a *App) scheduleWakeUpTriggerWithDelay(ctx context.Context, fluctlightID string, delay time.Duration) error {
	if a == nil || a.Redis == nil || strings.TrimSpace(fluctlightID) == "" {
		return nil
	}
	if delay <= 0 {
		delay = time.Duration(defaultWakeUpIntervalSeconds) * time.Second
	}
	return a.Redis.Set(ctx, wakeUpTriggerPrefix+fluctlightID, fluctlightID, delay).Err()
}

func wakeUpAfterCognitionDelay(_ int) time.Duration {
	return wakeUpFirstIdleDelay
}

// Cognition completion releases a preempted clock without changing the
// user-message idle epoch. The due time remains an absolute offset from the
// accepted message, even if the model took longer than ten minutes.
func (a *App) scheduleWakeUpAfterCognition(ctx context.Context, fluctlightID string) error {
	if a == nil || strings.TrimSpace(fluctlightID) == "" {
		return nil
	}
	settings := defaultWakeUpSettings()
	var err error
	if a.DB != nil && a.DB.Pool() != nil {
		settings, err = a.readWakeUpSettings(ctx)
		if err != nil {
			return err
		}
	}
	nextDue := a.now().UTC().Add(wakeUpAfterCognitionDelay(settings.IntervalSeconds))
	if a.DB != nil && a.DB.Pool() != nil {
		if err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('fluctlight_lifecycle:' || $1))`, fluctlightID); err != nil {
				return err
			}
			var payloadRaw []byte
			if err := tx.QueryRow(ctx, `SELECT payload FROM public.platform_workflow_intents WHERE intent_type='wake_up.current' AND payload->>'fluctlight_id'=$1 FOR UPDATE`, fluctlightID).Scan(&payloadRaw); err != nil {
				return err
			}
			if clock, ok := wakeUpIdleClockFromPayload(decodeObject(payloadRaw)); ok {
				nextDue = wakeUpIdleDue(clock, settings.IntervalSeconds)
			}
			command, err := tx.Exec(ctx, `
				UPDATE public.platform_workflow_intents
				SET status='completed',started_at=NULL,completed_at=COALESCE(completed_at,now()),last_error=NULL,next_attempt_at=$2
				WHERE intent_type='wake_up.current'
				  AND payload->>'fluctlight_id'=$1
				  AND status IN ('pending','retry','started','running','cancel_requested','superseded','completed','failed','dead_letter')`, fluctlightID, nextDue)
			if err != nil {
				return err
			}
			if command.RowsAffected() != 1 {
				return ErrNotFound
			}
			return nil
		}); err != nil {
			return err
		}
	}
	delay := time.Until(nextDue)
	if delay <= 0 {
		delay = time.Second
	}
	return a.scheduleWakeUpTriggerWithDelay(ctx, fluctlightID, delay)
}

func (a *App) scheduleCognitionFollowups(ctx context.Context, fluctlightID string) error {
	// The two clocks are independent.  A Redis failure on the Reflection hint
	// must not prevent the durable Wake-up clock from being moved (and vice
	// versa); return the first error only after both maintenance attempts ran.
	var firstErr error
	if err := a.schedulePendingReflectionTrigger(ctx, fluctlightID); err != nil {
		firstErr = err
	}
	if err := a.scheduleWakeUpAfterCognition(ctx, fluctlightID); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

// schedulePendingReflectionTrigger mirrors the authoritative PostgreSQL due
// time into Redis. In particular, cognition completion cannot move Reflection
// thirty minutes past the model's completion when the quiet boundary is based
// on an earlier chat publication.
func (a *App) schedulePendingReflectionTrigger(ctx context.Context, fluctlightID string) error {
	if a == nil || a.Redis == nil || strings.TrimSpace(fluctlightID) == "" {
		return nil
	}
	if a.DB == nil || a.DB.Pool() == nil {
		return a.scheduleReflectionTrigger(ctx, fluctlightID, reflectionQuietPeriod)
	}
	var due time.Time
	err := a.DB.Pool().QueryRow(ctx, `
		SELECT next_attempt_at
		FROM public.platform_workflow_intents
		WHERE intent_type='reflection.run'
		  AND payload->>'fluctlight_id'=$1
		  AND status IN ('pending','retry')
		  AND next_attempt_at IS NOT NULL
		ORDER BY created_at DESC,intent_id DESC
		LIMIT 1`, strings.TrimSpace(fluctlightID)).Scan(&due)
	if errors.Is(err, pgx.ErrNoRows) {
		return a.Redis.Del(ctx, reflectionTriggerPrefix+strings.TrimSpace(fluctlightID)).Err()
	}
	if err != nil {
		return err
	}
	delay := time.Until(due.UTC())
	if delay <= 0 {
		delay = time.Second
	}
	return a.scheduleReflectionTrigger(ctx, fluctlightID, delay)
}

// ScheduleWakeUpTriggers repairs Redis quiet-period hints for completed
// wake-up intents. An existing key is never rewritten at Worker startup. A
// missing key releases exactly one durable cycle, after which normal WakeUp
// settlement establishes the recurring ten-minute key.
func (a *App) ScheduleWakeUpTriggers(ctx context.Context) (int64, error) {
	if a == nil || a.Redis == nil {
		return 0, nil
	}
	settings, err := a.readWakeUpSettings(ctx)
	if err != nil {
		return 0, err
	}
	if _, err := a.DB.Pool().Exec(ctx, `
		UPDATE public.platform_workflow_intents
		SET next_attempt_at=now()+interval '10 minutes'
		WHERE intent_type='wake_up.current'
		  AND status='completed'
		  AND next_attempt_at IS NULL`); err != nil {
		return 0, err
	}
	if !settings.Enabled {
		return 0, nil
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT DISTINCT i.payload->>'fluctlight_id' FROM public.platform_workflow_intents i JOIN public.fluctlights f ON f.id=i.payload->>'fluctlight_id' WHERE i.intent_type='wake_up.current' AND i.status='completed' AND f.status='active' AND i.payload->>'fluctlight_id' IS NOT NULL`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	var scheduled int64
	for rows.Next() {
		var fluctlightID string
		if err := rows.Scan(&fluctlightID); err != nil {
			return scheduled, err
		}
		key := wakeUpTriggerPrefix + fluctlightID
		claimed, claimErr := a.Redis.SetNX(ctx, key, fluctlightID, time.Minute).Result()
		if claimErr != nil {
			return scheduled, fmt.Errorf("claim WakeUp Redis hint: %w", claimErr)
		}
		if !claimed {
			continue
		}
		released, releaseErr := a.releaseWakeUpIntentNow(ctx, fluctlightID, "startup_missing_redis_key")
		if releaseErr != nil {
			_ = a.Redis.Del(ctx, key).Err()
			return scheduled, releaseErr
		}
		if !released {
			continue
		}
		scheduled++
	}
	return scheduled, rows.Err()
}

func (a *App) releaseWakeUpIntentNow(ctx context.Context, fluctlightID, reason string) (bool, error) {
	var release wakeUpRelease
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('fluctlight_lifecycle:' || $1))`, fluctlightID); err != nil {
			return err
		}
		return tx.QueryRow(ctx, `
		UPDATE public.platform_workflow_intents AS i
		SET status='retry',started_at=NULL,completed_at=NULL,next_attempt_at=now(),
			payload=jsonb_set(i.payload,'{cycle}',to_jsonb(COALESCE((i.payload->>'cycle')::integer,0)+1),true)
		FROM public.fluctlights AS f
		WHERE i.intent_type='wake_up.current'
		  AND i.payload->>'fluctlight_id'=$1
		  AND i.status='completed'
		  AND f.id=$1 AND f.status='active'
		  AND NOT EXISTS (
			SELECT 1 FROM public.platform_workflow_intents c
			WHERE c.intent_type='cognition.processing'
			  AND c.payload->>'fluctlight_id'=$1
			  AND c.status IN ('pending','retry','started','running','cancel_requested')
		  )
		RETURNING i.intent_id,i.workflow_id,i.payload->>'fluctlight_id',
			(i.payload->>'cycle')::integer,i.next_attempt_at`, fluctlightID).
			Scan(&release.IntentID, &release.WorkflowID, &release.FluctlightID, &release.Cycle, &release.DueAt)
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	a.recordWakeUpReleaseDiagnostics(ctx, release, reason)
	return true, nil
}

// HandleRedisExpiredTrigger turns a best-effort keyevent into a durable
// dispatcher hint. Expiring a completed wake-up advances its cycle and makes
// the one-shot intent retryable; duplicate/lost notifications remain harmless
// because PostgreSQL and stable workflow IDs are authoritative.
func (a *App) HandleRedisExpiredTrigger(ctx context.Context, key string) error {
	if strings.TrimSpace(key) == "" {
		return nil
	}
	// Keyspace notifications are best-effort and can be delivered just after a
	// newer cognition rewrites the same debounce key.  If the key exists again,
	// this event is stale; leave the newer TTL untouched and let its own expiry
	// release the durable intent.
	if a != nil && a.Redis != nil {
		exists, err := a.Redis.Exists(ctx, key).Result()
		if err != nil {
			return err
		}
		if exists > 0 {
			return nil
		}
	}
	if strings.HasPrefix(key, reflectionTriggerPrefix) {
		identity := strings.TrimPrefix(key, reflectionTriggerPrefix)
		if identity == "" {
			return nil
		}
		if strings.HasPrefix(identity, "reflection_intent:") {
			_, err := a.DB.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET next_attempt_at=now() WHERE intent_id=$1 AND intent_type='reflection.run' AND status IN ('pending','retry')`, identity)
			return err
		}
		_, err := a.DB.Pool().Exec(ctx, `
			UPDATE public.platform_workflow_intents
			SET next_attempt_at=now()
			WHERE intent_id=(
				SELECT intent_id
				FROM public.platform_workflow_intents
				WHERE intent_type='reflection.run'
				  AND payload->>'fluctlight_id'=$1
				  AND status IN ('pending','retry')
				ORDER BY created_at DESC,intent_id DESC
				LIMIT 1
			)
			AND status IN ('pending','retry')`, identity)
		return err
	}
	if strings.HasPrefix(key, wakeUpTriggerPrefix) {
		fluctlightID := strings.TrimPrefix(key, wakeUpTriggerPrefix)
		if fluctlightID == "" {
			return nil
		}
		_, err := a.releaseWakeUpIntent(ctx, fluctlightID, "redis_expired")
		return err
	}
	return nil
}

// ReleaseDueWakeUpIntents is the PostgreSQL correctness path for recurring
// WakeUp. Redis expiry may call the same release boundary earlier, but a lost
// notification cannot strand a completed intent.
func (a *App) ReleaseDueWakeUpIntents(ctx context.Context, limit int) (int64, error) {
	if a == nil || a.DB == nil || a.DB.Pool() == nil {
		return 0, ErrWakeUpClockUnavailable
	}
	if limit < 1 || limit > wakeUpDueSweepLimit {
		limit = wakeUpDueSweepLimit
	}
	rows, err := a.DB.Pool().Query(ctx, `
		WITH candidates AS (
			SELECT i.intent_id,i.workflow_id,i.next_attempt_at,
				i.payload->>'fluctlight_id' AS fluctlight_id,
				COALESCE((i.payload->>'cycle')::integer,0)+1 AS next_cycle
			FROM public.platform_workflow_intents AS i
			JOIN public.fluctlights AS f ON f.id=i.payload->>'fluctlight_id'
			WHERE i.intent_type='wake_up.current'
			  AND i.status='completed'
			  AND i.next_attempt_at <= now()
			  AND f.status='active'
			ORDER BY i.next_attempt_at,i.intent_id
			FOR UPDATE OF i SKIP LOCKED
			LIMIT $1
		), released AS (
			UPDATE public.platform_workflow_intents AS i
			SET status='retry',
				started_at=NULL,
				completed_at=NULL,
				payload=jsonb_set(i.payload,'{cycle}',to_jsonb(c.next_cycle),true)
			FROM candidates AS c
			WHERE i.intent_id=c.intent_id
			  AND i.status='completed'
			  AND i.next_attempt_at=c.next_attempt_at
			RETURNING i.intent_id,i.workflow_id,i.payload->>'fluctlight_id' AS fluctlight_id,
				(i.payload->>'cycle')::integer AS cycle,i.next_attempt_at
		)
		SELECT intent_id,workflow_id,fluctlight_id,cycle,next_attempt_at FROM released`, limit)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	releases := make([]wakeUpRelease, 0)
	for rows.Next() {
		var release wakeUpRelease
		if err := rows.Scan(&release.IntentID, &release.WorkflowID, &release.FluctlightID, &release.Cycle, &release.DueAt); err != nil {
			return int64(len(releases)), err
		}
		releases = append(releases, release)
	}
	if err := rows.Err(); err != nil {
		return int64(len(releases)), err
	}
	for _, release := range releases {
		a.recordWakeUpReleaseDiagnostics(ctx, release, "postgres_due_sweep")
	}
	return int64(len(releases)), nil
}

func (a *App) releaseWakeUpIntent(ctx context.Context, fluctlightID, reason string) (bool, error) {
	var release wakeUpRelease
	err := a.DB.Pool().QueryRow(ctx, `
		UPDATE public.platform_workflow_intents AS i
		SET status='retry',
			started_at=NULL,
			completed_at=NULL,
			payload=jsonb_set(i.payload,'{cycle}',to_jsonb(COALESCE((i.payload->>'cycle')::integer,0)+1),true)
		FROM public.fluctlights AS f
		WHERE i.intent_type='wake_up.current'
		  AND i.payload->>'fluctlight_id'=$1
		  AND i.status='completed'
		  AND i.next_attempt_at <= now()
		  AND f.id=$1
		  AND f.status='active'
		RETURNING i.intent_id,i.workflow_id,i.payload->>'fluctlight_id',
			(i.payload->>'cycle')::integer,i.next_attempt_at`, fluctlightID).
		Scan(&release.IntentID, &release.WorkflowID, &release.FluctlightID, &release.Cycle, &release.DueAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	a.recordWakeUpReleaseDiagnostics(ctx, release, reason)
	return true, nil
}

func (a *App) recordWakeUpReleaseDiagnostics(ctx context.Context, release wakeUpRelease, reason string) {
	correlationID := wakeUpCycleCorrelation(release.FluctlightID, release.Cycle)
	if time.Since(release.DueAt) >= wakeUpOverdueGrace {
		a.RecordLifecycleDiagnosticBestEffort(ctx, LifecycleDiagnostic{
			Surface: "wake_up", Transition: LifecycleTransitionOverdue, Severity: "warn",
			FluctlightID: release.FluctlightID, CorrelationID: correlationID,
			IntentID: release.IntentID, WorkflowID: release.WorkflowID,
			Stage: "trigger", Status: "overdue", ReasonCode: "wake_up_due_overdue",
			Retryable: true, Attempt: release.Cycle,
			Metadata: map[string]any{"diagnostic_type": wakeUpOverdueEventType},
		})
	}
	a.RecordLifecycleDiagnosticBestEffort(ctx, LifecycleDiagnostic{
		Surface: "wake_up", Transition: LifecycleTransitionTriggerReleased,
		FluctlightID: release.FluctlightID, CorrelationID: correlationID,
		IntentID: release.IntentID, WorkflowID: release.WorkflowID,
		Stage: "trigger", Status: "retry", ReasonCode: reason,
		Retryable: true, Attempt: release.Cycle,
	})
}

func (a *App) AuditWakeUpClocks(ctx context.Context, limit int) (int64, error) {
	if a == nil || a.DB == nil || a.DB.Pool() == nil {
		return 0, ErrWakeUpClockUnavailable
	}
	if limit < 1 || limit > wakeUpDueSweepLimit {
		limit = wakeUpDueSweepLimit
	}
	// Recover wake_up.current intents stuck in started/running for >= 15 minutes.
	if _, reapErr := a.DB.Pool().Exec(ctx, `
		UPDATE public.platform_workflow_intents
		SET status='retry',
			started_at=NULL,
			completed_at=NULL,
			next_attempt_at=now(),
			last_error='reaped_stuck_wake_up'
		WHERE intent_type='wake_up.current'
		  AND status IN ('started','running','cancel_requested')
		  AND (started_at < now() - interval '15 minutes' OR (started_at IS NULL AND created_at < now() - interval '15 minutes'))
	`); reapErr != nil {
		return 0, reapErr
	}
	// Recover wake_up.current intents stuck in cancelled, failed, or dead_letter.
	if _, reapErr := a.DB.Pool().Exec(ctx, `
		UPDATE public.platform_workflow_intents AS i
		SET status='retry',
			started_at=NULL,
			completed_at=NULL,
			next_attempt_at=now(),
			last_error=COALESCE(i.last_error, 'reaped_dead_wake_up')
		FROM public.fluctlights AS f
		WHERE i.intent_type='wake_up.current'
		  AND i.payload->>'fluctlight_id'=f.id
		  AND f.status IN ('active', 'paused')
		  AND i.status IN ('cancelled', 'failed', 'dead_letter')
	`); reapErr != nil {
		return 0, reapErr
	}
	// Recover wake_up.current intents superseded where cognition has completed.
	if _, reapErr := a.DB.Pool().Exec(ctx, `
		UPDATE public.platform_workflow_intents AS i
		SET status='completed',
			started_at=NULL,
			completed_at=now(),
			next_attempt_at=now(),
			last_error=NULL
		FROM public.fluctlights AS f
		WHERE i.intent_type='wake_up.current'
		  AND i.payload->>'fluctlight_id'=f.id
		  AND f.status IN ('active', 'paused')
		  AND i.status='superseded'
		  AND NOT EXISTS (
			SELECT 1 FROM public.cognition_inbox AS c
			JOIN public.platform_workflow_intents AS w ON w.intent_id='cognition_intent:'||c.id
			WHERE c.fluctlight_id=f.id AND c.status IN ('pending','claimed')
			  AND w.status IN ('pending','retry','started','running','cancel_requested')
		  )
	`); reapErr != nil {
		return 0, reapErr
	}
	// Sweep overdue completed wake_up.current intents that missed triggers.
	if _, reapErr := a.DB.Pool().Exec(ctx, `
		UPDATE public.platform_workflow_intents AS i
		SET status='retry',
			started_at=NULL,
			completed_at=NULL,
			payload=jsonb_set(i.payload,'{cycle}',to_jsonb(COALESCE((i.payload->>'cycle')::integer,0)+1),true)
		FROM public.fluctlights AS f
		WHERE i.intent_type='wake_up.current'
		  AND i.payload->>'fluctlight_id'=f.id
		  AND f.status='active'
		  AND i.status='completed'
		  AND i.next_attempt_at <= now() - interval '15 minutes'
	`); reapErr != nil {
		return 0, reapErr
	}
	rows, err := a.DB.Pool().Query(ctx, `
		SELECT f.id
		FROM public.fluctlights AS f
		LEFT JOIN public.platform_workflow_intents AS i
		  ON i.intent_type='wake_up.current' AND i.payload->>'fluctlight_id'=f.id
		WHERE f.status='active' AND i.intent_id IS NULL
		ORDER BY f.id
		LIMIT $1`, limit)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	var count int64
	for rows.Next() {
		var fluctlightID string
		if err := rows.Scan(&fluctlightID); err != nil {
			return count, err
		}
		count++
		a.RecordLifecycleDiagnosticBestEffort(ctx, LifecycleDiagnostic{
			Surface: "wake_up", Transition: LifecycleTransitionOverdue, Severity: "error",
			FluctlightID: fluctlightID, CorrelationID: "wake_up:" + fluctlightID + ":missing",
			Stage: "clock", Status: "missing", ReasonCode: "wake_up_intent_missing",
			Retryable: true, Metadata: map[string]any{"diagnostic_type": wakeUpOverdueEventType},
		})
	}
	return count, rows.Err()
}

// RedisTriggerListener consumes non-durable expiration hints. Re-subscribing
// after disconnect and the Worker's periodic PG scans cover lost Pub/Sub
// messages and Redis restarts.
type RedisTriggerListener struct {
	App   *App
	Redis redis.UniversalClient
	Retry time.Duration
}

func NewRedisTriggerListener(app *App, client redis.UniversalClient) *RedisTriggerListener {
	return &RedisTriggerListener{App: app, Redis: client, Retry: time.Second}
}

func (l *RedisTriggerListener) Run(ctx context.Context) error {
	if l == nil || l.App == nil || l.Redis == nil {
		return nil
	}
	retry := l.Retry
	if retry <= 0 {
		retry = time.Second
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		pubsub := l.Redis.PSubscribe(ctx, "__keyevent@*__:expired")
		if _, err := pubsub.Receive(ctx); err != nil {
			_ = pubsub.Close()
			if !waitRedisTriggerRetry(ctx, retry) {
				return ctx.Err()
			}
			continue
		}
		channel := pubsub.Channel()
		for {
			select {
			case <-ctx.Done():
				_ = pubsub.Close()
				return ctx.Err()
			case message, ok := <-channel:
				if !ok {
					_ = pubsub.Close()
					if !waitRedisTriggerRetry(ctx, retry) {
						return ctx.Err()
					}
					goto reconnect
				}
				if err := l.App.HandleRedisExpiredTrigger(ctx, message.Payload); err != nil && !errors.Is(err, context.Canceled) {
					// The next PG scan retries the hint; listener errors must not
					// terminate the Worker.
					continue
				}
			}
		}
	reconnect:
	}
}

func waitRedisTriggerRetry(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
