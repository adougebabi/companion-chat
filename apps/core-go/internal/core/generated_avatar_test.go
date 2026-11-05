package core

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func TestGeneratedAvatarUsesCharacterOwnedPromptOnPhysicalRequest(t *testing.T) {
	profile := avatarCharacterProfile(map[string]any{
		"identity":           map[string]any{"name": "晓秋", "background": "喜欢旧书店", "database_id": "do-not-send"},
		"personality":        map[string]any{"description": "喜欢暖色和自然的瞬间", "openness": 0},
		"life_profile":       map[string]any{"preferences": map[string]any{"style": "胶片摄影"}},
		"personality_system": map[string]any{"active_profile_id": "chosen", "profiles": []any{map[string]any{"id": "chosen", "personality": map[string]any{"expression": "活泼"}}}},
	}, "chosen")
	dimensions, err := imageDimensions("1:1")
	if err != nil {
		t.Fatal(err)
	}
	concept := map[string]any{
		"purpose": "visual_identity", "stage": "avatar", "canonical_asset_id": "private-asset-id",
		"image_dimensions": dimensions,
		"avatar_character": profile,
		"visual_identity":  map[string]any{"identity_snapshot": map[string]any{"identity": map[string]any{"appearance": map[string]any{"description": "黑发，琥珀色眼睛"}}}},
	}
	var captured map[string]any
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		captured = cloneMap(payload)
		return fakeProviderResult{Text: "A vintage radio in a warm bookshop, softly lit by a window."}
	})
	app := &App{Provider: &ProviderClient{HTTP: &http.Client{Transport: router}}}
	ctx := withFrozenProviderAssignment(context.Background(), "media_prompt", providerAssignment{Role: "media_prompt", BaseURL: "http://controlled.invalid", ModelID: "avatar-designer", Timeout: time.Second})
	result, err := app.runMediaPromptTaskResult(ctx, MediaPromptTaskInput{Intent: mediaIntent{Kind: "image", Prompt: jsonString(concept)}})
	if err != nil {
		t.Fatal(err)
	}
	wire := jsonString(captured["messages"])
	for _, required := range []string{"喜欢旧书店", "喜欢暖色和自然的瞬间", "胶片摄影", "活泼", "黑发", "主体由你自主决定", "证件照", "英文AI绘画提示词", "768"} {
		if !strings.Contains(wire, required) {
			t.Errorf("physical avatar design request missing %q: %s", required, wire)
		}
	}
	for _, forbidden := range []string{"private-asset-id", "do-not-send", "front/side/back model views", "头部和肩部"} {
		if strings.Contains(wire, forbidden) {
			t.Errorf("avatar design request contained %q: %s", forbidden, wire)
		}
	}
	if captured["max_tokens"] != nil || captured["max_completion_tokens"] != nil || router.totalRequests() != 1 {
		t.Fatalf("invalid physical avatar request: %#v", captured)
	}
	if result.Prompt != "A vintage radio in a warm bookshop, softly lit by a window." {
		t.Fatalf("render prompt did not preserve the chosen non-human design: %s", result.Prompt)
	}
}

func TestAvatarCharacterProfileUsesCurrentPersonalityWithoutLeakingSelector(t *testing.T) {
	persona := map[string]any{"personality_system": map[string]any{"active_profile_id": "warm-id", "profiles": []any{
		map[string]any{"id": "warm-id", "personality": map[string]any{"description": "热情"}},
		map[string]any{"id": "cool-id", "personality": map[string]any{"description": "沉静"}},
	}}}
	before := jsonString(persona)
	current := avatarCharacterProfile(persona, "cool-id")
	if !strings.Contains(jsonString(current), "沉静") || strings.Contains(jsonString(current), "热情") || strings.Contains(jsonString(current), "cool-id") {
		t.Fatalf("avatar did not use the current profile: %s", jsonString(current))
	}
	if fallback := avatarCharacterProfile(persona, ""); !strings.Contains(jsonString(fallback), "热情") {
		t.Fatalf("missing runtime did not use the declared profile: %s", jsonString(fallback))
	}
	if jsonString(persona) != before {
		t.Fatal("avatar profile projection changed the source persona")
	}
}

func TestAvatarSnapshotUpgradePreservesFrozenAndSubmittedWork(t *testing.T) {
	legacy := map[string]any{"purpose": "visual_identity", "stage": "avatar"}
	if !avatarNeedsCharacterSnapshot(mediaIntent{}, legacy) {
		t.Fatal("legacy pending avatar needs a character snapshot before design")
	}
	if !avatarNeedsCharacterSnapshot(mediaIntent{ProviderPrompt: "old prompt", QualityVerdict: mediaQualityVerdictRetry}, legacy) {
		t.Fatal("a newly requested quality retry needs a character snapshot")
	}
	for _, intent := range []mediaIntent{{ProviderPrompt: "frozen old prompt"}, {ProviderJobID: "submitted-job"}} {
		if avatarNeedsCharacterSnapshot(intent, legacy) {
			t.Fatalf("existing prompt or submitted job must not be rewritten: %+v", intent)
		}
	}
	withSnapshot := map[string]any{"purpose": "visual_identity", "stage": "avatar", "avatar_character": map[string]any{}}
	if avatarNeedsCharacterSnapshot(mediaIntent{}, withSnapshot) {
		t.Fatal("an intentionally empty frozen character snapshot must not be reread")
	}
}

