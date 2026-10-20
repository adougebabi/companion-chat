package core

import (
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestPausedCharacterDoesNotCrossMidnightIntoScheduleOrDiary(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID := "paused-midnight-owner", "paused-midnight-fluctlight"
	seedLifeContextFluctlight(t, ctx, repository, ownerID, fluctlightID)
	app := &App{DB: repository}
	workflows := &backgroundCancellationRuntime{}
	app.Workflows = workflows
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload,status,started_at) VALUES($1,$2,'lifecycle','schedule.current_day',$3,'started',now())`, "schedule_intent:"+fluctlightID, "schedule:"+fluctlightID, jsonBytes(map[string]any{"fluctlight_id": fluctlightID})); err != nil {
		t.Fatal(err)
	}
	var revision int
	if err := repository.Pool().QueryRow(ctx, `SELECT current_revision FROM public.fluctlights WHERE id=$1`, fluctlightID).Scan(&revision); err != nil {
		t.Fatal(err)
	}
	if _, err := app.SetFluctlightStatus(ctx, ownerID, fluctlightID, "paused", "stop autonomous life", &revision); err != nil {
		t.Fatal(err)
	}
	var intentStatus, lastError string
	if err := repository.Pool().QueryRow(ctx, `SELECT status,COALESCE(last_error,'') FROM public.platform_workflow_intents WHERE intent_id=$1`, "schedule_intent:"+fluctlightID).Scan(&intentStatus, &lastError); err != nil {
		t.Fatal(err)
	}
	if intentStatus != "cancelled" || lastError != fluctlightPausedStopReason {
		t.Fatalf("schedule intent after pause = %s/%s", intentStatus, lastError)
	}
	if len(workflows.cancelled) != 1 || workflows.cancelled[0] != "go:schedule:"+fluctlightID {
		t.Fatalf("CurrentDayScheduleWorkflow cancel = %v", workflows.cancelled)
	}
	generated, err := app.EnsureCurrentDaySchedule(ctx, fluctlightID)
	if err != nil || stringValue(generated["status"]) != "inactive" || stringValue(generated["reason"]) != fluctlightPausedStopReason {
		t.Fatalf("midnight schedule = %#v err=%v", generated, err)
	}
	now := time.Now().UTC()
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		return insertPostScheduleLifecycleIntentsTx(ctx, tx, fluctlightID, now, "Asia/Shanghai", now)
	}); err != nil {
		t.Fatal(err)
	}
	var schedules, diaries, reviews int
	if err := repository.Pool().QueryRow(ctx, `SELECT (SELECT count(*) FROM public.life_schedules WHERE fluctlight_id=$1), (SELECT count(*) FROM public.fluctlight_diary_days WHERE fluctlight_id=$1), (SELECT count(*) FROM public.platform_workflow_intents WHERE intent_type='daily_review.current_day' AND payload->>'fluctlight_id'=$1)`, fluctlightID).Scan(&schedules, &diaries, &reviews); err != nil {
		t.Fatal(err)
	}
	if schedules != 0 || diaries != 0 || reviews != 0 {
		t.Fatalf("paused midnight created schedule=%d diary=%d daily_review=%d", schedules, diaries, reviews)
	}
}

func TestPausedCharacterAutonomousActivitiesDoNotRun(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID := "paused-activity-owner", "paused-activity-fluctlight"
	seedLifeContextFluctlight(t, ctx, repository, ownerID, fluctlightID)
	app := &App{DB: repository}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload,status,next_attempt_at) VALUES($1,$2,'lifecycle','wake_up.current',$3,'failed',now())`, "wake_up_intent:"+fluctlightID, "wake_up:"+fluctlightID, jsonBytes(map[string]any{"fluctlight_id": fluctlightID, "cycle": 1})); err != nil {
		t.Fatal(err)
	}
	diaryID := "diary_paused_activity"
	attemptID := diaryID + "_attempt_0"
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlight_diary_days(id,fluctlight_id,local_date,timezone,status,latest_attempt) VALUES($1,$2,'2026-10-18','Asia/Shanghai','pending',0)`, diaryID, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlight_diary_attempts(id,diary_id,attempt,status) VALUES($1,$2,0,'pending')`, attemptID, diaryID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlight_intentions(id,fluctlight_id,profile_id,goal_id,action,action_intent,expected_outcome,capability_constraints,trigger,confidence,expiration,evidence_refs,permission_snapshot,budget_snapshot,status,revision,idempotency_key,request_digest) VALUES($1,$2,NULL,NULL,'paused','check in','outcome','[]','{"type":"time"}','0.9',now()+interval '30 days','[]','{}','{}','qualified',1,$1,'0123456789abcdef0123456789abcdef')`, "paused-intention", fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.goal_planning_runs(id,fluctlight_id,watermark,available_at,status) VALUES($1,$2,0,now(),'pending')`, "paused-goal-plan", fluctlightID); err != nil {
		t.Fatal(err)
	}
	var revision int
	if err := repository.Pool().QueryRow(ctx, `SELECT current_revision FROM public.fluctlights WHERE id=$1`, fluctlightID).Scan(&revision); err != nil {
		t.Fatal(err)
	}
	if _, err := app.SetFluctlightStatus(ctx, ownerID, fluctlightID, "paused", "stop autonomous activities", &revision); err != nil {
		t.Fatal(err)
	}

	wake, err := app.ProcessWakeUp(ctx, fluctlightID, 1)
	if err != nil || stringValue(wake["status"]) != "paused" || stringValue(wake["reason"]) != fluctlightPausedStopReason {
		t.Fatalf("wakeup = %#v err=%v", wake, err)
	}
	var wakeups int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.cognition_wakeups WHERE fluctlight_id=$1`, fluctlightID).Scan(&wakeups); err != nil || wakeups != 0 {
		t.Fatalf("paused wakeup wrote %d cycles (%v)", wakeups, err)
	}
	if ensured, err := app.EnsureWakeUpIntents(ctx); err != nil || ensured != 0 {
		t.Fatalf("paused wakeup enqueue = %d err=%v", ensured, err)
	}
	var wakeStatus string
	if err := repository.Pool().QueryRow(ctx, `SELECT status FROM public.platform_workflow_intents WHERE intent_id=$1`, "wake_up_intent:"+fluctlightID).Scan(&wakeStatus); err != nil || wakeStatus != "completed" {
		t.Fatalf("paused wakeup intent status = %s err=%v", wakeStatus, err)
	}
	if repaired, err := app.RepairWakeUpClocks(ctx); err != nil || repaired != 0 {
		t.Fatalf("paused wakeup clock repair = %d err=%v", repaired, err)
	}
	if err := app.scheduleWakeUpAfterCognition(ctx, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT status FROM public.platform_workflow_intents WHERE intent_id=$1`, "wake_up_intent:"+fluctlightID).Scan(&wakeStatus); err != nil || wakeStatus != "completed" {
		t.Fatalf("cognition follow-up rearmed paused wakeup: %s err=%v", wakeStatus, err)
	}

	reflection, err := app.ProcessReflection(ctx, fluctlightID, "paused-reflection")
	if err != nil || stringValue(reflection["status"]) != "paused" {
		t.Fatalf("reflection = %#v err=%v", reflection, err)
	}
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		return insertReflectionIntentTx(ctx, tx, "reflection_intent:paused-activity", "reflection:paused-activity", map[string]any{"fluctlight_id": fluctlightID})
	}); err != nil {
		t.Fatal(err)
	}
	var reflections int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.platform_workflow_intents WHERE intent_id='reflection_intent:paused-activity'`).Scan(&reflections); err != nil || reflections != 0 {
		t.Fatalf("paused reflection enqueue = %d err=%v", reflections, err)
	}

	diary, err := app.ProcessDiaryGeneration(ctx, attemptID)
	if err != nil || stringValue(diary["status"]) != "inactive" {
		t.Fatalf("diary = %#v err=%v", diary, err)
	}
	var diaryBody *string
	if err := repository.Pool().QueryRow(ctx, `SELECT body FROM public.fluctlight_diary_attempts WHERE id=$1`, attemptID).Scan(&diaryBody); err != nil || diaryBody != nil {
		t.Fatalf("paused diary wrote a body: %v err=%v", diaryBody, err)
	}

	review, err := app.ProcessDailyReview(ctx, fluctlightID, "2026-10-20")
	if err != nil || stringValue(review["status"]) != "inactive" {
		t.Fatalf("daily review = %#v err=%v", review, err)
	}

	intention, err := app.ProcessIntentionTrigger(ctx, "paused-intention")
	if err != nil || stringValue(intention["status"]) != "inactive" || stringValue(intention["reason"]) != fluctlightPausedStopReason {
		t.Fatalf("intention = %#v err=%v", intention, err)
	}

	if _, err := app.EnsureVisualIdentityInitialization(ctx, fluctlightID, "manual", "paused-source"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("visual identity enqueue err=%v", err)
	}
	var sessions int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.fluctlight_visual_identity_sessions WHERE fluctlight_id=$1`, fluctlightID).Scan(&sessions); err != nil || sessions != 0 {
		t.Fatalf("paused visual session count = %d err=%v", sessions, err)
	}

	plan, err := app.ProcessGoalPlanningIntent(ctx, "paused-goal-plan")
	if err != nil || stringValue(plan["status"]) != "inactive" || plan["model_admission"] != false {
		t.Fatalf("goal plan = %#v err=%v", plan, err)
	}
	if repaired, err := app.RepairGoalPlanning(ctx, 20); err != nil || repaired != 0 {
		t.Fatalf("paused goal planning repair = %d err=%v", repaired, err)
	}
	if repaired, err := app.RepairGoalEvaluationCadence(ctx, 20); err != nil || repaired != 0 {
		t.Fatalf("paused goal evaluation repair = %d err=%v", repaired, err)
	}
	if repaired, err := app.RepairOwnerPromiseDeadlines(ctx, 20); err != nil || repaired != 0 {
		t.Fatalf("paused promise repair = %d err=%v", repaired, err)
	}
}

