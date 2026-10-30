package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const scheduleWorldPlaceInstruction = `Use identity.residence as the declared home when supplied; occupation is not a residential address. Never manufacture a home by appending 住所 to a job title. Keep home and workplace distinct unless the supplied facts explicitly place them together. Check the entire sequence: a commute needs a meaningful departure and destination consistent with the adjacent intervals; do not insert commuting between the same place. A journey is not arrival: use location_kind='transit' and omit place_ref/place_id while in transit. Concrete venue locations require place_ref or place_id; location_kind='unknown' may omit location entirely.
world_places contains the selected world's actual active places and its own timezone. Express schedule intervals in the requested character timezone, comparing actual instants with world-local time; never reinterpret one actor's display timezone as the world clock. Reuse their exact id in place_ref, or retain an existing explicit place_id. When a concrete location required by the supplied life facts is missing and world_places.configured=true, propose it once in new_places with a unique local key, concrete name and short description; reference that key in the relevant items' place_ref. Do not invent runtime IDs, duplicate a known place, or invent residence/workplace details. Missing address details may stay unknown. Unknown locations and unplanned intervals may omit place_ref and location. With no selected world, return no new_places. Core registers accepted proposals transactionally; a proposed place or scheduled interval does not prove arrival, NPC presence or acquaintance. Preserve completed intervals and their original location/place_id unchanged.`

type scheduleWorldPlaceContext struct {
	WorldID    string
	Revision   int
	Places     []map[string]any
	Timezone   string
	Name       string
	Background string
	Rules      string
}

func (a *App) readScheduleWorldPlaces(ctx context.Context, fid string) (*scheduleWorldPlaceContext, error) {
	result := &scheduleWorldPlaceContext{Places: []map[string]any{}}
	var raw []byte
	err := a.DB.Pool().QueryRow(ctx, `SELECT b.world_id,b.revision,w.timezone,w.name,w.background,w.rules,COALESCE((SELECT jsonb_agg(jsonb_build_object('id',p.id,'name',p.name,'description',p.description,'parent_place_id',p.parent_place_id) ORDER BY p.id) FROM public.world_places p WHERE p.world_id=b.world_id AND p.status='active'),'[]') FROM public.fluctlight_world_bindings b JOIN public.life_worlds w ON w.id=b.world_id WHERE b.fluctlight_id=$1`, fid).Scan(&result.WorldID, &result.Revision, &result.Timezone, &result.Name, &result.Background, &result.Rules, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return result, nil
	}
	if err != nil {
		return nil, err
	}
	for _, value := range decodeArray(raw) {
		result.Places = append(result.Places, mapValue(value))
	}
	return result, nil
}

func (c *scheduleWorldPlaceContext) modelView() map[string]any {
	if c == nil {
		return map[string]any{"configured": false, "places": []any{}}
	}
	return map[string]any{"configured": c.WorldID != "", "places": c.Places, "timezone": c.Timezone, "name": c.Name, "background": c.Background, "rules": c.Rules}
}

func (c *scheduleWorldPlaceContext) bind(payload map[string]any) {
	if c == nil {
		return
	}
	payload["_schedule_world"] = map[string]any{"world_id": c.WorldID, "revision": c.Revision}
}

func scheduleNewPlacesSchema() map[string]any {
	return arraySchema(objectSchema(map[string]any{"key": stringSchema(), "name": stringSchema(), "description": stringSchema(), "parent_place_id": stringSchema()}, []string{"key", "name", "description"}, false))
}

// Legacy plans receive a durable model reassessment, never a text-to-place
// guess or in-place history rewrite. World-aware plans (including transit)
// carry a marker so their intentional unknown positions do not cause loops.
func (a *App) EnsureLegacyScheduleWorldReconciliation(ctx context.Context, limit int) (int, error) {
	if limit < 1 || limit > 100 {
		limit = 50
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT f.id,s.id,b.revision FROM public.fluctlights f JOIN public.fluctlight_world_bindings b ON b.fluctlight_id=f.id JOIN public.life_schedules s ON s.fluctlight_id=f.id JOIN pg_timezone_names z ON z.name=COALESCE(NULLIF(f.identity->>'timezone',''),'Asia/Shanghai') WHERE f.status='active' AND s.status='accepted' AND s.place_binding_version=0 AND s.local_date=($1::timestamptz AT TIME ZONE z.name)::date AND NOT EXISTS(SELECT 1 FROM public.cognition_inbox c WHERE c.fluctlight_id=f.id AND c.idempotency_key='world-schedule-reconcile:'||s.id||':'||b.revision::text) AND EXISTS(SELECT 1 FROM public.life_schedule_items i LEFT JOIN public.world_places p ON p.id=i.place_id WHERE i.schedule_id=s.id AND i.end_at>$1 AND COALESCE(i.location,'')<>'' AND (i.place_id IS NULL OR p.world_id<>b.world_id)) ORDER BY s.generated_at,s.id LIMIT $2`, a.now().UTC(), limit)
	if err != nil {
		return 0, err
	}
	type target struct {
		fid, id  string
		revision int
	}
	targets := []target{}
	for rows.Next() {
		var t target
		if err := rows.Scan(&t.fid, &t.id, &t.revision); err != nil {
			rows.Close()
			return 0, err
		}
		targets = append(targets, t)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	for _, t := range targets {
		if err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
			if err := lockSharedWorldMembersTx(ctx, tx, t.fid); err != nil {
				return err
			}
			var current bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.life_schedules s JOIN public.fluctlight_world_bindings b ON b.fluctlight_id=s.fluctlight_id WHERE s.id=$1 AND s.status='accepted' AND s.place_binding_version=0 AND b.revision=$2)`, t.id, t.revision).Scan(&current); err != nil {
				return err
			}
			if !current {
				return nil
			}
			_, err := a.enqueueNativeFactTx(ctx, tx, t.fid, "", "", "life.world.schedule_reconcile", "world-schedule-reconcile:"+t.id+":"+fmt.Sprint(t.revision), map[string]any{"issue": "legacy_future_locations_not_bound_to_world", "reason": "Assess remaining schedule against the selected world's places and declared residence. Preserve completed history and actual Life; plans do not prove movement."})
			return err
		}); err != nil {
			return 0, err
		}
	}
	return len(targets), nil
}

