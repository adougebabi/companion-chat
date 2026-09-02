package bff

import (
	"io"
	"net/http"
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
		{name: "autonomy list", method: http.MethodGet, path: "/api/fluctlights/fl-1/autonomy-actions"},
		{name: "autonomy govern", method: http.MethodPost, path: "/api/autonomy-actions/action-1/govern", body: `{"status":"paused","reason":"test"}`},
		{name: "life event create", method: http.MethodPost, path: "/api/fluctlights/fl-1/events", body: `{"kind":"meeting","startAt":"2026-01-01T10:00:00Z","endAt":"2026-01-01T11:00:00Z","evidenceRefs":["evidence"]}`},
		{name: "life event cancel", method: http.MethodPost, path: "/api/fluctlights/fl-1/events/event-1/cancel"},
		{name: "life presence", method: http.MethodPut, path: "/api/fluctlights/fl-1/presence", body: `{"currentTask":"working"}`},
		{name: "schedule accept", method: http.MethodPost, path: "/api/fluctlights/fl-1/schedules", body: `{"localDate":"2026-01-01","timezone":"UTC","items":[{"startAt":"2026-01-01T10:00:00Z","endAt":"2026-01-01T11:00:00Z","activity":"work","scene":"office"}],"evidenceRefs":["evidence"]}`},
		{name: "schedule cancel", method: http.MethodPost, path: "/api/fluctlights/fl-1/schedules/schedule-1/cancel", body: `{"expectedRevision":0}`},
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
		{name: "diagnostics clear", method: http.MethodDelete, path: "/api/diagnostics"},
		{name: "diagnostic model runs", method: http.MethodGet, path: "/api/diagnostics/model-runs"},
		{name: "diagnostics export", method: http.MethodGet, path: "/api/diagnostics/export"},
		{name: "media", method: http.MethodGet, path: "/api/media/asset-1"},
	}
}

// TestEveryBrowserRouteHasAGoHandler is deliberately a route smoke matrix.
// Detailed mapping, error and stream assertions live beside the individual
// helpers; this test prevents a new/forgotten operation from silently falling
// through to 404 during the Go gateway rollout.
func TestEveryBrowserRouteHasAGoHandler(t *testing.T) {
	routes := browserRouteCases()

	for _, route := range routes {
		route := route
		t.Run(route.name, func(t *testing.T) {
			handler := testBFF(t, fakeCoreForRoute)
			headers := map[string]string{}
			cookies := map[string]string{}
			switch {
			case route.method == http.MethodOptions:
				headers["Origin"] = "https://fluctlight.local"
			case route.method != http.MethodGet && route.path != "/auth/login" && route.path != "/auth/setup":
				headers["Origin"] = "https://fluctlight.local"
				headers["X-CSRF-Token"] = "csrf"
				cookies[csrfCookieName] = "csrf"
				cookies[sessionCookieName] = "opaque"
			case route.path == "/auth/login" || route.path == "/auth/setup":
				headers["Origin"] = "https://fluctlight.local"
				headers["X-CSRF-Token"] = "csrf"
				cookies[csrfCookieName] = "csrf"
			default:
				cookies[sessionCookieName] = "opaque"
			}
			response := invoke(handler, route.method, "http://gateway.test"+route.path, route.body, headers, cookies)
			if response.Code != http.StatusOK && response.Code != http.StatusNoContent && response.Code != http.StatusPartialContent {
				t.Fatalf("route returned %d: %s", response.Code, response.Body.String())
			}
		})
	}
}

func fakeCoreForRoute(request *http.Request) (*http.Response, error) {
	path := request.URL.Path
	switch {
	case path == "/health/ready":
		return jsonResponse(http.StatusOK, `{"status":"ready","role":"api"}`), nil
	case path == "/internal/platform/ping":
		return jsonResponse(http.StatusOK, `{"status":"ok","role":"api"}`), nil
	case path == "/internal/auth/session":
		return jsonResponse(http.StatusOK, `{"authenticated":true,"actor_id":"owner"}`), nil
	case path == "/internal/auth/setup-status":
		return jsonResponse(http.StatusOK, `{"setup_available":true}`), nil
	case path == "/internal/auth/login" || path == "/internal/auth/setup":
		return jsonResponse(http.StatusOK, `{"authenticated":true,"actor_id":"owner","session_token":"session"}`), nil
	case path == "/internal/settings":
		return jsonResponse(http.StatusOK, `{"values":{},"configured_secrets":[]}`), nil
	case path == "/internal/capability-requests" && request.Method == http.MethodGet:
		return jsonResponse(http.StatusOK, `[]`), nil
	case strings.HasSuffix(path, "/review"):
		return jsonResponse(http.StatusOK, `{"id":"request-1","status":"accepted"}`), nil
	case strings.HasSuffix(path, "/models"):
		return jsonResponse(http.StatusOK, `{"endpoint_id":"endpoint","models":[]}`), nil
	case (path == "/internal/providers/endpoints" || path == "/internal/providers") && request.Method == http.MethodGet:
		return jsonResponse(http.StatusOK, `[]`), nil
	case path == "/internal/providers/roles":
		return jsonResponse(http.StatusOK, `{"role":"interaction","available":true,"capability_version":"v1"}`), nil
	case (path == "/internal/fluctlights" || path == "/internal/actor-groups" || path == "/internal/moments" || strings.HasSuffix(path, "/moments") || strings.HasSuffix(path, "/autonomy-actions")) && request.Method == http.MethodGet:
		return jsonResponse(http.StatusOK, `[]`), nil
	case (path == "/internal/diagnostics" || path == "/internal/diagnostics/model-runs") && request.Method == http.MethodGet || strings.HasSuffix(path, "/workflows"):
		return jsonResponse(http.StatusOK, `[]`), nil
	case path == "/internal/diagnostics" && request.Method == http.MethodDelete:
		return jsonResponse(http.StatusOK, `{"cleared":1}`), nil
	case strings.HasSuffix(path, "/history") || strings.HasSuffix(path, "/conversation") || strings.HasSuffix(path, "/conversations"):
		return jsonResponse(http.StatusOK, conversationPageJSON), nil
	case path == "/internal/diagnostics/export":
		return jsonResponse(http.StatusOK, `{}`), nil
	case strings.HasSuffix(path, "/turn"):
		return jsonResponseWithContentType(http.StatusOK, "{\"type\":\"completed\",\"turn_id\":\"turn-1\",\"sequence\":0,\"payload\":{}}\n", "application/x-ndjson"), nil
	case strings.HasPrefix(path, "/internal/media/"):
		status := http.StatusOK
		headers := http.Header{"Content-Type": []string{"image/png"}, "Content-Length": []string{"4"}, "ETag": []string{"etag"}}
		if request.Header.Get("Range") != "" {
			status = http.StatusPartialContent
			headers["Content-Range"] = []string{"bytes 0-3/4"}
			headers["Accept-Ranges"] = []string{"bytes"}
		}
		return &http.Response{StatusCode: status, ContentLength: 4, Header: headers, Body: io.NopCloser(strings.NewReader("data"))}, nil
	default:
		if request.Method == http.MethodGet {
			return jsonResponse(http.StatusOK, `{}`), nil
		}
		return &http.Response{StatusCode: http.StatusNoContent, Body: io.NopCloser(strings.NewReader(""))}, nil
	}
}

const conversationPageJSON = `{"conversation":{"id":"conversation-1","created_by_actor_id":"owner","title":null,"revision":0,"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:00:00Z"},"participants":[],"messages":[],"next_before_sequence":null}`
