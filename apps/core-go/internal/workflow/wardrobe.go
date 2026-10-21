package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type wardrobePreparationApplication interface {
	ProcessWardrobePreparation(context.Context, string) (map[string]any, error)
}

func WardrobePreparationWorkflow(ctx workflow.Context, input Input) (map[string]any, error) {
	if input.SessionID == "" {
		return nil, fmt.Errorf("wardrobe preparation session required")
	}
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: 15 * time.Minute, HeartbeatTimeout: time.Minute, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 1}})
	control, err := registerWorkflowControl(ctx)
	if err != nil {
		return nil, err
	}
	if err = control.waitUntilResumed(ctx); err != nil {
		return nil, err
	}
	var result map[string]any
	if err = workflow.ExecuteActivity(ctx, ProcessWardrobePreparationActivity, input).Get(ctx, &result); err != nil {
		return nil, err
	}
	if result["status"] == "awaiting_wear" {
		if err = workflow.Sleep(ctx, time.Minute); err != nil {
			return nil, err
		}
		return nil, workflow.NewContinueAsNewError(ctx, WardrobePreparationWorkflow, input)
	}
	return result, nil
}
func ProcessWardrobePreparationActivity(ctx context.Context, input Input) (map[string]any, error) {
	application, ok := app().(wardrobePreparationApplication)
	if !ok {
		return nil, fmt.Errorf("wardrobe preparation worker not configured")
	}
	ctx, input = prepareActivityLifecycleContext(ctx, input, "wardrobe")
	recordActivityLifecycle(app(), ctx, input, "wardrobe", core.LifecycleTransitionActivityStarted, "running", "wardrobe_preparation_started", nil)
	stop := startActivityHeartbeat(ctx, map[string]any{"session_id": input.SessionID, "phase": "wardrobe_preparation"})
	defer stop()
	result, err := application.ProcessWardrobePreparation(ctx, input.SessionID)
	if err != nil {
		recordActivityLifecycle(app(), ctx, input, "wardrobe", core.LifecycleTransitionFailed, "failed", "wardrobe_preparation_failed", err)
	} else {
		transition, status, reason := actionLifecycleOutcome(result)
		recordActivityLifecycle(app(), ctx, input, "wardrobe", transition, status, reason, nil)
	}
	return result, err
}