// Called only inside the accepted Schedule transaction, after replay/CAS and
// completed-history validation. No name-based guessing of existing identity.
func (a *App) acceptSchedulePlacesTx(ctx context.Context, tx pgx.Tx, fid string, payload map[string]any) error {
	proposals := arrayValue(payload["new_places"])
	worldContext, scoped := payload["_schedule_world"]
	usesRefs := false
	for _, raw := range arrayValue(payload["items"]) {
		if stringValue(mapValue(raw)["place_ref"]) != "" {
			usesRefs = true
		}
	}
	if !scoped && len(proposals) == 0 && !usesRefs {
		return nil
	}
	var world string
	var revision int
	err := tx.QueryRow(ctx, `SELECT world_id,revision FROM public.fluctlight_world_bindings WHERE fluctlight_id=$1 FOR UPDATE`, fid).Scan(&world, &revision)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if scoped {
		frozen := mapValue(worldContext)
		if world != stringValue(frozen["world_id"]) || revision != intValue(frozen["revision"]) {
			return ErrLifeContextStale
		}
	}
	if world == "" {
		if len(proposals) > 0 || usesRefs {
			return errors.New("schedule_place_world_required")
		}
		return nil
	}
	boundary, _ := time.Parse(time.RFC3339Nano, stringValue(payload["completed_before"]))
	for _, raw := range arrayValue(payload["items"]) {
		item := mapValue(raw)
		ref, id := stringValue(item["place_ref"]), stringValue(item["place_id"])
		start, _ := time.Parse(time.RFC3339Nano, stringValue(item["start_at"]))
		if !boundary.IsZero() && start.Before(boundary) && ref != "" && ref != id {
			return errors.New("schedule_completed_place_immutable")
		}
		if scoped && stringValue(item["location_kind"]) == "transit" && (ref != "" || id != "") {
			return errors.New("schedule_transit_place_invalid")
		}
		if scoped && stringValue(item["location"]) != "" && ref == "" && id == "" && item["location_kind"] != "transit" && (boundary.IsZero() || !start.Before(boundary)) {
			return errors.New("schedule_place_reference_required")
		}
	}
	refs := map[string]string{}
	used := map[string]bool{}
	for _, raw := range arrayValue(payload["items"]) {
		if ref := stringValue(mapValue(raw)["place_ref"]); ref != "" {
			used[ref] = true
		}
	}
	for _, raw := range proposals {
		p := mapValue(raw)
		key := strings.TrimSpace(stringValue(p["key"]))
		name := strings.TrimSpace(stringValue(p["name"]))
		parent := stringValue(p["parent_place_id"])
		if key == "" || refs[key] != "" || !used[key] || name == "" {
			return errors.New("schedule_place_proposal_invalid")
		}
		var duplicate bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.world_places WHERE world_id=$1 AND status='active' AND (id=$2 OR (name=$3 AND COALESCE(parent_place_id,'')=$4)))`, world, key, name, parent).Scan(&duplicate); err != nil {
			return err
		}
		// A concurrent create requires a fresh model selection, never choosing
		// the first same-name row or manufacturing a second identical place.
		if duplicate {
			return errors.New("schedule_place_proposal_conflict")
		}
		place, err := a.writeWorldPlaceTx(ctx, tx, fid, "", "POST", map[string]any{"world_id": world, "name": name, "description": p["description"], "parent_place_id": parent})
		if err != nil {
			return err
		}
		refs[key] = stringValue(place["id"])
	}
	for _, raw := range arrayValue(payload["items"]) {
		item := mapValue(raw)
		ref := stringValue(item["place_ref"])
		if ref == "" {
			continue
		}
		id := refs[ref]
		if id == "" {
			id = ref
		}
		if existing := stringValue(item["place_id"]); existing != "" && existing != id {
			return errors.New("schedule_place_reference_conflict")
		}
		if err := validateLifePlaceTx(ctx, tx, fid, id); err != nil {
			return err
		}
		item["place_id"] = id
		delete(item, "place_ref")
	}
	return nil
}
