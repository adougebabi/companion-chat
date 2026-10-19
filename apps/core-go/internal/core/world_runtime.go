package core

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// MatchWorldSurface is an explicit domain boundary, not an arbitrary proxy.
func MatchWorldSurface(method, path string) bool {
	p := surfaceParts(path)
	if len(p) == 4 && p[0] == "studio" && p[1] == "resources" && (p[3] == "publish" || p[3] == "apply" || p[3] == "apply-preview") {
		return method == "POST"
	}
	if len(p) < 3 || p[0] != "fluctlights" {
		return false
	}
	if p[2] == "nearby" {
		return len(p) == 3 && method == "GET"
	}
	if p[2] == "npcs" {
		if len(p) == 3 {
			return method == "GET"
		}
		if len(p) == 4 {
			return method == "GET" || method == "PUT" || method == "DELETE"
		}
		if len(p) == 5 {
			if p[4] == "encounters" {
				return method == "GET" || method == "POST"
			}
			return (p[4] == "presences" || p[4] == "opportunities") && method == "GET"
		}
		if len(p) == 6 && p[4] == "opportunities" {
			return method == "GET"
		}
		return len(p) == 7 && p[4] == "opportunities" && p[6] == "retry" && method == "POST"
	}
	if p[2] != "world" {
		return false
	}
	if len(p) >= 4 && p[3] == "items" {
		return (method == "GET" && (len(p) == 4 || len(p) == 5)) || (method == "PUT" && len(p) == 5)
	}
	if len(p) >= 4 && p[3] == "templates" {
		if method == "GET" {
			return len(p) == 4 || len(p) == 5 || (len(p) == 6 && p[5] == "runs") || (len(p) == 7 && p[5] == "runs")
		}
		return (method == "PUT" && len(p) == 5) || (method == "POST" && ((len(p) == 6 && p[5] == "runs") || (len(p) == 8 && p[5] == "runs" && p[7] == "retry")))
	}
	if len(p) >= 4 && p[3] == "events" {
		return method == "GET" && (len(p) == 4 || len(p) == 5)
	}
	return len(p) == 3 || (len(p) >= 4 && ((p[3] == "templates" && len(p) <= 8) || ((p[3] == "places" || p[3] == "catalog" || p[3] == "active") && len(p) <= 5)))
}

