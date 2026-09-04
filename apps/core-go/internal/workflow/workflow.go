package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
	enumspb "go.temporal.io/api/enums/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

const (
	LifecycleQueue               = "lifecycle"
	MediaQueue                   = "media"
	InteractionQueue             = "interaction"
	WorkerDeploymentName         = "fluctlight"
	DefaultWorkerBuildID         = "platform-v1"
	defaultWakeUpIntervalSeconds = 30 * 60
	minWakeUpIntervalSeconds     = 5 * 60
	maxWakeUpIntervalSeconds     = 24 * 60 * 60
	reconcileIntentQuery         = `SELECT intent_id,workflow_id,intent_type FROM public.platform_workflow_intents WHERE status IN ('pending','started','cancel_requested') OR (status='retry' AND (next_attempt_at IS NULL OR next_attempt_at <= now())) ORDER BY started_at NULLS LAST,created_at LIMIT $1`
)

var runtime struct {
	sync.RWMutex
	app *core.App
}

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
	IntentID     string `json:"intent_id"`
	FluctlightID string `json:"fluctlight_id"`
	LocalDate    string `json:"local_date"`
	Cycle        int    `json:"cycle"`
	ActionID     string `json:"action_id"`
	MemoryID     string `json:"memory_id"`
	Revision     int    `json:"revision"`
	InboxID      string `json:"inbox_id"`
}

// WakeUpWorkflow is the internal-life timer. Each activity invocation owns
// exactly one cycle; ContinueAsNew keeps the durable timer alive without
// allowing history to grow indefinitely. The cadence is returned by Core so a
// settings change takes effect on the next cycle after a Worker restart.
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
		// A provider/queue outage must not terminate the long-lived wake-up
		// timer. Advance the cycle after the bounded activity retry; the durable
		// cognition_wakeups unique key keeps a partially persisted prior cycle
		// idempotent while the next cycle gets a fresh chance to run.
		next := input
		next.Cycle++
		if sleepErr := workflow.Sleep(ctx, wakeUpInterval(nil)); sleepErr != nil {
			return nil, err
		}
		return nil, workflow.NewContinueAsNewError(ctx, WakeUpWorkflow, next)
	}
	if stringValue(result["status"]) == "inactive" {
		return result, nil
	}
	interval := wakeUpInterval(result)
	if err := control.waitUntilResumed(ctx); err != nil {
		return nil, err
	}
	if err := workflow.Sleep(ctx, interval); err != nil {
		return nil, err
	}
	next := input
	next.Cycle++
	return nil, workflow.NewContinueAsNewError(ctx, WakeUpWorkflow, next)
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
	if err := control.waitUntilResumed(ctx); err != nil {
		return nil, err
	}
	delay := nextLocalMidnightDelay(workflow.Now(ctx), stringValue(result["timezone"]))
	if err := workflow.Sleep(ctx, delay); err != nil {
		return nil, err
	}
	next := input
	next.LocalDate = ""
	return nil, workflow.NewContinueAsNewError(ctx, DailyReviewWorkflow, next)
}

func dailyReviewNeedsRetry(result map[string]any) bool {
	return stringValue(result["status"]) == "pending"
}

