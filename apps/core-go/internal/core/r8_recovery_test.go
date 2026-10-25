package core

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func seedR8Shop(t *testing.T, f independentToolE2EFixture) (string, string, string) {
	t.Helper()
	base := "/internal/fluctlights/" + f.fluctlightID
	world, err := f.app.DispatchWorldSurface(f.ctx, "POST", base+"/world/catalog", nil, f.ownerID, map[string]any{"name": "湖滨世界", "reason": "fixture", "idempotency_key": "r8-world"})
	if err != nil {
		t.Fatal(err)
	}
	wid := stringValue(world["id"])
	catalog, err := f.app.worldCatalog(f.ctx, f.fluctlightID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.app.DispatchWorldSurface(f.ctx, "PUT", base+"/world/active", nil, f.ownerID, map[string]any{"world_id": wid, "expected_revision": intValue(mapValue(catalog["active_binding"])["revision"]), "reason": "fixture", "idempotency_key": "r8-select"})
	if err != nil {
		t.Fatal(err)
	}
	categories := []any{map[string]any{"id": "shops", "name": "商店"}}
	saved, err := f.app.DispatchWorldSurface(f.ctx, "PUT", base+"/world/catalog/"+wid, nil, f.ownerID, map[string]any{"name": "湖滨世界", "expected_revision": 1, "venue_categories": categories, "reason": "fixture", "idempotency_key": "r8-category"})
	if err != nil {
		t.Fatal(err)
	}
	restored, err := (&App{DB: f.repository}).worldCatalog(f.ctx, f.fluctlightID)
	if err != nil {
		t.Fatal(err)
	}
	var actual map[string]any
	for _, raw := range arrayValue(restored["items"]) {
		if mapValue(raw)["id"] == wid {
			actual = mapValue(raw)
		}
	}
	if len(arrayValue(actual["venue_categories"])) != 1 || intValue(actual["revision"]) != intValue(saved["revision"]) {
		t.Fatal("category lost after reload", actual)
	}
	_, err = f.app.DispatchWorldSurface(f.ctx, "PUT", base+"/world/catalog/"+wid, nil, f.ownerID, map[string]any{"name": "湖滨世界", "expected_revision": saved["revision"], "venue_categories": []any{}, "reason": "fixture", "idempotency_key": "r8-bad-category"})
	if err == nil {
		t.Fatal("removed fixed category")
	}
	place, err := f.app.DispatchWorldSurface(f.ctx, "POST", base+"/world/places", nil, f.ownerID, map[string]any{"world_id": wid, "name": "湖滨银泰衣橱店", "venue_category_id": "shops", "reason": "fixture", "idempotency_key": "r8-place"})
	if err != nil {
		t.Fatal(err)
	}
	pid := stringValue(place["id"])
	definition := map[string]any{"schema": "item-definition.v1", "item_kind": "wearable", "category": "coat", "slot": "outer", "description": "燕麦色针织开衫"}
	resource, err := f.app.DispatchOwnerSurface(f.ctx, "POST", "/internal/studio/resources", nil, f.ownerID, map[string]any{"kind": "item", "name": "燕麦色针织开衫", "body": definition, "reason": "fixture", "idempotency_key": "r8-item"})
	if err != nil {
		t.Fatal(err)
	}
	rid := stringValue(resource["id"])
	pub, err := f.app.publishWorldResource(f.ctx, f.ownerID, rid, map[string]any{"expected_revision": resource["revision"], "reason": "fixture", "idempotency_key": "r8-publish"})
	if err != nil {
		t.Fatal(err)
	}
	preview, err := f.app.DispatchWorldApply(f.ctx, f.ownerID, rid, map[string]any{"fluctlight_id": f.fluctlightID, "resource_version": pub["published_version"]}, true)
	if err != nil {
		t.Fatal(err)
	}
	registered, err := f.app.DispatchWorldApply(f.ctx, f.ownerID, rid, map[string]any{"fluctlight_id": f.fluctlightID, "resource_version": pub["published_version"], "expected_instance_revision": preview["instance_revision"], "expected_runtime_revision": 0, "world_id": wid, "expected_world_revision": preview["world_binding_revision"], "choices": []any{}, "reason": "fixture", "idempotency_key": "r8-apply"}, false)
	if err != nil {
		t.Fatal(err)
	}
	iid := stringValue(registered["id"])
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.world_items SET place_id=$2 WHERE id=$1`, iid, pid); err != nil {
		t.Fatal(err)
	}
	return wid, pid, iid
}

func TestR8BoundShopCorrectableErrorsThenPurchaseAndWear(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	_, place, item := seedR8Shop(t, f)
	args := map[string]any{"kind": "virtual_shopping", "duration_minutes": 15, "reason": "购买目录中的开衫", "items": []any{map[string]any{"world_item_id": item, "item_kind": "wearable", "category": "coat", "slot": "outer", "description": "燕麦色针织开衫"}}}
	failed, err := f.app.ExecuteTool(f.ctx, f.request(lifeActivityStartCapabilityName, "missing-place", args))
	if err != nil || failed.Result.Status != "handled" || mapValue(failed.Result.Output)["warning_code"] != "shopping_catalogue_place_required" || mapValue(failed.Result.Output)["required_place_id"] != place {
		t.Fatal("unhelpful place recovery", failed, err)
	}
	var count int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.fluctlight_life_activity_runs WHERE fluctlight_id=$1`, f.fluctlightID).Scan(&count); err != nil || count != 0 {
		t.Fatal("failed admission started activity", count, err)
	}
	args["place_id"] = place
	// Replay r8's actual representation error through the native normalization.
	args["items"] = jsonString(args["items"])
	normalized, err := normalizeNativeModelArguments(jsonBytes(args), lifeActivityStartDefinition())
	if err != nil {
		t.Fatal(err)
	}
	started, err := f.app.ExecuteTool(f.ctx, f.request(lifeActivityStartCapabilityName, "corrected-shop", decodeObject(normalized)))
	if err != nil || started.Result.Status != "accepted" {
		t.Fatal(started, err)
	}
	activity := stringValue(mapValue(started.Result.Output)["activity_id"])
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.fluctlight_wardrobe_items WHERE source_world_item_id=$1`, item).Scan(&count); err != nil || count != 0 {
		t.Fatal("start granted inventory", count, err)
	}
	forceVirtualActivityDue(t, f, activity)
	setupVirtualActivityTestProvider(t, f, map[string]any{"status": "completed", "reason": "虚拟购物已完成", "acquired_item": map[string]any{"item_kind": "wearable", "category": "coat", "slot": "outer", "description": "燕麦色针织开衫"}})
	resolved, err := f.app.ExecuteTool(f.ctx, f.request(lifeActivityAdvanceCapabilityName, "shop-result", map[string]any{"activity_id": activity}))
	if err != nil || resolved.Result.Status != "completed" {
		t.Fatal(resolved, err)
	}
	acquired := stringValue(mapValue(resolved.Result.Output)["item_id"])
	record, err := f.app.OwnerInventoryItem(f.ctx, f.ownerID, f.fluctlightID, acquired)
	if err != nil || record["source_world_item_id"] != item || record["source_kind"] != "purchase_result" || record["source_verified"] != true {
		t.Fatal(record, err)
	}
	worn, err := f.app.ExecuteTool(f.ctx, f.request(wardrobeWearCapabilityName, "wear-shop-result", map[string]any{"mode": "partial", "item_ids": []any{acquired}}))
	if err != nil || worn.Result.Status != "completed" || len(r6WornIDs(t, f)) != 3 {
		t.Fatal("wear failed or removed unrelated items", worn, err)
	}
}

func TestR8MomentReadIsScopedReadOnlyAndPagesSameTimestamp(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	at := time.Date(2026, 10, 25, 15, 0, 0, 0, time.FixedZone("CST", 8*3600))
	for i := 0; i < 4; i++ {
		author, status, owner := f.fluctlightID, "visible", f.ownerID
		if i == 2 {
			author = f.ownerID
		}
		if i == 3 {
			status = "hidden"
		}
		if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.moments(id,owner_actor_id,owner_fluctlight_id,author_actor_id,text,status,media_asset_ids,created_at) VALUES($1,$2,$3,$4,$5,$6,'[]',$7)`, fmt.Sprintf("moment-r8-%d", i), owner, f.fluctlightID, author, fmt.Sprintf("真实动态 %d", i), status, at); err != nil {
			t.Fatal(err)
		}
	}
	tool := momentInspectCapability{repository: f.repository}
	inv := CapabilityInvocation{CapabilityName: momentInspectCapabilityName, Metadata: InvocationMetadata{FluctlightID: f.fluctlightID, AuthorizationActorID: f.ownerID}, Arguments: jsonBytes(map[string]any{"local_date": "2026-10-25", "limit": 1})}
	first, err := tool.Execute(f.ctx, inv, CapabilityContext{})
	if err != nil || len(arrayValue(mapValue(first.Output)["items"])) != 1 || mapValue(first.Output)["has_more"] != true {
		t.Fatal(first, err)
	}
	inv.Arguments = jsonBytes(map[string]any{"local_date": "2026-10-25", "limit": 1, "cursor": mapValue(first.Output)["next_cursor"]})
	second, err := tool.Execute(f.ctx, inv, CapabilityContext{})
	if err != nil || len(arrayValue(mapValue(second.Output)["items"])) != 1 || mapValue(second.Output)["has_more"] != false || jsonString(mapValue(second.Output)["items"]) == jsonString(mapValue(first.Output)["items"]) {
		t.Fatal(second, err)
	}
	inv.Metadata.AuthorizationActorID = f.foreignOwnerID
	if _, err := tool.Execute(f.ctx, inv, CapabilityContext{}); err == nil {
		t.Fatal("cross-owner query accepted")
	}
}

