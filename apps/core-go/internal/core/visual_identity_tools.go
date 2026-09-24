package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

const (
	// CapabilitySurfaceVisualIdentity is intentionally private to the complete
	// Visual Identity Agent. These tools must not leak into conversation,
	// WakeUp, autonomy, native-cognition, or reflection catalogs.
	CapabilitySurfaceVisualIdentity CapabilitySurface = "visual_identity"

	visualIdentityGenerateCandidateCapabilityName = "visual_identity.generate_candidate"
	visualIdentityCommitReviewCapabilityName      = "visual_identity.commit_review"
	visualIdentityFinalizeCapabilityName          = "visual_identity.finalize"
)

type visualIdentityAgentCapability struct {
	service visualIdentityAgentToolService
	name    string
}

type visualIdentityAgentToolService interface {
	executeVisualIdentityGenerateCandidateTx(context.Context, pgx.Tx, CapabilityInvocation, DirectToolTarget, string) (CapabilityResult, error)
	executeVisualIdentityCommitReviewTx(context.Context, pgx.Tx, CapabilityInvocation, DirectToolTarget, string) (CapabilityResult, error)
	executeVisualIdentityFinalizeTx(context.Context, pgx.Tx, CapabilityInvocation, DirectToolTarget, string) (CapabilityResult, error)
}

func visualIdentityAgentCapabilities(app *App) []Capability {
	var service visualIdentityAgentToolService
	if app != nil {
		service = app
	}
	return []Capability{
		visualIdentityAgentCapability{service: service, name: visualIdentityGenerateCandidateCapabilityName},
		visualIdentityAgentCapability{service: service, name: visualIdentityCommitReviewCapabilityName},
		visualIdentityAgentCapability{service: service, name: visualIdentityFinalizeCapabilityName},
	}
}

func (c visualIdentityAgentCapability) Definition() CapabilityDefinition {
	switch c.name {
	case visualIdentityGenerateCandidateCapabilityName:
		return visualIdentityGenerateCandidateCapabilityDefinition()
	case visualIdentityCommitReviewCapabilityName:
		return visualIdentityCommitReviewCapabilityDefinition()
	case visualIdentityFinalizeCapabilityName:
		return visualIdentityFinalizeCapabilityDefinition()
	default:
		return CapabilityDefinition{}
	}
}

func (c visualIdentityAgentCapability) RequiredContext() []ContextSlot { return nil }

func (c visualIdentityAgentCapability) Execute(_ context.Context, invocation CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	err := newCapabilityError("visual_identity_direct_execution_required", false, errors.New("visual identity tools require an authorized session target"))
	return failedCapabilityResultDetail(invocation, "visual_identity_direct_execution_required", false, err.Error()), err
}

// ExecuteTx exists so the canonical runtime can classify the capability as a
// transactional mutation. Production direct and Agent calls both enter through
// App.ExecuteTool, which supplies the authorized session target to
// ExecuteDirectTx.
func (c visualIdentityAgentCapability) ExecuteTx(_ context.Context, _ pgx.Tx, invocation CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	err := newCapabilityError("visual_identity_target_required", false, errors.New("visual identity session target is required"))
	return failedCapabilityResultDetail(invocation, "visual_identity_target_required", false, err.Error()), err
}

func (c visualIdentityAgentCapability) ExecuteDirectTx(ctx context.Context, tx pgx.Tx, invocation CapabilityInvocation, _ CapabilityContext, target DirectToolTarget) (CapabilityResult, error) {
	if c.service == nil {
		err := errors.New("visual identity service unavailable")
		return failedCapabilityResultDetail(invocation, "visual_identity_unavailable", true, err.Error()), err
	}
	sessionID, err := validateVisualIdentityToolTarget(invocation, target)
	if err != nil {
		return failedCapabilityResultDetail(invocation, "visual_identity_target_invalid", false, err.Error()), newCapabilityError("visual_identity_target_invalid", false, err)
	}
	switch c.name {
	case visualIdentityGenerateCandidateCapabilityName:
		return c.service.executeVisualIdentityGenerateCandidateTx(ctx, tx, invocation, target, sessionID)
	case visualIdentityCommitReviewCapabilityName:
		return c.service.executeVisualIdentityCommitReviewTx(ctx, tx, invocation, target, sessionID)
	case visualIdentityFinalizeCapabilityName:
		return c.service.executeVisualIdentityFinalizeTx(ctx, tx, invocation, target, sessionID)
	default:
		err := fmt.Errorf("unknown visual identity tool %q", c.name)
		return failedCapabilityResultDetail(invocation, "capability_not_found", false, err.Error()), err
	}
}

var (
	_ Capability              = visualIdentityAgentCapability{}
	_ TransactionalCapability = visualIdentityAgentCapability{}
	_ DirectToolCapability    = visualIdentityAgentCapability{}
)

