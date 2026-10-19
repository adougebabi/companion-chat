package browser

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
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
	return newSurfaceHandlerWith(core.SurfaceHooks{})
}

func newSurfaceHandlerWith(hooks core.SurfaceHooks) http.Handler {
	return newBrowserTestHandler(&surfaceMemoryBackend{surface: core.NewMemoryOwnerSurface(hooks)})
}

type memoryWardrobe struct {
	mu       sync.Mutex
	revision int
	worn     map[string]string
	used     map[string]bool
}

func (w *memoryWardrobe) apply(_ context.Context, _, _ string, edit core.WardrobeOwnerEdit) (core.WardrobeOwnerEditResult, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if edit.Expected != w.revision {
		return core.WardrobeOwnerEditResult{}, &core.SurfaceError{
			Status: http.StatusConflict, Code: "revision_conflict", Message: "Revision conflict",
			Details: map[string]any{"current_revision": w.revision, "expected_revision": edit.Expected, "resource": "wardrobe"},
		}
	}
	w.revision++
	if edit.Kind == "wear" {
		if w.worn == nil {
			w.worn = map[string]string{}
		}
		w.worn[edit.Slot] = edit.ItemID
	} else if edit.Active {
		if w.used == nil {
			w.used = map[string]bool{}
		}
		w.used[edit.ItemID] = true
	} else {
		delete(w.used, edit.ItemID)
	}
	return core.WardrobeOwnerEditResult{Revision: w.revision}, nil
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

	wardrobe := &memoryWardrobe{}
	directHandler := newSurfaceHandlerWith(core.SurfaceHooks{ApplyWardrobeEdit: wardrobe.apply})
	direct := `{"idempotencyKey":"wear-direct","itemId":"coat","slot":"outer","expectedRevision":0,"reason":"所有者修改","confirm":true}`
	saved := surfaceInvoke(directHandler, http.MethodPost, "/api/fluctlights/fl-1/wardrobe/wear", direct, true, true)
	savedBody := saved.json(t)
	if saved.code != http.StatusOK || savedBody["identity"] != "owner_edit" || savedBody["actor"] != "owner" || savedBody["recorded_by"] != "owner" || savedBody["revision"] != float64(1) || savedBody["owner_revision"] != float64(1) {
		t.Fatalf("direct wear = %d %s", saved.code, saved.body)
	}
	if wardrobe.revision != 1 || wardrobe.worn["outer"] != "coat" {
		t.Fatalf("direct wear did not land on wardrobe state: revision=%d worn=%v", wardrobe.revision, wardrobe.worn)
	}
	conflict := surfaceInvoke(directHandler, http.MethodPost, "/api/fluctlights/fl-1/wardrobe/wear", `{"idempotencyKey":"wear-direct-2","itemId":"coat","slot":"outer","expectedRevision":0,"reason":"再次修改","confirm":true}`, true, true)
	details := mapValue(conflict.json(t)["details"])
	if conflict.code != http.StatusConflict || conflict.json(t)["code"] != "revision_conflict" || details["current_revision"] != float64(1) || details["resource"] != "wardrobe" {
		t.Fatalf("conflict = %d %s", conflict.code, conflict.body)
	}
}

func TestOwnerWardrobeEditConflictsWithCharacterRevision(t *testing.T) {
	wardrobe := &memoryWardrobe{revision: 4, worn: map[string]string{"outer": "shirt"}}
	handler := newSurfaceHandlerWith(core.SurfaceHooks{ApplyWardrobeEdit: wardrobe.apply})
	stale := surfaceInvoke(handler, http.MethodPost, "/api/fluctlights/fl-1/wardrobe/wear", `{"idempotencyKey":"wear-stale","itemId":"coat","slot":"outer","expectedRevision":3,"reason":"所有者修改","confirm":true}`, true, true)
	details := mapValue(stale.json(t)["details"])
	if stale.code != http.StatusConflict || stale.json(t)["code"] != "revision_conflict" || details["current_revision"] != float64(4) {
		t.Fatalf("stale owner wear = %d %s", stale.code, stale.body)
	}
	if wardrobe.revision != 4 || wardrobe.worn["outer"] != "shirt" {
		t.Fatalf("stale owner wear changed wardrobe state: revision=%d worn=%v", wardrobe.revision, wardrobe.worn)
	}
	saved := surfaceInvoke(handler, http.MethodPost, "/api/fluctlights/fl-1/wardrobe/wear", `{"idempotencyKey":"wear-current","itemId":"coat","slot":"outer","expectedRevision":4,"reason":"所有者修改","confirm":true}`, true, true)
	body := saved.json(t)
	if saved.code != http.StatusOK || body["revision"] != float64(5) || body["owner_revision"] != float64(1) || body["identity"] != "owner_edit" || body["actor"] != "owner" || body["recorded_by"] != "owner" || body["status"] != "saved" {
		t.Fatalf("owner wear = %d %s", saved.code, saved.body)
	}
	if wardrobe.revision != 5 || wardrobe.worn["outer"] != "coat" {
		t.Fatalf("owner wear revision = %d worn=%v", wardrobe.revision, wardrobe.worn)
	}
	replay := surfaceInvoke(handler, http.MethodPost, "/api/fluctlights/fl-1/wardrobe/wear", `{"idempotencyKey":"wear-current","itemId":"coat","slot":"outer","expectedRevision":4,"reason":"所有者修改","confirm":true}`, true, true)
	if replay.code != http.StatusOK || replay.header.Get("Idempotent-Replayed") != "true" || replay.json(t)["revision"] != float64(5) || wardrobe.revision != 5 {
		t.Fatalf("owner wear replay = %d %s revision=%d", replay.code, replay.body, wardrobe.revision)
	}
	wardrobe.revision = 9
	useStale := surfaceInvoke(handler, http.MethodPost, "/api/fluctlights/fl-1/wardrobe/items/brush/use", `{"idempotencyKey":"use-stale","active":true,"expectedRevision":5,"reason":"所有者修改","confirm":true}`, true, true)
	useDetails := mapValue(useStale.json(t)["details"])
	if useStale.code != http.StatusConflict || useDetails["current_revision"] != float64(9) {
		t.Fatalf("stale owner use = %d %s", useStale.code, useStale.body)
	}
	used := surfaceInvoke(handler, http.MethodPost, "/api/fluctlights/fl-1/wardrobe/items/brush/use", `{"idempotencyKey":"use-current","active":true,"expectedRevision":9,"reason":"所有者修改","confirm":true}`, true, true)
	if used.code != http.StatusOK || used.json(t)["revision"] != float64(10) || used.json(t)["identity"] != "owner_edit" || wardrobe.revision != 10 || !wardrobe.used["brush"] {
		t.Fatalf("owner use = %d %s revision=%d used=%v", used.code, used.body, wardrobe.revision, wardrobe.used)
	}
}

