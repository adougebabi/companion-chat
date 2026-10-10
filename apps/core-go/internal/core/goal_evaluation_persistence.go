package core

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func goalEvidenceRefs(judgments []GoalCriterionJudgment) []string {
	refs := []string{}
	for _, j := range judgments {
		refs = mergeStableRefs(refs, j.EvidenceRefs)
	}
	return refs
}

func (a *App) commitGoalEvaluationTx(ctx context.Context, tx pgx.Tx, current GoalAuthority, candidate GoalEvaluationCandidate, sources map[string]GoalSource, requestID string) (GoalAuthority, error) {
	evaluationID := "goal_evaluated_" + stableDigest(requestID+"\x1f"+current.EntityID)
	var existing bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.goal_evaluations WHERE id=$1)`, evaluationID).Scan(&existing); err != nil {
		return GoalAuthority{}, err
	}
	if existing {
		return current, nil
	}
	for ref, source := range sources {
		live, err := readGoalSourceWith(ctx, tx, source.EventID, true)
		if err != nil {
			return GoalAuthority{}, err
		}
		if live.Version != source.Version || live.Valid != source.Valid || live.FluctlightID != source.FluctlightID {
			return GoalAuthority{}, errors.New("goal_evaluation_source_stale")
		}
		sources[ref] = live
	}
	next, record, judgments, err := ApplyGoalEvaluation(current, candidate, sources, a.now().UTC())
	if err != nil {
		return GoalAuthority{}, err
	}
	if goalStatusTerminal(current.Status) {
		return current, nil
	}
	if current.Status == GoalActive {
		next.ExecutionHint = map[string]any{"state": candidate.Impact, "blocker": candidate.Blocker, "wait_condition": candidate.WaitCondition, "next_step": candidate.NextStep, "evaluation_id": evaluationID}
		if candidate.NextReviewAt != nil {
			next.ExecutionHint["next_review_at"] = formatInstant(*candidate.NextReviewAt)
		}
		if candidate.NextStep == "" && candidate.WaitCondition == "" && candidate.Blocker == "" && next.Status != GoalCompleted {
			next.ExecutionHint["state"] = "needs_planning"
		}
	} else {
		next.ExecutionHint = map[string]any{"state": current.Status, "pending_evaluation_id": evaluationID, "ready_for_settlement": candidate.Impact == "completed"}
	}
	if candidate.Impact == "completed" {
		for _, j := range judgments {
			if j.Verdict == "satisfied" && j.Kind == "relationship" && candidate.RelationshipConfirmation == nil {
				return GoalAuthority{}, errors.New("goal_relationship_confirmation_required")
			}
		}
	}
	if candidate.RelationshipConfirmation != nil {
		relationshipRefs := []string{}
		for _, judgment := range judgments {
			if judgment.Kind == "relationship" && judgment.Verdict == "satisfied" {
				relationshipRefs = mergeStableRefs(relationshipRefs, judgment.EvidenceRefs)
			}
		}
		if current.Scope != "relationship" || len(relationshipRefs) == 0 {
			return GoalAuthority{}, errors.New("goal_relationship_resolution_scope_invalid")
		}
		for _, ref := range candidate.RelationshipConfirmation.EvidenceRefs {
			if !containsString(relationshipRefs, ref) {
				return GoalAuthority{}, errors.New("goal_relationship_resolution_evidence_mismatch")
			}
		}
		if candidate.Impact != "completed" {
			return GoalAuthority{}, errors.New("goal_relationship_resolution_requires_completion")
		}
		confirmation := candidate.RelationshipConfirmation
		if confirmation.TargetActorID != current.TargetActorID || current.TargetActorID == "" {
			return GoalAuthority{}, errors.New("goal_relationship_target_invalid")
		}
		both := GoalCriterionJudgment{Kind: "relationship", Discourse: "assertion", Subject: "both", EvidenceRefs: confirmation.EvidenceRefs}
		if err := validateGoalSourceForJudgment(current, both, sources); err != nil {
			return GoalAuthority{}, err
		}
		if current.Status == GoalActive {
			if err := a.confirmGoalRelationshipTx(ctx, tx, current, confirmation, evaluationID); err != nil {
				return GoalAuthority{}, err
			}
		}
	}
	if candidate.StageEvaluation != nil && candidate.StageEvaluation.Completed && current.Status == GoalActive && current.CurrentStageID == candidate.StageEvaluation.ID {
		next.CurrentStageID = ""
	}

	if len(candidate.CommitmentEvaluations) > 8 {
		return GoalAuthority{}, errors.New("goal_commitment_evaluation_limit")
	}
	for _, evaluation := range candidate.CommitmentEvaluations {
		if err := applyGoalObjectEvaluationTx(ctx, tx, current, evaluation, "commitment", sources, evaluationID); err != nil {
			return GoalAuthority{}, err
		}
	}
	if candidate.StageEvaluation != nil {
		if err := applyGoalObjectEvaluationTx(ctx, tx, current, *candidate.StageEvaluation, "stage", sources, evaluationID); err != nil {
			return GoalAuthority{}, err
		}
	}
	if next.Status == GoalCompleted {
		next.CurrentStageID = ""
	}
	if _, err := persistGoalAuthorityTx(ctx, tx, &current, next, record, evaluationID); err != nil {
		return GoalAuthority{}, err
	}
	refs := goalEvidenceRefs(judgments)
	// Stage and commitment judgments also establish durable source associations.
	// Keep their proof available after the source journal is consumed, without
	// treating subobject completion as satisfaction of the parent's criteria.
	if candidate.StageEvaluation != nil {
		refs = mergeStableRefs(refs, goalEvidenceRefs(candidate.StageEvaluation.Judgments))
	}
	for _, evaluation := range candidate.CommitmentEvaluations {
		refs = mergeStableRefs(refs, goalEvidenceRefs(evaluation.Judgments))
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.goal_evaluations(id,fluctlight_id,goal_id,request_id,goal_revision,criteria_version,strategy_version,impact,judgments,evidence_refs,blocker,wait_condition,next_step,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`, evaluationID, current.FluctlightID, current.EntityID, nullableString(requestID), current.Revision, effectiveGoalCriteriaVersion(current), goalEvaluationPolicyVersion, candidate.Impact, jsonBytes(judgments), jsonBytes(refs), candidate.Blocker, candidate.WaitCondition, candidate.NextStep, a.now().UTC()); err != nil {
		return GoalAuthority{}, err
	}
	for _, ref := range refs {
		source := sources[ref]
		status := "confirmed"
		if !source.Valid {
			status = "withdrawn"
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.goal_evidence_links(id,fluctlight_id,goal_id,source_event_id,status,reason) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(goal_id,source_event_id) DO UPDATE SET status=EXCLUDED.status,reason=EXCLUDED.reason,updated_at=now()`, "goal_evidence_"+stableDigest(current.EntityID+"\x1f"+ref), current.FluctlightID, current.EntityID, source.EventID, status, "versioned semantic assessment"); err != nil {
			return GoalAuthority{}, err
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE public.goal_evidence_links SET status='rejected',reason='candidate association not supported by this versioned assessment',updated_at=now() WHERE fluctlight_id=$1 AND goal_id=$2 AND status='candidate' AND source_event_id<>ALL($3::bigint[])`, current.FluctlightID, current.EntityID, goalSourceEventIDsForRefs(refs, sources)); err != nil {
		return GoalAuthority{}, err
	}
	if next.Status == GoalCompleted {
		disposition := "ended"
		if candidate.ResidualMotivation != "" {
			disposition = "residual_motivation_review"
		}
		followupID := ""
		if candidate.Followup != nil {
			disposition = "planner_review_requested"
		}
		if err := requestGoalPlanningTx(ctx, tx, current.FluctlightID, "resolution:"+evaluationID, "goal_resolved", map[string]any{"goal_id": current.EntityID, "residual_motivation": candidate.ResidualMotivation, "followup_hint": candidate.Followup}); err != nil {
			return GoalAuthority{}, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.goal_resolutions(goal_id,fluctlight_id,status,criteria_version,evidence_refs,residual_motivation,disposition,followup_goal_id) VALUES($1,$2,'completed',$3,$4,$5,$6,$7) ON CONFLICT(goal_id) DO NOTHING`, current.EntityID, current.FluctlightID, effectiveGoalCriteriaVersion(current), jsonBytes(refs), candidate.ResidualMotivation, disposition, nullableString(followupID)); err != nil {
			return GoalAuthority{}, err
		}
	}
	if err := appendOutboxTx(ctx, tx, "goal.evaluated", "goal", current.EntityID, current.FluctlightID, requestID, evaluationID, evaluationID, map[string]any{"goal_id": current.EntityID, "evaluation_id": evaluationID, "status": next.Status, "impact": candidate.Impact, "revision": next.Revision, "criteria_version": effectiveGoalCriteriaVersion(next)}); err != nil {
		return GoalAuthority{}, err
	}
	return next, nil
}

func applyGoalObjectEvaluationTx(ctx context.Context, tx pgx.Tx, parent GoalAuthority, candidate GoalObjectEvaluation, kind string, sources map[string]GoalSource, key string) error {
	stages, commitments, err := readGoalObjectsWith(ctx, tx, parent.FluctlightID, parent.EntityID)
	if err != nil {
		return err
	}
	revision, version := 0, 0
	state := ""
	if kind == "stage" {
		for _, stage := range stages {
			if stage.ID == candidate.ID {
				revision, version, state = stage.Revision, stage.CriteriaVersion, stage.Status
			}
		}
	} else {
		for _, c := range commitments {
			if c.ID == candidate.ID {
				revision, version, state = c.Revision, c.CriteriaVersion, c.Status
			}
		}
	}
	judgments, err := validateGoalObjectEvaluation(parent, candidate, kind, stages, commitments, sources)
	if err != nil {
		return err
	}
	if kind == "commitment" && candidate.Completed {
		for _, c := range commitments {
			if c.ID == candidate.ID {
				for _, j := range candidate.Judgments {
					if j.Verdict == "satisfied" {
						for _, ref := range j.EvidenceRefs {
							source, ok := sources[ref]
							if !ok || (c.WindowStart != nil && source.OccurredAt.Before(*c.WindowStart)) || (c.WindowEnd != nil && source.OccurredAt.After(*c.WindowEnd)) {
								return errors.New("goal_commitment_evidence_outside_window")
							}
						}
					}
				}
			}
		}
	}
	if candidate.Completed && parent.Status == GoalActive && (state == "active" || state == "blocked" || (kind == "commitment" && state == "expired")) {
		table := "public.goal_stages"
		if kind == "commitment" {
			table = "public.goal_commitments"
		}
		command, err := tx.Exec(ctx, `UPDATE `+table+` SET status='completed',revision=revision+1,updated_at=now() WHERE id=$1 AND fluctlight_id=$2 AND goal_id=$3 AND revision=$4`, candidate.ID, parent.FluctlightID, parent.EntityID, revision)
		if err != nil {
			return err
		}
		if command.RowsAffected() != 1 {
			return ErrConflict
		}
		if kind == "stage" {
			if _, err := tx.Exec(ctx, `UPDATE public.goal_commitments SET status='cancelled',blocker='stage resolved; remaining plan superseded',revision=revision+1,updated_at=now() WHERE fluctlight_id=$1 AND goal_id=$2 AND stage_id=$3 AND status IN ('active','blocked','paused','expired')`, parent.FluctlightID, parent.EntityID, candidate.ID); err != nil {
				return err
			}
		}
	}
	id := "goal_object_evaluation_" + stableDigest(key+"\x1f"+kind+"\x1f"+candidate.ID)
	impact := "no_change"
	if candidate.Completed {
		impact = "completed"
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.goal_evaluations(id,fluctlight_id,goal_id,stage_id,commitment_id,goal_revision,criteria_version,strategy_version,impact,judgments,evidence_refs,blocker,wait_condition,next_step,occurred_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'','','',$12) ON CONFLICT(id) DO NOTHING`, id, parent.FluctlightID, parent.EntityID, nullableString(map[string]string{"stage": candidate.ID}[kind]), nullableString(map[string]string{"commitment": candidate.ID}[kind]), revision, version, goalEvaluationPolicyVersion, impact, jsonBytes(judgments), jsonBytes(goalEvidenceRefs(judgments)), time.Now().UTC())
	return err
}

func applyGoalPlanTx(ctx context.Context, tx pgx.Tx, parent GoalAuthority, plan GoalPlanCandidate, key string, at time.Time) (GoalAuthority, error) {
	if parent.Status != GoalActive {
		return parent, nil
	}
	if plan.ExpectedRevision != parent.Revision || plan.CriteriaVersion != effectiveGoalCriteriaVersion(parent) {
		return GoalAuthority{}, errors.New("goal_plan_version_conflict")
	}
	if strings.TrimSpace(plan.Reason) == "" || (plan.NextStep == "" && plan.WaitCondition == "" && plan.NextReviewAt == nil) {
		return GoalAuthority{}, errors.New("goal_plan_next_step_required")
	}
	next := parent
	if plan.Stage != nil {
		stage := plan.Stage
		criteria := newGoalObjectCriteria("stage_"+stableDigest(key+parent.EntityID), 1, stage.Criteria)
		if stage.Operation != "skip" {
			if err := validateGoalCriteria(criteria); err != nil {
				return GoalAuthority{}, err
			}
		}
		switch stage.Operation {
		case "create":
			if parent.CurrentStageID != "" {
				return GoalAuthority{}, errors.New("goal_stage_already_current")
			}
			id := "stage_" + stableDigest(key+"\x1f"+parent.EntityID)
			if err := validateGoalStageDependenciesTx(ctx, tx, parent, id, stage.DependencyIDs); err != nil {
				return GoalAuthority{}, err
			}
			if stage.Purpose == "" || stage.Strategy == "" {
				return GoalAuthority{}, errors.New("goal_stage_plan_invalid")
			}
			if _, err := tx.Exec(ctx, `INSERT INTO public.goal_stages(id,fluctlight_id,goal_id,profile_id,purpose,strategy,entry_basis,exit_basis,criteria,criteria_version,status,revision,reason,dependency_ids) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,1,'active',1,$10,$11)`, id, parent.FluctlightID, parent.EntityID, nullableString(parent.ProfileID), stage.Purpose, stage.Strategy, stage.EntryBasis, stage.ExitBasis, jsonBytes(criteria), stage.Reason, jsonBytes(nonNilStringSlice(stage.DependencyIDs))); err != nil {
				return GoalAuthority{}, err
			}
			next.CurrentStageID = id
		case "skip":
			if stage.ID == "" || stage.Reason == "" {
				return GoalAuthority{}, errors.New("goal_stage_skip_reason_required")
			}
			command, err := tx.Exec(ctx, `UPDATE public.goal_stages SET status='skipped',reason=$4,revision=revision+1,updated_at=now() WHERE id=$1 AND fluctlight_id=$2 AND goal_id=$3 AND revision=$5 AND status='active'`, stage.ID, parent.FluctlightID, parent.EntityID, stage.Reason, stage.ExpectedRevision)
			if err != nil {
				return GoalAuthority{}, err
			}
			if command.RowsAffected() != 1 {
				return GoalAuthority{}, ErrConflict
			}
			if _, err := tx.Exec(ctx, `UPDATE public.goal_commitments SET status='cancelled',blocker='stage skipped; plan superseded',revision=revision+1,updated_at=now() WHERE fluctlight_id=$1 AND goal_id=$2 AND stage_id=$3 AND status IN ('active','blocked','paused','expired')`, parent.FluctlightID, parent.EntityID, stage.ID); err != nil {
				return GoalAuthority{}, err
			}
			next.CurrentStageID = ""
		case "adjust":
			stages, _, err := readGoalObjectsWith(ctx, tx, parent.FluctlightID, parent.EntityID)
			if err != nil {
				return GoalAuthority{}, err
			}
			var current *GoalStage
			for i := range stages {
				if stages[i].ID == stage.ID {
					current = &stages[i]
				}
			}
			if current == nil || current.Revision != stage.ExpectedRevision || current.Status != "active" {
				return GoalAuthority{}, errors.New("goal_stage_revision_conflict")
			}
			if stage.DependencyIDs == nil {
				stage.DependencyIDs = current.DependencyIDs
			}
			if err := validateGoalStageDependenciesTx(ctx, tx, parent, stage.ID, stage.DependencyIDs); err != nil {
				return GoalAuthority{}, err
			}
			before := GoalAuthority{EntityID: current.ID, CriteriaVersion: current.CriteriaVersion}
			for _, c := range current.Criteria {
				before.SuccessCriteria = append(before.SuccessCriteria, c.Text)
				before.CriterionIDs = append(before.CriterionIDs, c.ID)
			}
			after := before
			after.SuccessCriteria = stage.Criteria
			if !slices.Equal(before.SuccessCriteria, after.SuccessCriteria) || current.Purpose != stage.Purpose {
				after.CriteriaVersion++
			}
			ids := before.CriterionIDs
			if after.CriteriaVersion != before.CriteriaVersion {
				ids = reviseGoalCriterionIDs(before, after)
			}
			criteria = nil
			for i, text := range stage.Criteria {
				criteria = append(criteria, GoalCriterion{ID: ids[i], Text: text})
			}
			if _, err := tx.Exec(ctx, `UPDATE public.goal_stages SET purpose=$2,strategy=$3,entry_basis=$4,exit_basis=$5,criteria=$6,criteria_version=$8,dependency_ids=$9,revision=revision+1,reason=$7,updated_at=now() WHERE id=$1`, stage.ID, stage.Purpose, stage.Strategy, stage.EntryBasis, stage.ExitBasis, jsonBytes(criteria), stage.Reason, after.CriteriaVersion, jsonBytes(nonNilStringSlice(stage.DependencyIDs))); err != nil {
				return GoalAuthority{}, err
			}
		default:
			return GoalAuthority{}, errors.New("goal_stage_plan_operation_invalid")
		}
	}
	if plan.Commitment != nil {
		if err := applyGoalCommitmentPlanTx(ctx, tx, next, *plan.Commitment, key, at); err != nil {
			return GoalAuthority{}, err
		}
	}
	next.ExecutionHint = map[string]any{"state": "planned", "next_step": plan.NextStep, "wait_condition": plan.WaitCondition, "reason": plan.Reason}
	if plan.NextReviewAt != nil {
		next.ExecutionHint["next_review_at"] = formatInstant(*plan.NextReviewAt)
	}
	next.Revision++
	record := GoalGovernanceRecord{GoalRef: parent.Ref, Operation: GoalUpdate, FromStatus: parent.Status, ToStatus: next.Status, BaseRevision: parent.Revision, Revision: next.Revision, EvidenceRefs: parent.EvidenceRefs, Reason: "Goal plan: " + plan.Reason, PolicyVersion: goalEvaluationPolicyVersion, OccurredAt: at}
	_, err := persistGoalAuthorityTx(ctx, tx, &parent, next, record, "goal-plan:"+key+":"+parent.EntityID)
	return next, err
}

func (a *App) confirmGoalRelationshipTx(ctx context.Context, tx pgx.Tx, goal GoalAuthority, candidate *GoalRelationshipConfirmation, key string) error {
	if candidate.relationshipID == "" {
		return errors.New("goal_relationship_snapshot_required")
	}
	label := strings.TrimSpace(candidate.Label)
	if label == "" {
		return errors.New("goal_relationship_label_required")
	}
	_, err := a.editRelationshipTx(ctx, tx, goal.FluctlightID, goal.FluctlightID, candidate.TargetActorID, goal.ProfileID, candidate.relationshipRevision, map[string]any{"expected_relationship_id": candidate.relationshipID, "role": map[string]any{"label": label, "category": "mutually_confirmed"}, "evidence_refs": candidate.EvidenceRefs, "reason": "Goal evaluation " + key}, "goal_evaluation")
	return err
}

func goalSourceEventIDsForRefs(refs []string, sources map[string]GoalSource) []int64 {
	ids := []int64{}
	for _, ref := range refs {
		if source, ok := sources[ref]; ok {
			ids = append(ids, source.EventID)
		}
	}
	return ids
}

func nonNilStringSlice(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
