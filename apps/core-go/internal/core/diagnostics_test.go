package core

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
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
	if effective.TokenBudget != initializationMinimumOutputReserveTokens || effective.Timeout != initializationMinimumRequestTimeout {
		t.Fatalf("initialization assignment = %#v", effective)
	}
	ordinary, err := providerAssignmentForScenario(configured, "wake_up")
	if err != nil || ordinary.TokenBudget != configured.TokenBudget || ordinary.Timeout != configured.Timeout {
		t.Fatalf("ordinary assignment changed: %#v, %v", ordinary, err)
	}
	insufficient := configured
	insufficient.ContextWindowTokens = 55000
	preserved, err := providerAssignmentForScenario(insufficient, "initialization")
	if err == nil || err.Error() != "initialization_output_reserve_unavailable" || preserved.TokenBudget != insufficient.TokenBudget || preserved.Timeout != insufficient.Timeout {
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
	if !strings.Contains(body, `"safe_cause", boundedLifecycleCause(err.Error())`) {
		t.Fatal("diagnostic persistence warning omits the bounded failure cause")
	}
	if got := boundedLifecycleCause("Authorization: Bearer private-token"); got != "[REDACTED]" {
		t.Fatalf("sensitive persistence cause = %q", got)
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
	for _, stage := range []string{`"assignment"`, `"message_validation"`, `"wire_budget"`, `"payload_encode"`} {
		if !strings.Contains(body, "recordProviderPreflightFailure") || !strings.Contains(body, stage) {
			t.Fatalf("Provider preflight stage %s is not diagnosed", stage)
		}
	}
	queuedMarker := "RecordQueuedModelRun"
	if strings.Index(body, queuedMarker) < 0 {
		queuedMarker = "recordQueuedModelRun"
	}
	if strings.Index(body, "ensureProviderAttemptIdentity") > strings.Index(body, queuedMarker) {
		t.Fatal("Provider attempt identity is created after the queued diagnostic")
	}
}

func TestProviderPreflightDiagnosticsAreMetadataOnly(t *testing.T) {
	const canary = "PRIVATE_CHARACTER_CARD_CANARY"
	prompt := providerPreflightDiagnosticPrompt([]map[string]any{{"role": "user", "content": canary}})
	encoded := jsonString(prompt)
	if strings.Contains(encoded, canary) || stringValue(prompt["diagnostic_scope"]) != "metadata_only" || stringValue(prompt["prompt_digest"]) == "" {
		t.Fatalf("Provider preflight diagnostic leaked source content: %s", encoded)
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

func TestLifecycleMetadataRedactionHandlesTypedContainersAndCycles(t *testing.T) {
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
	if strings.Contains(encoded, "typed-map-secret") || strings.Contains(encoded, "typed-struct-secret") || strings.Contains(encoded, "[1,2,3,4]") {
		t.Fatalf("typed lifecycle metadata leaked: %s", encoded)
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
		"_ = factID":              1,  // intentionally discard a returned identity after checked persistence
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
