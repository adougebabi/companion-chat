package core

import "testing"

func TestNormalizeResponsePlanFiltersUnsupportedAndRepeatedClaims(t *testing.T) {
	context := ContextProjection{
		ContextRevision: 4,
		Hypotheses: []map[string]any{{
			"repetition_key": "我 最近 喜欢 火车",
			"evidence_refs":  []any{"fact-old"},
			"source_fact_id": "fact-old",
			"status":         "active",
		}},
	}
	decision := map[string]any{
		"action_type": "reply",
		"claims": []any{
			map[string]any{"kind": ClaimUnsupportedSelf, "content": "我小时候住在海边", "confidence": 0.9, "evidence_refs": []any{}},
			map[string]any{"kind": ClaimSupportedHypothesis, "content": "我最近喜欢火车", "confidence": 0.8, "evidence_refs": []any{"fact-old"}, "repetition_key": "我 最近 喜欢 火车"},
			map[string]any{"kind": ClaimUncertainHypothesis, "content": "我可能需要休息", "confidence": 0.4, "evidence_refs": []any{"fact-new"}},
		},
	}
	plan, err := normalizeResponsePlan(decision, "fact-new", context)
	if err != nil {
		t.Fatalf("normalizeResponsePlan() error = %v", err)
	}
	if got := len(arrayValue(plan["approved_claims"])); got != 0 {
		t.Fatalf("approved claims = %d, want 0", got)
	}
	if got := len(arrayValue(plan["uncertain_claims"])); got != 1 {
		t.Fatalf("uncertain claims = %d, want 1", got)
	}
	if got := len(arrayValue(plan["omitted_claims"])); got != 2 {
		t.Fatalf("omitted claims = %d, want 2", got)
	}
	self := mapValue(plan["self_evaluation"])
	if stringValue(self["mode"]) != "uncertain" {
		t.Fatalf("self evaluation mode = %q", stringValue(self["mode"]))
	}
}

func TestEvaluateClaimsRejectsInvalidKindsAndConfidence(t *testing.T) {
	context := ContextProjection{}
	if _, _, _, err := evaluateClaims([]any{map[string]any{"kind": "made_up", "content": "x", "confidence": 0.5}}, "fact", context); err == nil {
		t.Fatal("expected invalid kind error")
	}
	if _, _, _, err := evaluateClaims([]any{map[string]any{"kind": ClaimObservedFact, "content": "x", "confidence": 2, "evidence_refs": []any{"fact"}}}, "fact", context); err == nil {
		t.Fatal("expected invalid confidence error")
	}
}

func TestRepetitionKeyIsDeterministicAndBounded(t *testing.T) {
	left := repetitionKeyFor("我最近 很喜欢 安静的咖啡馆")
	right := repetitionKeyFor("我最近，很喜欢：安静的咖啡馆")
	if left != right {
		t.Fatalf("repetition keys differ: %q != %q", left, right)
	}
}

func TestCosineSimilarityHandlesDimensionsAndZeroVectors(t *testing.T) {
	if got := cosineSimilarity([]float64{1, 0}, []float64{1, 0}); got != 1 {
		t.Fatalf("cosine = %v, want 1", got)
	}
	if got := cosineSimilarity([]float64{1}, []float64{1, 0}); got != 0 {
		t.Fatalf("mismatched cosine = %v, want 0", got)
	}
	if got := cosineSimilarity([]float64{0, 0}, []float64{1, 0}); got != 0 {
		t.Fatalf("zero cosine = %v, want 0", got)
	}
}

func TestMemoryVisibilityIsAppliedBeforeRanking(t *testing.T) {
	if !memoryVisibleToActor("private", "fl", "owner", "owner", nil) {
		t.Fatal("owner should see private memory")
	}
	if memoryVisibleToActor("private", "fl", "owner", "other", nil) {
		t.Fatal("other actor should not see private memory")
	}
	if !memoryVisibleToActor("participants", "fl", "owner", "participant", []any{"participant"}) {
		t.Fatal("participant should see participant memory")
	}
	if memoryVisibleToActor("unknown", "fl", "owner", "owner", nil) {
		t.Fatal("unknown visibility must fail closed")
	}
}
