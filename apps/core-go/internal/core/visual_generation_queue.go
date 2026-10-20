package core

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

func visualGenerationReceipt(fid, requestID, sessionID, status string, briefRevision int, audit string) map[string]any {
	request := map[string]any{"id": requestID, "status": status}
	result := map[string]any{
		"fluctlight_id":  fid,
		"request_id":     requestID,
		"request":        request,
		"status":         status,
		"brief_revision": briefRevision,
		"audit_id":       audit,
	}
	if sessionID != "" {
		request["session_id"] = sessionID
		result["session_id"] = sessionID
		if status == "queued" {
			result["task_id"] = "visual_identity_intent:" + sessionID
		}
	}
	return result
}

func cancelInFlightVisualSessionsTx(ctx context.Context, tx pgx.Tx, fluctlightID, reason string) error {
	_, err := tx.Exec(ctx, `UPDATE public.fluctlight_visual_identity_sessions SET status='cancelled',last_error=$2,updated_at=now() WHERE fluctlight_id=$1 AND status IN ('queued','running','character_sheet_pending')`, fluctlightID, reason)
	return err
}

func readVisualGenerationRequestTx(ctx context.Context, tx pgx.Tx, fid, key string) (map[string]any, bool, error) {
	var id, status, sessionID, audit string
	var briefRevision int
	err := tx.QueryRow(ctx, `SELECT id,status,COALESCE(session_id,''),COALESCE(audit_id,''),expected_brief_revision FROM public.fluctlight_visual_generation_requests WHERE fluctlight_id=$1 AND idempotency_key=$2`, fid, key).Scan(&id, &status, &sessionID, &audit, &briefRevision)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return visualGenerationReceipt(fid, id, sessionID, status, briefRevision, audit), true, nil
}

// releaseVisualGenerationQueueTx starts the oldest pending owner request once
// no visual session is still queued, running, waiting on a character sheet, or
// awaiting review. A paused or retired character keeps the queue.
func (a *App) releaseVisualGenerationQueueTx(ctx context.Context, tx pgx.Tx, fluctlightID string) error {
	blocked, _, blockErr := autonomousActivityBlockedTx(ctx, tx, fluctlightID)
	if blockErr != nil || blocked {
		return blockErr
	}
	var occupied int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM public.fluctlight_visual_identity_sessions WHERE fluctlight_id=$1 AND status IN ('queued','running','character_sheet_pending','awaiting_review')`, fluctlightID).Scan(&occupied); err != nil {
		return err
	}
	if occupied > 0 {
		return nil
	}
	var requestID, sessionID string
	err := tx.QueryRow(ctx, `SELECT id,COALESCE(session_id,'') FROM public.fluctlight_visual_generation_requests WHERE fluctlight_id=$1 AND status='pending' ORDER BY queue_position,id LIMIT 1 FOR UPDATE`, fluctlightID).Scan(&requestID, &sessionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if sessionID == "" {
		return errors.New("visual_generation_request_session_missing")
	}
	promoted, err := tx.Exec(ctx, `UPDATE public.fluctlight_visual_identity_sessions SET status='queued',updated_at=now() WHERE id=$1 AND fluctlight_id=$2 AND status='pending'`, sessionID, fluctlightID)
	if err != nil {
		return err
	}
	if promoted.RowsAffected() != 1 {
		return errors.New("visual_generation_request_not_started")
	}
	workflowID := visualIdentityWorkflowID(sessionID)
	if _, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload) VALUES($1,$2,'lifecycle','visual_identity.initialize',$3) ON CONFLICT DO NOTHING`, "visual_identity_intent:"+sessionID, workflowID, jsonBytes(map[string]any{"intent_id": "visual_identity_intent:" + sessionID, "fluctlight_id": fluctlightID, "session_id": sessionID, "correlation_id": "visual_identity:" + sessionID, "causation_id": requestID})); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_visual_identities SET active_session_id=$2,updated_at=now() WHERE fluctlight_id=$1`, fluctlightID, sessionID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_visual_identities SET status='active' WHERE fluctlight_id=$1 AND canonical_asset_id IS NOT NULL AND character_sheet_asset_id IS NOT NULL`, fluctlightID); err != nil {
		return err
	}
	updated, err := tx.Exec(ctx, `UPDATE public.fluctlight_visual_generation_requests SET status='queued',updated_at=now() WHERE id=$1 AND status='pending'`, requestID)
	if err != nil {
		return err
	}
	if updated.RowsAffected() != 1 {
		return errors.New("visual_generation_request_not_started")
	}
	return nil
}

// ReleaseVisualGenerationQueueTx starts the oldest pending owner request in the
// caller's transaction. A paused or retired character leaves the queue pending.
func (a *App) ReleaseVisualGenerationQueueTx(ctx context.Context, tx pgx.Tx, fluctlightID string) error {
	if a == nil {
		return nil
	}
	return a.releaseVisualGenerationQueueTx(ctx, tx, fluctlightID)
}

func (a *App) ReleaseVisualGenerationQueue(ctx context.Context, fluctlightID string) error {
	if a == nil || a.DB == nil || a.DB.Pool() == nil {
		return nil
	}
	return withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		return a.releaseVisualGenerationQueueTx(ctx, tx, fluctlightID)
	})
}

func (a *App) RepairVisualGenerationQueue(ctx context.Context, limit int) error {
	if a == nil || a.DB == nil || a.DB.Pool() == nil {
		return nil
	}
	if limit < 1 {
		limit = 1
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT DISTINCT r.fluctlight_id FROM public.fluctlight_visual_generation_requests r JOIN public.fluctlights f ON f.id=r.fluctlight_id AND f.status='active' WHERE r.status='pending' AND NOT EXISTS (SELECT 1 FROM public.fluctlight_visual_identity_sessions s WHERE s.fluctlight_id=r.fluctlight_id AND s.status IN ('queued','running','character_sheet_pending','awaiting_review')) ORDER BY r.fluctlight_id LIMIT $1`, limit)
	if err != nil {
		return err
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, id := range ids {
		if err := a.ReleaseVisualGenerationQueue(ctx, id); err != nil {
			return err
		}
	}
	return nil
}
