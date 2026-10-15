package workflow

import (
	"context"
	"errors"
	"fmt"
	"time"

	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

const ReplyContinuationQueue = "reply-continuation-v1"

type replyContinuationService interface {
	ProcessReplyContinuation(context.Context, string) (map[string]any, error)
	FailReplyContinuation(context.Context, string, string) error
}

func ReplyContinuationWorkflow(ctx workflow.Context, draft string) (map[string]any, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: 15 * time.Minute, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 3}})
	var result map[string]any
	err := workflow.ExecuteActivity(ctx, ProcessReplyContinuationActivity, draft).Get(ctx, &result)
	return result, err
}
func ProcessReplyContinuationActivity(ctx context.Context, draft string) (map[string]any, error) {
	service, ok := app().(replyContinuationService)
	if !ok {
		return nil, errors.New("reply_continuation_service_unavailable")
	}
	return service.ProcessReplyContinuation(ctx, draft)
}

// Independent table and queue are both required: old platform dispatchers scan
// every platform intent regardless of task queue. They cannot read these rows.
func (d *Dispatcher) DispatchReplyContinuations(ctx context.Context, limit int) (int, error) {
	if limit < 1 {
		limit = 1
	}
	if limit > 50 {
		limit = 50
	}
	// Reconcile terminal sources/expiry even if no activity callback ran.
	rows, err := d.App.DB.Pool().Query(ctx, `SELECT d.id FROM public.conversation_reply_drafts d JOIN public.conversation_reply_continuations c ON c.draft_id=d.id LEFT JOIN public.cognition_inbox i ON i.id=d.source_fact_id WHERE c.status IN ('pending','started') AND d.status<>'published' AND (d.expires_at<=now() OR i.status='failed') LIMIT $1`, limit)
	if err != nil {
		return 0, err
	}
	expired := []string{}
	for rows.Next() {
		var ref string
		if err := rows.Scan(&ref); err != nil {
			rows.Close()
			return 0, err
		}
		expired = append(expired, ref)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	for _, ref := range expired {
		if err := d.App.FailReplyContinuation(ctx, ref, "reply_source_closed_or_expired"); err != nil {
			return 0, err
		}
	}
	rows, err = d.App.DB.Pool().Query(ctx, `SELECT c.draft_id,c.workflow_id,c.status,c.attempt_count FROM public.conversation_reply_continuations c JOIN public.conversation_reply_drafts d ON d.id=c.draft_id WHERE c.status IN ('pending','started') AND c.next_attempt_at<=now() AND NOT EXISTS(SELECT 1 FROM public.conversation_reply_actions r LEFT JOIN public.media_intents m ON m.id=r.media_intent_id WHERE r.draft_id=d.id AND (r.status='requested' OR (r.status='bound' AND COALESCE(m.status,'pending') NOT IN ('completed','failed','cancelled')))) ORDER BY c.next_attempt_at LIMIT $1`, limit)
	if err != nil {
		return 0, err
	}
	type item struct {
		draft, workflow, status string
		attempts                int
	}
	items := []item{}
	for rows.Next() {
		var i item
		if err := rows.Scan(&i.draft, &i.workflow, &i.status, &i.attempts); err != nil {
			rows.Close()
			return 0, err
		}
		items = append(items, i)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	count := 0
	for _, i := range items {
		if i.status == "started" {
			execution, err := d.Client.DescribeWorkflowExecution(ctx, i.workflow, "")
			if err != nil {
				var missing *serviceerror.NotFound
				if errors.As(err, &missing) {
					_, startErr := d.Client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{ID: i.workflow, TaskQueue: ReplyContinuationQueue, WorkflowExecutionTimeout: time.Hour, WorkflowIDReusePolicy: enumspb.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE_FAILED_ONLY}, ReplyContinuationWorkflow, i.draft)
					var duplicate *serviceerror.WorkflowExecutionAlreadyStarted
					if startErr != nil && !errors.As(startErr, &duplicate) {
						return count, startErr
					}
					count++
					continue
				}
				return count, err
			}
			if execution.WorkflowExecutionInfo.Status == enumspb.WORKFLOW_EXECUTION_STATUS_RUNNING {
				continue
			}
			// A lost completion update must settle using the persisted message.
			var published bool
			if err := d.App.DB.Pool().QueryRow(ctx, `SELECT status='published' FROM public.conversation_reply_drafts WHERE id=$1`, i.draft).Scan(&published); err != nil {
				return count, err
			}
			if published {
				if err := d.App.CompleteReplyContinuation(ctx, i.draft); err != nil {
					return count, err
				}
				continue
			}
			if i.attempts >= 3 {
				if err := d.App.FailReplyContinuation(ctx, i.draft, "reply_continuation_attempts_exhausted"); err != nil {
					return count, err
				}
				continue
			}
			_, err = d.App.DB.Pool().Exec(ctx, `UPDATE public.conversation_reply_continuations SET status='pending',next_attempt_at=now()+interval '10 seconds',last_error='reply_continuation_retry' WHERE draft_id=$1 AND status='started'`, i.draft)
			if err != nil {
				return count, err
			}
			continue
		}
		if i.attempts >= 3 {
			if err := d.App.FailReplyContinuation(ctx, i.draft, "reply_continuation_attempts_exhausted"); err != nil {
				return count, err
			}
			continue
		}
		// The claim precedes start; a process crash between the two is recovered by
		// retrying the same stable ID below, with no new side-effect identity.
		tag, err := d.App.DB.Pool().Exec(ctx, `UPDATE public.conversation_reply_continuations SET status='started',started_at=now(),attempt_count=attempt_count+1 WHERE draft_id=$1 AND status='pending'`, i.draft)
		if err != nil {
			return count, err
		}
		if tag.RowsAffected() != 1 {
			continue
		}
		_, err = d.Client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{ID: i.workflow, TaskQueue: ReplyContinuationQueue, WorkflowExecutionTimeout: time.Hour, WorkflowIDReusePolicy: enumspb.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE_FAILED_ONLY}, ReplyContinuationWorkflow, i.draft)
		var duplicate *serviceerror.WorkflowExecutionAlreadyStarted
		if err != nil && !errors.As(err, &duplicate) {
			_, _ = d.App.DB.Pool().Exec(ctx, `UPDATE public.conversation_reply_continuations SET status='pending',next_attempt_at=now()+interval '10 seconds',last_error='reply_workflow_start_failed' WHERE draft_id=$1`, i.draft)
			return count, fmt.Errorf("start reply continuation: %w", err)
		}
		count++
	}
	return count, nil
}
