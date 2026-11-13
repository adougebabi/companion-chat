package core

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func (a *App) resumeReviewedVisualSubmission(ctx context.Context, owner, fid, sessionID string, body map[string]any) (map[string]any, error) {
	if err := a.requireOwner(ctx, owner); err != nil {
		return nil, err
	}
	if _, err := a.DB.GetFluctlight(ctx, fid, owner); err != nil {
		return nil, err
	}
	if !eventKeys(body, "stage", "expected_attempt", "expected_visual_revision", "expected_media_revision", "workflow_id", "source_run_id", "expected_workflow_revision", "submission_reviewed", "reason", "idempotency_key") {
		return nil, ErrInvalidArguments
	}
	stage := stringValue(body["stage"])
	attempt, attemptOK := parseRevision(body["expected_attempt"])
	visualRevision, visualOK := parseRevision(body["expected_visual_revision"])
	mediaRevision, mediaOK := parseRevision(body["expected_media_revision"])
	workflowRevision, workflowOK := positiveInt64(body["expected_workflow_revision"])
	workflow := strings.TrimSpace(stringValue(body["workflow_id"]))
	run := strings.TrimSpace(stringValue(body["source_run_id"]))
	if !containsString([]string{"candidate", "character_sheet"}, stage) || !attemptOK || attempt < 1 || !visualOK || !mediaOK || !workflowOK || workflow == "" || len(workflow) > 256 || run == "" || len(run) > 256 {
		return nil, ErrInvalidArguments
	}
	if body["submission_reviewed"] != true {
		return nil, &SurfaceError{Status: 409, Code: "media_submission_review_required", Message: "请先人工核对原生成任务和已保存图片。"}
	}
	scope := "visual-submission-review:" + fid + ":" + sessionID + ":" + stage
	if saved, found, err := a.ownerWorldCommandReplay(ctx, owner, scope, body); err != nil || found {
		if err != nil {
			return nil, err
		}
		return a.finishReviewedVisualSubmission(ctx, owner, saved, body)
	}
	original, err := a.ownerWorkflowIntent(ctx, owner, workflow)
	if err != nil {
		return nil, err
	}
	input := decodeObject(original.Payload)
	if original.Kind != "visual_identity.initialize" || input["session_id"] != sessionID || input["fluctlight_id"] != fid || original.Revision != workflowRevision || !containsString([]string{"started", "paused"}, original.Status) {
		return nil, &SurfaceError{Status: 409, Code: "visual_submission_recovery_stale", Message: "原视觉工作流或版本已变化，请重新读取。"}
	}
	if a.Workflows == nil {
		return nil, ErrWorkflowRuntime
	}
	runtimeCtx, cancel := context.WithTimeout(ctx, workflowListTimeout)
	execution, err := a.Workflows.Status(runtimeCtx, original.Workflow, "")
	cancel()
	if err != nil {
		return nil, ErrWorkflowRuntime
	}
	if execution.RunID != run || !containsString(workflowControlActions(original.Kind, execution), "resume") {
		return nil, &SurfaceError{Status: 409, Code: "visual_submission_recovery_stale", Message: "只能接回当前仍暂停的原Run，请核对实际运行。"}
	}
	var mediaID string
	err = a.DB.Pool().QueryRow(ctx, `SELECT CASE WHEN $3='candidate' THEN COALESCE(t.media_intent_id,'') ELSE COALESCE(s.character_sheet_media_intent_id,'') END FROM public.fluctlight_visual_identity_sessions s JOIN public.fluctlight_visual_identity_attempts t ON t.session_id=s.id AND t.attempt_number=s.current_attempt WHERE s.id=$1 AND s.fluctlight_id=$2`, sessionID, fid, stage).Scan(&mediaID)
	if err != nil {
		return nil, err
	}
	if mediaID == "" {
		return nil, ErrNotFound
	}
	release, acquired, err := a.tryDailyReviewExecutionLock(ctx, "media-intent", mediaID)
	if err != nil {
		return nil, err
	}
	if !acquired {
		return nil, &SurfaceError{Status: 409, Code: "media_execution_active", Message: "原图片仍在处理，请重新读取。"}
	}
	defer release()
	ctx = context.WithValue(ctx, ownerCommandAuditActionKey{}, "visual_submission_review")
	reservation, err := a.worldCommand(ctx, owner, fid, scope, body, func(ctx context.Context, tx pgx.Tx) (map[string]any, error) {
		session, err := loadVisualIdentityToolSessionTx(ctx, tx, sessionID)
		if err != nil {
			return nil, err
		}
		var currentRevision int
		var activeSession string
		if err := tx.QueryRow(ctx, `SELECT current_revision,COALESCE(active_session_id,'') FROM public.fluctlight_visual_identities WHERE id=$1 AND fluctlight_id=$2 FOR UPDATE`, session.ProfileID, fid).Scan(&currentRevision, &activeSession); err != nil {
			return nil, err
		}
		var sessionWorkflow string
		if err := tx.QueryRow(ctx, `SELECT workflow_id FROM public.fluctlight_visual_identity_sessions WHERE id=$1`, sessionID).Scan(&sessionWorkflow); err != nil {
			return nil, err
		}
		if session.FluctlightID != fid || session.Attempt != attempt || currentRevision != visualRevision || activeSession != sessionID || sessionWorkflow != original.Workflow || !containsString([]string{"queued", "running", "character_sheet_pending"}, session.Status) {
			return nil, &SurfaceError{Status: 409, Code: "visual_submission_recovery_stale", Message: "当前视觉会话、尝试或规范图版本已变化。"}
		}
		currentMedia := session.MediaIntentID
		expectedStage := "seed"
		if stage == "character_sheet" {
			currentMedia = session.CharacterMediaIntentID
			expectedStage = "character_sheet"
		}
		if currentMedia != mediaID || stage == "candidate" && session.Decision != "" || stage == "character_sheet" && session.Status != "character_sheet_pending" {
			return nil, &SurfaceError{Status: 409, Code: "visual_submission_recovery_stale", Message: "原图片已不属于当前待核对阶段。"}
		}
		var storedWorkflowRevision int64
		var workflowStatus string
		if err := tx.QueryRow(ctx, `SELECT intent_revision,status FROM public.platform_workflow_intents WHERE intent_id=$1 AND workflow_id=$2 FOR SHARE`, original.ID, original.Workflow).Scan(&storedWorkflowRevision, &workflowStatus); err != nil {
			return nil, err
		}
		if storedWorkflowRevision != workflowRevision || !containsString([]string{"started", "paused"}, workflowStatus) {
			return nil, &SurfaceError{Status: 409, Code: "visual_submission_recovery_stale", Message: "原视觉工作流已变化。"}
		}
		var raw []byte
		if err := tx.QueryRow(ctx, `SELECT to_jsonb(i) FROM public.media_intents i WHERE id=$1 AND owner_fluctlight_id=$2 AND media_scope='fluctlight' AND studio_job_id IS NULL AND message_id IS NULL AND conversation_id IS NULL FOR UPDATE`, mediaID, fid).Scan(&raw); err != nil {
			return nil, err
		}
		media := decodeObject(raw)
		concept := decodeObject([]byte(stringValue(media["prompt"])))
		ambiguous := media["status"] == mediaSubmissionUnknownStatus || (media["status"] == "failed" && media["renderer_submission_started"] == true && stringValue(media["provider_job_id"]) == "")
		if intValue(media["revision"]) != mediaRevision || !ambiguous || concept["purpose"] != "visual_identity" || concept["stage"] != expectedStage {
			return nil, &SurfaceError{Status: 409, Code: "visual_submission_recovery_stale", Message: "原媒体版本或来源已变化，请保持暂停。"}
		}
		var active bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.platform_workflow_intents WHERE intent_type='media.generation' AND payload->>'intent_id'=$1 AND status IN ('pending','retry','started','paused','cancel_requested'))`, mediaID).Scan(&active); err != nil {
			return nil, err
		}
		if active {
			return nil, &SurfaceError{Status: 409, Code: "media_execution_active", Message: "原媒体工作流尚未结束，请先核对执行。"}
		}
		verdict := stringValue(media["quality_verdict"])
		if containsString([]string{mediaQualityVerdictReject, mediaQualityVerdictRetry, mediaQualityVerdictRetryAccepted}, verdict) {
			return nil, &SurfaceError{Status: 409, Code: "visual_submission_quality_required", Message: "原质量结论未通过，人工归属核对不能覆盖质量审核。"}
		}
		asset := "asset_" + mediaID
		var verified string
		if err := tx.QueryRow(ctx, `SELECT id FROM public.media_assets WHERE id=$1 AND owner_fluctlight_id=$2 AND media_scope='fluctlight' AND status='ready' AND provider_request_id=$3 AND workflow_id=$4 AND sha256<>'' FOR SHARE`, asset, fid, media["provider_request_id"], media["workflow_id"]).Scan(&verified); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return nil, &SurfaceError{Status: 409, Code: "media_submission_review_required", Message: "未找到可核对的原任务同源就绪图片，请保持暂停。"}
			}
			return nil, err
		}
		if _, err := tx.Exec(ctx, `UPDATE public.media_intents SET status='reviewed_ready',revision=revision+1 WHERE id=$1`, mediaID); err != nil {
			return nil, err
		}
		if err := a.markMediaIntentCompletedTx(ctx, tx, mediaID, asset); err != nil {
			return nil, err
		}
		return map[string]any{"id": mediaID, "fluctlight_id": fid, "session_id": sessionID, "stage": stage, "media_intent_id": mediaID, "asset_id": asset, "workflow_id": original.Workflow, "source_run_id": run, "media_status": "completed", "status": "result_ready"}, nil
	})
	if err != nil {
		return nil, err
	}
	return a.finishReviewedVisualSubmission(ctx, owner, reservation, body)
}

// A committed local result and a runtime resume acknowledgement are separate.
// Retrying reuses the existing immutable control command and original Run.
func (a *App) finishReviewedVisualSubmission(ctx context.Context, owner string, reservation, body map[string]any) (map[string]any, error) {
	controlKey := "visual-submission-resume:" + stableDigest(owner+":"+stringValue(body["idempotency_key"]))
	var alreadySent bool
	if err := a.DB.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.workflow_control_commands WHERE owner_actor_id=$1 AND idempotency_key=$2 AND status='sent')`, owner, controlKey).Scan(&alreadySent); err != nil {
		return nil, err
	}
	if !alreadySent {
		var current bool
		if err := a.DB.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.fluctlight_visual_identity_sessions s JOIN public.fluctlight_visual_identities v ON v.id=s.visual_identity_id WHERE s.id=$1 AND s.fluctlight_id=$2 AND v.active_session_id=s.id AND s.current_attempt=$3 AND v.current_revision=$4 AND s.status IN ('queued','running','character_sheet_pending'))`, reservation["session_id"], reservation["fluctlight_id"], body["expected_attempt"], body["expected_visual_revision"]).Scan(&current); err != nil {
			return nil, err
		}
		if !current {
			return nil, &SurfaceError{Status: 503, Code: "visual_submission_resume_unconfirmed", Message: "原图片结果已核对，当前会话已变化。请保留原提交并核对原运行的恢复结果。"}
		}
	}
	receipt, err := a.OwnerWorkflowCommand(ctx, owner, stringValue(reservation["workflow_id"]), "resume", map[string]any{"source_run_id": reservation["source_run_id"], "expected_revision": body["expected_workflow_revision"], "reason": body["reason"], "idempotency_key": controlKey})
	if err != nil {
		return nil, &SurfaceError{Status: 503, Code: "visual_submission_resume_unconfirmed", Message: "原图片结果已核对并结算，原工作流恢复尚未确认。请保留原提交身份继续核对。", Details: map[string]any{"media_intent_id": reservation["media_intent_id"], "workflow_id": reservation["workflow_id"], "source_run_id": reservation["source_run_id"]}}
	}
	result := cloneMap(reservation)
	result["status"] = "resume_requested"
	result["command_id"] = receipt["command_id"]
	return result, nil
}

func (a *App) readVisualSubmissionRecovery(ctx context.Context, owner, fid, sessionID string) (map[string]any, error) {
	result := map[string]any{"status": "unavailable"}
	var workflow string
	if err := a.DB.Pool().QueryRow(ctx, `SELECT workflow_id FROM public.fluctlight_visual_identity_sessions WHERE id=$1 AND fluctlight_id=$2`, sessionID, fid).Scan(&workflow); err != nil {
		return nil, err
	}
	rows, err := readWorldObjects(ctx, a.DB.Pool(), `SELECT jsonb_build_object('stage',source.stage,'source_prompt',m.prompt,'expected_stage',source.media_stage,'media_intent_id',m.id,'revision',m.revision,'ready',EXISTS(SELECT 1 FROM public.media_assets a WHERE a.id='asset_'||m.id AND a.owner_fluctlight_id=m.owner_fluctlight_id AND a.media_scope='fluctlight' AND a.status='ready' AND a.provider_request_id=m.provider_request_id AND a.workflow_id=m.workflow_id AND a.sha256<>'') AND COALESCE(m.quality_verdict,'') NOT IN ('retry','retry_accepted','reject')) FROM public.fluctlight_visual_identity_sessions s JOIN public.fluctlight_visual_identity_attempts t ON t.session_id=s.id AND t.attempt_number=s.current_attempt CROSS JOIN LATERAL (VALUES ('candidate',t.media_intent_id,'seed'),('character_sheet',s.character_sheet_media_intent_id,'character_sheet')) source(stage,media_id,media_stage) JOIN public.media_intents m ON m.id=source.media_id WHERE s.id=$1 AND s.fluctlight_id=$2 AND m.owner_fluctlight_id=$2 AND m.media_scope='fluctlight' AND m.studio_job_id IS NULL AND m.message_id IS NULL AND m.conversation_id IS NULL AND (m.status='submission_unknown' OR (m.status='failed' AND m.renderer_submission_started AND COALESCE(m.provider_job_id,'')=''))`, sessionID, fid)
	if err != nil {
		return nil, err
	}
	stages := make([]any, 0, len(rows))
	for _, raw := range rows {
		row := mapValue(raw)
		concept := decodeObject([]byte(stringValue(row["source_prompt"])))
		matches := concept["purpose"] == "visual_identity" && concept["stage"] == row["expected_stage"]
		delete(row, "source_prompt")
		delete(row, "expected_stage")
		if matches {
			stages = append(stages, row)
		}
	}
	result["stages"] = stages
	original, err := a.ownerWorkflowIntent(ctx, owner, workflow)
	if err != nil || original.Kind != "visual_identity.initialize" || !containsString([]string{"started", "paused"}, original.Status) {
		result["status"] = "original_workflow_unavailable"
		return result, nil
	}
	if a.Workflows == nil {
		return result, nil
	}
	runtimeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	execution, err := a.Workflows.Status(runtimeCtx, original.Workflow, "")
	if err != nil {
		return result, nil
	}
	if !containsString(workflowControlActions(original.Kind, execution), "resume") {
		result["status"] = "original_workflow_unavailable"
		return result, nil
	}
	result["status"] = "available"
	result["workflow_id"] = original.Workflow
	result["source_run_id"] = execution.RunID
	result["workflow_revision"] = original.Revision
	return result, nil
}
