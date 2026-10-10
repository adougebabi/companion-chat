package core

import (
	"context"
	"errors"
	"sort"
	"strings"
)

const goalEvaluationInstruction = `你正在评估有限范围内的目标真实证据并提出当前下一步，不是在编写剧情。
保留原始目标目的与每项成功定义。表达心意只要求实际明确发送表达；建立双方确认关系必须双方在同一相关语境明确确认。用户一般想恋爱不等于选择摇光。单方表达、沉默、引用、假设、未发送草稿、模型自述、未来计划均不证明双方确认。
只能引用输入source的ref。message已实际落库，可以证明author当前说出的话；转述/报告他人行为不等于行为发生。输入valid=false或can_support_success=false只能做反证/阻碍，不能作为成功。获取必须有输入中真实object ref及入库证据；接受异步任务不等于结果已经生成；获取不等于穿着。QUERY只支持真实信息，不能证明执行了业务动作。
逐标准返回satisfied/not_satisfied/unknown、kind、subject、discourse、真实证据ref与理由。同一criterion_ref只能有一条judgment；多个相关证据合并到该条evidence_refs和reason中，不得按消息重复判定同一标准。not_satisfied必须在criterion_quote逐字引用该criterion原文中的非空未满足要求；更细的类型、风格、数量、阅读反馈等建议只能放optional_improvement，不能冒充原标准或阻止原标准完成。satisfied/unknown的criterion_quote可以为空。quotation/hypothesis/plan/report/uncertain不能用于satisfied。直接由消息author本人说出的真实普通陈述按assertion判断；只有消息正文是在引用小说/他人原话、虚构内容或转述报告时才用quotation/report，不能因为一句话谈到小说就把用户本人的偏好陈述标成quotation。criterion_ref、goal_ref、object_ref、stage_ref、dependency_refs和actor ref只能从输入选择，不得返回任何原始标识或版本。kind为communication/relationship/acquisition/information/semantic；可验证的获取/双方关系不能弱化为泛泛semantic。subject为actor_self/target_actor/both/domain；双方关系只能both且使用双方实际消息，不允许用一个人的说法伪造另一人的确认。discourse支持assertion/domain_fact/quotation/hypothesis/plan/refusal/report/uncertain；不确定必须unknown。
Goal、Stage、Commitment分别评估。承诺或一次试探完成不增加长期关系进度。严格按输入criteria_policy（all/any及optional_refs）判断：完成条件满足时impact必须为completed；未满足时不得completed。reason、next_step、wait_condition与impact必须一致，不能在文字中宣布已达成而结构化字段仍为progressed。不得新增篇幅比例、数量或其他输入标准没有规定的阈值。父Goal直接满足时即可结束，不强制重演尚未执行的Stage/Commitment；当evaluation impact=completed时，不得再为同一Goal返回plan。自然发生的后续聊天不会重新打开已完成Goal或成为续建理由。仅有仍未解决且符合人格、尊重拒绝的上层动机时提出followup候选，并说明residual_motivation。
双方明确确认关系后，relationship_confirmation引用双方真实证据、对应target_actor_ref和简短准确label；只修改动态关系，不改人格或现实用户生活状态。
对活动长期/复杂Goal给一个当前Stage和适当短期Commitment；简单获取目标可直接等待/安排Intention。阶段不铺未来剧情，承诺跨日仍可继续，不按每日配额创建。已有有效阶段/承诺不要重复创建；skip记录理由不当完成。下一步应有可观察结果，或明确等待/解除条件/下次复核时间。普通聊天无相关机会时正常等待，不制造错失机会；拒绝、睡眠、异地、权限和未知外部结果均约束计划。不要提高亲密度、焦虑或频率假装推进。
存在reviews输入时，各对应Goal的evaluation必须返回review：明确原因、continue/wait/adjust/pause/abandon、解释、实际证据；错失机会必须引用当时Stage和可行替代，不从无行动推断偷懒。无机会/未到时机/睡眠/等待/拒绝/系统评估未完成与尝试无效分别分类。软期限只能提出复核/调整，不能自行延长硬期限。只评估提供的Goal与criterion ref。返回evaluations和plans数组；输入中的每个Goal必须恰有一条evaluation，即使另有plan也不能省略evaluation，plans不能代替评估。每个Goal最多一条plan。没有现有Stage/Commitment就省略stage_evaluation/commitment_evaluations；没有对应对象时省略review.stage_ref。创建Stage/Commitment时省略object_ref，新对象由Core生成；只有adjust/skip/abandon引用输入中的现有同类对象。引用字段不得填描述、书名或原因。若无适用事件用unknown/needs_evidence，并留下合理等待；模型或来源不明不等价no_change。plans只提出领域阶段/承诺/下一步，不调用工具、不编造已执行动作。`

