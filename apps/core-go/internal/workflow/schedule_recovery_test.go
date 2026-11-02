package workflow

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/migrations"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/mock"
	commonpb "go.temporal.io/api/common/v1"
	enumspb "go.temporal.io/api/enums/v1"
	workflowpb "go.temporal.io/api/workflow/v1"
	workflowservice "go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/mocks"
)

func scheduleRecoveryRepository(t *testing.T) (context.Context, *core.App) {
	t.Helper()
	databaseURL := os.Getenv("GO_CORE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("GO_CORE_TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	config.ConnConfig.Database = "postgres"
	admin, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	db := fmt.Sprintf("schedule_recovery_%d", time.Now().UnixNano())
	identifier := pgx.Identifier{db}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+identifier); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, stop := context.WithTimeout(context.Background(), 15*time.Second)
		defer stop()
		if _, err := admin.Exec(cleanup, "DROP DATABASE "+identifier); err != nil {
			t.Error(err)
		}
		admin.Close()
	})
	repoURL, err := url.Parse(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	repoURL.Path = "/" + db
	repo, err := core.NewPostgresRepository(ctx, repoURL.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(repo.Close)
	var connectedDatabase string
	if err := repo.Pool().QueryRow(ctx, "SELECT current_database()").Scan(&connectedDatabase); err != nil || connectedDatabase != db {
		t.Fatalf("test database isolation failed: got=%q want=%q err=%v", connectedDatabase, db, err)
	}
	if err := migrations.New(repo.Pool()).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	app := &core.App{DB: repo}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES('retry-owner','human','active'); INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES('retry-owner','hash','revision')`); err != nil {
		t.Fatal(err)
	}
	if _, err := app.AdmitFluctlight(ctx, "retry-owner", "retry-role", "日程重试", "blank_slate", "", nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	// Isolate dispatcher/reconciliation from unrelated initialization work.
	if _, err := repo.Pool().Exec(ctx, `DELETE FROM public.platform_workflow_intents WHERE payload->>'fluctlight_id'='retry-role' AND intent_type<>'schedule.current_day'; UPDATE public.platform_workflow_intents SET status='failed',attempt_count=1,last_error='context deadline exceeded',completed_at=now() WHERE intent_id='schedule_intent:retry-role'`); err != nil {
		t.Fatal(err)
	}
	return ctx, app
}

func TestPostgresScheduleBackoffExhaustionAndOwnerRecoveryDispatch(t *testing.T) {
	ctx, app := scheduleRecoveryRepository(t)
	const intent, workflowID = "schedule_intent:retry-role", "go:schedule:retry-role"
	state := enumspb.WORKFLOW_EXECUTION_STATUS_FAILED
	describes, starts := 0, 0
	var observeErr error
	var duringDescribe func()
	temporalClient := &mocks.Client{}
	iterator := &mocks.HistoryEventIterator{}
	iterator.On("HasNext").Return(false)
	temporalClient.On("GetWorkflowHistory", mock.Anything, workflowID, "observed-run", false, enumspb.HISTORY_EVENT_FILTER_TYPE_ALL_EVENT).Return(iterator)
	temporalClient.On("DescribeWorkflowExecution", mock.Anything, workflowID, "").Return(func(context.Context, string, string) (*workflowservice.DescribeWorkflowExecutionResponse, error) {
		describes++
		if duringDescribe != nil {
			duringDescribe()
		}
		if observeErr != nil {
			return nil, observeErr
		}
		return &workflowservice.DescribeWorkflowExecutionResponse{WorkflowExecutionInfo: &workflowpb.WorkflowExecutionInfo{Status: state, Execution: &commonpb.WorkflowExecution{RunId: "observed-run"}}}, nil
	})
	run := &mocks.WorkflowRun{}
	run.On("GetRunID").Return("new-run")
	temporalClient.On("ExecuteWorkflow", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(func(_ context.Context, options client.StartWorkflowOptions, _ interface{}, args ...interface{}) (client.WorkflowRun, error) {
		starts++
		if options.ID != workflowID || options.WorkflowIDReusePolicy != enumspb.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE_FAILED_ONLY || len(args) != 1 || args[0].(Input).IntentID != intent {
			t.Fatalf("recovery changed stable identity or reuse policy: %#v %#v", options, args)
		}
		state = enumspb.WORKFLOW_EXECUTION_STATUS_RUNNING
		return run, nil
	})
	dispatcher := &Dispatcher{App: app, Client: temporalClient}
	exec := func(sql string, args ...any) {
		t.Helper()
		if _, err := app.DB.Pool().Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	check := func(wantStatus string, wantAttempts int, delay time.Duration) {
		t.Helper()
		var status string
		var attempts int
		var next *time.Time
		var degraded bool
		if err := app.DB.Pool().QueryRow(ctx, `SELECT status,attempt_count,next_attempt_at,COALESCE((payload->>'initial_schedule_degraded')::boolean,false) FROM public.platform_workflow_intents WHERE intent_id=$1`, intent).Scan(&status, &attempts, &next, &degraded); err != nil {
			t.Fatal(err)
		}
		if status != wantStatus || attempts != wantAttempts || !degraded {
			t.Fatalf("status=%s attempts=%d degraded=%v; want=%s/%d", status, attempts, degraded, wantStatus, wantAttempts)
		}
		if delay > 0 && (next == nil || time.Until(*next) < delay-5*time.Second || time.Until(*next) > delay+5*time.Second) {
			t.Fatalf("backoff=%v want=%v", next, delay)
		}
		if wantStatus == "dead_letter" && next != nil {
			t.Fatal("exhausted schedule still has an automatic deadline", next)
		}
	}
	reconcile := func() {
		t.Helper()
		if _, err := dispatcher.ReconcileOnce(ctx, 100); err != nil {
			t.Fatal(err)
		}
	}
	dispatch := func() {
		t.Helper()
		if count, err := dispatcher.DispatchOnce(ctx, 100); err != nil || count != 1 {
			t.Fatal("schedule not dispatched", count, err)
		}
	}
	for attempt := 1; attempt < scheduleMaximumAttempts; attempt++ {
		reconcile()
		check("retry", attempt, time.Duration(attempt)*time.Minute)
		observed := describes
		reconcile()
		if count, err := dispatcher.DispatchOnce(ctx, 100); err != nil || count != 0 {
			t.Fatal("backoff bypassed", count, err)
		}
		exec("UPDATE public.platform_workflow_intents SET next_attempt_at=now() WHERE intent_id=$1", intent)
		reconcile() // The old failed Run must not settle the newly due retry.
		if describes != observed {
			t.Fatal("queued retry observed old run", describes, observed)
		}
		dispatch()
		check("started", attempt+1, 0)
		state = enumspb.WORKFLOW_EXECUTION_STATUS_FAILED
	}
	reconcile()
	check("dead_letter", scheduleMaximumAttempts, 0)
	if starts != 2 {
		t.Fatal("unexpected automatic executions", starts)
	}
	body := map[string]any{"reason": "模型已恢复，重试日程", "idempotency_key": "owner-recover"}
	recovered, err := app.DispatchOwnerSurface(ctx, "POST", "/internal/fluctlights/retry-role/schedules/recover", nil, "retry-owner", body)
	if err != nil || recovered["restarted"] != true || recovered["workflow_id"] != "schedule:retry-role" {
		t.Fatal("owner recovery failed", recovered, err)
	}
	check("pending", 0, 0)
	observed := describes
	reconcile()
	reconcile()
	if describes != observed {
		t.Fatal("owner recovery reconciled against old failed Run")
	}
	dispatch()
	check("started", 1, 0)
	if _, err := app.DispatchOwnerSurface(ctx, "POST", "/internal/fluctlights/retry-role/schedules/recover", nil, "retry-owner", body); err != nil {
		t.Fatal("recovery replay failed", err)
	}
	if count, err := dispatcher.DispatchOnce(ctx, 100); err != nil || count != 0 || starts != 3 {
		t.Fatal("recovery replay duplicated scheduler", count, starts, err)
	}
	// An exhausted execution budget cannot turn an observation outage into a
	// terminal failure. A genuinely running clock repairs the old failed ledger.
	exec("UPDATE public.platform_workflow_intents SET status='failed',attempt_count=3 WHERE intent_id=$1", intent)
	reconcile()
	check("started", 3, 0)
	observeErr = errors.New("Temporal temporarily unavailable")
	reconcile()
	check("started", 3, 0)
	observeErr = nil
	state = enumspb.WORKFLOW_EXECUTION_STATUS_FAILED
	exec("UPDATE public.platform_workflow_intents SET status='failed' WHERE intent_id=$1", intent)
	duringDescribe = func() {
		duringDescribe = nil
		_, err := app.DispatchOwnerSurface(ctx, "POST", "/internal/fluctlights/retry-role/schedules/recover", nil, "retry-owner", map[string]any{"reason": "并发恢复", "idempotency_key": "concurrent-recover"})
		if err != nil {
			t.Fatal(err)
		}
	}
	reconcile()
	check("pending", 0, 0) // The stale failure cannot undo concurrent recovery.
	var schedules, intents int
	if err := app.DB.Pool().QueryRow(ctx, `SELECT (SELECT count(*) FROM public.life_schedules),(SELECT count(*) FROM public.platform_workflow_intents WHERE intent_type='schedule.current_day')`).Scan(&schedules, &intents); err != nil || schedules != 0 || intents != 1 {
		t.Fatal("recovery invented a schedule or duplicated intent", schedules, intents, err)
	}
}