func enableVisualFixtureMediaOwner(t *testing.T, f visualIdentityToolFixture) {
	t.Helper()
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'controlled','test')`, f.ownerID); err != nil {
		t.Fatal(err)
	}
}

func acceptedVisualReferenceWithDerivedIntents(t *testing.T) (visualIdentityToolFixture, string, string, string) {
	t.Helper()
	f := newVisualIdentityToolFixture(t)
	enableVisualFixtureMediaOwner(t, f)
	candidate := f.generateCandidate(t, "derived-source")
	reference := f.makeCurrentCandidateReady(t, stringValue(mapValue(candidate.Result.Output)["media_intent_id"]))
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlight_visual_identity_sessions SET status='awaiting_review' WHERE id=$1`, f.sessionID); err != nil {
		t.Fatal(err)
	}
	body := map[string]any{"decision": "accepted", "expected_attempt": 1, "expected_visual_revision": 0, "idempotency_key": "derived-source-review"}
	path := "/internal/fluctlights/" + f.fluctlightID + "/visual-sessions/" + f.sessionID + "/review"
	if _, err := f.app.DispatchOwnerSurface(f.ctx, "POST", path, nil, f.ownerID, body); err != nil {
		t.Fatal(err)
	}
	var avatar, sheet string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT v.generated_avatar_media_intent_id,s.character_sheet_media_intent_id FROM public.fluctlight_visual_identities v JOIN public.fluctlight_visual_identity_sessions s ON s.visual_identity_id=v.id WHERE s.id=$1`, f.sessionID).Scan(&avatar, &sheet); err != nil {
		t.Fatal(err)
	}
	return f, reference, avatar, sheet
}

func completeDerivedCandidate(t *testing.T, f visualIdentityToolFixture, intentID, verdict string) string {
	t.Helper()
	asset := f.makeCurrentCandidateReady(t, intentID)
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.media_intents SET quality_verdict=$2,quality_candidate_sha256=$3 WHERE id=$1`, intentID, verdict, stableDigest(asset)); err != nil {
		t.Fatal(err)
	}
	if verdict == mediaQualityVerdictPass {
		if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.runtime_settings(key,value_json,revision) VALUES('vision.understanding','{"enabled":true,"model_ref":"vision_understanding"}',1) ON CONFLICT(key) DO UPDATE SET value_json=excluded.value_json,revision=1`); err != nil {
			t.Fatal(err)
		}
		if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlight_visual_asset_reviews SET review_data='{"vision_config_version":1,"vision_model_id":"controlled"}' WHERE media_intent_id=$1`, intentID); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.app.markMediaIntentCompleted(f.ctx, intentID, asset); err != nil {
		t.Fatal(err)
	}
	return asset
}

