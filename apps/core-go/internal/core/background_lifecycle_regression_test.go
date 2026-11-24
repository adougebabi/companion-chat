package core

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestScheduleBracketTimestampPreservesExplicitOffset(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	day := time.Date(2026, 11, 24, 0, 0, 0, 0, loc)
	for _, value := range []string{"2026-11-24[15:00:00.000+00:00]", "2026-11-24[23:00:00+08:00]", "2026-11-24[23:00:00]"} {
		got, err := parseScheduleTimeInLocation(value, day, loc)
		want := time.Date(2026, 11, 24, 23, 0, 0, 0, loc)
		if err != nil || !got.Equal(want) {
			t.Errorf("%s: got %v, %v; want %v", value, got, err, want)
		}
	}
	for _, value := range []string{"2026-11-24[25:00:00+08:00]", "2026-11-24[15:00]junk", "2026-11-24[15:00[00]]"} {
		if _, err := parseScheduleTimeInLocation(value, day, loc); err == nil {
			t.Errorf("accepted ambiguous/invalid timestamp %s", value)
		}
	}
}

func TestDailyGoalReviewClockUsesLocalThreeAM(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Shanghai")
	created := time.Date(2026, 11, 23, 9, 0, 0, 0, loc)
	for _, tc := range []struct {
		at  string
		due bool
	}{{"2026-11-24T02:59:59+08:00", false}, {"2026-11-24T03:00:00+08:00", true}, {"2026-11-24T10:25:00+08:00", true}} {
		at, _ := time.Parse(time.RFC3339, tc.at)
		date, _, due, err := dailyGoalReviewClock(at, created, "Asia/Shanghai")
		if err != nil || date != "2026-11-23" || due != tc.due {
			t.Fatalf("%s: %s %t %v", tc.at, date, due, err)
		}
	}
	created = time.Date(2026, 11, 24, 9, 0, 0, 0, loc)
	if _, _, due, err := dailyGoalReviewClock(created.Add(time.Hour), created, "Asia/Shanghai"); err != nil || due {
		t.Fatal("new personality received an overdue daily review", due, err)
	}
	for _, date := range []string{"2026-03-08", "2026-11-01"} {
		zone, _ := time.LoadLocation("America/New_York")
		day, _ := time.ParseInLocation("2006-01-02", date, zone)
		at := time.Date(day.Year(), day.Month(), day.Day(), 3, 0, 0, 0, zone)
		_, previous, due, err := dailyGoalReviewClock(at, day.AddDate(0, 0, -1), zone.String())
		if err != nil || !due || previous.In(zone).Format("2006-01-02") != day.AddDate(0, 0, -1).Format("2006-01-02") {
			t.Fatal(date, previous, due, err)
		}
	}
}

