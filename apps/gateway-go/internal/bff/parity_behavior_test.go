package bff

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestBFFMapsNestedBrowserPayloadsToCore(t *testing.T) {
	seen := map[string]struct {
		path string
		body map[string]any
	}{}
	handler := testBFF(t, func(request *http.Request) (*http.Response, error) {
		body := map[string]any{}
		if request.Body != nil {
			data, err := io.ReadAll(request.Body)
			if err != nil {
				t.Fatalf("read Core body: %v", err)
			}
			if len(data) > 0 {
				if err := json.Unmarshal(data, &body); err != nil {
					t.Fatalf("decode Core body: %v", err)
				}
			}
		}
		seen[request.URL.Path] = struct {
			path string
			body map[string]any
		}{path: request.URL.RequestURI(), body: body}

		switch {
		case strings.HasSuffix(request.URL.Path, "/turn"):
			return jsonResponseWithContentType(http.StatusOK, "{\"type\":\"completed\",\"turn_id\":\"turn\",\"sequence\":0,\"payload\":{}}\n", "application/x-ndjson"), nil
		case strings.HasSuffix(request.URL.Path, "/conversation"), strings.HasSuffix(request.URL.Path, "/history"):
			return jsonResponse(http.StatusOK, conversationPageJSON), nil
		default:
			return jsonResponse(http.StatusOK, `{}`), nil
		}
	})

	mutationHeaders := map[string]string{"Origin": "https://fluctlight.local", "X-CSRF-Token": "csrf"}
	mutationCookies := map[string]string{sessionCookieName: "opaque", csrfCookieName: "csrf"}

	settings := invoke(handler, http.MethodPut, "http://gateway.test/api/settings", `{"values":{"theme":"dark"},"secrets":{"provider:key":"secret"},"clearSecrets":["old:key"]}`, mutationHeaders, mutationCookies)
	if settings.Code != http.StatusOK {
		t.Fatalf("settings status = %d: %s", settings.Code, settings.Body.String())
	}
	assertCoreBody(t, seen, "/internal/settings", map[string]any{
		"values":        map[string]any{"theme": "dark"},
		"secrets":       map[string]any{"provider:key": "secret"},
		"clear_secrets": []any{"old:key"},
	})

	schedule := invoke(handler, http.MethodPost, "http://gateway.test/api/fluctlights/fl-1/schedules", `{"localDate":"2026-01-01","timezone":"UTC","items":[{"startAt":"2026-01-01T10:00:00Z","endAt":"2026-01-01T11:00:00Z","activity":"work","scene":"office","itemType":"focus","interruptionCost":0.25}],"evidenceRefs":["event"],"expectedRevision":3,"completedBefore":"2025-12-31T00:00:00Z"}`, mutationHeaders, mutationCookies)
	if schedule.Code != http.StatusOK {
		t.Fatalf("schedule status = %d: %s", schedule.Code, schedule.Body.String())
	}
	assertCoreBody(t, seen, "/internal/fluctlights/fl-1/schedules", map[string]any{
		"local_date":        "2026-01-01",
		"timezone":          "UTC",
		"evidence_refs":     []any{"event"},
		"expected_revision": float64(3),
		"completed_before":  "2025-12-31T00:00:00Z",
		"items": []any{map[string]any{
			"start_at":          "2026-01-01T10:00:00Z",
			"end_at":            "2026-01-01T11:00:00Z",
			"activity":          "work",
			"scene":             "office",
			"item_type":         "focus",
			"interruption_cost": float64(0.25),
		}},
	})

	activation := invoke(handler, http.MethodPost, "http://gateway.test/api/fluctlight-creations/activate", `{"requestId":"request","initializationMode":"llm_defined","identity":{},"personality":{},"behavioralPolicy":{},"lifeProfile":{},"foundationProvenance":{},"initialGoals":[{"name":"goal"}],"initialIntentions":[{"name":"intent"}]}`, mutationHeaders, mutationCookies)
	if activation.Code != http.StatusOK {
		t.Fatalf("activation status = %d: %s", activation.Code, activation.Body.String())
	}
	assertCoreBody(t, seen, "/internal/fluctlight-creations/activate", map[string]any{
		"request_id":            "request",
		"initialization_mode":   "llm_defined",
		"identity":              map[string]any{},
		"personality":           map[string]any{},
		"behavioral_policy":     map[string]any{},
		"life_profile":          map[string]any{},
		"foundation_provenance": map[string]any{},
		"initial_goals":         []any{map[string]any{"name": "goal"}},
		"initial_intentions":    []any{map[string]any{"name": "intent"}},
	})

	history := invoke(handler, http.MethodGet, "http://gateway.test/api/conversations/conversation-1/messages?beforeSequence=5&limit=10", "", nil, map[string]string{sessionCookieName: "opaque"})
	if history.Code != http.StatusOK {
		t.Fatalf("history status = %d: %s", history.Code, history.Body.String())
	}
	requestURI := seen["/internal/conversations/conversation-1/history"].path
	parsed, err := url.Parse(requestURI)
	if err != nil {
		t.Fatalf("parse history request URI: %v", err)
	}
	if parsed.Query().Get("limit") != "10" || parsed.Query().Get("before_sequence") != "5" {
		t.Fatalf("history request URI = %q", requestURI)
	}
}

