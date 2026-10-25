package core

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

func TestEveryFormalAgentSharesLosslessProtocolComposition(t *testing.T) {
	// Deliberately explicit: a new Agent must be included in the documented
	// composition matrix, rather than silently inheriting all conversation data.
	ids := strings.Fields("conversation_cognition conversation_reply reply_action_check wake_up takeover_judge takeover_reply wardrobe_preparation initialization persona_compilation media_prompt media_quality visual_identity_vision visual_identity_patch visual_identity conversation_summary schedule_generation native_cognition studio_assistant studio_text_preview npc_response daily_review diary_material diary_expression persistent_switch reflection schedule_replan goal_planner goal_evaluation virtual_activity_result")
	if len(ids) != len(FormalAgentDefinitions()) {
		t.Fatal("update the Agent composition inventory and matrix")
	}
	for _, id := range ids {
		t.Run(id, func(t *testing.T) {
			if _, ok := FormalAgentDefinitionByID(FormalAgentID(id)); !ok {
				t.Fatal("missing formal Agent")
			}
			input := []map[string]any{
				{"role": "system", "content": "task rule"}, {"role": "system", "content": "task rule"},
				{"role": "user", "content": []any{map[string]any{"type": "text", "text": ` {"id":"quoted","value":false} `}, map[string]any{"type": "image_url", "image_url": map[string]any{"url": "data:image/png;base64,bytes"}}}, "metadata": int64(9007199254740993)},
				{"role": "tool", "tool_call_id": "native-call", "content": `{"evidence_refs":["native"],"success":true}`},
				{"role": "user", "content": "Current original text\n {\"version\": 7}"},
			}
			got := formalPromptMessages(FormalAgentID(id), input)
			if len(got) != 4 || strings.Count(stringValue(got[0]["content"]), "task rule") != 1 || !strings.HasPrefix(stringValue(got[0]["content"]), "# 运行协议") || !reflect.DeepEqual(got[1:], input[2:]) {
				t.Fatalf("protocol duplicated or original data changed: %#v", got)
			}
			if !reflect.DeepEqual(formalPromptMessages(FormalAgentID(id), got), got) {
				t.Fatal("composition is not idempotent")
			}
		})
	}
}

