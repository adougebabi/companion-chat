package core

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestAgentModelBindingCandidatesPreferVariantThenAgentThenDefault(t *testing.T) {
	ctx := withAgentModelRoute(context.Background(), FormalAgentConversationSummary, "conversation_segment_v1")
	want := []string{"agent:conversation_summary:conversation_segment_v1", "agent:conversation_summary", "generic_llm"}
	if got := agentModelBindingCandidates(ctx, "generic_llm"); !reflect.DeepEqual(got, want) {
		t.Fatalf("candidates=%v want=%v", got, want)
	}
	ctx = withAgentModelRoute(context.Background(), FormalAgentConversationSummary, "unregistered_variant")
	if got := agentModelBindingCandidates(ctx, "generic_llm"); !reflect.DeepEqual(got, []string{"agent:conversation_summary", "generic_llm"}) {
		t.Fatalf("unknown variant widened the route: %v", got)
	}
	if got := agentModelBindingCandidates(ctx, visionUnderstandingModelRole); !reflect.DeepEqual(got, []string{visionUnderstandingModelRole}) {
		t.Fatalf("text override took over visual binding: %v", got)
	}
	ctx = withAgentModelRoute(context.Background(), FormalAgentConversationCognition, "conversation_turn_response")
	if got := agentModelBindingCandidates(ctx, "generic_llm"); !reflect.DeepEqual(got, []string{"agent:conversation_cognition:conversation_turn_response", "agent:conversation_cognition", conversationReplyModelRole}) {
		t.Fatalf("conversation did not inherit the configured expression model: %v", got)
	}
	ctx = withAgentModelRoute(context.Background(), FormalAgentExpressionLearning, expressionLearningSchema)
	if got := agentModelBindingCandidates(ctx, "generic_llm"); !reflect.DeepEqual(got, []string{"agent:expression_learning:expression_learning_v1", "agent:expression_learning", conversationReplyModelRole}) {
		t.Fatalf("expression learning did not inherit the configured text model: %v", got)
	}
}

func TestWriterAssignmentKeepsExplicitTaskTimeout(t *testing.T) {
	for _, tc := range []struct {
		source string
		want   time.Duration
	}{
		{conversationReplyModelRole, 300 * time.Second},
		{agentModelBindingKey(FormalAgentConversationReply, ""), 25 * time.Second},
		{agentModelBindingKey(FormalAgentConversationReply, "moment"), 25 * time.Second},
	} {
		ctx := withAgentModelRoute(context.Background(), FormalAgentConversationReply, "moment")
		ctx = withFrozenProviderAssignment(ctx, conversationReplyModelRole, providerAssignment{Role: conversationReplyModelRole, SourceBindingRole: tc.source, ModelID: "fixture", Timeout: 25 * time.Second})
		run := &privateReplyRun{Settings: defaultConversationReplySettings()}
		(&App{Provider: &ProviderClient{}}).freezePersonaWriterAssignment(ctx, run, FormalAgentConversationReply, "moment")
		if run.AssignmentError != nil || run.Assignment.Timeout != tc.want {
			t.Errorf("source=%q timeout=%s want=%s error=%v", tc.source, run.Assignment.Timeout, tc.want, run.AssignmentError)
		}
	}
}

func TestModelSamplingPatchDistinguishesMissingNullAndZero(t *testing.T) {
	for _, tc := range []struct {
		name      string
		payload   map[string]any
		key       string
		exclusive bool
		maximum   float64
		present   bool
		want      *float64
		valid     bool
	}{
		{name: "missing", payload: map[string]any{}, key: "temperature", maximum: 2, valid: true},
		{name: "clear", payload: map[string]any{"temperature": nil}, key: "temperature", maximum: 2, present: true, valid: true},
		{name: "zero", payload: map[string]any{"temperature": float64(0)}, key: "temperature", maximum: 2, present: true, want: floatPointer(0), valid: true},
		{name: "top_p", payload: map[string]any{"top_p": 0.65}, key: "top_p", exclusive: true, maximum: 1, present: true, want: floatPointer(0.65), valid: true},
		{name: "zero_top_p", payload: map[string]any{"top_p": float64(0)}, key: "top_p", exclusive: true, maximum: 1, present: true},
		{name: "too_high", payload: map[string]any{"temperature": 2.1}, key: "temperature", maximum: 2, present: true},
		{name: "string", payload: map[string]any{"temperature": "0.5"}, key: "temperature", maximum: 2, present: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, present, err := optionalModelSampling(tc.payload, tc.key, tc.exclusive, tc.maximum)
			if present != tc.present || (err == nil) != tc.valid || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("value=%v present=%t err=%v", got, present, err)
			}
		})
	}
}

