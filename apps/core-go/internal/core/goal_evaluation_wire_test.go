package core

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestGoalEvaluationWirePersistedClaimHasIdenticalBindingWithoutMutation(t *testing.T) {
	original := richGoalEvaluationWireSnapshot()
	before := jsonString(original)
	var restored goalEvaluationSnapshot
	if err := json.Unmarshal(jsonBytes(original), &restored); err != nil {
		t.Fatal(err)
	}
	if restored.Goals[0].Goal.EntityID != "" {
		t.Fatal("test no longer exercises Core-only identity omission")
	}
	_, stable, current, err := goalEvaluationWireInput(original)
	if err != nil {
		t.Fatal(err)
	}
	_, stableAgain, currentAgain, err := goalEvaluationWireInput(restored)
	if err != nil {
		t.Fatal(err)
	}
	if jsonString(stable) != jsonString(stableAgain) || jsonString(current) != jsonString(currentAgain) {
		t.Fatal("persisted claim changed provider references or semantics")
	}
	if jsonString(original) != before || restored.Goals[0].Goal.EntityID != "" {
		t.Fatal("binding mutated authoritative snapshot")
	}
}

func goalEvaluationProviderFixture(snapshot goalEvaluationSnapshot, output GoalEvaluationTaskOutput) map[string]any {
	binding, err := newGoalEvaluationWireBinding(snapshot)
	if err != nil {
		panic(err)
	}
	wire := goalEvaluationWireOutput{Evaluations: []goalEvaluationWireCandidate{}, Plans: []goalEvaluationWirePlan{}}
	for _, candidate := range output.Evaluations {
		entryRef := binding.goalRefsByID[candidate.GoalID]
		entry, valid := binding.goalsByRef[entryRef]
		if !valid || candidate.ExpectedRevision != entry.Goal.Revision || candidate.CriteriaVersion != effectiveGoalCriteriaVersion(entry.Goal) {
			entryRef = "goal:invalid"
		}
		item := goalEvaluationWireCandidate{GoalRef: entryRef, Judgments: goalEvaluationWireJudgmentFixtures(binding, candidate.GoalID, "goal", candidate.GoalID, candidate.Judgments), Impact: candidate.Impact, Blocker: candidate.Blocker, WaitCondition: candidate.WaitCondition, NextStep: candidate.NextStep, NextReviewAt: candidate.NextReviewAt, ResidualMotivation: candidate.ResidualMotivation, Followup: candidate.Followup}
		if candidate.StageEvaluation != nil {
			item.StageEvaluation = goalEvaluationWireObjectEvaluationFixture(binding, candidate.GoalID, "stage", *candidate.StageEvaluation)
		}
		for _, evaluation := range candidate.CommitmentEvaluations {
			item.CommitmentEvaluations = append(item.CommitmentEvaluations, *goalEvaluationWireObjectEvaluationFixture(binding, candidate.GoalID, "commitment", evaluation))
		}
		if candidate.RelationshipConfirmation != nil {
			item.RelationshipConfirmation = &goalEvaluationWireRelationshipConfirmation{TargetActorRef: fixtureRef(binding.actorRefs[candidate.RelationshipConfirmation.TargetActorID], "actor:invalid"), EvidenceRefs: goalEvaluationWireSourceRefFixtures(binding, candidate.RelationshipConfirmation.EvidenceRefs), Label: candidate.RelationshipConfirmation.Label}
		}
		if candidate.Review != nil {
			item.Review = &goalEvaluationWireReview{ReasonCategory: candidate.Review.ReasonCategory, Decision: candidate.Review.Decision, Explanation: candidate.Review.Explanation, EvidenceRefs: goalEvaluationWireSourceRefFixtures(binding, candidate.Review.EvidenceRefs), StageRef: fixtureRef(binding.stageRefsByID[candidate.Review.StageID], map[bool]string{true: "", false: "stage:invalid"}[candidate.Review.StageID == ""]), FeasibleAlternative: candidate.Review.FeasibleAlternative}
		}
		wire.Evaluations = append(wire.Evaluations, item)
	}
	for _, plan := range output.Plans {
		entryRef := binding.goalRefsByID[plan.GoalID]
		entry, valid := binding.goalsByRef[entryRef]
		if !valid || plan.ExpectedRevision != entry.Goal.Revision || plan.CriteriaVersion != effectiveGoalCriteriaVersion(entry.Goal) {
			entryRef = "goal:invalid"
		}
		item := goalEvaluationWirePlan{GoalRef: entryRef, Reason: plan.Reason, NextStep: plan.NextStep, WaitCondition: plan.WaitCondition, NextReviewAt: plan.NextReviewAt}
		if plan.Stage != nil {
			stageRef := ""
			if plan.Stage.ID != "" {
				stageRef = fixtureRef(binding.stageRefsByID[plan.Stage.ID], "stage:invalid")
				if stage := goalStageRevision(entry.Stages, plan.Stage.ID); stage != plan.Stage.ExpectedRevision {
					stageRef = "stage:invalid"
				}
			}
			item.Stage = &goalEvaluationWireStagePlan{Operation: plan.Stage.Operation, ObjectRef: stageRef, Purpose: plan.Stage.Purpose, Strategy: plan.Stage.Strategy, EntryBasis: plan.Stage.EntryBasis, ExitBasis: plan.Stage.ExitBasis, Criteria: plan.Stage.Criteria, Reason: plan.Stage.Reason}
			for _, dependencyID := range plan.Stage.DependencyIDs {
				ref := binding.stageRefsByID[dependencyID]
				if ref == "" {
					ref = binding.dependencyRefs[plan.GoalID+"\x1f"+dependencyID]
				}
				item.Stage.DependencyRefs = append(item.Stage.DependencyRefs, fixtureRef(ref, "dependency:invalid"))
			}
		}
		if plan.Commitment != nil {
			commitmentRef := ""
			if plan.Commitment.ID != "" {
				commitmentRef = fixtureRef(binding.commitmentRefs[plan.Commitment.ID], "commitment:invalid")
				if revision := goalCommitmentRevision(entry.Commitments, plan.Commitment.ID); revision != plan.Commitment.ExpectedRevision {
					commitmentRef = "commitment:invalid"
				}
			}
			item.Commitment = &goalEvaluationWireCommitmentPlan{Operation: plan.Commitment.Operation, Reason: plan.Commitment.Reason, ObjectRef: commitmentRef, ExpectedResult: plan.Commitment.ExpectedResult, Criteria: plan.Commitment.Criteria, WindowStart: plan.Commitment.WindowStart, WindowEnd: plan.Commitment.WindowEnd, OpportunityCondition: plan.Commitment.OpportunityCondition, Blocker: plan.Commitment.Blocker}
		}
		wire.Plans = append(wire.Plans, item)
	}
	return decodeObject(jsonBytes(wire))
}

