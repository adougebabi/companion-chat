package core

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// VisualIdentityAgentInput is the typed production input for the complete
// Visual Identity task. The durable session is the sole resume coordinate;
// callers do not select a model stage or provide model-owned observations.
type VisualIdentityAgentInput struct {
	SessionID string `json:"session_id"`
}

// VisualIdentityAgentOutput separates durable acceptance/waiting from final
// completion. A queued media intent is accepted work, never a completed image.
type VisualIdentityAgentOutput struct {
	SessionID              string `json:"session_id"`
	FluctlightID           string `json:"fluctlight_id"`
	Attempt                int    `json:"attempt"`
	Status                 string `json:"status"`
	Stage                  string `json:"stage"`
	Accepted               bool   `json:"accepted"`
	MediaIntentID          string `json:"media_intent_id,omitempty"`
	CandidateAssetID       string `json:"candidate_asset_id,omitempty"`
	CanonicalAssetID       string `json:"canonical_asset_id,omitempty"`
	CharacterMediaIntentID string `json:"character_sheet_media_intent_id,omitempty"`
	CharacterSheetAssetID  string `json:"character_sheet_asset_id,omitempty"`
	ErrorCode              string `json:"error_code,omitempty"`
	Summary                string `json:"summary,omitempty"`
}

type visualIdentityAgentState struct {
	SessionID              string
	FluctlightID           string
	OwnerActorID           string
	ProfileID              string
	SessionStatus          string
	Attempt                int
	MaxAttempts            int
	CharacterMediaIntentID string
	CharacterMediaStatus   string
	CharacterSheetAssetID  string
	CharacterAssetReady    bool
	CanonicalAssetID       string
	AttemptID              string
	SeedPrompt             string
	MediaIntentID          string
	MediaStatus            string
	CandidateAssetID       string
	CandidateAssetReady    bool
	Decision               string
	InputSnapshot          map[string]any
	RendererConstraints    map[string]any
	History                []map[string]any
	ActionRequired         string
	WaitingStage           string
}

const visualIdentityAgentInstruction = `You own one complete durable Visual Identity task. Use only the supplied session state and the dedicated tools.

The application sets action_required from authoritative persisted state:
- generate_candidate: call visual_identity.generate_candidate exactly once. Use reason=initial for attempt 1 and reason=regenerate for later attempts.
- commit_review: first inspect the actual image content block in this request. Then call visual_identity.commit_review exactly once with bounded observations and an honest accepted or regenerate decision. Accept only a complete, text-free, realistic human character profile card with one consistent face, front/side/back full-body views, six expressions, clothing/accessory/detail panels, color swatches, intro/personality/signature visual areas, a white minimalist background, and editorial 3:4 composition. Do not penalize missing readable lettering because text is intentionally forbidden.
- finalize: call visual_identity.finalize exactly once.
- none: do not call a tool; the durable media task is still running or the session is already terminal.

Never claim that queued or running media is complete. After any tool result, return the final response contract. status must reflect the durable result: waiting for accepted asynchronous work, awaiting_review when automatic work stopped, completed only after visual_identity.finalize committed the character-sheet asset, or failed for a terminal persisted failure. Do not return prose outside the response contract.`

