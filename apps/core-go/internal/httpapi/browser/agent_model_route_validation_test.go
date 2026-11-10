package browser

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type samplingBoundaryBackend struct {
	fakeBackend
	input map[string]any
}

func (b *samplingBoundaryBackend) DoJSON(ctx context.Context, method, endpoint, session string, body any) (map[string]any, error) {
	if endpoint == "/internal/providers/roles" {
		b.input = body.(map[string]any)
		return map[string]any{"available": true}, nil
	}
	return b.fakeBackend.DoJSON(ctx, method, endpoint, session, body)
}

func TestAgentModelRoleValidationRequiresExplicitSafeScope(t *testing.T) {
	base := map[string]any{"role": "agent:reflection:reflection_proposal_v2", "endpointId": "endpoint", "modelId": "model", "timeoutSeconds": float64(30), "expectedVersions": map[string]any{"binding:agent:reflection:reflection_proposal_v2": float64(0)}, "idempotencyKey": "save"}
	if !validateModelRole(base) {
		t.Fatal("valid agent variant binding was rejected")
	}
	clear := map[string]any{"role": "agent:reflection:reflection_proposal_v2", "clearOverride": true, "expectedVersions": map[string]any{}, "idempotencyKey": "clear"}
	if !validateModelRole(clear) {
		t.Fatal("explicit override removal was rejected")
	}
	for _, role := range []string{"generic_llm", "agent:", "agent:reflection:", "agent:reflection:foo:bar", "agent:../../secret"} {
		clear["role"] = role
		if validateModelRole(clear) {
			t.Fatalf("invalid or non-override removal accepted: %q", role)
		}
	}
	clear["role"] = "agent:reflection"
	clear["modelId"] = "ignored"
	if validateModelRole(clear) {
		t.Fatal("clear command accepted unrelated model fields")
	}
}

func TestModelRoleSamplingValidation(t *testing.T) {
	for _, tc := range []struct {
		temperature, topP any
		valid             bool
	}{
		{nil, nil, true}, {float64(0), float64(1), true}, {float64(2), float64(0.1), true},
		{float64(-0.1), float64(1), false}, {float64(2.1), float64(1), false}, {float64(1), float64(0), false}, {float64(1), float64(1.1), false}, {"0.5", float64(1), false},
	} {
		body := map[string]any{"role": "agent:reflection", "endpointId": "endpoint", "modelId": "model", "timeoutSeconds": float64(30), "expectedVersions": map[string]any{}, "idempotencyKey": "save", "temperature": tc.temperature, "topP": tc.topP}
		if got := validateModelRole(body); got != tc.valid {
			t.Errorf("temperature=%v topP=%v valid=%t", tc.temperature, tc.topP, got)
		}
	}
}

func TestModelSamplingCrossesBrowserBoundaryWithoutLosingNullOrZero(t *testing.T) {
	for _, tc := range []struct {
		name, extra        string
		temperaturePresent bool
		temperature, topP  any
	}{
		{"omitted", "", false, nil, nil},
		{"clear", `,"temperature":null`, true, nil, nil},
		{"values", `,"temperature":0,"topP":0.75`, true, float64(0), 0.75},
	} {
		t.Run(tc.name, func(t *testing.T) {
			backend := &samplingBoundaryBackend{}
			handler := newBrowserTestHandler(backend)
			body := `{"role":"agent:reflection","endpointId":"fixture","modelId":"fixture","timeoutSeconds":30,"expectedVersions":{},"idempotencyKey":"sampling"` + tc.extra + `}`
			request := httptest.NewRequest(http.MethodPut, "/api/providers/roles", strings.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("Origin", "https://fluctlight.test")
			request.Header.Set("X-CSRF-Token", "csrf")
			request.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "session"})
			request.AddCookie(&http.Cookie{Name: csrfCookieName, Value: "csrf"})
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			value, present := backend.input["temperature"]
			if response.Code != http.StatusOK || present != tc.temperaturePresent || value != tc.temperature || backend.input["top_p"] != tc.topP {
				t.Fatalf("status=%d body=%v response=%s", response.Code, backend.input, response.Body.String())
			}
		})
	}
}
