package browser

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type browserRouteCase struct {
	name   string
	method string
	path   string
	body   string
}

func browserRouteCases() []browserRouteCase {
	return []browserRouteCase{
		{name: "options", method: http.MethodOptions, path: "/api/platform/ping"},
		{name: "live", method: http.MethodGet, path: "/health/live"},
		{name: "ready", method: http.MethodGet, path: "/health/ready"},
		{name: "ping", method: http.MethodGet, path: "/api/platform/ping"},
		{name: "session", method: http.MethodGet, path: "/auth/session"},
		{name: "setup status", method: http.MethodGet, path: "/auth/setup-status"},
		{name: "logout", method: http.MethodPost, path: "/auth/logout"},
		{name: "revoke all", method: http.MethodPost, path: "/auth/revoke-all"},
		{name: "password", method: http.MethodPost, path: "/auth/password", body: `{"password":"long-enough-password"}`},
		{name: "setup", method: http.MethodPost, path: "/auth/setup", body: `{"setupToken":"setup-token-123456","password":"long-enough-password"}`},
		{name: "login", method: http.MethodPost, path: "/auth/login", body: `{"password":"long-enough-password"}`},
		{name: "settings get", method: http.MethodGet, path: "/api/settings"},
		{name: "settings put", method: http.MethodPut, path: "/api/settings", body: `{"values":{},"secrets":{},"clearSecrets":[]}`},
		{name: "capability request list", method: http.MethodGet, path: "/api/capability-requests"},
		{name: "capability request review", method: http.MethodPost, path: "/api/capability-requests/request-1/review", body: `{"status":"accepted","note":"reviewed"}`},
		{name: "provider endpoint put", method: http.MethodPut, path: "/api/providers/endpoints", body: `{"endpointId":"endpoint","kind":"openai","baseUrl":"http://provider","secretPurpose":"chat"}`},
		{name: "provider endpoint get", method: http.MethodGet, path: "/api/providers/endpoints"},
		{name: "provider models", method: http.MethodGet, path: "/api/providers/endpoints/endpoint/models"},
		{name: "provider bindings", method: http.MethodGet, path: "/api/providers"},
		{name: "provider role", method: http.MethodPut, path: "/api/providers/roles", body: `{"role":"interaction","endpointId":"endpoint","modelId":"model","tokenBudget":100,"timeoutSeconds":10}`},
		{name: "conversation create", method: http.MethodPost, path: "/api/conversations", body: `{"participantActorIds":["fl-1"]}`},
		{name: "fluctlight create", method: http.MethodPost, path: "/api/fluctlights", body: `{}`},
		{name: "creation analysis", method: http.MethodPost, path: "/api/fluctlight-creations/analysis", body: `{"description":"a new companion"}`},
		{name: "creation activation", method: http.MethodPost, path: "/api/fluctlight-creations/activate", body: `{"requestId":"request","initializationMode":"blank_slate","name":"blank"}`},
		{name: "fluctlight list", method: http.MethodGet, path: "/api/fluctlights"},
		{name: "actor group list", method: http.MethodGet, path: "/api/actor-groups"},
		{name: "actor group create", method: http.MethodPost, path: "/api/actor-groups", body: `{"name":"friends"}`},
		{name: "actor group member add", method: http.MethodPost, path: "/api/actor-groups/group/members", body: `{"actorId":"actor"}`},
		{name: "actor group member remove", method: http.MethodDelete, path: "/api/actor-groups/group/members/actor"},
		{name: "fluctlight get", method: http.MethodGet, path: "/api/fluctlights/fl-1"},
		{name: "direct conversation", method: http.MethodGet, path: "/api/fluctlights/fl-1/conversation"},
		{name: "fluctlight moments", method: http.MethodGet, path: "/api/fluctlights/fl-1/moments"},
		{name: "fluctlight moments read", method: http.MethodPost, path: "/api/fluctlights/fl-1/moments/read"},
		{name: "global moments", method: http.MethodGet, path: "/api/moments"},
		{name: "fluctlight detail", method: http.MethodGet, path: "/api/fluctlights/fl-1/detail"},
		{name: "developing self", method: http.MethodGet, path: "/api/fluctlights/fl-1/developing-self"},
		{name: "developing self rollback", method: http.MethodPost, path: "/api/fluctlights/fl-1/developing-self/claim-1/rollback", body: `{"expectedRevision":1,"reason":"test"}`},
		{name: "developing self forget", method: http.MethodPost, path: "/api/fluctlights/fl-1/developing-self/claim-1/forget", body: `{"expectedRevision":1,"reason":"test"}`},
		{name: "fluctlight status", method: http.MethodPut, path: "/api/fluctlights/fl-1/status", body: `{"status":"active","expectedRevision":0,"reason":"test"}`},
		{name: "fluctlight retire", method: http.MethodPost, path: "/api/fluctlights/fl-1/retire", body: `{"expectedRevision":0,"reason":"test"}`},
		{name: "foundation revision", method: http.MethodPost, path: "/api/fluctlights/fl-1/foundation-revisions", body: `{"changes":{"name":"new"},"expectedRevision":0,"reason":"test"}`},
		{name: "foundation accept", method: http.MethodPost, path: "/api/fluctlights/fl-1/foundation-revisions/rev-1/accept", body: `{"expectedRevision":0,"reason":"test"}`},
		{name: "foundation reject", method: http.MethodPost, path: "/api/fluctlights/fl-1/foundation-revisions/rev-1/reject", body: `{"expectedRevision":0,"reason":"test"}`},
		{name: "foundation rollback", method: http.MethodPost, path: "/api/fluctlights/fl-1/foundation-revisions/rollback", body: `{"targetRevision":0,"expectedRevision":0,"reason":"test"}`},
		{name: "moment comment", method: http.MethodPost, path: "/api/moments/moment-1/comments", body: `{"text":"comment"}`},
		{name: "memory revise", method: http.MethodPut, path: "/api/memories/memory-1", body: `{"expectedRevision":0,"content":"updated","evidenceRefs":["evidence"]}`},
		{name: "memory forget", method: http.MethodPost, path: "/api/memories/memory-1/forget", body: `{"expectedRevision":0,"evidenceRefs":["evidence"]}`},
		{name: "relationship rollback", method: http.MethodPost, path: "/api/fluctlights/fl-1/relationships/rollback", body: `{"targetActorId":"actor","targetRevision":0,"expectedRevision":0,"evidenceRefs":["evidence"]}`},
		{name: "relationship edit", method: http.MethodPut, path: "/api/fluctlights/fl-1/relationships/actor", body: `{"expectedRevision":0,"role":{"primary":"friend"},"metrics":{"trust":0.8},"trend":"improving","summary":"trusted","emotionalAssociation":{},"evidenceRefs":["evidence"],"reason":"corrected"}`},
		{name: "autonomy list", method: http.MethodGet, path: "/api/fluctlights/fl-1/autonomy-actions"},
		{name: "autonomy govern", method: http.MethodPost, path: "/api/autonomy-actions/action-1/govern", body: `{"status":"paused","reason":"test"}`},
		{name: "life event create", method: http.MethodPost, path: "/api/fluctlights/fl-1/events", body: `{"kind":"meeting","startAt":"2026-01-01T10:00:00Z","endAt":"2026-01-01T11:00:00Z","evidenceRefs":["evidence"],"expectedLifeContextRevision":"life_ctx_0123456789abcdef0123456789abcdef","idempotencyKey":"event-create-1"}`},
		{name: "life event cancel", method: http.MethodPost, path: "/api/fluctlights/fl-1/events/event-1/cancel", body: `{"expectedEventRevision":1,"expectedLifeContextRevision":"life_ctx_0123456789abcdef0123456789abcdef","idempotencyKey":"event-cancel-1"}`},
		{name: "life presence", method: http.MethodPut, path: "/api/fluctlights/fl-1/presence", body: `{"currentTask":"working","expectedLifeContextRevision":"life_ctx_0123456789abcdef0123456789abcdef","idempotencyKey":"presence-set-1"}`},
		{name: "schedule accept", method: http.MethodPost, path: "/api/fluctlights/fl-1/schedules", body: `{"localDate":"2026-01-01","timezone":"UTC","items":[{"startAt":"2026-01-01T10:00:00Z","endAt":"2026-01-01T11:00:00Z","activity":"work","scene":"office"}],"evidenceRefs":["evidence"],"expectedRevision":0,"expectedLifeContextRevision":"life_ctx_0123456789abcdef0123456789abcdef","idempotencyKey":"schedule-create-1"}`},
		{name: "schedule cancel", method: http.MethodPost, path: "/api/fluctlights/fl-1/schedules/schedule-1/cancel", body: `{"expectedRevision":1,"expectedLifeContextRevision":"life_ctx_0123456789abcdef0123456789abcdef","idempotencyKey":"schedule-cancel-1"}`},
		{name: "workflow list", method: http.MethodGet, path: "/api/diagnostics/workflows"},
		{name: "workflow status", method: http.MethodGet, path: "/api/diagnostics/workflows/workflow-1/status"},
		{name: "workflow history", method: http.MethodGet, path: "/api/diagnostics/workflows/workflow-1/history"},
		{name: "workflow pause", method: http.MethodPost, path: "/api/diagnostics/workflows/workflow-1/pause"},
		{name: "workflow resume", method: http.MethodPost, path: "/api/diagnostics/workflows/workflow-1/resume"},
		{name: "workflow cancel", method: http.MethodPost, path: "/api/diagnostics/workflows/workflow-1/cancel"},
		{name: "workflow reset", method: http.MethodPost, path: "/api/diagnostics/workflows/workflow-1/reset", body: `{"historyPoint":1}`},
		{name: "workflow restart", method: http.MethodPost, path: "/api/diagnostics/workflows/workflow-1/restart"},
		{name: "moment reaction", method: http.MethodPost, path: "/api/moments/moment-1/reactions", body: `{}`},
		{name: "moment hide", method: http.MethodPost, path: "/api/moments/moment-1/hide"},
		{name: "moment restore", method: http.MethodPost, path: "/api/moments/moment-1/restore"},
		{name: "conversation history", method: http.MethodGet, path: "/api/conversations/conversation-1/messages"},
		{name: "conversation read", method: http.MethodPost, path: "/api/conversations/conversation-1/read", body: `{"readSequence":1}`},
		{name: "conversation turn", method: http.MethodPost, path: "/api/conversations/conversation-1/turn", body: `{"text":"hello","fluctlightId":"fl-1","idempotencyKey":"turn-1"}`},
		{name: "diagnostics", method: http.MethodGet, path: "/api/diagnostics"},
		{name: "lifecycle diagnostics", method: http.MethodGet, path: "/api/diagnostics/lifecycle"},
		{name: "diagnostics clear", method: http.MethodDelete, path: "/api/diagnostics"},
		{name: "diagnostic model runs", method: http.MethodGet, path: "/api/diagnostics/model-runs"},
		{name: "diagnostic media prompts", method: http.MethodGet, path: "/api/diagnostics/media-prompts"},
		{name: "diagnostic media prompt retry", method: http.MethodPost, path: "/api/diagnostics/media-prompts/media-1/retry"},
		{name: "diagnostics export", method: http.MethodGet, path: "/api/diagnostics/export"},
		{name: "media", method: http.MethodGet, path: "/api/media/asset-1"},
	}
}

