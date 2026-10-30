package core

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

func worldResourceKind(kind string) bool {
	return kind == "world" || kind == "place" || kind == "npc" || kind == "event_template"
}

func (a *App) publishWorldResource(ctx context.Context, owner, id string, body map[string]any) (map[string]any, error) {
	ctx = context.WithValue(ctx, ownerCommandAuditActionKey{}, "studio_resource_publish")
	return a.worldCommand(ctx, owner, "", "publish:"+id, body, func(ctx context.Context, tx pgx.Tx) (map[string]any, error) {
		if err := lockStudioReferenceGraphTx(ctx, tx, owner); err != nil {
			return nil, err
		}
		rec, ok, err := (postgresSurfaceTx{tx}).Get(ctx, owner, id)
		if err != nil {
			return nil, err
		}
		if !ok || rec.Kind != "studio_resource" {
			return nil, ErrNotFound
		}
		if err := activeStudioResource(rec.Status); err != nil {
			return nil, err
		}
		kind := stringValue(rec.Payload["kind"])
		if !worldResourceKind(kind) && kind != "character_template" && kind != "visual_style" && kind != "persona_fragment" && kind != "item" && kind != "outfit" {
			return nil, invalid("This resource kind does not have a publisher")
		}
		expected, valid := parseRevision(body["expected_revision"])
		if !valid || expected != rec.Revision {
			return nil, conflict("studio_resource", expected, rec.Revision)
		}
		definition := mapValue(rec.Payload["body"])
		if containsPermanentHarm(definition) {
			return nil, harmError()
		}
		if kind == "world" {
			if err := validateWorldFramework(definition); err != nil {
				return nil, err
			}
		}
		if kind == "event_template" {
			if err := validateWorldEventDefinition(definition); err != nil {
				return nil, err
			}
		}
		if kind == "visual_style" {
			if err := validateVisualStyle(definition); err != nil {
				return nil, err
			}
		}
		if kind == "character_template" {
			if _, err := characterTemplateSource(definition); err != nil {
				return nil, err
			}
		}
		if kind == "persona_fragment" {
			if _, err := foundationDefinitionChanges(worldPublication{Kind: kind, Body: definition}); err != nil {
				return nil, err
			}
		}
		if kind == "item" {
			if err := validateItemDefinition(definition); err != nil {
				return nil, err
			}
		}
		if kind == "outfit" {
			if len([]rune(stringValue(rec.Payload["name"]))) > 128 {
				return nil, invalid("Outfit name must be at most 128 characters")
			}
			if _, err := outfitPublishedMembers(ctx, tx, owner, definition); err != nil {
				return nil, err
			}
			for _, raw := range arrayValue(definition["members"]) {
				if err := requireActiveStudioMemberTx(ctx, tx, owner, stringValue(mapValue(raw)["resource_id"])); err != nil {
					return nil, err
				}
			}
		}
		version := stringValue(rec.Payload["draft_version"])
		if version == "" {
			return nil, invalid("Save a draft before publishing")
		}
		if kind != "character_template" && kind != "visual_style" && kind != "persona_fragment" && kind != "item" && kind != "outfit" {
			for key, limit := range map[string]int{"background": 16000, "rules": 8000, "description": 8000, "role": 512} {
				if _, err := boundedWorldText(definition, key, limit, false); err != nil {
					return nil, err
				}
			}
		}
		var existing []byte
		err = tx.QueryRow(ctx, `SELECT body FROM public.studio_resource_versions WHERE resource_id=$1 AND version=$2`, id, version).Scan(&existing)
		if err == nil {
			if stableDigest(jsonString(decodeObject(existing))) != stableDigest(jsonString(definition)) {
				return nil, &SurfaceError{Status: 409, Code: "published_version_immutable", Message: "Save another draft version before changing a publication"}
			}
		} else if errors.Is(err, pgx.ErrNoRows) {
			if _, err = tx.Exec(ctx, `INSERT INTO public.studio_resource_versions(resource_id,version,owner_actor_id,kind,name,body) VALUES($1,$2,$3,$4,$5,$6)`, id, version, owner, kind, stringValue(rec.Payload["name"]), jsonBytes(definition)); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
		rec.Payload["published_version"] = version
		rec.Status = "published"
		rec.Revision++
		rec.UpdatedAt = a.now()
		versions := arrayValue(rec.Payload["versions"])
		for _, raw := range versions {
			v := mapValue(raw)
			if stringValue(v["version"]) == version {
				v["body"] = definition
			}
		}
		rec.Payload["versions"] = versions
		if err := (postgresSurfaceTx{tx}).Save(ctx, rec); err != nil {
			return nil, err
		}
		return map[string]any{"id": id, "published_version": version, "revision": rec.Revision, "status": "published", "life_write": false}, nil
	})
}

type worldPublication struct {
	ID, Version, Kind, Name string
	Body                    map[string]any
}

func readWorldPublication(ctx context.Context, q lifeContextQuerier, owner, id, version string) (worldPublication, error) {
	var kind, name string
	var raw []byte
	err := q.QueryRow(ctx, `SELECT kind,name,body FROM public.studio_resource_versions WHERE owner_actor_id=$1 AND resource_id=$2 AND version=$3`, owner, id, version).Scan(&kind, &name, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return worldPublication{}, &SurfaceError{Status: 409, Code: "resource_not_published", Message: "Publish this exact resource version before applying it"}
	}
	return worldPublication{ID: id, Version: version, Kind: kind, Name: name, Body: decodeObject(raw)}, err
}

// DispatchWorldApply retains the old record-only contract for other kinds.
func (a *App) DispatchWorldApply(ctx context.Context, owner, id string, body map[string]any, preview bool) (map[string]any, error) {
	var raw []byte
	var status string
	if err := a.DB.Pool().QueryRow(ctx, `SELECT payload,status FROM public.owner_surface_records WHERE id=$1 AND owner_actor_id=$2 AND kind='studio_resource'`, id, owner).Scan(&raw, &status); err != nil {
		return nil, err
	}
	if err := activeStudioResource(status); err != nil {
		return nil, err
	}
	// Publications are immutable typed sources. A later draft can change its
	// kind without changing the consumer for a previously published version.
	publication, err := readWorldPublication(ctx, a.DB.Pool(), owner, id, stringValue(body["resource_version"]))
	if err != nil {
		return nil, err
	}
	if publication.Kind == "visual_style" {
		return a.applyStudioVisualStyle(ctx, owner, id, body, preview)
	}
	if publication.Kind == "item" {
		return a.applyStudioItem(ctx, owner, id, body, publication, preview)
	}
	if publication.Kind == "outfit" {
		return a.applyStudioOutfit(ctx, owner, id, body, publication, preview)
	}
	if publication.Kind == "persona_fragment" || publication.Kind == "character_template" {
		return a.applyStudioFoundation(ctx, owner, id, body, publication, preview)
	}
	if !worldResourceKind(publication.Kind) {
		operation := "apply"
		if preview {
			operation = "apply-preview"
		}
		return a.DispatchOwnerSurface(ctx, "POST", "/internal/studio/resources/"+id+"/"+operation, nil, owner, body)
	}
	fid := stringValue(body["fluctlight_id"])
	fl, err := a.DB.GetFluctlight(ctx, fid, owner)
	if err != nil {
		return nil, err
	}
	if containsPermanentHarm(body) {
		return nil, harmError()
	}
	if containsPermanentHarm(publication.Body) {
		return nil, harmError()
	}
	if preview {
		current, err := a.worldApplyTarget(ctx, a.DB.Pool(), fid, publication, body)
		if err != nil {
			return nil, err
		}
		diff := worldApplyDiff(current, publication)
		operation := "update"
		if current["id"] == nil {
			operation = "create"
		} else if len(diff) == 0 {
			operation = "unchanged"
		}
		return map[string]any{"available": true, "operation": operation, "create_required": operation == "create", "diff": diff, "conflicts": diff, "instance_revision": fl.CurrentRevision, "runtime_revision": intValue(current["revision"]), "runtime_id": current["id"]}, nil
	}
	return a.worldCommand(ctx, owner, fid, "apply:"+id, body, func(ctx context.Context, tx pgx.Tx) (map[string]any, error) {
		source, found, err := (postgresSurfaceTx{tx}).Get(ctx, owner, id)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, ErrNotFound
		}
		if err := activeStudioResource(source.Status); err != nil {
			return nil, err
		}
		var instanceRevision int
		if err := tx.QueryRow(ctx, `SELECT current_revision FROM public.fluctlights WHERE id=$1 AND created_by_actor_id=$2 FOR UPDATE`, fid, owner).Scan(&instanceRevision); err != nil {
			return nil, err
		}
		expected, ok := parseRevision(body["expected_instance_revision"])
		if !ok || expected != instanceRevision {
			return nil, conflict("fluctlight", expected, instanceRevision)
		}
		current, err := a.worldApplyTarget(ctx, tx, fid, publication, body)
		if err != nil {
			return nil, err
		}
		runtimeExpected, ok := parseRevision(body["expected_runtime_revision"])
		if !ok || runtimeExpected != intValue(current["revision"]) {
			return nil, conflict("world_runtime", runtimeExpected, intValue(current["revision"]))
		}
		values := map[string]any{"name": publication.Name}
		for _, key := range []string{"background", "rules", "description", "role"} {
			values[key] = stringValue(publication.Body[key])
		}
		for _, key := range []string{"framework", "flavor", "item_pool", "alias", "venue_category_id", "group", "floor", "scene", "atmosphere", "furniture", "layout", "connections", "connection_place_ids", "room_items"} {
			if value, ok := publication.Body[key]; ok {
				values[key] = value
			}
		}
		if publication.Kind == "event_template" {
			values["definition"] = publication.Body
		}
		choices := map[string]string{}
		for _, raw := range arrayValue(body["choices"]) {
			choice := mapValue(raw)
			field, decision := stringValue(choice["field"]), stringValue(choice["decision"])
			if decision != "keep" && decision != "override" {
				return nil, invalid("choice must be keep or override")
			}
			choices[field] = decision
		}
		if current["id"] != nil {
			for _, raw := range worldApplyDiff(current, publication) {
				field := stringValue(mapValue(raw)["field"])
				switch choices[field] {
				case "keep":
					values[field] = current[field]
				case "override":
				default:
					return nil, &SurfaceError{Status: 409, Code: "apply_choices_required", Message: "Choose keep or override for every changed runtime field", Details: map[string]any{"field": field}}
				}
			}
		}
		if publication.Kind == "place" && current["id"] != nil {
			values["name"] = current["name"]
		}
		var result map[string]any
		switch publication.Kind {
		case "world":
			values["catalog_create"] = true
			values["expected_revision"] = runtimeExpected
			if current["id"] == nil {
				result, err = a.createCatalogWorldTx(ctx, tx, fid, values)
			} else {
				values["world_id"] = current["id"]
				result, err = a.writeWorldTx(ctx, tx, fid, values)
			}
			if err == nil {
				_, err = tx.Exec(ctx, `UPDATE public.life_worlds SET source_resource_id=$2,source_version=$3 WHERE id=$1`, result["id"], id, publication.Version)
			}
			if err == nil && values["framework"] != nil {
				source := publication
				source.Body = cloneMap(publication.Body)
				source.Body["framework"] = values["framework"]
				var frameResult map[string]any
				frameResult, err = a.materializeWorldFrameworkTx(ctx, tx, fid, stringValue(result["id"]), source)
				if err == nil {
					result["framework_result"] = frameResult
				}
			}
		case "place":
			values["expected_revision"] = runtimeExpected
			values["parent_place_id"] = body["parent_place_id"]
			method, target := "POST", ""
			if current["id"] != nil {
				method = "PUT"
				target = stringValue(current["id"])
			}
			result, err = a.writeWorldPlaceTx(ctx, tx, fid, target, method, values)
			if err == nil {
				_, err = tx.Exec(ctx, `UPDATE public.world_places SET source_resource_id=$2,source_version=$3 WHERE id=$1`, result["id"], id, publication.Version)
				if err == nil && current["id"] == nil {
					var resolved string
					if err := tx.QueryRow(ctx, `SELECT world_id FROM public.world_places WHERE id=$1`, result["id"]).Scan(&resolved); err != nil {
						return nil, err
					}
					place, readErr := readDefaultPlaceWith(ctx, tx, fid, resolved, stringValue(result["id"]), false)
					if readErr != nil {
						return nil, readErr
					}
					baseline, readErr := readPlaceDefaultWith(ctx, tx, place, owner, "current_definition")
					if readErr != nil {
						return nil, readErr
					}
					if err = writePlaceDefaultTx(ctx, tx, place, baseline, "首次应用的精确地点定义版本", "set_baseline", nil); err != nil {
						return nil, err
					}
				}
			}
		case "event_template":
			result, err = a.applyWorldEventTemplateTx(ctx, tx, fid, publication, current, values)
		case "npc":
			result, err = a.applyWorldNPCTx(ctx, tx, fid, publication, current, values, body)
		}
		if err != nil {
			return nil, err
		}
		result["status"] = "applied"
		result["applied"] = true
		result["resource_id"] = id
		result["resource_version"] = publication.Version
		result["instance_revision"] = instanceRevision
		result["runtime_revision"] = result["revision"]
		record, ok, err := (postgresSurfaceTx{tx}).Get(ctx, owner, id)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, ErrNotFound
		}
		refs := []any{}
		for _, raw := range arrayValue(record.Payload["referenced_by"]) {
			ref := mapValue(raw)
			if stringValue(ref["fluctlight_id"]) != fid || (ref["runtime_id"] != nil && ref["runtime_id"] != result["id"]) {
				refs = append(refs, ref)
			}
		}
		record.Payload["referenced_by"] = append(refs, map[string]any{"fluctlight_id": fid, "runtime_id": result["id"], "running_version": publication.Version, "status": "applied", "instance_revision": result["runtime_revision"]})
		record.UpdatedAt = a.now()
		if err := (postgresSurfaceTx{tx}).Save(ctx, record); err != nil {
			return nil, err
		}
		return result, nil
	})
}

