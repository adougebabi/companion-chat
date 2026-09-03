package core

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/minio/minio-go/v7"
)

const (
	visualIdentitySchemaVersion  = "visual-identity.v1"
	visualIdentityAdapterVersion = "chest-cup-adapter.v1"
	visualIdentityMaxAttempts    = 3
	// Renderer tuning values are intentionally centralized here so the owner
	// can adjust the if/else mapping after validating the dedicated LoRA. B/C/D
	// are provisional defaults until image tests establish better calibration.
	chestCupWeightA = -5.0
	chestCupWeightB = -3.0
	chestCupWeightC = -1.0
	chestCupWeightD = 1.0
)

const (
	visualIdentityStatusMissing           = "missing"
	visualIdentityStatusQueued            = "queued"
	visualIdentityStatusRunning           = "running"
	visualIdentityStatusAwaitingReview    = "awaiting_review"
	visualIdentityStatusActive            = "active"
	visualIdentityStatusFailed            = "failed"
	visualIdentityStatusRendererPending   = "renderer_config_pending"
	visualIdentityStageSessionCreated     = "session_created"
	visualIdentityStageSeedRequested      = "seed_requested"
	visualIdentityStageSeedReady          = "seed_ready"
	visualIdentityStageImageRequested     = "image_requested"
	visualIdentityStageImageReady         = "image_ready"
	visualIdentityStageVisionRequested    = "vision_requested"
	visualIdentityStageVisionReady        = "vision_ready"
	visualIdentityStagePatchRequested     = "patch_requested"
	visualIdentityStagePatchReady         = "patch_ready"
	visualIdentityStageRegenerate         = "regenerate"
	visualIdentityStageAccepted           = "accepted"
	visualIdentityStageCharacterRequested = "character_sheet_requested"
	visualIdentityStageCharacterReady     = "character_sheet_ready"
	visualIdentityStageCompleted          = "completed"
	visualIdentityStageFailed             = "failed"
)

// VisualIdentitySnapshot is the browser-safe, cognition-safe representation of
// the current visual identity aggregate. Large model responses and provider
// diagnostics stay in attempt rows and are never copied into this snapshot.
type VisualIdentitySnapshot struct {
	SchemaVersion         string         `json:"schema_version"`
	ID                    string         `json:"id"`
	FluctlightID          string         `json:"fluctlight_id"`
	Status                string         `json:"status"`
	CurrentRevision       int            `json:"current_revision"`
	IdentitySnapshot      map[string]any `json:"identity_snapshot"`
	RendererConstraints   map[string]any `json:"renderer_constraints"`
	CanonicalAssetID      string         `json:"canonical_asset_id,omitempty"`
	CharacterSheetAssetID string         `json:"character_sheet_asset_id,omitempty"`
	AdapterVersion        string         `json:"adapter_version"`
	ActiveSessionID       string         `json:"active_session_id,omitempty"`
}

// NormalizeChestCup canonicalizes the semantic input used by the renderer
// adapter. Unsupported values remain errors; the server never extrapolates a
// weight from an unknown label.
func NormalizeChestCup(value string) (string, error) {
	cup := strings.ToUpper(strings.TrimSpace(value))
	cup = strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(cup, "罩杯"), "杯"), " CUP"))
	if cup == "" {
		return "", errors.New("chest_cup_required")
	}
	switch cup {
	case "A", "B", "C", "D":
		return cup, nil
	default:
		return "", fmt.Errorf("unsupported chest cup %q", value)
	}
}

// chestCupToLoRAWeight is deliberately explicit: the mapping changes rarely
// and is part of the adapter's versioned code contract. The values are
// renderer tuning defaults and can be revised by bumping the adapter version.
func chestCupToLoRAWeight(value string) (float64, string, error) {
	cup, err := NormalizeChestCup(value)
	if err != nil {
		return 0, visualIdentityAdapterVersion, err
	}
	var weight float64
	switch cup {
	case "A":
		weight = chestCupWeightA
	case "B":
		weight = chestCupWeightB
	case "C":
		weight = chestCupWeightC
	case "D":
		weight = chestCupWeightD
	}
	if math.IsNaN(weight) || math.IsInf(weight, 0) || weight < -10 || weight > 10 {
		return 0, visualIdentityAdapterVersion, errors.New("chest_lora_weight_invalid")
	}
	return weight, visualIdentityAdapterVersion, nil
}

func chestRendererConstraints(lifeProfile map[string]any) (map[string]any, error) {
	appearance := mapValue(lifeProfile["appearance"])
	value := chestCupCandidate(appearance)
	if value == "" {
		return map[string]any{"schema_version": visualIdentitySchemaVersion, "adapter_version": visualIdentityAdapterVersion}, nil
	}
	cup, err := NormalizeChestCup(value)
	if err != nil {
		return nil, err
	}
	weight, adapterVersion, err := chestCupToLoRAWeight(cup)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"schema_version":    visualIdentitySchemaVersion,
		"chest_cup":         cup,
		"chest_lora_weight": weight,
		"adapter_version":   adapterVersion,
	}, nil
}

