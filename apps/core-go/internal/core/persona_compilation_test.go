package core

import (
	"strings"
	"testing"
)

func TestPersonaCompilationSourceExcludesDynamicStateAndOtherProfiles(t *testing.T) {
	core := map[string]any{
		"identity":     map[string]any{"name": "摇光", "current_mood": "生气"},
		"life_profile": map[string]any{"preferences": map[string]any{"drink": "喜欢咖啡，但不喜欢甜咖啡"}, "current_outfit": "红裙", "relationships": map[string]any{"sister": "姐姐", "relationship_progress": "今天生气"}},
		"personality_system": map[string]any{"core_conflict": "温暖与冷淡对风险有长期分歧", "integration": map[string]any{"shared_memory_policy": "共享已确认事实"}, "forced_activation": map[string]any{"private_rule": "机器规则"}, "profiles": []any{
			map[string]any{"id": "warm", "voice": "温暖", "current_scene": "厨房"},
			map[string]any{"id": "cool", "voice": "冷淡", "secrets": "另一人格的秘密"},
		}},
		"current_state": map[string]any{"mood": "sad"},
	}
	source, err := personaCompilationSource(PersonaCompilationInput{CorePersona: core, ProfileID: "warm"})
	if err != nil {
		t.Fatal(err)
	}
	encoded := jsonString(source)
	for _, forbidden := range []string{"生气", "红裙", "厨房", "另一人格的秘密", "current_state"} {
		if strings.Contains(encoded, forbidden) {
			t.Fatalf("dynamic or foreign source leaked %q: %s", forbidden, encoded)
		}
	}
	if strings.Contains(encoded, "姐姐") || !strings.Contains(encoded, "喜欢咖啡，但不喜欢甜咖啡") || !strings.Contains(encoded, "温暖") {
		t.Fatalf("stable source was lost: %s", encoded)
	}
	if !strings.Contains(encoded, "长期分歧") || !strings.Contains(encoded, "共享已确认事实") || strings.Contains(encoded, "机器规则") {
		t.Fatalf("shared semantics or machine-rule boundary wrong: %s", encoded)
	}
}

func TestPersonaCompilationSourceSeparatesCurrentAppearanceFromPreferencesAndHabits(t *testing.T) {
	core := map[string]any{
		"identity": map[string]any{"name": "摇光", "background_story": "过去曾留长发", "appearance": "当前长发"},
		"life_profile": map[string]any{
			"appearance": map[string]any{
				"description":       "现在留长发，穿白衬衫",
				"physical_features": map[string]any{"hair_length": "long"},
				"style_preferences": map[string]any{"hair": "喜欢长发造型"},
			},
			"life_habits": []any{"通常喝咖啡", "通常早睡"},
			"preferences": map[string]any{"coffee": "喜欢咖啡"},
		},
	}
	input := PersonaCompilationInput{CorePersona: core, ProfileID: "default", TargetBudgetRunes: 2000}
	source, err := personaCompilationSource(input)
	if err != nil {
		t.Fatal(err)
	}
	encoded := jsonString(source)
	for _, forbidden := range []string{"现在留长发", "穿白衬衫", "hair_length", "过去曾留长发", "当前长发"} {
		if strings.Contains(encoded, forbidden) {
			t.Fatalf("current appearance leaked into portrait source: %s", encoded)
		}
	}
	for _, stable := range []string{"喜欢长发造型", "通常喝咖啡", "通常早睡"} {
		if !strings.Contains(encoded, stable) {
			t.Fatalf("stable preference/habit lost: %s", encoded)
		}
	}
	if !personaCompilationSourcePathExists(source, "life_profile.life_habits.1") || personaCompilationSourcePathExists(source, "life_profile.life_habits.2") {
		t.Fatalf("array source refs do not match the retained habits: %s", encoded)
	}
	compiled, err := decodeCompiledWorkingPersona(map[string]any{"portrait_text": "摇光喜欢咖啡和长发造型，通常早睡。"}, source, input)
	if err != nil || compiled.PortraitText == "" || len(compiled.Facts) != 0 {
		t.Fatalf("plain-text portrait was rejected or expanded into facts: %#v err=%v", compiled, err)
	}
}

