package browser

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type providerAtomicBackend struct {
	*fakeBackend
	body   map[string]any
	writes int
}

func (b *providerAtomicBackend) DoJSON(ctx context.Context, method, path, session string, raw any) (map[string]any, error) {
	if path == "/internal/providers/endpoints" {
		b.writes++
		b.body = raw.(map[string]any)
		return map[string]any{}, nil
	}
	return b.fakeBackend.DoJSON(ctx, method, path, session, raw)
}
func TestProviderAtomicBrowserMapsOneScopedCredentialCommand(t *testing.T) {
	for _, clear := range []bool{false, true} {
		backend := &providerAtomicBackend{fakeBackend: &fakeBackend{}}
		body := map[string]any{"endpointId": "service", "kind": "openai-compatible", "baseUrl": "https://controlled.invalid/v1", "secretPurpose": "provider:service", "expectedVersions": map[string]any{"endpoint:service": 1, "secret:provider:service": 2}, "idempotencyKey": "one-command"}
		if clear {
			body["clearSecrets"] = []string{"provider:service"}
		} else {
			body["secrets"] = map[string]any{"provider:service": strings.Repeat("synthetic", 2)}
		}
		encoded, _ := json.Marshal(body)
		r := httptest.NewRequest(http.MethodPut, "https://api.test/api/providers/endpoints", strings.NewReader(string(encoded)))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", "https://fluctlight.test")
		r.Header.Set("X-CSRF-Token", "csrf")
		r.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf"})
		r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "opaque"})
		w := httptest.NewRecorder()
		newBrowserTestHandler(backend).ServeHTTP(w, r)
		if w.Code != 204 || backend.writes != 1 || backend.body["endpoint_id"] != "service" {
			t.Fatalf("status=%d writes=%d", w.Code, backend.writes)
		}
		if clear {
			if backend.body["clear_secrets"] == nil || backend.body["clearSecrets"] != nil {
				t.Fatal("clear DTO not explicitly mapped")
			}
		} else if backend.body["secrets"] == nil {
			t.Fatal("credential discarded")
		}
		if strings.Contains(w.Body.String(), "synthetic") {
			t.Fatal("credential returned")
		}
	}
}
func TestProviderAtomicBrowserRejectsForeignAndMalformedCredentials(t *testing.T) {
	for _, mutation := range []map[string]any{{"secrets": map[string]any{"other": "x"}}, {"secrets": map[string]any{"provider:service": true}}, {"secrets": map[string]any{"provider:service": strings.Repeat("x", 65537)}}, {"clearSecrets": []any{"other"}}, {"clearSecrets": []any{"provider:service"}, "secrets": map[string]any{"provider:service": "x"}}} {
		body := map[string]any{"endpointId": "service", "kind": "openai-compatible", "baseUrl": "https://controlled.invalid/v1", "secretPurpose": "provider:service", "expectedVersions": map[string]any{}, "idempotencyKey": "invalid"}
		for k, v := range mutation {
			body[k] = v
		}
		if validateProviderEndpoint(body) {
			t.Fatal("accepted invalid credential mutation")
		}
	}
}