func floatPointer(value float64) *float64 { return &value }

func TestWriterSamplingUsesBindingAndFallsBackPerField(t *testing.T) {
	ctx := withAgentModelRoute(context.Background(), FormalAgentConversationReply, "private_reply")
	ctx = withFrozenProviderAssignment(ctx, conversationReplyModelRole, providerAssignment{Role: conversationReplyModelRole, SourceBindingRole: agentModelBindingKey(FormalAgentConversationReply, "private_reply"), ModelID: "fixture", Timeout: time.Second, Temperature: floatPointer(0)})
	run := &privateReplyRun{Settings: defaultConversationReplySettings()}
	(&App{Provider: &ProviderClient{}}).freezePersonaWriterAssignment(ctx, run, FormalAgentConversationReply, "private_reply")
	if run.AssignmentError != nil || run.Assignment.Temperature == nil || *run.Assignment.Temperature != 0 || run.Assignment.TopP == nil || *run.Assignment.TopP != run.Settings.TopP {
		t.Fatalf("binding sampling or per-field fallback lost: %#v err=%v", run.Assignment, run.AssignmentError)
	}
}

func TestWriterPhysicalRequestUsesTaskTimeout(t *testing.T) {
	var remaining time.Duration
	var sentSampling map[string]any
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		sentSampling = payload
		return fakeProviderResult{Text: "一条受控回复。"}
	})
	client := &http.Client{Transport: projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		deadline, ok := request.Context().Deadline()
		if !ok {
			t.Error("physical request has no timeout")
		} else {
			remaining = time.Until(deadline)
		}
		return router.RoundTrip(request)
	})}
	app := &App{Provider: &ProviderClient{HTTP: client}}
	assignment := providerAssignment{Role: conversationReplyModelRole, SourceBindingRole: agentModelBindingKey(FormalAgentConversationReply, "moment"), BaseURL: "http://controlled.invalid", ModelID: "fixture", Timeout: 25 * time.Second, Temperature: floatPointer(0), TopP: floatPointer(0.65)}
	ctx := withAgentModelRoute(context.Background(), FormalAgentConversationReply, "moment")
	ctx = withFrozenProviderAssignment(ctx, conversationReplyModelRole, assignment)
	run := &privateReplyRun{Settings: defaultConversationReplySettings()}
	app.freezePersonaWriterAssignment(ctx, run, FormalAgentConversationReply, "moment")
	input := replyContextFixture()
	input.Scene = replySceneMoment
	_, err := app.RunConversationReplyTask(context.Background(), ConversationReplyTaskInput{Context: input, Assignment: run.Assignment, CorrelationID: "task-timeout-fixture", Concurrency: 1})
	if err != nil || remaining <= 20*time.Second || remaining > 25*time.Second || sentSampling["temperature"] != float64(0) || sentSampling["top_p"] != 0.65 {
		t.Fatalf("physical request did not use task configuration: remaining=%s sampling=%v err=%v", remaining, sentSampling, err)
	}
}

func TestMediaNormalizationCorrectionKeepsFrozenAssignment(t *testing.T) {
	concept := mediaPreparedConcept(t, "本人在卧室里的自拍", mediaTestPlan("person", "present", "yes", "full"), mediaTestContext("CURRENT"))
	concept["human_count"] = 1
	valid := mediaTestOutput(concept)
	invalid := cloneMap(valid)
	invalid["declared_human_count"] = 0
	var models []string
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		models = append(models, stringValue(payload["model"]))
		if len(models) == 1 {
			return fakeProviderResult{Structured: invalid}
		}
		return fakeProviderResult{Structured: valid}
	})
	app := &App{Provider: &ProviderClient{HTTP: &http.Client{Transport: router}}}
	ctx := withFrozenProviderAssignment(context.Background(), "media_prompt", providerAssignment{Role: "generic_llm", BaseURL: "http://controlled.invalid", ModelID: "frozen-normalizer", Timeout: time.Second})
	result, err := app.runBranchedMediaPrompt(ctx, MediaPromptTaskInput{}, concept)
	if err != nil || result.RepairCount != 1 || !reflect.DeepEqual(models, []string{"frozen-normalizer", "frozen-normalizer"}) {
		t.Fatalf("normalization correction lost its frozen model: models=%v repairs=%d err=%v", models, result.RepairCount, err)
	}
}

