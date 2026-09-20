package integration_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/httpapi"
)

type mockIntegrationRepo struct{}

func (mockIntegrationRepo) Ping(_ context.Context) error { return nil }
func (mockIntegrationRepo) ResolveSession(_ context.Context, token string) (string, error) {
	if token == "valid-session" {
		return "user-actor-1", nil
	}
	return "", core.ErrUnauthorized
}
func (mockIntegrationRepo) ListFluctlights(_ context.Context, _ string) ([]core.Fluctlight, error) {
	return []core.Fluctlight{{ID: "fl-e2e-1", Status: "active"}}, nil
}
func (mockIntegrationRepo) GetFluctlight(_ context.Context, id, _ string) (core.Fluctlight, error) {
	return core.Fluctlight{ID: id, Status: "active"}, nil
}
func (mockIntegrationRepo) DirectConversationID(_ context.Context, _, _ string) (string, error) {
	return "conv-e2e-1", nil
}
func (mockIntegrationRepo) History(_ context.Context, _, _ string, _ *int, _ int) (core.ConversationPage, error) {
	return core.ConversationPage{}, nil
}

func TestE2EPlatformPingAndFluctlightRouting(t *testing.T) {
	repo := mockIntegrationRepo{}
	serviceKey := "test-service-secret"
	server := httpapi.New(repo, serviceKey, nil)
	handler := server.Handler()

	// 1. Health check / Ping with service key
	req := httptest.NewRequest(http.MethodGet, "/internal/platform/ping", nil)
	req.Header.Set("X-Fluctlight-Service-Key", serviceKey)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected ping status 200, got %d", rec.Code)
	}

	// 2. Fluctlight list with authenticated session and service key
	listReq := httptest.NewRequest(http.MethodGet, "/internal/fluctlights", nil)
	listReq.Header.Set("X-Fluctlight-Service-Key", serviceKey)
	listReq.Header.Set("X-Fluctlight-Human-Session", "valid-session")
	listRec := httptest.NewRecorder()
	handler.ServeHTTP(listRec, listReq)

	if listRec.Code != http.StatusOK {
		t.Fatalf("expected fluctlights list status 200, got %d", listRec.Code)
	}
}
