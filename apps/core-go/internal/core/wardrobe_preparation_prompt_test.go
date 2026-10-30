package core

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestWardrobePreparationPromptProjectsTaskData(t *testing.T) {
	item := map[string]any{"id": "actual-top", "description": "唯一衣物描述", "slot": "upper_body", "clothing_type": "upper_body", "seasons": []any{}, "ownership": "owned", "availability": "available", "source_verified": true, "revision": 7, "source_ref": "private-source"}
	sections := map[string]any{
		"identity":           map[string]any{"name": "测试角色", "occupation": "书店店主", "birthday": "private-birthday"},
		"profile.voice":      map[string]any{"greeting": "private-greeting"},
		"current_appearance": map[string]any{"wardrobe_revision": 4, "wearing_state": "known", "body_fields": map[string]any{"injuries": map[string]any{"status": "unknown"}}, "worn_items": []any{item}},
	}
	life := map[string]any{"instant": "2026-10-30T01:36:26.918+00:00", "timezone": "Asia/Shanghai", "scene": nil, "activity": nil, "location": nil, "authority_status": "pending", "world_id": "private-world", "context_revision": "private-life"}
	plans := []map[string]any{{"id": "private-outfit", "name": "测试搭配", "tags": []any{"在家"}, "items": []any{item}}}
	messages := wardrobePreparationMessages(nil, sections, life, []map[string]any{item}, plans, true)
	input := decodeObject([]byte(stringValue(messages[1]["content"])))
	if got := mapValue(input["current_life"])["current_time"]; got != "2026-10-30T09:36:26.918+08:00" {
		t.Errorf("wrong local clock: %v", got)
	}
	text := jsonString(messages)
	if strings.Contains(text, "private-") || strings.Count(text, "唯一衣物描述") != 1 {
		t.Error("metadata leaked or clothing repeated")
	}
	if _, present := input["starter_items_allowed"]; present {
		t.Error("selection includes generation-only permission")
	}
	if !strings.Contains(stringValue(messages[0]["content"]), "outer 不能替代 upper_body") {
		t.Error("protocol omits actual body coverage rule")
	}
	if _, exists := mapValue(input["current_life"])["location"]; !exists {
		t.Error("explicit unknown location lost")
	}
	if refs := decisionServiceRefValues(mapValue(arrayValue(input["plans"])[0])["item_refs"]); !reflect.DeepEqual(refs, []string{"actual-top"}) {
		t.Fatal("executable wardrobe reference lost", refs)
	}
}

func TestWardrobePreparationPromptKeepsPreferencesAndInvalidPlanMembers(t *testing.T) {
	preference := map[string]any{"fabric": "喜欢棉质，避开粗羊毛", "version": json.Number("9007199254740993"), "properties": map[string]any{"item_ids": false}, "price": 0}
	foundation := map[string]any{"life_profile": map[string]any{"appearance": map[string]any{"style_preferences": preference, "daily_outfit_preferences": []any{"宽松长裤"}, "injuries": "private-historical-injury", "wardrobe_items": []any{"private-historical-stock"}}}}
	current := map[string]any{"body_fields": map[string]any{"injuries": map[string]any{"status": "known", "value": "当前脚踝不适"}}, "wearing_state": "unknown", "worn_items": []any{map[string]any{"id": "outside-stock", "description": "当前记录", "slot": "top", "availability": "available", "ownership": "owned", "source_verified": true}}}
	sections := map[string]any{"current_appearance": current, "profile.style_preferences": map[string]any{"fit": "舒适"}, "extensions": map[string]any{"core_persona.system_prompt": "旧卡原文，喜欢莫兰迪色；每句加括号。", "image_prompt": "private-duplicate-image", "mbti": "private-irrelevant-type", "core_persona.life_profile.appearance.style_preferences": map[string]any{"color": "青绿"}}}
	sharedPreferences := map[string]any{"clothing": "拒绝皮革", "allow_leather": false, "quota": 0, "version": json.Number("9007199254740993")}
	profilePreferences := map[string]any{"clothing": "不穿裙子", "allow_skirts": false, "quota": 0, "version": json.Number("9007199254740993")}
	sections["life_profile"] = map[string]any{"preferences": sharedPreferences, "character_constraints": []any{"服饰不得使用动物皮毛"}}
	sections["profile.preferences"] = profilePreferences
	sections["profile.character_constraints"] = []any{"只穿平底鞋"}
	items := []map[string]any{{"id": "dress", "slot": "full_body", "availability": "available", "ownership": "borrowed", "source_verified": true}}
	plans := []map[string]any{{"name": "含旧成员", "tags": []any{"日常"}, "items": []any{map[string]any{"id": "dress", "definition_slot_matches": false}, map[string]any{"id": "archived", "description": "private-archived-description"}}}}
	before := jsonString([]any{foundation, sections, items, plans})
	for _, selectOnly := range []bool{false, true} {
		messages := wardrobePreparationMessages(foundation, sections, map[string]any{}, items, plans, selectOnly)
		var input map[string]any
		decoder := json.NewDecoder(strings.NewReader(stringValue(messages[1]["content"])))
		decoder.UseNumber()
		if err := decoder.Decode(&input); err != nil {
			t.Fatal(err)
		}
		persona := mapValue(input["persona"])
		if jsonString(mapValue(persona["preferences"])["style_preferences"]) != jsonString(preference) || !reflect.DeepEqual(mapValue(input["current_appearance"])["body_fields"], current["body_fields"]) {
			t.Fatal("preference or current body data changed")
		}
		if jsonString(persona["general_preferences"]) != jsonString(sharedPreferences) || jsonString(persona["profile_preferences"]) != jsonString(profilePreferences) {
			t.Fatal("shared or selected profile clothing constraint lost")
		}
		if jsonString(persona["character_constraints"]) != jsonString(mapValue(sections["life_profile"])["character_constraints"]) || jsonString(persona["profile_character_constraints"]) != jsonString(sections["profile.character_constraints"]) {
			t.Fatal("shared or selected profile character constraint lost")
		}
		if strings.Contains(jsonString(messages), "private-") || strings.Count(jsonString(input), "旧卡原文") != 1 || strings.Contains(stringValue(messages[0]["content"]), "旧卡原文") {
			t.Fatal("irrelevant persona data leaked or legacy text promoted")
		}
		members := mapValue(arrayValue(input["plans"])[0])
		if !reflect.DeepEqual(decisionServiceRefValues(members["unusable_item_refs"]), []string{"archived", "dress"}) {
			t.Fatal("invalid plan silently became selectable", members)
		}
		catalog := arrayValue(input["registered_items"])
		if len(catalog) != 2 || mapValue(catalog[1])["selectable"] != false {
			t.Fatal("unregistered current wear became selectable", catalog)
		}
		if !selectOnly && input["starter_items_allowed"] != false {
			t.Fatal("existing full-body stock incorrectly allows starter stock")
		}
	}
	if got := jsonString([]any{foundation, sections, items, plans}); got != before {
		t.Fatal("projection mutated its source")
	}
}