func TestDailyGoalReviewPersistsOnceAndAdmitsWithoutNewActivity(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	plannerOwnerGoal(t, f, "daily-clock", false)
	loc, _ := time.LoadLocation("Asia/Shanghai")
	day := time.Now().In(loc)
	due := time.Date(day.Year(), day.Month(), day.Day(), 3, 0, 0, 0, loc)
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlights SET created_at=$2 WHERE id=$1`, f.fluctlightID, due.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_evaluation_requests SET status='succeeded' WHERE fluctlight_id=$1`, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_agent_cadence SET activity_revision=7,evaluation_activity_revision=7,evaluation_next_at=now(),last_activity_at=now()-interval '1 hour' WHERE fluctlight_id=$1`, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	queue := func(at time.Time) int {
		t.Helper()
		n := 0
		if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error { var err error; n, err = queueDailyGoalReviewsTx(f.ctx, tx, at, 20); return err }); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if n := queue(due.Add(-time.Second)); n != 0 {
		t.Fatal("queued before 03:00", n)
	}
	// A pre-existing review may have been queued by yesterday's reflection,
	// then skipped for having no new activity. The daily gate must rebind it.
	var legacy string
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		var err error
		legacy, err = queueGoalReviewsTx(f.ctx, tx, f.fluctlightID, "default", "reflection", due.AddDate(0, 0, -1))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	advanceGoalEvaluationCycleForTest(t, f, legacy)
	if _, prior, err := f.app.claimGoalEvaluation(f.ctx, legacy); err != nil || prior["reason"] != "awaiting_new_activity" {
		t.Fatal(prior, err)
	}
	if n := queue(due); n != 1 {
		t.Fatal("daily review not queued", n)
	}
	if n := queue(due.Add(time.Hour)); n != 0 {
		t.Fatal("daily review duplicated", n)
	}
	var id string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT daily_review_request_id FROM public.goal_agent_cadence WHERE fluctlight_id=$1`, f.fluctlightID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	advanceGoalEvaluationCycleForTest(t, f, id)
	snapshot, prior, err := f.app.claimGoalEvaluation(f.ctx, id)
	if err != nil || prior != nil || len(snapshot.Reviews) == 0 {
		t.Fatalf("daily claim without new activity: %#v %#v %v", snapshot, prior, err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_evaluation_requests SET status='succeeded',claimed_at=NULL WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	if n := queue(due.AddDate(0, 0, 1)); n != 1 {
		t.Fatal("next day review missing", n)
	}
}

func TestDailyGoalReviewCompletedWindowAdvancesMarker(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	plannerOwnerGoal(t, f, "completed-daily", false)
	loc, _ := time.LoadLocation("Asia/Shanghai")
	now := time.Now().In(loc)
	due := time.Date(now.Year(), now.Month(), now.Day(), 3, 0, 0, 0, loc)
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlights SET created_at=$2 WHERE id=$1`, f.fluctlightID, due.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		_, err := queueGoalReviewsTx(f.ctx, tx, f.fluctlightID, "default", "reflection", due.AddDate(0, 0, -1))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_reviews SET status='succeeded' WHERE fluctlight_id=$1`, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_evaluation_requests SET status='succeeded' WHERE fluctlight_id=$1`, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		count := 0
		if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
			var err error
			count, err = queueDailyGoalReviewsTx(f.ctx, tx, due, 1)
			return err
		}); err != nil || count != 1-i {
			t.Fatal("completed window starves scan", count, err)
		}
	}
	var marked bool
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT daily_review_date=$2::date AND daily_review_request_id IS NULL FROM public.goal_agent_cadence WHERE fluctlight_id=$1`, f.fluctlightID, due.AddDate(0, 0, -1).Format("2006-01-02")).Scan(&marked); err != nil || !marked {
		t.Fatal(marked, err)
	}
}

func TestNoActivityReviewAndToolRequestsSkipModelAdmission(t *testing.T) {
	for _, reason := range []string{"goal_review:daily_review", "goal_review:reflection", "tool_evaluation"} {
		t.Run(reason, func(t *testing.T) {
			f := seedWardrobeToolFixture(t)
			plannerOwnerGoal(t, f, "idle-review", false)
			if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_evaluation_requests SET status='succeeded' WHERE fluctlight_id=$1`, f.fluctlightID); err != nil {
				t.Fatal(err)
			}
			if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_agent_cadence SET evaluation_activity_revision=activity_revision,evaluation_next_at=now(),last_activity_at=now()-interval '1 hour' WHERE fluctlight_id=$1`, f.fluctlightID); err != nil {
				t.Fatal(err)
			}
			var id string
			if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
				var err error
				id, err = queueGoalEvaluationTx(f.ctx, tx, f.fluctlightID, "default", reason, "idle-review", nil)
				return err
			}); err != nil {
				t.Fatal(err)
			}
			advanceGoalEvaluationCycleForTest(t, f, id)
			_, prior, err := f.app.claimGoalEvaluation(f.ctx, id)
			if err != nil || prior["reason"] != "awaiting_new_activity" || prior["model_admission"] != false {
				t.Fatal(prior, err)
			}
		})
	}
}

