package core

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestVisionDisabledDomainCallsNeverRequestAndDoNotDisableTextStages(t *testing.T) {
	router := newFakeProviderRouter().otherwise(func(map[string]any) fakeProviderResult {
		t.Fatal("disabled vision made HTTP request")
		return fakeProviderResult{}
	})
	app := &App{Provider: &ProviderClient{HTTP: &http.Client{Transport: router}}}
	if _, err := app.RunMediaQualityTask(context.Background(), MediaQualityTaskInput{}); err == nil || err.Error() != "vision_disabled" {
		t.Fatalf("quality err=%v", err)
	}
	if _, err := app.RunVisualIdentityVisionTask(context.Background(), VisualIdentityVisionTaskInput{}); err == nil || err.Error() != "vision_disabled" {
		t.Fatalf("legacy vision err=%v", err)
	}
	definition, _ := FormalAgentDefinitionByID(FormalAgentVisualIdentity)
	if providerBindingRole(definition.Role) != "generic_llm" {
		t.Fatalf("text stage needs visual binding: %s", definition.Role)
	}
	if router.totalRequests() != 0 {
		t.Fatal("disabled called model")
	}
}

func TestVisionPhysicalInputUsesIndependentBindingAndExecutionTimeGate(t *testing.T) {
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		if stringValue(payload["model"]) != "independent-vision" {
			t.Fatalf("wrong model %v", payload["model"])
		}
		if !strings.Contains(jsonString(payload["messages"]), "data:image/png;base64,") {
			t.Fatal("pixels missing from actual request")
		}
		return fakeProviderResult{Text: "red,blue"}
	})
	provider := &ProviderClient{HTTP: &http.Client{Transport: router}}
	enabled := true
	ctx := context.WithValue(context.Background(), visionSettingsReaderKey{}, func(context.Context) (VisionUnderstandingSettings, error) {
		return VisionUnderstandingSettings{Enabled: enabled, ModelRef: visionUnderstandingModelRole}, nil
	})
	assignment := providerAssignment{Role: visionUnderstandingModelRole, BaseURL: "http://controlled.invalid", ModelID: "independent-vision", Timeout: time.Second, TokenBudget: 1024, ContextWindowTokens: 16384, MaxInputTokens: 8192, PromptBudgetPolicyVersion: promptBudgetPolicyVersionV1}
	pixels, _ := visionProbeImage()
	url, _ := mediaQualityImageDataURL("image/png", pixels)
	call := EinoModelCall{Assignment: assignment, Role: visionUnderstandingModelRole, Messages: []map[string]any{{"role": "user", "content": []any{map[string]any{"type": "image_url", "image_url": map[string]any{"url": url}}}}}}
	if _, err := provider.generateWithEino(ctx, call); err != nil {
		t.Fatal(err)
	}
	if router.totalRequests() != 1 {
		t.Fatal("ready vision did not call once")
	}
	enabled = false
	if _, err := provider.generateWithEino(ctx, call); err == nil || err.Error() != "vision_disabled" {
		t.Fatalf("changed gate err=%v", err)
	}
	if _, err := provider.streamWithEino(ctx, assignment, call.Messages, "stream", nil); err == nil || err.Error() != "vision_disabled" {
		t.Fatalf("stream disabled err=%v", err)
	}
	enabled = true
	call.Assignment.Role = "generic_llm"
	if _, err := provider.generateWithEino(ctx, call); err == nil || err.Error() != "vision_independent_binding_required" {
		t.Fatalf("generic image err=%v", err)
	}
	if router.totalRequests() != 1 {
		t.Fatal("blocked images caused HTTP")
	}
	// Execution-time physical policy rechecks rather than trusting a prior gate.
	ctx = context.WithValue(ctx, visionSettingsReaderKey{}, func(context.Context) (VisionUnderstandingSettings, error) {
		return VisionUnderstandingSettings{}, errors.New("db unavailable")
	})
	if _, err := provider.generateWithEino(ctx, call); err == nil || err.Error() != "vision_config_invalid" {
		t.Fatalf("gate failure err=%v", err)
	}
}

