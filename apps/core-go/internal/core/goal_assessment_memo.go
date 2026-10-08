package core

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// goalAssessmentMemo is Core-only durable evidence that one successful model
// assessment covered a Goal's semantic authority and the offered source set.
// It deliberately contains no model output and can only suppress another model
// call after the normal claim and authority/source checks have succeeded.
type goalAssessmentMemo struct {
	EvaluationPolicyVersion string   `json:"evaluation_policy_version"`
	GoalID                  string   `json:"goal_id"`
	AuthoritySignature      string   `json:"authority_signature"`
	ConstraintSignature     string   `json:"constraint_signature"`
	SourceFingerprints      []string `json:"source_fingerprints"`
	SucceededAt             string   `json:"succeeded_at"`
}

func goalAssessmentMemoFor(entry goalEvaluationGoal, sources []GoalSource, projection ContextProjection, at time.Time) goalAssessmentMemo {
	return goalAssessmentMemo{
		EvaluationPolicyVersion: goalEvaluationPolicyVersion,
		GoalID:                  entry.GoalID,
		AuthoritySignature:      goalAssessmentAuthoritySignature(entry, at),
		ConstraintSignature:     goalAssessmentConstraintSignature(projection),
		SourceFingerprints:      goalAssessmentSourceFingerprints(entry.Goal, sources),
		SucceededAt:             formatInstant(at.UTC()),
	}
}

func goalAssessmentAuthoritySignature(entry goalEvaluationGoal, at time.Time) string {
	goal := entry.Goal
	value := map[string]any{
		"schema_version":   goal.SchemaVersion,
		"profile_id":       goal.ProfileID,
		"desired_outcome":  goal.DesiredOutcome,
		"success_criteria": goal.SuccessCriteria,
		"criterion_ids":    goal.CriterionIDs,
		"criteria_version": effectiveGoalCriteriaVersion(goal),
		"criteria_policy":  goal.CriteriaPolicy,
		"deadline_policy":  goal.DeadlinePolicy,
		"deadline":         goal.Deadline,
		"motivation":       goal.Motivation,
		"scope":            goal.Scope,
		"target_actor_id":  goal.TargetActorID,
		"status":           goal.Status,
		"review_policy":    goal.ReviewPolicy,
		"stages":           goalAssessmentStageAuthority(entry.Stages),
		"commitments":      goalAssessmentCommitmentAuthority(entry.Commitments),
		"temporal_phase":   goalAssessmentTemporalPhase(entry, at),
	}
	return stableDigest(string(jsonBytes(value)))
}

func goalAssessmentTemporalPhase(entry goalEvaluationGoal, at time.Time) map[string]any {
	result := map[string]any{"deadline": temporalBoundaryPhase(entry.Goal.Deadline, at)}
	commitments := make([]map[string]any, 0, len(entry.Commitments))
	for _, commitment := range entry.Commitments {
		commitments = append(commitments, map[string]any{
			"window_start": temporalBoundaryPhase(commitment.WindowStart, at),
			"window_end":   temporalBoundaryPhase(commitment.WindowEnd, at),
		})
	}
	result["commitments"] = commitments
	return result
}

func temporalBoundaryPhase(boundary *time.Time, at time.Time) string {
	if boundary == nil {
		return "none"
	}
	if at.IsZero() || at.Before(*boundary) {
		return "before"
	}
	return "reached"
}

func goalAssessmentStageAuthority(values []GoalStage) []map[string]any {
	result := make([]map[string]any, 0, len(values))
	for _, stage := range values {
		result = append(result, map[string]any{
			"purpose": stage.Purpose, "strategy": stage.Strategy,
			"entry_basis": stage.EntryBasis, "exit_basis": stage.ExitBasis,
			"criteria": goalAssessmentCriterionTexts(stage.Criteria), "criteria_version": stage.CriteriaVersion,
			"status": stage.Status, "reason": stage.Reason,
		})
	}
	return result
}

func goalAssessmentCommitmentAuthority(values []GoalCommitment) []map[string]any {
	result := make([]map[string]any, 0, len(values))
	for _, commitment := range values {
		result = append(result, map[string]any{
			"expected_result": commitment.ExpectedResult,
			"criteria":        goalAssessmentCriterionTexts(commitment.Criteria), "criteria_version": commitment.CriteriaVersion,
			"window_start": commitment.WindowStart, "window_end": commitment.WindowEnd,
			"opportunity_condition": commitment.OpportunityCondition,
			"blocker":               commitment.Blocker, "status": commitment.Status,
		})
	}
	return result
}

func goalAssessmentCriterionTexts(values []GoalCriterion) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		result = append(result, value.Text)
	}
	return result
}

