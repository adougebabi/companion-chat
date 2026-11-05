package core

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestInitializationModelArraysRecoverPhysicalResponse(t *testing.T) {
	const description = "姓名：书店店主。她独居，喜欢阅读。"
	modelResult := map[string]any{"core_persona": map[string]any{
		"identity":     map[string]any{"name": "书店店主"},
		"life_profile": map[string]any{"relationship_seeds": []any{"独居"}, "life_habits": []any{"喜欢阅读"}},
	}}
	var captured map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{
			"message": map[string]any{"role": "assistant", "content": jsonString(modelResult)}, "finish_reason": "stop",
		}}})
	}))
	defer server.Close()
	app := &App{Provider: &ProviderClient{HTTP: server.Client()}}
	ctx := withFrozenProviderAssignment(context.Background(), "initialization", providerAssignment{Role: "initialization", BaseURL: server.URL, ModelID: "local-fixture", Timeout: time.Second})
	result, err := app.RunInitializationTask(ctx, InitializationTaskInput{Description: description})
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := prepareAnalyzedInitializationResponse(description, result)
	if err != nil {
		var failure *initializationAnalysisError
		if errors.As(err, &failure) {
			t.Fatalf("a malformed optional relationship item must not reject the model analysis: %s %s", failure.Path, failure.ValidationType)
		}
		t.Fatal(err)
	}
	life := mapValue(mapValue(prepared["core_persona"])["life_profile"])
	if len(arrayValue(life["relationship_seeds"])) != 0 || jsonString(life["life_habits"]) != `["喜欢阅读"]` {
		t.Fatalf("bad relationship should be isolated without losing the habit: %#v", life)
	}
	extensions := mapValue(prepared["extensions"])
	if mapValue(extensions["recovered_fields"])["core_persona.life_profile.relationship_seeds[0]"] != "独居" || len(arrayValue(extensions["initialization_warnings"])) == 0 {
		t.Fatalf("the malformed item must remain reviewable: %#v", extensions)
	}
	messages := arrayValue(captured["messages"])
	if len(messages) != 2 || mapValue(messages[1])["content"] != description || mapValue(captured["response_format"])["type"] != "json_object" || captured["max_tokens"] != nil || captured["max_completion_tokens"] != nil {
		t.Fatalf("physical request did not preserve the input and protocol: %#v", captured)
	}
	protocol := stringValue(mapValue(messages[0])["content"])
	for _, required := range []string{"Both core_persona.life_profile.relationship_seeds and initial_relationships", "target_actor_id:\"actor_user\"", "do not turn ordinary life facts into relationship objects", "Array field reference:"} {
		if !strings.Contains(protocol, required) {
			t.Errorf("physical model request is missing array guidance %q", required)
		}
	}
	marker := "Array field reference: "
	start := strings.Index(protocol, marker)
	if start < 0 {
		t.Fatal("physical model request has no schema-derived array reference")
	}
	var guide []map[string]any
	if err := json.NewDecoder(strings.NewReader(protocol[start+len(marker):])).Decode(&guide); err != nil {
		t.Fatalf("array field reference is not valid JSON: %v", err)
	}
	byPath := map[string]map[string]any{}
	for _, entry := range guide {
		byPath[stringValue(entry["path"])] = entry
	}
	for _, path := range []string{
		"core_persona.life_profile.relationship_seeds", "core_persona.identity.core_values",
		"core_persona.life_profile.appearance.wardrobe_items", "initial_goals", "initial_intentions", "initial_relationships",
		"core_persona.personality_system.profiles[].voice.speech_patterns", "core_persona.personality_system.profiles[].output_preferences.channels",
		"core_persona.personality_system.takeover_rules[].evidence_refs", "developing_self.claims[].evidence_refs",
	} {
		if _, exists := byPath[path]; !exists {
			t.Errorf("physical request omits array path %q", path)
		}
	}
	seedGuide := mapValue(byPath["core_persona.life_profile.relationship_seeds"]["items"])
	if seedGuide["type"] != "object" || jsonString(seedGuide["required"]) != `["target_actor_id"]` {
		t.Fatalf("physical request does not tell the model the seed item contract: %#v", seedGuide)
	}
	union := byPath["core_persona.personality_system.profiles[].behavior_loops"]
	if union["container"] != "object or array" {
		t.Fatalf("legal object-or-array alternative lost from model guidance: %#v", union)
	}
}

