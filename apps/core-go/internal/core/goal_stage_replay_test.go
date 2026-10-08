package core

import (
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestGoalStageCompletionReplayDoesNotRepeatNextStage(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	goalID := createDialogueGoalForClosure(t, f, []string{"a later mutual decision"})
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		goal, err := loadGoalAuthorityTx(f.ctx, tx, f.fluctlightID, "goal:ctx_"+stableDigest(goalID), ContextReference{EntityID: goalID, Revision: 1})
		if err != nil {
			return err
		}
		_, err = applyGoalPlanTx(f.ctx, tx, goal, GoalPlanCandidate{GoalID: goalID, ExpectedRevision: 1, CriteriaVersion: goal.CriteriaVersion, Reason: "current expression stage", NextStep: "express once", Stage: &GoalStagePlan{Operation: "create", Purpose: "express", Strategy: "communicate once", Criteria: []string{"actual expression"}, Reason: "current step"}, Commitment: &GoalCommitmentPlan{ExpectedResult: "actual expression", Criteria: []string{"actual expression"}}}, "replay-stage-plan", f.app.now())
		return err
	}); err != nil {
		t.Fatal(err)
	}
	messageID := insertGoalBoundaryMessage(t, f)
	seedCognitiveProviderRole(t, f.ctx, f.repository, "stage-replay-provider-"+f.suffix)
	assessments := 0
	firstStage := ""
	f.app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().on("goal_evaluation_v1", func(_ map[string]any) fakeProviderResult {
		assessments++
		snapshot := readProcessingGoalSnapshot(t, f)
		entry := snapshot.Goals[0]
		proof := ""
		for _, source := range snapshot.Sources {
			if source.ID == messageID && source.Valid {
				proof = source.Ref
			}
		}
		if proof == "" {
			t.Fatal("duplicate assessment lost original source")
		}
		candidate := GoalEvaluationCandidate{GoalID: goalID, ExpectedRevision: entry.Goal.Revision, CriteriaVersion: entry.Goal.CriteriaVersion, Judgments: []GoalCriterionJudgment{}, Impact: "needs_evidence", WaitCondition: "await mutual decision"}
		plans := []GoalPlanCandidate{}
		if assessments == 1 {
			stage := entry.Stages[0]
			firstStage = stage.ID
			candidate.StageEvaluation = &GoalObjectEvaluation{ID: stage.ID, ExpectedRevision: stage.Revision, CriteriaVersion: stage.CriteriaVersion, Completed: true, Reason: "actual expression", Judgments: []GoalCriterionJudgment{{CriterionID: stage.Criteria[0].ID, Verdict: "satisfied", Kind: "communication", Subject: "actor_self", Discourse: "assertion", EvidenceRefs: []string{proof}, Reason: "sent actual message"}}}
			plans = append(plans, GoalPlanCandidate{GoalID: goalID, ExpectedRevision: entry.Goal.Revision, CriteriaVersion: entry.Goal.CriteriaVersion, Reason: "next stage after actual expression", WaitCondition: "await voluntary response", Stage: &GoalStagePlan{Operation: "create", Purpose: "await response", Strategy: "respect choice", Criteria: []string{"actual response"}, Reason: "expression completed", DependencyIDs: []string{stage.ID}}})
		} else {
			if len(entry.Stages) != 1 || entry.Stages[0].ID == firstStage {
				t.Fatalf("completed stage leaked into snapshot: %#v", entry.Stages)
			}
		}
		return fakeProviderResult{Structured: decodeObject(jsonBytes(GoalEvaluationTaskOutput{Evaluations: []GoalEvaluationCandidate{candidate}, Plans: plans}))}
	})}
	firstRequest := latestPendingGoalRequest(t, f)
	if _, err := f.app.ProcessGoalEvaluationIntent(f.ctx, firstRequest); err != nil {
		t.Fatal(err)
	}
	var replayID string
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		var err error
		replayID, err = queueGoalEvaluationTx(f.ctx, tx, f.fluctlightID, "default", "duplicate_source_delivery", "distinct-redelivery", []string{goalID})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if replayID == firstRequest {
		t.Fatal("test must use a different delivery identity")
	}
	if _, err := f.app.ProcessGoalEvaluationIntent(f.ctx, replayID); err != nil {
		t.Fatal(err)
	}
	stages, commitments, err := readGoalObjectsWith(f.ctx, f.repository.Pool(), f.fluctlightID, goalID)
	if err != nil || len(stages) != 2 || len(commitments) != 1 {
		t.Fatalf("replay repeated plan: %#v %#v %v", stages, commitments, err)
	}
	for _, stage := range stages {
		if stage.ID == firstStage && (stage.Status != "completed" || stage.Revision != 2) {
			t.Fatalf("completed stage changed twice: %#v", stage)
		}
		if stage.ID != firstStage && (stage.Status != "active" || stage.Revision != 1 || len(stage.DependencyIDs) != 1 || stage.DependencyIDs[0] != firstStage) {
			t.Fatalf("dependency or next stage changed: %#v", stage)
		}
	}
	if commitments[0].Status != "cancelled" || commitments[0].Revision != 2 {
		t.Fatalf("redundant commitment repeated: %#v", commitments[0])
	}
	var status string
	var progress float64
	var objectEvaluations, resolutions int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status,progress FROM public.fluctlight_goals WHERE id=$1`, goalID).Scan(&status, &progress); err != nil {
		t.Fatal(err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.goal_evaluations WHERE stage_id=$1`, firstStage).Scan(&objectEvaluations); err != nil {
		t.Fatal(err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.goal_resolutions WHERE goal_id=$1`, goalID).Scan(&resolutions); err != nil {
		t.Fatal(err)
	}
	if status != "active" || progress != 0 || objectEvaluations != 1 || resolutions != 0 || assessments != 2 {
		t.Fatalf("stage replay polluted goal: %s progress=%f evaluations=%d resolutions=%d calls=%d", status, progress, objectEvaluations, resolutions, assessments)
	}
}