// Only slow facts that can change a Goal decision belong here. Projection
// clocks, revisions, current mood/drive decay, prompt refs and retrieval traces
// are intentionally excluded so ordinary runtime passage does not invalidate a
// successful assessment.
func goalAssessmentConstraintSignature(projection ContextProjection) string {
	value := map[string]any{
		"reference_timezone": projection.ReferenceTimezone,
		"working_profile":    projection.ReferenceIndex.ActiveProfileID,
		"core_persona":       semanticGoalConstraintValue(projection.CorePersona),
		"behavioral_policy":  semanticGoalConstraintValue(projection.BehavioralPolicy),
		"life_context":       semanticGoalConstraintValue(projection.LifeContext),
		"schedule":           semanticGoalConstraintValue(projection.Schedule),
		"capabilities":       semanticGoalConstraintValue(projection.Capabilities),
		"relationships":      semanticGoalConstraintValue(projection.Relationships),
		"actor_facts":        semanticGoalConstraintValue(projection.ActorFacts),
		"actors":             semanticGoalConstraintValue(projection.Actors),
	}
	return stableDigest(string(jsonBytes(value)))
}

func semanticGoalConstraintValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		result := map[string]any{}
		for key, item := range typed {
			if goalConstraintBookkeepingKey(key) {
				continue
			}
			result[key] = semanticGoalConstraintValue(item)
		}
		return result
	case []map[string]any:
		result := make([]any, 0, len(typed))
		for _, item := range typed {
			result = append(result, semanticGoalConstraintValue(item))
		}
		return result
	case []any:
		result := make([]any, 0, len(typed))
		for _, item := range typed {
			result = append(result, semanticGoalConstraintValue(item))
		}
		return result
	default:
		return value
	}
}

func goalConstraintBookkeepingKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	if key == "id" || key == "ref" || key == "refs" || key == "revision" || key == "as_of" || key == "now" || key == "instant" || key == "current_time" || key == "current_local_time" || key == "created_at" || key == "updated_at" || key == "recorded_at" || key == "occurred_at" {
		return true
	}
	return strings.HasSuffix(key, "_id") || strings.HasSuffix(key, "_ids") || strings.HasSuffix(key, "_ref") || strings.HasSuffix(key, "_refs") || strings.HasSuffix(key, "_revision")
}

// A matching memo was computed from a projection read before settlement. The
// lifecycle lock prevents a later writer from crossing us, while these stamps
// reject a Foundation or effective-Life change that committed before the lock.
// Current State is intentionally absent: affect/drive decay is not a Goal
// assessment constraint and must not turn ordinary clock churn into model I/O.
func verifyGoalAssessmentProjectionAuthorityTx(ctx context.Context, tx pgx.Tx, app *App, projection ContextProjection) error {
	if projection.ContextRevision < 0 || strings.TrimSpace(projection.LifeContextRevision) == "" {
		return errors.New("goal_assessment_projection_authority_required")
	}
	actualFoundation, _, actualLife, err := readCognitionAuthorityRevisionsWith(ctx, tx, projection.FluctlightID, app.now().UTC())
	if err != nil {
		return err
	}
	if actualFoundation != projection.ContextRevision {
		return ErrFoundationRevisionStale
	}
	if actualLife != projection.LifeContextRevision {
		return ErrLifeContextStale
	}
	return nil
}

func goalAssessmentSourceFingerprints(goal GoalAuthority, sources []GoalSource) []string {
	result := []string{}
	for _, source := range sources {
		if validateGoalSourceScope(goal, source.Ref, map[string]GoalSource{source.Ref: source}) != nil {
			continue
		}
		result = append(result, goalAssessmentSourceFingerprint(source))
	}
	slices.Sort(result)
	return slices.Compact(result)
}

func goalAssessmentSourceFingerprint(source GoalSource) string {
	value := map[string]any{
		"kind": source.Kind, "source_id": source.ID, "source_version": source.Version,
		"profile_id": source.ProfileID, "conversation_id": source.ConversationID,
		"subject_actor_id": source.SubjectActorID, "valid": source.Valid,
		"can_support_success": source.CanSupportSuccess,
	}
	return stableDigest(string(jsonBytes(value)))
}

func goalAssessmentMemoMatches(memo goalAssessmentMemo, current goalAssessmentMemo) bool {
	if memo.EvaluationPolicyVersion != goalEvaluationPolicyVersion || current.EvaluationPolicyVersion != goalEvaluationPolicyVersion || memo.GoalID != current.GoalID || memo.AuthoritySignature != current.AuthoritySignature || memo.ConstraintSignature != current.ConstraintSignature {
		return false
	}
	prior := map[string]bool{}
	for _, fingerprint := range memo.SourceFingerprints {
		prior[fingerprint] = true
	}
	for _, fingerprint := range current.SourceFingerprints {
		if !prior[fingerprint] {
			return false
		}
	}
	return true
}

