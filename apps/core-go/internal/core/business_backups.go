package core

import (
	"context"
	"errors"
	"net/url"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var isolatedRecoveryName = regexp.MustCompile(`^fluctlight_restore_[a-z0-9]{8,32}$`)

const backupJobProjection = `jsonb_build_object('id',id,'kind',kind,'source_backup_id',source_backup_id,'target_name',target_name,'status',status,'revision',revision,'attempt',attempt,'reason',reason,'stage',stage,'result',result,'steps',steps,'error_code',error_code,'created_at',created_at,'started_at',started_at,'finished_at',finished_at)`

func (a *App) OwnerBackupJobs(ctx context.Context, owner string, q url.Values) (map[string]any, error) {
	if err := a.requireOwner(ctx, owner); err != nil {
		return nil, err
	}
	scope := stableDigest("backups:" + owner)
	cursor, err := decodeOwnerReadCursor(q.Get("cursor"), scope)
	if err != nil {
		return nil, invalid("备份任务游标无效")
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT id,created_at,`+backupJobProjection+` FROM public.business_backup_jobs WHERE owner_actor_id=$1 AND ($2='' OR (created_at,id)<($3,$2)) ORDER BY created_at DESC,id DESC LIMIT 21`, owner, cursor.ID, cursor.Time)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []any{}
	times := []time.Time{}
	ids := []string{}
	for rows.Next() {
		var id string
		var at time.Time
		var raw []byte
		if err := rows.Scan(&id, &at, &raw); err != nil {
			return nil, err
		}
		items = append(items, decodeObject(raw))
		ids = append(ids, id)
		times = append(times, at)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	next := ""
	if len(items) > 20 {
		next = encodeOwnerReadCursor(ownerReadCursor{Scope: scope, ID: ids[19], Time: times[19]})
		items = items[:20]
	}
	return map[string]any{"items": items, "next_cursor": next, "strategy": "offline_compose_volume_v1", "execution": "通过有维护权限的本地备份工具执行；浏览器只提交任务和读取实际回执，不执行任意主机命令", "maintenance_required": true}, nil
}
func (a *App) OwnerBackupJob(ctx context.Context, owner, id string) (map[string]any, error) {
	if err := a.requireOwner(ctx, owner); err != nil {
		return nil, err
	}
	var raw []byte
	err := a.DB.Pool().QueryRow(ctx, `SELECT `+backupJobProjection+` FROM public.business_backup_jobs WHERE id=$1 AND owner_actor_id=$2`, id, owner).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return decodeObject(raw), err
}
func (a *App) OwnerBackupCommand(ctx context.Context, owner, id, action string, body map[string]any) (map[string]any, error) {
	if err := a.requireOwner(ctx, owner); err != nil {
		return nil, err
	}
	if !eventKeys(body, "reason", "idempotency_key", "expected_revision", "confirm", "target_name") {
		return nil, ErrInvalidArguments
	}
	if action == "restore" && (body["confirm"] != true || !isolatedRecoveryName.MatchString(stringValue(body["target_name"]))) {
		return nil, invalid("恢复只允许明确确认后的全新fluctlight_restore_隔离目标，不能覆盖运行环境")
	}
	if action != "restore" && (body["confirm"] != nil || (action != "retry" && body["target_name"] != nil)) {
		return nil, ErrInvalidArguments
	}
	ctx = context.WithValue(ctx, ownerCommandAuditActionKey{}, "business_backup_"+action)
	return a.worldCommand(ctx, owner, "", "business-backup:"+id+":"+action, body, func(ctx context.Context, tx pgx.Tx) (map[string]any, error) {
		if action == "create" {
			job := randomID("backup_")
			if _, err := tx.Exec(ctx, `INSERT INTO public.business_backup_jobs(id,owner_actor_id,kind,status,reason) VALUES($1,$2,'backup','queued',$3)`, job, owner, body["reason"]); err != nil {
				return nil, err
			}
			return map[string]any{"id": job, "status": "queued", "kind": "backup", "revision": 1, "stage": "awaiting_maintenance_operator", "accepted": true}, nil
		}
		var kind, status string
		var revision int
		var result []byte
		if err := tx.QueryRow(ctx, `SELECT kind,status,revision,result FROM public.business_backup_jobs WHERE id=$1 AND owner_actor_id=$2 FOR UPDATE`, id, owner).Scan(&kind, &status, &revision, &result); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, ErrNotFound
			}
			return nil, err
		}
		expected, ok := parseRevision(body["expected_revision"])
		if !ok || expected != revision {
			return nil, conflict("business_backup", expected, revision)
		}
		switch action {
		case "restore":
			if kind != "backup" || status != "completed" || decodeObject(result)["restorable"] != true {
				return nil, invalid("只有实际完整且已核验的业务备份可以用于隔离恢复")
			}
			job := randomID("restore_")
			if _, err := tx.Exec(ctx, `INSERT INTO public.business_backup_jobs(id,owner_actor_id,kind,source_backup_id,target_name,status,reason) VALUES($1,$2,'restore',$3,$4,'queued',$5)`, job, owner, id, body["target_name"], body["reason"]); err != nil {
				return nil, err
			}
			return map[string]any{"id": job, "status": "queued", "kind": "restore", "revision": 1, "stage": "awaiting_maintenance_operator", "accepted": true}, nil
		case "retry":
			if status != "failed" {
				return nil, invalid("只能明确重试已失败的维护任务；运行中或未知结果必须先核对原执行")
			}
			if kind == "restore" {
				target := stringValue(body["target_name"])
				if !isolatedRecoveryName.MatchString(target) {
					return nil, invalid("失败恢复的重试必须明确指定全新的隔离目标名，旧目标保留以便核对")
				}
				if _, err := tx.Exec(ctx, `UPDATE public.business_backup_jobs SET target_name=$2 WHERE id=$1`, id, target); err != nil {
					return nil, err
				}
			} else if body["target_name"] != nil {
				return nil, ErrInvalidArguments
			}
			if _, err := tx.Exec(ctx, `UPDATE public.business_backup_jobs SET status='queued',revision=revision+1,attempt=attempt+1,execution_digest=NULL,stage='awaiting_maintenance_operator',error_code=NULL,reason=$2,started_at=NULL,finished_at=NULL,updated_at=now() WHERE id=$1`, id, body["reason"]); err != nil {
				return nil, err
			}
		case "cancel":
			if status != "queued" {
				return nil, invalid("只能取消尚未开始的维护任务；已停止服务或写入产物的任务须由原维护工具恢复核对")
			}
			if _, err := tx.Exec(ctx, `UPDATE public.business_backup_jobs SET status='cancelled',revision=revision+1,finished_at=now(),updated_at=now() WHERE id=$1`, id); err != nil {
				return nil, err
			}
		default:
			return nil, ErrInvalidArguments
		}
		return map[string]any{"id": id, "status": map[string]string{"retry": "queued", "cancel": "cancelled"}[action], "revision": revision + 1, "kind": kind}, nil
	})
}
func matchBusinessBackups(method, path string) bool {
	p := surfaceParts(path)
	return len(p) >= 2 && len(p) <= 4 && p[0] == "ops" && p[1] == "backups" && (method == "GET" && (len(p) == 2 || len(p) == 3) || method == "POST" && (len(p) == 2 || len(p) == 4 && containsString([]string{"restore", "retry", "cancel"}, p[3])))
}
func (a *App) dispatchBusinessBackups(ctx context.Context, method, path string, q url.Values, owner string, body map[string]any) (map[string]any, error) {
	p := surfaceParts(path)
	if method == "GET" {
		if len(p) == 2 {
			return a.OwnerBackupJobs(ctx, owner, q)
		}
		return a.OwnerBackupJob(ctx, owner, p[2])
	}
	if len(p) == 2 {
		return a.OwnerBackupCommand(ctx, owner, "", "create", body)
	}
	return a.OwnerBackupCommand(ctx, owner, p[2], p[3], body)
}

// Operator methods are local Go composition seams, not browser endpoints.
// The operator must have deployment/DB access and an Owner-created durable job.
func ClaimBackupOperator(ctx context.Context, pool *pgxpool.Pool, id, token string) (map[string]any, error) {
	var result map[string]any
	err := withTransaction(ctx, pool, func(tx pgx.Tx) error {
		var owner, kind, status string
		var digest, target, source *string
		var attempt int
		if err := tx.QueryRow(ctx, `SELECT owner_actor_id,kind,status,execution_digest,target_name,source_backup_id,attempt FROM public.business_backup_jobs WHERE id=$1 FOR UPDATE`, id).Scan(&owner, &kind, &status, &digest, &target, &source, &attempt); err != nil {
			return err
		}
		if status == "running" {
			if token == "" || digest == nil || *digest != packSHA(token) {
				return invalid("维护任务已有执行者；必须携原私有执行文件恢复，不能重新认领")
			}
		} else if status == "queued" {
			if len(token) < 32 || len(token) > 256 {
				return invalid("须先持久化原维护执行身份，再认领任务")
			}
			if _, err := tx.Exec(ctx, `UPDATE public.business_backup_jobs SET status='running',execution_digest=$2,revision=revision+1,stage='operator_claimed',started_at=now(),updated_at=now() WHERE id=$1`, id, packSHA(token)); err != nil {
				return err
			}
		} else {
			return invalid("维护任务已终结，不能重复执行")
		}
		result = map[string]any{"id": id, "kind": kind, "attempt": attempt, "execution_token": token, "target_name": target, "source_backup_id": source}
		if source != nil {
			var raw []byte
			if err := tx.QueryRow(ctx, `SELECT operator_result FROM public.business_backup_jobs WHERE id=$1 AND owner_actor_id=$2 AND status='completed'`, *source, owner).Scan(&raw); err != nil {
				return err
			}
			result["source"] = decodeObject(raw)
		}
		return nil
	})
	return result, err
}
func RecordBackupOperator(ctx context.Context, pool *pgxpool.Pool, id, token, stage, status string, expectedAttempt int, result, private map[string]any) error {
	if !regexp.MustCompile(`^[a-z0-9_]{1,64}$`).MatchString(stage) || !containsString([]string{"running", "completed", "failed"}, status) {
		return ErrInvalidArguments
	}
	if status == "completed" {
		for _, key := range []string{"artifacts_verified", "database_counts_verified", "media_verified"} {
			if result[key] != true {
				return invalid("备份完成回执缺少实际核验结果")
			}
		}
		if private["artifact_dir"] == nil {
			return invalid("备份完成回执缺少真实产物目录")
		}
	}
	result = compactStateMap(result, []string{"source_inventory_verified", "restore_comparison_verified", "last_restore_job_id", "restorable", "artifacts_verified", "database_counts_verified", "media_verified", "migration_head", "source_head", "media_count", "legacy_media_versions", "manifest_digest", "artifact_count", "models_started", "isolated_target", "upgrade_head", "external_provider_state", "foreign_keys_validated"})
	return withTransaction(ctx, pool, func(tx pgx.Tx) error {
		var owner, kind string
		var actualAttempt int
		if err := tx.QueryRow(ctx, `SELECT owner_actor_id,kind,attempt FROM public.business_backup_jobs WHERE id=$1 AND execution_digest=$2 AND status='running' FOR UPDATE`, id, packSHA(token)).Scan(&owner, &kind, &actualAttempt); err != nil {
			return err
		}
		if actualAttempt != expectedAttempt {
			return invalid("原维护执行已被新的明确尝试取代，不能回写")
		}
		if kind == "restore" && status == "completed" && result["restore_comparison_verified"] != true {
			return invalid("隔离恢复完成回执缺少真实源数据对比核验，不能标记源备份已验证恢复")
		}
		step := map[string]any{"stage": stage, "status": status, "at": time.Now().UTC().Format(time.RFC3339Nano)}
		if _, err := tx.Exec(ctx, `UPDATE public.business_backup_jobs SET stage=$3::text,status=$4::text,revision=revision+1,result=$5,operator_result=$6,steps=steps||jsonb_build_array($7::jsonb),error_code=CASE WHEN $4::text='failed' THEN $3::text ELSE NULL END,finished_at=CASE WHEN $4::text IN ('completed','failed') THEN now() ELSE NULL END,updated_at=now() WHERE id=$1 AND execution_digest=$2`, id, packSHA(token), stage, status, jsonBytes(result), jsonBytes(private), jsonBytes(step)); err != nil {
			return err
		}
		if kind == "restore" && status == "completed" {
			if _, err := tx.Exec(ctx, `UPDATE public.business_backup_jobs parent SET result=parent.result||jsonb_build_object('restore_comparison_verified',true,'last_restore_job_id',$1::text),revision=parent.revision+1,updated_at=now() FROM public.business_backup_jobs child WHERE child.id=$1 AND child.source_backup_id=parent.id AND child.owner_actor_id=parent.owner_actor_id`, id); err != nil {
				return err
			}
		}
		_, err := tx.Exec(ctx, `INSERT INTO public.owner_surface_audits(id,owner_actor_id,fluctlight_id,identity,actor,action,reason,record_id) VALUES($1,$2,'','system_ops','maintenance_operator',$3,$4,$5)`, randomID("audit_backup_"), owner, "business_"+kind+"_"+status, stage, id)
		return err
	})
}

// Verify the archived startup key against actual encrypted settings without
// returning or logging their plaintext, purpose list, nonce or ciphertext.
func VerifyBackupSettingsKey(ctx context.Context, pool *pgxpool.Pool, encoded string) error {
	key, err := decodeSettingsKey(encoded)
	if err != nil {
		return errors.New("backup_settings_key_invalid")
	}
	rows, err := pool.Query(ctx, `SELECT purpose,nonce,ciphertext FROM public.setting_secrets`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var purpose string
		var nonce, ciphertext []byte
		if err := rows.Scan(&purpose, &nonce, &ciphertext); err != nil {
			return err
		}
		if _, err := decryptSecret(key, purpose, nonce, ciphertext); err != nil {
			return errors.New("backup_settings_key_mismatch")
		}
	}
	return rows.Err()
}

// Called only by the operator composition inside a newly restored isolated
// database, after byte/count verification and migration. Runtime/domain intents
// are preserved; only maintenance leases from the source deployment are closed.
func RecordIsolatedBackupRecovery(ctx context.Context, pool *pgxpool.Pool, operation, source, target string, result, private map[string]any) error {
	if !isolatedRecoveryName.MatchString(target) || operation == "" || len(operation) > 128 || source == "" {
		return ErrInvalidArguments
	}
	for _, key := range []string{"artifacts_verified", "database_counts_verified", "media_verified"} {
		if result[key] != true {
			return invalid("隔离恢复尚未核验")
		}
	}
	public := compactStateMap(result, []string{"source_inventory_verified", "restore_comparison_verified", "artifacts_verified", "database_counts_verified", "media_verified", "source_head", "upgrade_head", "media_count", "legacy_media_versions", "isolated_target", "manifest_digest", "models_started"})
	return withTransaction(ctx, pool, func(tx pgx.Tx) error {
		var owner string
		if err := tx.QueryRow(ctx, `SELECT human_actor_id FROM public.owner_accounts LIMIT 1`).Scan(&owner); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE public.business_backup_jobs SET status='failed',stage='source_maintenance_not_resumed',error_code='source_maintenance_not_resumed',revision=revision+1,finished_at=now(),updated_at=now() WHERE status='running' AND id<>$1`, source); err != nil {
			return err
		}
		sourceResult := cloneMap(public)
		sourceResult["restorable"] = true
		command, err := tx.Exec(ctx, `UPDATE public.business_backup_jobs SET status='completed',stage='restored_source_backup_verified',result=$2,operator_result=$3,revision=revision+1,finished_at=now(),updated_at=now() WHERE id=$1 AND kind='backup' AND owner_actor_id=$4`, source, jsonBytes(sourceResult), jsonBytes(map[string]any{"artifact_dir": private["source_artifact_dir"], "manifest_digest": result["manifest_digest"]}), owner)
		if err != nil {
			return err
		}
		if command.RowsAffected() != 1 {
			return invalid("隔离数据库不包含该备份来源任务")
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.business_backup_jobs(id,owner_actor_id,kind,source_backup_id,target_name,status,reason,stage,result,operator_result,started_at,finished_at) VALUES($1,$2,'restore',$3,$4,'completed','isolated recovery verified by local maintenance operator','isolated_restore_verified',$5,$6,now(),now()) ON CONFLICT(id) DO NOTHING`, operation, owner, source, target, jsonBytes(public), jsonBytes(private)); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO public.owner_surface_audits(id,owner_actor_id,fluctlight_id,identity,actor,action,reason,record_id) VALUES($1,$2,'','system_ops','maintenance_operator','isolated_recovery_verified','数据库与媒体校验后升级；源部署的维护租约不在隔离副本中继续，业务和工作流身份保留',$3)`, randomID("audit_recovery_"), owner, operation)
		return err
	})
}
