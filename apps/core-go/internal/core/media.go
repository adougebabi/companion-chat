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
	ID, Owner, Prompt, ProviderRequestID, ProviderJobID, WorkflowID string
	Kind, MimeType, Status                                          string
	ConversationID, MessageID, MomentID                             *string
}

func (a *App) ProcessMediaIntent(ctx context.Context, intentID string) error {
	stopHeartbeat := startMediaHeartbeat(ctx, intentID)
	defer stopHeartbeat()
	activity.RecordHeartbeat(ctx, map[string]any{"intent_id": intentID, "phase": "loading"})
	intent, err := a.readMediaIntent(ctx, intentID)
	if err != nil {
		return err
	}
	assetID := "asset_" + intent.ID
	var ready bool
	if err := a.DB.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.media_assets WHERE id=$1 AND status='ready')`, assetID).Scan(&ready); err != nil {
		return err
	}
	if ready {
		if err := a.publishMediaAsset(ctx, intent, assetID); err != nil {
			return err
		}
		return a.markMediaIntentCompleted(ctx, intent.ID)
	}
	config, err := a.runtimeValue(ctx, "media.comfyui")
	if err != nil {
		return err
	}
	baseURL, workflow, err := comfyConfig(config)
	if err != nil {
		return err
	}
	prompt := intent.Prompt
	providerJobID := intent.ProviderJobID
	if providerJobID == "" {
		// A retry with a persisted Provider job must poll that job directly. Do
		// not regenerate the prompt (or submit another request) after the
		// external job has already been accepted.
		activity.RecordHeartbeat(ctx, map[string]any{"intent_id": intentID, "phase": "prompt"})
		prompt = bindMediaPromptContext(prompt)
		value, providerErr := a.Provider.Text(ctx, "media_prompt", []map[string]any{{"role": "system", "content": "You are an elite AI Image Prompt Engineer and Cinematographer. Your objective is to take raw, user-provided or AI-generated prompt descriptions (often in JSON or plain text) and optimize them into professional, highly effective text-to-image prompts in English.\\n\\nYour optimization process MUST follow these core rules:\\n\\n1. MICRO-EXPRESSION TRANSLATION (Crucial for Vivid Emotion)\\nIf the input describes a mood or emotion, you MUST translate it into specific, physical facial movements. \\n- Avoid relying solely on abstract adjectives like 'happy' or 'embarrassed.'\\n- Instead, use descriptive micro-expressions. Example: For 'embarrassed/playful', use 'biting her lower lip, flushed pink cheeks, eyes crinkling at the corners, avoiding direct eye contact, a slight smirk.' \\n- If no expression is provided, deduce a natural, vivid expression that fits the scene context.\\n\\n2. INTENT ANALYSIS & LOGIC DEBUGGING (Camera Physics)\\nScan the input for spatial or logical contradictions and resolve them based on the primary intent. \\n- For a 'selfie' (1st-person POV): Strictly enforce selfie physics (e.g., 'POV selfie, looking into the lens, arm extended out of frame holding the device'). Remove any 3rd-person actions like 'looking down at their phone.'\\n- For a 'candid' (3rd-person): Enforce an observer perspective (e.g., 'shot from a distance, unaware of the camera').\\n\\n3. CINEMATOGRAPHY & PERSPECTIVE AUTO-FILL\\nInject missing camera details to anchor the composition:\\n- Framing: Specify shot type (Extreme Close-Up, Medium Close-Up, Full Body). \\n- Lens/Device: Specify camera gear (e.g., shot on iPhone 15 front camera, 85mm portrait lens, shallow depth of field, bokeh).\\n\\n4. PROMPT STRUCTURE\\nFormat the final prompt strictly in this order to prioritize important weights:\\n[Subject Details & Specific Micro-Expressions] + [Action/Posture] + [Setting & Environment] + [Camera Perspective & Framing] + [Lighting & Style].\\n\\n5. OUTPUT CONSTRAINT\\nReturn ONLY the finalized English prompt as a single continuous string. Do not output JSON. Do not include any greetings, explanations, or prefixes like 'Prompt:'.Return only the image prompt text."}, {"role": "user", "content": intent.Prompt}})
		if providerErr != nil || strings.TrimSpace(value) == "" {
			if providerErr != nil {
				return fmt.Errorf("media prompt generation failed: %w", providerErr)
			}
			return errors.New("media prompt generation returned empty text")
		}
		prompt = value
		activity.RecordHeartbeat(ctx, map[string]any{"intent_id": intentID, "phase": "submit"})
		workflow = replacePrompt(workflow, prompt)
		payload, _ := json.Marshal(map[string]any{"prompt": workflow})
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/prompt", bytes.NewReader(payload))
		if err != nil {
			return err
		}
		request.Header.Set("Content-Type", "application/json")
		client := a.Provider.HTTP
		if client == nil {
			client = &http.Client{Timeout: 30 * time.Second}
		}
		response, err := client.Do(request)
		if err != nil {
			return err
		}
		data, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		response.Body.Close()
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			detail := strings.TrimSpace(string(data))
			if len(detail) > 512 {
				detail = detail[:512]
			}
			return fmt.Errorf("ComfyUI returned HTTP %d: %s", response.StatusCode, detail)
		}
		var result map[string]any
		if err := json.Unmarshal(data, &result); err != nil {
			return err
		}
		providerJobID = stringValue(result["prompt_id"])
		if providerJobID == "" {
			return errors.New("ComfyUI prompt ID is missing")
		}
		if _, err := a.DB.Pool().Exec(ctx, `UPDATE public.media_intents SET provider_job_id=$2,status='running' WHERE id=$1`, intent.ID, providerJobID); err != nil {
			return err
		}
	}
	var output map[string]any
	for attempt := 0; attempt < 900; attempt++ {
		activity.RecordHeartbeat(ctx, map[string]any{"provider_job_id": providerJobID, "attempt": attempt})
		value, done, err := pollComfy(ctx, baseURL, providerJobID)
		if err != nil {
			return err
		}
		if done {
			output = value
			break
		}
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	if output == nil {
		return errors.New("media generation timed out")
	}
	contentType, content, err := downloadComfy(ctx, baseURL, output)
	if err != nil {
		return err
	}
	objectKey := "media/" + assetID + "/v1"
	_, err = a.Storage.PutObject(ctx, a.S3Bucket, objectKey, bytes.NewReader(content), int64(len(content)), minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return err
	}
	digest := sha256.Sum256(content)
	version := "v1"
	workflowID := intent.WorkflowID
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO public.media_assets (id,owner_fluctlight_id,version,kind,mime_type,byte_size,sha256,bucket,object_key,provider_request_id,workflow_id,status,ready_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'ready',now()) ON CONFLICT (id) DO NOTHING`, assetID, intent.Owner, version, intent.Kind, contentType, len(content), hex.EncodeToString(digest[:]), a.S3Bucket, objectKey, intent.ProviderRequestID, workflowID); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	if err := a.publishMediaAsset(ctx, intent, assetID); err != nil {
		return err
	}
	return a.markMediaIntentCompleted(ctx, intent.ID)
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
	err := a.DB.Pool().QueryRow(ctx, `SELECT id,owner_fluctlight_id,prompt,provider_request_id,COALESCE(provider_job_id,''),workflow_id,kind,mime_type,status,conversation_id,message_id,moment_id FROM public.media_intents WHERE id=$1`, intentID).Scan(&i.ID, &i.Owner, &i.Prompt, &i.ProviderRequestID, &i.ProviderJobID, &i.WorkflowID, &i.Kind, &i.MimeType, &i.Status, &i.ConversationID, &i.MessageID, &i.MomentID)
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

func replacePrompt(value map[string]any, prompt string) map[string]any {
	result := make(map[string]any, len(value))
	for key, child := range value {
		switch typed := child.(type) {
		case string:
			if typed == "{{prompt}}" {
				result[key] = prompt
			} else {
				result[key] = typed
			}
		case map[string]any:
			result[key] = replacePrompt(typed, prompt)
		case []any:
			items := make([]any, len(typed))
			for index, item := range typed {
				if nested, ok := item.(map[string]any); ok {
					items[index] = replacePrompt(nested, prompt)
				} else {
					items[index] = item
				}
			}
			result[key] = items
		default:
			result[key] = child
		}
	}
	return result
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