// RunVisualIdentityAgent is the only model-owning entry for the complete
// Visual Identity task. It uses the same formal Eino Runner and App.ExecuteTool
// bridge as every other Agent. Media waits are resumed through the same typed
// entry; no caller chooses vision versus patch or executes their results.
func (a *App) RunVisualIdentityAgent(ctx context.Context, input VisualIdentityAgentInput) (VisualIdentityAgentOutput, error) {
	if a == nil || a.DB == nil || a.DB.Pool() == nil {
		return VisualIdentityAgentOutput{}, errors.New("visual_identity_agent_database_unavailable")
	}
	sessionID := strings.TrimSpace(input.SessionID)
	if sessionID == "" {
		return VisualIdentityAgentOutput{}, errors.New("visual_identity_agent_session_required")
	}
	state, err := a.loadVisualIdentityAgentState(ctx, sessionID)
	if err != nil {
		return VisualIdentityAgentOutput{}, err
	}
	if state.ActionRequired == "" {
		return visualIdentityAgentOutputFromState(state, ""), nil
	}

	var imageContent map[string]any
	if state.ActionRequired == visualIdentityCommitReviewCapabilityName {
		imageContent, err = a.visualIdentityImageContent(ctx, state.CandidateAssetID)
		if err != nil {
			return VisualIdentityAgentOutput{}, err
		}
		if len(imageContent) == 0 {
			return VisualIdentityAgentOutput{}, errors.New("visual_identity_agent_real_image_required")
		}
	}

	registry := a.capabilityRegistry()
	definitions := capabilityCatalog(registry, CapabilitySurfaceVisualIdentity)
	if len(definitions) != 3 {
		return VisualIdentityAgentOutput{}, fmt.Errorf("visual_identity_agent_tool_catalog_invalid: got %d", len(definitions))
	}
	prompt := PromptAssemblyResult{
		Messages:       visualIdentityAgentMessages(state, imageContent),
		ResponseFormat: visualIdentityAgentResponseSchema(),
	}
	actionID := state.AttemptID
	if state.ActionRequired == visualIdentityFinalizeCapabilityName {
		actionID = state.SessionID + ":finalize"
	}
	operationID := visualIdentityAgentCheckpointOperationID(state)
	run, err := a.RunFormalAgent(WithProviderCorrelation(ctx, "visual_identity:"+state.SessionID), FormalAgentVisualIdentity, FormalAgentRunInput{
		Prompt: prompt, Definitions: definitions, SchemaName: "visual_identity_agent_response",
		Capability: &ADKCapabilityRequest{
			TargetKind: "visual_identity_session", TargetRef: state.SessionID,
			AuthorizationActorID: state.OwnerActorID, FluctlightID: state.FluctlightID,
			SourceFactID: state.SessionID, ActionID: actionID, OperationID: operationID,
			CorrelationID: "visual_identity:" + state.SessionID, Surface: CapabilitySurfaceVisualIdentity,
			Projection: ContextProjection{OwnerActorID: state.OwnerActorID, FluctlightID: state.FluctlightID, SourceFactID: state.SessionID},
		},
	})
	if err != nil {
		return VisualIdentityAgentOutput{}, err
	}
	if err := validateVisualIdentityAgentToolProgress(state.ActionRequired, run.Trace); err != nil {
		return VisualIdentityAgentOutput{}, err
	}

	after, err := a.loadVisualIdentityAgentState(ctx, sessionID)
	if err != nil {
		return VisualIdentityAgentOutput{}, err
	}
	modelStatus := strings.TrimSpace(stringValue(run.Completion.Structured["status"]))
	result := visualIdentityAgentOutputFromState(after, visualIdentityBoundedText(stringValue(run.Completion.Structured["summary"]), 1000))
	if modelStatus != result.Status {
		return VisualIdentityAgentOutput{}, fmt.Errorf("visual_identity_agent_final_state_mismatch: model=%s durable=%s", modelStatus, result.Status)
	}
	return result, nil
}

func visualIdentityAgentCheckpointOperationID(state visualIdentityAgentState) string {
	return strings.Join([]string{"visual_identity_agent", state.SessionID, fmt.Sprintf("attempt-%d", state.Attempt), state.ActionRequired}, ":")
}

func visualIdentityAgentMessages(state visualIdentityAgentState, imageContent map[string]any) []map[string]any {
	contextPayload := map[string]any{
		"session_id": state.SessionID, "fluctlight_id": state.FluctlightID,
		"session_status": state.SessionStatus, "attempt": state.Attempt, "max_attempts": state.MaxAttempts,
		"action_required": visualIdentityAgentActionLabel(state.ActionRequired), "waiting_stage": state.WaitingStage,
		"media_intent_id": state.MediaIntentID, "media_status": state.MediaStatus,
		"candidate_asset_id": state.CandidateAssetID, "candidate_asset_ready": state.CandidateAssetReady,
		"character_sheet_media_intent_id": state.CharacterMediaIntentID, "character_sheet_media_status": state.CharacterMediaStatus,
		"canonical_asset_id": state.CanonicalAssetID, "character_sheet_asset_id": state.CharacterSheetAssetID,
		"identity_snapshot": state.InputSnapshot, "renderer_constraints": state.RendererConstraints,
		"attempt_history": state.History, "required_card_sections": append([]string(nil), visualIdentityRequiredCardSections...),
		"expected_views": visualIdentityExpectedViews(),
	}
	content := []any{map[string]any{"type": "text", "text": jsonString(contextPayload)}}
	if len(imageContent) > 0 {
		// This is an actual OpenAI-compatible multimodal content block. The
		// image bytes are not represented as a path or a JSON-only attachment
		// reference; the formal adapter sends them in the model request.
		content = append(content, imageContent)
	}
	return (&PromptComposer{}).ComposeTaskMessages("visual_identity_agent", []map[string]any{
		{"role": "system", "content": visualIdentityAgentInstruction},
		{"role": "user", "content": content},
	})
}

func visualIdentityAgentActionLabel(capabilityName string) string {
	switch capabilityName {
	case visualIdentityGenerateCandidateCapabilityName:
		return "generate_candidate"
	case visualIdentityCommitReviewCapabilityName:
		return "commit_review"
	case visualIdentityFinalizeCapabilityName:
		return "finalize"
	default:
		return "none"
	}
}

