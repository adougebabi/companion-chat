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
		"id": "run-3", "role": "generic_llm", "binding_role": "generic_llm", "scenario": "wake_up", "priority": 70, "queue_pending_count": int64(3), "queue_position": int64(2),
		"model_id": "model-3", "prompt": json.RawMessage(`{}`), "status": "running", "correlation_id": "corr-3",
		"created_at": "2026-09-01T00:00:00Z", "queued_at": "2026-09-01T00:00:01Z", "started_at": "2026-09-01T00:00:02Z",
	}
	mapped := browserDiagnosticModelRun(row)
	if mapped["scenario"] != "wake_up" || mapped["bindingRole"] != "generic_llm" || mapped["priority"] != 70 || mapped["queuePendingCount"] != int64(3) || mapped["queuePosition"] != int64(2) || mapped["startedAt"] != row["started_at"] {
		t.Fatalf("queue lifecycle fields = %#v", mapped)
	}
}

func TestBrowserDiagnosticMediaPromptMapsProviderAndSubmittedPrompts(t *testing.T) {
	row := map[string]any{
		"id": "media-1", "media_intent_id": "media-1", "fluctlight_id": "fl-1", "kind": "image", "mime_type": "image/png",
		"prompt": json.RawMessage(`{"scene":"cafe"}`), "provider_prompt": "A person in a cafe", "submitted_prompt": "A person in a cafe, high detail",
		"provider_request_id": "request-1", "provider_job_id": "job-1", "workflow_id": "workflow-1", "status": "running",
		"correlation_id": "media:media-1", "created_at": "2026-09-01T00:00:00Z", "submitted_event_id": "event-1", "submitted_at": "2026-09-01T00:00:01Z",
		"model_run": map[string]any{"id": "run-1", "role": "media_prompt", "binding_role": "generic_llm", "scenario": "media_prompt", "model_id": "model-1", "prompt": json.RawMessage(`[]`), "status": "completed", "correlation_id": "media:media-1", "created_at": "2026-09-01T00:00:00Z"},
	}
	mapped := browserDiagnosticMediaPrompt(row)
	if mapped["mediaIntentId"] != "media-1" || mapped["providerPrompt"] != "A person in a cafe" || mapped["submittedPrompt"] != "A person in a cafe, high detail" || mapped["correlationId"] != "media:media-1" {
		t.Fatalf("media prompt mapping = %#v", mapped)
	}
	modelRun, ok := mapped["modelRun"].(map[string]any)
	if !ok || modelRun["role"] != "media_prompt" || modelRun["bindingRole"] != "generic_llm" {
		t.Fatalf("nested model run = %#v", mapped["modelRun"])
	}
}
