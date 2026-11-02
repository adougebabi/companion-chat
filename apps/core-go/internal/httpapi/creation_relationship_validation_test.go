package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
)

func TestCreationAnalysisRelationshipSeedsReturnsFieldValidation(t *testing.T) {
	server := New(fakeRepository{}, "service-key", nil)
	// Invalid analysis must fail before a repository or model is required.
	server.app = &core.App{}
	server.SetBrowserBoundary("http://fluctlight.local", false)
	handler := server.Handler()
	for _, path := range []string{"/api/fluctlight-creations/analysis", "/internal/fluctlight-creations/analysis"} {
		t.Run(path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"json":{"core_persona":{"identity":{"name":"书店店主"},"life_profile":{"relationship_seeds":["独居","养一只灰猫"]}},"initial_relationships":[]}}`))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Origin", "http://fluctlight.local")
			request.Header.Set("X-CSRF-Token", "csrf")
			request.Header.Set(serviceKeyHeader, "service-key")
			request.Header.Set(humanSessionHeader, "session")
			request.AddCookie(&http.Cookie{Name: "fluctlight_session", Value: "session"})
			request.AddCookie(&http.Cookie{Name: "fluctlight_csrf", Value: "csrf"})
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != http.StatusUnprocessableEntity {
				t.Fatalf("malformed seeds did not produce validation: %d %s", response.Code, response.Body.String())
			}
			var body map[string]any
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			failure := body
			if strings.HasPrefix(path, "/internal/") {
				failure = mapValue(body["detail"])
			}
			details := mapValue(failure["details"])
			validation := mapValue(details["validation_error"])
			if failure["code"] != "initialization_persona_invalid" || validation["path"] != "core_persona.life_profile.relationship_seeds[0]" || validation["type"] != "type_invalid" || !strings.HasPrefix(stringValue(details["correlation_id"]), "initialization-analysis:") || details["retryable"] != false {
				t.Fatalf("validation code/path/correlation lost: %#v", body)
			}
		})
	}
}
