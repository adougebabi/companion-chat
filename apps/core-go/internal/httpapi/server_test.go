package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
)

type fakeRepository struct{}

type testPublicDetailsError struct {
	code    string
	details map[string]any
}

func (e testPublicDetailsError) Error() string                 { return e.code }
func (e testPublicDetailsError) PublicDetails() map[string]any { return e.details }

func (fakeRepository) Ping(_ context.Context) error { return nil }
func (fakeRepository) ResolveSession(_ context.Context, token string) (string, error) {
	if token == "session" {
		return "human-1", nil
	}
	return "", core.ErrUnauthorized
}

func TestReadJSONRejectsTrailingValues(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"ok":true}{"extra":true}`))
	if _, ok := readJSON(request); ok {
		t.Fatal("expected concatenated JSON to be rejected")
	}
}

func TestWriteErrorDetailsKeepsBoundedOperationReason(t *testing.T) {
	response := httptest.NewRecorder()
	writeErrorDetails(response, http.StatusConflict, "diagnostics_media_retry_failed", map[string]any{"reason": "workflow restart failed"})
	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusConflict)
	}
	if !strings.Contains(response.Body.String(), `"reason":"workflow restart failed"`) {
		t.Fatalf("reason missing from error body: %s", response.Body.String())
	}
}

func TestProviderRoleErrorCodePreservesPreflightReason(t *testing.T) {
	cases := map[string]string{
		"provider_model_not_available":                                   "provider_model_not_available",
		"provider_models_unavailable: provider models returned HTTP 401": "provider_models_unavailable",
		"provider_endpoint_not_found":                                    "provider_endpoint_not_found",
		"provider_role_invalid":                                          "provider_role_invalid",
	}
	for message, want := range cases {
		if got := providerRoleErrorCode(errors.New(message)); got != want {
			t.Fatalf("providerRoleErrorCode(%q) = %q, want %q", message, got, want)
		}
	}
}

func TestActivationFailureDetailsSeparatePersonaConflictAndPersistence(t *testing.T) {
	personaCode, personaDetails := activationFailureDetails(testPublicDetailsError{
		code:    "initialization_persona_invalid",
		details: map[string]any{"validation_error": map[string]any{"type": "reference_invalid", "path": "initial_intentions[0].goal_index"}},
	}, "activation:fl-1")
	if personaCode != "activation_persona_invalid" || stringValue(personaDetails["correlation_id"]) != "activation:fl-1" || stringValue(mapValue(personaDetails["validation_error"])["path"]) != "initial_intentions[0].goal_index" {
		t.Fatalf("persona activation failure=%q %#v", personaCode, personaDetails)
	}
	conflictCode, _ := activationFailureDetails(core.ErrConflict, "activation:fl-1")
	persistenceCode, persistenceDetails := activationFailureDetails(errors.New("database unavailable"), "activation:fl-1")
	if conflictCode != "activation_request_conflict" || persistenceCode != "activation_persistence_failed" || stringValue(persistenceDetails["correlation_id"]) != "activation:fl-1" {
		t.Fatalf("activation failure codes conflict=%q persistence=%q details=%#v", conflictCode, persistenceCode, persistenceDetails)
	}
}

func TestActivationFailureDetailsPreserveWrappedPersonaAndBoundLogCode(t *testing.T) {
	wrapped := fmt.Errorf("activation validation: %w", testPublicDetailsError{
		code:    "initialization_persona_invalid",
		details: map[string]any{"validation_error": map[string]any{"type": "reference_invalid", "path": "initial_intentions[0].goal_index"}},
	})
	code, details := activationFailureDetails(wrapped, "activation:fl-1")
	if code != "activation_persona_invalid" || stringValue(mapValue(details["validation_error"])["path"]) != "initial_intentions[0].goal_index" {
		t.Fatalf("wrapped persona activation failure=%q %#v", code, details)
	}
	if got := activationFailureLogCode(fmt.Errorf("agency: %w", errors.New("initial_goal_profile_invalid"))); got != "initial_goal_profile_invalid" {
		t.Fatalf("wrapped activation log code=%q", got)
	}
	if got := activationFailureLogCode(errors.New("database rejected password=secret")); got != "unclassified" {
		t.Fatalf("unsafe activation log code=%q", got)
	}
}

func (fakeRepository) ListFluctlights(_ context.Context, _ string) ([]core.Fluctlight, error) {
	return []core.Fluctlight{{ID: "fl-1", Status: "active"}}, nil
}
func (fakeRepository) GetFluctlight(_ context.Context, _, _ string) (core.Fluctlight, error) {
	return core.Fluctlight{ID: "fl-1", Status: "active"}, nil
}
func (fakeRepository) DirectConversationID(_ context.Context, _, _ string) (string, error) {
	return "conversation-1", nil
}
func (fakeRepository) History(_ context.Context, _, _ string, _ *int, _ int) (core.ConversationPage, error) {
	return core.ConversationPage{}, nil
}

func TestServerProtectsCoreRoutesAndReturnsSnakeCase(t *testing.T) {
	server := New(fakeRepository{}, "service-key", nil)
	request := httptest.NewRequest(http.MethodGet, "/internal/fluctlights", nil)
	request.Header.Set(serviceKeyHeader, "service-key")
	request.Header.Set(humanSessionHeader, "session")
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}
	request = httptest.NewRequest(http.MethodGet, "/internal/platform/ping", nil)
	response = httptest.NewRecorder()
	server.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("missing service key status = %d, want 401", response.Code)
	}
}
