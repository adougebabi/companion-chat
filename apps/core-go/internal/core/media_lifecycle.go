package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/minio/minio-go/v7"
)

func mediaLifecycleRoute(method string, p []string) bool {
	return (len(p) == 2 && p[0] == "media-assets" && method == "DELETE") || (len(p) == 3 && p[0] == "media-assets" && p[2] == "references" && method == "GET") || (len(p) == 2 && p[0] == "media-deletions" && method == "GET") || (len(p) == 3 && p[0] == "media-deletions" && p[2] == "retry" && method == "POST")
}
func mediaDeletionWith(ctx context.Context, q lifeContextQuerier, owner, id string, lock bool) (map[string]any, error) {
	sql := `SELECT jsonb_build_object('id',id,'asset_id',asset_id,'status',status,'revision',revision,'attempt',attempt,'workflow_id',workflow_id,'task_id',intent_id,'error_code',error_code,'created_at',created_at,'updated_at',updated_at,'deleted_at',deleted_at,'execution_status',(SELECT w.status FROM public.platform_workflow_intents w WHERE w.intent_id=media_deletions.intent_id),'execution_error_code',(SELECT CASE WHEN w.last_error IS NULL THEN NULL ELSE 'workflow_execution_failed' END FROM public.platform_workflow_intents w WHERE w.intent_id=media_deletions.intent_id)) FROM public.media_deletions WHERE owner_actor_id=$1 AND id=$2`
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
func (a *App) dispatchMediaLifecycle(ctx context.Context, method string, p []string, q url.Values, owner string, body map[string]any) (map[string]any, error) {
	if method == "GET" && p[0] == "media-deletions" {
		return mediaDeletionWith(ctx, a.DB.Pool(), owner, p[1], false)
	}
	if method == "GET" {
		if _, err := a.ownerMediaAssetWith(ctx, a.DB.Pool(), owner, p[1], false); err != nil {
			return nil, err
		}
		return a.ownerMediaReferences(ctx, owner, p[1], q)
	}
	ctx = context.WithValue(ctx, ownerCommandAuditActionKey{}, "media_delete")
	return a.worldCommand(ctx, owner, "", method+":"+strings.Join(p, "/"), body, func(ctx context.Context, tx pgx.Tx) (map[string]any, error) {
		if p[0] == "media-deletions" {
			if !eventKeys(body, "expected_revision", "reason", "idempotency_key") {
				return nil, invalid("Unknown deletion retry field")
			}
			current, err := mediaDeletionWith(ctx, tx, owner, p[1], true)
			if err != nil {
				return nil, err
			}
			expected, ok := parseRevision(body["expected_revision"])
			if !ok || expected != intValue(current["revision"]) {
				return nil, conflict("media_deletion", expected, intValue(current["revision"]))
			}
			if current["status"] != "failed" {
				return nil, &SurfaceError{Status: 409, Code: "media_delete_not_retryable", Message: "原删除任务仍在处理或已经完成，请先查询实际结果"}
			}
			var active bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.platform_workflow_intents WHERE payload->>'deletion_id'=$1 AND status IN ('pending','started','retry','cancel_requested'))`, p[1]).Scan(&active); err != nil {
				return nil, err
			}
			if active {
				return nil, &SurfaceError{Status: 409, Code: "media_delete_execution_active", Message: "原删除执行尚未结束，请刷新任务结果后再重试"}
			}
			attempt := intValue(current["attempt"]) + 1
			workflow := fmt.Sprintf("media-delete:%s:%d", p[1], attempt)
			intent := "media_delete_intent_" + stableDigest(workflow)
			if _, err := tx.Exec(ctx, `UPDATE public.media_deletions SET status='pending',revision=revision+1,attempt=$2,workflow_id=$3,intent_id=$4,error_code=NULL,updated_at=now() WHERE id=$1`, p[1], attempt, workflow, intent); err != nil {
				return nil, err
			}
			if err := enqueueMediaDeletionTx(ctx, tx, owner, p[1], stringValue(current["asset_id"]), workflow, intent); err != nil {
				return nil, err
			}
			return mediaDeletionWith(ctx, tx, owner, p[1], false)
		}
		if !eventKeys(body, "expected_asset_revision", "confirm", "reason", "idempotency_key") || body["confirm"] != true {
			return nil, invalid("Explicit deletion confirmation required")
		}
		asset, err := a.ownerMediaAssetWith(ctx, tx, owner, p[1], true)
		if err != nil {
			return nil, err
		}
		expected, ok := parseRevision(body["expected_asset_revision"])
		if !ok || expected != intValue(asset["asset_revision"]) {
			return nil, conflict("media_asset", expected, intValue(asset["asset_revision"]))
		}
		var existing string
		err = tx.QueryRow(ctx, `SELECT id FROM public.media_deletions WHERE owner_actor_id=$1 AND asset_id=$2`, owner, p[1]).Scan(&existing)
		if err == nil {
			return mediaDeletionWith(ctx, tx, owner, existing, false)
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		if asset["status"] != "ready" && asset["status"] != "unavailable" {
			return nil, &SurfaceError{Status: 409, Code: "media_delete_status_invalid", Message: "当前资产状态不允许新删除任务，请核对原执行"}
		}
		references, err := mediaReferenceCountWith(ctx, tx, p[1])
		if err != nil {
			return nil, err
		}
		if references > 0 {
			return nil, &SurfaceError{Status: 409, Code: "media_asset_referenced", Message: "素材仍被实际对象或授权引用，请先处理引用", Details: map[string]any{"reference_count": references}}
		}
		if a.Storage == nil {
			return nil, &SurfaceError{Status: 503, Code: "media_storage_not_configured", Message: "对象存储未配置，尚未接受删除"}
		}
		if asset["origin"] == "owner_upload" && asset["read_state"] == "storage_scope_changed" {
			return nil, &SurfaceError{Status: 409, Code: "media_upload_storage_scope_changed", Message: "原上传的对象存储范围已改变，请恢复原配置后再删除"}
		}
		var raw []byte
		if err := tx.QueryRow(ctx, `SELECT jsonb_build_object('asset_id',id,'asset_version',version,'bucket',bucket,'object_key',object_key,'object_version',object_version,'sha256',sha256,'byte_size',byte_size,'mime_type',mime_type) FROM public.media_assets WHERE id=$1`, p[1]).Scan(&raw); err != nil {
			return nil, err
		}
		snapshot := decodeObject(raw)
		if version := stringValue(snapshot["object_version"]); version == "" || version == "null" {
			return nil, &SurfaceError{Status: 409, Code: "media_object_version_unknown", Message: "该旧资产缺少可追溯的对象版本，不能猜测删除目标"}
		}
		digest, err := hex.DecodeString(stringValue(snapshot["sha256"]))
		if err != nil || len(digest) != 32 || intValue(snapshot["byte_size"]) <= 0 || intValue(snapshot["byte_size"]) > int(OwnerMediaUploadMaxBytes) {
			return nil, &SurfaceError{Status: 409, Code: "media_delete_integrity_unverifiable", Message: "资产完整性资料不足，不能执行对象删除"}
		}
		snapshot["storage_scope"] = stableDigest(a.Storage.EndpointURL().String() + "\x1f" + stringValue(snapshot["bucket"]))
		id := randomID("media_delete_")
		workflow := "media-delete:" + id + ":1"
		intent := "media_delete_intent_" + stableDigest(workflow)
		if _, err := tx.Exec(ctx, `INSERT INTO public.media_deletions(id,owner_actor_id,asset_id,snapshot,status,workflow_id,intent_id) VALUES($1,$2,$3,$4,'pending',$5,$6)`, id, owner, p[1], jsonBytes(snapshot), workflow, intent); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `UPDATE public.media_assets SET status='tombstoned',tombstoned_at=now(),revision=revision+1 WHERE id=$1`, p[1]); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.media_tombstones(id,asset_id,reason) VALUES($1,$2,$3)`, id, p[1], body["reason"]); err != nil {
			return nil, err
		}
		if err := enqueueMediaDeletionTx(ctx, tx, owner, id, p[1], workflow, intent); err != nil {
			return nil, err
		}
		return mediaDeletionWith(ctx, tx, owner, id, false)
	})
}
func enqueueMediaDeletionTx(ctx context.Context, tx pgx.Tx, owner, id, asset, workflow, intent string) error {
	_, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload) VALUES($1,$2,'media','media.delete',$3)`, intent, workflow, jsonBytes(map[string]any{"intent_id": id, "deletion_id": id, "asset_id": asset, "owner_actor_id": owner}))
	return err
}

const mediaReferencesUnionSQL = `SELECT r.id,r.created_at,r.owner_fluctlight_id AS fluctlight_id,r.target_type,r.target_id,COALESCE((SELECT f.created_by_actor_id FROM public.fluctlights f WHERE f.id=r.owner_fluctlight_id),CASE WHEN r.target_type='moment' THEN (SELECT m.owner_actor_id FROM public.moments m WHERE m.id=r.target_id) END) AS owner_actor_id FROM public.media_references r WHERE r.asset_id=$1
 UNION ALL SELECT 'canonical:'||v.fluctlight_id,COALESCE(m.ready_at,m.created_at),v.fluctlight_id,'visual_identity',v.fluctlight_id,(SELECT f.created_by_actor_id FROM public.fluctlights f WHERE f.id=v.fluctlight_id) FROM public.fluctlight_visual_identities v JOIN public.media_assets m ON m.id=$1 WHERE v.canonical_asset_id=$1 OR v.character_sheet_asset_id=$1
 UNION ALL SELECT 'visual-attempt:'||a.id,a.created_at,s.fluctlight_id,'visual_attempt',s.id,(SELECT f.created_by_actor_id FROM public.fluctlights f WHERE f.id=s.fluctlight_id) FROM public.fluctlight_visual_identity_attempts a JOIN public.fluctlight_visual_identity_sessions s ON s.id=a.session_id WHERE (a.candidate_asset_id=$1 OR 'asset_'||a.media_intent_id=$1) AND s.status IN ('queued','running','character_sheet_pending','awaiting_review')
 UNION ALL SELECT 'visual-sheet:'||s.id,s.created_at,s.fluctlight_id,'visual_session',s.id,(SELECT f.created_by_actor_id FROM public.fluctlights f WHERE f.id=s.fluctlight_id) FROM public.fluctlight_visual_identity_sessions s WHERE 'asset_'||s.character_sheet_media_intent_id=$1 AND s.status IN ('queued','running','character_sheet_pending','awaiting_review')
 UNION ALL SELECT 'producer:'||i.id,i.created_at,i.owner_fluctlight_id,'media_producer',i.id,COALESCE(i.studio_owner_actor_id,(SELECT f.created_by_actor_id FROM public.fluctlights f WHERE f.id=i.owner_fluctlight_id)) FROM public.media_intents i WHERE 'asset_'||i.id=$1 AND i.status IN ('pending','running')

 UNION ALL SELECT 'material:'||m.id,m.created_at,NULL::varchar,'studio_material',m.id,m.owner_actor_id FROM public.studio_resource_materials m WHERE m.asset_id=$1 AND m.released_at IS NULL
 UNION ALL SELECT 'grant:'||g.id,g.created_at,CASE WHEN g.target_scope='fluctlight' THEN g.target_id ELSE NULL END,'model_reference_grant',g.id,g.owner_actor_id FROM public.media_reference_grants g WHERE g.asset_id=$1 AND g.status='active'
 UNION ALL SELECT 'generation:'||i.id,i.created_at,i.owner_fluctlight_id,'media_generation',i.id,COALESCE(i.studio_owner_actor_id,(SELECT f.created_by_actor_id FROM public.fluctlights f WHERE f.id=i.owner_fluctlight_id)) FROM public.media_intents i WHERE i.status IN ('pending','running') AND CASE WHEN i.prompt IS JSON OBJECT THEN i.prompt::jsonb->'reference_asset'->>'asset_id' ELSE NULL END=$1`

func mediaReferenceCountWith(ctx context.Context, q lifeContextQuerier, id string) (int, error) {
	var count int
	err := q.QueryRow(ctx, `SELECT count(*) FROM (`+mediaReferencesUnionSQL+`) r`, id).Scan(&count)
	return count, err
}
func (a *App) ownerMediaReferences(ctx context.Context, owner, id string, q url.Values) (map[string]any, error) {
	limit, err := reflectionReadPage(q)
	if err != nil {
		return nil, err
	}
	scope := stableDigest(jsonString([]any{"media-references", owner, id}))
	cursor, err := decodeOwnerReadCursor(q.Get("cursor"), scope)
	if err != nil {
		return nil, err
	}
	count, err := mediaReferenceCountWith(ctx, a.DB.Pool(), id)
	if err != nil {
		return nil, err
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT r.id,r.created_at,jsonb_build_object('id',r.id,'fluctlight_id',r.fluctlight_id,'target_type',r.target_type,'target_id',r.target_id,'created_at',r.created_at,'conversation_id',CASE WHEN r.target_type='conversation_message' THEN (SELECT m.conversation_id FROM public.conversation_messages m JOIN public.conversation_participants p ON p.conversation_id=m.conversation_id AND p.actor_id=$2 AND p.status='active' WHERE m.id=r.target_id LIMIT 1) WHEN r.target_type='conversation' THEN r.target_id ELSE NULL END) FROM (`+mediaReferencesUnionSQL+`) r WHERE r.owner_actor_id=$2 AND ($3='' OR (r.created_at,r.id)<($4,$3)) ORDER BY r.created_at DESC,r.id DESC LIMIT $5`, id, owner, cursor.ID, cursor.Time, limit+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	page, err := readGovernanceRows(rows, scope, limit)
	if err != nil {
		return nil, err
	}
	page["reference_count"] = count
	return page, nil
}