func (a *App) DispatchWorldSurface(ctx context.Context, method, path string, query url.Values, owner string, body map[string]any) (map[string]any, error) {
	p := surfaceParts(path)
	if p[0] == "studio" {
		if p[3] == "publish" {
			return a.publishWorldResource(ctx, owner, p[2], body)
		}
		return a.DispatchWorldApply(ctx, owner, p[2], body, p[3] == "apply-preview")
	}
	fid := p[1]
	if containsPermanentHarm(body) {
		return nil, harmError()
	}
	if _, err := a.DB.GetFluctlight(ctx, fid, owner); err != nil {
		return nil, err
	}
	if p[2] == "world" && len(p) >= 4 && p[3] == "events" {
		id := ""
		if len(p) == 5 {
			id = p[4]
		}
		return a.worldLifeEvents(ctx, owner, fid, id, query)
	}
	if p[2] == "world" && len(p) >= 4 && p[3] == "items" {
		return a.worldItemSurface(ctx, method, p, query, owner, fid, body)
	}
	if p[2] == "world" && len(p) >= 4 && p[3] == "templates" {
		return a.worldTemplateSurface(ctx, method, p, query, owner, fid, body)
	}
	if method == "GET" {
		if p[2] == "npcs" && len(p) == 6 && p[4] == "opportunities" {
			return a.WorldNPCOpportunity(ctx, owner, fid, p[3], p[5])
		}
		if p[2] == "npcs" && len(p) == 5 {
			return a.WorldNPCRecords(ctx, owner, fid, p[3], p[4], query)
		}
		if len(p) >= 4 && p[3] == "catalog" {
			if len(p) == 5 {
				return a.worldSnapshot(ctx, owner, fid, p[4])
			}
			return a.worldCatalog(ctx, fid)
		}
		if p[2] == "nearby" {
			return a.worldNearby(ctx, owner, fid)
		}
		if p[2] == "npcs" && len(p) == 4 {
			return a.worldNPCDetail(ctx, fid, p[3])
		}
		snapshot, err := a.WorldSnapshot(ctx, owner, fid)
		if err != nil {
			return nil, err
		}
		if p[2] == "npcs" {
			return map[string]any{"items": snapshot["npcs"], "next_cursor": nil}, nil
		}
		if len(p) == 5 {
			for _, raw := range arrayValue(snapshot["places"]) {
				item := mapValue(raw)
				if item["id"] == p[4] {
					return item, nil
				}
			}
			return nil, ErrNotFound
		}
		return snapshot, nil
	}
	return a.worldCommand(ctx, owner, fid, method+":"+path, body, func(ctx context.Context, tx pgx.Tx) (map[string]any, error) {
		if len(p) >= 4 && p[3] == "catalog" {
			return a.writeWorldCatalogTx(ctx, tx, fid, method, p, body)
		}
		if len(p) == 4 && p[3] == "active" {
			return a.selectWorldTx(ctx, tx, fid, body)
		}
		if p[2] == "npcs" {
			if len(p) == 7 && p[4] == "opportunities" && p[6] == "retry" {
				return a.retryNPCOpportunityTx(ctx, tx, owner, fid, p[3], p[5], body)
			}
			if len(p) == 5 {
				return a.recordNPCEncounterTx(ctx, tx, owner, fid, p[3], body)
			}
			if len(p) == 4 {
				return a.updateWorldNPCTx(ctx, tx, owner, fid, p[3], method, body)
			}
			return nil, ErrInvalidArguments
		}
		if len(p) == 3 && method == "PUT" {
			return a.writeWorldTx(ctx, tx, fid, body)
		}
		if len(p) >= 4 && p[3] == "places" {
			id := ""
			if len(p) == 5 {
				id = p[4]
			}
			return a.writeWorldPlaceTx(ctx, tx, fid, id, method, body)
		}
		return nil, ErrInvalidArguments
	})
}

// The existing owner command ledger provides one durable replay boundary.
// It never replaces domain state or returns success before the transaction commits.
func (a *App) worldCommand(ctx context.Context, owner, fid, operation string, body map[string]any, apply func(context.Context, pgx.Tx) (map[string]any, error)) (map[string]any, error) {
	key, reason := strings.TrimSpace(stringValue(body["idempotency_key"])), strings.TrimSpace(stringValue(body["reason"]))
	if key == "" || len(key) > 256 || reason == "" || len([]rune(reason)) > 1000 {
		return nil, invalid("idempotencyKey and a bounded reason are required")
	}
	digest := stableDigest(operation + ":" + jsonString(body))
	auditID := randomID("audit_world_")
	ctx = context.WithValue(ctx, worldCommandAuditKey{}, auditID)
	var result map[string]any
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if fid != "" {
			if err := lockLifeContextTx(ctx, tx, fid); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, owner+":"+key); err != nil {
			return err
		}
		var savedDigest string
		var raw []byte
		err := tx.QueryRow(ctx, `SELECT digest,response FROM public.owner_surface_idempotency WHERE owner_actor_id=$1 AND idempotency_key=$2`, owner, key).Scan(&savedDigest, &raw)
		if err == nil {
			if savedDigest != digest {
				return &SurfaceError{Status: 409, Code: "idempotency_conflict", Message: "This command key belongs to another operation"}
			}
			result = decodeObject(raw)
			result["_replay"] = true
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if fid != "" {
			if err := requireOwnerMutableTx(ctx, tx, owner, fid); err != nil {
				return err
			}
		}
		result, err = apply(ctx, tx)
		if err != nil {
			return err
		}
		auditAction, _ := ctx.Value(ownerCommandAuditActionKey{}).(string)
		if auditAction == "" {
			auditAction = "world_runtime"
		}
		if _, err = tx.Exec(ctx, `INSERT INTO public.owner_surface_audits(id,owner_actor_id,fluctlight_id,identity,actor,action,reason,record_id) VALUES($1,$2,$3,'owner_edit','owner',$4,$5,$6)`, auditID, owner, fid, auditAction, reason, stringValue(result["id"])); err != nil {
			return err
		}
		result["audit_id"] = auditID
		_, err = tx.Exec(ctx, `INSERT INTO public.owner_surface_idempotency(owner_actor_id,idempotency_key,digest,response) VALUES($1,$2,$3,$4)`, owner, key, digest, jsonBytes(result))
		return err
	})
	return result, err
}

