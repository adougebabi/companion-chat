package core

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"
)

var visualStyleFields = []string{"appearance", "clothing", "constraints", "references"}

type visualBriefSourceKey struct{}

func validateVisualStyle(body map[string]any) error {
	if body["schema"] != "visual-style.v1" || !eventKeys(body, "schema", "appearance", "clothing", "constraints", "references") {
		return invalid("Expected visual-style.v1 reference guidance")
	}
	if containsPermanentHarm(body) {
		return harmError()
	}
	for _, key := range visualStyleFields {
		value, ok := body[key].(string)
		if !ok || len([]rune(value)) > 1000 {
			return invalid("Visual style fields must be explicit text up to 1000 characters")
		}
	}
	return nil
}
func visualStyleDiff(current, definition map[string]any) []any {
	rows := []any{}
	for _, key := range visualStyleFields {
		if current == nil || jsonString(current[key]) != jsonString(definition[key]) {
			rows = append(rows, map[string]any{"field": key, "from": current[key], "to": definition[key]})
		}
	}
	return rows
}

func (a *App) applyStudioVisualStyle(ctx context.Context, owner, id string, body map[string]any, preview bool) (map[string]any, error) {
	fid := stringValue(body["fluctlight_id"])
	fl, err := a.DB.GetFluctlight(ctx, fid, owner)
	if err != nil {
		return nil, err
	}
	publication, err := readWorldPublication(ctx, a.DB.Pool(), owner, id, stringValue(body["resource_version"]))
	if err != nil {
		return nil, err
	}
	if publication.Kind != "visual_style" {
		return nil, invalid("Select an exact published visual_style version")
	}
	if err := validateVisualStyle(publication.Body); err != nil {
		return nil, err
	}
	if preview {
		current, err := readVisualBriefWith(ctx, a.DB.Pool(), fid)
		if err != nil {
			return nil, err
		}
		var runtimeID any
		if current != nil {
			runtimeID = fid
		}
		return map[string]any{"available": true, "target_kind": "visual_brief", "semantics": "Reference guidance only; canonical images, effective body and wearing stay unchanged", "diff": visualStyleDiff(current, publication.Body), "conflicts": []any{}, "instance_revision": fl.CurrentRevision, "runtime_revision": intValue(current["revision"]), "runtime_id": runtimeID}, nil
	}
	ctx = context.WithValue(ctx, ownerCommandAuditActionKey{}, "studio_visual_style_apply")
	return a.worldCommand(ctx, owner, fid, "apply-visual-style:"+id, body, func(ctx context.Context, tx pgx.Tx) (map[string]any, error) {
		rec, found, err := (postgresSurfaceTx{tx}).Get(ctx, owner, id)
		if err != nil {
			return nil, err
		}
		if !found || rec.Kind != "studio_resource" {
			return nil, ErrNotFound
		}
		if err := activeStudioResource(rec.Status); err != nil {
			return nil, err
		}
		var instance int
		if err := tx.QueryRow(ctx, `SELECT current_revision FROM public.fluctlights WHERE id=$1 AND created_by_actor_id=$2 FOR UPDATE`, fid, owner).Scan(&instance); err != nil {
			return nil, err
		}
		expected, ok := parseRevision(body["expected_instance_revision"])
		if !ok || expected != instance {
			return nil, conflict("fluctlight", expected, instance)
		}
		current, err := readVisualBriefWith(ctx, tx, fid)
		if err != nil {
			return nil, err
		}
		version := intValue(current["revision"])
		runtimeExpected, ok := parseRevision(body["expected_runtime_revision"])
		if !ok || runtimeExpected != version {
			return nil, conflict("visual_brief", runtimeExpected, version)
		}
		choices := map[string]string{}
		for _, raw := range arrayValue(body["choices"]) {
			choice := mapValue(raw)
			field, decision := stringValue(choice["field"]), stringValue(choice["decision"])
			valid := false
			for _, key := range visualStyleFields {
				valid = valid || key == field
			}
			if !valid || choices[field] != "" || (decision != "keep" && decision != "override") {
				return nil, invalid("Choose each visual reference field at most once")
			}
			choices[field] = decision
		}
		values := map[string]any{"expected_revision": version, "reason": strings.TrimSpace(stringValue(body["reason"]))}
		for _, key := range visualStyleFields {
			values[key] = publication.Body[key]
		}
		if current != nil {
			for _, raw := range visualStyleDiff(current, publication.Body) {
				field := stringValue(mapValue(raw)["field"])
				switch choices[field] {
				case "keep":
					values[field] = current[field]
				case "override":
				default:
					return nil, &SurfaceError{Status: 409, Code: "apply_choices_required", Message: "Choose keep or override for every changed visual reference", Details: map[string]any{"field": field}}
				}
			}
		}
		txCtx := context.WithValue(context.WithValue(ctx, surfaceDBTxKey{}, tx), visualBriefSourceKey{}, publication)
		saved, err := a.SaveOwnerVisualBrief(txCtx, owner, fid, values)
		if err != nil {
			return nil, err
		}
		return map[string]any{"id": fid, "resource_id": id, "resource_version": publication.Version, "status": "applied", "applied": true, "target_kind": "visual_brief", "runtime_revision": saved["revision"], "instance_revision": instance, "brief": saved["brief"], "life_write": false, "semantics": "Saved reference guidance for future visual sessions; no canonical/effective-state change"}, nil
	})
}

func (a *App) visualStyleReferences(ctx context.Context, owner, id string) ([]any, error) {
	return readWorldObjects(ctx, a.DB.Pool(), `SELECT jsonb_build_object('fluctlight_id',b.fluctlight_id,'runtime_id',b.fluctlight_id,'running_version',b.source_version,'instance_revision',b.revision,'target_kind','visual_brief','status','applied','overrides',b.source_overrides) FROM public.fluctlight_visual_briefs b JOIN public.fluctlights f ON f.id=b.fluctlight_id WHERE b.source_resource_id=$1 AND f.created_by_actor_id=$2 ORDER BY b.fluctlight_id`, id, owner)
}
