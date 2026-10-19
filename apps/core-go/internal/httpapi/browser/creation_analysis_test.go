package browser

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type creationInputBackend struct {
	fakeBackend
	input    map[string]any
	endpoint string
}

func (b *creationInputBackend) DoJSON(ctx context.Context, method, endpoint, session string, body any) (map[string]any, error) {
	if strings.HasPrefix(endpoint, "/internal/fluctlight-creations/analysis") {
		b.input = body.(map[string]any)
		b.endpoint = endpoint
		return map[string]any{"analysis_id": "actual_source"}, nil
	}
	return b.fakeBackend.DoJSON(ctx, method, endpoint, session, body)
}

func TestCreationTemplateSelectorAndRawInputUseActualPublicTransport(t *testing.T) {
	for _, source := range []struct{ body, field, expected string }{
		{`{"resourceId":"res-real","resourceVersion":"1"}`, "resource_id", "res-real"},
		{`{"json":{"core_persona":{"customKey":"kept"}}}`, "description", `{"core_persona":{"customKey":"kept"}}`},
	} {
		backend := &creationInputBackend{}
		handler := New(Options{Backend: backend, TrustedOrigin: "http://fluctlight.local"}).Handler()
		r := httptest.NewRequest(http.MethodPost, "/api/fluctlight-creations/analysis", strings.NewReader(source.body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", "http://fluctlight.local")
		r.Header.Set("X-CSRF-Token", "csrf")
		r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
		r.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf"})
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, r)
		if response.Code != 200 || backend.input[source.field] != source.expected || response.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatal("creation source changed at boundary", response.Code, backend.input, response.Body.String())
		}
	}
	for _, invalid := range []map[string]any{
		{"resourceId": "r"}, {"resourceVersion": "1"}, {"resourceId": "r", "resourceVersion": "1", "description": "override"}, {"description": "text", "json": map[string]any{"core_persona": map[string]any{}}}, {"json": []any{}},
	} {
		if validCreationAnalysisInput(invalid) {
			t.Fatal("ambiguous or invalid source accepted", invalid)
		}
	}
}

func TestCreationInputErrorsDoNotBecomeUnavailable(t *testing.T) {
	for _, status := range []int{400, 404, 409, 422} {
		response := httptest.NewRecorder()
		(&Server{}).creationError(response, &CoreError{Status: status, Code: "template_input_rejected"}, "analysis")
		if response.Code != status || !strings.Contains(response.Body.String(), "template_input_rejected") {
			t.Fatal("known template error became unavailable", status, response.Code, response.Body.String())
		}
	}
}