func TestInitializationModelArrayRecoveryPreservesValidDataAndReferences(t *testing.T) {
	goodSeed := map[string]any{"target_actor_id": "actor_user", "summary": "一起阅读"}
	goodGarment := map[string]any{"category": "top", "slot": "upper_body", "description": "白衬衫", "ownership": "owned", "available": true, "currently_worn": false}
	raw := map[string]any{
		"core_persona": map[string]any{
			"identity": map[string]any{"name": "林夏", "core_values": "诚实"},
			"life_profile": map[string]any{
				"life_habits": "晚间阅读", "character_constraints": []any{"不打扰", false, 0, nil},
				"relationship_seeds": []any{"独居", goodSeed, nil, map[string]any{"summary": "认识一位画家"}},
				"appearance":         map[string]any{"wardrobe_items": []any{goodGarment, "一件外套"}},
			},
			"personality_system": map[string]any{"profiles": []any{map[string]any{
				"id": "calm", "voice": map[string]any{"speech_patterns": "语速缓慢"},
				"behavior_loops":     map[string]any{"loops": "睡前阅读"},
				"output_preferences": map[string]any{"channels": []any{"text", 7}, "triggers": false},
			}}, "takeover_rules": map[string]any{"condition": "原始错误规则"}},
		},
		"initial_goals": []any{42, map[string]any{"description": "学习绘画"}, map[string]any{"description": "整理书架"}},
		"initial_intentions": []any{
			map[string]any{"action": "画一幅画", "goal_index": 1},
			map[string]any{"action": "分类书籍", "goal_index": 2},
			map[string]any{"action": "错误目标", "goal_index": 0},
		},
		"initial_relationships": []any{"一位朋友", map[string]any{"target_actor_id": "actor_user", "summary": "朋友"}},
		"extensions":            map[string]any{"custom": map[string]any{"core_values": "不要改动", "arbitrary_list": 42}, "recovered_fields": map[string]any{"core_persona.life_profile.relationship_seeds[0]": "旧记录"}},
	}
	before := jsonString(raw)
	prepared, err := prepareAnalyzedInitializationResponse("姓名：林夏，她喜欢阅读。", raw)
	if err != nil {
		t.Fatal(err)
	}
	if jsonString(raw) != before {
		t.Fatal("array repair mutated the model result")
	}
	persona := mapValue(prepared["core_persona"])
	life := mapValue(persona["life_profile"])
	if jsonString(mapValue(persona["identity"])["core_values"]) != `["诚实"]` || jsonString(life["life_habits"]) != `["晚间阅读"]` || jsonString(life["character_constraints"]) != `["不打扰",false,0,null]` {
		t.Fatalf("valid free-form items were lost: %#v", persona)
	}
	if jsonString(life["relationship_seeds"]) != jsonString([]any{goodSeed}) {
		t.Fatalf("valid relationship was not retained: %#v", life["relationship_seeds"])
	}
	if len(arrayValue(mapValue(life["appearance"])["wardrobe_items"])) != 0 {
		t.Fatal("a malformed wardrobe must not shift the stable item indexes")
	}
	profiles := arrayValue(mapValue(persona["personality_system"])["profiles"])
	profile := mapValue(profiles[0])
	if jsonString(mapValue(profile["voice"])["speech_patterns"]) != `["语速缓慢"]` || jsonString(mapValue(profile["behavior_loops"])["loops"]) != `["睡前阅读"]` || jsonString(mapValue(profile["output_preferences"])["channels"]) != `["text"]` || jsonString(mapValue(profile["output_preferences"])["triggers"]) != `[false]` {
		t.Fatalf("nested array recovery lost valid profile content: %#v", profile)
	}
	goals, intentions := arrayValue(prepared["initial_goals"]), arrayValue(prepared["initial_intentions"])
	if len(goals) != 2 || len(intentions) != 2 || intValue(mapValue(intentions[0])["goal_index"]) != 0 || intValue(mapValue(intentions[1])["goal_index"]) != 1 {
		t.Fatalf("a removed goal changed an intention's target: goals=%#v intentions=%#v", goals, intentions)
	}
	if len(arrayValue(prepared["initial_relationships"])) != 1 {
		t.Fatalf("valid primary relationship was lost: %#v", prepared["initial_relationships"])
	}
	extensions := mapValue(prepared["extensions"])
	recovered := mapValue(extensions["recovered_fields"])
	for key, want := range map[string]any{
		"core_persona.life_profile.relationship_seeds[0]": "旧记录", "core_persona.life_profile.relationship_seeds[0]#2": "独居",
		"core_persona.life_profile.appearance.wardrobe_items": []any{goodGarment, "一件外套"},
		"core_persona.personality_system.takeover_rules":      map[string]any{"condition": "原始错误规则"},
		"initial_goals[0]": 42, "initial_intentions[2]": map[string]any{"action": "错误目标", "goal_index": 0},
	} {
		if jsonString(recovered[key]) != jsonString(want) {
			t.Errorf("recovered %s = %s, want %s", key, jsonString(recovered[key]), jsonString(want))
		}
	}
	if jsonString(extensions["custom"]) != `{"arbitrary_list":42,"core_values":"不要改动"}` || len(arrayValue(extensions["initialization_warnings"])) == 0 {
		t.Fatalf("open extension changed or warnings missing: %#v", extensions)
	}
}