func TestR8PublishedFactsReachWriterButUncommittedTextDoesNot(t *testing.T) {
	app := &App{}
	s := newFactOnlyReplyService(app)
	trace := &ADKCapabilityTrace{}
	reply := CapabilityInvocation{CapabilityName: conversationReplyCapabilityName, SourceFactID: "source", Metadata: InvocationMetadata{AuthorizationActorID: "owner", FluctlightID: "self", ConversationID: "chat"}}
	for i, status := range []string{"completed", "accepted", "failed"} {
		inv := reply
		inv.CallID = fmt.Sprint(i)
		inv.CapabilityName = "moment.publish"
		trace.AppendInvocation(inv)
		trace.AppendResult(CapabilityResult{CallID: inv.CallID, CapabilityName: inv.CapabilityName, Status: status, Output: map[string]any{"target_kind": "moment", "text": status + " text"}})
	}
	inv := reply
	inv.CallID = "read"
	inv.CapabilityName = momentInspectCapabilityName
	trace.AppendInvocation(inv)
	trace.AppendResult(CapabilityResult{CallID: inv.CallID, CapabilityName: inv.CapabilityName, Status: "completed", Output: map[string]any{"items": []any{map[string]any{"text": "昨天真实发布", "published_at": "2026-10-24T15:00:00Z"}}, "has_more": false}})
	facts := factOnlyReplyToolFacts(s.currentActionFacts(WithADKCapabilityInvoker(context.Background(), &appADKCapabilityInvoker{app: app}, trace), reply))
	raw := jsonString(facts)
	if !strings.Contains(raw, "completed text") || !strings.Contains(raw, "昨天真实发布") || strings.Contains(raw, "accepted text") || strings.Contains(raw, "failed text") {
		t.Fatal(raw)
	}
}