func goalCriterionJudgmentSchema(criterionRefs, evidenceRefs []string) map[string]any {
	return objectSchema(map[string]any{
		"criterion_ref": enumStringSchema(criterionRefs...), "verdict": enumStringSchema("satisfied", "not_satisfied", "unknown"),
		"criterion_quote": stringSchema(), "optional_improvement": stringSchema(),
		"kind":          enumStringSchema("communication", "relationship", "acquisition", "information", "semantic"),
		"subject":       enumStringSchema("actor_self", "target_actor", "both", "domain"),
		"discourse":     enumStringSchema("assertion", "domain_fact", "quotation", "hypothesis", "plan", "refusal", "report", "uncertain"),
		"evidence_refs": boundedGoalRefArraySchema(evidenceRefs), "reason": stringSchema(),
	}, []string{"criterion_ref", "criterion_quote", "optional_improvement", "verdict", "kind", "subject", "discourse", "evidence_refs", "reason"}, false)
}

func goalObjectEvaluationSchema(objectRefs, criterionRefs, evidenceRefs []string) map[string]any {
	return objectSchema(map[string]any{"object_ref": enumStringSchema(objectRefs...), "judgments": arraySchema(goalCriterionJudgmentSchema(criterionRefs, evidenceRefs)), "completed": booleanSchema(), "reason": stringSchema()}, []string{"object_ref", "judgments", "completed", "reason"}, false)
}

// Selectors are a per-Goal contract, never a union shared by unrelated Goals.
func goalEvaluationResponseSchema(binding *goalEvaluationWireBinding) map[string]any {
	evaluations := []any{}
	plans := []any{}
	for _, ref := range sortedGoalBindingRefs(binding.goalsByRef) {
		entry := binding.goalsByRef[ref]
		scoped := *binding
		scoped.goalsByRef = map[string]goalEvaluationGoal{ref: entry}
		scoped.criteriaByRef = map[string]goalWireCriterionBinding{}
		for key, value := range binding.criteriaByRef {
			if value.GoalID == entry.GoalID {
				scoped.criteriaByRef[key] = value
			}
		}
		scoped.stagesByRef = map[string]goalWireObjectBinding{}
		for key, value := range binding.stagesByRef {
			if value.GoalID == entry.GoalID {
				scoped.stagesByRef[key] = value
			}
		}
		scoped.commitmentsByRef = map[string]goalWireObjectBinding{}
		for key, value := range binding.commitmentsByRef {
			if value.GoalID == entry.GoalID {
				scoped.commitmentsByRef[key] = value
			}
		}
		scoped.dependenciesByRef = map[string]goalWireObjectBinding{}
		for key, value := range binding.dependenciesByRef {
			if value.GoalID == entry.GoalID {
				scoped.dependenciesByRef[key] = value
			}
		}
		schema := goalEvaluationScopedResponseSchema(&scoped)
		properties := mapValue(schema["properties"])
		evaluation := mapValue(mapValue(properties["evaluations"])["items"])
		for _, review := range binding.snapshot.Reviews {
			if review.GoalID == entry.GoalID {
				evaluation["required"] = append(arrayValue(evaluation["required"]), "review")
				break
			}
		}
		evaluations = append(evaluations, evaluation)
		plans = append(plans, mapValue(mapValue(properties["plans"])["items"]))
	}
	// Cardinality belongs in the physical response contract as well as hydration:
	// a syntactically valid result for only the first Goal is not a final answer.
	evaluationArray := arraySchema(goalSchemaAlternatives(evaluations))
	evaluationArray["minItems"] = len(evaluations)
	evaluationArray["maxItems"] = len(evaluations)
	planArray := arraySchema(goalSchemaAlternatives(plans))
	planArray["maxItems"] = len(plans)
	return objectSchema(map[string]any{"evaluations": evaluationArray, "plans": planArray}, []string{"evaluations", "plans"}, false)
}
func goalSchemaAlternatives(variants []any) map[string]any {
	if len(variants) == 1 {
		return mapValue(variants[0])
	}
	return map[string]any{"anyOf": variants}
}
func goalObjectScopedEvaluationSchema(binding *goalEvaluationWireBinding, objects map[string]goalWireObjectBinding, kind string, evidence []string) map[string]any {
	variants := []any{}
	for _, ref := range sortedGoalBindingRefs(objects) {
		object := objects[ref]
		criteria := []string{}
		for key, c := range binding.criteriaByRef {
			if c.ObjectKind == kind && c.ObjectID == object.ID {
				criteria = append(criteria, key)
			}
		}
		sort.Strings(criteria)
		variants = append(variants, goalObjectEvaluationSchema([]string{ref}, criteria, evidence))
	}
	return goalSchemaAlternatives(variants)
}
func goalOperationPlanSchema(properties map[string]any, required []string, refs []string, operations []string) map[string]any {
	create := cloneMap(properties)
	delete(create, "object_ref")
	create["operation"] = enumStringSchema("create")
	required = append(append([]string(nil), required...), "operation")
	variants := []any{objectSchema(create, required, false)}
	if len(refs) > 0 {
		mutate := cloneMap(properties)
		mutate["operation"] = enumStringSchema(operations...)
		mutate["object_ref"] = enumStringSchema(refs...)
		variants = append(variants, objectSchema(mutate, append(append([]string(nil), required...), "object_ref"), false))
	}
	return goalSchemaAlternatives(variants)
}

