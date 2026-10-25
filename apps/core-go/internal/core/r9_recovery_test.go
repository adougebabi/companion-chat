package core

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
	aiagent "github.com/fluctlight/local-ai-companion/apps/core-go/internal/ai/agent"
	capabilitycontract "github.com/fluctlight/local-ai-companion/apps/core-go/internal/capability"
)

func TestR9DiagnosticKeepsLateToolResultsAndTypedEmptyLength(t *testing.T) {
	var messages []*schema.Message
	for i := 0; i < 180; i++ {
		messages = append(messages, schema.UserMessage(fmt.Sprint(i)))
	}
	messages = append(messages, &schema.Message{Role: schema.Tool, ToolCallID: "late-real-result", Content: `{"effect_applied":false,"warning_code":"not_due"}`})
	got := einoDiagnosticMessages(messages)
	if len(got) != len(messages) || got[180]["tool_call_id"] != "late-real-result" || !strings.Contains(jsonString(got[180]), "not_due") {
		t.Fatal("late results hidden", len(got))
	}
	err := fmt.Errorf("formal task: %w", &aiagent.EmptyFinalMessageError{Message: &schema.Message{ResponseMeta: &schema.ResponseMeta{FinishReason: "length", Usage: &schema.TokenUsage{CompletionTokens: 8192}}}})
	if code := initializationProviderErrorCode(err); code != "initialization_response_truncated" {
		t.Fatal(code)
	}
}

func TestR9PhysicalFalseIsExplicitForStructuredTasks(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	seedCognitiveProviderRole(t, f.ctx, f.repository, "r9-structured")
	for _, role := range []string{"initialization", "reflection"} {
		if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,required_capabilities,token_budget,timeout_seconds,retry_policy) VALUES($1,'r9-structured','structured-model','structured_output',4096,10,'{}')`, role); err != nil {
			t.Fatal(err)
		}
	}
	for _, stream := range []bool{false, true} {
		for _, sample := range []struct {
			id   FormalAgentID
			name string
		}{{FormalAgentInitialization, "initialization_response"}, {FormalAgentScheduleGeneration, "schedule_response"}, {FormalAgentConversationSummary, "conversation_segment_v1"}} {
			calls := 0
			f.app.Provider.HTTP = &http.Client{Transport: projectHealthRoundTripFunc(func(req *http.Request) (*http.Response, error) {
				raw, _ := io.ReadAll(req.Body)
				body := decodeObject(raw)
				calls++
				if value, ok := body["enable_thinking"]; !ok || value != false {
					t.Error("false was omitted", body)
				}
				for _, key := range []string{"max_tokens", "max_completion_tokens"} {
					if _, ok := body[key]; ok {
						t.Error("output cap restored", key)
					}
				}
				response := map[string]any{"choices": []any{map[string]any{"message": map[string]any{"role": "assistant", "content": `{"value":"ok"}`}, "finish_reason": "stop"}}}
				if body["stream"] == true {
					response["choices"] = []any{map[string]any{"delta": map[string]any{"role": "assistant", "content": `{"value":"ok"}`}, "finish_reason": "stop"}}
					out := embeddingHTTPResponse(req, 200, "data: "+jsonString(response)+"\n\ndata: [DONE]\n\n")
					out.Header.Set("Content-Type", "text/event-stream")
					return out, nil
				}
				return embeddingHTTPResponse(req, 200, jsonString(response)), nil
			})}
			shape := objectSchema(map[string]any{"value": stringSchema()}, []string{"value"}, false)
			_, err := f.app.RunFormalAgent(f.ctx, sample.id, FormalAgentRunInput{Prompt: PromptAssemblyResult{Messages: []map[string]any{{"role": "user", "content": "return the result"}}, ResponseFormat: shape}, SchemaName: sample.name, EnableThinking: false, EnableStreaming: stream})
			if err != nil || calls != 1 {
				t.Fatal(sample.name, stream, err, calls)
			}
		}
	}
}

func TestR9UnknownToolKeepsStartedActivityAndPublishesOneReply(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(fmt.Sprint(stream), func(t *testing.T) {
			router := newFakeProviderRouter()
			ctx, repo, app, owner, self, conversation := r5MomentFixture(t, "r9-unknown", router)
			phase, writers := 0, 0
			router.otherwise(func(payload map[string]any) fakeProviderResult {
				if payload["model"] == "r5-writer" {
					writers++
					if !strings.Contains(jsonString(payload["messages"]), "active_activities") {
						t.Error("Writer lost started activity")
					}
					return fakeProviderResult{Text: "购物已经开始，还没买到，先等这次结果。"}
				}
				phase++
				switch phase {
				case 1:
					return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("r9-load", capabilitycontract.CapabilityWireName(capabilityCatalogName), map[string]any{"names": []any{lifeActivityStartCapabilityName, wardrobeInspectCapabilityName}})}}
				case 2:
					if !r6PayloadHasTool(payload, lifeActivityStartCapabilityName) {
						t.Fatal("start schema not loaded")
					}
					return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("r9-start", capabilitycontract.CapabilityWireName(lifeActivityStartCapabilityName), map[string]any{"kind": "virtual_shopping", "category": "coat", "slot": "outer", "description": "受控开衫", "duration_minutes": 15, "reason": "接受这次虚拟购物"})}}
				case 3:
					return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("r9-canonical", wardrobeInspectCapabilityName, map[string]any{"operation": "wearing"})}}
				case 4:
					return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("r9-unknown", "wardrobe", map[string]any{"operation": "catalogue"})}}
				case 5:
					if !strings.Contains(jsonString(payload["messages"]), "capability_not_installed") {
						t.Error("unknown name produced no native feedback")
					}
					return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("r9-reply", capabilitycontract.CapabilityWireName(conversationReplyCapabilityName), map[string]any{})}}
				default:
					t.Error("unexpected loop", phase)
					return fakeProviderResult{Status: 400}
				}
			})
			result, err := app.handleTurn(ctx, owner, conversation, takeoverChainTurnPayload(self, "如果愿意，去买一件开衫，开始后告诉我。", "r9-unknown", "r9-unknown"), turnCallbacks{}, stream)
			if err != nil || writers != 1 || phase != 5 || stringValue(result.Assistant["text"]) == "" {
				t.Fatal(result, err, writers, phase)
			}
			var activities, grants, intents int
			if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.fluctlight_life_activity_runs WHERE fluctlight_id=$1 AND status='in_progress'`, self).Scan(&activities); err != nil || activities != 1 {
				t.Fatal(activities, err)
			}
			if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND source_kind='purchase_result'`, self).Scan(&grants); err != nil || grants != 0 {
				t.Fatal("start granted a purchase", grants, err)
			}
			if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.platform_workflow_intents WHERE intent_type='life.activity.resolve' AND payload->>'fluctlight_id'=$1`, self).Scan(&intents); err != nil || intents != 1 {
				t.Fatal("missing durable timer", intents, err)
			}
		})
	}
}

