package core

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestFormalAgentUsesAssemblyModelSnapshotWithoutSerializingCredential(t *testing.T) {
	var models []string
	var physicalDiagnostics map[string]any
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		models = append(models, stringValue(payload["model"]))
		return fakeProviderResult{Text: "a quiet window"}
	})
	client := &http.Client{Transport: projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		physicalDiagnostics = providerPromptDiagnostics(request.Context())
		return router.RoundTrip(request)
	})}
	app := &App{Provider: &ProviderClient{HTTP: client}}
	frozen := providerAssignment{Role: "generic_llm", SourceBindingRole: "cognitive_assessment", BindingRevision: 7, EndpointRevision: 3, EndpointID: "old-endpoint", SecretPurpose: "private-test-purpose", BaseURL: "http://controlled.invalid", ModelID: "old-model", Secret: "private-test-credential", Timeout: time.Second}
	prompt := PromptAssemblyResult{
		Messages:        []map[string]any{{"role": "system", "content": "render the frozen task"}, {"role": "user", "content": "a window"}},
		providerBinding: &promptProviderBinding{role: "media_prompt", assignment: frozen},
	}
	if strings.Contains(jsonString(prompt), frozen.Secret) {
		t.Fatal("process-local assignment credential was serialized with prompt data")
	}
	result, err := app.RunFormalAgent(context.Background(), FormalAgentMediaPrompt, FormalAgentRunInput{Prompt: prompt, SchemaName: "media_prompt_text"})
	if err != nil || result.Completion.Text != "a quiet window" || len(models) != 1 || models[0] != "old-model" {
		t.Fatalf("model snapshot was not used: models=%v result=%#v err=%v", models, result.Completion, err)
	}
	projection := mapValue(physicalDiagnostics["context_projection"])
	if projection["agent_id"] != string(FormalAgentMediaPrompt) || projection["task_variant"] != "media_prompt_text" || strings.Contains(jsonString(physicalDiagnostics), frozen.Secret) || strings.Contains(jsonString(physicalDiagnostics), frozen.SecretPurpose) {
		t.Fatalf("physical request trace is missing task identity or contains a credential: %#v", projection)
	}
	configuration := mapValue(physicalDiagnostics["model_configuration"])
	if configuration["source_binding_role"] != "cognitive_assessment" || intValue(configuration["binding_revision"]) != 7 || intValue(configuration["endpoint_revision"]) != 3 || configuration["model_id"] != "old-model" {
		t.Fatalf("physical request trace lost effective binding provenance: %#v", configuration)
	}
}

func TestFormalAgentRejectsConflictingAssemblyAndFrozenModel(t *testing.T) {
	requests := 0
	router := newFakeProviderRouter().otherwise(func(map[string]any) fakeProviderResult {
		requests++
		return fakeProviderResult{Text: "unexpected"}
	})
	app := &App{Provider: &ProviderClient{HTTP: &http.Client{Transport: router}}}
	old := providerAssignment{Role: "generic_llm", EndpointID: "endpoint-a", ModelID: "model-a"}
	changed := providerAssignment{Role: "generic_llm", EndpointID: "endpoint-b", ModelID: "model-b"}
	prompt := PromptAssemblyResult{Messages: []map[string]any{{"role": "user", "content": "test"}}, providerBinding: &promptProviderBinding{role: "media_prompt", assignment: old}}
	ctx := withFrozenProviderAssignment(context.Background(), "media_prompt", changed)
	_, err := app.RunFormalAgent(ctx, FormalAgentMediaPrompt, FormalAgentRunInput{Prompt: prompt, SchemaName: "media_prompt_text"})
	if err == nil || err.Error() != "formal_agent_prompt_binding_conflict" || requests != 0 {
		t.Fatalf("conflicting snapshots reached Provider: err=%v requests=%d", err, requests)
	}
}