func TestCompiledPersonaTextDoesNotRequireModelSourceRefs(t *testing.T) {
	input := PersonaCompilationInput{ProfileID: "warm"}
	source := map[string]any{"identity": map[string]any{"name": "摇光"}, "profile": map[string]any{"id": "warm"}, "life_profile": map[string]any{"preferences": map[string]any{"drink": "喜欢咖啡", "dessert": "讨厌太甜"}}}
	output := map[string]any{"portrait_text": "摇光喜欢咖啡，也不喜欢太甜。", "source_refs": []any{"extensions.core_persona.personality.surface"}}
	compiled, err := decodeCompiledWorkingPersona(output, source, input)
	if err != nil || compiled.PortraitText != output["portrait_text"] || len(compiled.Facts) != 0 {
		t.Fatalf("model-authored path still blocked plain text: %#v err=%v", compiled, err)
	}
}

func TestCompiledPersonaRejectsOverBudgetWithoutCuttingFacts(t *testing.T) {
	source := map[string]any{"profile": map[string]any{"id": "warm", "voice": "温暖"}}
	output := map[string]any{"profile_id": "warm", "facts": []any{map[string]any{"category": "language_expression", "text": strings.Repeat("温暖而诚实。", 100), "source_refs": []any{"profile.voice"}}}, "omissions": []any{}}
	_, err := decodeCompiledWorkingPersona(output, source, PersonaCompilationInput{ProfileID: "warm", TargetBudgetRunes: 512})
	if err == nil || !strings.Contains(err.Error(), "over_budget") {
		t.Fatalf("over-budget portrait was silently truncated or accepted: %v", err)
	}
}

func TestPersonaCompilationPreservesBehavioralPolicyAndProactiveBombardment(t *testing.T) {
	core := map[string]any{
		"identity": map[string]any{"name": "摇光"},
		"behavioral_policy": map[string]any{
			"high_frequency_daily_bombardment": "消息密度很高，一天可能发十几条到几十条，非常主动找用户",
			"initiative":                       0.95,
		},
		"personality": map[string]any{"openness": 0.8},
		"personality_system": map[string]any{
			"active_profile_id": "default",
			"profiles": []any{
				map[string]any{
					"id":   "default",
					"name": "摇光",
					"behavioral_policy": map[string]any{
						"response_style": "温和自然",
					},
				},
			},
		},
	}

	input := PersonaCompilationInput{CorePersona: core, ProfileID: "default", TargetBudgetRunes: 2000}
	source, err := personaCompilationSource(input)
	if err != nil {
		t.Fatalf("personaCompilationSource failed: %v", err)
	}

	encoded := jsonString(source)
	if !strings.Contains(encoded, "high_frequency_daily_bombardment") {
		t.Fatalf("source lost high_frequency_daily_bombardment: %s", encoded)
	}
	if !strings.Contains(encoded, "温和自然") {
		t.Fatalf("source lost profile response_style: %s", encoded)
	}

	baseline, err := synthesizeBaselineWorkingPersona(input)
	if err != nil {
		t.Fatalf("synthesizeBaselineWorkingPersona failed: %v", err)
	}
	if !strings.Contains(baseline.PortraitText, "high_frequency_daily_bombardment") && !strings.Contains(baseline.PortraitText, "非常主动找用户") {
		t.Fatalf("baseline text lost high_frequency_daily_bombardment: %q", baseline.PortraitText)
	}
	if len(baseline.Facts) != 0 {
		t.Fatalf("baseline persisted categorized facts: %#v", baseline.Facts)
	}

	derived := deriveWorkingPersonaBody(core)
	derivedPolicy := mapValue(derived["behavioral_policy"])
	if got := stringValue(derivedPolicy["high_frequency_daily_bombardment"]); got == "" {
		t.Fatalf("deriveWorkingPersonaBody lost high_frequency_daily_bombardment: %#v", derived)
	}
	if got := stringValue(derivedPolicy["response_style"]); got != "温和自然" {
		t.Fatalf("deriveWorkingPersonaBody lost response_style override: %#v", derivedPolicy)
	}
}