type worldCommandAuditKey struct{}
type ownerCommandAuditActionKey struct{}

func worldIDWith(ctx context.Context, q lifeContextQuerier, fid string) (string, error) {
	var id string
	err := q.QueryRow(ctx, `SELECT world_id FROM public.fluctlight_world_bindings WHERE fluctlight_id=$1`, fid).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", &SurfaceError{Status: 422, Code: "world_not_configured", Message: "Create a runtime world before adding places or NPCs"}
	}
	return id, err
}

func (a *App) WorldSnapshot(ctx context.Context, owner, fid string) (map[string]any, error) {
	return a.worldSnapshot(ctx, owner, fid, "")
}

func (a *App) worldSnapshot(ctx context.Context, owner, fid, selected string) (map[string]any, error) {
	if _, err := a.DB.GetFluctlight(ctx, fid, owner); err != nil {
		return nil, err
	}
	result := map[string]any{"available": true, "world": nil, "places": []any{}, "npcs": []any{}, "scenes": []any{}}
	var raw []byte
	err := a.DB.Pool().QueryRow(ctx, `SELECT jsonb_build_object('id',id,'fluctlight_id',fluctlight_id,'name',name,'background',background,'rules',rules,'flavor',flavor,'description',description,'item_pool',item_pool,'revision',revision,'source_resource_id',source_resource_id,'source_version',source_version) FROM public.life_worlds WHERE fluctlight_id=$1 AND id=COALESCE(NULLIF($2,''),(SELECT world_id FROM public.fluctlight_world_bindings WHERE fluctlight_id=$1))`, fid, selected).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		if selected != "" {
			return nil, ErrNotFound
		}
		return result, nil
	}
	if err != nil {
		return nil, err
	}
	world := decodeObject(raw)
	result["world"] = world
	places, err := readWorldObjects(ctx, a.DB.Pool(), `SELECT jsonb_build_object('id',id,'world_id',world_id,'name',name,'desc',description,'parent_place_id',parent_place_id,'status',status,'revision',revision,'source_resource_id',source_resource_id,'source_version',source_version) || details FROM public.world_places WHERE world_id=$1 ORDER BY name,id`, world["id"])
	if err != nil {
		return nil, err
	}
	result["places"] = places
	npcs, err := a.readWorldNPCs(ctx, fid, stringValue(world["id"]))
	if err != nil {
		return nil, err
	}
	result["npcs"] = npcs
	_, current, err := a.readLifeContextSnapshotAt(ctx, fid, a.now())
	if err != nil {
		return nil, err
	}
	result["current_context"] = current
	effects, err := activeWorldEffectsWith(ctx, a.DB.Pool(), fid, stringValue(world["id"]), "*", a.now().UTC())
	if err != nil {
		return nil, err
	}
	result["active_effects"] = effects
	return result, nil
}

func readWorldObjects(ctx context.Context, q lifeContextQuerier, sql string, args ...any) ([]any, error) {
	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []any{}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		out = append(out, decodeObject(raw))
	}
	return out, rows.Err()
}

func boundedWorldText(body map[string]any, key string, max int, required bool) (string, error) {
	value, ok := body[key].(string)
	value = strings.TrimSpace(value)
	if (!ok && body[key] != nil) || len([]rune(value)) > max || (required && value == "") {
		return "", invalid(key + " is invalid")
	}
	return value, nil
}

