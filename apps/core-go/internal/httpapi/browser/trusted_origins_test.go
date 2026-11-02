package browser

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTrustedOriginListMutationAndPreflightMatchAnyExactEntry(t *testing.T) {
	const trusted = " http://127.0.0.1:13001 , http://100.109.12.105:13001 "
	for _, origin := range []string{"http://127.0.0.1:13001", "http://100.109.12.105:13001", "http://100.109.12.105:13001.evil", "http://100.109.12.105:13002", "http://evil.test", "null", "", "http://127.0.0.1:13001,http://100.109.12.105:13001"} {
		t.Run(origin, func(t *testing.T) {
			expected := origin == "http://127.0.0.1:13001" || origin == "http://100.109.12.105:13001"
			req := httptest.NewRequest(http.MethodPut, "/api/settings", nil)
			req.Header.Set("Origin", origin)
			req.Header.Set("X-CSRF-Token", "matching")
			req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "matching"})
			recorder := httptest.NewRecorder()
			if allowed := mutationGuard(recorder, req, trusted); allowed != expected {
				t.Fatal("mutation membership", origin, allowed)
			}
			if !expected && (recorder.Code != http.StatusForbidden || !strings.Contains(recorder.Body.String(), "invalid_origin")) {
				t.Fatal(recorder.Code, recorder.Body.String())
			}
			recorder = httptest.NewRecorder()
			corsHeaders(recorder, req, trusted)
			if expected && (recorder.Header().Get("Access-Control-Allow-Origin") != origin || recorder.Header().Get("Access-Control-Allow-Credentials") != "true") {
				t.Fatal("CORS must echo matched origin", recorder.Header())
			}
			if !expected && recorder.Header().Get("Access-Control-Allow-Origin") != "" {
				t.Fatal("untrusted CORS", recorder.Header())
			}
			recorder = httptest.NewRecorder()
			optionsResponse(recorder, req, trusted)
			if expected && (recorder.Code != http.StatusNoContent || recorder.Header().Get("Access-Control-Allow-Origin") != origin) {
				t.Fatal("preflight", recorder.Code, recorder.Header())
			}
			if !expected && recorder.Code != http.StatusForbidden {
				t.Fatal("untrusted preflight", recorder.Code)
			}
			req.Header.Del("X-CSRF-Token")
			if csrfValid(req, trusted) {
				t.Fatal("origin membership bypassed CSRF")
			}
		})
	}
	req := httptest.NewRequest(http.MethodPut, "/api/settings", nil)
	req.Header.Set("Origin", "http://127.0.0.1:13001")
	req.Header.Set("X-CSRF-Token", "same")
	req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "same"})
	if csrfValid(req, "http://127.0.0.1:13001,") {
		t.Fatal("invalid configuration failed open")
	}
}

func TestTrustedOriginsPassActualHTTPMiddlewareAndKeepCookieDefaults(t *testing.T) {
	const trusted = "http://127.0.0.1:13001,http://100.109.12.105:13001"
	for _, origin := range strings.Split(trusted, ",") {
		backend := &fakeBackend{}
		server := New(Options{Backend: backend, TrustedOrigin: trusted})
		if server.secureCookies {
			t.Fatal("HTTP origin list unexpectedly forces secure cookies")
		}
		req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"password":"controlled-test-password"}`))
		req.Header.Set("Origin", origin)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-CSRF-Token", "csrf-test")
		req.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf-test"})
		recorder := httptest.NewRecorder()
		server.Handler().ServeHTTP(recorder, req)
		if recorder.Code != http.StatusOK || recorder.Header().Get("Access-Control-Allow-Origin") != origin {
			t.Fatal(origin, recorder.Code, recorder.Body.String(), recorder.Header())
		}
	}
	if New(Options{Backend: &fakeBackend{}, TrustedOrigin: "https://example.com,http://127.0.0.1:13001"}).secureCookies {
		t.Fatal("mixed list ignores HTTP cookie default")
	}
}
