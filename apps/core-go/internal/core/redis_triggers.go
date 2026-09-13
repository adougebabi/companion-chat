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

func (a *App) reflectionDelay(ctx context.Context) time.Duration {
	settings, err := a.readWakeUpSettings(ctx)
	if err != nil || settings.IntervalSeconds <= 0 {
		return defaultWakeUpIntervalSeconds * time.Second
	}
	return time.Duration(settings.IntervalSeconds) * time.Second
}

func (a *App) scheduleReflectionTrigger(ctx context.Context, intentID string, delay time.Duration) {
	if a == nil || a.Redis == nil || strings.TrimSpace(intentID) == "" {
		return
	}
	if delay <= 0 {
		delay = a.reflectionDelay(ctx)
	}
	_ = a.Redis.Set(ctx, reflectionTriggerPrefix+intentID, intentID, delay).Err()
}

func (a *App) scheduleWakeUpTrigger(ctx context.Context, fluctlightID string, intervalSeconds int) error {
	if a == nil || a.Redis == nil || strings.TrimSpace(fluctlightID) == "" {
		return nil
	}
	if intervalSeconds <= 0 {
		intervalSeconds = defaultWakeUpIntervalSeconds
	}
	return a.Redis.Set(ctx, wakeUpTriggerPrefix+fluctlightID, fluctlightID, time.Duration(intervalSeconds)*time.Second).Err()
}

// ScheduleWakeUpTriggers repairs Redis quiet-period hints for completed
// wake-up intents. It never creates a workflow and is safe to run at every
// Worker start.
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
		SET next_attempt_at=now()+($1 * interval '1 second')
		WHERE intent_type='wake_up.current'
		  AND status='completed'
		  AND next_attempt_at IS NULL`, settings.IntervalSeconds); err != nil {
		return 0, err
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT DISTINCT payload->>'fluctlight_id',next_attempt_at FROM public.platform_workflow_intents WHERE intent_type='wake_up.current' AND status='completed' AND next_attempt_at>now() AND payload->>'fluctlight_id' IS NOT NULL`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	var scheduled int64
	for rows.Next() {
		var fluctlightID string
		var dueAt time.Time
		if err := rows.Scan(&fluctlightID, &dueAt); err != nil {
			return scheduled, err
		}
		delay := time.Until(dueAt)
		seconds := int((delay + time.Second - 1) / time.Second)
		if seconds < 1 {
			seconds = 1
		}
		if err := a.scheduleWakeUpTrigger(ctx, fluctlightID, seconds); err != nil {
			return scheduled, fmt.Errorf("schedule WakeUp Redis hint: %w", err)
		}
		scheduled++
	}
	return scheduled, rows.Err()
}

// HandleRedisExpiredTrigger turns a best-effort keyevent into a durable
// dispatcher hint. Expiring a completed wake-up advances its cycle and makes
// the one-shot intent retryable; duplicate/lost notifications remain harmless
// because PostgreSQL and stable workflow IDs are authoritative.
func (a *App) HandleRedisExpiredTrigger(ctx context.Context, key string) error {
	if strings.HasPrefix(key, reflectionTriggerPrefix) {
		intentID := strings.TrimPrefix(key, reflectionTriggerPrefix)
		if intentID == "" {
			return nil
		}
		_, err := a.DB.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET next_attempt_at=now() WHERE intent_id=$1 AND intent_type='reflection.run' AND status IN ('pending','retry')`, intentID)
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
