package core

import (
	"context"
	"errors"
	"net/url"
	"strings"
)

func (a *App) SaveOwnerInventoryOutfit(ctx context.Context, owner, fid, id string, body map[string]any) (map[string]any, error) {
	tx, ok := SurfaceDBTx(ctx)
	if !ok {
		return nil, errors.New("outfit_transaction_required")
	}
	allowed := map[string]bool{"metadata_only": true, "tags": true, "name": true, "item_ids": true, "profile_id": true, "expected_revision": true, "expected_outfit_revision": true, "reason": true, "idempotency_key": true}
	for key := range body {
		if !allowed[key] {
			return nil, invalid("unexpected outfit field")
		}
	}
	expected, ok := parseRevision(body["expected_revision"])
	previous, previousOK := parseRevision(body["expected_outfit_revision"])
	name, reason, profile := strings.TrimSpace(stringValue(body["name"])), strings.TrimSpace(stringValue(body["reason"])), stringValue(body["profile_id"])
	if !ok || !previousOK || expected < 0 || previous < 0 || (id == "" && previous != 0) || (id != "" && previous < 1) || name == "" || len([]rune(name)) > 128 || reason == "" || len([]rune(reason)) > 1024 {
		return nil, invalid("outfit fields and versions required")
	}
	tags, err := normalizedOutfitTags(body["tags"])
	if err != nil {
		return nil, err
	}
	_, tagsSet := body["tags"]
	metadataOnly := body["metadata_only"] == true
	if metadataOnly && (id == "" || tagsSet) {
		return nil, invalid("name-only updates need an existing outfit and preserve tags")
	}
	ids := decisionServiceRefValues(body["item_ids"])
	if !metadataOnly && (len(ids) < 1 || len(ids) > 16 || len(ids) != len(arrayValue(body["item_ids"]))) {
		return nil, invalid("1–16 concrete item IDs required")
	}
	if err := lockLifeContextTx(ctx, tx, fid); err != nil {
		return nil, err
	}
	if err := requireOwnerMutableTx(ctx, tx, owner, fid); err != nil {
		return nil, err
	}
	active, err := newWardrobeService(a).effectiveProfileID(ctx, tx, fid, "")
	if err != nil {
		return nil, err
	}
	if profile != active {
		return nil, &SurfaceError{Status: 409, Code: "outfit_profile_changed", Message: "当前人格已变化，请核对套装归属后保存"}
	}
	audit, err := (postgresSurfaceTx{tx}).Audit(ctx, owner, fid, "outfit_save", reason, id)
	if err != nil {
		return nil, err
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('fluctlight.inventory.audit_id',$1,true)`, audit); err != nil {
		return nil, err
	}
	result, err := newWardrobeService(a).saveOutfitTx(ctx, tx, wardrobeOutfitWrite{FluctlightID: fid, ProfileID: profile, OutfitID: id, Name: name, ItemIDs: ids, Tags: tags, TagsSet: tagsSet, MetadataOnly: metadataOnly, OperationID: "owner-outfit:" + stringValue(body["idempotency_key"]), SourceRef: audit, ExpectedWardrobeRevision: expected, ExpectedOutfitRevision: previous})
	if err != nil {
		return nil, err
	}
	saved, err := readInventoryOutfitWith(ctx, tx, fid, stringValue(result["outfit_id"]))
	if err != nil {
		return nil, err
	}
	saved["active_profile_id"] = active
	if _, err := tx.Exec(ctx, `UPDATE public.owner_surface_audits SET record_id=$2 WHERE id=$1`, audit, result["outfit_id"]); err != nil {
		return nil, err
	}
	return map[string]any{"outfit": saved, "revision": result["wardrobe_revision"], "audit_id": audit}, nil
}
func (a *App) OwnerInventoryOutfitHistory(ctx context.Context, owner, fid, id string, q url.Values) (map[string]any, error) {
	if _, err := a.OwnerInventoryOutfit(ctx, owner, fid, id); err != nil {
		return nil, err
	}
	limit, err := reflectionReadPage(q)
	if err != nil {
		return nil, err
	}
	scope := stableDigest(jsonString([]any{"outfit-history", owner, fid, id}))
	cursor, err := decodeOwnerReadCursor(q.Get("cursor"), scope)
	if err != nil || (q.Get("cursor") != "" && (cursor.ID == "" || cursor.Time.IsZero())) {
		return nil, invalid("invalid outfit history cursor")
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT h.id,h.created_at,to_jsonb(h)||jsonb_build_object('owner_audit',(SELECT to_jsonb(a) FROM public.owner_surface_audits a WHERE a.id=h.source_ref AND a.owner_actor_id=$5)) FROM public.fluctlight_wardrobe_outfit_revisions h WHERE h.fluctlight_id=$1 AND h.outfit_id=$2 AND ($3='' OR (h.created_at,h.id)<($4,$3)) ORDER BY h.created_at DESC,h.id DESC LIMIT $6`, fid, id, cursor.ID, cursor.Time, owner, limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return readGovernanceRows(rows, scope, limit)
}

func (a *App) OwnerWearingHistory(ctx context.Context, owner, fid string, q url.Values) (map[string]any, error) {
	if _, err := a.DB.GetFluctlight(ctx, fid, owner); err != nil {
		return nil, err
	}
	limit, err := reflectionReadPage(q)
	if err != nil {
		return nil, err
	}
	scope := stableDigest(jsonString([]any{"wearing-history", owner, fid}))
	cursor, err := decodeOwnerReadCursor(q.Get("cursor"), scope)
	if err != nil || (q.Get("cursor") != "" && (cursor.ID == "" || cursor.Time.IsZero())) {
		return nil, invalid("invalid wearing history cursor")
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT h.id,h.created_at,to_jsonb(h)||jsonb_build_object('owner_audit',(SELECT to_jsonb(a) FROM public.owner_surface_audits a WHERE a.id=h.source_ref AND a.owner_actor_id=$5)) FROM public.fluctlight_wearing_history h WHERE h.fluctlight_id=$1 AND ($2='' OR (h.created_at,h.id)<($3,$2)) ORDER BY h.created_at DESC,h.id DESC LIMIT $4`, fid, cursor.ID, cursor.Time, limit+1, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return readGovernanceRows(rows, scope, limit)
}
