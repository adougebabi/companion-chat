package workflow

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	commonpb "go.temporal.io/api/common/v1"
	enumspb "go.temporal.io/api/enums/v1"
	workflowpb "go.temporal.io/api/workflow/v1"
	workflowservice "go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/mocks"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestWorldEventActivityStaleRequiresOverdueGeneration(t *testing.T) {
	now := time.Now().UTC()
	for _, test := range []struct {
		name     string
		age      time.Duration
		paused   bool
		activity string
		want     bool
	}{
		{"fresh", 14 * time.Minute, false, "GenerateWorldEventActivity", false},
		{"deadline", 15 * time.Minute, false, "GenerateWorldEventActivity", true},
		{"abandoned", 2 * time.Hour, false, "GenerateWorldEventActivity", true},
		{"paused", 2 * time.Hour, true, "GenerateWorldEventActivity", false},
		{"other activity", 2 * time.Hour, false, "AnotherActivity", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := &workflowservice.DescribeWorkflowExecutionResponse{PendingActivities: []*workflowpb.PendingActivityInfo{{ActivityType: &commonpb.ActivityType{Name: test.activity}, ScheduledTime: timestamppb.New(now.Add(-test.age)), Paused: test.paused}}}
			if got := worldEventActivityStale(response, now); got != test.want {
				t.Fatal(got, test.want)
			}
		})
	}
	if worldEventActivityStale(nil, now) || worldEventActivityStale(&workflowservice.DescribeWorkflowExecutionResponse{}, now) {
		t.Fatal("idle hourly clock was reclaimed")
	}
}

func TestPostgresWorldEventStaleActivityReclaimsAndDispatchesNextAttempt(t *testing.T) {
	ctx, app := scheduleRecoveryRepository(t)
	const intent, workflowID = "world_event_generation:reclaim-world", "go:world_event_generation:reclaim-world"
	if _, err := app.DB.Pool().Exec(ctx, `DELETE FROM public.platform_workflow_intents WHERE intent_id='schedule_intent:retry-role'`); err != nil {
		t.Fatal(err)
	}
	if _, err := app.DB.Pool().Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload,status,attempt_count,started_at) VALUES($1,$2,'world-events','world.events.generate','{"world_id":"reclaim-world"}','started',1,now()-interval '3 hours')`, intent, workflowID); err != nil {
		t.Fatal(err)
	}
	stale := &workflowservice.DescribeWorkflowExecutionResponse{
		WorkflowExecutionInfo: &workflowpb.WorkflowExecutionInfo{Status: enumspb.WORKFLOW_EXECUTION_STATUS_RUNNING, Execution: &commonpb.WorkflowExecution{RunId: "stale-run"}},
		PendingActivities:     []*workflowpb.PendingActivityInfo{{ActivityType: &commonpb.ActivityType{Name: "GenerateWorldEventActivity"}, ScheduledTime: timestamppb.New(time.Now().Add(-16 * time.Minute)), Attempt: 1}},
	}
	temporalClient := &mocks.Client{}
	var observeErr, terminateErr error
	temporalClient.On("DescribeWorkflowExecution", mock.Anything, workflowID, "").Return(func(context.Context, string, string) (*workflowservice.DescribeWorkflowExecutionResponse, error) {
		return stale, observeErr
	})
	temporalClient.On("TerminateWorkflow", mock.Anything, workflowID, "stale-run", "world_event_generation_timeout").Return(func(context.Context, string, string, string, ...interface{}) error { return terminateErr })
	d := &Dispatcher{App: app, Client: temporalClient, Started: map[string]struct{}{intent: {}}}
	check := func(want string, attempts int) {
		t.Helper()
		var status string
		var count int
		if err := app.DB.Pool().QueryRow(ctx, `SELECT status,attempt_count FROM public.platform_workflow_intents WHERE intent_id=$1`, intent).Scan(&status, &count); err != nil || status != want || count != attempts {
			t.Fatal(status, count, err)
		}
	}
	// The ledger can be hours old while the clock is sleeping normally.
	activity := stale.PendingActivities
	stale.PendingActivities = nil
	if count, err := d.ReconcileOnce(ctx, 100); err != nil || count != 0 {
		t.Fatal("healthy hourly timer reclaimed", count, err)
	}
	check("started", 1)
	temporalClient.AssertNotCalled(t, "TerminateWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	stale.PendingActivities = activity
	// Runtime observation and termination failures must not release a live Run.
	observeErr = errors.New("runtime unavailable")
	if count, err := d.ReconcileOnce(ctx, 100); err != nil || count != 0 {
		t.Fatal(count, err)
	}
	check("started", 1)
	temporalClient.AssertNotCalled(t, "TerminateWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	observeErr = nil
	terminateErr = errors.New("termination unavailable")
	if count, err := d.ReconcileOnce(ctx, 100); err == nil || count != 0 {
		t.Fatal("failed termination released attempt", count, err)
	}
	check("started", 1)
	terminateErr = nil
	if count, err := d.ReconcileOnce(ctx, 100); err != nil || count != 1 {
		t.Fatal(count, err)
	}
	check("retry", 1)
	if _, exists := d.Started[intent]; exists {
		t.Fatal("process dispatch cache retained reclaimed attempt")
	}
	if count, err := d.DispatchOnce(ctx, 100); err != nil || count != 0 {
		t.Fatal("retry bypassed durable backoff", count, err)
	}
	if _, err := app.DB.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET next_attempt_at=now() WHERE intent_id=$1`, intent); err != nil {
		t.Fatal(err)
	}
	run := &mocks.WorkflowRun{}
	run.On("GetRunID").Return("replacement-run")
	temporalClient.On("ExecuteWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(func(_ context.Context, options client.StartWorkflowOptions, _ interface{}, args ...interface{}) (client.WorkflowRun, error) {
		if options.ID != workflowID || options.WorkflowIDReusePolicy != enumspb.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE_FAILED_ONLY || args[0].(Input).IntentID != intent {
			t.Fatal("reclaim changed durable workflow identity", options, args)
		}
		return run, nil
	}).Once()
	if count, err := d.DispatchOnce(ctx, 100); err != nil || count != 1 {
		t.Fatal(count, err)
	}
	check("started", 2)
	temporalClient.AssertExpectations(t)
}
