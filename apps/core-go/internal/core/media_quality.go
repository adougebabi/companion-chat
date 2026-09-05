package core

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const (
	mediaQualityVerdictPass   = "pass"
	mediaQualityVerdictRetry  = "retry"
	mediaQualityVerdictReject = "reject"
	mediaQualityVerdictSkip   = "skipped"

	mediaQualitySchemaVersion = 1
	mediaQualityMaxViolations = 8
	mediaQualityMaxCodeLength = 128
	mediaQualityMaxDetail     = 512
	mediaQualityMaxGuidance   = 1200
	mediaQualityMaxImageBytes = 16 << 20
)

type mediaQualityViolation struct {
	Code     string
	Severity string
	Detail   string
}

type mediaQualityAcceptance struct {
	SchemaVersion int
	Verdict       string
	Violations    []mediaQualityViolation
	ObservedFacts map[string]bool
	RetryGuidance string
}

func normalizeMediaQualityAcceptance(value map[string]any) (mediaQualityAcceptance, error) {
	if value == nil || intValue(value["schema_version"]) != mediaQualitySchemaVersion {
		return mediaQualityAcceptance{}, errors.New("media quality schema version invalid")
	}
	verdict := stringValue(value["verdict"])
	if verdict != mediaQualityVerdictPass && verdict != mediaQualityVerdictRetry && verdict != mediaQualityVerdictReject {
		return mediaQualityAcceptance{}, errors.New("media quality verdict invalid")
	}
	rawViolations, ok := mediaQualityArray(value["violations"])
	if !ok || len(rawViolations) > mediaQualityMaxViolations {
		return mediaQualityAcceptance{}, errors.New("media quality violations invalid")
	}
	violations := make([]mediaQualityViolation, 0, len(rawViolations))
	for _, raw := range rawViolations {
		item := mapValue(raw)
		code := boundedMediaQualityText(item["code"], mediaQualityMaxCodeLength)
		severity := stringValue(item["severity"])
		detail := boundedMediaQualityText(item["detail"], mediaQualityMaxDetail)
		if code == "" || severity != "hard" || detail == "" {
			return mediaQualityAcceptance{}, errors.New("media quality violation invalid")
		}
		violations = append(violations, mediaQualityViolation{Code: code, Severity: severity, Detail: detail})
	}
	observed := mapValue(value["observed_facts"])
	observedFacts := make(map[string]bool, 5)
	for _, field := range []string{"subject_matches", "appearance_matches", "scene_matches", "capture_matches", "framing_matches"} {
		flag, ok := observed[field].(bool)
		if !ok {
			return mediaQualityAcceptance{}, errors.New("media quality observed facts invalid")
		}
		observedFacts[field] = flag
	}
	guidance := boundedMediaQualityText(value["retry_guidance"], mediaQualityMaxGuidance)
	if verdict == mediaQualityVerdictPass && len(violations) != 0 {
		return mediaQualityAcceptance{}, errors.New("media quality pass has violations")
	}
	if verdict == mediaQualityVerdictRetry && (len(violations) == 0 || guidance == "") {
		return mediaQualityAcceptance{}, errors.New("media quality retry guidance invalid")
	}
	return mediaQualityAcceptance{
		SchemaVersion: mediaQualitySchemaVersion,
		Verdict:       verdict,
		Violations:    violations,
		ObservedFacts: observedFacts,
		RetryGuidance: guidance,
	}, nil
}

func mediaQualityArray(value any) ([]any, bool) {
	switch typed := value.(type) {
	case []any:
		return typed, true
	case []map[string]any:
		items := make([]any, len(typed))
		for index, item := range typed {
			items[index] = item
		}
		return items, true
	default:
		return nil, false
	}
}

func boundedMediaQualityText(value any, max int) string {
	text := strings.TrimSpace(stringValue(value))
	if len([]rune(text)) > max {
		return ""
	}
	return text
}