func TestNestedFormalAgentDoesNotUseParentAssignment(t *testing.T) {
	var model string
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		model = stringValue(payload["model"])
		return fakeProviderResult{Structured: map[string]any{"ok": true}}
	})
	app := &App{Provider: &ProviderClient{HTTP: &http.Client{Transport: router}}}
	parent := providerAssignment{Role: "generic_llm", BaseURL: "http://controlled.invalid", ModelID: "parent-model", Timeout: time.Second}
	child := providerAssignment{Role: "generic_llm", BaseURL: "http://controlled.invalid", ModelID: "child-model", Timeout: time.Second}
	ctx := withAgentModelRoute(context.Background(), FormalAgentConversationCognition, "conversation_turn_response")
	ctx = withFrozenProviderAssignment(ctx, "cognitive_assessment", parent)
	schema := objectSchema(map[string]any{"ok": booleanSchema()}, []string{"ok"}, false)
	prompt := PromptAssemblyResult{Messages: []map[string]any{{"role": "user", "content": "nested task"}}, ResponseFormat: schema, providerBinding: &promptProviderBinding{role: "cognitive_assessment", assignment: child}}
	_, err := app.RunFormalAgent(ctx, FormalAgentScheduleReplan, FormalAgentRunInput{Prompt: prompt, SchemaName: "schedule_replan_plan"})
	if err != nil || model != "child-model" {
		t.Fatalf("nested Agent did not use its own assignment: model=%q err=%v", model, err)
	}
	if got, ok := providerAssignmentSnapshot(ctx, "cognitive_assessment"); !ok || got.ModelID != "parent-model" {
		t.Fatalf("nested run changed its parent snapshot: %#v %v", got, ok)
	}
}

func TestFrozenAssignmentFollowsTaskIdentityAndExplicitFallbackIsIsolated(t *testing.T) {
	assignment := providerAssignment{Role: "generic_llm", ModelID: "parent"}
	parent := withAgentModelRoute(context.Background(), FormalAgentConversationCognition, "conversation_turn_response")
	parent = withFrozenProviderAssignment(parent, "cognitive_assessment", assignment)
	child := withAgentModelRoute(parent, FormalAgentNPCResponse, "npc_response_v1")
	if _, ok := providerAssignmentSnapshot(child, "cognitive_assessment"); ok {
		t.Fatal("child Agent inherited the parent's frozen model")
	}
	if got := agentModelBindingCandidates(isolatedReplyModelContext(parent), "generic_llm"); len(got) != 1 || got[0] != "generic_llm" {
		t.Fatalf("explicit fallback inherited the parent Agent route: %v", got)
	}
	// A deliberate assignment supplied before entering a task remains valid
	// for that task, but does not become authority for another nested Agent.
	explicit := withFrozenProviderAssignment(context.Background(), "cognitive_assessment", assignment)
	first := withAgentModelRoute(explicit, FormalAgentConversationCognition, "conversation_turn_response")
	if _, ok := providerAssignmentSnapshot(first, "cognitive_assessment"); !ok {
		t.Fatal("first task did not adopt its explicit assignment")
	}
	if _, ok := providerAssignmentSnapshot(withAgentModelRoute(first, FormalAgentNPCResponse, "npc_response_v1"), "cognitive_assessment"); ok {
		t.Fatal("adopted assignment escaped to a second task")
	}
}

func TestMomentSharesOnlyTheSameFrozenBindingSource(t *testing.T) {
	defaultRole := conversationReplyModelRole
	agentRole := agentModelBindingKey(FormalAgentConversationReply, "")
	privateRole := agentModelBindingKey(FormalAgentConversationReply, "private_reply")
	momentRole := agentModelBindingKey(FormalAgentConversationReply, "moment")
	for _, tc := range []struct {
		moment, shared string
		want           bool
	}{
		{defaultRole, defaultRole, true},
		{agentRole, agentRole, true},
		{agentRole, privateRole, false},
		{momentRole, privateRole, false},
		{"", "", false},
	} {
		got := shareMomentWriterAssignment(providerAssignment{SourceBindingRole: tc.moment}, nil, providerAssignment{SourceBindingRole: tc.shared}, nil)
		if got != tc.want {
			t.Errorf("moment source %q, shared source %q: got %v want %v", tc.moment, tc.shared, got, tc.want)
		}
	}
}

func TestNestedVisualTaskKeepsItsExplicitVisionAssignment(t *testing.T) {
	var model string
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		model = stringValue(payload["model"])
		return fakeProviderResult{Structured: map[string]any{"ok": true}}
	})
	app := &App{Provider: &ProviderClient{HTTP: &http.Client{Transport: router}}}
	ctx := withAgentModelRoute(context.Background(), FormalAgentConversationReply, "private_reply")
	ctx = withAgentModelRoute(ctx, FormalAgentMediaQuality, "media_quality_acceptance_response")
	ctx = withFrozenProviderAssignment(ctx, visionUnderstandingModelRole, providerAssignment{Role: visionUnderstandingModelRole, BaseURL: "http://controlled.invalid", ModelID: "frozen-vision", Timeout: time.Second})
	schema := objectSchema(map[string]any{"ok": booleanSchema()}, []string{"ok"}, false)
	_, err := app.RunFormalAgent(ctx, FormalAgentMediaQuality, FormalAgentRunInput{Prompt: PromptAssemblyResult{Messages: []map[string]any{{"role": "user", "content": "image fixture"}}, ResponseFormat: schema}, SchemaName: "media_quality_acceptance_response"})
	if err != nil || model != "frozen-vision" {
		t.Fatalf("visual child lost its resolved assignment: model=%q err=%v", model, err)
	}
}

