package core

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const visualIdentityLiveConfigEnv = "FLUCTLIGHT_VISUAL_LIVE_CONFIG_FILE"

type visualIdentityLiveConfig struct {
	media          map[string]any
	s3Endpoint     string
	s3Region       string
	s3AccessKey    string
	s3SecretKey    string
	s3BucketPrefix string
	s3UseSSL       bool
	timeout        time.Duration
	maxTransitions int
	maxMediaRuns   int
}

// TestVisualIdentityLiveE2EPreflight performs only non-generative dependency
// checks. The strict runner executes it before any model row so an invalid
// ComfyUI/S3 configuration cannot be discovered after the other Agents ran.
func TestVisualIdentityLiveE2EPreflight(t *testing.T) {
	if strings.TrimSpace(os.Getenv("FLUCTLIGHT_LIVE_PROVIDER_TEST")) != "1" {
		t.Skip("set FLUCTLIGHT_LIVE_PROVIDER_TEST=1 to preflight real Visual Identity E2E")
	}
	config := loadVisualIdentityLiveConfig(t)
	probeVisualIdentityComfyUI(t, config)
	_, _ = prepareVisualIdentityLiveBucket(t, config)
}

// testFormalAgentE2EVisualIdentity drives the same production handlers used by
// the durable workflow. It deliberately does not insert completed media jobs or
// media_assets: ProcessMediaIntent must create both real images and persist the
// authoritative assets before the complete Agent can review/finalize them.
func testFormalAgentE2EVisualIdentity(t *testing.T) {
	config := loadVisualIdentityLiveConfig(t)
	probeVisualIdentityComfyUI(t, config)
	storage, bucket := prepareVisualIdentityLiveBucket(t, config)

	fixture := newFormalAgentE2EFixture(t)
	fixture.app.Storage = storage
	fixture.app.S3Bucket = bucket
	if fixture.app.Media != nil {
		fixture.app.Media = NewMediaService(fixture.repository.Pool(), storage, bucket, fixture.app)
	}
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `INSERT INTO public.runtime_settings(key,value_json) VALUES('media.comfyui',$1) ON CONFLICT(key) DO UPDATE SET value_json=excluded.value_json,updated_at=now()`, jsonString(config.media)); err != nil {
		t.Fatalf("install isolated media.comfyui setting: %v", err)
	}

	persona := map[string]any{
		"identity": map[string]any{
			"name": "澄光", "gender": "male", "age": 27, "nationality": "Chinese",
			"self_description": "A warm, grounded adult local AI companion with one stable human appearance.",
		},
		"life_profile": map[string]any{
			"city": "上海", "timezone": "Asia/Shanghai",
			"appearance": map[string]any{"hair": "black shoulder-length hair", "face_shape": "oval", "body_type": "slender", "outfit": "dark blue shirt and charcoal trousers"},
		},
	}
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `UPDATE public.fluctlights SET core_persona=$2,identity=$3,life_profile=$4 WHERE id=$1`, fixture.fluctlightID, jsonString(persona), jsonString(mapValue(persona["identity"])), jsonString(mapValue(persona["life_profile"]))); err != nil {
		t.Fatalf("seed Visual Identity persona: %v", err)
	}
	sessionID, err := fixture.app.EnsureVisualIdentityInitializationWithPersona(fixture.ctx, fixture.fluctlightID, "initialization", fixture.runPrefix+"-visual-identity", persona)
	if err != nil {
		t.Fatalf("create durable Visual Identity session: %v", err)
	}

	runCtx, cancel := context.WithTimeout(context.Background(), config.timeout)
	defer cancel()
	fixture.ctx = runCtx
	var completed VisualIdentityAgentOutput
	mediaRuns := make(map[string]int)
	for transition := 1; transition <= config.maxTransitions; transition++ {
		if err := runCtx.Err(); err != nil {
			t.Fatalf("Visual Identity live E2E deadline reached after %d transitions: %v", transition-1, err)
		}
		resultMap, processErr := fixture.app.ProcessVisualIdentity(runCtx, sessionID)
		if processErr != nil {
			t.Fatalf("production ProcessVisualIdentity transition %d failed: %v", transition, processErr)
		}
		completed = visualIdentityAgentOutputFromMap(resultMap)
		if completed.Attempt > visualIdentityMaxAttempts {
			t.Fatalf("Visual Identity Agent exceeded its product attempt guard: attempt=%d max=%d", completed.Attempt, visualIdentityMaxAttempts)
		}
		switch completed.Status {
		case "completed":
			requireVisualIdentityLiveCompletion(t, fixture, storage, bucket, sessionID, completed)
			return
		case "failed", "awaiting_review":
			t.Fatalf("Visual Identity Agent reached terminal non-success state: status=%s stage=%s attempt=%d error_code=%s", completed.Status, completed.Stage, completed.Attempt, completed.ErrorCode)
		case "waiting":
		default:
			t.Fatalf("Visual Identity Agent returned unknown durable status %q", completed.Status)
		}

		intentID, status := pendingVisualIdentityMediaIntent(t, fixture, completed)
		if intentID == "" {
			// A regenerate decision commits the next attempt before it creates the
			// next media intent. The following production handler transition owns
			// that generate_candidate call.
			if completed.Stage == "candidate_generation_required" {
				continue
			}
			t.Fatalf("waiting Visual Identity state has no actionable media intent: stage=%s attempt=%d", completed.Stage, completed.Attempt)
		}
		if status == "completed" {
			continue
		}
		mediaRuns[intentID]++
		if mediaRuns[intentID] > config.maxMediaRuns {
			t.Fatalf("media intent %s exceeded the live retry guard (%d)", intentID, config.maxMediaRuns)
		}
		mediaResult, mediaErr := fixture.app.ProcessMediaIntent(runCtx, intentID)
		if mediaErr != nil {
			t.Fatalf("production ProcessMediaIntent %s failed: %v", intentID, mediaErr)
		}
		switch mediaStatus := stringValue(mediaResult["status"]); mediaStatus {
		case "completed", "quality_retry":
		case "failed":
			t.Fatalf("real media intent %s failed quality or renderer acceptance: %#v", intentID, mediaResult)
		default:
			t.Fatalf("real media intent %s returned unknown status %q", intentID, mediaStatus)
		}
	}
	t.Fatalf("Visual Identity live E2E exceeded %d production-handler transitions; last status=%s stage=%s attempt=%d", config.maxTransitions, completed.Status, completed.Stage, completed.Attempt)
}

