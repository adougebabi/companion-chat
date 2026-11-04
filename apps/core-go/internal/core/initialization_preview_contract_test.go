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

func TestInitializationPreviewPhysicalRequestAndMissingFields(t *testing.T) {
	const description = "姓名：林夏。她有黑色长发，出身教师家庭，每晚散步；她有一件白衬衫，现在没有穿。她的开放性是0，从不使用表情。"
	modelResult := map[string]any{"core_persona": map[string]any{
		"identity":          map[string]any{"name": "林夏"},
		"personality":       map[string]any{"openness": 0},
		"behavioral_policy": map[string]any{"emoji_frequency": 0},
		"life_profile": map[string]any{
			"appearance": map[string]any{
				"description": "黑色长发",
				"wardrobe_items": []any{map[string]any{
					"category": "upper", "slot": "shirt", "description": "白衬衫",
					"ownership": "owned", "available": true, "currently_worn": false,
				}},
			},
			"social_background": map[string]any{"family": "教师家庭", "lives_alone": false},
			"life_habits":       []any{"每晚散步"},
		},
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
		t.Fatalf("a partial model result must remain reviewable: %v", err)
	}

	messages := arrayValue(captured["messages"])
	if len(messages) != 2 || mapValue(messages[1])["role"] != "user" || mapValue(messages[1])["content"] != description {
		t.Fatalf("the original description must be sent once and verbatim: %#v", messages)
	}
	if mapValue(captured["response_format"])["type"] != "json_object" || captured["max_tokens"] != nil || captured["max_completion_tokens"] != nil {
		t.Fatal("initialization must keep json_object and omit output token limits")
	}
	protocol := stringValue(mapValue(messages[0])["content"])
	const prefix = "Canonical JSON shape: "
	const suffix = ". Separate concrete finite wishes"
	start := strings.Index(protocol, prefix)
	if start < 0 {
		t.Fatal("physical request is missing the canonical initialization shape")
	}
	shapeText := protocol[start+len(prefix):]
	end := strings.Index(shapeText, suffix)
	if end < 0 {
		t.Fatal("physical request is missing the end of the canonical initialization shape")
	}
	var shape map[string]any
	if err := json.Unmarshal([]byte(shapeText[:end]), &shape); err != nil {
		t.Fatalf("canonical initialization shape is not JSON: %v", err)
	}
	persona := mapValue(shape["core_persona"])
	for group, keys := range map[string][]string{
		"identity":          {"name", "nickname", "gender", "age", "occupation", "height", "height_cm", "blood_type", "birthplace", "residence", "timezone", "birthday", "background", "background_story", "biography", "core_values", "worldview", "notes"},
		"personality":       {"openness", "conscientiousness", "extraversion", "agreeableness", "neuroticism", "curiosity", "independence", "patience", "empathy", "assertiveness", "humor", "sociability", "risk_tolerance"},
		"behavioral_policy": {"response_style", "message_length", "emoji_frequency", "punctuation_style", "humor_style", "sarcasm_tendency", "directness", "initiative", "topic_initiation", "silence_tolerance", "response_delay", "emotional_expression", "conflict_style", "refusal_style", "intimacy_expression"},
		"life_profile":      {"appearance", "social_background", "preferences", "life_habits", "recurring_commitments", "character_constraints", "relationship_seeds", "media_preferences"},
	} {
		for _, key := range keys {
			if _, ok := mapValue(persona[group])[key]; !ok {
				t.Errorf("physical request does not name preview field core_persona.%s.%s", group, key)
			}
		}
	}
	if _, ok := mapValue(mapValue(persona["life_profile"])["appearance"])["wardrobe_items"]; !ok {
		t.Error("physical request does not name the canonical appearance.wardrobe_items path")
	}
	for _, rule := range []string{"unknown fields may be omitted or null", "0 to 1", "response_delay", "short", "medium", "long", "preserve its wording in core_persona.life_profile.appearance.description"} {
		if !strings.Contains(protocol, rule) {
			t.Errorf("physical request is missing preview-field rule %q", rule)
		}
	}

	actual := mapValue(prepared["core_persona"])
	for group, present := range map[string]string{"personality": "openness", "behavioral_policy": "emoji_frequency"} {
		values := mapValue(actual[group])
		if n, ok := numberFloat(values[present]); !ok || n != 0 {
			t.Errorf("core_persona.%s.%s lost explicit zero: %#v", group, present, values[present])
		}
		for key, value := range values {
			if key != present && value != nil {
				t.Errorf("omitted core_persona.%s.%s was invented: %#v", group, key, value)
			}
		}
	}
	life := mapValue(actual["life_profile"])
	appearance := mapValue(life["appearance"])
	items := arrayValue(appearance["wardrobe_items"])
	if appearance["description"] != "黑色长发" || len(items) != 1 || mapValue(items[0])["currently_worn"] != false || mapValue(life["social_background"])["lives_alone"] != false || len(arrayValue(life["life_habits"])) != 1 {
		t.Fatalf("provided life facts or false values were lost: %#v", life)
	}
	if len(mapValue(life["preferences"])) != 0 || len(arrayValue(life["recurring_commitments"])) != 0 {
		t.Fatalf("omitted life details must stay empty: %#v", life)
	}
}

func TestInitializationPreviewPromotesWardrobeAliasWithoutOverwritingCanonical(t *testing.T) {
	alias := []any{map[string]any{"description": "白色亚麻衬衫", "ownership": "owned", "available": false, "currently_worn": false, "extra": 0}}
	canonical := []any{map[string]any{"description": "蓝色外套", "ownership": "unknown", "currently_worn": false}}
	for _, tc := range []struct {
		name       string
		appearance map[string]any
		want       []any
		conflict   bool
	}{
		{name: "missing canonical", appearance: map[string]any{"description": "乌黑长发"}, want: alias},
		{name: "null canonical", appearance: map[string]any{"wardrobe_items": nil}, want: alias},
		{name: "conflicting canonical", appearance: map[string]any{"wardrobe_items": canonical}, want: canonical, conflict: true},
		{name: "explicitly empty canonical", appearance: map[string]any{"wardrobe_items": []any{}}, want: []any{}, conflict: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := map[string]any{"core_persona": map[string]any{
				"identity":     map[string]any{"name": "林夏"},
				"life_profile": map[string]any{"appearance": tc.appearance, "wardrobe_items": alias},
			}}
			before := jsonString(raw)
			prepared, err := prepareAnalyzedInitializationResponse("姓名：林夏。她拥有一件白色亚麻衬衫。", raw)
			if err != nil {
				t.Fatal(err)
			}
			life := mapValue(mapValue(prepared["core_persona"])["life_profile"])
			if got := mapValue(life["appearance"])["wardrobe_items"]; jsonString(got) != jsonString(tc.want) {
				t.Errorf("canonical wardrobe = %s; want %s", jsonString(got), jsonString(tc.want))
			}
			preserved := mapValue(prepared["extensions"])["core_persona.life_profile.wardrobe_items"]
			if tc.conflict && jsonString(preserved) != jsonString(alias) {
				t.Errorf("conflicting alias must remain reviewable: %s", jsonString(preserved))
			}
			if !tc.conflict && preserved != nil {
				t.Errorf("promoted alias should not be duplicated in extensions: %s", jsonString(preserved))
			}
			if jsonString(raw) != before {
				t.Error("normalization must not mutate the provider result")
			}
		})
	}
}
