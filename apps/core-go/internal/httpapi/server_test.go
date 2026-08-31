package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
)

type fakeRepository struct{}

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
