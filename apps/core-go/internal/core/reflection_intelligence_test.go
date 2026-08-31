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
		"self_model_candidates": []any{map[string]any{
			"category": "preference", "claim": "我喜欢安静", "confidence": 0.8,
			"evidence_refs": []any{"fact-1"},
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
		"self_model_candidates":   []any{map[string]any{"type": "taste", "content": "我偏好克制的色彩", "confidence": 0.8, "evidence_refs": []any{"fact-1"}}},
		"relationship_candidates": []any{map[string]any{"counterparty_id": "human-1", "relationship_type": "collaborator", "evidence_refs": []any{"fact-1"}}},
	})
	memory := arrayValue(proposal["memory_candidates"])
	if len(memory) != 2 || stringValue(mapValue(memory[0])["type"]) != "semantic" || stringValue(mapValue(memory[0])["visibility"]) != "owner" || mapValue(memory[1])["emotional_significance"] != 0.0 {
		t.Fatalf("normalized memory candidates = %#v", memory)
	}
	self := arrayValue(proposal["self_model_candidates"])
	if len(self) != 1 || stringValue(mapValue(self[0])["category"]) != "taste" || stringValue(mapValue(self[0])["claim"]) != "我偏好克制的色彩" {
		t.Fatalf("normalized self-model candidates = %#v", self)
	}
	if got := len(arrayValue(proposal["relationship_candidates"])); got != 0 {
		t.Fatalf("incomplete relationship candidates = %d, want 0", got)
	}
}
