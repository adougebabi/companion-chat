package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/jackc/pgx/v5"
)

func TestADKTerminationReasonSeparatesAgentFailuresFromProviderTransport(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{name: "final", err: nil, want: "final_message"},
		{name: "cancelled", err: context.Canceled, want: "request_cancelled"},
		{name: "timeout", err: context.DeadlineExceeded, want: "request_timeout"},
		{name: "provider transport", err: fmt.Errorf("%w: unavailable", errProviderRequestFailed), want: "provider_request_failed"},
		{name: "invalid tool call", err: errors.New("tool_call_invalid"), want: "tool_call_invalid"},
		{name: "tool execution", err: errors.New("adk_run: tool execution tool_execution_failed: unavailable"), want: "tool_execution_failed"},
		{name: "agent failure", err: errors.New("tool execution failed"), want: "adk_run_failed"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := adkTerminationReason(testCase.err); got != testCase.want {
				t.Fatalf("adkTerminationReason(%v) = %q, want %q", testCase.err, got, testCase.want)
			}
		})
	}
}

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

func TestPromptDiagnosticsKeepTextAndCollectionBounded(t *testing.T) {
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
	if !strings.Contains(encoded, "Bearer secret") || !strings.Contains(encoded, `"secret"`) || !strings.Contains(encoded, "hidden") || strings.Contains(encoded, "[REDACTED]") {
		t.Fatalf("diagnostic text was changed: %s", encoded)
	}
}

func TestDiagnosticPreservesNonImageTextAndToolArguments(t *testing.T) {
	value := redactDiagnostic(map[string]any{
		"authorization":     "Bearer private-token",
		"reasoning_content": "reasoning text",
		"arguments":         `{"place":"library"}`,
		"content":           `{"tool_calls":[{"function":{"arguments":"private text"}}]}`,
		"inline_image":      "see data:image/png;base64,ZmFrZQ== now",
		"typed_images":      []string{"data:image/png;base64,ZmFrZQ=="},
		"typed_map":         map[string]string{"image": "data:image/png;base64,ZmFrZQ==", "note": "keep me"},
		"remote_image":      map[string]any{"type": "image_url", "image_url": map[string]any{"url": "https://media.invalid/private-image?grant=secret", "detail": "high"}},
		"image_base64":      "aW1hZ2VieXRlcw==",
		"image_url":         map[string]any{"url": "data:image/png;base64,ZmFrZQ=="},
	})
	encoded := jsonString(value)
	for _, expected := range []string{"Bearer private-token", "reasoning text", "library", "private text", "keep me", "REDACTED_IMAGE_DATA"} {
		if !strings.Contains(encoded, expected) {
			t.Fatalf("missing %q in diagnostic: %s", expected, encoded)
		}
	}
	if strings.Contains(encoded, "ZmFrZQ==") || strings.Contains(encoded, "aW1hZ2VieXRlcw==") || strings.Contains(encoded, "media.invalid") || strings.Contains(encoded, "[REDACTED]") || strings.Contains(encoded, "[REDACTED_STRUCTURED_TOOL_CALLS]") {
		t.Fatalf("unexpected diagnostic replacement: %s", encoded)
	}
}

func TestEinoDiagnosticMessagesKeepMultimodalTextWithImageMarker(t *testing.T) {
	imageURL := "data:image/png;base64,ZmFrZQ=="
	message := &schema.Message{Role: schema.User, UserInputMultiContent: []schema.MessageInputPart{
		{Type: schema.ChatMessagePartTypeText, Text: "frozen_media_concept:\n  subject: companion"},
		{Type: schema.ChatMessagePartTypeImageURL, Image: &schema.MessageInputImage{MessagePartCommon: schema.MessagePartCommon{URL: &imageURL}}},
	}}
	encoded := jsonString(redactDiagnostic(einoDiagnosticMessages([]*schema.Message{message})))
	if !strings.Contains(encoded, "frozen_media_concept:") || !strings.Contains(encoded, "REDACTED_IMAGE_DATA") || strings.Contains(encoded, "ZmFrZQ==") {
		t.Fatalf("multimodal diagnostic differs from model input: %s", encoded)
	}
}

