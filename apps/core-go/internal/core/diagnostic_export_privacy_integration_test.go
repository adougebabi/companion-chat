package core

import (
	"bytes"
	"encoding/json"
	"net/url"
	"strings"
	"testing"
)

func TestDiagnosticExportsExcludePrivateContentAndIdentifiers(t *testing.T) {
	ctx, repo, app, owner, fid, _ := setupMixedMediaReplyTurnWithTransport(t, "private-export", nil)
	privateText := "PRIVATE_USER_MESSAGE_74291"
	privateKey := "PRIVATE_DYNAMIC_KEY_74291"
	credential := "Bearer PRIVATE_BEARER_74291"
	correlation := "private-correlation-74291"
	prompt := []map[string]any{{"role": "user", "content": privateText, privateKey: "user supplied key"}}
	response := map[string]any{"content": "PRIVATE_ASSISTANT_74291", "reasoning_content": "PRIVATE_REASONING_74291", "tool_calls": []any{map[string]any{"arguments": "PRIVATE_TOOL_ARGUMENT_74291"}}}
	writeCtx := WithProviderAttemptIdentity(WithProviderScenario(ctx, "private-export-test"), "private-export-attempt")
	modelRunID, err := app.persistModelRunLifecycle(writeCtx, "cognitive_assessment", "PRIVATE_ENDPOINT_74291", "PRIVATE_MODEL_74291", correlation, "private-export-test", 80, prompt, response, providerRunCompleted, credential)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.diagnostic_events(id,event_type,severity,fluctlight_id,correlation_id,payload) VALUES('private-event','provider.fixture','error',$1,$2,$3)`, fid, correlation,
		jsonBytes(map[string]any{"run_id": correlation, "model_run_id": modelRunID, "provider_attempt_id": "private-export-attempt", "attempt": 739184250611, "error": credential, privateKey: privateText, "prompt": privateText})); err != nil {
		t.Fatal(err)
	}
	var stream bytes.Buffer
	app.DiagnosticDebugReadEnabled = true // Explicit authorization for raw diagnostic assertions.
	if err := app.ExportOwnerDiagnostics(ctx, owner, url.Values{"dataset": {"all"}, "correlationId": {correlation}, "q": {privateText}}, &stream); err != nil {
		t.Fatal(err)
	}
	legacy, err := app.DiagnosticsExportFiltered(ctx, owner, LifecycleDiagnosticsFilter{CorrelationID: correlation, Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	for name, output := range map[string]string{"stream": stream.String(), "legacy": jsonString(legacy)} {
		for _, forbidden := range []string{privateText, privateKey, credential, "PRIVATE_BEARER_74291", "PRIVATE_ASSISTANT_74291", "PRIVATE_REASONING_74291", "PRIVATE_TOOL_ARGUMENT_74291", "PRIVATE_ENDPOINT_74291", "PRIVATE_MODEL_74291", "739184250611", correlation, owner, fid} {
			if strings.Contains(output, forbidden) {
				t.Errorf("%s export leaked %q", name, forbidden)
			}
		}
	}
	lines := strings.Split(strings.TrimSpace(stream.String()), "\n")
	if len(lines) < 3 {
		t.Fatalf("stream lost records: lines=%d", len(lines))
	}
	var manifest, end map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &manifest); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &end); err != nil {
		t.Fatal(err)
	}
	if manifest["type"] != "manifest" || end["complete"] != true || manifest["snapshot"] != end["snapshot"] || intValue(mapValue(end["counts"])["model-runs"]) != 1 {
		t.Fatalf("privacy projection broke export completeness: manifest=%#v end=%#v", manifest, end)
	}
	var correlations []string
	var eventModel, modelID, eventAttempt, modelAttempt string
	for _, line := range lines[1 : len(lines)-1] {
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatal(err)
		}
		data := mapValue(mapValue(entry["record"])["data"])
		switch entry["dataset"] {
		case "events":
			eventModel, eventAttempt = stringValue(data["model_run_id"]), stringValue(data["provider_attempt_id"])
		case "model-runs":
			modelID, modelAttempt = stringValue(data["id"]), stringValue(data["provider_attempt_id"])
		}
		if value := stringValue(data["correlation_id"]); value != "" {
			correlations = append(correlations, value)
		}
	}
	if len(correlations) < 2 || !strings.HasPrefix(correlations[0], "anon_") {
		t.Fatalf("export lost pseudonymous trace linkage: %#v", correlations)
	}
	for _, value := range correlations[1:] {
		if value != correlations[0] {
			t.Fatalf("same trace identity changed within export: %#v", correlations)
		}
	}
	if eventModel == "" || eventModel != modelID || eventAttempt == "" || eventAttempt != modelAttempt {
		t.Fatalf("model attempt linkage lost: event=%s/%s model=%s/%s", eventModel, eventAttempt, modelID, modelAttempt)
	}
	legacyEvents := arrayValue(legacy["events"])
	if len(legacyEvents) == 0 || !strings.HasPrefix(stringValue(mapValue(legacyEvents[0])["fluctlight_id"]), "anon_") || mapValue(legacy["filters"])["limit"] != "[REDACTED]" {
		t.Fatalf("legacy export lost nullable linkage or exposed a filter: %#v", legacy)
	}
	ownerRuns, err := app.ModelRunsFiltered(ctx, owner, 10, correlation)
	if err != nil || len(ownerRuns) != 1 || !strings.Contains(jsonString(ownerRuns), privateText) {
		t.Fatal("export projection changed the separate Owner diagnostic view", err)
	}
}
