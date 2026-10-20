package workflow

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	commonpb "go.temporal.io/api/common/v1"
	enumspb "go.temporal.io/api/enums/v1"
	workflowpb "go.temporal.io/api/workflow/v1"
	workflowservice "go.temporal.io/api/workflowservice/v1"
)

func TestFailedScheduleLedgerRepairsOnlyObservedRunning(t *testing.T) {
	running := &workflowservice.DescribeWorkflowExecutionResponse{WorkflowExecutionInfo: &workflowpb.WorkflowExecutionInfo{Status: enumspb.WORKFLOW_EXECUTION_STATUS_RUNNING, Execution: &commonpb.WorkflowExecution{RunId: "current-run"}}}
	for _, status := range []enumspb.WorkflowExecutionStatus{enumspb.WORKFLOW_EXECUTION_STATUS_RUNNING, enumspb.WORKFLOW_EXECUTION_STATUS_COMPLETED, enumspb.WORKFLOW_EXECUTION_STATUS_FAILED, enumspb.WORKFLOW_EXECUTION_STATUS_CANCELED, enumspb.WORKFLOW_EXECUTION_STATUS_TERMINATED, enumspb.WORKFLOW_EXECUTION_STATUS_TIMED_OUT, enumspb.WORKFLOW_EXECUTION_STATUS_CONTINUED_AS_NEW} {
		t.Run(status.String(), func(t *testing.T) {
			observed := &workflowservice.DescribeWorkflowExecutionResponse{WorkflowExecutionInfo: &workflowpb.WorkflowExecutionInfo{Status: status, Execution: &commonpb.WorkflowExecution{RunId: "observed-run"}}}
			calls := 0
			exec := func(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
				calls++
				if sql != repairFailedScheduleLedgerSQL || len(args) != 1 || args[0] != "schedule-intent" {
					t.Fatal("repair lost stable identity")
				}
				return pgconn.NewCommandTag("UPDATE 1"), nil
			}
			repaired, err := reconcileFailedScheduleLedger(context.Background(), "schedule-intent", observed, nil, exec)
			want := status == enumspb.WORKFLOW_EXECUTION_STATUS_RUNNING
			if err != nil || repaired != want || (calls == 1) != want {
				t.Fatalf("repaired=%v writes=%d err=%v", repaired, calls, err)
			}
		})
	}
	noWrite := func(context.Context, string, ...any) (pgconn.CommandTag, error) {
		t.Fatal("unknown execution mutated ledger")
		return pgconn.CommandTag{}, nil
	}
	for _, value := range []*workflowservice.DescribeWorkflowExecutionResponse{nil, {}, {WorkflowExecutionInfo: &workflowpb.WorkflowExecutionInfo{Status: enumspb.WORKFLOW_EXECUTION_STATUS_RUNNING}}} {
		if repaired, err := reconcileFailedScheduleLedger(context.Background(), "schedule-intent", value, nil, noWrite); err != nil || repaired {
			t.Fatal("missing execution repaired", repaired, err)
		}
	}
	if repaired, err := reconcileFailedScheduleLedger(context.Background(), "schedule-intent", running, errors.New("describe unavailable/not found"), noWrite); err != nil || repaired {
		t.Fatal("failed observation repaired", repaired, err)
	}
	// Controlled command boundary: a CAS loser or an already repaired ledger emits no repair.
	for _, tag := range []string{"UPDATE 0", "UPDATE 1"} {
		repaired, err := reconcileFailedScheduleLedger(context.Background(), "schedule-intent", running, nil, func(context.Context, string, ...any) (pgconn.CommandTag, error) {
			return pgconn.NewCommandTag(tag), nil
		})
		if err != nil || repaired != (tag == "UPDATE 1") {
			t.Fatal(tag, repaired, err)
		}
	}
	dbErr := errors.New("write failed")
	if repaired, err := reconcileFailedScheduleLedger(context.Background(), "schedule-intent", running, nil, func(context.Context, string, ...any) (pgconn.CommandTag, error) { return pgconn.CommandTag{}, dbErr }); repaired || !errors.Is(err, dbErr) {
		t.Fatal("DB failure hidden", repaired, err)
	}
}

func TestScheduleLedgerObservationPreservesAttemptsAndClearsStaleTerminalFields(t *testing.T) {
	if !strings.Contains(reconcileIntentQuery, "'schedule.current_day'") {
		t.Fatal("failed schedule not selected")
	}
	for _, part := range []string{"intent_type='schedule.current_day'", "status='failed'", "completed_at=NULL", "next_attempt_at=NULL", "last_error=NULL"} {
		if !strings.Contains(repairFailedScheduleLedgerSQL, part) {
			t.Fatal("missing repair boundary", part)
		}
	}
	if strings.Contains(repairFailedScheduleLedgerSQL, "attempt_count") {
		t.Fatal("observation counted as execution")
	}
}