func readGoalAssessmentMemos(ctx context.Context, q lifeContextQuerier, owner, profile string, goalIDs []string) (map[string]goalAssessmentMemo, error) {
	result := map[string]goalAssessmentMemo{}
	if len(goalIDs) == 0 {
		return result, nil
	}
	rows, err := q.Query(ctx, `SELECT DISTINCT ON (memo->>'goal_id') memo FROM public.goal_evaluation_requests r CROSS JOIN LATERAL jsonb_array_elements(CASE WHEN jsonb_typeof(r.result->'assessment_memos')='array' THEN r.result->'assessment_memos' ELSE '[]'::jsonb END) memo WHERE r.fluctlight_id=$1 AND r.status='succeeded' AND memo->>'goal_id'=ANY($2::text[]) ORDER BY memo->>'goal_id',r.updated_at DESC,r.id DESC`, owner, goalIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var memo goalAssessmentMemo
		if err := json.Unmarshal(raw, &memo); err != nil {
			return nil, err
		}
		if memo.GoalID == "" || memo.AuthoritySignature == "" || memo.ConstraintSignature == "" {
			continue
		}
		result[memo.GoalID] = memo
	}
	return result, rows.Err()
}

func goalAssessmentEligible(snapshot goalEvaluationSnapshot, projection ContextProjection, prior map[string]goalAssessmentMemo, at time.Time) (goalEvaluationSnapshot, []string) {
	forced := map[string]bool{}
	for _, id := range snapshot.ForcedGoalIDs {
		forced[id] = true
	}
	reviewDue := map[string]bool{}
	for _, review := range snapshot.Reviews {
		reviewDue[review.GoalID] = true
	}
	admitted := admittedGoalEvaluationSources(snapshot)
	eligible := make([]goalEvaluationGoal, 0, len(snapshot.Goals))
	skipped := []string{}
	for _, entry := range snapshot.Goals {
		current := goalAssessmentMemoFor(entry, admitted, projection, at)
		memo, ok := prior[entry.GoalID]
		if forced[entry.GoalID] || reviewDue[entry.GoalID] || !ok || !goalAssessmentMemoMatches(memo, current) {
			eligible = append(eligible, entry)
			continue
		}
		skipped = append(skipped, entry.GoalID)
	}
	snapshot.Goals = eligible
	if len(snapshot.Reviews) > 0 {
		kept := snapshot.Reviews[:0]
		for _, review := range snapshot.Reviews {
			if slices.ContainsFunc(eligible, func(entry goalEvaluationGoal) bool { return entry.GoalID == review.GoalID }) {
				kept = append(kept, review)
			}
		}
		snapshot.Reviews = kept
	}
	return snapshot, skipped
}

func refreshGoalAssessmentEntryTx(ctx context.Context, tx pgx.Tx, goal GoalAuthority, sources []GoalSource) (goalEvaluationGoal, error) {
	entry := goalEvaluationGoal{GoalID: goal.EntityID, TargetActorID: goal.TargetActorID, Goal: goal, CurrentJudgments: []GoalCriterionJudgment{}}
	stages, commitments, err := readGoalObjectsWith(ctx, tx, goal.FluctlightID, goal.EntityID)
	if err != nil {
		return entry, err
	}
	for _, stage := range stages {
		if stage.ID == goal.CurrentStageID || stage.Status == "active" {
			entry.Stages = append(entry.Stages, stage)
		}
	}
	for _, commitment := range commitments {
		if commitment.Status == "active" || commitment.Status == "blocked" {
			entry.Commitments = append(entry.Commitments, commitment)
			continue
		}
		if commitment.Status != "expired" || len(entry.Commitments) >= 8 {
			continue
		}
		for _, source := range sources {
			if source.Valid && source.CanSupportSuccess && validateGoalSourceScope(goal, source.Ref, map[string]GoalSource{source.Ref: source}) == nil &&
				(commitment.WindowStart == nil || !source.OccurredAt.Before(*commitment.WindowStart)) &&
				(commitment.WindowEnd == nil || !source.OccurredAt.After(*commitment.WindowEnd)) {
				entry.Commitments = append(entry.Commitments, commitment)
				break
			}
		}
	}
	return entry, nil
}

func verifyGoalAssessmentSourcesTx(ctx context.Context, tx pgx.Tx, snapshot goalEvaluationSnapshot) error {
	expected := map[int64]string{}
	for _, source := range admittedGoalEvaluationSources(snapshot) {
		expected[source.EventID] = goalAssessmentSourceFingerprint(source)
	}
	for eventID, fingerprint := range expected {
		live, err := readGoalSourceWith(ctx, tx, eventID, true)
		if err != nil {
			return err
		}
		if goalAssessmentSourceFingerprint(live) != fingerprint {
			return errors.New("goal_evaluation_source_conflict")
		}
	}
	return nil
}

