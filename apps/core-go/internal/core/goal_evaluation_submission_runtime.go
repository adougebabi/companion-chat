package core

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

func (a *App) recordGoalEvaluationSubmissionErrors(ctx context.Context, snapshot goalEvaluationSnapshot, trace *ADKCapabilityTrace) error {
	if trace == nil {
		return nil
	}
	_, receipts := trace.Snapshot()
	errors := []map[string]any{}
	for _, receipt := range receipts {
		if isGoalEvaluationTool(receipt.CapabilityName) && receipt.ErrorCode != "" {
			errors = append(errors, map[string]any{"call_id": receipt.CallID, "capability_name": receipt.CapabilityName, "status": receipt.Status, "error_code": receipt.ErrorCode})
		}
	}
	recordCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_, err := a.DB.Pool().Exec(recordCtx, `UPDATE public.goal_evaluation_requests SET result=jsonb_set(result,'{submission_errors}',$3::jsonb,true),updated_at=now() WHERE id=$1 AND status='processing' AND claim_revision=$2`, snapshot.RequestID, snapshot.ClaimRevision, jsonBytes(errors))
	return err
}

func (a *App) goalEvaluationSubmissionResult(ctx context.Context, snapshot goalEvaluationSnapshot) (map[string]any, error) {
	var raw []byte
	err := a.DB.Pool().QueryRow(ctx, `SELECT result FROM public.goal_evaluation_requests WHERE id=$1`, snapshot.RequestID).Scan(&raw)
	return decodeObject(raw), err
}

const goalEvaluationNativeInstruction = `你是专用 Goal Evaluation Agent，评估输入中冻结的目标与真实证据。实际状态只由原生 Tool 提交，最终只返回 summary，文字不能修改目标。
只使用输入的 goal_ref、criterion_ref、object_ref、stage_ref、actor ref 和 eN 证据引用；Core 绑定真实 ID、版本、所有权和请求身份。每个目标必须通过 goal.evaluation.submit 提交一次根目标评估。accepted_submissions 中已有的提交已落库，不重复评估或改变它。逐个读取真实 Tool 结果；失败表示本次未提交，按 error_code 修正对应调用，不能声称成功，也不重做已接受的其他目标。
保留原目的和成功标准。每项 criterion_ref 只有一条 judgment，合并其证据。kind 为 communication/relationship/acquisition/information/semantic，subject 为 actor_self/target_actor/both/domain，discourse 为 assertion/domain_fact/quotation/hypothesis/plan/refusal/report/uncertain。证据不明用 unknown；quotation/hypothesis/plan/report/uncertain 不可 satisfied。not_satisfied 的 criterion_quote 必须逐字引用原标准中的非空未满足要求；其他建议只放 optional_improvement。不可新增原标准没有要求的类型、篇幅、阅读反馈或阈值。
消息证明 author 实际说出的话。用户本人说“看小说”是 assertion；转述别人做了什么不是业务事实。表达心意只要求自己实际明确发出表达；双方关系须双方在相关语境分别明确确认，用 both 与双方真实证据，用户一般想恋爱不等于选择摇光。模型自述、草稿、沉默、未来计划不能证明结果。获取须真实入库记录及 object ref；获取不等于穿着，accepted 不等于结果已生成。valid=false 或 can_support_success=false 不能证明成功。
按 criteria_policy 判定。原目标完成条件满足，impact 必须 completed；未满足不能 completed。父目标满足就先提交根目标完成，不强制重演阶段/承诺，不为它提交计划，不把“等待读后感”等后续聊天变为未完成要求。Core 返回 status=completed 才能报告完成；paused 返回 ready_for_settlement 仍保持暂停。
根目标 Tool 不接受阶段/承诺评估。未完成目标的现有阶段或承诺，用 goal.object.submit 独立评估；其完成不证明父目标完成。阶段/承诺报错不影响其他提交。父目标已完成后，无需再提交子对象。
需要当前计划时用 goal.plan.submit。已有有效对象不重复创建；create 省略 object_ref；adjust/skip/abandon 只引用本目标冻结的现有对象。新对象由 Core 创建，下轮评估其标准。先评估旧子对象，再修改计划。review.decision=adjust 必须先提交成功计划，再提交根目标和 review。其他目标优先先提交根评估，避免可选计划错误阻塞完成。每个目标最多一个计划、每个子对象最多一次成功评估。
reviews 中列出的目标必须在根目标提交中包含 review：reason_category、continue/wait/adjust/pause/abandon、解释、真实证据、feasible_alternative；无阶段省略 stage_ref。错失机会必须引用当时阶段和可行替代，不能从无行动推断偷懒。软期限仅复核，不延长硬期限。等待外部反馈、睡眠、拒绝、权限和未知结果限制下一步；不要制造配额、亲密度或频率。未完成活动目标提供可观察 next_step 或明确 wait_condition/blocker/next_review_at。
双方确认关系完成时，在根目标提交中附 relationship_confirmation（对应 target_actor_ref、双方 evidence_refs、准确 label）。只改变动态关系，不改变人格和用户现实状态。完成后的新方向只能作 followup 候选交给 Planner；不得重新打开原目标。最终 summary 简述实际 Tool 结果和未解决错误。`