func TestPostgresOwnerModelRunWriteReadExportReplacesOnlyImages(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID := "image-diag-owner", "image-diag-fluctlight"
	seedLifeContextFluctlight(t, ctx, repository, ownerID, fluctlightID)
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'hash','revision-1')`, ownerID); err != nil {
		t.Fatal(err)
	}
	correlationID := "image-diag:" + stableDigest(t.Name())
	prompt := []map[string]any{{"role": "user", "content": []any{
		map[string]any{"type": "text", "text": "private prompt with Bearer keep-me"},
		map[string]any{"type": "image_url", "image_url": map[string]any{"url": "https://media.invalid/signed-image?grant=hide-me", "detail": "high"}},
	}}}
	response := map[string]any{"content": "private answer", "reasoning_content": "private reasoning", "tool_calls": []any{map[string]any{"function": map[string]any{"arguments": `{"note":"private argument"}`}}}}
	writeCtx := WithProviderAttemptIdentity(WithProviderScenario(ctx, "media_quality_acceptance"), "image-diag-attempt")
	id, err := (&App{DB: repository}).persistModelRunLifecycle(writeCtx, "media_prompt", "", "test-model", correlationID, "media_quality_acceptance", 80, prompt, response, providerRunCompleted, "")
	if err != nil || id == "" {
		t.Fatalf("persist model run id=%q err=%v", id, err)
	}
	var stored []byte
	if err := repository.Pool().QueryRow(ctx, `SELECT prompt FROM public.diagnostic_model_runs WHERE id=$1`, id).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	app := &App{DB: repository}
	runs, err := app.ModelRunsFiltered(ctx, ownerID, 10, correlationID)
	if err != nil || len(runs) != 1 {
		t.Fatalf("Owner runs=%#v err=%v", runs, err)
	}
	exported, err := app.DiagnosticsExportFiltered(ctx, ownerID, LifecycleDiagnosticsFilter{CorrelationID: correlationID, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	for _, encoded := range []string{string(stored), jsonString(runs), jsonString(exported)} {
		for _, forbidden := range []string{"media.invalid", "hide-me"} {
			if strings.Contains(encoded, forbidden) {
				t.Fatalf("image locator %q reached diagnostics: %s", forbidden, encoded)
			}
		}
		for _, necessary := range []string{"REDACTED_IMAGE_DATA", "Bearer keep-me", "private prompt"} {
			if !strings.Contains(encoded, necessary) {
				t.Fatalf("Owner diagnostic lost %q: %s", necessary, encoded)
			}
		}
	}
	if exportedText := jsonString(exported); !strings.Contains(exportedText, "private reasoning") || !strings.Contains(exportedText, "private argument") {
		t.Fatalf("Owner export lost non-image response: %s", exportedText)
	}
}

func TestPromptDiagnosticsKeepCoreSourceRefs(t *testing.T) {
	trace := boundedPromptDiagnostics(map[string]any{
		"working_memory": map[string]any{"selected": []any{
			map[string]any{"kind": "summary", "source_refs": []string{"summary:ctx_safe", "message:message_internal_123"}},
		}},
	})
	encoded := jsonString(trace)
	if !strings.Contains(encoded, "ctx_safe") || !strings.Contains(encoded, "message_internal_123") || strings.Contains(encoded, ":diag_") {
		t.Fatalf("Core source refs changed in Owner diagnostics: %s", encoded)
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

func TestInitializationScenarioUsesFidelityBudgetAndTimeout(t *testing.T) {
	configured := providerAssignment{
		Timeout: 300 * time.Second, TokenBudget: 4096,
		ContextWindowTokens: 65536, MaxInputTokens: 49152,
		PromptBudgetPolicyVersion: promptBudgetPolicyVersionV1,
	}
	effective, err := providerAssignmentForScenario(configured, "initialization")
	if err != nil {
		t.Fatal(err)
	}
	if effective.TokenBudget != configured.TokenBudget || effective.Timeout != initializationMinimumRequestTimeout {
		t.Fatalf("initialization assignment = %#v", effective)
	}
	ordinary, err := providerAssignmentForScenario(configured, "wake_up")
	if err != nil || ordinary.TokenBudget != configured.TokenBudget || ordinary.Timeout != configured.Timeout {
		t.Fatalf("ordinary assignment changed: %#v, %v", ordinary, err)
	}
	insufficient := configured
	insufficient.ContextWindowTokens = 55000
	preserved, err := providerAssignmentForScenario(insufficient, "initialization")
	if err != nil || preserved.TokenBudget != insufficient.TokenBudget || preserved.Timeout != initializationMinimumRequestTimeout {
		t.Fatalf("insufficient initialization context = %#v, %v", preserved, err)
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

func TestModelRunPersistenceDoesNotReturnGhostID(t *testing.T) {
	id, err := (&App{}).persistModelRunLifecycle(
		context.Background(),
		"cognitive_assessment",
		"endpoint-1",
		"model-1",
		"turn:1",
		"reply",
		100,
		map[string]any{"messages": []any{}},
		nil,
		providerRunQueued,
		"",
	)
	if id != "" || !errors.Is(err, ErrDiagnosticsUnavailable) {
		t.Fatalf("persistModelRunLifecycle() = %q, %v; want no ghost ID and diagnostics unavailable", id, err)
	}
}

func TestDiagnosticPersistenceWarningIncludesBoundedSafeCause(t *testing.T) {
	source, err := os.ReadFile("lifecycle_diagnostics.go")
	if err != nil {
		t.Fatal(err)
	}
	body := sourceBetween(t, string(source), "func recordDiagnosticPersistenceFailure", "func validLifecycleIdentity")
	if !strings.Contains(body, `"safe_cause", boundedOperationalCause(err.Error())`) {
		t.Fatal("diagnostic persistence warning omits the bounded failure cause")
	}
	if got := boundedOperationalCause("Authorization: Bearer private-token"); got != "[REDACTED]" {
		t.Fatalf("operational warning exposed credential: %q", got)
	}
}

func TestProviderAttemptIdentityIsStableWithinInvocationAndDistinctAcrossRetries(t *testing.T) {
	explicit := WithProviderAttemptIdentity(context.Background(), " activity-1:2 ")
	if got := providerAttemptIdentity(ensureProviderAttemptIdentity(explicit)); got != "activity-1:2" {
		t.Fatalf("explicit Provider attempt identity = %q", got)
	}
	first := ensureProviderAttemptIdentity(context.Background())
	firstAgain := ensureProviderAttemptIdentity(first)
	second := ensureProviderAttemptIdentity(context.Background())
	if providerAttemptIdentity(first) == "" || providerAttemptIdentity(first) != providerAttemptIdentity(firstAgain) {
		t.Fatal("one Provider invocation did not retain one attempt identity")
	}
	if providerAttemptIdentity(first) == providerAttemptIdentity(second) {
		t.Fatal("separate Provider retries reused one attempt identity")
	}
}

func TestProviderModelRunIdentitySeparatesAttempts(t *testing.T) {
	prompt := []byte(`[{"role":"user","content":"same prompt"}]`)
	first, _ := providerModelRunIdentity("generic_llm", "endpoint-1", "model-1", "wake_up", "wake_up:fl-1:cycle:2", "attempt-1", prompt)
	firstTerminal, _ := providerModelRunIdentity("generic_llm", "endpoint-1", "model-1", "wake_up", "wake_up:fl-1:cycle:2", "attempt-1", prompt)
	second, _ := providerModelRunIdentity("generic_llm", "endpoint-1", "model-1", "wake_up", "wake_up:fl-1:cycle:2", "attempt-2", prompt)
	if first != firstTerminal {
		t.Fatalf("queued and terminal writes changed identity: %q != %q", first, firstTerminal)
	}
	if first == second {
		t.Fatalf("Provider retries collapsed into one model-run row: %q", first)
	}
}

func TestProviderRequestIdentityStaysStableAcrossDiagnosticAttempts(t *testing.T) {
	first := providerDiagnosticRequestID("cognitive_assessment", "wake_up:fl-1:cycle:2")
	second := providerDiagnosticRequestID("cognitive_assessment", "wake_up:fl-1:cycle:2")
	if first == "" || first != second {
		t.Fatalf("Provider request identity is not stable: %q != %q", first, second)
	}
}

func TestProviderPreflightFailuresAreDiagnosedBeforeQueue(t *testing.T) {
	source, err := os.ReadFile("provider.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	start := strings.Index(text, "func (p *ProviderClient) completeWithToolsSchemaMode")
	end := strings.Index(text, "func addVisualIdentityMediaPromptInstruction")
	if start < 0 || end <= start {
		t.Fatal("Provider completion boundary not found")
	}
	body := text[start:end]
	for _, stage := range []string{`"assignment"`, `"message_validation"`, `"payload_encode"`} {
		if !strings.Contains(body, "recordProviderPreflightFailure") || !strings.Contains(body, stage) {
			t.Fatalf("Provider preflight stage %s is not diagnosed", stage)
		}
	}
	if strings.Contains(body, `"wire_budget"`) || strings.Contains(body, "ErrPromptRequiredBudgetExceeded") {
		t.Fatal("Provider request path still intercepts an estimated token budget")
	}
	queuedMarker := "RecordQueuedModelRun"
	if strings.Index(body, queuedMarker) < 0 {
		queuedMarker = "recordQueuedModelRun"
	}
	if strings.Index(body, "ensureProviderAttemptIdentity") > strings.Index(body, queuedMarker) {
		t.Fatal("Provider attempt identity is created after the queued diagnostic")
	}
}

func TestProviderPreflightDiagnosticsKeepSourceText(t *testing.T) {
	const canary = "PRIVATE_CHARACTER_CARD_CANARY"
	prompt := providerPreflightDiagnosticPrompt([]map[string]any{{"role": "user", "content": canary}})
	encoded := jsonString(prompt)
	if !strings.Contains(encoded, canary) || strings.Contains(encoded, "metadata_only") {
		t.Fatalf("Provider preflight diagnostic replaced source content: %s", encoded)
	}
}

func TestProviderModelRunLifecycleCannotRegressFromTerminalToQueued(t *testing.T) {
	source, err := os.ReadFile("diagnostics.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, required := range []string{
		"public.diagnostic_model_runs.status IN ('completed','failed','cancelled','timeout')",
		"public.diagnostic_model_runs.status='running' AND excluded.status='queued'",
		"(status='running' AND $2 IN ('completed','failed','cancelled','timeout'))",
		"OR status=$2",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("model-run lifecycle monotonicity guard missing %q", required)
		}
	}
}

func TestPostgresModelRunLateTerminalCallbackIsAnIdempotentNoop(t *testing.T) {
	databaseURL := os.Getenv("GO_CORE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("GO_CORE_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	repository, err := NewPostgresRepository(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()

	modelRunID := "model_run_" + stableDigest(t.Name()+time.Now().UTC().Format(time.RFC3339Nano))
	if _, err := repository.Pool().Exec(ctx, `
		INSERT INTO public.diagnostic_model_runs(
			id,role,binding_role,scenario,priority,model_id,prompt,status,
			error_code,correlation_id,metrics,completed_at
		) VALUES($1,'initialization','generic_llm','initialization',80,
			'model-test','{}','failed','provider_http_error',$2,'{}',now())`,
		modelRunID, "initialization-analysis:test-late-terminal"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = repository.Pool().Exec(context.Background(), `DELETE FROM public.diagnostic_model_runs WHERE id=$1`, modelRunID)
	})

	lifecycleDiagnosticWarningState.Lock()
	delete(lifecycleDiagnosticWarningState.last, "model_run:state:*errors.errorString")
	lifecycleDiagnosticWarningState.Unlock()
	previousLogger := slog.Default()
	var logs bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previousLogger) })

	(&App{DB: repository}).updateModelRunState(ctx, modelRunID, providerRunTimeout, context.DeadlineExceeded)
	if strings.Contains(logs.String(), "diagnostic_model_run_state_not_written") {
		t.Fatalf("late terminal callback was misreported as missing persistence: %s", logs.String())
	}
	var status, errorCode string
	if err := repository.Pool().QueryRow(ctx, `SELECT status,COALESCE(error_code,'') FROM public.diagnostic_model_runs WHERE id=$1`, modelRunID).Scan(&status, &errorCode); err != nil {
		t.Fatal(err)
	}
	if status != providerRunFailed || errorCode != "provider_http_error" {
		t.Fatalf("first terminal state changed to status=%q error_code=%q", status, errorCode)
	}
}

func TestPostgresADKModelRunsKeepPhysicalRoundsAndSafeToolSummary(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID := "adk-round-owner", "adk-round-fluctlight"
	seedLifeContextFluctlight(t, ctx, repository, ownerID, fluctlightID)
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'hash','revision-1')`, ownerID); err != nil {
		t.Fatal(err)
	}
	correlationID := "adk-rounds:test"
	for index, callID := range []string{"call-one", "call-two"} {
		response := map[string]any{"content": "最终回答"}
		if index == 0 {
			response = map[string]any{"tool_calls": []any{map[string]any{"id": "tool-one", "function": map[string]any{"arguments": "private argument"}}}, "reasoning_content": "private reasoning"}
		}
		if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.diagnostic_model_runs(id,role,binding_role,scenario,model_id,prompt,response,status,correlation_id,metrics) VALUES($1,'cognitive_assessment','generic_llm','cognitive_assessment','test-model',$2,$3,'completed',$4,$5)`, "adk-round-"+callID, jsonBytes(map[string]any{"messages": []any{}}), jsonBytes(response), correlationID, jsonBytes(map[string]any{"run_id": "logical-adk-rounds", "model_call_id": callID})); err != nil {
			t.Fatal(err)
		}
		for _, event := range []struct {
			kind    string
			payload map[string]any
		}{
			{"adk.model.input", map[string]any{"model_call_id": callID, "sequence": index + 1}},
			{"adk.model.output", map[string]any{"model_call_id": callID, "sequence": index + 1, "status": "completed", "tool_call_ids": map[bool][]any{true: {"tool-one"}, false: {}}[index == 0]}},
		} {
			if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.diagnostic_events(id,event_type,severity,correlation_id,payload) VALUES($1,$2,'info',$3,$4)`, "adk-round-event-"+callID+"-"+event.kind, event.kind, correlationID, jsonBytes(event.payload)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.diagnostic_model_runs(id,role,binding_role,scenario,model_id,prompt,response,status,correlation_id,metrics) VALUES('adk-outer-summary','cognitive_assessment','generic_llm','cognitive_assessment','test-model','{}','{}','completed',$1,$2)`, correlationID, jsonBytes(map[string]any{"run_id": "logical-adk-rounds", "provider_attempt_id": "parent-attempt"})); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.diagnostic_events(id,event_type,severity,correlation_id,payload) VALUES('adk-round-tool','adk.tool.result','info',$1,$2)`, correlationID, jsonBytes(map[string]any{"model_call_id": "call-one", "call_id": "tool-one", "capability": "scene_event", "status": "completed", "arguments_digest": "secret-digest"})); err != nil {
		t.Fatal(err)
	}
	runs, err := (&App{DB: repository}).modelRunsFiltered(ctx, ownerID, 10, correlationID, "")
	if err != nil || len(runs) != 2 {
		t.Fatalf("model runs: rows=%#v err=%v", runs, err)
	}
	byCall := map[string]map[string]any{}
	for _, run := range runs {
		byCall[stringValue(run["model_call_id"])] = run
	}
	if encoded := jsonString(byCall["call-one"]); !strings.Contains(encoded, "private argument") || !strings.Contains(encoded, "private reasoning") || strings.Contains(encoded, "secret-digest") {
		t.Fatalf("first model response text or tool summary changed: %s", encoded)
	}
	if intValue(byCall["call-one"]["sequence"]) != 1 || byCall["call-one"]["round_count"] != int64(2) || stringValue(byCall["call-one"]["stage"]) != "tool_request" {
		t.Fatalf("first round mislabeled: %#v", byCall["call-one"])
	}
	if stringValue(byCall["call-two"]["stage"]) != "final_response" || intValue(byCall["call-two"]["sequence"]) != 2 {
		t.Fatalf("final round mislabeled: %#v", byCall["call-two"])
	}
	tools, ok := byCall["call-one"]["tool_summaries"].([]map[string]any)
	if !ok || len(tools) != 1 || stringValue(tools[0]["capability"]) != "scene_event" {
		t.Fatalf("safe Tool summary missing: %#v", tools)
	}
}