func visualIdentityGenerateCandidateCapabilityDefinition() CapabilityDefinition {
	return CapabilityDefinition{
		Name: visualIdentityGenerateCandidateCapabilityName, Version: "v1", Type: CapabilityTypeAction,
		Description: "Create or replay the durable media intent for the current Visual Identity candidate attempt.",
		Surfaces:    []CapabilitySurface{CapabilitySurfaceVisualIdentity}, FailurePolicy: FailurePolicyOptionalInternal,
		InputSchema: map[string]any{
			"type": "object", "additionalProperties": false,
			"required":   []any{"reason"},
			"properties": map[string]any{"reason": map[string]any{"type": "string", "enum": []any{"initial", "regenerate"}}},
		},
		OutputSchema:    visualIdentityToolOutputSchema(),
		SideEffectClass: "native_projection", SuccessBoundary: "durable_candidate_media_intent_created",
		CompletionBoundary: "candidate_asset_ready", OutcomeReferenceField: "media_intent_id",
		ConcurrencyClass: "exclusive", SupportsCancel: true, SupportsRetry: true,
	}
}

func visualIdentityCommitReviewCapabilityDefinition() CapabilityDefinition {
	return CapabilityDefinition{
		Name: visualIdentityCommitReviewCapabilityName, Version: "v1", Type: CapabilityTypeAction,
		Description: "Persist the real-image review, then regenerate or promote the current candidate and queue its character sheet.",
		Surfaces:    []CapabilitySurface{CapabilitySurfaceVisualIdentity}, FailurePolicy: FailurePolicyOptionalInternal,
		InputSchema: map[string]any{
			"type": "object", "additionalProperties": false,
			"required": []any{"decision", "identity_match", "confidence", "observations", "missing_sections", "summary", "feedback"},
			"properties": map[string]any{
				"decision":         map[string]any{"type": "string", "enum": []any{"accepted", "regenerate"}},
				"identity_match":   map[string]any{"type": "number", "minimum": 0.0, "maximum": 1.0},
				"confidence":       map[string]any{"type": "number", "minimum": 0.0, "maximum": 1.0},
				"observations":     map[string]any{"type": "array", "minItems": 1, "maxItems": 24, "items": map[string]any{"type": "string", "minLength": 1, "maxLength": 500}},
				"missing_sections": map[string]any{"type": "array", "maxItems": 16, "items": map[string]any{"type": "string", "minLength": 1, "maxLength": 200}},
				"summary":          map[string]any{"type": "string", "minLength": 1, "maxLength": 1000},
				"feedback":         map[string]any{"type": "string", "maxLength": 2000},
			},
		},
		OutputSchema:    visualIdentityToolOutputSchema(),
		SideEffectClass: "native_projection", SuccessBoundary: "visual_identity_review_committed",
		ConcurrencyClass: "exclusive", SupportsCancel: false, SupportsRetry: true,
	}
}

func visualIdentityFinalizeCapabilityDefinition() CapabilityDefinition {
	return CapabilityDefinition{
		Name: visualIdentityFinalizeCapabilityName, Version: "v1", Type: CapabilityTypeAction,
		Description: "Finalize the accepted Visual Identity after its durable character-sheet asset is ready.",
		Surfaces:    []CapabilitySurface{CapabilitySurfaceVisualIdentity}, FailurePolicy: FailurePolicyOptionalInternal,
		InputSchema:     map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{}},
		OutputSchema:    visualIdentityToolOutputSchema(),
		SideEffectClass: "native_projection", SuccessBoundary: "visual_identity_character_sheet_committed",
		ConcurrencyClass: "exclusive", SupportsCancel: false, SupportsRetry: true,
	}
}

func visualIdentityToolOutputSchema() map[string]any {
	return map[string]any{
		"type": "object", "additionalProperties": false,
		"required": []any{"session_id", "attempt", "status"},
		"properties": map[string]any{
			"session_id":                      map[string]any{"type": "string", "minLength": 1, "maxLength": 128},
			"attempt":                         map[string]any{"type": "integer", "minimum": 1},
			"next_attempt":                    map[string]any{"type": "integer", "minimum": 1},
			"status":                          map[string]any{"type": "string", "enum": []any{"pending", "queued", "running", "retry", "completed", "regenerating", "awaiting_review", "character_sheet_pending", "failed", "cancelled"}},
			"media_intent_id":                 map[string]any{"type": "string", "minLength": 1, "maxLength": 128},
			"task_id":                         map[string]any{"type": "string", "minLength": 1, "maxLength": 128},
			"candidate_asset_id":              map[string]any{"type": "string", "minLength": 1, "maxLength": 128},
			"canonical_asset_id":              map[string]any{"type": "string", "minLength": 1, "maxLength": 128},
			"character_sheet_media_intent_id": map[string]any{"type": "string", "minLength": 1, "maxLength": 128},
			"character_sheet_asset_id":        map[string]any{"type": "string", "minLength": 1, "maxLength": 128},
			"decision":                        map[string]any{"type": "string", "enum": []any{"accepted", "regenerate"}},
			"replayed":                        map[string]any{"type": "boolean"},
		},
	}
}

