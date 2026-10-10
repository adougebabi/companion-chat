package core

import (
	"strings"
	"testing"
	"time"
)

func goalEvaluationSatisfiedCandidate(snapshot goalEvaluationSnapshot) GoalEvaluationCandidate {
	entry := snapshot.Goals[0]
	proof := snapshot.Sources[0].Ref
	return GoalEvaluationCandidate{
		GoalID: entry.GoalID, ExpectedRevision: entry.Goal.Revision, CriteriaVersion: entry.Goal.CriteriaVersion,
		Impact:    "completed",
		Judgments: []GoalCriterionJudgment{{CriterionID: entry.Goal.CriterionIDs[0], Verdict: "satisfied", Kind: "communication", Subject: "actor_self", Discourse: "assertion", EvidenceRefs: []string{proof}, Reason: "直接作者的真实消息满足原标准"}},
	}
}

func TestGoalObjectCompletionMustMatchSatisfiedCriteria(t *testing.T) {
	snapshot := richGoalEvaluationWireSnapshot()
	snapshot.Sources[0].Data["participants"] = []any{snapshot.Goals[0].Goal.TargetActorID}
	binding, err := newGoalEvaluationWireBinding(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"stage", "commitment"} {
		t.Run(kind, func(t *testing.T) {
			candidate := goalEvaluationSatisfiedCandidate(snapshot)
			if kind == "stage" {
				stage := snapshot.Goals[0].Stages[0]
				candidate.StageEvaluation = &GoalObjectEvaluation{ID: stage.ID, ExpectedRevision: stage.Revision, CriteriaVersion: stage.CriteriaVersion, Completed: false, Judgments: []GoalCriterionJudgment{{CriterionID: stage.Criteria[0].ID, Verdict: "satisfied", Kind: "communication", Subject: "actor_self", Discourse: "assertion", EvidenceRefs: []string{snapshot.Sources[0].Ref}, Reason: "阶段标准已经满足"}}}
			} else {
				commitment := snapshot.Goals[0].Commitments[0]
				candidate.CommitmentEvaluations = []GoalObjectEvaluation{{ID: commitment.ID, ExpectedRevision: commitment.Revision, CriteriaVersion: commitment.CriteriaVersion, Completed: false, Judgments: []GoalCriterionJudgment{{CriterionID: commitment.Criteria[0].ID, Verdict: "satisfied", Kind: "communication", Subject: "actor_self", Discourse: "assertion", EvidenceRefs: []string{snapshot.Sources[0].Ref}, Reason: "承诺标准已经满足"}}}}
			}
			raw := goalEvaluationProviderFixture(snapshot, GoalEvaluationTaskOutput{Evaluations: []GoalEvaluationCandidate{candidate}, Plans: []GoalPlanCandidate{}})
			if _, err := goalEvaluationCandidateOutput(snapshot, binding, raw); err == nil || err.Error() != "goal_object_evaluation_completion_mismatch" {
				t.Fatalf("all-satisfied %s with completed=false accepted: %v", kind, err)
			}
			if kind == "stage" {
				candidate.StageEvaluation.Completed = true
			} else {
				candidate.CommitmentEvaluations[0].Completed = true
			}
			raw = goalEvaluationProviderFixture(snapshot, GoalEvaluationTaskOutput{Evaluations: []GoalEvaluationCandidate{candidate}, Plans: []GoalPlanCandidate{}})
			if _, err := goalEvaluationCandidateOutput(snapshot, binding, raw); err != nil {
				t.Fatalf("reciprocal %s completion rejected: %v", kind, err)
			}
		})
	}
}

