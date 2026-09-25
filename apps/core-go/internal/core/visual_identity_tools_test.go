package core

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type visualIdentityToolFixture struct {
	ctx          context.Context
	repository   *PostgresRepository
	app          *App
	ownerID      string
	fluctlightID string
	sessionID    string
}

func seedUnknownEffectiveLifeForTest(t *testing.T, ctx context.Context, repository *PostgresRepository, fluctlightID string) {
	t.Helper()
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlight_appearance_states(fluctlight_id,revision,state_json,source_kind) VALUES($1,0,'{}','initialization')`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlight_wardrobe_states(fluctlight_id,revision) VALUES($1,0)`, fluctlightID); err != nil {
		t.Fatal(err)
	}
}

func newVisualIdentityToolFixture(t *testing.T) visualIdentityToolFixture {
	t.Helper()
	ctx, repository := isolatedCoreTestRepository(t)
	suffix := stableDigest(t.Name() + fmt.Sprintf("-%d", time.Now().UnixNano()))[:20]
	ownerID := "visual_tool_owner_" + suffix
	fluctlightID := "visual_tool_fluctlight_" + suffix
	seedLifeContextFluctlight(t, ctx, repository, ownerID, fluctlightID)
	seedUnknownEffectiveLifeForTest(t, ctx, repository, fluctlightID)
	persona := map[string]any{
		"identity":     map[string]any{"name": "澄光", "gender": "male", "age": 24, "appearance": map[string]any{"hair": "black short hair", "face_shape": "oval"}},
		"life_profile": map[string]any{"appearance": map[string]any{"body_type": "slim"}},
	}
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.fluctlights SET core_persona=$2 WHERE id=$1`, fluctlightID, jsonBytes(persona)); err != nil {
		t.Fatal(err)
	}
	app := newTestApp(t, repository, nil)
	sessionID, err := app.EnsureVisualIdentityInitializationWithPersona(ctx, fluctlightID, "initialization", "owner-command-"+suffix, persona)
	if err != nil {
		t.Fatal(err)
	}
	return visualIdentityToolFixture{ctx: ctx, repository: repository, app: app, ownerID: ownerID, fluctlightID: fluctlightID, sessionID: sessionID}
}

func (fixture visualIdentityToolFixture) request(name, operation string, arguments map[string]any) ToolExecutionRequest {
	return ToolExecutionRequest{
		CapabilityName: name, OperationID: operation,
		AuthorizationActorID: fixture.ownerID, FluctlightID: fixture.fluctlightID,
		TargetKind: "visual_identity_session", TargetRef: fixture.sessionID,
		EvidenceID: "owner-command-" + operation, Surface: CapabilitySurfaceVisualIdentity,
		Arguments: jsonBytes(arguments),
	}
}

func (fixture visualIdentityToolFixture) generateCandidate(t *testing.T, operation string) ToolExecutionReceipt {
	t.Helper()
	receipt, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(visualIdentityGenerateCandidateCapabilityName, operation, map[string]any{"reason": "initial"}))
	if err != nil || receipt.Result.Status != "accepted" {
		t.Fatalf("generate candidate receipt=%#v err=%v", receipt, err)
	}
	return receipt
}

func (fixture visualIdentityToolFixture) makeCurrentCandidateReady(t *testing.T, mediaIntentID string) string {
	t.Helper()
	assetID := "asset_" + mediaIntentID
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `UPDATE public.media_intents SET status='completed',revision=revision+1 WHERE id=$1`, mediaIntentID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `INSERT INTO public.media_assets(id,owner_fluctlight_id,version,kind,mime_type,byte_size,sha256,bucket,object_key,provider_request_id,workflow_id,status,ready_at) SELECT $2,owner_fluctlight_id,'v1','image','image/png',68,$3,'visual-test-bucket',$4,provider_request_id,workflow_id,'ready',now() FROM public.media_intents WHERE id=$1`, mediaIntentID, assetID, stableDigest(assetID), "visual/"+assetID+".png"); err != nil {
		t.Fatal(err)
	}
	return assetID
}

func visualIdentityAcceptedReview() map[string]any {
	return map[string]any{
		"decision": "accepted", "identity_match": 0.96, "confidence": 0.94,
		"observations":     []any{"同一真人面孔贯穿正面、侧面与背面视图", "档案卡包含表情、服装、配饰、细节与色卡视觉区"},
		"missing_sections": []any{}, "summary": "候选图符合完整角色档案卡和身份连续性要求", "feedback": "",
	}
}

func TestVisualIdentityGenerateCandidateToolCommitsDurableIntentAndReplays(t *testing.T) {
	fixture := newVisualIdentityToolFixture(t)
	invalidTarget := fixture.request(visualIdentityGenerateCandidateCapabilityName, "visual-generate-invalid-target", map[string]any{"reason": "initial"})
	invalidTarget.TargetKind = "conversation"
	if receipt, targetErr := fixture.app.ExecuteTool(fixture.ctx, invalidTarget); targetErr == nil || receipt.Result.Status != "failed" || receipt.Result.ErrorCode != "visual_identity_target_invalid" {
		t.Fatalf("invalid target receipt=%#v err=%v", receipt, targetErr)
	}
	dependencyApp := &App{DB: fixture.repository}
	dependencyApp.ContextResolver = NewAppContextResolver(dependencyApp)
	dependencyApp.Capabilities = mustCapabilityRegistry(builtinCapabilities(nil)...)
	dependencyRequest := fixture.request(visualIdentityGenerateCandidateCapabilityName, "visual-generate-dependency", map[string]any{"reason": "initial"})
	if receipt, dependencyErr := dependencyApp.ExecuteTool(fixture.ctx, dependencyRequest); dependencyErr == nil || receipt.Result.Status != "failed" || receipt.Result.ErrorCode != "visual_identity_unavailable" {
		t.Fatalf("dependency receipt=%#v err=%v", receipt, dependencyErr)
	}
	request := fixture.request(visualIdentityGenerateCandidateCapabilityName, "visual-generate", map[string]any{"reason": "initial"})
	first, err := fixture.app.ExecuteTool(fixture.ctx, request)
	if err != nil || first.Result.Status != "accepted" {
		t.Fatalf("first receipt=%#v err=%v", first, err)
	}
	mediaIntentID := stringValue(mapValue(first.Result.Output)["media_intent_id"])
	if mediaIntentID == "" {
		t.Fatalf("media intent missing: %#v", first)
	}
	var mediaStatus, workflowStatus, prompt string
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT m.status,w.status,m.prompt FROM public.media_intents AS m JOIN public.platform_workflow_intents AS w ON w.workflow_id=m.workflow_id WHERE m.id=$1 AND w.intent_type='media.generation'`, mediaIntentID).Scan(&mediaStatus, &workflowStatus, &prompt); err != nil {
		t.Fatal(err)
	}
	if mediaStatus != "pending" || workflowStatus != "pending" || !strings.Contains(prompt, "character_design_sheet") || !strings.Contains(prompt, "front_full_body") {
		t.Fatalf("durable media intent status=%q workflow=%q prompt=%s", mediaStatus, workflowStatus, prompt)
	}
	replayed, err := fixture.app.ExecuteTool(fixture.ctx, request)
	if err != nil || !replayed.Replayed || stringValue(mapValue(replayed.Result.Output)["media_intent_id"]) != mediaIntentID {
		t.Fatalf("replay receipt=%#v err=%v", replayed, err)
	}
	conflict := request
	conflict.Arguments = jsonBytes(map[string]any{"reason": "regenerate"})
	if receipt, conflictErr := fixture.app.ExecuteTool(fixture.ctx, conflict); conflictErr == nil || receipt.Result.Status != "failed" || !errors.Is(conflictErr, ErrConflict) {
		t.Fatalf("operation conflict receipt=%#v err=%v", receipt, conflictErr)
	}
	var executions int
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.tool_executions WHERE fluctlight_id=$1 AND capability_name=$2`, fixture.fluctlightID, visualIdentityGenerateCandidateCapabilityName).Scan(&executions); err != nil || executions != 1 {
		t.Fatalf("tool execution rows=%d err=%v", executions, err)
	}
}

func TestProcessVisualIdentityReportsDurableProviderConfigurationWait(t *testing.T) {
	fixture := newVisualIdentityToolFixture(t)
	result, err := fixture.app.ProcessVisualIdentity(fixture.ctx, fixture.sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if stringValue(result["status"]) != "waiting" || stringValue(result["stage"]) != "provider_config_pending" || result["accepted"] != true || stringValue(result["error_code"]) != "visual_identity_agent_role_missing" {
		t.Fatalf("provider configuration wait=%#v", result)
	}
	var mediaIntents int
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.media_intents WHERE owner_fluctlight_id=$1`, fixture.fluctlightID).Scan(&mediaIntents); err != nil || mediaIntents != 0 {
		t.Fatalf("configuration wait created media intents=%d err=%v", mediaIntents, err)
	}
}

