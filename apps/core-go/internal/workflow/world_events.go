package workflow

import (
	"context"
	"fmt"
	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type worldEventGenerationApplication interface {
	GenerateWorldEventCycle(context.Context, string) (map[string]any, error)
}

// A world owns one durable clock. Missed hours are not replayed, and actor
// pause/profile changes do not stop the independent world clock.
func WorldEventGenerationWorkflow(ctx workflow.Context, input Input) (map[string]any, error) {
	if input.WorldID == "" {
		return nil, fmt.Errorf("world event world required")
	}
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: 15 * time.Minute, HeartbeatTimeout: time.Minute, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 1}})
	control, err := registerWorkflowControl(ctx)
	if err != nil {
		return nil, err
	}
	if err := control.waitUntilResumed(ctx); err != nil {
		return nil, err
	}
	var result map[string]any
	started := workflow.Now(ctx)
	if err := workflow.ExecuteActivity(ctx, GenerateWorldEventActivity, input).Get(ctx, &result); err != nil {
		workflow.GetLogger(ctx).Error("world event generation failed", "world_id", input.WorldID, "error", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := control.waitUntilResumed(ctx); err != nil {
		return nil, err
	}
	if result["status"] == "world_deleted" {
		return result, nil
	}
	now := workflow.Now(ctx)
	next := started.UTC().Truncate(time.Hour).Add(time.Hour)
	if raw, ok := result["next_at"].(string); ok {
		if at, err := time.Parse(time.RFC3339Nano, raw); err == nil {
			next = at
		}
	}
	if next.After(now) {
		if err := workflow.Sleep(ctx, next.Sub(now)); err != nil {
			return nil, err
		}
	}
	return nil, workflow.NewContinueAsNewError(ctx, WorldEventGenerationWorkflow, input)
}

func GenerateWorldEventActivity(ctx context.Context, input Input) (map[string]any, error) {
	application, ok := app().(worldEventGenerationApplication)
	if !ok {
		return nil, fmt.Errorf("world event generation worker not configured")
	}
	stop := startActivityHeartbeat(ctx, map[string]any{"world_id": input.WorldID, "phase": "world_event_generation"})
	defer stop()
	ctx = core.WithPromptDiagnostics(ctx, map[string]any{"world_clock_correlation_id": input.CorrelationID, "world_clock_intent_id": input.IntentID})
	return application.GenerateWorldEventCycle(ctx, input.WorldID)
}
