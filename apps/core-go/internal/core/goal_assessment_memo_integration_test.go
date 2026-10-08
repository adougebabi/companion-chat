package core

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestGoalAssessmentMemoSkipsDurablyAndOwnerForceOrNewProofReassesses(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	goalID := createDialogueGoalForClosure(t, f, []string{"receive one actual relevant response"})
	seedCognitiveProviderRole(t, f.ctx, f.repository, "goal-memo-provider-"+f.suffix)
	modelCalls := 0
	router := newFakeProviderRouter().on("goal_evaluation_v1", func(_ map[string]any) fakeProviderResult {
		modelCalls++
		snapshot := readProcessingGoalSnapshot(t, f)
		evaluations := make([]GoalEvaluationCandidate, 0, len(snapshot.Goals))
		for _, entry := range snapshot.Goals {
			evaluations = append(evaluations, GoalEvaluationCandidate{
				GoalID: entry.GoalID, ExpectedRevision: entry.Goal.Revision,
				CriteriaVersion: entry.Goal.CriteriaVersion, Judgments: []GoalCriterionJudgment{},
				Impact: "needs_evidence", WaitCondition: "await actual relevant response",
			})
		}
		return fakeProviderResult{Structured: goalEvaluationProviderFixture(snapshot, GoalEvaluationTaskOutput{Evaluations: evaluations, Plans: []GoalPlanCandidate{}})}
	})
	f.app.Provider.HTTP = &http.Client{Transport: router}

	firstRequest := latestPendingGoalRequest(t, f)
	if _, err := f.app.ProcessGoalEvaluationIntent(f.ctx, firstRequest); err != nil {
		t.Fatal(err)
	}
	if modelCalls != 1 {
		t.Fatalf("first assessment calls = %d", modelCalls)
	}
	var memoCount int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT jsonb_array_length(result->'assessment_memos') FROM public.goal_evaluation_requests WHERE id=$1`, firstRequest).Scan(&memoCount); err != nil || memoCount != 1 {
		t.Fatalf("durable memo count = %d err=%v", memoCount, err)
	}

	var replayID string
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		var err error
		replayID, err = queueGoalEvaluationTx(f.ctx, tx, f.fluctlightID, "", "duplicate_source_delivery", "memo-replay-"+f.suffix, []string{goalID})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	replay, err := f.app.ProcessGoalEvaluationIntent(f.ctx, replayID)
	if err != nil || modelCalls != 1 || !containsString(decisionServiceRefValues(replay["skipped_goals"]), goalID) {
		t.Fatalf("durable replay=%#v calls=%d err=%v", replay, modelCalls, err)
	}

	var pendingID, forcedID string
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		var err error
		pendingID, err = queueGoalEvaluationTx(f.ctx, tx, f.fluctlightID, "", "automatic_check", "auto-before-owner-"+f.suffix, []string{goalID})
		if err != nil {
			return err
		}
		forcedID, err = queueGoalEvaluationTx(f.ctx, tx, f.fluctlightID, "", "owner_reassess", "owner-force-"+f.suffix, []string{goalID})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if forcedID != pendingID {
		t.Fatalf("owner reassess did not merge into pending request: pending=%s forced=%s", pendingID, forcedID)
	}
	var reason string
	var rawSnapshot []byte
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT reason,snapshot FROM public.goal_evaluation_requests WHERE id=$1`, pendingID).Scan(&reason, &rawSnapshot); err != nil {
		t.Fatal(err)
	}
	var pendingSnapshot goalEvaluationSnapshot
	if err := json.Unmarshal(rawSnapshot, &pendingSnapshot); err != nil || reason != "owner_reassess" || !containsString(pendingSnapshot.ForcedGoalIDs, goalID) {
		t.Fatalf("merged owner force reason=%q snapshot=%#v err=%v", reason, pendingSnapshot, err)
	}
	if _, err := f.app.ProcessGoalEvaluationIntent(f.ctx, pendingID); err != nil || modelCalls != 2 {
		t.Fatalf("owner force calls=%d err=%v", modelCalls, err)
	}

	insertGoalBoundaryMessage(t, f)
	var newProofRequest string
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		var err error
		newProofRequest, err = queueGoalEvaluationTx(f.ctx, tx, f.fluctlightID, "", "new_proof", "new-proof-"+f.suffix, []string{goalID})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.ProcessGoalEvaluationIntent(f.ctx, newProofRequest); err != nil || modelCalls != 3 {
		t.Fatalf("new proof calls=%d err=%v", modelCalls, err)
	}

	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		var revision int
		if err := tx.QueryRow(f.ctx, `SELECT revision FROM public.fluctlight_goals WHERE id=$1`, goalID).Scan(&revision); err != nil {
			return err
		}
		current, err := loadGoalAuthorityTx(f.ctx, tx, f.fluctlightID, "goal:ctx_"+stableDigest(goalID), ContextReference{EntityID: goalID, Revision: revision})
		if err != nil {
			return err
		}
		next, record, err := ApplyGoalCommand(&current, GoalCommand{Operation: GoalUpdate, ExpectedRevision: current.Revision, Patch: GoalPatch{SuccessCriteria: []string{"receive two actual relevant responses"}}, EvidenceRefs: []string{"owner:criterion-change"}, Reason: "change success standard", OccurredAt: f.app.now()})
		if err != nil {
			return err
		}
		_, err = persistGoalAuthorityTx(f.ctx, tx, &current, next, record, "goal-memo-criterion-change-"+f.suffix)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	criteriaRequest := latestPendingGoalRequest(t, f)
	if _, err := f.app.ProcessGoalEvaluationIntent(f.ctx, criteriaRequest); err != nil || modelCalls != 4 {
		t.Fatalf("changed criteria calls=%d err=%v", modelCalls, err)
	}
}

