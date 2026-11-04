package core

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

func visualAssetReviewRoute(method string, p []string) bool {
	return len(p) >= 4 && p[0] == "fluctlights" && p[2] == "visual-assets" && ((method == "GET" && len(p) == 4) || (method == "POST" && len(p) == 5 && p[4] == "review"))
}

func (a *App) ownerVisualAssetReview(ctx context.Context, fid, reviewID string) (map[string]any, error) {
	var raw []byte
	err := a.DB.Pool().QueryRow(ctx, `SELECT jsonb_build_object('id',r.id,'fluctlight_id',r.fluctlight_id,'kind',r.kind,'visual_revision',r.visual_revision,'revision',r.revision,'candidate_asset_id',r.candidate_asset_id,'reference_asset_id',r.canonical_asset_id,'media_intent_id',r.media_intent_id,'status',CASE WHEN r.status='generating' AND i.status='failed' THEN 'failed' ELSE r.status END,'media_status',i.status,'review_source',r.review_source,'review_data',r.review_data - 'candidate_sha256' - 'vision_config_version' - 'vision_model_id','feedback',r.feedback,'created_at',r.created_at,'updated_at',r.updated_at) FROM public.fluctlight_visual_asset_reviews r JOIN public.media_intents i ON i.id=r.media_intent_id WHERE r.id=$1 AND r.fluctlight_id=$2`, reviewID, fid).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return decodeObject(raw), nil
}

// visualAssetOwnerCommand records a review/generation command without inventing
// a user-provided reason for an explicit acceptance.
func (a *App) visualAssetOwnerCommand(ctx context.Context, owner, fid, operation, auditAction string, body map[string]any, apply func(context.Context, pgx.Tx) (map[string]any, error)) (map[string]any, error) {
	key := strings.TrimSpace(stringValue(body["idempotency_key"]))
	if key == "" || len([]rune(key)) > 256 {
		return nil, invalid("idempotency key required")
	}
	digest := stableDigest(operation + ":" + jsonString(body))
	var result map[string]any
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if err := lockOwnerWorldMutationTx(ctx, tx, owner); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, owner+":"+key); err != nil {
			return err
		}
		var savedDigest string
		var raw []byte
		err := tx.QueryRow(ctx, `SELECT digest,response FROM public.owner_surface_idempotency WHERE owner_actor_id=$1 AND idempotency_key=$2`, owner, key).Scan(&savedDigest, &raw)
		if err == nil {
			if savedDigest != digest {
				return &SurfaceError{Status: 409, Code: "idempotency_conflict", Message: "This command key belongs to another operation"}
			}
			result = decodeObject(raw)
			result["_replay"] = true
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if err := requireOwnerMutableTx(ctx, tx, owner, fid); err != nil {
			return err
		}
		result, err = apply(ctx, tx)
		if err != nil {
			return err
		}
		auditID, err := (postgresSurfaceTx{tx}).Audit(ctx, owner, fid, auditAction, stringValue(body["feedback"]), stringValue(result["id"]))
		if err != nil {
			return err
		}
		result["audit_id"] = auditID
		_, err = tx.Exec(ctx, `INSERT INTO public.owner_surface_idempotency(owner_actor_id,idempotency_key,digest,response) VALUES($1,$2,$3,$4)`, owner, key, digest, jsonBytes(result))
		return err
	})
	return result, err
}

