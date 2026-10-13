package core

import (
	"context"
	"errors"
	"strings"

	"github.com/cloudwego/eino/schema"
	"github.com/jackc/pgx/v5"
)

type visionCapability struct {
	State      string
	Reason     string
	Version    int64
	Assignment providerAssignment
}

func (v visionCapability) err() error {
	if v.State == "ready" {
		return nil
	}
	return errors.New("vision_" + v.State)
}

func (a *App) resolveVisionCapability(ctx context.Context) visionCapability {
	settings, version, err := a.visionUnderstandingSettings(ctx)
	result := visionCapability{State: "disabled", Reason: "vision_disabled", Version: version}
	if err != nil {
		result.State = "unavailable"
		result.Reason = "vision_config_invalid"
		return result
	}
	if !settings.Enabled {
		return result
	}
	result.State = "unconfigured"
	result.Reason = "vision_model_unconfigured"
	if settings.ModelRef == "" || a.Provider == nil {
		return result
	}
	result.Assignment, err = a.Provider.assignment(ctx, visionUnderstandingModelRole)
	if err != nil {
		result.State = "unavailable"
		result.Reason = "vision_binding_unavailable"
		if errors.Is(err, pgx.ErrNoRows) {
			result.State = "unconfigured"
			result.Reason = "vision_model_unconfigured"
		}
		return result
	}
	result.State = "ready"
	result.Reason = ""
	return result
}

// This private hook is used by local controlled tests with no PostgreSQL. Real
// calls always read runtime_settings again before each physical submission.
type visionSettingsReaderKey struct{}

func (p *ProviderClient) currentVisionSettings(ctx context.Context) (VisionUnderstandingSettings, error) {
	if reader, ok := ctx.Value(visionSettingsReaderKey{}).(func(context.Context) (VisionUnderstandingSettings, error)); ok {
		return reader(ctx)
	}
	if p == nil || p.DB == nil || p.DB.Pool() == nil {
		return defaultVisionUnderstandingSettings(), nil
	}
	var raw string
	err := p.DB.Pool().QueryRow(ctx, `SELECT value_json FROM public.runtime_settings WHERE key=$1`, visionUnderstandingSettingKey).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return defaultVisionUnderstandingSettings(), nil
	}
	if err != nil {
		return VisionUnderstandingSettings{}, err
	}
	return decodeVisionUnderstandingSettings([]byte(raw))
}
func (p *ProviderClient) guardVisionInput(ctx context.Context, assignment providerAssignment, messages []*schema.Message) error {
	if !einoInputHasImage(messages) {
		return nil
	}
	settings, err := p.currentVisionSettings(ctx)
	if err != nil {
		return errors.New("vision_config_invalid")
	}
	if !settings.Enabled {
		return errors.New("vision_disabled")
	}
	if settings.ModelRef == "" {
		return errors.New("vision_unconfigured")
	}
	if assignment.Role != visionUnderstandingModelRole {
		return errors.New("vision_independent_binding_required")
	}
	return nil
}
func einoInputHasImage(messages []*schema.Message) bool {
	for _, message := range messages {
		if message != nil {
			for _, part := range message.UserInputMultiContent {
				if part.Type == schema.ChatMessagePartTypeImageURL {
					return true
				}
			}
		}
	}
	return false
}

type visualInspectionAuthorityKey struct{}
type visualInspectionAuthority struct {
	SessionID, AssetID, ModelID string
	ConfigVersion               int64
}

func (a *App) pauseVisualIdentityReview(ctx context.Context, state visualIdentityAgentState, reason string) (VisualIdentityAgentOutput, error) {
	err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE public.fluctlight_visual_identity_sessions SET status='awaiting_review',last_error=$2,updated_at=now() WHERE id=$1 AND status IN ('queued','running') AND current_attempt=$3`, state.SessionID, reason, state.Attempt)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return ErrConflict
		}
		if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_visual_identity_attempts SET candidate_asset_id=$2,vision_result=$3,status='awaiting_review',updated_at=now() WHERE id=$1 AND decision IS NULL`, state.AttemptID, state.CandidateAssetID, jsonBytes(map[string]any{"inspection_status": "skipped", "reason": reason, "provenance": "none"})); err != nil {
			return err
		}
		return appendVisualIdentityTimelineTx(ctx, tx, state.SessionID, state.AttemptID, state.FluctlightID, visualIdentityStageVisionReady, "skipped", "候选图片已保存，视觉检验未完成，等待显式评审", []string{state.CandidateAssetID}, map[string]any{"reason": reason, "inspection_status": "unverified"}, "visual_identity:"+state.SessionID)
	})
	if err != nil {
		return VisualIdentityAgentOutput{}, err
	}
	state.SessionStatus = "awaiting_review"
	state.ActionRequired = ""
	state.WaitingStage = "awaiting_review"
	result := visualIdentityAgentOutputFromState(state, "")
	result.ErrorCode = reason
	return result, nil
}

func visionFailureReason(err error) string {
	if err == nil {
		return ""
	}
	code := err.Error()
	for _, known := range []string{"vision_disabled", "vision_unconfigured", "vision_independent_binding_required"} {
		if strings.Contains(code, known) {
			return known
		}
	}
	return "vision_unavailable"
}

func (p *ProviderClient) guardVisionBeforeAssignment(ctx context.Context, role string, messages []map[string]any) error {
	input, err := providerMessagesToEino(messages)
	if err != nil {
		return err
	}
	if !einoInputHasImage(input) {
		return nil
	}
	assignment := providerAssignment{Role: providerBindingRole(role)}
	if frozen, ok := providerAssignmentSnapshot(ctx, role); ok {
		assignment = frozen
	}
	return p.guardVisionInput(ctx, assignment, input)
}
