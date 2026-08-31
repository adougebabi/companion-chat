package bff

import (
	"net/http"
	"strings"
	"testing"
)

func TestProtectedRoutesRejectMissingSessionOrCSRFBeforeCore(t *testing.T) {
	for _, route := range browserRouteCases() {
		route := route
		if route.method == http.MethodOptions || isPublicRoute(route.path) {
			continue
		}
		t.Run(route.name, func(t *testing.T) {
			coreCalls := 0
			handler := testBFF(t, func(request *http.Request) (*http.Response, error) {
				coreCalls++
				return jsonResponse(http.StatusOK, `{}`), nil
			})

			headers := map[string]string{}
			cookies := map[string]string{}
			if route.method == http.MethodGet {
				// A protected read must reject the missing opaque session before
				// making an internal request.
				response := invoke(handler, route.method, "http://gateway.test"+route.path, route.body, headers, cookies)
				if response.Code != http.StatusUnauthorized {
					t.Fatalf("missing session status = %d, body = %s", response.Code, response.Body.String())
				}
			} else {
				// A mutation with a session but no double-submit token must be
				// rejected by the Origin/CSRF guard before Core sees it.
				headers["Origin"] = "https://fluctlight.local"
				cookies[sessionCookieName] = "opaque"
				response := invoke(handler, route.method, "http://gateway.test"+route.path, route.body, headers, cookies)
				if response.Code != http.StatusForbidden || !strings.Contains(response.Body.String(), `"invalid_origin"`) {
					t.Fatalf("missing CSRF status = %d, body = %s", response.Code, response.Body.String())
				}
			}
			if coreCalls != 0 {
				t.Fatalf("Core calls = %d, want 0", coreCalls)
			}
		})
	}
}

func isPublicRoute(path string) bool {
	switch path {
	case "/health/live", "/health/ready", "/api/platform/ping", "/auth/session", "/auth/setup-status":
		return true
	default:
		return false
	}
}
