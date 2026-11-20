package workflow

import (
	"errors"
	"strings"
	"testing"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
)

func TestMediaCapacityClassificationAndActivityRetries(t *testing.T) {
	for _, sample := range []struct {
		name         string
		message      string
		code         string
		wantCapacity bool
		wantCalls    int32
	}{
		{
			name:      "renderer_gpu_400_is_ambiguous",
			message:   `ComfyUI returned HTTP 400: {"error":{"message":"Prompt requires ~1687MB GPU memory but only ~0MB is available"}}`,
			code:      "media_submission_ambiguous",
			wantCalls: 1,
		},
		{
			name:         "nonretryable_media_error_preserves_llm_capacity_evidence",
			message:      "provider HTTP 400: Prompt (2340 tokens) requires ~1687MB GPU memory but only ~0MB is available",
			code:         "media_prompt_validation_failed",
			wantCapacity: true,
			wantCalls:    1,
		},
		{
			name:         "raw_capacity_error_exhausts_bounded_activity_retries",
			message:      "provider HTTP 503: GPU error: out of memory",
			wantCapacity: true,
			wantCalls:    mediaActivityMaximumAttempts,
		},
	} {
		t.Run(sample.name, func(t *testing.T) {
			cause := errors.New(sample.message)
			var original error = cause
			if sample.code != "" {
				original = &core.CapabilityError{Code: sample.code, Retryable: false, Cause: cause}
			}
			converted := mediaActivityFailure(original)
			var applicationErr *temporal.ApplicationError
			if sample.code != "" {
				if !errors.As(converted, &applicationErr) || !applicationErr.NonRetryable() || applicationErr.Type() != sample.code {
					t.Fatalf("classification: got %v, want nonretryable ApplicationError type %q", converted, sample.code)
				}
			} else {
				if converted != original {
					t.Fatalf("raw capacity error was replaced: got %v, want original error", converted)
				}
				if errors.As(converted, &applicationErr) {
					t.Fatalf("raw capacity error became an ApplicationError: %v", converted)
				}
			}
			if !errors.Is(converted, cause) {
				t.Fatalf("classification lost original cause: %v", converted)
			}
			for _, failure := range []error{cause, original, converted} {
				if got := core.ProviderCapacityFailure(failure.Error()); got != sample.wantCapacity {
					t.Fatalf("ProviderCapacityFailure(%q) = %v, want %v", failure.Error(), got, sample.wantCapacity)
				}
				// These are the capacity refund/requeue gates used by ReconcileOnce.
				if providerCapacityIntent("media.generation") && core.ProviderCapacityFailure(failure.Error()) {
					t.Fatalf("media.generation unexpectedly qualifies for capacity refund: %v", failure)
				}
			}

			service := &ambiguousContextService{cause: cause, code: sample.code, retryable: false}
			previous := app()
			Configure(service)
			defer Configure(previous)
			var suite testsuite.WorkflowTestSuite
			env := suite.NewTestWorkflowEnvironment()
			env.RegisterActivity(ProcessMediaActivity)
			env.ExecuteWorkflow(MediaWorkflow, Input{IntentID: "media-capacity-" + sample.name})
			if !env.IsWorkflowCompleted() {
				t.Fatal("media workflow did not complete")
			}
			workflowErr := env.GetWorkflowError()
			if workflowErr == nil {
				t.Fatal("media workflow unexpectedly succeeded")
			}
			if got := service.calls.Load(); got != sample.wantCalls {
				t.Fatalf("activity calls = %d, want %d; workflow error: %v", got, sample.wantCalls, workflowErr)
			}
			if got := service.failures.Load(); got != 1 {
				t.Fatalf("terminal failure records = %d, want 1", got)
			}
			var workflowApplicationErr *temporal.ApplicationError
			if !errors.As(workflowErr, &workflowApplicationErr) {
				t.Fatalf("workflow lost serialized ApplicationError: %v", workflowErr)
			}
			if got, want := workflowApplicationErr.NonRetryable(), sample.code != ""; got != want {
				t.Fatalf("serialized NonRetryable = %v, want %v", got, want)
			}
			if sample.code != "" && workflowApplicationErr.Type() != sample.code {
				t.Fatalf("serialized type = %q, want %q", workflowApplicationErr.Type(), sample.code)
			}
			if !strings.Contains(workflowApplicationErr.Error(), sample.message) {
				t.Fatalf("serialized error lost failure message: %v", workflowApplicationErr)
			}
			if got := core.ProviderCapacityFailure(workflowApplicationErr.Error()); got != sample.wantCapacity {
				t.Fatalf("serialized capacity evidence = %v, want %v: %v", got, sample.wantCapacity, workflowApplicationErr)
			}
		})
	}
}

func TestMediaCapacityIntentScope(t *testing.T) {
	for _, sample := range []struct {
		intent string
		want   bool
	}{
		{"schedule.current_day", true},
		{"wardrobe.initialize", true},
		{"visual_identity.initialize", true},
		{"wake_up.current", true},
		{"media.generation", false},
		{"media.delete", false},
	} {
		t.Run(sample.intent, func(t *testing.T) {
			if got := providerCapacityIntent(sample.intent); got != sample.want {
				t.Fatalf("providerCapacityIntent(%q) = %v, want %v", sample.intent, got, sample.want)
			}
		})
	}
}
