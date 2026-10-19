package core

import (
	"errors"
	"net/url"
	"testing"
)

func TestVisualStyleRequiresExplicitGuidanceAndRejectsUnsupportedFields(t *testing.T) {
	base := map[string]any{"schema": "visual-style.v1", "appearance": "参考", "clothing": "遵循合法当前穿着", "constraints": "不修改生活事实", "references": "文字参考"}
	if err := validateVisualStyle(base); err != nil {
		t.Fatal(err)
	}
	for _, key := range visualStyleFields {
		missing := cloneMap(base)
		delete(missing, key)
		if validateVisualStyle(missing) == nil {
			t.Fatal("Missing explicit field admitted", key)
		}
	}
	bad := cloneMap(base)
	bad["permanent_harm"] = true
	if validateVisualStyle(bad) == nil {
		t.Fatal("Permanent injury admitted")
	}
	bad = cloneMap(base)
	bad["canonical_asset_id"] = "forged"
	if validateVisualStyle(bad) == nil {
		t.Fatal("Definition can claim a canonical asset")
	}
}

func TestStudioVisualStyleApplyPreservesEffectiveStateAndTracksOverrides(t *testing.T) {
	f := newVisualIdentityToolFixture(t)
	// Domain/queued-session fixtures only; no model, renderer or object I/O.
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlight_visual_identity_sessions SET status='cancelled' WHERE id=$1`, f.sessionID); err != nil {
		t.Fatal(err)
	}
	body := map[string]any{"schema": "visual-style.v1", "appearance": "风格A", "clothing": "当前合法穿着", "constraints": "保持有效身体", "references": "原参考"}
	r, err := f.app.DispatchOwnerSurface(f.ctx, "POST", "/internal/studio/resources", nil, f.ownerID, map[string]any{"kind": "visual_style", "name": "Style A", "body": body, "reason": "Create visual guidance", "idempotency_key": "style-create"})
	if err != nil {
		t.Fatal(err)
	}
	id := stringValue(r["id"])
	pub, err := f.app.publishWorldResource(f.ctx, f.ownerID, id, map[string]any{"expected_revision": r["revision"], "reason": "Publish style", "idempotency_key": "style-publish"})
	if err != nil {
		t.Fatal(err)
	}
	before, beforeBody, beforeWardrobe, err := f.app.readEffectiveLifeSnapshot(f.ctx, f.fluctlightID, f.app.now())
	if err != nil {
		t.Fatal(err)
	}
	preview, err := f.app.DispatchWorldApply(f.ctx, f.ownerID, id, map[string]any{"fluctlight_id": f.fluctlightID, "resource_version": pub["published_version"]}, true)
	if err != nil || preview["target_kind"] != "visual_brief" || intValue(preview["runtime_revision"]) != 0 {
		t.Fatal(preview, err)
	}
	command := map[string]any{"fluctlight_id": f.fluctlightID, "resource_version": pub["published_version"], "expected_instance_revision": preview["instance_revision"], "expected_runtime_revision": 0, "choices": []any{}, "reason": "Apply explicit reference", "idempotency_key": "style-apply"}
	applied, err := f.app.DispatchWorldApply(f.ctx, f.ownerID, id, command, false)
	if err != nil || applied["applied"] != true || applied["life_write"] != false {
		t.Fatal(applied, err)
	}
	replay, err := f.app.DispatchWorldApply(f.ctx, f.ownerID, id, command, false)
	if err != nil || intValue(replay["runtime_revision"]) != intValue(applied["runtime_revision"]) {
		t.Fatal("replay changed reference revision", replay, err)
	}
	saved, err := (&App{DB: f.repository}).OwnerVisualBrief(f.ctx, f.ownerID, f.fluctlightID)
	brief := mapValue(saved["brief"])
	if err != nil || brief["source_resource_id"] != id || brief["source_version"] != pub["published_version"] || brief["appearance"] != "风格A" {
		t.Fatal("source not durable", saved, err)
	}
	// Manual edits preserve the immutable origin and record their local fields.
	_, err = f.app.DispatchOwnerSurface(f.ctx, "PUT", "/internal/fluctlights/"+f.fluctlightID+"/visual-brief", nil, f.ownerID, map[string]any{"appearance": "本角色局部参考", "clothing": body["clothing"], "constraints": body["constraints"], "references": body["references"], "expected_revision": 1, "reason": "Local Owner refinement", "idempotency_key": "style-local"})
	if err != nil {
		t.Fatal(err)
	}
	local, _ := f.app.OwnerVisualBrief(f.ctx, f.ownerID, f.fluctlightID)
	if mapValue(mapValue(local["brief"])["source_overrides"])["appearance"] != "本角色局部参考" {
		t.Fatal("manual edit lost source/override", local)
	}
	changed := cloneMap(body)
	changed["appearance"] = "发布风格B"
	changed["clothing"] = "新服装参考"
	updated, err := f.app.DispatchOwnerSurface(f.ctx, "PUT", "/internal/studio/resources/"+id, nil, f.ownerID, map[string]any{"kind": "visual_style", "name": "Style B", "body": changed, "expected_version": r["draft_version"], "reason": "Save next style", "idempotency_key": "style-save-b"})
	if err != nil {
		t.Fatal(err)
	}
	publishedB, err := f.app.publishWorldResource(f.ctx, f.ownerID, id, map[string]any{"expected_revision": updated["revision"], "reason": "Publish B", "idempotency_key": "style-publish-b"})
	if err != nil {
		t.Fatal(err)
	}
	commandB := cloneMap(command)
	commandB["resource_version"], commandB["expected_runtime_revision"], commandB["idempotency_key"] = publishedB["published_version"], 2, "style-b-no-choices"
	_, err = f.app.DispatchWorldApply(f.ctx, f.ownerID, id, commandB, false)
	var conflict *SurfaceError
	if !errors.As(err, &conflict) || conflict.Code != "apply_choices_required" {
		t.Fatal("missing explicit choices did not fail", err)
	}
	commandB["choices"] = []any{map[string]any{"field": "appearance", "decision": "keep"}, map[string]any{"field": "clothing", "decision": "override"}}
	commandB["idempotency_key"] = "style-b-chosen"
	b, err := f.app.DispatchWorldApply(f.ctx, f.ownerID, id, commandB, false)
	if err != nil {
		t.Fatal(err)
	}
	br := mapValue(b["brief"])
	if br["appearance"] != "本角色局部参考" || br["clothing"] != "新服装参考" || br["source_version"] != publishedB["published_version"] {
		t.Fatal("field choices not applied", b)
	}
	generated, err := f.app.DispatchOwnerSurface(f.ctx, "POST", "/internal/fluctlights/"+f.fluctlightID+"/visual-identity/generate", nil, f.ownerID, map[string]any{"expected_visual_revision": 0, "expected_brief_revision": 3, "reason": "Freeze applied guidance", "idempotency_key": "style-generate"})
	if err != nil {
		t.Fatal(err)
	}
	var raw []byte
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT input_snapshot FROM public.fluctlight_visual_identity_attempts WHERE session_id=$1 AND attempt_number=1`, generated["session_id"]).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	frozen := mapValue(decodeObject(raw)["owner_visual_brief"])
	if frozen["source_version"] != publishedB["published_version"] || frozen["appearance"] != "本角色局部参考" || frozen["clothing"] != "新服装参考" {
		t.Fatal("real visual session did not freeze applied source", frozen)
	}
	refs, err := f.app.DispatchOwnerSurface(f.ctx, "GET", "/internal/studio/resources/"+id, nil, f.ownerID, nil)
	if err != nil || len(arrayValue(refs["referenced_by"])) != 1 {
		t.Fatal("actual reference readback missing", refs, err)
	}
	list, err := f.app.OwnerStudioResources(f.ctx, f.ownerID, url.Values{"kind": {"visual_style"}})
	if err != nil || len(arrayValue(list["items"])) != 1 || len(arrayValue(mapValue(arrayValue(list["items"])[0])["referenced_by"])) != 1 {
		t.Fatal("catalogue lost actual references", list, err)
	}
	_, err = f.app.studioCatalogCommand(f.ctx, f.ownerID, id, "archive", map[string]any{"expected_revision": publishedB["revision"], "reason": "Referenced style may not be archived", "idempotency_key": "style-archive-block"})
	if !errors.As(err, &conflict) || conflict.Code != "resource_referenced" {
		t.Fatal("archived referenced guidance", err)
	}
	// A later draft may use another type. Old published sources still dispatch
	// by their own immutable kind and retain their actual reference projection.
	_, err = f.app.DispatchOwnerSurface(f.ctx, "PUT", "/internal/studio/resources/"+id, nil, f.ownerID, map[string]any{"kind": "character_template", "name": "New template draft", "body": map[string]any{"schema": "character-template.v1", "description": "A fictional adult"}, "expected_version": updated["draft_version"], "reason": "Change only working draft type", "idempotency_key": "style-retag"})
	if err != nil {
		t.Fatal(err)
	}
	replay, err = f.app.DispatchWorldApply(f.ctx, f.ownerID, id, commandB, false)
	if err != nil || intValue(replay["runtime_revision"]) != intValue(b["runtime_revision"]) {
		t.Fatal("draft kind changed published application replay", replay, err)
	}
	oldPreview, err := f.app.DispatchWorldApply(f.ctx, f.ownerID, id, command, true)
	if err != nil || oldPreview["target_kind"] != "visual_brief" {
		t.Fatal("old visual publication lost consumer", oldPreview, err)
	}
	refs, err = f.app.DispatchOwnerSurface(f.ctx, "GET", "/internal/studio/resources/"+id, nil, f.ownerID, nil)
	if err != nil || len(arrayValue(refs["referenced_by"])) != 1 {
		t.Fatal("retagged resource lost actual visual reference", refs, err)
	}
	list, err = f.app.OwnerStudioResources(f.ctx, f.ownerID, url.Values{"kind": {"character_template"}})
	if err != nil || len(arrayValue(list["items"])) != 1 || len(arrayValue(mapValue(arrayValue(list["items"])[0])["referenced_by"])) != 1 {
		t.Fatal("retagged catalogue lost actual visual reference", list, err)
	}
	after, afterBody, afterWardrobe, err := f.app.readEffectiveLifeSnapshot(f.ctx, f.fluctlightID, f.app.now())
	if err != nil || beforeBody != afterBody || beforeWardrobe != afterWardrobe || jsonString(before["body_fields"]) != jsonString(after["body_fields"]) || jsonString(before["worn_items"]) != jsonString(after["worn_items"]) {
		t.Fatal("visual reference changed effective life", before, after, err)
	}
}
