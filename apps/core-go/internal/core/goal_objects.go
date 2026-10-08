package core

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
)

func nonNilGoalHint(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{"state": "needs_planning"}
	}
	return value
}
func nonNilGoalCriteriaPolicy(value map[string]any) map[string]any {
	if value == nil {
		return map[string]any{"mode": "all", "optional_ids": []string{}}
	}
	return value
}

func validateIntentionGoalObjectsTx(ctx context.Context, tx pgx.Tx, intention IntentionAuthority) error {
	requiresActive := !intentionStatusTerminal(intention.Status) && intention.Status != IntentionPaused
	if intention.StageID != "" {
		var profile, state string
		if err := tx.QueryRow(ctx, `SELECT COALESCE(profile_id,''),status FROM public.goal_stages WHERE id=$1 AND fluctlight_id=$2 AND goal_id=$3`, intention.StageID, intention.FluctlightID, intention.GoalEntityID).Scan(&profile, &state); err != nil {
			return err
		}
		if (profile != "" && profile != intention.ProfileID) || (requiresActive && state != "active") {
			return errors.New("intention_stage_scope_invalid")
		}
	}
	if intention.CommitmentID != "" {
		var profile, stage, state string
		var start, end *time.Time
		if err := tx.QueryRow(ctx, `SELECT COALESCE(profile_id,''),COALESCE(stage_id,''),status,window_start,window_end FROM public.goal_commitments WHERE id=$1 AND fluctlight_id=$2 AND goal_id=$3`, intention.CommitmentID, intention.FluctlightID, intention.GoalEntityID).Scan(&profile, &stage, &state, &start, &end); err != nil {
			return err
		}
		if (profile != "" && profile != intention.ProfileID) || (intention.StageID != "" && stage != intention.StageID) || (requiresActive && state != "active") {
			return errors.New("intention_commitment_scope_invalid")
		}
		if stage != "" {
			var stageProfile, stageState string
			if err := tx.QueryRow(ctx, `SELECT COALESCE(profile_id,''),status FROM public.goal_stages WHERE id=$1 AND fluctlight_id=$2 AND goal_id=$3`, stage, intention.FluctlightID, intention.GoalEntityID).Scan(&stageProfile, &stageState); err != nil {
				return err
			}
			if stageProfile != profile || (requiresActive && stageState != "active") {
				return errors.New("intention_commitment_stage_not_active")
			}
		}

	}
	return nil
}