func invokeBrowser(handler http.Handler, method, target, body string, headers map[string]string, cookies map[string]string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	for key, value := range cookies {
		request.AddCookie(&http.Cookie{Name: key, Value: value})
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func TestEveryBrowserOpenAPIRouteReachesTheInProcessBoundary(t *testing.T) {
	for _, route := range browserRouteCases() {
		route := route
		t.Run(route.name, func(t *testing.T) {
			backend := &fakeBackend{stream: func(writer http.ResponseWriter) {
				_, _ = writer.Write([]byte("{\"type\":\"completed\",\"turn_id\":\"turn-1\",\"sequence\":0,\"payload\":{}}\n"))
			}}
			handler := newBrowserTestHandler(backend)
			headers := map[string]string{}
			cookies := map[string]string{}
			switch {
			case route.method == http.MethodOptions:
				headers["Origin"] = "https://fluctlight.test"
			case route.method != http.MethodGet && route.path != "/auth/login" && route.path != "/auth/setup":
				headers["Origin"] = "https://fluctlight.test"
				headers["X-CSRF-Token"] = "csrf"
				cookies[csrfCookieName] = "csrf"
				cookies[sessionCookieName] = "opaque"
			case route.path == "/auth/login" || route.path == "/auth/setup":
				headers["Origin"] = "https://fluctlight.test"
				headers["X-CSRF-Token"] = "csrf"
				cookies[csrfCookieName] = "csrf"
			default:
				cookies[sessionCookieName] = "opaque"
			}
			response := invokeBrowser(handler, route.method, "https://api.test"+route.path, route.body, headers, cookies)
			if response.Code != http.StatusOK && response.Code != http.StatusNoContent && response.Code != http.StatusPartialContent {
				t.Fatalf("route returned %d: %s", response.Code, response.Body.String())
			}
		})
	}
}
