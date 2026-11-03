package workflow

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	commonpb "go.temporal.io/api/common/v1"
	enumspb "go.temporal.io/api/enums/v1"
	workflowpb "go.temporal.io/api/workflow/v1"
	workflowservice "go.temporal.io/api/workflowservice/v1"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/mocks"
)

func TestPostgresWardrobeBackoffExhaustionAndOwnerRecoveryDispatch(t *testing.T) {
	ctx, app := scheduleRecoveryRepository(t)
	if _, err := app.DB.Pool().Exec(ctx, `DELETE FROM public.platform_workflow_intents WHERE intent_id='schedule_intent:retry-role';
 INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload,status,attempt_count,last_error)
 SELECT intent_id,intent_id,'lifecycle','wardrobe.initialize',jsonb_build_object('fluctlight_id',fluctlight_id,'session_id',id),'failed',1,'context deadline exceeded' FROM public.fluctlight_wardrobe_preparations WHERE fluctlight_id='retry-role';
 UPDATE public.fluctlight_wardrobe_preparations SET status='failed',result='{"inventory_committed":true,"failure_reason":"old failure"}',updated_at=now()-interval '1 hour' WHERE fluctlight_id='retry-role'`); err != nil {
		t.Fatal(err)
	}
	const intent, workflowID = "wardrobe_initialize:retry-role", "go:wardrobe_initialize:retry-role"
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

		if err := app.DB.Pool().QueryRow(ctx, `SELECT status,attempt_count,next_attempt_at FROM public.platform_workflow_intents WHERE intent_id=$1`, intent).Scan(&status, &attempts, &next); err != nil {
			t.Fatal(err)
		}
		if status != wantStatus || attempts != wantAttempts {
			t.Fatalf("status=%s attempts=%d; want=%s/%d", status, attempts, wantStatus, wantAttempts)
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
		if count, err := app.RepairWardrobePreparations(ctx, 100); err != nil || count != 0 {
			t.Fatal("legacy repair bypassed managed backoff", count, err)
		}
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
	if count, err := app.RepairWardrobePreparations(ctx, 100); err != nil || count != 0 {
		t.Fatal("legacy repair reopened exhausted initializer", count, err)
	}
	if starts != 2 {
		t.Fatal("unexpected automatic executions", starts)
	}
	body := map[string]any{"reason": "模型已恢复，重试日程", "idempotency_key": "owner-recover"}
	recovered, err := app.DispatchOwnerSurface(ctx, "POST", "/internal/fluctlights/retry-role/wardrobe/recover", nil, "retry-owner", body)
	if err != nil || recovered["restarted"] != true || recovered["workflow_id"] != "wardrobe_initialize:retry-role" {
		t.Fatal("owner recovery failed", recovered, err)
	}
	var preparationStatus string
	var inventoryCommitted, staleFailure bool
	if err := app.DB.Pool().QueryRow(ctx, `SELECT status,result->'inventory_committed'='true'::jsonb,result ? 'failure_reason' FROM public.fluctlight_wardrobe_preparations WHERE fluctlight_id='retry-role'`).Scan(&preparationStatus, &inventoryCommitted, &staleFailure); err != nil || preparationStatus != "awaiting_wear" || !inventoryCommitted || staleFailure {
		t.Fatal("recovery lost checkpoint or retained failure", preparationStatus, inventoryCommitted, staleFailure, err)
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
	if _, err := app.DispatchOwnerSurface(ctx, "POST", "/internal/fluctlights/retry-role/wardrobe/recover", nil, "retry-owner", body); err != nil {
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
		_, err := app.DispatchOwnerSurface(ctx, "POST", "/internal/fluctlights/retry-role/wardrobe/recover", nil, "retry-owner", map[string]any{"reason": "并发恢复", "idempotency_key": "concurrent-recover"})
		if err != nil {
			t.Fatal(err)
		}
	}
	reconcile()
	check("pending", 0, 0) // The stale failure cannot undo concurrent recovery.
	var schedules, intents int
	if err := app.DB.Pool().QueryRow(ctx, `SELECT (SELECT count(*) FROM public.life_schedules),(SELECT count(*) FROM public.platform_workflow_intents WHERE intent_type='wardrobe.initialize')`).Scan(&schedules, &intents); err != nil || schedules != 0 || intents != 1 {
		t.Fatal("recovery invented a schedule or duplicated intent", schedules, intents, err)
	}
}

func TestPostgresWardrobePersonaRecoveryUnblocksOriginalInitializer(t *testing.T) {
	ctx, app := scheduleRecoveryRepository(t)
	if _, err := app.DB.Pool().Exec(ctx, `
 INSERT INTO public.life_worlds(id,fluctlight_id,name) VALUES('recovery-world','retry-role','恢复测试世界');
 INSERT INTO public.fluctlight_world_bindings(fluctlight_id,world_id,revision) VALUES('retry-role','recovery-world',1);
 INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload,status)
 VALUES('failed-persona','failed-persona','lifecycle','persona.compile','{"fluctlight_id":"retry-role","owner_actor_id":"retry-owner","expected_revision":0,"source":"initialization"}','failed');
 INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload)
 SELECT intent_id,intent_id,'lifecycle','wardrobe.initialize',jsonb_build_object('fluctlight_id',fluctlight_id,'session_id',id) FROM public.fluctlight_wardrobe_preparations WHERE fluctlight_id='retry-role'`); err != nil {
		t.Fatal(err)
	}
	eligible := func(kind string, want bool) {
		t.Helper()
		var actual bool
		if err := app.DB.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM (`+dispatcherCandidateQuery()+`) ready WHERE ready.intent_type=$2)`, 100, kind).Scan(&actual); err != nil || actual != want {
			t.Fatalf("%s dispatchable=%v want=%v err=%v", kind, actual, want, err)
		}
	}
	eligible("wardrobe.initialize", false)
	out, err := app.DispatchOwnerSurface(ctx, "POST", "/internal/fluctlights/retry-role/wardrobe/recover", nil, "retry-owner", map[string]any{"reason": "恢复上游准备", "idempotency_key": "recover-persona"})
	if err != nil || out["restarted"] != true || out["intent_id"] != "wardrobe_initialize:retry-role" || out["message"] != "已重新发起人格准备，完成后将自动继续初始衣柜准备。" {
		t.Fatal("wardrobe recovery did not resume dependency", out, err)
	}
	eligible("persona.compile", true)
	eligible("wardrobe.initialize", false)
	var persona string
	if err := app.DB.Pool().QueryRow(ctx, `SELECT intent_id FROM public.platform_workflow_intents WHERE intent_type='persona.compile' AND status='pending'`).Scan(&persona); err != nil {
		t.Fatal(err)
	}
	if _, err := app.ProcessPersonaPreparationIntent(ctx, persona); err != nil {
		t.Fatal("recovered persona could not complete", err)
	}
	eligible("wardrobe.initialize", true)
	var count int
	if err := app.DB.Pool().QueryRow(ctx, `SELECT count(*) FROM public.platform_workflow_intents WHERE intent_type='wardrobe.initialize'`).Scan(&count); err != nil || count != 1 {
		t.Fatal("recovery replaced or duplicated wardrobe initializer", count, err)
	}
}
