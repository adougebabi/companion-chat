package core

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

type liveResponseCapture struct {
	inner  http.RoundTripper
	mu     sync.Mutex
	shapes []map[string]any
}

func (capture *liveResponseCapture) RoundTrip(request *http.Request) (*http.Response, error) {
	response, err := capture.inner.RoundTrip(request)
	if err != nil || response == nil || response.Body == nil {
		return response, err
	}
	body, readErr := io.ReadAll(response.Body)
	_ = response.Body.Close()
	response.Body = io.NopCloser(bytes.NewReader(body))
	if readErr == nil {
		var envelope map[string]any
		if json.Unmarshal(body, &envelope) == nil {
			choices := arrayValue(envelope["choices"])
			if len(choices) > 0 {
				message := mapValue(mapValue(choices[0])["message"])
				shapes := make([]map[string]any, 0)
				for _, raw := range arrayValue(message["tool_calls"]) {
					call := mapValue(raw)
					function := mapValue(call["function"])
					name := stringValue(call["name"])
					if name == "" {
						name = stringValue(call["capability_name"])
					}
					if name == "" {
						name = stringValue(function["name"])
					}
					args := call["arguments"]
					if args == nil {
						args = function["arguments"]
					}
					argumentKeys := []string{}
					if object := mapValue(args); len(object) > 0 {
						for key := range object {
							argumentKeys = append(argumentKeys, key)
						}
					}
					id := stringValue(call["id"])
					if id == "" {
						id = stringValue(call["call_id"])
					}
					shapes = append(shapes, map[string]any{"name": name, "id_present": strings.TrimSpace(id) != "", "argument_keys": argumentKeys})
				}
				capture.mu.Lock()
				capture.shapes = append(capture.shapes, shapes...)
				capture.mu.Unlock()
			}
		}
	}
	return response, readErr
}

func (capture *liveResponseCapture) snapshot() []map[string]any {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	result := make([]map[string]any, len(capture.shapes))
	for index, shape := range capture.shapes {
		result[index] = cloneMap(shape)
	}
	return result
}

// TestLiveHandleTurnUsesRealProviderForPostCognitionPersonalityAssessment is the production
// chain smoke that the scripted Provider tests cannot provide. It uses a
// disposable PostgreSQL database, resolves the model through ProviderClient,
// runs HandleTurn, and verifies the durable active profile plus the assistant
// message. The test intentionally has no takeover_rules so the result isolates
// the post-cognition persistent-switch path instead of asking the live model to
// also arbitrate.
func TestLiveHandleTurnUsesRealProviderForPostCognitionPersonalityAssessment(t *testing.T) {
	if strings.TrimSpace(os.Getenv("FLUCTLIGHT_LIVE_PROVIDER_TEST")) != "1" {
		t.Skip("set FLUCTLIGHT_LIVE_PROVIDER_TEST=1 to call a real Provider")
	}
	if strings.TrimSpace(os.Getenv("GO_CORE_TEST_DATABASE_URL")) == "" {
		t.Skip("GO_CORE_TEST_DATABASE_URL is required for the live HandleTurn chain")
	}
	baseURL, model := liveProviderConfig(t)
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "live-turn-owner", "live-turn-fluctlight", "live-turn-conversation"
	takeoverChainSeedWithTakeoverRules(t, ctx, repository, ownerID, fluctlightID, conversationID, nil)
	endpointID := "takeover-chain-endpoint-" + fluctlightID
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.provider_endpoints SET base_url=$2 WHERE id=$1`, endpointID, strings.TrimRight(baseURL, "/")); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.model_roles SET model_id=$2,timeout_seconds=600,token_budget=4096 WHERE role='cognitive_assessment' AND provider_endpoint_id=$1`, endpointID, model); err != nil {
		t.Fatal(err)
	}

	responseCapture := &liveResponseCapture{inner: http.DefaultTransport}
	capture := captureProviderWirePayload(responseCapture)
	app := newTestApp(t, repository, capture)
	turnCtx, cancel := context.WithTimeout(context.Background(), liveProviderRequestTimeout()+2*time.Minute)
	defer cancel()
	result, err := app.HandleTurn(turnCtx, ownerID, conversationID, takeoverChainTurnPayload(
		fluctlightID,
		"安全确认已收到。请按 safety 规则把持久主导人格从 spark 切换为 twilight，并用暮光的风格回复我。",
		"live-turn-switch-1",
		"live-turn-switch-1",
	))
	if err != nil {
		t.Fatalf("live HandleTurn failed: %v; provider_tool_shapes=%#v", err, responseCapture.snapshot())
	}
	if strings.TrimSpace(stringValue(result.Assistant["text"])) == "" {
		t.Fatalf("live HandleTurn returned no assistant text: %#v", result.Assistant)
	}
	if got := capture.count(); got < 2 {
		t.Fatalf("persistent switch path must use a Main response and a post-cognition assessment, got %d", got)
	}
	var active string
	if err := repository.Pool().QueryRow(turnCtx, `SELECT active_profile_id FROM public.fluctlight_personality_runtime WHERE fluctlight_id=$1`, fluctlightID).Scan(&active); err != nil {
		t.Fatal(err)
	}
	if active != "twilight" {
		t.Fatalf("live persistent switch did not update active profile: %q; provider_tool_shapes=%#v", active, responseCapture.snapshot())
	}
	var messageCount int
	if err := repository.Pool().QueryRow(turnCtx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant'`, conversationID).Scan(&messageCount); err != nil {
		t.Fatal(err)
	}
	if messageCount != 1 {
		t.Fatalf("live HandleTurn assistant message count = %d, want 1", messageCount)
	}
}

