package core

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/minio/minio-go/v7"
	"go.temporal.io/sdk/activity"
)

type mediaIntent struct {
	ID, Owner, Prompt, ProviderPrompt, ProviderRequestID, ProviderJobID, WorkflowID string
	Kind, MimeType, Status, QualityRetryGuidance, QualityVerdict                    string
	QualityCandidateSHA                                                             string
	QualityRetryFeedback                                                            map[string]any
	QualityRetryCount                                                               int
	ConversationID, MessageID, MomentID                                             *string
}

func (a *App) ProcessMediaIntent(ctx context.Context, intentID string) (map[string]any, error) {
	stopHeartbeat := startMediaHeartbeat(ctx, intentID)
	defer stopHeartbeat()
	recordMediaHeartbeat(ctx, map[string]any{"intent_id": intentID, "phase": "loading"})
	intent, err := a.readMediaIntent(ctx, intentID)
	if err != nil {
		return nil, err
	}
	assetID := "asset_" + intent.ID
	var ready bool
	if err := a.DB.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.media_assets WHERE id=$1 AND status='ready')`, assetID).Scan(&ready); err != nil {
		return nil, err
	}
	if ready {
		if err := a.publishMediaAsset(ctx, intent, assetID); err != nil {
			return nil, err
		}
		if err := a.markMediaIntentCompleted(ctx, intent.ID, assetID); err != nil {
			return nil, err
		}
		return map[string]any{"intent_id": intent.ID, "status": "completed", "quality_verdict": intent.QualityVerdict}, nil
	}
	if intent.Status == "completed" {
		return map[string]any{"intent_id": intent.ID, "status": "completed", "quality_verdict": intent.QualityVerdict}, nil
	}
	if intent.Status == "failed" {
		return map[string]any{"intent_id": intent.ID, "status": "failed", "quality_verdict": intent.QualityVerdict}, nil
	}
	config, err := a.runtimeValue(ctx, "media.comfyui")
	if err != nil {
		return nil, err
	}
	baseURL, workflow, err := comfyConfig(config)
	if err != nil {
		return nil, err
	}
	workflow = selectComfyWorkflow(config, intent.Prompt, workflow)
	var concept map[string]any
	_ = json.Unmarshal([]byte(intent.Prompt), &concept)
	providerJobID := intent.ProviderJobID
	if providerJobID == "" {
		prompt := strings.TrimSpace(intent.ProviderPrompt)
		if stringValue(concept["purpose"]) == "visual_identity" {
			// Visual Identity is an exact owner template, not a request for the
			// generic media prompt model to rewrite. Persist the filled template
			// so retries and diagnostics use the same text.
			prompt = visualIdentityPromptFromConcept(concept)
			if prompt != strings.TrimSpace(intent.ProviderPrompt) {
				if err := a.persistMediaProviderPrompt(ctx, intent.ID, prompt); err != nil {
					return nil, err
				}
			}
		} else if prompt == "" || intent.QualityVerdict == mediaQualityVerdictRetry {
			recordMediaHeartbeat(ctx, map[string]any{"intent_id": intentID, "phase": "prompt"})
			providerCtx := WithProviderCorrelation(WithProviderScenario(ctx, "media_prompt"), "media:"+intent.ID)
			value, providerErr := a.RunMediaPromptTask(providerCtx, MediaPromptTaskInput{Intent: intent})
			if providerErr != nil || strings.TrimSpace(value) == "" {
				if providerErr != nil {
					return nil, fmt.Errorf("media prompt generation failed: %w", providerErr)
				}
				return nil, errors.New("media prompt generation returned empty text")
			}
			prompt = strings.TrimSpace(value)
			if err := a.persistMediaProviderPrompt(ctx, intent.ID, prompt); err != nil {
				return nil, err
			}
			intent.ProviderPrompt = prompt
			intent.QualityVerdict = ""
			intent.QualityCandidateSHA = ""
		}
		recordMediaHeartbeat(ctx, map[string]any{"intent_id": intentID, "phase": "submit"})
		constraints := map[string]any{}
		if json.Unmarshal([]byte(intent.Prompt), &concept) == nil {
			constraints = mediaRendererConstraints(concept)
		}
		referenceImageFilename := ""
		if mediaWorkflowNeedsVisualIdentityReference(workflow) {
			referenceAssetID := visualIdentityReferenceAssetID(concept)
			if referenceAssetID == "" {
				return nil, errors.New("visual_identity_reference_image_missing")
			}
			mimeType, content, contentErr := a.readMediaAssetContent(ctx, intent.Owner, referenceAssetID)
			if contentErr != nil {
				return nil, contentErr
			}
			referenceImageFilename, err = a.uploadComfyReferenceImage(ctx, baseURL, referenceAssetID, mimeType, content)
			if err != nil {
				return nil, err
			}
			a.recordDiagnosticEvent(ctx, "media.comfyui.reference_uploaded", "info", intent.Owner, intent.ProviderRequestID, "media:"+intent.ID, map[string]any{
				"media_intent_id": intent.ID,
				"asset_id":        referenceAssetID,
				"filename":        referenceImageFilename,
			})
		}
		workflow, err = replaceMediaPlaceholdersWithReference(workflow, prompt, constraints, referenceImageFilename)
		if err != nil {
			return nil, err
		}
		payload, _ := json.Marshal(map[string]any{"prompt": workflow})
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/prompt", bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		request.Header.Set("Content-Type", "application/json")
		a.recordDiagnosticEvent(ctx, "media.comfyui.prompt_submitted", "info", intent.Owner, intent.ProviderRequestID, "media:"+intent.ID, mediaComfyPromptSubmissionDiagnostic(intent, concept, prompt, workflow))
		client := a.Provider.HTTP
		if client == nil {
			client = &http.Client{Timeout: 30 * time.Second}
		}
		response, err := client.Do(request)
		if err != nil {
			return nil, err
		}
		data, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		response.Body.Close()
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			detail := strings.TrimSpace(string(data))
			if len(detail) > 512 {
				detail = detail[:512]
			}
			return nil, fmt.Errorf("ComfyUI returned HTTP %d: %s", response.StatusCode, detail)
		}
		var result map[string]any
		if err := json.Unmarshal(data, &result); err != nil {
			return nil, err
		}
		providerJobID = stringValue(result["prompt_id"])
		if providerJobID == "" {
			return nil, errors.New("ComfyUI prompt ID is missing")
		}
		if _, err := a.DB.Pool().Exec(ctx, `UPDATE public.media_intents SET provider_job_id=$2,status='running' WHERE id=$1`, intent.ID, providerJobID); err != nil {
			return nil, err
		}
		intent.ProviderJobID = providerJobID
		intent.Status = "running"
	}
	var output map[string]any
	for attempt := 0; attempt < 900; attempt++ {
		recordMediaHeartbeat(ctx, map[string]any{"provider_job_id": providerJobID, "attempt": attempt})
		value, done, err := pollComfy(ctx, baseURL, providerJobID)
		if err != nil {
			return nil, err
		}
		if done {
			output = value
			break
		}
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
	if output == nil {
		return nil, errors.New("media generation timed out")
	}
	contentType, content, err := downloadComfy(ctx, baseURL, output)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(content)
	candidateSHA := hex.EncodeToString(digest[:])
	quality := mediaQualityAcceptance{}
	qualityCheckVerdict := ""
	if (intent.QualityVerdict == mediaQualityVerdictPass || intent.QualityVerdict == mediaQualityVerdictSkip) && intent.QualityCandidateSHA == candidateSHA {
		quality.Verdict = intent.QualityVerdict
		qualityCheckVerdict = quality.Verdict
	} else {
		quality, err = a.evaluateMediaQuality(ctx, intent, contentType, content)
		if err != nil {
			if errors.Is(err, context.Canceled) && ctx.Err() != nil {
				return nil, err
			}
			quality = mediaQualityAcceptance{SchemaVersion: mediaQualitySchemaVersion, Verdict: mediaQualityVerdictSkip}
			qualityCheckVerdict = quality.Verdict
			reason := mediaQualityInfrastructureReason(err)
			a.recordDiagnosticEvent(ctx, "media.quality.acceptance", "warn", intent.Owner, "media:"+intent.ID, intent.ProviderRequestID, mediaQualityDiagnostic(quality, reason, candidateSHA, intent.QualityRetryCount))
			if err := a.persistMediaQualityVerdict(ctx, intent.ID, providerJobID, mediaQualityVerdictSkip, candidateSHA); err != nil {
				return nil, err
			}
			intent.QualityVerdict = mediaQualityVerdictSkip
			intent.QualityCandidateSHA = candidateSHA
		} else {
			qualityCheckVerdict = quality.Verdict
			a.recordDiagnosticEvent(ctx, "media.quality.acceptance", "info", intent.Owner, "media:"+intent.ID, intent.ProviderRequestID, mediaQualityDiagnostic(quality, "", candidateSHA, intent.QualityRetryCount))
			switch mediaQualityDisposition(intent.QualityRetryCount, quality.Verdict) {
			case mediaQualityVerdictPass:
				if err := a.persistMediaQualityVerdict(ctx, intent.ID, providerJobID, quality.Verdict, candidateSHA); err != nil {
					return nil, err
				}
				intent.QualityVerdict = quality.Verdict
				intent.QualityCandidateSHA = candidateSHA
			case mediaQualityActionRetry:
				feedback := mediaQualityRetryFeedback(quality)
				if err := a.prepareMediaQualityRetry(ctx, intent.ID, providerJobID, quality.RetryGuidance, feedback, candidateSHA); err != nil {
					return nil, err
				}
				return map[string]any{"intent_id": intent.ID, "status": "quality_retry", "quality_retry_count": intent.QualityRetryCount + 1}, nil
			case mediaQualityActionAcceptAfterRetry:
				actualVerdict := quality.Verdict
				if err := a.acceptMediaQualityAfterRetry(ctx, intent.ID, providerJobID, candidateSHA); err != nil {
					return nil, err
				}
				finalDiagnostic := mediaQualityDiagnostic(quality, "", candidateSHA, intent.QualityRetryCount)
				finalDiagnostic["actual_verdict"] = actualVerdict
				finalDiagnostic["delivery_verdict"] = mediaQualityVerdictRetryAccepted
				a.recordDiagnosticEvent(ctx, "media.quality.accepted_after_retry", "warn", intent.Owner, "media:"+intent.ID, intent.ProviderRequestID, finalDiagnostic)
				quality.Verdict = mediaQualityVerdictRetryAccepted
				intent.QualityVerdict = mediaQualityVerdictRetryAccepted
				intent.QualityCandidateSHA = candidateSHA
			default:
				return nil, errors.New("media quality verdict invalid")
			}
		}
	}
	objectKey := "media/" + assetID + "/v1"
	_, err = a.Storage.PutObject(ctx, a.S3Bucket, objectKey, bytes.NewReader(content), int64(len(content)), minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return nil, err
	}
	version := "v1"
	workflowID := intent.WorkflowID
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO public.media_assets (id,owner_fluctlight_id,version,kind,mime_type,byte_size,sha256,bucket,object_key,provider_request_id,workflow_id,status,ready_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'ready',now()) ON CONFLICT (id) DO NOTHING`, assetID, intent.Owner, version, intent.Kind, contentType, len(content), candidateSHA, a.S3Bucket, objectKey, intent.ProviderRequestID, workflowID); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		// Object storage succeeded but the authoritative asset transaction did
		// not. Preserve the ambiguity on the existing call outcome; the workflow
		// retries with the same intent/provider IDs and reconciles before final
		// completion instead of treating the asset as certainly absent.
		_ = a.markActionOutcomeUnknown(ctx, intent.ID, "media_asset_persistence_ambiguous")
		return nil, err
	}
	if err := a.publishMediaAsset(ctx, intent, assetID); err != nil {
		return nil, err
	}
	if err := a.markMediaIntentCompleted(ctx, intent.ID, assetID); err != nil {
		return nil, err
	}
	return map[string]any{
		"intent_id": intent.ID, "status": "completed", "quality_verdict": quality.Verdict,
		"quality_check_verdict": qualityCheckVerdict,
	}, nil
}