func TestStructuredAssemblyTaskPreservesBindingThroughProductionHelper(t *testing.T) {
	var model string
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		model = stringValue(payload["model"])
		return fakeProviderResult{Structured: map[string]any{"ok": true}}
	})
	app := &App{Provider: &ProviderClient{HTTP: &http.Client{Transport: router}}}
	frozen := providerAssignment{Role: "generic_llm", EndpointID: "old-endpoint", BaseURL: "http://controlled.invalid", ModelID: "old-structured-model", Timeout: time.Second}
	assembly := PromptAssemblyResult{Messages: []map[string]any{{"role": "system", "content": "task"}, {"role": "user", "content": "input"}}, providerBinding: &promptProviderBinding{role: "initialization", assignment: frozen}}
	schema := objectSchema(map[string]any{"ok": booleanSchema()}, []string{"ok"}, false)
	result, err := app.runFormalStructuredAssemblyTask(context.Background(), FormalAgentInitialization, assembly, nil, "initialization_response", schema, false, nil)
	if err != nil || model != frozen.ModelID || result.Completion.Structured["ok"] != true {
		t.Fatalf("structured production helper lost assembly binding: model=%q result=%#v err=%v", model, result.Completion.Structured, err)
	}
}

func TestStructuredAgentPhysicalRequestUsesFrozenSampling(t *testing.T) {
	var wire map[string]any
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		wire = payload
		return fakeProviderResult{Structured: map[string]any{"ok": true}}
	})
	app := &App{Provider: &ProviderClient{HTTP: &http.Client{Transport: router}}}
	frozen := providerAssignment{Role: "generic_llm", SourceBindingRole: agentModelBindingKey(FormalAgentReflection, "reflection_proposal_v2"), BaseURL: "http://controlled.invalid", ModelID: "fixture", Timeout: time.Second, Temperature: floatPointer(0), TopP: floatPointer(0.7)}
	schema := objectSchema(map[string]any{"ok": booleanSchema()}, []string{"ok"}, false)
	prompt := PromptAssemblyResult{Messages: []map[string]any{{"role": "user", "content": "fixture"}}, ResponseFormat: schema, providerBinding: &promptProviderBinding{role: "reflection", assignment: frozen}}
	_, err := app.RunFormalAgent(context.Background(), FormalAgentReflection, FormalAgentRunInput{Prompt: prompt, SchemaName: "reflection_proposal_v2"})
	if err != nil || wire["temperature"] != float64(0) || wire["top_p"] != 0.7 {
		t.Fatalf("structured task lost frozen sampling: wire=%v err=%v", wire, err)
	}
}

func TestLegacyAssignmentDiagnosticsDoNotInventConfigurationRevisions(t *testing.T) {
	configuration := providerAssignmentDiagnostics(providerAssignment{Role: "conversation_reply", ModelID: "frozen-model", Secret: "must-not-appear"})
	if configuration["source_status"] != "unrecorded_snapshot" || configuration["binding_revision"] != nil || configuration["endpoint_revision"] != nil || strings.Contains(jsonString(configuration), "must-not-appear") {
		t.Fatalf("legacy snapshot provenance was invented or leaked: %#v", configuration)
	}
}

