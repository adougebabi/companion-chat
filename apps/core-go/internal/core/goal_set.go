package core

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// GoalSetCommand changes the final set atomically. Lifecycle evidence remains
// owned by GoalEvaluation; this command cannot manufacture completion.
type GoalSetCommand struct {
	RecoveryPolicy        *GoalPlannerRecoveryPolicy `json:"recovery_policy,omitempty"`
	SourceReviews         []GoalSourceReviewDecision `json:"source_reviews,omitempty"`
	ReviewedGoalIDs       []string                   `json:"reviewed_goal_ids,omitempty"`
	ExpectedVersion       int64                      `json:"expected_version"`
	ExpectedFactsRevision string                     `json:"expected_facts_revision"`
	IdempotencyKey        string                     `json:"idempotency_key"`
	Reason                string                     `json:"reason"`
	AutoPlanningEnabled   *bool                      `json:"auto_planning_enabled,omitempty"`
	OrderingMode          string                     `json:"ordering_mode,omitempty"`
	MaxActiveGoals        *int                       `json:"max_active_goals,omitempty"`
	Order                 []string                   `json:"order,omitempty"`
	Changes               []GoalSetChange            `json:"changes,omitempty"`
	Dependencies          map[string][]string        `json:"dependencies,omitempty"`
	Decision              string                     `json:"decision,omitempty"`
	NextReviewAt          *time.Time                 `json:"next_review_at,omitempty"`
}
type GoalSetChange struct {
	ProfileID        string         `json:"profile_id,omitempty"`
	Scope            string         `json:"scope,omitempty"`
	Deadline         *time.Time     `json:"deadline,omitempty"`
	DeadlinePolicy   string         `json:"deadline_policy,omitempty"`
	ID               string         `json:"id"`
	Operation        string         `json:"operation"`
	ExpectedRevision int            `json:"expected_revision"`
	Outcome          string         `json:"desired_outcome"`
	Criteria         []string       `json:"success_criteria"`
	Motivation       string         `json:"motivation"`
	TargetActorID    string         `json:"target_actor_id"`
	Activate         bool           `json:"activate"`
	Source           map[string]any `json:"source"`
}

type GoalPlannerRecoveryPolicy struct {
	MergeWindowSeconds  int `json:"merge_window_seconds"`
	RetryBackoffSeconds int `json:"retry_backoff_seconds"`
	MaxAttempts         int `json:"max_attempts"`
	LeaseSeconds        int `json:"lease_seconds"`
	CandidateLimit      int `json:"candidate_limit"`
}

func (p GoalPlannerRecoveryPolicy) valid() bool {
	return p.MergeWindowSeconds >= 1 && p.MergeWindowSeconds <= 60 && p.RetryBackoffSeconds >= 5 && p.RetryBackoffSeconds <= 3600 && p.MaxAttempts >= 1 && p.MaxAttempts <= 10 && p.LeaseSeconds >= 60 && p.LeaseSeconds <= 600 && p.CandidateLimit >= 1 && p.CandidateLimit <= 20
}

type GoalSourceReviewDecision struct {
	SourceID         string   `json:"source_id"`
	ExpectedRevision int      `json:"expected_revision"`
	Operation        string   `json:"operation"`
	GoalID           string   `json:"goal_id,omitempty"`
	Outcome          string   `json:"desired_outcome,omitempty"`
	Criteria         []string `json:"success_criteria,omitempty"`
	Motivation       string   `json:"motivation,omitempty"`
}

