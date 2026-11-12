package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	conversationReplySettingKey   = "conversation.reply"
	visionUnderstandingSettingKey = "vision.understanding"
	conversationReplyModelRole    = "conversation_reply"
	visionUnderstandingModelRole  = "vision_understanding"
)

type ConversationReplySettings struct {
	ExpressionVersion string  `json:"expression_version,omitempty"`
	Mode              string  `json:"mode"`
	ModelRef          string  `json:"model_ref"`
	Temperature       float64 `json:"temperature"`
	TopP              float64 `json:"top_p"`
	MaxTokens         int     `json:"max_tokens,omitempty"`
	MaxInputTokens    int     `json:"max_input_tokens,omitempty"`
	TimeoutSeconds    int     `json:"timeout_seconds"`
	Concurrency       int     `json:"concurrency"`
	RetryLimit        int     `json:"retry_limit"`
	FailurePolicy     string  `json:"failure_policy"`
	MaxActionHandoffs int     `json:"max_action_handoffs"`
}

type VisionUnderstandingSettings struct {
	Enabled  bool   `json:"enabled"`
	ModelRef string `json:"model_ref"`
}

func defaultConversationReplySettings() ConversationReplySettings {
	return ConversationReplySettings{Mode: "direct", ModelRef: conversationReplyModelRole, ExpressionVersion: factOnlyReplyPromptVersion,
		Temperature: 0.8, TopP: 0.9, TimeoutSeconds: 300,
		Concurrency: 1, RetryLimit: 1, FailurePolicy: "error", MaxActionHandoffs: 2}
}

func defaultVisionUnderstandingSettings() VisionUnderstandingSettings {
	return VisionUnderstandingSettings{ModelRef: visionUnderstandingModelRole}
}

func decodeReplyVisionSetting[T any](raw []byte, defaults T) (T, error) {
	if trimmed := bytes.TrimSpace(raw); len(trimmed) == 0 || trimmed[0] != '{' {
		return defaults, ErrInvalidArguments
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&defaults); err != nil {
		return defaults, ErrInvalidArguments
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return defaults, ErrInvalidArguments
	}
	return defaults, nil
}

func decodeConversationReplySettings(raw []byte) (ConversationReplySettings, error) {
	value, err := decodeReplyVisionSetting(raw, defaultConversationReplySettings())
	if err != nil {
		return value, errors.New("conversation_reply_config_invalid")
	}
	if rawVersion, present := decodeObject(raw)["expression_version"]; present && strings.TrimSpace(stringValue(rawVersion)) == "" {
		return value, errors.New("conversation_reply_config_invalid")
	}
	if (value.Mode != "legacy" && value.Mode != "fact_only" && value.Mode != "direct") ||
		(!replyUsesTextChatProtocol(value.ExpressionVersion) && value.ExpressionVersion != factOnlyReplyLegacyPromptVersion) ||
		(value.ModelRef != "" && value.ModelRef != conversationReplyModelRole) ||
		(value.FailurePolicy != "error" && value.FailurePolicy != "legacy_fallback") ||
		math.IsNaN(value.Temperature) || math.IsInf(value.Temperature, 0) || value.Temperature < 0 || value.Temperature > 2 ||
		math.IsNaN(value.TopP) || math.IsInf(value.TopP, 0) || value.TopP <= 0 || value.TopP > 1 ||
		value.TimeoutSeconds < 1 || value.TimeoutSeconds > 3600 || value.Concurrency < 1 || value.Concurrency > 8 ||
		value.RetryLimit < 0 || value.RetryLimit > 2 || value.MaxActionHandoffs < 1 || value.MaxActionHandoffs > 2 {
		return value, errors.New("conversation_reply_config_invalid")
	}
	value.MaxTokens, value.MaxInputTokens = 0, 0
	return value, nil
}

func decodeVisionUnderstandingSettings(raw []byte) (VisionUnderstandingSettings, error) {
	value, err := decodeReplyVisionSetting(raw, defaultVisionUnderstandingSettings())
	if err != nil || (value.ModelRef != "" && value.ModelRef != visionUnderstandingModelRole) {
		return value, errors.New("vision_understanding_config_invalid")
	}
	return value, nil
}

func (a *App) readOptionalRuntimeSetting(ctx context.Context, key string) ([]byte, int64, error) {
	if a == nil || a.DB == nil || a.DB.Pool() == nil {
		return nil, 0, nil
	}
	var raw string
	var revision int64
	err := a.DB.Pool().QueryRow(ctx, `SELECT value_json,revision FROM public.runtime_settings WHERE key=$1`, key).Scan(&raw, &revision)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, 0, nil
	}
	return []byte(raw), revision, err
}

