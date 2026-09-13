package workflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
	enumspb "go.temporal.io/api/enums/v1"
	failurepb "go.temporal.io/api/failure/v1"
	historypb "go.temporal.io/api/history/v1"
	"go.temporal.io/api/serviceerror"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

const (
	LifecycleQueue               = "lifecycle"
	CriticalLifecycleQueue       = "lifecycle-critical"
	MediaQueue                   = "media"
	InteractionQueue             = "interaction"
	VisualIdentityQueue          = "visual-identity"
	WorkerDeploymentName         = "fluctlight"
	DefaultWorkerBuildID         = "platform-v1"
	mediaActivityMaximumAttempts = 3
	visualIdentityRetryDelay     = 5 * time.Second
	visualIdentityHeartbeatEvery = 10 * time.Second
	reflectionRetryDelay         = 5 * time.Minute
	reflectionMaximumAttempts    = 5
	defaultWakeUpIntervalSeconds = 30 * 60
	minWakeUpIntervalSeconds     = 5 * 60
	maxWakeUpIntervalSeconds     = 24 * 60 * 60
	dispatcherIntentOrder        = "CASE WHEN intent_type LIKE 'media.%' THEN 0 WHEN intent_type LIKE 'schedule.%' THEN 1 WHEN intent_type LIKE 'wake_up.%' THEN 2 WHEN intent_type LIKE 'daily_review.%' THEN 3 WHEN intent_type LIKE 'autonomy.%' THEN 4 WHEN intent_type LIKE 'capability.%' THEN 5 WHEN intent_type LIKE 'reflection.%' THEN 6 WHEN intent_type LIKE 'visual_identity.%' THEN 7 ELSE 8 END"
	reconcileIntentQuery         = `SELECT intent_id,workflow_id,intent_type,payload,COALESCE(status,'pending'),COALESCE(attempt_count,0) FROM public.platform_workflow_intents WHERE (status='pending' AND (next_attempt_at IS NULL OR next_attempt_at <= now())) OR status IN ('started','cancel_requested') OR (status='retry' AND (next_attempt_at IS NULL OR next_attempt_at <= now())) OR (status='failed' AND intent_type IN ('wake_up.current','cognition.processing','autonomy.action','capability.action','reflection.run')) ORDER BY started_at NULLS LAST,created_at LIMIT $1`
)

var runtime struct {
	sync.RWMutex
	app *core.App
}

var workflowDiagnosticSampleState = struct {
	sync.Mutex
	last map[string]time.Time
}{last: map[string]time.Time{}}

func Configure(app *core.App) {
	runtime.Lock()
	defer runtime.Unlock()
	runtime.app = app
}

func app() *core.App { runtime.RLock(); defer runtime.RUnlock(); return runtime.app }

// WorkerDeploymentBuildID returns the immutable build identity used by every
// queue worker and by the deployment bootstrap. Keeping this resolution in one
// place prevents a Worker from registering one version while the bootstrap
// routes another version as current.
func WorkerDeploymentBuildID() string {
	buildID := strings.TrimSpace(os.Getenv("TEMPORAL_WORKER_BUILD_ID"))
	if buildID == "" {
		return DefaultWorkerBuildID
	}
	return buildID
}

// WorkerDeploymentVersionSetter is the minimal Temporal control seam used by
// the startup bootstrap. Keeping it narrow also makes retry behavior testable
// without replacing the workflow runtime or its task queues.
type WorkerDeploymentVersionSetter interface {
	SetCurrentVersion(context.Context, client.WorkerDeploymentSetCurrentVersionOptions) (client.WorkerDeploymentSetCurrentVersionResponse, error)
}

// EnsureWorkerDeploymentCurrentVersion makes a fresh Temporal namespace ready
// for application workflows. Worker Deployment versions are created lazily by
// the first poller, so this call retries until the versioned pollers have been
// observed. It is safe on every restart: setting the already-current version is
// an idempotent operation, while a missing/invalid poller fails readiness rather
// than allowing new intents to accumulate in the unversioned queue.
func EnsureWorkerDeploymentCurrentVersion(ctx context.Context, handle WorkerDeploymentVersionSetter, buildID string) error {
	return ensureWorkerDeploymentCurrentVersion(ctx, handle, buildID, 2*time.Second, 60)
}

func ensureWorkerDeploymentCurrentVersion(ctx context.Context, handle WorkerDeploymentVersionSetter, buildID string, retryInterval time.Duration, maxAttempts int) error {
	buildID = strings.TrimSpace(buildID)
	if buildID == "" {
		return fmt.Errorf("Temporal Worker Deployment build ID is empty")
	}
	if handle == nil {
		return fmt.Errorf("Temporal Worker Deployment handle is nil")
	}
	if retryInterval < 0 {
		retryInterval = 0
	}
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		_, err := handle.SetCurrentVersion(ctx, client.WorkerDeploymentSetCurrentVersionOptions{BuildID: buildID})
		if err == nil {
			return nil
		}
		lastErr = err
		if attempt == maxAttempts-1 {
			break
		}
		timer := time.NewTimer(retryInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return fmt.Errorf("set Temporal Worker Deployment %q current version to %q: %w", WorkerDeploymentName, buildID, lastErr)
}

type Input struct {
	IntentID           string   `json:"intent_id"`
	CorrelationID      string   `json:"correlation_id"`
	CausationID        string   `json:"causation_id"`
	FluctlightID       string   `json:"fluctlight_id"`
	SessionID          string   `json:"session_id"`
	LocalDate          string   `json:"local_date"`
	Cycle              int      `json:"cycle"`
	ActionID           string   `json:"action_id"`
	MemoryID           string   `json:"memory_id"`
	Revision           int      `json:"revision"`
	ProviderEndpointID string   `json:"provider_endpoint_id"`
	ModelID            string   `json:"model_id"`
	InboxID            string   `json:"inbox_id"`
	IntentionID        string   `json:"intention_id"`
	DueAt              string   `json:"due_at"`
	ConversationID     string   `json:"conversation_id"`
	SourceMessageID    string   `json:"source_message_id"`
	SourceSequence     int      `json:"source_sequence"`
	FromSequence       int      `json:"from_sequence"`
	ToSequence         int      `json:"to_sequence"`
	SourceDigest       string   `json:"source_digest"`
	SourceMessageRefs  []string `json:"source_message_refs"`
}

// VisualIdentityWorkflow coordinates the image/vision/patch loop while the
// actual image provider remains owned by MediaWorkflow on the media queue.
// Continue-as-new keeps the orchestration history bounded; all durable state
// and candidate assets live in the Core-owned PostgreSQL tables.
func VisualIdentityWorkflow(ctx workflow.Context, input Input) (map[string]any, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: 20 * time.Minute, HeartbeatTimeout: 30 * time.Second, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 3}})
	control, err := registerWorkflowControl(ctx)
	if err != nil {
		return nil, err
	}
	if err := control.waitUntilResumed(ctx); err != nil {
		return nil, err
	}
	if input.SessionID == "" {
		return nil, fmt.Errorf("visual identity session is required")
	}
	var result map[string]any
	if err := workflow.ExecuteActivity(ctx, ProcessVisualIdentityActivity, input).Get(ctx, &result); err != nil {
		return nil, err
	}
	status := stringValue(result["status"])
	if status == "completed" || status == "failed" || status == "cancelled" || status == "awaiting_review" {
		return result, nil
	}
	if err := control.waitUntilResumed(ctx); err != nil {
		return nil, err
	}
	if err := workflow.Sleep(ctx, 5*time.Second); err != nil {
		return nil, err
	}
	return nil, workflow.NewContinueAsNewError(ctx, VisualIdentityWorkflow, input)
}

// WakeUpWorkflow executes one internal-life cycle. The next cycle is released
// by the Redis debounce hint after the configured quiet period; PostgreSQL and
// the stable workflow intent remain the durable authorities.
func WakeUpWorkflow(ctx workflow.Context, input Input) (map[string]any, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: 10 * time.Minute, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 2}})
	control, err := registerWorkflowControl(ctx)
	if err != nil {
		return nil, err
	}
	if err := control.waitUntilResumed(ctx); err != nil {
		return nil, err
	}
	var result map[string]any
	if err := workflow.ExecuteActivity(ctx, ProcessWakeUpActivity, input).Get(ctx, &result); err != nil {
		// A failed cycle is reconciled as a retryable intent. Do not schedule a
		// next quiet-period key when no wake-up fact was committed.
		return nil, err
	}
	if stringValue(result["status"]) == "inactive" {
		return result, nil
	}
	return result, nil
}

func wakeUpInterval(result map[string]any) time.Duration {
	seconds := defaultWakeUpIntervalSeconds
	if raw, ok := result["interval_seconds"]; ok {
		if parsed, valid := wakeUpNumber(raw); valid {
			seconds = int(parsed)
		}
	}
	if seconds < minWakeUpIntervalSeconds {
		seconds = minWakeUpIntervalSeconds
	}
	if seconds > maxWakeUpIntervalSeconds {
		seconds = maxWakeUpIntervalSeconds
	}
	return time.Duration(seconds) * time.Second
}

func wakeUpNumber(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case int:
		return float64(typed), true
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	default:
		return 0, false
	}
}

