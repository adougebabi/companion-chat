package core

import (
	"errors"
	"strings"
	"testing"
)

func twoGoalWireSnapshot() goalEvaluationSnapshot {
	s := richGoalEvaluationWireSnapshot()
	second := s.Goals[0]
	second.GoalID = "goal-two"
	second.Goal.EntityID = "goal-two"
	second.Goal.Scope = "general"
	second.Goal.TargetActorID = ""
	second.Goal.CriterionIDs = []string{"criterion-two"}
	second.Goal.SuccessCriteria = []string{"second condition"}
	second.Goal.CurrentStageID = ""
	second.Stages = nil
	second.Commitments = nil
	s.Goals = append(s.Goals, second)
	return s
}

func TestGoalEvaluationCrossGoalReviewGetsOneReplacement(t *testing.T) {
	input := twoGoalWireSnapshot()
	binding, _ := newGoalEvaluationWireBinding(input)
	evaluations := []GoalEvaluationCandidate{}
	for _, entry := range input.Goals {
		evaluations = append(evaluations, GoalEvaluationCandidate{GoalID: entry.GoalID, ExpectedRevision: entry.Goal.Revision, CriteriaVersion: entry.Goal.CriteriaVersion, Impact: "needs_evidence", WaitCondition: "wait", Judgments: []GoalCriterionJudgment{}})
	}
	valid := goalEvaluationProviderFixture(input, GoalEvaluationTaskOutput{Evaluations: evaluations, Plans: []GoalPlanCandidate{}})
	invalid := decodeObject(jsonBytes(valid))
	mapValue(arrayValue(invalid["evaluations"])[1])["review"] = map[string]any{"reason_category": "no_opportunity", "decision": "wait", "explanation": "wait", "evidence_refs": []string{}, "stage_ref": "stage:1.1", "feasible_alternative": ""}
	if _, err := binding.hydrateOutput(invalid); err == nil || err.Error() != "goal_evaluation_wire_review_stage_ref_invalid" {
		t.Fatal("sample no longer exercises cross-goal ownership", err)
	}
	for _, repeatInvalid := range []bool{false, true} {
		calls := 0
		run, err := runGoalEvaluationWithCorrection(input, binding, []map[string]any{{"role": "user", "content": "task"}}, func(messages []map[string]any) (ADKStructuredTaskResult, error) {
			calls++
			raw := invalid
			if calls == 2 {
				if !strings.Contains(jsonString(messages), "goal_evaluation_wire_review_stage_ref_invalid") {
					t.Fatal("missing scoped correction feedback")
				}
				if !repeatInvalid {
					raw = valid
				}
			}
			return ADKStructuredTaskResult{Completion: ProviderCompletion{Structured: raw}}, nil
		})
		if calls != 2 {
			t.Fatal("unbounded/absent correction", calls)
		}
		if repeatInvalid {
			if err == nil || !terminalGoalEvaluationContractError(err) {
				t.Fatal("invalid replacement still retries entire assessment", err)
			}
		} else if err != nil || len(arrayValue(run.Completion.Structured["evaluations"])) != 2 {
			t.Fatal("valid replacement not accepted", err)
		}
	}
}
func TestGoalEvaluationOutputFailuresAreTerminalAfterCorrection(t *testing.T) {
	for _, code := range []string{"goal_evaluation_wire_review_stage_ref_invalid", "goal_evaluation_wire_stage_ref_invalid", "goal_evaluation_wire_dependency_ref_invalid", "goal_evaluation_wire_commitment_ref_invalid", "goal_evaluation_wire_stage_operation_invalid"} {
		if !terminalGoalEvaluationContractError(errors.New(code)) {
			t.Fatal(code)
		}
	}
}

func TestGoalEvaluationModelSuccessDoesNotHideSettlementFailure(t *testing.T) {
	runs := []map[string]any{{"id": "model", "scenario": "goal_evaluation", "logical_run_id": "request", "status": "completed", "response": map[string]any{"impact": "completed"}}}
	applyGoalEvaluationSettlement(runs, map[string]map[string]any{"request": {"status": "failed", "error_code": "goal_evaluation_wire_review_stage_ref_invalid"}})
	if runs[0]["status"] != "completed" {
		t.Fatal("physical model terminal state was overwritten")
	}
	outcome := mapValue(runs[0]["goal_evaluation"])
	if outcome["status"] != "failed" || outcome["error_code"] != "goal_evaluation_wire_review_stage_ref_invalid" {
		t.Fatal("semantic failure missing from model details", outcome)
	}
}
func goalSchemaBranch(node map[string]any, goalRef string) map[string]any {
	variants := arrayValue(node["anyOf"])
	if len(variants) == 0 {
		return node
	}
	for _, raw := range variants {
		branch := mapValue(raw)
		refs := arrayValue(mapValue(mapValue(branch["properties"])["goal_ref"])["enum"])
		if len(refs) == 1 && refs[0] == goalRef {
			return branch
		}
	}
	return nil
}
func TestGoalEvaluationSchemaCannotAdvertiseOtherGoalsStage(t *testing.T) {
	binding, err := newGoalEvaluationWireBinding(twoGoalWireSnapshot())
	if err != nil {
		t.Fatal(err)
	}
	schema := goalEvaluationResponseSchema(binding)
	node := mapValue(mapValue(mapValue(schema["properties"])["evaluations"])["items"])
	second := goalSchemaBranch(node, "goal:2")
	properties := mapValue(second["properties"])
	if _, ok := mapValue(mapValue(properties["review"])["properties"])["stage_ref"]; ok {
		t.Fatal("goal2 review may select stage:1.1 owned by goal1")
	}
	if _, ok := properties["stage_evaluation"]; ok {
		t.Fatal("stage-less goal2 advertises another Goal's stage")
	}
	criteria := arrayValue(mapValue(mapValue(mapValue(mapValue(properties["judgments"])["items"])["properties"])["criterion_ref"])["enum"])
	if len(criteria) != 1 || criteria[0] != "criterion:2.1" {
		t.Fatal("cross-Goal criterion selection", criteria)
	}
}