func validateVisualIdentityToolTarget(invocation CapabilityInvocation, target DirectToolTarget) (string, error) {
	if strings.TrimSpace(target.Kind) != "visual_identity_session" || strings.TrimSpace(target.Ref) == "" {
		return "", errors.New("visual identity session target is required")
	}
	if strings.TrimSpace(target.FluctlightID) == "" || strings.TrimSpace(target.FluctlightID) != strings.TrimSpace(invocation.Metadata.FluctlightID) {
		return "", errors.New("visual identity Fluctlight target does not match invocation")
	}
	return strings.TrimSpace(target.Ref), nil
}

type visualIdentityToolSession struct {
	SessionID              string
	FluctlightID           string
	ProfileID              string
	Status                 string
	Attempt                int
	MaxAttempts            int
	CharacterMediaIntentID string
	AttemptID              string
	SeedPrompt             string
	MediaIntentID          string
	CandidateAssetID       string
	Decision               string
	InputSnapshot          map[string]any
	RendererConstraints    map[string]any
	PatchResult            map[string]any
}

func loadVisualIdentityToolSessionTx(ctx context.Context, tx pgx.Tx, sessionID string) (visualIdentityToolSession, error) {
	var result visualIdentityToolSession
	if err := tx.QueryRow(ctx, `SELECT id,fluctlight_id,visual_identity_id,status,current_attempt,max_attempts,COALESCE(character_sheet_media_intent_id,'') FROM public.fluctlight_visual_identity_sessions WHERE id=$1 FOR UPDATE`, sessionID).Scan(
		&result.SessionID, &result.FluctlightID, &result.ProfileID, &result.Status, &result.Attempt, &result.MaxAttempts, &result.CharacterMediaIntentID,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return result, ErrNotFound
		}
		return result, err
	}
	result.AttemptID = visualIdentityAttemptID(sessionID, result.Attempt)
	var inputSnapshot, constraints, patch []byte
	if err := tx.QueryRow(ctx, `SELECT COALESCE(seed_prompt,''),COALESCE(media_intent_id,''),COALESCE(candidate_asset_id,''),COALESCE(decision,''),input_snapshot,renderer_constraints,patch_result FROM public.fluctlight_visual_identity_attempts WHERE id=$1 FOR UPDATE`, result.AttemptID).Scan(
		&result.SeedPrompt, &result.MediaIntentID, &result.CandidateAssetID, &result.Decision, &inputSnapshot, &constraints, &patch,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return result, ErrNotFound
		}
		return result, err
	}
	result.InputSnapshot = decodeObject(inputSnapshot)
	result.RendererConstraints = decodeObject(constraints)
	result.PatchResult = decodeObject(patch)
	if result.MaxAttempts < 1 {
		result.MaxAttempts = visualIdentityMaxAttempts
	}
	return result, nil
}

