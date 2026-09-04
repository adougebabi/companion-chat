package core

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
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
	QualityRetryCount                                                               int
	ConversationID, MessageID, MomentID                                             *string
}

func (a *App) ProcessMediaIntent(ctx context.Context, intentID string) (map[string]any, error) {
	stopHeartbeat := startMediaHeartbeat(ctx, intentID)
	defer stopHeartbeat()
	activity.RecordHeartbeat(ctx, map[string]any{"intent_id": intentID, "phase": "loading"})
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
		if err := a.markMediaIntentCompleted(ctx, intent.ID); err != nil {
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
			activity.RecordHeartbeat(ctx, map[string]any{"intent_id": intentID, "phase": "prompt"})
			promptInput := bindMediaPromptContext(mediaPromptInput(intent))
			value, providerErr := a.Provider.Text(WithProviderScenario(ctx, "media_prompt"), "media_prompt", []map[string]any{{"role": "system", "content": mediaPromptInstruction}, {"role": "user", "content": promptInput}})
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
		activity.RecordHeartbeat(ctx, map[string]any{"intent_id": intentID, "phase": "submit"})
		constraints := map[string]any{}
		if json.Unmarshal([]byte(intent.Prompt), &concept) == nil {
			constraints = mapValue(concept["renderer_constraints"])
		}
		workflow, err = replaceMediaPlaceholders(workflow, prompt, constraints)
		if err != nil {
			return nil, err
		}
		a.recordDiagnosticEvent(ctx, "media.comfyui.prompt_submitted", "info", intent.Owner, intent.ProviderRequestID, "media:"+intent.ID, map[string]any{
			"media_intent_id":     intent.ID,
			"provider_request_id": intent.ProviderRequestID,
			"workflow_id":         intent.WorkflowID,
			"stage":               stringValue(concept["stage"]),
			"prompt":              visualIdentityBoundedText(prompt, 4000),
		})
		payload, _ := json.Marshal(map[string]any{"prompt": workflow})
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/prompt", bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		request.Header.Set("Content-Type", "application/json")
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
		activity.RecordHeartbeat(ctx, map[string]any{"provider_job_id": providerJobID, "attempt": attempt})
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
	if (intent.QualityVerdict == mediaQualityVerdictPass || intent.QualityVerdict == mediaQualityVerdictSkip) && intent.QualityCandidateSHA == candidateSHA {
		quality.Verdict = intent.QualityVerdict
	} else {
		quality, err = a.evaluateMediaQuality(ctx, intent, contentType, content)
		if err != nil {
			if errors.Is(err, context.Canceled) && ctx.Err() != nil {
				return nil, err
			}
			quality = mediaQualityAcceptance{SchemaVersion: mediaQualitySchemaVersion, Verdict: mediaQualityVerdictSkip}
			reason := mediaQualityInfrastructureReason(err)
			a.recordDiagnosticEvent(ctx, "media.quality.acceptance", "warn", intent.Owner, "media:"+intent.ID, intent.ProviderRequestID, mediaQualityDiagnostic(quality, reason, candidateSHA, intent.QualityRetryCount))
			if err := a.persistMediaQualityVerdict(ctx, intent.ID, providerJobID, mediaQualityVerdictSkip, candidateSHA); err != nil {
				return nil, err
			}
			intent.QualityVerdict = mediaQualityVerdictSkip
			intent.QualityCandidateSHA = candidateSHA
		} else {
			a.recordDiagnosticEvent(ctx, "media.quality.acceptance", "info", intent.Owner, "media:"+intent.ID, intent.ProviderRequestID, mediaQualityDiagnostic(quality, "", candidateSHA, intent.QualityRetryCount))
			switch quality.Verdict {
			case mediaQualityVerdictPass:
				if err := a.persistMediaQualityVerdict(ctx, intent.ID, providerJobID, quality.Verdict, candidateSHA); err != nil {
					return nil, err
				}
				intent.QualityVerdict = quality.Verdict
				intent.QualityCandidateSHA = candidateSHA
			case mediaQualityVerdictRetry:
				if intent.QualityRetryCount >= 1 {
					if err := a.rejectMediaQuality(ctx, intent.ID, providerJobID, candidateSHA); err != nil {
						return nil, err
					}
					return map[string]any{"intent_id": intent.ID, "status": "failed", "quality_verdict": mediaQualityVerdictReject}, nil
				}
				if err := a.prepareMediaQualityRetry(ctx, intent.ID, providerJobID, quality.RetryGuidance, candidateSHA); err != nil {
					return nil, err
				}
				return map[string]any{"intent_id": intent.ID, "status": "quality_retry", "quality_retry_count": intent.QualityRetryCount + 1}, nil
			case mediaQualityVerdictReject:
				if err := a.rejectMediaQuality(ctx, intent.ID, providerJobID, candidateSHA); err != nil {
					return nil, err
				}
				return map[string]any{"intent_id": intent.ID, "status": "failed", "quality_verdict": mediaQualityVerdictReject}, nil
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
		return nil, err
	}
	if err := a.publishMediaAsset(ctx, intent, assetID); err != nil {
		return nil, err
	}
	if err := a.markMediaIntentCompleted(ctx, intent.ID); err != nil {
		return nil, err
	}
	return map[string]any{"intent_id": intent.ID, "status": "completed", "quality_verdict": quality.Verdict}, nil
}

func (a *App) markMediaIntentCompleted(ctx context.Context, intentID string) error {
	command, err := a.DB.Pool().Exec(ctx, `UPDATE public.media_intents SET status='completed',revision=revision+1 WHERE id=$1 AND status IN ('pending','running')`, intentID)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 1 {
		return nil
	}
	var status string
	if err := a.DB.Pool().QueryRow(ctx, `SELECT status FROM public.media_intents WHERE id=$1`, intentID).Scan(&status); err != nil {
		return err
	}
	if status == "completed" {
		return nil
	}
	return fmt.Errorf("media intent cannot complete from status %s", status)
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
				activity.RecordHeartbeat(heartbeatCtx, map[string]any{"intent_id": intentID, "phase": "in-flight"})
			}
		}
	}()
	return func() {
		cancel()
		<-done
	}
}

