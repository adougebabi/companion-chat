package core

import (
	"net/url"
	"strings"
	"testing"
)

// These are management and staged-asset fixtures. No model, Comfy or storage
// bytes are exercised; real image/vision acceptance is delegated to user L09.
func TestOwnerVisualBriefPersistenceCASFrozenSessionAndNoLifeMutation(t *testing.T) {
	f := newVisualIdentityToolFixture(t)
	// The fixture's existing queued session is explicitly closed; old records
	// remain so a new Owner-triggered session can freeze the new brief.
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlight_visual_identity_sessions SET status='cancelled' WHERE id=$1`, f.sessionID); err != nil {
		t.Fatal(err)
	}
	before, beforeBody, beforeWardrobe, err := f.app.readEffectiveLifeSnapshot(f.ctx, f.fluctlightID, f.app.now())
	if err != nil {
		t.Fatal(err)
	}
	path := "/internal/fluctlights/" + f.fluctlightID + "/visual-brief"
	body := map[string]any{"appearance": "视觉参考A", "clothing": "不得覆盖真实穿着", "constraints": "保持同一张脸", "references": "参考说明", "expected_revision": 0, "reason": "管理测试", "idempotency_key": "brief-a"}
	saved, err := f.app.DispatchOwnerSurface(f.ctx, "PUT", path, nil, f.ownerID, body)
	if err != nil || intValue(saved["revision"]) != 1 {
		t.Fatalf("save %#v %v", saved, err)
	}
	restarted := &App{DB: f.repository}
	read, err := restarted.OwnerVisualBrief(f.ctx, f.ownerID, f.fluctlightID)
	if err != nil || mapValue(read["brief"])["appearance"] != "视觉参考A" {
		t.Fatal("not durable", read, err)
	}
	stale := cloneMap(body)
	stale["idempotency_key"] = "stale"
	if _, err := f.app.DispatchOwnerSurface(f.ctx, "PUT", path, nil, f.ownerID, stale); err == nil {
		t.Fatal("stale brief accepted")
	}
	generated, err := f.app.DispatchOwnerSurface(f.ctx, "POST", "/internal/fluctlights/"+f.fluctlightID+"/visual-identity/generate", nil, f.ownerID, map[string]any{"expected_visual_revision": 0, "expected_brief_revision": 1, "reason": "freeze references", "idempotency_key": "generate-a"})
	if err != nil {
		t.Fatal(err)
	}
	next := cloneMap(body)
	next["expected_revision"], next["appearance"], next["idempotency_key"] = 1, "视觉参考B", "brief-b"
	if _, err := f.app.DispatchOwnerSurface(f.ctx, "PUT", path, nil, f.ownerID, next); err != nil {
		t.Fatal(err)
	}
	var raw []byte
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT input_snapshot FROM public.fluctlight_visual_identity_attempts WHERE session_id=$1 AND attempt_number=1`, generated["session_id"]).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	snapshot := decodeObject(raw)
	if mapValue(snapshot["owner_visual_brief"])["appearance"] != "视觉参考A" {
		t.Fatal("submitted session changed brief", snapshot)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlights SET core_persona=jsonb_set(core_persona,'{life_profile,appearance}','{"hair_color":"wrong historical red","outfit":"wrong historical dress"}'::jsonb,true) WHERE id=$1`, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	if err := f.app.refreshVisualIdentityRendererConstraints(f.ctx, f.fluctlightID, visualIdentityProfileID(f.fluctlightID)); err != nil {
		t.Fatal(err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT input_snapshot FROM public.fluctlight_visual_identity_attempts WHERE session_id=$1 AND attempt_number=1`, generated["session_id"]).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if jsonString(decodeObject(raw)["life_profile"]) != jsonString(snapshot["life_profile"]) || decodeObject(raw)["source"] != "effective_life" || decodeObject(raw)["body_revision"] != snapshot["body_revision"] || decodeObject(raw)["wardrobe_revision"] != snapshot["wardrobe_revision"] {
		t.Fatal("renderer refresh replaced frozen effective Life", decodeObject(raw))
	}
	repeated, err := f.app.EnsureVisualIdentityInitializationWithPersona(f.ctx, f.fluctlightID, "wakeup", "repeated-freeze", map[string]any{"identity": map[string]any{"name": "wrong new baseline"}})
	if err != nil || repeated != generated["session_id"] {
		t.Fatal("active session was not reused", repeated, err)
	}
	if mapValue(decodeObject(raw)["owner_visual_brief"])["appearance"] != "视觉参考A" {
		t.Fatal("renderer refresh replaced frozen brief")
	}
	prompt := visualIdentityPromptFromConcept(map[string]any{"visual_identity": map[string]any{"identity_snapshot": decodeObject(raw)}})
	if !strings.Contains(prompt, "视觉参考A") || strings.Contains(prompt, "视觉参考B") {
		t.Fatal("reference consumer wrong", prompt)
	}
	model := jsonString(visualIdentityAgentSnapshot(decodeObject(raw)))
	if !strings.Contains(model, "视觉参考A") || strings.Contains(model, "updated_by_actor_id") {
		t.Fatal("model projection missing guidance or leaked metadata", model)
	}
	after, afterBody, afterWardrobe, err := f.app.readEffectiveLifeSnapshot(f.ctx, f.fluctlightID, f.app.now())
	if err != nil || beforeBody != afterBody || beforeWardrobe != afterWardrobe || jsonString(before["body_fields"]) != jsonString(after["body_fields"]) || jsonString(before["worn_items"]) != jsonString(after["worn_items"]) {
		t.Fatal("reference edit/generation mutated effective life", err)
	}
	history, err := f.app.OwnerVisualBriefHistory(f.ctx, f.ownerID, f.fluctlightID, url.Values{})
	if err != nil || len(arrayValue(history["items"])) != 2 {
		t.Fatal("missing revisions", history, err)
	}
}

