package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type studioApplication interface {
	ProcessStudioGeneration(context.Context, string) (map[string]any, error)
}

func StudioWorkflow(ctx workflow.Context, input Input) (map[string]any, error) {
	if input.IntentID == "" {
		return nil, fmt.Errorf("studio workflow input invalid")
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
	err = workflow.ExecuteActivity(ctx, ProcessStudioActivity, input).Get(ctx, &result)
	return result, err
}
func ProcessStudioActivity(ctx context.Context, input Input) (map[string]any, error) {
	application, ok := app().(studioApplication)
	if !ok {
		return nil, fmt.Errorf("studio worker not configured")
	}
	ctx, input = prepareActivityLifecycleContext(ctx, input, "studio")
	recordActivityLifecycle(app(), ctx, input, "studio", core.LifecycleTransitionActivityStarted, "running", "activity_started", nil)
	stop := startActivityHeartbeat(ctx, map[string]any{"studio_attempt_id": input.IntentID, "phase": "studio_generation"})
	defer stop()
	result, err := application.ProcessStudioGeneration(ctx, input.IntentID)
	if err != nil {
		recordActivityLifecycle(app(), ctx, input, "studio", core.LifecycleTransitionFailed, "failed", "studio_generation_failed", err)
	} else {
		transition, status, reason := actionLifecycleOutcome(result)
		recordActivityLifecycle(app(), ctx, input, "studio", transition, status, reason, nil)
	}
	return result, err
}