func (a *App) executeVisualIdentityGenerateCandidateTx(ctx context.Context, tx pgx.Tx, invocation CapabilityInvocation, target DirectToolTarget, sessionID string) (CapabilityResult, error) {
	session, err := loadVisualIdentityToolSessionTx(ctx, tx, sessionID)
	if err != nil {
		return failedCapabilityResultDetail(invocation, "visual_identity_session_load_failed", true, err.Error()), err
	}
	if session.FluctlightID != target.FluctlightID {
		err := errors.New("visual identity session is outside the authorized Fluctlight")
		return failedCapabilityResultDetail(invocation, "visual_identity_target_invalid", false, err.Error()), newCapabilityError("visual_identity_target_invalid", false, err)
	}
	if session.Status == "completed" {
		return visualIdentityToolResult(invocation, "completed", map[string]any{"session_id": sessionID, "attempt": session.Attempt, "status": "completed", "replayed": true}), nil
	}
	if session.Status != "queued" && session.Status != "running" {
		return visualIdentityBusinessRejection(invocation, "visual_identity_session_not_generatable", sessionID, session.Attempt, session.Status), nil
	}
	if rendererError := stringValue(session.RendererConstraints["error"]); rendererError != "" {
		if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_visual_identity_sessions SET status='awaiting_review',last_error=$2,updated_at=now() WHERE id=$1`, sessionID, rendererError); err != nil {
			return failedCapabilityResultDetail(invocation, "visual_identity_renderer_pending_write_failed", true, err.Error()), err
		}
		if err := appendVisualIdentityTimelineTx(ctx, tx, sessionID, session.AttemptID, session.FluctlightID, visualIdentityStageFailed, visualIdentityStatusRendererPending, "等待有效的胸部渲染配置", nil, map[string]any{"error_code": rendererError}, "visual_identity:"+sessionID); err != nil {
			return failedCapabilityResultDetail(invocation, "visual_identity_timeline_failed", true, err.Error()), err
		}
		result := visualIdentityBusinessRejection(invocation, "renderer_config_pending", sessionID, session.Attempt, "awaiting_review")
		result.Output = map[string]any{"session_id": sessionID, "attempt": session.Attempt, "status": "awaiting_review"}
		return result, nil
	}
	if session.MediaIntentID != "" {
		var mediaStatus, workflowID string
		if err := tx.QueryRow(ctx, `SELECT status,workflow_id FROM public.media_intents WHERE id=$1`, session.MediaIntentID).Scan(&mediaStatus, &workflowID); err != nil {
			return failedCapabilityResultDetail(invocation, "visual_identity_media_intent_load_failed", true, err.Error()), err
		}
		status := "accepted"
		if mediaStatus == "completed" {
			status = "completed"
		} else if mediaStatus == "failed" {
			return CapabilityResult{
				CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "failed", ErrorCode: "visual_identity_candidate_media_failed", Retryable: true,
				Output:            map[string]any{"session_id": sessionID, "attempt": session.Attempt, "status": "failed", "media_intent_id": session.MediaIntentID, "task_id": workflowID, "replayed": true},
				ProviderRequestID: invocation.ProviderRequestID, CorrelationID: "visual_identity:" + sessionID,
			}, nil
		}
		return visualIdentityToolResult(invocation, status, map[string]any{"session_id": sessionID, "attempt": session.Attempt, "status": mediaStatus, "media_intent_id": session.MediaIntentID, "task_id": workflowID, "replayed": true}), nil
	}
	identitySnapshot := cloneMap(session.InputSnapshot)
	if len(identitySnapshot) == 0 {
		var raw []byte
		if err := tx.QueryRow(ctx, `SELECT identity_snapshot FROM public.fluctlight_visual_identities WHERE id=$1`, session.ProfileID).Scan(&raw); err != nil {
			return failedCapabilityResultDetail(invocation, "visual_identity_profile_load_failed", true, err.Error()), err
		}
		identitySnapshot = decodeObject(raw)
	}
	var corePersonaRaw []byte
	if err := tx.QueryRow(ctx, `SELECT core_persona FROM public.fluctlights WHERE id=$1`, session.FluctlightID).Scan(&corePersonaRaw); err == nil && len(corePersonaRaw) > 0 {
		enrichIdentitySnapshotWithPersona(identitySnapshot, decodeObject(corePersonaRaw))
	}
	seedPrompt := visualIdentityPromptFromConcept(map[string]any{"visual_identity": map[string]any{"identity_snapshot": identitySnapshot}})
	if strings.TrimSpace(seedPrompt) == "" {
		return visualIdentityBusinessRejection(invocation, "seed_prompt_empty", sessionID, session.Attempt, "awaiting_review"), nil
	}
	mediaIntentID := "media_intent_" + stableDigest(session.AttemptID+":seed")
	workflowID := "media_workflow_" + stableDigest(mediaIntentID)
	requestID := "media_request_" + stableDigest(mediaIntentID)
	concept := map[string]any{
		"purpose": "visual_identity", "stage": "seed", "render_intent": "character_design_sheet",
		"subject_count": 1, "views": visualIdentityExpectedViews(), "prompt": seedPrompt,
		"visual_identity":      map[string]any{"identity_snapshot": identitySnapshot},
		"renderer_constraints": cloneMap(session.RendererConstraints),
	}
	if err := appendVisualIdentityTimelineTx(ctx, tx, sessionID, session.AttemptID, session.FluctlightID, visualIdentityStageSeedRequested, "running", "Visual Identity Agent 请求候选角色设计图", nil, nil, "visual_identity:"+sessionID); err != nil {
		return failedCapabilityResultDetail(invocation, "visual_identity_timeline_failed", true, err.Error()), err
	}
	command, err := tx.Exec(ctx, `UPDATE public.fluctlight_visual_identity_attempts SET seed_prompt=$2,media_intent_id=$3,status='image_queued',updated_at=now() WHERE id=$1 AND media_intent_id IS NULL`, session.AttemptID, seedPrompt, mediaIntentID)
	if err != nil {
		return failedCapabilityResultDetail(invocation, "visual_identity_attempt_update_failed", true, err.Error()), err
	}
	if command.RowsAffected() != 1 {
		return failedCapabilityResultDetail(invocation, "visual_identity_attempt_conflict", true, ErrConflict.Error()), ErrConflict
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.media_intents(id,owner_fluctlight_id,kind,mime_type,prompt,provider_request_id,workflow_id,status,revision) VALUES($1,$2,'image','image/png',$3,$4,$5,'pending',0) ON CONFLICT(id) DO NOTHING`, mediaIntentID, session.FluctlightID, jsonString(concept), requestID, workflowID); err != nil {
		return failedCapabilityResultDetail(invocation, "visual_identity_media_intent_failed", true, err.Error()), err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload) VALUES($1,$2,'media','media.generation',$3) ON CONFLICT DO NOTHING`, "media_workflow_intent:"+mediaIntentID, workflowID, jsonBytes(map[string]any{"intent_id": mediaIntentID, "provider_request_id": requestID, "fluctlight_id": session.FluctlightID})); err != nil {
		return failedCapabilityResultDetail(invocation, "visual_identity_workflow_intent_failed", true, err.Error()), err
	}
	if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_visual_identity_sessions SET status='running',updated_at=now() WHERE id=$1 AND status IN ('queued','running')`, sessionID); err != nil {
		return failedCapabilityResultDetail(invocation, "visual_identity_session_update_failed", true, err.Error()), err
	}
	if err := appendVisualIdentityTimelineTx(ctx, tx, sessionID, session.AttemptID, session.FluctlightID, visualIdentityStageSeedReady, "completed", "角色设计图提示已冻结", nil, map[string]any{"media_intent_id": mediaIntentID}, workflowID); err != nil {
		return failedCapabilityResultDetail(invocation, "visual_identity_timeline_failed", true, err.Error()), err
	}
	if err := appendVisualIdentityTimelineTx(ctx, tx, sessionID, session.AttemptID, session.FluctlightID, visualIdentityStageImageRequested, "queued", "角色设计图生成已排队", nil, map[string]any{"media_intent_id": mediaIntentID}, workflowID); err != nil {
		return failedCapabilityResultDetail(invocation, "visual_identity_timeline_failed", true, err.Error()), err
	}
	return visualIdentityToolResult(invocation, "accepted", map[string]any{"session_id": sessionID, "attempt": session.Attempt, "status": "pending", "media_intent_id": mediaIntentID, "task_id": workflowID, "replayed": false}), nil
}