// mediaRendererConstraints resolves the renderer-owned values from both
// durable media concept shapes. Newer cognition calls keep the authoritative
// visual identity under context_binding, while Visual Identity jobs also
// persist the same constraints at the concept root. Merge them so both the
// ordinary and Visual Identity ComfyUI workflows receive the LoRA weight.
func mediaRendererConstraints(concept map[string]any) map[string]any {
	result := cloneMap(mapValue(concept["renderer_constraints"]))
	if len(result) == 0 {
		result = make(map[string]any)
	}
	binding := mapValue(concept["context_binding"])
	visualIdentity := mapValue(binding["visual_identity"])
	nested := mapValue(visualIdentity["renderer_constraints"])
	if len(nested) == 0 {
		nested = mapValue(mapValue(concept["visual_identity"])["renderer_constraints"])
	}
	for key, value := range nested {
		if current, exists := result[key]; !exists || current == nil || current == "" {
			result[key] = value
		}
	}
	if seedVal, exists := concept["seed"]; exists && seedVal != nil {
		if _, has := result["seed"]; !has {
			result["seed"] = seedVal
		}
	}
	return result
}

func mediaComfyPromptSubmissionDiagnostic(intent mediaIntent, concept map[string]any, providerPrompt string, workflow map[string]any) map[string]any {
	boundedPrompt := visualIdentityBoundedText(providerPrompt, 4000)
	return map[string]any{
		"media_intent_id":     intent.ID,
		"provider_request_id": intent.ProviderRequestID,
		"workflow_id":         intent.WorkflowID,
		"stage":               stringValue(concept["stage"]),
		// Keep prompt for compatibility with the existing event reader; the
		// request_payload field is the authoritative ComfyUI HTTP body.
		"prompt":          boundedPrompt,
		"provider_prompt": boundedPrompt,
		"request_payload": map[string]any{"prompt": workflow},
	}
}

