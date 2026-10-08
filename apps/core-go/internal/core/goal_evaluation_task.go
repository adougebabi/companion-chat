package core

import "context"

const goalEvaluationInstruction = `你正在评估有限范围内的目标真实证据并提出当前下一步，不是在编写剧情。
保留原始目标目的与每项成功定义。表达心意只要求实际明确发送表达；建立双方确认关系必须双方在同一相关语境明确确认。用户一般想恋爱不等于选择摇光。单方表达、沉默、引用、假设、未发送草稿、模型自述、未来计划均不证明双方确认。
只能引用输入source的ref。message已实际落库，可以证明author当前说出的话；转述/报告他人行为不等于行为发生。输入valid=false或can_support_success=false只能做反证/阻碍，不能作为成功。获取必须真实物品ID及入库；接受异步任务不等于结果已经生成；获取不等于穿着。QUERY只支持真实信息，不能证明执行了业务动作。
逐标准返回satisfied/not_satisfied/unknown、kind、subject、discourse、真实证据ref与理由。kind为communication/relationship/acquisition/information/semantic；可验证的获取/双方关系不能弱化为泛泛semantic。subject为actor_self/target_actor/both/domain；双方关系只能both且使用双方实际消息，不允许用一个人的说法伪造另一人的确认。discourse支持assertion/domain_fact/quotation/hypothesis/plan/refusal/report/uncertain；不确定必须unknown。
Goal、Stage、Commitment分别评估。承诺或一次试探完成不增加长期关系进度。必要标准未全部满足，不得completed。父Goal直接满足时即可结束，不强制重演尚未执行的Stage/Commitment；结束后不要无条件续一个Goal。仅有仍未解决且符合人格、尊重拒绝的上层动机时提出followup候选，并说明residual_motivation。
双方明确确认关系后，relationship_confirmation引用双方真实证据、对应target_actor_id和简短准确label；只修改动态关系，不改人格或现实用户生活状态。
对活动长期/复杂Goal给一个当前Stage和适当短期Commitment；简单获取目标可直接等待/安排Intention。阶段不铺未来剧情，承诺跨日仍可继续，不按每日配额创建。已有有效阶段/承诺不要重复创建；skip记录理由不当完成。下一步应有可观察结果，或明确等待/解除条件/下次复核时间。普通聊天无相关机会时正常等待，不制造错失机会；拒绝、睡眠、异地、权限和未知外部结果均约束计划。不要提高亲密度、焦虑或频率假装推进。
存在reviews输入时，各对应Goal的evaluation必须返回review：明确原因、continue/wait/adjust/pause/abandon、解释、实际证据；错失机会必须引用当时Stage和可行替代，不从无行动推断偷懒。无机会/未到时机/睡眠/等待/拒绝/系统评估未完成与尝试无效分别分类。软期限只能提出复核/调整，不能自行延长硬期限。只评估提供的Goal ID/版本与criterion ID。返回evaluations和plans数组；每个Goal各最多一项。若无适用事件用unknown/needs_evidence，并留下合理等待；模型或来源不明不等价no_change。plans只提出领域阶段/承诺/下一步，不调用工具、不编造已执行动作。`

func goalCriterionJudgmentSchema() map[string]any {
	return objectSchema(map[string]any{
		"criterion_id": stringSchema(), "verdict": enumStringSchema("satisfied", "not_satisfied", "unknown"),
		"kind":          enumStringSchema("communication", "relationship", "acquisition", "information", "semantic"),
		"subject":       enumStringSchema("actor_self", "target_actor", "both", "domain"),
		"discourse":     enumStringSchema("assertion", "domain_fact", "quotation", "hypothesis", "plan", "refusal", "report", "uncertain"),
		"evidence_refs": arraySchema(stringSchema()), "reason": stringSchema(),
	}, []string{"criterion_id", "verdict", "kind", "subject", "discourse", "evidence_refs", "reason"}, false)
}

func goalObjectEvaluationSchema() map[string]any {
	return objectSchema(map[string]any{"id": stringSchema(), "expected_revision": integerSchema(), "criteria_version": integerSchema(), "judgments": arraySchema(goalCriterionJudgmentSchema()), "completed": booleanSchema(), "reason": stringSchema()}, []string{"id", "expected_revision", "criteria_version", "judgments", "completed", "reason"}, false)
}