func CognitionProcessingWorkflow(ctx workflow.Context, input Input) (map[string]any, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: 10 * time.Minute, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 3}})
	control, err := registerWorkflowControl(ctx)
	if err != nil {
		return nil, err
	}
	if err := control.waitUntilResumed(ctx); err != nil {
		return nil, err
	}
	var result map[string]any
	if err := workflow.ExecuteActivity(ctx, ProcessCognitionActivity, input).Get(ctx, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// IntentionTriggerWorkflow owns the durable wait/reassessment boundary for a
// typed Intention. Time triggers sleep on Temporal history; event and semantic
// triggers periodically ask Core whether a new authoritative fact exists.
func IntentionTriggerWorkflow(ctx workflow.Context, input Input) (map[string]any, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: 5 * time.Minute, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 3}})
	control, err := registerWorkflowControl(ctx)
	if err != nil {
		return nil, err
	}
	if err := control.waitUntilResumed(ctx); err != nil {
		return nil, err
	}
	if input.IntentionID == "" {
		return nil, fmt.Errorf("intention id is required")
	}
	if input.DueAt != "" {
		dueAt, parseErr := time.Parse(time.RFC3339Nano, input.DueAt)
		if parseErr != nil {
			return nil, fmt.Errorf("intention due_at invalid: %w", parseErr)
		}
		if delay := dueAt.Sub(workflow.Now(ctx)); delay > 0 {
			if err := workflow.Sleep(ctx, delay); err != nil {
				return nil, err
			}
		}
	}
	var result map[string]any
	if err := workflow.ExecuteActivity(ctx, ProcessIntentionTriggerActivity, input).Get(ctx, &result); err != nil {
		return nil, err
	}
	if stringValue(result["status"]) == "pending" {
		if err := workflow.Sleep(ctx, time.Minute); err != nil {
			return nil, err
		}
		return nil, workflow.NewContinueAsNewError(ctx, IntentionTriggerWorkflow, input)
	}
	return result, nil
}

func PlatformControlWorkflow(ctx workflow.Context, input Input) (map[string]any, error) {
	control, err := registerWorkflowControl(ctx)
	if err != nil {
		return nil, err
	}
	if err := control.waitUntilResumed(ctx); err != nil {
		return nil, err
	}
	var stop bool
	stopCh := workflow.GetSignalChannel(ctx, "stop")
	workflow.Go(ctx, func(ctx workflow.Context) {
		stopCh.Receive(ctx, &stop)
	})
	if err := workflow.Await(ctx, func() bool { return stop }); err != nil {
		return nil, err
	}
	return map[string]any{"status": "stopped", "intent_id": input.IntentID}, nil
}

func DailyReviewWorkflow(ctx workflow.Context, input Input) (map[string]any, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: 10 * time.Minute, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 3}})
	control, err := registerWorkflowControl(ctx)
	if err != nil {
		return nil, err
	}
	if err := control.waitUntilResumed(ctx); err != nil {
		return nil, err
	}
	var result map[string]any
	if err := workflow.ExecuteActivity(ctx, ProcessDailyReviewActivity, input).Get(ctx, &result); err != nil {
		return nil, err
	}
	if dailyReviewNeedsRetry(result) {
		// Activation enqueues schedule generation and daily review together.
		// If the review observes the schedule before it is accepted, keep the
		// durable workflow alive and retry after the schedule workflow has had a
		// chance to settle instead of losing the only daily contact decision. A
		// stale activation date must not be carried into the retry: schedule
		// generation always owns the current local day, so let the activity
		// recalculate it after the bounded wait.
		if err := workflow.Sleep(ctx, 5*time.Minute); err != nil {
			return nil, err
		}
		next := input
		next.LocalDate = ""
		return nil, workflow.NewContinueAsNewError(ctx, DailyReviewWorkflow, next)
	}
	if stringValue(result["status"]) == "inactive" {
		return result, nil
	}
	// A date-scoped daily-review workflow is one-shot after that date settles.
	// The next accepted local-day Schedule creates the next date-scoped intent;
	// keeping this workflow alive would accumulate one permanent reviewer per day.
	return result, nil
}

func dailyReviewNeedsRetry(result map[string]any) bool {
	return stringValue(result["status"]) == "pending"
}

func MediaWorkflow(ctx workflow.Context, input Input) (map[string]any, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: 20 * time.Minute, HeartbeatTimeout: 30 * time.Second, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: mediaActivityMaximumAttempts}})
	control, err := registerWorkflowControl(ctx)
	if err != nil {
		return nil, err
	}
	for attempt := 0; attempt < 2; attempt++ {
		if err := control.waitUntilResumed(ctx); err != nil {
			return nil, err
		}
		var result map[string]any
		if err := workflow.ExecuteActivity(ctx, ProcessMediaActivity, input).Get(ctx, &result); err != nil {
			return nil, err
		}
		if stringValue(result["status"]) == "quality_retry" {
			if attempt == 0 {
				continue
			}
			return nil, fmt.Errorf("media quality retry loop exceeded")
		}
		return result, nil
	}
	return nil, fmt.Errorf("media workflow completed without a result")
}

func CurrentDayScheduleWorkflow(ctx workflow.Context, input Input) (map[string]any, error) {
	// Initial schedule generation is a real structured Provider call and may
	// take several minutes on a local model. The previous 30s activity timeout
	// cancelled every first attempt before the Provider could respond.
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: 10 * time.Minute, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 2}})
	control, err := registerWorkflowControl(ctx)
	if err != nil {
		return nil, err
	}
	if err := control.waitUntilResumed(ctx); err != nil {
		return nil, err
	}
	var result map[string]any
	if err := workflow.ExecuteActivity(ctx, EnsureCurrentDayScheduleActivity, input).Get(ctx, &result); err != nil {
		return nil, err
	}
	result["intent_id"] = input.IntentID
	if err := control.waitUntilResumed(ctx); err != nil {
		return nil, err
	}
	if stringValue(result["status"]) == "pending" {
		if err := workflow.Sleep(ctx, 5*time.Minute); err != nil {
			return nil, err
		}
		return nil, workflow.NewContinueAsNewError(ctx, CurrentDayScheduleWorkflow, input)
	}
	if stringValue(result["status"]) == "inactive" {
		return result, nil
	}
	// Keep the stable lifecycle workflow alive across local-day boundaries. The
	// activity supplies the canonical timezone; workflow.Now is deterministic
	// and the timer survives Worker restarts without wall-clock calls in the
	// workflow body.
	delay := nextLocalMidnightDelay(workflow.Now(ctx), stringValue(result["timezone"]))
	if err := workflow.Sleep(ctx, delay); err != nil {
		return nil, err
	}
	if err := control.waitUntilResumed(ctx); err != nil {
		return nil, err
	}
	return nil, workflow.NewContinueAsNewError(ctx, CurrentDayScheduleWorkflow, input)
}

func nextLocalMidnightDelay(now time.Time, timezone string) time.Duration {
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return 24 * time.Hour
	}
	localNow := now.In(location)
	next := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, location).AddDate(0, 0, 1)
	delay := next.Sub(now)
	if delay < time.Second {
		return time.Second
	}
	return delay
}

type workflowControl struct {
	paused bool
}

func registerWorkflowControl(ctx workflow.Context) (*workflowControl, error) {
	// Keep an explicit replay marker for every long-lived workflow. Existing
	// histories resolve to DefaultVersion; new histories carry version 1 so a
	// future contract change can branch deterministically with GetVersion.
	_ = workflow.GetVersion(ctx, "go-core-workflow-contract", workflow.DefaultVersion, 1)
	control := &workflowControl{}
	if err := workflow.SetQueryHandler(ctx, "status", func() (string, error) {
		if control.paused {
			return "paused", nil
		}
		return "running", nil
	}); err != nil {
		return nil, err
	}
	pauseCh := workflow.GetSignalChannel(ctx, "pause")
	resumeCh := workflow.GetSignalChannel(ctx, "resume")
	workflow.Go(ctx, func(ctx workflow.Context) {
		var ignored any
		pauseCh.Receive(ctx, &ignored)
		if ctx.Err() == nil {
			control.paused = true
		}
	})
	workflow.Go(ctx, func(ctx workflow.Context) {
		var ignored any
		resumeCh.Receive(ctx, &ignored)
		if ctx.Err() == nil {
			control.paused = false
		}
	})
	return control, nil
}

func (control *workflowControl) waitUntilResumed(ctx workflow.Context) error {
	return workflow.Await(ctx, func() bool { return !control.paused })
}

func AutonomyActionWorkflow(ctx workflow.Context, input Input) (map[string]any, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: 10 * time.Minute, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 3}})
	control, err := registerWorkflowControl(ctx)
	if err != nil {
		return nil, err
	}
	if err := control.waitUntilResumed(ctx); err != nil {
		return nil, err
	}
	var result map[string]any
	if err := workflow.ExecuteActivity(ctx, ProcessAutonomyActionActivity, input).Get(ctx, &result); err != nil {
		return nil, err
	}
	if err := control.waitUntilResumed(ctx); err != nil {
		return nil, err
	}
	return result, nil
}

func CapabilityActionWorkflow(ctx workflow.Context, input Input) (map[string]any, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: 10 * time.Minute, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 3}})
	control, err := registerWorkflowControl(ctx)
	if err != nil {
		return nil, err
	}
	if err := control.waitUntilResumed(ctx); err != nil {
		return nil, err
	}
	var result map[string]any
	if err := workflow.ExecuteActivity(ctx, ProcessCapabilityActionActivity, input).Get(ctx, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func ReflectionWorkflow(ctx workflow.Context, input Input) (map[string]any, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: 10 * time.Minute, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 2}})
	control, err := registerWorkflowControl(ctx)
	if err != nil {
		return nil, err
	}
	if err := control.waitUntilResumed(ctx); err != nil {
		return nil, err
	}
	var result map[string]any
	if err := workflow.ExecuteActivity(ctx, ProcessReflectionActivity, input).Get(ctx, &result); err != nil {
		return nil, err
	}
	if err := control.waitUntilResumed(ctx); err != nil {
		return nil, err
	}
	return result, nil
}

func MemoryEmbeddingWorkflow(ctx workflow.Context, input Input) (map[string]any, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: 10 * time.Minute, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 2}})
	control, err := registerWorkflowControl(ctx)
	if err != nil {
		return nil, err
	}
	if err := control.waitUntilResumed(ctx); err != nil {
		return nil, err
	}
	var result map[string]any
	if err := workflow.ExecuteActivity(ctx, ProcessMemoryEmbeddingActivity, input).Get(ctx, &result); err != nil {
		return nil, err
	}
	if err := control.waitUntilResumed(ctx); err != nil {
		return nil, err
	}
	return result, nil
}

