package workflow

import (
	"errors"
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

func TestWorldEventClockFailedHourStillSchedulesNextHour(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.SetStartTime(time.Date(2026, 11, 2, 8, 13, 0, 0, time.UTC))
	env.RegisterActivity(GenerateWorldEventActivity)
	env.OnActivity(GenerateWorldEventActivity, mock.Anything, mock.Anything).Return(map[string]any(nil), errors.New("model timeout")).Once()
	env.ExecuteWorkflow(WorldEventGenerationWorkflow, Input{WorldID: "world", IntentID: "world-clock"})
	if !workflow.IsContinueAsNewError(env.GetWorkflowError()) || !env.Now().Equal(time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)) {
		t.Fatal("failed hour stuck or killed the durable clock", env.GetWorkflowError(), env.Now())
	}
}

func TestOwnerRequestRetryWaitsDurablyBeforeReassessment(t *testing.T) {
	for _, seconds := range []float64{15, 60} {
		var suite testsuite.WorkflowTestSuite
		env := suite.NewTestWorkflowEnvironment()
		start := time.Date(2026, 11, 2, 8, 30, 0, 0, time.UTC)
		env.SetStartTime(start)
		env.RegisterActivity(ProcessCognitionActivity)
		input := Input{InboxID: "original-owner-fact"}
		env.OnActivity(ProcessCognitionActivity, mock.Anything, input).Return(map[string]any{"status": "retry_scheduled", "retry_after_seconds": seconds}, nil).Once()
		env.ExecuteWorkflow(CognitionProcessingWorkflow, input)
		if !workflow.IsContinueAsNewError(env.GetWorkflowError()) || env.Now().Sub(start) != time.Duration(seconds)*time.Second {
			t.Fatal("retry did not preserve original durable wait", env.GetWorkflowError(), env.Now())
		}
	}
}
