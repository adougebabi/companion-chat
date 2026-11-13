package core

import (
	"context"

	"github.com/jackc/pgx/v5"
	"testing"
	"time"
)

func TestWakeUpConsecutiveIdleCyclesBackOffAndReplayKeepsDue(t *testing.T) {
	ctx, repo := isolatedCoreTestRepository(t)
	owner, self := "idle-backoff-owner", "idle-backoff-self"
	seedLifeContextFluctlight(t, ctx, repo, owner, self)
	seedCognitiveProviderRole(t, ctx, repo, "idle-backoff-provider")
	persistConversationReplyMode(t, ctx, repo, "legacy")
	calls := 0
	router := newFakeProviderRouter().otherwise(func(map[string]any) fakeProviderResult {
		calls++
		return fakeProviderResult{Structured: map[string]any{"action_type": "no_op", "response_intent": "fixture", "influences": []any{}, "evidence_refs": []any{}}}
	})
	app := newTestApp(t, repo, router)
	now := time.Now().UTC()
	app.Clock = fixedClock(now)
	life := currentLifeForTest(t, ctx, app, self, now)
	if _, err := app.AcceptSchedule(ctx, owner, self, fullDaySchedulePayloadForTest(now, "idle-backoff-schedule", stringValue(life["context_revision"]))); err != nil {
		t.Fatal(err)
	}
	if _, err := app.EnsureDirectConversation(ctx, owner, self); err != nil {
		t.Fatal(err)
	}
	if _, err := app.EnsureWakeUpIntents(ctx); err != nil {
		t.Fatal(err)
	}
	for index, delay := range []time.Duration{20 * time.Minute, 40 * time.Minute, time.Hour, time.Hour} {
		cycle := index + 1
		app.Clock = fixedClock(now)
		if _, err := repo.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET status='started',started_at=now(),payload=jsonb_set(payload,'{cycle}',to_jsonb($2::int)) WHERE intent_id=$1`, "wake_up_intent:"+self, cycle); err != nil {
			t.Fatal(err)
		}
		wakeCtx := WithWakeUpCycle(WithLifecycleIntentID(ctx, "wake_up_intent:"+self), cycle)
		result, err := app.ProcessWakeUp(wakeCtx, self, cycle)
		if err != nil || result["status"] != "no_op" {
			t.Fatalf("idle cycle %d result=%#v err=%v", cycle, result, err)
		}
		due, err := time.Parse(time.RFC3339Nano, stringValue(result["next_due_at"]))
		if err != nil || (due.Sub(now)-delay).Abs() > time.Second {
			t.Fatalf("idle cycle %d delay=%s want=%s err=%v", cycle, due.Sub(now), delay, err)
		}
		// Activity retry re-enters with the same durable cycle in started state.
		if _, err := repo.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET status='started' WHERE intent_id=$1`, "wake_up_intent:"+self); err != nil {
			t.Fatal(err)
		}
		replayed, err := app.ProcessWakeUp(wakeCtx, self, cycle)
		if err != nil || replayed["next_due_at"] != result["next_due_at"] || calls != cycle {
			t.Fatalf("replay moved due or called Provider again: calls=%d result=%#v err=%v", calls, replayed, err)
		}
		now = due
	}
}

func TestWakeUpBackoffLostRedisHintKeepsDurableFutureDue(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	backgroundTriggerRedis(t, f)
	if _, err := f.app.EnsureWakeUpIntents(f.ctx); err != nil {
		t.Fatal(err)
	}
	due := time.Now().UTC().Add(40 * time.Minute)
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.platform_workflow_intents SET next_attempt_at=$2,payload=payload||jsonb_build_object('idle_noop_streak',2,'idle_backoff_until',$3::text) WHERE intent_id=$1`, "wake_up_intent:"+f.fluctlightID, due, due.Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.ScheduleWakeUpTriggers(f.ctx); err != nil {
		t.Fatal(err)
	}
	var status string
	var cycle int
	var saved time.Time
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status,(payload->>'cycle')::int,next_attempt_at FROM public.platform_workflow_intents WHERE intent_id=$1`, "wake_up_intent:"+f.fluctlightID).Scan(&status, &cycle, &saved); err != nil || status != "completed" || cycle != 0 || saved.Sub(due).Abs() > time.Millisecond {
		t.Fatalf("lost Redis key released future WakeUp: status=%s cycle=%d due=%s want=%s err=%v", status, cycle, saved, due, err)
	}
	ttl := f.app.Redis.PTTL(f.ctx, wakeUpTriggerPrefix+f.fluctlightID).Val()
	if ttl < 39*time.Minute || ttl > 40*time.Minute+time.Second {
		t.Fatalf("recovered hint did not mirror durable due: %s", ttl)
	}
}

