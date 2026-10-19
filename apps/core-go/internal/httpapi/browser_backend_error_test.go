package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/httpapi/browser"
)

func TestBrowserConversationTurnErrorPreservesOnlyStableCode(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{name: "provider failure before first frame", err: errors.New("provider request failed: private upstream details"), want: "provider_request_failed"},
		{name: "final contract failure before first frame", err: errors.New("agent_final_contract_invalid: private validation details"), want: "agent_final_contract_invalid"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := browserConversationTurnError(testCase.err)
			var coreErr *browser.CoreError
			if !errors.As(got, &coreErr) || coreErr == nil {
				t.Fatalf("error = %T %v, want *browser.CoreError", got, got)
			}
			if coreErr.Code != testCase.want {
				t.Fatalf("code = %q, want %q", coreErr.Code, testCase.want)
			}
			if strings.Contains(coreErr.Error(), "private") {
				t.Fatalf("raw failure detail crossed the browser boundary: %q", coreErr.Error())
			}
		})
	}
}

func TestBrowserConversationTurnErrorLeavesUnknownFailuresForFallback(t *testing.T) {
	err := errors.New("private database detail")
	if got := browserConversationTurnError(err); got != err {
		t.Fatalf("error = %v, want original unknown error for route fallback", got)
	}
}

func TestBrowserInitializationAnalysisErrorPreservesCorrelatedFailures(t *testing.T) {
	for _, item := range []struct {
		code, validationType string
		status               int
	}{
		{"initialization_input_budget_exceeded", "configuration", http.StatusUnprocessableEntity},
		{"initialization_provider_timeout", "provider", http.StatusServiceUnavailable},
		{"initialization_persona_invalid", "semantic_invalid", http.StatusUnprocessableEntity},
		{"initialization_source_persistence_failed", "persistence", http.StatusInternalServerError},
	} {
		details := map[string]any{"correlation_id": "initialization-analysis:test", "validation_error": map[string]any{"type": item.validationType, "path": "provider_prompt_budget"}, "retryable": false}
		err := testPublicDetailsError{code: item.code, details: details}
		var mapped *browser.CoreError
		if got := browserInitializationAnalysisError(err); !errors.As(got, &mapped) || mapped.Code != item.code || mapped.Status != item.status || mapped.Details["correlation_id"] != details["correlation_id"] || mapped.Details["retryable"] != false || mapped.Message != "" {
			t.Fatalf("initialization error mapping lost safe metadata: %#v", got)
		}
	}
	if browserInitializationAnalysisError(nil) != nil {
		t.Fatal("successful analysis acquired an error")
	}
	unknown := errors.New("private upstream detail")
	if browserInitializationAnalysisError(unknown) != unknown {
		t.Fatal("unknown error must retain route fallback")
	}
}

func TestBrowserAgentRunDiagnosticsErrorKeepsAuthorizationAndFilterCodes(t *testing.T) {
	for _, testCase := range []struct {
		name, code string
		err        error
		status     int
	}{
		{name: "non-owner", err: core.ErrUnauthorized, status: http.StatusForbidden, code: "diagnostics_forbidden"},
		{name: "invalid-filter", err: core.ErrDiagnosticsFilterInvalid, status: http.StatusUnprocessableEntity, code: "diagnostics_filter_invalid"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			var mapped *browser.CoreError
			if err := browserAgentRunDiagnosticsError(testCase.err); !errors.As(err, &mapped) || mapped.Status != testCase.status || mapped.Code != testCase.code {
				t.Fatalf("Agent diagnostic error mapping = %#v", err)
			}
		})
	}
}
