package core

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

// enqueueGeneratedAvatarTx creates a distinct media intent bound to the exact
// accepted reference and visual revision. The caller owns the surrounding
// transaction and chooses whether an existing approved avatar remains visible.
func (a *App) enqueueGeneratedAvatarTx(ctx context.Context, tx pgx.Tx, fluctlightID, profileID, canonicalAssetID, sessionID, sourceKey string, revision int, snapshot, constraints map[string]any, keepPrevious bool, feedback string) (string, error) {
	intentID := "media_intent_" + stableDigest(sourceKey+":avatar")
	workflowID := "media_workflow_" + stableDigest(intentID)
	requestID := "media_request_" + stableDigest(intentID)
	character, err := readAvatarCharacterProfile(ctx, tx, fluctlightID)
	if err != nil {
		return "", err
	}
	concept := map[string]any{
		"purpose": "visual_identity", "stage": "avatar", "render_intent": "display_avatar", "subject_count": 1,
		"canonical_asset_id": canonicalAssetID, "visual_identity_reference_asset_id": canonicalAssetID,
		"visual_identity": map[string]any{"identity_snapshot": snapshot}, "renderer_constraints": constraints,
		"avatar_character": character,
	}
	if feedback != "" {
		concept["derived_review_feedback"] = feedback
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.media_intents(id,owner_fluctlight_id,kind,mime_type,prompt,provider_request_id,workflow_id,status,revision) VALUES($1,$2,'image','image/png',$3,$4,$5,'pending',0)`, intentID, fluctlightID, jsonString(concept), requestID, workflowID); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload) VALUES($1,$2,'media','media.generation',$3)`, "media_workflow_intent:"+intentID, workflowID, jsonBytes(map[string]any{"intent_id": intentID, "provider_request_id": requestID, "fluctlight_id": fluctlightID})); err != nil {
		return "", err
	}
	if err := createVisualAssetReviewTx(ctx, tx, fluctlightID, profileID, canonicalAssetID, sessionID, "avatar", intentID, revision); err != nil {
		return "", err
	}
	command, err := tx.Exec(ctx, `UPDATE public.fluctlight_visual_identities SET generated_avatar_media_intent_id=$2,generated_avatar_asset_id=CASE WHEN $5 THEN generated_avatar_asset_id ELSE NULL END,updated_at=now() WHERE id=$1 AND current_revision=$3 AND canonical_asset_id=$4`, profileID, intentID, revision, canonicalAssetID, keepPrevious)
	if err != nil {
		return "", err
	}
	if command.RowsAffected() != 1 {
		return "", errors.New("generated_avatar_reference_changed")
	}
	return intentID, nil
}

// Freeze the character's own relevant setting when the image intent is queued.
// Values inside the selected business fields remain intact; identifiers and
// unrelated runtime or scheduling records are not part of this task.
type avatarCharacterReader interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func avatarNeedsCharacterSnapshot(intent mediaIntent, concept map[string]any) bool {
	if stringValue(concept["purpose"]) != "visual_identity" || stringValue(concept["stage"]) != "avatar" || intent.ProviderJobID != "" {
		return false
	}
	if strings.TrimSpace(intent.ProviderPrompt) != "" && intent.QualityVerdict != mediaQualityVerdictRetry {
		return false
	}
	character, exists := concept["avatar_character"]
	return !exists || character == nil
}

func readAvatarCharacterProfile(ctx context.Context, reader avatarCharacterReader, fluctlightID string) (map[string]any, error) {
	var raw []byte
	var active string
	if err := reader.QueryRow(ctx, `SELECT f.core_persona,COALESCE(r.active_profile_id,'') FROM public.fluctlights f LEFT JOIN public.fluctlight_personality_runtime r ON r.fluctlight_id=f.id WHERE f.id=$1`, fluctlightID).Scan(&raw, &active); err != nil {
		return nil, err
	}
	return avatarCharacterProfile(decodeObject(raw), active), nil
}

func avatarCharacterProfile(persona map[string]any, activeProfileID string) map[string]any {
	profile := map[string]any{}
	if identity := compactStateMap(persona["identity"], []string{"name", "gender", "occupation", "background", "background_story", "biography", "core_values", "worldview", "notes"}); len(identity) > 0 {
		profile["identity"] = identity
	}
	for _, key := range []string{"personality", "behavioral_policy"} {
		if value, ok := persona[key]; ok && value != nil {
			profile[key] = copyPromptData(value)
		}
	}
	if life := compactStateMap(persona["life_profile"], []string{"preferences", "life_habits", "character_constraints", "media_preferences"}); len(life) > 0 {
		profile["life_profile"] = life
	}
	system := mapValue(persona["personality_system"])
	active := firstString(activeProfileID, stringValue(system["active_profile_id"]))
	for _, item := range arrayValue(system["profiles"]) {
		candidate := mapValue(item)
		if active != "" && stringValue(candidate["id"]) == active {
			if selected := compactStateMap(candidate, []string{"name", "personality", "behavioral_policy", "description"}); len(selected) > 0 {
				profile["active_profile"] = selected
			}
			break
		}
	}
	return profile
}