func (a *App) worldApplyCurrent(ctx context.Context, q lifeContextQuerier, fid string, p worldPublication) (map[string]any, error) {
	var raw []byte
	var err error
	switch p.Kind {
	case "world":
		err = q.QueryRow(ctx, `SELECT jsonb_build_object('id',id,'name',name,'background',background,'rules',rules,'flavor',flavor,'description',description,'item_pool',item_pool,'revision',revision) FROM public.life_worlds WHERE owner_actor_id=(SELECT created_by_actor_id FROM public.fluctlights WHERE id=$1) AND source_resource_id=$2 AND (fluctlight_id=$1 OR id=(SELECT world_id FROM public.fluctlight_world_bindings WHERE fluctlight_id=$1)) ORDER BY (id=(SELECT world_id FROM public.fluctlight_world_bindings WHERE fluctlight_id=$1)) DESC NULLS LAST,id LIMIT 1`, fid, p.ID).Scan(&raw)
	case "place":
		err = q.QueryRow(ctx, `SELECT jsonb_build_object('id',p.id,'name',p.name,'description',p.description,'revision',p.revision) || p.details FROM public.world_places p JOIN public.life_worlds w ON w.id=p.world_id WHERE w.id=(SELECT world_id FROM public.fluctlight_world_bindings WHERE fluctlight_id=$1) AND p.source_resource_id=$2`, fid, p.ID).Scan(&raw)
	case "event_template":
		err = q.QueryRow(ctx, `SELECT to_jsonb(t) FROM public.world_event_templates t WHERE world_id=(SELECT world_id FROM public.fluctlight_world_bindings WHERE fluctlight_id=$1) AND source_resource_id=$2`, fid, p.ID).Scan(&raw)
	case "npc":
		err = q.QueryRow(ctx, `SELECT jsonb_build_object('id',n.id,'name',n.name,'role',n.role,'background',n.background,'revision',n.revision) FROM public.world_npcs n JOIN public.life_worlds w ON w.id=n.world_id WHERE w.id=(SELECT world_id FROM public.fluctlight_world_bindings WHERE fluctlight_id=$1) AND n.source_resource_id=$2`, fid, p.ID).Scan(&raw)
	default:
		return nil, ErrInvalidArguments
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return map[string]any{}, nil
	}
	return decodeObject(raw), err
}

func worldApplyDiff(current map[string]any, p worldPublication) []any {
	if current["id"] == nil {
		return []any{}
	}
	fields := []string{}
	if p.Kind != "place" {
		fields = append(fields, "name")
	}
	switch p.Kind {
	case "world":
		fields = append(fields, "background", "rules")
		for _, key := range []string{"description", "flavor", "item_pool", "framework"} {
			if _, ok := p.Body[key]; ok {
				fields = append(fields, key)
			}
		}
	case "place":
		fields = append(fields, "description")
		for _, key := range []string{"alias", "group", "floor", "scene", "atmosphere", "furniture", "layout", "connections", "connection_place_ids", "room_items"} {
			if _, ok := p.Body[key]; ok {
				fields = append(fields, key)
			}
		}
	case "event_template":
		fields = append(fields, "definition")
	case "npc":
		fields = append(fields, "role", "background")
	}
	out := []any{}
	for _, field := range fields {
		next := p.Body[field]
		if next == nil {
			next = ""
		}
		if field == "definition" {
			next = p.Body
		}
		if field == "name" {
			next = p.Name
		}
		if jsonString(current[field]) != jsonString(next) {
			out = append(out, map[string]any{"field": field, "from": current[field], "to": next})
		}
	}
	return out
}