func TestVisualIdentityCommitReviewToolPreservesRejectedAssetAndCreatesNextAttempt(t *testing.T) {
	fixture := newVisualIdentityToolFixture(t)
	generated := fixture.generateCandidate(t, "visual-review-generate")
	mediaIntentID := stringValue(mapValue(generated.Result.Output)["media_intent_id"])
	assetID := fixture.makeCurrentCandidateReady(t, mediaIntentID)
	inconsistent := visualIdentityAcceptedReview()
	inconsistent["missing_sections"] = []any{"缺少侧面全身视图"}
	if receipt, reviewErr := fixture.app.ExecuteTool(fixture.ctx, fixture.request(visualIdentityCommitReviewCapabilityName, "visual-review-inconsistent", inconsistent)); reviewErr == nil || receipt.Result.Status != "failed" || receipt.Result.ErrorCode != "visual_identity_review_inconsistent" {
		t.Fatalf("inconsistent review receipt=%#v err=%v", receipt, reviewErr)
	}
	review := map[string]any{
		"decision": "regenerate", "identity_match": 0.31, "confidence": 0.91,
		"observations": []any{"侧面人物与正面人物面孔不一致"}, "missing_sections": []any{"一致的侧面全身视图"},
		"summary": "候选图不是同一角色的完整三视图", "feedback": "下一轮必须保持同一张脸",
	}
	request := fixture.request(visualIdentityCommitReviewCapabilityName, "visual-review-regenerate", review)
	receipt, err := fixture.app.ExecuteTool(fixture.ctx, request)
	if err != nil || receipt.Result.Status != "completed" || stringValue(mapValue(receipt.Result.Output)["status"]) != "regenerating" {
		t.Fatalf("review receipt=%#v err=%v", receipt, err)
	}
	var currentAttempt int
	var priorDecision, priorAsset, priorStatus string
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT s.current_attempt,a.decision,a.candidate_asset_id,a.status FROM public.fluctlight_visual_identity_sessions AS s JOIN public.fluctlight_visual_identity_attempts AS a ON a.session_id=s.id AND a.attempt_number=1 WHERE s.id=$1`, fixture.sessionID).Scan(&currentAttempt, &priorDecision, &priorAsset, &priorStatus); err != nil {
		t.Fatal(err)
	}
	if currentAttempt != 2 || priorDecision != "regenerate" || priorAsset != assetID || priorStatus != "rejected_not_self" {
		t.Fatalf("review settlement attempt=%d decision=%q asset=%q status=%q", currentAttempt, priorDecision, priorAsset, priorStatus)
	}
	var nextSnapshot []byte
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT input_snapshot FROM public.fluctlight_visual_identity_attempts WHERE session_id=$1 AND attempt_number=2`, fixture.sessionID).Scan(&nextSnapshot); err != nil {
		t.Fatal(err)
	}
	next := decodeObject(nextSnapshot)
	if stringValue(next["previous_asset_id"]) != assetID || len(mapValue(next["identity"])) == 0 {
		t.Fatalf("next attempt lost identity/history: %#v", next)
	}
	var historicalAssets int
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.media_assets WHERE id=$1 AND status='ready'`, assetID).Scan(&historicalAssets); err != nil || historicalAssets != 1 {
		t.Fatalf("rejected historical asset count=%d err=%v", historicalAssets, err)
	}
	replayed, err := fixture.app.ExecuteTool(fixture.ctx, request)
	if err != nil || !replayed.Replayed || stringValue(mapValue(replayed.Result.Output)["decision"]) != "regenerate" {
		t.Fatalf("review replay=%#v err=%v", replayed, err)
	}
	conflict := request
	conflictingReview := cloneMap(review)
	conflictingReview["summary"] = "same operation with different review"
	conflict.Arguments = jsonBytes(conflictingReview)
	if conflictReceipt, conflictErr := fixture.app.ExecuteTool(fixture.ctx, conflict); conflictErr == nil || conflictReceipt.Result.Status != "failed" || !errors.Is(conflictErr, ErrConflict) {
		t.Fatalf("review operation conflict receipt=%#v err=%v", conflictReceipt, conflictErr)
	}
}

func TestVisualIdentityFinalizeToolCommitsCanonicalCharacterSheetAndCompletion(t *testing.T) {
	fixture := newVisualIdentityToolFixture(t)
	generated := fixture.generateCandidate(t, "visual-finalize-generate")
	candidateIntentID := stringValue(mapValue(generated.Result.Output)["media_intent_id"])
	candidateAssetID := fixture.makeCurrentCandidateReady(t, candidateIntentID)
	reviewReceipt, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(visualIdentityCommitReviewCapabilityName, "visual-finalize-review", visualIdentityAcceptedReview()))
	if err != nil || reviewReceipt.Result.Status != "accepted" {
		t.Fatalf("accepted review receipt=%#v err=%v", reviewReceipt, err)
	}
	characterIntentID := stringValue(mapValue(reviewReceipt.Result.Output)["character_sheet_media_intent_id"])
	if characterIntentID == "" {
		t.Fatalf("character sheet intent missing: %#v", reviewReceipt)
	}
	prematureRequest := fixture.request(visualIdentityFinalizeCapabilityName, "visual-finalize-premature", map[string]any{})
	premature, err := fixture.app.ExecuteTool(fixture.ctx, prematureRequest)
	if err != nil || premature.Result.Status != "rejected" || premature.Result.ErrorCode != "visual_identity_character_sheet_not_ready" {
		t.Fatalf("premature finalize receipt=%#v err=%v", premature, err)
	}
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `UPDATE public.media_intents SET status='completed',revision=revision+1 WHERE id=$1`, characterIntentID); err != nil {
		t.Fatal(err)
	}
	finalizeRequest := fixture.request(visualIdentityFinalizeCapabilityName, "visual-finalize", map[string]any{})
	if missingReceipt, missingErr := fixture.app.ExecuteTool(fixture.ctx, finalizeRequest); missingErr == nil || missingReceipt.Result.Status != "failed" || missingReceipt.Result.ErrorCode != "visual_identity_character_sheet_asset_load_failed" {
		t.Fatalf("missing character asset receipt=%#v err=%v", missingReceipt, missingErr)
	}
	characterAssetID := "asset_" + characterIntentID
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `INSERT INTO public.media_assets(id,owner_fluctlight_id,version,kind,mime_type,byte_size,sha256,bucket,object_key,provider_request_id,workflow_id,status,ready_at) SELECT $2,owner_fluctlight_id,'v1','image','image/png',68,$3,'visual-test-bucket',$4,provider_request_id,workflow_id,'ready',now() FROM public.media_intents WHERE id=$1`, characterIntentID, characterAssetID, stableDigest(characterAssetID), "visual/"+characterAssetID+".png"); err != nil {
		t.Fatal(err)
	}
	finalized, err := fixture.app.ExecuteTool(fixture.ctx, finalizeRequest)
	if err != nil || finalized.Result.Status != "completed" || stringValue(mapValue(finalized.Result.Output)["character_sheet_asset_id"]) != characterAssetID {
		t.Fatalf("finalize receipt=%#v err=%v", finalized, err)
	}
	var sessionStatus, profileStatus, canonicalAsset, characterAsset string
	var revision int
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT s.status,v.status,v.current_revision,v.canonical_asset_id,v.character_sheet_asset_id FROM public.fluctlight_visual_identity_sessions AS s JOIN public.fluctlight_visual_identities AS v ON v.id=s.visual_identity_id WHERE s.id=$1`, fixture.sessionID).Scan(&sessionStatus, &profileStatus, &revision, &canonicalAsset, &characterAsset); err != nil {
		t.Fatal(err)
	}
	if sessionStatus != "completed" || profileStatus != "active" || revision != 1 || canonicalAsset != candidateAssetID || characterAsset != characterAssetID {
		t.Fatalf("final state session=%q profile=%q revision=%d canonical=%q character=%q", sessionStatus, profileStatus, revision, canonicalAsset, characterAsset)
	}
	var revisionAsset, revisionCharacter string
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT canonical_asset_id,character_sheet_asset_id FROM public.fluctlight_visual_identity_revisions WHERE visual_identity_id=$1 AND revision=1`, visualIdentityProfileID(fixture.fluctlightID)).Scan(&revisionAsset, &revisionCharacter); err != nil {
		t.Fatal(err)
	}
	if revisionAsset != candidateAssetID || revisionCharacter != characterAssetID {
		t.Fatalf("canonical revision assets candidate=%q character=%q", revisionAsset, revisionCharacter)
	}
	var completedEvents int
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.fluctlight_visual_identity_timeline WHERE session_id=$1 AND stage IN ('accepted','character_sheet_requested','character_sheet_ready','completed')`, fixture.sessionID).Scan(&completedEvents); err != nil || completedEvents != 4 {
		t.Fatalf("completion timeline rows=%d err=%v", completedEvents, err)
	}
	replayed, err := fixture.app.ExecuteTool(fixture.ctx, finalizeRequest)
	if err != nil || !replayed.Replayed || replayed.Result.Status != "completed" {
		t.Fatalf("finalize replay=%#v err=%v", replayed, err)
	}
}