func chestCupCandidate(appearance map[string]any) string {
	for _, key := range []string{"chest_cup", "cup_size"} {
		if value := strings.TrimSpace(stringValue(appearance[key])); value != "" {
			return value
		}
	}
	// Free-form fields such as bust/body_type/build are accepted only when they
	// already contain an explicit cup label. Values like “平胸” or “slim” are
	// ordinary body descriptions and must not be guessed into a LoRA weight.
	for _, key := range []string{"bust", "body_type", "build"} {
		candidate := strings.ToUpper(strings.TrimSpace(stringValue(appearance[key])))
		candidate = strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(candidate, "罩杯"), "杯"), " CUP"))
		if candidate == "A" || candidate == "B" || candidate == "C" || candidate == "D" {
			return candidate
		}
	}
	return ""
}

func rendererConstraintsForCorePersona(corePersona map[string]any) (map[string]any, error) {
	identity := mapValue(corePersona["identity"])
	lifeProfile := mapValue(corePersona["life_profile"])
	gender := strings.ToLower(strings.TrimSpace(stringValue(identity["gender"])))
	if gender == "male" || gender == "m" || gender == "男" || gender == "男性" {
		return map[string]any{
			"schema_version":        visualIdentitySchemaVersion,
			"chest_cup":             "not_applicable",
			"chest_lora_weight":     0.0,
			"chest_lora_applicable": false,
			"adapter_version":       visualIdentityAdapterVersion,
		}, nil
	}
	constraints, err := chestRendererConstraints(lifeProfile)
	if err != nil {
		return nil, err
	}
	if stringValue(constraints["chest_cup"]) == "" {
		if appearance := mapValue(identity["appearance"]); len(appearance) > 0 {
			value := chestCupCandidate(appearance)
			if value != "" {
				cup, normalizeErr := NormalizeChestCup(value)
				if normalizeErr != nil {
					return nil, normalizeErr
				}
				weight, version, weightErr := chestCupToLoRAWeight(cup)
				if weightErr != nil {
					return nil, weightErr
				}
				constraints["chest_cup"] = cup
				constraints["chest_lora_weight"] = weight
				constraints["adapter_version"] = version
			}
		}
		if value := chestCupCandidate(identity); value != "" {
			cup, normalizeErr := NormalizeChestCup(value)
			if normalizeErr != nil {
				return nil, normalizeErr
			}
			weight, version, weightErr := chestCupToLoRAWeight(cup)
			if weightErr != nil {
				return nil, weightErr
			}
			constraints["chest_cup"] = cup
			constraints["chest_lora_weight"] = weight
			constraints["adapter_version"] = version
		}
	}
	return constraints, nil
}

func visualIdentityProfileID(fluctlightID string) string {
	return "visual_identity_" + stableDigest(fluctlightID)
}

func visualIdentitySessionID(fluctlightID, triggerType, sourceFactID string) string {
	key := strings.TrimSpace(sourceFactID)
	if key == "" {
		key = triggerType
	}
	return "visual_identity_session_" + stableDigest(fluctlightID+":"+triggerType+":"+key)
}

func visualIdentityWorkflowID(sessionID string) string {
	return "visual_identity_workflow_" + stableDigest(sessionID)
}

func visualIdentityAttemptID(sessionID string, attempt int) string {
	return fmt.Sprintf("%s_attempt_%d", sessionID, attempt)
}

