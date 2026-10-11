package workflow

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
)

type mediaValidationService struct {
	ApplicationService
	calls, failures int
}

func (s *mediaValidationService) ProcessMediaIntent(context.Context, string) (map[string]any, error) {
	s.calls++
	return nil, fmt.Errorf("media prompt generation failed: %w", &core.CapabilityError{Code: "media_prompt_validation_failed", Retryable: false})
}
func (s *mediaValidationService) RecordMediaActivityFailure(context.Context, string, string) error {
	s.failures++
	return nil
}
func TestMediaValidationFailureDoesNotRestartNormalization(t *testing.T) {
	previous := app()
	service := &mediaValidationService{}
	Configure(service)
	t.Cleanup(func() { Configure(previous) })
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterActivity(ProcessMediaActivity)
	env.ExecuteWorkflow(MediaWorkflow, Input{IntentID: "invalid-prompt"})
	if !env.IsWorkflowCompleted() || env.GetWorkflowError() == nil {
		t.Fatal("invalid prompt workflow did not fail")
	}
	if service.calls != 1 || service.failures != 1 {
		t.Fatalf("normalization reset: calls=%d failures=%d", service.calls, service.failures)
	}
	var failure *temporal.ApplicationError
	if !errors.As(env.GetWorkflowError(), &failure) || !failure.NonRetryable() || failure.Type() != "media_prompt_validation_failed" {
		t.Fatalf("failure type=%v", env.GetWorkflowError())
	}
}
func TestMediaActivityKeepsTransportRetryAndCancellation(t *testing.T) {
	for _, err := range []error{context.Canceled, context.DeadlineExceeded, errors.New("transport failed"), &core.CapabilityError{Code: "provider_request_failed", Retryable: true}} {
		if mediaActivityFailure(err) != err {
			t.Fatalf("changed retry/cancel: %v", err)
		}
	}
}
