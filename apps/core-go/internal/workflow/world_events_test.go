package workflow

import (
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

func TestWorldEventClockContinuesAtNextHourWithoutCharacter(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.SetStartTime(time.Date(2026, 10, 30, 1, 15, 0, 0, time.UTC))
	env.RegisterActivity(GenerateWorldEventActivity)
	env.OnActivity(GenerateWorldEventActivity, mock.Anything, mock.Anything).Return(map[string]any{"status": "no_event"}, nil).Once()
	env.ExecuteWorkflow(WorldEventGenerationWorkflow, Input{WorldID: "shared-world", IntentID: "world-clock"})
	if !workflow.IsContinueAsNewError(env.GetWorkflowError()) {
		t.Fatal("hourly world clock was not durable", env.GetWorkflowError())
	}
	if got := env.Now().UTC(); !got.Equal(time.Date(2026, 10, 30, 2, 0, 0, 0, time.UTC)) {
		t.Fatal("hour clock drifted", got)
	}
}