func goalEvaluationScopedResponseSchema(binding *goalEvaluationWireBinding) map[string]any {
	goalRefs := sortedGoalBindingRefs(binding.goalsByRef)
	criterionRefs := []string{}
	for ref, c := range binding.criteriaByRef {
		if c.ObjectKind == "goal" {
			criterionRefs = append(criterionRefs, ref)
		}
	}
	sort.Strings(criterionRefs)
	evidenceRefs := sortedGoalBindingRefs(binding.sourcesByRef)
	stageRefs := sortedGoalBindingRefs(binding.stagesByRef)
	commitmentRefs := sortedGoalBindingRefs(binding.commitmentsByRef)
	actorRefs := []string{}
	seenActorRefs := map[string]bool{}
	for _, entry := range binding.goalsByRef {
		if entry.Goal.Scope == "relationship" && entry.Goal.TargetActorID != "" {
			if ref := binding.actorRefs[entry.Goal.TargetActorID]; ref != "" && !seenActorRefs[ref] {
				seenActorRefs[ref] = true
				actorRefs = append(actorRefs, ref)
			}
		}
	}
	sort.Strings(actorRefs)
	dependencyRefs := append(append([]string{}, stageRefs...), sortedGoalBindingRefs(binding.dependenciesByRef)...)

	evaluationProperties := map[string]any{
		"goal_ref":  enumStringSchema(goalRefs...),
		"judgments": arraySchema(goalCriterionJudgmentSchema(criterionRefs, evidenceRefs)), "impact": enumStringSchema("progressed", "no_change", "blocked", "regressed", "needs_evidence", "completed"),
		"blocker": stringSchema(), "wait_condition": stringSchema(), "next_step": stringSchema(), "next_review_at": stringSchema(),
		"review":              goalReviewSchema(stageRefs, evidenceRefs),
		"residual_motivation": stringSchema(), "followup": objectSchema(map[string]any{"desired_outcome": stringSchema(), "success_criteria": arraySchema(stringSchema()), "motivation": stringSchema()}, []string{"desired_outcome", "success_criteria", "motivation"}, false),
	}
	if len(stageRefs) > 0 {
		evaluationProperties["stage_evaluation"] = goalObjectScopedEvaluationSchema(binding, binding.stagesByRef, "stage", evidenceRefs)
	}
	if len(commitmentRefs) > 0 {
		evaluationProperties["commitment_evaluations"] = arraySchema(goalObjectScopedEvaluationSchema(binding, binding.commitmentsByRef, "commitment", evidenceRefs))
	}
	if len(actorRefs) > 0 {
		evaluationProperties["relationship_confirmation"] = objectSchema(map[string]any{"target_actor_ref": enumStringSchema(actorRefs...), "evidence_refs": boundedGoalRefArraySchema(evidenceRefs), "label": stringSchema()}, []string{"target_actor_ref", "evidence_refs", "label"}, false)
	}
	evaluation := objectSchema(evaluationProperties, []string{"goal_ref", "judgments", "impact", "blocker", "wait_condition", "next_step", "residual_motivation"}, false)
	stageProperties := map[string]any{"dependency_refs": boundedGoalRefArraySchema(dependencyRefs), "operation": enumStringSchema("create", "adjust", "skip"), "purpose": stringSchema(), "strategy": stringSchema(), "entry_basis": stringSchema(), "exit_basis": stringSchema(), "criteria": arraySchema(stringSchema()), "reason": stringSchema()}
	if len(stageRefs) > 0 {
		stageProperties["object_ref"] = enumStringSchema(stageRefs...)
	}
	stage := goalOperationPlanSchema(stageProperties, []string{"purpose", "strategy", "entry_basis", "exit_basis", "criteria", "reason"}, stageRefs, []string{"adjust", "skip"})
	commitmentProperties := map[string]any{"operation": enumStringSchema("create", "adjust", "abandon"), "reason": stringSchema(), "expected_result": stringSchema(), "criteria": arraySchema(stringSchema()), "window_start": stringSchema(), "window_end": stringSchema(), "opportunity_condition": stringSchema(), "blocker": stringSchema()}
	if len(commitmentRefs) > 0 {
		commitmentProperties["object_ref"] = enumStringSchema(commitmentRefs...)
	}
	commitment := goalOperationPlanSchema(commitmentProperties, []string{"expected_result", "criteria", "opportunity_condition", "blocker"}, commitmentRefs, []string{"adjust", "abandon"})
	plan := objectSchema(map[string]any{"goal_ref": enumStringSchema(goalRefs...), "reason": stringSchema(), "next_step": stringSchema(), "wait_condition": stringSchema(), "next_review_at": stringSchema(), "stage": stage, "commitment": commitment}, []string{"goal_ref", "reason", "next_step", "wait_condition"}, false)
	return objectSchema(map[string]any{"evaluations": arraySchema(evaluation), "plans": arraySchema(plan)}, []string{"evaluations", "plans"}, false)
}

