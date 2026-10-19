package workflow

import (
	"context"
	"time"

	"go.temporal.io/sdk/activity"
)

// Liveness/cancellation observation only; PostgreSQL intents own scheduling.
func startActivityHeartbeat(ctx context.Context, identity map[string]any) func() {
	if !activity.IsActivity(ctx) {
		return func() {}
	}
	activity.RecordHeartbeat(ctx, identity)
	heartbeatCtx, cancel := context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-heartbeatCtx.Done():
				return
			case <-ticker.C:
				activity.RecordHeartbeat(heartbeatCtx, identity)
			}
		}
	}()
	return cancel
}
