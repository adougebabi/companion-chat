package workflow

import (
	"context"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
	workflowservice "go.temporal.io/api/workflowservice/v1"
)

const worldEventActivityTimeout = 15 * time.Minute

// The clock normally remains RUNNING for its entire lifetime. Only an overdue
// generation Activity proves a stall; an old ledger started_at or attempt=1
// does not. In particular, never reclaim its hourly timer or a paused Activity.
func worldEventActivityStale(execution *workflowservice.DescribeWorkflowExecutionResponse, now time.Time) bool {
	for _, pending := range execution.GetPendingActivities() {
		if pending.GetActivityType().GetName() != "GenerateWorldEventActivity" || pending.GetPaused() || pending.GetScheduledTime() == nil {
			continue
		}
		if !now.Before(pending.GetScheduledTime().AsTime().Add(worldEventActivityTimeout)) {
			return true
		}
	}
	return false
}

func (d *Dispatcher) reclaimWorldEventActivity(ctx context.Context, input Input, workflowID, runID string, attempts int, payload []byte) (bool, error) {
	if runID == "" {
		return false, nil
	}
	var current bool
	if err := d.App.DB.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.platform_workflow_intents WHERE intent_id=$1 AND intent_type='world.events.generate' AND status='started' AND attempt_count=$2 AND payload=$3::jsonb)`, input.IntentID, attempts, payload).Scan(&current); err != nil || !current {
		return false, err
	}
	// Terminate the observed Run, never whichever Run happens to be current.
	// Do not requeue when the runtime cannot confirm termination.
	if err := d.Client.TerminateWorkflow(ctx, normalizedWorkflowID(workflowID), runID, "world_event_generation_timeout"); err != nil {
		return false, err
	}
	reclaimed, err := d.App.ReclaimWorldEventGenerationAttempt(ctx, input.IntentID, attempts, payload)
	if err != nil || !reclaimed {
		return reclaimed, err
	}
	delete(d.Started, input.IntentID)
	d.recordIntentLifecycle(ctx, input, "world.events.generate", workflowID, runID, core.LifecycleTransitionRetryScheduled, "workflow_reconcile", "retry", "world_event_generation_timeout", attempts, nil)
	return true, nil
}
