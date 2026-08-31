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
