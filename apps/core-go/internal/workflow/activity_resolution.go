package workflow

import (
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
	"time"
)

// Independent started activities own their durable timer, even while the first
// daily schedule is unavailable. Domain receipts alone determine the outcome.
func ActivityResolutionWorkflow(ctx workflow.Context, input Input) (map[string]any, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: 5 * time.Minute, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 3}})
	control, err := registerWorkflowControl(ctx)
	if err != nil {
		return nil, err
	}
	if err := control.waitUntilResumed(ctx); err != nil {
		return nil, err
	}
	due, err := time.Parse(time.RFC3339Nano, input.DueAt)
	if err != nil {
		return nil, err
	}
	if delay := due.Sub(workflow.Now(ctx)); delay > 0 {
		if err := workflow.Sleep(ctx, delay); err != nil {
			return nil, err
		}
	}
	if err := control.waitUntilResumed(ctx); err != nil {
		return nil, err
	}
	var result map[string]any
	if err := workflow.ExecuteActivity(ctx, ResolveScheduledLifeActivityActivity, input).Get(ctx, &result); err != nil {
		return nil, err
	}
	switch stringValue(result["status"]) {
	case "pending", "in_progress", "deferred":
		next, err := time.Parse(time.RFC3339Nano, stringValue(result["not_before"]))
		if err != nil || !next.After(workflow.Now(ctx)) {
			next = workflow.Now(ctx).Add(5 * time.Minute)
		}
		input.DueAt = next.UTC().Format(time.RFC3339Nano)
		return nil, workflow.NewContinueAsNewError(ctx, ActivityResolutionWorkflow, input)
	}
	return result, nil
}
