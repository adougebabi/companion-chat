package core

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func (a *App) readWorldNPCs(ctx context.Context, fid, world string) ([]any, error) {
	items, err := readWorldObjects(ctx, a.DB.Pool(), `SELECT jsonb_build_object('id',id,'world_id',world_id,'actor_id',actor_id,'source_resource_id',source_resource_id,'source_version',source_version,'name',name,'role',role,'bio',background,'home_place_id',home_place_id,'routine',routine,'status',status,'confirmed_place_id',confirmed_place_id,'confirmed_at',confirmed_at,'revision',revision,'full_fluctlight',false,'enabled',enabled,'position_mode',position_mode,'local_overrides',local_overrides,'runtime_clock',(SELECT to_jsonb(c) FROM public.world_npc_runtime_clocks c WHERE c.npc_id=world_npcs.id),'current_presence',(SELECT to_jsonb(pr) FROM public.world_npc_effective_presence_sources pr WHERE pr.npc_id=world_npcs.id AND pr.world_id=world_npcs.world_id AND pr.definition_revision=world_npcs.revision AND pr.status='active' AND pr.start_at<=$2 AND pr.end_at>$2 ORDER BY pr.start_at DESC,pr.id DESC LIMIT 1)) FROM public.world_npcs WHERE world_id=$1 ORDER BY name,id`, world, a.now().UTC())
	if err != nil {
		return nil, err
	}
	zone, err := readWorldTimezoneWith(ctx, a.DB.Pool(), world)
	if err != nil {
		return nil, err
	}
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return nil, err
	}
	for _, raw := range items {
		npc := mapValue(raw)
		if npc["status"] != "active" || npc["enabled"] == false {
			continue
		}
		if plan := npcPlannedPosition(npc, a.now().In(loc)); plan != nil {
			npc["planned_position"] = plan
		}
	}
	return items, nil
}

// A routine predicts a location. It never becomes a confirmed presence fact.
func npcPlannedPosition(npc map[string]any, at time.Time) map[string]any {
	if npc["position_mode"] == "unknown" {
		return nil
	}
	if npc["position_mode"] == "fixed" {
		if home := stringValue(npc["home_place_id"]); home != "" {
			return map[string]any{"place_id": home, "source": "fixed_prediction", "window_start": npcClockAt(at, 0).UTC().Format(time.RFC3339Nano), "window_end": npcNextBoundary(npc, at).UTC().Format(time.RFC3339Nano)}
		}
		return nil
	}
	minute := at.Hour()*60 + at.Minute()
	for _, raw := range arrayValue(npc["routine"]) {
		slot := mapValue(raw)
		start, ok1 := parseNPCClock(stringValue(slot["start"]))
		end, ok2 := parseNPCClock(stringValue(slot["end"]))
		if !ok1 || !ok2 {
			continue
		}
		day := at.Weekday()
		inRange := minute >= start && minute < end
		if end < start {
			inRange = minute >= start || minute < end
			if minute < end {
				day = at.AddDate(0, 0, -1).Weekday()
			}
		}
		weekend := day == time.Saturday || day == time.Sunday
		kind := stringValue(slot["day"])
		if inRange && (kind == "daily" || (kind == "weekday" && !weekend) || (kind == "weekend" && weekend)) {
			base := at
			if end < start && minute < end {
				base = at.AddDate(0, 0, -1)
			}
			windowStart := npcClockAt(base, start)
			windowEnd := npcClockAt(base, end)
			if end < start {
				windowEnd = npcClockAt(base.AddDate(0, 0, 1), end)
			}
			return map[string]any{"place_id": slot["place_id"], "activity": slot["activity"], "source": "schedule_prediction", "timezone": at.Location().String(), "window_start": windowStart.UTC().Format(time.RFC3339Nano), "window_end": windowEnd.UTC().Format(time.RFC3339Nano)}
		}
	}
	if home := stringValue(npc["home_place_id"]); home != "" {
		return map[string]any{"place_id": home, "source": "initial_place", "window_start": npcClockAt(at, 0).UTC().Format(time.RFC3339Nano), "window_end": npcNextBoundary(npc, at).UTC().Format(time.RFC3339Nano)}
	}
	return nil
}