func TestPersonaCompilationDropsNestedLegacyMutableExtensions(t *testing.T) {
	core := map[string]any{"identity": map[string]any{"name": "摇光"}, "extensions": map[string]any{
		"appearance":    map[string]any{"hair_length": "long", "hair_color": "red"},
		"nested":        []any{map[string]any{"hair_length": "short", "ritual": "每天画画"}},
		"stable_ritual": "睡前画画",
	}}
	source, err := personaCompilationSource(PersonaCompilationInput{CorePersona: core, ProfileID: "default"})
	if err != nil {
		t.Fatal(err)
	}
	encoded := jsonString(source)
	if strings.Contains(encoded, "hair_length") || strings.Contains(encoded, "hair_color") || strings.Contains(encoded, "long") || !strings.Contains(encoded, "睡前画画") || !strings.Contains(encoded, "每天画画") {
		t.Fatalf("nested extension classification failed: %s", encoded)
	}
}

func TestDecodeCompiledWorkingPersonaProfileTolerance(t *testing.T) {
	input := PersonaCompilationInput{ProfileID: "shenlu_main"}
	source := map[string]any{
		"identity": map[string]any{"name": "沈鹿", "nickname": "小鹿"},
		"profile":  map[string]any{"id": "shenlu_main", "name": "沈鹿主性格"},
	}
	baseOutput := func(profileID string) map[string]any {
		return map[string]any{
			"profile_id": profileID,
			"facts": []any{
				map[string]any{"category": "identity", "text": "沈鹿", "source_refs": []any{"identity.name"}},
			},
			"omissions": []any{},
		}
	}

	for _, tc := range []struct {
		name      string
		profileID string
		wantErr   bool
	}{
		{name: "exact match", profileID: "shenlu_main", wantErr: false},
		{name: "case insensitive", profileID: "Shenlu_Main", wantErr: false},
		{name: "trimmed _main suffix", profileID: "shenlu", wantErr: false},
		{name: "matches identity name", profileID: "沈鹿", wantErr: false},
		{name: "matches identity nickname", profileID: "小鹿", wantErr: false},
		{name: "matches profile name", profileID: "沈鹿主性格", wantErr: false},
		{name: "omitted profile_id", profileID: "", wantErr: false},
		{name: "model echo is not authority", profileID: "alice", wantErr: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			compiled, err := decodeCompiledWorkingPersona(baseOutput(tc.profileID), source, input)
			if tc.wantErr {
				if err == nil || !strings.Contains(err.Error(), "persona_compilation_profile_mismatch") {
					t.Fatalf("expected persona_compilation_profile_mismatch, got: %v", err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if compiled.ProfileID != "shenlu_main" {
					t.Fatalf("expected compiled.ProfileID to be normalized to shenlu_main, got: %q", compiled.ProfileID)
				}
			}
		})
	}
}

func TestDecodeCompiledWorkingPersonaFactsTolerance(t *testing.T) {
	input := PersonaCompilationInput{ProfileID: "shenlu_main"}
	source := map[string]any{
		"identity": map[string]any{"name": "沈鹿"},
		"profile":  map[string]any{"id": "shenlu_main"},
	}

	t.Run("envelope wrapped in persona_compilation_response", func(t *testing.T) {
		output := map[string]any{
			"persona_compilation_response": map[string]any{
				"profile_id": "shenlu_main",
				"facts": []any{
					map[string]any{"category": "identity", "text": "沈鹿", "source_refs": []any{"identity.name"}},
				},
				"omissions": []any{},
			},
		}
		compiled, err := decodeCompiledWorkingPersona(output, source, input)
		if err != nil {
			t.Fatalf("unexpected error for wrapped response: %v", err)
		}
		if len(compiled.Facts) != 1 || compiled.Facts[0].Text != "沈鹿" {
			t.Fatalf("unexpected compiled facts: %#v", compiled.Facts)
		}
	})

	t.Run("empty response fails instead of fabricating a portrait", func(t *testing.T) {
		output := map[string]any{
			"profile_id": "shenlu_main",
			"facts":      []any{},
			"omissions":  []any{},
		}
		if _, err := decodeCompiledWorkingPersona(output, source, input); err == nil || !strings.Contains(err.Error(), "persona_compilation_empty") {
			t.Fatalf("empty response fabricated a portrait: %v", err)
		}
	})

	t.Run("alternative key portrait instead of facts", func(t *testing.T) {
		output := map[string]any{
			"profile_id": "shenlu_main",
			"portrait": []any{
				map[string]any{"category": "identity", "text": "沈鹿", "source_refs": []any{"identity.name"}},
			},
			"omissions": []any{},
		}
		compiled, err := decodeCompiledWorkingPersona(output, source, input)
		if err != nil {
			t.Fatalf("unexpected error for alternative key portrait: %v", err)
		}
		if len(compiled.Facts) != 1 || compiled.Facts[0].Text != "沈鹿" {
			t.Fatalf("expected facts from portrait key, got: %#v", compiled.Facts)
		}
	})
}

func TestResolvePersonaSourceRefTolerance(t *testing.T) {
	source := map[string]any{
		"identity": map[string]any{"name": "沈鹿"},
		"personality": map[string]any{
			"core_drive": "寻找自我",
		},
		"life_profile": map[string]any{
			"preferences": map[string]any{"drink": "喜欢咖啡"},
		},
		"profile": map[string]any{
			"id": "default",
			"personality": map[string]any{
				"core_drive": "寻找自我",
			},
		},
	}

	t.Run("resolves prefixed paths", func(t *testing.T) {
		for _, tc := range []struct {
			raw      string
			expected string
		}{
			{"personality.core_drive", "personality.core_drive"},
			{"extensions.core_persona.personality.core_drive", "personality.core_drive"},
			{"core_persona.personality.core_drive", "personality.core_drive"},
			{"source.personality.core_drive", "personality.core_drive"},
			{"core_persona.identity.name", "identity.name"},
			{"source.life_profile.preferences.drink", "life_profile.preferences.drink"},
			{"extensions.nonexistent.fake", ""},
		} {
			resolved := resolvePersonaSourceRef(source, tc.raw)
			if resolved != tc.expected {
				t.Errorf("resolvePersonaSourceRef(%q) = %q, want %q", tc.raw, resolved, tc.expected)
			}
		}
	})

	t.Run("decodeCompiledWorkingPersona accepts extensions.core_persona.personality.core_drive", func(t *testing.T) {
		input := PersonaCompilationInput{ProfileID: "default"}
		output := map[string]any{
			"profile_id": "default",
			"facts": []any{
				map[string]any{
					"category":    "identity",
					"text":        "沈鹿",
					"source_refs": []any{"identity.name"},
				},
				map[string]any{
					"category":    "core_mechanisms",
					"text":        "寻找自我驱动",
					"source_refs": []any{"extensions.core_persona.personality.core_drive"},
				},
			},
			"omissions": []any{
				map[string]any{
					"source_ref": "extensions.core_persona.life_profile.preferences.drink",
					"reason":     "detailed source only",
				},
			},
		}
		compiled, err := decodeCompiledWorkingPersona(output, source, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(compiled.Facts) != 2 {
			t.Fatalf("expected 2 facts, got: %d", len(compiled.Facts))
		}
		if compiled.Facts[1].SourceRefs[0] != "personality.core_drive" {
			t.Fatalf("expected resolved source_ref personality.core_drive, got: %q", compiled.Facts[1].SourceRefs[0])
		}
		if len(compiled.Omissions) != 0 || !strings.Contains(compiled.PortraitText, "寻找自我驱动") {
			t.Fatalf("legacy text was lost or model omissions became authority: %#v", compiled)
		}
	})
}
