package core

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"
)

type sceneDecisionChain struct {
	InitialLifeRef      string
	InitialLifeRevision string
	ResultingLifeRef    string
	ResultingScene      string
	AssistantText       string
	InfluenceRef        string
	PreparedRevision    string
	OutcomeRefs         []string
	ProviderCalls       int
}

func TestSameObservationDifferentSceneChangesDecisionPlannerOutcomeAndNextProjection(t *testing.T) {
	chainA := runSceneDecisionChain(t, "书房", "继续在安静环境中阅读")
	chainB := runSceneDecisionChain(t, "客厅", "先整理共享空间再阅读")
	if chainA.InitialLifeRef == chainB.InitialLifeRef || chainA.InitialLifeRevision == chainB.InitialLifeRevision {
		t.Fatalf("different authoritative scenes shared initial identity: A=%#v B=%#v", chainA, chainB)
	}
	if chainA.AssistantText == chainB.AssistantText || chainA.ResultingScene == chainB.ResultingScene || chainA.ResultingLifeRef == chainB.ResultingLifeRef {
		t.Fatalf("different scenes did not change the structured chain: A=%#v B=%#v", chainA, chainB)
	}
	for _, chain := range []sceneDecisionChain{chainA, chainB} {
		if chain.ProviderCalls != 1 || chain.InfluenceRef != chain.InitialLifeRef || chain.PreparedRevision != chain.InitialLifeRevision || len(chain.OutcomeRefs) < 2 {
			t.Fatalf("scene chain lost cognition/planner/outcome causality: %#v", chain)
		}
	}
}