func TestMediaNormalizationQualityRetryCarriesRealFeedback(t *testing.T) {
	concept := mediaPreparedConcept(t, "本人在卧室里的自拍", mediaTestPlan("person", "present", "yes", "full"), mediaTestContext("CURRENT"))
	concept["human_count"] = 1
	feedback := map[string]any{"verdict": "retry", "violations": []any{map[string]any{"code": "capture_mismatch", "detail": "手机入镜"}}, "observed_facts": map[string]any{"capture_matches": false}, "retry_guidance": "保留卧室并移出手机"}
	for _, test := range []struct {
		name         string
		intent       mediaIntent
		wantFeedback map[string]any
		wantPrevious bool
		repair       bool
	}{
		{"initial", mediaIntent{ProviderPrompt: "stale prompt", QualityRetryFeedback: feedback}, nil, false, false},
		{"review feedback", mediaIntent{ProviderPrompt: "上一轮真实提示词", QualityRetryCount: 1, QualityRetryFeedback: feedback}, feedback, true, true},
		{"legacy guidance", mediaIntent{ProviderPrompt: "上一轮真实提示词", QualityRetryCount: 1, QualityRetryGuidance: "保留原画面"}, map[string]any{"retry_guidance": "保留原画面"}, true, false},
		{"missing feedback", mediaIntent{QualityRetryCount: 1}, nil, false, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			valid := mediaTestOutput(concept)
			invalid := cloneMap(valid)
			invalid["declared_human_count"] = 0
			var packets []map[string]any
			var models []string
			var systems []string
			router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
				models = append(models, stringValue(payload["model"]))
				for _, raw := range arrayValue(payload["messages"]) {
					message := mapValue(raw)
					if message["role"] == "system" {
						systems = append(systems, stringValue(message["content"]))
					}
					if message["role"] != "user" {
						continue
					}
					var packet map[string]any
					if json.Unmarshal([]byte(stringValue(message["content"])), &packet) == nil && packet["intent"] != nil {
						packets = append(packets, packet)
					}
				}
				if test.repair && len(models) == 1 {
					return fakeProviderResult{Structured: invalid}
				}
				return fakeProviderResult{Structured: valid}
			})
			app := &App{Provider: &ProviderClient{HTTP: &http.Client{Transport: router}}}
			ctx := withFrozenProviderAssignment(context.Background(), "media_prompt", providerAssignment{Role: "generic_llm", BaseURL: "http://controlled.invalid", ModelID: "frozen-normalizer", Timeout: time.Second})
			before := jsonString(concept)
			result, err := app.runBranchedMediaPrompt(ctx, MediaPromptTaskInput{Intent: test.intent}, concept)
			wantRequests := 1
			if test.repair {
				wantRequests = 2
			}
			if err != nil || len(packets) != wantRequests || len(models) != wantRequests || result.RepairCount != wantRequests-1 {
				t.Fatalf("retry requests=%d models=%v repairs=%d err=%v", len(packets), models, result.RepairCount, err)
			}
			for index, packet := range packets {
				if models[index] != "frozen-normalizer" || packet["intent"] != concept["intent"] || jsonString(packet["media_capture_plan"]) != jsonString(concept["media_capture_plan"]) {
					t.Fatalf("frozen intent, plan or model changed: %#v", packet)
				}
				actualFeedback, hasFeedback := packet["quality_feedback"]
				if hasFeedback != (test.wantFeedback != nil) || hasFeedback && jsonString(actualFeedback) != jsonString(test.wantFeedback) || (packet["previous_provider_prompt"] != nil) != test.wantPrevious {
					t.Fatalf("quality retry inputs changed: %#v", packet)
				}
				if (strings.Contains(systems[index], mediaNormalizationRetryInstruction)) != (test.intent.QualityRetryCount > 0) {
					t.Fatalf("quality retry protocol misplaced: %s", systems[index])
				}
				if test.wantPrevious && packet["previous_provider_prompt"] != test.intent.ProviderPrompt {
					t.Fatalf("previous prompt changed: %#v", packet)
				}
				if (packet["validation_error"] != nil) != (index > 0) {
					t.Fatalf("repair feedback misplaced: %#v", packet)
				}
			}
			if jsonString(concept) != before {
				t.Fatal("normalization mutated the frozen concept")
			}
		})
	}
}

