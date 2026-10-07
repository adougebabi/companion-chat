package core

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// ReconcileGoalAttempts uses committed receipts and operation identities. An
// unknown external operation is never reissued just because its deadline passed.
func (a *App) ReconcileGoalAttempts(ctx context.Context, limit int) (int, error) {
	if limit < 1 || limit > 100 {
		limit = 50
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT attempt_id,fluctlight_id FROM public.fluctlight_intention_attempts WHERE status IN ('running','waiting') AND deadline<=$1 ORDER BY deadline,attempt_id LIMIT $2`, a.now().UTC(), limit)
	if err != nil {
		return 0, err
	}
	type candidate struct{ id, owner string }
	items := []candidate{}
	for rows.Next() {
		var c candidate
		if err := rows.Scan(&c.id, &c.owner); err != nil {
			rows.Close()
			return 0, err
		}
		items = append(items, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	count := 0
	for _, c := range items {
		var inboxID string
		var committed agentCommittedOutcome
		waiting := false
		err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
			if err := lockLifeContextTx(ctx, tx, c.owner); err != nil {
				return err
			}
			var state, waitRef, actionID string
			var checks int
			var raw []byte
			var deadline *time.Time
			if err := tx.QueryRow(ctx, `SELECT status,COALESCE(wait_ref,''),result,deadline,action_id,reconciliation_count FROM public.fluctlight_intention_attempts WHERE attempt_id=$1 FOR UPDATE`, c.id).Scan(&state, &waitRef, &raw, &deadline, &actionID, &checks); err != nil {
				return err
			}
			if (state != "running" && state != "waiting") || deadline == nil || deadline.After(a.now()) {
				waiting = true
				return nil
			}
			frozen := decodeObject(raw)
			inboxID = stringValue(frozen["inbox_id"])
			if waitRef != "" || inboxID == "" {
				waiting = true
				if err := a.reconcileKnownAttemptOperationsTx(ctx, tx, c.owner, actionID); err != nil {
					return err
				}
				if err := tx.QueryRow(ctx, `SELECT status FROM public.fluctlight_intention_attempts WHERE attempt_id=$1`, c.id).Scan(&state); err != nil {
					return err
				}
				if state != "running" && state != "waiting" {
					return nil
				}
				checks++
				if checks >= 6 {
					if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_intention_attempts SET status='needs_reconciliation',deadline=NULL,reconciliation_count=$2,result=jsonb_set(result,'{recovery_reason}','"operation_state_requires_reconciliation"'::jsonb,true) WHERE attempt_id=$1`, c.id, checks); err != nil {
						return err
					}
					var intentionID string
					if err := tx.QueryRow(ctx, `SELECT COALESCE(intention_id,'') FROM public.fluctlight_intention_attempts WHERE attempt_id=$1`, c.id).Scan(&intentionID); err != nil {
						return err
					}
					if intentionID != "" {
						current, err := loadIntentionAuthorityByIDTx(ctx, tx, intentionID)
						if err != nil {
							return err
						}
						if current.Status == IntentionDue || current.Status == IntentionInProgress || current.Status == IntentionQualified {
							next, record, err := ApplyIntentionCommand(current, IntentionCommand{Operation: IntentionPause, ExpectedRevision: current.Revision, EvidenceRefs: current.EvidenceRefs, Reason: "operation_state_requires_reconciliation", OccurredAt: a.now().UTC()})
							if err != nil {
								return err
							}
							if _, err := persistIntentionAuthorityTx(ctx, tx, &current, next, record, "reconciliation-hold:"+c.id); err != nil {
								return err
							}
						}
					}
					return nil
				}
				_, err := tx.Exec(ctx, `UPDATE public.fluctlight_intention_attempts SET status='waiting',deadline=$2,reconciliation_count=$3,result=jsonb_set(result,'{recovery_reason}','"operation_state_requires_reconciliation"'::jsonb,true) WHERE attempt_id=$1`, c.id, a.now().Add(30*time.Minute), checks)
				return err
			}
			// Every mutation receipt and effect commits together. These are actual
			// persisted invocations, not reconstructed model ToolCalls.
			receipts, err := tx.Query(ctx, `SELECT invocation,result FROM public.tool_executions WHERE fluctlight_id=$1 AND invocation->>'source_fact_id'=$2 ORDER BY committed_at,operation_id`, c.owner, inboxID)
			if err != nil {
				return err
			}
			defer receipts.Close()
			for receipts.Next() {
				var invocationRaw, resultRaw []byte
				if err := receipts.Scan(&invocationRaw, &resultRaw); err != nil {
					return err
				}
				var invocation CapabilityInvocation
				var result CapabilityResult
				if err := json.Unmarshal(invocationRaw, &invocation); err != nil {
					return err
				}
				if err := json.Unmarshal(resultRaw, &result); err != nil {
					return err
				}
				committed.Invocations = append(committed.Invocations, invocation)
				committed.Results = append(committed.Results, result)
			}
			return receipts.Err()
		})
		if err != nil {
			return count, err
		}
		if !waiting {
			if err := a.failAgentTurnAfterRun(ctx, inboxID, committed, "intention_attempt_lease_expired"); err != nil && !errors.Is(err, ErrConflict) {
				return count, err
			}
		}
		count++
	}
	return count, nil
}

