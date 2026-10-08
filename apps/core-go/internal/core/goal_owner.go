package core

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type GoalOwnerCommand struct {
	ReviewPolicy        *GoalReviewPolicy `json:"review_policy,omitempty"`
	ResetReviewCounters bool              `json:"reset_review_counters"`
	Operation           string            `json:"operation"`
	ExpectedRevision    int               `json:"expected_revision"`
	IdempotencyKey      string            `json:"idempotency_key"`
	Reason              string            `json:"reason"`
	ProfileID           string            `json:"profile_id"`
	DesiredOutcome      *string           `json:"desired_outcome,omitempty"`
	SuccessCriteria     []string          `json:"success_criteria,omitempty"`
	Motivation          *string           `json:"motivation,omitempty"`
	Scope               *string           `json:"scope,omitempty"`
	Deadline            *time.Time        `json:"deadline,omitempty"`
	ClearDeadline       bool              `json:"clear_deadline"`
	DeadlinePolicy      *string           `json:"deadline_policy,omitempty"`
}

type GoalPage struct {
	Items      []map[string]any `json:"items"`
	NextCursor string           `json:"next_cursor,omitempty"`
}

type goalPageCursor struct {
	Owner   string    `json:"owner"`
	History bool      `json:"history"`
	At      time.Time `json:"at"`
	ID      string    `json:"id"`
}

