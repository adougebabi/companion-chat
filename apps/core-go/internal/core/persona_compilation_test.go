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
	output := map[string]any{"profile_id": "default", "facts": []any{
		map[string]any{"category": "identity", "text": "摇光", "source_refs": []any{"identity.name"}},
		map[string]any{"category": "stable_preferences", "text": "喜欢咖啡", "source_refs": []any{"life_profile.preferences.coffee"}},
		map[string]any{"category": "stable_preferences", "text": "喜欢长发造型", "source_refs": []any{"life_profile.style_preferences.hair"}},
		map[string]any{"category": "stable_preferences", "text": "通常喝咖啡", "source_refs": []any{"life_profile.life_habits.0"}},
	}, "omissions": []any{}}
	if _, err := decodeCompiledWorkingPersona(output, source, input); err == nil || !strings.Contains(err.Error(), "life_profile.life_habits.1") {
		t.Fatalf("an uncovered canonical habit was accepted: %v", err)
	}
	output["omissions"] = []any{map[string]any{"source_ref": "life_profile.life_habits.1", "reason": "full detail only"}}
	if _, err := decodeCompiledWorkingPersona(output, source, input); err != nil {
		t.Fatalf("indexed habit omission was rejected: %v", err)
	}
}

func TestCompiledPersonaRequiresSourceLinkedPreferences(t *testing.T) {
	input := PersonaCompilationInput{ProfileID: "warm"}
	source := map[string]any{"identity": map[string]any{"name": "摇光"}, "profile": map[string]any{"id": "warm"}, "life_profile": map[string]any{"preferences": map[string]any{"drink": "喜欢咖啡", "dessert": "讨厌太甜"}}}
	base := map[string]any{"profile_id": "warm", "facts": []any{
		map[string]any{"category": "identity", "text": "摇光", "source_refs": []any{"identity.name"}},
		map[string]any{"category": "stable_preferences", "text": "喜欢咖啡", "source_refs": []any{"life_profile.preferences.drink"}},
	}, "omissions": []any{}}
	if _, err := decodeCompiledWorkingPersona(base, source, input); err == nil || !strings.Contains(err.Error(), "preference_unaccounted") {
		t.Fatalf("missing preference was accepted: %v", err)
	}
	base["omissions"] = []any{map[string]any{"source_ref": "life_profile.preferences.dessert", "reason": "detailed source only"}}
	if _, err := decodeCompiledWorkingPersona(base, source, input); err != nil {
		t.Fatalf("diagnosed omission was rejected: %v", err)
	}
	base["facts"] = []any{map[string]any{"category": "stable_preferences", "text": "喜欢咖啡", "source_refs": []any{"life_profile.preferences.nonexistent"}}}
	if _, err := decodeCompiledWorkingPersona(base, source, input); err == nil || !strings.Contains(err.Error(), "ref_invalid") {
		t.Fatalf("fabricated source ref accepted: %v", err)
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
	foundBombardment := false
	for _, fact := range baseline.Facts {
		if strings.Contains(fact.Text, "high_frequency_daily_bombardment") || strings.Contains(fact.Text, "非常主动找用户") {
			foundBombardment = true
			break
		}
	}
	if !foundBombardment {
		t.Fatalf("synthesizeBaselineWorkingPersona did not produce fact for high_frequency_daily_bombardment: %#v", baseline.Facts)
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
		{name: "unrelated foreign profile", profileID: "alice", wantErr: true},
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