func requireLiveDatabaseProvider(t *testing.T) (string, string) {
	t.Helper()
	if strings.TrimSpace(os.Getenv("FLUCTLIGHT_LIVE_PROVIDER_TEST")) != "1" {
		t.Skip("set FLUCTLIGHT_LIVE_PROVIDER_TEST=1 to call a real Provider")
	}
	if strings.TrimSpace(os.Getenv("GO_CORE_TEST_DATABASE_URL")) == "" {
		t.Skip("GO_CORE_TEST_DATABASE_URL is required for the live durable chain")
	}
	return liveProviderConfig(t)
}

// TestLiveHandleTurnRealToolCallsReachDurableMediaAndReply proves that native
// Provider tool calls do not stop at the model boundary. It requires the real
// Provider and disposable PostgreSQL; no fake router or scripted completion is
// installed. The media workflow itself may be completed by the Worker/ComfyUI
// acceptance stack, while this test owns the Core freeze and durable intent
// boundary.
func TestLiveHandleTurnRealToolCallsReachDurableMediaAndReply(t *testing.T) {
	baseURL, model := requireLiveDatabaseProvider(t)
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "live-tool-owner", "live-tool-fluctlight", "live-tool-conversation"
	seedTurnConversation(t, ctx, repository, ownerID, fluctlightID, conversationID)
	endpointID := "live-tool-endpoint-" + fluctlightID
	seedCognitiveProviderRole(t, ctx, repository, endpointID)
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.provider_endpoints SET base_url=$2 WHERE id=$1`, endpointID, strings.TrimRight(baseURL, "/")); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.model_roles SET model_id=$2,timeout_seconds=600,token_budget=4096 WHERE role='cognitive_assessment' AND provider_endpoint_id=$1`, endpointID, model); err != nil {
		t.Fatal(err)
	}
	capture := captureProviderWirePayload(&liveResponseCapture{inner: http.DefaultTransport})
	app := newTestApp(t, repository, capture)
	turnCtx, cancel := context.WithTimeout(context.Background(), liveProviderRequestTimeout()+2*time.Minute)
	defer cancel()
	result, err := app.HandleTurn(turnCtx, ownerID, conversationID, map[string]any{
		"fluctlight_id": fluctlightID, "text": "请同时调用 media.image.generate 生成一张雨后昏暗卧室里 22 岁中国女性的真实图片，并用 conversation.reply 给我一句正常回复。不要只解释，必须真实调用两个能力。",
		"idempotency_key": "live-tool-image-reply", "turn_id": "live-tool-image-reply", "attachment_refs": []any{},
	})
	if err != nil {
		t.Fatalf("real HandleTurn tool flow failed: %v", err)
	}
	if strings.TrimSpace(stringValue(result.Assistant["text"])) == "" {
		t.Fatalf("real tool flow persisted no assistant text: %#v", result.Assistant)
	}
	var mediaCount, actionCount, assistantCount int
	if err := repository.Pool().QueryRow(turnCtx, `SELECT count(*) FROM public.media_intents WHERE owner_fluctlight_id=$1`, fluctlightID).Scan(&mediaCount); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(turnCtx, `SELECT count(*) FROM public.autonomy_actions WHERE fluctlight_id=$1`, fluctlightID).Scan(&actionCount); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(turnCtx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant'`, conversationID).Scan(&assistantCount); err != nil {
		t.Fatal(err)
	}
	if mediaCount < 1 || actionCount < 1 || assistantCount < 1 {
		t.Fatalf("real tool calls did not reach durable Core boundaries: media_intents=%d autonomy_actions=%d assistant_messages=%d", mediaCount, actionCount, assistantCount)
	}
}