func TestReplyTaskVariantKeepsDistinctScenesAndCorrections(t *testing.T) {
	for _, tc := range []struct {
		input ReplyContextInput
		want  string
	}{
		{ReplyContextInput{}, "private_reply"},
		{ReplyContextInput{Proactive: true}, "proactive"},
		{ReplyContextInput{Scene: replySceneMoment, Proactive: true}, "moment"},
		{ReplyContextInput{Proactive: true, StyleRepair: true, OutputCorrection: "repair"}, "proactive.style_repair"},
		{ReplyContextInput{Scene: replySceneMoment, OutputCorrection: "correct"}, "moment.correction"},
	} {
		if got := replyTaskVariant(tc.input); got != tc.want {
			t.Errorf("replyTaskVariant(%#v)=%q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestReplyPhysicalRequestTraceDistinguishesMomentAndProactive(t *testing.T) {
	for _, tc := range []struct {
		name      string
		configure func(*ReplyContextInput)
	}{
		{"moment", func(input *ReplyContextInput) { input.Scene = replySceneMoment }},
		{"proactive", func(input *ReplyContextInput) { input.Proactive = true }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var diagnostics map[string]any
			router := newFakeProviderRouter().otherwise(func(map[string]any) fakeProviderResult {
				return fakeProviderResult{Text: "这是一条受控测试消息。"}
			})
			client := &http.Client{Transport: projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
				diagnostics = providerPromptDiagnostics(request.Context())
				return router.RoundTrip(request)
			})}
			app := &App{Provider: &ProviderClient{HTTP: client}}
			input := replyContextFixture()
			tc.configure(&input)
			_, err := app.RunConversationReplyTask(context.Background(), ConversationReplyTaskInput{Context: input, Assignment: providerAssignment{Role: conversationReplyModelRole, BaseURL: "http://controlled.invalid", ModelID: "fixture", Timeout: time.Second}, CorrelationID: "variant-" + tc.name, Concurrency: 1})
			if err != nil {
				t.Fatal(err)
			}
			projection := mapValue(diagnostics["context_projection"])
			if projection["agent_id"] != "conversation_reply" || projection["task_variant"] != tc.name || projection["schema"] != "conversation_reply_text" {
				t.Fatalf("physical request used the wrong scene identity: %#v", projection)
			}
		})
	}
}

func TestRestoredAssignmentKeepsCredentialDestinationBoundary(t *testing.T) {
	old := providerAssignment{BaseURL: "https://old.invalid/v1", SecretPurpose: "old-purpose"}
	if purpose, err := restoredAssignmentSecretPurpose(old, "https://new.invalid/v1", "new-purpose"); err != nil || purpose != "old-purpose" {
		t.Fatalf("frozen credential reference was replaced: purpose=%q err=%v", purpose, err)
	}
	old.SecretPurpose = ""
	if purpose, err := restoredAssignmentSecretPurpose(old, "https://old.invalid/v1/", "current-purpose"); err != nil || purpose != "current-purpose" {
		t.Fatalf("unchanged legacy destination did not retain its credential route: purpose=%q err=%v", purpose, err)
	}
	if _, err := restoredAssignmentSecretPurpose(old, "https://new.invalid/v1", "new-purpose"); err == nil || err.Error() != "reply_snapshot_credential_source_unknown" {
		t.Fatalf("legacy destination change silently rebound credentials: %v", err)
	}
}

