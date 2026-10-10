package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type goalEvaluationGoal struct {
	RelationshipID       string                  `json:"relationship_id"`
	RelationshipRevision int                     `json:"relationship_revision"`
	GoalID               string                  `json:"goal_id"`
	TargetActorID        string                  `json:"target_actor_id"`
	Goal                 GoalAuthority           `json:"goal"`
	Stages               []GoalStage             `json:"stages"`
	Commitments          []GoalCommitment        `json:"commitments"`
	CurrentJudgments     []GoalCriterionJudgment `json:"current_judgments"`
}

type goalEvaluationSnapshot struct {
	Protocol             string               `json:"protocol,omitempty"`
	SkippedGoalIDs       []string             `json:"skipped_goal_ids,omitempty"`
	SkippedGoals         []goalEvaluationGoal `json:"skipped_goals,omitempty"`
	ProviderSourceIDs    []int64              `json:"provider_source_ids,omitempty"`
	UnprocessedSourceIDs []int64              `json:"unprocessed_source_ids,omitempty"`
	Reviews              []GoalReviewContext  `json:"reviews"`
	ClaimRevision        int                  `json:"claim_revision"`
	DeferredGoalIDs      []string             `json:"deferred_goal_ids"`
	ForcedGoalIDs        []string             `json:"forced_goal_ids,omitempty"`
	RequestID            string               `json:"request_id"`
	FluctlightID         string               `json:"fluctlight_id"`
	OwnerActorID         string               `json:"owner_actor_id"`
	ProfileID            string               `json:"profile_id"`
	Reason               string               `json:"reason"`
	Goals                []goalEvaluationGoal `json:"goals"`
	Sources              []GoalSource         `json:"sources"`
	SourceIDs            []int64              `json:"source_ids"`
}