func visualIdentityAgentResponseSchema() map[string]any {
	return map[string]any{
		"type": "object", "additionalProperties": false,
		"required": []any{"status", "stage", "summary"},
		"properties": map[string]any{
			"status":  map[string]any{"type": "string", "enum": []any{"waiting", "awaiting_review", "completed", "failed"}},
			"stage":   map[string]any{"type": "string", "minLength": 1, "maxLength": 128},
			"summary": map[string]any{"type": "string", "minLength": 1, "maxLength": 1000},
		},
	}
}

func validateVisualIdentityAgentToolProgress(expected string, trace *ADKCapabilityTrace) error {
	if trace == nil {
		return errors.New("visual_identity_agent_tool_trace_missing")
	}
	invocations, results := trace.Snapshot()
	matched := 0
	for _, invocation := range invocations {
		if invocation.CapabilityName == expected {
			matched++
		}
	}
	if matched != 1 {
		return fmt.Errorf("visual_identity_agent_required_tool_mismatch: expected=%s matched=%d", expected, matched)
	}
	for _, result := range results {
		if result.CapabilityName == expected && (result.Status == "completed" || result.Status == "accepted" || result.Status == "rejected") {
			return nil
		}
	}
	return fmt.Errorf("visual_identity_agent_required_tool_result_missing: %s", expected)
}

