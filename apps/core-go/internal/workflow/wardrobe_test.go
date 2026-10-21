package workflow

import (
	"errors"
	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/testsuite"
	temporalworkflow "go.temporal.io/sdk/workflow"
	"testing"
)

func TestWardrobePreparationWorkflowWaitsDurablyAndDoesNotPollModel(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterActivity(ProcessWardrobePreparationActivity)
	env.OnActivity(ProcessWardrobePreparationActivity, mock.Anything, mock.Anything).Return(map[string]any{"status": "awaiting_wear", "reason": "sleeping"}, nil).Once()
	env.ExecuteWorkflow(WardrobePreparationWorkflow, Input{SessionID: "prepared", FluctlightID: "fid"})
	var continued *temporalworkflow.ContinueAsNewError
	if !errors.As(env.GetWorkflowError(), &continued) {
		t.Fatal("waiting must continue the durable workflow", env.GetWorkflowError())
	}
	env.AssertExpectations(t)
}
func TestWardrobePreparationWorkflowReturnsActualCompletedResult(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterActivity(ProcessWardrobePreparationActivity)
	env.OnActivity(ProcessWardrobePreparationActivity, mock.Anything, mock.Anything).Return(map[string]any{"status": "completed", "wear_receipt": "real-result-pointer"}, nil).Once()
	env.ExecuteWorkflow(WardrobePreparationWorkflow, Input{SessionID: "prepared", FluctlightID: "fid"})
	if err := env.GetWorkflowError(); err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := env.GetWorkflowResult(&result); err != nil || result["status"] != "completed" {
		t.Fatal(result, err)
	}
	env.AssertExpectations(t)
}