func ConversationSummaryWorkflow(ctx workflow.Context, input Input) (map[string]any, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: 10 * time.Minute, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 3}})
	control, err := registerWorkflowControl(ctx)
	if err != nil {
		return nil, err
	}
	if err := control.waitUntilResumed(ctx); err != nil {
		return nil, err
	}
	if input.IntentID == "" || input.FluctlightID == "" || input.ConversationID == "" || input.SourceMessageID == "" || input.SourceSequence < 1 || input.FromSequence < 1 || input.ToSequence < input.FromSequence || input.ToSequence > input.SourceSequence || input.SourceDigest == "" || len(input.SourceMessageRefs) == 0 {
		return nil, fmt.Errorf("conversation summary input is invalid")
	}
	var result map[string]any
	if err := workflow.ExecuteActivity(ctx, ProcessConversationSummaryActivity, input).Get(ctx, &result); err != nil {
		return nil, err
	}
	if err := control.waitUntilResumed(ctx); err != nil {
		return nil, err
	}
	return result, nil
}

func ProcessDailyReviewActivity(ctx context.Context, input Input) (map[string]any, error) {
	application := app()
	if application == nil {
		return nil, fmt.Errorf("Go Core Worker is not configured")
	}
	return application.ProcessDailyReview(ctx, input.FluctlightID, input.LocalDate)
}

func ProcessWakeUpActivity(ctx context.Context, input Input) (map[string]any, error) {
	application := app()
	if application == nil {
		return nil, fmt.Errorf("Go Core Worker is not configured")
	}
	ctx, input = prepareActivityLifecycleContext(ctx, input, "wake_up")
	recordActivityLifecycle(application, ctx, input, "wake_up", core.LifecycleTransitionActivityStarted, "running", "activity_started", nil)
	slog.Default().Info("Go Worker wake-up activity started", "fluctlight_id", input.FluctlightID, "cycle", input.Cycle, "correlation_id", input.CorrelationID)
	result, err := application.ProcessWakeUp(ctx, input.FluctlightID, input.Cycle)
	if err != nil {
		recordActivityLifecycle(application, ctx, input, "wake_up", core.LifecycleTransitionFailed, "failed", "wake_up_activity_failed", err)
		slog.Default().Error("Go Worker wake-up activity failed", "fluctlight_id", input.FluctlightID, "cycle", input.Cycle, "correlation_id", input.CorrelationID, "error_type", fmt.Sprintf("%T", err))
	} else {
		transition, status, reason := wakeUpLifecycleOutcome(result)
		recordActivityLifecycle(application, ctx, input, "wake_up", transition, status, reason, nil)
		slog.Default().Info("Go Worker wake-up activity completed", "fluctlight_id", input.FluctlightID, "cycle", input.Cycle, "correlation_id", input.CorrelationID, "status", stringValue(result["status"]))
	}
	return result, err
}

func ProcessCognitionActivity(ctx context.Context, input Input) (map[string]any, error) {
	application := app()
	if application == nil {
		return nil, fmt.Errorf("Go Core Worker is not configured")
	}
	return application.ProcessCognitionInbox(ctx, input.InboxID)
}

func ProcessIntentionTriggerActivity(ctx context.Context, input Input) (map[string]any, error) {
	application := app()
	if application == nil {
		return nil, fmt.Errorf("Go Core Worker is not configured")
	}
	return application.ProcessIntentionTrigger(ctx, input.IntentionID)
}

func PlatformControlActivity(ctx context.Context, input Input) (map[string]any, error) {
	return map[string]any{"status": "ready", "intent_id": input.IntentID}, nil
}

func ProcessMediaActivity(ctx context.Context, input Input) (map[string]any, error) {
	application := app()
	if application == nil {
		return nil, fmt.Errorf("Go Core Worker is not configured")
	}
	result, err := application.ProcessMediaIntent(ctx, input.IntentID)
	if err != nil {
		// Keep the durable media target truthful after Temporal exhausts its
		// bounded activity retries. The workflow intent reconciliation records
		// the execution failure separately; this row is what product reads. Save
		// the bounded application error before reconciliation can replace it with
		// the generic workflow_terminal_failure fallback.
		terminal := !activity.IsActivity(ctx) || activity.GetInfo(ctx).Attempt >= mediaActivityMaximumAttempts
		if terminal {
			if repairErr := recordMediaActivityFailure(application, input.IntentID, err); repairErr != nil {
				slog.Default().Warn("Go Worker could not persist terminal media failure", "intent_id", input.IntentID, "error", repairErr)
			}
		}
		return nil, err
	}
	return result, nil
}