func seedWakeUpBackoffForTest(t *testing.T, ctx context.Context, repo *PostgresRepository, self string, now time.Time) {
	t.Helper()
	_, err := repo.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET status='completed',next_attempt_at=$2,payload=payload||jsonb_build_object('idle_noop_streak',3,'idle_backoff_until',$3::text) WHERE intent_id=$1`, "wake_up_intent:"+self, now.Add(time.Hour), now.Add(time.Hour).Format(time.RFC3339Nano))
	if err != nil {
		t.Fatal(err)
	}
}

func TestWakeUpBackoffNewMessageAndManualTriggerReset(t *testing.T) {
	ctx, repo, app, owner, self, conversation := durableReplyFixture(t, "backoff-reset")
	if _, err := app.EnsureWakeUpIntents(ctx); err != nil {
		t.Fatal(err)
	}
	seedWakeUpBackoffForTest(t, ctx, repo, self, time.Now().UTC())
	if _, err := app.AcceptTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": self, "text": "新的消息", "turn_id": "backoff-reset-turn", "idempotency_key": "backoff-reset-message", "attachment_refs": []any{}}); err != nil {
		t.Fatal(err)
	}
	var raw []byte
	var due time.Time
	if err := repo.Pool().QueryRow(ctx, `SELECT payload,next_attempt_at FROM public.platform_workflow_intents WHERE intent_id=$1`, "wake_up_intent:"+self).Scan(&raw, &due); err != nil {
		t.Fatal(err)
	}
	if p := decodeObject(raw); p["idle_noop_streak"] != nil || p["idle_backoff_until"] != nil || time.Until(due) > 10*time.Minute+time.Second {
		t.Fatalf("new message retained backoff: %s due=%s", raw, due)
	}
	seedWakeUpBackoffForTest(t, ctx, repo, self, time.Now().UTC())
	if result, err := app.TriggerWakeUp(ctx, owner, self); err != nil || result["status"] != "queued" {
		t.Fatalf("manual trigger: %#v %v", result, err)
	}
	if err := repo.Pool().QueryRow(ctx, `SELECT payload,next_attempt_at FROM public.platform_workflow_intents WHERE intent_id=$1`, "wake_up_intent:"+self).Scan(&raw, &due); err != nil {
		t.Fatal(err)
	}
	if p := decodeObject(raw); p["idle_backoff_until"] != nil || p["idle_noop_streak"] != nil || time.Until(due) > time.Second {
		t.Fatalf("manual trigger retained backoff: %s due=%s", raw, due)
	}
}

func TestWakeUpBackoffNewNativeFactResetsButReplayPreserves(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	if _, err := f.app.EnsureWakeUpIntents(f.ctx); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	seedWakeUpBackoffForTest(t, f.ctx, f.repository, f.fluctlightID, now)
	enqueue := func() {
		t.Helper()
		if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
			_, err := f.app.enqueueNativeFactTx(f.ctx, tx, f.fluctlightID, f.conversationID, "backoff-source", "life.presence.updated", "backoff-native", map[string]any{})
			return err
		}); err != nil {
			t.Fatal(err)
		}
	}
	enqueue()
	var raw []byte
	var due time.Time
	read := func() {
		t.Helper()
		if err := f.repository.Pool().QueryRow(f.ctx, `SELECT payload,next_attempt_at FROM public.platform_workflow_intents WHERE intent_id=$1`, "wake_up_intent:"+f.fluctlightID).Scan(&raw, &due); err != nil {
			t.Fatal(err)
		}
	}
	read()
	if decodeObject(raw)["idle_backoff_until"] != nil || time.Until(due) > 10*time.Minute+time.Second {
		t.Fatalf("new fact retained backoff: %s due=%s", raw, due)
	}
	seedWakeUpBackoffForTest(t, f.ctx, f.repository, f.fluctlightID, now)
	enqueue()
	if err := f.app.scheduleWakeUpAfterCognition(f.ctx, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	read()
	if intValue(decodeObject(raw)["idle_noop_streak"]) != 3 || due.Sub(now.Add(time.Hour)).Abs() > time.Millisecond {
		t.Fatalf("replay erased newer backoff: %s due=%s", raw, due)
	}
}

func TestWakeUpBackoffOutcomeAndScheduleBoundaries(t *testing.T) {
	now := time.Now().UTC()
	projection := ContextProjection{Schedule: map[string]any{"items": []any{map[string]any{"start_at": now.Add(7 * time.Minute).Format(time.RFC3339Nano), "end_at": now.Add(30 * time.Minute).Format(time.RFC3339Nano)}}}}
	decision := wakeUpBackoffForOutcome(context.Background(), nil, projection, agentCommittedOutcome{}, "no_op", "", now)
	if !decision.Idle || !decision.Deadline.Equal(now.Add(7*time.Minute)) {
		t.Fatalf("schedule boundary lost: %#v", decision)
	}
	ctx := context.WithValue(context.Background(), goalExecutionObligationsKey{}, []map[string]any{{"plan_id": "offered"}})
	if wakeUpBackoffForOutcome(ctx, nil, projection, agentCommittedOutcome{}, "no_op", "", now).Idle {
		t.Fatal("offered Goal obligation grew idle streak")
	}
	for _, status := range []string{"failed", "cancelled", "disabled"} {
		if wakeUpBackoffForOutcome(context.Background(), nil, projection, agentCommittedOutcome{}, status, "", now).Idle {
			t.Fatalf("%s grew idle streak", status)
		}
	}
}

func TestWakeUpBackoffPendingGoalCapsDurableDue(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	if _, err := f.app.EnsureWakeUpIntents(f.ctx); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	goalID := createDialogueGoalForClosure(t, f, []string{"actual evidence"})
	review := now.Add(6 * time.Minute)
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlight_goals SET execution_hint=$2 WHERE id=$1`, goalID, jsonBytes(map[string]any{"execution_handoff": map[string]any{"status": "pending", "attempts": 0, "not_before": now.Add(3 * time.Minute).Format(time.RFC3339Nano)}, "next_review_at": review.Format(time.RFC3339Nano)})); err != nil {
		t.Fatal(err)
	}
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		due, err := updateWakeUpNextDueTx(f.ctx, tx, f.fluctlightID, 1, 600, now, wakeUpBackoffDecision{Idle: true})
		if err == nil && !due.Equal(review) {
			t.Fatalf("pending Goal delayed: due=%s want=%s", due, review)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
}

