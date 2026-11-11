package workflow

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	commonpb "go.temporal.io/api/common/v1"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/api/serviceerror"
	workflowpb "go.temporal.io/api/workflow/v1"
	workflowservice "go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/mocks"
)

type dailyGenerationActivityService struct {
	ApplicationService
	intentID   string
	generation int64
}

func (s *dailyGenerationActivityService) ProcessConversationDailyMemoryIntentAt(_ context.Context, intentID string, generation int64) (map[string]any, error) {
	s.intentID, s.generation = intentID, generation
	return map[string]any{"status": "completed"}, nil
}

func TestDailyMemoryActivityForwardsWorkflowGeneration(t *testing.T) {
	previous := app()
	service := &dailyGenerationActivityService{}
	Configure(service)
	t.Cleanup(func() { Configure(previous) })
	for _, generation := range []int64{0, 3} {
		if _, err := ProcessConversationDailyMemoryActivity(context.Background(), Input{IntentID: "daily-activity", SourceGeneration: generation}); err != nil || service.intentID != "daily-activity" || service.generation != generation {
			t.Fatalf("activity did not forward the captured generation %d: %v", generation, err)
		}
	}
}

// A stable workflow ID may still name an older run after a new source set has
// reopened its intent. The durable generation must survive every ledger race.
func TestPostgresDailyMemoryReopenSurvivesOlderTemporalRun(t *testing.T) {
	ctx, app := scheduleRecoveryRepository(t)
	const intent, workflowID = "daily-generation-test", "go:daily-generation-test"
	if _, err := app.DB.Pool().Exec(ctx, `DELETE FROM public.platform_workflow_intents WHERE intent_id='schedule_intent:retry-role'`); err != nil {
		t.Fatal(err)
	}
	if _, err := app.DB.Pool().Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload,status,attempt_count,next_attempt_at)
		VALUES($1,$2,'lifecycle','conversation.daily_memory','{"intent_id":"daily-generation-test","source_generation":1}','retry',1,now())`, intent, workflowID); err != nil {
		t.Fatal(err)
	}
	check := func(wantStatus string, wantGeneration, wantAttempts int) {
		t.Helper()
		var status string
		var generation, attempts int
		if err := app.DB.Pool().QueryRow(ctx, `SELECT status,(payload->>'source_generation')::integer,attempt_count FROM public.platform_workflow_intents WHERE intent_id=$1`, intent).Scan(&status, &generation, &attempts); err != nil || status != wantStatus || generation != wantGeneration || attempts != wantAttempts {
			t.Fatalf("status=%s generation=%d attempts=%d want=%s/%d/%d err=%v", status, generation, attempts, wantStatus, wantGeneration, wantAttempts, err)
		}
	}
	due := func() {
		t.Helper()
		if _, err := app.DB.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET next_attempt_at=now() WHERE intent_id=$1`, intent); err != nil {
			t.Fatal(err)
		}
	}
	temporalClient := &mocks.Client{}
	oldRunning, changeDuringStart, changeDuringDescribe := true, false, false
	run := &mocks.WorkflowRun{}
	run.On("GetRunID").Return("new-run")
	temporalClient.On("ExecuteWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(func(_ context.Context, options client.StartWorkflowOptions, _ interface{}, args ...interface{}) (client.WorkflowRun, error) {
		var generation int64
		if err := app.DB.Pool().QueryRow(ctx, `SELECT (payload->>'source_generation')::bigint FROM public.platform_workflow_intents WHERE intent_id=$1`, intent).Scan(&generation); err != nil {
			t.Fatal(err)
		}
		if options.ID != workflowID || args[0].(Input).IntentID != intent || args[0].(Input).SourceGeneration != generation {
			t.Fatalf("unexpected workflow start: %#v %#v", options, args)
		}
		if oldRunning {
			return nil, serviceerror.NewWorkflowExecutionAlreadyStarted("old run still finishing", "request", "old-run")
		}
		if changeDuringStart {
			changeDuringStart = false
			if _, err := app.DB.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET status='retry',payload=jsonb_set(payload,'{source_generation}','3'),next_attempt_at=now() WHERE intent_id=$1`, intent); err != nil {
				t.Fatal(err)
			}
		}
		return run, nil
	})
	temporalClient.On("DescribeWorkflowExecution", mock.Anything, workflowID, "").Return(func(context.Context, string, string) (*workflowservice.DescribeWorkflowExecutionResponse, error) {
		if changeDuringDescribe {
			changeDuringDescribe = false
			if _, err := app.DB.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET status='retry',payload=jsonb_set(payload,'{source_generation}','2'),next_attempt_at=now() WHERE intent_id=$1`, intent); err != nil {
				t.Fatal(err)
			}
		}
		return &workflowservice.DescribeWorkflowExecutionResponse{WorkflowExecutionInfo: &workflowpb.WorkflowExecutionInfo{Status: enumspb.WORKFLOW_EXECUTION_STATUS_COMPLETED, Execution: &commonpb.WorkflowExecution{RunId: "old-run"}}}, nil
	})
	dispatcher := &Dispatcher{App: app, Client: temporalClient}
	if _, err := dispatcher.ReconcileOnce(ctx, 100); err != nil {
		t.Fatal(err)
	}
	check("retry", 1, 1)
	if _, err := dispatcher.DispatchOnce(ctx, 100); err != nil {
		t.Fatal(err)
	}
	check("retry", 1, 1)
	oldRunning = false
	due()
	if _, err := dispatcher.DispatchOnce(ctx, 100); err != nil {
		t.Fatal(err)
	}
	check("started", 1, 2)
	changeDuringDescribe = true
	if _, err := dispatcher.ReconcileOnce(ctx, 100); err != nil {
		t.Fatal(err)
	}
	check("retry", 2, 2)
	changeDuringStart = true
	due()
	if _, err := dispatcher.DispatchOnce(ctx, 100); err != nil {
		t.Fatal(err)
	}
	check("retry", 3, 2)
	due()
	if _, err := dispatcher.DispatchOnce(ctx, 100); err != nil {
		t.Fatal(err)
	}
	check("started", 3, 3)
}