func TestDerivedVisualModelPassRequiresCurrentVisionConfiguration(t *testing.T) {
	f, _, avatarIntent, _ := acceptedVisualReferenceWithDerivedIntents(t)
	avatar := f.makeCurrentCandidateReady(t, avatarIntent)
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.media_intents SET quality_verdict='pass',quality_candidate_sha256=$2 WHERE id=$1`, avatarIntent, stableDigest(avatar)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlight_visual_asset_reviews SET review_data='{"vision_config_version":1,"vision_model_id":"controlled"}' WHERE media_intent_id=$1`, avatarIntent); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.runtime_settings(key,value_json,revision) VALUES('vision.understanding','{"enabled":false,"model_ref":"vision_understanding"}',2) ON CONFLICT(key) DO UPDATE SET value_json=excluded.value_json,revision=2`); err != nil {
		t.Fatal(err)
	}
	if err := f.app.markMediaIntentCompleted(f.ctx, avatarIntent, avatar); err != nil {
		t.Fatal(err)
	}
	reviews, err := readCurrentVisualAssetReviewsWith(f.ctx, f.repository.Pool(), f.fluctlightID, 1)
	if err != nil || mapValue(reviews["avatar"])["status"] != "awaiting_review" {
		t.Fatalf("stale model pass should require manual review: %#v %v", reviews, err)
	}
}

func TestDerivedVisualCandidatesAwaitManualReviewWithoutVision(t *testing.T) {
	f, reference, avatarIntent, sheetIntent := acceptedVisualReferenceWithDerivedIntents(t)
	avatar := completeDerivedCandidate(t, f, avatarIntent, mediaQualityVerdictSkip)
	sheet := completeDerivedCandidate(t, f, sheetIntent, mediaQualityVerdictSkip)
	if reference == avatar || reference == sheet || avatar == sheet {
		t.Fatal("reference, sheet, and avatar are not independent assets")
	}
	for _, candidate := range []string{avatar, sheet} {
		count, err := mediaReferenceCountWith(f.ctx, f.repository.Pool(), candidate)
		if err != nil || count < 1 {
			t.Fatalf("unreviewed visual candidate is not protected from deletion: %q %d %v", candidate, count, err)
		}
	}
	reviews, err := readCurrentVisualAssetReviewsWith(f.ctx, f.repository.Pool(), f.fluctlightID, 1)
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"avatar", "character_sheet"} {
		if stringValue(mapValue(reviews[kind])["status"]) != "awaiting_review" {
			t.Fatalf("%s should await human review: %#v", kind, reviews[kind])
		}
	}
	state, err := f.app.loadVisualIdentityAgentState(f.ctx, f.sessionID)
	if err != nil || state.ActionRequired != "" || visualIdentityAgentOutputFromState(state, "").Status != "awaiting_review" {
		t.Fatalf("unreviewed character sheet should pause its workflow: %#v %v", state, err)
	}
	view, err := f.app.displayAvatarWith(f.ctx, f.repository.Pool(), f.ownerID, f.fluctlightID)
	if err != nil || view["avatar"] != nil || view["read_state"] != "awaiting_review" {
		t.Fatalf("unreviewed avatar or reference was displayed: %#v %v", view, err)
	}
}

func TestDerivedAvatarModelPass(t *testing.T) {
	f, _, avatarIntent, _ := acceptedVisualReferenceWithDerivedIntents(t)
	avatar := completeDerivedCandidate(t, f, avatarIntent, mediaQualityVerdictPass)
	var stored string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT COALESCE(generated_avatar_asset_id,'') FROM public.fluctlight_visual_identities WHERE fluctlight_id=$1`, f.fluctlightID).Scan(&stored); err != nil || stored != avatar {
		t.Fatalf("reviewed avatar not attached: %q %v", stored, err)
	}
	if count, err := mediaReferenceCountWith(f.ctx, f.repository.Pool(), avatar); err != nil || count < 1 {
		t.Fatalf("accepted generated avatar is not protected from deletion: %d %v", count, err)
	}
	reviews, err := readCurrentVisualAssetReviewsWith(f.ctx, f.repository.Pool(), f.fluctlightID, 1)
	if err != nil || mapValue(reviews["avatar"])["review_source"] != "vision" {
		t.Fatalf("model provenance not retained: %#v %v", reviews, err)
	}
}

func TestStaleAvatarCompletionCannotReplaceNewReference(t *testing.T) {
	f, _, avatarIntent, _ := acceptedVisualReferenceWithDerivedIntents(t)
	avatar := f.makeCurrentCandidateReady(t, avatarIntent)
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlight_visual_identities SET current_revision=2,canonical_asset_id='new-reference',generated_avatar_media_intent_id='new-avatar-intent' WHERE fluctlight_id=$1`, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	if err := f.app.markMediaIntentCompleted(f.ctx, avatarIntent, avatar); err != nil {
		t.Fatal(err)
	}
	var status, current string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT r.status,COALESCE(v.generated_avatar_asset_id,'') FROM public.fluctlight_visual_asset_reviews r JOIN public.fluctlight_visual_identities v ON v.id=r.visual_identity_id WHERE r.media_intent_id=$1`, avatarIntent).Scan(&status, &current); err != nil || status != "superseded" || current != "" {
		t.Fatalf("stale avatar completion affected current identity: status=%q avatar=%q err=%v", status, current, err)
	}
}