func (a *App) ProcessMediaDeletion(ctx context.Context, id, sourceWorkflow string) (result map[string]any, runErr error) {
	if a == nil || a.DB == nil {
		return nil, errors.New("media_deletion_service_unavailable")
	}
	var owner string
	if err := a.DB.Pool().QueryRow(ctx, `SELECT owner_actor_id FROM public.media_deletions WHERE id=$1`, id).Scan(&owner); err != nil {
		return nil, err
	}
	release, acquired, err := a.tryDailyReviewExecutionLock(ctx, owner, "media-delete:"+id)
	if err != nil {
		return nil, err
	}
	if !acquired {
		return nil, &SurfaceError{Status: 409, Code: "media_delete_execution_active", Message: "同一对象删除仍在处理"}
	}
	defer release()
	defer func() {
		if runErr != nil {
			cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			code := "media_delete_object_failed"
			var surface *SurfaceError
			if errors.As(runErr, &surface) {
				code = surface.Code
			}
			_, _ = a.DB.Pool().Exec(cleanup, `UPDATE public.media_deletions SET status='failed',error_code=$3,revision=revision+1,updated_at=now() WHERE id=$1 AND workflow_id=$2 AND status<>'deleted'`, id, mediaWorkflowIdentity(sourceWorkflow), code)
		}
	}()
	var snapshot map[string]any
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var raw []byte
		var status, workflow string
		if err := tx.QueryRow(ctx, `SELECT snapshot,status,workflow_id FROM public.media_deletions WHERE id=$1 AND owner_actor_id=$2 FOR UPDATE`, id, owner).Scan(&raw, &status, &workflow); err != nil {
			return err
		}
		if sourceWorkflow == "" || mediaWorkflowIdentity(sourceWorkflow) != mediaWorkflowIdentity(workflow) {
			return &SurfaceError{Status: 409, Code: "media_delete_execution_stale", Message: "删除执行已被新的明确尝试替代"}
		}
		if status == "deleted" {
			result = map[string]any{"id": id, "status": "deleted", "replayed": true}
			return nil
		}
		snapshot = decodeObject(raw)
		var current string
		if err := tx.QueryRow(ctx, `SELECT status FROM public.media_assets WHERE id=$1 FOR UPDATE`, snapshot["asset_id"]).Scan(&current); err != nil {
			return err
		}
		if current != "tombstoned" {
			return &SurfaceError{Status: 409, Code: "media_delete_asset_changed", Message: "删除对象状态已变化，原任务停止"}
		}
		count, err := mediaReferenceCountWith(ctx, tx, stringValue(snapshot["asset_id"]))
		if err != nil {
			return err
		}
		if count > 0 {
			return &SurfaceError{Status: 409, Code: "media_asset_referenced", Message: "删除期间出现了新的实际引用，尚未删除对象", Details: map[string]any{"reference_count": count}}
		}
		_, err = tx.Exec(ctx, `UPDATE public.media_deletions SET status='running',error_code=NULL,revision=revision+1,updated_at=now() WHERE id=$1`, id)
		return err
	})
	if err != nil {
		return nil, err
	}
	if result != nil {
		return result, nil
	}

	if a.Storage == nil {
		return nil, &SurfaceError{Status: 503, Code: "media_storage_not_configured", Message: "对象存储未配置，原删除任务保留"}
	}
	bucket, key, version := stringValue(snapshot["bucket"]), stringValue(snapshot["object_key"]), stringValue(snapshot["object_version"])
	if snapshot["storage_scope"] != stableDigest(a.Storage.EndpointURL().String()+"\x1f"+bucket) {
		return nil, &SurfaceError{Status: 409, Code: "media_delete_storage_scope_changed", Message: "对象存储范围已变化，未在新位置删除任何对象"}
	}
	stat, err := a.Storage.StatObject(ctx, bucket, key, minio.StatObjectOptions{VersionID: version})
	absent := mediaObjectVersionAbsent(err)
	if err != nil && !absent {
		return nil, mediaDeletionStorageError(err)
	}
	if !absent {
		if stat.Size != int64(intValue(snapshot["byte_size"])) || canonicalUploadMIME(stat.ContentType) != snapshot["mime_type"] {
			return nil, &SurfaceError{Status: 409, Code: "media_delete_integrity_conflict", Message: "待删对象与原资产的长度或类型不一致，未删除"}
		}
		object, err := a.Storage.GetObject(ctx, bucket, key, minio.GetObjectOptions{VersionID: version})
		if err != nil {
			return nil, mediaDeletionStorageError(err)
		}
		digest := sha256.New()
		size, readErr := io.Copy(digest, io.LimitReader(object, OwnerMediaUploadMaxBytes+1))
		object.Close()
		if readErr != nil {
			return nil, mediaDeletionStorageError(readErr)
		}
		if size != int64(intValue(snapshot["byte_size"])) || hex.EncodeToString(digest.Sum(nil)) != snapshot["sha256"] {
			return nil, &SurfaceError{Status: 409, Code: "media_delete_integrity_conflict", Message: "待删对象与原资产摘要不一致，未删除"}
		}
		if err := ctx.Err(); err != nil {
			return nil, mediaDeletionStorageError(err)
		}
		if err := a.Storage.RemoveObject(ctx, bucket, key, minio.RemoveObjectOptions{VersionID: version}); err != nil {
			return nil, mediaDeletionStorageError(err)
		}
		if _, err := a.Storage.StatObject(ctx, bucket, key, minio.StatObjectOptions{VersionID: version}); !mediaObjectVersionAbsent(err) {
			if err != nil {
				return nil, mediaDeletionStorageError(err)
			}
			return nil, &SurfaceError{Status: 503, Code: "media_delete_not_confirmed", Message: "指定对象版本删除尚未确认，原身份保持"}
		}
	}
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var workflow, status string
		if err := tx.QueryRow(ctx, `SELECT workflow_id,status FROM public.media_deletions WHERE id=$1 FOR UPDATE`, id).Scan(&workflow, &status); err != nil {
			return err
		}
		if mediaWorkflowIdentity(sourceWorkflow) != mediaWorkflowIdentity(workflow) {
			return ErrConflict
		}
		updated, err := tx.Exec(ctx, `UPDATE public.media_assets SET status='deleted',deleted_at=COALESCE(deleted_at,now()),revision=revision+1 WHERE id=$1 AND status='tombstoned' AND bucket=$2 AND object_key=$3 AND object_version=$4 AND sha256=$5`, snapshot["asset_id"], snapshot["bucket"], snapshot["object_key"], snapshot["object_version"], snapshot["sha256"])
		if err != nil {
			return err
		}
		if updated.RowsAffected() != 1 {
			return ErrConflict
		}
		if _, err := tx.Exec(ctx, `UPDATE public.media_deletions SET status='deleted',deleted_at=COALESCE(deleted_at,now()),error_code=NULL,revision=revision+1,updated_at=now() WHERE id=$1`, id); err != nil {
			return err
		}
		return appendOutboxTx(ctx, tx, "media.asset.deleted", "media_asset", stringValue(snapshot["asset_id"]), "", id, "media-delete:"+id, "media-deleted:"+id, map[string]any{"asset_id": snapshot["asset_id"], "deletion_id": id, "owner_actor_id": owner})
	})
	if err != nil {
		return nil, &SurfaceError{Status: 503, Code: "media_delete_result_unknown", Message: "对象版本已处理但提交结果未确认，请查询或重试原删除"}
	}
	return map[string]any{"id": id, "asset_id": snapshot["asset_id"], "status": "deleted", "object_version_absent": true}, nil
}
func mediaObjectVersionAbsent(err error) bool {
	if err == nil {
		return false
	}
	switch minio.ToErrorResponse(err).Code {
	case "NoSuchKey", "NoSuchObject", "NoSuchVersion", "NotFound":
		return true
	}
	return false
}
func mediaDeletionStorageError(err error) error {
	return &SurfaceError{Status: 503, Code: "media_delete_object_failed", Message: "对象版本删除尚未完成，原任务保持", Details: map[string]any{"storage_error_code": ownerMediaStorageErrorCode(err)}}
}

func (a *App) ReconcileMediaDeletionTerminal(ctx context.Context, intentID, status string) error {
	if status != "completed" && status != "failed" && status != "cancelled" && status != "terminated" && status != "timed_out" {
		return nil
	}
	code := "media_delete_workflow_terminal"
	if status == "completed" {
		code = "media_delete_result_incomplete"
	}
	_, err := a.DB.Pool().Exec(ctx, `UPDATE public.media_deletions d SET status='failed',error_code=COALESCE(error_code,$2),revision=revision+1,updated_at=now() FROM public.platform_workflow_intents i WHERE i.intent_id=$1 AND d.intent_id=i.intent_id AND d.workflow_id=i.workflow_id AND d.status IN ('pending','running')`, intentID, code)
	return err
}
