package core

import (
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func TestVisualIdentitySessionKeepsTextModelAcrossCheckpoints(t *testing.T) {
	f := newVisualIdentityToolFixture(t)
	seedCognitiveProviderRole(t, f.ctx, f.repository, "visual-session-snapshot-endpoint")
	settingsKey := []byte("0123456789abcdef0123456789abcdef")
	f.app.Provider.SettingsKey = settingsKey
	encrypted, err := encryptSecret(settingsKey, "turn-secret", "test-visual-session-secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.setting_secrets(purpose,ciphertext,nonce) VALUES('turn-secret',$1,$2)`, encrypted.ciphertext, encrypted.nonce); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.model_roles SET model_id='first-model' WHERE role='cognitive_assessment'`); err != nil {
		t.Fatal(err)
	}
	var models []string
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		models = append(models, stringValue(payload["model"]))
		if payloadHasToolResult(payload) {
			return fakeProviderResult{Structured: map[string]any{"status": "waiting", "stage": "image_pending", "summary": "候选已受理"}}
		}
		reason := "initial"
		if len(models) > 2 {
			reason = "regenerate"
		}
		return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("visual-session-"+reason, visualIdentityGenerateCandidateCapabilityName, map[string]any{"reason": reason})}}
	})
	f.app.Provider.HTTP = &http.Client{Transport: router}
	first, err := f.app.RunVisualIdentityAgent(f.ctx, VisualIdentityAgentInput{SessionID: f.sessionID})
	if err != nil || first.Status != "waiting" || first.MediaIntentID == "" {
		t.Fatalf("first candidate was not accepted: status=%q err=%v", first.Status, err)
	}
	var provenance []byte
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT model_provenance FROM public.fluctlight_visual_identity_sessions WHERE id=$1`, f.sessionID).Scan(&provenance); err != nil || strings.Contains(string(provenance), "test-visual-session-secret") {
		t.Fatalf("visual session snapshot exposed a credential or could not be read: %v", err)
	}
	state, err := f.app.loadVisualIdentityAgentState(f.ctx, f.sessionID)
	if err != nil || strings.Contains(jsonString(visualIdentityAgentMessages(state, nil)), "turn-secret") {
		t.Fatalf("internal model provenance reached visual Agent input: %v", err)
	}
	f.makeCurrentCandidateReady(t, first.MediaIntentID)
	review := visualIdentityAcceptedReview()
	review["decision"], review["summary"], review["feedback"] = "regenerate", "候选需重新生成", "调整候选中的身份细节"
	manual, err := f.app.ExecuteTool(f.ctx, f.request(visualIdentityCommitReviewCapabilityName, "visual-session-manual-regenerate", review))
	if err != nil || manual.Result.Status != "completed" {
		t.Fatalf("manual review did not start the next attempt: status=%q err=%v", manual.Result.Status, err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.model_roles SET model_id='next-model' WHERE role='cognitive_assessment'`); err != nil {
		t.Fatal(err)
	}
	restarted := newTestApp(t, f.repository, router)
	restarted.Provider.SettingsKey = settingsKey
	second, err := restarted.RunVisualIdentityAgent(f.ctx, VisualIdentityAgentInput{SessionID: f.sessionID})
	if err != nil || second.Status != "waiting" || second.MediaIntentID == "" || second.MediaIntentID == first.MediaIntentID {
		t.Fatalf("next candidate did not retain its distinct operation: first=%q second=%q status=%q err=%v", first.MediaIntentID, second.MediaIntentID, second.Status, err)
	}
	if len(models) != 4 || models[0] != "first-model" || models[1] != "first-model" || models[2] != "first-model" || models[3] != "first-model" {
		t.Fatalf("visual identity session changed its text model: %v", models)
	}
}

func TestLegacyVisualSessionReportsUnfrozenBinding(t *testing.T) {
	f := newVisualIdentityToolFixture(t)
	seedCognitiveProviderRole(t, f.ctx, f.repository, "legacy-visual-snapshot-endpoint")
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlight_visual_identity_sessions SET model_provenance='{}' WHERE id=$1`, f.sessionID); err != nil {
		t.Fatal(err)
	}
	calls := 0
	router := newFakeProviderRouter().otherwise(func(map[string]any) fakeProviderResult {
		calls++
		return fakeProviderResult{Status: http.StatusBadRequest}
	})
	f.app.Provider.HTTP = &http.Client{Transport: router}
	if _, err := f.app.RunVisualIdentityAgent(f.ctx, VisualIdentityAgentInput{SessionID: f.sessionID}); err == nil || calls != 1 {
		t.Fatalf("controlled legacy session did not reach its existing path: calls=%d err=%v", calls, err)
	}
	var count int
	err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.diagnostic_events WHERE correlation_id=$1 AND event_type='visual_identity.model_binding.legacy_unfrozen' AND payload->>'stage'='text'`, "visual_identity:"+f.sessionID).Scan(&count)
	if err != nil || count != 1 {
		t.Fatalf("legacy session did not record its unfrozen binding: count=%d err=%v", count, err)
	}
}