func fixtureRef(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func goalEvaluationWireSourceRefFixtures(binding *goalEvaluationWireBinding, values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, fixtureRef(binding.sourceRefs[value], "source:invalid"))
	}
	return result
}

func goalEvaluationWireJudgmentFixtures(binding *goalEvaluationWireBinding, goalID, kind, objectID string, values []GoalCriterionJudgment) []goalEvaluationWireJudgment {
	result := make([]goalEvaluationWireJudgment, 0, len(values))
	for _, value := range values {
		criterionRef := fixtureRef(binding.criterionRefs[kind+"\x1f"+objectID+"\x1f"+value.CriterionID], "criterion:invalid")
		quote := value.criterionQuote
		if value.Verdict == "not_satisfied" && quote == "" {
			quote = binding.criterionText(binding.criteriaByRef[criterionRef])
		}
		result = append(result, goalEvaluationWireJudgment{CriterionRef: criterionRef, CriterionQuote: quote, OptionalImprovement: value.optionalImprovement, Verdict: value.Verdict, Kind: value.Kind, Subject: value.Subject, Discourse: value.Discourse, EvidenceRefs: goalEvaluationWireSourceRefFixtures(binding, value.EvidenceRefs), Reason: value.Reason})
	}
	return result
}

func goalEvaluationWireObjectEvaluationFixture(binding *goalEvaluationWireBinding, goalID, kind string, value GoalObjectEvaluation) *goalEvaluationWireObjectEvaluation {
	ref := binding.stageRefsByID[value.ID]
	entry := binding.goalsByRef[binding.goalRefsByID[goalID]]
	if kind == "commitment" {
		ref = binding.commitmentRefs[value.ID]
		if goalCommitmentRevision(entry.Commitments, value.ID) != value.ExpectedRevision || goalCommitmentCriteriaVersion(entry.Commitments, value.ID) != value.CriteriaVersion {
			ref = "commitment:invalid"
		}
	} else if goalStageRevision(entry.Stages, value.ID) != value.ExpectedRevision || goalStageCriteriaVersion(entry.Stages, value.ID) != value.CriteriaVersion {
		ref = "stage:invalid"
	}
	return &goalEvaluationWireObjectEvaluation{ObjectRef: fixtureRef(ref, kind+":invalid"), Judgments: goalEvaluationWireJudgmentFixtures(binding, goalID, kind, value.ID, value.Judgments), Completed: value.Completed, Reason: value.Reason}
}

