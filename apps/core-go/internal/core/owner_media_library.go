package core

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5"
)

const ownedMediaAssetSQL = `((m.media_scope='fluctlight' AND EXISTS(SELECT 1 FROM public.fluctlights f WHERE f.id=m.owner_fluctlight_id AND f.created_by_actor_id=$1)) OR (m.media_scope='studio' AND m.studio_owner_actor_id=$1 AND EXISTS(SELECT 1 FROM public.studio_jobs j WHERE j.id=m.studio_job_id AND j.owner_actor_id=$1)) OR (m.media_scope='owner_upload' AND m.upload_owner_actor_id=$1 AND EXISTS(SELECT 1 FROM public.owner_media_uploads u WHERE u.id=m.owner_upload_id AND u.owner_actor_id=$1 AND u.asset_id=m.id)))`

const mediaAssetReadGrantSQL = `((m.media_scope='fluctlight' AND EXISTS(SELECT 1 FROM public.fluctlights f WHERE f.id=m.owner_fluctlight_id AND f.created_by_actor_id=$1)) OR (m.media_scope='studio' AND m.studio_owner_actor_id=$1 AND EXISTS(SELECT 1 FROM public.studio_jobs sj JOIN public.media_intents si ON si.studio_job_id=sj.id WHERE sj.id=m.studio_job_id AND sj.owner_actor_id=$1 AND si.studio_owner_actor_id=$1 AND si.media_scope='studio' AND si.status='completed' AND m.id='asset_'||si.id AND (sj.status='completed' OR EXISTS(SELECT 1 FROM public.studio_resource_materials sr WHERE sr.job_id=sj.id AND sr.owner_actor_id=$1 AND sr.asset_id=m.id AND sr.source_digest=sj.source_digest AND sr.released_at IS NULL)))) OR (m.media_scope='owner_upload' AND m.upload_owner_actor_id=$1 AND EXISTS(SELECT 1 FROM public.owner_media_uploads ou WHERE ou.id=m.owner_upload_id AND ou.owner_actor_id=$1 AND ou.asset_id=m.id AND ou.status='ready' AND ou.sha256=m.sha256 AND ou.byte_size=m.byte_size AND ou.mime_type=m.mime_type AND ou.bucket=m.bucket AND ou.object_key=m.object_key)))`

const mediaLibraryProjectionSQL = `jsonb_build_object('id',m.id,'asset_id',m.id,'version',m.version,'asset_revision',m.revision,'kind',m.kind,'mime_type',m.mime_type,'byte_size',m.byte_size,'sha256',m.sha256,'status',m.status,'origin',m.media_scope,'created_at',m.created_at,'ready_at',m.ready_at,'fluctlight_id',m.owner_fluctlight_id,'studio_job_id',m.studio_job_id,'resource_id',j.resource_id,'upload_id',m.owner_upload_id,'metadata_revision',COALESCE(a.revision,0),'title',CASE WHEN a.revision IS NOT NULL THEN a.title ELSE COALESCE(u.filename,j.snapshot->>'name','') END,'hidden',COALESCE(a.hidden,false),'storage_scope',u.storage_scope,'source_filename',u.filename,'source_upload_status',u.status,'source_job_status',j.status,'retained',EXISTS(SELECT 1 FROM public.studio_resource_materials r WHERE r.asset_id=m.id AND r.owner_actor_id=$1 AND r.job_id=m.studio_job_id AND r.released_at IS NULL),'generation_intent_id',i.id,'generation_status',i.status,'source_concept',i.prompt,'deletion_id',(SELECT d.id FROM public.media_deletions d WHERE d.asset_id=m.id AND d.owner_actor_id=$1),'grant_valid',` + mediaAssetReadGrantSQL + `)`
const mediaLibraryJoinsSQL = ` LEFT JOIN public.media_asset_annotations a ON a.asset_id=m.id AND a.owner_actor_id=$1 LEFT JOIN public.owner_media_uploads u ON u.id=m.owner_upload_id AND u.owner_actor_id=$1 LEFT JOIN public.studio_jobs j ON j.id=m.studio_job_id AND j.owner_actor_id=$1 LEFT JOIN public.media_intents i ON m.id='asset_'||i.id `

