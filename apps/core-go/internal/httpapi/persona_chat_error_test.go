package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/httpapi/browser"
)

func TestPersonaChatErrorsKeepReadableReasonWithoutCompilerDetails(t *testing.T) {
	for _, code := range []string{"persona_preparation_failed", "persona_preparation_pending"} {
		err := &core.CapabilityError{Code: code, Cause: errors.New("compiler password=private-provider-value")}
		mapped := browserConversationTurnError(err)
		var public *browser.CoreError
		if !errors.As(mapped, &public) || public.Code != code || public.Status != http.StatusConflict || public.Message != core.PersonaPreparationMessage(code) {
			t.Fatalf("browser error=%#v", mapped)
		}
		response := httptest.NewRecorder()
		writeNDJSONError(response, "persona-turn", err)
		var frame struct {
			Payload map[string]string `json:"payload"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &frame); err != nil || frame.Payload["code"] != code || frame.Payload["message"] != core.PersonaPreparationMessage(code) {
			t.Fatalf("NDJSON body=%s err=%v", response.Body.String(), err)
		}
		if strings.Contains(response.Body.String(), "password") || strings.Contains(public.Message, "compiler") {
			t.Fatal("compiler details leaked")
		}
	}
}