func recordMediaActivityFailure(application *core.App, intentID string, err error) error {
	if application == nil || strings.TrimSpace(intentID) == "" || err == nil {
		return nil
	}
	message := boundedTemporalFailureMessage(&failurepb.Failure{Message: err.Error()})
	if message == "" {
		return nil
	}
	// Activity contexts may already be cancelled when the final attempt is
	// being reported. A short detached context keeps the diagnostic write from
	// being lost while retaining a strict upper bound on the repair query.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(context.Background()), 2*time.Second)
	defer cancel()
	tx, err := application.DB.Pool().Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `UPDATE public.media_intents SET status='failed',revision=revision+1 WHERE id=$1 AND status IN ('pending','running')`, intentID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE public.platform_workflow_intents SET last_error=$2 WHERE workflow_id=(SELECT workflow_id FROM public.media_intents WHERE id=$1) AND intent_type='media.generation' AND status IN ('pending','started','retry')`, intentID, message); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func ProcessAutonomyActionActivity(ctx context.Context, input Input) (map[string]any, error) {
	application := app()
	if application == nil {
		return nil, fmt.Errorf("Go Core Worker is not configured")
	}
	return processActionActivity(application, ctx, input, "autonomy", application.ProcessAutonomyAction)
}

func ProcessCapabilityActionActivity(ctx context.Context, input Input) (map[string]any, error) {
	application := app()
	if application == nil {
		return nil, fmt.Errorf("Go Core Worker is not configured")
	}
	return processActionActivity(application, ctx, input, "capability", application.ProcessCapabilityAction)
}

func processActionActivity(application *core.App, ctx context.Context, input Input, surface string, execute func(context.Context, string) (map[string]any, error)) (map[string]any, error) {
	ctx, input = prepareActivityLifecycleContext(ctx, input, surface)
	recordActivityLifecycle(application, ctx, input, surface, core.LifecycleTransitionActivityStarted, "running", "activity_started", nil)
	result, err := execute(ctx, input.ActionID)
	if err != nil {
		recordActivityLifecycle(application, ctx, input, surface, core.LifecycleTransitionFailed, "failed", "action_activity_failed", err)
		return nil, err
	}
	transition, status, reason := actionLifecycleOutcome(result)
	recordActivityLifecycle(application, ctx, input, surface, transition, status, reason, nil)
	return result, nil
}

func actionLifecycleOutcome(result map[string]any) (core.LifecycleTransition, string, string) {
	status := firstString(result["status"], firstString(result["action_status"], "completed"))
	transition := core.LifecycleTransitionCompletedNoop
	if status == "completed" {
		transition = core.LifecycleTransitionCompletedActionable
	}
	return transition, status, firstString(result["reason"], firstString(result["error_code"], "action_settled"))
}

func ProcessReflectionActivity(ctx context.Context, input Input) (map[string]any, error) {
	application := app()
	if application == nil {
		return nil, fmt.Errorf("Go Core Worker is not configured")
	}
	ctx, input = prepareActivityLifecycleContext(ctx, input, "reflection")
	recordActivityLifecycle(application, ctx, input, "reflection", core.LifecycleTransitionActivityStarted, "running", "activity_started", nil)
	result, err := application.ProcessReflection(ctx, input.FluctlightID, input.CorrelationID)
	if err != nil {
		recordActivityLifecycle(application, ctx, input, "reflection", core.LifecycleTransitionFailed, "failed", "reflection_activity_failed", err)
		return nil, err
	}
	transition, status, reason := reflectionLifecycleOutcome(result)
	recordActivityLifecycle(application, ctx, input, "reflection", transition, status, reason, nil)
	return result, nil
}

func wakeUpLifecycleOutcome(result map[string]any) (core.LifecycleTransition, string, string) {
	transition := core.LifecycleTransitionCompletedNoop
	status := stringValue(result["status"])
	if status == "queued" {
		transition = core.LifecycleTransitionQueued
	} else if actionType := stringValue(result["action_type"]); status == "completed" && actionType != "" && actionType != "no_op" {
		transition = core.LifecycleTransitionCompletedActionable
	}
	return transition, status, firstString(result["reason"], "wake_up_completed")
}

func reflectionLifecycleOutcome(result map[string]any) (core.LifecycleTransition, string, string) {
	transition := core.LifecycleTransitionCompletedNoop
	status := stringValue(result["status"])
	if status == "applied" {
		transition = core.LifecycleTransitionCompletedActionable
	}
	return transition, status, firstString(result["reason"], firstString(status, "reflection_completed"))
}

func prepareActivityLifecycleContext(ctx context.Context, input Input, surface string) (context.Context, Input) {
	input.CorrelationID = lifecycleCorrelationForIntent(input.IntentID, surface, input)
	ctx = core.WithProviderCorrelation(ctx, input.CorrelationID)
	if activity.IsActivity(ctx) {
		info := activity.GetInfo(ctx)
		attemptIdentity := strings.Join([]string{
			info.WorkflowExecution.RunID,
			info.ActivityID,
			fmt.Sprint(info.Attempt),
		}, ":")
		ctx = core.WithProviderAttemptIdentity(ctx, attemptIdentity)
	}
	return ctx, input
}

func recordActivityLifecycle(application *core.App, ctx context.Context, input Input, surface string, transition core.LifecycleTransition, status, reason string, activityErr error) {
	if application == nil {
		return
	}
	info := activity.Info{}
	if activity.IsActivity(ctx) {
		info = activity.GetInfo(ctx)
	}
	application.RecordLifecycleDiagnosticBestEffort(ctx, activityLifecycleDiagnostic(input, surface, transition, status, reason, activityErr, info))
}

func activityLifecycleDiagnostic(input Input, surface string, transition core.LifecycleTransition, status, reason string, activityErr error, info activity.Info) core.LifecycleDiagnostic {
	diagnostic := core.LifecycleDiagnostic{
		Surface: surface, Transition: transition,
		FluctlightID: input.FluctlightID, CorrelationID: input.CorrelationID,
		CausationID: input.CausationID, IntentID: input.IntentID,
		WorkflowID: info.WorkflowExecution.ID, RunID: info.WorkflowExecution.RunID,
		ActivityType: info.ActivityType.Name, ActivityID: info.ActivityID,
		Stage: "activity", Status: status, ReasonCode: reason,
		Attempt: int(info.Attempt),
	}
	if diagnostic.CorrelationID == "" {
		diagnostic.CorrelationID = lifecycleCorrelationForIntent(input.IntentID, surface, input)
	}
	if activityErr != nil {
		diagnostic.Severity = "error"
		diagnostic.ErrorCategory = "activity"
		diagnostic.ErrorCode = reason
		diagnostic.Retryable = true
		diagnostic.SafeCause = boundedTemporalFailureMessage(&failurepb.Failure{Message: activityErr.Error()})
	}
	return diagnostic
}

func ProcessMemoryEmbeddingActivity(ctx context.Context, input Input) (map[string]any, error) {
	application := app()
	if application == nil {
		return nil, fmt.Errorf("Go Core Worker is not configured")
	}
	return application.ProcessMemoryEmbeddingIntentAt(ctx, input.IntentID, input.MemoryID, input.Revision, input.ProviderEndpointID, input.ModelID)
}

func ProcessConversationSummaryActivity(ctx context.Context, input Input) (map[string]any, error) {
	application := app()
	if application == nil {
		return nil, fmt.Errorf("Go Core Worker is not configured")
	}
	return application.ProcessConversationSummaryIntent(ctx, input.IntentID, input.FluctlightID, input.ConversationID, input.SourceMessageID, input.SourceSequence, input.FromSequence, input.ToSequence, input.SourceDigest, input.SourceMessageRefs)
}

func EnsureCurrentDayScheduleActivity(ctx context.Context, input Input) (map[string]any, error) {
	application := app()
	if application == nil {
		return nil, fmt.Errorf("Go Core Worker is not configured")
	}
	return application.EnsureCurrentDaySchedule(ctx, input.FluctlightID)
}

func ProcessVisualIdentityActivity(ctx context.Context, input Input) (map[string]any, error) {
	stopHeartbeat := startVisualIdentityHeartbeat(ctx, input.SessionID)
	defer stopHeartbeat()
	activity.RecordHeartbeat(ctx, map[string]any{"session_id": input.SessionID, "phase": "loading"})
	application := app()
	if application == nil {
		return nil, fmt.Errorf("Go Core Worker is not configured")
	}
	ctx, input = prepareActivityLifecycleContext(ctx, input, "visual_identity")
	recordActivityLifecycle(application, ctx, input, "visual_identity", core.LifecycleTransitionActivityStarted, "running", "activity_started", nil)
	result, err := application.ProcessVisualIdentity(ctx, input.SessionID)
	if err != nil {
		recordActivityLifecycle(application, ctx, input, "visual_identity", core.LifecycleTransitionFailed, "failed", "visual_identity_activity_failed", err)
		return nil, err
	}
	transition := core.LifecycleTransitionCompletedNoop
	status := firstString(result["status"], "waiting")
	if status == "completed" {
		transition = core.LifecycleTransitionCompletedActionable
	} else if status == "waiting" || status == "queued" || status == "running" {
		transition = core.LifecycleTransitionQueued
	}
	recordActivityLifecycle(application, ctx, input, "visual_identity", transition, status, firstString(result["error_code"], firstString(result["stage"], "visual_identity_checkpoint")), nil)
	return result, nil
}

func startVisualIdentityHeartbeat(ctx context.Context, sessionID string) func() {
	if !activity.IsActivity(ctx) {
		return func() {}
	}
	heartbeatCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(visualIdentityHeartbeatEvery)
		defer ticker.Stop()
		for {
			select {
			case <-heartbeatCtx.Done():
				return
			case <-ticker.C:
				activity.RecordHeartbeat(heartbeatCtx, map[string]any{"session_id": sessionID, "phase": "in_flight"})
			}
		}
	}()
	return func() {
		cancel()
		<-done
	}
}

// StartWorkers starts exactly one worker per canonical task queue and returns a
// fatal-error channel. WakeUp and Reflection use an isolated critical lane;
// their registrations remain on the general lifecycle lane so histories that
// were scheduled there before the isolation can still complete. The caller
// must terminate the process when the fatal-error channel receives an error;
// otherwise a live-but-not-polling Worker would silently leave durable intents
// stuck in PostgreSQL.
func StartWorkers(ctx context.Context, temporalClient client.Client, logger *slog.Logger) ([]worker.Worker, <-chan error, error) {
	if logger == nil {
		logger = slog.Default()
	}
	queues := []string{LifecycleQueue, CriticalLifecycleQueue, MediaQueue, InteractionQueue, VisualIdentityQueue}
	workers := make([]worker.Worker, 0, len(queues))
	fatalErrors := make(chan error, len(queues))
	buildID := WorkerDeploymentBuildID()
	for _, queue := range queues {
		// General lifecycle work keeps two slots. WakeUp and Reflection receive
		// two additional slots on CriticalLifecycleQueue, so Provider-backed
		// Daily Review/Schedule/Summary work cannot consume their capacity.
		concurrency := 2
		if queue == CriticalLifecycleQueue {
			concurrency = 2
		}
		if queue == MediaQueue {
			concurrency = 1
		}
		if queue == VisualIdentityQueue {
			concurrency = 1
		}
		if queue == InteractionQueue {
			concurrency = 2
		}
		w := worker.New(temporalClient, queue, worker.Options{
			MaxConcurrentActivityExecutionSize: concurrency,
			DeploymentOptions: worker.DeploymentOptions{
				UseVersioning: true,
				Version: worker.WorkerDeploymentVersion{
					DeploymentName: WorkerDeploymentName,
					BuildID:        buildID,
				},
				DefaultVersioningBehavior: workflow.VersioningBehaviorAutoUpgrade,
			},
			OnFatalError: func(err error) {
				select {
				case fatalErrors <- err:
				default:
				}
			},
		})
		switch queue {
		case LifecycleQueue:
			// Keep WakeUp and Reflection registered here until every history that
			// predates CriticalLifecycleQueue has completed or been drained.
			w.RegisterWorkflow(WakeUpWorkflow)
			w.RegisterWorkflow(DailyReviewWorkflow)
			w.RegisterWorkflow(CurrentDayScheduleWorkflow)
			w.RegisterWorkflow(ReflectionWorkflow)
			w.RegisterWorkflow(IntentionTriggerWorkflow)
			w.RegisterWorkflow(MemoryEmbeddingWorkflow)
			w.RegisterWorkflow(ConversationSummaryWorkflow)
			w.RegisterWorkflow(PlatformControlWorkflow)
			w.RegisterWorkflow(VisualIdentityWorkflow)
			w.RegisterActivity(ProcessDailyReviewActivity)
			w.RegisterActivity(ProcessWakeUpActivity)
			w.RegisterActivity(EnsureCurrentDayScheduleActivity)
			w.RegisterActivity(ProcessReflectionActivity)
			w.RegisterActivity(ProcessIntentionTriggerActivity)
			w.RegisterActivity(ProcessMemoryEmbeddingActivity)
			w.RegisterActivity(ProcessConversationSummaryActivity)
			w.RegisterActivity(PlatformControlActivity)
			w.RegisterActivity(ProcessVisualIdentityActivity)
		case CriticalLifecycleQueue:
			w.RegisterWorkflow(WakeUpWorkflow)
			w.RegisterWorkflow(ReflectionWorkflow)
			w.RegisterActivity(ProcessWakeUpActivity)
			w.RegisterActivity(ProcessReflectionActivity)
		case VisualIdentityQueue:
			// New Visual Identity executions use an isolated capacity lane. Keep
			// the lifecycle registrations above until pre-isolation histories have
			// completed or continued-as-new on their original task queue.
			w.RegisterWorkflow(VisualIdentityWorkflow)
			w.RegisterActivity(ProcessVisualIdentityActivity)
		case MediaQueue:
			w.RegisterWorkflow(MediaWorkflow)
			w.RegisterActivity(ProcessMediaActivity)
		case InteractionQueue:
			w.RegisterWorkflow(AutonomyActionWorkflow)
			w.RegisterWorkflow(CapabilityActionWorkflow)
			w.RegisterWorkflow(CognitionProcessingWorkflow)
			w.RegisterActivity(ProcessAutonomyActionActivity)
			w.RegisterActivity(ProcessCapabilityActionActivity)
			w.RegisterActivity(ProcessCognitionActivity)
		}
		workers = append(workers, w)
	}
	for _, current := range workers {
		go func(current worker.Worker) {
			// Run(nil) owns the worker lifecycle and reports both startup and
			// fatal poller errors. Passing nil deliberately disables the SDK's
			// process-wide signal channel; shutdown is coordinated below.
			if err := current.Run(nil); err != nil && ctx.Err() == nil {
				select {
				case fatalErrors <- err:
				default:
				}
			}
		}(current)
	}
	go func() {
		<-ctx.Done()
		for _, current := range workers {
			current.Stop()
		}
	}()
	return workers, fatalErrors, nil
}

type Dispatcher struct {
	App     *core.App
	Client  client.Client
	Started map[string]struct{}
}

// ReconcileOnce reflects Temporal terminal states back into the durable intent
// ledger. Dispatch is intentionally at-least-once; this pass closes the crash
// window where Temporal accepted a start but PostgreSQL was not updated.
func (d *Dispatcher) ReconcileOnce(ctx context.Context, limit int) (int, error) {
	if limit < 1 {
		limit = 1
	}
	rows, err := d.App.DB.Pool().Query(ctx, reconcileIntentQuery, limit)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var intentID, workflowID, intentType, currentIntentStatus string
		var payload []byte
		var attemptCount int
		if err := rows.Scan(&intentID, &workflowID, &intentType, &payload, &currentIntentStatus, &attemptCount); err != nil {
			return count, err
		}
		var input Input
		if err := json.Unmarshal(payload, &input); err != nil {
			input = Input{IntentID: intentID}
		} else {
			input = hydrateLifecycleInput(intentID, intentType, payload, input)
		}
		if intentType == "cognition.processing" {
			var intentStatus, inboxStatus string
			var claimedAt *time.Time
			lookupErr := d.App.DB.Pool().QueryRow(ctx, `SELECT i.status,COALESCE(c.status,''),c.claimed_at FROM public.platform_workflow_intents i LEFT JOIN public.cognition_inbox c ON c.id=i.payload->>'inbox_id' WHERE i.intent_id=$1`, intentID).Scan(&intentStatus, &inboxStatus, &claimedAt)
			if lookupErr != nil {
				d.recordIntentLifecycle(ctx, input, intentType, workflowID, "", core.LifecycleTransitionFailed, "reconcile_dependency_lookup", "retry", "cognition_dependency_lookup_failed", attemptCount, lookupErr)
				continue
			}
			if intentStatus == "failed" && (inboxStatus == "pending" || (inboxStatus == "claimed" && (claimedAt == nil || time.Since(*claimedAt) >= 10*time.Minute))) {
				command, err := d.App.DB.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET status='retry',next_attempt_at=now(),started_at=NULL,completed_at=NULL,last_error=NULL WHERE intent_id=$1 AND status='failed'`, intentID)
				if err != nil {
					return count, err
				}
				if command.RowsAffected() != 1 {
					continue
				}
				d.recordIntentLifecycle(ctx, input, intentType, workflowID, "", core.LifecycleTransitionRetryScheduled, "workflow_reconcile", "retry", "cognition_retry_scheduled", attemptCount, nil)
				if d.Started != nil {
					delete(d.Started, intentID)
				}
				count++
				continue
			}
		}
		execution, describeErr := d.Client.DescribeWorkflowExecution(ctx, normalizedWorkflowID(workflowID), "")
		if describeErr != nil {
			// A just-started execution may not be visible immediately. Leave the
			// intent untouched and let the next pass retry the lookup.
			d.recordIntentLifecycle(ctx, input, intentType, workflowID, "", core.LifecycleTransitionFailed, "workflow_describe", "retry", "workflow_describe_failed", attemptCount, describeErr)
			continue
		}
		if execution == nil || execution.WorkflowExecutionInfo == nil {
			d.recordIntentLifecycle(ctx, input, intentType, workflowID, "", core.LifecycleTransitionFailed, "workflow_describe", "retry", "workflow_describe_empty", attemptCount, errors.New("workflow_describe_empty"))
			continue
		}
		runID := ""
		if execution.WorkflowExecutionInfo.GetExecution() != nil {
			runID = execution.WorkflowExecutionInfo.GetExecution().GetRunId()
		}
		status := execution.WorkflowExecutionInfo.GetStatus()
		if status == enumspb.WORKFLOW_EXECUTION_STATUS_RUNNING {
			if currentIntentStatus != "started" {
				command, err := d.App.DB.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET status='started',started_at=COALESCE(started_at,now()),attempt_count=attempt_count+1,last_error=NULL WHERE intent_id=$1 AND (status IS NULL OR status IN ('pending','retry'))`, intentID)
				if err != nil {
					return count, err
				}
				if command.RowsAffected() == 1 {
					d.recordIntentLifecycle(ctx, input, intentType, workflowID, runID, core.LifecycleTransitionAlreadyRunning, "workflow_reconcile", "started", "workflow_running_ledger_repaired", attemptCount+1, nil)
					if d.Started != nil {
						d.Started[intentID] = struct{}{}
					}
					count++
				}
			}
			continue
		}
		intentStatus := strings.ToLower(status.String())
		if status == enumspb.WORKFLOW_EXECUTION_STATUS_COMPLETED {
			intentStatus = "completed"
		} else if status == enumspb.WORKFLOW_EXECUTION_STATUS_CANCELED {
			intentStatus = "cancelled"
		} else if status == enumspb.WORKFLOW_EXECUTION_STATUS_FAILED || status == enumspb.WORKFLOW_EXECUTION_STATUS_TIMED_OUT || status == enumspb.WORKFLOW_EXECUTION_STATUS_TERMINATED {
			intentStatus = "failed"
		}
		terminalFailure := ""
		if intentStatus == "failed" {
			var historyErr error
			terminalFailure, historyErr = d.terminalFailureReason(ctx, normalizedWorkflowID(workflowID), runID)
			if historyErr != nil {
				// Keep the intent eligible for reconciliation. A transient history
				// read failure must not make us commit the generic fallback forever.
				slog.Default().Warn("Go Worker could not read terminal workflow failure", "intent_id", intentID, "workflow_id", workflowID, "correlation_id", input.CorrelationID, "error_type", fmt.Sprintf("%T", historyErr))
				d.recordIntentLifecycle(ctx, input, intentType, workflowID, runID, core.LifecycleTransitionFailed, "workflow_history", "retry", "workflow_history_read_failed", attemptCount, historyErr)
				continue
			}
		}
		if intentType == "wake_up.current" {
			var fluctlightStatus string
			lookupErr := d.App.DB.Pool().QueryRow(ctx, `SELECT status FROM public.fluctlights WHERE id=(SELECT payload->>'fluctlight_id' FROM public.platform_workflow_intents WHERE intent_id=$1)`, intentID).Scan(&fluctlightStatus)
			if lookupErr != nil {
				d.recordIntentLifecycle(ctx, input, intentType, workflowID, runID, core.LifecycleTransitionFailed, "reconcile_dependency_lookup", "retry", "wakeup_dependency_lookup_failed", attemptCount, lookupErr)
				continue
			}
			if wakeUpIntentShouldRetry(fluctlightStatus, intentStatus) {
				command, err := d.App.DB.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET status='retry',next_attempt_at=now()+interval '5 minutes',started_at=NULL,completed_at=NULL,last_error=COALESCE(NULLIF($2,''),last_error,'wake_up_workflow_terminal') WHERE intent_id=$1 AND status IN ('pending','started','failed')`, intentID, terminalFailure)
				if err != nil {
					return count, err
				}
				if command.RowsAffected() != 1 {
					continue
				}
				d.recordIntentLifecycle(ctx, input, intentType, workflowID, runID, core.LifecycleTransitionRetryScheduled, "workflow_reconcile", "retry", "wake_up_workflow_terminal", attemptCount, errors.New(firstString(terminalFailure, "wake_up_workflow_terminal")))
				slog.Default().Warn("Go Worker wake-up workflow requeued after terminal execution", "intent_id", intentID, "workflow_id", workflowID, "temporal_status", intentStatus, "fluctlight_status", fluctlightStatus, "next_attempt", "5m")
				if d.Started != nil {
					delete(d.Started, intentID)
				}
				count++
				continue
			}
		}
		if (intentType == "autonomy.action" || intentType == "capability.action") && intentStatus == "failed" {
			var actionStatus string
			lookupErr := d.App.DB.Pool().QueryRow(ctx, `SELECT status FROM public.autonomy_actions WHERE id=(SELECT payload->>'action_id' FROM public.platform_workflow_intents WHERE intent_id=$1)`, intentID).Scan(&actionStatus)
			if lookupErr != nil {
				d.recordIntentLifecycle(ctx, input, intentType, workflowID, runID, core.LifecycleTransitionFailed, "reconcile_dependency_lookup", "retry", "action_dependency_lookup_failed", attemptCount, lookupErr)
				continue
			}
			if actionIntentShouldRetry(actionStatus) {
				command, err := d.App.DB.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET status='retry',next_attempt_at=now()+interval '5 seconds',started_at=NULL,completed_at=NULL,last_error=COALESCE(NULLIF($2,''),last_error,'action_workflow_terminal') WHERE intent_id=$1 AND status='failed'`, intentID, terminalFailure)
				if err != nil {
					return count, err
				}
				if command.RowsAffected() != 1 {
					continue
				}
				d.recordIntentLifecycle(ctx, input, intentType, workflowID, runID, core.LifecycleTransitionRetryScheduled, "workflow_reconcile", "retry", "action_workflow_terminal", attemptCount, errors.New(firstString(terminalFailure, "action_workflow_terminal")))
				if d.Started != nil {
					delete(d.Started, intentID)
				}
				count++
				continue
			}
		}
		if intentType == "reflection.run" && intentStatus == "failed" {
			var fluctlightID, fluctlightStatus string
			var attemptCount int
			var hasEvidence bool
			err := d.App.DB.Pool().QueryRow(ctx, `
				SELECT f.id,f.status,i.attempt_count,
					EXISTS(
						SELECT 1
						FROM public.cognition_inbox AS c
						WHERE c.fluctlight_id=f.id
						  AND c.status='processed'
						  AND c.sequence>COALESCE((
							SELECT watermark
							FROM public.cognition_reflection_windows
							WHERE fluctlight_id=f.id
						  ),0)
					)
				FROM public.platform_workflow_intents AS i
				JOIN public.fluctlights AS f ON f.id=i.payload->>'fluctlight_id'
				WHERE i.intent_id=$1`, intentID).Scan(&fluctlightID, &fluctlightStatus, &attemptCount, &hasEvidence)
			if err != nil {
				slog.Default().Warn("Go Worker Reflection retry eligibility lookup failed",
					"intent_id", intentID,
					"workflow_id", workflowID,
					"error_type", fmt.Sprintf("%T", err),
				)
				d.recordIntentLifecycle(ctx, input, intentType, workflowID, runID, core.LifecycleTransitionFailed, "reconcile_dependency_lookup", "retry", "reflection_dependency_lookup_failed", attemptCount, err)
				continue
			}
			if reflectionIntentShouldRetry(fluctlightStatus, hasEvidence, attemptCount) {
				command, err := d.App.DB.Pool().Exec(ctx, `
					UPDATE public.platform_workflow_intents
					SET status='retry',
						next_attempt_at=now()+($2 * interval '1 second'),
						started_at=NULL,
						completed_at=NULL,
						last_error=COALESCE(NULLIF($3,''),'reflection_workflow_terminal')
					WHERE intent_id=$1 AND status IN ('started','failed')`,
					intentID, int64(reflectionRetryDelay/time.Second), terminalFailure,
				)
				if err != nil {
					return count, err
				}
				if command.RowsAffected() != 1 {
					continue
				}
				d.App.RecordLifecycleDiagnosticBestEffort(ctx, core.LifecycleDiagnostic{
					Surface: "reflection", Transition: core.LifecycleTransitionRetryScheduled, Severity: "warn",
					FluctlightID: fluctlightID, CorrelationID: input.CorrelationID, CausationID: input.CausationID,
					IntentID: intentID, WorkflowID: normalizedWorkflowID(workflowID), RunID: runID,
					Stage: "workflow_reconcile", Status: "retry",
					ReasonCode: "reflection_workflow_terminal", ErrorCode: "reflection_workflow_terminal",
					Retryable: true, Attempt: attemptCount, MaxAttempts: reflectionMaximumAttempts,
					NextDueAt: time.Now().UTC().Add(reflectionRetryDelay), SafeCause: terminalFailure,
				})
				if d.Started != nil {
					delete(d.Started, intentID)
				}
				count++
				continue
			}
			command, err := d.App.DB.Pool().Exec(ctx, `
				UPDATE public.platform_workflow_intents
				SET status='dead_letter',
					completed_at=COALESCE(completed_at,now()),
					last_error=COALESCE(NULLIF($2,''),'reflection_retry_exhausted')
				WHERE intent_id=$1 AND status IN ('started','failed')`, intentID, terminalFailure)
			if err != nil {
				return count, err
			}
			if command.RowsAffected() == 1 {
				d.App.RecordLifecycleDiagnosticBestEffort(ctx, core.LifecycleDiagnostic{
					Surface: "reflection", Transition: core.LifecycleTransitionFailed, Severity: "error",
					FluctlightID: fluctlightID, CorrelationID: input.CorrelationID, CausationID: input.CausationID,
					IntentID: intentID, WorkflowID: normalizedWorkflowID(workflowID), RunID: runID,
					Stage: "workflow_reconcile", Status: "dead_letter",
					ReasonCode: "reflection_retry_exhausted", ErrorCode: "reflection_retry_exhausted",
					Retryable: false, Attempt: min(attemptCount, reflectionMaximumAttempts), MaxAttempts: reflectionMaximumAttempts,
					SafeCause: terminalFailure,
				})
				if d.Started != nil {
					delete(d.Started, intentID)
				}
				count++
			}
			continue
		}
		if intentType == "visual_identity.initialize" && intentStatus == "failed" {
			var sessionStatus string
			lookupErr := d.App.DB.Pool().QueryRow(ctx, `SELECT status FROM public.fluctlight_visual_identity_sessions WHERE id=(SELECT payload->>'session_id' FROM public.platform_workflow_intents WHERE intent_id=$1)`, intentID).Scan(&sessionStatus)
			if lookupErr != nil {
				d.recordIntentLifecycle(ctx, input, intentType, workflowID, runID, core.LifecycleTransitionFailed, "reconcile_dependency_lookup", "retry", "visual_identity_dependency_lookup_failed", attemptCount, lookupErr)
				continue
			}
			if sessionStatus == "queued" || sessionStatus == "running" {
				command, err := d.App.DB.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET status='retry',next_attempt_at=now()+($2 * interval '1 second'),started_at=NULL,completed_at=NULL,last_error=COALESCE(NULLIF($3,''),'visual_identity_workflow_terminal') WHERE intent_id=$1 AND status IN ('pending','retry','started')`, intentID, int64(visualIdentityRetryDelay/time.Second), terminalFailure)
				if err != nil {
					return count, err
				}
				if command.RowsAffected() != 1 {
					continue
				}
				slog.Default().Warn("Go Worker visual identity workflow requeued after terminal failure", "intent_id", intentID, "workflow_id", workflowID, "temporal_status", intentStatus, "session_status", sessionStatus, "next_attempt", visualIdentityRetryDelay, "failure", terminalFailure)
				if d.Started != nil {
					delete(d.Started, intentID)
				}
				count++
				continue
			}
		}
		if intentType == "media.generation" && intentStatus == "failed" {
			// Activity code may not run on a timeout, cancellation, or worker
			// crash. Close the product-facing media target here as the final
			// reconciliation fallback so the retry action remains available.
			command, err := d.App.DB.Pool().Exec(ctx, `UPDATE public.media_intents SET status='failed',revision=revision+1 WHERE id=(SELECT payload->>'intent_id' FROM public.platform_workflow_intents WHERE intent_id=$1) AND status IN ('pending','running')`, intentID)
			if err != nil {
				return count, err
			}
			if command.RowsAffected() == 0 {
				var mediaStatus string
				if err := d.App.DB.Pool().QueryRow(ctx, `SELECT status FROM public.media_intents WHERE id=(SELECT payload->>'intent_id' FROM public.platform_workflow_intents WHERE intent_id=$1)`, intentID).Scan(&mediaStatus); err != nil {
					return count, fmt.Errorf("verify terminal media settlement %s: %w", intentID, err)
				}
				if mediaStatus == "pending" || mediaStatus == "running" {
					return count, fmt.Errorf("verify terminal media settlement %s: media_terminal_status_not_written", intentID)
				}
			}
		}
		command, err := d.App.DB.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET status=$2::varchar,completed_at=COALESCE(completed_at,now()),last_error=CASE WHEN $2::varchar='failed' THEN COALESCE(NULLIF(last_error,''),NULLIF($3,''),'workflow_terminal_failure') ELSE last_error END WHERE intent_id=$1 AND (status IS NULL OR status IN ('pending','started','cancel_requested','retry','failed'))`, intentID, intentStatus, terminalFailure)
		if err != nil {
			return count, err
		}
		if command.RowsAffected() != 1 {
			continue
		}
		if intentStatus == "failed" {
			d.recordIntentLifecycle(ctx, input, intentType, workflowID, runID, core.LifecycleTransitionFailed, "workflow_reconcile", intentStatus, "workflow_terminal_failed", attemptCount, errors.New(firstString(terminalFailure, "workflow_terminal_failure")))
		} else if intentStatus == "cancelled" {
			d.recordIntentLifecycle(ctx, input, intentType, workflowID, runID, core.LifecycleTransitionCancelled, "workflow_reconcile", intentStatus, "workflow_cancelled", attemptCount, nil)
		}
		if d.Started != nil {
			delete(d.Started, intentID)
		}
		count++
	}
	return count, rows.Err()
}

// terminalFailureReason reads the terminal workflow event because Temporal's
// visibility response intentionally omits the failure payload. The reason is
// used only as a bounded diagnostic string; stack traces and encoded details
// are never copied into the product-facing error.
func (d *Dispatcher) terminalFailureReason(ctx context.Context, workflowID, runID string) (string, error) {
	if d == nil || d.Client == nil || strings.TrimSpace(workflowID) == "" {
		return "", nil
	}
	iter := d.Client.GetWorkflowHistory(ctx, workflowID, runID, false, enumspb.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT)
	last := ""
	for iter.HasNext() {
		event, err := iter.Next()
		if err != nil {
			return "", fmt.Errorf("read workflow history: %w", err)
		}
		if event == nil {
			continue
		}
		if reason := temporalTerminalFailureMessage(event); reason != "" {
			last = reason
		}
	}
	return last, nil
}

func temporalTerminalFailureMessage(event *historypb.HistoryEvent) string {
	if event == nil || event.GetEventType() != enumspb.EVENT_TYPE_WORKFLOW_EXECUTION_FAILED {
		return ""
	}
	attrs := event.GetWorkflowExecutionFailedEventAttributes()
	if attrs == nil {
		return ""
	}
	return boundedTemporalFailureMessage(attrs.GetFailure())
}

func boundedTemporalFailureMessage(failure *failurepb.Failure) string {
	if failure == nil {
		return ""
	}
	// Temporal wraps Activity failures several times. The leaf is the useful
	// application error (for example the provider/placeholder failure), while
	// the outer message is usually only "activity task failed".
	message := ""
	for current := failure; current != nil; current = current.GetCause() {
		if value := strings.TrimSpace(current.GetMessage()); value != "" {
			message = value
		}
	}
	message = strings.Join(strings.Fields(message), " ")
	if runes := []rune(message); len(runes) > 1024 {
		message = string(runes[:1024])
	}
	return message
}

func wakeUpIntentShouldRetry(fluctlightStatus, workflowStatus string) bool {
	if fluctlightStatus != "active" && fluctlightStatus != "paused" {
		return false
	}
	return workflowStatus == "failed"
}

func actionIntentShouldRetry(actionStatus string) bool {
	switch strings.TrimSpace(actionStatus) {
	case "frozen", "running":
		// A terminal Temporal failure before the action's own settlement boundary
		// leaves the action executable. Requeue the intent so a worker restart or
		// transient provider/database error cannot strand the capability forever.
		return true
	default:
		return false
	}
}

func reflectionIntentShouldRetry(fluctlightStatus string, hasEvidence bool, attemptCount int) bool {
	return fluctlightStatus == "active" && hasEvidence && attemptCount < reflectionMaximumAttempts
}

// workflowIDReusePolicy gives wake-up recovery the reuse semantics required by
// its stable workflow ID while keeping one-shot intents protected from
// accidental duplicate starts.
func workflowIDReusePolicy(intentType string) enumspb.WorkflowIdReusePolicy {
	if intentType == "wake_up.current" {
		// Reconciliation deliberately retries both failed and completed wake-up
		// executions for live Fluctlights. ALLOW_DUPLICATE is therefore required
		// for a stable wake-up ID; the cognition_wakeups cycle key keeps each
		// cycle idempotent even when a terminal execution is replayed.
		return enumspb.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE
	}
	if intentType == "visual_identity.initialize" {
		// Reconciliation retries only a terminal failed Visual Identity workflow.
		// The stable workflow ID must therefore admit a new run after failure but
		// must not reopen a successfully completed identity workflow.
		return enumspb.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE_FAILED_ONLY
	}
	if intentType == "reflection.run" {
		return enumspb.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE_FAILED_ONLY
	}
	return enumspb.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE
}

func workflowStartOptions(workflowID, taskQueue, intentType string) client.StartWorkflowOptions {
	return client.StartWorkflowOptions{
		ID:                                       workflowID,
		TaskQueue:                                taskQueue,
		WorkflowIDReusePolicy:                    workflowIDReusePolicy(intentType),
		WorkflowExecutionErrorWhenAlreadyStarted: true,
	}
}

func (d *Dispatcher) DispatchOnce(ctx context.Context, limit int) (int, error) {
	if limit < 1 {
		limit = 1
	}
	if d.Started == nil {
		d.Started = make(map[string]struct{})
	}
	// PostgreSQL status is authoritative. Do not exclude IDs from the
	// in-memory Started map: a prior dispatch can have been reset to pending by
	// reconciliation/retry while this process still retains the old map entry.
	// Such an intent must be eligible for dispatch again.
	// Rank one candidate per intent class before admitting a second item from
	// any class. Priority still orders each fairness round, but one large media
	// or visual backlog cannot consume the entire dispatcher LIMIT.
	query := fmt.Sprintf(`
		WITH eligible AS (
			SELECT intent_id,workflow_id,task_queue,intent_type,payload,COALESCE(attempt_count,0) AS attempt_count,created_at,
				%s AS intent_priority,
				ROW_NUMBER() OVER (
					PARTITION BY intent_type
					ORDER BY created_at,intent_id
				) AS class_rank
			FROM public.platform_workflow_intents
			WHERE (status IS NULL OR status IN ('pending','retry'))
			  AND (next_attempt_at IS NULL OR next_attempt_at <= now())
		)
		SELECT intent_id,workflow_id,task_queue,intent_type,payload,attempt_count
		FROM eligible
		ORDER BY class_rank,intent_priority,created_at,intent_id
		LIMIT $1`, dispatcherIntentOrder)
	rows, err := d.App.DB.Pool().Query(ctx, query, limit)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		if count >= limit {
			break
		}
		var intentID, workflowID, queue, intentType string
		var payload []byte
		var attemptCount int
		if err := rows.Scan(&intentID, &workflowID, &queue, &intentType, &payload, &attemptCount); err != nil {
			return count, err
		}
		var input Input
		if err := json.Unmarshal(payload, &input); err != nil {
			reason := boundedTemporalFailureMessage(&failurepb.Failure{Message: "workflow payload invalid: " + err.Error()})
			command, writeErr := d.App.DB.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET status='failed',last_error=$2,attempt_count=attempt_count+1,completed_at=now() WHERE intent_id=$1 AND status IN ('pending','retry')`, intentID, reason)
			if writeErr != nil {
				return count, fmt.Errorf("settle invalid workflow payload %s: %w", intentID, writeErr)
			}
			if intentType == "media.generation" {
				if _, writeErr := d.App.DB.Pool().Exec(ctx, `UPDATE public.media_intents SET status='failed',revision=revision+1 WHERE id=(SELECT payload->>'intent_id' FROM public.platform_workflow_intents WHERE intent_id=$1) AND status IN ('pending','running')`, intentID); writeErr != nil {
					return count, fmt.Errorf("settle invalid media workflow payload %s: %w", intentID, writeErr)
				}
			}
			if command.RowsAffected() == 1 {
				d.recordIntentLifecycle(ctx, Input{IntentID: intentID}, intentType, workflowID, "", core.LifecycleTransitionFailed, "payload_decode", "failed", "workflow_payload_invalid", attemptCount+1, err)
			}
			slog.Default().Warn("Go Worker intent payload invalid; marked failed", "intent_id", intentID, "error", err)
			continue
		}
		input = hydrateLifecycleInput(intentID, intentType, payload, input)
		if intentType == "cognition.processing" {
			var claimed bool
			if err := d.App.DB.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.cognition_inbox WHERE id=$1 AND status='claimed' AND claimed_at > now()-interval '10 minutes')`, input.InboxID).Scan(&claimed); err != nil {
				return count, err
			}
			if claimed {
				// The synchronous NDJSON responder owns this fact while it is
				// generating the visible reply. Its atomic claim prevents a second
				// Worker execution; dispatch after the claim expires is the durable
				// crash-recovery path.
				continue
			}
		}
		workflowFn := any(nil)
		taskQueue := queue
		// Every post-cutover execution receives a stable Go namespace. This
		// fences closed/active executions created by the retired runtime while
		// preserving deterministic replay for this intent after restarts.
		goWorkflowID := normalizedWorkflowID(workflowID)
		switch intentType {
		case "visual_identity.initialize":
			workflowFn = VisualIdentityWorkflow
			taskQueue = VisualIdentityQueue
		case "wake_up.current":
			workflowFn = WakeUpWorkflow
			taskQueue = CriticalLifecycleQueue
		case "daily_review.current_day":
			workflowFn = DailyReviewWorkflow
			taskQueue = LifecycleQueue
		case "media.generation":
			var exists bool
			if err := d.App.DB.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.media_intents WHERE id=$1)`, input.IntentID).Scan(&exists); err != nil {
				return count, err
			}
			if !exists {
				command, writeErr := d.App.DB.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET status='failed',last_error='media_intent_not_found',attempt_count=attempt_count+1,completed_at=now() WHERE intent_id=$1 AND (status IS NULL OR status IN ('pending','retry'))`, intentID)
				if writeErr != nil {
					return count, fmt.Errorf("settle missing media intent %s: %w", intentID, writeErr)
				}
				if command.RowsAffected() == 1 {
					d.recordIntentLifecycle(ctx, input, intentType, goWorkflowID, "", core.LifecycleTransitionFailed, "dependency_lookup", "failed", "media_intent_not_found", attemptCount+1, errors.New("media_intent_not_found"))
				}
				continue
			}
			workflowFn = MediaWorkflow
			taskQueue = MediaQueue
		case "schedule.current_day":
			workflowFn = CurrentDayScheduleWorkflow
			taskQueue = LifecycleQueue
		case "autonomy.action":
			workflowFn = AutonomyActionWorkflow
			taskQueue = InteractionQueue
		case "capability.action":
			workflowFn = CapabilityActionWorkflow
			taskQueue = InteractionQueue
		case "reflection.run":
			workflowFn = ReflectionWorkflow
			taskQueue = CriticalLifecycleQueue
		case "intention.trigger":
			workflowFn = IntentionTriggerWorkflow
			taskQueue = LifecycleQueue
		case "memory.embedding":
			workflowFn = MemoryEmbeddingWorkflow
			taskQueue = LifecycleQueue
		case "conversation.summary":
			workflowFn = ConversationSummaryWorkflow
			taskQueue = LifecycleQueue
		case "cognition.processing":
			workflowFn = CognitionProcessingWorkflow
			taskQueue = InteractionQueue
		case "platform.control":
			workflowFn = PlatformControlWorkflow
			taskQueue = LifecycleQueue
		default:
			slog.Default().Warn("Go Worker intent type unsupported; leaving pending", "intent_id", intentID, "intent_type", intentType)
			continue
		}
		d.recordIntentLifecycle(ctx, input, intentType, goWorkflowID, "", core.LifecycleTransitionQueued, "dispatcher", "queued", "dispatcher_selected", attemptCount+1, nil)
		execution, err := d.Client.ExecuteWorkflow(ctx, workflowStartOptions(goWorkflowID, taskQueue, intentType), workflowFn, input)
		if err != nil && !temporal.IsWorkflowExecutionAlreadyStartedError(err) {
			slog.Default().Warn("Go Worker workflow start failed", "intent_id", intentID, "error", err)
			command, writeErr := d.App.DB.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET status='retry',last_error=$2,attempt_count=attempt_count+1,next_attempt_at=now()+interval '5 seconds' WHERE intent_id=$1 AND (status IS NULL OR status IN ('pending','retry'))`, intentID, boundedTemporalFailureMessage(&failurepb.Failure{Message: err.Error()}))
			if writeErr != nil {
				return count, fmt.Errorf("settle workflow start failure %s: %w", intentID, writeErr)
			}
			if command.RowsAffected() != 1 {
				currentStatus, statusErr := d.readWorkflowIntentStatus(ctx, intentID)
				if statusErr != nil {
					return count, fmt.Errorf("read workflow start failure settlement %s: %w", intentID, statusErr)
				}
				if dispatchIntentRaceSettled(currentStatus) {
					continue
				}
				return count, fmt.Errorf("settle workflow start failure %s: workflow_start_retry_not_written", intentID)
			}
			d.recordIntentLifecycle(ctx, input, intentType, goWorkflowID, "", core.LifecycleTransitionRetryScheduled, "workflow_start", "retry", "workflow_start_failed", attemptCount+1, err)
			continue
		}
		alreadyStarted := temporal.IsWorkflowExecutionAlreadyStartedError(err)
		runID := workflowRunIdentity(execution, err)
		command, statusErr := d.App.DB.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET status='started',started_at=COALESCE(started_at,now()),attempt_count=attempt_count+1,last_error=NULL WHERE intent_id=$1 AND (status IS NULL OR status IN ('pending','retry'))`, intentID)
		if statusErr != nil || command.RowsAffected() != 1 {
			// Temporal already accepted the start. Leave the durable row visible
			// to the next reconciliation pass rather than hiding a DB failure in
			// the in-memory Started set.
			if statusErr != nil {
				slog.Default().Warn("Go Worker intent status update failed after Temporal start", "intent_id", intentID, "error", statusErr)
			} else {
				slog.Default().Warn("Go Worker intent status changed before Temporal start settlement", "intent_id", intentID, "workflow_id", goWorkflowID)
			}
			if statusErr != nil {
				d.recordIntentLifecycle(ctx, input, intentType, goWorkflowID, runID, core.LifecycleTransitionFailed, "workflow_start_settlement", "retry", "workflow_start_settlement_failed", attemptCount+1, statusErr)
				return count, fmt.Errorf("settle workflow start %s: %w", intentID, statusErr)
			}
			currentStatus, readErr := d.readWorkflowIntentStatus(ctx, intentID)
			if readErr != nil {
				return count, fmt.Errorf("read workflow start settlement %s: %w", intentID, readErr)
			}
			if dispatchIntentRaceSettled(currentStatus) {
				if currentStatus == "started" {
					d.recordIntentLifecycle(ctx, input, intentType, goWorkflowID, runID, core.LifecycleTransitionAlreadyRunning, "workflow_start_settlement", "started", "workflow_start_race_reconciled", attemptCount+1, nil)
					d.Started[intentID] = struct{}{}
					count++
				}
				continue
			}
			settlementErr := errors.New("workflow_start_status_not_written")
			d.recordIntentLifecycle(ctx, input, intentType, goWorkflowID, runID, core.LifecycleTransitionFailed, "workflow_start_settlement", "retry", "workflow_start_settlement_failed", attemptCount+1, settlementErr)
			return count, fmt.Errorf("settle workflow start %s: %w", intentID, settlementErr)
		}
		if alreadyStarted {
			slog.Default().Info("Go Worker workflow already running; intent ledger reconciled", "intent_id", intentID, "workflow_id", goWorkflowID, "workflow_type", intentType)
			d.recordIntentLifecycle(ctx, input, intentType, goWorkflowID, runID, core.LifecycleTransitionAlreadyRunning, "workflow_start", "started", "workflow_already_running", attemptCount+1, nil)
		} else {
			slog.Default().Info("Go Worker workflow dispatched", "intent_id", intentID, "workflow_id", goWorkflowID, "workflow_type", intentType)
			d.recordIntentLifecycle(ctx, input, intentType, goWorkflowID, runID, core.LifecycleTransitionDispatched, "workflow_start", "started", "workflow_dispatched", attemptCount+1, nil)
		}
		d.Started[intentID] = struct{}{}
		count++
	}
	return count, rows.Err()
}

func normalizedWorkflowID(workflowID string) string {
	if strings.HasPrefix(workflowID, "go:") {
		return workflowID
	}
	return "go:" + workflowID
}

func workflowRunIdentity(execution client.WorkflowRun, startErr error) string {
	if execution != nil && strings.TrimSpace(execution.GetRunID()) != "" {
		return strings.TrimSpace(execution.GetRunID())
	}
	var alreadyStartedErr *serviceerror.WorkflowExecutionAlreadyStarted
	if errors.As(startErr, &alreadyStartedErr) {
		return strings.TrimSpace(alreadyStartedErr.RunId)
	}
	return ""
}

func (d *Dispatcher) readWorkflowIntentStatus(ctx context.Context, intentID string) (string, error) {
	var status string
	if err := d.App.DB.Pool().QueryRow(ctx, `SELECT COALESCE(status,'pending') FROM public.platform_workflow_intents WHERE intent_id=$1`, intentID).Scan(&status); err != nil {
		return "", err
	}
	return strings.TrimSpace(status), nil
}

func dispatchIntentRaceSettled(status string) bool {
	status = strings.TrimSpace(status)
	return status != "" && status != "pending" && status != "retry"
}

func inputMap(payload []byte) map[string]any {
	var result map[string]any
	if err := json.Unmarshal(payload, &result); err != nil {
		return map[string]any{}
	}
	return result
}
func stringValue(value any) string {
	if result, ok := value.(string); ok {
		return result
	}
	return ""
}

func firstString(value any, fallback string) string {
	if result := strings.TrimSpace(stringValue(value)); result != "" {
		return result
	}
	return strings.TrimSpace(fallback)
}

func lifecycleCorrelationForIntent(intentID, surface string, input Input) string {
	if correlationID := strings.TrimSpace(input.CorrelationID); correlationID != "" {
		return correlationID
	}
	if surface == "wake_up" && strings.TrimSpace(input.FluctlightID) != "" && input.Cycle >= 0 {
		return fmt.Sprintf("wake_up:%s:cycle:%d", strings.TrimSpace(input.FluctlightID), input.Cycle)
	}
	if intentID = strings.TrimSpace(intentID); intentID != "" {
		return intentID
	}
	if input.IntentID = strings.TrimSpace(input.IntentID); input.IntentID != "" {
		return input.IntentID
	}
	if fluctlightID := strings.TrimSpace(input.FluctlightID); fluctlightID != "" {
		return strings.TrimSpace(surface) + ":" + fluctlightID
	}
	return strings.TrimSpace(surface) + ":unknown"
}

func lifecycleSurfaceForIntent(intentType string) string {
	switch intentType {
	case "wake_up.current":
		return "wake_up"
	case "reflection.run":
		return "reflection"
	case "visual_identity.initialize":
		return "visual_identity"
	case "media.generation":
		return "media"
	case "daily_review.current_day":
		return "daily_review"
	case "schedule.current_day":
		return "schedule"
	case "autonomy.action":
		return "autonomy"
	case "capability.action":
		return "capability"
	case "cognition.processing":
		return "cognition"
	case "memory.embedding":
		return "memory"
	case "conversation.summary":
		return "conversation_summary"
	case "intention.trigger":
		return "intention"
	case "platform.control":
		return "platform"
	default:
		return "workflow"
	}
}

func hydrateLifecycleInput(intentID, intentType string, payload []byte, input Input) Input {
	values := inputMap(payload)
	if input.IntentID == "" {
		input.IntentID = intentID
	}
	if input.ActionID == "" {
		input.ActionID = stringValue(values["action_id"])
	}
	if input.MemoryID == "" {
		input.MemoryID = stringValue(values["memory_id"])
	}
	if input.FluctlightID == "" {
		input.FluctlightID = stringValue(values["fluctlight_id"])
	}
	if input.CorrelationID == "" {
		input.CorrelationID = stringValue(values["correlation_id"])
	}
	if input.CausationID == "" {
		input.CausationID = stringValue(values["causation_id"])
	}
	if input.CausationID == "" {
		for _, key := range []string{"source_fact_id", "action_id", "inbox_id", "wake_up_id"} {
			if value := strings.TrimSpace(stringValue(values[key])); value != "" {
				input.CausationID = value
				break
			}
		}
	}
	input.CorrelationID = lifecycleCorrelationForIntent(intentID, lifecycleSurfaceForIntent(intentType), input)
	return input
}

func (d *Dispatcher) recordIntentLifecycle(ctx context.Context, input Input, intentType, workflowID, runID string, transition core.LifecycleTransition, stage, status, reason string, attempt int, lifecycleErr error) {
	if d == nil || d.App == nil {
		return
	}
	surface := lifecycleSurfaceForIntent(intentType)
	input.CorrelationID = lifecycleCorrelationForIntent(input.IntentID, surface, input)
	if (stage == "workflow_describe" || stage == "reconcile_dependency_lookup") && !shouldRecordWorkflowDiagnostic(input.CorrelationID+":"+stage+":"+reason+":"+fmt.Sprint(attempt), time.Now().UTC()) {
		return
	}
	diagnostic := core.LifecycleDiagnostic{
		Surface: surface, Transition: transition,
		FluctlightID: input.FluctlightID, CorrelationID: input.CorrelationID,
		CausationID: input.CausationID, IntentID: input.IntentID,
		WorkflowID: normalizedWorkflowID(workflowID), RunID: runID,
		Stage: stage, Status: status, ReasonCode: reason, Attempt: max(attempt, 0),
	}
	if lifecycleErr != nil {
		diagnostic.Severity = "error"
		diagnostic.ErrorCategory = "workflow"
		diagnostic.ErrorCode = reason
		diagnostic.Retryable = status == "retry" || status == "queued" || status == "started"
		diagnostic.SafeCause = boundedTemporalFailureMessage(&failurepb.Failure{Message: lifecycleErr.Error()})
	}
	d.App.RecordLifecycleDiagnosticBestEffort(ctx, diagnostic)
}

func shouldRecordWorkflowDiagnostic(key string, now time.Time) bool {
	workflowDiagnosticSampleState.Lock()
	defer workflowDiagnosticSampleState.Unlock()
	last := workflowDiagnosticSampleState.last[key]
	if !last.IsZero() && now.Sub(last) < time.Minute {
		return false
	}
	workflowDiagnosticSampleState.last[key] = now
	return true
}
