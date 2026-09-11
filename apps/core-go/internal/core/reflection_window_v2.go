package core

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

func (a *App) claimReflectionWindow(ctx context.Context, fluctlightID string, watermark, stateRevision int) error {
	return withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var status string
		var updatedAt time.Time
		err := tx.QueryRow(ctx, `SELECT status,updated_at FROM public.cognition_reflection_windows WHERE fluctlight_id=$1 FOR UPDATE`, fluctlightID).Scan(&status, &updatedAt)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if err == nil && status == "running" && time.Since(updatedAt) < 15*time.Minute {
			return ErrConflict
		}
		if errors.Is(err, pgx.ErrNoRows) {
			_, err = tx.Exec(ctx, `INSERT INTO public.cognition_reflection_windows(fluctlight_id,watermark,state_revision,status) VALUES($1,$2,$3,'running')`, fluctlightID, watermark, stateRevision)
			return err
		}
		_, err = tx.Exec(ctx, `UPDATE public.cognition_reflection_windows SET status='running',state_revision=$2,updated_at=now() WHERE fluctlight_id=$1`, fluctlightID, stateRevision)
		return err
	})
}

func (a *App) setReflectionWindowIdle(ctx context.Context, fluctlightID string) error {
	_, err := a.DB.Pool().Exec(ctx, `UPDATE public.cognition_reflection_windows SET status='idle',updated_at=now() WHERE fluctlight_id=$1 AND status='running'`, fluctlightID)
	return err
}