func validateNPCRoutine(ctx context.Context, q lifeContextQuerier, world, home string, value any) ([]any, error) {
	routine, ok := value.([]any)
	if value == nil {
		routine = []any{}
		ok = true
	}
	if !ok || len(routine) > 42 {
		return nil, invalid("routine must contain at most 42 slots")
	}
	places := []string{}
	if home != "" {
		places = append(places, home)
	}
	for i, raw := range routine {
		slot, ok := raw.(map[string]any)
		if !ok {
			return nil, invalid("routine slot is invalid")
		}
		for key := range slot {
			if key != "day" && key != "start" && key != "end" && key != "place_id" && key != "activity" {
				return nil, invalid("unsupported routine field")
			}
		}
		day := stringValue(slot["day"])
		if day != "daily" && day != "weekday" && day != "weekend" {
			return nil, invalid("routine day is invalid")
		}
		start, a := parseNPCClock(stringValue(slot["start"]))
		end, b := parseNPCClock(stringValue(slot["end"]))
		if !a || !b || start == end {
			return nil, invalid("routine time is invalid")
		}
		place := stringValue(slot["place_id"])
		if place == "" {
			return nil, invalid("routine place is required")
		}
		places = append(places, place)
		if _, err := boundedWorldText(slot, "activity", 512, true); err != nil {
			return nil, err
		}
		for j := 0; j < i; j++ {
			other := mapValue(routine[j])
			otherDay := stringValue(other["day"])
			if day != otherDay && day != "daily" && otherDay != "daily" {
				continue
			}
			os, _ := parseNPCClock(stringValue(other["start"]))
			oe, _ := parseNPCClock(stringValue(other["end"]))
			for m := 0; m < 1440; m++ {
				inside := func(x, y int) bool {
					if y > x {
						return m >= x && m < y
					}
					return m >= x || m < y
				}
				if inside(start, end) && inside(os, oe) {
					return nil, invalid("routine slots overlap")
				}
			}
		}
	}
	for _, place := range places {
		var exists bool
		if err := q.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.world_places WHERE id=$1 AND world_id=$2 AND status='active')`, place, world).Scan(&exists); err != nil {
			return nil, err
		}
		if !exists {
			return nil, invalid("NPC place is outside this runtime world")
		}
	}
	return routine, nil
}

func (a *App) applyWorldNPCTx(ctx context.Context, tx pgx.Tx, fid string, p worldPublication, current, values, command map[string]any) (map[string]any, error) {
	world, err := worldIDWith(ctx, tx, fid)
	if err != nil {
		return nil, err
	}
	id := stringValue(current["id"])
	if id != "" {
		// Local edits remain explicit overrides; definition upgrades retain identity.
		var overridesRaw []byte
		if err := tx.QueryRow(ctx, `SELECT local_overrides FROM public.world_npcs WHERE id=$1 AND world_id=$2`, id, world).Scan(&overridesRaw); err != nil {
			return nil, err
		}
		overrides := decodeObject(overridesRaw)
		for _, field := range []string{"name", "role", "background"} {
			if value, ok := overrides[field]; ok {
				values[field] = value
			}
		}
		_, err = tx.Exec(ctx, `UPDATE public.world_npcs SET name=$2,role=$3,background=$4,source_version=$5,revision=revision+1,updated_at=now() WHERE id=$1 AND world_id=$6`, id, values["name"], values["role"], values["background"], p.Version, world)
		if err != nil {
			return nil, err
		}
		if err := invalidateNPCPresenceTx(ctx, tx, id, a.now().UTC(), "npc_definition_upgraded"); err != nil {
			return nil, err
		}
		return map[string]any{"id": id, "revision": intValue(current["revision"]) + 1}, nil
	}
	home := stringValue(command["home_place_id"])
	routine, err := validateNPCRoutine(ctx, tx, world, home, command["routine"])
	if err != nil {
		return nil, err
	}
	id, actor := randomID("npc_"), randomID("actor_npc_")
	if _, err := tx.Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'npc','active')`, actor); err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.world_npcs(id,world_id,actor_id,source_resource_id,source_version,name,role,background,home_place_id,routine) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, id, world, actor, p.ID, p.Version, values["name"], values["role"], values["background"], nullableString(home), jsonBytes(routine))
	return map[string]any{"id": id, "actor_id": actor, "revision": 1}, err
}

