package workflow

import (
	"context"

	"github.com/jackc/pgx/v5/pgconn"
	enumspb "go.temporal.io/api/enums/v1"
	workflowservice "go.temporal.io/api/workflowservice/v1"
)

const repairFailedScheduleLedgerSQL = `UPDATE public.platform_workflow_intents SET status='started',started_at=COALESCE(started_at,now()),completed_at=NULL,next_attempt_at=NULL,last_error=NULL WHERE intent_id=$1 AND intent_type='schedule.current_day' AND status='failed'`

// This is observation repair, not another attempt. The previous failure remains
// in lifecycle history; a long-lived daily scheduler waits until local midnight.
func reconcileFailedScheduleLedger(ctx context.Context, intentID string, execution *workflowservice.DescribeWorkflowExecutionResponse, describeErr error, exec func(context.Context, string, ...any) (pgconn.CommandTag, error)) (bool, error) {
	if describeErr != nil || execution == nil || execution.WorkflowExecutionInfo == nil {
		return false, nil
	}
	info := execution.WorkflowExecutionInfo
	if info.GetStatus() != enumspb.WORKFLOW_EXECUTION_STATUS_RUNNING || info.GetExecution().GetRunId() == "" {
		return false, nil
	}
	command, err := exec(ctx, repairFailedScheduleLedgerSQL, intentID)
	return err == nil && command.RowsAffected() == 1, err
}