func TestOwnerReviewsDerivedImagesIndependentlyAndPreservesManualAvatar(t *testing.T) {
	f, _, avatarIntent, sheetIntent := acceptedVisualReferenceWithDerivedIntents(t)
	avatar := completeDerivedCandidate(t, f, avatarIntent, mediaQualityVerdictSkip)
	completeDerivedCandidate(t, f, sheetIntent, mediaQualityVerdictSkip)
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.owner_surface_records(id,owner_actor_id,fluctlight_id,kind,resource_key,status,payload,revision) VALUES('manual-avatar-fixture',$1,$2,'visual_identity','current','saved','{"mode":"asset","asset_id":"chosen-by-owner"}',1)`, f.ownerID, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	reviews, err := readCurrentVisualAssetReviewsWith(f.ctx, f.repository.Pool(), f.fluctlightID, 1)
	if err != nil {
		t.Fatal(err)
	}
	avatarReview := mapValue(reviews["avatar"])
	avatarPath := "/internal/fluctlights/" + f.fluctlightID + "/visual-assets/" + stringValue(avatarReview["id"]) + "/review"
	accept := map[string]any{"decision": "accepted", "expected_visual_revision": 1, "expected_review_revision": 1, "idempotency_key": "accept-generated-avatar"}
	result, err := f.app.DispatchOwnerMediaSurface(f.ctx, "POST", avatarPath, nil, f.ownerID, accept)
	if err != nil || result["candidate_asset_id"] != avatar {
		t.Fatalf("manual avatar acceptance: %#v %v", result, err)
	}
	replay, err := f.app.DispatchOwnerMediaSurface(f.ctx, "POST", avatarPath, nil, f.ownerID, accept)
	if err != nil || replay["_replay"] != true {
		t.Fatalf("manual avatar review did not replay: %#v %v", replay, err)
	}
	var selection []byte
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT payload FROM public.owner_surface_records WHERE id='manual-avatar-fixture'`).Scan(&selection); err != nil || stringValue(decodeObject(selection)["asset_id"]) != "chosen-by-owner" {
		t.Fatalf("generated avatar overwrote explicit selection: %s %v", selection, err)
	}
	sheetReview := mapValue(reviews["character_sheet"])
	sheetPath := "/internal/fluctlights/" + f.fluctlightID + "/visual-assets/" + stringValue(sheetReview["id"]) + "/review"
	reject := map[string]any{"decision": "regenerate", "feedback": "补齐侧面视图", "expected_visual_revision": 1, "expected_review_revision": 1, "idempotency_key": "reject-generated-sheet"}
	rejected, err := f.app.DispatchOwnerMediaSurface(f.ctx, "POST", sheetPath, nil, f.ownerID, reject)
	if err != nil || stringValue(rejected["next_intent_id"]) == "" {
		t.Fatalf("sheet rejection: %#v %v", rejected, err)
	}
	var prompt string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT prompt FROM public.media_intents WHERE id=$1`, rejected["next_intent_id"]).Scan(&prompt); err != nil || !strings.Contains(visualIdentityPromptFromConcept(decodeObject([]byte(prompt))), "补齐侧面视图") {
		t.Fatalf("sheet feedback not carried to regeneration: %s %v", prompt, err)
	}
	stale := map[string]any{"decision": "accepted", "expected_visual_revision": 1, "expected_review_revision": 1, "idempotency_key": "stale-sheet-review"}
	if _, err := f.app.DispatchOwnerMediaSurface(f.ctx, "POST", sheetPath, nil, f.ownerID, stale); err == nil {
		t.Fatal("already reviewed sheet accepted again")
	}
}

func TestOwnerAcceptsSheetAndResumesOnlyThatWorkflow(t *testing.T) {
	f, reference, avatarIntent, sheetIntent := acceptedVisualReferenceWithDerivedIntents(t)
	completeDerivedCandidate(t, f, avatarIntent, mediaQualityVerdictSkip)
	sheet := completeDerivedCandidate(t, f, sheetIntent, mediaQualityVerdictSkip)
	reviews, err := readCurrentVisualAssetReviewsWith(f.ctx, f.repository.Pool(), f.fluctlightID, 1)
	if err != nil {
		t.Fatal(err)
	}
	row := mapValue(reviews["character_sheet"])
	path := "/internal/fluctlights/" + f.fluctlightID + "/visual-assets/" + stringValue(row["id"]) + "/review"
	body := map[string]any{"decision": "accepted", "expected_visual_revision": 1, "expected_review_revision": 1, "idempotency_key": "accept-sheet"}
	if _, err := f.app.DispatchOwnerMediaSurface(f.ctx, "POST", path, nil, f.ownerID, body); err != nil {
		t.Fatal(err)
	}
	state, err := f.app.loadVisualIdentityAgentState(f.ctx, f.sessionID)
	if err != nil || state.ActionRequired != visualIdentityFinalizeCapabilityName {
		t.Fatalf("accepted sheet did not resume finalization: %#v %v", state, err)
	}
	result, err := f.app.ExecuteTool(f.ctx, f.request(visualIdentityFinalizeCapabilityName, "finalize-reviewed-sheet", map[string]any{}))
	if err != nil || result.Result.Status != "completed" || mapValue(result.Result.Output)["character_sheet_asset_id"] != sheet {
		t.Fatalf("reviewed sheet did not finalize: %#v %v", result, err)
	}
	var canonical, storedSheet, storedAvatar string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT canonical_asset_id,character_sheet_asset_id,COALESCE(generated_avatar_asset_id,'') FROM public.fluctlight_visual_identities WHERE fluctlight_id=$1`, f.fluctlightID).Scan(&canonical, &storedSheet, &storedAvatar); err != nil || canonical != reference || storedSheet != sheet || storedAvatar != "" {
		t.Fatalf("sheet acceptance changed another image: %q %q %q %v", canonical, storedSheet, storedAvatar, err)
	}
	var resumes int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.platform_workflow_intents WHERE intent_id LIKE 'visual_asset_review_resume:%' AND payload->>'session_id'=$1`, f.sessionID).Scan(&resumes); err != nil || resumes != 1 {
		t.Fatalf("sheet review continuation missing or duplicated: %d %v", resumes, err)
	}
}

func TestExistingReferenceCanQueueAvatarWithoutChangingSelection(t *testing.T) {
	f := newVisualIdentityToolFixture(t)
	enableVisualFixtureMediaOwner(t, f)
	candidate := f.generateCandidate(t, "legacy-avatar-source")
	reference := f.makeCurrentCandidateReady(t, stringValue(mapValue(candidate.Result.Output)["media_intent_id"]))
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlight_visual_identities SET status='active',current_revision=1,canonical_asset_id=$2 WHERE fluctlight_id=$1`, f.fluctlightID, reference); err != nil {
		t.Fatal(err)
	}
	path := "/internal/fluctlights/" + f.fluctlightID + "/display-avatar/generate"
	body := map[string]any{"expected_visual_revision": 1, "idempotency_key": "legacy-avatar-generate"}
	result, err := f.app.DispatchOwnerMediaSurface(f.ctx, "POST", path, nil, f.ownerID, body)
	if err != nil || stringValue(result["id"]) == "" {
		t.Fatalf("old role avatar generation: %#v %v", result, err)
	}
	replay, err := f.app.DispatchOwnerMediaSurface(f.ctx, "POST", path, nil, f.ownerID, body)
	if err != nil || replay["_replay"] != true || replay["id"] != result["id"] {
		t.Fatalf("old role generation replay: %#v %v", replay, err)
	}
	body["idempotency_key"] = "legacy-avatar-another-click"
	again, err := f.app.DispatchOwnerMediaSurface(f.ctx, "POST", path, nil, f.ownerID, body)
	if err != nil || again["id"] != result["id"] {
		t.Fatalf("duplicate pending avatar queued: %#v %v", again, err)
	}
}

