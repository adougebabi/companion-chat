package core

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Elapsed time is a clock fact, never fabricated execution/failure evidence.
// The existing Goal evaluator may still fulfill an expired Commitment from
// valid evidence whose actual occurrence was inside the agreed window.
func (a *App) RepairOwnerPromiseDeadlines(ctx context.Context, limit int) (int, error) {
	if limit < 1 || limit > 100 {
		return 0, ErrInvalidArguments
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT p.request_id,p.owner_actor_id,p.fluctlight_id FROM public.owner_promise_links p JOIN public.goal_commitments c ON c.id=p.commitment_id WHERE p.cancelled_at IS NULL AND p.window_end<=$1 AND c.status IN ('active','blocked','paused','expired') AND NOT EXISTS(SELECT 1 FROM public.owner_promise_history h WHERE h.request_id=p.request_id AND h.operation='window_elapsed' AND h.after_value->>'window_end'=to_jsonb(p)->>'window_end') ORDER BY p.window_end,p.request_id LIMIT $2`, a.now().UTC(), limit)
	if err != nil {
		return 0, err
	}
	type due struct{ id, owner, fid string }
	items := []due{}
	for rows.Next() {
		var p due
		if err := rows.Scan(&p.id, &p.owner, &p.fid); err != nil {
			rows.Close()
			return 0, err
		}
		items = append(items, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	count := 0
	for _, item := range items {
		err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
			if err := lockLifeContextTx(ctx, tx, item.fid); err != nil {
				return err
			}
			p, err := readPromiseLinkTx(ctx, tx, item.owner, item.fid, item.id, true)
			if err != nil {
				return err
			}
			var eligible bool
			if err := tx.QueryRow(ctx, `SELECT p.cancelled_at IS NULL AND p.window_end<=$2 AND c.status IN ('active','blocked','paused','expired') AND NOT EXISTS(SELECT 1 FROM public.owner_promise_history h WHERE h.request_id=p.request_id AND h.operation='window_elapsed' AND h.after_value->>'window_end'=to_jsonb(p)->>'window_end') FROM public.owner_promise_links p JOIN public.goal_commitments c ON c.id=p.commitment_id WHERE p.request_id=$1`, item.id, a.now().UTC()).Scan(&eligible); err != nil {
				return err
			}
			if !eligible {
				return nil
			}
			if _, err := tx.Exec(ctx, `UPDATE public.goal_commitments SET status='expired',revision=revision+1,updated_at=now() WHERE id=$1 AND status IN ('active','blocked')`, p["commitment_id"]); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE public.owner_promise_links SET revision=revision+1,updated_at=now() WHERE request_id=$1`, item.id); err != nil {
				return err
			}
			source := fmt.Sprint("promise-deadline:", item.id, ":", p["revision"])
			if err := appendPromiseHistoryTx(ctx, tx, item.id, "window_elapsed", source, "Clock window elapsed; evidence evaluation remains authoritative", p, a.now()); err != nil {
				return err
			}
			if _, err := a.enqueueNativeFactTx(ctx, tx, item.fid, "", item.id, "life.promise.window_elapsed", source, map[string]any{"promise_request_id": item.id, "goal_id": p["goal_id"], "commitment_id": p["commitment_id"], "window_end": p["window_end"], "fulfillment_unproven": true}); err != nil {
				return err
			}
			count++
			return nil
		})
		if err != nil {
			return count, err
		}
	}
	return count, nil
}