func TestInitializationModelArrayRecoveryDoesNotManufactureACharacter(t *testing.T) {
	value := map[string]any{"core_persona": map[string]any{"life_profile": map[string]any{"relationship_seeds": []any{"独居"}}}}
	_, err := prepareAnalyzedInitializationResponse("独居", value)
	var failure *initializationAnalysisError
	if !errors.As(err, &failure) || failure.Code != "initialization_response_semantic_empty" {
		t.Fatalf("isolated invalid content must not manufacture a character: %#v %v", failure, err)
	}
}

func TestInitializationModelArrayRecoveryProtectsProfileReferences(t *testing.T) {
	value := map[string]any{
		"core_persona": map[string]any{
			"identity": map[string]any{"name": "林夏"},
			"life_profile": map[string]any{"relationship_seeds": []any{
				map[string]any{"target_actor_id": "actor_user", "profile_id": "missing", "summary": "未知人格的关系"},
				map[string]any{"target_actor_id": "actor_user", "profile_id": "steady", "summary": "一起阅读"},
			}},
			"personality_system": map[string]any{
				"profiles":          []any{"坏人格", map[string]any{"id": "steady"}, map[string]any{"id": "other"}},
				"active_profile_id": "missing",
				"takeover_rules":    []any{map[string]any{"id": "rule", "kind": "turn_takeover", "version": "turn-takeover.v1", "condition": "请求帮助", "target_profile_id": "missing"}},
			},
		},
		"initial_goals":      []any{map[string]any{"description": "不明确目标", "profile_id": "missing"}, map[string]any{"description": "阅读", "profile_id": "steady"}},
		"initial_intentions": []any{map[string]any{"action": "翻开书", "profile_id": "steady", "goal_index": 1}},
	}
	prepared, err := prepareAnalyzedInitializationResponse("姓名：林夏，她喜欢阅读。", value)
	if err != nil {
		t.Fatal(err)
	}
	persona := mapValue(prepared["core_persona"])
	system := mapValue(persona["personality_system"])
	if system["active_profile_id"] != "default" || len(arrayValue(system["profiles"])) != 2 || len(arrayValue(system["takeover_rules"])) != 0 {
		t.Fatalf("invalid profile was adopted or a rule was fabricated: %#v", system)
	}
	seeds := arrayValue(mapValue(persona["life_profile"])["relationship_seeds"])
	goals, intentions := arrayValue(prepared["initial_goals"]), arrayValue(prepared["initial_intentions"])
	if len(seeds) != 1 || mapValue(seeds[0])["profile_id"] != "steady" || len(goals) != 1 || len(intentions) != 1 || intValue(mapValue(intentions[0])["goal_index"]) != 0 {
		t.Fatalf("valid scoped facts were lost or rebound: seeds=%#v goals=%#v intentions=%#v", seeds, goals, intentions)
	}
	recovered := mapValue(mapValue(prepared["extensions"])["recovered_fields"])
	for _, path := range []string{"core_persona.personality_system.profiles[0]", "core_persona.personality_system.active_profile_id", "core_persona.personality_system.takeover_rules[0]", "core_persona.life_profile.relationship_seeds[0]", "initial_goals[0]"} {
		if _, ok := recovered[path]; !ok {
			t.Errorf("missing preserved profile-related source %s", path)
		}
	}
}

