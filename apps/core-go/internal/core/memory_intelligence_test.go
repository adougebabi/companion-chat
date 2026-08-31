package core

import "testing"

func TestNormalizeMemoryRecordRequiresEvidenceAndBoundedSemantics(t *testing.T) {
	base := map[string]any{
		"type": "semantic", "content": "用户喜欢安静的咖啡馆", "confidence": 0.9,
		"importance": 0.7, "emotional_significance": 0.3, "visibility": "private",
		"evidence_refs": []any{"fact-1"}, "idempotency_key": "memory:fact-1",
	}
	record, err := normalizeMemoryRecord("fl", base)
	if err != nil {
		t.Fatalf("normalizeMemoryRecord() error = %v", err)
	}
	if record.ID == "" || record.Type != "semantic" || record.Confidence != 0.9 {
		t.Fatalf("record = %#v", record)
	}
	delete(base, "evidence_refs")
	if _, err := normalizeMemoryRecord("fl", base); err == nil {
		t.Fatal("expected evidence requirement")
	}
	base["evidence_refs"] = []any{"fact-1"}
	base["confidence"] = 2
	if _, err := normalizeMemoryRecord("fl", base); err == nil {
		t.Fatal("expected confidence bound error")
	}
}

func TestNormalizeMemoryRecordDefaultsOptionalEmotionalSignificance(t *testing.T) {
	record, err := normalizeMemoryRecord("fl", map[string]any{
		"type": "semantic", "content": "用户喜欢安静", "confidence": 0.9,
		"importance": 0.7, "visibility": "owner",
		"evidence_refs": []any{"fact-1"}, "idempotency_key": "memory:optional-emotion",
	})
	if err != nil {
		t.Fatalf("normalizeMemoryRecord() error = %v", err)
	}
	if record.EmotionalSignificance != 0 {
		t.Fatalf("emotional significance = %v, want 0", record.EmotionalSignificance)
	}
}
