package core

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/jackc/pgx/v5"
)

func readStudioMaterialWith(ctx context.Context, q lifeContextQuerier, owner, id string, lock bool) (map[string]any, error) {
	sql := `SELECT to_jsonb(m) FROM public.studio_resource_materials m WHERE owner_actor_id=$1 AND id=$2`
	if lock {
		sql += " FOR UPDATE"
	}
	var raw []byte
	err := q.QueryRow(ctx, sql, owner, id).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return decodeObject(raw), err
}
func recordStudioMaterialHistory(ctx context.Context, tx pgx.Tx, owner, action, reason string, before, after map[string]any) error {
	var previous any
	if before != nil {
		previous = jsonBytes(before)
	}
	_, err := tx.Exec(ctx, `INSERT INTO public.studio_material_history(id,material_id,owner_actor_id,revision,action,before_value,after_value,reason) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, randomID("studio_material_revision_"), after["id"], owner, after["revision"], action, previous, jsonBytes(after), reason)
	return err
}
func retainStudioMaterialTx(ctx context.Context, tx pgx.Tx, owner string, job map[string]any, text, asset any, label string) (map[string]any, error) {
	var raw []byte
	err := tx.QueryRow(ctx, `SELECT to_jsonb(m) FROM public.studio_resource_materials m WHERE job_id=$1 AND owner_actor_id=$2 FOR UPDATE`, job["id"], owner).Scan(&raw)
	var before map[string]any
	if err == nil {
		before = decodeObject(raw)
		if before["source_digest"] != job["source_digest"] || before["resource_id"] != job["resource_id"] || before["text"] != text || before["asset_id"] != asset {
			return nil, ErrConflict
		}
		if before["released_at"] == nil {
			return studioMaterialReceipt(before), nil
		}
		if _, err := tx.Exec(ctx, `UPDATE public.studio_resource_materials SET released_at=NULL,revision=revision+1 WHERE id=$1 AND owner_actor_id=$2`, before["id"], owner); err != nil {
			return nil, err
		}
	} else if errors.Is(err, pgx.ErrNoRows) {
		id := "studio_material_" + stableDigest(owner+":"+stringValue(job["id"]))
		if _, err := tx.Exec(ctx, `INSERT INTO public.studio_resource_materials(id,owner_actor_id,resource_id,job_id,source_digest,text,asset_id,label) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, id, owner, job["resource_id"], job["id"], job["source_digest"], text, asset, label); err != nil {
			return nil, err
		}
	} else {
		return nil, err
	}
	if err := tx.QueryRow(ctx, `SELECT to_jsonb(m) FROM public.studio_resource_materials m WHERE job_id=$1 AND owner_actor_id=$2`, job["id"], owner).Scan(&raw); err != nil {
		return nil, err
	}
	after := decodeObject(raw)
	if err := recordStudioMaterialHistory(ctx, tx, owner, "retain", "所有者明确保留原任务结果", before, after); err != nil {
		return nil, err
	}
	return studioMaterialReceipt(after), nil
}
func studioMaterialReceipt(material map[string]any) map[string]any {
	return map[string]any{"id": material["id"], "job_id": material["job_id"], "resource_id": material["resource_id"], "asset_id": material["asset_id"], "revision": material["revision"], "retained": material["released_at"] == nil, "status": "saved", "life_write": false}
}
func (a *App) setStudioMaterialRetention(ctx context.Context, owner, id string, retain bool, body map[string]any) (map[string]any, error) {
	if !eventKeys(body, "expected_revision", "reason", "idempotency_key") {
		return nil, invalid("Unknown material release field")
	}
	action := "studio_material_release"
	if retain {
		action = "studio_material_retain"
	}
	ctx = context.WithValue(ctx, ownerCommandAuditActionKey{}, action)
	return a.worldCommand(ctx, owner, "", fmt.Sprintf("studio-material-retention:%s:%t", id, retain), body, func(ctx context.Context, tx pgx.Tx) (map[string]any, error) {
		before, err := readStudioMaterialWith(ctx, tx, owner, id, true)
		if err != nil {
			return nil, err
		}
		expected, ok := parseRevision(body["expected_revision"])
		if !ok || expected != intValue(before["revision"]) {
			return nil, conflict("studio_material", expected, intValue(before["revision"]))
		}
		if retain {
			if before["released_at"] == nil {
				return before, nil
			}
			var source []byte
			if err := tx.QueryRow(ctx, `SELECT to_jsonb(j) FROM public.studio_jobs j WHERE id=$1 AND owner_actor_id=$2 AND source_digest=$3`, before["job_id"], owner, before["source_digest"]).Scan(&source); err != nil {
				return nil, err
			}
			job := decodeObject(source)
			if before["asset_id"] != nil {
				var verified string
				if err := tx.QueryRow(ctx, `SELECT a.id FROM public.media_assets a JOIN public.media_intents i ON a.id='asset_'||i.id WHERE a.id=$1 AND a.status='ready' AND a.media_scope='studio' AND a.studio_owner_actor_id=$2 AND a.studio_job_id=$3 AND i.status='completed' AND i.studio_owner_actor_id=$2 AND i.studio_job_id=$3 FOR SHARE OF a`, before["asset_id"], owner, before["job_id"]).Scan(&verified); err != nil {
					if errors.Is(err, pgx.ErrNoRows) {
						return nil, &SurfaceError{Status: 409, Code: "studio_material_asset_unavailable", Message: "原图片已删除或来源不可用，不能恢复保留"}
					}
					return nil, err
				}
			}
			if _, err := tx.Exec(ctx, `UPDATE public.studio_resource_materials SET released_at=NULL,revision=revision+1 WHERE id=$1`, id); err != nil {
				return nil, err
			}
			after, err := readStudioMaterialWith(ctx, tx, owner, id, false)
			if err != nil {
				return nil, err
			}
			if after["source_digest"] != job["source_digest"] {
				return nil, ErrConflict
			}
			if err := recordStudioMaterialHistory(ctx, tx, owner, "retain", stringValue(body["reason"]), before, after); err != nil {
				return nil, err
			}
			return after, nil
		}
		if before["released_at"] != nil {
			return before, nil
		}
		if _, err := tx.Exec(ctx, `UPDATE public.studio_resource_materials SET released_at=now(),revision=revision+1 WHERE id=$1 AND owner_actor_id=$2`, id, owner); err != nil {
			return nil, err
		}
		after, err := readStudioMaterialWith(ctx, tx, owner, id, false)
		if err != nil {
			return nil, err
		}
		if err := recordStudioMaterialHistory(ctx, tx, owner, "release", stringValue(body["reason"]), before, after); err != nil {
			return nil, err
		}
		return after, nil

	})
}
func (a *App) studioMaterialHistory(ctx context.Context, owner, id string, q url.Values) (map[string]any, error) {
	if _, err := readStudioMaterialWith(ctx, a.DB.Pool(), owner, id, false); err != nil {
		return nil, err
	}
	limit, err := reflectionReadPage(q)
	if err != nil {
		return nil, err
	}
	scope := stableDigest(jsonString([]any{"studio-material-history", owner, id}))
	cursor, err := decodeOwnerReadCursor(q.Get("cursor"), scope)
	if err != nil {
		return nil, err
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT id,created_at,to_jsonb(h) FROM public.studio_material_history h WHERE owner_actor_id=$1 AND material_id=$2 AND ($3='' OR (created_at,id)<($4,$3)) ORDER BY created_at DESC,id DESC LIMIT $5`, owner, id, cursor.ID, cursor.Time, limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return readGovernanceRows(rows, scope, limit)
}

func (a *App) studioMaterialPage(ctx context.Context, owner string, q url.Values) (map[string]any, error) {
	released := q.Get("released")
	if released == "" {
		released = "false"
	}
	if released != "false" && released != "true" {
		return nil, invalid("Invalid material retention filter")
	}
	limit, err := reflectionReadPage(q)
	if err != nil {
		return nil, err
	}
	scope := stableDigest(jsonString([]any{"studio-materials", owner, q.Get("resource_id"), released}))
	cursor, err := decodeOwnerReadCursor(q.Get("cursor"), scope)
	if err != nil {
		return nil, err
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT id,created_at,to_jsonb(m) FROM public.studio_resource_materials m WHERE owner_actor_id=$1 AND ($2='' OR resource_id=$2) AND (released_at IS NOT NULL)=$3 AND ($4='' OR (created_at,id)<($5,$4)) ORDER BY created_at DESC,id DESC LIMIT $6`, owner, q.Get("resource_id"), released == "true", cursor.ID, cursor.Time, limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return readGovernanceRows(rows, scope, limit)
}