func TestInitializationModelArrayRecoveryHandlesMalformedContainersAndAliases(t *testing.T) {
	value := map[string]any{
		"core_persona": map[string]any{
			"identity":     map[string]any{"name": "林夏", "values": "善良"},
			"life_profile": map[string]any{"relationship_seeds": nil, "wardrobe_items": []any{map[string]any{"description": "衣服", "available": "yes"}}},
		},
		"relationships": []any{"独居"},
		"initial_goals": "读完一本书", "initial_intentions": "阅读",
	}
	prepared, err := prepareAnalyzedInitializationResponse("姓名：林夏，她想读完一本书。", value)
	if err != nil {
		t.Fatal(err)
	}
	persona := mapValue(prepared["core_persona"])
	life := mapValue(persona["life_profile"])
	if jsonString(mapValue(persona["identity"])["core_values"]) != `["善良"]` || len(arrayValue(life["relationship_seeds"])) != 0 || len(arrayValue(mapValue(life["appearance"])["wardrobe_items"])) != 0 || len(arrayValue(prepared["initial_relationships"])) != 0 {
		t.Fatalf("malformed arrays or aliases were not safely recovered: %#v", prepared)
	}
	goals, intentions := arrayValue(prepared["initial_goals"]), arrayValue(prepared["initial_intentions"])
	if len(goals) != 1 || mapValue(goals[0])["description"] != "读完一本书" || len(intentions) != 1 || mapValue(intentions[0])["action"] != "阅读" || intValue(mapValue(intentions[0])["goal_index"]) != 0 {
		t.Fatalf("unambiguous goal and intention text were lost: %#v %#v", goals, intentions)
	}
	recovered := mapValue(mapValue(prepared["extensions"])["recovered_fields"])
	for _, path := range []string{"core_persona.identity.values", "core_persona.life_profile.relationship_seeds", "core_persona.life_profile.wardrobe_items", "relationships[0]", "initial_goals", "initial_intentions"} {
		if _, ok := recovered[path]; !ok {
			t.Errorf("missing original value for %s", path)
		}
	}
}

func TestInitializationModelArrayRecoveryHandlesRootPersonaAliases(t *testing.T) {
	value := map[string]any{
		"identity":     map[string]any{"name": "林夏", "values": "真诚"},
		"life_profile": map[string]any{"relationship_seeds": []any{"独居"}, "life_habits": "阅读"},
	}
	before := jsonString(value)
	prepared, err := prepareAnalyzedInitializationResponse("姓名：林夏，她独居并喜欢阅读。", value)
	if err != nil {
		t.Fatal(err)
	}
	persona := mapValue(prepared["core_persona"])
	life := mapValue(persona["life_profile"])
	if jsonString(value) != before || jsonString(mapValue(persona["identity"])["core_values"]) != `["真诚"]` || jsonString(life["life_habits"]) != `["阅读"]` || len(arrayValue(life["relationship_seeds"])) != 0 {
		t.Fatalf("root aliases did not preserve and recover the same source: %#v", prepared)
	}
	if mapValue(mapValue(prepared["extensions"])["recovered_fields"])["core_persona.life_profile.relationship_seeds[0]"] != "独居" {
		t.Fatal("root-level relationship alias lost its original value")
	}
}