func ensureGoalSetTx(ctx context.Context, tx pgx.Tx, owner string) error {
	if err := lockLifeContextTx(ctx, tx, owner); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `INSERT INTO public.goal_set_policies(fluctlight_id) VALUES($1) ON CONFLICT DO NOTHING`, owner)
	return err
}
func checkGoalCapacityTx(ctx context.Context, tx pgx.Tx, before *GoalAuthority, after GoalAuthority) error {
	if err := ensureGoalSetTx(ctx, tx, after.FluctlightID); err != nil {
		return err
	}
	if after.Status == GoalCandidate && before == nil {
		var count, limit int
		if err := tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM public.fluctlight_goals WHERE fluctlight_id=$1 AND status='candidate'),candidate_limit FROM public.goal_set_policies WHERE fluctlight_id=$1 FOR UPDATE`, after.FluctlightID).Scan(&count, &limit); err != nil {
			return err
		}
		if count >= limit {
			return newCapabilityError("goal_candidate_capacity_exceeded", false, ErrConflict)
		}
	}
	if after.Status != GoalActive || (before != nil && before.Status == GoalActive) {
		return nil
	}
	var count, limit int
	if err := tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM public.fluctlight_goals WHERE fluctlight_id=$1 AND status='active' AND id<>$2),max_active_goals FROM public.goal_set_policies WHERE fluctlight_id=$1 FOR UPDATE`, after.FluctlightID, after.EntityID).Scan(&count, &limit); err != nil {
		return err
	}
	if count >= limit {
		return newCapabilityError("goal_capacity_exceeded", false, ErrConflict)
	}
	return nil
}
func (a *App) GoalSetSnapshot(ctx context.Context, actor, owner, profile string) (map[string]any, error) {
	if _, err := a.DB.GetFluctlight(ctx, owner, actor); err != nil {
		return nil, err
	}
	var result map[string]any
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if err := ensureGoalSetTx(ctx, tx, owner); err != nil {
			return err
		}
		var err error
		result, err = readGoalSetWith(ctx, tx, owner, profile)
		return err
	})
	return result, err
}
func readGoalSetWith(ctx context.Context, q lifeContextQuerier, owner, profile string) (map[string]any, error) {
	var raw []byte
	if err := q.QueryRow(ctx, `SELECT to_jsonb(p)||jsonb_build_object('active_count',(SELECT count(*) FROM public.fluctlight_goals WHERE fluctlight_id=$1 AND status='active')) FROM public.goal_set_policies p WHERE fluctlight_id=$1`, owner).Scan(&raw); err != nil {
		return nil, err
	}
	result := decodeObject(raw)
	result["capacity_violation"] = intValue(result["active_count"]) > intValue(result["max_active_goals"])
	facts, err := readCurrentFactsRevisionWith(ctx, q, owner)
	if err != nil {
		return nil, err
	}
	result["facts_revision"] = facts
	rows, err := q.Query(ctx, `SELECT to_jsonb(g)-'request_digest'-'idempotency_key' FROM public.fluctlight_goals g WHERE fluctlight_id=$1 AND ($2='*' OR profile_id IS NULL OR profile_id=$2) AND status IN ('active','candidate','paused') ORDER BY effective_order,created_at,id LIMIT 40`, owner, profile)
	if err != nil {
		return nil, err
	}
	items := []any{}
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			rows.Close()
			return nil, err
		}
		items = append(items, decodeObject(data))
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	result["goals"] = items
	rows, err = q.Query(ctx, `SELECT d.goal_id,d.prerequisite_id,p.status FROM public.goal_dependencies d JOIN public.fluctlight_goals g ON g.id=d.goal_id JOIN public.fluctlight_goals p ON p.id=d.prerequisite_id WHERE d.fluctlight_id=$1 AND ($2='*' OR (g.profile_id IS NULL OR g.profile_id=$2)) ORDER BY d.goal_id,d.prerequisite_id LIMIT 100`, owner, profile)
	if err != nil {
		return nil, err
	}
	deps := []any{}
	for rows.Next() {
		var g, p, s string
		if err := rows.Scan(&g, &p, &s); err != nil {
			rows.Close()
			return nil, err
		}
		deps = append(deps, map[string]any{"goal_id": g, "prerequisite_id": p, "satisfied": s == "completed", "status": s})
	}
	rows.Close()
	result["dependencies"] = deps
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sourceRows, err := q.Query(ctx, `SELECT to_jsonb(r) FROM public.goal_initial_source_reviews r WHERE fluctlight_id=$1 AND ($2='*' OR profile_id IS NULL OR profile_id=$2) AND status='requires_semantic_review' ORDER BY created_at,source_id LIMIT 20`, owner, profile)
	if err != nil {
		return nil, err
	}
	reviews := []any{}
	for sourceRows.Next() {
		var raw []byte
		if err := sourceRows.Scan(&raw); err != nil {
			sourceRows.Close()
			return nil, err
		}
		reviews = append(reviews, decodeObject(raw))
	}
	sourceRows.Close()
	result["source_reviews"] = reviews
	return result, sourceRows.Err()
}
func (a *App) ApplyGoalSetCommand(ctx context.Context, actor, owner string, c GoalSetCommand) (map[string]any, error) {
	if _, err := a.DB.GetFluctlight(ctx, owner, actor); err != nil {
		return nil, err
	}
	var result map[string]any
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var err error
		result, err = a.applyGoalSetTx(ctx, tx, actor, owner, "*", "", c)
		return err
	})
	return result, err
}
func (a *App) applyGoalSetTx(ctx context.Context, tx pgx.Tx, actor, owner, profile, runID string, c GoalSetCommand) (map[string]any, error) {
	if c.ExpectedVersion < 1 || strings.TrimSpace(c.IdempotencyKey) == "" || len(c.IdempotencyKey) > 256 || strings.TrimSpace(c.Reason) == "" || len(c.Reason) > 2000 || len(c.Changes) > 20 || len(c.Order) > 40 || len(c.Dependencies) > 40 {
		return nil, ErrInvalidArguments
	}
	if err := ensureGoalSetTx(ctx, tx, owner); err != nil {
		return nil, err
	}
	digest := stableDigest(jsonString(c))
	var priorDigest string
	var raw []byte
	err := tx.QueryRow(ctx, `SELECT request_digest,result FROM public.goal_set_commands WHERE fluctlight_id=$1 AND idempotency_key=$2`, owner, c.IdempotencyKey).Scan(&priorDigest, &raw)
	if err == nil {
		if digest != priorDigest {
			return nil, ErrConflict
		}
		return decodeObject(raw), nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	var version int64
	var enabled bool
	var mode string
	var limit, leaseSeconds int
	if err := tx.QueryRow(ctx, `SELECT revision,auto_planning_enabled,ordering_mode,max_active_goals,lease_seconds FROM public.goal_set_policies WHERE fluctlight_id=$1 FOR UPDATE`, owner).Scan(&version, &enabled, &mode, &limit, &leaseSeconds); err != nil {
		return nil, err
	}
	if version != c.ExpectedVersion {
		return nil, newCapabilityError("goal_set_version_conflict", true, ErrConflict)
	}
	if c.ExpectedFactsRevision != "" {
		if err := a.requireCurrentFactsRevisionTx(ctx, tx, owner, c.ExpectedFactsRevision); err != nil {
			return nil, err
		}
	}
	if runID != "" && strings.TrimSpace(c.ExpectedFactsRevision) == "" {
		return nil, ErrInvalidArguments
	}
	if runID != "" {
		var instanceStatus string
		if err := tx.QueryRow(ctx, `SELECT status FROM public.fluctlights WHERE id=$1 FOR UPDATE`, owner).Scan(&instanceStatus); err != nil {
			return nil, err
		}
		if instanceStatus != "active" {
			return nil, newCapabilityError("goal_planner_instance_inactive", false, ErrUnauthorized)
		}
		parts := strings.Split(runID, "@")
		if len(parts) != 2 {
			return nil, ErrUnauthorized
		}
		claim, parseErr := strconv.Atoi(parts[1])
		if parseErr != nil {
			return nil, ErrUnauthorized
		}
		runID = parts[0]
		var actualClaim int
		var runMode, status string
		var committedRaw []byte
		var expiredLease bool
		var claimed *time.Time
		if err := tx.QueryRow(ctx, `SELECT mode,status,claimed_at,claim_revision,result,claimed_at IS NULL OR claimed_at<now()-($3::integer)*interval '1 second' FROM public.goal_planning_runs WHERE id=$1 AND fluctlight_id=$2 FOR UPDATE`, runID, owner, leaseSeconds).Scan(&runMode, &status, &claimed, &actualClaim, &committedRaw, &expiredLease); err != nil {
			return nil, err
		}
		if decodeObject(committedRaw)["commit_succeeded"] == true {
			return nil, newCapabilityError("goal_plan_already_committed", false, ErrConflict)
		}
		if actualClaim != claim || status != "processing" || claimed == nil || expiredLease {
			return nil, newCapabilityError("goal_planner_lease_expired", false, ErrConflict)
		}
		activeProfile, err := (&intentionService{}).resolveProfile(ctx, tx, owner, "")
		if err != nil {
			return nil, err
		}
		if activeProfile != profile {
			return nil, newCapabilityError("goal_planner_profile_changed", false, ErrConflict)
		}
		if !enabled || runMode != "apply" {
			return nil, newCapabilityError("goal_planning_suggestions_only", false, ErrUnauthorized)
		}
		policy, err := a.evaluateAutonomyPolicyWithReader(ctx, tx, owner, "capability", a.now(), "", false)
		if err != nil {
			return nil, err
		}
		if !policy.Allowed {
			return nil, newCapabilityError("goal_planning_policy_blocked", false, ErrUnauthorized)
		}
		if _, err := tx.Exec(ctx, `SELECT set_config('fluctlight.goal_planner_run',$1,true)`, runID); err != nil {
			return nil, err
		}
		if c.AutoPlanningEnabled != nil || c.MaxActiveGoals != nil || c.OrderingMode != "" || c.RecoveryPolicy != nil {
			return nil, ErrUnauthorized
		}
	}
	if c.RecoveryPolicy != nil {
		if !c.RecoveryPolicy.valid() {
			return nil, ErrInvalidArguments
		}
		var candidates int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM public.fluctlight_goals WHERE fluctlight_id=$1 AND status='candidate'`, owner).Scan(&candidates); err != nil {
			return nil, err
		}
		if candidates > c.RecoveryPolicy.CandidateLimit {
			return nil, newCapabilityError("goal_candidate_capacity_exceeded", false, ErrConflict)
		}
	}
	if c.MaxActiveGoals != nil {
		if *c.MaxActiveGoals < 1 || *c.MaxActiveGoals > 5 {
			return nil, ErrInvalidArguments
		}
		limit = *c.MaxActiveGoals
	}
	if c.OrderingMode != "" {
		if c.OrderingMode != "automatic" && c.OrderingMode != "manual" {
			return nil, ErrInvalidArguments
		}
		mode = c.OrderingMode
	}
	if len(c.SourceReviews) > 20 || (runID != "" && len(c.SourceReviews) > 0) {
		return nil, ErrUnauthorized
	}
	for _, review := range c.SourceReviews {
		if review.Operation == "candidate" {
			sourceProfile := ""
			if err := tx.QueryRow(ctx, `SELECT COALESCE(profile_id,'') FROM public.goal_initial_source_reviews WHERE fluctlight_id=$1 AND source_id=$2`, owner, review.SourceID).Scan(&sourceProfile); err != nil {
				return nil, err
			}
			c.Changes = append(c.Changes, GoalSetChange{ID: "review:" + review.SourceID, ProfileID: sourceProfile, Operation: "create", Outcome: review.Outcome, Criteria: review.Criteria, Motivation: review.Motivation, Activate: false, Source: map[string]any{"initial_source_id": review.SourceID, "source_type": "owner_semantic_review"}})
		}
	}
	ids := map[string]string{}
	applied := []any{}
	// Pauses first, then activation: all changes share one rollback boundary.
	for _, phase := range []bool{true, false} {
		for _, change := range c.Changes {
			if (change.Operation == "pause") != phase {
				continue
			}
			id := change.ID
			var goal GoalAuthority
			var record GoalGovernanceRecord
			var before *GoalAuthority
			refs := []string{"owner:" + actor}
			if runID != "" {
				refs = []string{"planner:" + runID}
			}
			if change.Operation == "create" {
				if id == "" || ids[id] != "" {
					return nil, ErrInvalidArguments
				}
				local := id
				id = "goal_plan_" + stableDigest(owner+":"+c.IdempotencyKey+":"+local)
				ids[local] = id
				if change.TargetActorID != "" {
					if err := a.requireVisibleActorWith(ctx, tx, owner, actor, profile, change.TargetActorID); err != nil {
						return nil, err
					}
				}
				status := GoalCandidate
				if change.Activate {
					status = GoalActive
				}
				p := profile
				if change.ProfileID != "" {
					if runID != "" && change.ProfileID != profile {
						return nil, ErrUnauthorized
					}
					var persona []byte
					if err := tx.QueryRow(ctx, `SELECT core_persona FROM public.fluctlights WHERE id=$1`, owner).Scan(&persona); err != nil {
						return nil, err
					}
					if _, ok := personalityProfileIDs(decodeObject(persona))[change.ProfileID]; !ok {
						return nil, ErrInvalidArguments
					}
					p = change.ProfileID
				}
				if p == "*" {
					p = ""
				}
				goal, record, err = CreateGoalAuthority(GoalAuthority{EntityID: id, SchemaVersion: goalAuthoritySchemaVersion, Ref: "goal:ctx_" + stableDigest(id), FluctlightID: owner, ProfileID: p, DesiredOutcome: change.Outcome, SuccessCriteria: change.Criteria, Motivation: change.Motivation, Scope: firstString(change.Scope, "general"), Deadline: change.Deadline, DeadlinePolicy: firstString(change.DeadlinePolicy, "soft"), TargetActorID: change.TargetActorID, Status: status, Revision: 1}, refs, a.now())
				if err == nil {
					var duplicate string
					err = tx.QueryRow(ctx, `SELECT id FROM public.fluctlight_goals WHERE fluctlight_id=$1 AND COALESCE(target_actor_id,'')=$2 AND lower(trim(desired_outcome))=lower(trim($3)) LIMIT 1`, owner, change.TargetActorID, change.Outcome).Scan(&duplicate)
					if err == nil {
						return nil, newCapabilityError("goal_duplicate_history", false, ErrConflict)
					}
					if errors.Is(err, pgx.ErrNoRows) {
						err = nil
					}
				}
			} else {
				if change.Operation != "pause" && change.Operation != "resume" && change.Operation != "cancel" {
					return nil, ErrInvalidArguments
				}
				g, loadErr := loadGoalAuthorityTx(ctx, tx, owner, "goal:ctx_"+stableDigest(id), ContextReference{EntityID: id, Revision: change.ExpectedRevision})
				if loadErr != nil {
					return nil, loadErr
				}
				if profile != "*" && g.ProfileID != "" && g.ProfileID != profile {
					return nil, ErrUnauthorized
				}
				if runID != "" {
					var protected bool
					if err := tx.QueryRow(ctx, `SELECT owner_protected FROM public.fluctlight_goals WHERE id=$1`, id).Scan(&protected); err != nil {
						return nil, err
					}
					if protected || change.Operation == "cancel" {
						return nil, ErrUnauthorized
					}
				}
				before = &g
				goal, record, err = ApplyGoalCommand(&g, GoalCommand{Operation: GoalLifecycleOperation(change.Operation), ExpectedRevision: change.ExpectedRevision, EvidenceRefs: refs, Reason: c.Reason, OccurredAt: a.now()})
			}
			if err != nil {
				return nil, err
			}
			record.ActorID, record.Source, record.Reason = actor, "goal_set", c.Reason
			if _, err := persistGoalAuthorityTx(ctx, tx, before, goal, record, "goal-set:"+owner+":"+c.IdempotencyKey+":"+id); err != nil {
				return nil, err
			}
			if change.Operation == "create" {
				source := cloneMap(change.Source)
				if source == nil {
					source = map[string]any{}
				}
				source["planner_run_id"] = runID
				source["reason"] = c.Reason
				if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_goals SET planner_source=$2 WHERE id=$1`, id, jsonBytes(source)); err != nil {
					return nil, err
				}
			}
			applied = append(applied, map[string]any{"goal_id": id, "status": goal.Status, "revision": goal.Revision})
		}
	}
	resolve := func(id string) string {
		if v := ids[id]; v != "" {
			return v
		}
		return id
	}
	for _, review := range c.SourceReviews {
		var revision int
		var status string
		var raw []byte
		if err := tx.QueryRow(ctx, `SELECT revision,status,raw_source FROM public.goal_initial_source_reviews WHERE fluctlight_id=$1 AND source_id=$2 FOR UPDATE`, owner, review.SourceID).Scan(&revision, &status, &raw); err != nil {
			return nil, err
		}
		if revision != review.ExpectedRevision || status != "requires_semantic_review" {
			return nil, ErrConflict
		}
		goalID := review.GoalID
		decision := map[string]any{"operation": review.Operation, "reason": c.Reason, "actor_id": actor}
		switch review.Operation {
		case "candidate":
			goalID = resolve("review:" + review.SourceID)
			status = "candidate_saved"
		case "link_goal":
			status = "linked_existing"
		case "dismiss":
			status = "dismissed_non_action_source"
		default:
			return nil, ErrInvalidArguments
		}
		if review.Operation != "dismiss" {
			var visible bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.fluctlight_goals WHERE id=$1 AND fluctlight_id=$2 AND ($3='*' OR profile_id IS NULL OR profile_id=$3))`, goalID, owner, profile).Scan(&visible); err != nil {
				return nil, err
			}
			if !visible {
				return nil, ErrUnauthorized
			}
			if _, err := tx.Exec(ctx, `INSERT INTO public.goal_initial_imports(fluctlight_id,source_id,input_version,goal_id,result) VALUES($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`, owner, review.SourceID, stableDigest(string(raw)), goalID, status); err != nil {
				return nil, err
			}
			decision["goal_id"] = goalID
		}
		if _, err := tx.Exec(ctx, `UPDATE public.goal_initial_source_reviews SET status=$3,revision=revision+1,decision=$4 WHERE fluctlight_id=$1 AND source_id=$2`, owner, review.SourceID, status, jsonBytes(decision)); err != nil {
			return nil, err
		}
	}
	for goalID := range c.Dependencies {
		var visible bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.fluctlight_goals WHERE id=$1 AND fluctlight_id=$2 AND ($3='*' OR profile_id IS NULL OR profile_id=$3))`, resolve(goalID), owner, profile).Scan(&visible); err != nil {
			return nil, err
		}
		if !visible {
			return nil, ErrUnauthorized
		}
	}
	for goalID, parents := range c.Dependencies {
		goalID = resolve(goalID)
		if len(parents) > 20 {
			return nil, ErrInvalidArguments
		}
		if _, err := tx.Exec(ctx, `DELETE FROM public.goal_dependencies WHERE fluctlight_id=$1 AND goal_id=$2`, owner, goalID); err != nil {
			return nil, err
		}
		for _, parent := range parents {
			parent = resolve(parent)
			if err := validateGoalDependencyTx(ctx, tx, owner, profile, goalID, parent); err != nil {
				return nil, err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO public.goal_dependencies(fluctlight_id,goal_id,prerequisite_id) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, owner, goalID, parent); err != nil {
				return nil, err
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_goals SET updated_at=now() WHERE fluctlight_id=$1 AND id=$2`, owner, goalID); err != nil {
			return nil, err
		}

	}
	for _, id := range c.ReviewedGoalIDs {
		tag, err := tx.Exec(ctx, `UPDATE public.fluctlight_goals SET context_review_required=false WHERE fluctlight_id=$1 AND id=$2 AND ($3='*' OR profile_id IS NULL OR profile_id=$3)`, owner, resolve(id), profile)
		if err != nil {
			return nil, err
		}
		if tag.RowsAffected() != 1 {
			return nil, ErrUnauthorized
		}
	}
	if len(c.Order) > 0 {
		if runID != "" && mode == "manual" {
			return nil, newCapabilityError("goal_manual_order_protected", false, ErrConflict)
		}
		var count int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM public.fluctlight_goals WHERE fluctlight_id=$1 AND status='active' AND ($2='*' OR profile_id IS NULL OR profile_id=$2)`, owner, profile).Scan(&count); err != nil {
			return nil, err
		}
		if len(c.Order) != count {
			return nil, ErrInvalidArguments
		}
		positions := []int64{}
		if profile != "*" {
			rows, err := tx.Query(ctx, `SELECT effective_order FROM public.fluctlight_goals WHERE fluctlight_id=$1 AND status='active' AND (profile_id IS NULL OR profile_id=$2) ORDER BY effective_order,created_at,id`, owner, profile)
			if err != nil {
				return nil, err
			}
			for rows.Next() {
				var order int64
				if err := rows.Scan(&order); err != nil {
					rows.Close()
					return nil, err
				}
				positions = append(positions, order)
			}
			rows.Close()
			if err := rows.Err(); err != nil {
				return nil, err
			}
		}
		seen := map[string]bool{}
		for i, id := range c.Order {
			id = resolve(id)
			if seen[id] {
				return nil, ErrInvalidArguments
			}
			seen[id] = true
			position := int64(i + 1)
			if len(positions) > 0 {
				position = positions[i]
			}
			tag, err := tx.Exec(ctx, `UPDATE public.fluctlight_goals SET effective_order=$3 WHERE fluctlight_id=$1 AND id=$2 AND status='active' AND ($4='*' OR profile_id IS NULL OR profile_id=$4)`, owner, id, position, profile)
			if err != nil {
				return nil, err
			}
			if tag.RowsAffected() != 1 {
				return nil, ErrUnauthorized
			}
		}
	}
	var count int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM public.fluctlight_goals WHERE fluctlight_id=$1 AND status='active'`, owner).Scan(&count); err != nil {
		return nil, err
	}
	if c.MaxActiveGoals != nil && count > limit {
		return nil, newCapabilityError("goal_capacity_exceeded", false, ErrConflict)
	}
	if c.AutoPlanningEnabled != nil {
		enabled = *c.AutoPlanningEnabled
	}
	if _, err := tx.Exec(ctx, `UPDATE public.goal_set_policies SET revision=revision+1,auto_planning_enabled=$2,ordering_mode=$3,max_active_goals=$4,updated_at=now() WHERE fluctlight_id=$1`, owner, enabled, mode, limit); err != nil {
		return nil, err
	}
	if c.RecoveryPolicy != nil {
		p := *c.RecoveryPolicy
		if _, err := tx.Exec(ctx, `UPDATE public.goal_set_policies SET merge_window_seconds=$2,retry_backoff_seconds=$3,max_attempts=$4,lease_seconds=$5,candidate_limit=$6 WHERE fluctlight_id=$1`, owner, p.MergeWindowSeconds, p.RetryBackoffSeconds, p.MaxAttempts, p.LeaseSeconds, p.CandidateLimit); err != nil {
			return nil, err
		}
	}
	result, err := readGoalSetWith(ctx, tx, owner, profile)
	if err != nil {
		return nil, err
	}
	result["commit_succeeded"] = true
	result["applied"] = applied
	result["id_mapping"] = ids
	result["reason"] = c.Reason
	if _, err := tx.Exec(ctx, `INSERT INTO public.goal_set_commands(fluctlight_id,idempotency_key,request_digest,result) VALUES($1,$2,$3,$4)`, owner, c.IdempotencyKey, digest, jsonBytes(result)); err != nil {
		return nil, err
	}
	if runID != "" {
		_, err = tx.Exec(ctx, `UPDATE public.goal_planning_runs SET result=$2,updated_at=now() WHERE id=$1`, runID, jsonBytes(result))
	} else if c.AutoPlanningEnabled != nil && enabled {
		err = requestGoalPlanningTx(ctx, tx, owner, "policy:"+c.IdempotencyKey, "auto_planning_enabled", nil)
	}
	return result, err
}
func validateGoalDependencyTx(ctx context.Context, tx pgx.Tx, owner, profile, goal, parent string) error {
	if goal == parent {
		return newCapabilityError("goal_dependency_cycle", false, ErrInvalidArguments)
	}
	var gProfile, pProfile string
	if err := tx.QueryRow(ctx, `SELECT COALESCE(g.profile_id,''),COALESCE(p.profile_id,'') FROM public.fluctlight_goals g JOIN public.fluctlight_goals p ON p.fluctlight_id=g.fluctlight_id WHERE g.id=$1 AND p.id=$2 AND g.fluctlight_id=$3`, goal, parent, owner).Scan(&gProfile, &pProfile); err != nil {
		return ErrUnauthorized
	}
	if (pProfile != "" && gProfile != pProfile) || (profile != "*" && ((gProfile != "" && gProfile != profile) || (pProfile != "" && pProfile != profile))) {
		return ErrUnauthorized
	}
	var cycle bool
	if err := tx.QueryRow(ctx, `WITH RECURSIVE ancestors(id) AS (SELECT prerequisite_id FROM public.goal_dependencies WHERE goal_id=$1 UNION SELECT d.prerequisite_id FROM public.goal_dependencies d JOIN ancestors a ON d.goal_id=a.id) SELECT EXISTS(SELECT 1 FROM ancestors WHERE id=$2)`, parent, goal).Scan(&cycle); err != nil {
		return err
	}
	if cycle {
		return newCapabilityError("goal_dependency_cycle", false, ErrConflict)
	}
	return nil
}
func requireGoalDependenciesTx(ctx context.Context, tx pgx.Tx, owner, id string) error {
	var blocked, review bool
	if err := tx.QueryRow(ctx, `SELECT context_review_required,EXISTS(SELECT 1 FROM public.goal_dependencies d JOIN public.fluctlight_goals p ON p.id=d.prerequisite_id WHERE d.fluctlight_id=$1 AND d.goal_id=$2 AND (p.status<>'completed' OR p.context_review_required)) FROM public.fluctlight_goals WHERE fluctlight_id=$1 AND id=$2`, owner, id).Scan(&review, &blocked); err != nil {
		return err
	}
	if review {
		return newCapabilityError("goal_context_review_required", false, ErrConflict)
	}
	if blocked {
		return newCapabilityError("goal_dependency_unsatisfied", false, ErrConflict)
	}
	return nil
}
func requestGoalPlanningTx(ctx context.Context, tx pgx.Tx, owner, source, reason string, data map[string]any) error {
	if data == nil {
		data = map[string]any{}
	}
	if data["profile_id"] == nil && stringValue(data["goal_id"]) != "" {
		var sourceProfile string
		if err := tx.QueryRow(ctx, `SELECT COALESCE(profile_id,'') FROM public.fluctlight_goals WHERE fluctlight_id=$1 AND id=$2`, owner, stringValue(data["goal_id"])).Scan(&sourceProfile); err != nil {
			return err
		}
		data["profile_id"] = sourceProfile
	}
	if data["profile_id"] == nil {
		var profile string
		if err := tx.QueryRow(ctx, `SELECT COALESCE(active_profile_id,'') FROM public.fluctlight_personality_runtime WHERE fluctlight_id=$1`, owner).Scan(&profile); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		data["profile_id"] = profile
	}
	_, err := tx.Exec(ctx, `SELECT public.request_goal_planning($1,$2,$3,$4)`, owner, source, reason, jsonBytes(data))
	return err
}
func (a *App) requireVisibleActorWith(ctx context.Context, q lifeContextQuerier, owner, actor, profile, target string) error {
	var visible bool
	err := q.QueryRow(ctx, `SELECT $2=$3 OR $2=$1 OR EXISTS(SELECT 1 FROM public.relationships WHERE owner_fluctlight_id=$1 AND target_actor_id=$2 AND ($4='*' OR profile_id IS NULL OR profile_id=$4))`, owner, target, actor, profile).Scan(&visible)
	if err != nil {
		return err
	}
	if !visible {
		return fmt.Errorf("%w: actor not visible", ErrUnauthorized)
	}
	return nil
}

func readGoalPolicyWith(ctx context.Context, q lifeContextQuerier, owner string) (map[string]any, error) {
	var raw []byte
	err := q.QueryRow(ctx, `SELECT jsonb_build_object('active_count',(SELECT count(*) FROM public.fluctlight_goals WHERE fluctlight_id=$1 AND status='active'),'max_active_goals',COALESCE(p.max_active_goals,5),'collection_version',COALESCE(p.revision,0),'auto_planning_enabled',p.auto_planning_enabled,'ordering_mode',COALESCE(p.ordering_mode,'automatic')) FROM (SELECT $1::text owner) f LEFT JOIN public.goal_set_policies p ON p.fluctlight_id=f.owner`, owner).Scan(&raw)
	if err != nil {
		return nil, err
	}
	return decodeObject(raw), nil
}