func TestInvalidVisualSnapshotDoesNotBlockCompletedSession(t *testing.T) {
	f := newVisualIdentityToolFixture(t)
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlight_visual_identity_sessions SET status='completed',model_provenance='null'::jsonb WHERE id=$1`, f.sessionID); err != nil {
		t.Fatal(err)
	}
	result, err := f.app.RunVisualIdentityAgent(f.ctx, VisualIdentityAgentInput{SessionID: f.sessionID})
	if err != nil || result.Status != "completed" {
		t.Fatalf("completed session unexpectedly required a valid snapshot: status=%q err=%v", result.Status, err)
	}
	seedCognitiveProviderRole(t, f.ctx, f.repository, "invalid-visual-snapshot-endpoint")
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlight_visual_identity_sessions SET status='queued' WHERE id=$1`, f.sessionID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.RunVisualIdentityAgent(f.ctx, VisualIdentityAgentInput{SessionID: f.sessionID}); !errors.Is(err, errVisualModelSnapshotInvalid) {
		t.Fatalf("invalid snapshot silently selected a model: %v", err)
	}
}

func TestVisualSessionMissingVisionBindingStillPausesForManualReview(t *testing.T) {
	f := newVisualIdentityToolFixture(t)
	seedCognitiveProviderRole(t, f.ctx, f.repository, "visual-unconfigured-endpoint")
	enableControlledVision(t, f.ctx, f.repository)
	if _, err := f.repository.Pool().Exec(f.ctx, `DELETE FROM public.model_roles WHERE role='vision_understanding'`); err != nil {
		t.Fatal(err)
	}
	receipt := f.generateCandidate(t, "visual-unconfigured-candidate")
	f.makeCurrentCandidateReady(t, stringValue(mapValue(receipt.Result.Output)["media_intent_id"]))
	// No object storage is configured: missing vision binding must still pause
	// before attempting to read the candidate, as in the existing workflow.
	result, err := f.app.RunVisualIdentityAgent(f.ctx, VisualIdentityAgentInput{SessionID: f.sessionID})
	if err != nil || result.Status != "awaiting_review" || result.ErrorCode != "vision_model_unconfigured" {
		t.Fatalf("missing visual binding did not retain manual-review behavior: status=%q code=%q err=%v", result.Status, result.ErrorCode, err)
	}
}

func TestVisualSessionDoesNotFreezeBeforePluginAdmission(t *testing.T) {
	f := newVisualIdentityToolFixture(t)
	seedCognitiveProviderRole(t, f.ctx, f.repository, "visual-plugin-admission-endpoint")
	enableControlledVision(t, f.ctx, f.repository)
	receipt := f.generateCandidate(t, "visual-plugin-admission-candidate")
	f.makeCurrentCandidateReady(t, stringValue(mapValue(receipt.Result.Output)["media_intent_id"]))
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.runtime_settings(key,value_json) VALUES('plugins.domains','{"image":"invalid","moment":true}') ON CONFLICT(key) DO UPDATE SET value_json=excluded.value_json`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.RunVisualIdentityAgent(f.ctx, VisualIdentityAgentInput{SessionID: f.sessionID}); err == nil || !strings.Contains(err.Error(), "domain_plugins_config_invalid") {
		t.Fatalf("invalid plugin configuration did not stop admission: %v", err)
	}
	var hasSnapshot bool
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT model_provenance ? $2 FROM public.fluctlight_visual_identity_sessions WHERE id=$1`, f.sessionID, visualModelSnapshotsKey).Scan(&hasSnapshot); err != nil || hasSnapshot {
		t.Fatalf("model was frozen before plugin admission: saved=%t err=%v", hasSnapshot, err)
	}
}