func (a *App) executeVisualIdentityCommitReviewTx(ctx context.Context, tx pgx.Tx, invocation CapabilityInvocation, target DirectToolTarget, sessionID string) (CapabilityResult, error) {
	var review map[string]any
	if err := json.Unmarshal(invocation.Arguments, &review); err != nil {
		return failedCapabilityResultDetail(invocation, "visual_identity_review_invalid", false, err.Error()), newCapabilityError("visual_identity_review_invalid", false, err)
	}
	decision := stringValue(review["decision"])
	if decision == "accepted" && len(arrayValue(review["missing_sections"])) > 0 {
		err := errors.New("accepted review cannot report missing required sections")
		return failedCapabilityResultDetail(invocation, "visual_identity_review_inconsistent", false, err.Error()), newCapabilityError("visual_identity_review_inconsistent", false, err)
	}
	session, err := loadVisualIdentityToolSessionTx(ctx, tx, sessionID)
	if err != nil {
		return failedCapabilityResultDetail(invocation, "visual_identity_session_load_failed", true, err.Error()), err
	}
	if session.FluctlightID != target.FluctlightID {
		err := errors.New("visual identity session is outside the authorized Fluctlight")
		return failedCapabilityResultDetail(invocation, "visual_identity_target_invalid", false, err.Error()), newCapabilityError("visual_identity_target_invalid", false, err)
	}
	if session.Decision != "" {
		if session.Decision != decision || stableDigest(jsonString(session.PatchResult)) != stableDigest(jsonString(review)) {
			return failedCapabilityResultDetail(invocation, "visual_identity_review_conflict", false, ErrConflict.Error()), newCapabilityError("visual_identity_review_conflict", false, ErrConflict)
		}
		return visualIdentityToolResult(invocation, "completed", map[string]any{"session_id": sessionID, "attempt": session.Attempt, "status": session.Status, "decision": session.Decision, "candidate_asset_id": session.CandidateAssetID, "replayed": true}), nil
	}
	if session.Status != "running" && session.Status != "queued" {
		return visualIdentityBusinessRejection(invocation, "visual_identity_session_not_reviewable", sessionID, session.Attempt, session.Status), nil
	}
	if session.MediaIntentID == "" {
		return visualIdentityBusinessRejection(invocation, "visual_identity_candidate_missing", sessionID, session.Attempt, "awaiting_review"), nil
	}
	var mediaStatus string
	if err := tx.QueryRow(ctx, `SELECT status FROM public.media_intents WHERE id=$1`, session.MediaIntentID).Scan(&mediaStatus); err != nil {
		return failedCapabilityResultDetail(invocation, "visual_identity_media_intent_load_failed", true, err.Error()), err
	}
	if mediaStatus != "completed" {
		return visualIdentityBusinessRejection(invocation, "visual_identity_candidate_not_ready", sessionID, session.Attempt, mediaStatus), nil
	}
	assetID := session.CandidateAssetID
	if assetID == "" {
		assetID = "asset_" + session.MediaIntentID
	}
	var assetStatus, assetOwner string
	if err := tx.QueryRow(ctx, `SELECT status,owner_fluctlight_id FROM public.media_assets WHERE id=$1`, assetID).Scan(&assetStatus, &assetOwner); err != nil {
		return failedCapabilityResultDetail(invocation, "visual_identity_candidate_asset_load_failed", true, err.Error()), err
	}
	if assetStatus != "ready" || assetOwner != session.FluctlightID {
		err := errors.New("visual identity candidate asset is not ready or not owned by the session Fluctlight")
		return failedCapabilityResultDetail(invocation, "visual_identity_candidate_asset_invalid", false, err.Error()), newCapabilityError("visual_identity_candidate_asset_invalid", false, err)
	}
	vision := map[string]any{
		"identity_match": review["identity_match"], "confidence": review["confidence"],
		"observations": review["observations"], "missing_sections": review["missing_sections"],
	}
	status := "rejected_not_self"
	if decision == "accepted" {
		status = "accepted"
	}
	command, err := tx.Exec(ctx, `UPDATE public.fluctlight_visual_identity_attempts SET candidate_asset_id=$2,vision_result=$3,patch_result=$4,decision=$5,feedback=$6,status=$7,updated_at=now() WHERE id=$1 AND decision IS NULL`, session.AttemptID, assetID, jsonBytes(vision), jsonBytes(review), decision, nullableString(stringValue(review["feedback"])), status)
	if err != nil {
		return failedCapabilityResultDetail(invocation, "visual_identity_review_write_failed", true, err.Error()), err
	}
	if command.RowsAffected() != 1 {
		return failedCapabilityResultDetail(invocation, "visual_identity_review_conflict", true, ErrConflict.Error()), ErrConflict
	}
	for _, event := range []struct {
		stage, status, summary string
	}{
		{visualIdentityStageImageReady, "completed", "候选角色设计图已载入 Visual Identity Agent"},
		{visualIdentityStageVisionRequested, "running", "Visual Identity Agent 正在查看真实候选图片"},
		{visualIdentityStageVisionReady, "completed", "真实候选图片视觉理解完成"},
		{visualIdentityStagePatchRequested, "running", "Visual Identity Agent 正在生成身份评审"},
		{visualIdentityStagePatchReady, "completed", "身份评审已提交"},
	} {
		if err := appendVisualIdentityTimelineTx(ctx, tx, sessionID, session.AttemptID, session.FluctlightID, event.stage, event.status, event.summary, []string{assetID}, map[string]any{"decision": decision}, "visual_identity:"+sessionID); err != nil {
			return failedCapabilityResultDetail(invocation, "visual_identity_timeline_failed", true, err.Error()), err
		}
	}
	if decision == "regenerate" {
		if err := appendVisualIdentityTimelineTx(ctx, tx, sessionID, session.AttemptID, session.FluctlightID, visualIdentityStageRegenerate, "rejected_not_self", visualIdentityBoundedText(stringValue(review["summary"]), 512), []string{assetID}, map[string]any{"decision": decision}, "visual_identity:"+sessionID); err != nil {
			return failedCapabilityResultDetail(invocation, "visual_identity_timeline_failed", true, err.Error()), err
		}
		if session.Attempt >= session.MaxAttempts {
			if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_visual_identity_sessions SET status='awaiting_review',last_error='max_attempts',updated_at=now() WHERE id=$1`, sessionID); err != nil {
				return failedCapabilityResultDetail(invocation, "visual_identity_session_update_failed", true, err.Error()), err
			}
			return visualIdentityToolResult(invocation, "completed", map[string]any{"session_id": sessionID, "attempt": session.Attempt, "status": "awaiting_review", "decision": decision, "candidate_asset_id": assetID, "replayed": false}), nil
		}
		nextAttempt := session.Attempt + 1
		nextAttemptID := visualIdentityAttemptID(sessionID, nextAttempt)
		nextSnapshot := cloneMap(session.InputSnapshot)
		nextSnapshot["previous_asset_id"] = assetID
		nextSnapshot["previous_review"] = cloneMap(review)
		if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_visual_identity_attempts(id,session_id,visual_identity_id,fluctlight_id,attempt_number,status,input_snapshot,renderer_constraints) VALUES($1,$2,$3,$4,$5,'queued',$6,$7)`, nextAttemptID, sessionID, session.ProfileID, session.FluctlightID, nextAttempt, jsonBytes(nextSnapshot), jsonBytes(session.RendererConstraints)); err != nil {
			return failedCapabilityResultDetail(invocation, "visual_identity_next_attempt_failed", true, err.Error()), err
		}
		if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_visual_identity_sessions SET current_attempt=$2,status='running',updated_at=now() WHERE id=$1`, sessionID, nextAttempt); err != nil {
			return failedCapabilityResultDetail(invocation, "visual_identity_session_update_failed", true, err.Error()), err
		}
		return visualIdentityToolResult(invocation, "completed", map[string]any{"session_id": sessionID, "attempt": session.Attempt, "next_attempt": nextAttempt, "status": "regenerating", "decision": decision, "candidate_asset_id": assetID, "replayed": false}), nil
	}
	if err := appendVisualIdentityTimelineTx(ctx, tx, sessionID, session.AttemptID, session.FluctlightID, visualIdentityStageAccepted, "completed", visualIdentityBoundedText(stringValue(review["summary"]), 512), []string{assetID}, map[string]any{"decision": decision}, "visual_identity:"+sessionID); err != nil {
		return failedCapabilityResultDetail(invocation, "visual_identity_timeline_failed", true, err.Error()), err
	}
	var canonicalSnapshotRaw []byte
	if err := tx.QueryRow(ctx, `SELECT identity_snapshot FROM public.fluctlight_visual_identities WHERE id=$1`, session.ProfileID).Scan(&canonicalSnapshotRaw); err != nil {
		return failedCapabilityResultDetail(invocation, "visual_identity_profile_load_failed", true, err.Error()), err
	}
	canonicalSnapshot := decodeObject(canonicalSnapshotRaw)
	var corePersonaRaw []byte
	if err := tx.QueryRow(ctx, `SELECT core_persona FROM public.fluctlights WHERE id=$1`, session.FluctlightID).Scan(&corePersonaRaw); err == nil && len(corePersonaRaw) > 0 {
		enrichIdentitySnapshotWithPersona(canonicalSnapshot, decodeObject(corePersonaRaw))
	}
	characterIntentID, err := a.promoteVisualIdentityCanonicalTx(ctx, tx, sessionID, session.AttemptID, session.ProfileID, session.FluctlightID, assetID, canonicalSnapshot, session.RendererConstraints)
	if err != nil {
		return failedCapabilityResultDetail(invocation, "visual_identity_canonical_save_failed", true, err.Error()), err
	}
	return visualIdentityToolResult(invocation, "accepted", map[string]any{
		"session_id": sessionID, "attempt": session.Attempt, "status": "character_sheet_pending", "decision": decision,
		"candidate_asset_id": assetID, "canonical_asset_id": assetID, "character_sheet_media_intent_id": characterIntentID, "replayed": false,
	}), nil
}

func (a *App) executeVisualIdentityFinalizeTx(ctx context.Context, tx pgx.Tx, invocation CapabilityInvocation, target DirectToolTarget, sessionID string) (CapabilityResult, error) {
	session, err := loadVisualIdentityToolSessionTx(ctx, tx, sessionID)
	if err != nil {
		return failedCapabilityResultDetail(invocation, "visual_identity_session_load_failed", true, err.Error()), err
	}
	if session.FluctlightID != target.FluctlightID {
		err := errors.New("visual identity session is outside the authorized Fluctlight")
		return failedCapabilityResultDetail(invocation, "visual_identity_target_invalid", false, err.Error()), newCapabilityError("visual_identity_target_invalid", false, err)
	}
	if session.Status == "completed" {
		var assetID string
		_ = tx.QueryRow(ctx, `SELECT COALESCE(character_sheet_asset_id,'') FROM public.fluctlight_visual_identities WHERE id=$1`, session.ProfileID).Scan(&assetID)
		return visualIdentityToolResult(invocation, "completed", map[string]any{"session_id": sessionID, "attempt": session.Attempt, "status": "completed", "character_sheet_asset_id": assetID, "replayed": true}), nil
	}
	if session.Status != "character_sheet_pending" || session.CharacterMediaIntentID == "" {
		return visualIdentityBusinessRejection(invocation, "visual_identity_character_sheet_not_requested", sessionID, session.Attempt, session.Status), nil
	}
	var mediaStatus string
	if err := tx.QueryRow(ctx, `SELECT status FROM public.media_intents WHERE id=$1`, session.CharacterMediaIntentID).Scan(&mediaStatus); err != nil {
		return failedCapabilityResultDetail(invocation, "visual_identity_character_sheet_intent_load_failed", true, err.Error()), err
	}
	if mediaStatus != "completed" {
		return visualIdentityBusinessRejection(invocation, "visual_identity_character_sheet_not_ready", sessionID, session.Attempt, mediaStatus), nil
	}
	assetID := "asset_" + session.CharacterMediaIntentID
	var assetStatus, assetOwner string
	if err := tx.QueryRow(ctx, `SELECT status,owner_fluctlight_id FROM public.media_assets WHERE id=$1`, assetID).Scan(&assetStatus, &assetOwner); err != nil {
		return failedCapabilityResultDetail(invocation, "visual_identity_character_sheet_asset_load_failed", true, err.Error()), err
	}
	if assetStatus != "ready" || assetOwner != session.FluctlightID {
		err := errors.New("visual identity character-sheet asset is not ready or not owned by the session Fluctlight")
		return failedCapabilityResultDetail(invocation, "visual_identity_character_sheet_asset_invalid", false, err.Error()), newCapabilityError("visual_identity_character_sheet_asset_invalid", false, err)
	}
	profileCommand, err := tx.Exec(ctx, `UPDATE public.fluctlight_visual_identities SET status='active',character_sheet_asset_id=$2,active_session_id=$3,updated_at=now() WHERE id=$1`, session.ProfileID, assetID, sessionID)
	if err != nil {
		return failedCapabilityResultDetail(invocation, "visual_identity_profile_finalize_failed", true, err.Error()), err
	}
	if profileCommand.RowsAffected() != 1 {
		return failedCapabilityResultDetail(invocation, "visual_identity_profile_finalize_conflict", true, ErrConflict.Error()), ErrConflict
	}
	if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_visual_identity_revisions SET character_sheet_asset_id=$2 WHERE visual_identity_id=$1 AND revision=(SELECT current_revision FROM public.fluctlight_visual_identities WHERE id=$1)`, session.ProfileID, assetID); err != nil {
		return failedCapabilityResultDetail(invocation, "visual_identity_revision_finalize_failed", true, err.Error()), err
	}
	sessionCommand, err := tx.Exec(ctx, `UPDATE public.fluctlight_visual_identity_sessions SET status='completed',updated_at=now() WHERE id=$1 AND status='character_sheet_pending'`, sessionID)
	if err != nil {
		return failedCapabilityResultDetail(invocation, "visual_identity_session_finalize_failed", true, err.Error()), err
	}
	if sessionCommand.RowsAffected() != 1 {
		return failedCapabilityResultDetail(invocation, "visual_identity_session_finalize_conflict", true, ErrConflict.Error()), ErrConflict
	}
	if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_visual_identity_attempts SET status='completed',updated_at=now() WHERE id=$1`, session.AttemptID); err != nil {
		return failedCapabilityResultDetail(invocation, "visual_identity_attempt_finalize_failed", true, err.Error()), err
	}
	if _, err := a.settleActionOutcomeByExternalRefTx(ctx, tx, sessionID, ActionOutcomeCompleted, map[string]any{"session_id": sessionID, "asset_id": assetID, "delivery_status": "visual_identity_ready"}, ""); err != nil {
		return failedCapabilityResultDetail(invocation, "visual_identity_outcome_finalize_failed", true, err.Error()), err
	}
	if err := appendVisualIdentityTimelineTx(ctx, tx, sessionID, session.AttemptID, session.FluctlightID, visualIdentityStageCharacterReady, "completed", "character sheet 已生成", []string{assetID}, nil, "visual_identity:"+sessionID); err != nil {
		return failedCapabilityResultDetail(invocation, "visual_identity_timeline_failed", true, err.Error()), err
	}
	if err := appendVisualIdentityTimelineTx(ctx, tx, sessionID, session.AttemptID, session.FluctlightID, visualIdentityStageCompleted, "completed", "Visual Identity Agent 已完成完整任务", []string{assetID}, nil, "visual_identity:"+sessionID); err != nil {
		return failedCapabilityResultDetail(invocation, "visual_identity_timeline_failed", true, err.Error()), err
	}
	return visualIdentityToolResult(invocation, "completed", map[string]any{"session_id": sessionID, "attempt": session.Attempt, "status": "completed", "character_sheet_asset_id": assetID, "replayed": false}), nil
}

func visualIdentityToolResult(invocation CapabilityInvocation, status string, output map[string]any) CapabilityResult {
	return CapabilityResult{
		CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: status, Output: output,
		ProviderRequestID: invocation.ProviderRequestID, CorrelationID: "visual_identity:" + stringValue(output["session_id"]),
	}
}

func visualIdentityBusinessRejection(invocation CapabilityInvocation, code, sessionID string, attempt int, status string) CapabilityResult {
	return CapabilityResult{
		CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "rejected", ErrorCode: code, Retryable: false,
		Output:            map[string]any{"session_id": sessionID, "attempt": attempt, "status": firstString(status, "failed")},
		ProviderRequestID: invocation.ProviderRequestID, CorrelationID: "visual_identity:" + sessionID,
	}
}