func (a *App) markMediaIntentCompleted(ctx context.Context, intentID, assetID string) error {
	return withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var owner, prompt, status string
		if err := tx.QueryRow(ctx, `SELECT owner_fluctlight_id,prompt,status FROM public.media_intents WHERE id=$1 FOR UPDATE`, intentID).Scan(&owner, &prompt, &status); err != nil {
			return err
		}
		if status != "pending" && status != "running" && status != "completed" {
			return fmt.Errorf("media intent cannot complete from status %s", status)
		}
		observed := map[string]any{"media_intent_id": intentID, "asset_id": assetID, "delivery_status": "asset_ready"}
		if status != "completed" {
			var captureBody, captureWardrobe *int
			appearance := mapValue(mapValue(decodeObject([]byte(prompt))["context_binding"])["appearance"])
			if raw, exists := appearance["body_revision"]; exists {
				value := intValue(raw)
				captureBody = &value
			}
			if raw, exists := appearance["wardrobe_revision"]; exists {
				value := intValue(raw)
				captureWardrobe = &value
			}
			var stale *bool
			if captureBody != nil || captureWardrobe != nil {
				var currentBody, currentWardrobe int
				// Hold both authority rows until the completion commit. A concurrent
				// haircut or wear cannot commit between this comparison and the
				// media intent's stale marker.
				if err := tx.QueryRow(ctx, `SELECT revision FROM public.fluctlight_appearance_states WHERE fluctlight_id=$1 FOR SHARE`, owner).Scan(&currentBody); err != nil {
					return err
				}
				if err := tx.QueryRow(ctx, `SELECT revision FROM public.fluctlight_wardrobe_states WHERE fluctlight_id=$1 FOR SHARE`, owner).Scan(&currentWardrobe); err != nil {
					return err
				}
				changed := (captureBody != nil && *captureBody != currentBody) || (captureWardrobe != nil && *captureWardrobe != currentWardrobe)
				stale = &changed
				observed["context_stale_at_completion"] = changed
			}
			command, err := tx.Exec(ctx, `UPDATE public.media_intents SET status='completed',revision=revision+1,context_stale_at_completion=$2,capture_body_revision=$3,capture_wardrobe_revision=$4 WHERE id=$1 AND status IN ('pending','running')`, intentID, stale, captureBody, captureWardrobe)
			if err != nil || command.RowsAffected() != 1 {
				if err == nil {
					err = ErrConflict
				}
				return err
			}
		}
		_, err := a.settleActionOutcomeByExternalRefTx(ctx, tx, intentID, ActionOutcomeCompleted, observed, "")
		return err
	})
}