func TestGoalEvidenceSharesPayloadWithoutMergingEvents(t *testing.T) {
	snapshot := richGoalEvaluationWireSnapshot()
	first := snapshot.Sources[0]
	second := first
	second.EventID, second.Ref = 78, "source-second"
	second.OccurredAt = first.OccurredAt.Add(time.Minute)
	second.Valid = false
	snapshot.Sources = append(snapshot.Sources, second)
	before := jsonString(snapshot)
	binding, _, current, err := goalEvaluationWireInput(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	sources := arrayValue(current["sources"])
	payloads := mapValue(current["evidence_payloads"])
	if len(sources) != 2 || len(payloads) != 1 || mapValue(sources[0])["payload_ref"] != mapValue(sources[1])["payload_ref"] || mapValue(sources[0])["ref"] == mapValue(sources[1])["ref"] || mapValue(sources[0])["valid"] == mapValue(sources[1])["valid"] {
		t.Fatalf("events merged or data repeated: %s", jsonString(current))
	}
	refs, err := binding.hydrateSourceRefs([]string{"e1", "e2"})
	if err != nil || !reflect.DeepEqual(refs, []string{first.Ref, second.Ref}) || jsonString(snapshot) != before {
		t.Fatalf("frozen source binding changed: %v %v", refs, err)
	}
}

func TestGoalEvidenceKeepsDomainSchemasOpaqueAndProjectsCatalogResults(t *testing.T) {
	schema := map[string]any{"schema_version": "domain.v1", "$ref": "#/definitions/item", "properties": map[string]any{"item_ids": map[string]any{"type": "array"}, "evidence_refs": map[string]any{"type": "array"}, "goal_id": map[string]any{"type": "string", "default": "domain-id"}}, "required": []any{"item_ids", "evidence_refs"}}
	snapshot := goalEvaluationSnapshot{Sources: []GoalSource{
		{EventID: 1, Ref: "source:1", Kind: "outcome", Data: map[string]any{"capability": "domain.query", "observed": map[string]any{"schema": schema, "id": "domain-record", "version": 7}}},
		{EventID: 2, Ref: "source:2", Kind: "outcome", Data: map[string]any{"capability": capabilityCatalogName, "status": "completed", "observed": map[string]any{"items": []any{map[string]any{"name": "wardrobe.wear", "purpose": strings.Repeat("tool documentation", 100), "parameters": schema}}, "loaded": []any{"wardrobe.wear"}, "unavailable": []any{"other.tool"}}}},
		{EventID: 3, Ref: "source:3", Kind: "activity", Data: map[string]any{"status": "completed", "result": map[string]any{"event_id": "internal-event", "item_id": "actual-item", "description": "真实取得的衣物", "value_json": schema}}},
	}}
	before := jsonString(snapshot)
	binding, _, current, err := goalEvaluationWireInput(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	sources := arrayValue(current["sources"])
	domain := mapValue(mapValue(sources[0])["data"])["observed"]
	if jsonString(domain) != jsonString(snapshot.Sources[0].Data["observed"]) || binding.objectRefs["domain-record"] != "" || binding.objectRefs["domain-id"] != "" {
		t.Fatal("domain data rewritten or schema discovered as an entity")
	}
	catalog := jsonString(mapValue(sources[1])["data"])
	if strings.Contains(catalog, "parameters") || strings.Contains(catalog, "tool documentation") || !strings.Contains(catalog, "wardrobe.wear") || !strings.Contains(catalog, "other.tool") {
		t.Fatalf("catalog evidence is not semantic: %s", catalog)
	}
	activity := jsonString(mapValue(sources[2])["data"])
	if strings.Contains(activity, "internal-event") || strings.Contains(activity, "actual-item") || !strings.Contains(activity, "真实取得的衣物") || binding.objectRefs["actual-item"] == "" || jsonString(snapshot) != before {
		t.Fatalf("activity envelope leaked or semantic body lost: %s", activity)
	}
}

func TestGoalEvaluationViewHasOneGoalOwnerAndNoRawChat(t *testing.T) {
	snapshot := richGoalEvaluationWireSnapshot()
	projection := ContextProjection{Goals: []map[string]any{{"id": "goal-secret", "desired_outcome": "selected unique goal"}, {"id": "other-goal", "desired_outcome": "other relevant goal", "state": "active"}}}
	copy := goalEvaluationPromptProjection(snapshot, projection)
	view := compactCognitionContextForSurface(copy, ProviderContextSurfaceGoalEvaluation)
	if len(projection.Goals) != 2 || strings.Contains(jsonString(view), "selected unique goal") || !strings.Contains(jsonString(view), "other relevant goal") || view["goals"] != nil || providerContextSurfaceAllowsRecentHistory(ProviderContextSurfaceGoalEvaluation) || providerContextSurfaceAllowsSummaries(ProviderContextSurfaceGoalEvaluation) {
		t.Fatalf("goal ownership/history drift: %s", jsonString(view))
	}
}

func TestPromptScheduleUsesOneItemAndOneTimePair(t *testing.T) {
	item := map[string]any{"ref": "item:1", "start_at": "2026-10-23T10:00:00+08:00", "end_at": "2026-10-23T11:00:00+08:00", "activity": "阅读", "local_start_time": "10:00", "local_start_at": "2026-10-23T10:00:00+08:00", "local_end_time": "11:00", "local_end_at": "2026-10-23T11:00:00+08:00", "local_time_range": "10:00 - 11:00"}
	input := map[string]any{"items": []any{item}, "current_item": item, "timezone": "Asia/Shanghai"}
	before := jsonString(input)
	view := promptScheduleView(input)
	if view["current_item_index"] != 0 || view["current_item"] != nil || strings.Count(jsonString(view), "阅读") != 1 || strings.Contains(jsonString(view), "local_start") || jsonString(input) != before {
		t.Fatalf("schedule repeated or original mutated: %s", jsonString(view))
	}
}

func TestTaskViewsHideEnvelopeButKeepRequiredBusinessIdentifiers(t *testing.T) {
	body := map[string]any{"schema_version": "business.v1", "item_id": "definition-field", "user_json": map[string]any{"version": 7}}
	refs := []any{map[string]any{"resource_id": "allowed-target", "resource_version": 3}}
	snapshot := map[string]any{"owner_actor_id": "private-owner", "resource_id": "edited-resource", "resource_revision": 7, "resource_version": 2, "resource_kind": "outfit", "name": "名字", "body": body, "instruction": "原文", "allowed_references": refs}
	studio := studioPromptView(snapshot)
	if !reflect.DeepEqual(studio["body"], body) || !reflect.DeepEqual(studio["allowed_references"], refs) || strings.Contains(jsonString(studio), "private-owner") || strings.Contains(jsonString(studio), "edited-resource") {
		t.Fatal("Studio view crossed definition/envelope boundary")
	}
	npc := npcMomentPromptView(map[string]any{"npc_id": "private-npc", "revision": 5, "name": "NPC", "moment": map[string]any{"id": "private-moment", "author_actor_id": "private-author", "text": "公开内容"}})
	if strings.Contains(jsonString(npc), "private-") || !strings.Contains(jsonString(npc), "公开内容") {
		t.Fatal("NPC bookkeeping leaked")
	}
	request := map[string]any{"item_id": "exact-tool-target", "_request_id": "private-request", "expected_item_revision": 2, "_render_intent": "完整原始意图", "_item_snapshot": map[string]any{"id": "private-row", "revision": 2, "description": "物品定义"}}
	before := jsonString(request)
	owner := ownerLifePromptView("item_photo", request)
	if owner["item_id"] != "exact-tool-target" || owner["_render_intent"] != "完整原始意图" || strings.Contains(jsonString(owner), "private-") || !strings.Contains(jsonString(owner), "物品定义") || jsonString(request) != before {
		t.Fatal("Owner request lost binding or leaked CAS")
	}
}

func TestMetadataProjectionPreservesOpaqueUserDataAndIdentifierLikeWords(t *testing.T) {
	text := "  文档名称 memory_1234567890abcdef；不要改写。  "
	value := map[string]any{"source_fact_id": "private-source", "text": text, "valid": false, "avoid": "保留", "value_json": map[string]any{"id": "user-defined", "version": 7, "large": json.Number("9007199254740993")}, "schema": map[string]any{"properties": map[string]any{"item_ids": map[string]any{"type": "array"}}}}
	for _, project := range []func(any) any{stripProviderMetadata, stripProviderContextMetadata} {
		got := mapValue(project(value))
		if got["source_fact_id"] != nil || got["text"] != text || got["valid"] != false || got["avoid"] != "保留" || !reflect.DeepEqual(got["value_json"], value["value_json"]) || !reflect.DeepEqual(got["schema"], value["schema"]) {
			t.Fatalf("metadata projection rewrote ordinary/domain data: %#v", got)
		}
	}
}

func TestInitializationTaskPreservesOwnerJSONWithoutSystemPromotion(t *testing.T) {
	description := " { \"core_persona\": {\"identity\": {\"name\": \"OWNER_UNTRUSTED_NAME\"}}, \"notes\": \"原始资料\" } "
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Error(err)
		}
		http.Error(w, "fixture ends after capturing the request", http.StatusBadRequest)
	}))
	defer server.Close()
	app := &App{Provider: &ProviderClient{HTTP: server.Client()}}
	ctx := withFrozenProviderAssignment(context.Background(), "initialization", providerAssignment{Role: "initialization", BaseURL: server.URL, ModelID: "local-fixture", Timeout: time.Second, TokenBudget: 1})
	_, err := app.RunInitializationTask(ctx, InitializationTaskInput{Description: description})
	if err == nil || captured == nil {
		t.Fatalf("fixture must capture actual task request and stop: %v", err)
	}
	messages := arrayValue(captured["messages"])
	if len(messages) != 2 || mapValue(messages[1])["content"] != description || strings.Contains(stringValue(mapValue(messages[0])["content"]), "OWNER_UNTRUSTED_NAME") || strings.Contains(stringValue(mapValue(messages[0])["content"]), "# 人格设定") || captured["max_tokens"] != nil || captured["max_completion_tokens"] != nil {
		t.Fatalf("Owner JSON altered/promoted or output capped: %#v", messages)
	}
}