func goalAssessmentHasUnofferedNewSource(snapshot goalEvaluationSnapshot) bool {
	hasUnprocessed := false
	hasOfferedUnprocessed := false
	for _, id := range snapshot.UnprocessedSourceIDs {
		hasUnprocessed = true
		if slices.Contains(snapshot.ProviderSourceIDs, id) {
			hasOfferedUnprocessed = true
		}
	}
	return hasUnprocessed && !hasOfferedUnprocessed
}

func goalAssessmentMemoValues(values []goalAssessmentMemo) []any {
	result := make([]any, 0, len(values))
	for _, value := range values {
		result = append(result, value)
	}
	return result
}

func verifySkippedGoalAssessmentTx(ctx context.Context, tx pgx.Tx, snapshot goalEvaluationSnapshot, projection ContextProjection, skipped []string, at time.Time) error {
	if len(skipped) == 0 {
		return nil
	}
	memos, err := readGoalAssessmentMemos(ctx, tx, snapshot.FluctlightID, snapshot.ProfileID, skipped)
	if err != nil {
		return err
	}
	for _, frozen := range snapshot.Goals {
		if !containsString(skipped, frozen.GoalID) {
			continue
		}
		live, err := loadGoalAuthorityTx(ctx, tx, frozen.Goal.FluctlightID, frozen.Goal.Ref, ContextReference{EntityID: frozen.GoalID, Revision: frozen.Goal.Revision})
		if err != nil {
			return err
		}
		entry, err := refreshGoalAssessmentEntryTx(ctx, tx, live, admittedGoalEvaluationSources(snapshot))
		if err != nil {
			return err
		}
		if !goalAssessmentMemoMatches(memos[frozen.GoalID], goalAssessmentMemoFor(entry, admittedGoalEvaluationSources(snapshot), projection, at)) {
			return ErrConflict
		}
	}
	return nil
}

func settleSkippedGoalAssessment(ctx context.Context, app *App, snapshot goalEvaluationSnapshot, projection ContextProjection, skipped []string) (map[string]any, error) {
	result := map[string]any{"status": "succeeded", "request_id": snapshot.RequestID, "evaluated_goals": []string{}, "skipped_goals": skipped, "source_ids": snapshot.SourceIDs, "reason": "assessment_memo_match"}
	err := withTransaction(ctx, app.DB.Pool(), func(tx pgx.Tx) error {
		if err := lockLifeContextTx(ctx, tx, snapshot.FluctlightID); err != nil {
			return err
		}
		if err := verifyGoalAssessmentProjectionAuthorityTx(ctx, tx, app, projection); err != nil {
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
		if err := verifyGoalAssessmentSourcesTx(ctx, tx, snapshot); err != nil {
			return err
		}
		if err := verifySkippedGoalAssessmentTx(ctx, tx, snapshot, projection, skipped, app.now()); err != nil {
			return err
		}
		if len(snapshot.ProviderSourceIDs) > 0 && len(snapshot.DeferredGoalIDs) == 0 {
			if _, err := tx.Exec(ctx, `UPDATE public.goal_source_events SET processed_at=now() WHERE id=ANY($1::bigint[]) AND fluctlight_id=$2`, snapshot.ProviderSourceIDs, snapshot.FluctlightID); err != nil {
				return err
			}
		}
		if len(snapshot.DeferredGoalIDs) > 0 {
			remainderID, err := queueGoalEvaluationTx(ctx, tx, snapshot.FluctlightID, snapshot.ProfileID, "assessment_batch_remainder", snapshot.RequestID+":remainder", snapshot.DeferredGoalIDs)
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE public.goal_reviews SET evaluation_request_id=$1 WHERE evaluation_request_id=$2 AND status='pending' AND goal_id=ANY($3::text[])`, remainderID, snapshot.RequestID, snapshot.DeferredGoalIDs); err != nil {
				return err
			}
		}
		if len(snapshot.DeferredGoalIDs) == 0 {
			var remaining bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.goal_source_events WHERE fluctlight_id=$1 AND ($2='' OR profile_id IS NULL OR profile_id=$2) AND processed_at IS NULL)`, snapshot.FluctlightID, snapshot.ProfileID).Scan(&remaining); err != nil {
				return err
			}
			if remaining {
				active := make([]string, 0, len(snapshot.Goals))
				for _, entry := range snapshot.Goals {
					if entry.Goal.Status == GoalActive || entry.Goal.Status == GoalPaused {
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
		_, err := tx.Exec(ctx, `UPDATE public.goal_evaluation_requests SET status='succeeded',result=$2,claimed_at=NULL,error_code=NULL,updated_at=now() WHERE id=$1`, snapshot.RequestID, jsonBytes(result))
		return err
	})
	return result, err
}
