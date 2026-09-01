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

func TestDailyReviewNeedsRetryWhenScheduleIsPending(t *testing.T) {
	if !dailyReviewNeedsRetry(map[string]any{"status": "pending"}) {
		t.Fatal("pending daily review should be retried")
	}
	if dailyReviewNeedsRetry(map[string]any{"status": "completed"}) {
		t.Fatal("completed daily review should not be retried")
	}
}

func TestWakeUpIntervalIsBounded(t *testing.T) {
	if got, want := wakeUpInterval(map[string]any{"interval_seconds": 1}), 5*time.Minute; got != want {
		t.Fatalf("minimum interval = %s, want %s", got, want)
	}
	if got, want := wakeUpInterval(map[string]any{"interval_seconds": 7 * 60}), 7*time.Minute; got != want {
		t.Fatalf("configured interval = %s, want %s", got, want)
	}
	if got, want := wakeUpInterval(map[string]any{"interval_seconds": 100 * 24 * 60 * 60}), 24*time.Hour; got != want {
		t.Fatalf("maximum interval = %s, want %s", got, want)
	}
}

func TestWorkflowFunctionRegistryIncludesPlatformBoundaries(t *testing.T) {
	for _, intentType := range []string{"cognition.processing", "platform.control", "wake_up.current"} {
		if fn, err := workflowFunction(intentType); err != nil || fn == nil {
			t.Fatalf("workflowFunction(%q) = %#v, %v", intentType, fn, err)
		}
	}
}

func TestPlatformControlWorkflowStopsOnSignal(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow("stop", true)
	}, time.Second)
	env.ExecuteWorkflow(PlatformControlWorkflow, Input{IntentID: "control-1"})
	if !env.IsWorkflowCompleted() {
		t.Fatal("platform control workflow did not complete")
	}
	var result map[string]any
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatal(err)
	}
	if result["status"] != "stopped" {
		t.Fatalf("status = %#v, want stopped", result["status"])
	}
}
