package core

import "testing"

func TestValidateReflectionProposalRequiresWindowEvidenceAndNumericFields(t *testing.T) {
	allowed := map[string]struct{}{"fact-1": {}}
	valid := map[string]any{
		"memory_candidates": []any{map[string]any{
			"type": "semantic", "content": "用户喜欢安静", "confidence": 0.9,
			"importance": 0.8, "emotional_significance": 0.2, "visibility": "private",
			"evidence_refs": []any{"fact-1"},
		}},
		"developing_self_candidates": []any{map[string]any{
			"category": "preference", "claim": "我喜欢安静", "value": map[string]any{"preference": "安静"}, "confidence": 0.8,
			"evidence_refs": []any{"fact-1"}, "provenance": map[string]any{"source": "reflection"},
		}},
	}
	if err := validateReflectionProposal(valid, allowed); err != nil {
		t.Fatalf("valid proposal rejected = %v", err)
	}
	invalid := map[string]any{
		"memory_candidates": []any{map[string]any{
			"type": "semantic", "content": "用户喜欢安静", "confidence": 0.9,
			"importance": 0.8, "emotional_significance": 0.2, "visibility": "private",
			"evidence_refs": []any{"fact-foreign"},
		}},
	}
	if err := validateReflectionProposal(invalid, allowed); err == nil {
		t.Fatal("foreign evidence should be rejected")
	}
}

func TestNormalizeReflectionProposalKeepsValidAliasesAndDropsIncompleteCandidates(t *testing.T) {
	proposal := normalizeReflectionProposal(map[string]any{
		"memory_candidates": []any{
			map[string]any{"memory_type": "user_preference", "scope": "conversation", "content": "喜欢蓝灰色", "confidence": 0.9, "importance": 0.8, "emotional_significance": 0.4},
			map[string]any{"memory_type": "context", "scope": "conversation", "content": "使用安全默认值", "confidence": 0.7, "importance": 0.3},
		},
		"developing_self_candidates": []any{map[string]any{"type": "preference", "content": "我偏好克制的色彩", "value": map[string]any{"taste": "克制"}, "confidence": 0.8, "provenance": map[string]any{"source": "reflection"}, "evidence_refs": []any{"fact-1"}}},
		"relationship_candidates":    []any{map[string]any{"counterparty_id": "human-1", "relationship_type": "collaborator", "evidence_refs": []any{"fact-1"}}},
	})
	memory := arrayValue(proposal["memory_candidates"])
	if len(memory) != 2 || stringValue(mapValue(memory[0])["type"]) != "semantic" || stringValue(mapValue(memory[0])["visibility"]) != "owner" || mapValue(memory[1])["emotional_significance"] != 0.0 {
		t.Fatalf("normalized memory candidates = %#v", memory)
	}
	self := arrayValue(proposal["developing_self_candidates"])
	if len(self) != 1 || stringValue(mapValue(self[0])["category"]) != "preference" || stringValue(mapValue(self[0])["claim"]) != "我偏好克制的色彩" {
		t.Fatalf("normalized developing-self candidates = %#v", self)
	}
	if got := len(arrayValue(proposal["relationship_candidates"])); got != 1 || !boolValueForTest(mapValue(arrayValue(proposal["relationship_candidates"])[0])["__invalid_candidate"]) {
		t.Fatalf("incomplete relationship candidates = %#v", proposal["relationship_candidates"])
	}
}

func TestNormalizeReflectionRelationshipRequiresCompleteCASnapshot(t *testing.T) {
	proposal := normalizeReflectionProposal(map[string]any{
		"relationship_candidates": []any{
			map[string]any{
				"target_actor_id": "actor_user",
				"trend":           "improving",
				"role":            map[string]any{"label": "恋人"},
				"metrics":         map[string]any{"trust": 0.8},
				"evidence_refs":   []any{"fact-1"},
			},
			map[string]any{
				"counterparty_id":   "actor_user",
				"relationship_type": "朋友",
				"trend":             "stable",
				"metrics":           map[string]any{},
				"expected_revision": 2,
				"evidence_refs":     []any{"fact-1"},
			},
		},
	})
	items := arrayValue(proposal["relationship_candidates"])
	if len(items) != 2 {
		t.Fatalf("normalized relationships = %#v", items)
	}
	if !boolValueForTest(mapValue(items[0])["__invalid_candidate"]) {
		t.Fatal("relationship without expected_revision must be rejected")
	}
	second := mapValue(items[1])
	if boolValueForTest(second["__invalid_candidate"]) || stringValue(mapValue(second["role"])["label"]) != "朋友" || intValue(second["expected_revision"]) != 2 {
		t.Fatalf("relationship alias normalization = %#v", second)
	}
	if err := validateReflectionProposal(proposal, map[string]struct{}{"fact-1": {}}); err == nil {
		t.Fatal("incomplete relationship snapshot should fail closed")
	}
}

