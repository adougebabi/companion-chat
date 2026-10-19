package core

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// A failed media child is a persisted terminal result. Replaying its parent
// cannot regenerate that child, and must not create an endless five-second
// Workflow loop. A later explicit retry can resume through media recovery.
func (a *App) settleVisualIdentityMediaFailure(ctx context.Context, state visualIdentityAgentState, cause error) (bool, error) {
	if cause == nil || (cause.Error() != "visual_identity_candidate_media_failed" && cause.Error() != "visual_identity_character_sheet_media_failed") {
		return false, nil
	}
	mediaID := state.MediaIntentID
	if cause.Error() == "visual_identity_character_sheet_media_failed" {
		mediaID = state.CharacterMediaIntentID
	}
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var currentAttempt int
		var sessionStatus string
		if err := tx.QueryRow(ctx, `SELECT current_attempt,status FROM public.fluctlight_visual_identity_sessions WHERE id=$1 FOR UPDATE`, state.SessionID).Scan(&currentAttempt, &sessionStatus); err != nil {
			return err
		}
		if sessionStatus == "failed" || sessionStatus == "cancelled" || sessionStatus == "completed" {
			return nil
		}
		if currentAttempt != state.Attempt {
			return ErrConflict
		}
		var mediaStatus string
		if err := tx.QueryRow(ctx, `SELECT status FROM public.media_intents WHERE id=$1 FOR SHARE`, mediaID).Scan(&mediaStatus); err != nil {
			return err
		}
		if mediaStatus != "failed" {
			return ErrConflict
		}
		updated, err := tx.Exec(ctx, `UPDATE public.fluctlight_visual_identity_sessions SET status='failed',last_error=$2,updated_at=now() WHERE id=$1 AND status IN ('queued','running','character_sheet_pending')`, state.SessionID, cause.Error())
		if err != nil {
			return err
		}
		if updated.RowsAffected() == 0 {
			return nil
		}
		if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_visual_identity_attempts SET status='failed',error_code=$2,updated_at=now() WHERE id=$1 AND status NOT IN ('completed','cancelled')`, state.AttemptID, cause.Error()); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_visual_identities SET status='failed',updated_at=now() WHERE id=$1 AND active_session_id=$2 AND status<>'active'`, state.ProfileID, state.SessionID); err != nil {
			return err
		}
		return appendVisualIdentityTimelineTx(ctx, tx, state.SessionID, state.AttemptID, state.FluctlightID, visualIdentityStageFailed, "failed", cause.Error(), nil, map[string]any{"media_intent_id": mediaID}, "visual_identity:"+state.SessionID)
	})
	return err == nil, err
}
