package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

type personaPreparationApplication interface {
	ProcessPersonaPreparationIntent(context.Context, string) (map[string]any, error)
}

func PersonaPreparationWorkflow(ctx workflow.Context, input Input) (map[string]any, error) {
	if input.IntentID == "" || input.FluctlightID == "" {
		return nil, fmt.Errorf("persona preparation input is invalid")
	}
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: 10 * time.Minute, HeartbeatTimeout: time.Minute, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 3}})
	control, err := registerWorkflowControl(ctx)
	if err != nil {
		return nil, err
	}
	if err := control.waitUntilResumed(ctx); err != nil {
		return nil, err
	}
	var result map[string]any
	err = workflow.ExecuteActivity(ctx, ProcessPersonaPreparationActivity, input).Get(ctx, &result)
	return result, err
}

func ProcessPersonaPreparationActivity(ctx context.Context, input Input) (map[string]any, error) {
	application, ok := app().(personaPreparationApplication)
	if !ok {
		return nil, fmt.Errorf("persona preparation Worker is not configured")
	}
	ctx, input = prepareActivityLifecycleContext(ctx, input, "persona_compilation")
	recordActivityLifecycle(app(), ctx, input, "persona_compilation", core.LifecycleTransitionActivityStarted, "running", "activity_started", nil)
	stop := startActivityHeartbeat(ctx, map[string]any{"intent_id": input.IntentID, "phase": "persona_compilation"})
	defer stop()
	result, err := application.ProcessPersonaPreparationIntent(ctx, input.IntentID)
	if err != nil {
		recordActivityLifecycle(app(), ctx, input, "persona_compilation", core.LifecycleTransitionFailed, "failed", "persona_compilation_retrying", err)
	} else {
		transition, status, reason := actionLifecycleOutcome(result)
		recordActivityLifecycle(app(), ctx, input, "persona_compilation", transition, status, reason, nil)
	}
	return result, err
}