func TestPausedCharacterWorldAndNPCActivitiesDoNotRun(t *testing.T) {
	fixture, _, _, _ := seedNPCEncounterRuntime(t, true)
	var revision int
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT current_revision FROM public.fluctlights WHERE id=$1`, fixture.fluctlightID).Scan(&revision); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.app.SetFluctlightStatus(fixture.ctx, fixture.ownerID, fixture.fluctlightID, "paused", "stop world activity", &revision); err != nil {
		t.Fatal(err)
	}
	if count, err := fixture.app.RepairWorldNPCPresence(fixture.ctx, 20); err != nil || count != 0 {
		t.Fatalf("paused NPC presence repair = %d err=%v", count, err)
	}
	if count, err := fixture.app.RepairNPCOpportunities(fixture.ctx, 20); err != nil || count != 0 {
		t.Fatalf("paused NPC opportunity repair = %d err=%v", count, err)
	}
	if count, err := fixture.app.RepairWorldEvents(fixture.ctx, 20); err != nil || count != 0 {
		t.Fatalf("paused world event repair = %d err=%v", count, err)
	}
	var presence, runs int
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT (SELECT count(*) FROM public.world_npc_presence_runs r JOIN public.life_worlds w ON w.id=r.world_id WHERE w.fluctlight_id=$1), (SELECT count(*) FROM public.world_event_runs WHERE fluctlight_id=$1)`, fixture.fluctlightID).Scan(&presence, &runs); err != nil {
		t.Fatal(err)
	}
	if presence != 0 || runs != 0 {
		t.Fatalf("paused world activity created presence=%d events=%d", presence, runs)
	}
}