func TestDerivedVisualCandidatesAndAcceptedAvatarBlockDeletion(t *testing.T) {
	f, _, avatarIntent, sheetIntent := acceptedVisualReferenceWithDerivedIntents(t)
	avatar := completeDerivedCandidate(t, f, avatarIntent, mediaQualityVerdictSkip)
	sheet := completeDerivedCandidate(t, f, sheetIntent, mediaQualityVerdictSkip)
	assertReferenced := func(assetID, key string) {
		t.Helper()
		asset, err := f.app.ownerMediaAssetWith(f.ctx, f.repository.Pool(), f.ownerID, assetID, false)
		if err != nil {
			t.Fatal(err)
		}
		_, err = f.app.DispatchOwnerMediaSurface(f.ctx, "DELETE", "/internal/media-assets/"+assetID, nil, f.ownerID, map[string]any{
			"expected_asset_revision": asset["asset_revision"], "confirm": true, "reason": "验证引用保护", "idempotency_key": key,
		})
		var surface *SurfaceError
		if !errors.As(err, &surface) || surface.Code != "media_asset_referenced" {
			t.Fatalf("referenced asset %s was not protected: %v", assetID, err)
		}
	}
	assertReferenced(avatar, "delete-pending-avatar")
	assertReferenced(sheet, "delete-pending-sheet")

	reviews, err := readCurrentVisualAssetReviewsWith(f.ctx, f.repository.Pool(), f.fluctlightID, 1)
	if err != nil {
		t.Fatal(err)
	}
	row := mapValue(reviews["avatar"])
	path := "/internal/fluctlights/" + f.fluctlightID + "/visual-assets/" + stringValue(row["id"]) + "/review"
	if _, err := f.app.DispatchOwnerMediaSurface(f.ctx, "POST", path, nil, f.ownerID, map[string]any{
		"decision": "accepted", "expected_visual_revision": 1, "expected_review_revision": 1, "idempotency_key": "accept-protected-avatar",
	}); err != nil {
		t.Fatal(err)
	}
	assertReferenced(avatar, "delete-accepted-avatar")
}