func TestR9RepeatedMomentReadFinishesWithFactsInsteadOfMoreActions(t *testing.T) {
	router := newFakeProviderRouter()
	ctx, repo, app, owner, self, conversation := r5MomentFixture(t, "r9-read-loop", router)
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.moments(id,owner_actor_id,owner_fluctlight_id,author_actor_id,text,media_asset_ids) VALUES('r9-post',$1,$2,$2,'今天在书店读完了那本随笔。','[]')`, owner, self); err != nil {
		t.Fatal(err)
	}
	phase := 0
	router.otherwise(func(payload map[string]any) fakeProviderResult {
		if payload["model"] == "r5-writer" {
			if !strings.Contains(jsonString(payload["messages"]), "今天在书店读完了那本随笔") {
				t.Error("actual post absent from Writer")
			}
			return fakeProviderResult{Text: "那条写的是：今天在书店读完了那本随笔。"}
		}
		phase++
		if phase == 1 {
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("r9-load", capabilitycontract.CapabilityWireName(capabilityCatalogName), map[string]any{"names": []any{momentInspectCapabilityName}})}}
		}
		if phase <= 4 {
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall(fmt.Sprintf("r9-read-%d", phase), capabilitycontract.CapabilityWireName(momentInspectCapabilityName), map[string]any{})}}
		}
		if phase == 5 {
			tools := arrayValue(payload["tools"])
			if len(tools) != 1 || !r6PayloadHasTool(payload, conversationReplyCapabilityName) || payload["enable_thinking"] != false {
				t.Error("loop did not enter expression", payload["tools"])
			}
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("r9-reply", capabilitycontract.CapabilityWireName(conversationReplyCapabilityName), map[string]any{})}}
		}
		t.Error("query loop continued", phase)
		return fakeProviderResult{Status: 400}
	})
	result, err := app.handleTurn(ctx, owner, conversation, takeoverChainTurnPayload(self, "今天发的动态原文是什么？", "r9-recall", "r9-recall"), turnCallbacks{}, false)
	if err != nil || phase != 5 || !strings.Contains(stringValue(result.Assistant["text"]), "随笔") {
		t.Fatal(result, err, phase)
	}
	var posts int
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.moments WHERE owner_fluctlight_id=$1`, self).Scan(&posts); err != nil || posts != 1 {
		t.Fatal(posts, err)
	}
}

