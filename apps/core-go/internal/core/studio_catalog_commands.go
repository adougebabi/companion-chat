package core

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func (a *App) OwnerStudioVersions(ctx context.Context, owner, id string, query url.Values) (map[string]any, error) {
	var found bool
	if err := a.DB.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.owner_surface_records WHERE owner_actor_id=$1 AND id=$2 AND kind='studio_resource')`, owner, id).Scan(&found); err != nil {
		return nil, err
	}
	if !found {
		return nil, ErrNotFound
	}
	at, version, ok := decodeCursor(query.Get("cursor"))
	if !ok {
		return nil, invalid("Invalid publication cursor")
	}
	var cursorAt any
	if query.Get("cursor") != "" {
		cursorAt = at
	}
	limit := pageLimit(query)
	rows, err := a.DB.Pool().Query(ctx, `SELECT version,kind,name,created_at FROM public.studio_resource_versions WHERE owner_actor_id=$1 AND resource_id=$2 AND ($3::timestamptz IS NULL OR (created_at,version)<($3,$4)) ORDER BY created_at DESC,version DESC LIMIT $5`, owner, id, cursorAt, version, limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []any{}
	var times []time.Time
	for rows.Next() {
		var v, k, n string
		var created time.Time
		if err := rows.Scan(&v, &k, &n, &created); err != nil {
			return nil, err
		}
		times = append(times, created)
		items = append(items, map[string]any{"id": id, "version": v, "kind": k, "name": n, "created_at": created.UTC().Format(time.RFC3339Nano)})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	var next any
	if len(items) > limit {
		next = encodeCursor(times[limit-1], stringValue(mapValue(items[limit-1])["version"]))
		items = items[:limit]
	}
	return map[string]any{"items": items, "next_cursor": next}, nil
}

func (a *App) studioCatalogCommand(ctx context.Context, owner, id, operation string, body map[string]any) (map[string]any, error) {
	ctx = context.WithValue(ctx, ownerCommandAuditActionKey{}, "studio_resource_"+operation)
	return a.worldCommand(ctx, owner, "", "studio-resource:"+id+":"+operation, body, func(ctx context.Context, tx pgx.Tx) (map[string]any, error) {
		if err := lockStudioReferenceGraphTx(ctx, tx, owner); err != nil {
			return nil, err
		}
		if !eventKeys(body, "expected_revision", "reason", "idempotency_key", "source", "source_version", "name") {
			return nil, invalid("Unknown resource command fields")
		}
		rec, ok, err := (postgresSurfaceTx{tx}).Get(ctx, owner, id)
		if err != nil {
			return nil, err
		}
		if !ok || rec.Kind != "studio_resource" {
			return nil, ErrNotFound
		}
		expected, valid := parseRevision(body["expected_revision"])
		if !valid || expected != rec.Revision {
			return nil, conflict("studio_resource", expected, rec.Revision)
		}
		if operation == "copy" {
			name := strings.TrimSpace(stringValue(body["name"]))
			if name == "" || len([]rune(name)) > 200 {
				return nil, invalid("Copy name must contain 1..200 characters")
			}
			version := stringValue(body["source_version"])
			var definition map[string]any
			var kind string
			switch body["source"] {
			case "draft":
				if version != stringValue(rec.Payload["draft_version"]) {
					return nil, conflict("studio_resource", expected, rec.Revision)
				}
				definition = mapValue(rec.Payload["body"])
				kind = stringValue(rec.Payload["kind"])
			case "published":
				p, err := readWorldPublication(ctx, tx, owner, id, version)
				if err != nil {
					return nil, err
				}
				definition = p.Body
				kind = p.Kind
			default:
				return nil, invalid("Copy source must be an explicit draft or publication")
			}
			if containsPermanentHarm(definition) {
				return nil, harmError()
			}
			now := a.now()
			copyID := randomID("res_")
			copied := surfaceRecord{ID: copyID, OwnerID: owner, Kind: "studio_resource", ResourceKey: copyID, Status: "draft", Revision: 1, CreatedAt: now, UpdatedAt: now, Payload: map[string]any{"name": name, "kind": kind, "body": cloneMap(definition), "draft_version": "1", "published_version": nil, "referenced_by": []any{}, "versions": []any{map[string]any{"version": "1", "created_at": now.UTC().Format(time.RFC3339Nano)}}, "copied_from": map[string]any{"resource_id": id, "version": version, "source": body["source"], "source_revision": rec.Revision}}}
			if err := (postgresSurfaceTx{tx}).Save(ctx, copied); err != nil {
				return nil, err
			}
			return (&OwnerSurface{}).resourceView(copied, ""), nil
		}
		if body["source"] != nil || body["source_version"] != nil || body["name"] != nil {
			return nil, invalid("Archive/restore cannot change the definition")
		}
		switch operation {
		case "archive":
			if rec.Status == "archived" {
				return nil, invalid("Resource is already archived")
			}
			var referenced bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.life_worlds WHERE source_resource_id=$1)
			 OR EXISTS(SELECT 1 FROM public.world_places WHERE source_resource_id=$1)
			 OR EXISTS(SELECT 1 FROM public.world_npcs WHERE source_resource_id=$1)
			 OR EXISTS(SELECT 1 FROM public.world_event_templates WHERE source_resource_id=$1)
             OR EXISTS(SELECT 1 FROM public.world_items WHERE source_resource_id=$1)
             OR EXISTS(SELECT 1 FROM public.fluctlight_wardrobe_outfits WHERE source_resource_id=$1)
             OR EXISTS(SELECT 1 FROM public.studio_resource_versions v JOIN public.owner_surface_records parent ON parent.id=v.resource_id AND parent.owner_actor_id=v.owner_actor_id WHERE v.owner_actor_id=$2 AND parent.id<>$1 AND v.kind='outfit' AND parent.status<>'archived' AND jsonb_path_exists(v.body,'$.members[*].resource_id ? (@ == $id)',jsonb_build_object('id',$1::text)))
             OR EXISTS(SELECT 1 FROM public.fluctlight_wardrobe_items WHERE source_resource_id=$1)
             OR EXISTS(SELECT 1 FROM public.fluctlight_visual_briefs WHERE source_resource_id=$1)
             OR EXISTS(SELECT 1 FROM public.fluctlight_foundation_revisions WHERE source_resource_id=$1 AND status IN ('proposed','accepted'))
			 OR EXISTS(SELECT 1 FROM public.fluctlight_initialization_sources s JOIN public.fluctlight_initialization_source_links l ON l.source_id=s.id WHERE s.owner_actor_id=$2 AND s.classification_evidence->'template_source'->>'resource_id'=$1)
			 OR EXISTS(SELECT 1 FROM public.owner_surface_records r WHERE r.owner_actor_id=$2 AND r.kind='studio_resource' AND r.id<>$1 AND r.status<>'archived' AND jsonb_path_exists(r.payload->'body','$.**.resource_id ? (@ == $id)',jsonb_build_object('id',$1::text)))`, id, owner).Scan(&referenced); err != nil {
				return nil, err
			}
			if referenced {
				return nil, worldReferenced("Resource is referenced by an instance or another active definition")
			}
			rec.Status = "archived"
			rec.Payload["archived_at"] = a.now().UTC().Format(time.RFC3339Nano)
		case "restore":
			if rec.Status != "archived" {
				return nil, invalid("Resource is not archived")
			}
			rec.Status = "draft"
			if stringValue(rec.Payload["published_version"]) != "" {
				rec.Status = "published"
			}
			delete(rec.Payload, "archived_at")
		default:
			return nil, errors.New("studio_catalog_operation_invalid")
		}
		rec.Revision++
		rec.UpdatedAt = a.now()
		if err := (postgresSurfaceTx{tx}).Save(ctx, rec); err != nil {
			return nil, err
		}
		return (&OwnerSurface{}).resourceView(rec, ""), nil
	})
}

func activeStudioResource(status string) error {
	if status == "archived" {
		return &SurfaceError{Status: 409, Code: "studio_resource_archived", Message: "Restore the archived resource before using or editing it"}
	}
	return nil
}