func (a *App) writeWorldTx(ctx context.Context, tx pgx.Tx, fid string, body map[string]any) (map[string]any, error) {
	name, err := boundedWorldText(body, "name", 256, true)
	if err != nil {
		return nil, err
	}
	background, err := boundedWorldText(body, "background", 16000, false)
	if err != nil {
		return nil, err
	}
	rules, err := boundedWorldText(body, "rules", 8000, false)
	if err != nil {
		return nil, err
	}
	expected, ok := parseRevision(body["expected_revision"])
	if !ok {
		return nil, invalid("expectedRevision is required")
	}
	var id string
	var revision int
	err = tx.QueryRow(ctx, `SELECT id,revision FROM public.life_worlds WHERE id=COALESCE(NULLIF($2,''),(SELECT world_id FROM public.fluctlight_world_bindings WHERE fluctlight_id=$1)) AND fluctlight_id=$1 FOR UPDATE`, fid, stringValue(body["world_id"])).Scan(&id, &revision)
	if errors.Is(err, pgx.ErrNoRows) {
		if expected != 0 {
			return nil, conflict("world", expected, 0)
		}
		if stringValue(body["world_id"]) != "" {
			return nil, ErrNotFound
		}
		id = randomID("world_")
		revision = 1
		_, err = tx.Exec(ctx, `INSERT INTO public.life_worlds(id,fluctlight_id,name,background,rules) VALUES($1,$2,$3,$4,$5)`, id, fid, name, background, rules)
	} else if err == nil {
		if revision != expected {
			return nil, conflict("world", expected, revision)
		}
		revision++
		_, err = tx.Exec(ctx, `UPDATE public.life_worlds SET name=$2,background=CASE WHEN $6 THEN $3 ELSE background END,rules=CASE WHEN $7 THEN $4 ELSE rules END,revision=$5,updated_at=now() WHERE id=$1`, id, name, background, rules, revision, body["background"] != nil, body["rules"] != nil)
	}
	if err != nil {
		return nil, err
	}
	if body["catalog_create"] != true {
		if _, err = tx.Exec(ctx, `INSERT INTO public.fluctlight_world_bindings(fluctlight_id,world_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, fid, id); err != nil {
			return nil, err
		}
	}
	if err = writeWorldExtrasTx(ctx, tx, id, body); err != nil {
		return nil, err
	}
	return map[string]any{"id": id, "revision": revision, "status": "saved"}, nil
}

func (a *App) writeWorldPlaceTx(ctx context.Context, tx pgx.Tx, fid, id, method string, body map[string]any) (map[string]any, error) {
	world := stringValue(body["world_id"])
	var err error
	if world == "" {
		world, err = worldIDWith(ctx, tx, fid)
		if err != nil {
			return nil, err
		}
	} else {
		var owned bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.life_worlds WHERE id=$1 AND fluctlight_id=$2)`, world, fid).Scan(&owned); err != nil {
			return nil, err
		}
		if !owned {
			return nil, ErrNotFound
		}
	}
	if method == "DELETE" {
		var revision int
		if err := tx.QueryRow(ctx, `SELECT revision FROM public.world_places WHERE id=$1 AND world_id=$2 FOR UPDATE`, id, world).Scan(&revision); err != nil {
			return nil, err
		}
		expected, ok := parseRevision(body["expected_revision"])
		if !ok || expected != revision {
			return nil, conflict("place", expected, revision)
		}
		referenced, err := worldPlaceReferenced(ctx, tx, world, id)
		if err != nil {
			return nil, err
		}
		if referenced {
			return nil, worldReferenced("Place has stable runtime or historical references")
		}
		_, err = tx.Exec(ctx, `DELETE FROM public.world_places WHERE id=$1 AND world_id=$2`, id, world)
		return map[string]any{"id": id, "status": "deleted"}, err
	}
	name, err := boundedWorldText(body, "name", 256, true)
	if err != nil {
		return nil, err
	}
	description, err := boundedWorldText(body, "description", 8000, false)
	if err != nil {
		return nil, err
	}
	parent := stringValue(body["parent_place_id"])
	if parent != "" {
		if parent == id {
			return nil, invalid("a place cannot contain itself")
		}
		var found bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.world_places WHERE world_id=$1 AND id=$2 AND status='active')`, world, parent).Scan(&found); err != nil {
			return nil, err
		}
		if !found {
			return nil, invalid("parent place is outside this world")
		}
	}
	if parent != "" && id != "" {
		var cyclic bool
		if err := tx.QueryRow(ctx, `WITH RECURSIVE ancestors AS (SELECT id,parent_place_id FROM public.world_places WHERE id=$1 AND world_id=$2 UNION SELECT p.id,p.parent_place_id FROM public.world_places p JOIN ancestors a ON p.id=a.parent_place_id WHERE p.world_id=$2) SELECT EXISTS(SELECT 1 FROM ancestors WHERE id=$3)`, parent, world, id).Scan(&cyclic); err != nil {
			return nil, err
		}
		if cyclic {
			return nil, invalid("place hierarchy would contain a cycle")
		}
	}
	details, err := validatePlaceDetails(ctx, tx, world, id, body)
	if err != nil {
		return nil, err
	}
	if id == "" && method == "POST" {
		id = randomID("place_")
		_, err = tx.Exec(ctx, `INSERT INTO public.world_places(id,world_id,name,description,parent_place_id,details) VALUES($1,$2,$3,$4,$5,$6)`, id, world, name, description, nullableString(parent), jsonBytes(details))
		if err != nil {
			return nil, err
		}
		place, err := readDefaultPlaceWith(ctx, tx, fid, world, id, false)
		if err != nil {
			return nil, err
		}
		baseline := map[string]any{"source_kind": "initial_saved", "definition": placeDefaultsFields(place), "source_resource_id": nil, "source_version": nil}
		if err := writePlaceDefaultTx(ctx, tx, place, baseline, "地点首次保存的配置字段", "initial_saved", nil); err != nil {
			return nil, err
		}
		return map[string]any{"id": id, "revision": 1, "status": "saved"}, nil
	}
	expected, ok := parseRevision(body["expected_revision"])
	if !ok {
		return nil, invalid("expectedRevision is required")
	}
	var revision int
	var beforeRaw []byte
	if err := tx.QueryRow(ctx, `SELECT revision,to_jsonb(p) FROM public.world_places p WHERE id=$1 AND world_id=$2 FOR UPDATE`, id, world).Scan(&revision, &beforeRaw); err != nil {
		return nil, err
	}
	if expected != revision {
		return nil, conflict("place", expected, revision)
	}
	status := stringValue(body["status"])
	if status == "" {
		status = "active"
	}
	if status != "active" && status != "archived" {
		return nil, ErrInvalidArguments
	}
	if status == "archived" {
		referenced, err := worldPlaceReferenced(ctx, tx, world, id)
		if err != nil {
			return nil, err
		}
		if referenced {
			return nil, &SurfaceError{Status: 409, Code: "resource_referenced", Message: "This place has NPC, child-place, or historical references"}
		}
	}
	_, err = tx.Exec(ctx, `UPDATE public.world_places SET name=$3,description=$4,parent_place_id=$5,status=$6,details=$7,revision=revision+1,updated_at=now() WHERE id=$1 AND world_id=$2`, id, world, name, description, nullableString(parent), status, jsonBytes(details))
	if err != nil {
		return nil, err
	}
	if err := recordPlaceFieldEditTx(ctx, tx, fid, world, id, decodeObject(beforeRaw), stringValue(body["reason"])); err != nil {
		return nil, err
	}
	return map[string]any{"id": id, "revision": revision + 1, "status": "saved"}, nil
}

func parseNPCClock(value string) (int, bool) {
	t, err := time.Parse("15:04", value)
	if err != nil {
		return 0, false
	}
	return t.Hour()*60 + t.Minute(), true
}

// Replay the committed command before an external execution lock. The normal
// command transaction still repeats this check to serialize first admissions.
func (a *App) ownerWorldCommandReplay(ctx context.Context, owner, operation string, body map[string]any) (map[string]any, bool, error) {
	var digest string
	var raw []byte
	err := a.DB.Pool().QueryRow(ctx, `SELECT digest,response FROM public.owner_surface_idempotency WHERE owner_actor_id=$1 AND idempotency_key=$2`, owner, stringValue(body["idempotency_key"])).Scan(&digest, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if digest != stableDigest(operation+":"+jsonString(body)) {
		return nil, true, &SurfaceError{Status: 409, Code: "idempotency_conflict", Message: "该原提交身份已属于其他内容"}
	}
	result := decodeObject(raw)
	result["_replay"] = true
	return result, true, nil
}
