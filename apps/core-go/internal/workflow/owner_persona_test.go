package workflow

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/testsuite"
)

func TestOwnerPersonaSwitchWorkflowRetriesAssessmentAndReturnsTerminal(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.OnActivity(ProcessOwnerPersonaSwitchActivity, mock.Anything, mock.Anything).Return(nil, errors.New("persona_switch_assessment_retrying")).Once()
	env.OnActivity(ProcessOwnerPersonaSwitchActivity, mock.Anything, mock.Anything).Return(map[string]any{"request_id": "req-1", "status": "completed", "active_profile_id": "b"}, nil).Once()
	env.ExecuteWorkflow(OwnerPersonaSwitchWorkflow, Input{IntentID: "req-1", FluctlightID: "fl-1"})
	if err := env.GetWorkflowError(); err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := env.GetWorkflowResult(&result); err != nil || result["status"] != "completed" {
		t.Fatalf("result=%v err=%v", result, err)
	}
	env.AssertExpectations(t)
	if fn, err := workflowFunction("persona.owner_switch"); err != nil || fn == nil {
		t.Fatalf("management registry=%v err=%v", fn, err)
	}
}

func TestOwnerPersonaSwitchWorkflowPreservesRejectedTerminal(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.OnActivity(ProcessOwnerPersonaSwitchActivity, mock.Anything, mock.Anything).Return(map[string]any{"request_id": "req-1", "status": "rejected", "error_code": "persona_switch_condition_not_satisfied"}, nil).Once()
	env.ExecuteWorkflow(OwnerPersonaSwitchWorkflow, Input{IntentID: "req-1", FluctlightID: "fl-1"})
	if err := env.GetWorkflowError(); err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := env.GetWorkflowResult(&result); err != nil || result["status"] != "rejected" {
		t.Fatalf("result=%v err=%v", result, err)
	}
	env.AssertExpectations(t)
}
