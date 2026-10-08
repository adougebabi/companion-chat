package core

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestInitializedLongGoalQueuesOneFocusedPlanWithoutMechanicalSchedules(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		return f.app.insertAgency(f.ctx, tx, f.fluctlightID, f.ownerID, []any{map[string]any{"description": "长期提升绘画能力", "success_criteria": []string{"完成有实际作品的学习目标"}, "motivation": "持续创作"}}, nil, map[string]struct{}{"default": {}})
	}); err != nil {
		t.Fatal(err)
	}
	requestID := latestPendingGoalRequest(t, f)
	var intents int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.platform_workflow_intents WHERE intent_id=$1 AND intent_type='goal.evaluate'`, requestID).Scan(&intents); err != nil || intents != 1 {
		t.Fatalf("initial Goal planning not durable: %d %v", intents, err)
	}
	seedCognitiveProviderRole(t, f.ctx, f.repository, "initial-plan-"+f.suffix)
	end := f.app.now().Add(48 * time.Hour)
	f.app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().on("goal_evaluation_v1", func(_ map[string]any) fakeProviderResult {
		snapshot := readProcessingGoalSnapshot(t, f)
		entry := snapshot.Goals[0]
		if len(entry.Stages) != 0 || len(entry.Commitments) != 0 {
			t.Fatal("initialization fabricated preplanned objects")
		}
		return fakeProviderResult{Structured: decodeObject(jsonBytes(GoalEvaluationTaskOutput{Evaluations: []GoalEvaluationCandidate{{GoalID: entry.GoalID, ExpectedRevision: entry.Goal.Revision, CriteriaVersion: entry.Goal.CriteriaVersion, Judgments: []GoalCriterionJudgment{}, Impact: "needs_evidence", WaitCondition: "wait for a suitable study opportunity"}}, Plans: []GoalPlanCandidate{{GoalID: entry.GoalID, ExpectedRevision: entry.Goal.Revision, CriteriaVersion: entry.Goal.CriteriaVersion, Reason: "one current learning stage", WaitCondition: "wait for suitable opportunity", Stage: &GoalStagePlan{Operation: "create", Purpose: "observe current skills", Strategy: "one small exercise", Criteria: []string{"actual exercise feedback"}, Reason: "focused current stage"}, Commitment: &GoalCommitmentPlan{ExpectedResult: "one actual exercise", Criteria: []string{"actual exercise"}, WindowEnd: &end}}}}))}
	})}
	if _, err := f.app.ProcessGoalEvaluationIntent(f.ctx, requestID); err != nil {
		t.Fatal(err)
	}
	goalID := "goal_initial_" + f.fluctlightID + "_0"
	stages, commitments, err := readGoalObjectsWith(f.ctx, f.repository.Pool(), f.fluctlightID, goalID)
	if err != nil || len(stages) != 1 || len(commitments) != 1 || stages[0].Status != "active" || commitments[0].Status != "active" {
		t.Fatalf("focused plan missing: %#v %#v %v", stages, commitments, err)
	}
	for _, table := range []string{"public.life_schedules", "public.life_schedule_items", "public.fluctlight_intentions"} {
		var count int
		predicate := "fluctlight_id=$1"
		if table == "public.life_schedule_items" {
			predicate = "schedule_id IN (SELECT id FROM public.life_schedules WHERE fluctlight_id=$1)"
		}
		if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM `+table+` WHERE `+predicate, f.fluctlightID).Scan(&count); err != nil || count != 0 {
			t.Fatalf("planning created mechanical schedule/action %s: %d %v", table, count, err)
		}
	}
}

func TestNeutralGoalReviewCyclesDoNotPenalizeWaitingOrCreateActions(t *testing.T) {
	for _, category := range []string{"no_opportunity", "sleeping", "waiting_external"} {
		t.Run(category, func(t *testing.T) {
			f := seedWardrobeToolFixture(t)
			id := createDialogueGoalForClosure(t, f, []string{"actual result"})
			date, start, end, err := actorGoalReviewWindow(f.app.now(), "Asia/Shanghai")
			if err != nil {
				t.Fatal(err)
			}
			for i := 1; i <= 2; i++ {
				oldStart := start.AddDate(0, 0, -i)
				if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.goal_reviews(id,fluctlight_id,goal_id,local_date,timezone,window_start,window_end,status,reason_category,decision,explanation) VALUES($1,$2,$3,$4,'Asia/Shanghai',$5,$6,'succeeded',$7,'wait','neutral prior cycle')`, fmt.Sprintf("neutral-%s-%d", f.suffix, i), f.fluctlightID, id, oldStart.In(mustGoalReviewLocation(t, "Asia/Shanghai")).Format("2006-01-02"), oldStart, oldStart.AddDate(0, 0, 1), category); err != nil {
					t.Fatal(err)
				}
			}
			review := GoalReviewContext{ID: "neutral-current-" + f.suffix, GoalID: id, LocalDate: date, Timezone: "Asia/Shanghai", WindowStart: start, WindowEnd: end, Revision: 1}
			if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.goal_reviews(id,fluctlight_id,goal_id,local_date,timezone,window_start,window_end) VALUES($1,$2,$3,$4,$5,$6,$7)`, review.ID, f.fluctlightID, id, date, review.Timezone, start, end); err != nil {
				t.Fatal(err)
			}
			if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
				goal, err := loadGoalAuthorityTx(f.ctx, tx, f.fluctlightID, "goal:ctx_"+stableDigest(id), ContextReference{EntityID: id, Revision: 1})
				if err != nil {
					return err
				}
				return commitGoalReviewTx(f.ctx, tx, goal, review, &GoalReviewDecision{ReasonCategory: category, Decision: "wait", Explanation: "reasonable waiting, no failure penalty", EvidenceRefs: []string{}}, map[string]GoalSource{}, "", nil)
			}); err != nil {
				t.Fatal(err)
			}
			var status, decision string
			var revision int
			var progress float64
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT g.status,g.revision,g.progress,r.decision FROM public.fluctlight_goals g JOIN public.goal_reviews r ON r.goal_id=g.id WHERE r.id=$1`, review.ID).Scan(&status, &revision, &progress, &decision); err != nil {
				t.Fatal(err)
			}
			if status != "active" || revision != 1 || progress != 0 || decision != "wait" {
				t.Fatalf("neutral cycle penalized Goal: %s rev=%d progress=%f %s", status, revision, progress, decision)
			}
			for _, table := range []string{"public.fluctlight_intentions", "public.life_schedule_items"} {
				var count int
				predicate := "fluctlight_id=$1"
				if table == "public.life_schedule_items" {
					predicate = "schedule_id IN (SELECT id FROM public.life_schedules WHERE fluctlight_id=$1)"
				}
				if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM `+table+` WHERE `+predicate, f.fluctlightID).Scan(&count); err != nil || count != 0 {
					t.Fatalf("neutral review invented action %s: %d %v", table, count, err)
				}
			}
		})
	}
}