// ensureVisualIdentityInitializationTx is shared by Fluctlight creation and
// WakeUp. It only writes domain state and a committed workflow intent; the
// Worker owns every Provider/Temporal side effect after the transaction.
func (a *App) ensureVisualIdentityInitializationTx(ctx context.Context, tx pgx.Tx, fluctlightID, triggerType, sourceFactID string, corePersona map[string]any) (string, error) {
	if strings.TrimSpace(fluctlightID) == "" {
		return "", errors.New("visual_identity_fluctlight_required")
	}
	profileID := visualIdentityProfileID(fluctlightID)
	lifeProfile := mapValue(corePersona["life_profile"])
	constraints, constraintErr := rendererConstraintsForCorePersona(corePersona)
	profileStatus := visualIdentityStatusMissing
	if constraintErr != nil {
		profileStatus = visualIdentityStatusRendererPending
		constraints = map[string]any{"schema_version": visualIdentitySchemaVersion, "adapter_version": visualIdentityAdapterVersion, "error": constraintErr.Error()}
	}
	identitySnapshot := map[string]any{
		"schema_version": visualIdentitySchemaVersion,
		"identity":       cloneMap(mapValue(corePersona["identity"])),
		"life_profile":   cloneMap(lifeProfile),
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_visual_identities(id,fluctlight_id,status,current_revision,identity_snapshot,renderer_constraints,adapter_version) VALUES($1,$2,$3,0,$4,$5,$6) ON CONFLICT(fluctlight_id) DO NOTHING`, profileID, fluctlightID, profileStatus, jsonBytes(identitySnapshot), jsonBytes(constraints), visualIdentityAdapterVersion); err != nil {
		return "", err
	}
	if constraintErr == nil {
		// A profile created by an earlier build may still be missing a semantic
		// cup label because the model placed it under identity.body_type/build.
		// Repair only the pre-canonical, missing profile and queued attempts; a
		// canonical revision or an attempt with a frozen media intent is immutable.
		if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_visual_identities SET identity_snapshot=$2,renderer_constraints=$3,adapter_version=$4,updated_at=now() WHERE id=$1 AND status='missing' AND current_revision=0`, profileID, jsonBytes(identitySnapshot), jsonBytes(constraints), visualIdentityAdapterVersion); err != nil {
			return "", err
		}
		if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_visual_identity_attempts SET input_snapshot=$2,renderer_constraints=$3,updated_at=now() WHERE visual_identity_id=$1 AND media_intent_id IS NULL AND status='queued'`, profileID, jsonBytes(identitySnapshot), jsonBytes(constraints)); err != nil {
			return "", err
		}
	}
	var existingSession string
	err := tx.QueryRow(ctx, `SELECT id FROM public.fluctlight_visual_identity_sessions WHERE fluctlight_id=$1 AND status IN ('queued','running') ORDER BY created_at DESC LIMIT 1`, fluctlightID).Scan(&existingSession)
	if err == nil {
		return existingSession, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	sessionID := visualIdentitySessionID(fluctlightID, triggerType, sourceFactID)
	workflowID := visualIdentityWorkflowID(sessionID)
	if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_visual_identity_sessions(id,visual_identity_id,fluctlight_id,trigger_type,workflow_id,source_fact_id,max_attempts,current_attempt,status) VALUES($1,$2,$3,$4,$5,$6,$7,1,'queued') ON CONFLICT(id) DO NOTHING`, sessionID, profileID, fluctlightID, triggerType, workflowID, nullableString(sourceFactID), visualIdentityMaxAttempts); err != nil {
		return "", err
	}
	attemptID := visualIdentityAttemptID(sessionID, 1)
	if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_visual_identity_attempts(id,session_id,visual_identity_id,fluctlight_id,attempt_number,status,input_snapshot,renderer_constraints) VALUES($1,$2,$3,$4,1,'queued',$5,$6) ON CONFLICT(session_id,attempt_number) DO NOTHING`, attemptID, sessionID, profileID, fluctlightID, jsonBytes(identitySnapshot), jsonBytes(constraints)); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_visual_identities SET status=$2,active_session_id=$3,updated_at=now() WHERE id=$1`, profileID, profileStatus, sessionID); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload) VALUES($1,$2,'lifecycle','visual_identity.initialize',$3) ON CONFLICT DO NOTHING`, "visual_identity_intent:"+sessionID, workflowID, jsonBytes(map[string]any{"intent_id": "visual_identity_intent:" + sessionID, "fluctlight_id": fluctlightID, "session_id": sessionID})); err != nil {
		return "", err
	}
	if err := appendVisualIdentityTimelineTx(ctx, tx, sessionID, attemptID, fluctlightID, visualIdentityStageSessionCreated, "queued", "Visual Identity 初始化已排队", nil, map[string]any{"trigger": triggerType}, workflowID); err != nil {
		return "", err
	}
	return sessionID, nil
}

func appendVisualIdentityTimelineTx(ctx context.Context, tx pgx.Tx, sessionID, attemptID, fluctlightID, stage, status, summary string, assetIDs []string, metadata map[string]any, correlationID string) error {
	id := "visual_identity_event_" + stableDigest(strings.Join([]string{sessionID, attemptID, stage, status, summary, strings.Join(assetIDs, ",")}, ":"))
	if metadata == nil {
		metadata = map[string]any{}
	}
	if assetIDs == nil {
		assetIDs = []string{}
	}
	_, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_visual_identity_timeline(id,session_id,attempt_id,fluctlight_id,stage,status,summary,asset_ids,metadata,correlation_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT(id) DO NOTHING`, id, sessionID, nullableString(attemptID), fluctlightID, stage, status, visualIdentityBoundedText(summary, 512), jsonBytes(assetIDs), jsonBytes(metadata), visualIdentityBoundedText(correlationID, 128))
	return err
}

func visualIdentityBoundedText(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit < 1 {
		return ""
	}
	runes := []rune(value)
	if len(runes) > limit {
		return string(runes[:limit])
	}
	return value
}

// EnsureVisualIdentityInitialization creates or reuses the initialization
// intent outside a caller-owned transaction (WakeUp uses the Tx variant).
func (a *App) EnsureVisualIdentityInitialization(ctx context.Context, fluctlightID, triggerType, sourceFactID string) (string, error) {
	var persona map[string]any
	var raw []byte
	if err := a.DB.Pool().QueryRow(ctx, `SELECT core_persona FROM public.fluctlights WHERE id=$1 AND status <> 'retired'`, fluctlightID).Scan(&raw); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrNotFound
		}
		return "", err
	}
	if err := json.Unmarshal(raw, &persona); err != nil {
		return "", err
	}
	var sessionID string
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var err error
		sessionID, err = a.ensureVisualIdentityInitializationTx(ctx, tx, fluctlightID, triggerType, sourceFactID, persona)
		return err
	})
	return sessionID, err
}