func loadVisualIdentityLiveConfig(t *testing.T) visualIdentityLiveConfig {
	t.Helper()
	path := strings.TrimSpace(os.Getenv(visualIdentityLiveConfigEnv))
	if path == "" {
		t.Fatalf("%s is required when FLUCTLIGHT_LIVE_PROVIDER_TEST=1 runs visual_identity", visualIdentityLiveConfigEnv)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", visualIdentityLiveConfigEnv, err)
	}
	var root map[string]any
	if err := json.Unmarshal(content, &root); err != nil {
		t.Fatalf("parse %s JSON: %v", visualIdentityLiveConfigEnv, err)
	}
	media := mapValue(root["media_comfyui"])
	if len(media) == 0 {
		media = mapValue(root["media.comfyui"])
	}
	if _, _, err := comfyConfig(media); err != nil {
		t.Fatalf("%s media_comfyui is invalid: %v", visualIdentityLiveConfigEnv, err)
	}
	s3 := mapValue(root["s3"])
	config := visualIdentityLiveConfig{
		media:          cloneMap(media),
		s3Endpoint:     visualIdentityLiveValue("FLUCTLIGHT_VISUAL_LIVE_S3_ENDPOINT", s3, "endpoint"),
		s3Region:       visualIdentityLiveValue("FLUCTLIGHT_VISUAL_LIVE_S3_REGION", s3, "region"),
		s3AccessKey:    visualIdentityLiveValue("FLUCTLIGHT_VISUAL_LIVE_S3_ACCESS_KEY", s3, "access_key", "accessKey"),
		s3SecretKey:    visualIdentityLiveValue("FLUCTLIGHT_VISUAL_LIVE_S3_SECRET_KEY", s3, "secret_key", "secretKey"),
		s3BucketPrefix: visualIdentityLiveValue("FLUCTLIGHT_VISUAL_LIVE_S3_BUCKET_PREFIX", s3, "bucket_prefix", "bucketPrefix"),
		timeout:        25 * time.Minute,
		maxTransitions: 24,
		maxMediaRuns:   3,
	}
	if config.s3Region == "" {
		config.s3Region = "us-east-1"
	}
	if value := strings.TrimSpace(os.Getenv("FLUCTLIGHT_VISUAL_LIVE_S3_USE_SSL")); value != "" {
		config.s3UseSSL, err = strconv.ParseBool(value)
		if err != nil {
			t.Fatalf("FLUCTLIGHT_VISUAL_LIVE_S3_USE_SSL must be a boolean")
		}
	} else if raw, ok := s3["use_ssl"]; ok {
		config.s3UseSSL, err = visualIdentityLiveBool(raw)
		if err != nil {
			t.Fatalf("%s s3.use_ssl must be a boolean", visualIdentityLiveConfigEnv)
		}
	} else if raw, ok := s3["useSSL"]; ok {
		config.s3UseSSL, err = visualIdentityLiveBool(raw)
		if err != nil {
			t.Fatalf("%s s3.useSSL must be a boolean", visualIdentityLiveConfigEnv)
		}
	} else {
		config.s3UseSSL = strings.HasPrefix(config.s3Endpoint, "https://")
	}
	for name, value := range map[string]string{
		"S3 endpoint": config.s3Endpoint, "S3 access key": config.s3AccessKey,
		"S3 secret key": config.s3SecretKey, "S3 bucket prefix": config.s3BucketPrefix,
	} {
		if strings.TrimSpace(value) == "" {
			t.Fatalf("%s is missing from %s and its FLUCTLIGHT_VISUAL_LIVE_S3_* override", name, visualIdentityLiveConfigEnv)
		}
	}
	parsedEndpoint, parseErr := url.Parse(config.s3Endpoint)
	if parseErr != nil || parsedEndpoint.Host == "" || (parsedEndpoint.Scheme != "http" && parsedEndpoint.Scheme != "https") || (parsedEndpoint.Path != "" && parsedEndpoint.Path != "/") {
		t.Fatalf("Visual Identity S3 endpoint must be an HTTP(S) origin")
	}
	if !regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,39}[a-z0-9]$`).MatchString(config.s3BucketPrefix) {
		t.Fatalf("Visual Identity S3 bucket prefix must be 3-41 lowercase letters, digits, or hyphens")
	}
	if raw := strings.TrimSpace(os.Getenv("FLUCTLIGHT_VISUAL_LIVE_TIMEOUT")); raw != "" {
		config.timeout, err = time.ParseDuration(raw)
		if err != nil || config.timeout <= 0 {
			t.Fatalf("FLUCTLIGHT_VISUAL_LIVE_TIMEOUT must be a positive Go duration")
		}
	}
	if raw := strings.TrimSpace(os.Getenv("FLUCTLIGHT_VISUAL_LIVE_MAX_TRANSITIONS")); raw != "" {
		config.maxTransitions, err = strconv.Atoi(raw)
		if err != nil || config.maxTransitions < 1 || config.maxTransitions > 100 {
			t.Fatalf("FLUCTLIGHT_VISUAL_LIVE_MAX_TRANSITIONS must be in 1..100")
		}
	}
	if raw := strings.TrimSpace(os.Getenv("FLUCTLIGHT_VISUAL_LIVE_MAX_MEDIA_RUNS")); raw != "" {
		config.maxMediaRuns, err = strconv.Atoi(raw)
		if err != nil || config.maxMediaRuns < 1 || config.maxMediaRuns > 10 {
			t.Fatalf("FLUCTLIGHT_VISUAL_LIVE_MAX_MEDIA_RUNS must be in 1..10")
		}
	}
	return config
}

func visualIdentityLiveValue(envName string, values map[string]any, keys ...string) string {
	if value := strings.TrimSpace(os.Getenv(envName)); value != "" {
		return value
	}
	for _, key := range keys {
		if value := strings.TrimSpace(stringValue(values[key])); value != "" {
			return value
		}
	}
	return ""
}

func visualIdentityLiveBool(value any) (bool, error) {
	switch typed := value.(type) {
	case bool:
		return typed, nil
	case string:
		return strconv.ParseBool(strings.TrimSpace(typed))
	default:
		return false, fmt.Errorf("unsupported boolean type %T", value)
	}
}

func probeVisualIdentityComfyUI(t *testing.T, config visualIdentityLiveConfig) {
	t.Helper()
	baseURL, _, _ := comfyConfig(config.media)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/system_stats", nil)
	if err != nil {
		t.Fatalf("build ComfyUI preflight request: %v", err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("ComfyUI preflight failed: %v", err)
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		t.Fatalf("ComfyUI preflight returned HTTP %d", response.StatusCode)
	}
}

func prepareVisualIdentityLiveBucket(t *testing.T, config visualIdentityLiveConfig) (*minio.Client, string) {
	t.Helper()
	parsed, _ := url.Parse(config.s3Endpoint)
	storage, err := minio.New(parsed.Host, &minio.Options{
		Creds:  credentials.NewStaticV4(config.s3AccessKey, config.s3SecretKey, ""),
		Secure: config.s3UseSSL, Region: config.s3Region,
	})
	if err != nil {
		t.Fatalf("create isolated Visual Identity S3 client: %v", err)
	}
	bucket := fmt.Sprintf("%s-%s", config.s3BucketPrefix, stableDigest(t.Name() + fmt.Sprintf("-%d", time.Now().UnixNano()))[:12])
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := storage.MakeBucket(ctx, bucket, minio.MakeBucketOptions{Region: config.s3Region}); err != nil {
		t.Fatalf("create isolated Visual Identity S3 bucket: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cleanupCancel()
		for object := range storage.ListObjects(cleanupCtx, bucket, minio.ListObjectsOptions{Recursive: true}) {
			if object.Err == nil {
				if removeErr := storage.RemoveObject(cleanupCtx, bucket, object.Key, minio.RemoveObjectOptions{}); removeErr != nil {
					t.Errorf("remove isolated Visual Identity object: %v", removeErr)
				}
			}
		}
		if removeErr := storage.RemoveBucket(cleanupCtx, bucket); removeErr != nil {
			t.Errorf("remove isolated Visual Identity bucket: %v", removeErr)
		}
	})
	return storage, bucket
}

func visualIdentityAgentOutputFromMap(value map[string]any) VisualIdentityAgentOutput {
	return VisualIdentityAgentOutput{
		SessionID: stringValue(value["session_id"]), FluctlightID: stringValue(value["fluctlight_id"]),
		Attempt: intValue(value["attempt"]), Status: stringValue(value["status"]), Stage: stringValue(value["stage"]),
		Accepted: value["accepted"] == true, MediaIntentID: stringValue(value["media_intent_id"]),
		CandidateAssetID: stringValue(value["asset_id"]), CanonicalAssetID: stringValue(value["canonical_asset_id"]),
		CharacterMediaIntentID: stringValue(value["character_sheet_media_intent_id"]), CharacterSheetAssetID: stringValue(value["character_sheet_asset_id"]),
		ErrorCode: stringValue(value["error_code"]), Summary: stringValue(value["summary"]),
	}
}

func pendingVisualIdentityMediaIntent(t *testing.T, fixture *formalAgentE2EFixture, output VisualIdentityAgentOutput) (string, string) {
	t.Helper()
	for _, intentID := range []string{output.CharacterMediaIntentID, output.MediaIntentID} {
		if intentID == "" {
			continue
		}
		var status string
		if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT status FROM public.media_intents WHERE id=$1`, intentID).Scan(&status); err != nil {
			t.Fatalf("read Visual Identity media intent %s: %v", intentID, err)
		}
		if status != "completed" {
			return intentID, status
		}
	}
	return "", "completed"
}