func MediaWorkflow(ctx workflow.Context, input Input) (map[string]any, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: 20 * time.Minute, HeartbeatTimeout: 30 * time.Second, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 3}})
	control, err := registerWorkflowControl(ctx)
	if err != nil {
		return nil, err
	}
	if err := control.waitUntilResumed(ctx); err != nil {
		return nil, err
	}
	var result map[string]any
	if err := workflow.ExecuteActivity(ctx, ProcessMediaActivity, input).Get(ctx, &result); err != nil {
		return nil, err
	}
	if err := control.waitUntilResumed(ctx); err != nil {
		return nil, err
	}
	return result, nil
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
	slog.Default().Info("Go Worker wake-up activity started", "fluctlight_id", input.FluctlightID, "cycle", input.Cycle)
	result, err := application.ProcessWakeUp(ctx, input.FluctlightID, input.Cycle)
	if err != nil {
		slog.Default().Error("Go Worker wake-up activity failed", "fluctlight_id", input.FluctlightID, "cycle", input.Cycle, "error", err)
	} else {
		slog.Default().Info("Go Worker wake-up activity completed", "fluctlight_id", input.FluctlightID, "cycle", input.Cycle, "status", stringValue(result["status"]))
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

func PlatformControlActivity(ctx context.Context, input Input) (map[string]any, error) {
	return map[string]any{"status": "ready", "intent_id": input.IntentID}, nil
}

func ProcessMediaActivity(ctx context.Context, input Input) (map[string]any, error) {
	application := app()
	if application == nil {
		return nil, fmt.Errorf("Go Core Worker is not configured")
	}
	if err := application.ProcessMediaIntent(ctx, input.IntentID); err != nil {
		// Keep the durable media target truthful after Temporal exhausts its
		// bounded activity retries. The workflow intent reconciliation records
		// the execution failure separately; this row is what product reads.
		_, _ = application.DB.Pool().Exec(ctx, `UPDATE public.media_intents SET status='failed',revision=revision+1 WHERE id=$1 AND status IN ('pending','running')`, input.IntentID)
		return nil, err
	}
	return map[string]any{"intent_id": input.IntentID, "status": "completed"}, nil
}

func ProcessAutonomyActionActivity(ctx context.Context, input Input) (map[string]any, error) {
	application := app()
	if application == nil {
		return nil, fmt.Errorf("Go Core Worker is not configured")
	}
	return application.ProcessAutonomyAction(ctx, input.ActionID)
}

func ProcessCapabilityActionActivity(ctx context.Context, input Input) (map[string]any, error) {
	application := app()
	if application == nil {
		return nil, fmt.Errorf("Go Core Worker is not configured")
	}
	return application.ProcessCapabilityAction(ctx, input.ActionID)
}

func ProcessReflectionActivity(ctx context.Context, input Input) (map[string]any, error) {
	application := app()
	if application == nil {
		return nil, fmt.Errorf("Go Core Worker is not configured")
	}
	return application.ProcessReflection(ctx, input.FluctlightID, input.IntentID)
}

func ProcessMemoryEmbeddingActivity(ctx context.Context, input Input) (map[string]any, error) {
	application := app()
	if application == nil {
		return nil, fmt.Errorf("Go Core Worker is not configured")
	}
	return application.ProcessMemoryEmbeddingAt(ctx, input.MemoryID, input.Revision)
}

func EnsureCurrentDayScheduleActivity(ctx context.Context, input Input) (map[string]any, error) {
	application := app()
	if application == nil {
		return nil, fmt.Errorf("Go Core Worker is not configured")
	}
	return application.EnsureCurrentDaySchedule(ctx, input.FluctlightID)
}

// StartWorkers starts exactly one worker per canonical task queue and returns a
// fatal-error channel. The caller must terminate the process when that channel
// receives an error; otherwise a live-but-not-polling Worker would silently
// leave durable intents stuck in PostgreSQL.
func StartWorkers(ctx context.Context, temporalClient client.Client, logger *slog.Logger) ([]worker.Worker, <-chan error, error) {
	if logger == nil {
		logger = slog.Default()
	}
	queues := []string{LifecycleQueue, MediaQueue, InteractionQueue}
	workers := make([]worker.Worker, 0, len(queues))
	fatalErrors := make(chan error, len(queues))
	buildID := WorkerDeploymentBuildID()
	for _, queue := range queues {
		// Keep two lifecycle slots so a provider-backed daily review cannot
		// starve the independent schedule/bootstrap activity. Provider-heavy
		// work remains bounded and interactive chat is handled in its own path.
		concurrency := 2
		if queue == MediaQueue {
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
			w.RegisterWorkflow(WakeUpWorkflow)
			w.RegisterWorkflow(DailyReviewWorkflow)
			w.RegisterWorkflow(CurrentDayScheduleWorkflow)
			w.RegisterWorkflow(ReflectionWorkflow)
			w.RegisterWorkflow(MemoryEmbeddingWorkflow)
			w.RegisterWorkflow(PlatformControlWorkflow)
			w.RegisterActivity(ProcessDailyReviewActivity)
			w.RegisterActivity(ProcessWakeUpActivity)
			w.RegisterActivity(EnsureCurrentDayScheduleActivity)
			w.RegisterActivity(ProcessReflectionActivity)
			w.RegisterActivity(ProcessMemoryEmbeddingActivity)
			w.RegisterActivity(PlatformControlActivity)
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
		var intentID, workflowID, intentType string
		if err := rows.Scan(&intentID, &workflowID, &intentType); err != nil {
			return count, err
		}
		execution, describeErr := d.Client.DescribeWorkflowExecution(ctx, normalizedWorkflowID(workflowID), "")
		if describeErr != nil {
			// A just-started execution may not be visible immediately. Leave the
			// intent untouched and let the next pass retry the lookup.
			continue
		}
		if execution == nil || execution.WorkflowExecutionInfo == nil {
			continue
		}
		status := execution.WorkflowExecutionInfo.GetStatus()
		if status == enumspb.WORKFLOW_EXECUTION_STATUS_RUNNING {
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
		if intentType == "wake_up.current" {
			var fluctlightStatus string
			if err := d.App.DB.Pool().QueryRow(ctx, `SELECT status FROM public.fluctlights WHERE id=(SELECT payload->>'fluctlight_id' FROM public.platform_workflow_intents WHERE intent_id=$1)`, intentID).Scan(&fluctlightStatus); err == nil && wakeUpIntentShouldRetry(fluctlightStatus, intentStatus) {
				if _, err := d.App.DB.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET status='retry',next_attempt_at=now()+interval '5 minutes',started_at=NULL,completed_at=NULL,last_error=COALESCE(last_error,'wake_up_workflow_terminal') WHERE intent_id=$1`, intentID); err != nil {
					return count, err
				}
				slog.Default().Warn("Go Worker wake-up workflow requeued after terminal execution", "intent_id", intentID, "workflow_id", workflowID, "temporal_status", intentStatus, "fluctlight_status", fluctlightStatus, "next_attempt", "5m")
				if d.Started != nil {
					delete(d.Started, intentID)
				}
				count++
				continue
			}
		}
		if _, err := d.App.DB.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET status=$2::varchar,completed_at=COALESCE(completed_at,now()),last_error=CASE WHEN $2::varchar='failed' THEN COALESCE(last_error,'workflow_terminal_failure') ELSE last_error END WHERE intent_id=$1`, intentID, intentStatus); err != nil {
			return count, err
		}
		if d.Started != nil {
			delete(d.Started, intentID)
		}
		count++
	}
	return count, rows.Err()
}

func wakeUpIntentShouldRetry(fluctlightStatus, workflowStatus string) bool {
	if fluctlightStatus != "active" && fluctlightStatus != "paused" {
		return false
	}
	return workflowStatus == "failed" || workflowStatus == "completed"
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
	return enumspb.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE
}

func (d *Dispatcher) DispatchOnce(ctx context.Context, limit int) (int, error) {
	if limit < 1 {
		limit = 1
	}
	if d.Started == nil {
		d.Started = make(map[string]struct{})
	}
	query := `SELECT intent_id,workflow_id,task_queue,intent_type,payload FROM public.platform_workflow_intents`
	args := make([]any, 0, len(d.Started)+1)
	if len(d.Started) > 0 {
		placeholders := make([]string, 0, len(d.Started))
		index := 1
		for intentID := range d.Started {
			placeholders = append(placeholders, fmt.Sprintf("$%d", index))
			args = append(args, intentID)
			index++
		}
		query += ` WHERE intent_id NOT IN (` + strings.Join(placeholders, ",") + `)`
		query += ` AND (status IS NULL OR status IN ('pending','retry')) AND (next_attempt_at IS NULL OR next_attempt_at <= now())`
	} else {
		query += ` WHERE (status IS NULL OR status IN ('pending','retry')) AND (next_attempt_at IS NULL OR next_attempt_at <= now())`
	}
	args = append(args, limit)
	query += fmt.Sprintf(` ORDER BY CASE WHEN intent_type LIKE 'schedule.%%' THEN 0 WHEN intent_type LIKE 'wake_up.%%' THEN 1 WHEN intent_type LIKE 'daily_review.%%' THEN 2 WHEN intent_type LIKE 'autonomy.%%' THEN 3 WHEN intent_type LIKE 'capability.%%' THEN 4 WHEN intent_type LIKE 'media.%%' THEN 5 WHEN intent_type LIKE 'reflection.%%' THEN 6 ELSE 7 END, created_at, intent_id LIMIT $%d`, len(args))
	rows, err := d.App.DB.Pool().Query(ctx, query, args...)
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
		if err := rows.Scan(&intentID, &workflowID, &queue, &intentType, &payload); err != nil {
			return count, err
		}
		if _, ok := d.Started[intentID]; ok {
			continue
		}
		var input Input
		if err := json.Unmarshal(payload, &input); err != nil {
			slog.Default().Warn("Go Worker intent payload invalid; leaving pending", "intent_id", intentID, "error", err)
			continue
		}
		if input.IntentID == "" {
			input.IntentID = intentID
		}
		if input.ActionID == "" {
			input.ActionID = stringValue(inputMap(payload)["action_id"])
		}
		if input.MemoryID == "" {
			input.MemoryID = stringValue(inputMap(payload)["memory_id"])
		}
		if input.FluctlightID == "" {
			input.FluctlightID = stringValue(inputMap(payload)["fluctlight_id"])
		}
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
		case "wake_up.current":
			workflowFn = WakeUpWorkflow
			taskQueue = LifecycleQueue
		case "daily_review.current_day":
			workflowFn = DailyReviewWorkflow
			taskQueue = LifecycleQueue
		case "media.generation":
			var exists bool
			if err := d.App.DB.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.media_intents WHERE id=$1)`, input.IntentID).Scan(&exists); err != nil {
				return count, err
			}
			if !exists {
				_, _ = d.App.DB.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET status='failed',last_error='media_intent_not_found',attempt_count=attempt_count+1,completed_at=now() WHERE intent_id=$1`, intentID)
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
			taskQueue = LifecycleQueue
		case "memory.embedding":
			workflowFn = MemoryEmbeddingWorkflow
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
		_, err := d.Client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{ID: goWorkflowID, TaskQueue: taskQueue, WorkflowIDReusePolicy: workflowIDReusePolicy(intentType)}, workflowFn, input)
		if err != nil && !temporal.IsWorkflowExecutionAlreadyStartedError(err) {
			slog.Default().Warn("Go Worker workflow start failed", "intent_id", intentID, "error", err)
			_, _ = d.App.DB.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET status='retry',last_error=$2,attempt_count=attempt_count+1,next_attempt_at=now()+interval '5 seconds' WHERE intent_id=$1`, intentID, err.Error())
			continue
		}
		slog.Default().Info("Go Worker workflow dispatched", "intent_id", intentID, "workflow_id", goWorkflowID, "workflow_type", intentType)
		command, statusErr := d.App.DB.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET status='started',started_at=COALESCE(started_at,now()),attempt_count=attempt_count+1,last_error=NULL WHERE intent_id=$1`, intentID)
		if statusErr != nil || command.RowsAffected() != 1 {
			// Temporal already accepted the start. Leave the durable row visible
			// to the next reconciliation pass rather than hiding a DB failure in
			// the in-memory Started set.
			if statusErr != nil {
				slog.Default().Warn("Go Worker intent status update failed after Temporal start", "intent_id", intentID, "error", statusErr)
			}
			continue
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

func inputMap(payload []byte) map[string]any {
	var result map[string]any
	_ = json.Unmarshal(payload, &result)
	return result
}
func stringValue(value any) string {
	if result, ok := value.(string); ok {
		return result
	}
	return ""
}