func TestWardrobePreparationLifePromptViewPreservesKnownWeather(t *testing.T) {
	weather := map[string]any{"rain": true, "temperature": 0, "observation_id": "domain-observation", "version": 2}
	life := map[string]any{
		"instant": "2026-10-30T01:36:26.918+00:00", "timezone": "invalid-zone", "location": nil,
		"presence":      map[string]any{"id": "private-presence", "current_task": "户外散步", "user_presence": nil},
		"world_effects": []any{map[string]any{"run_id": "private-run", "event_id": "private-event", "description": "已知天气", "start_at": "2026-10-30T01:00:00Z", "end_at": "2026-10-30T02:00:00Z", "effects": []any{map[string]any{"kind": "weather", "scope": "world", "value": weather}}}},
	}
	view := wardrobeLifePromptView(life)
	if view["timezone"] != "UTC" || view["current_time"] != "2026-10-30T01:36:26.918+00:00" || strings.Contains(jsonString(view), "private-") {
		t.Fatal("invalid timezone guessed a local time or leaked provenance")
	}
	event := mapValue(arrayValue(view["world_effects"])[0])
	effect := mapValue(arrayValue(event["effects"])[0])
	if !reflect.DeepEqual(effect["value"], weather) || event["start_at"] != "2026-10-30T01:00:00Z" || event["end_at"] != "2026-10-30T02:00:00Z" || mapValue(view["presence"])["current_task"] != "户外散步" {
		t.Fatal("known weather/validity/current activity lost")
	}
	if _, exists := wardrobeLifePromptView(map[string]any{})["current_time"]; exists {
		t.Fatal("unknown clock was replaced by wall clock")
	}
}