func TestStudioApplyRecordsChoicesWithoutWritingInstance(t *testing.T) {
	handler := newSurfaceHandler()
	created := surfaceInvoke(handler, http.MethodPost, "/api/studio/resources", `{"idempotencyKey":"res-1","kind":"character_template","name":"模板","body":{}}`, true, true)
	if created.code != http.StatusCreated {
		t.Fatalf("create resource = %d %s", created.code, created.body)
	}
	resourceID := stringValue(created.json(t)["id"])
	applied := surfaceInvoke(handler, http.MethodPost, "/api/studio/resources/"+resourceID+"/apply", `{"idempotencyKey":"apply-1","fluctlightId":"fl-1","resourceVersion":"1","reason":"记录","choices":[{"field":"name","decision":"keep"},{"field":"tone","decision":"override"}]}`, true, true)
	body := applied.json(t)
	if applied.code != http.StatusOK || body["status"] != "recorded_not_applied" || body["runtime_effect"] != "recorded_only" || body["applied"] != false || body["instance_revision_unchanged"] != true {
		t.Fatalf("apply = %d %s", applied.code, applied.body)
	}
	if _, exists := body["applied_version"]; exists {
		t.Fatalf("apply presented a version as applied: %s", applied.body)
	}
	if _, exists := body["applied_at"]; exists {
		t.Fatalf("apply presented an apply time: %s", applied.body)
	}
	if conflicts, exists := body["conflicts"]; !exists || conflicts != nil {
		t.Fatalf("conflicts = %#v", body["conflicts"])
	}
	if body["instance_revision"] != nil || body["resource_version"] != "1" {
		t.Fatalf("instance version changed: %s", applied.body)
	}
	choices := body["choices"].([]any)
	if len(choices) != 2 || mapValue(choices[0])["decision"] != "keep" || mapValue(choices[1])["decision"] != "override" {
		t.Fatalf("choices were not kept: %#v", body["choices"])
	}
	resource := surfaceInvoke(handler, http.MethodGet, "/api/studio/resources/"+resourceID, "", true, false)
	refs := resource.json(t)["referenced_by"].([]any)
	ref := mapValue(refs[0])
	if resource.code != http.StatusOK || len(refs) != 1 || ref["status"] != "recorded_not_applied" || ref["selected_version"] != "1" || ref["applied_version"] != nil {
		t.Fatalf("resource reference = %d %s", resource.code, resource.body)
	}

	pinnedHandler := newSurfaceHandlerWith(core.SurfaceHooks{InstanceRevision: func(context.Context, string, string) (int, error) { return 7, nil }})
	createdPinned := surfaceInvoke(pinnedHandler, http.MethodPost, "/api/studio/resources", `{"idempotencyKey":"res-2","kind":"place","name":"地点","body":{}}`, true, true)
	pinnedID := stringValue(createdPinned.json(t)["id"])
	stale := surfaceInvoke(pinnedHandler, http.MethodPost, "/api/studio/resources/"+pinnedID+"/apply", `{"idempotencyKey":"apply-stale","fluctlightId":"fl-1","resourceVersion":"1","reason":"记录","expectedInstanceRevision":6,"choices":[{"field":"name","decision":"keep"}]}`, true, true)
	if stale.code != http.StatusConflict || mapValue(stale.json(t)["details"])["current_revision"] != float64(7) {
		t.Fatalf("stale instance = %d %s", stale.code, stale.body)
	}
	current := surfaceInvoke(pinnedHandler, http.MethodPost, "/api/studio/resources/"+pinnedID+"/apply", `{"idempotencyKey":"apply-current","fluctlightId":"fl-1","resourceVersion":"1","reason":"记录","expectedInstanceRevision":7,"choices":[{"field":"name","decision":"keep"}]}`, true, true)
	currentBody := current.json(t)
	if current.code != http.StatusOK || currentBody["status"] != "recorded_not_applied" || currentBody["instance_revision"] != float64(7) || currentBody["instance_revision_unchanged"] != true || currentBody["applied"] != false {
		t.Fatalf("pinned apply = %d %s", current.code, current.body)
	}
}

