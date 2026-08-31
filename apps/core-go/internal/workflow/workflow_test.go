package workflow

import (
	"testing"
	"time"

	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

func TestNextLocalMidnightDelayUsesConfiguredTimezone(t *testing.T) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 31, 23, 30, 0, 0, location)
	if got, want := nextLocalMidnightDelay(now, "Asia/Shanghai"), 30*time.Minute; got != want {
		t.Fatalf("delay = %s, want %s", got, want)
	}
}

func TestWorkflowControlSignalsUpdateStatusQuery(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	var observed string
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow("pause", nil)
	}, time.Second)
	env.RegisterDelayedCallback(func() {
		value, err := env.QueryWorkflow("status")
		if err == nil {
			_ = value.Get(&observed)
		}
		env.CancelWorkflow()
	}, 2*time.Second)
	env.ExecuteWorkflow(func(ctx workflow.Context) (string, error) {
		control, err := registerWorkflowControl(ctx)
		if err != nil {
			return "", err
		}
		if err := workflow.Sleep(ctx, time.Hour); err != nil {
			return "", err
		}
		return controlStatus(control), nil
	})
	if observed != "paused" {
		t.Fatalf("status query = %q, want paused", observed)
	}
}

func controlStatus(control *workflowControl) string {
	if control.paused {
		return "paused"
	}
	return "running"
}

func TestNextLocalMidnightDelayFallsBackForUnknownTimezone(t *testing.T) {
	if got, want := nextLocalMidnightDelay(time.Now(), "not/a/zone"), 24*time.Hour; got != want {
		t.Fatalf("delay = %s, want %s", got, want)
	}
}
