package core

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5"
)

// Advancing stored scheduling timestamps simulates elapsed time in isolated
// database tests. Production admission remains enabled and is exercised again.
func advanceGoalEvaluationCycleForTest(t *testing.T, f independentToolE2EFixture, id string) {
	t.Helper()
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_agent_cadence SET evaluation_next_at=now() WHERE fluctlight_id=$1`, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_evaluation_requests SET available_at=now() WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
}

func TestGoalAgentCadenceEvaluationPendingFrozenAndProfiles(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	goal := plannerOwnerGoal(t, f, "cadence", false)
	id := latestPendingGoalRequest(t, f)
	first, prior, err := f.app.claimGoalEvaluation(f.ctx, id)
	if err != nil || prior != nil {
		t.Fatal(first, prior, err)
	}
	var interval bool
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT evaluation_next_at BETWEEN now()+interval '29 minutes 59 seconds' AND now()+interval '30 minutes' FROM public.goal_agent_cadence WHERE fluctlight_id=$1`, f.fluctlightID).Scan(&interval); err != nil || !interval {
		t.Fatal("not a 30 minute clock", interval, err)
	}
	frozen := first
	frozen.Protocol = goalEvaluationToolProtocol
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_evaluation_requests SET status='retry',claimed_at=NULL,available_at=now(),snapshot=$2 WHERE id=$1`, id, jsonBytes(frozen)); err != nil {
		t.Fatal(err)
	}
	for _, profile := range []string{"default", "other-profile"} {
		request := id
		if profile != "default" {
			request = "cross-profile-" + f.suffix
			if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.goal_evaluation_requests(id,fluctlight_id,profile_id,reason,status,available_at,snapshot) VALUES($1,$2,$3,'test','pending',now(),$4)`, request, f.fluctlightID, profile, jsonBytes(goalEvaluationSnapshot{Protocol: goalEvaluationToolProtocol, RequestID: request, FluctlightID: f.fluctlightID, ProfileID: profile})); err != nil {
				t.Fatal(err)
			}
		}
		result, err := f.app.ProcessGoalEvaluationIntent(f.ctx, request)
		if err != nil || result["reason"] != "evaluation_cadence" || result["not_before"] == nil {
			t.Fatal(result, err)
		}
		var attempts int
		if err := f.repository.Pool().QueryRow(f.ctx, `SELECT attempt_count FROM public.goal_evaluation_requests WHERE id=$1`, request).Scan(&attempts); err != nil || attempts != map[bool]int{true: 1, false: 0}[request == id] {
			t.Fatal("defer spent attempt", attempts, err)
		}
	}
	// Exactly one second before the boundary (29m59s) still cannot claim.
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_agent_cadence SET evaluation_next_at=now()+interval '1 second' WHERE fluctlight_id=$1`, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_evaluation_requests SET available_at=now() WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	if _, p, e := f.app.claimGoalEvaluation(f.ctx, id); e != nil || p["status"] != "deferred" {
		t.Fatal(p, e)
	}
	advanceGoalEvaluationCycleForTest(t, f, id)
	next, p, e := f.app.claimGoalEvaluation(f.ctx, id)
	if e != nil || p != nil || next.Protocol != goalEvaluationToolProtocol || next.Goals[0].GoalID != goal {
		t.Fatal("30m frozen claim", next, p, e)
	}
	// A pending producer's own future merge time is also authoritative.
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_evaluation_requests SET status='retry',claimed_at=NULL,available_at=now()+interval '1 hour' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	advanceGoalEvaluationCycleForTest(t, f, id)
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_evaluation_requests SET available_at=now()+interval '1 hour' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	if _, p, e := f.app.claimGoalEvaluation(f.ctx, id); e != nil || p["status"] != "deferred" {
		t.Fatal(p, e)
	}
}

func TestGoalAgentCadencePlannerFullAllSourcesAndOldQueue(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	for i := 0; i < 5; i++ {
		plannerOwnerGoal(t, f, fmt.Sprint("full-", i), false)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_planning_runs SET status='capacity_full' WHERE fluctlight_id=$1`, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	calls := 0
	f.app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().on("goal_planner_v1", func(map[string]any) fakeProviderResult { calls++; return fakeProviderResult{Status: 503} })}
	for i, reason := range []string{"owner_request", "actor_context_changed", "profile_changed", "schedule_accepted_daily", "goal_lifecycle_changed"} {
		if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
			return requestGoalPlanningTx(f.ctx, tx, f.fluctlightID, fmt.Sprint("owner:full-", i), reason, nil)
		}); err != nil {
			t.Fatal(err)
		}
		id := fmt.Sprint("legacy-full-", i, "-", f.suffix)
		if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.goal_planning_runs(id,fluctlight_id,watermark,available_at) VALUES($1,$2,0,now())`, id, f.fluctlightID); err != nil {
			t.Fatal(err)
		}
		result, err := f.app.ProcessGoalPlanningIntent(f.ctx, id)
		if err != nil || result["status"] != "capacity_full" || calls != 0 {
			t.Fatal(reason, result, calls, err)
		}
	}
	if _, err := f.app.RepairGoalPlanning(f.ctx, 20); err != nil {
		t.Fatal(err)
	}
	var consumed int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.goal_planning_events WHERE fluctlight_id=$1 AND processed_run_id IS NOT NULL`, f.fluctlightID).Scan(&consumed); err != nil || consumed != 0 {
		t.Fatal(consumed, err)
	}
}

