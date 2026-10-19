package core

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
)

// A media retry is a domain attempt, never a generic Temporal restart. Reuse
// persisted provider work by default; generating again needs an explicit mode.
func (a *App) RetryOwnerDiagnosticMedia(ctx context.Context, owner, id string, body map[string]any) (map[string]any, error) {
	if err := a.requireOwner(ctx, owner); err != nil {
		return nil, err
	}
	if !eventKeys(body, "expected_revision", "mode", "idempotency_key", "reason") {
		return nil, ErrInvalidArguments
	}
	expected, ok := positiveInt64(body["expected_revision"])
	mode := stringValue(body["mode"])
	if !ok || !containsString([]string{"resume", "regenerate"}, mode) {
		return nil, invalid("媒体版本和明确的恢复方式是必需的")
	}
	if replay, found, err := a.ownerWorldCommandReplay(ctx, owner, "media-recovery:"+id, body); err != nil || found {
		return replay, err
	}
	release, acquired, err := a.tryDailyReviewExecutionLock(ctx, "media-intent", id)
	if err != nil {
		return nil, err
	}
	if !acquired {
		return nil, &SurfaceError{Status: 409, Code: "media_execution_active", Message: "原媒体处理仍在执行，请先查询实际结果"}
	}
	defer release()
	ctx = context.WithValue(ctx, ownerCommandAuditActionKey{}, "media_recovery")
	return a.worldCommand(ctx, owner, "", "media-recovery:"+id, body, func(ctx context.Context, tx pgx.Tx) (map[string]any, error) {
		var raw []byte
		if err := tx.QueryRow(ctx, `SELECT to_jsonb(m) FROM public.media_intents m WHERE id=$1 FOR UPDATE`, id).Scan(&raw); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, ErrNotFound
			}
			return nil, err
		}
		previous := decodeObject(raw)
		if int64(intValue(previous["revision"])) != expected {
			return nil, &SurfaceError{Status: 409, Code: "revision_conflict", Message: "媒体任务状态已变化，请核对实际结果"}
		}
		if stringValue(previous["studio_job_id"]) != "" || decodeObject([]byte(stringValue(previous["prompt"])))["purpose"] == "visual_identity" {
			return nil, &SurfaceError{Status: 422, Code: "media_domain_recovery_required", Message: "创作或视觉身份媒体必须从原任务或视觉会话恢复"}
		}
		if fid := stringValue(previous["owner_fluctlight_id"]); fid != "" {
			if err := requireOwnerMutableTx(ctx, tx, owner, fid); err != nil {
				return nil, err
			}
		} else {
			return nil, ErrUnauthorized
		}
		if previous["status"] != "failed" {
			return nil, &SurfaceError{Status: 409, Code: "media_not_retryable", Message: "只有真实失败的媒体任务可以恢复"}
		}
		var active bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.platform_workflow_intents WHERE intent_type='media.generation' AND payload->>'intent_id'=$1 AND status IN ('pending','retry','started','paused','cancel_requested'))`, id).Scan(&active); err != nil {
			return nil, err
		}
		if active {
			return nil, &SurfaceError{Status: 409, Code: "media_execution_active", Message: "原媒体工作流尚未结束，请先查询实际执行"}
		}
		var assetReady bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.media_assets WHERE id=$1 AND status='ready')`, "asset_"+id).Scan(&assetReady); err != nil {
			return nil, err
		}
		if mode == "regenerate" && assetReady {
			return nil, &SurfaceError{Status: 409, Code: "media_result_already_ready", Message: "真实资产已就绪，请恢复原结果发布；不会覆盖已生成的资产"}
		}
		attempt := randomID("media_recovery_")
		wf := "media-recovery:" + attempt
		intent := "media_recovery_intent:" + attempt
		if _, err := tx.Exec(ctx, `INSERT INTO public.media_recovery_attempts(id,owner_actor_id,media_intent_id,source_workflow_id,workflow_id,intent_id,mode,reason,previous) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, attempt, owner, id, previous["workflow_id"], wf, intent, mode, body["reason"], raw); err != nil {
			return nil, err
		}
		// Resume retains prompt, provider job, quality verdict, retries and candidate.
		// Explicit regeneration records them above before clearing only those stages.
		if _, err := tx.Exec(ctx, `UPDATE public.media_intents SET workflow_id=$2,status=CASE WHEN $3='resume' AND provider_job_id IS NOT NULL THEN 'running' ELSE 'pending' END,revision=revision+1,provider_job_id=CASE WHEN $3='regenerate' THEN NULL ELSE provider_job_id END,quality_verdict=CASE WHEN $3='regenerate' THEN '' ELSE quality_verdict END,quality_candidate_sha256=CASE WHEN $3='regenerate' THEN NULL ELSE quality_candidate_sha256 END,quality_checked_at=CASE WHEN $3='regenerate' THEN NULL ELSE quality_checked_at END WHERE id=$1`, id, wf, mode); err != nil {
			return nil, err
		}
		payload := map[string]any{"intent_id": id, "fluctlight_id": previous["owner_fluctlight_id"], "provider_request_id": previous["provider_request_id"], "correlation_id": "media:" + id, "media_recovery_id": attempt}
		if _, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload) VALUES($1,$2,'media','media.generation',$3)`, intent, wf, jsonBytes(payload)); err != nil {
			return nil, err
		}
		return map[string]any{"id": attempt, "media_intent_id": id, "workflow_id": wf, "intent_id": intent, "status": "retry_queued", "mode": mode, "previous_result_reused": assetReady}, nil
	})
}