func goalEvaluationAcceptedWireResults(binding *goalEvaluationWireBinding, result map[string]any) []map[string]any {
	values := []map[string]any{}
	for _, record := range goalSubmissionRecords(result) {
		values = append(values, cloneMap(record.Output))
	}
	return values
}

func goalEvaluationSubmissionCoverage(snapshot goalEvaluationSnapshot, records map[string]goalSubmissionRecord) ([]string, []string) {
	accepted, missing := []string{}, []string{}
	for _, entry := range snapshot.Goals {
		if _, ok := records[goalSubmissionKey(goalEvaluationSubmit, entry.GoalID, "")]; ok {
			accepted = append(accepted, entry.GoalID)
		} else {
			missing = append(missing, entry.GoalID)
		}
	}
	return accepted, missing
}

func goalEvaluationNeedsFreshSnapshot(result map[string]any) bool {
	for _, value := range arrayValue(result["submission_errors"]) {
		switch stringValue(mapValue(value)["error_code"]) {
		case "conflict", "goal_evaluation_source_stale", "goal_submission_authority_stale", "goal_object_evaluation_version_conflict", "goal_stage_revision_conflict", "foundation_revision_stale", "life_context_stale":
			return true
		}
	}
	return false
}

func goalEvaluationReplacementTargets(snapshot goalEvaluationSnapshot, result map[string]any) []string {
	return mergeStableRefs(decisionServiceRefValues(result["unresolved_goals"]), snapshot.DeferredGoalIDs)
}

