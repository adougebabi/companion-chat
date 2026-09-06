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

func TestNormalizeMemoryRecordCanonicalizesPersonalityPerspectives(t *testing.T) {
	record, err := normalizeMemoryRecord("fl", map[string]any{
		"type": "episodic", "content": "actor_user 昨天很累", "confidence": 0.9,
		"importance": 0.7, "evidence_refs": []any{"fact-1"}, "source_fact_id": "fact-1",
		"personality_perspectives": []any{map[string]any{
			"profile_id": "warm", "interpretation": "应该主动关心", "emotion": "担心",
		}}, "idempotency_key": "memory:perspective",
	})
	if err != nil {
		t.Fatalf("normalizeMemoryRecord() error = %v", err)
	}
	if len(record.PersonalityPerspectives) != 1 || !containsStringValue(arrayValue(mapValue(record.PersonalityPerspectives[0])["evidence_refs"]), "fact-1") {
		t.Fatalf("perspectives = %#v", record.PersonalityPerspectives)
	}
	if _, err := normalizeMemoryRecord("fl", map[string]any{
		"type": "episodic", "content": "重复视角", "confidence": 0.9, "importance": 0.7,
		"evidence_refs": []any{"fact-1"}, "idempotency_key": "memory:duplicate-perspective",
		"personality_perspectives": []any{
			map[string]any{"profile_id": "warm", "interpretation": "a"},
			map[string]any{"profile_id": "warm", "interpretation": "b"},
		},
	}); err == nil {
		t.Fatal("expected duplicate profile perspective rejection")
	}
}

func TestValidatePersonalityPerspectiveProfiles(t *testing.T) {
	persona := map[string]any{"personality_system": map[string]any{"profiles": []any{map[string]any{"id": "warm"}}}}
	if !validatePersonalityPerspectiveProfiles(persona, []any{map[string]any{"profile_id": "warm"}}) {
		t.Fatal("expected declared profile to be accepted")
	}
	if validatePersonalityPerspectiveProfiles(persona, []any{map[string]any{"profile_id": "unknown"}}) {
		t.Fatal("expected unknown profile to be rejected")
	}
}