func (a *App) readVisualIdentity(ctx context.Context, fluctlightID string) (VisualIdentitySnapshot, error) {
	var result VisualIdentitySnapshot
	var identity, constraints []byte
	err := a.DB.Pool().QueryRow(ctx, `SELECT id,fluctlight_id,status,current_revision,identity_snapshot,renderer_constraints,COALESCE(canonical_asset_id,''),COALESCE(character_sheet_asset_id,''),adapter_version,COALESCE(active_session_id,'') FROM public.fluctlight_visual_identities WHERE fluctlight_id=$1`, fluctlightID).Scan(&result.ID, &result.FluctlightID, &result.Status, &result.CurrentRevision, &identity, &constraints, &result.CanonicalAssetID, &result.CharacterSheetAssetID, &result.AdapterVersion, &result.ActiveSessionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return VisualIdentitySnapshot{SchemaVersion: visualIdentitySchemaVersion, FluctlightID: fluctlightID, Status: visualIdentityStatusMissing, IdentitySnapshot: map[string]any{}, RendererConstraints: map[string]any{}, AdapterVersion: visualIdentityAdapterVersion}, nil
	}
	if err != nil {
		return result, err
	}
	result.SchemaVersion = visualIdentitySchemaVersion
	result.IdentitySnapshot = decodeObject(identity)
	result.RendererConstraints = decodeObject(constraints)
	return result, nil
}

func (a *App) readVisualIdentityDetail(ctx context.Context, fluctlightID string) (map[string]any, error) {
	snapshot, err := a.readVisualIdentity(ctx, fluctlightID)
	if err != nil {
		return nil, err
	}
	result := map[string]any{
		"schema_version":           snapshot.SchemaVersion,
		"id":                       snapshot.ID,
		"fluctlight_id":            snapshot.FluctlightID,
		"status":                   snapshot.Status,
		"current_revision":         snapshot.CurrentRevision,
		"identity_snapshot":        snapshot.IdentitySnapshot,
		"renderer_constraints":     snapshot.RendererConstraints,
		"canonical_asset_id":       snapshot.CanonicalAssetID,
		"character_sheet_asset_id": snapshot.CharacterSheetAssetID,
		"adapter_version":          snapshot.AdapterVersion,
		"active_session_id":        snapshot.ActiveSessionID,
		"timeline":                 []map[string]any{},
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT session_id,COALESCE(attempt_id,''),stage,status,summary,asset_ids,metadata,correlation_id,occurred_at FROM public.fluctlight_visual_identity_timeline WHERE fluctlight_id=$1 ORDER BY occurred_at,id LIMIT 200`, fluctlightID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	timeline := make([]map[string]any, 0)
	for rows.Next() {
		var sessionID, attemptID, stage, status, summary, correlation string
		var assets, metadata []byte
		var occurred time.Time
		if err := rows.Scan(&sessionID, &attemptID, &stage, &status, &summary, &assets, &metadata, &correlation, &occurred); err != nil {
			return nil, err
		}
		timeline = append(timeline, map[string]any{"session_id": sessionID, "attempt_id": attemptID, "stage": stage, "status": status, "summary": summary, "asset_ids": decodeArray(assets), "metadata": decodeObject(metadata), "correlation_id": correlation, "occurred_at": occurred.Format(time.RFC3339Nano)})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	result["timeline"] = timeline
	return result, nil
}

func (a *App) hasActiveVisualIdentity(ctx context.Context, fluctlightID string) (bool, error) {
	var active bool
	err := a.DB.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.fluctlight_visual_identities WHERE fluctlight_id=$1 AND status='active' AND canonical_asset_id IS NOT NULL)`, fluctlightID).Scan(&active)
	return active, err
}