func TestWakeUpStopsIdenticalRejectedArgumentsAndAllowsCorrection(t *testing.T) {
	trace := &ADKCapabilityTrace{}
	invoker := &appADKCapabilityInvoker{trace: trace, scopeRegistry: mustCapabilityRegistry(builtinCapabilities(nil)...)}
	ctx := withADKCapabilityContext(context.Background(), invoker, trace, nil)
	args := jsonBytes(map[string]any{"kind": "virtual_shopping", "reason": "test"})
	for i := 0; i < 3; i++ {
		id := fmt.Sprint(i)
		trace.Append(CapabilityInvocation{CallID: id, CapabilityName: lifeActivityStartCapabilityName, Arguments: args}, CapabilityResult{CallID: id, CapabilityName: lifeActivityStartCapabilityName, Status: "handled", Output: map[string]any{"effect_applied": false, "retry_same_arguments": false, "warning_code": "tool_precondition_unresolved"}})
		if !wakeUpRepeatedRejectedArguments(trace, lifeActivityStartCapabilityName, args) {
			t.Fatal("ignored retry_same_arguments=false")
		}
	}
	if wakeUpRepeatedRejectedArguments(trace, lifeActivityStartCapabilityName, jsonBytes(map[string]any{"kind": "haircut", "reason": "corrected"})) {
		t.Fatal("blocked changed arguments")
	}
	if _, err := (wakeUpPhysicalRequestPolicy{}).DecidePhysicalModelRequest(ctx); err == nil || !strings.Contains(err.Error(), "tool_progress_stalled") {
		t.Fatal("unbounded stalled wake", err)
	}
	trace.Append(CapabilityInvocation{CallID: "progress", CapabilityName: lifeActivityStartCapabilityName}, CapabilityResult{CallID: "progress", CapabilityName: lifeActivityStartCapabilityName, Status: "accepted"})
	decision, err := (wakeUpPhysicalRequestPolicy{}).DecidePhysicalModelRequest(ctx)
	if err != nil || decision.AllowedToolNames != nil || wakeUpRepeatedRejectedArguments(trace, lifeActivityStartCapabilityName, args) {
		t.Fatal("progress did not reopen ordinary selection", decision, err)
	}
}

