package bff

import (
	"encoding/json"
	"testing"
)

func TestBrowserDiagnosticModelRunPreservesPromptArrays(t *testing.T) {
	row := map[string]any{
		"id":       "run-1",
		"role":     "cognitive_assessment",
		"model_id": "model-1",
		"prompt": []any{
			map[string]any{"role": "system", "content": "Return JSON"},
			map[string]any{"role": "user", "content": "What happened?"},
		},
		"status":         "completed",
		"correlation_id": "corr-1",
		"created_at":     "2026-09-01T00:00:00Z",
	}

	mapped := browserDiagnosticModelRun(row)
	prompt, ok := mapped["prompt"].([]any)
	if !ok || len(prompt) != 2 {
		t.Fatalf("prompt = %#v, want two message entries", mapped["prompt"])
	}
}

func TestBrowserDiagnosticModelRunDecodesRawJSONPrompt(t *testing.T) {
	row := map[string]any{
		"id":             "run-2",
		"role":           "reflection",
		"model_id":       "model-2",
		"prompt":         json.RawMessage(`[{"role":"user","content":"Reflect"}]`),
		"status":         "completed",
		"correlation_id": "corr-2",
		"created_at":     "2026-09-01T00:00:00Z",
	}

	mapped := browserDiagnosticModelRun(row)
	prompt, ok := mapped["prompt"].([]any)
	if !ok || len(prompt) != 1 {
		t.Fatalf("raw prompt = %#v, want one decoded message", mapped["prompt"])
	}
}

func TestBrowserDiagnosticModelRunMapsQueueLifecycleFields(t *testing.T) {
	row := map[string]any{
		"id": "run-3", "role": "generic_llm", "binding_role": "generic_llm", "scenario": "wake_up", "priority": 70,
		"model_id": "model-3", "prompt": json.RawMessage(`{}`), "status": "running", "correlation_id": "corr-3",
		"created_at": "2026-09-01T00:00:00Z", "queued_at": "2026-09-01T00:00:01Z", "started_at": "2026-09-01T00:00:02Z",
	}
	mapped := browserDiagnosticModelRun(row)
	if mapped["scenario"] != "wake_up" || mapped["bindingRole"] != "generic_llm" || mapped["priority"] != 70 || mapped["startedAt"] != row["started_at"] {
		t.Fatalf("queue lifecycle fields = %#v", mapped)
	}
}