func (a *App) loadVisualIdentityAgentState(ctx context.Context, sessionID string) (visualIdentityAgentState, error) {
	var state visualIdentityAgentState
	if err := a.DB.Pool().QueryRow(ctx, `SELECT s.id,s.fluctlight_id,f.created_by_actor_id,s.visual_identity_id,s.status,s.current_attempt,s.max_attempts,COALESCE(s.character_sheet_media_intent_id,''),COALESCE(v.canonical_asset_id,''),COALESCE(v.character_sheet_asset_id,'') FROM public.fluctlight_visual_identity_sessions AS s JOIN public.fluctlights AS f ON f.id=s.fluctlight_id JOIN public.fluctlight_visual_identities AS v ON v.id=s.visual_identity_id WHERE s.id=$1`, sessionID).Scan(
		&state.SessionID, &state.FluctlightID, &state.OwnerActorID, &state.ProfileID, &state.SessionStatus, &state.Attempt, &state.MaxAttempts,
		&state.CharacterMediaIntentID, &state.CanonicalAssetID, &state.CharacterSheetAssetID,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return state, ErrNotFound
		}
		return state, err
	}
	if state.MaxAttempts < 1 {
		state.MaxAttempts = visualIdentityMaxAttempts
	}
	state.AttemptID = visualIdentityAttemptID(sessionID, state.Attempt)
	var inputSnapshot, constraints []byte
	if err := a.DB.Pool().QueryRow(ctx, `SELECT COALESCE(seed_prompt,''),COALESCE(media_intent_id,''),COALESCE(candidate_asset_id,''),COALESCE(decision,''),input_snapshot,renderer_constraints FROM public.fluctlight_visual_identity_attempts WHERE id=$1`, state.AttemptID).Scan(
		&state.SeedPrompt, &state.MediaIntentID, &state.CandidateAssetID, &state.Decision, &inputSnapshot, &constraints,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return state, ErrNotFound
		}
		return state, err
	}
	state.InputSnapshot = decodeObject(inputSnapshot)
	state.RendererConstraints = decodeObject(constraints)

	if state.MediaIntentID != "" {
		if err := a.DB.Pool().QueryRow(ctx, `SELECT status FROM public.media_intents WHERE id=$1`, state.MediaIntentID).Scan(&state.MediaStatus); err != nil {
			return state, err
		}
		if state.MediaStatus == "completed" && state.CandidateAssetID == "" {
			state.CandidateAssetID = "asset_" + state.MediaIntentID
		}
	}
	if state.CandidateAssetID != "" {
		var owner, status string
		err := a.DB.Pool().QueryRow(ctx, `SELECT owner_fluctlight_id,status FROM public.media_assets WHERE id=$1`, state.CandidateAssetID).Scan(&owner, &status)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return state, err
		}
		state.CandidateAssetReady = err == nil && owner == state.FluctlightID && status == "ready"
	}
	if state.CharacterMediaIntentID != "" {
		if err := a.DB.Pool().QueryRow(ctx, `SELECT status FROM public.media_intents WHERE id=$1`, state.CharacterMediaIntentID).Scan(&state.CharacterMediaStatus); err != nil {
			return state, err
		}
		if state.CharacterMediaStatus == "completed" {
			assetID := "asset_" + state.CharacterMediaIntentID
			var owner, status string
			err := a.DB.Pool().QueryRow(ctx, `SELECT owner_fluctlight_id,status FROM public.media_assets WHERE id=$1`, assetID).Scan(&owner, &status)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return state, err
			}
			state.CharacterAssetReady = err == nil && owner == state.FluctlightID && status == "ready"
		}
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT attempt_number,COALESCE(candidate_asset_id,''),COALESCE(decision,''),status FROM public.fluctlight_visual_identity_attempts WHERE session_id=$1 ORDER BY attempt_number`, sessionID)
	if err != nil {
		return state, err
	}
	defer rows.Close()
	for rows.Next() {
		var attempt int
		var assetID, decision, status string
		if err := rows.Scan(&attempt, &assetID, &decision, &status); err != nil {
			return state, err
		}
		state.History = append(state.History, map[string]any{"attempt": attempt, "candidate_asset_id": assetID, "decision": decision, "status": status})
	}
	if err := rows.Err(); err != nil {
		return state, err
	}
	if err := classifyVisualIdentityAgentState(&state); err != nil {
		return state, err
	}
	return state, nil
}

func classifyVisualIdentityAgentState(state *visualIdentityAgentState) error {
	if state == nil {
		return errors.New("visual_identity_agent_state_required")
	}
	switch state.SessionStatus {
	case "completed", "cancelled", "failed", visualIdentityStatusAwaitingReview:
		state.WaitingStage = state.SessionStatus
		return nil
	case "character_sheet_pending":
		if state.CharacterMediaStatus == "failed" {
			return errors.New("visual_identity_character_sheet_media_failed")
		}
		if state.CharacterMediaStatus == "completed" {
			if !state.CharacterAssetReady {
				return errors.New("visual_identity_character_sheet_asset_not_ready")
			}
			assetID := "asset_" + state.CharacterMediaIntentID
			if state.CharacterSheetAssetID == "" {
				state.CharacterSheetAssetID = assetID
			}
			state.ActionRequired = visualIdentityFinalizeCapabilityName
			state.WaitingStage = "character_sheet_ready"
			return nil
		}
		state.WaitingStage = "character_sheet_" + firstString(state.CharacterMediaStatus, "pending")
		return nil
	case "queued", "running":
	default:
		return fmt.Errorf("visual_identity_session_status_invalid: %s", state.SessionStatus)
	}
	if stringValue(state.RendererConstraints["error"]) != "" || state.MediaIntentID == "" {
		state.ActionRequired = visualIdentityGenerateCandidateCapabilityName
		state.WaitingStage = "candidate_generation_required"
		return nil
	}
	switch state.MediaStatus {
	case "pending", "running", "retry":
		state.WaitingStage = "image_" + state.MediaStatus
		return nil
	case "failed":
		return errors.New("visual_identity_candidate_media_failed")
	case "completed":
		if !state.CandidateAssetReady {
			return errors.New("visual_identity_candidate_asset_not_ready")
		}
		if state.Decision == "" {
			state.ActionRequired = visualIdentityCommitReviewCapabilityName
			state.WaitingStage = "candidate_review_required"
			return nil
		}
		return errors.New("visual_identity_review_settlement_incomplete")
	default:
		return fmt.Errorf("visual_identity_media_status_invalid: %s", state.MediaStatus)
	}
}

func visualIdentityAgentOutputFromState(state visualIdentityAgentState, summary string) VisualIdentityAgentOutput {
	result := VisualIdentityAgentOutput{
		SessionID: state.SessionID, FluctlightID: state.FluctlightID, Attempt: state.Attempt,
		Status: "waiting", Stage: firstString(state.WaitingStage, "waiting"), Accepted: true,
		MediaIntentID: state.MediaIntentID, CandidateAssetID: state.CandidateAssetID,
		CanonicalAssetID: state.CanonicalAssetID, CharacterMediaIntentID: state.CharacterMediaIntentID,
		CharacterSheetAssetID: state.CharacterSheetAssetID, Summary: summary,
	}
	switch state.SessionStatus {
	case "completed":
		result.Status, result.Stage, result.Accepted = "completed", "completed", true
	case visualIdentityStatusAwaitingReview:
		result.Status, result.Stage, result.Accepted = "awaiting_review", "awaiting_review", false
	case "failed", "cancelled":
		result.Status, result.Stage, result.Accepted = "failed", state.SessionStatus, false
		result.ErrorCode = "visual_identity_session_" + state.SessionStatus
	}
	return result
}

func (output VisualIdentityAgentOutput) asMap() map[string]any {
	return map[string]any{
		"session_id": output.SessionID, "fluctlight_id": output.FluctlightID, "attempt": output.Attempt,
		"status": output.Status, "stage": output.Stage, "accepted": output.Accepted,
		"media_intent_id": output.MediaIntentID, "asset_id": output.CandidateAssetID,
		"canonical_asset_id": output.CanonicalAssetID, "character_sheet_media_intent_id": output.CharacterMediaIntentID,
		"character_sheet_asset_id": output.CharacterSheetAssetID, "error_code": output.ErrorCode, "summary": output.Summary,
	}
}