func TestRestoredAssignmentRejectsRotatedCredentialAfterEndpointChange(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	seedCognitiveProviderRole(t, ctx, repository, "snapshot-destination-endpoint")
	provider := &ProviderClient{DB: repository, SettingsKey: []byte("0123456789abcdef0123456789abcdef")}
	original, err := encryptSecret(provider.SettingsKey, "turn-secret", "original-fixture-secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.setting_secrets(purpose,ciphertext,nonce) VALUES('turn-secret',$1,$2)`, original.ciphertext, original.nonce); err != nil {
		t.Fatal(err)
	}
	frozen, err := provider.assignment(ctx, "cognitive_assessment")
	if err != nil {
		t.Fatal(err)
	}
	frozen.Secret = ""
	rotated, err := encryptSecret(provider.SettingsKey, "turn-secret", "rotated-fixture-secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.provider_endpoints SET base_url='http://new-destination.invalid' WHERE id='snapshot-destination-endpoint'`); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.setting_secrets SET ciphertext=$1,nonce=$2 WHERE purpose='turn-secret'`, rotated.ciphertext, rotated.nonce); err != nil {
		t.Fatal(err)
	}
	app := &App{DB: repository, Provider: provider}
	if err := app.restoreAssignmentCredential(ctx, &frozen); err == nil || !strings.Contains(err.Error(), "snapshot_credential_destination_changed") || frozen.Secret != "" {
		t.Fatalf("rotated credential was accepted for the former endpoint: err=%v", err)
	}
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.provider_endpoints SET base_url='http://turn.invalid' WHERE id='snapshot-destination-endpoint'`); err != nil {
		t.Fatal(err)
	}
	if err := app.restoreAssignmentCredential(ctx, &frozen); err == nil || !strings.Contains(err.Error(), "snapshot_credential_destination_changed") || frozen.Secret != "" {
		t.Fatalf("endpoint A-to-B-to-A accepted a credential rotated at B: err=%v", err)
	}
}

func TestProviderAssignmentRecordsActualLegacySourceAndVersion(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	seedCognitiveProviderRole(t, ctx, repository, "model-provenance-endpoint")
	provider := &ProviderClient{DB: repository, SettingsKey: []byte("0123456789abcdef0123456789abcdef")}
	first, err := provider.assignment(ctx, "cognitive_assessment")
	if err != nil || first.SourceBindingRole != "cognitive_assessment" || first.BindingRevision < 1 || first.EndpointRevision < 1 {
		t.Fatalf("legacy binding source/version missing: source=%q binding=%d endpoint=%d err=%v", first.SourceBindingRole, first.BindingRevision, first.EndpointRevision, err)
	}
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.model_roles SET model_id='new-model' WHERE role='cognitive_assessment'`); err != nil {
		t.Fatal(err)
	}
	second, err := provider.assignment(ctx, "cognitive_assessment")
	if err != nil || second.ModelID != "new-model" || second.BindingRevision <= first.BindingRevision || first.ModelID == second.ModelID {
		t.Fatalf("binding revision did not reflect the saved change: first=%q/%d second=%q/%d err=%v", first.ModelID, first.BindingRevision, second.ModelID, second.BindingRevision, err)
	}
	firstSecret, err := encryptSecret(provider.SettingsKey, "turn-secret", "first-fixture-secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.setting_secrets(purpose,ciphertext,nonce) VALUES('turn-secret',$1,$2)`, firstSecret.ciphertext, firstSecret.nonce); err != nil {
		t.Fatal(err)
	}
	beforeRotation, err := provider.assignment(ctx, "cognitive_assessment")
	if err != nil || beforeRotation.CredentialRevision < 1 {
		t.Fatalf("credential version was not captured: revision=%d err=%v", beforeRotation.CredentialRevision, err)
	}
	secondSecret, err := encryptSecret(provider.SettingsKey, "turn-secret", "second-fixture-secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.setting_secrets SET ciphertext=$1,nonce=$2 WHERE purpose='turn-secret'`, secondSecret.ciphertext, secondSecret.nonce); err != nil {
		t.Fatal(err)
	}
	settings := defaultConversationReplySettings()
	contract, err := replyExpressionContract(settings.ExpressionVersion)
	if err != nil {
		t.Fatal(err)
	}
	beforeRotation.Secret = ""
	run := &privateReplyRun{}
	app := &App{DB: repository, Provider: provider}
	if err := app.restoreReplyRunBinding(ctx, run, jsonBytes(settings), 1, map[string]any{"assignment": beforeRotation, "reply_contract": contract}); err != nil {
		t.Fatal(err)
	}
	if run.Assignment.ModelID != beforeRotation.ModelID || run.Assignment.Secret != "second-fixture-secret" || run.Assignment.CredentialRevision <= beforeRotation.CredentialRevision {
		t.Fatalf("unchanged destination did not accept credential rotation: model=%q credential revision=%d", run.Assignment.ModelID, run.Assignment.CredentialRevision)
	}
	newDestinationSecret, err := encryptSecret(provider.SettingsKey, "new-purpose", "different-service-secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.setting_secrets(purpose,ciphertext,nonce) VALUES('new-purpose',$1,$2)`, newDestinationSecret.ciphertext, newDestinationSecret.nonce); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.provider_endpoints SET base_url='http://different.invalid',secret_purpose='new-purpose' WHERE id='model-provenance-endpoint'`); err != nil {
		t.Fatal(err)
	}
	if err := app.restoreReplyRunBinding(ctx, &privateReplyRun{}, jsonBytes(settings), 1, map[string]any{"assignment": beforeRotation, "reply_contract": contract}); err == nil || err.Error() != "provider_snapshot_credential_destination_changed" {
		t.Fatalf("changed destination accepted a credential rotated after the snapshot: %v", err)
	}
	unchangedCredential := run.Assignment
	unchangedCredential.Secret = ""
	if err := app.restoreAssignmentCredential(ctx, &unchangedCredential); err != nil || unchangedCredential.Secret != "second-fixture-secret" {
		t.Fatalf("unchanged original credential could not be restored for its frozen destination: %v", err)
	}
	legacy := beforeRotation
	legacy.SecretPurpose = ""
	if err := app.restoreReplyRunBinding(ctx, &privateReplyRun{}, jsonBytes(settings), 1, map[string]any{"assignment": legacy, "reply_contract": contract}); err == nil || err.Error() != "reply_snapshot_credential_source_unknown" {
		t.Fatalf("legacy snapshot accepted another destination's credential: %v", err)
	}
}