func TestInitializationModelArrayRecoveryKeepsConflictingValueAlias(t *testing.T) {
	value := map[string]any{"core_persona": map[string]any{"identity": map[string]any{
		"name": "林夏", "core_values": []any{"诚实"}, "values": []any{"善良"},
	}}}
	prepared, err := prepareAnalyzedInitializationResponse("姓名：林夏。", value)
	if err != nil {
		t.Fatal(err)
	}
	identity := mapValue(mapValue(prepared["core_persona"])["identity"])
	if jsonString(identity["core_values"]) != `["诚实"]` {
		t.Fatalf("an alias replaced the canonical values: %#v", identity)
	}
	recovered := mapValue(mapValue(prepared["extensions"])["recovered_fields"])
	if jsonString(recovered["core_persona.identity.values"]) != `["善良"]` {
		t.Fatalf("conflicting alias was not preserved: %#v", recovered)
	}
}

func TestInitializationModelArrayRecoveryWarnsWhenOriginalAlreadyPreserved(t *testing.T) {
	value := map[string]any{
		"core_persona": map[string]any{"identity": map[string]any{"name": "林夏", "core_values": "诚实"}},
		"extensions":   map[string]any{"recovered_fields": map[string]any{"core_persona.identity.core_values": "诚实"}},
	}
	prepared, err := prepareAnalyzedInitializationResponse("姓名：林夏，她重视诚实。", value)
	if err != nil {
		t.Fatal(err)
	}
	extensions := mapValue(prepared["extensions"])
	if mapValue(extensions["recovered_fields"])["core_persona.identity.core_values"] != "诚实" {
		t.Fatal("existing original value changed")
	}
	for _, raw := range arrayValue(extensions["initialization_warnings"]) {
		if mapValue(raw)["path"] == "core_persona.identity.core_values" {
			return
		}
	}
	t.Fatal("duplicate stored source suppressed the current recovery warning")
}

func TestInitializationModelArrayRecoveryRejectsAmbiguousGoalLinks(t *testing.T) {
	value := map[string]any{
		"core_persona":  map[string]any{"identity": map[string]any{"name": "林夏"}},
		"initial_goals": []any{map[string]any{"id": "same", "description": "学习绘画"}, map[string]any{"id": "same", "description": "整理书架"}, map[string]any{"id": "clear", "description": "阅读"}},
		"initial_intentions": []any{
			map[string]any{"action": "画画", "linked_goal_id": "same"},
			map[string]any{"action": "写字", "linked_goal_id": "missing"},
			map[string]any{"action": "翻开书", "linked_goal_id": "clear"},
		},
	}
	prepared, err := prepareAnalyzedInitializationResponse("姓名：林夏。", value)
	if err != nil {
		t.Fatal(err)
	}
	intentions := arrayValue(prepared["initial_intentions"])
	if len(intentions) != 1 || mapValue(intentions[0])["action"] != "翻开书" || intValue(mapValue(intentions[0])["goal_index"]) != 2 {
		t.Fatalf("an ambiguous linked goal was silently selected: %#v", intentions)
	}
	recovered := mapValue(mapValue(prepared["extensions"])["recovered_fields"])
	for _, path := range []string{"initial_intentions[0]", "initial_intentions[1]"} {
		if _, ok := recovered[path]; !ok {
			t.Errorf("ambiguous intention not retained: %s", path)
		}
	}
}