func richGoalEvaluationWireSnapshot() goalEvaluationSnapshot {
	deadline := time.Date(2026, 10, 9, 10, 30, 0, 0, time.UTC)
	return goalEvaluationSnapshot{RequestID: "request-secret", FluctlightID: "fluctlight-secret", OwnerActorID: "owner-secret", ProfileID: "profile-secret", Goals: []goalEvaluationGoal{{GoalID: "goal-secret", TargetActorID: "target-secret", Goal: GoalAuthority{EntityID: "goal-secret", FluctlightID: "fluctlight-secret", ProfileID: "profile-secret", TargetActorID: "target-secret", Revision: 7, CriteriaVersion: 3, CriterionIDs: []string{"criterion-required", "criterion-optional"}, SuccessCriteria: []string{"发送明确表达\n保留换行", "可选：得到真实回复，含逗号, comma"}, CriteriaPolicy: map[string]any{"mode": "all", "optional_ids": []any{"criterion-optional"}}, DesiredOutcome: "建立真实而尊重的沟通", Motivation: "重视彼此", Scope: "relationship", Status: GoalActive, Deadline: &deadline, DeadlinePolicy: "hard", CurrentStageID: "stage-secret", ExecutionHint: map[string]any{"enabled": false, "attempts": 0, "goal_id": "goal-secret"}}, Stages: []GoalStage{{ID: "stage-secret", FluctlightID: "fluctlight-secret", GoalID: "goal-secret", ProfileID: "profile-secret", Purpose: "先清楚表达", Strategy: "一次真实消息", EntryBasis: "已有合适语境", ExitBasis: "消息实际发送", Criteria: []GoalCriterion{{ID: "stage-criterion-secret", Text: "消息实际发送"}}, CriteriaVersion: 4, Status: "active", Revision: 9, DependencyIDs: []string{"older-stage-secret"}}}, Commitments: []GoalCommitment{{ID: "commitment-secret", FluctlightID: "fluctlight-secret", GoalID: "goal-secret", StageID: "stage-secret", ProfileID: "profile-secret", ExpectedResult: "一次真实表达", Criteria: []GoalCriterion{{ID: "commitment-criterion-secret", Text: "真实发送"}}, CriteriaVersion: 2, Status: "blocked", Revision: 5, Blocker: "等待合适时间", WindowEnd: &deadline}}}}, Sources: []GoalSource{{EventID: 77, Ref: "source-raw-secret", Kind: "message", ID: "message-secret", Version: "message-version-secret", FluctlightID: "fluctlight-secret", ProfileID: "profile-secret", ConversationID: "conversation-secret", SubjectActorID: "fluctlight-secret", OccurredAt: deadline.Add(-time.Hour), RecordedAt: deadline, Valid: true, CanSupportSuccess: true, GoalIDs: []string{"goal-secret"}, Data: map[string]any{"text": "我喜欢你。\n这是认真表达。", "message_kind": "assistant", "message_id": "message-secret", "delivered": true, "optional": false, "score": 0, "metadata": map[string]any{"source_ref": "source-raw-secret"}}}}}
}