func runSceneDecisionChain(t *testing.T, initialScene, targetScene string) sceneDecisionChain {
	t.Helper()
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID, conversationID := "scene-chain-owner", "scene-chain-fluctlight", "scene-chain-conversation"
	seedLifeContextFluctlight(t, ctx, repository, ownerID, fluctlightID)
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.conversations(id,created_by_actor_id,title) VALUES($1,$2,'scene chain')`, conversationID, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.conversation_heads(conversation_id,next_sequence) VALUES($1,1)`, conversationID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.conversation_participants(conversation_id,actor_id,role,status) VALUES($1,$2,'owner','active'),($1,$3,'member','active')`, conversationID, ownerID, fluctlightID); err != nil {
		t.Fatal(err)
	}
	app := &App{DB: repository}
	pendingLife := currentLifeForTest(t, ctx, app, fluctlightID, time.Now().UTC())
	now := time.Now().UTC()
	if _, err := app.CreateLifeEvent(ctx, ownerID, fluctlightID, map[string]any{
		"kind": "owner_event", "start_at": now.Add(-time.Minute).Format(time.RFC3339), "end_at": now.Add(3 * time.Hour).Format(time.RFC3339),
		"scene": initialScene, "activity": "阅读", "evidence_refs": []any{"scene-chain-initial"},
		"expected_life_context_revision": pendingLife["context_revision"], "idempotency_key": "scene-chain-initial-" + initialScene,
	}); err != nil {
		t.Fatal(err)
	}
	initialProjection := lifeProjectionForTest(t, ctx, app, ownerID, fluctlightID, "scene-chain-observation")
	initialLifeRevision := initialProjection.LifeContextRevision
	endpointID := "scene-chain-endpoint"
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status,checked_at) VALUES($1,'openai_compatible','http://scene-chain.invalid','scene-chain-secret','ready',now())`, endpointID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,required_capabilities,token_budget,timeout_seconds,retry_policy) VALUES('cognitive_assessment',$1,'scene-chain-model','structured_output,tool_calling',4096,10,'{}')`, endpointID); err != nil {
		t.Fatal(err)
	}
	providerCalls := 0
	providerLifeRef := ""
	providerLifeRevision := ""
	assistantText := "我会根据" + initialScene + "的状态调整接下来的安排。"
	app.Provider = &ProviderClient{DB: repository, HTTP: &http.Client{Transport: projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		providerCalls++
		body, _ := io.ReadAll(request.Body)
		bodyText := string(body)
		if !strings.Contains(bodyText, initialScene) {
			return nil, fmt.Errorf("Provider did not receive initial Scene %q", initialScene)
		}
		providerLifeRef = regexp.MustCompile(`life_context:ctx_[a-f0-9]{32}`).FindString(bodyText)
		providerLifeRevision = regexp.MustCompile(`life_ctx_[a-f0-9]{32}`).FindString(bodyText)
		if providerLifeRef == "" || providerLifeRevision == "" {
			return nil, errors.New("Provider request omitted Life Context authority")
		}
		structured := map[string]any{
			"action_type": "reply", "response_intent": "根据当前 Scene 回应并更新事实", "visible_text": assistantText,
			"tool_calls": []any{},
			"influences": []any{map[string]any{"ref": providerLifeRef, "role": "constrains", "confidence": 0.95, "note": "当前 Scene 约束回复和场景切换"}},
			"appraisal": map[string]any{
				"relevance": 0.7, "goal_congruence": 0.6, "reward": 0.5, "loss": 0.2, "social_threat": 0.0,
				"controllability": 0.8, "responsibility": 0.6, "relationship_significance": 0.5, "expected_effect": 0.7,
				"evidence_refs": []any{}, "event_kind": "conversation", "direction": "positive", "drive_signals": []any{},
			},
		}
		response := map[string]any{"choices": []any{map[string]any{"message": map[string]any{
			"content": jsonString(structured),
			"tool_calls": []any{
				map[string]any{"id": "scene-chain-switch", "type": "function", "function": map[string]any{"name": "scene_event", "arguments": jsonString(map[string]any{"operation": "switch", "scene": targetScene, "activity": "阅读", "confidence": 0.9})}},
				map[string]any{"id": "scene-chain-reply", "type": "function", "function": map[string]any{"name": "conversation.reply", "arguments": jsonString(map[string]any{"text": assistantText})}},
			},
		}}}}
		return embeddingHTTPResponse(request, http.StatusOK, string(jsonBytes(response))), nil
	})}}
	app.ContextResolver = NewAppContextResolver(app)
	app.Capabilities = app.capabilityRegistry()
	runtime, err := NewCapabilityRuntime(app.Capabilities, app.ContextResolver)
	if err != nil {
		t.Fatal(err)
	}
	app.Runtime = runtime
	turn, err := app.HandleTurn(ctx, ownerID, conversationID, map[string]any{
		"fluctlight_id": fluctlightID, "text": "请根据现在的环境安排一下", "idempotency_key": "scene-chain-turn", "turn_id": "scene-chain-turn-1", "attachment_refs": []any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if stringValue(turn.Assistant["text"]) != assistantText {
		t.Fatalf("assistant=%#v", turn.Assistant)
	}
	if providerLifeRevision != initialLifeRevision {
		t.Fatalf("Provider Life revision=%q want %q", providerLifeRevision, initialLifeRevision)
	}
	var frozenPayload []byte
	if err := repository.Pool().QueryRow(ctx, `SELECT payload FROM public.cognition_frozen_actions WHERE inbox_id=(SELECT id FROM public.cognition_inbox WHERE fluctlight_id=$1 AND idempotency_key='scene-chain-turn')`, fluctlightID).Scan(&frozenPayload); err != nil {
		t.Fatal(err)
	}
	payload := decodeObject(frozenPayload)
	influences := arrayValue(mapValue(payload["decision"])["influences"])
	if len(influences) != 1 {
		t.Fatalf("frozen influences=%#v", influences)
	}
	invocations, err := capabilityInvocationsFromValue(payload["capability_invocations"])
	if err != nil || len(invocations) != 2 {
		t.Fatalf("frozen invocations=%#v err=%v", invocations, err)
	}
	var sceneInvocation CapabilityInvocation
	for _, invocation := range invocations {
		if invocation.CapabilityName == "scene_event" {
			sceneInvocation = invocation
		}
	}
	plan, err := scenePlanFromInvocation(sceneInvocation)
	if err != nil {
		t.Fatal(err)
	}
	nextProjection := lifeProjectionForTest(t, ctx, app, ownerID, fluctlightID, "scene-chain-next-observation")
	outcomeRefs := make([]string, 0, len(nextProjection.RecentOutcomes))
	for _, outcome := range nextProjection.RecentOutcomes {
		if ref := stringValue(outcome["ref"]); ref != "" {
			outcomeRefs = append(outcomeRefs, ref)
		}
	}
	if stringValue(nextProjection.LifeContext["scene"]) != targetScene || len(outcomeRefs) < 2 {
		t.Fatalf("next projection scene=%#v outcomes=%#v", nextProjection.LifeContext, nextProjection.RecentOutcomes)
	}
	return sceneDecisionChain{
		InitialLifeRef: providerLifeRef, InitialLifeRevision: initialLifeRevision,
		ResultingLifeRef: stringValue(nextProjection.LifeContext["ref"]), ResultingScene: stringValue(nextProjection.LifeContext["scene"]),
		AssistantText: stringValue(turn.Assistant["text"]), InfluenceRef: stringValue(mapValue(influences[0])["ref"]),
		PreparedRevision: plan.ExpectedLifeContextRevision, OutcomeRefs: outcomeRefs, ProviderCalls: providerCalls,
	}
}