func goalReviewSchema(stageRefs, evidenceRefs []string) map[string]any {
	properties := map[string]any{"reason_category": enumStringSchema("progressed", "no_opportunity", "not_yet_due", "waiting_external", "ineffective_attempt", "blocked", "missed_opportunity", "evaluation_incomplete", "refusal", "sleeping"), "decision": enumStringSchema("continue", "wait", "adjust", "pause", "abandon"), "explanation": stringSchema(), "evidence_refs": boundedGoalRefArraySchema(evidenceRefs), "feasible_alternative": stringSchema()}
	if len(stageRefs) > 0 {
		properties["stage_ref"] = enumStringSchema(stageRefs...)
	}
	return objectSchema(properties, []string{"reason_category", "decision", "explanation", "evidence_refs", "feasible_alternative"}, false)
}

func boundedGoalRefArraySchema(refs []string) map[string]any {
	if len(refs) == 0 {
		return map[string]any{"type": "array", "maxItems": 0, "items": stringSchema()}
	}
	return arraySchema(enumStringSchema(refs...))
}

func sortedGoalBindingRefs[T any](values map[string]T) []string {
	refs := make([]string, 0, len(values))
	for ref := range values {
		if ref != "" {
			refs = append(refs, ref)
		}
	}
	sort.Strings(refs)
	return refs
}

func sortedTargetActorRefs(values map[string]string) []string {
	refs := make([]string, 0, len(values))
	for ref := range values {
		if strings.HasPrefix(ref, "actor_target:") {
			refs = append(refs, ref)
		}
	}
	sort.Strings(refs)
	return refs
}

func (a *App) RunGoalEvaluationTask(ctx context.Context, input goalEvaluationSnapshot, projection ContextProjection) (ProjectionTaskResult, error) {
	binding, stable, current, err := goalEvaluationWireInput(input)
	if err != nil {
		return ProjectionTaskResult{}, err
	}
	schema := goalEvaluationResponseSchema(binding)
	ctx = withProviderStableTaskContext(ctx, stable)
	providerInput := jsonString(current)
	assembly, refreshed, err := a.assembleProjectionPromptForSurface(ctx, ProviderContextSurfaceReflection, projection, "cognitive_assessment", []string{providerContextAuthorityRule, goalEvaluationInstruction}, providerInput, nil, "goal_evaluation_v1", schema)
	if err != nil {
		return ProjectionTaskResult{}, err
	}
	providerCtx := WithPromptDiagnostics(WithProviderCorrelation(WithProviderScenario(ctx, "goal_evaluation"), input.RequestID), assembly.Diagnostics)
	run, err := runGoalEvaluationWithCorrection(input, binding, assembly.Messages, func(messages []map[string]any) (ADKStructuredTaskResult, error) {
		// Keep the bounded completion reserve available for all Goals' final JSON.
		// Reasoning sidecars are diagnostic only and cannot settle omitted Goals.
		return a.runFormalStructuredTask(providerCtx, FormalAgentGoalEvaluation, messages, nil, "goal_evaluation_v1", schema, false, nil)
	})

	return ProjectionTaskResult{Completion: run.Completion, Projection: refreshed, Diagnostics: assembly.Diagnostics, Trace: run.Trace}, err
}