func TestWardrobePreparationPromptPhysicalRequest(t *testing.T) {
	for _, selectOnly := range []bool{false, true} {
		t.Run(map[bool]string{false: "generate", true: "select"}[selectOnly], func(t *testing.T) {
			captured := make(chan map[string]any, 1)
			router := newFakeProviderRouter().on("wardrobe_preparation_v1", func(payload map[string]any) fakeProviderResult {
				captured <- payload
				return fakeProviderResult{Structured: map[string]any{"items": []any{}, "plans": []any{}, "wear_item_refs": []any{}, "reason": "受控传输测试"}}
			})
			life := map[string]any{"instant": "2026-10-30T01:36:26.918+00:00", "timezone": "Asia/Shanghai", "revision": 7, "context_revision": "private-life"}
			messages := wardrobePreparationMessages(nil, nil, life, nil, nil, selectOnly)
			ctx := WithADKCapabilityInvoker(context.Background(), &adkFakeInvoker{}, &ADKCapabilityTrace{})
			_, err := (&ProviderClient{HTTP: &http.Client{Transport: router}}).generateWithEino(ctx, EinoModelCall{
				Assignment: providerAssignment{Role: "cognitive_assessment", BaseURL: "http://wardrobe.invalid", ModelID: "fake", Timeout: 10 * time.Second, TokenBudget: 1},
				Role:       "cognitive_assessment", Scenario: "wardrobe_preparation", Messages: formalPromptMessages(FormalAgentWardrobePreparation, messages),
				JSONMode: true, SchemaName: "wardrobe_preparation_v1", ResponseSchema: wardrobePreparationSchema(), ProviderRequestID: "wardrobe-prompt-test",
			})
			if err != nil {
				t.Fatal(err)
			}
			wire := <-captured
			for _, key := range []string{"max_tokens", "max_completion_tokens", "tools"} {
				if _, exists := wire[key]; exists {
					t.Error("unexpected wire parameter", key)
				}
			}
			physical := arrayValue(wire["messages"])
			if len(physical) != 2 || strings.Count(jsonString(physical), "# 运行协议") != 1 || strings.Contains(jsonString(physical), "private-") || wire["response_format"] == nil {
				t.Fatal("physical protocol/structured transport changed")
			}
			input := decodeObject([]byte(stringValue(mapValue(physical[1])["content"])))
			if input["select_only"] != selectOnly || mapValue(input["current_life"])["current_time"] != "2026-10-30T09:36:26.918+08:00" {
				t.Fatal("physical task input not projected", input)
			}
			system := stringValue(mapValue(physical[0])["content"])
			if selectOnly && strings.Contains(system, "新增 items 最多") || !selectOnly && strings.Contains(system, "将所有选中衣物的真实 id") {
				t.Error("other stage instruction leaked")
			}
		})
	}
}

// Opt-in reconstruction of a private capture, never a checked-in fixture or a
// live model-quality claim. Report aggregate sizes only.
func TestWardrobePreparationPromptPrivateCapture(t *testing.T) {
	path := os.Getenv("GO_CORE_WARDROBE_PROMPT_CAPTURE")
	if path == "" {
		t.Skip("private wardrobe capture not configured")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	start := strings.Index(string(raw), "[")
	if start < 0 {
		t.Fatal("capture has no request")
	}
	var request []map[string]any
	if err := json.NewDecoder(strings.NewReader(string(raw[start:]))).Decode(&request); err != nil || len(request) != 2 {
		t.Fatal("invalid capture request", err)
	}
	input := decodeObject([]byte(stringValue(request[1]["content"])))
	var items, plans []map[string]any
	for _, value := range arrayValue(input["registered_items"]) {
		items = append(items, mapValue(value))
	}
	for _, value := range arrayValue(input["plans"]) {
		plans = append(plans, mapValue(value))
	}
	messages := wardrobePreparationMessages(nil, mapValue(input["persona"]), mapValue(input["current_life"]), items, plans, input["select_only"] == true)
	before, after := 0, 0
	for _, message := range request {
		before += len([]rune(stringValue(message["content"])))
	}
	for _, message := range messages {
		after += len([]rune(stringValue(message["content"])))
	}
	projected := decodeObject([]byte(stringValue(messages[1]["content"])))
	if after >= before || len(arrayValue(projected["registered_items"])) != len(items) || len(arrayValue(projected["plans"])) != len(plans) {
		t.Fatal("private capture projection lost stock/plans or did not shrink")
	}
	// The supplied response omitted upper_body. Prove the production validator
	// rejects that exact recorded selection; a declared outer cannot cover it.
	responseStart := strings.LastIndex(string(raw), "\nresponse")
	var response map[string]any
	if responseStart < 0 || json.Unmarshal(raw[responseStart+len("\nresponse"):], &response) != nil {
		t.Fatal("invalid capture response")
	}
	output := decodeObject([]byte(stringValue(response["content"])))
	selected := map[string]bool{}
	for _, id := range decisionServiceRefValues(output["wear_item_refs"]) {
		selected[id] = true
	}
	chosen := []map[string]any{}
	for _, item := range items {
		if selected[stringValue(item["id"])] {
			chosen = append(chosen, item)
		}
	}
	if len(chosen) != len(selected) || wardrobeCoversBody(chosen) {
		t.Fatal("capture does not reproduce incomplete body coverage")
	}
	for _, item := range items {
		if item["slot"] == "upper_body" {
			chosen = append(chosen, item)
			break
		}
	}
	if !usableWardrobeCoverage(chosen) {
		t.Fatal("adding recorded usable upper_body did not restore coverage")
	}
	t.Logf("reconstructed message content_chars before=%d after=%d reduction=%.1f%% items=%d plans=%d; Provider NOT_CALLED", before, after, 100*(1-float64(after)/float64(before)), len(items), len(plans))
}