func createVisualAssetReviewTx(ctx context.Context, tx pgx.Tx, fluctlightID, profileID, canonicalAssetID, sessionID, kind, intentID string, revision int) error {
	_, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_visual_asset_reviews(id,fluctlight_id,visual_identity_id,visual_revision,canonical_asset_id,session_id,kind,media_intent_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, "visual_asset_review_"+stableDigest(intentID), fluctlightID, profileID, revision, canonicalAssetID, nullableString(sessionID), kind, intentID)
	return err
}

func (a *App) enqueueCharacterSheetRetryTx(ctx context.Context, tx pgx.Tx, fluctlightID, profileID, canonicalAssetID, sessionID, sourceKey string, revision int, snapshot, constraints map[string]any, feedback string) (string, error) {
	intentID := "media_intent_" + stableDigest(sourceKey+":character-sheet")
	workflowID := "media_workflow_" + stableDigest(intentID)
	requestID := "media_request_" + stableDigest(intentID)
	concept := map[string]any{"purpose": "visual_identity", "stage": "character_sheet", "canonical_asset_id": canonicalAssetID, "visual_identity_reference_asset_id": canonicalAssetID, "visual_identity": map[string]any{"identity_snapshot": snapshot}, "renderer_constraints": constraints, "derived_review_feedback": feedback}
	if _, err := tx.Exec(ctx, `INSERT INTO public.media_intents(id,owner_fluctlight_id,kind,mime_type,prompt,provider_request_id,workflow_id,status,revision) VALUES($1,$2,'image','image/png',$3,$4,$5,'pending',0)`, intentID, fluctlightID, jsonString(concept), requestID, workflowID); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload) VALUES($1,$2,'media','media.generation',$3)`, "media_workflow_intent:"+intentID, workflowID, jsonBytes(map[string]any{"intent_id": intentID, "provider_request_id": requestID, "fluctlight_id": fluctlightID})); err != nil {
		return "", err
	}
	if err := createVisualAssetReviewTx(ctx, tx, fluctlightID, profileID, canonicalAssetID, sessionID, "character_sheet", intentID, revision); err != nil {
		return "", err
	}
	command, err := tx.Exec(ctx, `UPDATE public.fluctlight_visual_identity_sessions SET character_sheet_media_intent_id=$2,status='character_sheet_pending',updated_at=now() WHERE id=$1 AND fluctlight_id=$3 AND status='character_sheet_pending'`, sessionID, intentID, fluctlightID)
	if err != nil {
		return "", err
	}
	if command.RowsAffected() != 1 {
		return "", errors.New("visual_identity_character_sheet_session_changed")
	}
	return intentID, nil
}

func (a *App) recordVisualAssetQuality(ctx context.Context, intentID string, quality mediaQualityAcceptance, actualVerdict, reason, candidateSHA string) error {
	data := mediaQualityRetryFeedback(quality)
	data["verdict"] = actualVerdict
	data["candidate_sha256"] = candidateSHA
	if actualVerdict == mediaQualityVerdictPass {
		data["vision_config_version"] = quality.VisionConfigVersion
		data["vision_model_id"] = quality.VisionModelID
	}
	if reason != "" {
		data["reason"] = reason
	}
	_, err := a.DB.Pool().Exec(ctx, `UPDATE public.fluctlight_visual_asset_reviews SET review_data=$2,updated_at=now() WHERE media_intent_id=$1 AND status='generating'`, intentID, jsonBytes(data))
	return err
}

func (a *App) settleVisualAssetCandidateTx(ctx context.Context, tx pgx.Tx, owner, intentID, assetID string) error {
	var reviewID, fid, profileID, canonical, kind, status string
	var visualRevision int
	var reviewData []byte
	err := tx.QueryRow(ctx, `SELECT id,fluctlight_id,visual_identity_id,visual_revision,canonical_asset_id,kind,status,review_data FROM public.fluctlight_visual_asset_reviews WHERE media_intent_id=$1 FOR UPDATE`, intentID).Scan(&reviewID, &fid, &profileID, &visualRevision, &canonical, &kind, &status, &reviewData)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil // Older media jobs have no derived-image review record.
	}
	if err != nil {
		return err
	}
	if status != "generating" {
		return nil
	}
	var currentRevision int
	var currentCanonical, currentAvatarIntent string
	if err := tx.QueryRow(ctx, `SELECT current_revision,COALESCE(canonical_asset_id,''),COALESCE(generated_avatar_media_intent_id,'') FROM public.fluctlight_visual_identities WHERE id=$1 AND fluctlight_id=$2 FOR UPDATE`, profileID, fid).Scan(&currentRevision, &currentCanonical, &currentAvatarIntent); err != nil {
		return err
	}
	if owner != fid || currentRevision != visualRevision || currentCanonical != canonical || (kind == "avatar" && currentAvatarIntent != intentID) {
		_, err := tx.Exec(ctx, `UPDATE public.fluctlight_visual_asset_reviews SET status='superseded',revision=revision+1,updated_at=now() WHERE id=$1 AND status='generating'`, reviewID)
		return err
	}
	var assetOwner, assetStatus, assetSHA, qualityVerdict, qualitySHA string
	if err := tx.QueryRow(ctx, `SELECT a.owner_fluctlight_id,a.status,a.sha256,COALESCE(i.quality_verdict,''),COALESCE(i.quality_candidate_sha256,'') FROM public.media_assets a JOIN public.media_intents i ON i.id=$2 WHERE a.id=$1`, assetID, intentID).Scan(&assetOwner, &assetStatus, &assetSHA, &qualityVerdict, &qualitySHA); err != nil {
		return err
	}
	if assetOwner != fid || assetStatus != "ready" || assetSHA == "" {
		return errors.New("visual_asset_candidate_invalid")
	}
	newStatus, source := "awaiting_review", any(nil)
	if qualityVerdict == mediaQualityVerdictPass && qualitySHA == assetSHA {
		var raw string
		var currentVersion int64
		settingsErr := tx.QueryRow(ctx, `SELECT value_json,revision FROM public.runtime_settings WHERE key=$1 FOR SHARE`, visionUnderstandingSettingKey).Scan(&raw, &currentVersion)
		if settingsErr != nil && !errors.Is(settingsErr, pgx.ErrNoRows) {
			return settingsErr
		}
		settings, decodeErr := decodeVisionUnderstandingSettings([]byte(raw))
		storedVersion := int64(intValue(decodeObject(reviewData)["vision_config_version"]))
		if settingsErr == nil && decodeErr == nil && settings.Enabled && storedVersion > 0 && storedVersion == currentVersion {
			newStatus, source = "accepted", "vision"
		}
	}
	if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_visual_asset_reviews SET candidate_asset_id=$2,status=$3,review_source=$4,revision=revision+1,updated_at=now() WHERE id=$1 AND status='generating'`, reviewID, assetID, newStatus, source); err != nil {
		return err
	}
	if kind == "avatar" && newStatus == "accepted" {
		return applyGeneratedAvatarTx(ctx, tx, profileID, visualRevision, canonical, intentID, assetID)
	}
	return nil
}