func TestGeneratedAvatarRealMediaHTTPWaitsForManualReview(t *testing.T) {
	f, reference, avatarIntent, _ := acceptedVisualReferenceWithDerivedIntents(t)
	legacyPersona := map[string]any{"identity": map[string]any{"name": "澄光"}, "personality_system": map[string]any{"active_profile_id": "warm", "profiles": []any{map[string]any{"id": "warm", "personality": map[string]any{"description": "热情"}}, map[string]any{"id": "cool", "personality": map[string]any{"description": "沉静喜欢老唱片"}}}}}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlights SET core_persona=$2 WHERE id=$1`, f.fluctlightID, jsonBytes(legacyPersona)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.fluctlight_personality_runtime(fluctlight_id,active_profile_id) VALUES($1,'cool') ON CONFLICT(fluctlight_id) DO UPDATE SET active_profile_id='cool'`, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.media_intents SET prompt=(prompt::jsonb-'avatar_character')::text WHERE id=$1`, avatarIntent); err != nil {
		t.Fatal(err)
	}
	png := ownerMediaPNG(t)
	s3 := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"controlled-etag"`)
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Last-Modified", "Mon, 19 Oct 2026 10:00:00 GMT")
		if r.Method == http.MethodPut {
			_, _ = io.Copy(io.Discard, r.Body)
			return
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(png)))
		if r.Method == http.MethodGet {
			_, _ = w.Write(png)
		}
	}))
	defer s3.Close()
	storage, err := minio.New(strings.TrimPrefix(s3.URL, "https://"), &minio.Options{Secure: true, Transport: s3.Client().Transport, Creds: credentials.NewStaticV4("controlled", "controlled", ""), Region: "us-east-1", BucketLookup: minio.BucketLookupPath})
	if err != nil {
		t.Fatal(err)
	}
	f.app.Storage, f.app.S3Bucket = storage, "visual-test-bucket"
	var submitted map[string]any
	var designRequest map[string]any
	uploaded := false
	comfy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/chat/completions"):
			if err := json.NewDecoder(r.Body).Decode(&designRequest); err != nil {
				t.Error(err)
			}
			_, _ = io.WriteString(w, `{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"A relaxed portrait in a sunlit reading room."}}]}`)
		case r.URL.Path == "/upload/image":
			file, _, err := r.FormFile("image")
			if err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			content, _ := io.ReadAll(file)
			file.Close()
			uploaded = bytes.Equal(content, png)
			_, _ = io.WriteString(w, `{"name":"confirmed-reference.png"}`)
		case r.URL.Path == "/prompt":
			if err := json.NewDecoder(r.Body).Decode(&submitted); err != nil {
				t.Error(err)
			}
			_, _ = io.WriteString(w, `{"prompt_id":"avatar-job"}`)
		case r.URL.Path == "/history/avatar-job":
			_, _ = io.WriteString(w, `{"avatar-job":{"outputs":{"save":{"images":[{"filename":"avatar.png","type":"output"}]}}}}`)
		case r.URL.Path == "/view":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(png)
		default:
			w.WriteHeader(404)
		}
	}))
	defer comfy.Close()
	f.app.Provider.HTTP = comfy.Client()
	graph := map[string]any{
		"positive":  map[string]any{"class_type": "CLIPTextEncode", "inputs": map[string]any{"text": "{{prompt}}"}},
		"reference": map[string]any{"class_type": "LoadImage", "inputs": map[string]any{"image": "{{visual_identity_reference_image}}"}},
	}
	config := map[string]any{"baseUrl": comfy.URL, "workflow": graph, "visual_identity_workflows": map[string]any{"avatar": graph}}
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.runtime_settings(key,value_json) VALUES('media.comfyui',$1) ON CONFLICT(key) DO UPDATE SET value_json=excluded.value_json`, jsonBytes(config)); err != nil {
		t.Fatal(err)
	}
	ctx := withFrozenProviderAssignment(f.ctx, "media_prompt", providerAssignment{Role: "media_prompt", BaseURL: comfy.URL, ModelID: "avatar-designer", Timeout: time.Second})
	result, err := f.app.ProcessMediaIntent(ctx, avatarIntent)
	if err != nil || result["status"] != "completed" || !uploaded {
		t.Fatalf("avatar media execution: %#v uploaded=%v err=%v", result, uploaded, err)
	}
	if wire := jsonString(designRequest["messages"]); !strings.Contains(wire, "沉静喜欢老唱片") || strings.Contains(wire, "热情") {
		t.Fatalf("legacy pending avatar did not freeze its current personality: %s", wire)
	}
	var frozenPrompt string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT prompt FROM public.media_intents WHERE id=$1`, avatarIntent).Scan(&frozenPrompt); err != nil || !strings.Contains(frozenPrompt, "沉静喜欢老唱片") {
		t.Fatalf("legacy avatar character profile was not frozen: %s %v", frozenPrompt, err)
	}
	wire := jsonString(submitted)
	if !strings.Contains(wire, "sunlit reading room") || !strings.Contains(wire, "confirmed-reference.png") || strings.Contains(wire, "正面/侧面/背面三视图") {
		t.Fatalf("wrong ComfyUI request: %s", wire)
	}
	reviews, err := readCurrentVisualAssetReviewsWith(f.ctx, f.repository.Pool(), f.fluctlightID, 1)
	avatar := mapValue(reviews["avatar"])
	if err != nil || avatar["status"] != "awaiting_review" || avatar["candidate_asset_id"] == reference {
		t.Fatalf("generated avatar should be a separate candidate awaiting review: %#v %v", reviews, err)
	}
}