func TestVisualIdentityAgentUsesFormalRunnerAndToolReceiptForCandidateGeneration(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	suffix := stableDigest(t.Name() + fmt.Sprintf("-%d", time.Now().UnixNano()))[:20]
	ownerID, fluctlightID := "visual_agent_owner_"+suffix, "visual_agent_fluctlight_"+suffix
	seedLifeContextFluctlight(t, ctx, repository, ownerID, fluctlightID)
	seedUnknownEffectiveLifeForTest(t, ctx, repository, fluctlightID)
	persona := map[string]any{"identity": map[string]any{"name": "澄光", "gender": "male"}, "life_profile": map[string]any{"appearance": map[string]any{"hair": "black hair"}}}
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.fluctlights SET core_persona=$2 WHERE id=$1`, fluctlightID, jsonBytes(persona)); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	requests := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		defer request.Body.Close()
		var payload map[string]any
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Errorf("decode provider request: %v", err)
			return
		}
		body := jsonString(payload)
		mu.Lock()
		requests = append(requests, body)
		sequence := len(requests)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if sequence == 1 {
			_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","content":"","tool_calls":[{"id":"visual-generate-call","type":"function","function":{"name":"visual_identity.generate_candidate","arguments":"{\"reason\":\"initial\"}"}}]}}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"{\"status\":\"waiting\",\"stage\":\"image_pending\",\"summary\":\"候选图片任务已受理\"}"}}]}`))
	}))
	defer server.Close()
	endpointID := "visual-agent-endpoint-" + suffix
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status,checked_at) VALUES($1,'openai_compatible',$2,'visual-agent-secret','ready',now())`, endpointID, server.URL); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,required_capabilities,token_budget,timeout_seconds,retry_policy) VALUES('generic_llm',$1,'visual-agent-model','structured_output,tool_calling,multimodal_input',4096,30,'{}')`, endpointID); err != nil {
		t.Fatal(err)
	}
	app := newTestApp(t, repository, server.Client().Transport)
	sessionID, err := app.EnsureVisualIdentityInitializationWithPersona(ctx, fluctlightID, "initialization", "visual-agent-source-"+suffix, persona)
	if err != nil {
		t.Fatal(err)
	}
	result, err := app.RunVisualIdentityAgent(ctx, VisualIdentityAgentInput{SessionID: sessionID})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "waiting" || !result.Accepted || result.MediaIntentID == "" {
		t.Fatalf("visual Agent result=%#v", result)
	}
	expectedRunID := visualIdentityAgentCheckpointOperationID(visualIdentityAgentState{SessionID: sessionID, Attempt: 1, ActionRequired: visualIdentityGenerateCandidateCapabilityName})
	var runStatus string
	var linkedExecutions int
	if err := repository.Pool().QueryRow(ctx, `SELECT status FROM public.agent_runs WHERE fluctlight_id=$1 AND agent_id=$2 AND run_id=$3`, fluctlightID, FormalAgentVisualIdentity, expectedRunID).Scan(&runStatus); err != nil || runStatus != "completed" {
		t.Fatalf("durable visual Agent run status=%q err=%v", runStatus, err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.tool_executions WHERE fluctlight_id=$1 AND agent_id=$2 AND run_id=$3 AND capability_name=$4`, fluctlightID, FormalAgentVisualIdentity, expectedRunID, visualIdentityGenerateCandidateCapabilityName).Scan(&linkedExecutions); err != nil || linkedExecutions != 1 {
		t.Fatalf("durable visual Tool linkage count=%d err=%v", linkedExecutions, err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(requests) != 2 || !strings.Contains(requests[1], "visual_identity.generate_candidate") || !strings.Contains(requests[1], result.MediaIntentID) {
		t.Fatalf("formal runner requests=%d second=%s", len(requests), func() string {
			if len(requests) > 1 {
				return requests[1]
			}
			return ""
		}())
	}
}

func TestVisualIdentityAgentPreservesAcceptedGenerationWhenFinalModelFails(t *testing.T) {
	fixture := newVisualIdentityToolFixture(t)
	var mu sync.Mutex
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		requestCount++
		sequence := requestCount
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if sequence == 1 {
			_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","content":"","tool_calls":[{"id":"visual-commit-before-failure","type":"function","function":{"name":"visual_identity.generate_candidate","arguments":"{\"reason\":\"initial\"}"}}]}}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"not-a-valid-final-contract"}}]}`))
	}))
	defer server.Close()
	endpointID := "visual-failure-endpoint-" + stableDigest(fixture.sessionID)[:20]
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status,checked_at) VALUES($1,'openai_compatible',$2,'visual-failure-secret','ready',now())`, endpointID, server.URL); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,required_capabilities,token_budget,timeout_seconds,retry_policy) VALUES('generic_llm',$1,'visual-failure-model','structured_output,tool_calling,multimodal_input',4096,30,'{}')`, endpointID); err != nil {
		t.Fatal(err)
	}
	fixture.app.Provider.HTTP = server.Client()
	if _, err := fixture.app.RunVisualIdentityAgent(fixture.ctx, VisualIdentityAgentInput{SessionID: fixture.sessionID}); err == nil {
		t.Fatal("invalid final model output unexpectedly succeeded")
	}
	expectedRunID := visualIdentityAgentCheckpointOperationID(visualIdentityAgentState{SessionID: fixture.sessionID, Attempt: 1, ActionRequired: visualIdentityGenerateCandidateCapabilityName})
	var failedRunStatus string
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT status FROM public.agent_runs WHERE fluctlight_id=$1 AND agent_id=$2 AND run_id=$3`, fixture.fluctlightID, FormalAgentVisualIdentity, expectedRunID).Scan(&failedRunStatus); err != nil || failedRunStatus != "failed" {
		t.Fatalf("failed durable visual Agent run status=%q err=%v", failedRunStatus, err)
	}
	var mediaIntentID, mediaStatus string
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT id,status FROM public.media_intents WHERE owner_fluctlight_id=$1`, fixture.fluctlightID).Scan(&mediaIntentID, &mediaStatus); err != nil {
		t.Fatal(err)
	}
	if mediaIntentID == "" || mediaStatus != "pending" {
		t.Fatalf("committed media intent was lost after final model failure: id=%q status=%q", mediaIntentID, mediaStatus)
	}
	resumed, err := fixture.app.RunVisualIdentityAgent(fixture.ctx, VisualIdentityAgentInput{SessionID: fixture.sessionID})
	if err != nil || resumed.Status != "waiting" || resumed.MediaIntentID != mediaIntentID {
		t.Fatalf("durable resume result=%#v err=%v", resumed, err)
	}
	mu.Lock()
	defer mu.Unlock()
	if requestCount != 2 {
		t.Fatalf("durable resume repeated the model/tool run: requests=%d", requestCount)
	}
}

