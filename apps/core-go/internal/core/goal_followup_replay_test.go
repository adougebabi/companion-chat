package core

import (
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestValidResidualFollowupDoesNotDuplicateAcrossCompletionReplay(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	goalID := createDialogueGoalForClosure(t, f, []string{"actual expression"})
	messageID := insertGoalBoundaryMessage(t, f)
	requestID := latestPendingGoalRequest(t, f)
	snapshot, _, err := f.app.claimGoalEvaluation(f.ctx, requestID)
	if err != nil {
		t.Fatal(err)
	}
	goal := snapshot.Goals[0].Goal
	sources := map[string]GoalSource{}
	proof := ""
	for _, source := range snapshot.Sources {
		sources[source.Ref] = source
		if source.ID == messageID {
			proof = source.Ref
		}
	}
	candidate := GoalEvaluationCandidate{GoalID: goalID, ExpectedRevision: goal.Revision, CriteriaVersion: goal.CriteriaVersion, Impact: "completed", ResidualMotivation: "still understand each other's expectations", Followup: &GoalFollowupCandidate{DesiredOutcome: "understand future expectations", SuccessCriteria: []string{"actual voluntary discussion"}, Motivation: "unresolved mutual understanding"}, Judgments: []GoalCriterionJudgment{{CriterionID: goal.CriterionIDs[0], Verdict: "satisfied", Kind: "communication", Subject: "actor_self", Discourse: "assertion", EvidenceRefs: []string{proof}, Reason: "actual expression"}}}
	current := goal
	for _, delivery := range []string{requestID, requestID, "distinct-completion-redelivery"} {
		if delivery == "distinct-completion-redelivery" {
			candidate.ExpectedRevision = current.Revision
		}
		if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
			var err error
			current, err = f.app.commitGoalEvaluationTx(f.ctx, tx, current, candidate, sources, delivery)
			return err
		}); err != nil {
			t.Fatal(err)
		}
	}
	var goals, resolutions, evaluations, events int
	var disposition string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.fluctlight_goals WHERE fluctlight_id=$1`, f.fluctlightID).Scan(&goals); err != nil {
		t.Fatal(err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*),max(disposition) FROM public.goal_resolutions WHERE goal_id=$1`, goalID).Scan(&resolutions, &disposition); err != nil {
		t.Fatal(err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.goal_evaluations WHERE goal_id=$1`, goalID).Scan(&evaluations); err != nil {
		t.Fatal(err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.platform_outbox_events WHERE aggregate_id=$1 AND kind='goal.evaluated'`, goalID).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if goals != 1 || resolutions != 1 || evaluations != 1 || events != 1 || disposition != "planner_review_requested" {
		t.Fatalf("completion replay duplicated followup: goals=%d resolutions=%d evaluations=%d events=%d %s", goals, resolutions, evaluations, events, disposition)
	}
}