func TestDerivedVisualModelReviewReceivesReferenceAndCandidate(t *testing.T) {
	f, _, avatarIntent, _ := acceptedVisualReferenceWithDerivedIntents(t)
	reference := ownerMediaPNG(t)
	candidate := append(append([]byte(nil), reference...), 1)
	s3 := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"controlled-etag"`)
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Last-Modified", "Mon, 19 Oct 2026 10:00:00 GMT")
		w.Header().Set("Content-Length", strconv.Itoa(len(reference)))
		if r.Method == http.MethodGet {
			_, _ = w.Write(reference)
		}
	}))
	defer s3.Close()
	storage, err := minio.New(strings.TrimPrefix(s3.URL, "https://"), &minio.Options{Secure: true, Transport: s3.Client().Transport, Creds: credentials.NewStaticV4("controlled", "controlled", ""), Region: "us-east-1", BucketLookup: minio.BucketLookupPath})
	if err != nil {
		t.Fatal(err)
	}
	f.app.Storage = storage
	var payload map[string]any
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/json")
		content := jsonString(map[string]any{"schema_version": 1, "verdict": "pass", "violations": []any{}, "retry_guidance": "", "observed_facts": map[string]any{"subject_matches": true, "appearance_matches": true, "scene_matches": true, "capture_matches": true, "framing_matches": true}})
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": content}}}})
	}))
	defer provider.Close()
	seedCognitiveProviderRole(t, f.ctx, f.repository, "derived-vision-endpoint")
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.provider_endpoints SET base_url=$2 WHERE id=$1`, "derived-vision-endpoint", provider.URL); err != nil {
		t.Fatal(err)
	}
	enableControlledVision(t, f.ctx, f.repository)
	f.app.Provider.HTTP = provider.Client()
	intent, err := f.app.readMediaIntent(f.ctx, avatarIntent)
	if err != nil {
		t.Fatal(err)
	}
	intent.ProviderPrompt = "A vintage radio, arranged for a square social avatar."
	result, err := f.app.RunMediaQualityTask(f.ctx, MediaQualityTaskInput{Intent: intent, ContentType: "image/png", Content: candidate})
	if err != nil || result.Verdict != "pass" || result.VisionConfigVersion != 1 {
		t.Fatalf("model review failed: %#v %v", result, err)
	}
	wire := jsonString(payload["messages"])
	for _, required := range []string{"data:image/png;base64," + base64.StdEncoding.EncodeToString(reference), "data:image/png;base64," + base64.StdEncoding.EncodeToString(candidate), "第一张图片是已确认的角色形象参考", "Do not require a person or face", "Only a genuine pass"} {
		if !strings.Contains(wire, required) {
			t.Fatalf("model review request missing %q: %s", required, wire)
		}
	}
	if strings.Contains(wire, "delivers the second candidate even when its verdict is retry or reject") {
		t.Fatal("derived image review incorrectly promises delivery of a failed candidate")
	}
	if payload["max_tokens"] != nil || payload["max_completion_tokens"] != nil {
		t.Fatal("model review unexpectedly contains output token limits")
	}
}

func TestGeneratedAvatarRealMediaHTTPModelReview(t *testing.T) {
	t.Run("pass", func(t *testing.T) { testGeneratedAvatarRealMediaHTTPModelReview(t, "pass") })
	t.Run("reject after retry", func(t *testing.T) { testGeneratedAvatarRealMediaHTTPModelReview(t, "reject") })
}