func applyGeneratedAvatarTx(ctx context.Context, tx pgx.Tx, profileID string, visualRevision int, canonical, intentID, assetID string) error {
	command, err := tx.Exec(ctx, `UPDATE public.fluctlight_visual_identities SET generated_avatar_asset_id=$5,updated_at=now() WHERE id=$1 AND current_revision=$2 AND canonical_asset_id=$3 AND generated_avatar_media_intent_id=$4`, profileID, visualRevision, canonical, intentID, assetID)
	if err != nil {
		return err
	}
	if command.RowsAffected() != 1 {
		return errors.New("generated_avatar_reference_changed")
	}
	_, err = tx.Exec(ctx, `UPDATE public.fluctlight_visual_identity_revisions SET generated_avatar_asset_id=$3 WHERE visual_identity_id=$1 AND revision=$2`, profileID, visualRevision, assetID)
	return err
}

func readCurrentVisualAssetReviewsWith(ctx context.Context, q lifeContextQuerier, fluctlightID string, visualRevision int) (map[string]any, error) {
	rows, err := q.Query(ctx, `SELECT DISTINCT ON (r.kind) r.kind,jsonb_build_object('id',r.id,'kind',r.kind,'visual_revision',r.visual_revision,'revision',r.revision,'candidate_asset_id',r.candidate_asset_id,'reference_asset_id',r.canonical_asset_id,'media_intent_id',r.media_intent_id,'status',CASE WHEN r.status='generating' AND i.status='failed' THEN 'failed' ELSE r.status END,'media_status',i.status,'review_source',r.review_source,'review_data',r.review_data - 'candidate_sha256' - 'vision_config_version' - 'vision_model_id','feedback',r.feedback,'created_at',r.created_at,'updated_at',r.updated_at) FROM public.fluctlight_visual_asset_reviews r JOIN public.media_intents i ON i.id=r.media_intent_id WHERE r.fluctlight_id=$1 AND r.visual_revision=$2 ORDER BY r.kind,r.created_at DESC,r.id DESC`, fluctlightID, visualRevision)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := map[string]any{}
	for rows.Next() {
		var kind string
		var raw []byte
		if err := rows.Scan(&kind, &raw); err != nil {
			return nil, err
		}
		result[kind] = decodeObject(raw)
	}
	return result, rows.Err()
}