func TestGoalAssessmentFailureNeverCreatesMemoAndRetriesModel(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	createDialogueGoalForClosure(t, f, []string{"receive one actual relevant response"})
	seedCognitiveProviderRole(t, f.ctx, f.repository, "goal-memo-retry-provider-"+f.suffix)
	modelCalls := 0
	f.app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().on("goal_evaluation_v1", func(_ map[string]any) fakeProviderResult {
		modelCalls++
		if modelCalls == 1 {
			return fakeProviderResult{Status: http.StatusBadGateway}
		}
		snapshot := readProcessingGoalSnapshot(t, f)
		entry := snapshot.Goals[0]
		candidate := GoalEvaluationCandidate{GoalID: entry.GoalID, ExpectedRevision: entry.Goal.Revision, CriteriaVersion: entry.Goal.CriteriaVersion, Judgments: []GoalCriterionJudgment{}, Impact: "needs_evidence", WaitCondition: "await actual relevant response"}
		return fakeProviderResult{Structured: goalEvaluationProviderFixture(snapshot, GoalEvaluationTaskOutput{Evaluations: []GoalEvaluationCandidate{candidate}, Plans: []GoalPlanCandidate{}})}
	})}
	requestID := latestPendingGoalRequest(t, f)
	if _, err := f.app.ProcessGoalEvaluationIntent(f.ctx, requestID); err == nil {
		t.Fatal("provider failure unexpectedly succeeded")
	} else {
		t.Logf("first failure=%v calls=%d", err, modelCalls)
	}
	var memoCount int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT CASE WHEN jsonb_typeof(result->'assessment_memos')='array' THEN jsonb_array_length(result->'assessment_memos') ELSE 0 END FROM public.goal_evaluation_requests WHERE id=$1`, requestID).Scan(&memoCount); err != nil || memoCount != 0 {
		t.Fatalf("failed assessment memo count = %d err=%v", memoCount, err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_evaluation_requests SET available_at=$2 WHERE id=$1`, requestID, time.Now().UTC().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	if result, err := f.app.ProcessGoalEvaluationIntent(f.ctx, requestID); err != nil || modelCalls != 2 {
		t.Fatalf("retry result=%v calls=%d err=%v", result, modelCalls, err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT jsonb_array_length(result->'assessment_memos') FROM public.goal_evaluation_requests WHERE id=$1`, requestID).Scan(&memoCount); err != nil || memoCount != 1 {
		t.Fatalf("successful retry memo count = %d err=%v", memoCount, err)
	}
}

func TestGoalRevisionJournalDoesNotReopenReviewWatermark(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	goalID := createDialogueGoalForClosure(t, f, []string{"receive one actual relevant response"})
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		_, err := queueGoalReviewsTx(f.ctx, tx, f.fluctlightID, "", "initial", f.app.now())
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_reviews SET status='succeeded',reason_category='no_opportunity',decision='wait',explanation='fixture settled review' WHERE goal_id=$1`, goalID); err != nil {
		t.Fatal(err)
	}
	var initialRevision int
	var initialWatermark int64
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT revision,source_watermark FROM public.goal_reviews WHERE goal_id=$1`, goalID).Scan(&initialRevision, &initialWatermark); err != nil {
		t.Fatal(err)
	}
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		var revision int
		if err := tx.QueryRow(f.ctx, `SELECT revision FROM public.fluctlight_goals WHERE id=$1`, goalID).Scan(&revision); err != nil {
			return err
		}
		current, err := loadGoalAuthorityTx(f.ctx, tx, f.fluctlightID, "goal:ctx_"+stableDigest(goalID), ContextReference{EntityID: goalID, Revision: revision})
		if err != nil {
			return err
		}
		next, record, err := ApplyGoalCommand(&current, GoalCommand{Operation: GoalUpdate, ExpectedRevision: current.Revision, Patch: GoalPatch{SuccessCriteria: []string{"receive two actual relevant responses"}}, EvidenceRefs: []string{"owner:review-watermark-change"}, Reason: "revise standard", OccurredAt: f.app.now()})
		if err != nil {
			return err
		}
		_, err = persistGoalAuthorityTx(f.ctx, tx, &current, next, record, "goal-review-watermark-"+f.suffix)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	var requestID string
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		var err error
		requestID, err = queueGoalReviewsTx(f.ctx, tx, f.fluctlightID, "", "after-goal-revision", f.app.now())
		return err
	}); err != nil {
		t.Fatal(err)
	}
	var state string
	var revision int
	var watermark int64
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status,revision,source_watermark FROM public.goal_reviews WHERE goal_id=$1`, goalID).Scan(&state, &revision, &watermark); err != nil {
		t.Fatal(err)
	}
	if requestID != "" || state != "succeeded" || revision != initialRevision || watermark != initialWatermark {
		t.Fatalf("goal revision reopened review: request=%q state=%s revision=%d watermark=%d", requestID, state, revision, watermark)
	}
}
