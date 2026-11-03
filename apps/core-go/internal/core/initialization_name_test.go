package core

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPrepareAnalyzedInitializationResponsePreservesExplicitName(t *testing.T) {
	for _, item := range []struct {
		name, description, analyzedName, nickname, want string
	}{
		{"reported_split", "名字叫FT-林夏，24岁，独立插画师。", "林夏", "FT-林夏", "FT-林夏"},
		{"leading_card", "FT-林夏，24岁，女性，独立插画师。", "林夏", "FT-林夏", "FT-林夏"},
		{"leading_without_nickname", "FT-林夏，喜欢猫和雨天。", "林夏", "", "FT-林夏"},
		{"labeled_name", "姓名：FT-林夏；昵称：小夏。", "林夏", "小夏", "FT-林夏"},
		{"quoted_name", "她的名字是「FT-林夏」，24岁。", "林夏", "小夏", "FT-林夏"},
		{"create_request", "请创建一个名为FT-林夏，24岁的角色。", "林夏", "FT-林夏", "FT-林夏"},
		{"natural_create_request", "请创建一个名为FT-林夏的角色。", "林夏", "FT-林夏", "FT-林夏"},
		{"called", "她叫FT-林夏，喜欢画画。", "林夏", "FT-林夏", "FT-林夏"},
		{"english_hyphen", "Name: Anne-Marie, an illustrator.", "Marie", "Annie", "Anne-Marie"},
		{"leading_english_hyphen", "Anne-Marie, an illustrator.", "Anne", "", "Anne-Marie"},
		{"quoted_spaces", "名字：\"FT-Anne Marie\"，喜欢画画。", "Marie", "Annie", "FT-Anne Marie"},
		{"unicode_dash", "FT－林夏，独立插画师。", "林夏", "", "FT－林夏"},
		{"multiple_prefixes", "姓名：FT-QA-林夏，独立插画师。", "林夏", "", "FT-QA-林夏"},
		{"nickname_is_separate", "姓名：林夏，昵称：FT-林夏。", "林夏", "FT-林夏", "林夏"},
		{"nickname_only", "昵称：FT-林夏，喜欢画画。", "林夏", "FT-林夏", "林夏"},
		{"no_name", "创建一个温柔慢热、喜欢猫的独立插画师。", "林夏", "FT-林夏", "林夏"},
		{"owner_name", "我的名字叫FT-用户，角色是一位插画师。", "林夏", "FT-林夏", "林夏"},
		{"friend_name", "她的朋友名为FT-小雨，她喜欢和朋友画画。", "林夏", "FT-林夏", "林夏"},
		{"incidental_mention", "她喜欢FT-林夏的画作。", "林夏", "FT-林夏", "林夏"},
		{"called_is_not_naming", "她叫我不要迟到，喜欢画画。", "林夏", "FT-林夏", "林夏"},
		{"naming_clause_with_prose", "她叫FT-林夏今年24岁，喜欢画画。", "林夏", "FT-林夏", "林夏"},
		{"conflicting_declarations", "姓名：FT-林夏。名字：FT-小雨。", "林夏", "FT-林夏", "林夏"},
	} {
		t.Run(item.name, func(t *testing.T) {
			raw := map[string]any{"core_persona": map[string]any{
				"identity":           map[string]any{"name": item.analyzedName, "nickname": item.nickname, "occupation": "独立插画师"},
				"personality_system": defaultPersonalitySystem(),
			}}
			before := jsonString(raw)
			prepared, err := prepareAnalyzedInitializationResponse(item.description, raw)
			if err != nil {
				t.Fatal(err)
			}
			identity := mapValue(mapValue(prepared["core_persona"])["identity"])
			if identity["name"] != item.want || identity["nickname"] != item.nickname || identity["occupation"] != "独立插画师" {
				t.Fatalf("identity = %#v; want name %q with other fields preserved", identity, item.want)
			}
			if !validInitialization(prepared) || jsonString(raw) != before {
				t.Fatal("correction invalidated initialization or mutated the Provider result")
			}
		})
	}
}

func TestPrepareAnalyzedInitializationResponseRejectsEmptyProviderResult(t *testing.T) {
	if _, err := prepareAnalyzedInitializationResponse("姓名：FT-林夏", map[string]any{}); err == nil {
		t.Fatal("an explicit name must not turn an empty Provider result into an accepted persona")
	}
}

func TestInitializationExplicitNameRequestAndCorrection(t *testing.T) {
	const description = "姓名：FT-林夏，24岁，独立插画师，昵称：小夏。"
	var captured map[string]any
	// Reproduce the Provider's reported split, regardless of the improved
	// instruction, so the deterministic correction is exercised as well.
	raw := map[string]any{"core_persona": defaultCorePersona("", "林夏")}
	mapValue(mapValue(raw["core_persona"])["identity"])["nickname"] = "FT-林夏"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{
			"message": map[string]any{"role": "assistant", "content": jsonString(raw)}, "finish_reason": "stop",
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
		t.Fatal(err)
	}
	if got := mapValue(mapValue(prepared["core_persona"])["identity"])["name"]; got != "FT-林夏" {
		t.Fatalf("analyzed identity.name = %v", got)
	}
	messages := arrayValue(captured["messages"])
	if len(messages) != 2 || mapValue(messages[1])["role"] != "user" || mapValue(messages[1])["content"] != description {
		t.Fatalf("Owner description must reach the Provider once and verbatim: %#v", messages)
	}
	protocol := stringValue(mapValue(messages[0])["content"])
	for _, rule := range []string{"complete character name verbatim in core_persona.identity.name", "Never split or strip a name's prefix or hyphenated parts", "nickname only for a separately supplied nickname"} {
		if !strings.Contains(protocol, rule) {
			t.Fatalf("Provider request missing name rule %q", rule)
		}
	}
	if captured["max_tokens"] != nil || captured["max_completion_tokens"] != nil {
		t.Fatal("initialization request must not set output token limits")
	}
}
