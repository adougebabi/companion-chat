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
	ConversationID, MomentID                                        *string
}

func (a *App) ProcessMediaIntent(ctx context.Context, intentID string) error {
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
		return a.publishMediaAsset(ctx, intent, assetID)
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
	if value, providerErr := a.Provider.Text(ctx, "media_prompt", []map[string]any{{"role": "system", "content": "Return only the image prompt text."}, {"role": "user", "content": intent.Prompt}}); providerErr == nil && strings.TrimSpace(value) != "" {
		prompt = value
	}
	providerJobID := intent.ProviderJobID
	if providerJobID == "" {
		workflow = replacePrompt(workflow, prompt)
		payload, _ := json.Marshal(map[string]any{"prompt": workflow})
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/prompt", bytes.NewReader(payload))
		if err != nil {
			return err
		}
		request.Header.Set("Content-Type", "application/json")
		response, err := a.Provider.HTTP.Do(request)
		if err != nil {
			return err
		}
		data, _ := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		response.Body.Close()
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return fmt.Errorf("ComfyUI returned HTTP %d", response.StatusCode)
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
	objectKey := "media/" + assetID + "/1"
	_, err = a.Storage.PutObject(ctx, a.S3Bucket, objectKey, bytes.NewReader(content), int64(len(content)), minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return err
	}
	digest := sha256.Sum256(content)
	version := "1"
	workflowID := intent.WorkflowID
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `INSERT INTO public.media_assets (id,owner_fluctlight_id,version,kind,mime_type,byte_size,sha256,bucket,object_key,provider_request_id,workflow_id,status,ready_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'ready',now()) ON CONFLICT (id) DO NOTHING`, assetID, intent.Owner, version, intent.Kind, contentType, len(content), hex.EncodeToString(digest[:]), a.S3Bucket, objectKey, intent.ProviderRequestID, workflowID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE public.media_intents SET status='completed',revision=revision+1 WHERE id=$1`, intent.ID); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	return a.publishMediaAsset(ctx, intent, assetID)
}

func (a *App) readMediaIntent(ctx context.Context, intentID string) (mediaIntent, error) {
	var i mediaIntent
	err := a.DB.Pool().QueryRow(ctx, `SELECT id,owner_fluctlight_id,prompt,provider_request_id,COALESCE(provider_job_id,''),workflow_id,kind,mime_type,status,conversation_id,moment_id FROM public.media_intents WHERE id=$1`, intentID).Scan(&i.ID, &i.Owner, &i.Prompt, &i.ProviderRequestID, &i.ProviderJobID, &i.WorkflowID, &i.Kind, &i.MimeType, &i.Status, &i.ConversationID, &i.MomentID)
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
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, false, err
	}
	defer response.Body.Close()
	if response.StatusCode == 404 {
		return nil, false, nil
	}
	data, _ := io.ReadAll(io.LimitReader(response.Body, 4<<20))
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
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return "", nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", nil, fmt.Errorf("ComfyUI output returned HTTP %d", response.StatusCode)
	}
	content, _ := io.ReadAll(io.LimitReader(response.Body, 32<<20))
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
		_, err := a.DB.Pool().Exec(ctx, `UPDATE public.moments SET media_asset_ids=COALESCE(media_asset_ids,'[]'::jsonb)||$2::jsonb WHERE id=$1 AND NOT (COALESCE(media_asset_ids,'[]'::jsonb) @> $2::jsonb)`, *intent.MomentID, jsonBytes([]string{assetID}))
		if err == nil {
			_, err = a.DB.Pool().Exec(ctx, `INSERT INTO public.media_references (id,asset_id,owner_fluctlight_id,target_type,target_id) VALUES ($1,$2,$3,'moment',$4) ON CONFLICT DO NOTHING`, "media_ref_"+stableDigest(intent.ID+":moment"), assetID, intent.Owner, *intent.MomentID)
		}
		return err
	}
	return nil
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
		_, err = io.Copy(writer, object)
		return err
	}
	length := end - start + 1
	writer.Header().Set("Content-Length", fmt.Sprintf("%d", length))
	writer.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, size))
	writer.WriteHeader(http.StatusPartialContent)
	_, err = io.CopyN(writer, object, length)
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
