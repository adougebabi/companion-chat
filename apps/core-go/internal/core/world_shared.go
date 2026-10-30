package core

import (
	"context"
	"github.com/jackc/pgx/v5"
)

type worldMutationActorKey struct{}

func readWorldTimezoneWith(ctx context.Context, q lifeContextQuerier, world string) (string, error) {
	var zone string
	err := q.QueryRow(ctx, `SELECT timezone FROM public.life_worlds WHERE id=$1`, world).Scan(&zone)
	return zone, err
}

func lockWorldRuntimeTx(ctx context.Context, tx pgx.Tx, world string) error {
	var owner string
	if err := tx.QueryRow(ctx, `SELECT owner_actor_id FROM public.life_worlds WHERE id=$1`, world).Scan(&owner); err != nil {
		return err
	}
	return lockOwnerWorldMutationTx(ctx, tx, owner)
}

// The conservative Owner mutation coordinator covers source/target worlds and
// Studio writes as well as membership changes. It holds only database locks,
// never a model request; actor data and world memberships remain independent.
func lockOwnerWorldMutationTx(ctx context.Context, tx pgx.Tx, owner string) error {
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "shared-world-owner:"+owner); err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `SELECT id FROM public.fluctlights WHERE created_by_actor_id=$1 ORDER BY id`, owner)
	if err != nil {
		return err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := lockLifeContextTx(ctx, tx, id); err != nil {
			return err
		}
	}
	return nil
}

func lockSharedWorldMembersTx(ctx context.Context, tx pgx.Tx, fid string, extraWorlds ...string) error {
	var owner string
	if err := tx.QueryRow(ctx, `SELECT created_by_actor_id FROM public.fluctlights WHERE id=$1`, fid).Scan(&owner); err != nil {
		return err
	}
	return lockOwnerWorldMutationTx(ctx, tx, owner)
}

func (a *App) createWorldMemberOpportunitiesTx(ctx context.Context, tx pgx.Tx, world string, npc, presence map[string]any) error {
	rows, err := tx.Query(ctx, `SELECT f.id,f.created_by_actor_id FROM public.fluctlight_world_bindings b JOIN public.fluctlights f ON f.id=b.fluctlight_id WHERE b.world_id=$1 AND f.status='active' ORDER BY f.id`, world)
	if err != nil {
		return err
	}
	type member struct{ fid, owner string }
	members := []member{}
	for rows.Next() {
		var m member
		if err := rows.Scan(&m.fid, &m.owner); err != nil {
			rows.Close()
			return err
		}
		members = append(members, m)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, m := range members {
		if err := a.createNPCOpportunityTx(ctx, tx, m.owner, m.fid, world, npc, presence, a.now().UTC()); err != nil {
			return err
		}
	}
	return nil
}