func (a *App) generateOwnerAvatar(ctx context.Context, owner, fid string, body map[string]any) (map[string]any, error) {
	if !eventKeys(body, "expected_visual_revision", "idempotency_key") {
		return nil, invalid("unknown avatar generation field")
	}
	expected, ok := parseRevision(body["expected_visual_revision"])
	if !ok || expected < 0 {
		return nil, invalid("visual revision required")
	}
	return a.visualAssetOwnerCommand(ctx, owner, fid, "generated-avatar:"+fid, "generated_avatar_request", body, func(ctx context.Context, tx pgx.Tx) (map[string]any, error) {
		var profileID, canonical, sessionID string
		var revision int
		var snapshotRaw, constraintsRaw []byte
		err := tx.QueryRow(ctx, `SELECT id,current_revision,COALESCE(canonical_asset_id,''),COALESCE(active_session_id,''),identity_snapshot,renderer_constraints FROM public.fluctlight_visual_identities WHERE fluctlight_id=$1 FOR UPDATE`, fid).Scan(&profileID, &revision, &canonical, &sessionID, &snapshotRaw, &constraintsRaw)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && canonical == "") {
			return nil, &SurfaceError{Status: 422, Code: "identity_pending", Message: "请先确认形象参考图，再生成头像"}
		}
		if err != nil {
			return nil, err
		}
		if revision != expected {
			return nil, &SurfaceError{Status: 409, Code: "visual_generation_conflict", Message: "形象参考已变化，请重新读取后生成头像"}
		}
		var existingID, reviewID, status, mediaStatus string
		err = tx.QueryRow(ctx, `SELECT r.media_intent_id,r.id,r.status,i.status FROM public.fluctlight_visual_asset_reviews r JOIN public.media_intents i ON i.id=r.media_intent_id WHERE r.fluctlight_id=$1 AND r.visual_revision=$2 AND r.kind='avatar' ORDER BY r.created_at DESC,r.id DESC LIMIT 1`, fid, revision).Scan(&existingID, &reviewID, &status, &mediaStatus)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		if err == nil && (status == "awaiting_review" || status == "generating" && mediaStatus != "failed") {
			return map[string]any{"id": existingID, "review_id": reviewID, "fluctlight_id": fid, "visual_revision": revision, "status": status}, nil
		}
		source := "owner-avatar:" + stableDigest(owner+":"+fid+":"+stringValue(body["idempotency_key"]))
		intentID, err := a.enqueueGeneratedAvatarTx(ctx, tx, fid, profileID, canonical, sessionID, source, revision, decodeObject(snapshotRaw), decodeObject(constraintsRaw), true, "")
		if err != nil {
			return nil, err
		}
		return map[string]any{"id": intentID, "review_id": "visual_asset_review_" + stableDigest(intentID), "fluctlight_id": fid, "visual_revision": revision, "status": "queued"}, nil
	})
}