func TestGoalEvaluationWireOmitsRawAuthorityAndHydratesFrozenValues(t *testing.T) {
	snapshot := richGoalEvaluationWireSnapshot()
	durableBefore := jsonString(snapshot)
	binding, stable, current, err := goalEvaluationWireInput(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	wire := jsonString(map[string]any{"stable_definitions": stable, "current": current, "schema": goalEvaluationResponseSchema(binding)})
	for _, forbidden := range []string{"goal-secret", "criterion-required", "criterion-optional", "stage-secret", "commitment-secret", "source-raw-secret", "fluctlight-secret", "owner-secret", "target-secret", "profile-secret", "conversation-secret", "message-secret", "expected_revision", "criteria_version", "goal_id", "criterion_id", "target_actor_id", "dependency_ids"} {
		if strings.Contains(wire, forbidden) {
			t.Fatalf("raw authority escaped Provider wire: %s in %s", forbidden, wire)
		}
	}
	for _, preserved := range []string{"发送明确表达", "可选：得到真实回复", "我喜欢你。\\n这是认真表达。", `"delivered":true`, `"score":0`, "criterion:1.1", "stage:1.1", "commitment:1.1", "e1", "actor_self"} {
		if !strings.Contains(wire, preserved) {
			t.Fatalf("semantic value missing: %s", preserved)
		}
	}
	internal := GoalEvaluationTaskOutput{Evaluations: []GoalEvaluationCandidate{{GoalID: "goal-secret", ExpectedRevision: 7, CriteriaVersion: 3, Judgments: []GoalCriterionJudgment{{CriterionID: "criterion-required", Verdict: "satisfied", Kind: "communication", Subject: "actor_self", Discourse: "assertion", EvidenceRefs: []string{"source-raw-secret"}, Reason: "真实消息"}}, Impact: "progressed", NextStep: "等待回复", StageEvaluation: &GoalObjectEvaluation{ID: "stage-secret", ExpectedRevision: 9, CriteriaVersion: 4, Judgments: []GoalCriterionJudgment{{CriterionID: "stage-criterion-secret", Verdict: "satisfied", Kind: "communication", Subject: "actor_self", Discourse: "assertion", EvidenceRefs: []string{"source-raw-secret"}, Reason: "真实消息"}}, Completed: true, Reason: "阶段完成"}, CommitmentEvaluations: []GoalObjectEvaluation{{ID: "commitment-secret", ExpectedRevision: 5, CriteriaVersion: 2, Judgments: []GoalCriterionJudgment{{CriterionID: "commitment-criterion-secret", Verdict: "satisfied", Kind: "communication", Subject: "actor_self", Discourse: "assertion", EvidenceRefs: []string{"source-raw-secret"}, Reason: "真实消息"}}, Completed: true, Reason: "承诺完成"}}, RelationshipConfirmation: &GoalRelationshipConfirmation{TargetActorID: "target-secret", EvidenceRefs: []string{"source-raw-secret"}, Label: "确认关系"}, Review: &GoalReviewDecision{ReasonCategory: "progressed", Decision: "continue", Explanation: "有真实进展", EvidenceRefs: []string{"source-raw-secret"}, StageID: "stage-secret"}}}, Plans: []GoalPlanCandidate{{GoalID: "goal-secret", ExpectedRevision: 7, CriteriaVersion: 3, Reason: "根据真实结果调整", WaitCondition: "等待下一事实", Stage: &GoalStagePlan{Operation: "adjust", ID: "stage-secret", ExpectedRevision: 9, DependencyIDs: []string{"older-stage-secret"}, Purpose: "保持尊重", Strategy: "等待", Criteria: []string{"真实回应"}}, Commitment: &GoalCommitmentPlan{Operation: "adjust", ID: "commitment-secret", ExpectedRevision: 5, ExpectedResult: "真实回应", Criteria: []string{"实际发生"}}}}}
	hydrated, err := binding.hydrateOutput(goalEvaluationProviderFixture(snapshot, internal))
	if err != nil {
		t.Fatal(err)
	}
	if hydrated.Evaluations[0].GoalID != "goal-secret" || hydrated.Evaluations[0].ExpectedRevision != 7 || hydrated.Evaluations[0].CriteriaVersion != 3 || hydrated.Evaluations[0].Judgments[0].CriterionID != "criterion-required" || hydrated.Evaluations[0].Judgments[0].EvidenceRefs[0] != "source-raw-secret" {
		t.Fatalf("frozen authority was not restored: %#v", hydrated)
	}
	if hydrated.Evaluations[0].StageEvaluation.ID != "stage-secret" || hydrated.Evaluations[0].CommitmentEvaluations[0].ID != "commitment-secret" || hydrated.Evaluations[0].RelationshipConfirmation.TargetActorID != "target-secret" || hydrated.Evaluations[0].Review.StageID != "stage-secret" || hydrated.Plans[0].Stage.DependencyIDs[0] != "older-stage-secret" || hydrated.Plans[0].Commitment.ID != "commitment-secret" {
		t.Fatalf("typed object/actor/dependency refs were not restored: %#v", hydrated)
	}
	if jsonString(snapshot) != durableBefore {
		t.Fatal("Provider projection mutated the frozen durable snapshot")
	}
}

func TestGoalEvaluationResponseSchemaUsesOnlyFrozenOfferedRefs(t *testing.T) {
	snapshot := richGoalEvaluationWireSnapshot()
	binding, err := newGoalEvaluationWireBinding(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	schema := goalEvaluationResponseSchema(binding)
	evaluation := mapValue(mapValue(mapValue(schema["properties"])["evaluations"])["items"])
	evaluationProperties := mapValue(evaluation["properties"])
	assertSchemaEnum := func(label string, node map[string]any, want ...string) {
		t.Helper()
		values := arrayValue(node["enum"])
		if len(values) != len(want) {
			t.Fatalf("%s enum=%#v want=%#v", label, values, want)
		}
		for index, expected := range want {
			if stringValue(values[index]) != expected || stringValue(values[index]) == "" {
				t.Fatalf("%s enum=%#v want=%#v", label, values, want)
			}
		}
	}
	assertSchemaEnum("goal", mapValue(evaluationProperties["goal_ref"]), "goal:1")
	judgment := mapValue(mapValue(evaluationProperties["judgments"])["items"])
	assertSchemaEnum("criterion", mapValue(mapValue(judgment["properties"])["criterion_ref"]), "criterion:1.1", "criterion:1.2")
	assertSchemaEnum("evidence", mapValue(mapValue(mapValue(mapValue(judgment["properties"])["evidence_refs"])["items"])), "e1")
	assertSchemaEnum("stage", mapValue(mapValue(mapValue(evaluationProperties["stage_evaluation"])["properties"])["object_ref"]), "stage:1.1")
	assertSchemaEnum("commitment", mapValue(mapValue(mapValue(mapValue(evaluationProperties["commitment_evaluations"])["items"])["properties"])["object_ref"]), "commitment:1.1")
	assertSchemaEnum("actor", mapValue(mapValue(mapValue(evaluationProperties["relationship_confirmation"])["properties"])["target_actor_ref"]), "actor_target:1")
	wire := jsonString(schema)
	for _, raw := range []string{"goal-secret", "criterion-required", "stage-secret", "commitment-secret", "source-raw-secret", "target-secret"} {
		if strings.Contains(wire, raw) {
			t.Fatalf("raw authority escaped bounded schema: %s", raw)
		}
	}
}

func TestGoalEvaluationResponseSchemaOmitsUnavailableObjectRefs(t *testing.T) {
	snapshot := richGoalEvaluationWireSnapshot()
	snapshot.Goals[0].Stages = nil
	snapshot.Goals[0].Commitments = nil
	snapshot.Goals[0].Goal.CurrentStageID = ""
	binding, err := newGoalEvaluationWireBinding(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	schema := goalEvaluationResponseSchema(binding)
	evaluation := mapValue(mapValue(mapValue(schema["properties"])["evaluations"])["items"])
	evaluationProperties := mapValue(evaluation["properties"])
	for _, field := range []string{"stage_evaluation", "commitment_evaluations"} {
		if _, ok := evaluationProperties[field]; ok {
			t.Fatalf("no-object schema advertised %s", field)
		}
	}
	if _, ok := mapValue(mapValue(evaluationProperties["review"])["properties"])["stage_ref"]; ok {
		t.Fatal("review advertised a stage ref when no stage was served")
	}
	plan := mapValue(mapValue(mapValue(schema["properties"])["plans"])["items"])
	planProperties := mapValue(plan["properties"])
	if _, ok := mapValue(mapValue(planProperties["stage"])["properties"])["object_ref"]; ok {
		t.Fatal("stage creation schema advertised an unavailable object_ref")
	}
	if _, ok := mapValue(mapValue(planProperties["commitment"])["properties"])["object_ref"]; ok {
		t.Fatal("commitment creation schema advertised an unavailable object_ref")
	}
}

func TestGoalEvaluationWireRejectsUnknownWrongKindDuplicateAndLegacyAuthority(t *testing.T) {
	snapshot := richGoalEvaluationWireSnapshot()
	binding, _ := newGoalEvaluationWireBinding(snapshot)
	valid := goalEvaluationProviderFixture(snapshot, GoalEvaluationTaskOutput{Evaluations: []GoalEvaluationCandidate{{GoalID: "goal-secret", ExpectedRevision: 7, CriteriaVersion: 3, Judgments: []GoalCriterionJudgment{{CriterionID: "criterion-required", Verdict: "unknown", Kind: "communication", Subject: "actor_self", Discourse: "uncertain", EvidenceRefs: []string{}, Reason: "等待"}}, Impact: "needs_evidence", WaitCondition: "等待"}}, Plans: []GoalPlanCandidate{}})
	duplicateJudgment := decodeObject(jsonBytes(valid))
	evaluation := mapValue(arrayValue(duplicateJudgment["evaluations"])[0])
	evaluation["judgments"] = append(arrayValue(evaluation["judgments"]), arrayValue(evaluation["judgments"])[0])
	if _, err := binding.hydrateOutput(duplicateJudgment); err == nil || err.Error() != "goal_evaluation_wire_criterion_ref_invalid" {
		t.Fatalf("duplicate criterion accepted: %v", err)
	}
	badCases := []map[string]any{
		{"evaluations": []any{map[string]any{"goal_ref": "goal:404", "judgments": []any{}, "impact": "needs_evidence", "blocker": "", "wait_condition": "等待", "next_step": "", "residual_motivation": ""}}, "plans": []any{}},
		{"evaluations": []any{map[string]any{"goal_ref": "goal:1", "judgments": []any{map[string]any{"criterion_ref": "stage:1.1", "verdict": "unknown", "kind": "semantic", "subject": "domain", "discourse": "uncertain", "evidence_refs": []any{}, "reason": "wrong kind"}}, "impact": "needs_evidence", "blocker": "", "wait_condition": "等待", "next_step": "", "residual_motivation": ""}}, "plans": []any{}},
		{"evaluations": append(arrayValue(valid["evaluations"]), arrayValue(valid["evaluations"])[0]), "plans": []any{}},
		{"evaluations": []any{map[string]any{"goal_ref": "goal:1", "goal_id": "goal-secret", "judgments": []any{}, "impact": "needs_evidence", "blocker": "", "wait_condition": "等待", "next_step": "", "residual_motivation": ""}}, "plans": []any{}},
		goalEvaluationProviderFixture(snapshot, GoalEvaluationTaskOutput{Evaluations: []GoalEvaluationCandidate{{GoalID: "goal-secret", ExpectedRevision: 999, CriteriaVersion: 3, Judgments: []GoalCriterionJudgment{}, Impact: "needs_evidence", WaitCondition: "等待"}}, Plans: []GoalPlanCandidate{}}),
	}
	for index, value := range badCases {
		if _, err := binding.hydrateOutput(value); err == nil {
			t.Fatalf("bad case %d accepted: %#v", index, value)
		}
	}
}

func TestGoalEvaluationWireRejectsForeignInstanceAndPrivateProfileObjects(t *testing.T) {
	mutations := []func(*goalEvaluationSnapshot){
		func(snapshot *goalEvaluationSnapshot) { snapshot.Goals[0].Goal.FluctlightID = "foreign-instance" },
		func(snapshot *goalEvaluationSnapshot) { snapshot.Goals[0].Goal.ProfileID = "other-profile" },
		func(snapshot *goalEvaluationSnapshot) { snapshot.Goals[0].Stages[0].ProfileID = "other-profile" },
		func(snapshot *goalEvaluationSnapshot) { snapshot.Goals[0].Commitments[0].ProfileID = "other-profile" },
		func(snapshot *goalEvaluationSnapshot) { snapshot.Sources[0].FluctlightID = "foreign-instance" },
		func(snapshot *goalEvaluationSnapshot) { snapshot.Sources[0].ProfileID = "other-profile" },
	}
	for index, mutate := range mutations {
		snapshot := richGoalEvaluationWireSnapshot()
		mutate(&snapshot)
		if _, err := newGoalEvaluationWireBinding(snapshot); err == nil {
			t.Fatalf("foreign authority case %d entered Provider binding", index)
		}
	}
	shared := richGoalEvaluationWireSnapshot()
	shared.Goals[0].Goal.ProfileID = ""
	shared.Goals[0].Stages[0].ProfileID = "profile-secret"
	shared.Goals[0].Commitments[0].ProfileID = "profile-secret"
	if _, err := newGoalEvaluationWireBinding(shared); err != nil {
		t.Fatalf("active profile child of shared Goal was rejected: %v", err)
	}
}

func TestGoalEvaluationSharedEvidenceRetainsActualPublishingProfile(t *testing.T) {
	snapshot := richGoalEvaluationWireSnapshot()
	snapshot.ProfileID = ""
	snapshot.Goals[0].Goal.ProfileID = ""
	snapshot.Goals[0].Stages = nil
	snapshot.Goals[0].Commitments = nil
	snapshot.Goals[0].Goal.CurrentStageID = ""
	if _, err := newGoalEvaluationWireBinding(snapshot); err != nil {
		t.Fatalf("shared Goal lost actual Actor communication evidence: %v", err)
	}
	snapshot.Goals[0].Goal.ProfileID = "profile-secret"
	snapshot.ProfileID = "profile-secret"
	snapshot.Sources[0].ProfileID = "other-profile"
	if _, err := newGoalEvaluationWireBinding(snapshot); err == nil {
		t.Fatal("private Goal accepted another profile's source")
	}
}

func TestGoalEvaluationTOONHybridPreservesSemanticsAndMeasuresSameData(t *testing.T) {
	snapshot := richGoalEvaluationWireSnapshot()
	legacyBaseline := jsonString(snapshot)
	_, stable, current, err := goalEvaluationWireInput(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	jsonWire := renderGoalEvaluationProviderInput(stable, current)
	toonWire := formatProviderPromptContentWithMode(jsonString(map[string]any{"stable_definitions": stable, "current": current}), true)
	for _, value := range []string{"发送明确表达", "保留换行", "可选：得到真实回复", "我喜欢你。", "这是认真表达。", "false", "0"} {
		if !strings.Contains(toonWire, value) {
			t.Fatalf("TOON/YAML lost value %q: %s", value, toonWire)
		}
	}
	if strings.Index(jsonWire, `"stable_definitions":`) > strings.Index(jsonWire, `"current":`) {
		t.Fatal("stable definitions must precede volatile current input")
	}
	t.Logf("goal fixture old durable baseline bytes=%d estimated_tokens=%d; semantic JSON bytes=%d estimated_tokens=%d; semantic TOON/YAML bytes=%d estimated_tokens=%d", len([]byte(legacyBaseline)), EstimatePromptTokens(legacyBaseline), len([]byte(jsonWire)), EstimatePromptTokens(jsonWire), len([]byte(toonWire)), EstimatePromptTokens(toonWire))
}

func TestGoalEvaluationStableDefinitionsIgnoreVolatileEvidenceAndState(t *testing.T) {
	first := richGoalEvaluationWireSnapshot()
	_, stableBefore, currentBefore, err := goalEvaluationWireInput(first)
	if err != nil {
		t.Fatal(err)
	}
	second := richGoalEvaluationWireSnapshot()
	second.Goals[0].Goal.Status = GoalPaused
	second.Goals[0].Goal.Deadline = nil
	second.Goals[0].Stages[0].Status = "completed"
	second.Goals[0].Commitments[0].Blocker = "新的真实阻碍"
	second.Sources[0].Data["text"] = "新的证据"
	second.Sources[0].OccurredAt = second.Sources[0].OccurredAt.Add(time.Minute)
	_, stableAfter, currentAfter, err := goalEvaluationWireInput(second)
	if err != nil {
		t.Fatal(err)
	}
	if jsonString(stableBefore) != jsonString(stableAfter) {
		t.Fatalf("volatile facts changed stable definitions\nbefore=%s\nafter=%s", jsonString(stableBefore), jsonString(stableAfter))
	}
	if jsonString(currentBefore) == jsonString(currentAfter) {
		t.Fatal("volatile current input did not change")
	}
	second.Goals[0].Goal.DesiredOutcome = "经Owner明确修订的新目标"
	_, stableChanged, _, err := goalEvaluationWireInput(second)
	if err != nil {
		t.Fatal(err)
	}
	if jsonString(stableBefore) == jsonString(stableChanged) {
		t.Fatal("semantic definition change failed to invalidate stable prefix")
	}
}
