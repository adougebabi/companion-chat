package core

import (
	"testing"
	"time"
)

func assessmentMemoFixture() (goalEvaluationGoal, []GoalSource, ContextProjection) {
	deadline := time.Date(2026, 10, 20, 8, 0, 0, 0, time.UTC)
	entry := goalEvaluationGoal{
		GoalID: "goal-1",
		Goal: GoalAuthority{
			EntityID: "goal-1", SchemaVersion: goalAuthoritySchemaVersion, Ref: "goal:ctx_one",
			FluctlightID: "self", ProfileID: "default", DesiredOutcome: "learn Go",
			SuccessCriteria: []string{"ship one real change"}, CriterionIDs: []string{"criterion-1"},
			CriteriaVersion: 2, CriteriaPolicy: map[string]any{"mode": "all"}, Motivation: "practice",
			Scope: "general", Deadline: &deadline, DeadlinePolicy: "soft", Status: GoalActive, Revision: 7,
			ExecutionHint: map[string]any{"evaluation_id": "evaluation-one", "next_step": "write code"},
		},
		Stages:           []GoalStage{{ID: "stage-one", GoalID: "goal-1", Purpose: "make change", Strategy: "small patch", Criteria: []GoalCriterion{{ID: "stage-criterion-one", Text: "tests pass"}}, CriteriaVersion: 1, Status: "active", Revision: 3}},
		CurrentJudgments: []GoalCriterionJudgment{{CriterionID: "criterion-1", Verdict: "unknown", Reason: "old judgment"}},
	}
	sources := []GoalSource{{Ref: "source:11", EventID: 11, Kind: "message", ID: "message-one", Version: "v1", FluctlightID: "self", ProfileID: "default", ConversationID: "conversation-one", SubjectActorID: "self", Valid: true, CanSupportSuccess: true}}
	projection := ContextProjection{
		AsOf: "2026-10-08T10:00:00.000+00:00", ReferenceTimezone: "Asia/Shanghai",
		CurrentStateRevision: 5, CurrentState: map[string]any{"mood": "calm"}, InnerState: map[string]any{"valence": 0.1},
		CorePersona:      map[string]any{"revision": 4, "values": []any{"honesty"}},
		BehavioralPolicy: map[string]any{"autonomy": "enabled", "revision": 2},
		LifeContext:      map[string]any{"scene": "home", "revision": 8, "current_time": "18:00"},
		Capabilities:     []map[string]any{{"id": "capability-id", "name": "conversation.reply", "enabled": true}},
	}
	return entry, sources, projection
}

func TestGoalAssessmentMemoIgnoresEvaluationBookkeepingAndClockChurn(t *testing.T) {
	entry, sources, projection := assessmentMemoFixture()
	at := time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)
	base := goalAssessmentMemoFor(entry, sources, projection, at)

	entry.Goal.Revision++
	entry.Goal.Ref = "goal:ctx_changed"
	entry.Goal.ExecutionHint = map[string]any{"evaluation_id": "evaluation-two", "next_step": "different generated plan"}
	entry.CurrentJudgments = []GoalCriterionJudgment{{CriterionID: "criterion-1", Verdict: "satisfied", Reason: "new judgment"}}
	entry.Stages[0].ID = "stage-storage-id-changed"
	entry.Stages[0].Revision++
	entry.Stages[0].Criteria[0].ID = "criterion-storage-id-changed"
	projection.AsOf = "2026-10-08T10:00:01.000+00:00"
	projection.CurrentStateRevision++
	projection.CurrentState["mood"] = "neutral"
	projection.InnerState["valence"] = -0.2
	projection.CorePersona["revision"] = 99
	projection.LifeContext["revision"] = 99
	projection.LifeContext["instant"] = "2026-10-08T10:00:01.000+00:00"
	projection.LifeContext["current_time"] = "18:01"
	projection.Capabilities[0]["id"] = "another-storage-id"

	current := goalAssessmentMemoFor(entry, sources, projection, at.Add(time.Hour))
	if !goalAssessmentMemoMatches(base, current) {
		t.Fatalf("bookkeeping or clock churn dirtied memo: base=%#v current=%#v", base, current)
	}
}

func TestGoalAssessmentMemoChangesWhenTemporalBoundaryIsCrossed(t *testing.T) {
	entry, sources, projection := assessmentMemoFixture()
	before := time.Date(2026, 10, 20, 7, 59, 59, 0, time.UTC)
	base := goalAssessmentMemoFor(entry, sources, projection, before)
	if goalAssessmentMemoMatches(base, goalAssessmentMemoFor(entry, sources, projection, before.Add(time.Second))) {
		t.Fatal("crossing the Goal deadline retained the prior assessment memo")
	}

	entry.Goal.Deadline = nil
	windowEnd := before.Add(time.Second)
	entry.Commitments = []GoalCommitment{{ExpectedResult: "one review", WindowEnd: &windowEnd, Status: "active"}}
	base = goalAssessmentMemoFor(entry, sources, projection, before)
	if goalAssessmentMemoMatches(base, goalAssessmentMemoFor(entry, sources, projection, windowEnd)) {
		t.Fatal("crossing the commitment window retained the prior assessment memo")
	}
}

