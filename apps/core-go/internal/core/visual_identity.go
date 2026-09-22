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

// visualIdentityStageOrder is the domain order for timeline projection. All
// events created inside one PostgreSQL transaction receive the same
// transaction timestamp, so ordering by occurred_at alone can put
// image_requested before the preceding seed_ready event. Keep the mapping in
// code next to the stage constants and persist it for deterministic reads.
func visualIdentityStageOrder(stage string) int {
	switch stage {
	case visualIdentityStageSessionCreated:
		return 10
	case visualIdentityStageSeedRequested:
		return 20
	case visualIdentityStageSeedReady:
		return 30
	case visualIdentityStageImageRequested:
		return 40
	case visualIdentityStageImageReady:
		return 50
	case visualIdentityStageVisionRequested:
		return 60
	case visualIdentityStageVisionReady:
		return 70
	case visualIdentityStagePatchRequested:
		return 80
	case visualIdentityStagePatchReady:
		return 90
	case visualIdentityStageRegenerate:
		return 100
	case visualIdentityStageAccepted:
		return 110
	case visualIdentityStageCharacterRequested:
		return 120
	case visualIdentityStageCharacterReady:
		return 130
	case visualIdentityStageCompleted:
		return 140
	case visualIdentityStageFailed:
		return 150
	default:
		// Unknown stages should remain visible but sort after the known
		// lifecycle rather than accidentally interleaving with initialization.
		return 1000
	}
}

// visualIdentityExpectedViews is the canonical three-panel layout requested
// for the character sheet: front, side and back views of one consistent face.
func visualIdentityExpectedViews() []string {
	return []string{"front_full_body", "side_full_body", "back_full_body"}
}

var visualIdentityRequiredCardSections = []string{
	"人物基础资料视觉区",
	"正面、侧面、背面三视图",
	"平静、微笑、侧眸、思考、惊讶、冷漠六种表情",
	"服装拆解视觉区",
	"包、领结、腕表、耳饰、发饰配饰展示",
	"眼睛、嘴唇、发型、校徽细节特写",
	"角色专属色卡",
	"人物简介视觉区",
	"性格关键词视觉区",
	"角色签名视觉区",
}

var visualIdentityRequiredCardSectionsText = strings.Join(visualIdentityRequiredCardSections, "; ")

const visualIdentityPromptTemplate = `角色设定卡
%s
制作完整角色档案卡，白色极简背景，高级时尚杂志排版，3:4 竖图。
画面必须包含以下视觉分区：人物基础资料视觉区、正面/侧面/背面三视图、六种不同表情、服装拆解视觉区、包/领结/腕表/耳饰/发饰配饰展示、眼睛/嘴唇/发型/校徽细节特写、角色专属色卡、人物简介视觉区、性格关键词视觉区、角色签名视觉区。
所有分区只用版式、人物小图、示意图、色块和留白表达，禁止生成任何文字、字母、数字、标题、标签、签名文字或伪文字；可以保留空白信息栏和空白签名线，但不要填入字符。
整体视觉：真实真人、高级商业摄影、柔和棚拍光、高清皮肤纹理、服装材质真实、人物五官统一、三视图保持同一张脸、非动漫、非Q版。`

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
	if appearance == nil {
		return ""
	}
	for _, key := range []string{"chest_cup", "cup_size"} {
		if raw := strings.TrimSpace(stringValue(appearance[key])); raw != "" {
			if value := explicitChestCup(raw); value != "" {
				return value
			}
			// These keys are semantic cup fields. Preserve an unsupported
			// non-empty value so NormalizeChestCup can reject it explicitly
			// instead of silently treating it as "not set".
			return raw
		}
	}
	// Free-form fields such as bust/body_type/build are accepted only when they
	// already contain an explicit cup label. Values like “平胸” or “slim” are
	// ordinary body descriptions and must not be guessed into a LoRA weight.
	for _, key := range []string{"bust", "bust_size", "body_type", "build", "chest"} {
		raw := strings.TrimSpace(stringValue(appearance[key]))
		if candidate := explicitChestCup(raw); candidate != "" {
			return candidate
		}
		// Preserve an explicitly cup-shaped but unsupported label (for
		// example "E cup") so the adapter reports renderer_config_pending.
		upper := strings.ToUpper(raw)
		if strings.Contains(upper, "CUP") || strings.Contains(upper, "罩杯") || strings.Contains(upper, "杯") {
			return raw
		}
	}
	return ""
}