func TestBFFErrorMappingsPreserveRouteSpecificPolicy(t *testing.T) {
	mutationHeaders := map[string]string{"Origin": "https://fluctlight.local", "X-CSRF-Token": "csrf"}
	mutationCookies := map[string]string{sessionCookieName: "opaque", csrfCookieName: "csrf"}

	analysis := testBFF(t, func(request *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusUnprocessableEntity, `{"detail":{"code":"foundation_invalid","message":"Foundation invalid","details":{"field":"name"}}}`), nil
	})
	analysisResponse := invoke(analysis, http.MethodPost, "http://gateway.test/api/fluctlight-creations/analysis", `{"description":"make one"}`, mutationHeaders, mutationCookies)
	if analysisResponse.Code != http.StatusUnprocessableEntity || !strings.Contains(analysisResponse.Body.String(), `"foundation_invalid"`) || !strings.Contains(analysisResponse.Body.String(), `"field":"name"`) {
		t.Fatalf("analysis error = %d %s", analysisResponse.Code, analysisResponse.Body.String())
	}

	activation := testBFF(t, func(request *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusInternalServerError, `{"detail":{"code":"provider_down","message":"provider down","details":{"secret":"must-not-leak"}}}`), nil
	})
	activationResponse := invoke(activation, http.MethodPost, "http://gateway.test/api/fluctlight-creations/activate", `{"requestId":"request","initializationMode":"blank_slate","identity":{}}`, mutationHeaders, mutationCookies)
	if activationResponse.Code != http.StatusServiceUnavailable || strings.Contains(activationResponse.Body.String(), "must-not-leak") {
		t.Fatalf("activation error = %d %s", activationResponse.Code, activationResponse.Body.String())
	}

	media := testBFF(t, func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusRequestedRangeNotSatisfiable, Body: io.NopCloser(strings.NewReader("{}")), Header: http.Header{"Content-Type": []string{"application/json"}}}, nil
	})
	mediaResponse := invoke(media, http.MethodGet, "http://gateway.test/api/media/asset-1", "", nil, map[string]string{sessionCookieName: "opaque"})
	if mediaResponse.Code != http.StatusNotFound || !strings.Contains(mediaResponse.Body.String(), `"media_unavailable"`) {
		t.Fatalf("media error = %d %s", mediaResponse.Code, mediaResponse.Body.String())
	}
}

func TestBFFProviderRoleErrorPreservesCoreReason(t *testing.T) {
	handler := testBFF(t, func(request *http.Request) (*http.Response, error) {
		if request.URL.Path == "/internal/providers/roles" {
			return jsonResponse(http.StatusUnprocessableEntity, `{"code":"provider_model_not_available","message":"selected model is not listed"}`), nil
		}
		return jsonResponse(http.StatusOK, `{}`), nil
	})
	response := invoke(handler, http.MethodPut, "http://gateway.test/api/providers/roles", `{"role":"reflection","endpointId":"endpoint","modelId":"model","tokenBudget":100,"timeoutSeconds":10}`, map[string]string{
		"Origin":       "https://fluctlight.local",
		"X-CSRF-Token": "csrf",
	}, map[string]string{sessionCookieName: "opaque", csrfCookieName: "csrf"})
	if response.Code != http.StatusUnprocessableEntity || !strings.Contains(response.Body.String(), `"provider_model_not_available"`) {
		t.Fatalf("provider role error = %d %s", response.Code, response.Body.String())
	}
}

