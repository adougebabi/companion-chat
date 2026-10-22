package workflow

import (
	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/testsuite"
	"testing"
)

func TestOwnerScheduleWorkflowQueriesOneOriginalRequestAndReturnsActualResult(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterActivity(ProcessOwnerScheduleActivity)
	actual := map[string]any{"status": "completed", "disposition": "scheduled", "schedule_item_id": "actual-persisted-item"}
	env.OnActivity(ProcessOwnerScheduleActivity, mock.Anything, mock.MatchedBy(func(input Input) bool { return input.SessionID == "original" && input.FluctlightID == "fid" })).Return(actual, nil).Once()
	env.ExecuteWorkflow(OwnerScheduleWorkflow, Input{SessionID: "original", FluctlightID: "fid"})
	if err := env.GetWorkflowError(); err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := env.GetWorkflowResult(&result); err != nil || result["schedule_item_id"] != "actual-persisted-item" {
		t.Fatal(result, err)
	}
	env.AssertExpectations(t)
}
