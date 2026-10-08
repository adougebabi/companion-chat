package core

import (
	"strings"
	"testing"
)

func TestGoalEvaluationPersonaAndRuntimeOmitNestedBookkeeping(t *testing.T) {
	persona := map[string]any{"profile_id": "profile-secret", "working_persona": map[string]any{"profile_id": "profile-secret", "core_mechanisms": []string{`{"id":"profile-secret","name":"晴岚","trait":"尊重对方","score":0,"enabled":false}`}}}
	before := jsonString(persona)
	wire := jsonString(goalEvaluationPersonaSemantics(persona))
	for _, forbidden := range []string{"profile-secret", "profile_id"} {
		if strings.Contains(wire, forbidden) {
			t.Fatalf("portrait bookkeeping leaked: %s", wire)
		}
	}
	for _, required := range []string{"晴岚", "尊重对方", "false"} {
		if !strings.Contains(wire, required) {
			t.Fatalf("portrait semantics lost: %s", wire)
		}
	}
	if jsonString(persona) != before {
		t.Fatal("semantic projection mutated compiled persona")
	}
	execution := map[string]any{"stage": "waiting", "current_stage": map[string]any{"id": "stage-secret", "criteria_version": 8, "revision": 7, "status": "active", "purpose": "真实沟通"}, "commitments": []any{map[string]any{"id": "commit-secret", "profile_id": "profile-secret", "window_end": "2026-10-09", "status": "blocked"}}}
	view := jsonString(goalRuntimeSemantics(execution))
	for _, forbidden := range []string{"stage-secret", "commit-secret", "profile-secret", "criteria_version", "revision", "profile_id"} {
		if strings.Contains(view, forbidden) {
			t.Fatalf("execution bookkeeping leaked: %s", view)
		}
	}
	for _, required := range []string{"waiting", "active", "真实沟通", "2026-10-09", "blocked"} {
		if !strings.Contains(view, required) {
			t.Fatalf("execution semantics lost: %s", view)
		}
	}
}