func requireVisualIdentityLiveCompletion(t *testing.T, fixture *formalAgentE2EFixture, storage *minio.Client, bucket, sessionID string, output VisualIdentityAgentOutput) {
	t.Helper()
	var sessionStatus, profileStatus, canonicalAssetID, characterAssetID string
	var revision int
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT s.status,v.status,v.current_revision,COALESCE(v.canonical_asset_id,''),COALESCE(v.character_sheet_asset_id,'') FROM public.fluctlight_visual_identity_sessions AS s JOIN public.fluctlight_visual_identities AS v ON v.id=s.visual_identity_id WHERE s.id=$1`, sessionID).Scan(&sessionStatus, &profileStatus, &revision, &canonicalAssetID, &characterAssetID); err != nil {
		t.Fatalf("read completed Visual Identity aggregate: %v", err)
	}
	if output.Status != "completed" || sessionStatus != "completed" || profileStatus != "active" || revision < 1 || canonicalAssetID == "" || characterAssetID == "" || canonicalAssetID == characterAssetID {
		t.Fatalf("incomplete Visual Identity aggregate: output=%s session=%s profile=%s revision=%d canonical_present=%v character_present=%v distinct=%v", output.Status, sessionStatus, profileStatus, revision, canonicalAssetID != "", characterAssetID != "", canonicalAssetID != characterAssetID)
	}
	assetDigests := make(map[string]string, 2)
	for _, assetID := range []string{canonicalAssetID, characterAssetID} {
		var assetStatus, assetBucket, objectKey, mimeType, expectedSHA string
		var byteSize int64
		if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT status,bucket,object_key,mime_type,byte_size,sha256 FROM public.media_assets WHERE id=$1`, assetID).Scan(&assetStatus, &assetBucket, &objectKey, &mimeType, &byteSize, &expectedSHA); err != nil {
			t.Fatalf("read completed Visual Identity asset: %v", err)
		}
		if assetStatus != "ready" || assetBucket != bucket || objectKey == "" || !strings.HasPrefix(mimeType, "image/") || byteSize <= 0 {
			t.Fatalf("Visual Identity asset is not a real ready image object: id=%s status=%s isolated_bucket=%v object_present=%v image_mime=%v bytes=%d", assetID, assetStatus, assetBucket == bucket, objectKey != "", strings.HasPrefix(mimeType, "image/"), byteSize)
		}
		object, err := storage.GetObject(fixture.ctx, assetBucket, objectKey, minio.GetObjectOptions{})
		if err != nil {
			t.Fatalf("open completed Visual Identity object: %v", err)
		}
		content, readErr := io.ReadAll(io.LimitReader(object, 32<<20))
		_ = object.Close()
		if readErr != nil || len(content) == 0 {
			t.Fatalf("read completed Visual Identity object: bytes=%d err=%v", len(content), readErr)
		}
		digest := sha256.Sum256(content)
		actualSHA := hex.EncodeToString(digest[:])
		if actualSHA != expectedSHA {
			t.Fatalf("Visual Identity object checksum differs from authoritative asset row")
		}
		assetDigests[assetID] = actualSHA
	}

	type ledgerRow struct {
		capability string
		invocation CapabilityInvocation
		result     CapabilityResult
	}
	rows, err := fixture.repository.Pool().Query(fixture.ctx, `SELECT capability_name,invocation,result FROM public.tool_executions WHERE fluctlight_id=$1 AND agent_id=$2 ORDER BY committed_at,operation_id`, fixture.fluctlightID, FormalAgentVisualIdentity)
	if err != nil {
		t.Fatalf("read Visual Identity Tool ledger: %v", err)
	}
	defer rows.Close()
	ledger := make([]ledgerRow, 0)
	seen := make(map[string]bool)
	for rows.Next() {
		var row ledgerRow
		var invocationJSON, resultJSON []byte
		if err := rows.Scan(&row.capability, &invocationJSON, &resultJSON); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(invocationJSON, &row.invocation); err != nil {
			t.Fatalf("decode Visual Identity Tool invocation: %v", err)
		}
		if err := json.Unmarshal(resultJSON, &row.result); err != nil {
			t.Fatalf("decode Visual Identity Tool result: %v", err)
		}
		if row.invocation.CallID == "" || row.invocation.ProviderRequestID == "" || row.result.CallID != row.invocation.CallID || row.result.ProviderRequestID != row.invocation.ProviderRequestID {
			t.Fatalf("Visual Identity Tool ledger lost native request/call/result identity for %s", row.capability)
		}
		seen[row.capability] = true
		ledger = append(ledger, row)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	for _, capability := range []string{visualIdentityGenerateCandidateCapabilityName, visualIdentityCommitReviewCapabilityName, visualIdentityFinalizeCapabilityName} {
		if !seen[capability] {
			t.Fatalf("completed Visual Identity session has no committed %s Tool ledger row", capability)
		}
	}

	requests := fixture.spy.snapshot()
	for _, row := range ledger {
		feedbackFound := false
		for _, request := range requests {
			messages, _ := json.Marshal(request["messages"])
			encoded := string(messages)
			if strings.Contains(encoded, row.invocation.CallID) && strings.Contains(encoded, row.capability) && strings.Contains(encoded, stringValue(mapValue(row.result.Output)["status"])) {
				feedbackFound = true
				break
			}
		}
		if !feedbackFound {
			t.Fatalf("committed %s Tool result was not found in a subsequent model input", row.capability)
		}
	}
	candidateDigest := assetDigests[canonicalAssetID]
	imageDigestFound := false
	for _, request := range requests {
		if digest, ok := formalAgentRequestImageDigest(request); ok && digest == candidateDigest {
			imageDigestFound = true
			break
		}
	}
	if !imageDigestFound {
		t.Fatalf("the actual canonical candidate bytes were not carried as image_url in the Visual Identity model request")
	}
}

func formalAgentRequestImageDigest(request map[string]any) (string, bool) {
	for _, rawMessage := range arrayValue(request["messages"]) {
		message := mapValue(rawMessage)
		for _, rawContent := range arrayValue(message["content"]) {
			content := mapValue(rawContent)
			if stringValue(content["type"]) != "image_url" {
				continue
			}
			value := stringValue(mapValue(content["image_url"])["url"])
			marker := ";base64,"
			index := strings.Index(value, marker)
			if !strings.HasPrefix(value, "data:image/") || index < 0 {
				continue
			}
			decoded, err := base64.StdEncoding.DecodeString(value[index+len(marker):])
			if err != nil || len(decoded) == 0 {
				continue
			}
			digest := sha256.Sum256(decoded)
			return hex.EncodeToString(digest[:]), true
		}
	}
	return "", false
}