func TestR8RepeatedDiscoveryCoolsDownOnlyIdenticalSuccessfulLoads(t *testing.T) {
	trace := &ADKCapabilityTrace{}
	load := CapabilityResult{CapabilityName: capabilityCatalogName, Status: "completed", Output: map[string]any{"loaded": []any{"moment.inspect"}, "unavailable": []any{}}}
	trace.AppendResult(load)
	if len(repeatedDiscoveryExclusions(trace)) != 0 {
		t.Fatal("first load was suppressed")
	}
	trace.AppendResult(load)
	if len(repeatedDiscoveryExclusions(trace)) != 2 {
		t.Fatal("duplicate disclosure was not suppressed")
	}
	trace.AppendResult(load)
	ctx := WithADKCapabilityInvoker(context.Background(), &appADKCapabilityInvoker{app: &App{}}, trace)
	if _, err := (conversationPublicationPhysicalRequestPolicy{}).DecidePhysicalModelRequest(ctx); err == nil || !strings.Contains(err.Error(), "tool_discovery_no_progress") {
		t.Fatal("ignored cooldown was not bounded", err)
	}
	trace.AppendResult(CapabilityResult{CapabilityName: momentInspectCapabilityName, Status: "completed"})
	if len(repeatedDiscoveryExclusions(trace)) != 0 {
		t.Fatal("actual progress did not restore discovery")
	}
	trace.AppendResult(load)
	other := load
	other.Output = map[string]any{"loaded": []any{"wardrobe.inspect"}}
	trace.AppendResult(other)
	if len(repeatedDiscoveryExclusions(trace)) != 0 {
		t.Fatal("different schema load was suppressed")
	}
}

