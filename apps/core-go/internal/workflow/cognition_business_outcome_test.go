package workflow

import (
	"errors"
	"fmt"
	"testing"

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

func TestCognitionBusinessFailureIsNotWorkflowSuccess(t *testing.T) {
	for _, version := range []temporalworkflow.Version{temporalworkflow.DefaultVersion, 1} {
		t.Run(fmt.Sprint(version), func(t *testing.T) {
			var suite testsuite.WorkflowTestSuite
			env := suite.NewTestWorkflowEnvironment()
			input := Input{InboxID: "failed-inbox"}
			env.OnGetVersion("cognition-business-outcome-v1", temporalworkflow.DefaultVersion, 1).Return(version)
			env.OnActivity(ProcessCognitionActivity, mock.Anything, input).Return(map[string]any{"status": "failed", "error_code": "direct_media_delivery_unresolved"}, nil).Once()
			env.ExecuteWorkflow(CognitionProcessingWorkflow, input)
			if version == temporalworkflow.DefaultVersion {
				if err := env.GetWorkflowError(); err != nil {
					t.Fatal("historical replay changed", err)
				}
			} else {
				var failure *temporal.ApplicationError
				if !errors.As(env.GetWorkflowError(), &failure) || !failure.NonRetryable() || failure.Type() != "cognition_business_failed" {
					t.Fatal("business failure completed", env.GetWorkflowError())
				}
			}
			env.AssertExpectations(t)
		})
	}
}

func TestCognitionReconcileCompletedExecutionKeepsBusinessFailure(t *testing.T) {
	ctx, app := scheduleRecoveryRepository(t)
	if _, err := app.DB.Pool().Exec(ctx, `DELETE FROM public.platform_workflow_intents`); err != nil {
		t.Fatal(err)
	}
	const intent, workflowID, inbox = "chat-outcome-intent", "chat-outcome-workflow", "chat-outcome-inbox"
	if _, err := app.DB.Pool().Exec(ctx, `INSERT INTO public.cognition_inbox(id,fluctlight_id,sequence,event_type,payload,causation_id,correlation_id,idempotency_key,occurred_at,status,error_code) VALUES($1,'retry-role',1,'conversation.message','{"turn_id":"chat-outcome-turn"}','source','turn:chat-outcome-turn','test',now(),'failed','direct_media_delivery_unresolved')`, inbox); err != nil {
		t.Fatal(err)
	}
	if _, err := app.DB.Pool().Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload,status,attempt_count) VALUES($1,$2,'cognition','cognition.processing',jsonb_build_object('inbox_id',$3::text,'fluctlight_id','retry-role'),'started',1)`, intent, workflowID, inbox); err != nil {
		t.Fatal(err)
	}
	c := &mocks.Client{}
	c.On("DescribeWorkflowExecution", mock.Anything, normalizedWorkflowID(workflowID), "").Return(&workflowservice.DescribeWorkflowExecutionResponse{WorkflowExecutionInfo: &workflowpb.WorkflowExecutionInfo{Status: enumspb.WORKFLOW_EXECUTION_STATUS_COMPLETED, Execution: &commonpb.WorkflowExecution{RunId: "completed-run"}}}, nil)
	d := &Dispatcher{App: app, Client: c}
	if _, err := d.ReconcileOnce(ctx, 100); err != nil {
		t.Fatal(err)
	}
	var status, code string
	if err := app.DB.Pool().QueryRow(ctx, `SELECT status,last_error FROM public.platform_workflow_intents WHERE intent_id=$1`, intent).Scan(&status, &code); err != nil || status != "failed" || code != "direct_media_delivery_unresolved" {
		t.Fatal(status, code, err)
	}
	c.AssertExpectations(t)
}
