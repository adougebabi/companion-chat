package browser

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type backgroundConflictBackend struct{ fakeBackend }

func (b *backgroundConflictBackend) DoJSON(ctx context.Context, method, endpoint, session string, body any) (map[string]any, error) {
	if strings.HasSuffix(endpoint, "/actor-user-background") {
		return nil, &CoreError{Status: 409, Code: "actor_user_background_conflict", Details: map[string]any{"resource": "actor_user_background"}}
	}
	return b.fakeBackend.DoJSON(ctx, method, endpoint, session, body)
}
func TestOwnerBackgroundConflictRemains409AtPublicBoundary(t *testing.T) {
	handler := New(Options{Backend: &backgroundConflictBackend{}, TrustedOrigin: "http://fluctlight.local"}).Handler()
	request := httptest.NewRequest(http.MethodPut, "/api/fluctlights/fl-real/actor-user-background", strings.NewReader(`{"background":{"name":"测试参与者"},"operation":"correct","reason":"修正资料","idempotencyKey":"background-conflict","expectedCurrentFactsRevision":"facts_gen_1"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "http://fluctlight.local")
	request.Header.Set("X-CSRF-Token", "csrf")
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
	request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), "actor_user_background_conflict") {
		t.Fatalf("lost CAS conflict: %d %s", response.Code, response.Body.String())
	}
}

func TestCompleteAboutYouBoundaryAcceptsFieldsAndPreservesReasonContracts(t *testing.T) {
	fields := []string{"name", "full_name", "age", "gender", "occupation", "background", "interests", "preferences", "location_scope", "location", "timezone", "relationship_distance", "meeting_confirmed", "personality", "habits", "relationship_description", "shared_information", "wants_you_to_know", "rules"}
	for _, key := range fields {
		values := []any{nil, "中文值"}
		if key == "meeting_confirmed" {
			values = []any{nil, false}
		}
		for _, value := range values {
			if !validateActorUserInput(map[string]any{"background": map[string]any{key: value}}) {
				t.Fatalf("field rejected %s=%v", key, value)
			}
		}
		if validateActorUserInput(map[string]any{"background": map[string]any{key: []any{"invalid"}}}) {
			t.Fatalf("array accepted %s", key)
		}
	}
	actor := map[string]any{"background": map[string]any{"rules": "先询问\n再建议"}, "expectedContextVersion": "version", "idempotencyKey": "key", "operation": "correct", "reason": strings.Repeat("明", 333)}
	if !validateActorContextUpdate(actor) {
		t.Fatal("999 UTF-8 byte Actor reason rejected")
	}
	actor["reason"] = strings.Repeat("明", 334)
	if validateActorContextUpdate(actor) {
		t.Fatal("1002 UTF-8 byte Actor reason accepted")
	}
	owner := map[string]any{"background": actor["background"], "expectedCurrentFactsRevision": "version", "idempotencyKey": "key", "operation": "correct", "reason": strings.Repeat("明", 500)}
	if !validateActorUserBackgroundUpdate(owner) {
		t.Fatal("500 codepoint Owner reason rejected")
	}
	owner["reason"] = strings.Repeat("明", 501)
	if validateActorUserBackgroundUpdate(owner) {
		t.Fatal("501 codepoint Owner reason accepted")
	}
}