func TestGoalAgentCadencePlannerFailedDayAndExactPrivateEpisodes(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	seedCognitiveProviderRole(t, f.ctx, f.repository, "cadence-fail-"+f.suffix)
	calls := 0
	f.app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().on("goal_planner_v1", func(map[string]any) fakeProviderResult { calls++; return fakeProviderResult{Status: 503} })}
	if _, err := f.app.RepairGoalPlanning(f.ctx, 20); err != nil {
		t.Fatal(err)
	}
	var id string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT id FROM public.goal_planning_runs WHERE fluctlight_id=$1 AND status='pending'`, f.fluctlightID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_planning_runs SET available_at=now() WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.ProcessGoalPlanningIntent(f.ctx, id); err != nil || calls == 0 {
		t.Fatal(calls, err)
	}
	spent := calls
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_planning_runs SET available_at=now() WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	result, err := f.app.ProcessGoalPlanningIntent(f.ctx, id)
	if err != nil || result["status"] != "awaiting_trigger" || calls != spent {
		t.Fatal("failure reset episode", result, calls, err)
	}
	for i := 0; i < 3; i++ {
		if _, err := f.app.RepairGoalPlanning(f.ctx, 20); err != nil {
			t.Fatal(err)
		}
	}
	var pending int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.goal_planning_runs WHERE fluctlight_id=$1 AND status='pending'`, f.fluctlightID).Scan(&pending); err != nil || pending != 0 {
		t.Fatal(pending, err)
	}
	// A low-sequence private completion survives a later shared daily admission.
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.fluctlight_goals(id,fluctlight_id,status,desired_outcome,description,profile_id) VALUES($1,$2,'completed','done','done','private')`, "done-"+f.suffix, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	var seq int64
	if err := f.repository.Pool().QueryRow(f.ctx, `INSERT INTO public.goal_planning_events(fluctlight_id,source_key,reason,payload) VALUES($1,'private-done','goal_lifecycle_changed',jsonb_build_object('goal_id',$2::text,'status','completed','profile_id','private')) RETURNING seq`, f.fluctlightID, "done-"+f.suffix).Scan(&seq); err != nil {
		t.Fatal(err)
	}
	for _, profile := range []string{"default", "private"} {
		var eligible bool
		if err := f.repository.Pool().QueryRow(f.ctx, `SELECT EXISTS(SELECT 1 FROM public.goal_planner_episode_keys($1,$2,$3))`, f.fluctlightID, profile, seq).Scan(&eligible); err != nil || eligible != (profile == "private") {
			t.Fatal(profile, eligible, err)
		}
	}
	// Local date, rather than a rolling 24-hour window, determines daily keys.
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_planner_admissions SET episode_key='daily:1900-01-01' WHERE fluctlight_id=$1 AND episode_key LIKE 'daily:%'`, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	var eligible bool
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT EXISTS(SELECT 1 FROM public.goal_planner_episode_keys($1,'default',0))`, f.fluctlightID).Scan(&eligible); err != nil || eligible {
		t.Fatal("date alone created an uncaptured episode", eligible, err)
	}
	if _, err := f.app.RepairGoalPlanning(f.ctx, 20); err != nil {
		t.Fatal(err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT EXISTS(SELECT 1 FROM public.goal_planner_episode_keys($1,'default',(SELECT max(seq) FROM public.goal_planning_events WHERE fluctlight_id=$1)))`, f.fluctlightID).Scan(&eligible); err != nil || !eligible {
		t.Fatal("repair failed to capture legal daily event", eligible, err)
	}
}

