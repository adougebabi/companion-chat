package browser

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type goalBoundaryBackend struct {
	fakeBackend
	body     any
	endpoint string
	failure  error
}

func (b *goalBoundaryBackend) DoJSON(ctx context.Context, method, endpoint, session string, body any) (map[string]any, error) {
	if strings.Contains(endpoint, "/goals") {
		b.body, b.endpoint = body, endpoint
		if b.failure != nil {
			return nil, b.failure
		}
		return map[string]any{"goal_id": "goal-1", "revision": 2, "status": "paused"}, nil
	}
	return b.fakeBackend.DoJSON(ctx, method, endpoint, session, body)
}
func TestBrowserGoalCommandPreservesCASConflictAndRejectsInjection(t *testing.T) {
	b := &goalBoundaryBackend{failure: &CoreError{Status: 409, Code: "conflict"}}
	handler := New(Options{Backend: b, TrustedOrigin: "http://fluctlight.local"}).Handler()
	request := httptest.NewRequest(http.MethodPost, "/api/fluctlights/fl-1/goals/goal-1/pause", strings.NewReader(`{"expectedRevision":1,"reason":"pause","idempotencyKey":"stable-key"}`))
	request.Header.Set("Origin", "http://fluctlight.local")
	request.Header.Set("X-CSRF-Token", "csrf")
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
	request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != 409 || b.endpoint != "/internal/fluctlights/fl-1/goals/goal-1/pause" {
		t.Fatalf("conflict mapped to %d body=%s endpoint=%s", response.Code, response.Body.String(), b.endpoint)
	}
	body := b.body.(map[string]any)
	if body["operation"] != "pause" || body["expected_revision"] != float64(1) || body["idempotency_key"] != "stable-key" {
		t.Fatalf("mutation mapping %#v", body)
	}
	for _, field := range []string{"actorId", "source", "actionPlan", "intentionId", "status", "progress", "operation"} {
		if validateGoalCommand(map[string]any{"reason": "change", "idempotencyKey": "key", field: "forged"}) {
			t.Fatalf("caller forged %s", field)
		}
	}
}

func TestBrowserGoalEvidenceForwardsCursorWithSession(t *testing.T) {
	b := &goalBoundaryBackend{}
	handler := New(Options{Backend: b, TrustedOrigin: "http://fluctlight.local"}).Handler()
	request := httptest.NewRequest(http.MethodGet, "/api/fluctlights/fl-1/goals/goal-1/evidence?limit=7&cursor=opaque%2Bcursor", nil)
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != 200 || b.endpoint != "/internal/fluctlights/fl-1/goals/goal-1/evidence?limit=7&cursor=opaque%2Bcursor" {
		t.Fatalf("evidence route: %d %s", response.Code, b.endpoint)
	}
	request = httptest.NewRequest(http.MethodGet, "/api/fluctlights/fl-1/goals/goal-1/evidence", nil)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != 401 {
		t.Fatalf("evidence route allowed missing session: %d", response.Code)
	}
}
