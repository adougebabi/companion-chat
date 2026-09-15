package workflow

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
	"github.com/stretchr/testify/mock"
	enumspb "go.temporal.io/api/enums/v1"
	failurepb "go.temporal.io/api/failure/v1"
	historypb "go.temporal.io/api/history/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/converter"
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

func TestSafeActivityErrorCauseUsesStableProviderCode(t *testing.T) {
	if got := safeActivityErrorCause(errors.New("tool_call_invalid")); got != "tool_call_invalid" {
		t.Fatalf("safe activity cause = %q, want stable provider code", got)
	}
	diagnostic := activityLifecycleDiagnostic(Input{FluctlightID: "fl-1", CorrelationID: "wake_up:fl-1:cycle:1"}, "wake_up", core.LifecycleTransitionFailed, "failed", "wake_up_activity_failed", errors.New("tool_call_invalid"), activity.Info{})
	if diagnostic.ErrorCode != "tool_call_invalid" || diagnostic.SafeCause != "tool_call_invalid" {
		t.Fatalf("activity diagnostic = %#v", diagnostic)
	}
	if got := safeActivityErrorCause(errors.New("media prompt generation failed: provider timeout")); got != "media prompt generation failed: provider timeout" {
		t.Fatalf("non-provider activity cause = %q", got)
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

func TestIntentionTriggerWorkflowUsesDurableTemporalTimerBeforeActivity(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	dueAt := env.Now().Add(2 * time.Minute).UTC()
	activityAt := time.Time{}
	env.OnActivity(ProcessIntentionTriggerActivity, mock.Anything, mock.Anything).Return(func(context.Context, Input) (map[string]any, error) {
		activityAt = env.Now()
		return map[string]any{"status": "due", "intention_id": "intention-1"}, nil
	})
	env.ExecuteWorkflow(IntentionTriggerWorkflow, Input{IntentionID: "intention-1", DueAt: dueAt.Format(time.RFC3339Nano)})
	if err := env.GetWorkflowError(); err != nil {
		t.Fatal(err)
	}
	if activityAt.Before(dueAt) {
		t.Fatalf("Intention trigger activity ran before durable timer: activity=%s due=%s", activityAt, dueAt)
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
		{name: "paused completed", fluctlightStatus: "paused", workflowStatus: "completed", want: false},
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

func TestVisualIdentityStartOptionsAllowFailedRecoveryAndExposeDuplicateStart(t *testing.T) {
	options := workflowStartOptions("go:visual-identity-1", LifecycleQueue, "visual_identity.initialize")
	if options.WorkflowIDReusePolicy != enumspb.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE_FAILED_ONLY {
		t.Fatalf("visual identity reuse policy = %v, want allow duplicate failed only", options.WorkflowIDReusePolicy)
	}
	if !options.WorkflowExecutionErrorWhenAlreadyStarted {
		t.Fatal("visual identity duplicate start must return an explicit AlreadyStarted error")
	}
}

func TestProcessVisualIdentityActivityRecordsHeartbeatBeforeWork(t *testing.T) {
	Configure(nil)
	t.Cleanup(func() { Configure(nil) })
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestActivityEnvironment()
	env.RegisterActivity(ProcessVisualIdentityActivity)
	heartbeats := 0
	env.SetOnActivityHeartbeatListener(func(_ *activity.Info, details converter.EncodedValues) {
		var value map[string]any
		if err := details.Get(&value); err != nil {
			t.Fatalf("decode heartbeat details: %v", err)
		}
		if value["session_id"] == "visual-session-1" {
			heartbeats++
		}
	})
	_, err := env.ExecuteActivity(ProcessVisualIdentityActivity, Input{SessionID: "visual-session-1"})
	if err == nil {
		t.Fatal("unconfigured activity unexpectedly succeeded")
	}
	if heartbeats == 0 {
		t.Fatal("visual identity activity did not heartbeat before entering work")
	}
}

func TestVisualIdentityRecoveryTimingIsBounded(t *testing.T) {
	if visualIdentityRetryDelay < 5*time.Second || visualIdentityRetryDelay > time.Minute {
		t.Fatalf("visual identity retry delay = %s, want bounded backoff", visualIdentityRetryDelay)
	}
	if visualIdentityHeartbeatEvery <= 0 || visualIdentityHeartbeatEvery >= 30*time.Second {
		t.Fatalf("visual identity heartbeat interval = %s, want below 30s timeout", visualIdentityHeartbeatEvery)
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

func TestWakeUpTerminalFailureRemainsContinuouslyRecoverable(t *testing.T) {
	if !strings.Contains(reconcileIntentQuery, "intent_type='wake_up.current'") &&
		!strings.Contains(reconcileIntentQuery, "intent_type IN ('wake_up.current'") {
		t.Fatal("failed WakeUp intents are not part of continuous reconciliation")
	}
}

func TestLifecycleCorrelationForIntentUsesStableWakeUpCycleIdentity(t *testing.T) {
	wake := Input{IntentID: "wake-intent", FluctlightID: "fluctlight-1", Cycle: 7}
	if got, want := lifecycleCorrelationForIntent(wake.IntentID, "wake_up", wake), "wake_up:fluctlight-1:cycle:7"; got != want {
		t.Fatalf("wake-up correlation = %q, want %q", got, want)
	}
	wake.CorrelationID = "explicit-correlation"
	if got := lifecycleCorrelationForIntent(wake.IntentID, "wake_up", wake); got != wake.CorrelationID {
		t.Fatalf("explicit correlation was replaced: %q", got)
	}
	reflection := Input{IntentID: "reflection-intent", FluctlightID: "fluctlight-1"}
	if got := lifecycleCorrelationForIntent(reflection.IntentID, "reflection", reflection); got != reflection.IntentID {
		t.Fatalf("legacy Reflection correlation = %q, want intent identity", got)
	}
}

func TestDispatcherHydratesCorrelationAndCausationBeforeWorkflowStart(t *testing.T) {
	payload := []byte(`{"fluctlight_id":"fluctlight-1","cycle":3,"source_fact_id":"fact-1"}`)
	input := hydrateLifecycleInput("wake-intent", "wake_up.current", payload, Input{Cycle: 3})
	if input.IntentID != "wake-intent" || input.FluctlightID != "fluctlight-1" {
		t.Fatalf("durable identity was not hydrated: %#v", input)
	}
	if input.CorrelationID != "wake_up:fluctlight-1:cycle:3" || input.CausationID != "fact-1" {
		t.Fatalf("lifecycle identity was not hydrated: %#v", input)
	}
	explicit := hydrateLifecycleInput("reflection-intent", "reflection.run", []byte(`{"correlation_id":"reflection-root","causation_id":"turn-1"}`), Input{})
	if explicit.CorrelationID != "reflection-root" || explicit.CausationID != "turn-1" {
		t.Fatalf("explicit lifecycle identity was replaced: %#v", explicit)
	}
}

func TestActivityLifecycleDiagnosticIncludesTemporalIdentityAndAttempt(t *testing.T) {
	input := Input{IntentID: "reflection-intent", FluctlightID: "fluctlight-1", CorrelationID: "reflection-root", CausationID: "turn-1"}
	info := activity.Info{
		WorkflowExecution: workflow.Execution{ID: "go:reflection", RunID: "run-1"},
		ActivityType:      activity.Type{Name: "ProcessReflectionActivity"},
		ActivityID:        "activity-1",
		Attempt:           2,
	}
	diagnostic := activityLifecycleDiagnostic(input, "reflection", core.LifecycleTransitionActivityStarted, "running", "activity_started", nil, info)
	if diagnostic.CorrelationID != input.CorrelationID || diagnostic.CausationID != input.CausationID || diagnostic.IntentID != input.IntentID {
		t.Fatalf("domain identity missing from Activity diagnostic: %#v", diagnostic)
	}
	if diagnostic.WorkflowID != "go:reflection" || diagnostic.RunID != "run-1" || diagnostic.ActivityType != "ProcessReflectionActivity" || diagnostic.ActivityID != "activity-1" || diagnostic.Attempt != 2 {
		t.Fatalf("Temporal identity missing from Activity diagnostic: %#v", diagnostic)
	}
}

func TestActivityLifecycleDistinguishesNoopAndActionableOutcomes(t *testing.T) {
	wakeNoop, status, reason := wakeUpLifecycleOutcome(map[string]any{"status": "completed", "action_type": "no_op", "reason": "no_action_selected"})
	if wakeNoop != core.LifecycleTransitionCompletedNoop || status != "completed" || reason != "no_action_selected" {
		t.Fatalf("WakeUp no-op lifecycle = %q %q %q", wakeNoop, status, reason)
	}
	wakeQueued, queuedStatus, _ := wakeUpLifecycleOutcome(map[string]any{"status": "queued", "action_type": "capability"})
	if wakeQueued != core.LifecycleTransitionQueued || queuedStatus != "queued" {
		t.Fatalf("WakeUp queued lifecycle = %q %q", wakeQueued, queuedStatus)
	}
	reflectionNoop, _, reflectionReason := reflectionLifecycleOutcome(map[string]any{"status": "no_op", "reason": "no_evidence"})
	if reflectionNoop != core.LifecycleTransitionCompletedNoop || reflectionReason != "no_evidence" {
		t.Fatalf("Reflection no-op lifecycle = %q %q", reflectionNoop, reflectionReason)
	}
	reflectionAction, _, _ := reflectionLifecycleOutcome(map[string]any{"status": "applied"})
	if reflectionAction != core.LifecycleTransitionCompletedActionable {
		t.Fatalf("Reflection actionable lifecycle = %q", reflectionAction)
	}
	action, actionStatus, _ := actionLifecycleOutcome(map[string]any{"status": "completed"})
	if action != core.LifecycleTransitionCompletedActionable || actionStatus != "completed" {
		t.Fatalf("settled action lifecycle = %q %q", action, actionStatus)
	}
}

func TestDispatcherCASCompetitionDoesNotFailSettledIntent(t *testing.T) {
	for _, status := range []string{"started", "completed", "failed", "cancelled", "cancel_requested", "dead_letter"} {
		if !dispatchIntentRaceSettled(status) {
			t.Fatalf("concurrent status %q was not recognized as settled", status)
		}
	}
	for _, status := range []string{"", "pending", "retry"} {
		if dispatchIntentRaceSettled(status) {
			t.Fatalf("unsettled status %q was treated as a benign CAS race", status)
		}
	}
}

func TestAlreadyRunningTransitionRetainsTemporalRunIdentity(t *testing.T) {
	err := serviceerror.NewWorkflowExecutionAlreadyStarted("already started", "request-1", "run-1")
	if got := workflowRunIdentity(nil, err); got != "run-1" {
		t.Fatalf("already-running run identity = %q, want run-1", got)
	}
}

func TestWorkflowCriticalPersistenceWritesAreNeverIgnored(t *testing.T) {
	source, err := os.ReadFile("workflow.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	start := strings.Index(text, "func (d *Dispatcher) ReconcileOnce")
	end := strings.Index(text, "func normalizedWorkflowID")
	if start < 0 || end <= start {
		t.Fatal("Dispatcher lifecycle ownership boundaries not found")
	}
	body := text[start:end]
	if strings.Contains(body, "_, _ = d.App.DB.Pool().Exec") {
		t.Fatal("Dispatcher/Reconcile still ignores an authoritative database write")
	}
	for _, reason := range []string{
		"cognition_dependency_lookup_failed",
		"wakeup_dependency_lookup_failed",
		"action_dependency_lookup_failed",
		"reflection_dependency_lookup_failed",
		"visual_identity_dependency_lookup_failed",
		"workflow_start_settlement_failed",
		"media_terminal_status_not_written",
	} {
		if !strings.Contains(body, reason) {
			t.Fatalf("Reconcile dependency failure %q is not fail-closed and observable", reason)
		}
	}
}

func TestReconcileDoesNotGuessCompletedWorkflowOutcome(t *testing.T) {
	source, err := os.ReadFile("workflow.go")
	if err != nil {
		t.Fatal(err)
	}
	body := sourceBetweenWorkflow(t, string(source), "func (d *Dispatcher) ReconcileOnce", "func (d *Dispatcher) terminalFailureReason")
	if strings.Contains(body, "transition := core.LifecycleTransitionCompletedNoop") {
		t.Fatal("Reconcile still guesses every completed Workflow was a domain no-op")
	}
	if !strings.Contains(body, "LifecycleTransitionCancelled") || !strings.Contains(body, "workflow_cancelled") {
		t.Fatal("Temporal cancellation is not represented distinctly")
	}
	for _, fragment := range []string{
		"WorkflowID: normalizedWorkflowID(workflowID), RunID: runID",
		"workflowID, runID, core.LifecycleTransitionRetryScheduled",
		"workflowID, runID, core.LifecycleTransitionFailed",
	} {
		if !strings.Contains(body, fragment) {
			t.Fatalf("terminal/retry diagnostic lost Run ID at %q", fragment)
		}
	}
}

func sourceBetweenWorkflow(t *testing.T, source, start, end string) string {
	t.Helper()
	startIndex := strings.Index(source, start)
	if startIndex < 0 {
		t.Fatalf("source start %q not found", start)
	}
	endOffset := strings.Index(source[startIndex:], end)
	if endOffset < 0 {
		t.Fatalf("source end %q not found", end)
	}
	return source[startIndex : startIndex+endOffset]
}

func TestReflectionTerminalFailureRemainsContinuouslyRecoverable(t *testing.T) {
	options := workflowStartOptions("go:reflection:fact-1", LifecycleQueue, "reflection.run")
	if options.WorkflowIDReusePolicy != enumspb.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE_FAILED_ONLY {
		t.Fatalf("Reflection reuse policy = %v, want failed-only recovery", options.WorkflowIDReusePolicy)
	}
	if !strings.Contains(reconcileIntentQuery, "'reflection.run'") {
		t.Fatal("failed Reflection intents are absent from continuous reconciliation")
	}
}

func TestReconcileSkipsPendingIntentsUntilTheirDueTime(t *testing.T) {
	if !strings.Contains(reconcileIntentQuery, "(status='pending' AND (next_attempt_at IS NULL OR next_attempt_at <= now()))") {
		t.Fatalf("reconciliation can misdiagnose future pending intents: %s", reconcileIntentQuery)
	}
}

func TestReflectionRetryRequiresLiveFluctlightEvidenceAndBudget(t *testing.T) {
	if reflectionRetryDelay*time.Duration(reflectionMaximumAttempts) < 15*time.Minute {
		t.Fatalf("Reflection retry budget %s cannot outlive the 15m window lease", reflectionRetryDelay*time.Duration(reflectionMaximumAttempts))
	}
	if !reflectionIntentShouldRetry("active", true, reflectionMaximumAttempts-1) {
		t.Fatal("recoverable Reflection was not retried")
	}
	for _, test := range []struct {
		status      string
		hasEvidence bool
		attempts    int
	}{
		{status: "paused", hasEvidence: true, attempts: 1},
		{status: "active", hasEvidence: false, attempts: 1},
		{status: "active", hasEvidence: true, attempts: reflectionMaximumAttempts},
	} {
		if reflectionIntentShouldRetry(test.status, test.hasEvidence, test.attempts) {
			t.Fatalf("unexpected Reflection retry for %#v", test)
		}
	}
}

func TestActionIntentRetryOnlyWhenActionRemainsExecutable(t *testing.T) {
	for _, status := range []string{"frozen", "running"} {
		if !actionIntentShouldRetry(status) {
			t.Fatalf("action status %q should be retryable", status)
		}
	}
	for _, status := range []string{"completed", "failed", "cancelled", "paused", "deferred", "cancel_requested", ""} {
		if actionIntentShouldRetry(status) {
			t.Fatalf("terminal action status %q should not be requeued", status)
		}
	}
}

func TestDispatcherPrioritizesLifecycleRecoveryBeforeVisualIdentityRetries(t *testing.T) {
	visualIndex := strings.Index(dispatcherIntentOrder, "WHEN intent_type LIKE 'visual_identity.%'")
	if visualIndex < 0 {
		t.Fatalf("visual identity missing from dispatcher order: %s", dispatcherIntentOrder)
	}
	for _, intentPrefix := range []string{"media.%", "wake_up.%", "daily_review.%", "reflection.%"} {
		index := strings.Index(dispatcherIntentOrder, "WHEN intent_type LIKE '"+intentPrefix+"'")
		if index < 0 || index > visualIndex {
			t.Fatalf("%s must be dispatched before visual identity retries: %s", intentPrefix, dispatcherIntentOrder)
		}
	}
}

func TestDispatcherFairSelectionRanksEachIntentClassBeforeBacklog(t *testing.T) {
	source, err := os.ReadFile("workflow.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(source)
	for _, required := range []string{
		"ROW_NUMBER() OVER",
		"PARTITION BY intent_type",
		"class_rank",
		"ORDER BY class_rank",
	} {
		if !strings.Contains(body, required) {
			t.Fatalf("dispatcher fair selection missing %q", required)
		}
	}
}

func TestVisualIdentityUsesDedicatedQueueWithLifecycleCompatibility(t *testing.T) {
	source, err := os.ReadFile("workflow.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	if !strings.Contains(text, "VisualIdentityQueue") ||
		!strings.Contains(text, `"visual-identity"`) ||
		!strings.Contains(text, "case VisualIdentityQueue:") ||
		strings.Count(text, "RegisterWorkflow(VisualIdentityWorkflow)") < 2 ||
		!strings.Contains(text, "taskQueue = VisualIdentityQueue") {
		t.Fatal("Visual Identity does not have an isolated queue plus lifecycle history compatibility")
	}
}

func TestWakeUpAndReflectionUseCriticalQueueWithLifecycleCompatibility(t *testing.T) {
	source, err := os.ReadFile("workflow.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	legacyLane := sourceBetweenWorkflow(t, text, "case LifecycleQueue:", "case CriticalLifecycleQueue:")
	criticalLane := sourceBetweenWorkflow(t, text, "case CriticalLifecycleQueue:", "case VisualIdentityQueue:")
	dispatcher := sourceBetweenWorkflow(t, text, "func (d *Dispatcher) DispatchOnce", "func normalizedWorkflowID")
	wakeUpRoute := sourceBetweenWorkflow(t, dispatcher, `case "wake_up.current":`, `case "daily_review.current_day":`)
	reflectionRoute := sourceBetweenWorkflow(t, dispatcher, `case "reflection.run":`, `case "intention.trigger":`)

	if !strings.Contains(text, `CriticalLifecycleQueue       = "lifecycle-critical"`) ||
		!strings.Contains(text, "queues := []string{LifecycleQueue, CriticalLifecycleQueue,") ||
		!strings.Contains(text, "if queue == CriticalLifecycleQueue") {
		t.Fatal("critical lifecycle lane is missing its dedicated Worker capacity")
	}
	for _, registration := range []string{
		"RegisterWorkflow(WakeUpWorkflow)",
		"RegisterWorkflow(ReflectionWorkflow)",
		"RegisterActivity(ProcessWakeUpActivity)",
		"RegisterActivity(ProcessReflectionActivity)",
	} {
		if !strings.Contains(legacyLane, registration) {
			t.Fatalf("legacy lifecycle lane lost history-compatible registration %q", registration)
		}
		if !strings.Contains(criticalLane, registration) {
			t.Fatalf("critical lifecycle lane missing registration %q", registration)
		}
	}
	for _, forbidden := range []string{"DailyReviewWorkflow", "ProcessDailyReviewActivity"} {
		if strings.Contains(criticalLane, forbidden) {
			t.Fatalf("critical lifecycle lane must not register Provider-heavy %q", forbidden)
		}
	}
	if !strings.Contains(wakeUpRoute, "taskQueue = CriticalLifecycleQueue") {
		t.Fatal("new WakeUp runs are not routed to the critical lifecycle lane")
	}
	if !strings.Contains(reflectionRoute, "taskQueue = CriticalLifecycleQueue") {
		t.Fatal("new Reflection runs are not routed to the critical lifecycle lane")
	}
}

func TestWorkflowFunctionRegistryIncludesPlatformBoundaries(t *testing.T) {
	for _, intentType := range []string{"cognition.processing", "platform.control", "wake_up.current", "capability.action", "visual_identity.initialize", "conversation.summary"} {
		if fn, err := workflowFunction(intentType); err != nil || fn == nil {
			t.Fatalf("workflowFunction(%q) = %#v, %v", intentType, fn, err)
		}
	}
}

func TestConversationSummaryWorkflowExecutesOneSourceBoundedActivity(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	activityCalls := 0
	input := Input{
		IntentID: "summary-intent", FluctlightID: "fluctlight-1", ConversationID: "conversation-1",
		SourceMessageID: "message-64", SourceSequence: 64, FromSequence: 1, ToSequence: 40,
		SourceDigest: "source-digest", SourceMessageRefs: []string{"message:message-1", "message:message-40"},
	}
	env.OnActivity(ProcessConversationSummaryActivity, mock.Anything, input).Return(func(context.Context, Input) (map[string]any, error) {
		activityCalls++
		return map[string]any{"status": "active", "from_sequence": 1, "to_sequence": 40}, nil
	})
	env.ExecuteWorkflow(ConversationSummaryWorkflow, input)
	if err := env.GetWorkflowError(); err != nil {
		t.Fatal(err)
	}
	if activityCalls != 1 {
		t.Fatalf("activity calls = %d, want 1", activityCalls)
	}
	var result map[string]any
	if err := env.GetWorkflowResult(&result); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(result["from_sequence"]) != "1" || fmt.Sprint(result["to_sequence"]) != "40" {
		t.Fatalf("workflow result = %#v", result)
	}
}

func TestConversationSummaryWorkflowRejectsInvalidWindowBeforeActivity(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.ExecuteWorkflow(ConversationSummaryWorkflow, Input{IntentID: "summary-invalid", FluctlightID: "fluctlight-1", ConversationID: "conversation-1", SourceMessageID: "message-1", SourceSequence: 1, FromSequence: 2, ToSequence: 1, SourceDigest: "digest", SourceMessageRefs: []string{"message:message-1"}})
	if err := env.GetWorkflowError(); err == nil || !strings.Contains(err.Error(), "conversation summary input is invalid") {
		t.Fatalf("workflow error = %v", err)
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
