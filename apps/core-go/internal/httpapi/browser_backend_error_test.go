package httpapi

import (
	"errors"
	"strings"
	"testing"

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