// ProcessVisualIdentity advances one durable state-machine checkpoint. The
// Worker may call it repeatedly; each checkpoint is idempotent and keeps large
// provider payloads in PostgreSQL rather than Temporal history.
func (a *App) ProcessVisualIdentity(ctx context.Context, sessionID string) (map[string]any, error) {
	var fluctlightID, profileID, sessionStatus, characterSheetIntentID string
	var attempt, maxAttempts int
	if err := a.DB.Pool().QueryRow(ctx, `SELECT fluctlight_id,visual_identity_id,status,current_attempt,max_attempts,COALESCE(character_sheet_media_intent_id,'') FROM public.fluctlight_visual_identity_sessions WHERE id=$1`, sessionID).Scan(&fluctlightID, &profileID, &sessionStatus, &attempt, &maxAttempts, &characterSheetIntentID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if sessionStatus == "completed" || sessionStatus == "cancelled" || sessionStatus == "failed" || sessionStatus == visualIdentityStatusAwaitingReview {
		return map[string]any{"session_id": sessionID, "fluctlight_id": fluctlightID, "status": sessionStatus, "attempt": attempt}, nil
	}
	if maxAttempts < 1 {
		maxAttempts = visualIdentityMaxAttempts
	}
	if sessionStatus == "character_sheet_pending" {
		if characterSheetIntentID == "" {
			return nil, errors.New("character_sheet_intent_missing")
		}
		var mediaStatus string
		if err := a.DB.Pool().QueryRow(ctx, `SELECT status FROM public.media_intents WHERE id=$1`, characterSheetIntentID).Scan(&mediaStatus); err != nil {
			return nil, err
		}
		if mediaStatus != "completed" {
			return map[string]any{"session_id": sessionID, "attempt": attempt, "status": "waiting", "stage": "character_sheet_" + mediaStatus, "media_intent_id": characterSheetIntentID}, nil
		}
		assetID := "asset_" + characterSheetIntentID
		if err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_visual_identities SET status='active',character_sheet_asset_id=$2,active_session_id=$3,updated_at=now() WHERE id=$1`, profileID, assetID, sessionID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_visual_identity_revisions SET character_sheet_asset_id=$2 WHERE visual_identity_id=$1 AND revision=(SELECT current_revision FROM public.fluctlight_visual_identities WHERE id=$1)`, profileID, assetID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_visual_identity_sessions SET status='completed',updated_at=now() WHERE id=$1`, sessionID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_visual_identity_attempts SET status='completed',updated_at=now() WHERE id=$1`, visualIdentityAttemptID(sessionID, attempt)); err != nil {
				return err
			}
			if err := appendVisualIdentityTimelineTx(ctx, tx, sessionID, visualIdentityAttemptID(sessionID, attempt), fluctlightID, visualIdentityStageCharacterReady, "completed", "character sheet 已生成", []string{assetID}, nil, "visual_identity:"+sessionID); err != nil {
				return err
			}
			return appendVisualIdentityTimelineTx(ctx, tx, sessionID, visualIdentityAttemptID(sessionID, attempt), fluctlightID, visualIdentityStageCompleted, "completed", "Visual Identity 工作流完成", []string{assetID}, nil, "visual_identity:"+sessionID)
		}); err != nil {
			return nil, err
		}
		return map[string]any{"session_id": sessionID, "attempt": attempt, "status": "completed", "stage": "completed", "character_sheet_asset_id": assetID}, nil
	}
	attemptID := visualIdentityAttemptID(sessionID, attempt)
	var seedPrompt, mediaIntentID, candidateAssetID, decision string
	var inputSnapshot, constraints, visionResult, patchResult []byte
	if err := a.DB.Pool().QueryRow(ctx, `SELECT COALESCE(seed_prompt,''),COALESCE(media_intent_id,''),COALESCE(candidate_asset_id,''),COALESCE(decision,''),input_snapshot,renderer_constraints,vision_result,patch_result FROM public.fluctlight_visual_identity_attempts WHERE id=$1`, attemptID).Scan(&seedPrompt, &mediaIntentID, &candidateAssetID, &decision, &inputSnapshot, &constraints, &visionResult, &patchResult); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if rendererError := stringValue(decodeObject(constraints)["error"]); rendererError != "" {
		_, _ = a.DB.Pool().Exec(ctx, `UPDATE public.fluctlight_visual_identity_sessions SET status='awaiting_review',last_error=$2,updated_at=now() WHERE id=$1`, sessionID, rendererError)
		_ = a.recordVisualIdentityStage(ctx, sessionID, attemptID, fluctlightID, visualIdentityStageFailed, visualIdentityStatusRendererPending, "等待有效的胸部渲染配置", nil)
		return map[string]any{"session_id": sessionID, "attempt": attempt, "status": visualIdentityStatusRendererPending, "stage": "renderer_config_pending", "error_code": rendererError}, nil
	}
	if seedPrompt == "" {
		if _, err := a.DB.Pool().Exec(ctx, `UPDATE public.fluctlight_visual_identity_sessions SET status='running',updated_at=now() WHERE id=$1 AND status IN ('queued','running')`, sessionID); err != nil {
			return nil, err
		}
		_ = a.recordVisualIdentityStage(ctx, sessionID, attemptID, fluctlightID, visualIdentityStageSeedRequested, "running", "正在生成“自己”的三视图文本种子", nil)
		identity, err := a.readVisualIdentity(ctx, fluctlightID)
		if err != nil {
			return nil, err
		}
		completion, err := a.Provider.StructuredWithSchema(ctx, "visual_identity_patch", []map[string]any{{"role": "system", "content": "Generate the first pure text-to-image prompt for a single human character turnaround reference sheet. The result must depict exactly one consistent human character, full body, shown in front view, side view, and back view together on one neutral plain studio/reference-sheet background. This is a character design reference, not an artistic scene or editorial photo: no abstract silhouette, no landscape, no decorative environment, no extra people, no collage of unrelated subjects, no logo or text. Return only the structured visual identity patch contract and do not invent unsupported identity facts."}, {"role": "user", "content": jsonString(map[string]any{"stage": "seed", "render_intent": "character_turnaround", "subject_count": 1, "views": []string{"front", "side", "back"}, "visual_identity": identity, "input_snapshot": decodeObject(inputSnapshot), "renderer_constraints": decodeObject(constraints)})}}, "visual_identity_seed_response", visualIdentitySeedResponseSchema(), false)
		if err != nil {
			if visualIdentityProviderPending(err) {
				_ = a.recordVisualIdentityStage(ctx, sessionID, attemptID, fluctlightID, visualIdentityStageSeedRequested, "pending", "等待 visual_identity_patch 模型角色配置", nil)
				return map[string]any{"session_id": sessionID, "attempt": attempt, "status": "waiting", "stage": "provider_config_pending", "error_code": "visual_identity_patch_role_missing"}, nil
			}
			return nil, err
		}
		seedPrompt = strings.TrimSpace(stringValue(completion["seed_prompt"]))
		if seedPrompt == "" {
			return map[string]any{"session_id": sessionID, "status": visualIdentityStatusAwaitingReview, "stage": "awaiting_input", "error_code": "seed_prompt_empty"}, nil
		}
		mediaIntentID = "media_intent_" + stableDigest(attemptID+":seed")
		workflowID := "media_workflow_" + stableDigest(mediaIntentID)
		requestID := "media_request_" + stableDigest(mediaIntentID)
		concept := map[string]any{"purpose": "visual_identity", "stage": "seed", "render_intent": "character_turnaround", "subject_count": 1, "views": []string{"front", "side", "back"}, "prompt": seedPrompt, "visual_identity": identity, "renderer_constraints": decodeObject(constraints)}
		err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_visual_identity_attempts SET seed_prompt=$2,media_intent_id=$3,status='image_queued',updated_at=now() WHERE id=$1 AND seed_prompt IS NULL`, attemptID, seedPrompt, mediaIntentID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO public.media_intents(id,owner_fluctlight_id,kind,mime_type,prompt,provider_request_id,workflow_id,status,revision) VALUES($1,$2,'image','image/png',$3,$4,$5,'pending',0) ON CONFLICT(id) DO NOTHING`, mediaIntentID, fluctlightID, jsonString(concept), requestID, workflowID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload) VALUES($1,$2,'media','media.generation',$3) ON CONFLICT DO NOTHING`, "media_workflow_intent:"+mediaIntentID, workflowID, jsonBytes(map[string]any{"intent_id": mediaIntentID, "provider_request_id": requestID, "fluctlight_id": fluctlightID})); err != nil {
				return err
			}
			if err := appendVisualIdentityTimelineTx(ctx, tx, sessionID, attemptID, fluctlightID, visualIdentityStageImageRequested, "queued", "纯文生图已排队", nil, map[string]any{"media_intent_id": mediaIntentID}, workflowID); err != nil {
				return err
			}
			return appendVisualIdentityTimelineTx(ctx, tx, sessionID, attemptID, fluctlightID, visualIdentityStageSeedReady, "completed", "三视图文本种子已生成，等待图片工作流", nil, map[string]any{"media_intent_id": mediaIntentID}, workflowID)
		})
		if err != nil {
			return nil, err
		}
		return map[string]any{"session_id": sessionID, "attempt": attempt, "status": "waiting", "stage": "image_queued", "media_intent_id": mediaIntentID}, nil
	}
	if candidateAssetID == "" && mediaIntentID != "" {
		var mediaStatus string
		if err := a.DB.Pool().QueryRow(ctx, `SELECT status FROM public.media_intents WHERE id=$1`, mediaIntentID).Scan(&mediaStatus); err != nil {
			return nil, err
		}
		if mediaStatus != "completed" {
			return map[string]any{"session_id": sessionID, "attempt": attempt, "status": "waiting", "stage": "image_" + mediaStatus, "media_intent_id": mediaIntentID}, nil
		}
		candidateAssetID = "asset_" + mediaIntentID
		if _, err := a.DB.Pool().Exec(ctx, `UPDATE public.fluctlight_visual_identity_attempts SET candidate_asset_id=$2,status='vision_queued',updated_at=now() WHERE id=$1 AND candidate_asset_id IS NULL`, attemptID, candidateAssetID); err != nil {
			return nil, err
		}
		_ = a.recordVisualIdentityStage(ctx, sessionID, attemptID, fluctlightID, visualIdentityStageImageReady, "completed", "候选图已生成", []string{candidateAssetID})
	}
	if len(visionResult) == 0 {
		_ = a.recordVisualIdentityStage(ctx, sessionID, attemptID, fluctlightID, visualIdentityStageVisionRequested, "running", "正在进行视觉理解", []string{candidateAssetID})
		imageContent, imageErr := a.visualIdentityImageContent(ctx, candidateAssetID)
		if imageErr != nil {
			return nil, imageErr
		}
		visionUserContent := []any{map[string]any{"type": "text", "text": jsonString(map[string]any{"asset_id": candidateAssetID, "render_intent": "character_turnaround", "expected_subject": "one_human_character", "expected_views": []string{"front", "side", "back"}, "visual_identity": decodeObject(inputSnapshot), "renderer_constraints": decodeObject(constraints)})}}
		if imageContent != nil {
			visionUserContent = append(visionUserContent, imageContent)
		}
		completion, err := a.Provider.StructuredWithSchema(ctx, "visual_identity_vision", []map[string]any{{"role": "system", "content": "Inspect the supplied candidate image for visual identity continuity. The required target is exactly one full-body human character turnaround reference showing front, side, and back views together. If the image is an art photo, abstract silhouette, landscape, object-only image, or lacks a human subject or any required view, report a low identity_match and make that mismatch explicit in observations. Return bounded structured observations only."}, {"role": "user", "content": visionUserContent}}, "visual_identity_vision_response", visualIdentityVisionResponseSchema(), false)
		if err != nil {
			if visualIdentityProviderPending(err) {
				_ = a.recordVisualIdentityStage(ctx, sessionID, attemptID, fluctlightID, visualIdentityStageVisionRequested, "pending", "等待 visual_identity_vision 模型角色配置", []string{candidateAssetID})
				return map[string]any{"session_id": sessionID, "attempt": attempt, "status": "waiting", "stage": "provider_config_pending", "error_code": "visual_identity_vision_role_missing"}, nil
			}
			return nil, err
		}
		if err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_visual_identity_attempts SET vision_result=$2,status='patch_queued',updated_at=now() WHERE id=$1 AND vision_result='{}'::jsonb`, attemptID, jsonBytes(completion)); err != nil {
				return err
			}
			return appendVisualIdentityTimelineTx(ctx, tx, sessionID, attemptID, fluctlightID, visualIdentityStageVisionReady, "completed", "视觉理解完成", []string{candidateAssetID}, map[string]any{"confidence": completion["confidence"]}, "visual_identity:"+sessionID)
		}); err != nil {
			return nil, err
		}
		return map[string]any{"session_id": sessionID, "attempt": attempt, "status": "waiting", "stage": "patch_queued", "asset_id": candidateAssetID}, nil
	}
	if decision == "" {
		_ = a.recordVisualIdentityStage(ctx, sessionID, attemptID, fluctlightID, visualIdentityStagePatchRequested, "running", "正在评审并生成身份补丁", []string{candidateAssetID})
		completion, err := a.Provider.StructuredWithSchema(ctx, "visual_identity_patch", []map[string]any{{"role": "system", "content": "Review the candidate against the visual identity and return accepted or regenerate. Acceptance is allowed only for exactly one consistent full-body human character turnaround reference with front, side, and back views together on a neutral plain background. An art photo, abstract silhouette, landscape, object-only image, missing person, or missing view must be decision=regenerate. Preserve the explicit decision and a structured patch."}, {"role": "user", "content": jsonString(map[string]any{"stage": "review", "render_intent": "character_turnaround", "expected_subject": "one_human_character", "expected_views": []string{"front", "side", "back"}, "visual_identity": decodeObject(inputSnapshot), "renderer_constraints": decodeObject(constraints), "vision": decodeObject(visionResult), "candidate_asset_id": candidateAssetID})}}, "visual_identity_patch_response", visualIdentityPatchResponseSchema(), false)
		if err != nil {
			if visualIdentityProviderPending(err) {
				_ = a.recordVisualIdentityStage(ctx, sessionID, attemptID, fluctlightID, visualIdentityStagePatchRequested, "pending", "等待 visual_identity_patch 模型角色配置", []string{candidateAssetID})
				return map[string]any{"session_id": sessionID, "attempt": attempt, "status": "waiting", "stage": "provider_config_pending", "error_code": "visual_identity_patch_role_missing"}, nil
			}
			return nil, err
		}
		decision = firstString(stringValue(completion["decision"]), "regenerate")
		if decision != "accepted" && decision != "regenerate" {
			return nil, errors.New("visual_identity_decision_invalid")
		}
		err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_visual_identity_attempts SET patch_result=$2,decision=$3,feedback=$4,status=$5,updated_at=now() WHERE id=$1 AND decision IS NULL`, attemptID, jsonBytes(completion), decision, nullableString(stringValue(completion["feedback"])), map[bool]string{true: "accepted", false: "rejected_not_self"}[decision == "accepted"]); err != nil {
				return err
			}
			if err := appendVisualIdentityTimelineTx(ctx, tx, sessionID, attemptID, fluctlightID, visualIdentityStagePatchReady, "completed", "身份补丁已生成", []string{candidateAssetID}, map[string]any{"decision": decision}, "visual_identity:"+sessionID); err != nil {
				return err
			}
			stage := visualIdentityStageRegenerate
			status := "rejected_not_self"
			if decision == "accepted" {
				stage, status = visualIdentityStageAccepted, "completed"
			}
			return appendVisualIdentityTimelineTx(ctx, tx, sessionID, attemptID, fluctlightID, stage, status, visualIdentityBoundedText(stringValue(completion["summary"]), 512), []string{candidateAssetID}, map[string]any{"decision": decision}, "visual_identity:"+sessionID)
		})
		if err != nil {
			return nil, err
		}
		if decision == "regenerate" {
			if attempt >= maxAttempts {
				_, _ = a.DB.Pool().Exec(ctx, `UPDATE public.fluctlight_visual_identity_sessions SET status='awaiting_review',last_error='max_attempts',updated_at=now() WHERE id=$1`, sessionID)
				return map[string]any{"session_id": sessionID, "attempt": attempt, "status": visualIdentityStatusAwaitingReview, "stage": "max_attempts"}, nil
			}
			nextAttempt := attempt + 1
			nextID := visualIdentityAttemptID(sessionID, nextAttempt)
			if err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
				if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_visual_identity_attempts(id,session_id,visual_identity_id,fluctlight_id,attempt_number,status,input_snapshot,renderer_constraints) VALUES($1,$2,$3,$4,$5,'queued',$6,$7) ON CONFLICT(session_id,attempt_number) DO NOTHING`, nextID, sessionID, profileID, fluctlightID, nextAttempt, jsonBytes(map[string]any{"previous_asset_id": candidateAssetID, "previous_patch": completion}), jsonBytes(decodeObject(constraints))); err != nil {
					return err
				}
				if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_visual_identity_sessions SET current_attempt=$2,updated_at=now() WHERE id=$1`, sessionID, nextAttempt); err != nil {
					return err
				}
				return appendVisualIdentityTimelineTx(ctx, tx, sessionID, nextID, fluctlightID, visualIdentityStageRegenerate, "queued", "评审为不是自己，开始下一轮生成", nil, map[string]any{"previous_asset_id": candidateAssetID}, "visual_identity:"+sessionID)
			}); err != nil {
				return nil, err
			}
			return map[string]any{"session_id": sessionID, "attempt": nextAttempt, "status": "waiting", "stage": "regenerating"}, nil
		}
		// Canonical promotion is deliberately a separate transaction and only
		// runs after an accepted decision. Character-sheet generation is queued
		// as another ordinary media intent so user-supplied workflow JSON remains
		// the renderer's responsibility.
		if err := a.promoteVisualIdentityCanonical(ctx, sessionID, attemptID, profileID, fluctlightID, candidateAssetID, decodeObject(inputSnapshot), decodeObject(constraints)); err != nil {
			return nil, err
		}
		return map[string]any{"session_id": sessionID, "attempt": attempt, "status": "waiting", "stage": "character_sheet_queued", "canonical_asset_id": candidateAssetID}, nil
	}
	return map[string]any{"session_id": sessionID, "attempt": attempt, "status": "waiting", "stage": "character_sheet_queued"}, nil
}

func visualIdentityProviderPending(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "provider role visual_identity_") && strings.Contains(message, " unavailable")
}

func (a *App) visualIdentityImageContent(ctx context.Context, assetID string) (map[string]any, error) {
	if a.Storage == nil || strings.TrimSpace(assetID) == "" {
		return nil, nil
	}
	var bucket, objectKey, mime string
	if err := a.DB.Pool().QueryRow(ctx, `SELECT bucket,object_key,mime_type FROM public.media_assets WHERE id=$1 AND status='ready'`, assetID).Scan(&bucket, &objectKey, &mime); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	object, err := a.Storage.GetObject(ctx, bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer object.Close()
	content, err := io.ReadAll(io.LimitReader(object, 8<<20))
	if err != nil {
		return nil, err
	}
	if len(content) == 0 {
		return nil, errors.New("visual_identity_image_empty")
	}
	if mime == "" {
		mime = "image/png"
	}
	return map[string]any{"type": "image_url", "image_url": map[string]any{"url": "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(content)}}, nil
}

func (a *App) recordVisualIdentityStage(ctx context.Context, sessionID, attemptID, fluctlightID, stage, status, summary string, assets []string) error {
	return withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		return appendVisualIdentityTimelineTx(ctx, tx, sessionID, attemptID, fluctlightID, stage, status, summary, assets, nil, "visual_identity:"+sessionID)
	})
}

func (a *App) promoteVisualIdentityCanonical(ctx context.Context, sessionID, attemptID, profileID, fluctlightID, assetID string, identitySnapshot, constraints map[string]any) error {
	return withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var revision int
		if err := tx.QueryRow(ctx, `SELECT current_revision FROM public.fluctlight_visual_identities WHERE id=$1 FOR UPDATE`, profileID).Scan(&revision); err != nil {
			return err
		}
		newRevision := revision + 1
		revisionID := "visual_identity_revision_" + stableDigest(sessionID+":"+attemptID)
		if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_visual_identities SET status='active',current_revision=$2,identity_snapshot=$3,renderer_constraints=$4,canonical_asset_id=$5,active_session_id=$6,updated_at=now() WHERE id=$1`, profileID, newRevision, jsonBytes(identitySnapshot), jsonBytes(constraints), assetID, sessionID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_visual_identity_revisions(id,visual_identity_id,fluctlight_id,revision,base_revision,identity_snapshot,renderer_constraints,canonical_asset_id,adapter_version,source,idempotency_key) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,'visual_identity_workflow',$10) ON CONFLICT(id) DO NOTHING`, revisionID, profileID, fluctlightID, newRevision, revision, jsonBytes(identitySnapshot), jsonBytes(constraints), assetID, visualIdentityAdapterVersion, "visual-identity:"+sessionID+":"+attemptID); err != nil {
			return err
		}
		characterIntentID := "media_intent_" + stableDigest(attemptID+":character-sheet")
		characterWorkflowID := "media_workflow_" + stableDigest(characterIntentID)
		characterRequestID := "media_request_" + stableDigest(characterIntentID)
		characterConcept := map[string]any{"purpose": "visual_identity", "stage": "character_sheet", "canonical_asset_id": assetID, "visual_identity": identitySnapshot, "renderer_constraints": constraints}
		if _, err := tx.Exec(ctx, `INSERT INTO public.media_intents(id,owner_fluctlight_id,kind,mime_type,prompt,provider_request_id,workflow_id,status,revision) VALUES($1,$2,'image','image/png',$3,$4,$5,'pending',0) ON CONFLICT(id) DO NOTHING`, characterIntentID, fluctlightID, jsonString(characterConcept), characterRequestID, characterWorkflowID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload) VALUES($1,$2,'media','media.generation',$3) ON CONFLICT DO NOTHING`, "media_workflow_intent:"+characterIntentID, characterWorkflowID, jsonBytes(map[string]any{"intent_id": characterIntentID, "provider_request_id": characterRequestID, "fluctlight_id": fluctlightID})); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_visual_identity_sessions SET status='character_sheet_pending',character_sheet_media_intent_id=$2,updated_at=now() WHERE id=$1`, sessionID, characterIntentID); err != nil {
			return err
		}
		return appendVisualIdentityTimelineTx(ctx, tx, sessionID, attemptID, fluctlightID, visualIdentityStageCharacterRequested, "queued", "canonical 已确认，等待 character sheet", []string{assetID}, map[string]any{"revision": newRevision, "media_intent_id": characterIntentID}, "visual_identity:"+sessionID)
	})
}