func TestR8SegmentParticipantsAndNativeSchema(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	seedCognitiveProviderRole(t, f.ctx, f.repository, "r8-summary")
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.model_roles SET role='reflection' WHERE provider_endpoint_id='r8-summary'`); err != nil {
		t.Fatal(err)
	}
	calls := 0
	app := newTestApp(t, f.repository, projectHealthRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		raw, _ := io.ReadAll(req.Body)
		body := decodeObject(raw)
		calls++
		if calls == 1 {
			schema := mapValue(mapValue(body["response_format"])["json_schema"])
			if schema["name"] != "conversation_segment_v1" || !strings.Contains(string(raw), `actor_self`) || !strings.Contains(string(raw), `actor_user`) || !strings.Contains(string(raw), `苏晚`) {
				t.Fatal("identity or native schema missing", string(raw))
			}
			return embeddingHTTPResponse(req, 200, jsonString(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": "# Markdown is invalid"}}}})), nil
		}
		if len(arrayValue(body["tools"])) != 0 {
			t.Fatal("summary repair received tools")
		}
		value := map[string]any{"schema_version": conversationSegmentSchemaVersion, "summary": "苏晚与用户谈论尚未完成的购物。", "ending_state": "购物尚未完成", "open_threads": []any{"等待结果"}, "core_events": []any{"用户提出购物"}}
		return embeddingHTTPResponse(req, 200, jsonString(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": jsonString(value)}}}})), nil
	}))
	source := []ConversationSummarySourceMessage{{AuthorActorID: f.ownerID, Kind: "user", Text: "试试这件开衫", CreatedAt: time.Now()}, {AuthorActorID: f.fluctlightID, Kind: "assistant", Text: "我还没买到", CreatedAt: time.Now()}}
	got, err := app.RunConversationSegmentTask(f.ctx, ConversationSegmentTaskInput{Messages: source, OwnerActorID: f.ownerID, FluctlightID: f.fluctlightID, SelfName: "苏晚"})
	if err != nil || got.SchemaVersion != conversationSegmentSchemaVersion || calls != 2 {
		t.Fatal(got, err, calls)
	}
}

func TestR8LiveSegmentAndMomentRecall(t *testing.T) {
	base, model := liveProviderConfig(t)
	f := seedWardrobeToolFixture(t)
	seedCognitiveProviderRole(t, f.ctx, f.repository, "r8-live")
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.provider_endpoints SET base_url=$1 WHERE id='r8-live'`, base); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,required_capabilities,token_budget,timeout_seconds,retry_policy) VALUES('reflection','r8-live',$1,'structured_output',4096,180,'{}')`, model); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.model_roles SET model_id=$1,timeout_seconds=180 WHERE provider_endpoint_id='r8-live'`, model); err != nil {
		t.Fatal(err)
	}
	wire := &r7LiveWire{}
	dir := os.Getenv("FLUCTLIGHT_LIVE_EVIDENCE_DIR")
	if dir != "" {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		wire.path = filepath.Join(dir, "r8-live-wire.json")
	}
	app := newTestApp(t, f.repository, wire)
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()
	f.ctx = ctx
	ctx = withFrozenProviderAssignment(ctx, conversationReplyModelRole, providerAssignment{Role: conversationReplyModelRole, BaseURL: base, ModelID: model, Timeout: 180 * time.Second})
	source := []ConversationSummarySourceMessage{{AuthorActorID: f.ownerID, Kind: "user", Text: "开衫买到了吗？", CreatedAt: time.Now().Add(-time.Minute)}, {AuthorActorID: f.fluctlightID, Kind: "assistant", Text: "还没买到，刚才购买没有成功。", CreatedAt: time.Now()}}
	summary, err := app.RunConversationSegmentTask(ctx, ConversationSegmentTaskInput{Messages: source, OwnerActorID: f.ownerID, FluctlightID: f.fluctlightID, SelfName: "苏晚"})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("live segment: %+v", summary)
	if _, err := f.repository.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3);`, f.ownerID, f.fluctlightID, f.conversationID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(ctx, `INSERT INTO public.moments(id,owner_actor_id,owner_fluctlight_id,author_actor_id,text,media_asset_ids) VALUES('r8-live-post',$1,$2,$2,'今天在书店读完了那本湖边散步随笔。','[]')`, f.ownerID, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	result, err := app.handleTurn(ctx, f.ownerID, f.conversationID, takeoverChainTurnPayload(f.fluctlightID, "你今天发的那条动态写了什么？请告诉我原文。", "r8-recall", "r8-recall"), turnCallbacks{}, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("live recall: %s", stringValue(result.Assistant["text"]))
	var posts int
	if err := f.repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.moments WHERE owner_fluctlight_id=$1`, f.fluctlightID).Scan(&posts); err != nil || posts != 1 {
		t.Fatal("recall published a new post", posts, err)
	}
	// Pure queries do not create mutation-ledger rows. Verify their actual
	// server-generated ToolResult on the subsequent physical Provider request.
	queried := false
	for _, row := range wire.rows {
		for _, raw := range arrayValue(mapValue(row["request"])["messages"]) {
			message := mapValue(raw)
			if message["role"] == "tool" && message["name"] == "moment_inspect" {
				result := decodeObject([]byte(stringValue(message["content"])))
				queried = queried || (result["status"] == "completed" && strings.Contains(jsonString(mapValue(result["output"])["items"]), "湖边散步随笔"))
			}
		}
	}
	if !queried {
		t.Fatal("no actual successful moment read in native ToolResults")
	}
	if !strings.Contains(stringValue(result.Assistant["text"]), "湖边散步随笔") {
		t.Fatal("reply lost actual publication", result.Assistant)
	}
}