// A changed authority cannot be corrected with frozen versions. Preserve
// accepted siblings and queue only unresolved roots with a fresh snapshot.
func (a *App) replaceStaleGoalEvaluation(ctx context.Context, snapshot goalEvaluationSnapshot, result map[string]any) (map[string]any, error) {
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if err := lockLifeContextTx(ctx, tx, snapshot.FluctlightID); err != nil {
			return err
		}
		missing := goalEvaluationReplacementTargets(snapshot, result)
		rows, err := tx.Query(ctx, `SELECT id FROM public.fluctlight_goals WHERE fluctlight_id=$1 AND (profile_id IS NULL OR profile_id=$2) AND status IN ('active','paused') AND id=ANY($3::text[]) ORDER BY id`, snapshot.FluctlightID, snapshot.ProfileID, missing)
		if err != nil {
			return err
		}
		active := []string{}
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			active = append(active, id)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		replacement := ""
		if len(active) > 0 {
			replacement, err = queueGoalEvaluationTx(ctx, tx, snapshot.FluctlightID, snapshot.ProfileID, "assessment_snapshot_changed", snapshot.RequestID+":fresh", active)
		}
		if err != nil {
			return err
		}
		if replacement != "" {
			if _, err := tx.Exec(ctx, `UPDATE public.goal_reviews SET evaluation_request_id=$1 WHERE evaluation_request_id=$2 AND status='pending' AND goal_id=ANY($3::text[])`, replacement, snapshot.RequestID, active); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE public.goal_reviews r SET status='superseded',explanation='Goal ended before replacement evaluation',updated_at=now() FROM public.fluctlight_goals g WHERE r.evaluation_request_id=$1 AND r.status='pending' AND r.goal_id=ANY($2::text[]) AND g.id=r.goal_id AND g.status NOT IN ('active','paused')`, snapshot.RequestID, missing); err != nil {
			return err
		}
		result["replacement_request_id"] = replacement
		tag, err := tx.Exec(ctx, `UPDATE public.goal_evaluation_requests SET status='failed',result=$3,claimed_at=NULL,error_code='goal_evaluation_snapshot_stale',updated_at=now() WHERE id=$1 AND status='processing' AND claim_revision=$2`, snapshot.RequestID, snapshot.ClaimRevision, jsonBytes(result))
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return ErrConflict
		}
		return nil
	})
	return result, err
}

// Finalization reads the durable journal, never the assistant's final DTO or
// trace as a substitute for native execution. It cannot roll back Tool commits.
func (a *App) finalizeGoalEvaluationSubmissions(ctx context.Context, snapshot, claimed goalEvaluationSnapshot, projection ContextProjection) (map[string]any, error) {
	var result map[string]any
	var missing []string
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if err := lockLifeContextTx(ctx, tx, snapshot.FluctlightID); err != nil {
			return err
		}
		var state string
		var claim int
		var raw []byte
		if err := tx.QueryRow(ctx, `SELECT status,claim_revision,result FROM public.goal_evaluation_requests WHERE id=$1 FOR UPDATE`, snapshot.RequestID).Scan(&state, &claim, &raw); err != nil {
			return err
		}
		if state != "processing" || claim != snapshot.ClaimRevision {
			return ErrConflict
		}
		result = decodeObject(raw)
		if result == nil {
			result = map[string]any{}
		}
		records := goalSubmissionRecords(result)
		accepted, unresolved := goalEvaluationSubmissionCoverage(snapshot, records)
		missing = unresolved
		result["request_id"], result["evaluated_goals"], result["unresolved_goals"], result["skipped_goals"], result["source_ids"] = snapshot.RequestID, accepted, missing, snapshot.SkippedGoalIDs, snapshot.SourceIDs
		outcomes := []map[string]any{}
		for _, id := range accepted {
			record := records[goalSubmissionKey(goalEvaluationSubmit, id, "")]
			outcome := cloneMap(record.Output)
			outcome["goal_id"] = id
			// Later accepted plans/review commands may change the same Goal; use
			// its fenced live state for Owner diagnostics.
			var status string
			var progress float64
			var hint []byte
			if err := tx.QueryRow(ctx, `SELECT status,progress,execution_hint FROM public.fluctlight_goals WHERE id=$1 AND fluctlight_id=$2`, id, snapshot.FluctlightID).Scan(&status, &progress, &hint); err != nil {
				return err
			}
			outcome["status"], outcome["progress"], outcome["ready_for_settlement"] = status, progress, decodeObject(hint)["ready_for_settlement"] == true
			outcomes = append(outcomes, outcome)
		}
		result["goal_outcomes"] = outcomes
		if len(missing) > 0 {
			result["status"] = "partial"
			_, err := tx.Exec(ctx, `UPDATE public.goal_evaluation_requests SET result=$2,updated_at=now() WHERE id=$1`, snapshot.RequestID, jsonBytes(result))
			return err
		}
		// Skipped assessments require their original authority before consuming
		// the shared source journal. Accepted Tool submissions already fenced it.
		claimed.Goals = append(claimed.Goals, snapshot.SkippedGoals...)
		if err := verifySkippedGoalAssessmentTx(ctx, tx, claimed, projection, snapshot.SkippedGoalIDs, a.now()); err != nil {
			return err
		}
		if err := invalidateGoalSourceLinksTx(ctx, tx, snapshot.Sources); err != nil {
			return err
		}
		// Re-read fingerprints: a source changed after one Goal's submission
		// remains pending rather than being consumed as if all saw its new version.
		if len(snapshot.DeferredGoalIDs) == 0 {
			for _, source := range admittedGoalEvaluationSources(snapshot) {
				live, err := readGoalSourceWith(ctx, tx, source.EventID, true)
				if err != nil {
					return err
				}
				if source.Kind != "goal_revision" && goalAssessmentSourceFingerprint(live) != goalAssessmentSourceFingerprint(source) {
					if err := invalidateGoalSourceLinksTx(ctx, tx, []GoalSource{live}); err != nil {
						return err
					}
					continue
				}
				if _, err := tx.Exec(ctx, `UPDATE public.goal_source_events SET processed_at=now() WHERE id=$1 AND fluctlight_id=$2`, source.EventID, snapshot.FluctlightID); err != nil {
					return err
				}
			}
		}
		if len(snapshot.DeferredGoalIDs) > 0 {
			remainder, err := queueGoalEvaluationTx(ctx, tx, snapshot.FluctlightID, snapshot.ProfileID, "assessment_batch_remainder", snapshot.RequestID+":remainder", snapshot.DeferredGoalIDs)
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE public.goal_reviews SET evaluation_request_id=$1 WHERE evaluation_request_id=$2 AND status='pending' AND goal_id=ANY($3::text[])`, remainder, snapshot.RequestID, snapshot.DeferredGoalIDs); err != nil {
				return err
			}
		}
		if len(snapshot.DeferredGoalIDs) == 0 {
			var remaining bool
			if err := tx.QueryRow(ctx, pendingLinkedGoalSourceSQL, snapshot.FluctlightID, snapshot.ProfileID).Scan(&remaining); err != nil {
				return err
			}
			if remaining {
				active := []string{}
				for _, entry := range snapshot.Goals {
					var status string
					if err := tx.QueryRow(ctx, `SELECT status FROM public.fluctlight_goals WHERE id=$1`, entry.GoalID).Scan(&status); err != nil {
						return err
					}
					if status == string(GoalActive) || status == string(GoalPaused) {
						active = append(active, entry.GoalID)
					}
				}
				if len(active) > 0 {
					if _, err := queueGoalEvaluationTx(ctx, tx, snapshot.FluctlightID, snapshot.ProfileID, "assessment_source_remainder", snapshot.RequestID+":source-remainder", active); err != nil {
						return err
					}
				}
			}
		}
		result["status"] = "succeeded"
		_, err := tx.Exec(ctx, `UPDATE public.goal_evaluation_requests SET status='succeeded',result=$2,claimed_at=NULL,error_code=NULL,updated_at=now() WHERE id=$1`, snapshot.RequestID, jsonBytes(result))
		return err
	})
	if err == nil && len(missing) > 0 {
		err = errors.New("goal_evaluation_native_submission_missing")
	}
	return result, err
}