func TestVisualIdentitySessionKeepsVisionModelAndCurrentAdmission(t *testing.T) {
	f := newVisualIdentityToolFixture(t)
	seedCognitiveProviderRole(t, f.ctx, f.repository, "visual-session-vision-endpoint")
	enableControlledVision(t, f.ctx, f.repository)
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.model_roles SET model_id='first-vision' WHERE role='vision_understanding'`); err != nil {
		t.Fatal(err)
	}
	receipt := f.generateCandidate(t, "visual-session-vision-generate")
	mediaID := stringValue(mapValue(receipt.Result.Output)["media_intent_id"])
	f.makeCurrentCandidateReady(t, mediaID)
	content := ownerMediaPNG(t)
	s3 := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"controlled-etag"`)
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Last-Modified", "Mon, 19 Oct 2026 10:00:00 GMT")
		w.Header().Set("Content-Length", strconv.Itoa(len(content)))
		if r.Method == http.MethodGet {
			_, _ = w.Write(content)
		} else {
			_, _ = io.Copy(io.Discard, r.Body)
		}
	}))
	defer s3.Close()
	storage, err := minio.New(strings.TrimPrefix(s3.URL, "https://"), &minio.Options{Secure: true, Transport: s3.Client().Transport, Creds: credentials.NewStaticV4("controlled", "controlled", ""), Region: "us-east-1", BucketLookup: minio.BucketLookupPath})
	if err != nil {
		t.Fatal(err)
	}
	f.app.Storage, f.app.S3Bucket = storage, "visual-test-bucket"
	var models []string
	var reviewImage string
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		models = append(models, stringValue(payload["model"]))
		if len(models) == 1 {
			return fakeProviderResult{Status: http.StatusBadRequest}
		}
		if len(models) == 2 {
			for _, rawMessage := range arrayValue(payload["messages"]) {
				message := mapValue(rawMessage)
				if message["role"] != "user" {
					continue
				}
				for _, rawPart := range arrayValue(message["content"]) {
					part := mapValue(rawPart)
					if part["type"] == "image_url" {
						reviewImage = stringValue(mapValue(part["image_url"])["url"])
					}
				}
			}
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("visual-review-snapshot", visualIdentityCommitReviewCapabilityName, visualIdentityAcceptedReview())}}
		}
		return fakeProviderResult{Structured: map[string]any{"status": "waiting", "stage": "character_sheet_pending", "summary": "审核已提交"}}
	})
	f.app.Provider.HTTP = &http.Client{Transport: router}
	first, err := f.app.RunVisualIdentityAgent(f.ctx, VisualIdentityAgentInput{SessionID: f.sessionID})
	if err != nil || first.Status != "awaiting_review" || len(models) != 1 {
		t.Fatalf("first controlled review did not pause: status=%q models=%v err=%v", first.Status, models, err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.model_roles SET model_id='next-vision' WHERE role='vision_understanding'`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.runtime_settings SET revision=revision+1 WHERE key='vision.understanding'`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.fluctlight_visual_identity_attempts(id,session_id,visual_identity_id,fluctlight_id,attempt_number,status,input_snapshot,renderer_constraints,media_intent_id,candidate_asset_id) SELECT $2,session_id,visual_identity_id,fluctlight_id,2,'queued',input_snapshot,renderer_constraints,media_intent_id,candidate_asset_id FROM public.fluctlight_visual_identity_attempts WHERE session_id=$1 AND attempt_number=1`, f.sessionID, visualIdentityAttemptID(f.sessionID, 2)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlight_visual_identity_sessions SET current_attempt=2,status='running' WHERE id=$1`, f.sessionID); err != nil {
		t.Fatal(err)
	}
	restarted := newTestApp(t, f.repository, router)
	restarted.Storage, restarted.S3Bucket = storage, "visual-test-bucket"
	result, err := restarted.RunVisualIdentityAgent(f.ctx, VisualIdentityAgentInput{SessionID: f.sessionID})
	if err != nil || result.Status != "waiting" || len(models) != 3 || models[0] != "first-vision" || models[1] != "first-vision" || models[2] != "first-vision" {
		t.Fatalf("vision checkpoint changed its model or failed admission: status=%q models=%v err=%v", result.Status, models, err)
	}
	if reviewImage != "data:image/png;base64,"+base64.StdEncoding.EncodeToString(content) {
		t.Fatal("restored visual review did not receive the image as a native content block")
	}
	var accepted bool
	err = f.repository.Pool().QueryRow(f.ctx, `SELECT decision='accepted' AND vision_result->>'model_id'='first-vision' AND (vision_result->>'config_version')::bigint=(SELECT revision FROM public.runtime_settings WHERE key='vision.understanding') FROM public.fluctlight_visual_identity_attempts WHERE id=$1`, visualIdentityAttemptID(f.sessionID, 2)).Scan(&accepted)
	if err != nil || !accepted {
		t.Fatalf("restored review did not commit with the current admission version: accepted=%t err=%v", accepted, err)
	}
}
