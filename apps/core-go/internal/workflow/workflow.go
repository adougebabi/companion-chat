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
	LifecycleQueue   = "lifecycle"
	MediaQueue       = "media"
	InteractionQueue = "interaction"
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

type Input struct {
	IntentID     string `json:"intent_id"`
	FluctlightID string `json:"fluctlight_id"`
	LocalDate    string `json:"local_date"`
	ActionID     string `json:"action_id"`
	MemoryID     string `json:"memory_id"`
}

func DailyReviewWorkflow(ctx workflow.Context, input Input) (map[string]any, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: 10 * time.Minute, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 3}})
	var result map[string]any
	if err := workflow.ExecuteActivity(ctx, ProcessDailyReviewActivity, input).Get(ctx, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func MediaWorkflow(ctx workflow.Context, input Input) (map[string]any, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: 20 * time.Minute, HeartbeatTimeout: 30 * time.Second, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 3}})
	var result map[string]any
	if err := workflow.ExecuteActivity(ctx, ProcessMediaActivity, input).Get(ctx, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func CurrentDayScheduleWorkflow(ctx workflow.Context, input Input) (map[string]any, error) {
	// Initial schedule generation is a real structured Provider call and may
	// take several minutes on a local model. The previous 30s activity timeout
	// cancelled every first attempt before the Provider could respond.
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: 10 * time.Minute, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 2}})
	var result map[string]any
	if err := workflow.ExecuteActivity(ctx, EnsureCurrentDayScheduleActivity, input).Get(ctx, &result); err != nil {
		return nil, err
	}
	result["intent_id"] = input.IntentID
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

func AutonomyActionWorkflow(ctx workflow.Context, input Input) (map[string]any, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: 10 * time.Minute, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 3}})
	var result map[string]any
	if err := workflow.ExecuteActivity(ctx, ProcessAutonomyActionActivity, input).Get(ctx, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func ReflectionWorkflow(ctx workflow.Context, input Input) (map[string]any, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: 10 * time.Minute, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 2}})
	var result map[string]any
	if err := workflow.ExecuteActivity(ctx, ProcessReflectionActivity, input).Get(ctx, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func MemoryEmbeddingWorkflow(ctx workflow.Context, input Input) (map[string]any, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: 10 * time.Minute, RetryPolicy: &temporal.RetryPolicy{MaximumAttempts: 2}})
	var result map[string]any
	if err := workflow.ExecuteActivity(ctx, ProcessMemoryEmbeddingActivity, input).Get(ctx, &result); err != nil {
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

func ProcessMediaActivity(ctx context.Context, input Input) (map[string]any, error) {
	application := app()
	if application == nil {
		return nil, fmt.Errorf("Go Core Worker is not configured")
	}
	if err := application.ProcessMediaIntent(ctx, input.IntentID); err != nil {
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
	return application.ProcessMemoryEmbedding(ctx, input.MemoryID)
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
	buildID := strings.TrimSpace(os.Getenv("TEMPORAL_WORKER_BUILD_ID"))
	if buildID == "" {
		// The existing Temporal namespace routes its current deployment to
		// platform-v1. Cutover fences the retired Python executions first; the
		// replacement Go worker claims that deployment version until an operator
		// deliberately promotes a new immutable build ID.
		buildID = "platform-v1"
	}
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
					DeploymentName: "fluctlight",
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
		w.RegisterWorkflow(DailyReviewWorkflow)
		w.RegisterWorkflow(MediaWorkflow)
		w.RegisterWorkflow(CurrentDayScheduleWorkflow)
		w.RegisterWorkflow(AutonomyActionWorkflow)
		w.RegisterWorkflow(ReflectionWorkflow)
		w.RegisterWorkflow(MemoryEmbeddingWorkflow)
		w.RegisterActivity(ProcessDailyReviewActivity)
		w.RegisterActivity(ProcessMediaActivity)
		w.RegisterActivity(ProcessAutonomyActionActivity)
		w.RegisterActivity(ProcessReflectionActivity)
		w.RegisterActivity(ProcessMemoryEmbeddingActivity)
		w.RegisterActivity(EnsureCurrentDayScheduleActivity)
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
	query += fmt.Sprintf(` ORDER BY CASE WHEN intent_type LIKE 'schedule.%%' THEN 0 WHEN intent_type LIKE 'daily_review.%%' THEN 1 WHEN intent_type LIKE 'autonomy.%%' THEN 2 WHEN intent_type LIKE 'media.%%' THEN 3 WHEN intent_type LIKE 'reflection.%%' THEN 4 ELSE 5 END, created_at, intent_id LIMIT $%d`, len(args))
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
		workflowFn := any(nil)
		taskQueue := queue
		// Every post-cutover execution receives a stable Go namespace. This
		// fences closed/active executions created by the retired runtime while
		// preserving deterministic replay for this intent after restarts.
		goWorkflowID := workflowID
		if !strings.HasPrefix(goWorkflowID, "go:") {
			goWorkflowID = "go:" + goWorkflowID
		}
		switch intentType {
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
		case "reflection.run":
			workflowFn = ReflectionWorkflow
			taskQueue = LifecycleQueue
		case "memory.embedding":
			workflowFn = MemoryEmbeddingWorkflow
			taskQueue = LifecycleQueue
		default:
			slog.Default().Warn("Go Worker intent type unsupported; leaving pending", "intent_id", intentID, "intent_type", intentType)
			continue
		}
		_, err := d.Client.ExecuteWorkflow(ctx, client.StartWorkflowOptions{ID: goWorkflowID, TaskQueue: taskQueue, WorkflowIDReusePolicy: enumspb.WORKFLOW_ID_REUSE_POLICY_REJECT_DUPLICATE}, workflowFn, input)
		if err != nil && !temporal.IsWorkflowExecutionAlreadyStartedError(err) {
			slog.Default().Warn("Go Worker workflow start failed", "intent_id", intentID, "error", err)
			_, _ = d.App.DB.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET status='retry',last_error=$2,attempt_count=attempt_count+1,next_attempt_at=now()+interval '5 seconds' WHERE intent_id=$1`, intentID, err.Error())
			continue
		}
		slog.Default().Info("Go Worker workflow dispatched", "intent_id", intentID, "workflow_id", goWorkflowID, "workflow_type", intentType)
		d.Started[intentID] = struct{}{}
		_, _ = d.App.DB.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET status='started',started_at=COALESCE(started_at,now()),attempt_count=attempt_count+1,last_error=NULL WHERE intent_id=$1`, intentID)
		count++
	}
	return count, rows.Err()
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