func (a *App) claimGoalEvaluation(ctx context.Context, id string) (goalEvaluationSnapshot, map[string]any, error) {
	snapshot := goalEvaluationSnapshot{RequestID: id, Goals: []goalEvaluationGoal{}, Sources: []GoalSource{}}
	var prior map[string]any
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `SELECT fluctlight_id FROM public.goal_evaluation_requests WHERE id=$1`, id).Scan(&snapshot.FluctlightID); err != nil {
			return err
		}
		if err := lockLifeContextTx(ctx, tx, snapshot.FluctlightID); err != nil {
			return err
		}
		var status string
		var attempts, claimRevision int
		var available time.Time
		var goalRaw, resultRaw, pendingSnapshotRaw []byte
		var claimed *time.Time
		var errorCode *string
		if err := tx.QueryRow(ctx, `SELECT fluctlight_id,profile_id,reason,status,attempt_count,goal_ids,result,snapshot,claimed_at,claim_revision,available_at,error_code FROM public.goal_evaluation_requests WHERE id=$1 FOR UPDATE`, id).Scan(&snapshot.FluctlightID, &snapshot.ProfileID, &snapshot.Reason, &status, &attempts, &goalRaw, &resultRaw, &pendingSnapshotRaw, &claimed, &claimRevision, &available, &errorCode); err != nil {
			return err
		}
		if pendingSnapshot := decodeObject(pendingSnapshotRaw); pendingSnapshot != nil {
			snapshot.ForcedGoalIDs = decisionServiceRefValues(arrayValue(pendingSnapshot["forced_goal_ids"]))
		}
		if status == "succeeded" {
			prior = decodeObject(resultRaw)
			return nil
		}
		if status == "failed" {
			reason := "evaluation_failed"
			if errorCode != nil && *errorCode != "" {
				reason = *errorCode
			}
			prior = map[string]any{"status": "failed", "reason": reason}
			return nil
		}
		if status == "processing" && claimed != nil && time.Since(*claimed) < 5*time.Minute {
			prior = map[string]any{"status": "deferred"}
			return nil
		}
		if attempts >= 5 {
			if _, err := tx.Exec(ctx, `UPDATE public.goal_evaluation_requests SET status='failed',claimed_at=NULL,error_code=COALESCE(error_code,'evaluation_retry_exhausted'),updated_at=now() WHERE id=$1`, id); err != nil {
				return err
			}
			prior = map[string]any{"status": "failed", "reason": "evaluation_retry_exhausted"}
			return nil
		}
		if status == "retry" && time.Now().Before(available) {
			prior = map[string]any{"status": "deferred", "not_before": available.UTC().Format(time.RFC3339Nano)}
			return nil
		}
		var otherID string
		var otherClaimed time.Time
		err := tx.QueryRow(ctx, `SELECT id,claimed_at FROM public.goal_evaluation_requests WHERE fluctlight_id=$1 AND profile_id=$2 AND status='processing' AND id<>$3 FOR UPDATE`, snapshot.FluctlightID, snapshot.ProfileID, id).Scan(&otherID, &otherClaimed)
		if err == nil {
			if time.Since(otherClaimed) < 5*time.Minute {
				prior = map[string]any{"status": "deferred"}
				return nil
			}
			if _, err := tx.Exec(ctx, `UPDATE public.goal_evaluation_requests SET status='retry',claim_revision=claim_revision+1,claimed_at=NULL,error_code='assessment_lease_expired',available_at=now() WHERE id=$1`, otherID); err != nil {
				return err
			}
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		snapshot.ClaimRevision = claimRevision + 1
		var frozen goalEvaluationSnapshot
		if json.Unmarshal(pendingSnapshotRaw, &frozen) == nil && frozen.Protocol == goalEvaluationToolProtocol {
			frozen.ClaimRevision = snapshot.ClaimRevision
			snapshot = frozen
			_, err := tx.Exec(ctx, `UPDATE public.goal_evaluation_requests SET status='processing',attempt_count=attempt_count+1,claim_revision=$2,claimed_at=now(),snapshot=$3,error_code=NULL,updated_at=now() WHERE id=$1`, id, snapshot.ClaimRevision, jsonBytes(snapshot))
			return err
		}
		if err := lockLifeContextTx(ctx, tx, snapshot.FluctlightID); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `SELECT created_by_actor_id FROM public.fluctlights WHERE id=$1`, snapshot.FluctlightID).Scan(&snapshot.OwnerActorID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE public.goal_reviews r SET status='superseded',explanation='Goal ended before review; history retained',updated_at=now() FROM public.fluctlight_goals g WHERE r.evaluation_request_id=$1 AND r.goal_id=g.id AND r.status='pending' AND g.status NOT IN ('active','paused')`, id); err != nil {
			return err
		}
		goalIDs := decisionServiceRefValues(decodeArray(goalRaw))
		rows, err := tx.Query(ctx, `SELECT id,revision FROM public.fluctlight_goals WHERE fluctlight_id=$1 AND (profile_id IS NULL OR profile_id=$2) AND status IN ('active','paused') AND (jsonb_array_length($3::jsonb)=0 OR $3::jsonb ? id) ORDER BY updated_at DESC,id DESC`, snapshot.FluctlightID, snapshot.ProfileID, jsonBytes(goalIDs))
		if err != nil {
			return err
		}
		type target struct {
			id       string
			revision int
		}
		targets := []target{}
		for rows.Next() {
			var t target
			if err := rows.Scan(&t.id, &t.revision); err != nil {
				rows.Close()
				return err
			}
			targets = append(targets, t)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		if len(targets) > 6 {
			for _, t := range targets[6:] {
				snapshot.DeferredGoalIDs = append(snapshot.DeferredGoalIDs, t.id)
			}
			targets = targets[:6]
		}
		expiredCommitments := map[string][]GoalCommitment{}
		selectedIDs := []string{}
		priorSourceIDs := []int64{}
		for _, target := range targets {
			goal, err := loadGoalAuthorityTx(ctx, tx, snapshot.FluctlightID, "goal:ctx_"+stableDigest(target.id), ContextReference{EntityID: target.id, Revision: target.revision})
			if err != nil {
				return err
			}
			if goal.TargetActorID == "" && goal.Scope == "relationship" {
				goal.TargetActorID = snapshot.OwnerActorID
			}
			if _, err := tx.Exec(ctx, `UPDATE public.goal_commitments SET status='expired',held_by_goal=false,revision=revision+1,updated_at=now() WHERE fluctlight_id=$1 AND goal_id=$2 AND window_end<=$3 AND status IN ('active','blocked')`, snapshot.FluctlightID, target.id, a.now()); err != nil {
				return err
			}
			stages, commitments, err := readGoalObjectsWith(ctx, tx, snapshot.FluctlightID, target.id)
			if err != nil {
				return err
			}
			focusedStages := []GoalStage{}
			focusedCommitments := []GoalCommitment{}
			for _, stage := range stages {
				if stage.ID == goal.CurrentStageID || stage.Status == "active" {
					focusedStages = append(focusedStages, stage)
				}
			}
			for _, c := range commitments {
				if c.Status == "active" || c.Status == "blocked" {
					focusedCommitments = append(focusedCommitments, c)
				} else if c.Status == "expired" {
					expiredCommitments[goal.EntityID] = append(expiredCommitments[goal.EntityID], c)
				}
			}
			if len(focusedCommitments) > 8 {
				return errors.New("goal_commitment_context_limit")
			}
			entry := goalEvaluationGoal{GoalID: goal.EntityID, TargetActorID: goal.TargetActorID, Goal: goal, Stages: focusedStages, Commitments: focusedCommitments, CurrentJudgments: []GoalCriterionJudgment{}}
			if goal.TargetActorID != "" {
				err := tx.QueryRow(ctx, `SELECT id,revision FROM public.relationships WHERE owner_fluctlight_id=$1 AND target_actor_id=$2 AND (profile_id=$3 OR profile_id IS NULL) ORDER BY CASE WHEN profile_id=$3 THEN 0 ELSE 1 END LIMIT 1`, goal.FluctlightID, goal.TargetActorID, goal.ProfileID).Scan(&entry.RelationshipID, &entry.RelationshipRevision)
				if err != nil && !errors.Is(err, pgx.ErrNoRows) {
					return err
				}
			}
			var raw []byte
			err = tx.QueryRow(ctx, `SELECT judgments FROM public.goal_evaluations WHERE goal_id=$1 AND criteria_version=$2 AND stage_id IS NULL AND commitment_id IS NULL ORDER BY created_at DESC,id DESC LIMIT 1`, goal.EntityID, effectiveGoalCriteriaVersion(goal)).Scan(&raw)
			if err == nil {
				if err := json.Unmarshal(raw, &entry.CurrentJudgments); err != nil {
					return err
				}
			} else if !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			for _, j := range entry.CurrentJudgments {
				for _, ref := range j.EvidenceRefs {
					var eventID int64
					if _, err := fmt.Sscanf(ref, "source:%d", &eventID); err == nil {
						priorSourceIDs = append(priorSourceIDs, eventID)
					}
				}
			}
			selectedIDs = append(selectedIDs, target.id)
			snapshot.Goals = append(snapshot.Goals, entry)
		}
		rows, err = tx.Query(ctx, `SELECT id,goal_id,local_date::text,timezone,window_start,window_end,source_watermark,revision FROM public.goal_reviews WHERE evaluation_request_id=$1 AND status='pending' AND goal_id=ANY($2::text[]) ORDER BY goal_id,id`, id, selectedIDs)
		if err != nil {
			return err
		}
		for rows.Next() {
			var review GoalReviewContext
			if err := rows.Scan(&review.ID, &review.GoalID, &review.LocalDate, &review.Timezone, &review.WindowStart, &review.WindowEnd, &review.SourceWatermark, &review.Revision); err != nil {
				rows.Close()
				return err
			}
			for _, entry := range snapshot.Goals {
				g := entry.Goal
				if g.EntityID == review.GoalID && g.DeadlinePolicy == "soft" && g.Deadline != nil && !a.now().Before(*g.Deadline) {
					review.DeadlineOverdue = true
				}
			}
			snapshot.Reviews = append(snapshot.Reviews, review)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		rows, err = tx.Query(ctx, `SELECT e.id FROM public.goal_source_events e WHERE e.fluctlight_id=$1 AND ($2='' OR e.profile_id IS NULL OR e.profile_id=$2) AND e.processed_at IS NULL ORDER BY CASE WHEN EXISTS(SELECT 1 FROM public.goal_evidence_links l WHERE l.source_event_id=e.id AND l.goal_id=ANY($3::text[])) THEN 0 ELSE 1 END,CASE WHEN e.source_kind='message' THEN 0 ELSE 1 END,e.recorded_at DESC,e.id DESC LIMIT 32`, snapshot.FluctlightID, snapshot.ProfileID, selectedIDs)
		if err != nil {
			return err
		}
		seenSources := map[int64]bool{}
		for rows.Next() {
			var eventID int64
			if err := rows.Scan(&eventID); err != nil {
				rows.Close()
				return err
			}
			snapshot.SourceIDs = append(snapshot.SourceIDs, eventID)
			snapshot.UnprocessedSourceIDs = append(snapshot.UnprocessedSourceIDs, eventID)
			seenSources[eventID] = true
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		for _, eventID := range priorSourceIDs {
			if !seenSources[eventID] {
				snapshot.SourceIDs = append(snapshot.SourceIDs, eventID)
				seenSources[eventID] = true
			}
		}
		rows, err = tx.Query(ctx, `SELECT DISTINCT source_event_id FROM public.goal_evidence_links WHERE goal_id=ANY($1::text[]) AND status IN ('candidate','confirmed','withdrawn') ORDER BY source_event_id DESC LIMIT 32`, selectedIDs)
		if err != nil {
			return err
		}
		for rows.Next() {
			var eventID int64
			if err := rows.Scan(&eventID); err != nil {
				rows.Close()
				return err
			}
			if !seenSources[eventID] {
				snapshot.SourceIDs = append(snapshot.SourceIDs, eventID)
				seenSources[eventID] = true
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		if len(snapshot.SourceIDs) > 128 {
			return errors.New("goal_evidence_context_overflow")
		}

		for _, eventID := range snapshot.SourceIDs {
			source, err := readGoalSourceWith(ctx, tx, eventID, false)
			if err != nil {
				return err
			}
			snapshot.Sources = append(snapshot.Sources, source)
		}
		// Timely proof may arrive after a commitment window closes. Freeze only
		// expired commitments with potentially applicable proof, within the same bound.
		for i := range snapshot.Goals {
			entry := &snapshot.Goals[i]
			expired := expiredCommitments[entry.GoalID]
			for j := len(expired) - 1; j >= 0 && len(entry.Commitments) < 8; j-- {
				commitment := expired[j]
				for _, source := range snapshot.Sources {
					if source.Valid && source.CanSupportSuccess && validateGoalSourceScope(entry.Goal, source.Ref, map[string]GoalSource{source.Ref: source}) == nil &&
						(commitment.WindowStart == nil || !source.OccurredAt.Before(*commitment.WindowStart)) &&
						(commitment.WindowEnd == nil || !source.OccurredAt.After(*commitment.WindowEnd)) {
						entry.Commitments = append(entry.Commitments, commitment)
						break
					}
				}
			}
		}
		_, err = tx.Exec(ctx, `UPDATE public.goal_evaluation_requests SET status='processing',attempt_count=attempt_count+1,claim_revision=$4,claimed_at=now(),source_ids=$2,snapshot=$3,error_code=NULL,updated_at=now() WHERE id=$1`, id, jsonBytes(snapshot.SourceIDs), jsonBytes(snapshot), snapshot.ClaimRevision)
		return err
	})
	return snapshot, prior, err
}

func (a *App) ProcessGoalEvaluationIntent(ctx context.Context, id string) (map[string]any, error) {
	var logicalOwner string
	if err := a.DB.Pool().QueryRow(ctx, `SELECT fluctlight_id FROM public.goal_evaluation_requests WHERE id=$1`, id).Scan(&logicalOwner); err != nil {
		return nil, err
	}
	ctx, releaseLogical, logicalErr := a.enterLogicalRun(ctx, logicalOwner, "goal_evaluation")
	if logicalErr != nil {
		return nil, logicalErr
	}
	defer releaseLogical()

	if err := a.reconcileGoalEvaluationSources(ctx, id); err != nil {
		return nil, err
	}
	snapshot, prior, err := a.claimGoalEvaluation(ctx, id)
	if err != nil {
		return nil, err
	}
	if prior != nil {
		return prior, nil
	}
	if len(snapshot.Goals) == 0 {
		return a.settleEmptyGoalEvaluation(ctx, snapshot)
	}
	snapshot, err = admitGoalEvaluationSourceInput(snapshot)
	if err != nil {
		return nil, a.failGoalEvaluation(ctx, id, snapshot.ClaimRevision, err)
	}
	if goalAssessmentHasUnofferedNewSource(snapshot) {
		return nil, a.failGoalEvaluation(ctx, id, snapshot.ClaimRevision, errors.New("goal_evaluation_new_source_budget_blocked"))
	}
	projection, err := a.BuildContextProjectionFor(ctx, ContextProjectionRequest{AuthorizationActorID: snapshot.OwnerActorID, TargetActorID: snapshot.OwnerActorID, FluctlightID: snapshot.FluctlightID, WorkingProfileID: snapshot.ProfileID, SourceFactID: id, TriggerSource: "goal_evaluation", MemoryOperation: MemoryForReflection, MemoryConversationMode: MemoryConversationGlobalOnly})
	if err != nil {
		return nil, a.failGoalEvaluation(ctx, id, snapshot.ClaimRevision, err)
	}
	goalIDs := make([]string, 0, len(snapshot.Goals))
	for _, entry := range snapshot.Goals {
		goalIDs = append(goalIDs, entry.GoalID)
	}
	priorMemos, err := readGoalAssessmentMemos(ctx, a.DB.Pool(), snapshot.FluctlightID, snapshot.ProfileID, goalIDs)
	if err != nil {
		return nil, a.failGoalEvaluation(ctx, id, snapshot.ClaimRevision, err)
	}
	claimedSnapshot := snapshot
	skippedGoals := snapshot.SkippedGoalIDs
	if snapshot.Protocol != goalEvaluationToolProtocol {
		snapshot, skippedGoals = goalAssessmentEligible(snapshot, projection, priorMemos, a.now())
	}
	if len(snapshot.Goals) == 0 {
		result, settleErr := settleSkippedGoalAssessment(ctx, a, claimedSnapshot, projection, skippedGoals)
		if settleErr != nil {
			return nil, a.failGoalEvaluation(ctx, id, claimedSnapshot.ClaimRevision, settleErr)
		}
		return result, nil
	}
	if snapshot.Protocol != goalEvaluationToolProtocol && a.kevService().Enabled(ctx, "goal.completion_check") {
		selected := make([]goalEvaluationGoal, 0, len(snapshot.Goals))
		var notBefore time.Time
		for _, entry := range snapshot.Goals {
			forced := false
			for _, goalID := range snapshot.ForcedGoalIDs {
				if goalID == entry.GoalID {
					forced = true
				}
			}
			for _, review := range snapshot.Reviews {
				if review.GoalID == entry.GoalID {
					forced = true
				}
			}
			if forced {
				selected = append(selected, entry)
				continue
			}
			candidate := entry.GoalID + ":" + stableDigest(jsonString(map[string]any{"goal": entry.Goal, "sources": snapshot.SourceIDs}))
			allowed, until, gateErr := a.kevAutomaticGate(ctx, "goal.completion_check", snapshot.FluctlightID, candidate, map[string]any{"goal": entry.Goal, "new_sources": admittedGoalEvaluationSources(snapshot)})
			if gateErr != nil {
				return nil, gateErr
			}
			if allowed {
				selected = append(selected, entry)
			} else {
				snapshot.DeferredGoalIDs = append(snapshot.DeferredGoalIDs, entry.GoalID)
				if until.After(notBefore) {
					notBefore = until
				}
			}
		}
		if len(selected) == 0 {
			tag, err := a.DB.Pool().Exec(ctx, `UPDATE public.goal_evaluation_requests SET status='retry',claimed_at=NULL,attempt_count=GREATEST(attempt_count-1,0),available_at=$3,updated_at=now() WHERE id=$1 AND status='processing' AND claim_revision=$2`, id, snapshot.ClaimRevision, notBefore)
			if err != nil {
				return nil, err
			}
			if tag.RowsAffected() != 1 {
				return nil, ErrConflict
			}
			return map[string]any{"status": "deferred", "reason": "kev_deferred", "not_before": notBefore.Format(time.RFC3339Nano)}, nil
		}
		snapshot.Goals = selected
	}
	// Freeze the offered refs once; retries preserve already committed submissions.
	snapshot.Protocol = goalEvaluationToolProtocol
	snapshot.SkippedGoalIDs = skippedGoals
	if len(snapshot.SkippedGoals) == 0 {
		for _, entry := range claimedSnapshot.Goals {
			if containsString(skippedGoals, entry.GoalID) {
				snapshot.SkippedGoals = append(snapshot.SkippedGoals, entry)
			}
		}
	}
	// Record the exact offered source subset without dropping the full CAS snapshot.
	command, err := a.DB.Pool().Exec(ctx, `UPDATE public.goal_evaluation_requests SET snapshot=$3 WHERE id=$1 AND status='processing' AND claim_revision=$2`, id, snapshot.ClaimRevision, jsonBytes(snapshot))
	if err != nil {
		return nil, a.failGoalEvaluation(ctx, id, snapshot.ClaimRevision, err)
	}
	if command.RowsAffected() != 1 {
		return nil, ErrConflict
	}
	priorResult, err := a.goalEvaluationSubmissionResult(ctx, snapshot)
	if err != nil {
		return nil, a.failGoalEvaluation(ctx, id, snapshot.ClaimRevision, err)
	}
	_, missing := goalEvaluationSubmissionCoverage(snapshot, goalSubmissionRecords(priorResult))
	var task ProjectionTaskResult
	var runErr error
	if len(missing) > 0 {
		task, runErr = a.RunGoalEvaluationTask(ctx, snapshot, projection)
		if err := a.recordGoalEvaluationSubmissionErrors(ctx, snapshot, task.Trace); err != nil {
			return nil, a.failGoalEvaluation(ctx, id, snapshot.ClaimRevision, errors.Join(runErr, err))
		}
	}
	// Even a later model failure cannot erase accepted Tool results. Persist
	// coverage/outcomes before recording that failure; do not consume sources yet.
	if runErr != nil {
		result, readErr := a.goalEvaluationSubmissionResult(ctx, snapshot)
		return result, a.failGoalEvaluation(ctx, id, snapshot.ClaimRevision, errors.Join(runErr, readErr))
	}
	result, err := a.finalizeGoalEvaluationSubmissions(ctx, snapshot, claimedSnapshot, projection)
	if err != nil {
		if err.Error() == "goal_evaluation_native_submission_missing" && goalEvaluationNeedsFreshSnapshot(result) {
			return a.replaceStaleGoalEvaluation(ctx, snapshot, result)
		}
		return result, a.failGoalEvaluation(ctx, id, snapshot.ClaimRevision, err)
	}
	return result, nil
}

func decodeStructuredValue(value map[string]any, target any) error {
	return json.Unmarshal(jsonBytes(value), target)
}

func (a *App) failGoalEvaluation(ctx context.Context, id string, claimRevision int, cause error) error {
	recordCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	code := boundedLifecycleCause(cause.Error())
	terminal := terminalGoalEvaluationContractError(cause)
	if _, err := a.DB.Pool().Exec(recordCtx, `UPDATE public.goal_evaluation_requests SET status=CASE WHEN $4 OR attempt_count>=5 THEN 'failed' ELSE 'retry' END,claimed_at=NULL,error_code=$2,available_at=now()+interval '1 minute',updated_at=now() WHERE id=$1 AND status='processing' AND claim_revision=$3`, id, code, claimRevision, terminal); err != nil {
		return fmt.Errorf("record goal evaluation failure: %w (assessment: %v)", err, cause)
	}
	return cause
}

func terminalGoalEvaluationContractError(cause error) bool {
	if cause == nil {
		return false
	}
	code := cause.Error()
	if strings.HasPrefix(code, "goal_evaluation_wire_") || errors.Is(cause, errADKFinalContractInvalid) {
		return true
	}
	for _, fixed := range []string{
		"goal_assessment_coverage_missing",
		"goal_evaluation_scope_invalid",
		"goal_review_adjust_plan_required",
		"goal_assessment_next_step_missing",
		"goal_review_evaluation_required",
		"goal_stage_evaluation_scope_invalid",
		"goal_commitment_evaluation_scope_invalid",
		"goal_plan_scope_invalid",
		"goal_evaluation_wire_output_invalid",
		"goal_evaluation_wire_goal_ref_invalid",
		"goal_evaluation_wire_goal_duplicate",
		"goal_evaluation_wire_criterion_ref_invalid",
		"goal_evaluation_wire_commitment_evaluation_duplicate",
		"goal_evaluation_wire_source_ref_invalid",
		"goal_evaluation_wire_actor_ref_invalid",
		"goal_evaluation_wire_plan_ref_invalid",
	} {
		if code == fixed {
			return true
		}
	}
	return false
}

func (a *App) settleEmptyGoalEvaluation(ctx context.Context, snapshot goalEvaluationSnapshot) (map[string]any, error) {
	result := map[string]any{"status": "succeeded", "request_id": snapshot.RequestID, "reason": "no_active_scoped_goal"}
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if err := lockLifeContextTx(ctx, tx, snapshot.FluctlightID); err != nil {
			return err
		}
		var state string
		var revision int
		if err := tx.QueryRow(ctx, `SELECT status,claim_revision FROM public.goal_evaluation_requests WHERE id=$1 FOR UPDATE`, snapshot.RequestID).Scan(&state, &revision); err != nil {
			return err
		}
		if state != "processing" || revision != snapshot.ClaimRevision {
			return ErrConflict
		}
		if err := invalidateGoalSourceLinksTx(ctx, tx, snapshot.Sources); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE public.goal_source_events SET processed_at=now() WHERE id=ANY($1::bigint[]) AND fluctlight_id=$2`, snapshot.SourceIDs, snapshot.FluctlightID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `UPDATE public.goal_evaluation_requests SET status='succeeded',result=$2,claimed_at=NULL,updated_at=now() WHERE id=$1`, snapshot.RequestID, jsonBytes(result))
		return err
	})
	return result, err
}

