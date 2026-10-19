package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/minio/minio-go/v7"
)

func mediaReferenceGrantRoute(method string, p []string) bool {
	return len(p) > 0 && p[0] == "media-reference-grants" && ((len(p) == 1 && (method == "GET" || method == "POST")) || (len(p) == 2 && method == "GET") || (len(p) == 3 && ((p[2] == "revoke" && method == "POST") || ((p[2] == "history" || p[2] == "uses") && method == "GET"))))
}
func readMediaReferenceGrant(ctx context.Context, q lifeContextQuerier, owner, id string, lock bool) (map[string]any, error) {
	var raw []byte
	sql := `SELECT to_jsonb(g)-'target_resource_id'-'target_fluctlight_id' FROM public.media_reference_grants g WHERE owner_actor_id=$1 AND id=$2`
	if lock {
		sql += " FOR UPDATE"
	}
	err := q.QueryRow(ctx, sql, owner, id).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return decodeObject(raw), err
}
func (a *App) dispatchMediaReferenceGrants(ctx context.Context, method string, p []string, q url.Values, owner string, body map[string]any) (map[string]any, error) {
	if method == "GET" && len(p) == 1 {
		return a.mediaReferenceGrantPage(ctx, owner, q)
	}
	if method == "GET" {
		grant, err := readMediaReferenceGrant(ctx, a.DB.Pool(), owner, p[1], false)
		if err != nil {
			return nil, err
		}
		if len(p) == 2 {
			return grant, nil
		}
		return a.mediaReferenceGrantRecords(ctx, owner, p[1], p[2], q)
	}
	if containsPermanentHarm(body) {
		return nil, harmError()
	}
	ctx = context.WithValue(ctx, ownerCommandAuditActionKey{}, "media_reference_grant")
	fid := ""
	if len(p) == 1 && body["target_scope"] == "fluctlight" {
		fid = stringValue(body["target_id"])
	}
	return a.worldCommand(ctx, owner, fid, method+":"+strings.Join(p, "/"), body, func(ctx context.Context, tx pgx.Tx) (map[string]any, error) {
		if len(p) == 3 {
			if !eventKeys(body, "expected_revision", "reason", "idempotency_key") {
				return nil, invalid("Unknown reference revocation field")
			}
			before, err := readMediaReferenceGrant(ctx, tx, owner, p[1], true)
			if err != nil {
				return nil, err
			}
			expected, ok := parseRevision(body["expected_revision"])
			if !ok || expected != intValue(before["revision"]) {
				return nil, conflict("media_reference_grant", expected, intValue(before["revision"]))
			}
			if before["status"] != "active" {
				return nil, invalid("Reference authorization is already revoked")
			}
			if _, err := tx.Exec(ctx, `UPDATE public.media_reference_grants SET status='revoked',revision=revision+1,updated_at=now() WHERE id=$1 AND owner_actor_id=$2`, p[1], owner); err != nil {
				return nil, err
			}
			after, err := readMediaReferenceGrant(ctx, tx, owner, p[1], false)
			if err != nil {
				return nil, err
			}
			if err := recordMediaReferenceGrantHistory(ctx, tx, owner, before, after, stringValue(body["reason"])); err != nil {
				return nil, err
			}
			return after, nil
		}
		if !eventKeys(body, "asset_id", "expected_asset_revision", "target_scope", "target_id", "purpose", "confirm", "reason", "idempotency_key") {
			return nil, invalid("Unknown reference authorization field")
		}
		if body["confirm"] != true || !eventEnum(stringValue(body["target_scope"]), "studio_resource", "fluctlight") || !eventEnum(stringValue(body["purpose"]), "scene", "animal", "object", "person") {
			return nil, invalid("Explicit target, reference purpose and confirmation required")
		}
		targetID := stringValue(body["target_id"])
		if body["target_scope"] == "studio_resource" {
			if err := lockStudioReferenceGraphTx(ctx, tx, owner); err != nil {
				return nil, err
			}
			target, exists, err := (postgresSurfaceTx{tx}).Get(ctx, owner, targetID)
			if err != nil {
				return nil, err
			}
			if !exists || target.Kind != "studio_resource" {
				return nil, ErrNotFound
			}
			if err := activeStudioResource(target.Status); err != nil {
				return nil, err
			}
		} else {
			if err := requireOwnerMutableTx(ctx, tx, owner, targetID); err != nil {
				return nil, err
			}
		}
		asset, err := a.ownerMediaAssetWith(ctx, tx, owner, stringValue(body["asset_id"]), true)
		if err != nil {
			return nil, err
		}
		expected, ok := parseRevision(body["expected_asset_revision"])
		if !ok || expected != intValue(asset["asset_revision"]) {
			return nil, conflict("media_asset", expected, intValue(asset["asset_revision"]))
		}
		if asset["kind"] != "image" || asset["readable"] != true || intValue(asset["byte_size"]) > 32<<20 {
			return nil, &SurfaceError{Status: 422, Code: "media_reference_image_unreadable", Message: "参考需要当前可读取且不超过32MiB的图片素材"}
		}
		id := randomID("media_grant_")
		if _, err := tx.Exec(ctx, `INSERT INTO public.media_reference_grants(id,owner_actor_id,asset_id,asset_version,asset_revision,sha256,byte_size,mime_type,target_scope,target_id,purpose,provenance,status) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'owner_explicit_unverified_content','active')`, id, owner, asset["id"], asset["version"], asset["asset_revision"], asset["sha256"], asset["byte_size"], asset["mime_type"], body["target_scope"], targetID, body["purpose"]); err != nil {
			return nil, err
		}
		after, err := readMediaReferenceGrant(ctx, tx, owner, id, false)
		if err != nil {
			return nil, err
		}
		if err := recordMediaReferenceGrantHistory(ctx, tx, owner, nil, after, stringValue(body["reason"])); err != nil {
			return nil, err
		}
		return after, nil
	})
}
func recordMediaReferenceGrantHistory(ctx context.Context, tx pgx.Tx, owner string, before, after map[string]any, reason string) error {
	var previous any
	if before != nil {
		previous = jsonBytes(before)
	}
	_, err := tx.Exec(ctx, `INSERT INTO public.media_reference_grant_history(id,grant_id,owner_actor_id,revision,before_value,after_value,reason) VALUES($1,$2,$3,$4,$5,$6,$7)`, randomID("media_grant_rev_"), after["id"], owner, after["revision"], previous, jsonBytes(after), reason)
	return err
}
func (a *App) mediaReferenceGrantPage(ctx context.Context, owner string, q url.Values) (map[string]any, error) {
	status := q.Get("status")
	if status == "" {
		status = "active"
	}
	if !eventEnum(status, "active", "revoked") || !eventEnum(q.Get("target_scope"), "", "studio_resource", "fluctlight") {
		return nil, invalid("Invalid reference grant filter")
	}
	limit, err := reflectionReadPage(q)
	if err != nil {
		return nil, err
	}
	scope := stableDigest(jsonString([]any{"media-reference-grants", owner, status, q.Get("asset_id"), q.Get("target_id"), q.Get("target_scope")}))
	cursor, err := decodeOwnerReadCursor(q.Get("cursor"), scope)
	if err != nil {
		return nil, err
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT id,created_at,to_jsonb(g)-'target_resource_id'-'target_fluctlight_id' FROM public.media_reference_grants g WHERE owner_actor_id=$1 AND status=$2 AND ($3='' OR asset_id=$3) AND ($4='' OR target_id=$4) AND ($8='' OR target_scope=$8) AND ($5='' OR (created_at,id)<($6,$5)) ORDER BY created_at DESC,id DESC LIMIT $7`, owner, status, q.Get("asset_id"), q.Get("target_id"), cursor.ID, cursor.Time, limit+1, q.Get("target_scope"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return readGovernanceRows(rows, scope, limit)
}

func (a *App) ownerMediaReferenceActor(ctx context.Context, fid string, reference, concept map[string]any) (string, error) {
	id := stringValue(mapValue(concept["owner_media_request"])["request_id"])
	if id == "" {
		return "", ErrUnauthorized
	}
	var actor string
	err := a.DB.Pool().QueryRow(ctx, `SELECT r.owner_actor_id FROM public.owner_surface_records r JOIN public.fluctlights f ON f.id=r.fluctlight_id AND f.created_by_actor_id=r.owner_actor_id WHERE r.id=$1 AND r.fluctlight_id=$2 AND r.kind='owner_request' AND r.payload->>'kind' IN ('media_generate','item_photo') AND r.payload->'payload'->'reference_asset'=$3::jsonb`, id, fid, jsonBytes(reference)).Scan(&actor)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrUnauthorized
	}
	return actor, err
}
func (a *App) mediaReferenceGrantRecords(ctx context.Context, owner, id, kind string, q url.Values) (map[string]any, error) {
	limit, err := reflectionReadPage(q)
	if err != nil {
		return nil, err
	}
	scope := stableDigest(jsonString([]any{"media-grant-records", owner, id, kind}))
	cursor, err := decodeOwnerReadCursor(q.Get("cursor"), scope)
	if err != nil {
		return nil, err
	}
	sql := `SELECT id,created_at,to_jsonb(h) FROM public.media_reference_grant_history h WHERE owner_actor_id=$1 AND grant_id=$2 AND ($3='' OR (created_at,id)<($4,$3)) ORDER BY created_at DESC,id DESC LIMIT $5`
	if kind == "uses" {
		sql = `SELECT id,created_at,jsonb_build_object('id',id,'resource_id',resource_id,'status',status,'kind',kind,'created_at',created_at,'reference',snapshot->'reference_asset') FROM public.studio_jobs WHERE owner_actor_id=$1 AND snapshot->'reference_asset'->>'grant_id'=$2 AND ($3='' OR (created_at,id)<($4,$3))
 UNION ALL SELECT r.id,r.created_at,jsonb_build_object('id',r.id,'fluctlight_id',r.fluctlight_id,'status',r.status,'media_status',i.status,'kind','owner_image_request','created_at',r.created_at,'reference',r.payload->'payload'->'reference_asset') FROM public.owner_surface_records r LEFT JOIN public.media_intents i ON i.owner_request_id=r.id WHERE r.owner_actor_id=$1 AND r.kind='owner_request' AND r.payload->>'kind' IN ('media_generate','item_photo') AND r.payload->'payload'->'reference_asset'->>'grant_id'=$2 AND ($3='' OR (r.created_at,r.id)<($4,$3)) ORDER BY created_at DESC,id DESC LIMIT $5`
	}
	rows, err := a.DB.Pool().Query(ctx, sql, owner, id, cursor.ID, cursor.Time, limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return readGovernanceRows(rows, scope, limit)
}

// A readable image is not itself a model authorization. Explicit grants bind
// exact content to one Owner, target resource and declared use; content remains
// unverified rather than being relabeled as a visual inspection result.
func (a *App) validateExplicitMediaReference(ctx context.Context, q lifeContextQuerier, owner, target string, reference map[string]any) error {
	return a.validateExplicitMediaReferenceFor(ctx, q, owner, "studio_resource", target, reference)
}
func (a *App) validateExplicitMediaReferenceFor(ctx context.Context, q lifeContextQuerier, owner, scope, target string, reference map[string]any) error {
	if !eventKeys(reference, "asset_id", "purpose", "grant_id", "grant_revision") {
		return invalid("Invalid explicit media reference")
	}
	grant, err := readMediaReferenceGrant(ctx, q, owner, stringValue(reference["grant_id"]), false)
	if err != nil {
		return err
	}
	revision, ok := parseRevision(reference["grant_revision"])
	if !ok || revision != intValue(grant["revision"]) || grant["status"] != "active" {
		return &SurfaceError{Status: 409, Code: "media_reference_authorization_changed", Message: "图片参考授权已撤销或版本变化，请重新核对"}
	}
	if target == "" || grant["target_scope"] != scope || grant["target_id"] != target || grant["asset_id"] != reference["asset_id"] || grant["purpose"] != reference["purpose"] {
		return &SurfaceError{Status: 403, Code: "media_reference_scope_forbidden", Message: "这份参考授权不适用于当前目标或用途"}
	}
	asset, err := a.ownerMediaAssetWith(ctx, q, owner, stringValue(grant["asset_id"]), false)
	if err != nil {
		return err
	}
	if asset["readable"] != true || asset["kind"] != "image" || asset["version"] != grant["asset_version"] || asset["sha256"] != grant["sha256"] || intValue(asset["asset_revision"]) != intValue(grant["asset_revision"]) || intValue(asset["byte_size"]) != intValue(grant["byte_size"]) || asset["mime_type"] != grant["mime_type"] {
		return &SurfaceError{Status: 409, Code: "media_reference_source_changed", Message: "原参考图片来源或内容已不可用，请核对原素材"}
	}
	return nil
}

func (a *App) readExplicitStudioReference(ctx context.Context, owner string, reference, concept map[string]any) (string, []byte, error) {
	target := stringValue(mapValue(concept["studio_source"])["resource_id"])
	return a.readExplicitMediaReference(ctx, owner, "studio_resource", target, reference)
}
func (a *App) readExplicitMediaReference(ctx context.Context, owner, scope, target string, reference map[string]any) (string, []byte, error) {
	if err := a.validateExplicitMediaReferenceFor(ctx, a.DB.Pool(), owner, scope, target, reference); err != nil {
		return "", nil, err
	}
	if a.Storage == nil {
		return "", nil, errors.New("media_reference_storage_unavailable")
	}
	var bucket, key, version, mimeType, checksum string
	var size int64
	if err := a.DB.Pool().QueryRow(ctx, `SELECT m.bucket,m.object_key,COALESCE(m.object_version,''),m.mime_type,m.sha256,m.byte_size FROM public.media_assets m WHERE m.id=$2 AND m.status='ready' AND `+mediaAssetReadGrantSQL, owner, reference["asset_id"]).Scan(&bucket, &key, &version, &mimeType, &checksum, &size); err != nil {
		return "", nil, err
	}
	object, err := a.Storage.GetObject(ctx, bucket, key, minio.GetObjectOptions{VersionID: version})
	if err != nil {
		return "", nil, err
	}
	defer object.Close()
	content, err := io.ReadAll(io.LimitReader(object, (32<<20)+1))
	if err != nil {
		return "", nil, err
	}
	digest := sha256.Sum256(content)
	if int64(len(content)) != size || len(content) > 32<<20 || hex.EncodeToString(digest[:]) != checksum {
		return "", nil, errors.New("media_reference_content_integrity_failed")
	}
	// Revocation during a slow object read must stop the subsequent model upload.
	if err := a.validateExplicitMediaReferenceFor(ctx, a.DB.Pool(), owner, scope, target, reference); err != nil {
		return "", nil, err
	}
	return mimeType, content, nil
}