func (a *App) reconcileKnownAttemptOperationsTx(ctx context.Context, tx pgx.Tx, owner, actionID string) error {
	rows, err := tx.Query(ctx, `SELECT external_ref,completion_boundary FROM public.cognition_action_outcomes WHERE fluctlight_id=$1 AND action_id=$2 AND external_ref IS NOT NULL AND status IN ('pending','unknown') ORDER BY call_id LIMIT 64`, owner, actionID)
	if err != nil {
		return err
	}
	type operation struct{ id, boundary string }
	ops := []operation{}
	for rows.Next() {
		var op operation
		if err := rows.Scan(&op.id, &op.boundary); err != nil {
			rows.Close()
			return err
		}
		ops = append(ops, op)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, op := range ops {
		status, assetID, eventID := "", "", ""
		observed := map[string]any{}
		switch op.boundary {
		case "final_media_asset_ready", "candidate_asset_ready":
			err = tx.QueryRow(ctx, `SELECT i.status,COALESCE((SELECT a.id FROM public.media_assets a WHERE a.provider_request_id=i.provider_request_id AND a.owner_fluctlight_id=i.owner_fluctlight_id AND a.status='ready' ORDER BY a.ready_at DESC,a.id DESC LIMIT 1),'') FROM public.media_intents i WHERE i.id=$1 AND i.owner_fluctlight_id=$2`, op.id, owner).Scan(&status, &assetID)
			if status == "completed" && assetID == "" {
				status = "unknown"
			}
			observed = map[string]any{"media_intent_id": op.id, "asset_id": assetID, "delivery_status": "asset_ready"}
		case "visual_identity_ready":
			err = tx.QueryRow(ctx, `SELECT s.status,COALESCE(v.character_sheet_asset_id,'') FROM public.fluctlight_visual_identity_sessions s JOIN public.fluctlight_visual_identities v ON v.id=s.visual_identity_id WHERE s.id=$1 AND s.fluctlight_id=$2`, op.id, owner).Scan(&status, &assetID)
			var ready bool
			if status == "completed" {
				if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.media_assets WHERE id=$1 AND owner_fluctlight_id=$2 AND status='ready')`, assetID, owner).Scan(&ready); err != nil {
					return err
				}
				if !ready {
					status = "unknown"
				}
			}
			observed = map[string]any{"session_id": op.id, "asset_id": assetID, "delivery_status": "visual_identity_ready"}
		case "virtual_activity_resolved":
			var raw []byte
			err = tx.QueryRow(ctx, `SELECT status,result_json FROM public.fluctlight_life_activity_runs WHERE id=$1 AND fluctlight_id=$2`, op.id, owner).Scan(&status, &raw)
			eventID = stringValue(decodeObject(raw)["event_id"])
			if status == "completed" && eventID == "" {
				status = "unknown"
			}
			observed = map[string]any{"event_id": eventID, "resulting_state_ref": eventID}
		default:
			continue
		}
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			return err
		}
		terminal := ActionOutcomeStatus("")
		code := ""
		switch status {
		case "completed":
			terminal = ActionOutcomeCompleted
		case "failed":
			terminal = ActionOutcomeFailed
			code = "async_operation_failed"
		case "cancelled":
			terminal = ActionOutcomeCancelled
			code = "async_operation_cancelled"
		}
		if terminal != "" && terminal != ActionOutcomeCompleted {
			observed["delivery_status"] = status
			delete(observed, "asset_id")
		}
		if terminal != "" {
			if _, err := a.settleActionOutcomeByExternalRefTx(ctx, tx, op.id, terminal, observed, code); err != nil {
				return err
			}
		}
	}
	return nil
}