func TestR9ShoppingTimerRecoveryExtensionDedupAndSelectedWear(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	_, place, item := seedR8Shop(t, f)
	args := map[string]any{"kind": "virtual_shopping", "duration_minutes": 15, "category": "coat", "slot": "outer", "description": "燕麦色针织开衫", "world_item_id": item, "place_id": place, "wear_after_acquisition": true, "reason": "买到后决定换上这件开衫"}
	started, err := f.app.ExecuteTool(f.ctx, f.request(lifeActivityStartCapabilityName, "r9-start", args))
	if err != nil || started.Result.Status != "accepted" {
		t.Fatal(started, err)
	}
	id := stringValue(mapValue(started.Result.Output)["activity_id"])
	var beforeDue, beforeEnd time.Time
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT not_before,active_until FROM public.fluctlight_life_activity_runs WHERE id=$1`, id).Scan(&beforeDue, &beforeEnd); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `DELETE FROM public.platform_workflow_intents WHERE intent_id=$1`, "activity_resolution:"+id); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err := f.app.EnsureLifeActivityResolutionIntents(f.ctx, 100); err != nil {
			t.Fatal(err)
		}
	}
	for i, minutes := range []int{30, 45, 60, 90, 60} {
		request := f.request(lifeActivityAdvanceCapabilityName, fmt.Sprintf("r9-extend-%d", i), map[string]any{"activity_id": id, "extend_minutes": minutes, "reason": "继续购物"})
		request.EvidenceID = "same-user-turn"
		result, err := f.app.ExecuteTool(f.ctx, request)
		if err != nil {
			t.Fatal(result, err)
		}
		if i > 0 && (result.Result.Status != "handled" || mapValue(result.Result.Output)["warning_code"] != "activity_extension_already_recorded") {
			t.Fatal("repeated extension applied", result)
		}
	}
	var due, end time.Time
	var intents int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT not_before,active_until FROM public.fluctlight_life_activity_runs WHERE id=$1`, id).Scan(&due, &end); err != nil || !due.Equal(beforeDue) || !end.Equal(beforeEnd.Add(30*time.Minute)) {
		t.Fatal(due, end, err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.platform_workflow_intents WHERE intent_id=$1`, "activity_resolution:"+id).Scan(&intents); err != nil || intents != 1 {
		t.Fatal(intents, err)
	}
	pending, err := f.app.ResolveScheduledLifeActivity(f.ctx, id)
	if err != nil || pending["status"] != "pending" {
		t.Fatal(pending, err)
	}
	f.app.Clock = fixedClock(due.Add(time.Second))
	setupVirtualActivityTestProvider(t, f, map[string]any{"status": "completed", "reason": "虚拟购买完成", "acquired_item": map[string]any{"item_kind": "wearable", "category": "coat", "slot": "outer", "description": "燕麦色针织开衫"}})
	resolved, err := f.app.ResolveScheduledLifeActivity(f.ctx, id)
	if err != nil || resolved["status"] != "completed" {
		t.Fatal(resolved, err)
	}
	var acquired string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT id FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND source_world_item_id=$2`, f.fluctlightID, item).Scan(&acquired); err != nil {
		t.Fatal(err)
	}
	ids := r6WornIDs(t, f)
	if len(ids) != 3 || !containsString(ids, acquired) {
		t.Fatal("selected post-acquisition wear missing or removed existing clothes", ids)
	}
	if _, err := f.app.ResolveScheduledLifeActivity(f.ctx, id); err != nil {
		t.Fatal(err)
	}
	var wearCalls int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.tool_executions WHERE fluctlight_id=$1 AND capability_name='wardrobe.wear'`, f.fluctlightID).Scan(&wearCalls); err != nil || wearCalls != 1 {
		t.Fatal("followup replay repeated wearing", wearCalls, err)
	}
}

func TestR9InitialScheduleRecoveryReusesIdentityAndReplays(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	id := "schedule_intent:" + f.fluctlightID
	workflow := "schedule:" + f.fluctlightID
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload,status,last_error,attempt_count) VALUES($1,$2,'lifecycle','schedule.current_day',$3,'failed','empty length',1) ON CONFLICT(intent_id) DO UPDATE SET status='failed',last_error='empty length'`, id, workflow, jsonBytes(map[string]any{"fluctlight_id": f.fluctlightID})); err != nil {
		t.Fatal(err)
	}
	path := "/internal/fluctlights/" + f.fluctlightID + "/schedules/recover"
	body := map[string]any{"idempotency_key": "r9-recover", "reason": "模型已核对，恢复原日程"}
	for i := 0; i < 2; i++ {
		result, err := f.app.DispatchOwnerSurface(f.ctx, "POST", path, nil, f.ownerID, body)
		if err != nil || result["intent_id"] != id || result["workflow_id"] != workflow || result["status"] != "pending" {
			t.Fatal(result, err)
		}
	}
	var count int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.platform_workflow_intents WHERE payload->>'fluctlight_id'=$1 AND intent_type='schedule.current_day'`, f.fluctlightID).Scan(&count); err != nil || count != 1 {
		t.Fatal("recovery created another scheduler", count, err)
	}
	if _, err := f.app.DispatchOwnerSurface(f.ctx, "POST", path, nil, f.foreignOwnerID, map[string]any{"idempotency_key": "foreign", "reason": "bad"}); err == nil {
		t.Fatal("foreign recovery accepted")
	}
}

func TestR9NewPlacePreviewRequiresCreationAndActuallyApplies(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	_, _, _ = seedR8Shop(t, f)
	r, err := f.app.DispatchOwnerSurface(f.ctx, "POST", "/internal/studio/resources", nil, f.ownerID, map[string]any{"kind": "place", "name": "西湖边散步入口", "body": map[string]any{"description": "湖边的公共入口", "floor": ""}, "reason": "定义具体地点", "idempotency_key": "r9-place"})
	if err != nil {
		t.Fatal(err)
	}
	id := stringValue(r["id"])
	pub, err := f.app.publishWorldResource(f.ctx, f.ownerID, id, map[string]any{"expected_revision": r["revision"], "reason": "发布", "idempotency_key": "r9-place-publish"})
	if err != nil {
		t.Fatal(err)
	}
	preview, err := f.app.DispatchWorldApply(f.ctx, f.ownerID, id, map[string]any{"fluctlight_id": f.fluctlightID, "resource_version": pub["published_version"]}, true)
	if err != nil || preview["create_required"] != true || preview["operation"] != "create" || len(arrayValue(preview["diff"])) != 0 {
		t.Fatal(preview, err)
	}
	result, err := f.app.DispatchWorldApply(f.ctx, f.ownerID, id, map[string]any{"fluctlight_id": f.fluctlightID, "resource_version": pub["published_version"], "expected_instance_revision": preview["instance_revision"], "expected_runtime_revision": 0, "choices": []any{}, "reason": "确认创建", "idempotency_key": "r9-place-apply"}, false)
	if err != nil || result["applied"] != true {
		t.Fatal(result, err)
	}
	var name, floor string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT name,COALESCE(details->>'floor','') FROM public.world_places WHERE id=$1`, result["id"]).Scan(&name, &floor); err != nil || name != "西湖边散步入口" || floor != "" {
		t.Fatal(name, floor, err)
	}
	after, err := f.app.DispatchWorldApply(f.ctx, f.ownerID, id, map[string]any{"fluctlight_id": f.fluctlightID, "resource_version": pub["published_version"]}, true)
	if err != nil || after["create_required"] != false {
		t.Fatal(after, err)
	}
}

func TestR9ActivityRepairSkipsHealthyTimersSoOlderRowsCannotStarveMissingTimer(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.fluctlight_life_activity_runs(id,fluctlight_id,profile_id,kind,status,scheduled_at,not_before,operation_id) SELECT 'r9-repair-'||n,$1,'default','virtual_shopping','in_progress',now(),now()+n*interval '1 minute','r9-repair-'||n FROM generate_series(1,101) n`, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload,status) SELECT 'activity_resolution:r9-repair-'||n,'activity_resolution:r9-repair-'||n,'lifecycle','life.activity.resolve','{}','started' FROM generate_series(1,100)n`); err != nil {
		t.Fatal(err)
	}
	count, err := f.app.EnsureLifeActivityResolutionIntents(f.ctx, 100)
	if err != nil || count != 1 {
		t.Fatal("healthy timers starved missing timer", count, err)
	}
	count, err = f.app.EnsureLifeActivityResolutionIntents(f.ctx, 100)
	if err != nil || count != 0 {
		t.Fatal("repair repeatedly counted unchanged healthy rows", count, err)
	}
}