func TestVisionOptionalReviewPausesReadyCandidateAndRejectsForgedReview(t *testing.T) {
	fixture := newVisualIdentityToolFixture(t)
	generated := fixture.generateCandidate(t, "vision-disabled-candidate")
	asset := fixture.makeCurrentCandidateReady(t, stringValue(mapValue(generated.Result.Output)["media_intent_id"]))
	output, err := fixture.app.RunVisualIdentityAgent(fixture.ctx, VisualIdentityAgentInput{SessionID: fixture.sessionID})
	if err != nil {
		t.Fatal(err)
	}
	if output.Status != "awaiting_review" || output.CandidateAssetID != asset || output.CanonicalAssetID != "" {
		t.Fatalf("skip=%#v", output)
	}
	repeated, err := fixture.app.RunVisualIdentityAgent(fixture.ctx, VisualIdentityAgentInput{SessionID: fixture.sessionID})
	if err != nil || repeated.Status != "awaiting_review" {
		t.Fatalf("paused retried: %#v %v", repeated, err)
	}
	args := visualIdentityAcceptedReview()
	request := fixture.request(visualIdentityCommitReviewCapabilityName, "forged-review", args)
	delete(args, "review_source")
	request.Arguments = jsonBytes(args)
	if _, err := fixture.app.ExecuteTool(fixture.ctx, request); err == nil {
		t.Fatal("forged review accepted")
	}
}

func TestVisionRawProviderEntryRejectsBeforeAssignmentOrSecretLookup(t *testing.T) {
	// A nil DB would panic in assignment/secret. These raw entrances must reject
	// the image first, including requests that bypass the typed domain helpers.
	provider := &ProviderClient{}
	messages := []map[string]any{{"role": "user", "content": []any{map[string]any{"type": "image_url", "image_url": map[string]any{"url": "data:image/png;base64,abc"}}}}}
	for _, settings := range []VisionUnderstandingSettings{{Enabled: false, ModelRef: visionUnderstandingModelRole}, {Enabled: true, ModelRef: ""}} {
		ctx := context.WithValue(context.Background(), visionSettingsReaderKey{}, func(context.Context) (VisionUnderstandingSettings, error) { return settings, nil })
		if _, err := provider.completeWithToolsSchema(ctx, "generic_llm", messages, false, nil, "", nil, false); err == nil {
			t.Fatal("image reached assignment")
		}
		if _, err := provider.StreamText(ctx, "action_realization", messages, nil); err == nil {
			t.Fatal("stream reached assignment")
		}
	}
}

// Explicitly opt older controlled image tests into the independent binding;
// the shared App fixture retains disabled defaults for all other tests.
func enableControlledVision(t *testing.T, ctx context.Context, repo *PostgresRepository) {
	t.Helper()
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,required_capabilities,token_budget,timeout_seconds,retry_policy,context_window_tokens,max_input_tokens,prompt_budget_policy_version) SELECT 'vision_understanding',provider_endpoint_id,model_id,'structured_output,tool_calling,multimodal_input',token_budget,timeout_seconds,retry_policy,context_window_tokens,max_input_tokens,prompt_budget_policy_version FROM public.model_roles WHERE role IN ('generic_llm','cognitive_assessment') ORDER BY CASE WHEN role='generic_llm' THEN 0 ELSE 1 END LIMIT 1 ON CONFLICT(role) DO NOTHING`); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.runtime_settings(key,value_json,revision) VALUES('vision.understanding','{"enabled":true,"model_ref":"vision_understanding"}',1) ON CONFLICT(key) DO UPDATE SET value_json=excluded.value_json,revision=runtime_settings.revision+1`); err != nil {
		t.Fatal(err)
	}
}