func TestValidateReflectionRelationshipRejectsUnsupportedTrendAndMetrics(t *testing.T) {
	base := map[string]any{
		"target_actor_id":   "actor_user",
		"role":              map[string]any{"label": "朋友"},
		"metrics":           map[string]any{"trust": 0.8},
		"expected_revision": 0,
		"evidence_refs":     []any{"fact-1"},
	}
	invalidTrend := map[string]any{"relationship_candidates": []any{map[string]any{"target_actor_id": "actor_user", "role": base["role"], "metrics": base["metrics"], "trend": "unknown", "expected_revision": 0, "evidence_refs": []any{"fact-1"}}}}
	if err := validateReflectionProposal(invalidTrend, map[string]struct{}{"fact-1": {}}); err == nil {
		t.Fatal("unsupported relationship trend should be rejected")
	}
	invalidMetrics := map[string]any{"relationship_candidates": []any{map[string]any{"target_actor_id": "actor_user", "role": base["role"], "metrics": map[string]any{"trust": 2}, "trend": "stable", "expected_revision": 0, "evidence_refs": []any{"fact-1"}}}}
	if err := validateReflectionProposal(invalidMetrics, map[string]struct{}{"fact-1": {}}); err == nil {
		t.Fatal("out-of-range relationship metric should be rejected")
	}
}

func boolValueForTest(value any) bool {
	result, _ := value.(bool)
	return result
}

func TestFilterReflectionEvidencePreservesForeignReferencesForFailClosedValidation(t *testing.T) {
	proposal := map[string]any{
		"memory_candidates": []any{map[string]any{
			"type": "semantic", "content": "喜欢蓝灰色", "importance": 0.8,
			"evidence_refs": []any{"foreign", "memory-1"},
		}},
	}
	filtered := filterReflectionEvidence(proposal, map[string]struct{}{"memory-1": {}})
	items := arrayValue(filtered["memory_candidates"])
	if len(items) != 1 || len(arrayValue(mapValue(items[0])["evidence_refs"])) != 2 {
		t.Fatalf("evidence was silently repaired: %#v", items)
	}
	if err := validateReflectionProposal(filtered, map[string]struct{}{"memory-1": {}}); err == nil {
		t.Fatal("foreign evidence should remain invalid")
	}
}

func TestReflectionNormalizesTypedDriveAndPreferenceSlots(t *testing.T) {
	proposal := normalizeReflectionProposal(map[string]any{
		"drive_recalibration_candidates": []any{map[string]any{"slot_key": "achievement", "name": "成就感", "meaning": "完成重要目标", "schema": "pressure", "value": map[string]any{"pressure": 0.8, "salience": 0.7, "direction": "toward_completion"}, "confidence": 0.9, "evidence_refs": []any{"fact-1"}}},
		"preference_revision_candidates": []any{map[string]any{"key": "quiet_hours", "label": "安静时段", "description": "偏好安静环境", "value_schema": "categorical", "value": map[string]any{"selected": "evening"}, "confidence": 0.8, "evidence_refs": []any{"fact-1"}}},
	})
	drive := mapValue(arrayValue(proposal["drive_candidates"])[0])
	if stringValue(drive["key"]) != "achievement" || stringValue(drive["value_schema"]) != "pressure" {
		t.Fatalf("drive slot = %#v", drive)
	}
	preference := mapValue(arrayValue(proposal["preference_candidates"])[0])
	if stringValue(preference["key"]) != "quiet_hours" || stringValue(preference["value_schema"]) != "categorical" {
		t.Fatalf("preference slot = %#v", preference)
	}
	if err := validateReflectionProposal(proposal, map[string]struct{}{"fact-1": {}}); err != nil {
		t.Fatalf("typed slots rejected = %v", err)
	}
}

func TestReflectionEvidenceRefsUseOneStableSequenceRefPerSourceFact(t *testing.T) {
	evidence := []map[string]any{
		{"sequence": 16, "event_type": "conversation.turn"},
		{"sequence": 17, "event_type": "autonomy.result"},
		{"sequence": 17, "event_type": "cognition.appraisal"},
	}
	refs := reflectionEvidenceRefs(evidence)
	if len(refs) != 2 || refs[0] != "sequence:16" || refs[1] != "sequence:17" {
		t.Fatalf("reflection evidence refs = %#v", refs)
	}
}
