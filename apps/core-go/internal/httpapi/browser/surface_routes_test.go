package browser

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
)

type surfaceMemoryBackend struct {
	fakeBackend
	surface *core.OwnerSurface
}

func (b *surfaceMemoryBackend) DoJSON(ctx context.Context, method, endpoint, session string, body any) (map[string]any, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	if !core.MatchOwnerSurface(method, parsed.Path) {
		return b.fakeBackend.DoJSON(ctx, method, endpoint, session, body)
	}
	values, _ := body.(map[string]any)
	result, dispatchErr := b.surface.Dispatch(ctx, method, parsed.Path, parsed.Query(), "owner-1", values)
	if dispatchErr != nil {
		var surfaceErr *core.SurfaceError
		if errors.As(dispatchErr, &surfaceErr) {
			return nil, &CoreError{Status: surfaceErr.Status, Code: surfaceErr.Code, Message: surfaceErr.Message, Details: surfaceErr.Details, Replay: surfaceErr.Replay}
		}
		return nil, dispatchErr
	}
	return result, nil
}

func newSurfaceHandler() http.Handler {
	return newBrowserTestHandler(&surfaceMemoryBackend{surface: core.NewMemoryOwnerSurface(core.SurfaceHooks{})})
}

func surfaceInvoke(handler http.Handler, method, path, body string, session, csrf bool) *httpResponse {
	headers := map[string]string{}
	cookies := map[string]string{}
	if method != http.MethodGet && method != http.MethodHead {
		headers["Origin"] = "https://fluctlight.test"
		if csrf {
			headers["X-CSRF-Token"] = "csrf"
			cookies[csrfCookieName] = "csrf"
		}
	}
	if session {
		cookies[sessionCookieName] = "opaque"
	}
	response := invokeBrowser(handler, method, "https://api.test"+path, body, headers, cookies)
	return &httpResponse{code: response.Code, body: response.Body.Bytes(), header: response.Header()}
}

type httpResponse struct {
	code   int
	body   []byte
	header http.Header
}

func (r httpResponse) json(t *testing.T) map[string]any {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal(r.body, &payload); err != nil {
		t.Fatalf("decode %d %s: %v", r.code, r.body, err)
	}
	return payload
}

func TestSurfaceRoutesRequireOwnerSessionAndCSRF(t *testing.T) {
	handler := newSurfaceHandler()
	unauthenticated := surfaceInvoke(handler, http.MethodPost, "/api/fluctlights/fl-1/promises", `{"idempotencyKey":"p1","title":"一起吃饭"}`, false, true)
	if unauthenticated.code != http.StatusUnauthorized || unauthenticated.json(t)["code"] != "unauthenticated" {
		t.Fatalf("unauthenticated = %d %s", unauthenticated.code, unauthenticated.body)
	}
	missingCSRF := surfaceInvoke(handler, http.MethodPost, "/api/fluctlights/fl-1/promises", `{"idempotencyKey":"p1","title":"一起吃饭"}`, true, false)
	if missingCSRF.code != http.StatusForbidden || missingCSRF.json(t)["code"] != "invalid_origin" {
		t.Fatalf("csrf = %d %s", missingCSRF.code, missingCSRF.body)
	}
}

func TestSurfaceIdempotencyReplayAndConflict(t *testing.T) {
	handler := newSurfaceHandler()
	body := `{"idempotencyKey":"wear-1","itemId":"coat","slot":"outer"}`
	first := surfaceInvoke(handler, http.MethodPost, "/api/fluctlights/fl-1/wardrobe/wear", body, true, true)
	if first.code != http.StatusAccepted {
		t.Fatalf("wear = %d %s", first.code, first.body)
	}
	requestID := stringValue(mapValue(first.json(t)["request"])["id"])
	if requestID == "" || mapValue(first.json(t)["request"])["status"] != "pending" {
		t.Fatalf("wear did not create a pending request: %s", first.body)
	}
	replay := surfaceInvoke(handler, http.MethodPost, "/api/fluctlights/fl-1/wardrobe/wear", body, true, true)
	if replay.code != http.StatusAccepted || replay.header.Get("Idempotent-Replayed") != "true" {
		t.Fatalf("replay = %d %s replay=%q", replay.code, replay.body, replay.header.Get("Idempotent-Replayed"))
	}
	if stringValue(mapValue(replay.json(t)["request"])["id"]) != requestID {
		t.Fatalf("replay changed the request: %s", replay.body)
	}
	reused := surfaceInvoke(handler, http.MethodPost, "/api/fluctlights/fl-1/wardrobe/wear", `{"idempotencyKey":"wear-1","itemId":"hat","slot":"head"}`, true, true)
	if reused.code != http.StatusUnprocessableEntity || reused.json(t)["code"] != "idempotency_key_reused" {
		t.Fatalf("reused = %d %s", reused.code, reused.body)
	}

	direct := `{"idempotencyKey":"wear-direct","itemId":"coat","slot":"outer","expectedRevision":0,"reason":"所有者修改","confirm":true}`
	saved := surfaceInvoke(handler, http.MethodPost, "/api/fluctlights/fl-1/wardrobe/wear", direct, true, true)
	if saved.code != http.StatusOK || saved.json(t)["identity"] != "owner_edit" || saved.json(t)["actor"] != "owner" {
		t.Fatalf("direct wear = %d %s", saved.code, saved.body)
	}
	conflict := surfaceInvoke(handler, http.MethodPost, "/api/fluctlights/fl-1/wardrobe/wear", `{"idempotencyKey":"wear-direct-2","itemId":"coat","slot":"outer","expectedRevision":0,"reason":"再次修改","confirm":true}`, true, true)
	details := mapValue(conflict.json(t)["details"])
	if conflict.code != http.StatusConflict || conflict.json(t)["code"] != "revision_conflict" || details["current_revision"] != float64(1) {
		t.Fatalf("conflict = %d %s", conflict.code, conflict.body)
	}
}