func TestWakeUpNoProgressLoopIsBoundedThroughRealAgent(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	seedCognitiveProviderRole(t, f.ctx, f.repository, "bounded-wake-"+f.suffix)
	f.seedAcceptedSchedule(t)
	if _, err := f.app.EnsureWakeUpIntents(f.ctx); err != nil {
		t.Fatal(err)
	}
	calls := 0
	router := newFakeProviderRouter().on("wake_up_response", func(map[string]any) fakeProviderResult {
		calls++
		if calls > 5 {
			return fakeProviderResult{Status: 500}
		}
		return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall(fmt.Sprint("bad-", calls), lifeActivityStartCapabilityName, map[string]any{"kind": "virtual_shopping", "intention_ref": "intention:ctx_0123456789abcdef0123456789abcdef", "reason": "test missing intention"})}}
	})
	f.app.Provider.HTTP = &http.Client{Transport: router}
	_, err := f.app.ProcessWakeUp(f.ctx, f.fluctlightID, 1)
	if err == nil || !strings.Contains(err.Error(), "tool_progress_stalled") || calls > 3 {
		t.Fatalf("unbounded real Agent loop: calls=%d err=%v", calls, err)
	}
	var effects int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.fluctlight_life_activity_runs WHERE fluctlight_id=$1`, f.fluctlightID).Scan(&effects); err != nil || effects != 0 {
		t.Fatal(effects, err)
	}
	spent := calls
	if _, err := f.app.ProcessWakeUp(f.ctx, f.fluctlightID, 1); err != nil || calls != spent {
		t.Fatal("replay repeated provider calls", calls, err)
	}
}

func TestFailedWakeUpWithoutEffectsBacksOff(t *testing.T) {
	registry := mustCapabilityRegistry(builtinCapabilities(nil)...)
	outcome := agentCommittedOutcome{Results: []CapabilityResult{{CapabilityName: lifeActivityStartCapabilityName, Status: "handled", Output: map[string]any{"effect_applied": false}}}}
	decision := wakeUpBackoffForOutcome(context.Background(), registry, ContextProjection{}, outcome, "failed", "agent_run_failed", time.Now())
	if !decision.Idle {
		t.Fatal("failed no-effect cycle retained hot retry clock")
	}
}

func TestAutomaticGoalReviewsCannotBypassNewActivity(t *testing.T) {
	for _, reason := range []string{"goal_review:daily_review", "goal_review:reflection", "tool_evaluation"} {
		if goalEvaluationAllowsNoNewActivity(reason) {
			t.Errorf("%s bypassed new activity without daily admission", reason)
		}
	}
}

func TestDueInfluenceRefreshKeepsEntityAuthority(t *testing.T) {
	expected := ContextReference{Kind: ContextReferenceIntention, EntityID: "due-intention", Revision: 2}
	current := ContextReferenceIndex{ByRef: map[string]ContextReference{
		"new-ref":    {Kind: ContextReferenceIntention, EntityID: "due-intention", Revision: 3},
		"other-ref":  {Kind: ContextReferenceIntention, EntityID: "other-intention", Revision: 3},
		"wrong-kind": {Kind: ContextReferenceGoal, EntityID: "due-intention", Revision: 3},
	}}
	if !decisionReferencesDueEntity([]string{"new-ref"}, current, expected) {
		t.Fatal("committed revision lost the original due entity")
	}
	for _, ref := range []string{"old-ref", "other-ref", "wrong-kind"} {
		if decisionReferencesDueEntity([]string{ref}, current, expected) {
			t.Fatal("unoffered or unrelated influence accepted", ref)
		}
	}
}

func TestLifeActivityNativeContractBindsIntentionReference(t *testing.T) {
	contract, ok := any(lifeActivityStartCapability{}).(nativeToolContract)
	if !ok {
		t.Fatal("activity model contract does not bind opaque intention references")
	}
	ref := "intention:ctx_0123456789abcdef0123456789abcdef"
	projection := ContextProjection{ReferenceIndex: ContextReferenceIndex{ActiveProfileID: "current", ByRef: map[string]ContextReference{
		ref: {Kind: ContextReferenceIntention, EntityID: "canonical-intention", Revision: 1, Snapshot: jsonBytes(map[string]any{"profile_id": "current"})},
	}}}
	raw := jsonBytes(map[string]any{"kind": "virtual_shopping", "reason": "a purchase", "intention_ref": ref})
	bound, err := contract.BindNativeArguments(raw, projection)
	if err != nil || stringValue(decodeObject(bound)["intention_id"]) != "canonical-intention" {
		t.Fatalf("binding failed: %s %v", bound, err)
	}
	for _, wrong := range []string{"canonical-intention", "intention:ctx_abcdef0123456789abcdef0123456789", "goal:ctx_0123456789abcdef0123456789abcdef"} {
		if _, err := contract.BindNativeArguments(jsonBytes(map[string]any{"intention_ref": wrong}), projection); err == nil {
			t.Errorf("bound an unoffered reference: %s", wrong)
		}
	}
	legacy, err := contract.BindNativeArguments(jsonBytes(map[string]any{"intention_id": ref}), projection)
	if err != nil || decodeObject(legacy)["intention_id"] != "canonical-intention" {
		t.Fatal("legacy reference spelling was lost", string(legacy), err)
	}
	if _, err := contract.BindNativeArguments(jsonBytes(map[string]any{"intention_id": "canonical-intention"}), projection); err == nil {
		t.Fatal("model DB ID bypassed reference binding")
	}
	entry := projection.ReferenceIndex.ByRef[ref]
	entry.Snapshot = jsonBytes(map[string]any{"profile_id": "other"})
	projection.ReferenceIndex.ByRef[ref] = entry
	if _, err := contract.BindNativeArguments(raw, projection); err == nil {
		t.Fatal("bound a foreign profile intention")
	}
}