func readGoalObjectsWith(ctx context.Context, q lifeContextQuerier, owner, goalID string) ([]GoalStage, []GoalCommitment, error) {
	stages := []GoalStage{}
	commitments := []GoalCommitment{}
	rows, err := q.Query(ctx, `SELECT id,fluctlight_id,goal_id,COALESCE(profile_id,''),purpose,strategy,entry_basis,exit_basis,criteria,criteria_version,status,revision,reason,dependency_ids FROM public.goal_stages WHERE fluctlight_id=$1 AND goal_id=$2 ORDER BY created_at,id`, owner, goalID)
	if err != nil {
		return nil, nil, err
	}
	for rows.Next() {
		var stage GoalStage
		var raw, dependencyRaw []byte
		if err := rows.Scan(&stage.ID, &stage.FluctlightID, &stage.GoalID, &stage.ProfileID, &stage.Purpose, &stage.Strategy, &stage.EntryBasis, &stage.ExitBasis, &raw, &stage.CriteriaVersion, &stage.Status, &stage.Revision, &stage.Reason, &dependencyRaw); err != nil {
			rows.Close()
			return nil, nil, err
		}
		if err := decodeGoalCriteria(raw, &stage.Criteria); err != nil {
			rows.Close()
			return nil, nil, err
		}
		stage.DependencyIDs = decisionServiceRefValues(decodeArray(dependencyRaw))
		stages = append(stages, stage)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	rows, err = q.Query(ctx, `SELECT id,fluctlight_id,goal_id,COALESCE(stage_id,''),COALESCE(profile_id,''),expected_result,criteria,criteria_version,window_start,window_end,opportunity_condition,blocker,status,revision FROM public.goal_commitments WHERE fluctlight_id=$1 AND goal_id=$2 ORDER BY created_at,id`, owner, goalID)
	if err != nil {
		return nil, nil, err
	}
	for rows.Next() {
		var c GoalCommitment
		var raw []byte
		if err := rows.Scan(&c.ID, &c.FluctlightID, &c.GoalID, &c.StageID, &c.ProfileID, &c.ExpectedResult, &raw, &c.CriteriaVersion, &c.WindowStart, &c.WindowEnd, &c.OpportunityCondition, &c.Blocker, &c.Status, &c.Revision); err != nil {
			rows.Close()
			return nil, nil, err
		}
		if err := decodeGoalCriteria(raw, &c.Criteria); err != nil {
			rows.Close()
			return nil, nil, err
		}
		commitments = append(commitments, c)
	}
	rows.Close()
	return stages, commitments, rows.Err()
}

func decodeGoalCriteria(raw []byte, result *[]GoalCriterion) error {
	return json.Unmarshal(raw, result)
}

func applyGoalCommitmentPlanTx(ctx context.Context, tx pgx.Tx, parent GoalAuthority, c GoalCommitmentPlan, key string, at time.Time) error {
	if c.Operation == "" {
		if c.ID == "" {
			c.Operation = "create"
		} else {
			c.Operation = "adjust"
		}
	}
	if c.Operation == "abandon" {
		if c.ID == "" || c.Reason == "" {
			return errors.New("goal_commitment_abandon_reason_required")
		}
		cmd, err := tx.Exec(ctx, `UPDATE public.goal_commitments SET status='cancelled',blocker=$5,revision=revision+1,updated_at=now() WHERE id=$1 AND fluctlight_id=$2 AND goal_id=$3 AND revision=$4 AND status IN ('active','blocked','paused','expired')`, c.ID, parent.FluctlightID, parent.EntityID, c.ExpectedRevision, c.Reason)
		if err != nil {
			return err
		}
		if cmd.RowsAffected() != 1 {
			return ErrConflict
		}
		return nil
	}
	if c.ExpectedResult == "" || len(c.Criteria) == 0 || (c.WindowStart != nil && c.WindowEnd != nil && !c.WindowEnd.After(*c.WindowStart)) {
		return errors.New("goal_commitment_plan_invalid")
	}
	if parent.DeadlinePolicy == "hard" && parent.Deadline != nil && c.WindowEnd != nil && c.WindowEnd.After(*parent.Deadline) {
		return errors.New("goal_commitment_after_hard_deadline")
	}
	state := "active"
	if c.Blocker != "" {
		state = "blocked"
	}
	if c.WindowEnd != nil && !at.Before(*c.WindowEnd) {
		state = "expired"
	}
	if c.Operation == "create" {
		if c.ID != "" {
			return errors.New("goal_commitment_create_identity_invalid")
		}
		var active bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.goal_commitments WHERE fluctlight_id=$1 AND goal_id=$2 AND status IN ('active','blocked','paused'))`, parent.FluctlightID, parent.EntityID).Scan(&active); err != nil {
			return err
		}
		if active {
			return errors.New("goal_commitment_already_active")
		}
		id := "commitment_" + stableDigest(key+"\x1f"+parent.EntityID)
		criteria := newGoalObjectCriteria(id, 1, c.Criteria)
		if err := validateGoalCriteria(criteria); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO public.goal_commitments(id,fluctlight_id,goal_id,stage_id,profile_id,expected_result,criteria,criteria_version,window_start,window_end,opportunity_condition,blocker,status,revision) VALUES($1,$2,$3,$4,$5,$6,$7,1,$8,$9,$10,$11,$12,1)`, id, parent.FluctlightID, parent.EntityID, nullableString(parent.CurrentStageID), nullableString(parent.ProfileID), c.ExpectedResult, jsonBytes(criteria), c.WindowStart, c.WindowEnd, c.OpportunityCondition, c.Blocker, state)
		return err
	}
	if c.Operation != "adjust" {
		return errors.New("goal_commitment_operation_invalid")
	}
	_, commitments, err := readGoalObjectsWith(ctx, tx, parent.FluctlightID, parent.EntityID)
	if err != nil {
		return err
	}
	var current *GoalCommitment
	for i := range commitments {
		if commitments[i].ID == c.ID {
			current = &commitments[i]
		}
	}
	if current == nil || current.Revision != c.ExpectedRevision || current.Status == "completed" || current.Status == "cancelled" {
		return errors.New("goal_commitment_revision_conflict")
	}
	before := GoalAuthority{EntityID: current.ID, CriteriaVersion: current.CriteriaVersion}
	for _, criterion := range current.Criteria {
		before.CriterionIDs = append(before.CriterionIDs, criterion.ID)
		before.SuccessCriteria = append(before.SuccessCriteria, criterion.Text)
	}
	version := current.CriteriaVersion
	ids := before.CriterionIDs
	if !slices.Equal(before.SuccessCriteria, c.Criteria) || current.ExpectedResult != c.ExpectedResult {
		after := before
		after.SuccessCriteria = c.Criteria
		after.CriteriaVersion++
		version = after.CriteriaVersion
		ids = reviseGoalCriterionIDs(before, after)
	}
	criteria := []GoalCriterion{}
	for i, text := range c.Criteria {
		criteria = append(criteria, GoalCriterion{ID: ids[i], Text: text})
	}
	if err := validateGoalCriteria(criteria); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE public.goal_commitments SET expected_result=$5,criteria=$6,criteria_version=$7,window_start=$8,window_end=$9,opportunity_condition=$10,blocker=$11,status=$12,revision=revision+1,updated_at=now() WHERE id=$1 AND fluctlight_id=$2 AND goal_id=$3 AND revision=$4`, c.ID, parent.FluctlightID, parent.EntityID, c.ExpectedRevision, c.ExpectedResult, jsonBytes(criteria), version, c.WindowStart, c.WindowEnd, c.OpportunityCondition, c.Blocker, state)
	return err
}

func cascadeGoalObjectsTx(ctx context.Context, tx pgx.Tx, before *GoalAuthority, after GoalAuthority, at time.Time) error {
	if before == nil {
		return nil
	}
	for _, table := range []string{"public.goal_stages", "public.goal_commitments"} {
		if goalStatusTerminal(after.Status) {
			if _, err := tx.Exec(ctx, `UPDATE `+table+` SET status='cancelled',held_by_goal=false,revision=revision+1,updated_at=now() WHERE fluctlight_id=$1 AND goal_id=$2 AND status NOT IN ('completed','cancelled','skipped')`, after.FluctlightID, after.EntityID); err != nil {
				return err
			}
		} else if after.Status == GoalPaused || effectiveGoalCriteriaVersion(*before) != effectiveGoalCriteriaVersion(after) {
			hold := after.Status == GoalPaused
			if _, err := tx.Exec(ctx, `UPDATE `+table+` SET status='paused',held_by_goal=$3,revision=revision+1,updated_at=now() WHERE fluctlight_id=$1 AND goal_id=$2 AND status IN ('active','blocked')`, after.FluctlightID, after.EntityID, hold); err != nil {
				return err
			}
		} else if before.Status == GoalPaused && after.Status == GoalActive {
			state := "'active'"
			if table == "public.goal_commitments" {
				state = "CASE WHEN window_end IS NOT NULL AND window_end <= $3 THEN 'expired' WHEN blocker <> '' THEN 'blocked' ELSE 'active' END"
			}
			args := []any{after.FluctlightID, after.EntityID}
			if table == "public.goal_commitments" {
				args = append(args, at)
			}
			if _, err := tx.Exec(ctx, `UPDATE `+table+` SET status=`+state+`,held_by_goal=false,revision=revision+1,updated_at=now() WHERE fluctlight_id=$1 AND goal_id=$2 AND status='paused' AND held_by_goal`, args...); err != nil {
				return err
			}
		}
	}
	return nil
}

func requireGoalCommitmentWindowTx(ctx context.Context, tx pgx.Tx, intention IntentionAuthority, at time.Time) error {
	if intention.CommitmentID == "" {
		return nil
	}
	var state string
	var start, end *time.Time
	if err := tx.QueryRow(ctx, `SELECT status,window_start,window_end FROM public.goal_commitments WHERE id=$1 AND fluctlight_id=$2 AND goal_id=$3`, intention.CommitmentID, intention.FluctlightID, intention.GoalEntityID).Scan(&state, &start, &end); err != nil {
		return err
	}
	if state != "active" {
		return errors.New("intention_commitment_not_active")
	}
	if start != nil && at.Before(*start) {
		return errors.New("intention_commitment_window_not_started")
	}
	if end != nil && !at.Before(*end) {
		return errors.New("intention_commitment_window_ended")
	}
	return nil
}

func validateGoalStageDependenciesTx(ctx context.Context, tx pgx.Tx, parent GoalAuthority, stageID string, ids []string) error {
	if len(ids) > 8 {
		return errors.New("goal_stage_dependency_limit")
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if id == stageID || id == "" || seen[id] {
			return errors.New("goal_stage_dependency_invalid")
		}
		seen[id] = true
		var state string
		if err := tx.QueryRow(ctx, `SELECT status FROM public.goal_stages WHERE id=$1 AND fluctlight_id=$2 AND goal_id=$3`, id, parent.FluctlightID, parent.EntityID).Scan(&state); err != nil {
			return err
		}
		if state != "completed" {
			return errors.New("goal_stage_dependency_not_completed")
		}
	}
	return nil
}
