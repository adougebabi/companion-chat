package core

import "testing"

func TestNormalizeDevelopingSelfClaimRequiresEvidenceMetadata(t *testing.T) {
	claim, err := normalizeDevelopingSelfClaim(map[string]any{
		"category":      "preference",
		"claim":         "我可能喜欢安静的环境",
		"value":         map[string]any{"preference": "安静"},
		"confidence":    0.72,
		"evidence_refs": []any{"fact-1"},
		"provenance":    map[string]any{"source": "reflection", "method": "repeated_behavior"},
	}, "reflection")
	if err != nil {
		t.Fatalf("normalizeDevelopingSelfClaim() error = %v", err)
	}
	if claim["category"] != "preference" || claim["claim"] != "我可能喜欢安静的环境" {
		t.Fatalf("normalized claim = %#v", claim)
	}
}

func TestNormalizeDevelopingSelfClaimRejectsCoreCategories(t *testing.T) {
	for _, category := range []string{"identity", "core_value", "behavioral_policy", "personality"} {
		_, err := normalizeDevelopingSelfClaim(map[string]any{
			"category": category, "claim": "不应自动修改核心人格", "value": map[string]any{}, "confidence": 0.9,
			"evidence_refs": []any{"fact-1"}, "provenance": map[string]any{"source": "reflection"},
		}, "reflection")
		if err == nil {
			t.Fatalf("category %q should be rejected", category)
		}
	}
}

func TestContextProjectionFromValuePreservesLayeredAuthority(t *testing.T) {
	value := map[string]any{
		"fluctlight_id":   "fl-1",
		"core_persona":    map[string]any{"authority": "hard_constraint", "data": map[string]any{"personality": map[string]any{"independence": 0.9}}},
		"developing_self": []any{map[string]any{"claim": "我可能喜欢安静", "confidence": 0.7}},
		"current_state":   map[string]any{"authority": "transient_state", "data": map[string]any{"mood": "开心"}},
	}
	projection, ok := contextProjectionFromValue(value)
	if !ok {
		t.Fatal("contextProjectionFromValue() returned false")
	}
	if projection.FluctlightID != "fl-1" || projection.CorePersona["authority"] != "hard_constraint" || len(projection.DevelopingSelf) != 1 || projection.CurrentState["authority"] != "transient_state" {
		t.Fatalf("projection = %#v", projection)
	}
}

func TestValidInitializationUsesLayeredContract(t *testing.T) {
	valid := map[string]any{
		"core_persona": map[string]any{
			"identity":          map[string]any{"name": "冷静的她", "timezone": "Asia/Shanghai"},
			"personality":       map[string]any{"independence": 0.9},
			"behavioral_policy": map[string]any{"response_style": "克制"},
			"life_profile":      map[string]any{"character_constraints": []any{"不刻意讨好"}},
		},
		"developing_self": map[string]any{"claims": []any{map[string]any{
			"category": "preference", "claim": "我可能喜欢安静", "value": map[string]any{"preference": "安静"}, "confidence": 0.8,
			"evidence_refs": []any{}, "provenance": map[string]any{"source": "owner_defined"},
		}}},
		"initial_goals": []any{}, "initial_intentions": []any{},
	}
	if !validInitialization(valid) {
		t.Fatal("layered initialization should be valid")
	}
	invalid := map[string]any{"core_persona": valid["core_persona"], "developing_self": map[string]any{"claims": []any{map[string]any{"category": "personality", "claim": "漂移", "value": map[string]any{}, "confidence": 0.9, "provenance": map[string]any{"source": "reflection"}}}}, "initial_goals": []any{}, "initial_intentions": []any{}}
	if validInitialization(invalid) {
		t.Fatal("core-targeted claim should be invalid")
	}
}

func TestLayeredContractsDoNotExposeLegacyAutomaticPersonalityTargets(t *testing.T) {
	reflection := reflectionResponseSchema()
	for _, key := range []string{"personality_candidates", "self_model_candidates"} {
		if containsSchemaRequired(reflection, key) {
			t.Fatalf("legacy reflection key %q is still required", key)
		}
	}
	if _, ok := defaultProvenance()["self_model"]; ok {
		t.Fatal("new instances must not initialize provenance.self_model")
	}
}