func (a *App) reviewOwnerVisualAsset(ctx context.Context, owner, fid, reviewID string, body map[string]any) (map[string]any, error) {
	if !eventKeys(body, "decision", "feedback", "expected_visual_revision", "expected_review_revision", "idempotency_key") {
		return nil, invalid("unknown generated visual review field")
	}
	decision := stringValue(body["decision"])
	feedback := stringValue(body["feedback"])
	visualRevision, visualOK := parseRevision(body["expected_visual_revision"])
	reviewRevision, reviewOK := parseRevision(body["expected_review_revision"])
	if !visualOK || visualRevision < 0 || !reviewOK || reviewRevision < 0 || (decision != "accepted" && decision != "regenerate") || len([]rune(feedback)) > 2000 || (decision == "regenerate" && strings.TrimSpace(feedback) == "") {
		return nil, invalid("generated visual review decision, versions and correction feedback required")
	}
	if value, present := body["feedback"]; present {
		if _, ok := value.(string); !ok {
			return nil, invalid("generated visual review feedback must be text")
		}
	}
	return a.visualAssetOwnerCommand(ctx, owner, fid, "visual-asset-review:"+reviewID, "visual_asset_manual_review", body, func(ctx context.Context, tx pgx.Tx) (map[string]any, error) {
		var profileID, canonical, sessionID, kind, intentID, assetID, status string
		var storedVisualRevision, storedReviewRevision int
		err := tx.QueryRow(ctx, `SELECT visual_identity_id,visual_revision,canonical_asset_id,COALESCE(session_id,''),kind,media_intent_id,COALESCE(candidate_asset_id,''),status,revision FROM public.fluctlight_visual_asset_reviews WHERE id=$1 AND fluctlight_id=$2 FOR UPDATE`, reviewID, fid).Scan(&profileID, &storedVisualRevision, &canonical, &sessionID, &kind, &intentID, &assetID, &status, &storedReviewRevision)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		if err != nil {
			return nil, err
		}
		var currentRevision int
		var currentCanonical, currentAvatarIntent string
		var snapshotRaw, constraintsRaw []byte
		if err := tx.QueryRow(ctx, `SELECT current_revision,COALESCE(canonical_asset_id,''),COALESCE(generated_avatar_media_intent_id,''),identity_snapshot,renderer_constraints FROM public.fluctlight_visual_identities WHERE id=$1 AND fluctlight_id=$2 FOR UPDATE`, profileID, fid).Scan(&currentRevision, &currentCanonical, &currentAvatarIntent, &snapshotRaw, &constraintsRaw); err != nil {
			return nil, err
		}
		if storedVisualRevision != visualRevision || currentRevision != visualRevision || currentCanonical != canonical || storedReviewRevision != reviewRevision || status != "awaiting_review" || (kind == "avatar" && currentAvatarIntent != intentID) {
			return nil, &SurfaceError{Status: 409, Code: "visual_review_conflict", Message: "图片或形象参考已变化，请重新读取后审核"}
		}
		if kind == "character_sheet" {
			var currentSheetIntent string
			if err := tx.QueryRow(ctx, `SELECT COALESCE(character_sheet_media_intent_id,'') FROM public.fluctlight_visual_identity_sessions WHERE id=$1 AND fluctlight_id=$2 FOR UPDATE`, sessionID, fid).Scan(&currentSheetIntent); err != nil {
				return nil, err
			}
			if currentSheetIntent != intentID {
				return nil, &SurfaceError{Status: 409, Code: "visual_review_conflict", Message: "角色设定图已变化，请重新读取后审核"}
			}
		}
		var assetOwner, assetStatus, mediaStatus string
		if err := tx.QueryRow(ctx, `SELECT a.owner_fluctlight_id,a.status,i.status FROM public.media_assets a JOIN public.media_intents i ON i.id=$2 WHERE a.id=$1`, assetID, intentID).Scan(&assetOwner, &assetStatus, &mediaStatus); err != nil {
			return nil, err
		}
		if assetID == "" || assetOwner != fid || assetStatus != "ready" || mediaStatus != "completed" {
			return nil, &SurfaceError{Status: 422, Code: "visual_candidate_unavailable", Message: "图片尚未准备好，请重新读取"}
		}
		newStatus := "accepted"
		if decision == "regenerate" {
			newStatus = "rejected"
		}
		if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_visual_asset_reviews SET status=$2,review_source='manual',feedback=$3,revision=revision+1,updated_at=now() WHERE id=$1 AND status='awaiting_review'`, reviewID, newStatus, feedback); err != nil {
			return nil, err
		}
		result := map[string]any{"id": reviewID, "fluctlight_id": fid, "kind": kind, "decision": decision, "candidate_asset_id": assetID, "visual_revision": visualRevision, "status": newStatus}
		if decision == "accepted" && kind == "avatar" {
			if err := applyGeneratedAvatarTx(ctx, tx, profileID, visualRevision, canonical, intentID, assetID); err != nil {
				return nil, err
			}
		}
		if decision == "regenerate" {
			source := "derived-review:" + stableDigest(owner+":"+reviewID+":"+stringValue(body["idempotency_key"]))
			var next string
			if kind == "avatar" {
				next, err = a.enqueueGeneratedAvatarTx(ctx, tx, fid, profileID, canonical, sessionID, source, visualRevision, decodeObject(snapshotRaw), decodeObject(constraintsRaw), true, feedback)
			} else {
				next, err = a.enqueueCharacterSheetRetryTx(ctx, tx, fid, profileID, canonical, sessionID, source, visualRevision, decodeObject(snapshotRaw), decodeObject(constraintsRaw), feedback)
			}
			if err != nil {
				return nil, err
			}
			result["next_intent_id"] = next
		}
		if kind == "character_sheet" {
			if err := enqueueVisualAssetReviewResumeTx(ctx, tx, fid, sessionID, reviewID, stringValue(body["idempotency_key"])); err != nil {
				return nil, err
			}
		}
		return result, nil
	})
}

func enqueueVisualAssetReviewResumeTx(ctx context.Context, tx pgx.Tx, fid, sessionID, reviewID, key string) error {
	intentID := "visual_asset_review_resume:" + stableDigest(reviewID+":"+key)
	workflowID := "visual_asset_review_workflow_" + stableDigest(intentID)
	_, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload) VALUES($1,$2,'lifecycle','visual_identity.initialize',$3) ON CONFLICT(intent_id) DO NOTHING`, intentID, workflowID, jsonBytes(map[string]any{"intent_id": intentID, "session_id": sessionID, "fluctlight_id": fid, "correlation_id": intentID, "causation_id": reviewID}))
	return err
}
