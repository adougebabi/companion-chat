package core

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// Trusted provenance, never a public API parameter. Ambient events retain their
// real Life Event/body evidence without becoming scene or location authority.
type lifeEventRuntimeEffectKey struct{}
type lifeEventRuntimeEffect struct {
	Ambient bool
	Facts   map[string]any
}

func activeWorldEffectsWith(ctx context.Context, q lifeContextQuerier, fid, world, place string, at time.Time) ([]any, error) {
	return readWorldObjects(ctx, q, `SELECT jsonb_build_object('run_id',r.id,'event_id',r.life_event_id,'template_id',r.template_id,'template_revision',r.template_revision,'source','virtual_world_event','start_at',r.started_at,'end_at',r.end_at,'description',r.definition->>'description','effects',(SELECT jsonb_agg(e) FROM jsonb_array_elements(r.definition->'effects') e WHERE (e->>'scope'<>'character' OR r.fluctlight_id=$1) AND ($3='*' OR e->>'scope' IN ('world','character') OR (e->>'scope'='place' AND e->>'place_id'=$3)))) FROM public.world_event_runs r LEFT JOIN public.life_events le ON le.id=r.life_event_id WHERE (r.life_event_id IS NULL OR le.status='confirmed') AND r.world_id=$2 AND r.status='applied' AND r.started_at<=$4 AND r.end_at>$4 AND EXISTS(SELECT 1 FROM jsonb_array_elements(r.definition->'effects') e WHERE (e->>'scope'<>'character' OR r.fluctlight_id=$1) AND ($3='*' OR e->>'scope' IN ('world','character') OR e->>'place_id'=$3)) ORDER BY r.started_at,r.id LIMIT 32`, fid, world, place, at)
}
func requireWorldPlaceOpenTx(ctx context.Context, tx pgx.Tx, fid, place string, at time.Time) error {
	if place == "" {
		return nil
	}
	var closed bool
	err := tx.QueryRow(ctx, `SELECT COALESCE((SELECT e->>'value'='closed' FROM public.world_event_runs r JOIN public.fluctlight_world_bindings b ON b.world_id=r.world_id AND b.fluctlight_id=$1 LEFT JOIN public.life_events le ON le.id=r.life_event_id CROSS JOIN LATERAL jsonb_array_elements(r.definition->'effects') e WHERE (r.life_event_id IS NULL OR le.status='confirmed') AND r.world_id=(SELECT world_id FROM public.fluctlight_world_bindings WHERE fluctlight_id=$1) AND r.status='applied' AND r.started_at<=$3 AND r.end_at>$3 AND e->>'kind'='place_availability' AND e->>'place_id'=$2 ORDER BY r.started_at DESC,r.id DESC LIMIT 1),false)`, fid, place, at).Scan(&closed)
	if err != nil {
		return err
	}
	if closed {
		return newCapabilityError("world_place_temporarily_closed", false, ErrConflict)
	}
	return nil
}