func (a *App) readMediaIntent(ctx context.Context, intentID string) (mediaIntent, error) {
	var i mediaIntent
	err := a.DB.Pool().QueryRow(ctx, `SELECT id,owner_fluctlight_id,prompt,COALESCE(provider_prompt,''),provider_request_id,COALESCE(provider_job_id,''),workflow_id,kind,mime_type,status,quality_retry_count,COALESCE(quality_retry_guidance,''),COALESCE(quality_verdict,''),COALESCE(quality_candidate_sha256,''),conversation_id,message_id,moment_id FROM public.media_intents WHERE id=$1`, intentID).Scan(&i.ID, &i.Owner, &i.Prompt, &i.ProviderPrompt, &i.ProviderRequestID, &i.ProviderJobID, &i.WorkflowID, &i.Kind, &i.MimeType, &i.Status, &i.QualityRetryCount, &i.QualityRetryGuidance, &i.QualityVerdict, &i.QualityCandidateSHA, &i.ConversationID, &i.MessageID, &i.MomentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return i, ErrNotFound
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
	result := make(map[string]any, len(value))
	for key, child := range value {
		replaced, err := replaceMediaPlaceholderValue(child, prompt, constraints)
		if err != nil {
			return nil, err
		}
		result[key] = replaced
	}
	return result, nil
}

func replaceMediaPlaceholderValue(value any, prompt string, constraints map[string]any) (any, error) {
	switch typed := value.(type) {
	case string:
		if typed == "{{prompt}}" {
			return prompt, nil
		}
		if typed == "{{chest_lora_weight}}" || typed == "{{renderer_constraints.chest_lora_weight}}" {
			weight, ok := rendererConstraintWeight(constraints)
			if !ok {
				return nil, errors.New("chest_lora_weight_missing")
			}
			return weight, nil
		}
		if strings.Contains(typed, "{{chest_lora_weight}}") {
			weight, ok := rendererConstraintWeight(constraints)
			if !ok {
				return nil, errors.New("chest_lora_weight_missing")
			}
			return strings.ReplaceAll(typed, "{{chest_lora_weight}}", strconv.FormatFloat(weight, 'f', -1, 64)), nil
		}
		return typed, nil
	case map[string]any:
		return replaceMediaPlaceholders(typed, prompt, constraints)
	case []any:
		items := make([]any, len(typed))
		for index, item := range typed {
			replaced, err := replaceMediaPlaceholderValue(item, prompt, constraints)
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
