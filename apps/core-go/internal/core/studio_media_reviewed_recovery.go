package core

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// Resume only the original, already persisted Studio image after explicit
// Owner review. Unknown external results and missing assets stay paused.
func (a *App) resumeReviewedStudioMedia(ctx context.Context, owner, id string, body map[string]any) (map[string]any, error) {
	if err := a.requireOwner(ctx, owner); err != nil {
		return nil, err
	}
	if !eventKeys(body, "expected_revision", "expected_attempt", "expected_media_revision", "submission_reviewed", "reason", "idempotency_key") {
		return nil, ErrInvalidArguments
	}
	expected, jobRevisionOK := parseRevision(body["expected_revision"])
	expectedMedia, mediaRevisionOK := parseRevision(body["expected_media_revision"])
	expectedAttempt, attemptOK := positiveInt64(body["expected_attempt"])
	if !jobRevisionOK || !mediaRevisionOK || !attemptOK {
		return nil, ErrInvalidArguments
	}
	if body["submission_reviewed"] != true {
		return nil, &SurfaceError{Status: 409, Code: "media_submission_review_required", Message: "请先人工核对原生成任务及现有图片。"}
	}
	scope := "studio-media-reviewed:" + id
	if replay, found, err := a.ownerWorldCommandReplay(ctx, owner, scope, body); err != nil || found {
		return replay, err
	}
	mid := "studio_media_" + stableDigest(id)
	release, acquired, err := a.tryDailyReviewExecutionLock(ctx, "media-intent", mid)
	if err != nil {
		return nil, err
	}
	if !acquired {
		return nil, &SurfaceError{Status: 409, Code: "studio_media_execution_active", Message: "原图片处理仍在执行，请重新读取原任务。"}
	}
	defer release()
	ctx = context.WithValue(ctx, ownerCommandAuditActionKey{}, "studio_media_submission_review")
	return a.worldCommand(ctx, owner, "", scope, body, func(ctx context.Context, tx pgx.Tx) (map[string]any, error) {
		job, err := readStudioJobWith(ctx, tx, owner, id, true)
		if err != nil {
			return nil, err
		}
		if expected != intValue(job["revision"]) {
			return nil, conflict("studio_job", expected, intValue(job["revision"]))
		}
		if job["kind"] != "image_preview" || !containsString([]string{"running", "failed"}, stringValue(job["status"])) || int(expectedAttempt) != intValue(job["latest_attempt"]) {
			return nil, &SurfaceError{Status: 409, Code: "studio_media_review_stale", Message: "原任务或当前尝试已变化，请重新读取。"}
		}
		var mediaRaw []byte
		if err := tx.QueryRow(ctx, `SELECT to_jsonb(i) FROM public.media_intents i WHERE id=$1 AND media_scope='studio' AND studio_job_id=$2 AND studio_owner_actor_id=$3 FOR UPDATE`, mid, id, owner).Scan(&mediaRaw); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, ErrNotFound
			}
			return nil, err
		}
		media := decodeObject(mediaRaw)
		if expectedMedia != intValue(media["revision"]) {
			return nil, conflict("studio_media", expectedMedia, intValue(media["revision"]))
		}
		ambiguous := media["status"] == mediaSubmissionUnknownStatus || (media["status"] == "failed" && media["renderer_submission_started"] == true && stringValue(media["provider_job_id"]) == "")
		if !ambiguous {
			return nil, &SurfaceError{Status: 409, Code: "studio_media_review_stale", Message: "原图片已离开待人工核对状态。"}
		}
		var attemptID string
		var outputRaw []byte
		if err := tx.QueryRow(ctx, `SELECT id,output FROM public.studio_job_attempts WHERE job_id=$1 AND attempt=$2 AND status IN ('running','failed') FOR UPDATE`, id, expectedAttempt).Scan(&attemptID, &outputRaw); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, ErrConflict
			}
			return nil, err
		}
		output := decodeObject(outputRaw)
		if output["media_intent_id"] != mid || output["workflow_id"] != media["workflow_id"] {
			return nil, &SurfaceError{Status: 409, Code: "studio_media_review_stale", Message: "无法确认图片属于当前原始尝试，请保留暂停状态。"}
		}
		var active bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.platform_workflow_intents WHERE intent_type='media.generation' AND payload->>'intent_id'=$1 AND status IN ('pending','retry','started','paused','cancel_requested'))`, mid).Scan(&active); err != nil {
			return nil, err
		}
		if active {
			return nil, &SurfaceError{Status: 409, Code: "studio_media_execution_active", Message: "原图片工作流尚未结束，请先核对原运行。"}
		}
		verdict := stringValue(media["quality_verdict"])
		if verdict == mediaQualityVerdictRetry || verdict == mediaQualityVerdictRetryAccepted || verdict == mediaQualityVerdictReject {
			return nil, &SurfaceError{Status: 409, Code: mediaQualityFailureCode(verdict), Message: "原图片质量检查未通过，请保留原结论和图片供核对。"}
		}
		var asset string
		if err := tx.QueryRow(ctx, `SELECT id FROM public.media_assets WHERE id=$1 AND status='ready' AND media_scope='studio' AND studio_job_id=$2 AND studio_owner_actor_id=$3 AND provider_request_id=$4 AND workflow_id=$5 FOR SHARE`, "asset_"+mid, id, owner, media["provider_request_id"], media["workflow_id"]).Scan(&asset); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, &SurfaceError{Status: 409, Code: "media_submission_review_required", Message: "未找到可核对的原任务同源就绪图片，请保持暂停。"}
			}
			return nil, err
		}
		recovery := randomID("studio_media_review_")
		workflow := "studio-media-review:" + recovery
		workflowIntent := "media_review_intent:" + recovery
		review := map[string]any{"owner_actor_id": owner, "previous_workflow_id": media["workflow_id"], "previous_media_status": media["status"], "media_revision": expectedMedia, "asset_id": asset, "reason": body["reason"]}
		output["workflow_id"] = workflow
		if _, err := tx.Exec(ctx, `UPDATE public.media_intents SET status='reviewed_ready',workflow_id=$2,revision=revision+1 WHERE id=$1`, mid, workflow); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `UPDATE public.studio_job_attempts SET status='running',output=$2,error_code=NULL,finished_at=NULL,provenance=provenance||jsonb_build_object('submission_review',$3::jsonb) WHERE id=$1`, attemptID, jsonBytes(output), jsonBytes(review)); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `UPDATE public.studio_jobs SET status='running',result=$2,error_code=NULL,revision=revision+1,updated_at=now() WHERE id=$1`, id, jsonBytes(output)); err != nil {
			return nil, err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload) VALUES($1,$2,'media','media.generation',$3)`, workflowIntent, workflow, jsonBytes(map[string]any{"intent_id": mid, "studio_attempt_id": attemptID, "owner_actor_id": owner, "submission_reviewed": true})); err != nil {
			return nil, err
		}
		return readStudioJobWith(ctx, tx, owner, id, false)
	})
}