// startMediaHeartbeat keeps the activity lease alive while the provider prompt
// call, object download, or other non-polling step is in flight. The polling
// loop below also records detailed progress, but without this guard a slow
// local Provider can exceed the 30-second heartbeat timeout before the first
// poll and Temporal retries the activity unnecessarily.
func startMediaHeartbeat(ctx context.Context, intentID string) func() {
	if !activity.IsActivity(ctx) {
		return func() {}
	}
	heartbeatCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-heartbeatCtx.Done():
				return
			case <-ticker.C:
				recordMediaHeartbeat(heartbeatCtx, map[string]any{"intent_id": intentID, "phase": "in-flight"})
			}
		}
	}()
	return func() {
		cancel()
		<-done
	}
}

// recordMediaHeartbeat is safe for both Temporal activity execution and
// direct Core invocation (tests, local recovery, and administrative retries).
// The Temporal SDK panics when RecordHeartbeat receives a non-activity
// context, so every media heartbeat must pass through this guard.
func recordMediaHeartbeat(ctx context.Context, details any) {
	if activity.IsActivity(ctx) {
		activity.RecordHeartbeat(ctx, details)
	}
}

func (a *App) readMediaIntent(ctx context.Context, intentID string) (mediaIntent, error) {
	var i mediaIntent
	var qualityRetryFeedback []byte
	err := a.DB.Pool().QueryRow(ctx, `SELECT id,owner_fluctlight_id,prompt,COALESCE(provider_prompt,''),provider_request_id,COALESCE(provider_job_id,''),workflow_id,kind,mime_type,status,quality_retry_count,COALESCE(quality_retry_guidance,''),COALESCE(quality_retry_feedback,'{}'::jsonb),COALESCE(quality_verdict,''),COALESCE(quality_candidate_sha256,''),conversation_id,message_id,moment_id FROM public.media_intents WHERE id=$1`, intentID).Scan(&i.ID, &i.Owner, &i.Prompt, &i.ProviderPrompt, &i.ProviderRequestID, &i.ProviderJobID, &i.WorkflowID, &i.Kind, &i.MimeType, &i.Status, &i.QualityRetryCount, &i.QualityRetryGuidance, &qualityRetryFeedback, &i.QualityVerdict, &i.QualityCandidateSHA, &i.ConversationID, &i.MessageID, &i.MomentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return i, ErrNotFound
	}
	if err == nil && len(qualityRetryFeedback) > 0 {
		if decodeErr := json.Unmarshal(qualityRetryFeedback, &i.QualityRetryFeedback); decodeErr != nil {
			return i, decodeErr
		}
	}
	return i, err
}

func (a *App) runtimeValue(ctx context.Context, key string) (map[string]any, error) {
	var raw string
	if err := a.DB.Pool().QueryRow(ctx, `SELECT value_json FROM public.runtime_settings WHERE key=$1`, key).Scan(&raw); err != nil {
		return nil, err
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, err
	}
	return result, nil
}

func comfyConfig(value map[string]any) (string, map[string]any, error) {
	base := stringValue(value["baseUrl"])
	if base == "" {
		base = stringValue(value["base_url"])
	}
	if base == "" {
		return "", nil, errors.New("media.comfyui base URL is missing")
	}
	if _, err := url.Parse(base); err != nil {
		return "", nil, err
	}
	workflow := mapValue(value["workflow"])
	if len(workflow) == 0 {
		return "", nil, errors.New("media.comfyui workflow is missing")
	}
	return strings.TrimRight(base, "/"), workflow, nil
}