func TestBFFCookiesAndOriginMatrix(t *testing.T) {
	handler := testBFF(t, func(request *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, `{}`), nil
	})

	preflight := invoke(handler, http.MethodOptions, "http://gateway.test/api/settings", "", map[string]string{"Origin": "https://fluctlight.local"}, nil)
	if preflight.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d", preflight.Code)
	}
	if preflight.Header().Get("Access-Control-Allow-Methods") != "GET,POST,PUT,DELETE,OPTIONS" || preflight.Header().Get("Access-Control-Allow-Headers") != "content-type,range,x-csrf-token" {
		t.Fatalf("preflight headers = %#v", preflight.Header())
	}
	if !strings.Contains(preflight.Header().Get("Set-Cookie"), "fluctlight_csrf=") {
		t.Fatalf("preflight did not issue CSRF cookie: %q", preflight.Header().Get("Set-Cookie"))
	}

	unknown := invoke(handler, http.MethodGet, "http://gateway.test/not-a-route", "", nil, nil)
	if unknown.Code != http.StatusNotFound || !strings.Contains(unknown.Header().Get("Set-Cookie"), "fluctlight_csrf=") {
		t.Fatalf("unknown response = %d %q", unknown.Code, unknown.Header().Get("Set-Cookie"))
	}

	logout := invoke(handler, http.MethodPost, "http://gateway.test/auth/logout", "", map[string]string{"Origin": "https://fluctlight.local", "X-CSRF-Token": "csrf"}, map[string]string{csrfCookieName: "csrf"})
	if logout.Code != http.StatusNoContent || !strings.Contains(logout.Header().Get("Set-Cookie"), "fluctlight_session=;") {
		t.Fatalf("logout without session = %d %q", logout.Code, logout.Header().Get("Set-Cookie"))
	}
}

func TestBFFTurnMapsMissingCoreBodyToBadGateway(t *testing.T) {
	handler := testBFF(t, func(request *http.Request) (*http.Response, error) {
		if strings.HasSuffix(request.URL.Path, "/turn") {
			return &http.Response{StatusCode: http.StatusOK, Body: nil}, nil
		}
		return jsonResponse(http.StatusOK, `{}`), nil
	})
	response := invoke(handler, http.MethodPost, "http://gateway.test/api/conversations/conversation-1/turn", `{"text":"hello","fluctlightId":"fl-1","idempotencyKey":"turn-1"}`, map[string]string{"Origin": "https://fluctlight.local", "X-CSRF-Token": "csrf"}, map[string]string{sessionCookieName: "opaque", csrfCookieName: "csrf"})
	if response.Code != http.StatusBadGateway || !strings.Contains(response.Body.String(), `"conversation_turn_failed"`) {
		t.Fatalf("missing Core stream body = %d %s", response.Code, response.Body.String())
	}
}

func TestBFFMediaUsesDefaultContentType(t *testing.T) {
	handler := testBFF(t, func(request *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, ContentLength: 5, Body: io.NopCloser(strings.NewReader("bytes")), Header: http.Header{}}, nil
	})
	response := invoke(handler, http.MethodGet, "http://gateway.test/api/media/asset-1", "", nil, map[string]string{sessionCookieName: "opaque"})
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "application/octet-stream" {
		t.Fatalf("media default content type = %d %q", response.Code, response.Header().Get("Content-Type"))
	}
}

func TestBFFValidatesSchemaBeforeSecurityGuards(t *testing.T) {
	coreCalls := 0
	handler := testBFF(t, func(request *http.Request) (*http.Response, error) {
		coreCalls++
		return jsonResponse(http.StatusOK, `{}`), nil
	})

	invalidPassword := invoke(handler, http.MethodPost, "http://gateway.test/auth/password", `{"password":"short"}`, nil, nil)
	if invalidPassword.Code != http.StatusBadRequest {
		t.Fatalf("invalid password without origin = %d %s, want 400", invalidPassword.Code, invalidPassword.Body.String())
	}

	tooLongEndpoint := strings.Repeat("e", 129)
	invalidPath := invoke(handler, http.MethodGet, "http://gateway.test/api/providers/endpoints/"+tooLongEndpoint+"/models", "", nil, nil)
	if invalidPath.Code != http.StatusBadRequest {
		t.Fatalf("invalid endpoint without session = %d %s, want 400", invalidPath.Code, invalidPath.Body.String())
	}
	if coreCalls != 0 {
		t.Fatalf("Core calls = %d, want 0 for invalid requests", coreCalls)
	}
}

func assertCoreBody(t *testing.T, seen map[string]struct {
	path string
	body map[string]any
}, path string, want map[string]any) {
	t.Helper()
	entry, ok := seen[path]
	if !ok {
		t.Fatalf("Core request %q not observed; got %v", path, seen)
	}
	got, err := json.Marshal(entry.body)
	if err != nil {
		t.Fatalf("marshal got body: %v", err)
	}
	wantBytes, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("marshal want body: %v", err)
	}
	if string(got) != string(wantBytes) {
		t.Fatalf("Core body for %s = %s, want %s", path, got, wantBytes)
	}
}
