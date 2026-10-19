package core

import (
	"context"
	"errors"
	"net/url"

	"github.com/jackc/pgx/v5"
)

func displayAvatarRoute(method string, p []string) bool {
	return len(p) >= 3 && p[0] == "fluctlights" && ((len(p) == 3 && p[2] == "display-avatar" && (method == "GET" || method == "PUT")) || (len(p) == 4 && p[2] == "display-avatar" && p[3] == "history" && method == "GET") || (len(p) == 3 && p[2] == "visual-identity" && method == "PUT"))
}
func (a *App) dispatchDisplayAvatar(ctx context.Context, method string, p []string, q url.Values, owner string, body map[string]any) (map[string]any, error) {
	fid := p[1]
	if _, err := a.DB.GetFluctlight(ctx, fid, owner); err != nil {
		return nil, err
	}
	if method == "GET" && len(p) == 4 {
		return a.ownerDisplayAvatarHistory(ctx, owner, fid, q)
	}
	if method == "GET" {
		return a.displayAvatarWith(ctx, a.DB.Pool(), owner, fid)
	}
	if !eventKeys(body, "mode", "asset_id", "expected_asset_revision", "expected_revision", "reason", "idempotency_key") {
		return nil, invalid("Unknown display avatar field")
	}
	mode := stringValue(body["mode"])
	if mode != "asset" && mode != "canonical" {
		return nil, invalid("Choose a display asset or canonical avatar")
	}
	if mode == "canonical" && (body["asset_id"] != nil || body["expected_asset_revision"] != nil) {
		return nil, invalid("Canonical display selection cannot carry another asset")
	}
	ctx = context.WithValue(ctx, ownerCommandAuditActionKey{}, "display_avatar_select")
	return a.worldCommand(ctx, owner, fid, "display-avatar:"+fid, body, func(ctx context.Context, tx pgx.Tx) (map[string]any, error) {
		before, err := a.displayAvatarWith(ctx, tx, owner, fid)
		if err != nil {
			return nil, err
		}
		expected, ok := parseRevision(body["expected_revision"])
		if !ok || expected != intValue(before["revision"]) {
			return nil, conflict("display_avatar", expected, intValue(before["revision"]))
		}
		selection := map[string]any{"mode": mode, "source": "owner_selection", "kind": "display_avatar"}
		if mode == "asset" {
			id := stringValue(body["asset_id"])
			if id == "" {
				return nil, invalid("An actual image asset is required")
			}
			asset, err := a.ownerMediaAssetWith(ctx, tx, owner, id, true)
			if err != nil {
				return nil, err
			}
			revision, ok := parseRevision(body["expected_asset_revision"])
			if !ok || revision != intValue(asset["asset_revision"]) {
				return nil, conflict("media_asset", revision, intValue(asset["asset_revision"]))
			}
			if asset["kind"] != "image" || asset["readable"] != true {
				return nil, &SurfaceError{Status: 422, Code: "avatar_asset_unreadable", Message: "显示头像需要当前可读取的图片素材"}
			}
			for _, field := range []string{"asset_id", "version", "sha256", "origin"} {
				selection[field] = asset[field]
			}
			selection["asset_revision"] = asset["asset_revision"]
		}
		surface := postgresSurfaceTx{tx}
		revision, err := surface.Bump(ctx, owner, fid, "visual:"+fid, expected)
		if err != nil {
			return nil, err
		}
		now := a.now().UTC()
		rec := surfaceRecord{ID: "visual_identity_" + shortHash(owner+"|"+fid), OwnerID: owner, FluctlightID: fid, Kind: "visual_identity", ResourceKey: "current", Status: "saved", Revision: revision, CreatedAt: now, UpdatedAt: now, Payload: selection}
		if err := surface.Save(ctx, rec); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM public.media_references WHERE owner_fluctlight_id=$1 AND target_type='display_avatar' AND target_id=$1`, fid); err != nil {
			return nil, err
		}
		if mode == "asset" {
			if _, err := tx.Exec(ctx, `INSERT INTO public.media_references(id,asset_id,owner_fluctlight_id,target_type,target_id) VALUES($1,$2,$3,'display_avatar',$3)`, "avatar_ref_"+shortHash(owner+"|"+fid), selection["asset_id"], fid); err != nil {
				return nil, err
			}
		}
		history := surfaceRecord{ID: randomID("avatar_revision_"), OwnerID: owner, FluctlightID: fid, Kind: "display_avatar_history", ResourceKey: randomID("avatar_history_"), Status: "saved", Revision: revision, CreatedAt: now, UpdatedAt: now, Payload: map[string]any{"before_value": before, "after_value": selection, "reason": body["reason"], "actor_id": owner}}
		if err := surface.Save(ctx, history); err != nil {
			return nil, err
		}
		return a.displayAvatarWith(ctx, tx, owner, fid)
	})
}
func (a *App) displayAvatarWith(ctx context.Context, q lifeContextQuerier, owner, fid string) (map[string]any, error) {
	var revision int
	err := q.QueryRow(ctx, `SELECT revision FROM public.owner_surface_records WHERE owner_actor_id=$1 AND fluctlight_id=$2 AND kind='scope' AND resource_key=$3`, owner, fid, "visual:"+fid).Scan(&revision)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	var raw []byte
	err = q.QueryRow(ctx, `SELECT payload FROM public.owner_surface_records WHERE owner_actor_id=$1 AND fluctlight_id=$2 AND kind='visual_identity' AND resource_key='current'`, owner, fid).Scan(&raw)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	selection := decodeObject(raw)
	mode := "canonical"
	if len(selection) > 0 {
		mode = stringValue(selection["mode"])
		if mode == "" {
			mode = "legacy"
		}
	}
	result := map[string]any{"fluctlight_id": fid, "revision": revision, "mode": mode, "selection": selection, "avatar": nil, "read_state": "not_configured"}
	var id string
	if mode == "canonical" {
		err = q.QueryRow(ctx, `SELECT COALESCE(canonical_asset_id,'') FROM public.fluctlight_visual_identities WHERE fluctlight_id=$1`, fid).Scan(&id)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
	} else {
		id = stringValue(selection["asset_id"])
	}
	if mode == "legacy" {
		result["read_state"] = "legacy_selection_unverified"
		return result, nil
	}
	if id == "" {
		return result, nil
	}
	asset, err := a.ownerMediaAssetWith(ctx, q, owner, id, false)
	if errors.Is(err, ErrNotFound) {
		result["read_state"] = "source_unavailable"
		return result, nil
	}
	if err != nil {
		return nil, err
	}
	result["read_state"] = asset["read_state"]
	if asset["kind"] != "image" {
		result["read_state"] = "image_required"
		return result, nil
	}
	if mode == "asset" && (selection["sha256"] != asset["sha256"] || selection["version"] != asset["version"]) {
		result["read_state"] = "source_identity_changed"
		return result, nil
	}
	if asset["readable"] == true {
		result["avatar"] = map[string]any{"asset_id": id, "url": asset["url"], "source": asset["origin"], "kind": "display_avatar", "factual": false, "label": asset["label"]}
	}
	return result, nil
}
func (a *App) ownerDisplayAvatarHistory(ctx context.Context, owner, fid string, q url.Values) (map[string]any, error) {
	limit, err := reflectionReadPage(q)
	if err != nil {
		return nil, err
	}
	scope := stableDigest(jsonString([]any{"display-avatar-history", owner, fid}))
	cursor, err := decodeOwnerReadCursor(q.Get("cursor"), scope)
	if err != nil {
		return nil, err
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT id,created_at,jsonb_build_object('id',id,'revision',revision,'created_at',created_at)||payload FROM public.owner_surface_records WHERE owner_actor_id=$1 AND fluctlight_id=$2 AND kind='display_avatar_history' AND ($3='' OR (created_at,id)<($4,$3)) ORDER BY created_at DESC,id DESC LIMIT $5`, owner, fid, cursor.ID, cursor.Time, limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return readGovernanceRows(rows, scope, limit)
}