func TestSurfaceForbiddenWrites(t *testing.T) {
	handler := newSurfaceHandler()
	cases := []struct {
		name, method, path, body, code string
	}{
		{name: "moment author", method: http.MethodPost, path: "/api/moments", body: `{"idempotencyKey":"m1","text":"hi","authorFluctlightId":"fl-1"}`, code: "moment_author_forbidden"},
		{name: "photo revision", method: http.MethodPost, path: "/api/fluctlights/fl-1/wardrobe/items/item-1/photos", body: `{"idempotencyKey":"photo","prompt":"窗边","expectedRevision":1}`, code: "photo_cannot_change_facts"},
		{name: "diary preview", method: http.MethodPost, path: "/api/fluctlights/fl-1/diary", body: `{"idempotencyKey":"diary","body":"草稿","source":"preview"}`, code: "preview_life_write_forbidden"},
		{name: "preview life write", method: http.MethodPost, path: "/api/studio/previews", body: `{"idempotencyKey":"preview","text":"试","lifeWrite":true}`, code: "preview_life_write_forbidden"},
		{name: "template harm", method: http.MethodPut, path: "/api/fluctlights/fl-1/world/templates/permanent_harm", body: `{"idempotencyKey":"harm","enabled":true,"expectedRevision":0,"reason":"no"}`, code: "permanent_harm_forbidden"},
		{name: "pack facts", method: http.MethodPost, path: "/api/studio/packs", body: `{"idempotencyKey":"pack","pack":{"resources":[],"memories":[]}}`, code: "pack_contains_runtime_facts"},
		{name: "autonomy harm", method: http.MethodPut, path: "/api/fluctlights/fl-1/autonomy-policy", body: `{"idempotencyKey":"auto","expectedRevision":0,"reason":"no","permanentHarm":true}`, code: "permanent_harm_forbidden"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			response := surfaceInvoke(handler, test.method, test.path, test.body, true, true)
			if response.code != http.StatusUnprocessableEntity || response.json(t)["code"] != test.code {
				t.Fatalf("%s = %d %s", test.name, response.code, response.body)
			}
			if strings.Contains(string(response.body), "asset_id") {
				t.Fatalf("%s invented an asset id: %s", test.name, response.body)
			}
		})
	}
}

func TestSurfaceUnreadCountAndTimelineStayUnprovided(t *testing.T) {
	handler := newSurfaceHandler()
	unread := surfaceInvoke(handler, http.MethodGet, "/api/notifications/unread-count", "", true, false)
	if unread.code != http.StatusNotFound || unread.json(t)["code"] != "not_provided" {
		t.Fatalf("unread = %d %s", unread.code, unread.body)
	}
	timeline := surfaceInvoke(newBrowserTestHandler(&fakeBackend{}), http.MethodGet, "/api/ops/timeline-checkpoints", "", true, false)
	if timeline.code != http.StatusNotFound {
		t.Fatalf("timeline = %d %s", timeline.code, timeline.body)
	}
}

func TestSurfaceRoutesMatchCoreAndMatrix(t *testing.T) {
	cases := map[string]struct{}{}
	for _, route := range browserRouteCases() {
		cases[route.method+" "+route.path] = struct{}{}
	}
	replacements := map[string]string{
		":fluctlightId": "fl-1", ":entryId": "entry-1", ":templateId": "template-1", ":placeId": "place-1",
		":itemId": "item-1", ":scheduleId": "schedule-1", ":reflectionId": "reflection-1", ":notificationId": "notice-1",
		":resourceId": "resource-1", ":backupId": "backup-1", ":taskId": "task-1", ":memoryId": "memory-1",
		":requestId": "request-1", ":groupId": "group-1",
	}
	for _, route := range surfaceRoutes {
		sample := route.pattern
		for key, value := range replacements {
			sample = strings.ReplaceAll(sample, key, value)
		}
		if _, ok := cases[route.method+" "+sample]; !ok {
			t.Fatalf("matrix is missing %s %s", route.method, sample)
		}
		if !core.MatchOwnerSurface(route.method, "/internal"+strings.TrimPrefix(sample, "/api")) {
			t.Fatalf("core matcher missed %s %s", route.method, sample)
		}
	}
}

func mapValue(value any) map[string]any {
	if result, ok := value.(map[string]any); ok {
		return result
	}
	return map[string]any{}
}
