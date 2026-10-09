package core

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
)

// Bounded source linkage only. Semantic ambiguity is retained for review; no
// model calls or lifecycle changes are performed by the data migration.
func (a *App) MigrateGoalPlannerSources(ctx context.Context, actor, owner, after string, limit int, apply bool) (map[string]any, error) {
	if _, err := a.DB.GetFluctlight(ctx, owner, actor); err != nil {
		return nil, err
	}
	if limit < 1 || limit > 100 {
		return nil, ErrInvalidArguments
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT id,status,revision FROM public.fluctlight_goals WHERE fluctlight_id=$1 AND id>$2 ORDER BY id LIMIT $3`, owner, after, limit+1)
	if err != nil {
		return nil, err
	}
	items := []map[string]any{}
	next := ""
	for rows.Next() {
		var id, status string
		var rev int
		if err := rows.Scan(&id, &status, &rev); err != nil {
			rows.Close()
			return nil, err
		}
		if len(items) == limit {
			next = stringValue(items[len(items)-1]["id"])
			break
		}
		items = append(items, map[string]any{"id": id, "status": status, "revision": rev})
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	linked := 0
	if apply {
		err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
			if err := lockLifeContextTx(ctx, tx, owner); err != nil {
				return err
			}
			for _, item := range items {
				tag, err := tx.Exec(ctx, `INSERT INTO public.goal_initial_imports(fluctlight_id,source_id,input_version,goal_id,result) SELECT fluctlight_id,id,'legacy-v1',id,'linked_existing' FROM public.fluctlight_goals WHERE fluctlight_id=$1 AND id=$2 AND id LIKE 'goal_initial_%' ON CONFLICT DO NOTHING`, owner, stringValue(item["id"]))
				if err != nil {
					return err
				}
				linked += int(tag.RowsAffected())
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	var active, pending, capacity int
	if err := a.DB.Pool().QueryRow(ctx, `SELECT (SELECT count(*) FROM public.fluctlight_goals WHERE fluctlight_id=$1 AND status='active'),(SELECT count(*) FROM public.goal_initial_source_reviews WHERE fluctlight_id=$1 AND status='requires_semantic_review'),COALESCE((SELECT max_active_goals FROM public.goal_set_policies WHERE fluctlight_id=$1),5)`, owner).Scan(&active, &pending, &capacity); err != nil {
		return nil, err
	}
	return map[string]any{"fluctlight_id": owner, "apply": apply, "items": items, "next_cursor": next, "linked": linked, "active_count": active, "max_active_goals": capacity, "capacity_violation": active > capacity, "semantic_reviews": pending, "batch_identity": fmt.Sprint(owner, ":", after)}, nil
}