func MatchOwnerMediaSurface(method, path string) bool {
	p := surfaceParts(path)
	if mediaLifecycleRoute(method, p) {
		return true
	}
	if mediaReferenceGrantRoute(method, p) {
		return true
	}
	if ownerMediaReadRoute(method, p) {
		return true
	}
	if displayAvatarRoute(method, p) {
		return true
	}
	if len(p) == 1 && p[0] == "media-library" {
		return method == "GET"
	}
	if len(p) == 2 && p[0] == "media-assets" {
		return method == "GET" || method == "PUT"
	}
	return method == "GET" && len(p) == 2 && p[0] == "media-uploads" && p[1] == "by-key"
}

func (a *App) DispatchOwnerMediaSurface(ctx context.Context, method, path string, q url.Values, owner string, body map[string]any) (map[string]any, error) {
	if !MatchOwnerMediaSurface(method, path) {
		return nil, ErrNotFound
	}
	if err := a.requireMediaOwner(ctx, owner); err != nil {
		return nil, err
	}
	p := surfaceParts(path)
	if mediaLifecycleRoute(method, p) {
		return a.dispatchMediaLifecycle(ctx, method, p, q, owner, body)
	}
	if ownerMediaReadRoute(method, p) {
		return a.ownerMediaGenerationRead(ctx, owner, p, q)
	}
	if mediaReferenceGrantRoute(method, p) {
		return a.dispatchMediaReferenceGrants(ctx, method, p, q, owner, body)
	}
	if displayAvatarRoute(method, p) {
		return a.dispatchDisplayAvatar(ctx, method, p, q, owner, body)
	}
	if p[0] == "media-uploads" {
		key := strings.TrimSpace(q.Get("key"))
		if key == "" || len(key) > 256 {
			return nil, invalid("Original upload key required")
		}
		var raw []byte
		err := a.DB.Pool().QueryRow(ctx, `SELECT jsonb_build_object('id',u.id,'status',u.status,'asset_id',u.asset_id,'filename',u.filename,'mime_type',u.mime_type,'byte_size',u.byte_size,'sha256',u.sha256,'revision',u.revision,'error_code',u.error_code,'created_at',u.created_at,'asset_status',(SELECT m.status FROM public.media_assets m WHERE m.id=u.asset_id AND m.owner_upload_id=u.id AND m.upload_owner_actor_id=u.owner_actor_id)) FROM public.owner_media_uploads u WHERE owner_actor_id=$1 AND idempotency_key=$2`, owner, key).Scan(&raw)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		if err != nil {
			return nil, err
		}
		result := decodeObject(raw)
		if result["status"] == "ready" {
			asset, readErr := a.ownerMediaAssetWith(ctx, a.DB.Pool(), owner, stringValue(result["asset_id"]), false)
			if readErr != nil {
				return nil, readErr
			}
			result["asset"] = asset
		}
		return result, nil
	}
	if method == "GET" && p[0] == "media-library" {
		return a.ownerMediaLibrary(ctx, owner, q)
	}
	if method == "GET" {
		if fid := q.Get("fluctlight_id"); fid != "" {
			if _, err := a.DB.GetFluctlight(ctx, fid, owner); err != nil {
				return nil, err
			}
			var related bool
			if err := a.DB.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.media_assets m WHERE m.id=$1 AND (m.owner_fluctlight_id=$2 OR EXISTS(SELECT 1 FROM public.media_references r WHERE r.asset_id=m.id AND r.owner_fluctlight_id=$2)))`, p[1], fid).Scan(&related); err != nil {
				return nil, err
			}
			if !related {
				return nil, ErrNotFound
			}
		}
		return a.ownerMediaAssetWith(ctx, a.DB.Pool(), owner, p[1], false)
	}
	if !eventKeys(body, "expected_asset_revision", "expected_metadata_revision", "title", "hidden", "reason", "idempotency_key") {
		return nil, invalid("Unknown media metadata field")
	}
	ctx = context.WithValue(ctx, ownerCommandAuditActionKey{}, "media_metadata_edit")
	return a.worldCommand(ctx, owner, "", "media-metadata:"+p[1], body, func(ctx context.Context, tx pgx.Tx) (map[string]any, error) {
		before, err := a.ownerMediaAssetWith(ctx, tx, owner, p[1], true)
		if err != nil {
			return nil, err
		}
		asset, ok := parseRevision(body["expected_asset_revision"])
		metadata, valid := parseRevision(body["expected_metadata_revision"])
		if !ok || asset != intValue(before["asset_revision"]) {
			return nil, conflict("media_asset", asset, intValue(before["asset_revision"]))
		}
		if !valid || metadata != intValue(before["metadata_revision"]) {
			return nil, conflict("media_metadata", metadata, intValue(before["metadata_revision"]))
		}
		title, ok := body["title"].(string)
		hidden, present := body["hidden"].(bool)
		if !ok || len([]rune(title)) > 200 || !present {
			return nil, invalid("Explicit media title and visibility required")
		}
		if before["status"] == "deleted" {
			return nil, invalid("Deleted media metadata cannot be edited")
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.media_asset_annotations(owner_actor_id,asset_id,title,hidden,revision) VALUES($1,$2,$3,$4,1) ON CONFLICT(owner_actor_id,asset_id) DO UPDATE SET title=excluded.title,hidden=excluded.hidden,revision=media_asset_annotations.revision+1,updated_at=now()`, owner, p[1], title, hidden); err != nil {
			return nil, err
		}
		return a.ownerMediaAssetWith(ctx, tx, owner, p[1], false)
	})
}