func invalidateGoalSourceLinksTx(ctx context.Context, tx pgx.Tx, sources []GoalSource) error {
	for _, source := range sources {
		if !source.Valid {
			if _, err := tx.Exec(ctx, `UPDATE public.goal_evidence_links SET status='withdrawn',reason='authoritative source withdrawn or version changed',updated_at=now() WHERE source_event_id IN (SELECT id FROM public.goal_source_events WHERE fluctlight_id=$1 AND source_kind=$2 AND source_id=$3)`, source.FluctlightID, source.Kind, source.ID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO public.goal_evidence_review_flags(goal_id,source_event_id,reason) SELECT DISTINCT l.goal_id,$4::bigint,'ended Goal source requires review' FROM public.goal_evidence_links l JOIN public.fluctlight_goals g ON g.id=l.goal_id JOIN public.goal_source_events e ON e.id=l.source_event_id WHERE e.fluctlight_id=$1 AND e.source_kind=$2 AND e.source_id=$3 AND g.status IN ('completed','cancelled','abandoned') ON CONFLICT DO NOTHING`, source.FluctlightID, source.Kind, source.ID, source.EventID); err != nil {
				return err
			}
		}
	}
	return nil
}

func goalHasNextState(goal GoalAuthority) bool {
	hint := goal.ExecutionHint
	return stringValue(hint["next_step"]) != "" || stringValue(hint["wait_condition"]) != "" || stringValue(hint["blocker"]) != "" || stringValue(hint["next_review_at"]) != ""
}

// Source reconciliation commits independently of Provider success. Historical
// resolutions are flagged rather than rewritten, even when no Goal is active.
func (a *App) reconcileGoalEvaluationSources(ctx context.Context, id string) error {
	return withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var owner string
		if err := tx.QueryRow(ctx, `SELECT fluctlight_id FROM public.goal_evaluation_requests WHERE id=$1`, id).Scan(&owner); err != nil {
			return err
		}
		if err := lockLifeContextTx(ctx, tx, owner); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT DISTINCT e.id FROM public.goal_evidence_links l JOIN public.goal_source_events e ON e.id=l.source_event_id WHERE e.fluctlight_id=$1 AND l.status IN ('confirmed','candidate') ORDER BY e.id LIMIT 128`, owner)
		if err != nil {
			return err
		}
		ids := []int64{}
		for rows.Next() {
			var eventID int64
			if err := rows.Scan(&eventID); err != nil {
				rows.Close()
				return err
			}
			ids = append(ids, eventID)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		sources := []GoalSource{}
		for _, eventID := range ids {
			source, err := readGoalSourceWith(ctx, tx, eventID, true)
			if err != nil {
				return err
			}
			if !source.Valid {
				sources = append(sources, source)
			}
		}
		return invalidateGoalSourceLinksTx(ctx, tx, sources)
	})
}

// Object IDs and versions are limited to the authority frozen in this claim.
func validateGoalObjectEvaluationScope(entry goalEvaluationGoal, candidate GoalEvaluationCandidate) error {
	if candidate.StageEvaluation != nil {
		found := false
		for _, stage := range entry.Stages {
			if stage.ID == candidate.StageEvaluation.ID && stage.Revision == candidate.StageEvaluation.ExpectedRevision && stage.CriteriaVersion == candidate.StageEvaluation.CriteriaVersion {
				found = true
			}
		}
		if !found {
			return errors.New("goal_stage_evaluation_scope_invalid")
		}
	}
	seen := map[string]bool{}
	for _, evaluation := range candidate.CommitmentEvaluations {
		found := false
		for _, commitment := range entry.Commitments {
			if commitment.ID == evaluation.ID && commitment.Revision == evaluation.ExpectedRevision && commitment.CriteriaVersion == evaluation.CriteriaVersion {
				found = true
			}
		}
		if !found || seen[evaluation.ID] {
			return errors.New("goal_commitment_evaluation_scope_invalid")
		}
		seen[evaluation.ID] = true
	}
	return nil
}