func TestPostgresADKToolDiagnosticUsesRecordedPhysicalCallIdentity(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID := "tool-link-owner", "tool-link-fluctlight"
	seedLifeContextFluctlight(t, ctx, repository, ownerID, fluctlightID)
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'hash','revision-1')`, ownerID); err != nil {
		t.Fatal(err)
	}
	trace := &ADKCapabilityTrace{}
	trace.RecordModelToolCalls("physical-request-1", 1, []schema.ToolCall{{ID: "tool-1", Function: schema.FunctionCall{Name: "scene_event"}}})
	invoker := &appADKCapabilityInvoker{app: &App{DB: repository}, request: ADKCapabilityRequest{FluctlightID: fluctlightID, CorrelationID: "agent-correlation-1", Surface: CapabilitySurfaceConversation}, trace: trace}
	invoker.recordADKToolDiagnostic(ctx, "adk.tool.result", "tool-1", "scene_event", "failed", "scene_plan_invalid", `{"location":"PRIVATE"}`)
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.diagnostic_model_runs(id,role,binding_role,scenario,model_id,prompt,response,status,correlation_id,metrics) VALUES('tool-link-model','cognitive_assessment','generic_llm','cognitive_assessment','test-model','{}','{}','completed','agent-correlation-1',$1)`, jsonBytes(map[string]any{"run_id": "agent-correlation-1", "model_call_id": "physical-request-1"})); err != nil {
		t.Fatal(err)
	}
	var payload []byte
	if err := repository.Pool().QueryRow(ctx, `SELECT payload FROM public.diagnostic_events WHERE correlation_id='agent-correlation-1' AND event_type='adk.tool.result' ORDER BY created_at DESC LIMIT 1`).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	decoded := decodeObject(payload)
	if decoded["model_call_id"] != "physical-request-1" || decoded["run_id"] != "agent-correlation-1" || !strings.Contains(string(payload), "PRIVATE") {
		t.Fatalf("Tool event did not retain physical call identity and arguments: %s", payload)
	}
	runs, err := (&App{DB: repository}).ModelRunsFiltered(ctx, ownerID, 10, "agent-correlation-1")
	if err != nil || len(runs) != 1 {
		t.Fatalf("physical rounds: %#v %v", runs, err)
	}
	summaries, ok := runs[0]["tool_summaries"].([]map[string]any)
	if !ok || len(summaries) != 1 || summaries[0]["call_id"] != "tool-1" || summaries[0]["error_code"] != "scene_plan_invalid" {
		t.Fatalf("real Tool callback not attached to physical model run: %#v", runs[0]["tool_summaries"])
	}
}