func testGeneratedAvatarRealMediaHTTPModelReview(t *testing.T, verdict string) {
	t.Helper()
	f, _, avatarIntent, _ := acceptedVisualReferenceWithDerivedIntents(t)
	reference := ownerMediaPNG(t)
	candidate := append(append([]byte(nil), reference...), 1)
	s3 := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"controlled-etag"`)
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Last-Modified", "Mon, 19 Oct 2026 10:00:00 GMT")
		if r.Method == http.MethodPut {
			_, _ = io.Copy(io.Discard, r.Body)
			return
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(reference)))
		if r.Method == http.MethodGet {
			_, _ = w.Write(reference)
		}
	}))
	defer s3.Close()
	storage, err := minio.New(strings.TrimPrefix(s3.URL, "https://"), &minio.Options{Secure: true, Transport: s3.Client().Transport, Creds: credentials.NewStaticV4("controlled", "controlled", ""), Region: "us-east-1", BucketLookup: minio.BucketLookupPath})
	if err != nil {
		t.Fatal(err)
	}
	f.app.Storage, f.app.S3Bucket = storage, "visual-test-bucket"
	var submissions []map[string]any
	comfy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/upload/image":
			_, _ = io.WriteString(w, `{"name":"reference.png"}`)
		case "/prompt":
			var submitted map[string]any
			if err := json.NewDecoder(r.Body).Decode(&submitted); err != nil {
				t.Error(err)
			}
			submissions = append(submissions, submitted)
			_, _ = io.WriteString(w, `{"prompt_id":"avatar-model-job"}`)
		case "/history/avatar-model-job":
			_, _ = io.WriteString(w, `{"avatar-model-job":{"outputs":{"save":{"images":[{"filename":"avatar.png","type":"output"}]}}}}`)
		case "/view":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(candidate)
		default:
			w.WriteHeader(404)
		}
	}))
	defer comfy.Close()
	var modelPayload map[string]any
	designCalls := 0
	var designRequests []string
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(jsonString(request["messages"]), "真实地思考你会为自己选择一张怎样的社交头像") {
			designCalls++
			designRequests = append(designRequests, jsonString(request["messages"]))
			_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": "A relaxed portrait with dark short hair."}}}})
			return
		}
		modelPayload = request
		violations, guidance := []any{}, ""
		if verdict == "reject" {
			violations = []any{map[string]any{"code": "appearance_mismatch", "severity": "hard", "detail": "发型与参考不一致"}}
			guidance = "保留参考图的深色短发"
		}
		content := jsonString(map[string]any{"schema_version": 1, "verdict": verdict, "violations": violations, "retry_guidance": guidance, "observed_facts": map[string]any{"subject_matches": true, "appearance_matches": verdict == "pass", "scene_matches": true, "capture_matches": true, "framing_matches": true}})
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": content}}}})
	}))
	defer provider.Close()
	seedCognitiveProviderRole(t, f.ctx, f.repository, "avatar-e2e-vision")
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.provider_endpoints SET base_url=$2 WHERE id=$1`, "avatar-e2e-vision", provider.URL); err != nil {
		t.Fatal(err)
	}
	enableControlledVision(t, f.ctx, f.repository)
	f.app.Provider.HTTP = &http.Client{}
	graph := map[string]any{"positive": map[string]any{"class_type": "CLIPTextEncode", "inputs": map[string]any{"text": "{{prompt}}"}}, "reference": map[string]any{"class_type": "LoadImage", "inputs": map[string]any{"image": "{{visual_identity_reference_image}}"}}}
	config := map[string]any{"baseUrl": comfy.URL, "workflow": graph, "visual_identity_workflows": map[string]any{"avatar": graph}}
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.runtime_settings(key,value_json) VALUES('media.comfyui',$1) ON CONFLICT(key) DO UPDATE SET value_json=excluded.value_json`, jsonBytes(config)); err != nil {
		t.Fatal(err)
	}
	ctx := withFrozenProviderAssignment(f.ctx, "media_prompt", providerAssignment{Role: "media_prompt", BaseURL: provider.URL, ModelID: "avatar-designer", Timeout: time.Second})
	result, err := f.app.ProcessMediaIntent(ctx, avatarIntent)
	if verdict == "reject" {
		if err != nil || result["status"] != "quality_retry" {
			t.Fatalf("first rejection should request another candidate: %#v %v", result, err)
		}
		result, err = f.app.ProcessMediaIntent(ctx, avatarIntent)
	}
	if err != nil || result["status"] != "completed" {
		t.Fatalf("avatar execution failed: %#v %v", result, err)
	}
	if designCalls != len(submissions) {
		t.Fatalf("avatar design must precede each generated candidate: designs=%d submissions=%d", designCalls, len(submissions))
	}
	wire := jsonString(modelPayload["messages"])
	if !strings.Contains(wire, base64.StdEncoding.EncodeToString(reference)) || !strings.Contains(wire, base64.StdEncoding.EncodeToString(candidate)) || !strings.Contains(wire, "Do not require a person or face") {
		t.Fatal("physical model request lost reference, candidate, or avatar prompt")
	}
	reviews, err := readCurrentVisualAssetReviewsWith(f.ctx, f.repository.Pool(), f.fluctlightID, 1)
	row := mapValue(reviews["avatar"])
	if verdict == "reject" {
		if len(submissions) != 2 || len(designRequests) != 2 || !strings.Contains(designRequests[1], "保留参考图的深色短发") || !strings.Contains(jsonString(submissions[1]), "dark short hair") {
			t.Fatalf("model feedback was not passed to the second generation: %#v", submissions)
		}
		if err != nil || row["status"] != "awaiting_review" || row["candidate_asset_id"] != "asset_"+avatarIntent || mapValue(row["review_data"])["verdict"] != "reject" {
			t.Fatalf("second rejection must wait for human review: %#v %v", reviews, err)
		}
		var stored string
		if err := f.repository.Pool().QueryRow(f.ctx, `SELECT COALESCE(generated_avatar_asset_id,'') FROM public.fluctlight_visual_identities WHERE fluctlight_id=$1`, f.fluctlightID).Scan(&stored); err != nil || stored != "" {
			t.Fatalf("rejected avatar became a display source: %q %v", stored, err)
		}
		return
	}
	if err != nil || row["status"] != "accepted" || row["review_source"] != "vision" || row["candidate_asset_id"] != "asset_"+avatarIntent {
		t.Fatalf("model pass not committed to the generated avatar: %#v %v", reviews, err)
	}
	var stored string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT COALESCE(generated_avatar_asset_id,'') FROM public.fluctlight_visual_identities WHERE fluctlight_id=$1`, f.fluctlightID).Scan(&stored); err != nil || stored != "asset_"+avatarIntent {
		t.Fatalf("model-approved avatar not available as display source: %q %v", stored, err)
	}
}
