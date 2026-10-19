package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type ownerPersonaSwitchApplication interface {
	ProcessOwnerPersonaSwitchIntent(context.Context, string) (map[string]any, error)
}

func OwnerPersonaSwitchWorkflow(ctx workflow.Context, input Input) (map[string]any, error) {
	if input.IntentID == "" || input.FluctlightID == "" {
		return nil, fmt.Errorf("owner persona switch input is invalid")
	}
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: 10 * time.Minute, HeartbeatTimeout: time.Minute, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 3}})
	control, err := registerWorkflowControl(ctx)
	if err != nil {
		return nil, err
	}
	if err = control.waitUntilResumed(ctx); err != nil {
		return nil, err
	}
	var result map[string]any
	err = workflow.ExecuteActivity(ctx, ProcessOwnerPersonaSwitchActivity, input).Get(ctx, &result)
	return result, err
}

func ProcessOwnerPersonaSwitchActivity(ctx context.Context, input Input) (map[string]any, error) {
	application, ok := app().(ownerPersonaSwitchApplication)
	if !ok {
		return nil, fmt.Errorf("Owner persona Worker is not configured")
	}
	ctx, input = prepareActivityLifecycleContext(ctx, input, "persona_owner_switch")
	recordActivityLifecycle(app(), ctx, input, "persona_owner_switch", core.LifecycleTransitionActivityStarted, "running", "activity_started", nil)
	stop := startOwnerPersonaHeartbeat(ctx, input.IntentID)
	defer stop()
	result, err := application.ProcessOwnerPersonaSwitchIntent(ctx, input.IntentID)
	if err != nil {
		recordActivityLifecycle(app(), ctx, input, "persona_owner_switch", core.LifecycleTransitionFailed, "failed", "persona_switch_assessment_retrying", err)
	} else {
		transition, status, reason := actionLifecycleOutcome(result)
		recordActivityLifecycle(app(), ctx, input, "persona_owner_switch", transition, status, reason, nil)
	}
	return result, err
}

// This timer only reports Activity liveness/cancellation to Temporal; the
// committed PostgreSQL intent remains the sole scheduling authority.
func startOwnerPersonaHeartbeat(ctx context.Context, id string) func() {
	return startActivityHeartbeat(ctx, map[string]any{"request_id": id, "phase": "assessment"})
}