func TestGoalCompletionMismatchGetsOneCompleteReplacement(t *testing.T) {
	snapshot := richGoalEvaluationWireSnapshot()
	snapshot.Sources[0].Data["participants"] = []any{snapshot.Goals[0].Goal.TargetActorID}
	binding, err := newGoalEvaluationWireBinding(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	invalid := goalEvaluationSatisfiedCandidate(snapshot)
	invalid.Impact = "progressed"
	valid := invalid
	valid.Impact = "completed"
	calls := 0
	_, err = runGoalEvaluationWithCorrection(snapshot, binding, []map[string]any{{"role": "user", "content": "evaluate"}}, func(messages []map[string]any) (ADKStructuredTaskResult, error) {
		calls++
		if calls == 2 && !strings.Contains(jsonString(messages), "goal_evaluation_completion_impact_mismatch") {
			t.Fatal("completion mismatch was not explained to the replacement call")
		}
		candidate := invalid
		if calls == 2 {
			candidate = valid
		}
		return ADKStructuredTaskResult{Completion: ProviderCompletion{Structured: goalEvaluationProviderFixture(snapshot, GoalEvaluationTaskOutput{Evaluations: []GoalEvaluationCandidate{candidate}, Plans: []GoalPlanCandidate{}})}}, nil
	})
	if err != nil || calls != 2 {
		t.Fatalf("complete replacement was not accepted: calls=%d err=%v", calls, err)
	}
}

func TestGoalObjectInvalidReplacementIsTerminal(t *testing.T) {
	snapshot := richGoalEvaluationWireSnapshot()
	snapshot.Sources[0].Data["participants"] = []any{snapshot.Goals[0].Goal.TargetActorID}
	binding, err := newGoalEvaluationWireBinding(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	candidate := goalEvaluationSatisfiedCandidate(snapshot)
	stage := snapshot.Goals[0].Stages[0]
	candidate.StageEvaluation = &GoalObjectEvaluation{ID: stage.ID, ExpectedRevision: stage.Revision, CriteriaVersion: stage.CriteriaVersion, Completed: false, Judgments: []GoalCriterionJudgment{{CriterionID: stage.Criteria[0].ID, Verdict: "satisfied", Kind: "communication", Subject: "actor_self", Discourse: "assertion", EvidenceRefs: []string{snapshot.Sources[0].Ref}, Reason: "阶段标准已经满足"}}}
	raw := goalEvaluationProviderFixture(snapshot, GoalEvaluationTaskOutput{Evaluations: []GoalEvaluationCandidate{candidate}, Plans: []GoalPlanCandidate{}})
	calls := 0
	_, err = runGoalEvaluationWithCorrection(snapshot, binding, nil, func([]map[string]any) (ADKStructuredTaskResult, error) {
		calls++
		return ADKStructuredTaskResult{Completion: ProviderCompletion{Structured: raw}}, nil
	})
	if calls != 2 || err == nil || err.Error() != "goal_object_evaluation_completion_mismatch" || !terminalGoalEvaluationContractError(err) {
		t.Fatalf("invalid replacement was not terminal: calls=%d err=%v", calls, err)
	}
}

func TestGoalEvaluationRejectsInventedUnmetClauseAndCorrectsCompleteRecommendation(t *testing.T) {
	snapshot := richGoalEvaluationWireSnapshot()
	entry := &snapshot.Goals[0]
	entry.Goal.SchemaVersion = goalAuthoritySchemaVersion
	entry.Goal.Ref = "goal:ctx_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	entry.Goal.Scope = "general"
	entry.Goal.TargetActorID = ""
	entry.TargetActorID = ""
	entry.Goal.CriterionIDs = []string{"criterion_recommendation"}
	entry.Goal.SuccessCriteria = []string{"根据对方实际提供的阅读偏好，推荐一本具体小说并给出理由"}
	entry.Goal.CriteriaPolicy = map[string]any{"mode": "all"}
	entry.Goal.CurrentStageID = ""
	entry.Stages = nil
	entry.Commitments = nil
	snapshot.Sources[0].Data["text"] = "推荐《银河系漫游指南》，因为它节奏轻快且富有想象力。"
	preference := snapshot.Sources[0]
	preference.ID += "-preference"
	preference.Ref = "actual-preference"
	preference.SubjectActorID = "reader"
	preference.Data = map[string]any{"message_kind": "user", "text": "我平时也会看看小说"}
	snapshot.Sources = append(snapshot.Sources, preference)
	binding, err := newGoalEvaluationWireBinding(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	unmet := GoalEvaluationCandidate{
		GoalID: entry.GoalID, ExpectedRevision: entry.Goal.Revision, CriteriaVersion: entry.Goal.CriteriaVersion,
		Impact: "needs_evidence", WaitCondition: "等待阅读反馈",
		Judgments: []GoalCriterionJudgment{{CriterionID: "criterion_recommendation", Verdict: "not_satisfied", Kind: "communication", Subject: "actor_self", Discourse: "assertion", EvidenceRefs: []string{snapshot.Sources[0].Ref}, Reason: "增加了原标准没有要求的阅读反馈"}},
	}
	invalid := goalEvaluationProviderFixture(snapshot, GoalEvaluationTaskOutput{Evaluations: []GoalEvaluationCandidate{unmet}, Plans: []GoalPlanCandidate{}})
	judgment := mapValue(arrayValue(mapValue(arrayValue(invalid["evaluations"])[0])["judgments"])[0])
	judgment["criterion_quote"] = "阅读后给出反馈"
	judgment["optional_improvement"] = "可以以后聊阅读感受"
	complete := goalEvaluationSatisfiedCandidate(snapshot)
	complete.Judgments[0].CriterionID = "criterion_recommendation"
	complete.Judgments[0].EvidenceRefs = append(complete.Judgments[0].EvidenceRefs, preference.Ref)
	valid := goalEvaluationProviderFixture(snapshot, GoalEvaluationTaskOutput{Evaluations: []GoalEvaluationCandidate{complete}, Plans: []GoalPlanCandidate{}})
	calls := 0
	run, err := runGoalEvaluationWithCorrection(snapshot, binding, []map[string]any{{"role": "user", "content": "evaluate"}}, func(messages []map[string]any) (ADKStructuredTaskResult, error) {
		calls++
		if calls == 2 && !strings.Contains(jsonString(messages), "goal_evaluation_wire_criterion_quote_invalid") {
			t.Fatal("correction omitted the precise quote failure")
		}
		raw := invalid
		if calls == 2 {
			raw = valid
		}
		return ADKStructuredTaskResult{Completion: ProviderCompletion{Structured: raw}}, nil
	})
	if err != nil || calls != 2 || len(arrayValue(run.Completion.Structured["evaluations"])) != 1 {
		t.Fatalf("valid complete replacement not accepted: calls=%d err=%v", calls, err)
	}
	var settled GoalEvaluationTaskOutput
	if err := decodeStructuredValue(run.Completion.Structured, &settled); err != nil {
		t.Fatal(err)
	}
	sources := map[string]GoalSource{}
	for _, source := range snapshot.Sources {
		sources[source.Ref] = source
	}
	next, _, _, err := ApplyGoalEvaluation(entry.Goal, settled.Evaluations[0], sources, time.Now())
	if err != nil || next.Status != GoalCompleted || next.Progress != 1 {
		t.Fatalf("replacement did not settle actual Goal: status=%s progress=%v err=%v", next.Status, next.Progress, err)
	}
	paused := entry.Goal
	paused.Status = GoalPaused
	next, _, _, err = ApplyGoalEvaluation(paused, settled.Evaluations[0], sources, time.Now())
	if err != nil || next.Status != GoalPaused || next.Progress != paused.Progress {
		t.Fatalf("completed assessment changed paused governance: status=%s progress=%v err=%v", next.Status, next.Progress, err)
	}
}

func TestGoalEvaluationDirectUtteranceDiscourseGetsCorrection(t *testing.T) {
	snapshot := richGoalEvaluationWireSnapshot()
	snapshot.Sources[0].Data["participants"] = []any{snapshot.Goals[0].Goal.TargetActorID}
	binding, _ := newGoalEvaluationWireBinding(snapshot)
	valid := goalEvaluationSatisfiedCandidate(snapshot)
	invalid := goalEvaluationSatisfiedCandidate(snapshot)
	invalid.Judgments[0].Discourse = "quotation"
	calls := 0
	_, err := runGoalEvaluationWithCorrection(snapshot, binding, nil, func(messages []map[string]any) (ADKStructuredTaskResult, error) {
		calls++
		candidate := invalid
		if calls == 2 {
			if !strings.Contains(jsonString(messages), "goal_judgment_not_actual_event") {
				t.Fatal("missing precise evidence correction")
			}
			candidate = valid
		}
		return ADKStructuredTaskResult{Completion: ProviderCompletion{Structured: goalEvaluationProviderFixture(snapshot, GoalEvaluationTaskOutput{Evaluations: []GoalEvaluationCandidate{candidate}, Plans: []GoalPlanCandidate{}})}}, nil
	})
	if err != nil || calls != 2 {
		t.Fatal("direct statement correction failed", calls, err)
	}
}

func TestGoalEvaluationTrueUnmetCriterionStaysUncompleted(t *testing.T) {
	snapshot := richGoalEvaluationWireSnapshot()
	entry := snapshot.Goals[0]
	binding, err := newGoalEvaluationWireBinding(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	candidate := GoalEvaluationCandidate{
		GoalID: entry.GoalID, ExpectedRevision: entry.Goal.Revision, CriteriaVersion: entry.Goal.CriteriaVersion,
		Impact: "needs_evidence", WaitCondition: "等待明确表达",
		Judgments: []GoalCriterionJudgment{{CriterionID: entry.Goal.CriterionIDs[0], Verdict: "not_satisfied", Kind: "communication", Subject: "actor_self", Discourse: "uncertain", EvidenceRefs: []string{}, Reason: "尚未发送明确表达"}},
	}
	raw := goalEvaluationProviderFixture(snapshot, GoalEvaluationTaskOutput{Evaluations: []GoalEvaluationCandidate{candidate}, Plans: []GoalPlanCandidate{}})
	output, err := goalEvaluationCandidateOutput(snapshot, binding, raw)
	if err != nil {
		t.Fatalf("truthful unmet original requirement rejected: %v", err)
	}
	if output.Evaluations[0].Impact == "completed" || output.Evaluations[0].Judgments[0].criterionQuote != entry.Goal.SuccessCriteria[0] {
		t.Fatalf("unmet criterion changed completion semantics: %#v", output.Evaluations[0])
	}
}

func TestGoalEvaluationJudgmentSchemaRequiresGapQuoteFields(t *testing.T) {
	schema := goalCriterionJudgmentSchema([]string{"criterion:1.1"}, []string{"e1"})
	properties := mapValue(schema["properties"])
	for _, field := range []string{"criterion_quote", "optional_improvement"} {
		if _, ok := properties[field]; !ok || !containsString(decisionServiceRefValues(schema["required"]), field) {
			t.Fatalf("wire schema does not require %s", field)
		}
	}
}
