package core

import (
	"context"
	"os"
	"strings"
	"testing"
)

func TestPromptDiagnosticsAlwaysReturnsMutableMap(t *testing.T) {
	for name, ctx := range map[string]context.Context{
		"missing": context.Background(),
		"nil":     WithPromptDiagnostics(context.Background(), nil),
	} {
		diagnostics := providerPromptDiagnostics(ctx)
		diagnostics["prompt_budget"] = map[string]any{"estimated_input_tokens": 1}
		if mapValue(diagnostics["prompt_budget"])["estimated_input_tokens"] == nil {
			t.Fatalf("%s diagnostics map is not mutable: %#v", name, diagnostics)
		}
	}
}

func TestPromptDiagnosticsAreRedactedAndCollectionBounded(t *testing.T) {
	ranking := make([]any, 100)
	for index := range ranking {
		ranking[index] = map[string]any{"memory_id": "internal", "score": index, "api_key": "secret", "reasoning": "hidden"}
	}
	trace := boundedPromptDiagnostics(map[string]any{"fluctlight_id": "fl-1", "long_term_memory": map[string]any{"ranking": ranking}, "authorization": "Bearer secret"})
	values := arrayValue(mapValue(trace["long_term_memory"])["ranking"])
	if len(values) != 64 {
		t.Fatalf("bounded ranking length = %d", len(values))
	}
	encoded := jsonString(trace)
	if strings.Contains(encoded, "Bearer secret") || strings.Contains(encoded, `"secret"`) || strings.Contains(encoded, "hidden") || !strings.Contains(encoded, "[REDACTED]") {
		t.Fatalf("diagnostic redaction failed: %s", encoded)
	}
}

func TestProviderUsageAndWireBudgetDiagnosticsNormalizeActuals(t *testing.T) {
	usage := normalizeProviderUsage(map[string]any{"usage": map[string]any{"prompt_tokens": 123, "completion_tokens": 45, "total_tokens": 168, "private": "drop"}})
	if len(usage) != 3 || intValue(usage["prompt_tokens"]) != 123 || intValue(usage["completion_tokens"]) != 45 {
		t.Fatalf("usage = %#v", usage)
	}
	assignment := providerAssignment{TokenBudget: 4096, ContextWindowTokens: 65536, MaxInputTokens: 49152, PromptBudgetPolicyVersion: promptBudgetPolicyVersionV1}
	messages := []map[string]any{{"role": "system", "content": "stable"}, {"role": "user", "content": "[RUNTIME CONTEXT]\n{}\n[/RUNTIME CONTEXT]"}, {"role": "assistant", "content": "recent"}, {"role": "user", "content": "current"}}
	tools := []map[string]any{{"type": "function"}}
	schema := map[string]any{"type": "json_schema"}
	metrics := mergeProviderPromptBudgetDiagnostics(nil, messages, tools, schema, assignment, 321)
	if intValue(metrics["estimated_input_tokens"]) != 321 || intValue(metrics["output_reserve_tokens"]) != 4096 || intValue(mapValue(metrics["section_counts"])["runtime"]) != 1 || intValue(mapValue(metrics["section_counts"])["recent"]) != 1 || intValue(mapValue(metrics["section_counts"])["tools"]) != 1 || intValue(mapValue(metrics["section_counts"])["response_schema"]) != 1 {
		t.Fatalf("wire metrics = %#v", metrics)
	}
}

func TestDetailedPromptMetricsStayOutOfOrdinaryModelRunsAPIAndRemainPrunable(t *testing.T) {
	content, err := os.ReadFile("operations.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(content)
	start := strings.Index(text, "func (a *App) ModelRunsFiltered")
	end := strings.Index(text[start:], "func (a *App) MediaPromptsFiltered")
	if start < 0 || end < 0 {
		t.Fatal("ModelRuns API boundary not found")
	}
	modelRuns := text[start : start+end]
	if strings.Contains(modelRuns, "metrics") || strings.Contains(modelRuns, "actual_prompt_tokens") || strings.Contains(modelRuns, "fluctlight_id") {
		t.Fatalf("detailed prompt metrics leaked into ordinary ModelRuns API: %s", modelRuns)
	}
	if !strings.Contains(text, `"diagnostic_model_runs"`) || !strings.Contains(text, `"DELETE FROM public."+table`) {
		t.Fatal("diagnostic model runs are no longer covered by clear/prune authority")
	}
	// Diagnostics are explicitly non-fatal when their store is unavailable.
	(&App{}).recordDiagnosticEvent(nil, "prompt.test", "info", "", "", "", map[string]any{"ok": true})
}