func mediaQualityImageDataURL(contentType string, content []byte) (string, error) {
	if len(content) == 0 || len(content) > mediaQualityMaxImageBytes {
		return "", errors.New("media quality candidate image size invalid")
	}
	mime := strings.ToLower(strings.TrimSpace(strings.SplitN(contentType, ";", 2)[0]))
	switch mime {
	case "image/png", "image/jpeg", "image/webp", "image/gif":
	default:
		return "", fmt.Errorf("media quality candidate MIME %q unsupported", mime)
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(content), nil
}

func mediaQualityMessages(intent mediaIntent, contentType string, content []byte) ([]map[string]any, error) {
	dataURL, err := mediaQualityImageDataURL(contentType, content)
	if err != nil {
		return nil, err
	}
	textPayload := map[string]any{
		"media_kind":           intent.Kind,
		"frozen_media_concept": compactMediaConceptForProvider(intent.Prompt),
		"provider_prompt":      intent.ProviderPrompt,
		"quality_retry_count":  intent.QualityRetryCount,
	}
	textContent, err := json.Marshal(textPayload)
	if err != nil {
		return nil, err
	}
	contentParts := []any{
		map[string]any{"type": "text", "text": string(textContent)},
		map[string]any{"type": "image_url", "image_url": map[string]any{"url": dataURL}},
	}
	return []map[string]any{
		{"role": "system", "content": mediaQualityAcceptanceInstruction},
		{"role": "user", "content": contentParts},
	}, nil
}

func (a *App) evaluateMediaQuality(ctx context.Context, intent mediaIntent, contentType string, content []byte) (mediaQualityAcceptance, error) {
	if a == nil || a.Provider == nil {
		return mediaQualityAcceptance{}, errors.New("media quality provider unavailable")
	}
	messages, err := mediaQualityMessages(intent, contentType, content)
	if err != nil {
		return mediaQualityAcceptance{}, err
	}
	value, err := a.Provider.StructuredWithSchema(WithProviderScenario(ctx, "media_quality_acceptance"), "media_prompt", messages, "media_quality_acceptance_response", mediaQualityAcceptanceResponseSchema(), false)
	if err != nil {
		return mediaQualityAcceptance{}, err
	}
	return normalizeMediaQualityAcceptance(value)
}

func mediaQualityDiagnostic(result mediaQualityAcceptance, reason string, candidateSHA string, retryCount int) map[string]any {
	violations := make([]any, 0, len(result.Violations))
	for _, violation := range result.Violations {
		violations = append(violations, map[string]any{"code": violation.Code, "severity": violation.Severity, "detail": violation.Detail})
	}
	payload := map[string]any{
		"schema_version":      mediaQualitySchemaVersion,
		"verdict":             result.Verdict,
		"quality_retry_count": retryCount,
		"candidate_sha256":    candidateSHA,
		"violations":          violations,
		"observed_facts":      result.ObservedFacts,
		"retry_guidance":      result.RetryGuidance,
	}
	if reason != "" {
		payload["reason"] = reason
	}
	return payload
}

func mediaPromptInput(intent mediaIntent) string {
	if intent.QualityRetryCount == 0 || strings.TrimSpace(intent.QualityRetryGuidance) == "" {
		return compactMediaConceptForProvider(intent.Prompt)
	}
	return jsonString(map[string]any{
		"frozen_media_concept":     compactMediaConceptForProvider(intent.Prompt),
		"previous_provider_prompt": intent.ProviderPrompt,
		"quality_feedback": map[string]any{
			"retry_guidance": intent.QualityRetryGuidance,
		},
	})
}

func (a *App) persistMediaProviderPrompt(ctx context.Context, intentID, prompt string) error {
	command, err := a.DB.Pool().Exec(ctx, `UPDATE public.media_intents SET provider_prompt=$2,quality_verdict='',quality_candidate_sha256=NULL,quality_checked_at=NULL,revision=revision+1 WHERE id=$1 AND status IN ('pending','running')`, intentID, prompt)
	if err != nil {
		return err
	}
	if command.RowsAffected() != 1 {
		return errors.New("media provider prompt cannot be persisted")
	}
	return nil
}

func (a *App) persistMediaQualityVerdict(ctx context.Context, intentID, providerJobID, verdict, candidateSHA string) error {
	command, err := a.DB.Pool().Exec(ctx, `UPDATE public.media_intents SET quality_verdict=$2,quality_candidate_sha256=$3,quality_checked_at=now(),revision=revision+1 WHERE id=$1 AND provider_job_id=$4 AND status='running'`, intentID, verdict, candidateSHA, providerJobID)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 1 {
		return nil
	}
	return errors.New("media quality verdict cannot be persisted")
}

func (a *App) prepareMediaQualityRetry(ctx context.Context, intentID, providerJobID, guidance, candidateSHA string) error {
	command, err := a.DB.Pool().Exec(ctx, `UPDATE public.media_intents SET quality_retry_count=quality_retry_count+1,quality_retry_guidance=$2,quality_verdict='retry',quality_candidate_sha256=$3,quality_checked_at=now(),provider_job_id=NULL,status='pending',revision=revision+1 WHERE id=$1 AND provider_job_id=$4 AND status='running' AND quality_retry_count=0`, intentID, guidance, candidateSHA, providerJobID)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 1 {
		return nil
	}
	var count int
	var verdict, currentJob string
	if err := a.DB.Pool().QueryRow(ctx, `SELECT quality_retry_count,COALESCE(quality_verdict,''),COALESCE(provider_job_id,'') FROM public.media_intents WHERE id=$1`, intentID).Scan(&count, &verdict, &currentJob); err != nil {
		return err
	}
	if count == 1 && verdict == mediaQualityVerdictRetry && currentJob == "" {
		return nil
	}
	return errors.New("media quality retry cannot be prepared")
}

func (a *App) rejectMediaQuality(ctx context.Context, intentID, providerJobID, candidateSHA string) error {
	command, err := a.DB.Pool().Exec(ctx, `UPDATE public.media_intents SET quality_verdict='reject',quality_candidate_sha256=$2,quality_checked_at=now(),status='failed',revision=revision+1 WHERE id=$1 AND provider_job_id=$3 AND status='running'`, intentID, candidateSHA, providerJobID)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 1 {
		return nil
	}
	var status, verdict string
	if err := a.DB.Pool().QueryRow(ctx, `SELECT status,COALESCE(quality_verdict,'') FROM public.media_intents WHERE id=$1`, intentID).Scan(&status, &verdict); err != nil {
		return err
	}
	if status == "failed" && verdict == mediaQualityVerdictReject {
		return nil
	}
	return errors.New("media quality reject cannot be persisted")
}

func mediaQualityInfrastructureReason(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "quality_provider_timeout"
	}
	if errors.Is(err, context.Canceled) {
		return "quality_provider_cancelled"
	}
	return "quality_verification_unavailable"
}