func TestGoalAssessmentMemoChangesForSemanticAuthorityAndConstraints(t *testing.T) {
	entry, sources, projection := assessmentMemoFixture()
	base := goalAssessmentMemoFor(entry, sources, projection, time.Now())

	tests := []struct {
		name   string
		change func(*goalEvaluationGoal, *ContextProjection)
	}{
		{"criterion", func(entry *goalEvaluationGoal, _ *ContextProjection) {
			entry.Goal.SuccessCriteria[0] = "ship two real changes"
		}},
		{"stage standard", func(entry *goalEvaluationGoal, _ *ContextProjection) {
			entry.Stages[0].Criteria[0].Text = "integration tests pass"
		}},
		{"commitment window", func(entry *goalEvaluationGoal, _ *ContextProjection) {
			end := time.Date(2026, 10, 9, 8, 0, 0, 0, time.UTC)
			entry.Commitments = []GoalCommitment{{ExpectedResult: "one review", CriteriaVersion: 1, WindowEnd: &end, Status: "active"}}
		}},
		{"permission", func(_ *goalEvaluationGoal, projection *ContextProjection) {
			projection.BehavioralPolicy["autonomy"] = "paused"
		}},
		{"life constraint", func(_ *goalEvaluationGoal, projection *ContextProjection) {
			projection.LifeContext["scene"] = "sleeping"
		}},
		{"effective life phase", func(_ *goalEvaluationGoal, projection *ContextProjection) {
			projection.LifeContext["effective_at"] = "2026-10-08T11:00:00.000+00:00"
			projection.LifeContext["expires_at"] = "2026-10-08T12:00:00.000+00:00"
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			changedEntry, changedSources, changedProjection := assessmentMemoFixture()
			test.change(&changedEntry, &changedProjection)
			if goalAssessmentMemoMatches(base, goalAssessmentMemoFor(changedEntry, changedSources, changedProjection, time.Now())) {
				t.Fatal("semantic change retained assessment memo")
			}
		})
	}
}

func TestGoalAssessmentMemoAllowsSourceSubsetButRejectsNewOrWithdrawnProof(t *testing.T) {
	entry, sources, projection := assessmentMemoFixture()
	at := time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)
	extra := sources[0]
	extra.Ref, extra.EventID, extra.ID = "source:12", 12, "message-two"
	prior := goalAssessmentMemoFor(entry, append(sources, extra), projection, at)
	if !goalAssessmentMemoMatches(prior, goalAssessmentMemoFor(entry, sources, projection, at)) {
		t.Fatal("same-proof source subset should remain covered")
	}

	newSource := sources[0]
	newSource.Ref, newSource.EventID, newSource.ID = "source:13", 13, "message-three"
	if goalAssessmentMemoMatches(prior, goalAssessmentMemoFor(entry, append(sources, newSource), projection, at)) {
		t.Fatal("new proof was swallowed by prior memo")
	}
	withdrawn := sources[0]
	withdrawn.Valid = false
	withdrawn.CanSupportSuccess = false
	if goalAssessmentMemoMatches(prior, goalAssessmentMemoFor(entry, []GoalSource{withdrawn}, projection, at)) {
		t.Fatal("source withdrawal was swallowed by prior memo")
	}
}

func TestGoalAssessmentEligibilityHonorsOwnerForceAndDueReview(t *testing.T) {
	entry, sources, projection := assessmentMemoFixture()
	at := time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)
	prior := map[string]goalAssessmentMemo{"goal-1": goalAssessmentMemoFor(entry, sources, projection, at)}
	snapshot := goalEvaluationSnapshot{Goals: []goalEvaluationGoal{entry}, Sources: sources, ProviderSourceIDs: []int64{11}}

	eligible, skipped := goalAssessmentEligible(snapshot, projection, prior, at)
	if len(eligible.Goals) != 0 || len(skipped) != 1 {
		t.Fatalf("unchanged assessment eligibility = %#v skipped=%#v", eligible.Goals, skipped)
	}
	snapshot.ForcedGoalIDs = []string{"goal-1"}
	eligible, skipped = goalAssessmentEligible(snapshot, projection, prior, at)
	if len(eligible.Goals) != 1 || len(skipped) != 0 {
		t.Fatalf("owner force was ignored: eligible=%#v skipped=%#v", eligible.Goals, skipped)
	}
	snapshot.ForcedGoalIDs = nil
	snapshot.Reviews = []GoalReviewContext{{GoalID: "goal-1", ID: "review-one"}}
	eligible, skipped = goalAssessmentEligible(snapshot, projection, prior, at)
	if len(eligible.Goals) != 1 || len(skipped) != 0 || len(eligible.Reviews) != 1 {
		t.Fatalf("due review was ignored: eligible=%#v skipped=%#v reviews=%#v", eligible.Goals, skipped, eligible.Reviews)
	}
}

func TestGoalAssessmentNoProgressDetectsMandatoryOnlyAdmission(t *testing.T) {
	snapshot := goalEvaluationSnapshot{UnprocessedSourceIDs: []int64{20}, ProviderSourceIDs: []int64{11}}
	if !goalAssessmentHasUnofferedNewSource(snapshot) {
		t.Fatal("mandatory old proof hiding all new proof must fail visibly")
	}
	snapshot.ProviderSourceIDs = append(snapshot.ProviderSourceIDs, 20)
	if goalAssessmentHasUnofferedNewSource(snapshot) {
		t.Fatal("offered new proof must be allowed to make finite progress")
	}
}

func TestGoalAssessmentMemoRejectsPriorEvaluationPolicy(t *testing.T) {
	entry, sources, projection := assessmentMemoFixture()
	current := goalAssessmentMemoFor(entry, sources, projection, time.Now())
	for _, priorPolicy := range []string{"", "goal.evaluation.v1"} {
		prior := current
		prior.EvaluationPolicyVersion = priorPolicy
		if goalAssessmentMemoMatches(prior, current) {
			t.Fatalf("obsolete policy reused: %q", priorPolicy)
		}
	}
	if !goalAssessmentMemoMatches(current, current) {
		t.Fatal("current policy memo did not match")
	}
}