// explicitChestCup accepts the canonical A/B/C/D value and the small set of
// legacy forms emitted by earlier initialization prompts (for example
// "A cup", "A罩杯", or "A cup胸"). It deliberately does not infer a cup from
// ordinary body words such as "slim" or "平胸".
func explicitChestCup(value string) string {
	candidate := strings.ToUpper(strings.TrimSpace(value))
	if candidate == "" {
		return ""
	}
	for _, cup := range []string{"A", "B", "C", "D"} {
		if candidate == cup {
			return cup
		}
		for _, suffix := range []string{" CUP", "CUP", "罩杯", "杯"} {
			if !strings.HasPrefix(candidate, cup+suffix) {
				continue
			}
			remainder := strings.TrimSpace(strings.TrimPrefix(candidate, cup+suffix))
			// Some old visible descriptions used "A cup胸". Keep this
			// compatibility path narrow and explicit; never match arbitrary
			// prose after the label.
			if remainder == "" || remainder == "胸" || remainder == "胸部" {
				return cup
			}
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
		if value := chestCupCandidate(lifeProfile); value != "" {
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
	if stringValue(constraints["chest_cup"]) == "" {
		if physicalTraits := mapValue(lifeProfile["physical_traits"]); len(physicalTraits) > 0 {
			if value := chestCupCandidate(physicalTraits); value != "" {
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
		if stringValue(constraints["chest_cup"]) == "" {
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
	}
	return constraints, nil
}

// normalizeVisualIdentityFoundation establishes the one canonical semantic
// location for the cup label. Initialization models may emit legacy aliases
// while older data is still being migrated, but newly persisted Persona data
// always carries life_profile.appearance.chest_cup.
func normalizeVisualIdentityFoundation(corePersona map[string]any) {
	if corePersona == nil {
		return
	}
	lifeProfile := mapValue(corePersona["life_profile"])
	appearance := mapValue(lifeProfile["appearance"])
	if raw := stringValue(appearance["chest_cup"]); raw != "" {
		// Even the canonical key may contain a legacy decorated value in an
		// older foundation revision. Normalize it in place when it is valid;
		// leave unsupported values untouched so the renderer can surface a
		// configuration-pending error instead of guessing.
		if cup, err := NormalizeChestCup(raw); err == nil {
			appearance["chest_cup"] = cup
		}
	} else {
		sources := []map[string]any{
			appearance,
			lifeProfile,
			mapValue(lifeProfile["physical_traits"]),
			mapValue(mapValue(corePersona["identity"])["appearance"]),
			mapValue(corePersona["identity"]),
			corePersona,
		}
		for _, source := range sources {
			value := chestCupCandidate(source)
			if value == "" {
				continue
			}
			cup, err := NormalizeChestCup(value)
			if err == nil {
				appearance["chest_cup"] = cup
			}
			break
		}
	}
	lifeProfile["appearance"] = appearance
	corePersona["life_profile"] = lifeProfile
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
	normalizeVisualIdentityFoundation(corePersona)
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
		if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_visual_identities SET status='missing',identity_snapshot=$2,renderer_constraints=$3,adapter_version=$4,updated_at=now() WHERE id=$1 AND status IN ('missing','renderer_config_pending') AND current_revision=0`, profileID, jsonBytes(identitySnapshot), jsonBytes(constraints), visualIdentityAdapterVersion); err != nil {
			return "", err
		}
		if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_visual_identity_attempts SET input_snapshot=$2,renderer_constraints=$3,updated_at=now() WHERE visual_identity_id=$1 AND media_intent_id IS NULL AND status='queued'`, profileID, jsonBytes(identitySnapshot), jsonBytes(constraints)); err != nil {
			return "", err
		}
		if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_visual_identity_attempts AS a SET input_snapshot=$2,renderer_constraints=$3,updated_at=now() FROM public.media_intents AS m WHERE a.visual_identity_id=$1 AND a.media_intent_id=m.id AND a.candidate_asset_id IS NULL AND m.provider_job_id IS NULL AND a.status IN ('queued','image_queued','vision_queued','patch_queued')`, profileID, jsonBytes(identitySnapshot), jsonBytes(constraints)); err != nil {
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
	if _, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload) VALUES($1,$2,'lifecycle','visual_identity.initialize',$3) ON CONFLICT DO NOTHING`, "visual_identity_intent:"+sessionID, workflowID, jsonBytes(map[string]any{"intent_id": "visual_identity_intent:" + sessionID, "fluctlight_id": fluctlightID, "session_id": sessionID, "correlation_id": "visual_identity:" + sessionID, "causation_id": sourceFactID})); err != nil {
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
	_, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_visual_identity_timeline(id,session_id,attempt_id,fluctlight_id,stage,stage_order,status,summary,asset_ids,metadata,correlation_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) ON CONFLICT(id) DO NOTHING`, id, sessionID, nullableString(attemptID), fluctlightID, stage, visualIdentityStageOrder(stage), status, visualIdentityBoundedText(summary, 512), jsonBytes(assetIDs), jsonBytes(metadata), visualIdentityBoundedText(correlationID, 128))
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

// visualIdentityPromptFromConcept renders the owner's exact three-panel
// template. The Provider may still be called for ordinary media prompts, but
// it must not rewrite this Visual Identity composition.
func visualIdentityPromptFromConcept(concept map[string]any) string {
	description := visualIdentityCharacterDescription(mapValue(concept["visual_identity"]))
	if description == "" {
		description = "保持同一张脸的角色"
	}
	return fmt.Sprintf(visualIdentityPromptTemplate, description)
}

func visualIdentityPromptFromSnapshot(snapshot VisualIdentitySnapshot) string {
	return visualIdentityPromptFromConcept(map[string]any{
		"visual_identity": map[string]any{
			"identity_snapshot": snapshot.IdentitySnapshot,
		},
	})
}

func visualIdentityCharacterDescription(visualIdentity map[string]any) string {
	if len(visualIdentity) == 0 {
		return ""
	}
	snapshot := mapValue(visualIdentity["identity_snapshot"])
	if len(snapshot) == 0 {
		snapshot = visualIdentity
	}
	identity := mapValue(snapshot["identity"])
	lifeProfile := mapValue(snapshot["life_profile"])
	appearance := mapValue(lifeProfile["appearance"])
	if visible := stringValue(identity["visible_text"]); visible != "" {
		return visible
	}
	parts := make([]string, 0, 8)
	if age := firstVisualIdentityString(identity["age"], appearance["age"]); age != "" {
		parts = append(parts, age)
	}
	if gender := stringValue(identity["gender"]); gender != "" {
		parts = append(parts, gender)
	}
	if nationality := firstVisualIdentityString(identity["nationality"], identity["ethnicity"]); nationality != "" {
		parts = append(parts, nationality)
	}
	if face := firstVisualIdentityString(appearance["face_shape"], identity["face_shape"]); face != "" {
		parts = append(parts, face+" face")
	}
	if body := firstVisualIdentityString(appearance["body_type"], identity["body_type"], identity["build"]); body != "" {
		parts = append(parts, body+" build")
	}
	if hair := firstVisualIdentityString(appearance["hair"], identity["hair"]); hair != "" && hair != "未知" {
		parts = append(parts, hair+" hair")
	}
	if cup := firstVisualIdentityString(appearance["chest_cup"], identity["chest_cup"]); cup != "" {
		parts = append(parts, cup+" cup chest")
	}
	return strings.Join(parts, ", ")
}

func firstVisualIdentityString(values ...any) string {
	for _, value := range values {
		if result := stringValue(value); result != "" {
			return result
		}
		switch typed := value.(type) {
		case int:
			return fmt.Sprintf("%d", typed)
		case int64:
			return fmt.Sprintf("%d", typed)
		case float64:
			if !math.IsNaN(typed) && !math.IsInf(typed, 0) {
				return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.4f", typed), "0"), ".")
			}
		}
	}
	return ""
}

// enforceVisualIdentityTurnaroundPrompt is kept as a small compatibility
// helper for callers that only have a plain character description. It renders
// the exact owner-supplied template and never appends an alternative layout.
func enforceVisualIdentityTurnaroundPrompt(prompt, stage string) string {
	if stage != "seed" && stage != "character_sheet" {
		return strings.TrimSpace(prompt)
	}
	value := strings.TrimSpace(prompt)
	if value == "" {
		return value
	}
	return fmt.Sprintf(visualIdentityPromptTemplate, value)
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
	return a.EnsureVisualIdentityInitializationWithPersona(ctx, fluctlightID, triggerType, sourceFactID, persona)
}

// EnsureVisualIdentityInitializationWithPersona consumes the resolved/frozen
// Core Persona chosen by the Capability context. The transaction still checks
// the live Fluctlight lifecycle before creating a durable workflow intent, but
// it does not silently replace the decision snapshot with a second persona
// read.
func (a *App) EnsureVisualIdentityInitializationWithPersona(ctx context.Context, fluctlightID, triggerType, sourceFactID string, persona map[string]any) (string, error) {
	if len(persona) == 0 {
		return "", errors.New("visual identity persona context is required")
	}
	var sessionID string
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var status string
		if err := tx.QueryRow(ctx, `SELECT status FROM public.fluctlights WHERE id=$1 FOR SHARE`, fluctlightID).Scan(&status); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		if status == "retired" {
			return ErrNotFound
		}
		var err error
		sessionID, err = a.ensureVisualIdentityInitializationTx(ctx, tx, fluctlightID, triggerType, sourceFactID, persona)
		return err
	})
	return sessionID, err
}

func (a *App) EnsureVisualIdentityInitializationWithPersonaTx(ctx context.Context, tx pgx.Tx, fluctlightID, triggerType, sourceFactID string, persona map[string]any) (string, error) {
	return a.ensureVisualIdentityInitializationTx(ctx, tx, fluctlightID, triggerType, sourceFactID, persona)
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
	rows, err := a.DB.Pool().Query(ctx, `SELECT session_id,COALESCE(attempt_id,''),stage,stage_order,status,summary,asset_ids,metadata,correlation_id,occurred_at FROM public.fluctlight_visual_identity_timeline WHERE fluctlight_id=$1 ORDER BY occurred_at,stage_order,id LIMIT 200`, fluctlightID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	timeline := make([]map[string]any, 0)
	for rows.Next() {
		var sessionID, attemptID, stage, status, summary, correlation string
		var stageOrder int
		var assets, metadata []byte
		var occurred time.Time
		if err := rows.Scan(&sessionID, &attemptID, &stage, &stageOrder, &status, &summary, &assets, &metadata, &correlation, &occurred); err != nil {
			return nil, err
		}
		timeline = append(timeline, map[string]any{"session_id": sessionID, "attempt_id": attemptID, "stage": stage, "stage_order": stageOrder, "status": status, "summary": summary, "asset_ids": decodeArray(assets), "metadata": decodeObject(metadata), "correlation_id": correlation, "occurred_at": occurred.Format(time.RFC3339Nano)})
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

// visualIdentityWakeupNeedsInitialization suppresses the repeated missing
// identity notice while an initialization session is already progressing. A
// missing canonical during queued/running/awaiting-review is an in-flight
// workflow state, not a request to start or announce the same workflow again.
func (a *App) visualIdentityWakeupNeedsInitialization(ctx context.Context, fluctlightID string) (bool, error) {
	active, err := a.hasActiveVisualIdentity(ctx, fluctlightID)
	if err != nil {
		return false, err
	}
	if active {
		return false, nil
	}
	var pending bool
	if err := a.DB.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.fluctlight_visual_identity_sessions WHERE fluctlight_id=$1 AND status IN ('queued','running','character_sheet_pending','awaiting_review'))`, fluctlightID).Scan(&pending); err != nil {
		return false, err
	}
	return !pending, nil
}

// ProcessVisualIdentity is the durable workflow entry. Every actionable
// checkpoint now enters the complete formal Visual Identity Agent; Temporal
// supplies only the stable session resume coordinate.
func (a *App) ProcessVisualIdentity(ctx context.Context, sessionID string) (map[string]any, error) {
	state, err := a.loadVisualIdentityAgentState(ctx, strings.TrimSpace(sessionID))
	if err != nil {
		return nil, err
	}
	if state.SessionStatus == "queued" || state.SessionStatus == "running" {
		if err := a.refreshVisualIdentityRendererConstraints(ctx, state.FluctlightID, state.ProfileID); err != nil {
			return nil, err
		}
	}
	result, err := a.RunVisualIdentityAgent(ctx, VisualIdentityAgentInput{SessionID: sessionID})
	if err != nil {
		if visualIdentityProviderPending(err) {
			return map[string]any{
				"session_id": sessionID, "fluctlight_id": state.FluctlightID, "attempt": state.Attempt,
				"status": "waiting", "stage": "provider_config_pending", "accepted": true,
				"error_code": "visual_identity_agent_role_missing",
			}, nil
		}
		return nil, err
	}
	return result.asMap(), nil
}

func visualIdentityJSONEmpty(raw []byte) bool {
	value := strings.TrimSpace(string(raw))
	return value == "" || value == "{}" || value == "null"
}

func visualIdentityProviderPending(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return (strings.Contains(message, "provider role visual_identity_") || strings.Contains(message, "provider role generic_llm")) && strings.Contains(message, " unavailable")
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

func (a *App) refreshVisualIdentityRendererConstraints(ctx context.Context, fluctlightID, profileID string) error {
	var raw []byte
	if err := a.DB.Pool().QueryRow(ctx, `SELECT core_persona FROM public.fluctlights WHERE id=$1 AND status <> 'retired'`, fluctlightID).Scan(&raw); err != nil {
		return err
	}
	var persona map[string]any
	if err := json.Unmarshal(raw, &persona); err != nil {
		return err
	}
	normalizeVisualIdentityFoundation(persona)
	constraints, err := rendererConstraintsForCorePersona(persona)
	if err != nil {
		// Keep a compatibility-created profile diagnosable. Swallowing this
		// error leaves the aggregate as plain "missing" and lets the worker
		// create an image without a valid renderer constraint.
		constraints = map[string]any{
			"schema_version":  visualIdentitySchemaVersion,
			"adapter_version": visualIdentityAdapterVersion,
			"error":           err.Error(),
		}
	}
	identitySnapshot := map[string]any{
		"schema_version": visualIdentitySchemaVersion,
		"identity":       cloneMap(mapValue(persona["identity"])),
		"life_profile":   cloneMap(mapValue(persona["life_profile"])),
	}
	return withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		profileStatus := "missing"
		if stringValue(constraints["error"]) != "" {
			profileStatus = visualIdentityStatusRendererPending
		}
		if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_visual_identities SET status=$2,identity_snapshot=$3,renderer_constraints=$4,adapter_version=$5,updated_at=now() WHERE id=$1 AND status IN ('missing','renderer_config_pending') AND current_revision=0`, profileID, profileStatus, jsonBytes(identitySnapshot), jsonBytes(constraints), visualIdentityAdapterVersion); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_visual_identity_attempts SET input_snapshot=$2,renderer_constraints=$3,updated_at=now() WHERE visual_identity_id=$1 AND media_intent_id IS NULL AND status IN ('queued','image_queued')`, profileID, jsonBytes(identitySnapshot), jsonBytes(constraints)); err != nil {
			return err
		}
		// A compatibility-created attempt may already have a pending media
		// intent from the previous build. It is still safe to repair its frozen
		// renderer snapshot until ComfyUI has accepted a provider job; after
		// that point the attempt must remain immutable.
		if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_visual_identity_attempts AS a SET input_snapshot=$2,renderer_constraints=$3,updated_at=now() FROM public.media_intents AS m WHERE a.visual_identity_id=$1 AND a.media_intent_id=m.id AND a.candidate_asset_id IS NULL AND m.provider_job_id IS NULL AND a.status IN ('queued','image_queued','vision_queued','patch_queued')`, profileID, jsonBytes(identitySnapshot), jsonBytes(constraints)); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT media_intent_id FROM public.fluctlight_visual_identity_attempts WHERE visual_identity_id=$1 AND media_intent_id IS NOT NULL AND candidate_asset_id IS NULL`, profileID)
		if err != nil {
			return err
		}
		mediaIntentIDs := make([]string, 0)
		for rows.Next() {
			var mediaIntentID string
			if err := rows.Scan(&mediaIntentID); err != nil {
				rows.Close()
				return err
			}
			mediaIntentIDs = append(mediaIntentIDs, mediaIntentID)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		// pgx does not allow another statement on the same connection while a
		// result set is open. Close the cursor before locking each media intent;
		// otherwise this recovery path fails with the opaque "conn busy" error
		// immediately after ComfyUI has finished successfully.
		rows.Close()
		for _, mediaIntentID := range mediaIntentIDs {
			var providerJobID, status, prompt string
			if err := tx.QueryRow(ctx, `SELECT COALESCE(provider_job_id,''),status,prompt FROM public.media_intents WHERE id=$1 FOR UPDATE`, mediaIntentID).Scan(&providerJobID, &status, &prompt); err != nil {
				return err
			}
			if providerJobID != "" || (status != "failed" && status != "pending" && status != "retry" && status != "running") {
				continue
			}
			updatedPrompt, promptErr := mergeVisualIdentityRendererConstraints(prompt, constraints)
			if promptErr != nil {
				continue
			}
			if _, err := tx.Exec(ctx, `UPDATE public.media_intents SET prompt=$2,status='pending',revision=revision+1 WHERE id=$1 AND provider_job_id IS NULL`, mediaIntentID, updatedPrompt); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE public.platform_workflow_intents SET status='pending',next_attempt_at=now(),started_at=NULL,completed_at=NULL,last_error=NULL WHERE intent_id=$1`, "media_workflow_intent:"+mediaIntentID); err != nil {
				return err
			}
		}
		return nil
	})
}

func mergeVisualIdentityRendererConstraints(prompt string, constraints map[string]any) (string, error) {
	var concept map[string]any
	if err := json.Unmarshal([]byte(prompt), &concept); err != nil {
		return "", err
	}
	if stringValue(concept["purpose"]) != "visual_identity" {
		return prompt, nil
	}
	concept["renderer_constraints"] = cloneMap(constraints)
	data, err := json.Marshal(concept)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (a *App) promoteVisualIdentityCanonicalTx(ctx context.Context, tx pgx.Tx, sessionID, attemptID, profileID, fluctlightID, assetID string, identitySnapshot, constraints map[string]any) (string, error) {
	var revision int
	if err := tx.QueryRow(ctx, `SELECT current_revision FROM public.fluctlight_visual_identities WHERE id=$1 FOR UPDATE`, profileID).Scan(&revision); err != nil {
		return "", err
	}
	newRevision := revision + 1
	revisionID := "visual_identity_revision_" + stableDigest(sessionID+":"+attemptID)
	if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_visual_identities SET status='active',current_revision=$2,identity_snapshot=$3,renderer_constraints=$4,canonical_asset_id=$5,active_session_id=$6,updated_at=now() WHERE id=$1`, profileID, newRevision, jsonBytes(identitySnapshot), jsonBytes(constraints), assetID, sessionID); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_visual_identity_revisions(id,visual_identity_id,fluctlight_id,revision,base_revision,identity_snapshot,renderer_constraints,canonical_asset_id,adapter_version,source,idempotency_key) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,'visual_identity_agent',$10)`, revisionID, profileID, fluctlightID, newRevision, revision, jsonBytes(identitySnapshot), jsonBytes(constraints), assetID, visualIdentityAdapterVersion, "visual-identity:"+sessionID+":"+attemptID); err != nil {
		return "", err
	}
	characterIntentID := "media_intent_" + stableDigest(attemptID+":character-sheet")
	characterWorkflowID := "media_workflow_" + stableDigest(characterIntentID)
	characterRequestID := "media_request_" + stableDigest(characterIntentID)
	characterConcept := map[string]any{"purpose": "visual_identity", "stage": "character_sheet", "canonical_asset_id": assetID, "visual_identity": identitySnapshot, "renderer_constraints": constraints}
	if _, err := tx.Exec(ctx, `INSERT INTO public.media_intents(id,owner_fluctlight_id,kind,mime_type,prompt,provider_request_id,workflow_id,status,revision) VALUES($1,$2,'image','image/png',$3,$4,$5,'pending',0)`, characterIntentID, fluctlightID, jsonString(characterConcept), characterRequestID, characterWorkflowID); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload) VALUES($1,$2,'media','media.generation',$3)`, "media_workflow_intent:"+characterIntentID, characterWorkflowID, jsonBytes(map[string]any{"intent_id": characterIntentID, "provider_request_id": characterRequestID, "fluctlight_id": fluctlightID})); err != nil {
		return "", err
	}
	command, err := tx.Exec(ctx, `UPDATE public.fluctlight_visual_identity_sessions SET status='character_sheet_pending',character_sheet_media_intent_id=$2,updated_at=now() WHERE id=$1 AND current_attempt=(SELECT attempt_number FROM public.fluctlight_visual_identity_attempts WHERE id=$3) AND status IN ('queued','running')`, sessionID, characterIntentID, attemptID)
	if err != nil {
		return "", err
	}
	if command.RowsAffected() != 1 {
		return "", errors.New("visual_identity_canonical_session_conflict")
	}
	if err := appendVisualIdentityTimelineTx(ctx, tx, sessionID, attemptID, fluctlightID, visualIdentityStageCharacterRequested, "queued", "canonical 已确认，等待 character sheet", []string{assetID}, map[string]any{"revision": newRevision, "media_intent_id": characterIntentID}, "visual_identity:"+sessionID); err != nil {
		return "", err
	}
	return characterIntentID, nil
}