func TestOwnerVisualManualReviewFencesReplayAndAtomicContinuation(t *testing.T) {
	f := newVisualIdentityToolFixture(t)
	candidate := f.generateCandidate(t, "manual-api-candidate")
	media := stringValue(mapValue(candidate.Result.Output)["media_intent_id"])
	asset := f.makeCurrentCandidateReady(t, media)
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlight_visual_identity_sessions SET status='awaiting_review' WHERE id=$1`, f.sessionID); err != nil {
		t.Fatal(err)
	}
	body := map[string]any{"decision": "accepted", "expected_attempt": 1, "expected_visual_revision": 0, "idempotency_key": "manual-review-api"}
	path := "/internal/fluctlights/" + f.fluctlightID + "/visual-sessions/" + f.sessionID + "/review"
	reviewed, err := f.app.DispatchOwnerSurface(f.ctx, "POST", path, nil, f.ownerID, body)
	if err != nil || mapValue(reviewed["result"])["canonical_asset_id"] != asset {
		t.Fatalf("review %#v %v", reviewed, err)
	}
	replay, err := f.app.DispatchOwnerSurface(f.ctx, "POST", path, nil, f.ownerID, body)
	if err != nil || replay["replayed"] != true {
		t.Fatal("manual retry did not replay", replay, err)
	}
	management, err := f.app.OwnerVisualIdentity(f.ctx, f.ownerID, f.fluctlightID)
	if err != nil {
		t.Fatal(err)
	}
	review := mapValue(mapValue(arrayValue(mapValue(management["session"])["attempts"])[0])["review"])
	if review["source"] != "manual" || review["identity_match"] != nil || review["confidence"] != nil || review["observations"] != nil || stringValue(review["reason"]) != "" {
		t.Fatal("manual review not readable", review)
	}
	var resumes int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.platform_workflow_intents WHERE intent_id LIKE 'visual_review_resume:%' AND payload->>'session_id'=$1`, f.sessionID).Scan(&resumes); err != nil || resumes != 1 {
		t.Fatal("non-atomic or duplicate continuation", resumes, err)
	}
	var source string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT source FROM public.fluctlight_visual_identity_revisions WHERE fluctlight_id=$1 AND revision=1`, f.fluctlightID).Scan(&source); err != nil || source != "owner_manual_review" {
		t.Fatal("manual review mislabeled model", source, err)
	}
	body["idempotency_key"], body["expected_attempt"] = "different-command", 2
	if _, err := f.app.DispatchOwnerSurface(f.ctx, "POST", path, nil, f.ownerID, body); err == nil {
		t.Fatal("wrong candidate attempt accepted")
	}
}

func TestOwnerVisualManualRejectionCarriesFeedbackToNextAttempt(t *testing.T) {
	f := newVisualIdentityToolFixture(t)
	candidate := f.generateCandidate(t, "manual-reject-candidate")
	media := stringValue(mapValue(candidate.Result.Output)["media_intent_id"])
	f.makeCurrentCandidateReady(t, media)
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlight_visual_identity_sessions SET status='awaiting_review' WHERE id=$1`, f.sessionID); err != nil {
		t.Fatal(err)
	}
	body := map[string]any{"decision": "regenerate", "feedback": "侧面视图缺失，请补齐", "expected_attempt": 1, "expected_visual_revision": 0, "idempotency_key": "manual-reject"}
	path := "/internal/fluctlights/" + f.fluctlightID + "/visual-sessions/" + f.sessionID + "/review"
	result, err := f.app.DispatchOwnerSurface(f.ctx, "POST", path, nil, f.ownerID, body)
	if err != nil || intValue(mapValue(result["result"])["next_attempt"]) != 2 {
		t.Fatalf("manual rejection %#v %v", result, err)
	}
	var snapshotRaw []byte
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT input_snapshot FROM public.fluctlight_visual_identity_attempts WHERE session_id=$1 AND attempt_number=2`, f.sessionID).Scan(&snapshotRaw); err != nil {
		t.Fatal(err)
	}
	prompt := visualIdentityPromptFromConcept(map[string]any{"visual_identity": map[string]any{"identity_snapshot": decodeObject(snapshotRaw)}})
	if !strings.Contains(prompt, "侧面视图缺失，请补齐") {
		t.Fatalf("next generation lost manual feedback: %s", prompt)
	}
	var timelineSummary string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT summary FROM public.fluctlight_visual_identity_timeline WHERE session_id=$1 AND stage='regenerate' ORDER BY occurred_at DESC LIMIT 1`, f.sessionID).Scan(&timelineSummary); err != nil || !strings.Contains(timelineSummary, "侧面视图缺失") {
		t.Fatalf("manual feedback missing from timeline: %q, %v", timelineSummary, err)
	}
}