// selectComfyWorkflow chooses an explicitly named workflow variant from the
// persisted media settings. Visual Identity concepts carry their purpose/stage
// as structured JSON, so this switch never parses natural-language prompts or
// guesses a renderer from wording. The existing workflow remains the fallback
// for legacy Scene Image jobs.
func selectComfyWorkflow(config map[string]any, prompt string, fallback map[string]any) map[string]any {
	var concept map[string]any
	if json.Unmarshal([]byte(prompt), &concept) != nil {
		return fallback
	}
	if stringValue(concept["purpose"]) != "visual_identity" {
		return fallback
	}
	stage := stringValue(concept["stage"])
	for _, key := range []string{"visual_identity_workflow", "visual_identity_workflows"} {
		variants := mapValue(config[key])
		if stage != "" {
			if selected := mapValue(variants[stage]); len(selected) > 0 {
				return selected
			}
		}
		if len(variants) > 0 && key == "visual_identity_workflow" {
			return variants
		}
	}
	return fallback
}

func replacePrompt(value map[string]any, prompt string) map[string]any {
	result, _ := replaceMediaPlaceholders(value, prompt, nil)
	return result
}

func replaceMediaPlaceholders(value map[string]any, prompt string, constraints map[string]any) (map[string]any, error) {
	return replaceMediaPlaceholdersWithReference(value, prompt, constraints, "")
}

func replaceMediaPlaceholdersWithReference(value map[string]any, prompt string, constraints map[string]any, referenceImageFilename string) (map[string]any, error) {
	seed := resolveMediaSeed(constraints)
	return replaceMediaPlaceholdersWithReferenceAndSeed(value, prompt, constraints, referenceImageFilename, seed)
}

func replaceMediaPlaceholdersWithReferenceAndSeed(value map[string]any, prompt string, constraints map[string]any, referenceImageFilename string, seed int64) (map[string]any, error) {
	if strings.Contains(prompt, "{{seed}}") {
		prompt = strings.ReplaceAll(prompt, "{{seed}}", strconv.FormatInt(seed, 10))
	}
	result := make(map[string]any, len(value))
	for key, child := range value {
		replaced, err := replaceMediaPlaceholderValueWithSeed(child, prompt, constraints, referenceImageFilename, seed)
		if err != nil {
			return nil, err
		}
		result[key] = replaced
	}
	return result, nil
}

func replaceMediaPlaceholderValue(value any, prompt string, constraints map[string]any, referenceImageFilename string) (any, error) {
	return replaceMediaPlaceholderValueWithSeed(value, prompt, constraints, referenceImageFilename, resolveMediaSeed(constraints))
}

func replaceMediaPlaceholderValueWithSeed(value any, prompt string, constraints map[string]any, referenceImageFilename string, seed int64) (any, error) {
	switch typed := value.(type) {
	case string:
		if typed == "{{prompt}}" {
			return prompt, nil
		}
		if typed == "{{seed}}" || typed == "{{renderer_constraints.seed}}" {
			return seed, nil
		}
		if typed == "{{visual_identity_reference_image}}" {
			if referenceImageFilename == "" {
				return nil, errors.New("visual_identity_reference_image_missing")
			}
			return referenceImageFilename, nil
		}
		if typed == "{{chest_lora_weight}}" || typed == "{{renderer_constraints.chest_lora_weight}}" {
			weight, ok := rendererConstraintWeight(constraints)
			if !ok {
				return nil, errors.New("chest_lora_weight_missing")
			}
			return weight, nil
		}
		res := typed
		hasPrompt := strings.Contains(res, "{{prompt}}")
		hasSeed := strings.Contains(res, "{{seed}}") || strings.Contains(res, "{{renderer_constraints.seed}}")
		hasWeight := strings.Contains(res, "{{chest_lora_weight}}") || strings.Contains(res, "{{renderer_constraints.chest_lora_weight}}")
		if hasPrompt || hasSeed || hasWeight {
			if hasPrompt {
				res = strings.ReplaceAll(res, "{{prompt}}", prompt)
			}
			if hasSeed {
				formattedSeed := strconv.FormatInt(seed, 10)
				res = strings.ReplaceAll(res, "{{seed}}", formattedSeed)
				res = strings.ReplaceAll(res, "{{renderer_constraints.seed}}", formattedSeed)
			}
			if hasWeight {
				weight, ok := rendererConstraintWeight(constraints)
				if !ok {
					return nil, errors.New("chest_lora_weight_missing")
				}
				formattedWeight := strconv.FormatFloat(weight, 'f', -1, 64)
				res = strings.ReplaceAll(res, "{{chest_lora_weight}}", formattedWeight)
				res = strings.ReplaceAll(res, "{{renderer_constraints.chest_lora_weight}}", formattedWeight)
			}
			return res, nil
		}
		return typed, nil
	case map[string]any:
		return replaceMediaPlaceholdersWithReferenceAndSeed(typed, prompt, constraints, referenceImageFilename, seed)
	case []any:
		items := make([]any, len(typed))
		for index, item := range typed {
			replaced, err := replaceMediaPlaceholderValueWithSeed(item, prompt, constraints, referenceImageFilename, seed)
			if err != nil {
				return nil, err
			}
			items[index] = replaced
		}
		return items, nil
	default:
		return value, nil
	}
}

func resolveMediaSeed(constraints map[string]any) int64 {
	if constraints != nil {
		if s, ok := int64Value(constraints["seed"]); ok && s > 0 {
			return s
		}
	}
	return randomMediaSeed()
}

func randomMediaSeed() int64 {
	max := big.NewInt(9007199254740991)
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return time.Now().UnixNano()&0x1FFFFFFFFFFFFF + 1
	}
	return n.Int64() + 1
}