func TestMediaNormalizationRejectsInvalidConceptBeforeProviderLookup(t *testing.T) {
	_, err := (&App{}).runBranchedMediaPrompt(context.Background(), MediaPromptTaskInput{}, map[string]any{})
	if err == nil || err.Error() == "model_task_provider_unavailable" {
		t.Fatalf("invalid media concept reached provider lookup: %v", err)
	}
}

func TestMediaNormalizationCorrectionDoesNotRebindAfterConfigurationChange(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	seedCognitiveProviderRole(t, ctx, repository, "normalization-route-endpoint")
	role := agentModelBindingKey(FormalAgentMediaPrompt, "media_prompt_normalized")
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,timeout_seconds) VALUES($1,'normalization-route-endpoint','first-model',30)`, role); err != nil {
		t.Fatal(err)
	}
	concept := mediaPreparedConcept(t, "本人在卧室里的自拍", mediaTestPlan("person", "present", "yes", "full"), mediaTestContext("CURRENT"))
	concept["human_count"] = 1
	valid := mediaTestOutput(concept)
	invalid := cloneMap(valid)
	invalid["declared_human_count"] = 0
	var models []string
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		models = append(models, stringValue(payload["model"]))
		if len(models) == 1 {
			if _, err := repository.Pool().Exec(ctx, `UPDATE public.model_roles SET model_id='new-model' WHERE role=$1`, role); err != nil {
				t.Error(err)
			}
			return fakeProviderResult{Structured: invalid}
		}
		return fakeProviderResult{Structured: valid}
	})
	app := &App{DB: repository, Provider: &ProviderClient{DB: repository, HTTP: &http.Client{Transport: router}}}
	result, err := app.runBranchedMediaPrompt(ctx, MediaPromptTaskInput{}, concept)
	if err != nil || result.RepairCount != 1 || !reflect.DeepEqual(models, []string{"first-model", "first-model"}) {
		t.Fatalf("configuration change rebound normalization correction: models=%v repairs=%d err=%v", models, result.RepairCount, err)
	}
}

func TestMediaNormalizationAssignmentFailureKeepsDiagnostics(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ctx = WithProviderCorrelation(ctx, "normalizer-assignment-failure")
	concept := mediaPreparedConcept(t, "无人的街道", mediaTestPlan("landscape", "none", "no", "unknown"), mediaTestContext("FACE"))
	app := &App{DB: repository, Provider: &ProviderClient{DB: repository}}
	if _, err := app.runBranchedMediaPrompt(ctx, MediaPromptTaskInput{Intent: mediaIntent{ID: "failed-normalization"}}, concept); err == nil {
		t.Fatal("missing model binding was accepted")
	}
	var preflight, normalized int
	err := repository.Pool().QueryRow(ctx, `SELECT count(*) FILTER (WHERE event_type='provider.preflight.failed' AND payload->>'stage'='assignment' AND payload->>'scenario'='media_prompt'), count(*) FILTER (WHERE event_type='media.prompt.normalized' AND payload->>'validation_passed'='false') FROM public.diagnostic_events WHERE correlation_id=$1`, "normalizer-assignment-failure").Scan(&preflight, &normalized)
	if err != nil || preflight != 1 || normalized != 1 {
		t.Fatalf("failure diagnostics missing or duplicated: preflight=%d normalized=%d err=%v", preflight, normalized, err)
	}
}

func TestAgentModelBindingSaveSelectAndFrozenRun(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	seedCognitiveProviderRole(t, ctx, repository, "agent-routing-endpoint")
	for _, binding := range []struct{ role, model string }{
		{"agent:reflection", "agent-model"},
		{"agent:reflection:reflection_proposal_v2", "variant-model"},
	} {
		if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,timeout_seconds) VALUES($1,'agent-routing-endpoint',$2,30)`, binding.role, binding.model); err != nil {
			t.Fatal(err)
		}
	}
	provider := &ProviderClient{DB: repository}
	variantCtx := withAgentModelRoute(ctx, FormalAgentReflection, "reflection_proposal_v2")
	first, err := provider.assignment(variantCtx, "reflection")
	if err != nil || first.ModelID != "variant-model" || first.SourceBindingRole != "agent:reflection:reflection_proposal_v2" {
		t.Fatalf("variant did not select its binding: model=%q source=%q err=%v", first.ModelID, first.SourceBindingRole, err)
	}
	agentCtx := withAgentModelRoute(ctx, FormalAgentReflection, "other_variant")
	if agent, err := provider.assignment(agentCtx, "reflection"); err != nil || agent.ModelID != "agent-model" {
		t.Fatalf("agent override did not apply: %#v err=%v", agent, err)
	}
	otherCtx := withAgentModelRoute(ctx, FormalAgentNativeCognition, "native_cognition_response")
	if other, err := provider.assignment(otherCtx, "cognitive_assessment"); err != nil || other.ModelID != "turn-model" {
		t.Fatalf("another task changed: %#v err=%v", other, err)
	}
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.model_roles SET model_id='new-variant-model' WHERE role='agent:reflection:reflection_proposal_v2'`); err != nil {
		t.Fatal(err)
	}
	if current, err := provider.assignment(variantCtx, "reflection"); err != nil || current.ModelID != "new-variant-model" {
		t.Fatalf("saved change was not read: %#v err=%v", current, err)
	}
	if frozen, err := provider.assignment(withFrozenProviderAssignment(variantCtx, "reflection", first), "reflection"); err != nil || frozen.ModelID != "variant-model" {
		t.Fatalf("in-progress assignment was silently rebound: %#v err=%v", frozen, err)
	}
	var actualModel string
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		actualModel = stringValue(payload["model"])
		return fakeProviderResult{Structured: map[string]any{"ok": true}}
	})
	provider.HTTP = &http.Client{Transport: router}
	app := &App{DB: repository, Provider: provider}
	schema := objectSchema(map[string]any{"ok": booleanSchema()}, []string{"ok"}, false)
	_, err = app.RunFormalAgent(ctx, FormalAgentReflection, FormalAgentRunInput{Prompt: PromptAssemblyResult{Messages: []map[string]any{{"role": "user", "content": "fixture"}}, ResponseFormat: schema}, SchemaName: "reflection_proposal_v2"})
	if err != nil || actualModel != "new-variant-model" {
		t.Fatalf("formal physical request ignored variant binding: model=%q err=%v", actualModel, err)
	}
}

func TestAgentModelConfigurationSavesReadsRunsAndClears(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	const owner, endpoint = "agent-route-owner", "agent-route-endpoint"
	const targetModel = "llmfan46/G4-MeroMero-26B-A4B-it-uncensored-heretic-4bit-mxfp4"
	const internalModel = "Huihui-Qwen3.8-27B-3.8bpw"
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'fixture','fixture')`, owner); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status,checked_at) VALUES($1,'openai_compatible','http://controlled.invalid','agent-route-secret','ready',now())`, endpoint); err != nil {
		t.Fatal(err)
	}
	var actualModel string
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		actualModel = stringValue(payload["model"])
		return fakeProviderResult{Structured: map[string]any{"ok": true}}
	})
	client := &http.Client{Transport: projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		if strings.HasSuffix(request.URL.Path, "/models") {
			return embeddingHTTPResponse(request, http.StatusOK, jsonString(map[string]any{"data": []any{map[string]any{"id": targetModel}, map[string]any{"id": "agent-fixture"}, map[string]any{"id": internalModel}, map[string]any{"id": "vision-fixture"}}})), nil
		}
		return router.RoundTrip(request)
	})}
	provider := &ProviderClient{DB: repository, HTTP: client}
	app := &App{DB: repository, Provider: provider}
	save := func(role, model, key string) {
		t.Helper()
		current, err := app.ReadSettings(ctx, owner)
		if err != nil {
			t.Fatal(err)
		}
		body := map[string]any{"role": role, "endpoint_id": endpoint, "model_id": model, "timeout_seconds": 30, "expected_versions": current["versions"], "idempotency_key": key}
		if err := app.ConfigureProviderRole(ctx, owner, body); err != nil {
			t.Fatal(err)
		}
	}
	save(conversationReplyModelRole, targetModel, "route-default")
	save(visionUnderstandingModelRole, "vision-fixture", "route-vision")
	mainCtx := withAgentModelRoute(ctx, FormalAgentConversationCognition, "conversation_turn_response")
	if main, err := provider.assignment(mainCtx, "cognitive_assessment"); err != nil || main.ModelID != targetModel {
		t.Fatalf("character default required another model: %#v err=%v", main, err)
	}
	if vision, err := provider.assignment(withAgentModelRoute(ctx, FormalAgentMediaQuality, "media_quality_acceptance_response"), visionUnderstandingModelRole); err != nil || vision.ModelID != "vision-fixture" {
		t.Fatalf("text default took over vision: %#v err=%v", vision, err)
	}
	schema := objectSchema(map[string]any{"ok": booleanSchema()}, []string{"ok"}, false)
	_, err := app.RunFormalAgent(ctx, FormalAgentExpressionLearning, FormalAgentRunInput{Prompt: PromptAssemblyResult{Messages: []map[string]any{{"role": "user", "content": "fixture"}}, ResponseFormat: schema}, SchemaName: expressionLearningSchema})
	if err != nil || actualModel != targetModel {
		t.Fatalf("default text request lost the configured target model: model=%q err=%v", actualModel, err)
	}
	save("agent:reflection", "agent-fixture", "route-agent")
	save("agent:reflection:reflection_proposal_v2", internalModel, "route-variant")
	variantCtx := withAgentModelRoute(ctx, FormalAgentReflection, "reflection_proposal_v2")
	if other, err := provider.assignment(mainCtx, "cognitive_assessment"); err != nil || other.ModelID != targetModel {
		t.Fatalf("another task changed with reflection override: %#v err=%v", other, err)
	}
	catalog, err := app.OwnerConfigurationCatalog(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, raw := range arrayValue(catalog["agent_model_routes"]) {
		route := mapValue(raw)
		if route["key"] == "agent:reflection:reflection_proposal_v2" {
			found = route["source"] == "variant" && route["model_id"] == internalModel
		}
	}
	if !found {
		t.Fatal("saved variant was not visible as the effective route")
	}
	_, err = app.RunFormalAgent(ctx, FormalAgentReflection, FormalAgentRunInput{Prompt: PromptAssemblyResult{Messages: []map[string]any{{"role": "user", "content": "fixture"}}, ResponseFormat: schema}, SchemaName: "reflection_proposal_v2"})
	if err != nil || actualModel != internalModel {
		t.Fatalf("next formal request did not use saved variant: model=%q err=%v", actualModel, err)
	}
	current, err := app.ReadSettings(ctx, owner)
	if err != nil {
		t.Fatal(err)
	}
	clear := map[string]any{"role": "agent:reflection:reflection_proposal_v2", "clear_override": true, "expected_versions": current["versions"], "idempotency_key": "route-clear"}
	if err := app.ConfigureProviderRole(ctx, owner, clear); err != nil {
		t.Fatal(err)
	}
	if err := app.ConfigureProviderRole(ctx, owner, clear); err != nil {
		t.Fatalf("idempotent clear failed: %v", err)
	}
	if inherited, err := provider.assignment(variantCtx, "reflection"); err != nil || inherited.ModelID != "agent-fixture" {
		t.Fatalf("clear did not restore Agent inheritance: %#v err=%v", inherited, err)
	}
}

func TestAgentSamplingConfigurationSavesRunsAndClearsPerField(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	const owner, endpoint, role = "sampling-owner", "sampling-endpoint", "agent:reflection:reflection_proposal_v2"
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'fixture','fixture')`, owner); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status,checked_at) VALUES($1,'openai_compatible','http://controlled.invalid','sampling-secret','ready',now())`, endpoint); err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		wire = payload
		return fakeProviderResult{Structured: map[string]any{"ok": true}}
	})
	client := &http.Client{Transport: projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		if strings.HasSuffix(request.URL.Path, "/models") {
			return embeddingHTTPResponse(request, http.StatusOK, `{"data":[{"id":"sampling-model"}]}`), nil
		}
		return router.RoundTrip(request)
	})}
	app := &App{DB: repository, Provider: &ProviderClient{DB: repository, HTTP: client}}
	save := func(key string, patch map[string]any) {
		t.Helper()
		settings, err := app.ReadSettings(ctx, owner)
		if err != nil {
			t.Fatal(err)
		}
		body := map[string]any{"role": role, "endpoint_id": endpoint, "model_id": "sampling-model", "timeout_seconds": 30, "expected_versions": settings["versions"], "idempotency_key": key}
		for k, v := range patch {
			body[k] = v
		}
		if err := app.ConfigureProviderRole(ctx, owner, body); err != nil {
			t.Fatal(err)
		}
	}
	save("sampling-first", map[string]any{"temperature": float64(0), "top_p": 0.6})
	route := withAgentModelRoute(ctx, FormalAgentReflection, "reflection_proposal_v2")
	assignment, err := app.Provider.assignment(route, "reflection")
	if err != nil || assignment.Temperature == nil || *assignment.Temperature != 0 || assignment.TopP == nil || *assignment.TopP != 0.6 {
		t.Fatalf("sampling not read: %#v err=%v", assignment, err)
	}
	schema := objectSchema(map[string]any{"ok": booleanSchema()}, []string{"ok"}, false)
	_, err = app.RunFormalAgent(route, FormalAgentReflection, FormalAgentRunInput{Prompt: PromptAssemblyResult{Messages: []map[string]any{{"role": "user", "content": "fixture"}}, ResponseFormat: schema}, SchemaName: "reflection_proposal_v2"})
	if err != nil || wire["temperature"] != float64(0) || wire["top_p"] != 0.6 {
		t.Fatalf("physical request lost sampling: %v err=%v", wire, err)
	}
	save("sampling-preserve", map[string]any{})
	kept, err := app.Provider.assignment(route, "reflection")
	if err != nil || kept.Temperature == nil || *kept.Temperature != 0 || kept.TopP == nil || *kept.TopP != 0.6 {
		t.Fatalf("legacy save reset sampling: %#v err=%v", kept, err)
	}
	save("sampling-clear", map[string]any{"temperature": nil})
	cleared, err := app.Provider.assignment(route, "reflection")
	if err != nil || cleared.Temperature != nil || cleared.TopP == nil || *cleared.TopP != 0.6 {
		t.Fatalf("explicit clear changed another field: %#v err=%v", cleared, err)
	}
}

func TestAgentModelBindingKeysAreUniqueValidAndFitExistingColumn(t *testing.T) {
	seen := map[string]FormalAgentID{}
	for agent, variants := range agentModelVariants {
		if _, ok := FormalAgentDefinitionByID(agent); !ok || !validAgentModelBindingRole(agentModelBindingKey(agent, "")) {
			t.Fatalf("unregistered agent route %q", agent)
		}
		for _, variant := range variants {
			key := agentModelBindingKey(agent, variant)
			if !validAgentModelBindingRole(key) || len(key) > 64 {
				t.Fatalf("invalid or oversized binding key %q", key)
			}
			if previous, duplicate := seen[variant]; duplicate {
				t.Fatalf("schema variant %q maps ambiguously to %s and %s", variant, previous, agent)
			}
			seen[variant] = agent
		}
	}
	for _, agent := range []FormalAgentID{FormalAgentMediaQuality, FormalAgentVisualIdentityVision, FormalAgentVisualIdentity} {
		if validAgentModelBindingRole(agentModelBindingKey(agent, "")) {
			t.Fatalf("unverified image-input agent %s accepted a text override", agent)
		}
	}
	for _, invalid := range []string{"agent:missing", "agent:reflection:", "agent:reflection:unknown", "agent:reflection:reflection_proposal_v2:extra"} {
		if validAgentModelBindingRole(invalid) {
			t.Fatalf("invalid route accepted: %q", invalid)
		}
	}
}
