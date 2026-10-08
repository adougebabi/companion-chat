package core

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

type GoalReconciliationItem struct {
	GoalID              string   `json:"goal_id"`
	Revision            int      `json:"revision"`
	Status              string   `json:"status"`
	ProfileID           string   `json:"profile_id"`
	Reasons             []string `json:"reasons"`
	PendingSources      int      `json:"pending_sources"`
	UnsettledAttempts   int      `json:"unsettled_attempts"`
	SourceWatermark     int64    `json:"source_watermark"`
	EvaluationRequestID string   `json:"evaluation_request_id,omitempty"`
}
type GoalReconciliationReport struct {
	FluctlightID string                   `json:"fluctlight_id"`
	Items        []GoalReconciliationItem `json:"items"`
	NextCursor   string                   `json:"next_cursor,omitempty"`
	Digest       string                   `json:"digest"`
	Applied      bool                     `json:"applied"`
}

// ReconcileGoalStock inspects an explicit Owner scope in bounded ID batches.
// Apply requires its reviewed digest and never retries a physical operation.
// Existing facts, lifecycle, wording and completed history stay authoritative.
func (a *App) ReconcileGoalStock(ctx context.Context, actorID, owner, cursor string, limit int, apply bool, expectedDigest string) (GoalReconciliationReport, error) {
	report := GoalReconciliationReport{FluctlightID: owner, Items: []GoalReconciliationItem{}}
	if _, err := a.DB.GetFluctlight(ctx, owner, actorID); err != nil {
		return report, err
	}
	if limit < 1 || limit > 100 || len(cursor) > 128 {
		return report, ErrInvalidArguments
	}
	inspect := func(q DBTX) (GoalReconciliationReport, error) {
		page := report
		rows, err := q.Query(ctx, `SELECT g.id,g.revision,g.status,COALESCE(g.profile_id,''),
  EXISTS(SELECT 1 FROM public.goal_resolutions r WHERE r.goal_id=g.id),
  EXISTS(SELECT 1 FROM public.goal_migration_review_flags f WHERE f.goal_id=g.id),
  EXISTS(SELECT 1 FROM public.fluctlight_intentions i WHERE i.goal_id=g.id AND i.status IN ('qualified','due','in_progress') AND i.expiration>$4),
  COALESCE(g.current_stage_id,''),
  (SELECT count(*) FROM public.goal_source_events e WHERE e.fluctlight_id=g.fluctlight_id AND e.processed_at IS NULL AND (e.profile_id IS NULL OR g.profile_id IS NULL OR e.profile_id=g.profile_id)),
  (SELECT count(*) FROM public.fluctlight_intention_attempts a WHERE a.fluctlight_id=g.fluctlight_id AND a.goal_ref='goal:ctx_'||substr(encode(digest(g.id,'sha256'),'hex'),1,32) AND a.status IN ('running','waiting','needs_reconciliation')),
  (SELECT COALESCE(max(id),0) FROM public.goal_source_events e WHERE e.fluctlight_id=g.fluctlight_id)
  FROM public.fluctlight_goals g WHERE g.fluctlight_id=$1 AND g.id>$2 ORDER BY g.id LIMIT $3`, owner, cursor, limit+1, a.now())
		if err != nil {
			return page, err
		}
		defer rows.Close()
		for rows.Next() {
			var item GoalReconciliationItem
			var resolution, flagged, scheduled bool
			var stage string
			if err := rows.Scan(&item.GoalID, &item.Revision, &item.Status, &item.ProfileID, &resolution, &flagged, &scheduled, &stage, &item.PendingSources, &item.UnsettledAttempts, &item.SourceWatermark); err != nil {
				return page, err
			}
			if len(page.Items) == limit {
				page.NextCursor = page.Items[len(page.Items)-1].GoalID
				break
			}
			item.Reasons = []string{}
			if item.Status == "completed" && !resolution && !flagged {
				item.Reasons = append(item.Reasons, "legacy_completion_evidence_requires_review")
			}
			if item.Status == "active" && stage == "" && !scheduled {
				item.Reasons = append(item.Reasons, "focused_planning_or_wait_required")
			}
			if (item.Status == "active" || item.Status == "paused") && item.PendingSources > 0 {
				item.Reasons = append(item.Reasons, "unprocessed_actual_sources")
			}
			if item.UnsettledAttempts > 0 {
				item.Reasons = append(item.Reasons, "operation_reconciliation_required_no_reissue")
			}
			page.Items = append(page.Items, item)
		}
		if err := rows.Err(); err != nil {
			return page, err
		}
		page.Digest = stableDigest(jsonString(map[string]any{"actor": actorID, "owner": owner, "cursor": cursor, "limit": limit, "items": page.Items}))
		return page, nil
	}
	if !apply {
		return inspect(a.DB.Pool())
	}
	if strings.TrimSpace(expectedDigest) == "" {
		return report, errors.New("goal_reconciliation_reviewed_digest_required")
	}
	batchID := "goal_reconciliation_" + stableDigest(actorID+"\x1f"+owner+"\x1f"+expectedDigest)
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if err := lockLifeContextTx(ctx, tx, owner); err != nil {
			return err
		}
		var old []byte
		if err := tx.QueryRow(ctx, `SELECT result FROM public.goal_reconciliation_batches WHERE id=$1 AND actor_id=$2 AND fluctlight_id=$3`, batchID, actorID, owner).Scan(&old); err == nil {
			if err := decodeStructuredValue(decodeObject(old), &report); err != nil {
				return err
			}
			return nil
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		current, err := inspect(tx)
		if err != nil {
			return err
		}
		if current.Digest != expectedDigest {
			return ErrConflict
		}
		report = current
		for index, item := range report.Items {
			if containsString(item.Reasons, "legacy_completion_evidence_requires_review") {
				if _, err := tx.Exec(ctx, `INSERT INTO public.goal_migration_review_flags(goal_id,fluctlight_id,goal_revision,reason,snapshot) SELECT id,fluctlight_id,revision,'legacy completion evidence not revalidated; retain history',to_jsonb(g) FROM public.fluctlight_goals g WHERE fluctlight_id=$1 AND id=$2 AND revision=$3 AND status='completed' ON CONFLICT(goal_id) DO NOTHING`, owner, item.GoalID, item.Revision); err != nil {
					return err
				}
			}
			if (item.Status == "active" || item.Status == "paused") && (containsString(item.Reasons, "focused_planning_or_wait_required") || containsString(item.Reasons, "unprocessed_actual_sources")) {
				// Queue assessment only, not a purchase/send/action retry or forced Stage.
				id, err := queueGoalEvaluationTx(ctx, tx, owner, item.ProfileID, "owner_stock_reconciliation", batchID+":"+item.GoalID, []string{item.GoalID})
				if err != nil {
					return err
				}
				report.Items[index].EvaluationRequestID = id
			}
		}
		report.Applied = true
		_, err = tx.Exec(ctx, `INSERT INTO public.goal_reconciliation_batches(id,actor_id,fluctlight_id,request_digest,result) VALUES($1,$2,$3,$4,$5)`, batchID, actorID, owner, expectedDigest, jsonBytes(report))
		return err
	})
	return report, err
}