func (a *App) conversationReplySettings(ctx context.Context) (ConversationReplySettings, int64, error) {
	raw, revision, err := a.readOptionalRuntimeSetting(ctx, conversationReplySettingKey)
	if err != nil || len(raw) == 0 {
		return defaultConversationReplySettings(), revision, err
	}
	value, err := decodeConversationReplySettings(raw)
	return value, revision, err
}

func (a *App) visionUnderstandingSettings(ctx context.Context) (VisionUnderstandingSettings, int64, error) {
	raw, revision, err := a.readOptionalRuntimeSetting(ctx, visionUnderstandingSettingKey)
	if err != nil || len(raw) == 0 {
		return defaultVisionUnderstandingSettings(), revision, err
	}
	value, err := decodeVisionUnderstandingSettings(raw)
	return value, revision, err
}

// Routing identities can be independent while their physical requests share
// the existing generated-model capacity across API and Worker processes.
func providerQueueBindingRole(role string) string {
	if role == "embedding" {
		return "embedding"
	}
	return "generic_llm"
}

type frozenProviderAssignmentKey struct{}
type frozenProviderAssignment struct {
	role   string
	value  providerAssignment
	route  agentModelRoute
	scoped bool
}

func withFrozenProviderAssignment(ctx context.Context, role string, value providerAssignment) context.Context {
	route, scoped := ctx.Value(agentModelRouteKey{}).(agentModelRoute)
	return context.WithValue(ctx, frozenProviderAssignmentKey{}, frozenProviderAssignment{role: role, value: value, route: route, scoped: scoped && route.agent != ""})
}

func providerAssignmentSnapshot(ctx context.Context, role string) (providerAssignment, bool) {
	value, ok := ctx.Value(frozenProviderAssignmentKey{}).(frozenProviderAssignment)
	if !ok || value.role != role {
		return providerAssignment{}, false
	}
	if value.scoped {
		route, ok := ctx.Value(agentModelRouteKey{}).(agentModelRoute)
		if !ok || route != value.route {
			return providerAssignment{}, false
		}
	}
	return value.value, true
}

func restoredAssignmentSecretPurpose(assignment providerAssignment, currentBaseURL, currentPurpose string) (string, error) {
	if assignment.SecretPurpose != "" {
		return assignment.SecretPurpose, nil
	}
	// Old snapshots did not record the credential reference. Reusing the
	// endpoint's current reference is safe only while the destination remains
	// the same; otherwise a new credential could be sent to the old URL.
	if strings.TrimRight(assignment.BaseURL, "/") == "" || strings.TrimRight(assignment.BaseURL, "/") != strings.TrimRight(currentBaseURL, "/") {
		return "", errors.New("reply_snapshot_credential_source_unknown")
	}
	return currentPurpose, nil
}

func (a *App) restoreAssignmentCredential(ctx context.Context, assignment *providerAssignment) error {
	// Read destination, credential and revision in one database snapshot. A
	// changed destination must never receive a credential rotated for another
	// endpoint under the same secret purpose.
	var currentBaseURL, currentPurpose string
	var ciphertext, nonce []byte
	var endpointRevision, credentialRevision int64
	err := a.DB.Pool().QueryRow(ctx, `SELECT e.base_url,COALESCE(e.secret_purpose,''),s.ciphertext,s.nonce,COALESCE((SELECT revision FROM public.configuration_revisions WHERE resource_key='endpoint:'||e.id),0),COALESCE((SELECT revision FROM public.configuration_revisions WHERE resource_key='secret:'||(CASE WHEN $2::text<>'' THEN $2::text ELSE e.secret_purpose END)),0) FROM public.provider_endpoints e LEFT JOIN public.setting_secrets s ON s.purpose=(CASE WHEN $2::text<>'' THEN $2::text ELSE e.secret_purpose END) WHERE e.id=$1`, assignment.EndpointID, assignment.SecretPurpose).Scan(&currentBaseURL, &currentPurpose, &ciphertext, &nonce, &endpointRevision, &credentialRevision)
	if err != nil {
		return err
	}
	purpose, err := restoredAssignmentSecretPurpose(*assignment, currentBaseURL, currentPurpose)
	if err != nil {
		return err
	}
	// An endpoint may have moved away and back since the snapshot. If both its
	// revision and the credential changed, the current URL alone cannot prove
	// that the new credential was intended for the frozen destination.
	endpointChanged := strings.TrimRight(assignment.BaseURL, "/") != strings.TrimRight(currentBaseURL, "/") || assignment.SecretPurpose != currentPurpose || (assignment.EndpointRevision > 0 && assignment.EndpointRevision != endpointRevision)
	if assignment.SecretPurpose != "" && endpointChanged {
		if credentialRevision != assignment.CredentialRevision || (ciphertext != nil && assignment.CredentialRevision <= 0) {
			return errors.New("provider_snapshot_credential_destination_changed")
		}
	}
	secret := ""
	if ciphertext != nil {
		secret, err = decryptSecret(a.Provider.SettingsKey, purpose, nonce, ciphertext)
		if err != nil {
			return err
		}
	}
	assignment.Secret, assignment.CredentialRevision = secret, credentialRevision
	return nil
}