func (a *App) ownerMediaAssetWith(ctx context.Context, q lifeContextQuerier, owner, id string, lock bool) (map[string]any, error) {
	suffix := ""
	if lock {
		// Acquire the asset lock in a separate statement. A SELECT that joins
		// annotations before waiting for this lock can retain a stale annotation
		// snapshot while another command only updates that annotation row.
		var locked string
		err := q.QueryRow(ctx, `SELECT m.id FROM public.media_assets m WHERE `+ownedMediaAssetSQL+` AND m.id=$2 FOR UPDATE OF m`, owner, id).Scan(&locked)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		if err != nil {
			return nil, err
		}
	}
	var raw []byte
	err := q.QueryRow(ctx, `SELECT `+mediaLibraryProjectionSQL+` FROM public.media_assets m `+mediaLibraryJoinsSQL+` WHERE `+ownedMediaAssetSQL+` AND m.id=$2`+suffix, owner, id).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return a.mediaLibraryView(decodeObject(raw)), nil
}

func (a *App) mediaLibraryView(row map[string]any) map[string]any {
	readable := row["status"] == "ready" && row["grant_valid"] == true
	state := "ready"
	if row["status"] == "ready" && !readable {
		state = "source_unavailable"
	}
	origin := stringValue(row["origin"])
	if row["status"] != "ready" {
		state = stringValue(row["status"])
	}
	label := "生成媒体·非事实"
	if row["kind"] == "image" {
		label = "生成图·非事实"
	}
	switch origin {
	case "owner_upload":
		label = "用户上传素材·非角色事实"
		if row["source_upload_status"] != "ready" {
			readable = false
			state = "upload_unsettled"
		}
		if row["storage_scope"] != a.ownerUploadStorageScope() {
			readable = false
			state = "storage_scope_changed"
		}
	case "studio":
		label = "虚构创作素材·不进入生活"
		if row["generation_status"] != "completed" || (row["source_job_status"] != "completed" && row["retained"] != true) {
			readable = false
			state = "studio_result_unavailable"
		}
	}
	row["readable"], row["read_state"], row["label"], row["factual"] = readable, state, label, false
	if readable {
		row["url"] = "/api/media/" + stringValue(row["id"])
	} else {
		row["url"] = nil
	}
	if a.Storage == nil {
		readable = false
		row["readable"] = false
		row["read_state"] = "storage_not_configured"
		row["url"] = nil
	}
	delete(row, "storage_scope")
	delete(row, "grant_valid")
	if text, ok := row["source_concept"].(string); ok && text != "" {
		parsed := decodeObject([]byte(text))
		if len(parsed) == 0 {
			row["source_state"] = "legacy_unstructured"
			row["source_snapshot"] = nil
		} else {
			snapshot := map[string]any{}
			for _, key := range []string{"purpose", "media_plan_version", "media_capture_plan", "context_binding", "studio_source", "reference_asset", "normalized_output"} {
				if value, present := parsed[key]; present {
					snapshot[key] = value
				}
			}
			row["source_state"] = "recorded"
			row["source_snapshot"] = snapshot
		}
	} else {
		row["source_state"] = "source_unrecorded"
		if origin == "owner_upload" {
			row["source_state"] = "owner_original"
		}
		row["source_snapshot"] = nil
	}
	delete(row, "source_concept")
	return row
}