func int64Value(value any) (int64, bool) {
	switch v := value.(type) {
	case int64:
		return v, true
	case int:
		return int64(v), true
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 || v > 9007199254740991 {
			return 0, false
		}
		return int64(v), true
	case json.Number:
		if i, err := v.Int64(); err == nil && i > 0 {
			return i, true
		}
		if f, err := v.Float64(); err == nil && !math.IsNaN(f) && !math.IsInf(f, 0) && f > 0 && f <= 9007199254740991 {
			return int64(f), true
		}
	case string:
		if i, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64); err == nil && i > 0 {
			return i, true
		}
	}
	return 0, false
}

func mediaWorkflowNeedsVisualIdentityReference(value any) bool {
	switch typed := value.(type) {
	case string:
		return typed == "{{visual_identity_reference_image}}"
	case map[string]any:
		for _, child := range typed {
			if mediaWorkflowNeedsVisualIdentityReference(child) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if mediaWorkflowNeedsVisualIdentityReference(child) {
				return true
			}
		}
	}
	return false
}

func visualIdentityReferenceAssetID(concept map[string]any) string {
	for _, key := range []string{"visual_identity_reference_asset_id", "reference_asset_id", "input_image_asset_id"} {
		if value := stringValue(concept[key]); value != "" {
			return value
		}
	}
	for _, value := range []any{concept["context_binding"], concept["visual_identity"]} {
		visualIdentity := mapValue(mapValue(value)["visual_identity"])
		if len(visualIdentity) == 0 {
			visualIdentity = mapValue(value)
		}
		for _, key := range []string{"reference_asset_id", "character_sheet_asset_id", "canonical_asset_id"} {
			if assetID := stringValue(visualIdentity[key]); assetID != "" {
				return assetID
			}
		}
	}
	for _, key := range []string{"character_sheet_asset_id", "canonical_asset_id"} {
		if assetID := stringValue(concept[key]); assetID != "" {
			return assetID
		}
	}
	return ""
}

func (a *App) readMediaAssetContent(ctx context.Context, owner, assetID string) (string, []byte, error) {
	if a.Storage == nil || strings.TrimSpace(assetID) == "" {
		return "", nil, errors.New("visual_identity_reference_storage_unavailable")
	}
	var bucket, objectKey, mimeType string
	query := `SELECT bucket,object_key,mime_type FROM public.media_assets WHERE id=$1 AND status='ready'`
	args := []any{assetID}
	if strings.TrimSpace(owner) != "" {
		query = `SELECT bucket,object_key,mime_type FROM public.media_assets WHERE id=$1 AND owner_fluctlight_id=$2 AND status='ready'`
		args = append(args, owner)
	}
	if err := a.DB.Pool().QueryRow(ctx, query, args...).Scan(&bucket, &objectKey, &mimeType); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil, ErrNotFound
		}
		return "", nil, err
	}
	object, err := a.Storage.GetObject(ctx, bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return "", nil, err
	}
	defer object.Close()
	content, err := io.ReadAll(io.LimitReader(object, 32<<20))
	if err != nil {
		return "", nil, err
	}
	if len(content) == 0 {
		return "", nil, errors.New("visual_identity_reference_image_empty")
	}
	return mimeType, content, nil
}

