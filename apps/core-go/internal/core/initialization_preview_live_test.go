package core

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"
)

// This opt-in test exercises the production task and a public synthetic card.
// Unlike the controlled HTTP test, its result depends on the configured model.
func TestLiveProviderPreviewFieldsInitialization(t *testing.T) {
	baseURL, model := liveProviderConfig(t)
	const description = "角色姓名林夏，女，26岁，住在上海，是一名插画师。她有乌黑长发和棕色眼睛；父母都是教师，她在温暖的家庭中长大。她喜欢绿茶，每天晚上散步。她拥有一件白色亚麻衬衫，今天没有穿它。她的共同性格数值：开放性0.70、共情0.85；表达：消息短（short），表情频率0（从不用表情），调侃倾向0.20，回复延迟45秒。没有提供她的血型或出生地。"
	manifest := InitializationExpectationManifest{
		Classification: "single",
		Assertions: []InitializationSemanticAssertion{
			{ID: "name", Path: "core_persona.identity.name", Contains: "林夏", Basis: "explicit"},
			{ID: "appearance", Path: "core_persona.life_profile.appearance.description", ContainsAny: []string{"乌黑", "黑发", "长发"}, Basis: "explicit"},
			{ID: "family", Path: "core_persona.life_profile.social_background", Contains: "教师", Basis: "explicit"},
			{ID: "preference", Path: "core_persona.life_profile.preferences", Contains: "绿茶", Basis: "explicit"},
			{ID: "habit", Path: "core_persona.life_profile.life_habits", Contains: "散步", Basis: "explicit"},
			{ID: "wardrobe", Path: "core_persona.life_profile.appearance.wardrobe_items[0]", Contains: "衬衫", Basis: "explicit"},
			{ID: "openness", Path: "core_persona.personality.openness", Contains: "0.7", Basis: "explicit"},
			{ID: "empathy", Path: "core_persona.personality.empathy", Contains: "0.85", Basis: "explicit"},
			{ID: "message_length", Path: "core_persona.behavioral_policy.message_length", Contains: "short", Basis: "explicit"},
			{ID: "emoji_frequency", Path: "core_persona.behavioral_policy.emoji_frequency", Contains: "0", Basis: "explicit"},
			{ID: "sarcasm", Path: "core_persona.behavioral_policy.sarcasm_tendency", Contains: "0.2", Basis: "explicit"},
			{ID: "response_delay", Path: "core_persona.behavioral_policy.response_delay", Contains: "45", Basis: "explicit"},
		},
		Forbidden: []InitializationInventionGuard{
			{ID: "blood_type", Path: "core_persona.identity.blood_type"},
			{ID: "birthplace", Path: "core_persona.identity.birthplace"},
		},
	}
	app := &App{Provider: &ProviderClient{HTTP: &http.Client{Timeout: initializationMinimumRequestTimeout}}}
	ctx, cancel := context.WithTimeout(context.Background(), initializationMinimumRequestTimeout)
	defer cancel()
	ctx = withFrozenProviderAssignment(ctx, "initialization", providerAssignment{
		Role: "initialization", BaseURL: baseURL, ModelID: model,
		Secret: strings.TrimSpace(os.Getenv("FLUCTLIGHT_LIVE_PROVIDER_API_KEY")), Timeout: initializationMinimumRequestTimeout,
	})
	result, err := app.RunInitializationTask(ctx, InitializationTaskInput{Description: description})
	if err != nil {
		t.Fatalf("live initialization task failed: %v", err)
	}
	rawLife := mapValue(mapValue(result["core_persona"])["life_profile"])
	_, rawCanonical := mapValue(rawLife["appearance"])["wardrobe_items"].([]any)
	_, rawAlias := rawLife["wardrobe_items"].([]any)
	t.Logf("public fixture wardrobe paths before normalization: canonical=%t life_alias=%t", rawCanonical, rawAlias)
	prepared, err := prepareAnalyzedInitializationResponse(description, result)
	if err != nil {
		t.Fatalf("live initialization result rejected: %v", err)
	}
	report := EvaluateInitializationCoverage(prepared, manifest)
	for _, item := range report.Items {
		if item.Category != "preserved" {
			t.Errorf("live initialization %s at %s: %s", item.ID, item.Path, item.Category)
			if item.ID == "wardrobe" {
				t.Logf("public fixture wardrobe locations: raw appearance=%s; raw life wardrobe=%s; prepared extensions=%s", jsonString(mapValue(rawLife["appearance"])), jsonString(rawLife["wardrobe_items"]), jsonString(prepared["extensions"]))
			}
			if item.ID == "appearance" {
				t.Logf("public fixture appearance: raw=%s; prepared=%s", jsonString(rawLife["appearance"]), jsonString(mapValue(mapValue(mapValue(prepared["core_persona"])["life_profile"])["appearance"])))
			}
		}
	}
	for path, expected := range map[string]float64{
		"core_persona.personality.openness": 0.7, "core_persona.personality.empathy": 0.85,
		"core_persona.behavioral_policy.emoji_frequency": 0, "core_persona.behavioral_policy.sarcasm_tendency": 0.2,
		"core_persona.behavioral_policy.response_delay": 45,
	} {
		value, _ := initializationValueAtPath(prepared, path)
		if got, ok := numberFloat(value); !ok || got != expected {
			t.Errorf("live initialization %s = %v; want %v", path, value, expected)
		}
	}
	item, _ := initializationValueAtPath(prepared, "core_persona.life_profile.appearance.wardrobe_items[0].currently_worn")
	if item != false {
		t.Errorf("live initialization wardrobe currently_worn = %v; want false", item)
	}
}