func (a *App) ListGoals(ctx context.Context, actorID, owner string, history bool, limit int, cursor string) (GoalPage, error) {
	page := GoalPage{Items: []map[string]any{}}
	if _, err := a.DB.GetFluctlight(ctx, owner, actorID); err != nil {
		return page, err
	}
	if limit < 1 || limit > 50 {
		return page, fmt.Errorf("%w: goal page limit must be 1..50", ErrInvalidArguments)
	}
	var before *time.Time
	beforeID := ""
	if cursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(cursor)
		if err != nil {
			return page, ErrInvalidArguments
		}
		var c goalPageCursor
		if json.Unmarshal(raw, &c) != nil || c.Owner != owner || c.History != history || c.ID == "" || c.At.IsZero() {
			return page, ErrInvalidArguments
		}
		before, beforeID = &c.At, c.ID
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT to_jsonb(g)-'request_digest'-'idempotency_key',updated_at,id FROM public.fluctlight_goals g WHERE fluctlight_id=$1 AND ((status IN ('completed','cancelled','abandoned'))=$2) AND ($3::timestamptz IS NULL OR (updated_at,id)<($3,$4)) ORDER BY updated_at DESC,id DESC LIMIT $5`, owner, history, before, beforeID, limit+1)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	var lastAt time.Time
	lastID := ""
	for rows.Next() {
		var raw []byte
		var at time.Time
		var id string
		if err := rows.Scan(&raw, &at, &id); err != nil {
			return page, err
		}
		if len(page.Items) == limit {
			page.NextCursor = base64.RawURLEncoding.EncodeToString(jsonBytes(goalPageCursor{owner, history, lastAt, lastID}))
			break
		}
		page.Items = append(page.Items, decodeObject(raw))
		lastAt, lastID = at, id
	}
	return page, rows.Err()
}

func (a *App) GoalDetail(ctx context.Context, actorID, owner, goalID string) (map[string]any, error) {
	if _, err := a.DB.GetFluctlight(ctx, owner, actorID); err != nil {
		return nil, err
	}
	var raw []byte
	if err := a.DB.Pool().QueryRow(ctx, `SELECT to_jsonb(g)-'request_digest'-'idempotency_key' FROM public.fluctlight_goals g WHERE fluctlight_id=$1 AND id=$2`, owner, goalID).Scan(&raw); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	result := decodeObject(raw)
	execution, err := readGoalExecutionStateWith(ctx, a.DB.Pool(), owner, goalID, stringValue(result["status"]), a.now())
	if err != nil {
		return nil, err
	}
	result["execution"] = execution
	// Details are bounded. Older records are requested via the history endpoint.
	for key, query := range map[string]string{
		"stages":                 `SELECT to_jsonb(s) FROM public.goal_stages s WHERE fluctlight_id=$1 AND goal_id=$2 ORDER BY updated_at DESC,id DESC LIMIT 20`,
		"commitments":            `SELECT to_jsonb(c) FROM public.goal_commitments c WHERE fluctlight_id=$1 AND goal_id=$2 ORDER BY updated_at DESC,id DESC LIMIT 20`,
		"evaluations":            `SELECT to_jsonb(e) FROM public.goal_evaluations e WHERE fluctlight_id=$1 AND goal_id=$2 ORDER BY created_at DESC,id DESC LIMIT 20`,
		"migration_review_flags": `SELECT to_jsonb(r) FROM public.goal_migration_review_flags r WHERE fluctlight_id=$1 AND goal_id=$2`,
		"reviews":                `SELECT to_jsonb(r) FROM public.goal_reviews r WHERE fluctlight_id=$1 AND goal_id=$2 ORDER BY window_end DESC,id DESC LIMIT 20`,
		"review_revisions":       `SELECT to_jsonb(h) FROM public.goal_review_revisions h JOIN public.goal_reviews r ON r.id=h.review_id WHERE r.fluctlight_id=$1 AND r.goal_id=$2 ORDER BY h.created_at DESC,h.id DESC LIMIT 20`,
		"resolutions":            `SELECT to_jsonb(r) FROM public.goal_resolutions r WHERE fluctlight_id=$1 AND goal_id=$2 ORDER BY created_at DESC LIMIT 1`,
	} {
		rows, err := a.DB.Pool().Query(ctx, query, owner, goalID)
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
		result[key] = items
	}
	evidence, err := a.GoalEvidence(ctx, actorID, owner, goalID, 20, "")
	if err != nil {
		return nil, err
	}
	result["evidence"], result["evidence_next_cursor"] = evidence.Items, evidence.NextCursor
	return result, nil
}

func (a *App) ApplyOwnerGoalCommand(ctx context.Context, actorID, owner, goalID string, command GoalOwnerCommand) (map[string]any, error) {
	if _, err := a.DB.GetFluctlight(ctx, owner, actorID); err != nil {
		return nil, err
	}
	if strings.TrimSpace(command.Reason) == "" || len([]rune(command.Reason)) > 1000 || strings.TrimSpace(command.IdempotencyKey) == "" || len(command.IdempotencyKey) > 128 {
		return nil, ErrInvalidArguments
	}
	digest := stableDigest(jsonString(map[string]any{"goal_id": goalID, "command": command}))
	var result map[string]any
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if err := lockLifeContextTx(ctx, tx, owner); err != nil {
			return err
		}
		var priorDigest string
		var prior []byte
		err := tx.QueryRow(ctx, `SELECT request_digest,result FROM public.goal_owner_commands WHERE fluctlight_id=$1 AND actor_id=$2 AND idempotency_key=$3`, owner, actorID, command.IdempotencyKey).Scan(&priorDigest, &prior)
		if err == nil {
			if priorDigest != digest {
				return ErrConflict
			}
			result = decodeObject(prior)
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		refs := []string{"owner:" + actorID}
		key := "owner-goal:" + stableDigest(owner+"\x1f"+actorID+"\x1f"+command.IdempotencyKey)
		var goal GoalAuthority
		var record GoalGovernanceRecord
		var before *GoalAuthority
		if command.Operation == "create" {
			if goalID != "" || command.ExpectedRevision != 0 || command.DesiredOutcome == nil || command.Motivation == nil {
				return ErrInvalidArguments
			}
			if command.ProfileID != "" {
				var persona []byte
				if err := tx.QueryRow(ctx, `SELECT core_persona FROM public.fluctlights WHERE id=$1`, owner).Scan(&persona); err != nil {
					return err
				}
				if _, ok := personalityProfileIDs(decodeObject(persona))[command.ProfileID]; !ok {
					return errors.New("goal_profile_invalid")
				}
			}
			goalID = "goal_owner_" + stableDigest(key)
			scope := "general"
			if command.Scope != nil {
				scope = *command.Scope
			}
			deadlinePolicy := "soft"
			if command.DeadlinePolicy != nil {
				deadlinePolicy = *command.DeadlinePolicy
			}
			target := ""
			if scope == "relationship" {
				target = actorID
			}
			goal, record, err = CreateGoalAuthority(GoalAuthority{EntityID: goalID, SchemaVersion: goalAuthoritySchemaVersion, Ref: "goal:ctx_" + stableDigest(goalID), FluctlightID: owner, ProfileID: command.ProfileID, DesiredOutcome: *command.DesiredOutcome, SuccessCriteria: command.SuccessCriteria, Motivation: *command.Motivation, Scope: scope, TargetActorID: target, Deadline: command.Deadline, DeadlinePolicy: deadlinePolicy, ReviewPolicy: command.ReviewPolicy, Status: GoalActive, Revision: 1}, refs, a.now())
		} else {
			var revision int
			if err := tx.QueryRow(ctx, `SELECT revision FROM public.fluctlight_goals WHERE fluctlight_id=$1 AND id=$2`, owner, goalID).Scan(&revision); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return ErrNotFound
				}
				return err
			}
			if revision != command.ExpectedRevision {
				return ErrConflict
			}
			goal, err = loadGoalAuthorityTx(ctx, tx, owner, "goal:ctx_"+stableDigest(goalID), ContextReference{EntityID: goalID, Revision: revision})
			if err != nil {
				return err
			}
			if command.ProfileID != "" && command.ProfileID != goal.ProfileID {
				return ErrInvalidArguments
			}
			if command.Scope != nil && *command.Scope != goal.Scope {
				return errors.New("goal_scope_change_requires_new_goal")
			}

			if command.Operation == "reassess" {
				if goal.Status != GoalActive && goal.Status != GoalPaused {
					return errors.New("goal_evaluation_terminal")
				}
				requestID, err := queueGoalEvaluationTx(ctx, tx, owner, goal.ProfileID, "owner_reassess", key, []string{goalID})
				if err != nil {
					return err
				}
				result = map[string]any{"goal_id": goalID, "revision": goal.Revision, "evaluation_request_id": requestID, "status": "pending"}
			} else {
				if command.ResetReviewCounters {
					policy := effectiveGoalReviewPolicy(goal.ReviewPolicy)
					if command.ReviewPolicy != nil {
						policy = *command.ReviewPolicy
					}
					at := a.now()
					policy.ResetAfter = &at
					command.ReviewPolicy = &policy
				}
				current := goal
				before = &current
				goal, record, err = ApplyGoalCommand(&current, GoalCommand{Operation: GoalLifecycleOperation(command.Operation), ExpectedRevision: command.ExpectedRevision, Patch: GoalPatch{ReviewPolicy: command.ReviewPolicy, DesiredOutcome: command.DesiredOutcome, SuccessCriteria: command.SuccessCriteria, Motivation: command.Motivation, Scope: command.Scope, Deadline: command.Deadline, ClearDeadline: command.ClearDeadline, DeadlinePolicy: command.DeadlinePolicy}, EvidenceRefs: refs, Reason: command.Reason, OccurredAt: a.now()})
			}
		}
		if err != nil {
			return err
		}
		if result == nil {
			record.ActorID, record.Source, record.Reason = actorID, "owner", command.Reason
			if _, err := persistGoalAuthorityTx(ctx, tx, before, goal, record, key); err != nil {
				return err
			}
			result = map[string]any{"goal_id": goal.EntityID, "revision": goal.Revision, "status": goal.Status, "criteria_version": goal.CriteriaVersion}
		}
		_, err = tx.Exec(ctx, `INSERT INTO public.goal_owner_commands(fluctlight_id,actor_id,idempotency_key,request_digest,result) VALUES($1,$2,$3,$4,$5)`, owner, actorID, command.IdempotencyKey, digest, jsonBytes(result))
		return err
	})
	return result, err
}

func (a *App) GoalHistory(ctx context.Context, actorID, owner, goalID string, limit int, cursor string) (GoalPage, error) {
	page := GoalPage{Items: []map[string]any{}}
	if _, err := a.DB.GetFluctlight(ctx, owner, actorID); err != nil {
		return page, err
	}
	if limit < 1 || limit > 50 {
		return page, ErrInvalidArguments
	}
	var found bool
	if err := a.DB.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.fluctlight_goals WHERE fluctlight_id=$1 AND id=$2)`, owner, goalID).Scan(&found); err != nil {
		return page, err
	}
	if !found {
		return page, ErrNotFound
	}
	var before *time.Time
	beforeID := ""
	scope := owner + "/" + goalID
	if cursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(cursor)
		if err != nil {
			return page, ErrInvalidArguments
		}
		var c goalPageCursor
		if json.Unmarshal(raw, &c) != nil || c.Owner != scope || !c.History || c.ID == "" || c.At.IsZero() {
			return page, ErrInvalidArguments
		}
		before, beforeID = &c.At, c.ID
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT to_jsonb(r)-'request_digest'-'idempotency_key',created_at,id FROM public.fluctlight_goal_revisions r WHERE fluctlight_id=$1 AND goal_id=$2 AND ($3::timestamptz IS NULL OR (created_at,id)<($3,$4)) ORDER BY created_at DESC,id DESC LIMIT $5`, owner, goalID, before, beforeID, limit+1)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	var lastAt time.Time
	lastID := ""
	for rows.Next() {
		var raw []byte
		var at time.Time
		var id string
		if err := rows.Scan(&raw, &at, &id); err != nil {
			return page, err
		}
		if len(page.Items) == limit {
			page.NextCursor = base64.RawURLEncoding.EncodeToString(jsonBytes(goalPageCursor{scope, true, lastAt, lastID}))
			break
		}
		page.Items = append(page.Items, decodeObject(raw))
		lastAt, lastID = at, id
	}
	return page, rows.Err()
}

func (a *App) GoalEvidence(ctx context.Context, actorID, owner, goalID string, limit int, cursor string) (GoalPage, error) {
	page := GoalPage{Items: []map[string]any{}}
	if _, err := a.DB.GetFluctlight(ctx, owner, actorID); err != nil {
		return page, err
	}
	if limit < 1 || limit > 50 {
		return page, ErrInvalidArguments
	}
	var found bool
	if err := a.DB.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.fluctlight_goals WHERE fluctlight_id=$1 AND id=$2)`, owner, goalID).Scan(&found); err != nil {
		return page, err
	}
	if !found {
		return page, ErrNotFound
	}
	var before *time.Time
	beforeID := ""
	scope := owner + "/" + goalID + "/evidence"
	if cursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(cursor)
		if err != nil {
			return page, ErrInvalidArguments
		}
		var c goalPageCursor
		if json.Unmarshal(raw, &c) != nil || c.Owner != scope || !c.History || c.ID == "" || c.At.IsZero() {
			return page, ErrInvalidArguments
		}
		before, beforeID = &c.At, c.ID
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT to_jsonb(l)||jsonb_build_object('source',to_jsonb(e)),e.recorded_at,l.id FROM public.goal_evidence_links l JOIN public.goal_source_events e ON e.id=l.source_event_id AND e.fluctlight_id=l.fluctlight_id WHERE l.fluctlight_id=$1 AND l.goal_id=$2 AND ($3::timestamptz IS NULL OR (e.recorded_at,l.id)<($3,$4)) ORDER BY e.recorded_at DESC,l.id DESC LIMIT $5`, owner, goalID, before, beforeID, limit+1)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	var lastAt time.Time
	lastID := ""
	for rows.Next() {
		var raw []byte
		var at time.Time
		var id string
		if err := rows.Scan(&raw, &at, &id); err != nil {
			return page, err
		}
		if len(page.Items) == limit {
			page.NextCursor = base64.RawURLEncoding.EncodeToString(jsonBytes(goalPageCursor{scope, true, lastAt, lastID}))
			break
		}
		page.Items = append(page.Items, decodeObject(raw))
		lastAt, lastID = at, id
	}
	return page, rows.Err()
}
