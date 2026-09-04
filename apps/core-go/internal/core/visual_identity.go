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
// for the character sheet. It intentionally has two front-facing panels (a
// close-up portrait and a full-body standing pose) and one back full-body
// panel; there is no side-view requirement in this workflow.
func visualIdentityExpectedViews() []string {
	return []string{"front_closeup", "front_full_body", "back_full_body"}
}

const visualIdentityPromptTemplate = "Character design sheet, three separate panels on a white background. Left: front close-up portrait of %s. Center: front full body standing straight. Right: back full body from behind. Symmetrical pose, no side view, high resolution concept art."

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
		description = "the same consistent character"
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
	var sessionID string
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var err error
		sessionID, err = a.ensureVisualIdentityInitializationTx(ctx, tx, fluctlightID, triggerType, sourceFactID, persona)
		return err
	})
	return sessionID, err
}

func (a *App) visualIdentityConversationID(ctx context.Context, fluctlightID string) (string, error) {
	var conversationID string
	err := a.DB.Pool().QueryRow(ctx, `SELECT conversation_id FROM public.fluctlight_direct_conversations WHERE fluctlight_actor_id=$1 ORDER BY created_at LIMIT 1`, fluctlightID).Scan(&conversationID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return conversationID, err
}

// backfillVisualIdentityConversationAssets repairs media intents created by
// older builds before Visual Identity media had a conversation target. It is
// safe to run repeatedly because both the conversation message idempotency
// key and media reference use stable IDs.
func (a *App) backfillVisualIdentityConversationAssets(ctx context.Context, fluctlightID string) error {
	conversationID, err := a.visualIdentityConversationID(ctx, fluctlightID)
	if err != nil || conversationID == "" {
		return err
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT id FROM public.media_intents WHERE owner_fluctlight_id=$1 AND status='completed' ORDER BY created_at`, fluctlightID)
	if err != nil {
		return err
	}
	intentIDs := make([]string, 0)
	for rows.Next() {
		var intentID string
		if err := rows.Scan(&intentID); err != nil {
			rows.Close()
			return err
		}
		intentIDs = append(intentIDs, intentID)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	for _, intentID := range intentIDs {
		intent, intentErr := a.readMediaIntent(ctx, intentID)
		if intentErr != nil {
			return intentErr
		}
		var concept map[string]any
		if json.Unmarshal([]byte(intent.Prompt), &concept) != nil || stringValue(concept["purpose"]) != "visual_identity" {
			continue
		}
		assetID := "asset_" + intent.ID
		var ready bool
		if err := a.DB.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.media_assets WHERE id=$1 AND owner_fluctlight_id=$2 AND status='ready')`, assetID, fluctlightID).Scan(&ready); err != nil {
			return err
		}
		if !ready {
			continue
		}
		if intent.ConversationID == nil {
			if _, err := a.DB.Pool().Exec(ctx, `UPDATE public.media_intents SET conversation_id=$2 WHERE id=$1 AND conversation_id IS NULL`, intent.ID, conversationID); err != nil {
				return err
			}
			intent.ConversationID = &conversationID
		}
		if err := a.publishMediaAsset(ctx, intent, assetID); err != nil {
			return err
		}
	}
	return nil
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
		if sessionStatus == "completed" {
			if err := a.backfillVisualIdentityConversationAssets(ctx, fluctlightID); err != nil {
				return nil, err
			}
		}
		return map[string]any{"session_id": sessionID, "fluctlight_id": fluctlightID, "status": sessionStatus, "attempt": attempt}, nil
	}
	if maxAttempts < 1 {
		maxAttempts = visualIdentityMaxAttempts
	}
	if sessionStatus == "queued" || sessionStatus == "running" {
		// Re-read the current Core Persona on every checkpoint so a profile
		// created by an earlier build can pick up explicit identity.body_type,
		// identity.chest, or identity.build cup data before its next media intent
		// is frozen. Existing media intents remain immutable.
		if err := a.refreshVisualIdentityRendererConstraints(ctx, fluctlightID, profileID); err != nil {
			return nil, err
		}
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
		characterIntent, intentErr := a.readMediaIntent(ctx, characterSheetIntentID)
		if intentErr != nil {
			return nil, intentErr
		}
		if characterIntent.ConversationID == nil {
			if conversationID, conversationErr := a.visualIdentityConversationID(ctx, fluctlightID); conversationErr != nil {
				return nil, conversationErr
			} else if conversationID != "" {
				if _, updateErr := a.DB.Pool().Exec(ctx, `UPDATE public.media_intents SET conversation_id=$2 WHERE id=$1 AND conversation_id IS NULL`, characterSheetIntentID, conversationID); updateErr != nil {
					return nil, updateErr
				}
				characterIntent.ConversationID = &conversationID
			}
		}
		if characterIntent.ConversationID != nil {
			if publishErr := a.publishMediaAsset(ctx, characterIntent, assetID); publishErr != nil {
				return nil, publishErr
			}
		}
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
		_ = a.recordVisualIdentityStage(ctx, sessionID, attemptID, fluctlightID, visualIdentityStageSeedRequested, "running", "正在生成“自己”的角色设计图文本提示", nil)
		identity, err := a.readVisualIdentity(ctx, fluctlightID)
		if err != nil {
			return nil, err
		}
		// The owner supplied the exact three-panel template. Seed creation only
		// fills {人物描述}; it is intentionally deterministic and does not let a
		// generic LLM rewrite the layout or reintroduce a side view.
		seedPrompt = visualIdentityPromptFromSnapshot(identity)
		if seedPrompt == "" {
			return map[string]any{"session_id": sessionID, "status": visualIdentityStatusAwaitingReview, "stage": "awaiting_input", "error_code": "seed_prompt_empty"}, nil
		}
		mediaIntentID = "media_intent_" + stableDigest(attemptID+":seed")
		workflowID := "media_workflow_" + stableDigest(mediaIntentID)
		requestID := "media_request_" + stableDigest(mediaIntentID)
		conversationID, conversationErr := a.visualIdentityConversationID(ctx, fluctlightID)
		if conversationErr != nil {
			return nil, conversationErr
		}
		concept := map[string]any{"purpose": "visual_identity", "stage": "seed", "render_intent": "character_design_sheet", "subject_count": 1, "views": visualIdentityExpectedViews(), "prompt": seedPrompt, "visual_identity": identity, "renderer_constraints": decodeObject(constraints)}
		err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_visual_identity_attempts SET seed_prompt=$2,media_intent_id=$3,status='image_queued',updated_at=now() WHERE id=$1 AND seed_prompt IS NULL`, attemptID, seedPrompt, mediaIntentID); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO public.media_intents(id,owner_fluctlight_id,kind,mime_type,prompt,provider_request_id,workflow_id,conversation_id,status,revision) VALUES($1,$2,'image','image/png',$3,$4,$5,$6,'pending',0) ON CONFLICT(id) DO NOTHING`, mediaIntentID, fluctlightID, jsonString(concept), requestID, workflowID, nullableString(conversationID)); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload) VALUES($1,$2,'media','media.generation',$3) ON CONFLICT DO NOTHING`, "media_workflow_intent:"+mediaIntentID, workflowID, jsonBytes(map[string]any{"intent_id": mediaIntentID, "provider_request_id": requestID, "fluctlight_id": fluctlightID})); err != nil {
				return err
			}
			if err := appendVisualIdentityTimelineTx(ctx, tx, sessionID, attemptID, fluctlightID, visualIdentityStageImageRequested, "queued", "角色设计图纯文生图已排队", nil, map[string]any{"media_intent_id": mediaIntentID}, workflowID); err != nil {
				return err
			}
			return appendVisualIdentityTimelineTx(ctx, tx, sessionID, attemptID, fluctlightID, visualIdentityStageSeedReady, "completed", "角色设计图文本提示已生成，等待图片工作流", nil, map[string]any{"media_intent_id": mediaIntentID}, workflowID)
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
		candidateIntent, intentErr := a.readMediaIntent(ctx, mediaIntentID)
		if intentErr != nil {
			return nil, intentErr
		}
		if candidateIntent.ConversationID == nil {
			if conversationID, conversationErr := a.visualIdentityConversationID(ctx, fluctlightID); conversationErr != nil {
				return nil, conversationErr
			} else if conversationID != "" {
				if _, updateErr := a.DB.Pool().Exec(ctx, `UPDATE public.media_intents SET conversation_id=$2 WHERE id=$1 AND conversation_id IS NULL`, mediaIntentID, conversationID); updateErr != nil {
					return nil, updateErr
				}
				candidateIntent.ConversationID = &conversationID
			}
		}
		if candidateIntent.ConversationID != nil {
			if publishErr := a.publishMediaAsset(ctx, candidateIntent, candidateAssetID); publishErr != nil {
				return nil, publishErr
			}
		}
		if _, err := a.DB.Pool().Exec(ctx, `UPDATE public.fluctlight_visual_identity_attempts SET candidate_asset_id=$2,status='vision_queued',updated_at=now() WHERE id=$1 AND candidate_asset_id IS NULL`, attemptID, candidateAssetID); err != nil {
			return nil, err
		}
		_ = a.recordVisualIdentityStage(ctx, sessionID, attemptID, fluctlightID, visualIdentityStageImageReady, "completed", "候选角色设计图已生成", []string{candidateAssetID})
	}
	if visualIdentityJSONEmpty(visionResult) {
		_ = a.recordVisualIdentityStage(ctx, sessionID, attemptID, fluctlightID, visualIdentityStageVisionRequested, "running", "正在进行视觉理解", []string{candidateAssetID})
		imageContent, imageErr := a.visualIdentityImageContent(ctx, candidateAssetID)
		if imageErr != nil {
			return nil, imageErr
		}
		visionUserContent := []any{map[string]any{"type": "text", "text": jsonString(map[string]any{"asset_id": candidateAssetID, "render_intent": "character_design_sheet", "expected_subject": "one_human_character", "expected_views": visualIdentityExpectedViews(), "panel_layout": map[string]string{"left": "front_closeup_portrait", "center": "front_full_body_standing", "right": "back_full_body"}, "visual_identity": decodeObject(inputSnapshot), "renderer_constraints": decodeObject(constraints)})}}
		if imageContent != nil {
			visionUserContent = append(visionUserContent, imageContent)
		}
		completion, err := a.Provider.StructuredWithSchema(ctx, "visual_identity_vision", []map[string]any{{"role": "system", "content": "Inspect the supplied candidate image for visual identity continuity. The required target is one character design sheet with exactly three separate panels on a white background: left front close-up portrait, center front full-body standing straight, right back full-body from behind. There is explicitly no side-view panel. If the image is an art photo, abstract silhouette, landscape, object-only image, missing a person, missing any required panel, or shows a side view instead of the center front full body, report a low identity_match and make that mismatch explicit in observations. Return bounded structured observations only."}, {"role": "user", "content": visionUserContent}}, "visual_identity_vision_response", visualIdentityVisionResponseSchema(), false)
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
		completion, err := a.Provider.StructuredWithSchema(ctx, "visual_identity_patch", []map[string]any{{"role": "system", "content": "Review the candidate against the visual identity and return accepted or regenerate. Acceptance is allowed only for one character design sheet with exactly three separate panels on a white background: left front close-up portrait, center front full body standing straight, right back full body from behind. There is explicitly no side-view panel. An art photo, abstract silhouette, landscape, object-only image, missing person, missing panel, or side-view substitution must be decision=regenerate. Preserve the explicit decision and a structured patch."}, {"role": "user", "content": jsonString(map[string]any{"stage": "review", "render_intent": "character_design_sheet", "expected_subject": "one_human_character", "expected_views": visualIdentityExpectedViews(), "panel_layout": map[string]string{"left": "front_closeup_portrait", "center": "front_full_body_standing", "right": "back_full_body"}, "visual_identity": decodeObject(inputSnapshot), "renderer_constraints": decodeObject(constraints), "vision": decodeObject(visionResult), "candidate_asset_id": candidateAssetID})}}, "visual_identity_patch_response", visualIdentityPatchResponseSchema(), false)
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

func visualIdentityJSONEmpty(raw []byte) bool {
	value := strings.TrimSpace(string(raw))
	return value == "" || value == "{}" || value == "null"
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