func TestAcceptedReferenceQueuesIndependentAvatarAndCharacterSheet(t *testing.T) {
	f := newVisualIdentityToolFixture(t)
	candidate := f.generateCandidate(t, "avatar-source-candidate")
	media := stringValue(mapValue(candidate.Result.Output)["media_intent_id"])
	canonical := f.makeCurrentCandidateReady(t, media)
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlight_visual_identity_sessions SET status='awaiting_review' WHERE id=$1`, f.sessionID); err != nil {
		t.Fatal(err)
	}
	body := map[string]any{"decision": "accepted", "expected_attempt": 1, "expected_visual_revision": 0, "idempotency_key": "avatar-source-review"}
	path := "/internal/fluctlights/" + f.fluctlightID + "/visual-sessions/" + f.sessionID + "/review"
	if _, err := f.app.DispatchOwnerSurface(f.ctx, "POST", path, nil, f.ownerID, body); err != nil {
		t.Fatal(err)
	}
	var avatarIntent, sheetIntent string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT COALESCE(v.generated_avatar_media_intent_id,''),COALESCE(s.character_sheet_media_intent_id,'') FROM public.fluctlight_visual_identities v JOIN public.fluctlight_visual_identity_sessions s ON s.visual_identity_id=v.id WHERE s.id=$1`, f.sessionID).Scan(&avatarIntent, &sheetIntent); err != nil {
		t.Fatal(err)
	}
	if avatarIntent == "" || sheetIntent == "" || avatarIntent == sheetIntent || "asset_"+avatarIntent == canonical {
		t.Fatalf("three separate image identities not queued: reference=%q sheet=%q avatar=%q", canonical, sheetIntent, avatarIntent)
	}
	var raw string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT prompt FROM public.media_intents WHERE id=$1`, avatarIntent).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	concept := decodeObject([]byte(raw))
	if stringValue(concept["stage"]) != "avatar" || visualIdentityReferenceAssetID(concept) != canonical || !strings.Contains(visualIdentityPromptFromConcept(concept), "角色头像") {
		t.Fatalf("avatar intent lost its dedicated prompt or accepted reference: %#v", concept)
	}
	if _, err := f.app.DispatchOwnerSurface(f.ctx, "POST", path, nil, f.ownerID, body); err != nil {
		t.Fatalf("replay failed: %v", err)
	}
	var count int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.media_intents WHERE owner_fluctlight_id=$1 AND id=$2`, f.fluctlightID, avatarIntent).Scan(&count); err != nil || count != 1 {
		t.Fatalf("avatar intent duplicated: count=%d err=%v", count, err)
	}
}
