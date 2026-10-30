package core

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func (a *App) applyWorldEventTemplateTx(ctx context.Context, tx pgx.Tx, fid string, p worldPublication, current, values map[string]any) (map[string]any, error) {
	world, err := worldIDWith(ctx, tx, fid)
	if err != nil {
		return nil, err
	}
	d := mapValue(values["definition"])
	if err = validateWorldEventDefinition(d); err != nil {
		return nil, err
	}
	if err = validateWorldEventReferencesTx(ctx, tx, world, d); err != nil {
		return nil, err
	}
	id := stringValue(current["id"])
	if id == "" {
		id = randomID("world_template_")
	}
	var revision int
	var enabled bool
	err = tx.QueryRow(ctx, `INSERT INTO public.world_event_templates(id,world_id,source_resource_id,source_version,name,definition,next_check_at) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(world_id,source_resource_id) DO UPDATE SET source_version=EXCLUDED.source_version,name=EXCLUDED.name,definition=EXCLUDED.definition,revision=world_event_templates.revision+1,next_check_at=EXCLUDED.next_check_at,error_code=NULL,updated_at=now() RETURNING revision, enabled`, id, world, p.ID, p.Version, values["name"], jsonBytes(d), a.now().UTC()).Scan(&revision, &enabled)
	return map[string]any{"id": id, "revision": revision, "world_id": world, "enabled": enabled, "life_write": false}, err
}
func readWorldTemplateWith(ctx context.Context, q lifeContextQuerier, fid, id string, lock bool) (map[string]any, error) {
	sql := `SELECT to_jsonb(t) FROM public.world_event_templates t JOIN public.life_worlds w ON w.id=t.world_id WHERE w.owner_actor_id=(SELECT created_by_actor_id FROM public.fluctlights WHERE id=$1) AND t.id=$2`
	if lock {
		sql += " FOR UPDATE OF t"
	}
	var raw []byte
	err := q.QueryRow(ctx, sql, fid, id).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return decodeObject(raw), err
}
func (a *App) worldTemplateSurface(ctx context.Context, method string, p []string, q url.Values, owner, fid string, body map[string]any) (map[string]any, error) {
	if method == "GET" {
		if len(p) == 4 {
			world, err := worldIDWith(ctx, a.DB.Pool(), fid)
			if err != nil {
				return nil, err
			}
			items, err := readWorldObjects(ctx, a.DB.Pool(), `SELECT to_jsonb(t)||jsonb_build_object('template_id',t.id,'category',t.definition->>'category','permanent_harm',false,'locked',false) FROM public.world_event_templates t WHERE world_id=$1 AND generation_id IS NULL ORDER BY name,id LIMIT 201`, world)
			if err != nil {
				return nil, err
			}
			if len(items) > 200 {
				return nil, invalid("event catalogue exceeds bounded settings view; use the studio catalogue")
			}
			return map[string]any{"items": items, "world_id": world, "catalog_status": "runtime", "runtime_effect": "worker_executed"}, nil
		}
		t, err := readWorldTemplateWith(ctx, a.DB.Pool(), fid, p[4], false)
		if err != nil {
			return nil, err
		}
		if len(p) == 5 {
			return t, nil
		}
		if len(p) == 7 {
			return a.worldEventRun(ctx, owner, fid, p[4], p[6])
		}
		return a.worldEventRuns(ctx, owner, fid, t, q)
	}
	return a.worldCommand(ctx, owner, fid, method+":world-template:"+strings.Join(p, "/"), body, func(ctx context.Context, tx pgx.Tx) (map[string]any, error) {
		t, err := readWorldTemplateWith(ctx, tx, fid, p[4], true)
		if err != nil {
			return nil, err
		}
		expected, ok := parseRevision(body["expected_revision"])
		if !ok || expected != intValue(t["revision"]) {
			return nil, conflict("world_event_template", expected, intValue(t["revision"]))
		}
		world, err := worldIDWith(ctx, tx, fid)
		if err != nil {
			return nil, err
		}
		if t["world_id"] != world {
			return nil, invalid("select this template's world before changing its runtime")
		}
		if method == "PUT" && len(p) == 5 {
			enabled, ok := body["enabled"].(bool)
			if !ok {
				return nil, invalid("enabled must be a boolean")
			}
			if err := validateWorldEventDefinition(mapValue(t["definition"])); err != nil {
				return nil, err
			}
			_, err = tx.Exec(ctx, `UPDATE public.world_event_templates SET enabled=$2,revision=revision+1,next_check_at=$3,error_code=NULL,updated_at=now() WHERE id=$1`, t["id"], enabled, a.now().UTC())
			return map[string]any{"id": t["id"], "template_id": t["id"], "enabled": enabled, "revision": expected + 1, "runtime_effect": "worker_executed"}, err
		}
		if len(p) == 8 && p[7] == "retry" {
			r, err := readWorldEventRunWith(ctx, tx, owner, fid, p[4], p[6], true)
			if err != nil {
				return nil, err
			}
			expectedRun, ok := parseRevision(body["expected_run_revision"])
			if !ok || expectedRun != intValue(r["revision"]) {
				return nil, conflict("world_event_run", expectedRun, intValue(r["revision"]))
			}
			if r["status"] != "failed" {
				return nil, invalid("only failed executions can be retried")
			}
			expires, err := time.Parse(time.RFC3339Nano, stringValue(r["expires_at"]))
			if err != nil {
				return nil, err
			}
			if !expires.After(a.now()) {
				return nil, invalid("the original execution window expired; create a new controlled trigger")
			}
			if t["enabled"] != true || intValue(t["revision"]) != intValue(r["template_revision"]) {
				return nil, invalid("the original enabled template revision is no longer current")
			}
			_, err = tx.Exec(ctx, `UPDATE public.world_event_runs SET status='queued',error_code=NULL,revision=revision+1,updated_at=now() WHERE id=$1`, r["id"])
			return map[string]any{"id": r["id"], "status": "queued", "revision": expectedRun + 1}, err
		}
		if len(p) != 6 || p[5] != "runs" || method != "POST" {
			return nil, ErrInvalidArguments
		}
		if t["enabled"] != true {
			return nil, invalid("enable the applied runtime template before triggering")
		}
		due := a.now().UTC()
		if body["scheduled_at"] != nil {
			due, err = time.Parse(time.RFC3339Nano, stringValue(body["scheduled_at"]))
			if err != nil || due.Before(a.now().Add(-time.Minute)) || due.After(a.now().Add(30*24*time.Hour)) {
				return nil, invalid("scheduledAt must be an instant within the next 30 days")
			}
		}
		return a.queueWorldEventTx(ctx, tx, owner, fid, t, "manual", "owner-world-event:"+owner+":"+fid+":"+stringValue(body["idempotency_key"]), due)
	})
}
func (a *App) queueWorldEventTx(ctx context.Context, tx pgx.Tx, owner, fid string, t map[string]any, trigger, key string, due time.Time) (map[string]any, error) {
	d := mapValue(t["definition"])
	if err := validateWorldEventDefinition(d); err != nil {
		return nil, err
	}
	id := "world_event_" + stableDigest(fid+":"+key)
	var raw []byte
	err := tx.QueryRow(ctx, `INSERT INTO public.world_event_runs(id,world_id,template_id,fluctlight_id,owner_actor_id,template_revision,definition,trigger_kind,source_key,status,due_at,expires_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,'queued',$10,$11) ON CONFLICT(source_key) DO UPDATE SET source_key=EXCLUDED.source_key RETURNING to_jsonb(world_event_runs)`, id, t["world_id"], t["id"], nullableString(fid), owner, t["revision"], jsonBytes(d), trigger, key, due, due.Add(30*time.Minute)).Scan(&raw)
	return decodeObject(raw), err
}
func readWorldEventRunWith(ctx context.Context, q lifeContextQuerier, owner, fid, template, id string, lock bool) (map[string]any, error) {
	sql := `SELECT to_jsonb(r) FROM public.world_event_runs r WHERE owner_actor_id=$1 AND (fluctlight_id=$2 OR fluctlight_id IS NULL AND world_id=(SELECT world_id FROM public.fluctlight_world_bindings WHERE fluctlight_id=$2)) AND template_id=$3 AND id=$4`
	if lock {
		sql += " FOR UPDATE"
	}
	var raw []byte
	err := q.QueryRow(ctx, sql, owner, fid, template, id).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return decodeObject(raw), err
}
func (a *App) worldEventRun(ctx context.Context, owner, fid, template, id string) (map[string]any, error) {
	return readWorldEventRunWith(ctx, a.DB.Pool(), owner, fid, template, id, false)
}
func (a *App) worldEventRuns(ctx context.Context, owner, fid string, t map[string]any, q url.Values) (map[string]any, error) {
	limit, err := reflectionReadPage(q)
	if err != nil {
		return nil, err
	}
	scope := stableDigest(jsonString([]any{"world-events", owner, fid, t["id"]}))
	cursor, err := decodeOwnerReadCursor(q.Get("cursor"), scope)
	if err != nil || (q.Get("cursor") != "" && (cursor.ID == "" || cursor.Time.IsZero())) {
		return nil, invalid("invalid world event cursor")
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT id,created_at,to_jsonb(r) FROM public.world_event_runs r WHERE owner_actor_id=$1 AND (fluctlight_id=$2 OR fluctlight_id IS NULL AND world_id=(SELECT world_id FROM public.fluctlight_world_bindings WHERE fluctlight_id=$2)) AND template_id=$3 AND ($4='' OR (created_at,id)<($5,$4)) ORDER BY created_at DESC,id DESC LIMIT $6`, owner, fid, t["id"], cursor.ID, cursor.Time, limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return readGovernanceRows(rows, scope, limit)
}
