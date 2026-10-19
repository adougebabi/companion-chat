package core

import (
	"context"
	"errors"
	"net/url"

	"github.com/jackc/pgx/v5"
)

const lifeEventWorldSQL = `COALESCE(e.result#>>'{runtime_effect,world_id}',(SELECT world_id FROM public.world_encounters n WHERE n.life_event_id=e.id LIMIT 1),(SELECT world_id FROM public.world_places p WHERE p.id=e.result->>'place_id'))`
const lifeEventReadSQL = `to_jsonb(e)||jsonb_build_object('world_id',` + lifeEventWorldSQL + `,'title',COALESCE(NULLIF(e.activity,''),e.kind))`

// Unknown legacy world attribution stays explicit instead of being filled with
// the currently selected world. Queries never produce an event or a movement.
func (a *App) worldLifeEvents(ctx context.Context, owner, fid, id string, q url.Values) (map[string]any, error) {
	if id != "" {
		var raw []byte
		err := a.DB.Pool().QueryRow(ctx, `SELECT `+lifeEventReadSQL+` FROM public.life_events e WHERE e.fluctlight_id=$1 AND e.id=$2`, fid, id).Scan(&raw)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		if err != nil {
			return nil, err
		}
		result := decodeObject(raw)
		impacts, err := readWorldObjects(ctx, a.DB.Pool(), `SELECT jsonb_build_object('id',id,'type','memory','title',left(content,200),'status',status) FROM public.memories WHERE owner_fluctlight_id=$1 AND (event_refs ? $2 OR evidence_refs ? $2) ORDER BY updated_at DESC,id LIMIT 101`, fid, id)
		if err != nil {
			return nil, err
		}
		more := len(impacts) > 100
		if more {
			impacts = impacts[:100]
		}
		encounters, err := readWorldObjects(ctx, a.DB.Pool(), `SELECT jsonb_build_object('id',id,'type','npc_encounter','title',summary,'npc_id',npc_id,'relationship_id',relationship_id,'memory_id',memory_id) FROM public.world_encounters WHERE life_event_id=$1 ORDER BY occurred_at,id LIMIT 100`, id)
		if err != nil {
			return nil, err
		}
		impacts = append(impacts, encounters...)
		runs, err := readWorldObjects(ctx, a.DB.Pool(), `SELECT jsonb_build_object('id',id,'type','world_event_execution','title',definition->>'description','template_id',template_id,'status',status,'recovery_status',recovery_status) FROM public.world_event_runs WHERE fluctlight_id=$1 AND life_event_id=$2 ORDER BY created_at,id LIMIT 100`, fid, id)
		if err != nil {
			return nil, err
		}
		impacts = append(impacts, runs...)
		result["impacts"], result["more_memory_impacts"] = impacts, more
		result["impact_scope"] = "Direct recorded references; no inferred downstream causality"
		return result, nil
	}
	world, err := worldIDWith(ctx, a.DB.Pool(), fid)
	if err != nil {
		return nil, err
	}
	limit, err := reflectionReadPage(q)
	if err != nil {
		return nil, err
	}
	scope := stableDigest(jsonString([]string{"world-life-events", owner, fid, world}))
	cursor, err := decodeOwnerReadCursor(q.Get("cursor"), scope)
	if err != nil || (q.Get("cursor") != "" && (cursor.ID == "" || cursor.Time.IsZero())) {
		return nil, invalid("invalid world event cursor")
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT e.id,e.start_at,`+lifeEventReadSQL+` FROM public.life_events e WHERE e.fluctlight_id=$1 AND (`+lifeEventWorldSQL+`=$2 OR `+lifeEventWorldSQL+` IS NULL) AND ($3='' OR (e.start_at,e.id)<($4,$3)) ORDER BY e.start_at DESC,e.id DESC LIMIT $5`, fid, world, cursor.ID, cursor.Time, limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	page, err := readGovernanceRows(rows, scope, limit)
	if err != nil {
		return nil, err
	}
	page["world_id"], page["includes_unattributed_history"] = world, true
	return page, nil
}