func TestWakeUpBackoffScheduleDeadlineAndActiveCycleReset(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	if _, err := f.app.EnsureWakeUpIntents(f.ctx); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	boundary := now.Add(4 * time.Minute)
	projection := ContextProjection{ActiveActivities: []map[string]any{{"active_until": boundary.Format(time.RFC3339Nano)}}}
	decision := wakeUpBackoffForOutcome(f.ctx, nil, projection, agentCommittedOutcome{}, "no_op", "", now)
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		due, err := updateWakeUpNextDueTx(f.ctx, tx, f.fluctlightID, 1, 600, now, decision)
		if err == nil && !due.Equal(boundary) {
			t.Fatalf("activity boundary delayed: due=%s want=%s", due, boundary)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	// Actual work, sleeping checks and failures use the existing base cadence.
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		due, err := updateWakeUpNextDueTx(f.ctx, tx, f.fluctlightID, 2, 600, boundary, wakeUpBackoffDecision{})
		if err == nil && !due.Equal(boundary.Add(10*time.Minute)) {
			t.Fatalf("active cycle kept idle delay: %s", due)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	var raw []byte
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT payload FROM public.platform_workflow_intents WHERE intent_id=$1`, "wake_up_intent:"+f.fluctlightID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if p := decodeObject(raw); p["idle_noop_streak"] != nil || p["idle_backoff_until"] != nil {
		t.Fatalf("active cycle kept streak: %s", raw)
	}
}

func TestWakeUpBackoffSupersededRecoveryKeepsFutureDue(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	if _, err := f.app.EnsureWakeUpIntents(f.ctx); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	for _, recovery := range []string{"ensure", "audit"} {
		t.Run(recovery, func(t *testing.T) {
			seedWakeUpBackoffForTest(t, f.ctx, f.repository, f.fluctlightID, now)
			if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.platform_workflow_intents SET status='superseded' WHERE intent_id=$1`, "wake_up_intent:"+f.fluctlightID); err != nil {
				t.Fatal(err)
			}
			var err error
			if recovery == "ensure" {
				_, err = f.app.EnsureWakeUpIntents(f.ctx)
			} else {
				_, err = f.app.AuditWakeUpClocks(f.ctx, 10)
			}
			if err != nil {
				t.Fatal(err)
			}
			var status string
			var due time.Time
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status,next_attempt_at FROM public.platform_workflow_intents WHERE intent_id=$1`, "wake_up_intent:"+f.fluctlightID).Scan(&status, &due); err != nil || status != "completed" || due.Sub(now.Add(time.Hour)).Abs() > time.Millisecond {
				t.Fatalf("recovery advanced future backoff: status=%s due=%s err=%v", status, due, err)
			}
		})
	}
}