func (a *App) ownerMediaLibrary(ctx context.Context, owner string, q url.Values) (map[string]any, error) {
	scope, fid, search, kind := q.Get("scope"), q.Get("fluctlight_id"), strings.TrimSpace(q.Get("q")), q.Get("kind")
	if scope == "" {
		scope = "all"
	}
	if !eventEnum(scope, "all", "fluctlight", "studio", "owner_upload") || !eventEnum(kind, "", "image", "video", "audio") || len([]rune(search)) > 200 {
		return nil, invalid("Invalid media library filters")
	}
	hidden := q.Get("hidden")
	if hidden == "" {
		hidden = "false"
	}
	if hidden != "true" && hidden != "false" {
		return nil, invalid("Invalid hidden filter")
	}
	if fid != "" {
		if _, err := a.DB.GetFluctlight(ctx, fid, owner); err != nil {
			return nil, err
		}
	}
	limit, err := reflectionReadPage(q)
	if err != nil {
		return nil, err
	}
	cursorScope := stableDigest(jsonString([]any{"media-library", owner, scope, fid, search, hidden, kind}))
	cursor, err := decodeOwnerReadCursor(q.Get("cursor"), cursorScope)
	if err != nil {
		return nil, err
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT m.id,m.created_at,`+mediaLibraryProjectionSQL+` FROM public.media_assets m `+mediaLibraryJoinsSQL+` WHERE `+ownedMediaAssetSQL+` AND m.status<>'deleted' AND ($2='all' OR m.media_scope=$2) AND ($3='' OR m.owner_fluctlight_id=$3 OR EXISTS(SELECT 1 FROM public.media_references r WHERE r.asset_id=m.id AND r.owner_fluctlight_id=$3)) AND COALESCE(a.hidden,false)=$4 AND ($5='' OR strpos(lower(COALESCE(a.title,u.filename,j.snapshot->>'name','')),lower($5))>0) AND ($6='' OR (m.created_at,m.id)<($7,$6)) AND ($9='' OR m.kind=$9) ORDER BY m.created_at DESC,m.id DESC LIMIT $8`, owner, scope, fid, hidden == "true", search, cursor.ID, cursor.Time, limit+1, kind)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	page, err := readGovernanceRows(rows, cursorScope, limit)
	if err != nil {
		return nil, err
	}
	for _, raw := range arrayValue(page["items"]) {
		a.mediaLibraryView(mapValue(raw))
	}
	return page, nil
}