func goalEvaluationCandidateOutput(input goalEvaluationSnapshot, binding *goalEvaluationWireBinding, raw map[string]any) (GoalEvaluationTaskOutput, error) {
	output, err := binding.hydrateOutput(raw)
	if err != nil {
		return output, err
	}
	if len(missingGoalEvaluationCoverage(input, output)) > 0 {
		return output, errors.New("goal_assessment_coverage_missing")
	}
	if err := validateGoalEvaluationOutput(binding.snapshot, output); err != nil {
		return output, err
	}
	return output, nil
}

type terminalGoalEvaluationOutputError struct{ cause error }

func (e terminalGoalEvaluationOutputError) Error() string { return e.cause.Error() }
func (e terminalGoalEvaluationOutputError) Unwrap() error { return errADKFinalContractInvalid }

func correctableGoalEvaluationOutputError(err error) bool {
	if err == nil {
		return false
	}
	code := err.Error()
	return code == "goal_assessment_coverage_missing" || strings.HasPrefix(code, "goal_evaluation_wire_") ||
		code == "goal_evaluation_unknown_requires_evidence" || code == "goal_evaluation_impact_invalid" || strings.HasPrefix(code, "goal_judgment_") ||
		strings.HasPrefix(code, "goal_evaluation_completion_") || strings.HasPrefix(code, "goal_completion_") ||
		strings.HasPrefix(code, "goal_object_")
}

func runGoalEvaluationWithCorrection(input goalEvaluationSnapshot, binding *goalEvaluationWireBinding, messages []map[string]any, run func([]map[string]any) (ADKStructuredTaskResult, error)) (ADKStructuredTaskResult, error) {
	result, err := run(messages)
	if err != nil {
		return result, err
	}
	output, err := goalEvaluationCandidateOutput(input, binding, result.Completion.Structured)
	corrected := false
	if correctableGoalEvaluationOutputError(err) {
		corrected = true
		correction := append([]map[string]any(nil), messages...)
		correction = append(correction, map[string]any{"role": "assistant", "content": jsonString(result.Completion.Structured)}, map[string]any{"role": "user", "content": err.Error() + ": return one complete replacement object for all offered goals. Each criterion, stage, commitment and dependency must belong to its goal_ref. Completion must be reciprocal: all mandatory criteria satisfied requires completed, and completed requires all mandatory criteria satisfied. Every not_satisfied judgment must quote a literal nonempty substring of its frozen criterion in criterion_quote; put optional refinements only in optional_improvement. Do not copy stage:1.1 into other goals. A goal without an existing stage uses operation=create and omits object_ref; adjust/skip/abandon requires that goal's existing object_ref. Include requested reviews and exactly one evaluation per goal. Use only the frozen response schema and offered refs."})
		result, err = run(correction)
		if err != nil {
			return result, err
		}
		output, err = goalEvaluationCandidateOutput(input, binding, result.Completion.Structured)
	}
	if corrected && err != nil && correctableGoalEvaluationOutputError(err) && !errors.Is(err, errADKFinalContractInvalid) {
		err = terminalGoalEvaluationOutputError{cause: err}
	}
	if err == nil {
		result.Completion.Structured = decodeObject(jsonBytes(output))
	}
	return result, err
}

func missingGoalEvaluationCoverage(input goalEvaluationSnapshot, output GoalEvaluationTaskOutput) []string {
	covered := make(map[string]bool, len(output.Evaluations))
	for _, evaluation := range output.Evaluations {
		covered[evaluation.GoalID] = true
	}
	missing := make([]string, 0)
	for _, entry := range input.Goals {
		if !covered[entry.GoalID] {
			missing = append(missing, entry.GoalID)
		}
	}
	return missing
}

func goalEventCandidatesSchema() map[string]any {
	return arraySchema(objectSchema(map[string]any{"goal_ref": stringSchema(), "reason": stringSchema()}, []string{"goal_ref", "reason"}, false))
}

// compactGoalEvaluationInput is retained for focused admission/tests. It is a
// semantic Provider packet, never the durable snapshot.
func compactGoalEvaluationInput(input goalEvaluationSnapshot) map[string]any {
	_, stable, current, err := goalEvaluationWireInput(input)
	if err != nil {
		return map[string]any{"wire_error": err.Error()}
	}
	return map[string]any{"stable_definitions": stable, "current": current}
}