func TestInitializationModelArrayRecoveryRejectsAmbiguousProfileLinks(t *testing.T) {
	value := map[string]any{
		"core_persona": map[string]any{
			"identity": map[string]any{"name": "林夏"},
			"personality_system": map[string]any{
				"profiles":          []any{map[string]any{"id": "same", "name": "甲"}, map[string]any{"id": "same", "name": "乙"}, map[string]any{"id": "clear", "name": "丙"}},
				"active_profile_id": "same",
				"takeover_rules":    []any{map[string]any{"id": "r", "kind": "turn_takeover", "version": "turn-takeover.v1", "condition": "请求帮助", "target_profile_id": "same"}},
			},
		},
		"initial_relationships": []any{map[string]any{"target_actor_id": "actor_user", "profile_id": "same", "summary": "乙与用户"}, map[string]any{"target_actor_id": "actor_user", "profile_id": "clear", "summary": "丙与用户"}},
		"initial_goals":         []any{map[string]any{"description": "不明确目标", "profile_id": "same"}, map[string]any{"description": "阅读", "profile_id": "clear"}},
		"initial_intentions":    []any{map[string]any{"action": "翻开书", "profile_id": "clear", "goal_index": 1}},
	}
	prepared, err := prepareAnalyzedInitializationResponse("姓名：林夏。", value)
	if err != nil {
		t.Fatal(err)
	}
	system := mapValue(mapValue(prepared["core_persona"])["personality_system"])
	if system["active_profile_id"] != "default" || len(arrayValue(system["takeover_rules"])) != 0 {
		t.Fatalf("ambiguous profile was selected or given an executable rule: %#v", system)
	}
	relationships, intentions := arrayValue(prepared["initial_relationships"]), arrayValue(prepared["initial_intentions"])
	if len(relationships) != 1 || mapValue(relationships[0])["profile_id"] != "clear" || len(intentions) != 1 || intValue(mapValue(intentions[0])["goal_index"]) != 0 {
		t.Fatalf("ambiguous profile reference was rebound: %#v %#v", relationships, intentions)
	}
	recovered := mapValue(mapValue(prepared["extensions"])["recovered_fields"])
	for _, path := range []string{"core_persona.personality_system.profiles", "core_persona.personality_system.active_profile_id", "core_persona.personality_system.takeover_rules[0]", "initial_relationships[0]", "initial_goals[0]"} {
		if _, ok := recovered[path]; !ok {
			t.Errorf("ambiguous profile source not retained: %s", path)
		}
	}
}

func TestInitializationModelArrayRecoveryCoversDeclaredArrayPaths(t *testing.T) {
	guide := initializationArraySchemaGuide()
	if len(guide) < 41 {
		t.Fatalf("initialization schema unexpectedly has only %d array paths", len(guide))
	}
	for _, raw := range guide {
		entry := mapValue(raw)
		path := stringValue(entry["path"])
		t.Run(path, func(t *testing.T) {
			root := map[string]any{"core_persona": map[string]any{"identity": map[string]any{"name": "数组测试"}}}
			parent := root
			parts := strings.Split(path, ".")
			for index, part := range parts {
				name := strings.TrimSuffix(part, "[]")
				if index == len(parts)-1 {
					parent[name] = false // Always wrong as an array container.
					break
				}
				if strings.HasSuffix(part, "[]") {
					child := map[string]any{}
					if name == "profiles" {
						child["id"] = "profile-one"
					}
					if name == "initial_relationships" {
						child["target_actor_id"] = "actor_user"
					}
					parent[name] = []any{child}
					parent = child
					continue
				}
				child, ok := parent[name].(map[string]any)
				if !ok {
					child = map[string]any{}
					parent[name] = child
				}
				parent = child
			}
			repaired := recoverAnalyzedInitializationArrays(root)
			actualPath := strings.ReplaceAll(path, "[]", "[0]")
			recovered := mapValue(mapValue(repaired["extensions"])["recovered_fields"])
			if saved, ok := recovered[actualPath]; !ok || saved != false {
				t.Errorf("invalid container %s not preserved: %#v", actualPath, recovered)
			}
			var current any = repaired
			for _, part := range parts {
				name := strings.TrimSuffix(part, "[]")
				current = mapValue(current)[name]
				if strings.HasSuffix(part, "[]") {
					items := arrayValue(current)
					if len(items) == 0 {
						t.Fatalf("missing parent item for %s", actualPath)
					}
					current = items[0]
				}
			}
			if _, ok := current.([]any); !ok {
				t.Errorf("invalid container %s was not replaced by an array: %#v", actualPath, current)
			}
		})
	}
}
