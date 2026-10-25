package workflow

import (
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/testsuite"
)

func TestR9IndependentActivityWaitsForOriginalDueAndReturnsActualResult(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	now := time.Now().UTC()
	env.SetStartTime(now)
	input := Input{IntentID: "activity_resolution:real", FluctlightID: "role", ActivityID: "real", DueAt: now.Add(time.Hour).Format(time.RFC3339Nano)}
	called := false
	env.OnActivity(ResolveScheduledLifeActivityActivity, mock.Anything, input).Return(map[string]any{"activity_id": "real", "status": "completed"}, nil).Run(func(_ mock.Arguments) {
		called = true
		if env.Now().Before(now.Add(time.Hour)) {
			t.Error("resolution executed before elapsed time")
		}
	}).Once()
	env.ExecuteWorkflow(ActivityResolutionWorkflow, input)
	if err := env.GetWorkflowError(); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("timer did not execute result")
	}
	var result map[string]any
	if err := env.GetWorkflowResult(&result); err != nil || result["status"] != "completed" {
		t.Fatal(result, err)
	}
	env.AssertExpectations(t)
	for _, kind := range []string{"schedule.current_day", "life.activity.resolve"} {
		if workflowIDReusePolicy(kind) != enumspb.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE_FAILED_ONLY {
			t.Fatal("recovery cannot reuse original failed identity", kind)
		}
	}
}

func TestR9PauseDuringDueWaitDefersResolutionUntilResume(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	now := time.Now().UTC()
	env.SetStartTime(now)
	input := Input{IntentID: "activity_resolution:paused", FluctlightID: "role", ActivityID: "paused", DueAt: now.Add(time.Hour).Format(time.RFC3339Nano)}
	env.RegisterDelayedCallback(func() { env.SignalWorkflow("pause", nil) }, 30*time.Minute)
	env.RegisterDelayedCallback(func() { env.SignalWorkflow("resume", nil) }, 2*time.Hour)
	env.OnActivity(ResolveScheduledLifeActivityActivity, mock.Anything, input).Return(map[string]any{"status": "completed"}, nil).Run(func(mock.Arguments) {
		if env.Now().Before(now.Add(2 * time.Hour)) {
			t.Error("paused resolution executed")
		}
	}).Once()
	env.ExecuteWorkflow(ActivityResolutionWorkflow, input)
	if err := env.GetWorkflowError(); err != nil {
		t.Fatal(err)
	}
	env.AssertExpectations(t)
}
