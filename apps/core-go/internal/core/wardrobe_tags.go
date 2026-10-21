package core

import (
	"context"
	"strings"
)

func (a *App) OwnerWardrobeTags(ctx context.Context, owner, fid string) (map[string]any, error) {
	if _, err := a.DB.GetFluctlight(ctx, fid, owner); err != nil {
		return nil, err
	}
	var tags []string
	var revision int
	if err := a.DB.Pool().QueryRow(ctx, `SELECT outfit_tags,revision FROM public.fluctlight_wardrobe_states WHERE fluctlight_id=$1`, fid).Scan(&tags, &revision); err != nil {
		return nil, err
	}
	return map[string]any{"tags": tags, "revision": revision}, nil
}

// One aggregate CAS covers the directory and every affected plan, including
// plans whose archived members must remain visible in their historical form.
func (a *App) SaveOwnerWardrobeTags(ctx context.Context, owner, fid string, body map[string]any) (map[string]any, error) {
	tx, ok := SurfaceDBTx(ctx)
	if !ok {
		return nil, ErrInvalidArguments
	}
	if !eventKeys(body, "tags", "renames", "expected_revision", "reason", "idempotency_key") {
		return nil, ErrInvalidArguments
	}
	if _, present := body["tags"]; !present {
		return nil, invalid("explicit tag directory required")
	}
	tags, err := normalizedOutfitTags(body["tags"])
	if err != nil {
		return nil, err
	}
	expected, ok := parseRevision(body["expected_revision"])
	reason := strings.TrimSpace(stringValue(body["reason"]))
	if !ok || reason == "" {
		return nil, ErrInvalidArguments
	}
	if err = lockLifeContextTx(ctx, tx, fid); err != nil {
		return nil, err
	}
	if err = requireOwnerMutableTx(ctx, tx, owner, fid); err != nil {
		return nil, err
	}
	var revision int
	var previousTags []string
	if err = tx.QueryRow(ctx, `SELECT revision,outfit_tags FROM public.fluctlight_wardrobe_states WHERE fluctlight_id=$1 FOR UPDATE`, fid).Scan(&revision, &previousTags); err != nil {
		return nil, err
	}
	if revision != expected {
		return nil, conflict("wardrobe", expected, revision)
	}
	allowed := map[string]bool{}
	for _, t := range tags {
		allowed[t] = true
	}
	renames := mapValue(body["renames"])
	for old, raw := range renames {
		target, ok := raw.(string)
		if !ok || !allowed[target] || !requestedToolName(previousTags, old) {
			return nil, invalid("tag rename must reference existing and retained names")
		}
	}
	audit, err := (postgresSurfaceTx{tx}).Audit(ctx, owner, fid, "wardrobe_tags", reason, fid)
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, `SELECT id FROM public.fluctlight_wardrobe_outfits WHERE fluctlight_id=$1 ORDER BY id FOR UPDATE`, fid)
	if err != nil {
		return nil, err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		before, readErr := readInventoryOutfitWith(ctx, tx, fid, id)
		if readErr != nil {
			return nil, readErr
		}
		oldTags, _ := normalizedOutfitTags(before["tags"])
		next := []string{}
		for _, tag := range oldTags {
			if renamed, ok := renames[tag]; ok {
				tag = stringValue(renamed)
			}
			if allowed[tag] {
				next = append(next, tag)
			}
		}
		next, _ = normalizedOutfitTags(next)
		if jsonString(next) == jsonString(oldTags) {
			continue
		}
		if _, err = tx.Exec(ctx, `UPDATE public.fluctlight_wardrobe_outfits SET tags=$3,revision=revision+1,updated_at=now() WHERE fluctlight_id=$1 AND id=$2`, fid, id, next); err != nil {
			return nil, err
		}
		after, readErr := readInventoryOutfitWith(ctx, tx, fid, id)
		if readErr != nil {
			return nil, readErr
		}
		if _, err = tx.Exec(ctx, `INSERT INTO public.fluctlight_wardrobe_outfit_revisions(id,fluctlight_id,outfit_id,profile_id,revision,source_ref,before_value,after_value)VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, randomID("outfit_revision_"), fid, id, stringValue(after["profile_id"]), intValue(after["revision"]), audit, jsonBytes(before), jsonBytes(after)); err != nil {
			return nil, err
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE public.fluctlight_wardrobe_states SET outfit_tags=$2,revision=revision+1,updated_at=now() WHERE fluctlight_id=$1`, fid, tags); err != nil {
		return nil, err
	}
	return map[string]any{"tags": tags, "revision": revision + 1, "audit_id": audit}, nil
}
