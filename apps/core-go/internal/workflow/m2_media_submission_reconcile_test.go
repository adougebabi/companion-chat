package workflow

import (
	"errors"
	"testing"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
	"github.com/stretchr/testify/mock"
	commonpb "go.temporal.io/api/common/v1"
	enumspb "go.temporal.io/api/enums/v1"
	workflowpb "go.temporal.io/api/workflow/v1"
	workflowservice "go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/mocks"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
	temporalworkflow "go.temporal.io/sdk/workflow"
)

func TestAmbiguousMediaActivityIsNonRetryable(t *testing.T) {
	err := mediaActivityFailure(&core.CapabilityError{Code: "media_submission_ambiguous", Retryable: false})
	var failure *temporal.ApplicationError
	if !errors.As(err, &failure) || !failure.NonRetryable() || failure.Type() != "media_submission_ambiguous" {
		t.Fatalf("ambiguous submission was not terminal for the activity: %v", err)
	}
}

func TestVisualWorkflowWaitsForResumeAfterAmbiguousSubmission(t *testing.T) {
	for _, stage := range []string{"image_submission_unknown", "character_sheet_submission_unknown"} {
		t.Run(stage, func(t *testing.T) {
			var suite testsuite.WorkflowTestSuite
			env := suite.NewTestWorkflowEnvironment()
			input := Input{SessionID: "visual-uncertain"}
			env.OnActivity(ProcessVisualIdentityActivity, mock.Anything, input).Return(map[string]any{"status": "waiting", "stage": stage}, nil).Once()
			observed := ""
			env.RegisterDelayedCallback(func() {
				value, err := env.QueryWorkflow("status")
				if err != nil || value.Get(&observed) != nil {
					t.Errorf("query paused status: %v", err)
				}
				env.SignalWorkflow("resume", nil)
			}, time.Minute)
			env.ExecuteWorkflow(VisualIdentityWorkflow, input)
			var continued *temporalworkflow.ContinueAsNewError
			if observed != "paused" || !errors.As(env.GetWorkflowError(), &continued) {
				t.Fatalf("visual workflow did not wait for explicit resume: status=%s err=%v", observed, env.GetWorkflowError())
			}
		})
	}
}

func TestVisualWorkflowLegacyHistoryCanContinueAsNew(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	input := Input{SessionID: "legacy-visual-uncertain"}
	env.OnGetVersion("visual-media-submission-unknown-pause", temporalworkflow.DefaultVersion, 1).Return(temporalworkflow.DefaultVersion)
	env.OnActivity(ProcessVisualIdentityActivity, mock.Anything, input).Return(map[string]any{"status": "waiting", "stage": "image_submission_unknown"}, nil).Once()
	env.ExecuteWorkflow(VisualIdentityWorkflow, input)
	var continued *temporalworkflow.ContinueAsNewError
	if !errors.As(env.GetWorkflowError(), &continued) {
		t.Fatalf("legacy workflow history did not retain its continuation: %v", env.GetWorkflowError())
	}
}

// Temporal visibility is controlled; PostgreSQL and the Dispatcher path are real.
func TestMediaReconcilePreservesUnknownSubmissionAfterWorkerLoss(t *testing.T) {
	ctx, app := scheduleRecoveryRepository(t)
	if _, err := app.DB.Pool().Exec(ctx, `DELETE FROM public.platform_workflow_intents WHERE intent_id='schedule_intent:retry-role'`); err != nil {
		t.Fatal(err)
	}
	const mediaID, intentID, workflowID = "m2-reconcile-media", "m2-reconcile-intent", "go:m2-reconcile-workflow"
	if _, err := app.DB.Pool().Exec(ctx, `INSERT INTO public.media_intents(id,owner_fluctlight_id,kind,mime_type,prompt,provider_request_id,workflow_id,status,renderer_submission_started) VALUES($1,'retry-role','image','image/png','{}','m2-reconcile-request',$2,'submitting',true)`, mediaID, workflowID); err != nil {
		t.Fatal(err)
	}
	if _, err := app.DB.Pool().Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload,status,attempt_count) VALUES($1,$2,'media','media.generation','{"intent_id":"m2-reconcile-media"}','started',1)`, intentID, workflowID); err != nil {
		t.Fatal(err)
	}
	temporalClient := &mocks.Client{}
	iterator := &mocks.HistoryEventIterator{}
	iterator.On("HasNext").Return(false)
	temporalClient.On("GetWorkflowHistory", mock.Anything, workflowID, "lost-worker", false, enumspb.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT).Return(iterator)
	temporalClient.On("DescribeWorkflowExecution", mock.Anything, workflowID, "").Return(&workflowservice.DescribeWorkflowExecutionResponse{WorkflowExecutionInfo: &workflowpb.WorkflowExecutionInfo{Status: enumspb.WORKFLOW_EXECUTION_STATUS_FAILED, Execution: &commonpb.WorkflowExecution{RunId: "lost-worker"}}}, nil)
	d := &Dispatcher{App: app, Client: temporalClient}
	if _, err := d.ReconcileOnce(ctx, 100); err != nil {
		t.Fatal(err)
	}
	var mediaStatus, workflowStatus string
	if err := app.DB.Pool().QueryRow(ctx, `SELECT m.status,w.status FROM public.media_intents m JOIN public.platform_workflow_intents w ON w.workflow_id=m.workflow_id WHERE m.id=$1`, mediaID).Scan(&mediaStatus, &workflowStatus); err != nil || mediaStatus != "submission_unknown" || workflowStatus != "failed" {
		t.Fatalf("media=%s workflow=%s err=%v", mediaStatus, workflowStatus, err)
	}
}
