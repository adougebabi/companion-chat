package workflow

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	enumspb "go.temporal.io/api/enums/v1"
	failurepb "go.temporal.io/api/failure/v1"
	historypb "go.temporal.io/api/history/v1"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

func TestTemporalTerminalFailureMessagePreservesActivityCause(t *testing.T) {
	event := &historypb.HistoryEvent{
		EventType: enumspb.EVENT_TYPE_WORKFLOW_EXECUTION_FAILED,
		Attributes: &historypb.HistoryEvent_WorkflowExecutionFailedEventAttributes{
			WorkflowExecutionFailedEventAttributes: &historypb.WorkflowExecutionFailedEventAttributes{
				Failure: &failurepb.Failure{
					Message: "activity task failed",
					Cause:   &failurepb.Failure{Message: "media prompt generation failed: provider timeout"},
				},
			},
		},
	}
	if got, want := temporalTerminalFailureMessage(event), "media prompt generation failed: provider timeout"; got != want {
		t.Fatalf("temporalTerminalFailureMessage() = %q, want %q", got, want)
	}
}

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

func TestMediaWorkflowContinuesOneQualityRetry(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	attempts := 0
	env.OnActivity(ProcessMediaActivity, mock.Anything, mock.Anything).Return(func(context.Context, Input) (map[string]any, error) {
		attempts++
		if attempts == 1 {
			return map[string]any{"status": "quality_retry", "quality_retry_count": 1}, nil
		}
		return map[string]any{"status": "completed", "quality_verdict": "pass"}, nil
	})
	env.ExecuteWorkflow(MediaWorkflow, Input{IntentID: "media-quality-1"})
	if !env.IsWorkflowCompleted() {
		t.Fatal("media workflow did not complete")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("media workflow error = %v", err)
	}
	if attempts != 2 {
		t.Fatalf("activity attempts = %d, want 2", attempts)
	}
	var result map[string]any
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatal(err)
	}
	if result["status"] != "completed" || result["quality_verdict"] != "pass" {
		t.Fatalf("workflow result = %#v", result)
	}
}

func TestMediaWorkflowStopsAfterOneQualityRetry(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	attempts := 0
	env.OnActivity(ProcessMediaActivity, mock.Anything, mock.Anything).Return(func(context.Context, Input) (map[string]any, error) {
		attempts++
		return map[string]any{"status": "quality_retry", "quality_retry_count": attempts}, nil
	})
	env.ExecuteWorkflow(MediaWorkflow, Input{IntentID: "media-quality-2"})
	if !env.IsWorkflowCompleted() {
		t.Fatal("media workflow did not complete")
	}
	if attempts != 2 {
		t.Fatalf("activity attempts = %d, want 2", attempts)
	}
	if err := env.GetWorkflowError(); err == nil || !strings.Contains(err.Error(), "quality retry loop exceeded") {
		t.Fatalf("workflow error = %v, want quality retry loop error", err)
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

func TestWakeUpIntentRetriesOnlyForLiveFluctlights(t *testing.T) {
	for _, test := range []struct {
		name             string
		fluctlightStatus string
		workflowStatus   string
		want             bool
	}{
		{name: "active failed", fluctlightStatus: "active", workflowStatus: "failed", want: true},
		{name: "paused completed", fluctlightStatus: "paused", workflowStatus: "completed", want: true},
		{name: "active cancelled", fluctlightStatus: "active", workflowStatus: "cancelled", want: false},
		{name: "retired failed", fluctlightStatus: "retired", workflowStatus: "failed", want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := wakeUpIntentShouldRetry(test.fluctlightStatus, test.workflowStatus); got != test.want {
				t.Fatalf("wakeUpIntentShouldRetry(%q,%q) = %t, want %t", test.fluctlightStatus, test.workflowStatus, got, test.want)
			}
		})
	}
}

func TestWorkflowIDReusePolicyAllowsWakeUpRecovery(t *testing.T) {
	if got := workflowIDReusePolicy("wake_up.current"); got != enumspb.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE {
		t.Fatalf("wake-up reuse policy = %v, want allow duplicate", got)
	}
	if got := workflowIDReusePolicy("schedule.current_day"); got != enumspb.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE {
		t.Fatalf("schedule reuse policy = %v, want reject duplicate", got)
	}
}

func TestWakeUpRetryBackoffIsNotRequeuedBeforeDueTime(t *testing.T) {
	// The SQL candidate predicate intentionally excludes retry rows while their
	// next_attempt_at is in the future. Without this boundary ReconcileOnce
	// repeatedly pushed the wake-up five minutes forward every second, making a
	// failed stable workflow appear permanently dormant.
	if !strings.Contains(reconcileIntentQuery, "status='retry'") || !strings.Contains(reconcileIntentQuery, "next_attempt_at <= now()") {
		t.Fatalf("retry reconciliation query does not preserve backoff: %s", reconcileIntentQuery)
	}
}

func TestWorkflowFunctionRegistryIncludesPlatformBoundaries(t *testing.T) {
	for _, intentType := range []string{"cognition.processing", "platform.control", "wake_up.current", "capability.action", "visual_identity.initialize"} {
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