func TestVisualIdentityAgentReviewsRealObjectImageAndSavesCanonical(t *testing.T) {
	endpoint := strings.TrimSpace(os.Getenv("FLUCTLIGHT_VISUAL_TEST_S3_ENDPOINT"))
	accessKey := strings.TrimSpace(os.Getenv("FLUCTLIGHT_VISUAL_TEST_S3_ACCESS_KEY"))
	secretKey := strings.TrimSpace(os.Getenv("FLUCTLIGHT_VISUAL_TEST_S3_SECRET_KEY"))
	bucket := strings.TrimSpace(os.Getenv("FLUCTLIGHT_VISUAL_TEST_S3_BUCKET"))
	if endpoint == "" || accessKey == "" || secretKey == "" || bucket == "" {
		t.Skip("isolated Visual Identity object-storage test configuration is not set")
	}
	storage, err := minio.New(strings.TrimPrefix(strings.TrimPrefix(endpoint, "http://"), "https://"), &minio.Options{
		Creds: credentials.NewStaticV4(accessKey, secretKey, ""), Secure: strings.HasPrefix(endpoint, "https://"),
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, repository := isolatedCoreTestRepository(t)
	if err := storage.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
		exists, existsErr := storage.BucketExists(ctx, bucket)
		if existsErr != nil || !exists {
			t.Fatalf("prepare isolated visual bucket: %v (exists error %v)", err, existsErr)
		}
	}
	pngBytes, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatal(err)
	}
	suffix := stableDigest(t.Name() + fmt.Sprintf("-%d", time.Now().UnixNano()))[:20]
	objectKey := "visual-agent/" + suffix + "/candidate.png"
	if _, err := storage.PutObject(ctx, bucket, objectKey, strings.NewReader(string(pngBytes)), int64(len(pngBytes)), minio.PutObjectOptions{ContentType: "image/png"}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = storage.RemoveObject(context.Background(), bucket, objectKey, minio.RemoveObjectOptions{}) })

	ownerID, fluctlightID := "visual_review_owner_"+suffix, "visual_review_fluctlight_"+suffix
	seedLifeContextFluctlight(t, ctx, repository, ownerID, fluctlightID)
	seedUnknownEffectiveLifeForTest(t, ctx, repository, fluctlightID)
	persona := map[string]any{"identity": map[string]any{"name": "澄光", "gender": "male"}, "life_profile": map[string]any{"appearance": map[string]any{"hair": "black hair"}}}
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.fluctlights SET core_persona=$2 WHERE id=$1`, fluctlightID, jsonBytes(persona)); err != nil {
		t.Fatal(err)
	}

	review := visualIdentityAcceptedReview()
	var mu sync.Mutex
	requests := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		defer request.Body.Close()
		var payload map[string]any
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Errorf("decode provider request: %v", err)
			return
		}
		body := jsonString(payload)
		mu.Lock()
		requests = append(requests, body)
		sequence := len(requests)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if sequence == 1 {
			_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{
				"finish_reason": "tool_calls", "message": map[string]any{"role": "assistant", "content": "", "tool_calls": []any{map[string]any{
					"id": "visual-review-call", "type": "function", "function": map[string]any{"name": visualIdentityCommitReviewCapabilityName, "arguments": jsonString(review)},
				}}},
			}}})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{
			"finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": jsonString(map[string]any{"status": "waiting", "stage": "character_sheet_pending", "summary": "真实候选图片已接受并保存 canonical"})},
		}}})
	}))
	defer server.Close()
	endpointID := "visual-review-endpoint-" + suffix
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status,checked_at) VALUES($1,'openai_compatible',$2,'visual-review-secret','ready',now())`, endpointID, server.URL); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,required_capabilities,token_budget,timeout_seconds,retry_policy) VALUES('generic_llm',$1,'visual-review-model','structured_output,tool_calling,multimodal_input',4096,30,'{}')`, endpointID); err != nil {
		t.Fatal(err)
	}
	app := newTestApp(t, repository, server.Client().Transport)
	app.Storage = storage
	sessionID, err := app.EnsureVisualIdentityInitializationWithPersona(ctx, fluctlightID, "initialization", "visual-review-source-"+suffix, persona)
	if err != nil {
		t.Fatal(err)
	}
	generated, err := app.ExecuteTool(ctx, ToolExecutionRequest{
		CapabilityName: visualIdentityGenerateCandidateCapabilityName, OperationID: "visual-review-generate-" + suffix,
		AuthorizationActorID: ownerID, FluctlightID: fluctlightID, TargetKind: "visual_identity_session", TargetRef: sessionID,
		EvidenceID: "visual-review-source-" + suffix, Surface: CapabilitySurfaceVisualIdentity,
		Arguments: jsonBytes(map[string]any{"reason": "initial"}),
	})
	if err != nil || generated.Result.Status != "accepted" {
		t.Fatalf("generate receipt=%#v err=%v", generated, err)
	}
	mediaIntentID := stringValue(mapValue(generated.Result.Output)["media_intent_id"])
	assetID := "asset_" + mediaIntentID
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.media_intents SET status='completed',revision=revision+1 WHERE id=$1`, mediaIntentID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.media_assets(id,owner_fluctlight_id,version,kind,mime_type,byte_size,sha256,bucket,object_key,provider_request_id,workflow_id,status,ready_at) SELECT $2,owner_fluctlight_id,'v1','image','image/png',$3,$4,$5,$6,provider_request_id,workflow_id,'ready',now() FROM public.media_intents WHERE id=$1`, mediaIntentID, assetID, len(pngBytes), stableDigest(string(pngBytes)), bucket, objectKey); err != nil {
		t.Fatal(err)
	}

	result, err := app.RunVisualIdentityAgent(ctx, VisualIdentityAgentInput{SessionID: sessionID})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "waiting" || result.CanonicalAssetID != assetID || result.CharacterMediaIntentID == "" {
		t.Fatalf("review Agent result=%#v", result)
	}
	encodedPNG := base64.StdEncoding.EncodeToString(pngBytes)
	mu.Lock()
	defer mu.Unlock()
	if len(requests) != 2 || !strings.Contains(requests[0], "data:image/png;base64,"+encodedPNG) {
		t.Fatalf("first formal model request did not carry the actual object bytes; requests=%d", len(requests))
	}
	if !strings.Contains(requests[1], result.CharacterMediaIntentID) || !strings.Contains(requests[1], visualIdentityCommitReviewCapabilityName) {
		t.Fatalf("second formal model request did not carry the committed review receipt")
	}
	var canonicalID string
	var revision int
	if err := repository.Pool().QueryRow(ctx, `SELECT canonical_asset_id,current_revision FROM public.fluctlight_visual_identities WHERE fluctlight_id=$1`, fluctlightID).Scan(&canonicalID, &revision); err != nil {
		t.Fatal(err)
	}
	if canonicalID != assetID || revision != 1 {
		t.Fatalf("canonical save canonical=%q revision=%d", canonicalID, revision)
	}
}

func TestVisualIdentityAgentMessagesCarryRealImageContentBlock(t *testing.T) {
	image := map[string]any{"type": "image_url", "image_url": map[string]any{"url": "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAAB", "detail": "high"}}
	messages := visualIdentityAgentMessages(visualIdentityAgentState{
		SessionID: "session-1", FluctlightID: "fl-1", SessionStatus: "running", Attempt: 1, MaxAttempts: 3,
		ActionRequired: visualIdentityCommitReviewCapabilityName, CandidateAssetID: "asset-1", CandidateAssetReady: true,
		InputSnapshot: map[string]any{"identity": map[string]any{"name": "澄光"}}, RendererConstraints: map[string]any{"adapter_version": visualIdentityAdapterVersion},
	}, image)
	if len(messages) != 2 {
		t.Fatalf("messages=%#v", messages)
	}
	content, ok := messages[1]["content"].([]any)
	if !ok || len(content) != 2 || mapValue(content[1])["type"] != "image_url" || !strings.HasPrefix(stringValue(mapValue(mapValue(content[1])["image_url"])["url"]), "data:image/png;base64,") {
		t.Fatalf("real multimodal image block missing: %#v", messages[1]["content"])
	}
	registry := mustCapabilityRegistry(builtinCapabilities(&App{})...)
	if got := capabilityDefinitionNames(registry.Catalog(CapabilitySurfaceVisualIdentity)); !phase8EqualStrings(got, []string{visualIdentityGenerateCandidateCapabilityName, visualIdentityCommitReviewCapabilityName, visualIdentityFinalizeCapabilityName}) {
		t.Fatalf("visual Agent catalog=%v", got)
	}
	for _, surface := range []CapabilitySurface{CapabilitySurfaceConversation, CapabilitySurfaceWakeUp, CapabilitySurfaceAutonomy, CapabilitySurfaceNativeCognition, CapabilitySurfaceReflection} {
		for _, definition := range registry.Catalog(surface) {
			if strings.HasPrefix(definition.Name, "visual_identity.") && definition.Name != "visual_identity.initialize" {
				t.Fatalf("private visual tool %q leaked into %s", definition.Name, surface)
			}
		}
	}
}