func (a *App) uploadComfyReferenceImage(ctx context.Context, baseURL, assetID, mimeType string, content []byte) (string, error) {
	if strings.TrimSpace(baseURL) == "" || len(content) == 0 {
		return "", errors.New("visual_identity_reference_upload_invalid")
	}
	extension := ".png"
	switch strings.ToLower(strings.TrimSpace(mimeType)) {
	case "image/jpeg", "image/jpg":
		extension = ".jpg"
	case "image/webp":
		extension = ".webp"
	}
	filename := "visual_identity_reference_" + stableDigest(assetID) + extension
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("type", "input"); err != nil {
		return "", err
	}
	if err := writer.WriteField("overwrite", "true"); err != nil {
		return "", err
	}
	part, err := writer.CreateFormFile("image", filename)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(content); err != nil {
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(baseURL, "/")+"/upload/image", &body)
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", writer.FormDataContentType())
	client := a.Provider.HTTP
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	data, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		detail := visualIdentityBoundedText(strings.TrimSpace(string(data)), 512)
		return "", fmt.Errorf("ComfyUI reference image upload returned HTTP %d: %s", response.StatusCode, detail)
	}
	var result struct {
		Name      string `json:"name"`
		Subfolder string `json:"subfolder"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", err
	}
	if strings.TrimSpace(result.Name) == "" {
		return "", errors.New("ComfyUI reference image filename is missing")
	}
	if strings.Trim(strings.TrimSpace(result.Subfolder), "/") != "" {
		return strings.Trim(strings.TrimSpace(result.Subfolder), "/") + "/" + result.Name, nil
	}
	return result.Name, nil
}

func rendererConstraintWeight(constraints map[string]any) (float64, bool) {
	if constraints == nil {
		return 0, false
	}
	value, ok := numberFloat(constraints["chest_lora_weight"])
	if !ok || math.IsNaN(value) || math.IsInf(value, 0) || value < -10 || value > 10 {
		return 0, false
	}
	return value, true
}

func pollComfy(ctx context.Context, baseURL, jobID string) (map[string]any, bool, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/history/"+url.PathEscape(jobID), nil)
	if err != nil {
		return nil, false, err
	}
	response, err := (&http.Client{Timeout: 30 * time.Second}).Do(request)
	if err != nil {
		return nil, false, err
	}
	defer response.Body.Close()
	if response.StatusCode == 404 {
		return nil, false, nil
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if err != nil {
		return nil, false, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, false, fmt.Errorf("ComfyUI history returned HTTP %d", response.StatusCode)
	}
	var history map[string]any
	if err := json.Unmarshal(data, &history); err != nil {
		return nil, false, err
	}
	entry := mapValue(history[jobID])
	status := mapValue(entry["status"])
	if value := stringValue(status["status_str"]); value == "error" || value == "failed" || value == "cancelled" {
		return nil, false, fmt.Errorf("ComfyUI job failed")
	}
	outputs := mapValue(entry["outputs"])
	for _, node := range outputs {
		nodeMap := mapValue(node)
		for _, kind := range []string{"images", "videos", "audio"} {
			files := arrayValue(nodeMap[kind])
			if len(files) > 0 {
				return mapValue(files[0]), true, nil
			}
		}
	}
	return nil, false, nil
}

func downloadComfy(ctx context.Context, baseURL string, output map[string]any) (string, []byte, error) {
	filename := stringValue(output["filename"])
	if filename == "" {
		return "", nil, errors.New("ComfyUI output filename is missing")
	}
	query := url.Values{"filename": {filename}, "subfolder": {stringValue(output["subfolder"])}, "type": {firstString(output["type"], "output")}}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/view?"+query.Encode(), nil)
	if err != nil {
		return "", nil, err
	}
	response, err := (&http.Client{Timeout: 30 * time.Second}).Do(request)
	if err != nil {
		return "", nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", nil, fmt.Errorf("ComfyUI output returned HTTP %d", response.StatusCode)
	}
	content, err := io.ReadAll(io.LimitReader(response.Body, 32<<20))
	if err != nil {
		return "", nil, err
	}
	if len(content) == 0 {
		return "", nil, errors.New("ComfyUI output is empty")
	}
	contentType := strings.Split(response.Header.Get("Content-Type"), ";")[0]
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	return contentType, content, nil
}

func (a *App) publishMediaAsset(ctx context.Context, intent mediaIntent, assetID string) error {
	if intent.MessageID != nil {
		return withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
			var conversationID string
			var attachments []byte
			if err := tx.QueryRow(ctx, `SELECT conversation_id,attachment_refs FROM public.conversation_messages WHERE id=$1 FOR UPDATE`, *intent.MessageID).Scan(&conversationID, &attachments); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return fmt.Errorf("media target message not found: %w", ErrNotFound)
				}
				return err
			}
			existingRefs := decodeArray(attachments)
			assetRefs := appendUniqueAssetRef(existingRefs, assetID)
			if !containsStringValue(existingRefs, assetID) {
				if _, err := tx.Exec(ctx, `UPDATE public.conversation_messages SET attachment_refs=$2 WHERE id=$1`, *intent.MessageID, jsonBytes(assetRefs)); err != nil {
					return err
				}
			}
			_, err := tx.Exec(ctx, `INSERT INTO public.media_references (id,asset_id,owner_fluctlight_id,target_type,target_id) VALUES ($1,$2,$3,'conversation_message',$4) ON CONFLICT DO NOTHING`, "media_ref_"+stableDigest(assetID+":conversation_message:"+*intent.MessageID), assetID, intent.Owner, *intent.MessageID)
			return err
		})
	}
	if intent.ConversationID != nil {
		err := withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
			var existing string
			if err := tx.QueryRow(ctx, `SELECT id FROM public.conversation_messages WHERE conversation_id=$1 AND idempotency_key=$2`, *intent.ConversationID, "media:"+intent.ID+":conversation-result").Scan(&existing); err == nil {
				return nil
			} else if !errors.Is(err, pgx.ErrNoRows) {
				return err
			}
			var seq int
			if err := tx.QueryRow(ctx, `SELECT next_sequence FROM public.conversation_heads WHERE conversation_id=$1 FOR UPDATE`, *intent.ConversationID).Scan(&seq); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE public.conversation_heads SET next_sequence=$2 WHERE conversation_id=$1`, *intent.ConversationID, seq+1); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `INSERT INTO public.conversation_messages (id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key) VALUES ($1,$2,$3,$4,'media_reference','图片已生成。',$5,$6)`, randomID("message_"), *intent.ConversationID, seq, intent.Owner, jsonBytes([]string{assetID}), "media:"+intent.ID+":conversation-result")
			if err == nil {
				_, err = tx.Exec(ctx, `INSERT INTO public.media_references (id,asset_id,owner_fluctlight_id,target_type,target_id) VALUES ($1,$2,$3,'conversation',$4) ON CONFLICT DO NOTHING`, "media_ref_"+stableDigest(intent.ID+":conversation"), assetID, intent.Owner, *intent.ConversationID)
			}
			return err
		})
		if err != nil {
			return err
		}
	}
	if intent.MomentID != nil {
		return withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
			var existing []byte
			if err := tx.QueryRow(ctx, `SELECT COALESCE(media_asset_ids,'[]'::jsonb) FROM public.moments WHERE id=$1 FOR UPDATE`, *intent.MomentID).Scan(&existing); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return fmt.Errorf("media target moment not found: %w", ErrNotFound)
				}
				return err
			}
			existingRefs := decodeArray(existing)
			assetRefs := appendUniqueAssetRef(existingRefs, assetID)
			if !containsStringValue(existingRefs, assetID) {
				if _, err := tx.Exec(ctx, `UPDATE public.moments SET media_asset_ids=$2 WHERE id=$1`, *intent.MomentID, jsonBytes(assetRefs)); err != nil {
					return err
				}
			}
			_, err := tx.Exec(ctx, `INSERT INTO public.media_references (id,asset_id,owner_fluctlight_id,target_type,target_id) VALUES ($1,$2,$3,'moment',$4) ON CONFLICT DO NOTHING`, "media_ref_"+stableDigest(assetID+":moment:"+*intent.MomentID), assetID, intent.Owner, *intent.MomentID)
			return err
		})
	}
	return nil
}

