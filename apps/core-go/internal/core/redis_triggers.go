package core

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	reflectionTriggerPrefix = "fluctlight:reflection:due:"
	wakeUpTriggerPrefix     = "fluctlight:wakeup:due:"
)

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

func (a *App) scheduleWakeUpTrigger(ctx context.Context, fluctlightID string, intervalSeconds int) {
	if a == nil || a.Redis == nil || strings.TrimSpace(fluctlightID) == "" {
		return
	}
	if intervalSeconds <= 0 {
		intervalSeconds = defaultWakeUpIntervalSeconds
	}
	_ = a.Redis.Set(ctx, wakeUpTriggerPrefix+fluctlightID, fluctlightID, time.Duration(intervalSeconds)*time.Second).Err()
}

// ScheduleWakeUpTriggers repairs Redis hints for durable wake-up intents. It
// never creates a workflow and is safe to run at every Worker start.
func (a *App) ScheduleWakeUpTriggers(ctx context.Context) (int64, error) {
	if a == nil || a.Redis == nil {
		return 0, nil
	}
	settings, err := a.readWakeUpSettings(ctx)
	if err != nil {
		return 0, err
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT DISTINCT payload->>'fluctlight_id' FROM public.platform_workflow_intents WHERE intent_type='wake_up.current' AND status IN ('pending','retry','started') AND payload->>'fluctlight_id' IS NOT NULL`)
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
		a.scheduleWakeUpTrigger(ctx, fluctlightID, settings.IntervalSeconds)
		scheduled++
	}
	return scheduled, rows.Err()
}

// HandleRedisExpiredTrigger turns a best-effort keyevent into a durable
// dispatcher hint. The PG status predicate and stable workflow IDs make
// duplicate/lost notifications harmless; the regular dispatcher scan remains
// the recovery path when Pub/Sub is disconnected.
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
		_, err := a.DB.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET next_attempt_at=now() WHERE intent_type='wake_up.current' AND payload->>'fluctlight_id'=$1 AND status IN ('pending','retry')`, fluctlightID)
		return err
	}
	return nil
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
