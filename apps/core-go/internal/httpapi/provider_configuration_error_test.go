package httpapi

import (
	"errors"
	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/httpapi/browser"
	"testing"
)

func TestProviderConfigurationValidationIsNotAnUnknownGatewayFailure(t *testing.T) {
	for _, code := range []string{"provider_prompt_budget_invalid", "prompt_budget_policy_unknown", "provider_model_not_available"} {
		var got *browser.CoreError
		if !errors.As(browserProviderRoleValidationError(errors.New(code)), &got) || got.Status != 422 || got.Code != code || got.Details["configuration_admission"] != "rejected" {
			t.Fatalf("%s became %#v", code, got)
		}
	}
	for _, err := range []error{nil, errors.New("database commit failed"), errors.New("provider_models_unavailable: transient HTTP error"), errors.New("private error containing provider_prompt_budget_invalid text")} {
		if browserProviderRoleValidationError(err) != nil {
			t.Fatalf("misclassified unknown outcome %v", err)
		}
	}
}