func (a *App) updateWorldNPCTx(ctx context.Context, tx pgx.Tx, owner, fid, id, method string, body map[string]any) (map[string]any, error) {
	world, err := worldIDWith(ctx, tx, fid)
	if err != nil {
		return nil, err
	}
	var actor string
	var enabled bool
	var mode string
	var revision int
	if err := tx.QueryRow(ctx, `SELECT actor_id,revision,enabled,position_mode FROM public.world_npcs WHERE id=$1 AND world_id=$2 FOR UPDATE`, id, world).Scan(&actor, &revision, &enabled, &mode); err != nil {
		return nil, err
	}
	expected, ok := parseRevision(body["expected_revision"])
	if !ok || expected != revision {
		return nil, conflict("npc", expected, revision)
	}
	if method == "DELETE" {
		var referenced bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.world_encounters WHERE npc_id=$1) OR EXISTS(SELECT 1 FROM public.relationships WHERE target_actor_id=$2) OR EXISTS(SELECT 1 FROM public.memories WHERE actor_refs ? $2) OR EXISTS(SELECT 1 FROM public.conversation_participants WHERE actor_id=$2) OR EXISTS(SELECT 1 FROM public.world_npc_presence_runs WHERE npc_id=$1) OR EXISTS(SELECT 1 FROM public.world_npc_opportunities WHERE npc_id=$1) OR EXISTS(SELECT 1 FROM public.world_event_templates t WHERE EXISTS(SELECT 1 FROM jsonb_array_elements(t.definition->'effects') e WHERE e->>'npc_id'=$1)) OR EXISTS(SELECT 1 FROM public.world_event_runs r WHERE EXISTS(SELECT 1 FROM jsonb_array_elements(r.definition->'effects') e WHERE e->>'npc_id'=$1))`, id, actor).Scan(&referenced); err != nil {
			return nil, err
		}
		if referenced {
			return nil, worldReferenced("NPC has encounters, relationships, memory or conversation references")
		}
		if _, err := tx.Exec(ctx, `DELETE FROM public.world_npcs WHERE id=$1 AND world_id=$2`, id, world); err != nil {
			return nil, err
		}
		// Retain the retired Actor identity to avoid recycling historical identities.
		if _, err := tx.Exec(ctx, `UPDATE public.actors SET status='inactive' WHERE id=$1`, actor); err != nil {
			return nil, err
		}
		return map[string]any{"id": id, "actor_id": actor, "status": "deleted"}, nil
	}
	if method != "PUT" {
		return nil, ErrInvalidArguments
	}
	name, err := boundedWorldText(body, "name", 256, true)
	if err != nil {
		return nil, err
	}
	role, err := boundedWorldText(body, "role", 512, false)
	if err != nil {
		return nil, err
	}
	background, err := boundedWorldText(body, "background", 16000, false)
	if err != nil {
		return nil, err
	}
	status := stringValue(body["status"])
	if status != "active" && status != "archived" {
		return nil, ErrInvalidArguments
	}
	home := stringValue(body["home_place_id"])
	routine, err := validateNPCRoutine(ctx, tx, world, home, body["routine"])
	if err != nil {
		return nil, err
	}
	if value, ok := body["position_mode"]; ok {
		var valid bool
		mode, valid = value.(string)
		if !valid {
			return nil, invalid("positionMode must be a string")
		}
	}
	if value, ok := body["enabled"]; ok {
		var valid bool
		enabled, valid = value.(bool)
		if !valid {
			return nil, invalid("enabled must be a boolean")
		}
	}
	if mode == "" {
		mode = "routine"
	}
	if mode != "routine" && mode != "fixed" && mode != "unknown" {
		return nil, invalid("positionMode is invalid")
	}
	overrides := map[string]any{}
	var sourceID, sourceVersion string
	if err := tx.QueryRow(ctx, `SELECT source_resource_id,source_version FROM public.world_npcs WHERE id=$1`, id).Scan(&sourceID, &sourceVersion); err != nil {
		return nil, err
	}
	source, err := readWorldPublication(ctx, tx, owner, sourceID, sourceVersion)
	if err != nil {
		return nil, err
	}
	for field, value := range map[string]string{"name": name, "role": role, "background": background} {
		base := stringValue(source.Body[field])
		if field == "name" {
			base = source.Name
		}
		if value != base {
			overrides[field] = value
		}
	}
	_, err = tx.Exec(ctx, `UPDATE public.world_npcs SET name=$3,role=$4,background=$5,status=$6,home_place_id=$7,routine=$8,enabled=$9,position_mode=$10,local_overrides=$11,revision=revision+1,updated_at=now() WHERE id=$1 AND world_id=$2`, id, world, name, role, background, status, nullableString(home), jsonBytes(routine), enabled, mode, jsonBytes(overrides))
	if err != nil {
		return nil, err
	}
	if err := invalidateNPCPresenceTx(ctx, tx, id, a.now().UTC(), "npc_definition_changed"); err != nil {
		return nil, err
	}
	return map[string]any{"id": id, "actor_id": actor, "revision": revision + 1, "status": "saved", "history_preserved": true}, nil
}

func (a *App) worldNPCDetail(ctx context.Context, fid, id string) (map[string]any, error) {
	world, err := worldIDWith(ctx, a.DB.Pool(), fid)
	if err != nil {
		return nil, err
	}
	items, err := a.readWorldNPCs(ctx, fid, world)
	if err != nil {
		return nil, err
	}
	var npc map[string]any
	for _, raw := range items {
		if mapValue(raw)["id"] == id {
			npc = mapValue(raw)
			break
		}
	}
	if npc == nil {
		return nil, ErrNotFound
	}
	encounters, err := readWorldObjects(ctx, a.DB.Pool(), `SELECT jsonb_build_object('id',id,'npc_id',npc_id,'place_id',place_id,'life_event_id',life_event_id,'relationship_id',relationship_id,'memory_id',memory_id,'summary',summary,'evidence_refs',evidence_refs,'occurred_at',occurred_at,'source_kind',source_kind,'opportunity_id',opportunity_id) FROM public.world_encounters WHERE world_id=$1 AND npc_id=$2 AND life_event_id IN(SELECT id FROM public.life_events WHERE fluctlight_id=$3) ORDER BY occurred_at DESC,id LIMIT 100`, world, id, fid)
	if err != nil {
		return nil, err
	}
	references, err := readWorldObjects(ctx, a.DB.Pool(), `SELECT jsonb_build_object('id',id) FROM ((SELECT id FROM public.world_npc_presence_runs WHERE npc_id=$1 ORDER BY created_at DESC LIMIT 1) UNION ALL (SELECT id FROM public.world_npc_opportunities WHERE npc_id=$1 AND fluctlight_id=$2 ORDER BY created_at DESC LIMIT 1)) refs`, id, fid)
	if err != nil {
		return nil, err
	}
	ids := []any{}
	for _, raw := range references {
		ids = append(ids, mapValue(raw)["id"])
	}
	npc["runtime_reference_ids"] = ids
	npc["encounters"] = encounters
	context, err := a.readActorContextWith(ctx, a.DB.Pool(), fid, stringValue(npc["actor_id"]))
	if err != nil {
		return nil, err
	}
	npc["relationship_context"] = context
	return npc, nil
}

func (a *App) recordNPCEncounterTx(ctx context.Context, tx pgx.Tx, owner, fid, id string, body map[string]any) (map[string]any, error) {
	if body["confirmed"] != true {
		return nil, invalid("Explicit confirmation of an occurred encounter is required")
	}
	summary, err := boundedWorldText(body, "summary", 2000, true)
	if err != nil {
		return nil, err
	}
	start, e1 := parseScheduleTime(stringValue(body["start_at"]))
	end, e2 := parseScheduleTime(stringValue(body["end_at"]))
	now := a.now().UTC()
	if e1 != nil || e2 != nil || !end.After(start) || end.After(now) || end.Sub(start) > 24*time.Hour {
		return nil, invalid("Record an already finished encounter of at most one day")
	}
	if _, err := a.requireLifeContextRevisionTx(ctx, tx, fid, stringValue(body["expected_life_context_revision"]), now); err != nil {
		return nil, err
	}
	world, err := worldIDWith(ctx, tx, fid)
	if err != nil {
		return nil, err
	}
	var actor, name string
	if err := tx.QueryRow(ctx, `SELECT actor_id,name FROM public.world_npcs WHERE id=$1 AND world_id=$2 AND status='active' AND enabled FOR UPDATE`, id, world).Scan(&actor, &name); err != nil {
		return nil, err
	}
	place := stringValue(body["place_id"])
	var placeName string
	if err := tx.QueryRow(ctx, `SELECT name FROM public.world_places WHERE id=$1 AND world_id=$2 AND status='active'`, place, world).Scan(&placeName); err != nil {
		return nil, err
	}
	refs := arrayValue(body["evidence_refs"])
	if len(refs) == 0 || len(refs) > 32 {
		return nil, invalid("Encounter evidence is required")
	}
	for _, ref := range refs {
		if strings.TrimSpace(stringValue(ref)) == "" || len(stringValue(ref)) > 256 {
			return nil, ErrInvalidArguments
		}
	}
	for index, ref := range refs {
		if stringValue(ref) == "owner_confirmation" {
			// This source is the same transaction's attributed owner audit, not
			// an invented external event or an unverified reference string.
			audit, _ := ctx.Value(worldCommandAuditKey{}).(string)
			if audit == "" {
				return nil, ErrInvalidArguments
			}
			refs[index] = audit
			continue
		}
		var visible bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.life_events WHERE id=$1 AND fluctlight_id=$2) OR EXISTS(SELECT 1 FROM public.conversation_messages m JOIN public.conversation_participants p ON p.conversation_id=m.conversation_id WHERE m.id=$1 AND p.actor_id=$2) OR EXISTS(SELECT 1 FROM public.conversations c JOIN public.conversation_participants p ON p.conversation_id=c.id WHERE c.id=$1 AND p.actor_id=$2 AND c.created_by_actor_id=$3)`, stringValue(ref), fid, owner).Scan(&visible); err != nil {
			return nil, err
		}
		if !visible {
			return nil, invalid("Encounter evidence is not visible in this character scope")
		}
	}
	return a.commitNPCEncounterTx(ctx, tx, owner, fid, world, id, actor, name, place, placeName, summary, refs, start, end, stringValue(body["expected_life_context_revision"]), stringValue(body["idempotency_key"]), stringValue(body["reason"]), npcEncounterAttribution{Kind: "owner_confirmed_encounter", ActorID: owner, SourceFactID: "owner:" + owner, Direct: true, Establish: true})
}