func TestSurfaceForbiddenWrites(t *testing.T) {
	handler := newSurfaceHandler()
	cases := []struct {
		name, method, path, body, code string
	}{
		{name: "moment author", method: http.MethodPost, path: "/api/moments", body: `{"idempotencyKey":"m1","text":"hi","authorFluctlightId":"fl-1"}`, code: "moment_author_forbidden"},
		{name: "diary preview", method: http.MethodPost, path: "/api/fluctlights/fl-1/diary", body: `{"idempotencyKey":"diary","body":"草稿","source":"preview"}`, code: "preview_life_write_forbidden"},
		{name: "preview life write", method: http.MethodPost, path: "/api/studio/previews", body: `{"idempotencyKey":"preview","text":"试","lifeWrite":true}`, code: "preview_life_write_forbidden"},
		{name: "template harm", method: http.MethodPut, path: "/api/fluctlights/fl-1/world/templates/permanent_harm", body: `{"idempotencyKey":"harm","enabled":true,"expectedRevision":0,"reason":"no"}`, code: "permanent_harm_forbidden"},
		{name: "pack facts", method: http.MethodPost, path: "/api/studio/packs", body: `{"idempotencyKey":"pack","pack":{"resources":[],"memories":[]}}`, code: "pack_contains_runtime_facts"},
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
	covered := map[string]bool{}
	for _, test := range browserRouteCases() {
		best := ""
		specificity := -1
		for _, route := range surfaceRoutes {
			template := surfaceTemplate(route.pattern)
			if route.method != test.method || !concretePathMatchesTemplate(test.path, template) {
				continue
			}
			score := templateSpecificity(template)
			if score > specificity {
				best = route.method + " " + route.pattern
				specificity = score
			}
		}
		if best != "" {
			covered[best] = true
		}
	}
	for _, route := range surfaceRoutes {
		if !covered[route.method+" "+route.pattern] {
			t.Errorf("matrix is missing %s %s", route.method, route.pattern)
		}
		// Matcher identity is independent of the fixture's chosen sample ID.
		parts := strings.Split(route.pattern, "/")
		for i, part := range parts {
			if strings.HasPrefix(part, ":") {
				parts[i] = "fixture-" + strings.TrimPrefix(part, ":")
			}
		}
		sample := strings.Join(parts, "/")
		if !core.MatchOwnerSurface(route.method, "/internal"+strings.TrimPrefix(sample, "/api")) && !core.MatchWorldSurface(route.method, "/internal"+strings.TrimPrefix(sample, "/api")) && !core.MatchStudioJobSurface(route.method, "/internal"+strings.TrimPrefix(sample, "/api")) && !core.MatchOwnerMediaSurface(route.method, "/internal"+strings.TrimPrefix(sample, "/api")) {
			t.Errorf("core matcher missed %s %s", route.method, sample)
		}
	}
}

func surfaceTemplate(pattern string) string {
	parts := strings.Split(pattern, "/")
	for i, part := range parts {
		if strings.HasPrefix(part, ":") {
			parts[i] = "{" + strings.TrimPrefix(part, ":") + "}"
		}
	}
	return strings.Join(parts, "/")
}

func mapValue(value any) map[string]any {
	if result, ok := value.(map[string]any); ok {
		return result
	}
	return map[string]any{}
}

func TestStrictSurfaceWritesRejectFactAndHarmFieldsBeforeDispatch(t *testing.T) {
	cases := []struct{ name, path, method, body string }{
		{"photo", "/api/fluctlights/fl-1/wardrobe/items/item-1/photos", http.MethodPost, `{"idempotencyKey":"photo","prompt":"The item by the window","reason":"Owner image request","expectedItemRevision":1,"expectedRevision":1}`},
		{"autonomy", "/api/fluctlights/fl-1/autonomy-policy", http.MethodPut, `{"idempotencyKey":"auto","expectedRevision":0,"reason":"Owner preference","policySource":"override","mode":"paused","permanentHarm":true}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			backend := &fakeBackend{}
			response := surfaceInvoke(newBrowserTestHandler(backend), tc.method, tc.path, tc.body, true, true)
			if response.code != 400 || response.json(t)["code"] != "invalid_request" || len(backend.calls) != 0 {
				t.Fatalf("forbidden write escaped validation: %d %s calls=%v", response.code, response.body, backend.calls)
			}
		})
	}
}