func TestGoalAgentCadenceEvaluationRepairWithoutNewEvents(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	plannerOwnerGoal(t, f, "quiet-periodic", false)
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_evaluation_requests SET status='succeeded' WHERE fluctlight_id=$1`, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err := f.app.RepairGoalEvaluationCadence(f.ctx, 20); err != nil {
			t.Fatal(err)
		}
	}
	var pending int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.goal_evaluation_requests WHERE fluctlight_id=$1 AND status='pending' AND reason='periodic_due'`, f.fluctlightID).Scan(&pending); err != nil || pending != 1 {
		t.Fatal(pending, err)
	}
}

func TestGoalAgentCadencePlannerHintsNeedCapturedDailyEvent(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	plannerOwnerGoal(t, f, "free-slot-hints", false)
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_planning_events SET processed_run_id='fixture-established' WHERE fluctlight_id=$1`, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_planning_runs SET status='awaiting_trigger' WHERE fluctlight_id=$1`, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	calls := 0
	f.app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().on("goal_planner_v1", func(map[string]any) fakeProviderResult { calls++; return fakeProviderResult{Status: 503} })}
	for i, reason := range []string{"actor_context_changed", "profile_changed", "cognition_wish", "reflection_wish", "goal_lifecycle_changed", "goal_lifecycle_changed"} {
		source := fmt.Sprint("hint-without-day-", i)
		payload := map[string]any{"status": []string{"", "", "", "", "paused", "cancelled"}[i]}
		if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
			return requestGoalPlanningTx(f.ctx, tx, f.fluctlightID, source, reason, payload)
		}); err != nil {
			t.Fatal(err)
		}
		var pending int
		if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.goal_planning_runs WHERE fluctlight_id=$1 AND status='pending'`, f.fluctlightID).Scan(&pending); err != nil || pending != 0 {
			t.Fatal("hint created a daily budget", reason, pending, err)
		}
		// A pre-upgrade queued run must use the identical predicate at claim.
		id := fmt.Sprint("legacy-hint-", i, "-", f.suffix)
		if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.goal_planning_runs(id,fluctlight_id,watermark,available_at) VALUES($1,$2,0,now())`, id, f.fluctlightID); err != nil {
			t.Fatal(err)
		}
		result, err := f.app.ProcessGoalPlanningIntent(f.ctx, id)
		if err != nil || result["status"] != "awaiting_trigger" || calls != 0 {
			t.Fatal(reason, result, calls, err)
		}
		// Even a direct native Agent entry with a valid claim cannot mint a budget.
		if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_planning_runs SET status='processing',claim_revision=1,claimed_at=now(),profile_id='default',mode='apply',watermark=(SELECT max(seq) FROM public.goal_planning_events WHERE fluctlight_id=$2) WHERE id=$1`, id, f.fluctlightID); err != nil {
			t.Fatal(err)
		}
		result, err = f.app.admitGoalPlannerModel(f.ctx, f.fluctlightID, f.ownerID, "default", id+"@1", "apply")
		if err != nil || result["status"] != "awaiting_trigger" {
			t.Fatal("native admission bypass", result, err)
		}
		if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_planning_runs SET status='awaiting_trigger',claimed_at=NULL WHERE id=$1`, id); err != nil {
			t.Fatal(err)
		}
	}
	// Yesterday's schedule cannot grant today's budget, even when a new hint arrives.
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.goal_planning_events(fluctlight_id,source_key,reason,payload) VALUES($1,'old-day','schedule_accepted_daily','{"local_date":"1900-01-01"}')`, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	var eligible bool
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT EXISTS(SELECT 1 FROM public.goal_planner_episode_keys($1,'default',(SELECT max(seq) FROM public.goal_planning_events WHERE fluctlight_id=$1)))`, f.fluctlightID).Scan(&eligible); err != nil || eligible {
		t.Fatal("old day granted current budget", eligible, err)
	}
	// A captured legitimate day event whose queue was lost may be awakened by a hint.
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.goal_planning_events(fluctlight_id,source_key,reason,payload) VALUES($1,'captured-day','schedule_accepted_daily',jsonb_build_object('local_date',public.goal_planner_local_day($1)))`, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		return requestGoalPlanningTx(f.ctx, tx, f.fluctlightID, "wake-captured-day", "actor_context_changed", nil)
	}); err != nil {
		t.Fatal(err)
	}
	var pending int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.goal_planning_runs WHERE fluctlight_id=$1 AND status='pending'`, f.fluctlightID).Scan(&pending); err != nil || pending != 1 || calls != 0 {
		t.Fatal("captured day not woken", pending, calls, err)
	}
	var consumed int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.goal_planning_events WHERE fluctlight_id=$1 AND source_key LIKE 'hint-without-day-%' AND processed_run_id IS NOT NULL`, f.fluctlightID).Scan(&consumed); err != nil || consumed != 0 {
		t.Fatal("hints consumed without model", consumed, err)
	}
}