func TestPostgresAgentRunDiagnosticsShowLogicalFailureWithoutChangingPhysicalStatus(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID := "agent-diagnostic-owner", "agent-diagnostic-fluctlight"
	seedLifeContextFluctlight(t, ctx, repository, ownerID, fluctlightID)
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'hash','revision-1')`, ownerID); err != nil {
		t.Fatal(err)
	}
	correlationID := "agent-diagnostic:failed"
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.agent_runs(fluctlight_id,agent_id,run_id,input_digest,correlation_id,status,failure_stage,failure_code,error_detail,finished_at) VALUES($1,'conversation_cognition','agent-run-1','digest',$2,'failed','tool','capability_prepare_failed','tool execution capability_prepare_failed: invalid arguments',now())`, fluctlightID, correlationID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.diagnostic_model_runs(id,role,binding_role,scenario,model_id,prompt,response,status,correlation_id,metrics) VALUES('agent-model-1','cognitive_assessment','generic_llm','cognitive_assessment','test-model','{}','{}','completed',$1,$2)`, correlationID, jsonBytes(map[string]any{"run_id": correlationID, "model_call_id": "physical-request-1"})); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.agent_runs(fluctlight_id,agent_id,run_id,input_digest,status,error_detail,finished_at) VALUES($1,'conversation_cognition','legacy-run','legacy-digest','failed','Authorization: Bearer private-token',now())`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.diagnostic_events(id,event_type,severity,fluctlight_id,correlation_id,payload) VALUES('agent-termination-duplicate','agent.run.termination','error',$1,$2,$3),('agent-termination-pre-admission','agent.run.termination','error',$1,'pre-admission-failure',$4)`, fluctlightID, correlationID, jsonBytes(map[string]any{"status": "failed", "reason": "agent_run_failed", "safe_cause": "duplicate"}), jsonBytes(map[string]any{"status": "failed", "stage": "agent_run", "failure_stage": "model_input", "reason": "agent_run_failed", "safe_cause": "prompt budget exceeded", "agent_id": "conversation_cognition"})); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.agent_runs(fluctlight_id,agent_id,run_id,input_digest,correlation_id,status,finished_at) VALUES($1,'conversation_cognition','formal-completed-before-settlement','digest-2','downstream-failure','completed',now())`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.diagnostic_events(id,event_type,severity,fluctlight_id,correlation_id,payload) VALUES('agent-termination-downstream','agent.run.termination','error',$1,'downstream-failure',$2)`, fluctlightID, jsonBytes(map[string]any{"status": "failed", "stage": "settlement", "reason": "agent_cognition_settlement_failed", "safe_cause": "settlement conflict", "agent_id": "conversation_cognition"})); err != nil {
		t.Fatal(err)
	}
	app := &App{DB: repository}
	agentRuns, err := app.AgentRunsFiltered(ctx, ownerID, 10, correlationID)
	if err != nil || len(agentRuns) != 1 {
		t.Fatalf("logical runs: %#v %v", agentRuns, err)
	}
	if agentRuns[0]["status"] != "failed" || agentRuns[0]["failure_stage"] != "tool" || agentRuns[0]["failure_code"] != "capability_prepare_failed" || !strings.Contains(stringValue(agentRuns[0]["safe_cause"]), "invalid arguments") {
		t.Fatalf("logical failure omitted: %#v", agentRuns[0])
	}
	modelRuns, err := app.ModelRunsFiltered(ctx, ownerID, 10, correlationID)
	if err != nil || len(modelRuns) != 1 || modelRuns[0]["status"] != "completed" {
		t.Fatalf("physical status changed or missing: %#v %v", modelRuns, err)
	}
	if _, err := app.AgentRunsFiltered(ctx, "foreign-owner", 10, correlationID); err == nil {
		t.Fatal("non-owner read Agent diagnostics")
	}
	preAdmission, err := app.AgentRunsFiltered(ctx, ownerID, 10, "pre-admission-failure")
	if err != nil || len(preAdmission) != 1 || preAdmission[0]["source"] != "termination_event" || preAdmission[0]["failure_stage"] != "model_input" || preAdmission[0]["safe_cause"] != "prompt budget exceeded" {
		t.Fatalf("pre-admission Agent failure missing: %#v %v", preAdmission, err)
	}
	downstream, err := app.AgentRunsFiltered(ctx, ownerID, 10, "downstream-failure")
	if err != nil || len(downstream) != 2 {
		t.Fatalf("post-model failure hidden by formal completion: %#v %v", downstream, err)
	}
	seenCompleted, seenFailure := false, false
	for _, run := range downstream {
		seenCompleted = seenCompleted || run["source"] == "agent_runs" && run["status"] == "completed"
		seenFailure = seenFailure || run["source"] == "termination_event" && run["status"] == "failed" && run["failure_stage"] == "settlement"
	}
	if !seenCompleted || !seenFailure {
		t.Fatalf("post-model status layers conflated: %#v", downstream)
	}
	allRuns, err := app.AgentRunsFiltered(ctx, ownerID, 10, "")
	if err != nil || len(allRuns) != 5 {
		t.Fatalf("all logical runs: %#v %v", allRuns, err)
	}
	for _, run := range allRuns {
		if run["run_id"] == "legacy-run" && (run["association_status"] != "unknown" || run["safe_cause"] != "Authorization: Bearer private-token") {
			t.Fatalf("legacy run inferred association or changed Owner cause: %#v", run)
		}
	}
}

func TestPostgresADKModelRunFailureAndLegacyRowDoNotInventFinalStage(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID := "adk-legacy-owner", "adk-legacy-fluctlight"
	seedLifeContextFluctlight(t, ctx, repository, ownerID, fluctlightID)
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'hash','revision-1')`, ownerID); err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct{ id, status, metrics string }{
		{"adk-failed", "failed", `{"run_id":"legacy-corr","model_call_id":"failed-call"}`},
		{"adk-old", "completed", `{}`},
	} {
		if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.diagnostic_model_runs(id,role,binding_role,scenario,model_id,prompt,status,correlation_id,metrics) VALUES($1,'cognitive_assessment','generic_llm','cognitive_assessment','test-model','{}',$2,'legacy-corr',$3)`, row.id, row.status, row.metrics); err != nil {
			t.Fatal(err)
		}
	}
	runs, err := (&App{DB: repository}).modelRunsFiltered(ctx, ownerID, 10, "legacy-corr", "")
	if err != nil || len(runs) != 2 {
		t.Fatalf("model runs: rows=%#v err=%v", runs, err)
	}
	for _, run := range runs {
		switch run["id"] {
		case "adk-failed":
			if run["stage"] != "failed" || run["sequence"] != nil || run["response"] != nil {
				t.Fatalf("failed physical request misrepresented: %#v", run)
			}
		case "adk-old":
			if run["stage"] != "unknown" || run["sequence"] != nil || run["response"] != nil {
				t.Fatalf("old row invented final answer or sequence: %#v", run)
			}
		}
	}
}

func TestPostgresCurrentNonADKModelRunsHaveRoundsAndStages(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID := "fixed-round-owner", "fixed-round-fluctlight"
	seedLifeContextFluctlight(t, ctx, repository, ownerID, fluctlightID)
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'hash','revision-1')`, ownerID); err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct{ id, status, attempt string }{
		{"fixed-round-one", "completed", "attempt-one"},
		{"fixed-round-two", "timeout", "attempt-two"},
	} {
		metrics := map[string]any{"run_id": "fixed-round-run", "provider_attempt_id": row.attempt}
		if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.diagnostic_model_runs(id,role,binding_role,scenario,model_id,prompt,response,status,correlation_id,metrics) VALUES($1,'cognitive_assessment','generic_llm','schedule_generation','test-model','{}','{}',$2,'fixed-round-correlation',$3)`, row.id, row.status, jsonBytes(metrics)); err != nil {
			t.Fatal(err)
		}
	}
	runs, err := (&App{DB: repository}).modelRunsFiltered(ctx, ownerID, 10, "fixed-round-correlation", "")
	if err != nil || len(runs) != 2 {
		t.Fatalf("model runs: %#v %v", runs, err)
	}
	byID := map[string]map[string]any{}
	for _, run := range runs {
		byID[stringValue(run["id"])] = run
	}
	if intValue(byID["fixed-round-one"]["sequence"]) != 1 || byID["fixed-round-one"]["stage"] != "final_response" {
		t.Fatalf("first current run mislabeled: %#v", byID["fixed-round-one"])
	}
	if intValue(byID["fixed-round-two"]["sequence"]) != 2 || byID["fixed-round-two"]["stage"] != "timeout" {
		t.Fatalf("second current run mislabeled: %#v", byID["fixed-round-two"])
	}
}

func TestNonADKModelRunStageNeedsResponseForFinalAnswer(t *testing.T) {
	for _, testCase := range []struct {
		status   string
		response any
		want     string
	}{
		{"completed", map[string]any{"content": "完成"}, "final_response"},
		{"completed", nil, "unknown"},
		{"running", nil, "pending"},
		{"timeout", nil, "timeout"},
		{"failed", nil, "failed"},
	} {
		if got := nonADKModelRunStage(testCase.status, testCase.response); got != testCase.want {
			t.Fatalf("stage for %q/%#v = %q, want %q", testCase.status, testCase.response, got, testCase.want)
		}
	}
}

func TestPostgresProviderTimeoutPersistsOneTypedTerminalState(t *testing.T) {
	databaseURL := os.Getenv("GO_CORE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("GO_CORE_TEST_DATABASE_URL is not set")
	}
	ctx := WithProviderAttemptIdentity(WithProviderScenario(context.Background(), "initialization"), "provider-attempt-timeout-test")
	repository, err := NewPostgresRepository(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()
	correlationID := "initialization-analysis:" + stableDigest(t.Name()+time.Now().UTC().Format(time.RFC3339Nano))
	t.Cleanup(func() {
		_, _ = repository.Pool().Exec(context.Background(), `DELETE FROM public.provider_provenance WHERE correlation_id=$1`, correlationID)
		_, _ = repository.Pool().Exec(context.Background(), `DELETE FROM public.diagnostic_model_runs WHERE correlation_id=$1`, correlationID)
	})

	provider := &ProviderClient{DB: repository}
	provider.recordProviderFailure(ctx, providerAssignment{EndpointID: "endpoint-test", ModelID: "model-test"}, "initialization", correlationID, []map[string]any{{"role": "user", "content": "private-card"}}, "request_timeout")
	var status, errorCode string
	var count int
	if err := repository.Pool().QueryRow(ctx, `SELECT min(status),min(COALESCE(error_code,'')),count(*) FROM public.diagnostic_model_runs WHERE correlation_id=$1`, correlationID).Scan(&status, &errorCode, &count); err != nil {
		t.Fatal(err)
	}
	if count != 1 || status != providerRunTimeout || errorCode != "request_timeout" {
		t.Fatalf("timeout model run = count=%d status=%q error_code=%q", count, status, errorCode)
	}
	source, err := os.ReadFile("provider.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(source), "recordProviderFailure(ctx, assignment, role, correlationID, messages, err.Error())") {
		t.Fatal("Provider transport failure still persists a raw URL/error as error_code")
	}
}

func TestPostgresProviderCancellationPersistsTerminalStateAfterContextCancellation(t *testing.T) {
	databaseURL := os.Getenv("GO_CORE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("GO_CORE_TEST_DATABASE_URL is not set")
	}
	baseCtx := WithProviderAttemptIdentity(WithProviderScenario(context.Background(), "reflection"), "provider-attempt-cancelled-test")
	repository, err := NewPostgresRepository(baseCtx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()
	correlationID := "reflection:" + stableDigest(t.Name()+time.Now().UTC().Format(time.RFC3339Nano))
	t.Cleanup(func() {
		_, _ = repository.Pool().Exec(context.Background(), `DELETE FROM public.provider_provenance WHERE correlation_id=$1`, correlationID)
		_, _ = repository.Pool().Exec(context.Background(), `DELETE FROM public.diagnostic_model_runs WHERE correlation_id=$1`, correlationID)
	})

	cancelledCtx, cancel := context.WithCancel(baseCtx)
	cancel()
	provider := &ProviderClient{DB: repository}
	provider.recordProviderFailure(cancelledCtx, providerAssignment{EndpointID: "endpoint-test", ModelID: "model-test"}, "reflection", correlationID, []map[string]any{{"role": "user", "content": "private-reflection"}}, "request_cancelled")

	var status, errorCode, scenario, attemptID string
	var count int
	if err := repository.Pool().QueryRow(context.Background(), `SELECT min(status),min(COALESCE(error_code,'')),min(scenario),min(metrics->>'provider_attempt_id'),count(*) FROM public.diagnostic_model_runs WHERE correlation_id=$1`, correlationID).Scan(&status, &errorCode, &scenario, &attemptID, &count); err != nil {
		t.Fatal(err)
	}
	if count != 1 || status != providerRunCancelled || errorCode != "request_cancelled" || scenario != "reflection" || attemptID != "provider-attempt-cancelled-test" {
		t.Fatalf("cancelled model run = count=%d status=%q error_code=%q scenario=%q attempt=%q", count, status, errorCode, scenario, attemptID)
	}
}

func TestPostgresPhysicalModelResponseSurvivesLaterAgentFailure(t *testing.T) {
	databaseURL := os.Getenv("GO_CORE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("GO_CORE_TEST_DATABASE_URL is not set")
	}
	ctx := WithProviderAttemptIdentity(WithProviderScenario(context.Background(), "cognitive_assessment"), "provider-attempt-response-preserved")
	repository, err := NewPostgresRepository(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()
	correlationID := "conversation-response-preserved:" + stableDigest(t.Name()+time.Now().UTC().Format(time.RFC3339Nano))
	t.Cleanup(func() {
		_, _ = repository.Pool().Exec(context.Background(), `DELETE FROM public.provider_provenance WHERE correlation_id=$1`, correlationID)
		_, _ = repository.Pool().Exec(context.Background(), `DELETE FROM public.diagnostic_model_runs WHERE correlation_id=$1`, correlationID)
	})
	support := newProviderRuntimeSupport(repository)
	modelRunID := support.RecordQueuedModelRun(ctx, "cognitive_assessment", "endpoint-test", "model-test", correlationID, "cognitive_assessment", 1, []map[string]any{{"role": "user", "content": "request"}})
	if modelRunID == "" {
		t.Fatal("queued model run was not persisted")
	}
	support.UpdateModelRunResponse(ctx, modelRunID, map[string]any{"text": "LLM response", "structured": map[string]any{"action_type": "reply"}})
	support.UpdateModelRunState(ctx, modelRunID, providerRunFailed, errors.New("physical_provider_transport_failed"))
	var status, errorCode string
	var response []byte
	if err := repository.Pool().QueryRow(context.Background(), `SELECT status,COALESCE(error_code,''),COALESCE(response,'null'::jsonb) FROM public.diagnostic_model_runs WHERE id=$1`, modelRunID).Scan(&status, &errorCode, &response); err != nil {
		t.Fatal(err)
	}
	if status != providerRunFailed || errorCode != "provider_request_failed" || string(response) == "null" || !strings.Contains(string(response), "LLM response") {
		t.Fatalf("physical response was lost: status=%q error=%q response=%s", status, errorCode, response)
	}
	var visibleResponseCount int
	if err := repository.Pool().QueryRow(context.Background(), `SELECT count(*) FROM public.diagnostic_model_runs WHERE correlation_id=$1 AND response IS NOT NULL`, correlationID).Scan(&visibleResponseCount); err != nil {
		t.Fatal(err)
	}
	if visibleResponseCount != 1 {
		t.Fatalf("diagnostic correlation has no visible Provider response: count=%d", visibleResponseCount)
	}
}

func TestProviderProvenanceRoleParameterHasOnePostgresType(t *testing.T) {
	source, err := os.ReadFile("diagnostics.go")
	if err != nil {
		t.Fatal(err)
	}
	body := sourceBetween(t, string(source), "func (a *App) persistModelRunLifecycle", "func providerModelRunIdentity")
	for _, required := range []string{`$2::varchar(64)`, `role=$2::varchar(64)`} {
		if !strings.Contains(body, required) {
			t.Fatalf("provider provenance role parameter is ambiguously typed at %q", required)
		}
	}
}

func TestLifecycleProviderAttemptsRemainDistinct(t *testing.T) {
	base := LifecycleDiagnostic{
		Surface: "wake_up", Transition: LifecycleTransitionProviderQueued,
		CorrelationID: "wake_up:fl-1:cycle:2", ModelRunID: "model-run-1",
		ProviderAttemptID: "attempt-1", Stage: "provider_queue", Status: "queued",
		ReasonCode: "provider_request_queued",
	}
	second := base
	second.ProviderAttemptID = "attempt-2"
	second.ModelRunID = "model-run-2"
	if lifecycleDiagnosticID(base) == lifecycleDiagnosticID(second) {
		t.Fatal("different Provider attempts collapsed into one lifecycle event")
	}
	payload, err := lifecycleDiagnosticPayload(base, time.Now().UTC())
	if err != nil || stringValue(payload["provider_attempt_id"]) != "attempt-1" {
		t.Fatalf("Provider attempt missing from lifecycle payload: %#v, %v", payload, err)
	}
}

func TestLifecycleMetadataKeepsTypedContainersAndRejectsCycles(t *testing.T) {
	type typedMetadata struct {
		Authorization string `json:"authorization"`
		Note          string `json:"note"`
	}
	event := LifecycleDiagnostic{
		Surface: "wake_up", Transition: LifecycleTransitionFailed,
		CorrelationID: "wake_up:fl-1:cycle:2",
		Metadata: map[string]any{
			"typed_map":    map[string]string{"Authorization": "Bearer typed-map-secret", "note": "safe"},
			"typed_struct": typedMetadata{Authorization: "Bearer typed-struct-secret", Note: "safe"},
			"binary":       [4]byte{1, 2, 3, 4},
		},
	}
	payload, err := lifecycleDiagnosticPayload(event, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	encoded := jsonString(payload)
	if !strings.Contains(encoded, "typed-map-secret") || !strings.Contains(encoded, "typed-struct-secret") || strings.Contains(encoded, "[REDACTED]") {
		t.Fatalf("typed lifecycle metadata changed: %s", encoded)
	}
	cycle := map[string]any{}
	cycle["self"] = cycle
	event.Metadata = cycle
	if _, err := lifecycleDiagnosticPayload(event, time.Now().UTC()); err == nil {
		t.Fatal("cyclic lifecycle metadata was accepted")
	}
}

func TestLifecycleDiagnosticsPreserveFirstAndLatestFailureCause(t *testing.T) {
	event := LifecycleDiagnostic{
		Surface: "reflection", Transition: LifecycleTransitionFailed,
		CorrelationID: "reflection-1", ErrorCode: "provider_timeout", SafeCause: "first timeout",
	}
	payload, err := lifecycleDiagnosticPayload(event, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if stringValue(payload["first_error_code"]) != "provider_timeout" || stringValue(payload["latest_error_code"]) != "provider_timeout" || stringValue(payload["first_safe_cause"]) != "first timeout" || stringValue(payload["latest_safe_cause"]) != "first timeout" {
		t.Fatalf("first/latest lifecycle cause missing: %#v", payload)
	}
	source, err := os.ReadFile("lifecycle_diagnostics.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"first_error_code", "latest_error_code", "first_safe_cause", "latest_safe_cause"} {
		if !strings.Contains(string(source), required) {
			t.Fatalf("lifecycle aggregation does not preserve %q", required)
		}
	}
}

func TestProviderPreflightErrorsHaveStableCategoryAndRetryability(t *testing.T) {
	for _, test := range []struct {
		name      string
		stage     string
		err       error
		category  string
		code      string
		retryable bool
	}{
		{name: "unassigned", stage: "assignment", err: pgx.ErrNoRows, category: "configuration", code: "provider_role_unassigned"},
		{name: "invalid role", stage: "assignment", err: errors.New("provider role bad invalid"), category: "configuration", code: "provider_role_invalid"},
		{name: "role preflight", stage: "assignment", err: errors.New("provider role generic_llm preflight failed"), category: "configuration", code: "provider_role_preflight_failed"},
		{name: "wire budget", stage: "wire_budget", err: ErrPromptRequiredBudgetExceeded, category: "budget", code: "prompt_required_budget_exceeded"},
		{name: "store", stage: "assignment", err: errors.New("database unavailable"), category: "infrastructure", code: "provider_store_unavailable", retryable: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			category, code, retryable := classifyProviderPreflightError(test.stage, test.err)
			if category != test.category || code != test.code || retryable != test.retryable {
				t.Fatalf("classifyProviderPreflightError() = %q, %q, %t", category, code, retryable)
			}
		})
	}
}

func TestLifecycleOwningFilesUseOnlyExplicitBestEffortAssignments(t *testing.T) {
	files := []string{
		"lifecycle_diagnostics.go", "wakeup.go", "workflow_ops.go", "provider.go",
		"diagnostics.go", "visual_identity.go", "../workflow/workflow.go",
	}
	allowed := map[string]int{
		"workflow.GetVersion":     1,  // deterministic version marker returns no error
		"setReflectionWindowIdle": 11, // this best-effort cleanup diagnoses its own failure
		"DB.Pool().QueryRow":      2,  // diagnostic-only Fluctlight enrichment must not recurse on sink failure
	}
	observed := map[string]int{}
	for _, file := range files {
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for lineNumber, line := range strings.Split(string(source), "\n") {
			if strings.Contains(line, "_, _ =") {
				t.Fatalf("%s:%d ignores a multi-result call: %s", file, lineNumber+1, line)
			}
			if !strings.Contains(line, "_ =") {
				continue
			}
			matched := false
			for token := range allowed {
				if strings.Contains(line, token) {
					observed[token]++
					matched = true
					break
				}
			}
			if !matched {
				t.Fatalf("%s:%d has an unapproved ignored result: %s", file, lineNumber+1, line)
			}
		}
	}
	for token, expected := range allowed {
		if observed[token] != expected {
			t.Fatalf("best-effort allowlist %q count = %d, want %d", token, observed[token], expected)
		}
	}
}

func TestLifecycleTraceDistinguishesNoopActionableRetryAndFailure(t *testing.T) {
	correlationID := "wake_up:fl-1:cycle:9"
	trace := []LifecycleDiagnostic{
		{Surface: "wake_up", Transition: LifecycleTransitionQueued, CorrelationID: correlationID, IntentID: "wake-intent", WorkflowID: "go:wake", Stage: "dispatcher", Status: "queued", ReasonCode: "dispatcher_selected", Attempt: 1},
		{Surface: "wake_up", Transition: LifecycleTransitionDispatched, CorrelationID: correlationID, IntentID: "wake-intent", WorkflowID: "go:wake", RunID: "run-1", Stage: "workflow_start", Status: "started", ReasonCode: "workflow_dispatched", Attempt: 1},
		{Surface: "wake_up", Transition: LifecycleTransitionActivityStarted, CorrelationID: correlationID, IntentID: "wake-intent", WorkflowID: "go:wake", RunID: "run-1", ActivityID: "activity-1", Stage: "activity", Status: "running", ReasonCode: "activity_started", Attempt: 1},
		{Surface: "wake_up", Transition: LifecycleTransitionProviderQueued, CorrelationID: correlationID, ModelRunID: "model-run-1", ProviderAttemptID: "provider-attempt-1", Stage: "provider_queue", Status: "queued", ReasonCode: "provider_request_queued"},
		{Surface: "wake_up", Transition: LifecycleTransitionCompletedNoop, CorrelationID: correlationID, IntentID: "wake-intent", WorkflowID: "go:wake", RunID: "run-1", ActivityID: "activity-1", Stage: "activity", Status: "no_op", ReasonCode: "no_action_selected", Attempt: 1},
		{Surface: "wake_up", Transition: LifecycleTransitionNextCycleScheduled, CorrelationID: correlationID, IntentID: "wake-intent", WorkflowID: "go:wake", Stage: "clock", Status: "scheduled", ReasonCode: "wake_up_cycle_completed", Attempt: 9, NextDueAt: time.Now().UTC().Add(time.Minute)},
	}
	want := []LifecycleTransition{LifecycleTransitionQueued, LifecycleTransitionDispatched, LifecycleTransitionActivityStarted, LifecycleTransitionProviderQueued, LifecycleTransitionCompletedNoop, LifecycleTransitionNextCycleScheduled}
	seen := map[string]struct{}{}
	for index, event := range trace {
		if event.Transition != want[index] || event.CorrelationID != correlationID {
			t.Fatalf("trace[%d] = %#v", index, event)
		}
		if err := event.Validate(); err != nil {
			t.Fatalf("trace[%d] invalid: %v", index, err)
		}
		id := lifecycleDiagnosticID(event)
		if _, duplicate := seen[id]; duplicate {
			t.Fatalf("trace transition %q collapsed into a prior event", event.Transition)
		}
		seen[id] = struct{}{}
	}
	for name, event := range map[string]LifecycleDiagnostic{
		"actionable": {Surface: "capability", Transition: LifecycleTransitionCompletedActionable, CorrelationID: correlationID, Stage: "activity", Status: "completed", ReasonCode: "action_settled"},
		"retry":      {Surface: "wake_up", Transition: LifecycleTransitionRetryScheduled, CorrelationID: correlationID, Stage: "workflow_reconcile", Status: "retry", ReasonCode: "wake_up_workflow_terminal"},
		"failure":    {Surface: "wake_up", Transition: LifecycleTransitionFailed, CorrelationID: correlationID, Stage: "provider_preflight", Status: "failed", ReasonCode: "provider_store_unavailable", ErrorCode: "provider_store_unavailable", Retryable: true},
	} {
		if err := event.Validate(); err != nil {
			t.Fatalf("%s outcome invalid: %v", name, err)
		}
	}
}

func TestLifecycleDiagnosticsFilterBuildsAllPostgresPredicates(t *testing.T) {
	filter, err := (LifecycleDiagnosticsFilter{
		Limit: 900, FluctlightID: " fl-1 ", CorrelationID: "corr-1", IntentID: "intent-1",
		WorkflowID: "go:wake-1", RunID: "run-1", Surface: "wake_up", Status: "retry",
	}).Normalized()
	if err != nil || filter.Limit != 500 || filter.FluctlightID != "fl-1" {
		t.Fatalf("normalized filter = %#v, %v", filter, err)
	}
	eventQuery, eventArgs := buildLifecycleDiagnosticQuery(filter)
	intentQuery, intentArgs := buildWorkflowIntentSnapshotQuery(filter)
	for _, required := range []string{"fluctlight_id", "correlation_id", "payload->>'intent_id'", "payload->>'workflow_id'", "payload->>'run_id'", "payload->>'surface'", "payload->>'status'"} {
		if !strings.Contains(eventQuery, required) {
			t.Fatalf("lifecycle event query missing %q: %s", required, eventQuery)
		}
	}
	if !strings.Contains(eventQuery, "ORDER BY created_at DESC,id DESC LIMIT") || !strings.HasSuffix(eventQuery, "ORDER BY created_at ASC,id ASC") {
		t.Fatalf("lifecycle query does not select the newest window in timeline order: %s", eventQuery)
	}
	for _, required := range []string{"platform_workflow_intents", "diagnostic_events", "next_attempt_at", "attempt_count", "i.intent_type", "COALESCE(i.status,'pending')"} {
		if !strings.Contains(intentQuery, required) {
			t.Fatalf("workflow-intent snapshot query missing %q: %s", required, intentQuery)
		}
	}
	if len(eventArgs) != 8 || len(intentArgs) != 8 {
		t.Fatalf("filter argument counts event=%d intent=%d", len(eventArgs), len(intentArgs))
	}
}

func TestLifecycleDiagnosticsRejectsUnsafeFilterValues(t *testing.T) {
	for _, filter := range []LifecycleDiagnosticsFilter{
		{CorrelationID: strings.Repeat("x", 129)},
		{Surface: "wake up"},
		{Status: "retry;drop"},
	} {
		if _, err := filter.Normalized(); !errors.Is(err, ErrDiagnosticsFilterInvalid) {
			t.Fatalf("unsafe filter %#v error = %v", filter, err)
		}
	}
}

func TestLifecycleDiagnosticsExportUsesSameFilterAndPostgresSnapshot(t *testing.T) {
	source, err := os.ReadFile("operations.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	exportStart := strings.Index(text, "func (a *App) DiagnosticsExportFiltered")
	exportEnd := strings.Index(text[exportStart:], "func (a *App) RecoverStaleModelRuns")
	if exportStart < 0 || exportEnd < 0 {
		t.Fatal("filtered diagnostics export boundary missing")
	}
	exportBody := text[exportStart : exportStart+exportEnd]
	for _, required := range []string{"filter.Normalized()", "DiagnosticsFiltered", "ModelRunsFiltered", "LifecycleDiagnostics", `"workflow_intents"`, `"filters"`} {
		if !strings.Contains(exportBody, required) {
			t.Fatalf("filtered export missing %q", required)
		}
	}
	snapshotStart := strings.Index(text, "func (a *App) workflowIntentSnapshots")
	snapshotEnd := strings.Index(text[snapshotStart:], "func normalizedDiagnosticWorkflowID")
	if snapshotStart < 0 || snapshotEnd < 0 || strings.Contains(text[snapshotStart:snapshotStart+snapshotEnd], "a.Workflows") {
		t.Fatal("PostgreSQL intent snapshot depends on the optional Temporal runtime")
	}
	if !strings.Contains(text[snapshotStart:snapshotStart+snapshotEnd], "boundedLifecycleCause(*lastError)") {
		t.Fatal("PostgreSQL intent snapshot exposes an unbounded workflow error")
	}
}
