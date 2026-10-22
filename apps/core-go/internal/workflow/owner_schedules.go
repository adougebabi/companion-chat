package workflow

import (
	"context"
	"fmt"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
	"time"
)

type ownerScheduleApplication interface {
	ProcessOwnerScheduleRequest(context.Context, string) (map[string]any, error)
}

func OwnerScheduleWorkflow(ctx workflow.Context, input Input) (map[string]any, error) {
	if input.SessionID == "" {
		return nil, fmt.Errorf("Owner schedule request required")
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
	err = workflow.ExecuteActivity(ctx, ProcessOwnerScheduleActivity, input).Get(ctx, &result)
	return result, err
}
func ProcessOwnerScheduleActivity(ctx context.Context, input Input) (map[string]any, error) {
	application, ok := app().(ownerScheduleApplication)
	if !ok {
		return nil, fmt.Errorf("Owner schedule worker unavailable")
	}
	ctx, input = prepareActivityLifecycleContext(ctx, input, "owner_schedule")
	stop := startActivityHeartbeat(ctx, map[string]any{"request_id": input.SessionID, "phase": "schedule_planning"})
	defer stop()
	return application.ProcessOwnerScheduleRequest(ctx, input.SessionID)
}