func providerSamplingFields(assignment providerAssignment) map[string]any {
	fields := map[string]any{}
	if assignment.Temperature != nil {
		fields["temperature"] = *assignment.Temperature
	}
	if assignment.TopP != nil {
		fields["top_p"] = *assignment.TopP
	}
	return fields
}

// Model configuration provenance is diagnostic metadata, never prompt input.
// Legacy persisted assignments lack revisions and remain explicitly unknown.
func providerAssignmentDiagnostics(assignment providerAssignment) map[string]any {
	result := map[string]any{"binding_role": assignment.Role, "endpoint_id": assignment.EndpointID, "model_id": assignment.ModelID, "timeout_ms": assignment.Timeout.Milliseconds()}
	if assignment.SourceBindingRole == "" {
		result["source_status"] = "unrecorded_snapshot"
	} else {
		result["source_status"] = "recorded"
		result["source_binding_role"] = assignment.SourceBindingRole
	}
	for key, revision := range map[string]int64{"binding_revision": assignment.BindingRevision, "endpoint_revision": assignment.EndpointRevision, "credential_revision": assignment.CredentialRevision} {
		if revision > 0 {
			result[key] = revision
		}
	}
	for key, value := range providerSamplingFields(assignment) {
		result[key] = value
	}
	return result
}

func (a *App) replyVisionCapabilityStatus(ctx context.Context, values map[string]any) map[string]any {
	reply, replyErr := decodeConversationReplySettings(jsonBytes(values[conversationReplySettingKey]))
	vision, visionErr := decodeVisionUnderstandingSettings(jsonBytes(values[visionUnderstandingSettingKey]))
	result := map[string]any{}
	for _, item := range []struct {
		role      string
		enabled   bool
		ref       string
		configErr error
	}{
		{conversationReplyModelRole, reply.Mode == "fact_only" || reply.Mode == "direct", reply.ModelRef, replyErr},
		{visionUnderstandingModelRole, vision.Enabled, vision.ModelRef, visionErr},
	} {
		status := map[string]any{"state": "disabled", "config_complete": false, "last_call_status": "not_called", "last_error": ""}
		result[item.role] = status
		if item.configErr != nil {
			status["state"] = "unavailable"
			status["last_error"] = item.configErr.Error()
			continue
		}
		if item.enabled {
			status["state"] = "unconfigured"
		}
		if item.ref == "" {
			continue
		}
		var model, endpointStatus string
		err := a.configurationReader(ctx).QueryRow(ctx, `SELECT r.model_id,e.capability_status FROM public.model_roles r JOIN public.provider_endpoints e ON e.id=r.provider_endpoint_id WHERE r.role=$1`, item.ref).Scan(&model, &endpointStatus)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			status["state"] = "unavailable"
			status["last_error"] = "capability_status_lookup_failed"
			continue
		}
		status["config_complete"] = true
		status["model_id"] = model
		if item.enabled {
			status["state"] = "ready"
		}
		if item.enabled && endpointStatus == "failed" {
			status["state"] = "unavailable"
		}
		var last, lastError string
		var at time.Time
		err = a.configurationReader(ctx).QueryRow(ctx, `SELECT status,COALESCE(error_code,''),created_at FROM public.diagnostic_model_runs WHERE binding_role=$1 ORDER BY created_at DESC,id DESC LIMIT 1`, item.ref).Scan(&last, &lastError, &at)
		if err == nil {
			status["last_call_status"] = last
			status["last_error"] = lastError
			status["last_call_at"] = at.UTC().Format(time.RFC3339Nano)
			if item.enabled && (last == "failed" || last == "timeout") {
				status["state"] = "unavailable"
			}
		} else if !errors.Is(err, pgx.ErrNoRows) {
			status["last_error"] = "capability_last_call_unavailable"
		}
	}
	return result
}