func appendUniqueAssetRef(refs []any, assetID string) []any {
	if containsStringValue(refs, assetID) {
		return refs
	}
	return append(refs, assetID)
}

func (a *App) ServeMedia(ctx context.Context, writer http.ResponseWriter, assetID, rangeHeader string) error {
	var bucket, key, mime, etag string
	var size int
	err := a.DB.Pool().QueryRow(ctx, `SELECT bucket,object_key,mime_type,byte_size,COALESCE(etag,'') FROM public.media_assets WHERE id=$1 AND status='ready'`, assetID).Scan(&bucket, &key, &mime, &size, &etag)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	start, end, partial, err := parseRange(rangeHeader, int64(size))
	if err != nil {
		writer.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", size))
		return &rangeError{}
	}
	options := minio.GetObjectOptions{}
	if partial {
		if err := options.SetRange(start, end); err != nil {
			return err
		}
	}
	object, err := a.Storage.GetObject(ctx, bucket, key, options)
	if err != nil {
		return err
	}
	defer object.Close()
	writer.Header().Set("Content-Type", mime)
	writer.Header().Set("Accept-Ranges", "bytes")
	if etag != "" {
		writer.Header().Set("ETag", etag)
	}
	if !partial {
		writer.Header().Set("Content-Length", fmt.Sprintf("%d", size))
		data, readErr := io.ReadAll(io.LimitReader(object, int64(size)+1))
		if readErr != nil {
			return readErr
		}
		if len(data) != size {
			return errors.New("media object size mismatch")
		}
		writer.WriteHeader(http.StatusOK)
		_, err = writer.Write(data)
		return err
	}
	length := end - start + 1
	writer.Header().Set("Content-Length", fmt.Sprintf("%d", length))
	writer.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, size))
	data, readErr := io.ReadAll(io.LimitReader(object, length+1))
	if readErr != nil {
		return readErr
	}
	if int64(len(data)) != length {
		return errors.New("media range size mismatch")
	}
	writer.WriteHeader(http.StatusPartialContent)
	_, err = writer.Write(data)
	return err
}

type rangeError struct{}

func (*rangeError) Error() string { return "invalid byte range" }

func parseRange(value string, size int64) (int64, int64, bool, error) {
	if strings.TrimSpace(value) == "" {
		return 0, size - 1, false, nil
	}
	if size <= 0 || !strings.HasPrefix(value, "bytes=") {
		return 0, 0, false, &rangeError{}
	}
	raw := strings.TrimSpace(strings.TrimPrefix(value, "bytes="))
	if strings.Contains(raw, ",") {
		return 0, 0, false, &rangeError{}
	}
	parts := strings.SplitN(raw, "-", 2)
	if len(parts) != 2 {
		return 0, 0, false, &rangeError{}
	}
	var start, end int64
	var err error
	if strings.TrimSpace(parts[0]) == "" {
		n, e := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
		if e != nil || n <= 0 {
			return 0, 0, false, &rangeError{}
		}
		if n > size {
			n = size
		}
		return size - n, size - 1, true, nil
	}
	start, err = strconv.ParseInt(strings.TrimSpace(parts[0]), 10, 64)
	if err != nil || start < 0 || start >= size {
		return 0, 0, false, &rangeError{}
	}
	if strings.TrimSpace(parts[1]) == "" {
		end = size - 1
	} else if end, err = strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64); err != nil || end < start {
		return 0, 0, false, &rangeError{}
	}
	if end >= size {
		end = size - 1
	}
	return start, end, true, nil
}

func (a *App) AuthorizeAsset(ctx context.Context, actorID, assetID string) error {
	var owner string
	err := a.DB.Pool().QueryRow(ctx, `SELECT m.owner_fluctlight_id FROM public.media_assets m JOIN public.fluctlights f ON f.id=m.owner_fluctlight_id WHERE m.id=$1 AND m.status='ready' AND f.created_by_actor_id=$2`, assetID, actorID).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
