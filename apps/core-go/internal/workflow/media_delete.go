package workflow

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type mediaDeletionApplication interface {
	ProcessMediaDeletion(context.Context, string, string) (map[string]any, error)
}

func MediaDeletionWorkflow(ctx workflow.Context, input Input) (map[string]any, error) {
	if input.IntentID == "" {
		return nil, fmt.Errorf("media deletion input invalid")
	}
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: 5 * time.Minute, HeartbeatTimeout: time.Minute, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 3}})
	control, err := registerWorkflowControl(ctx)
	if err != nil {
		return nil, err
	}
	if err := control.waitUntilResumed(ctx); err != nil {
		return nil, err
	}
	var result map[string]any
	err = workflow.ExecuteActivity(ctx, ProcessMediaDeletionActivity, input).Get(ctx, &result)
	return result, err
}
func ProcessMediaDeletionActivity(ctx context.Context, input Input) (map[string]any, error) {
	application, ok := app().(mediaDeletionApplication)
	if !ok {
		return nil, fmt.Errorf("media deletion worker not configured")
	}
	ctx, input = prepareActivityLifecycleContext(ctx, input, "media_delete")
	recordActivityLifecycle(app(), ctx, input, "media_delete", core.LifecycleTransitionActivityStarted, "running", "media_delete_started", nil)
	stop := startActivityHeartbeat(ctx, map[string]any{"deletion_id": input.IntentID, "phase": "verify_and_delete_exact_version"})
	defer stop()
	result, err := application.ProcessMediaDeletion(ctx, input.IntentID, activity.GetInfo(ctx).WorkflowExecution.ID)
	if err != nil {
		recordActivityLifecycle(app(), ctx, input, "media_delete", core.LifecycleTransitionFailed, "failed", "media_delete_failed", err)
		var surface *core.SurfaceError
		if errors.As(err, &surface) && surface.Status >= 400 && surface.Status < 500 {
			return nil, temporal.NewNonRetryableApplicationError(surface.Message, surface.Code, err)
		}
	} else {
		recordActivityLifecycle(app(), ctx, input, "media_delete", core.LifecycleTransitionCompletedActionable, "completed", "object_version_deleted", nil)
	}
	return result, err
}
