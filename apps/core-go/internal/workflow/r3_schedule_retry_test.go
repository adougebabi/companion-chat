package workflow

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/testsuite"
)

func TestR3ScheduleGenerationFailureStopsAfterActivityRetries(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.OnActivity(EnsureCurrentDayScheduleActivity, mock.Anything, mock.Anything).Return(nil, errors.New("schedule_generation_failed")).Twice()
	env.ExecuteWorkflow(CurrentDayScheduleWorkflow, Input{IntentID: "schedule_intent:role", FluctlightID: "role"})
	if err := env.GetWorkflowError(); err == nil {
		t.Fatal("real failure returned indefinite pending")
	}
	env.AssertExpectations(t)
}
