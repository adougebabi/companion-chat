package core

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	capabilitycontract "github.com/fluctlight/local-ai-companion/apps/core-go/internal/capability"
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
	second.Goal.CriteriaPolicy = map[string]any{"mode": "all"}
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

func TestGoalEvaluationSchemaRejectsMissingGoalBeforeHydration(t *testing.T) {
	input := twoGoalWireSnapshot()
	binding, err := newGoalEvaluationWireBinding(input)
	if err != nil {
		t.Fatal(err)
	}
	evaluations := []GoalEvaluationCandidate{}
	for _, entry := range input.Goals {
		evaluations = append(evaluations, GoalEvaluationCandidate{GoalID: entry.GoalID, ExpectedRevision: entry.Goal.Revision, CriteriaVersion: effectiveGoalCriteriaVersion(entry.Goal), Impact: "needs_evidence", Judgments: []GoalCriterionJudgment{}})
	}
	valid := goalEvaluationProviderFixture(input, GoalEvaluationTaskOutput{Evaluations: evaluations, Plans: []GoalPlanCandidate{}})
	schema := goalEvaluationResponseSchema(binding)
	if err := capabilitycontract.ValidateCapabilitySchemaValue(valid, schema); err != nil {
		t.Fatal("complete batch rejected", err)
	}
	incomplete := decodeObject(jsonBytes(valid))
	incomplete["evaluations"] = arrayValue(incomplete["evaluations"])[:1]
	if err := capabilitycontract.ValidateCapabilitySchemaValue(incomplete, schema); err == nil {
		t.Fatal("one-goal response accepted for a two-goal input")
	}
	excess := decodeObject(jsonBytes(valid))
	excess["evaluations"] = append(arrayValue(excess["evaluations"]), arrayValue(valid["evaluations"])[0])
	if err := capabilitycontract.ValidateCapabilitySchemaValue(excess, schema); err == nil {
		t.Fatal("excess evaluations accepted")
	}
}

func TestGoalEvaluationPhysicalContractDisablesThinkingAndRepairsMissingGoal(t *testing.T) {
	input := twoGoalWireSnapshot()
	binding, _ := newGoalEvaluationWireBinding(input)
	schema := goalEvaluationResponseSchema(binding)
	items := []any{}
	for _, ref := range sortedGoalBindingRefs(binding.goalsByRef) {
		items = append(items, map[string]any{"goal_ref": ref, "judgments": []any{}, "impact": "needs_evidence", "blocker": "", "wait_condition": "wait", "next_step": "wait", "residual_motivation": ""})
	}
	for _, adk := range []bool{false, true} {
		t.Run(map[bool]string{false: "direct", true: "adk"}[adk], func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				var payload map[string]any
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					t.Error(err)
				}
				if value, present := payload["enable_thinking"]; !present || value != false {
					t.Error("Goal evaluation must explicitly disable server-default thinking", value)
				}
				physicalSchema := mapValue(mapValue(mapValue(payload["response_format"])["json_schema"])["schema"])
				if intValue(mapValue(mapValue(physicalSchema["properties"])["evaluations"])["minItems"]) != 2 {
					t.Error("physical schema lost complete-Goal cardinality")
				}
				output := map[string]any{"evaluations": items, "plans": []any{}}
				if adk && calls == 1 {
					output["evaluations"] = items[:1]
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": jsonString(output)}}}})
			}))
			defer server.Close()
			ctx := context.Background()
			if adk {
				trace := &ADKCapabilityTrace{}
				ctx = WithADKCapabilityInvoker(ctx, adkTraceInvoker{trace: trace}, trace)
			}
			p := &ProviderClient{HTTP: server.Client()}
			response, err := p.generateWithEino(ctx, EinoModelCall{Assignment: providerAssignment{Role: "cognitive_assessment", BaseURL: server.URL, ModelID: "fake", Timeout: 10 * time.Second, TokenBudget: 4096}, Role: "cognitive_assessment", Scenario: "goal_evaluation", Messages: []map[string]any{{"role": "user", "content": "Evaluate every offered goal"}}, JSONMode: true, SchemaName: "goal_evaluation_v1", ResponseSchema: schema, ProviderRequestID: "provider-goal", CorrelationID: "goal-request"})
			if err != nil || response.Message == nil {
				t.Fatal("complete replacement failed", err)
			}
			wantCalls := 1
			if adk {
				wantCalls = 2
			}
			if calls != wantCalls {
				t.Fatal("unexpected repair count", calls)
			}
		})
	}
}