// TestLiveWakeUpRealToolCallsReachDurableActionAndReflection proves the
// background Wake-up path, not just an interactive turn. It requires the real
// Provider and disposable PostgreSQL and asserts the Wake-up fact, action,
// media intent and reflection intent are all durable after one cycle.
func TestLiveWakeUpRealToolCallsReachDurableActionAndReflection(t *testing.T) {
	baseURL, model := requireLiveDatabaseProvider(t)
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID := "live-wake-owner", "live-wake-fluctlight"
	seedLifeContextFluctlight(t, ctx, repository, ownerID, fluctlightID)
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.fluctlights SET behavioral_policy=$2 WHERE id=$1`, fluctlightID, jsonString(map[string]any{"required_action_type": "proactive_message", "autonomy": "每次 Wake-up 都必须主动联系 Owner，同时生成图片并发送，不能选择 no_op。"})); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.autonomy_policies(fluctlight_id,mode,allowed_actions,budget_remaining,quiet_hours,concurrency_limit,revision) VALUES($1,'active',$2,'10','{}',4,0)`, fluctlightID, jsonBytes([]string{"proactive_message", "moment", "capability"})); err != nil {
		t.Fatal(err)
	}
	endpointID := "live-wake-endpoint-" + fluctlightID
	seedCognitiveProviderRole(t, ctx, repository, endpointID)
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.provider_endpoints SET base_url=$2 WHERE id=$1`, endpointID, strings.TrimRight(baseURL, "/")); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.model_roles SET model_id=$2,timeout_seconds=600,token_budget=4096 WHERE role='cognitive_assessment' AND provider_endpoint_id=$1`, endpointID, model); err != nil {
		t.Fatal(err)
	}
	app := newTestApp(t, repository, captureProviderWirePayload(&liveResponseCapture{inner: http.DefaultTransport}))
	wakeCtx, cancel := context.WithTimeout(context.Background(), liveProviderRequestTimeout()+2*time.Minute)
	defer cancel()
	result, err := app.ProcessWakeUp(wakeCtx, fluctlightID, 1)
	if err != nil {
		t.Fatalf("real Wake-up tool flow failed: %v", err)
	}
	if stringValue(result["status"]) == "blocked" || stringValue(result["reason"]) == "policy_action_not_allowed" || stringValue(result["reason"]) == "policy_budget_exhausted" {
		t.Fatalf("real Wake-up was blocked by policy: %#v", result)
	}
	var wakeCount, actionCount, mediaCount, reflectionCount int
	if err := repository.Pool().QueryRow(wakeCtx, `SELECT count(*) FROM public.cognition_wakeups WHERE fluctlight_id=$1 AND cycle=$2`, fluctlightID, 1).Scan(&wakeCount); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(wakeCtx, `SELECT count(*) FROM public.autonomy_actions WHERE fluctlight_id=$1`, fluctlightID).Scan(&actionCount); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(wakeCtx, `SELECT count(*) FROM public.media_intents WHERE owner_fluctlight_id=$1`, fluctlightID).Scan(&mediaCount); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(wakeCtx, `SELECT count(*) FROM public.platform_workflow_intents WHERE intent_type='reflection.run' AND payload->>'fluctlight_id'=$1`, fluctlightID).Scan(&reflectionCount); err != nil {
		t.Fatal(err)
	}
	if wakeCount != 1 || actionCount < 1 || mediaCount < 1 || reflectionCount < 1 {
		t.Fatalf("real Wake-up tool calls did not reach durable boundaries: wakeups=%d actions=%d media=%d reflections=%d result=%#v", wakeCount, actionCount, mediaCount, reflectionCount, result)
	}
}