func TestGoalItemEvidenceOmitsInventoryProvenance(t *testing.T) {
	value := map[string]any{"id": "domain-value", "version": 9, "schema": map[string]any{"properties": map[string]any{"source_world_item_id": map[string]any{"type": "string"}}}}
	item := map[string]any{"id": "inventory-item", "description": "蓝色外套", "ownership": "owned", "availability": "available", "source_resource_id": "private-resource", "source_world_item_id": "private-world-item", "source_world_item_revision": 8, "value_json": value}
	snapshot := goalEvaluationSnapshot{Sources: []GoalSource{{EventID: 1, Ref: "source:1", Kind: "item", Data: map[string]any{"item": item}}}}
	binding, _, current, err := goalEvaluationWireInput(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	projected := mapValue(mapValue(mapValue(arrayValue(current["sources"])[0])["data"])["item"])
	if projected["ref"] == nil || binding.objectRefs["inventory-item"] == "" || strings.Contains(jsonString(projected), "private-") || projected["source_world_item_revision"] != nil || !reflect.DeepEqual(projected["value_json"], value) || projected["description"] != "蓝色外套" {
		t.Fatalf("inventory projection leaked provenance or changed body: %#v", projected)
	}
}

func TestNPCOpportunityViewKeepsExecutableTargetAndKnownState(t *testing.T) {
	view := npcOpportunityPromptView(map[string]any{"id": "opportunity-target", "revision": 3, "place_name": "公园", "source_event_id": "private-event", "npc_presence_id": "private-presence", "npc": map[string]any{"actor_id": "private-actor", "public_role": "花匠", "known": false}})
	if view["id"] != "opportunity-target" || view["revision"] != 3 || mapValue(view["npc"])["public_role"] != "花匠" || mapValue(view["npc"])["known"] != false || strings.Contains(jsonString(view), "private-") {
		t.Fatalf("opportunity lost its public semantics or leaked provenance: %#v", view)
	}
}

func TestWorkingMemoryKeepsDistinctClaimsWithSharedSources(t *testing.T) {
	working, err := ResolveWorkingMemory(WorkingMemoryInput{
		ActiveCandidates:  []PromptFragment{{Kind: PromptFragmentActiveMemory, Content: map[string]any{"content": "喜欢咖啡"}, SourceRefs: []string{"source:1"}}},
		RetrievedMemories: []PromptFragment{{Kind: PromptFragmentRetrievedMemory, Content: map[string]any{"content": "不喜欢甜咖啡"}, SourceRefs: []string{"source:1"}}},
	}, DefaultWorkingMemoryPolicy())
	if err != nil || len(working.Active) != 1 || len(working.Retrieved) != 1 {
		t.Fatalf("same source lost a distinct claim: %#v %v", working, err)
	}
}

func TestPromptCompositionUsesJSONWithNoShapeDependentEncoding(t *testing.T) {
	for _, data := range []map[string]any{{"rows": []any{}}, {"rows": []any{map[string]any{"a": false}}}, {"rows": []any{map[string]any{"a": false}, map[string]any{"a": true}}}} {
		wire := renderProviderRuntimeContext(data)
		if !json.Valid([]byte(wire)) || jsonString(decodeObject([]byte(wire))) != jsonString(data) {
			t.Fatalf("shape changed encoding or values: %s", wire)
		}
	}
}

// Reconstruct a supplied prompt's semantic views locally. This does not call a
// model or reproduce its original database snapshot. Only aggregate sizes are
// printed; private input stays outside the repository.
func TestGoalPromptPrivateSampleReplay(t *testing.T) {
	path := os.Getenv("FLUCTLIGHT_PROMPT_REPLAY_PATH")
	if path == "" {
		t.Skip("private prompt replay not configured")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	packet := decodeObject(raw)
	current, runtime := mapValue(packet["current"]), mapValue(packet["runtime"])
	sources := arrayValue(current["sources"])
	for _, raw := range sources {
		source := mapValue(raw)
		source["data"] = compactGoalEvaluationSourceData(GoalSource{Kind: stringValue(source["kind"]), Data: mapValue(source["data"])})
	}
	if payloads := shareGoalEvidencePayloads(sources); len(payloads) > 0 {
		current["evidence_payloads"] = payloads
	}
	// This capture's two runtime goals are exactly its two selected goals.
	for _, key := range []string{"goals", "goal_policy", "intentions", "recent_outcomes", "visual_identity", "current_speaker"} {
		delete(runtime, key)
	}
	working := WorkingMemory{}
	keys := make([]string, 0, len(runtime))
	for key := range runtime {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		working.RuntimeFacts = append(working.RuntimeFacts, PromptFragment{Kind: PromptFragmentRuntimeFact, Required: true, Content: map[string]any{"kind": key, "value": runtime[key]}})
	}
	assembled, err := AssemblePromptContext(PromptAssemblyInput{Surface: ProviderContextSurfaceGoalEvaluation, Role: "cognitive_assessment", OperationRules: []string{goalEvaluationNativeInstruction}, CorePersona: mapValue(packet["persona"]), StableTaskContext: mapValue(packet["stable"]), WorkingMemory: working, CurrentInput: renderProviderJSON(current), Policy: DefaultPromptBudgetPolicy(1)})
	if err != nil {
		t.Fatal(err)
	}
	chars := 0
	for _, message := range assembled.Messages {
		chars += len([]rune(stringValue(message["content"])))
	}
	before := intValue(packet["before_chars"])
	if chars >= before || len(sources) != 32 {
		t.Fatalf("replay failed: before=%d after=%d sources=%d", before, chars, len(sources))
	}
	t.Logf("reconstructed sample content_chars before=%d after=%d reduction=%.1f%% sources=%d messages=%d shared_payloads=%d; Provider NOT_CALLED", before, chars, 100*(1-float64(chars)/float64(before)), len(sources), len(assembled.Messages), len(mapValue(current["evidence_payloads"])))
}
