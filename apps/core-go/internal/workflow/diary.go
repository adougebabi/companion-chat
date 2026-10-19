package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type diaryApplication interface {
	ProcessDiaryGeneration(context.Context, string) (map[string]any, error)
}

func DiaryWorkflow(ctx workflow.Context, input Input) (map[string]any, error) {
	if input.DiaryAttemptID == "" || input.FluctlightID == "" {
		return nil, fmt.Errorf("diary workflow input invalid")
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
	err = workflow.ExecuteActivity(ctx, ProcessDiaryActivity, input).Get(ctx, &result)
	return result, err
}
func ProcessDiaryActivity(ctx context.Context, input Input) (map[string]any, error) {
	application, ok := app().(diaryApplication)
	if !ok {
		return nil, fmt.Errorf("diary worker not configured")
	}
	ctx, input = prepareActivityLifecycleContext(ctx, input, "diary")
	recordActivityLifecycle(app(), ctx, input, "diary", core.LifecycleTransitionActivityStarted, "running", "activity_started", nil)
	stop := startActivityHeartbeat(ctx, map[string]any{"diary_attempt_id": input.DiaryAttemptID, "phase": "diary_generation"})
	defer stop()
	result, err := application.ProcessDiaryGeneration(ctx, input.DiaryAttemptID)
	if err != nil {
		recordActivityLifecycle(app(), ctx, input, "diary", core.LifecycleTransitionFailed, "failed", "diary_generation_failed", err)
	} else {
		transition, status, reason := actionLifecycleOutcome(result)
		recordActivityLifecycle(app(), ctx, input, "diary", transition, status, reason, nil)
	}
	return result, err
}