func goalEvaluationResponseSchema() map[string]any {
	evaluation := objectSchema(map[string]any{
		"goal_id": stringSchema(), "expected_revision": integerSchema(), "criteria_version": integerSchema(),
		"judgments": arraySchema(goalCriterionJudgmentSchema()), "impact": enumStringSchema("progressed", "no_change", "blocked", "regressed", "needs_evidence", "completed"),
		"blocker": stringSchema(), "wait_condition": stringSchema(), "next_step": stringSchema(), "next_review_at": stringSchema(),
		"stage_evaluation": goalObjectEvaluationSchema(), "commitment_evaluations": arraySchema(goalObjectEvaluationSchema()),
		"relationship_confirmation": objectSchema(map[string]any{"target_actor_id": stringSchema(), "evidence_refs": arraySchema(stringSchema()), "label": stringSchema()}, []string{"target_actor_id", "evidence_refs", "label"}, false),
		"review":                    objectSchema(map[string]any{"reason_category": enumStringSchema("progressed", "no_opportunity", "not_yet_due", "waiting_external", "ineffective_attempt", "blocked", "missed_opportunity", "evaluation_incomplete", "refusal", "sleeping"), "decision": enumStringSchema("continue", "wait", "adjust", "pause", "abandon"), "explanation": stringSchema(), "evidence_refs": arraySchema(stringSchema()), "stage_id": stringSchema(), "feasible_alternative": stringSchema()}, []string{"reason_category", "decision", "explanation", "evidence_refs", "stage_id", "feasible_alternative"}, false),
		"residual_motivation":       stringSchema(), "followup": objectSchema(map[string]any{"desired_outcome": stringSchema(), "success_criteria": arraySchema(stringSchema()), "motivation": stringSchema()}, []string{"desired_outcome", "success_criteria", "motivation"}, false),
	}, []string{"goal_id", "expected_revision", "criteria_version", "judgments", "impact", "blocker", "wait_condition", "next_step", "residual_motivation"}, false)
	stage := objectSchema(map[string]any{"dependency_ids": arraySchema(stringSchema()), "operation": enumStringSchema("create", "adjust", "skip"), "id": stringSchema(), "expected_revision": integerSchema(), "purpose": stringSchema(), "strategy": stringSchema(), "entry_basis": stringSchema(), "exit_basis": stringSchema(), "criteria": arraySchema(stringSchema()), "reason": stringSchema()}, []string{"operation", "expected_revision", "purpose", "strategy", "entry_basis", "exit_basis", "criteria", "reason"}, false)
	commitment := objectSchema(map[string]any{"operation": enumStringSchema("create", "adjust", "abandon"), "reason": stringSchema(), "id": stringSchema(), "expected_revision": integerSchema(), "expected_result": stringSchema(), "criteria": arraySchema(stringSchema()), "window_start": stringSchema(), "window_end": stringSchema(), "opportunity_condition": stringSchema(), "blocker": stringSchema()}, []string{"expected_revision", "expected_result", "criteria", "opportunity_condition", "blocker"}, false)
	plan := objectSchema(map[string]any{"goal_id": stringSchema(), "expected_revision": integerSchema(), "criteria_version": integerSchema(), "reason": stringSchema(), "next_step": stringSchema(), "wait_condition": stringSchema(), "next_review_at": stringSchema(), "stage": stage, "commitment": commitment}, []string{"goal_id", "expected_revision", "criteria_version", "reason", "next_step", "wait_condition"}, false)
	return objectSchema(map[string]any{"evaluations": arraySchema(evaluation), "plans": arraySchema(plan)}, []string{"evaluations", "plans"}, false)
}

func (a *App) RunGoalEvaluationTask(ctx context.Context, input goalEvaluationSnapshot, projection ContextProjection) (ProjectionTaskResult, error) {
	schema := goalEvaluationResponseSchema()
	assembly, refreshed, err := a.assembleProjectionPromptForSurface(ctx, ProviderContextSurfaceReflection, projection, "cognitive_assessment", []string{providerContextAuthorityRule, goalEvaluationInstruction}, jsonString(compactGoalEvaluationInput(input)), nil, "goal_evaluation_v1", schema)
	if err != nil {
		return ProjectionTaskResult{}, err
	}
	providerCtx := WithPromptDiagnostics(WithProviderCorrelation(WithProviderScenario(ctx, "goal_evaluation"), input.RequestID), assembly.Diagnostics)
	run, err := a.runFormalStructuredTask(providerCtx, FormalAgentGoalEvaluation, assembly.Messages, nil, "goal_evaluation_v1", schema, true, nil)
	return ProjectionTaskResult{Completion: run.Completion, Projection: refreshed, Diagnostics: assembly.Diagnostics, Trace: run.Trace}, err
}

func goalEventCandidatesSchema() map[string]any {
	return arraySchema(objectSchema(map[string]any{"goal_ref": stringSchema(), "reason": stringSchema()}, []string{"goal_ref", "reason"}, false))
}

// Keep the full CAS snapshot durable, but send only decision-relevant authority.
// IDs, criteria and evidence stay intact; audit/retry metadata is read on demand.
func compactGoalEvaluationInput(input goalEvaluationSnapshot) map[string]any {
	goals := []any{}
	for _, entry := range input.Goals {
		g := entry.Goal
		criteria := []any{}
		for i, id := range g.CriterionIDs {
			criteria = append(criteria, map[string]any{"id": id, "text": g.SuccessCriteria[i]})
		}
		goals = append(goals, map[string]any{"goal_id": g.EntityID, "revision": g.Revision, "criteria_version": effectiveGoalCriteriaVersion(g), "criteria": criteria, "criteria_policy": nonNilGoalCriteriaPolicy(g.CriteriaPolicy), "desired_outcome": g.DesiredOutcome, "motivation": g.Motivation, "status": g.Status, "scope": g.Scope, "profile_id": g.ProfileID, "target_actor_id": g.TargetActorID, "deadline": g.Deadline, "deadline_policy": g.DeadlinePolicy, "current_stage_id": g.CurrentStageID, "execution_hint": g.ExecutionHint, "stages": entry.Stages, "commitments": entry.Commitments, "current_judgments": entry.CurrentJudgments, "review_policy": effectiveGoalReviewPolicy(g.ReviewPolicy)})
	}
	sources := []any{}
	for _, s := range admittedGoalEvaluationSources(input) {
		if s.Kind == "goal_revision" {
			continue
		}
		sources = append(sources, map[string]any{"ref": s.Ref, "kind": s.Kind, "id": s.ID, "version": s.Version, "profile_id": s.ProfileID, "conversation_id": s.ConversationID, "subject_actor_id": s.SubjectActorID, "occurred_at": s.OccurredAt, "valid": s.Valid, "can_support_success": s.CanSupportSuccess, "goal_ids": s.GoalIDs, "data": compactGoalEvaluationSourceData(s)})
	}
	return map[string]any{"actor_self": input.FluctlightID, "actor_user": input.OwnerActorID, "goals": goals, "sources": sources, "reviews": input.Reviews}
}
