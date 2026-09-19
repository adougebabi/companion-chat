package browser

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeBackend struct {
	stream func(http.ResponseWriter)
	calls  []string
}

func (f *fakeBackend) Health(context.Context) error { return nil }

func (f *fakeBackend) DoJSON(_ context.Context, method, endpoint, _ string, _ any) (map[string]any, error) {
	f.calls = append(f.calls, method+" "+endpoint)
	switch endpoint {
	case "/internal/auth/session":
		return map[string]any{"authenticated": true, "actor_id": "owner-1"}, nil
	case "/internal/auth/setup-status":
		return map[string]any{"setup_available": false}, nil
	case "/internal/auth/login", "/internal/auth/setup":
		return map[string]any{"authenticated": true, "actor_id": "owner-1", "session_token": "session"}, nil
	case "/internal/settings":
		return map[string]any{"values": map[string]any{}, "configured_secrets": []any{}}, nil
	default:
		return map[string]any{}, nil
	}
}

func (f *fakeBackend) DoAny(_ context.Context, method, endpoint, session string, body any) (any, error) {
	if strings.Contains(endpoint, "/diagnostics/lifecycle") {
		return map[string]any{"events": []any{}, "workflow_intents": []any{}, "filters": map[string]any{}}, nil
	}
	if strings.Contains(endpoint, "/diagnostics/export") {
		return map[string]any{}, nil
	}
	if strings.Contains(endpoint, "/diagnostics") || strings.Contains(endpoint, "/providers") || strings.Contains(endpoint, "/actor-groups") || strings.Contains(endpoint, "/fluctlights") || strings.Contains(endpoint, "/moments") || strings.Contains(endpoint, "/capability-requests") {
		return []any{}, nil
	}
	value, err := f.DoJSON(context.Background(), method, endpoint, session, body)
	return value, err
}

func (f *fakeBackend) DoValue(ctx context.Context, method, endpoint, session string, body, out any) error {
	value, err := f.DoAny(ctx, method, endpoint, session, body)
	if err != nil {
		return err
	}
	encoded, _ := json.Marshal(value)
	return json.Unmarshal(encoded, out)
}

func (f *fakeBackend) StreamTurn(_ context.Context, _ string, _ string, _ map[string]any, writer http.ResponseWriter) error {
	if f.stream != nil {
		f.stream(writer)
		return nil
	}
	_, _ = io.WriteString(writer, `{"type":"completed","turn_id":"turn-1","sequence":0,"payload":{}}`+"\n")
	return nil
}

func (f *fakeBackend) Media(_ context.Context, _ string, _ string, _ string, writer http.ResponseWriter) error {
	writer.Header().Set("Content-Type", "image/png")
	_, _ = writer.Write([]byte("png"))
	return nil
}

func newBrowserTestHandler(backend Backend) http.Handler {
	return New(Options{Backend: backend, TrustedOrigin: "https://fluctlight.test", SecureCookies: true}).Handler()
}

func TestBrowserHandlerKeepsAuthAndCSRFAtPublicBoundary(t *testing.T) {
	backend := &fakeBackend{}
	handler := newBrowserTestHandler(backend)

	sessionRequest := httptest.NewRequest(http.MethodGet, "https://api.test/auth/session", nil)
	sessionRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "opaque"})
	sessionResponse := httptest.NewRecorder()
	handler.ServeHTTP(sessionResponse, sessionRequest)
	if sessionResponse.Code != http.StatusOK || !strings.Contains(sessionResponse.Body.String(), `"authenticated":true`) {
		t.Fatalf("session status/body = %d/%s", sessionResponse.Code, sessionResponse.Body.String())
	}

	request := httptest.NewRequest(http.MethodPut, "https://api.test/api/settings", strings.NewReader(`{"values":{}}`))
	request.Header.Set("Origin", "https://fluctlight.test")
	request.Header.Set("X-CSRF-Token", "csrf")
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "opaque"})
	request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("settings status = %d, body = %s", response.Code, response.Body.String())
	}

	missingCSRF := httptest.NewRequest(http.MethodPut, "https://api.test/api/settings", strings.NewReader(`{"values":{}}`))
	missingCSRF.Header.Set("Origin", "https://fluctlight.test")
	missingCSRF.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "opaque"})
	missingResponse := httptest.NewRecorder()
	handler.ServeHTTP(missingResponse, missingCSRF)
	if missingResponse.Code != http.StatusForbidden {
		t.Fatalf("missing CSRF status = %d", missingResponse.Code)
	}
}

func TestBrowserHandlerTranslatesCoreNDJSONWithoutHTTPHop(t *testing.T) {
	backend := &fakeBackend{stream: func(writer http.ResponseWriter) {
		_, _ = io.WriteString(writer, `{"type":"token","turn_id":"turn-1","sequence":0,"payload":{"text":"你"}}`+"\n")
		_, _ = io.WriteString(writer, `{"type":"completed","turn_id":"turn-1","sequence":1,"payload":{}}`+"\n")
	}}
	handler := newBrowserTestHandler(backend)
	request := httptest.NewRequest(http.MethodPost, "https://api.test/api/conversations/conversation-1/turn", strings.NewReader(`{"text":"hi","fluctlightId":"fl-1","idempotencyKey":"idem-1"}`))
	request.Header.Set("Origin", "https://fluctlight.test")
	request.Header.Set("X-CSRF-Token", "csrf")
	request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "opaque"})
	request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("turn status = %d, body = %s", response.Code, response.Body.String())
	}
	if !strings.Contains(response.Body.String(), `"type":"token"`) || !strings.Contains(response.Body.String(), `"type":"completed"`) {
		t.Fatalf("translated NDJSON = %s", response.Body.String())
	}
	if strings.Contains(response.Body.String(), "turn_id") {
		t.Fatalf("Core snake_case envelope leaked: %s", response.Body.String())
	}
}

func TestBrowserHandlerRejectsUnknownAPIRouteAndInvalidOrigin(t *testing.T) {
	handler := newBrowserTestHandler(&fakeBackend{})
	unknown := httptest.NewRequest(http.MethodGet, "https://api.test/api/not-real", nil)
	unknownResponse := httptest.NewRecorder()
	handler.ServeHTTP(unknownResponse, unknown)
	if unknownResponse.Code != http.StatusNotFound {
		t.Fatalf("unknown route status = %d", unknownResponse.Code)
	}

	options := httptest.NewRequest(http.MethodOptions, "https://api.test/api/settings", nil)
	options.Header.Set("Origin", "https://evil.test")
	optionsResponse := httptest.NewRecorder()
	handler.ServeHTTP(optionsResponse, options)
	if optionsResponse.Code != http.StatusForbidden {
		t.Fatalf("invalid origin preflight status = %d", optionsResponse.Code)
	}
}
