package bff

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) { return fn(request) }

func testBFF(t *testing.T, fn roundTripFunc) http.Handler {
	t.Helper()
	base, err := url.Parse("http://core.invalid")
	if err != nil {
		t.Fatal(err)
	}
	return New(Options{
		CoreBaseURL:    base,
		CoreServiceKey: "service-key",
		TrustedOrigin:  "https://fluctlight.local",
		SecureCookies:  true,
		Client:         &http.Client{Transport: fn},
	}).Handler()
}

func invoke(handler http.Handler, method, target, body string, headers map[string]string, cookies map[string]string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	for name, value := range cookies {
		request.AddCookie(&http.Cookie{Name: name, Value: value})
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func TestBFFSecurityAndArrayQueries(t *testing.T) {
	var seen *http.Request
	handler := testBFF(t, func(request *http.Request) (*http.Response, error) {
		seen = request
		if request.URL.Path == "/internal/fluctlights" {
			return jsonResponse(http.StatusOK, `[{"id":"fl-1","status":"active"}]`), nil
		}
		return jsonResponse(http.StatusOK, `{}`), nil
	})

	preflight := invoke(handler, http.MethodOptions, "http://gateway.test/api/fluctlights", "", map[string]string{
		"Origin": "https://fluctlight.local",
	}, nil)
	if preflight.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want 204", preflight.Code)
	}
	if preflight.Header().Get("Access-Control-Allow-Origin") != "https://fluctlight.local" {
		t.Fatalf("allow origin = %q", preflight.Header().Get("Access-Control-Allow-Origin"))
	}
	if !strings.Contains(preflight.Header().Get("Set-Cookie"), "fluctlight_csrf=") {
		t.Fatalf("preflight did not issue CSRF cookie: %q", preflight.Header().Get("Set-Cookie"))
	}

	read := invoke(handler, http.MethodGet, "http://gateway.test/api/fluctlights", "", nil, map[string]string{sessionCookieName: "opaque"})
	if read.Code != http.StatusOK || !strings.Contains(read.Body.String(), `"id":"fl-1"`) {
		t.Fatalf("array response = %d %s", read.Code, read.Body.String())
	}
	if seen == nil || seen.Header.Get(serviceKeyHeader) != "service-key" || seen.Header.Get(humanSessionHeader) != "opaque" {
		t.Fatalf("Core identity headers = %#v", seen.Header)
	}

	blocked := invoke(handler, http.MethodPut, "http://gateway.test/api/settings", `{}`, nil, map[string]string{sessionCookieName: "opaque"})
	if blocked.Code != http.StatusForbidden || !strings.Contains(blocked.Body.String(), `"invalid_origin"`) {
		t.Fatalf("blocked mutation = %d %s", blocked.Code, blocked.Body.String())
	}
}

func TestBFFMapsSettingsAndConversationTurn(t *testing.T) {
	var coreBody map[string]any
	handler := testBFF(t, func(request *http.Request) (*http.Response, error) {
		if request.URL.Path == "/internal/settings" {
			data, _ := io.ReadAll(request.Body)
			_ = json.Unmarshal(data, &coreBody)
			return jsonResponse(http.StatusOK, `{"values":{"theme":"dark"},"configured_secrets":["provider:key"]}`), nil
		}
		if strings.HasSuffix(request.URL.Path, "/turn") {
			return jsonResponseWithContentType(http.StatusOK, "{\"type\":\"token\",\"turn_id\":\"turn-1\",\"sequence\":0,\"payload\":{\"text\":\"hi\"}}\n{\"type\":\"completed\",\"turn_id\":\"turn-1\",\"sequence\":1,\"payload\":{}}\n", "application/x-ndjson"), nil
		}
		return jsonResponse(http.StatusOK, `{}`), nil
	})
	settings := invoke(handler, http.MethodPut, "http://gateway.test/api/settings", `{"values":{"theme":"dark"},"secrets":{"provider:key":"private"},"clearSecrets":["old:key"]}`, map[string]string{
		"Origin":       "https://fluctlight.local",
		"X-CSRF-Token": "csrf",
	}, map[string]string{sessionCookieName: "opaque", csrfCookieName: "csrf"})
	if settings.Code != http.StatusOK || strings.Contains(settings.Body.String(), "private") || !strings.Contains(settings.Body.String(), "configuredSecrets") {
		t.Fatalf("settings response = %d %s", settings.Code, settings.Body.String())
	}
	if coreBody["clear_secrets"] == nil {
		t.Fatalf("snake_case settings body = %#v", coreBody)
	}

	turn := invoke(handler, http.MethodPost, "http://gateway.test/api/conversations/c-1/turn", `{"text":"hello","fluctlightId":"fl-1","idempotencyKey":"id-1"}`, map[string]string{
		"Origin":       "https://fluctlight.local",
		"X-CSRF-Token": "csrf",
	}, map[string]string{sessionCookieName: "opaque", csrfCookieName: "csrf"})
	if turn.Code != http.StatusOK || !strings.Contains(turn.Header().Get("Content-Type"), "application/x-ndjson") || !strings.Contains(turn.Body.String(), `"turnId":"turn-1"`) {
		t.Fatalf("turn response = %d %s", turn.Code, turn.Body.String())
	}
}

func jsonResponse(status int, body string) *http.Response {
	return jsonResponseWithContentType(status, body, "application/json")
}

func jsonResponseWithContentType(status int, body, contentType string) *http.Response {
	return &http.Response{StatusCode: status, ContentLength: int64(len(body)), Header: http.Header{"Content-Type": []string{contentType}}, Body: io.NopCloser(strings.NewReader(body))}
}