func TestR8LiveCatalogueShopping(t *testing.T) {
	base, model := liveProviderConfig(t)
	f := seedWardrobeToolFixture(t)
	_, _, worldItem := seedR8Shop(t, f)
	seedCognitiveProviderRole(t, f.ctx, f.repository, "r8-live-shop")
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.provider_endpoints SET base_url=$1 WHERE id='r8-live-shop'`, base); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.model_roles SET model_id=$1,timeout_seconds=180 WHERE provider_endpoint_id='r8-live-shop'`, model); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, f.ownerID, f.fluctlightID, f.conversationID); err != nil {
		t.Fatal(err)
	}
	wire := &r7LiveWire{}
	if dir := os.Getenv("FLUCTLIGHT_LIVE_EVIDENCE_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		wire.path = filepath.Join(dir, "r8-shopping-wire.json")
	}
	app := newTestApp(t, f.repository, wire)
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()
	ctx = withFrozenProviderAssignment(ctx, conversationReplyModelRole, providerAssignment{Role: conversationReplyModelRole, BaseURL: base, ModelID: model, Timeout: 180 * time.Second})
	f.ctx, f.app = ctx, app
	result, err := app.handleTurn(ctx, f.ownerID, f.conversationID, takeoverChainTurnPayload(f.fluctlightID, "去湖滨银泰衣橱店买那件燕麦色针织开衫吧，买到以后再换上。先看看店里目录，开始后告诉我。", "r8-shopping", "r8-shopping"), turnCallbacks{}, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("live shopping start: %s", stringValue(result.Assistant["text"]))
	var activity string
	if err := f.repository.Pool().QueryRow(ctx, `SELECT id FROM public.fluctlight_life_activity_runs WHERE fluctlight_id=$1 AND kind='virtual_shopping' AND status='in_progress'`, f.fluctlightID).Scan(&activity); err != nil {
		t.Fatal("no real shopping started", err)
	}
	var acquiredBefore int
	if err := f.repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.fluctlight_wardrobe_items WHERE source_world_item_id=$1`, worldItem).Scan(&acquiredBefore); err != nil || acquiredBefore != 0 {
		t.Fatal("start prematurely acquired item", acquiredBefore, err)
	}
	// Controlled elapsed-time fixture only; result semantics remain real LLM,
	// and production admission/settlement commits the inventory and provenance.
	forceVirtualActivityDue(t, f, activity)
	resolved, err := app.ExecuteTool(ctx, f.request(lifeActivityAdvanceCapabilityName, "live-shop-result", map[string]any{"activity_id": activity}))
	if err != nil || resolved.Result.Status != "completed" {
		t.Fatal("live result did not acquire", resolved, err)
	}
	item := stringValue(mapValue(resolved.Result.Output)["item_id"])
	if item == "" {
		t.Fatal("missing actual acquired item", resolved)
	}
	result, err = app.handleTurn(ctx, f.ownerID, f.conversationID, takeoverChainTurnPayload(f.fluctlightID, "刚才买的燕麦色针织开衫到了，麻烦换上它，其他衣服保留。按实际衣柜核对，换好后告诉我。", "r8-shop-wear", "r8-shop-wear"), turnCallbacks{}, false)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("live shopping wear: %s", stringValue(result.Assistant["text"]))
	ids := r6WornIDs(t, f)
	found := false
	for _, id := range ids {
		if id == item {
			found = true
		}
	}
	if !found || len(ids) != 3 {
		t.Fatal("purchased item was not worn or unrelated clothing removed", ids, item)
	}
}
