package browser

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type browserRouteCase struct {
	name     string
	method   string
	path     string
	body     string
	statuses []int
}

func TestBrowserScheduleWriteCannotCreateExecutableActionLink(t *testing.T) {
	request := map[string]any{
		"localDate": "2026-09-27", "timezone": "Asia/Shanghai",
		"expectedRevision": float64(0), "expectedLifeContextRevision": "life_ctx_0123456789abcdef0123456789abcdef", "idempotencyKey": "plain-schedule",
		"evidenceRefs": []any{"owner:plain-schedule"},
		"items":        []any{map[string]any{"startAt": "2026-09-27T10:00:00+08:00", "endAt": "2026-09-27T11:00:00+08:00", "activity": "阅读", "scene": "书房"}},
	}
	if !validateSchedule(request) {
		t.Fatal("ordinary descriptive schedule item was rejected")
	}
	item := request["items"].([]any)[0].(map[string]any)
	item["intentionId"] = "intention-1"
	item["actionPlan"] = map[string]any{"capability": "life.activity.start", "kind": "hair_dye"}
	if validateSchedule(request) {
		t.Fatal("browser schedule write accepted an executable action link")
	}
}

func browserRouteCases() []browserRouteCase {
	return append([]browserRouteCase{
		{name: "schedule local calendar", method: http.MethodGet, path: "/api/fluctlights/fl-1/schedules?from=2026-10-22&to=2026-10-23"},
		{name: "schedule original request", method: http.MethodGet, path: "/api/fluctlights/fl-1/schedules/requests/request-1"},
		{name: "request non-overwriting schedule", method: http.MethodPost, path: "/api/fluctlights/fl-1/schedules/requests", body: `{"localDate":"2026-10-22","expectedRevision":0,"expectedLifeContextRevision":"life_ctx_original","reason":"missing day","idempotencyKey":"schedule-original"}`},
		{name: "publication versions", method: http.MethodGet, path: "/api/studio/resources/res-1/versions"},
		{name: "copy resource", method: http.MethodPost, path: "/api/studio/resources/res-1/copy", body: `{"expectedRevision":1,"source":"published","sourceVersion":"1","name":"Copy","reason":"Owner test","idempotencyKey":"copy-test"}`},
		{name: "archive resource", method: http.MethodDelete, path: "/api/studio/resources/res-1", body: `{"expectedRevision":1,"reason":"Owner test","idempotencyKey":"archive-test"}`},
		{name: "restore resource", method: http.MethodPost, path: "/api/studio/resources/res-1/restore", body: `{"expectedRevision":1,"reason":"Owner test","idempotencyKey":"restore-test"}`},
		{name: "exact Studio publication", method: http.MethodGet, path: "/api/studio/resources/res-1/versions/1"},
		{name: "creation from template", method: http.MethodPost, path: "/api/fluctlight-creations/analysis", body: `{"resourceId":"res-real","resourceVersion":"1"}`},
		{name: "managed memory list", method: http.MethodGet, path: "/api/fluctlights/fl-1/memories?limit=30"},
		{name: "managed memory detail", method: http.MethodGet, path: "/api/memories/memory-1"},
		{name: "memory revisions", method: http.MethodGet, path: "/api/memories/memory-1/revisions"},
		{name: "memory sources", method: http.MethodGet, path: "/api/memories/memory-1/sources?revision=0"},
		{name: "memory compensation rollback", method: http.MethodPost, path: "/api/memories/memory-1/rollback", body: `{"expectedRevision":2,"targetRevision":0,"evidenceRefs":["owner:confirmation"]}`},
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
		{name: "Kev diagnostics", method: http.MethodGet, path: "/api/diagnostics/kev-decisions"},
		{name: "Vision test", method: http.MethodPost, path: "/api/settings/vision/test", body: `{}`},
		{name: "Kev connection", method: http.MethodPost, path: "/api/settings/kev/test-connection", body: `{}`},
		{name: "settings get", method: http.MethodGet, path: "/api/settings"},
		{name: "settings put", method: http.MethodPut, path: "/api/settings", body: `{"values":{},"secrets":{},"clearSecrets":[],"expectedVersions":{},"idempotencyKey":"settings-test"}`},
		{name: "capability request list", method: http.MethodGet, path: "/api/capability-requests"},
		{name: "capability request review", method: http.MethodPost, path: "/api/capability-requests/request-1/review", body: `{"status":"accepted","note":"reviewed","expectedRevision":1,"idempotencyKey":"cap-review-1"}`},
		{name: "provider endpoint put", method: http.MethodPut, path: "/api/providers/endpoints", body: `{"endpointId":"endpoint","kind":"openai","baseUrl":"http://provider","secretPurpose":"chat","expectedVersions":{"endpoint:endpoint":0},"idempotencyKey":"provider-endpoint-1"}`},
		{name: "provider endpoint get", method: http.MethodGet, path: "/api/providers/endpoints"},
		{name: "provider models", method: http.MethodGet, path: "/api/providers/endpoints/endpoint/models"},
		{name: "provider bindings", method: http.MethodGet, path: "/api/providers"},
		{name: "provider role", method: http.MethodPut, path: "/api/providers/roles", body: `{"role":"conversation_reply","endpointId":"endpoint","modelId":"model","tokenBudget":100,"timeoutSeconds":10,"expectedVersions":{},"idempotencyKey":"provider-role-1"}`},
		{name: "conversation create", method: http.MethodPost, path: "/api/conversations", body: `{"participantActorIds":["fl-1"]}`},
		{name: "fluctlight create", method: http.MethodPost, path: "/api/fluctlights", body: `{}`},
		{name: "creation analysis", method: http.MethodPost, path: "/api/fluctlight-creations/analysis", body: `{"description":"a new companion"}`},
		{name: "persona preparation recover", method: http.MethodPost, path: "/api/fluctlights/fl-1/persona-preparation/recover", body: `{"expectedRevision":1,"idempotencyKey":"persona-recover-1"}`},
		{name: "creation activation", method: http.MethodPost, path: "/api/fluctlight-creations/activate", body: `{"requestId":"request","initializationMode":"blank_slate","name":"blank"}`},
		{name: "fluctlight list", method: http.MethodGet, path: "/api/fluctlights"},
		{name: "actor group list", method: http.MethodGet, path: "/api/actor-groups"},
		{name: "actor group create", method: http.MethodPost, path: "/api/actor-groups", body: `{"name":"friends"}`},
		{name: "actor group member add", method: http.MethodPost, path: "/api/actor-groups/group/members", body: `{"actorId":"actor"}`},
		{name: "actor group member remove", method: http.MethodDelete, path: "/api/actor-groups/group/members/actor"},
		{name: "fluctlight get", method: http.MethodGet, path: "/api/fluctlights/fl-1"},
		{name: "conversation directory", method: http.MethodGet, path: "/api/fluctlights/fl-1/conversations"},
		{name: "new direct conversation", method: http.MethodPost, path: "/api/fluctlights/fl-1/conversations", body: `{"idempotencyKey":"new-session"}`},
		{name: "select direct conversation", method: http.MethodPost, path: "/api/fluctlights/fl-1/conversations/conv-1/select", body: `{}`},
		{name: "search all conversation messages", method: http.MethodGet, path: "/api/fluctlights/fl-1/messages/search?q=hello"},
		{name: "exact conversation message", method: http.MethodGet, path: "/api/conversations/conv-1/messages/msg-1"},
		{name: "direct conversation", method: http.MethodGet, path: "/api/fluctlights/fl-1/conversation"},
		{name: "fluctlight moments", method: http.MethodGet, path: "/api/fluctlights/fl-1/moments"},
		{name: "fluctlight moments read", method: http.MethodPost, path: "/api/fluctlights/fl-1/moments/read"},
		{name: "global moments", method: http.MethodGet, path: "/api/moments"},
		{name: "actor user background update", method: http.MethodPut, path: "/api/fluctlights/fl-1/actor-user-background", body: `{"background":{"name":"Fixture Owner","occupation":"编辑","background":"验收样本","interests":"绘画","preferences":null,"location_scope":"同城","location":"上海","timezone":null,"relationship_distance":"线上交流","meeting_confirmed":false},"operation":"correct","reason":"initial settings","idempotencyKey":"user-1","expectedCurrentFactsRevision":"facts_gen_1"}`},
		{name: "goal set read", method: http.MethodGet, path: "/api/fluctlights/fl-1/goal-set"},
		{name: "goal set save", method: http.MethodPut, path: "/api/fluctlights/fl-1/goal-set", body: `{"expectedVersion":1,"expectedFactsRevision":"facts","idempotencyKey":"set-key","reason":"Owner","orderingMode":"manual","order":[]}`},
		{name: "goal planning history", method: http.MethodGet, path: "/api/fluctlights/fl-1/goal-planning"},
		{name: "goal planning request", method: http.MethodPost, path: "/api/fluctlights/fl-1/goal-planning", body: `{"idempotencyKey":"request-key"}`},
		{name: "actor context read", method: http.MethodGet, path: "/api/fluctlights/fl-1/actor-context/actor-1"},
		{name: "actor context write", method: http.MethodPut, path: "/api/fluctlights/fl-1/actor-context/actor-1", body: `{"expectedContextVersion":"v1","idempotencyKey":"actor-key","operation":"change","reason":"Owner","background":{"location":"unknown"}}`},
		{name: "goal list", method: http.MethodGet, path: "/api/fluctlights/fl-1/goals"},
		{name: "goal create", method: http.MethodPost, path: "/api/fluctlights/fl-1/goals", body: `{"desiredOutcome":"result","successCriteria":["fact"],"motivation":"need","reason":"create","idempotencyKey":"goal-create"}`},
		{name: "goal detail", method: http.MethodGet, path: "/api/fluctlights/fl-1/goals/goal-1"},
		{name: "goal update", method: http.MethodPut, path: "/api/fluctlights/fl-1/goals/goal-1", body: `{"expectedRevision":1,"reason":"edit","idempotencyKey":"goal-edit"}`},
		{name: "goal history", method: http.MethodGet, path: "/api/fluctlights/fl-1/goals/goal-1/history"},
		{name: "goal evidence", method: http.MethodGet, path: "/api/fluctlights/fl-1/goals/goal-1/evidence"},
		{name: "goal pause", method: http.MethodPost, path: "/api/fluctlights/fl-1/goals/goal-1/pause", body: `{"expectedRevision":1,"reason":"govern","idempotencyKey":"goal-pause"}`},
		{name: "goal resume", method: http.MethodPost, path: "/api/fluctlights/fl-1/goals/goal-1/resume", body: `{"expectedRevision":1,"reason":"govern","idempotencyKey":"goal-resume"}`},
		{name: "goal cancel", method: http.MethodPost, path: "/api/fluctlights/fl-1/goals/goal-1/cancel", body: `{"expectedRevision":1,"reason":"govern","idempotencyKey":"goal-cancel"}`},
		{name: "goal abandon", method: http.MethodPost, path: "/api/fluctlights/fl-1/goals/goal-1/abandon", body: `{"expectedRevision":1,"reason":"govern","idempotencyKey":"goal-abandon"}`},
		{name: "goal reassess", method: http.MethodPost, path: "/api/fluctlights/fl-1/goals/goal-1/reassess", body: `{"expectedRevision":1,"reason":"govern","idempotencyKey":"goal-reassess"}`},
		{name: "fluctlight detail", method: http.MethodGet, path: "/api/fluctlights/fl-1/detail"},
		{name: "fluctlight wardrobe", method: http.MethodGet, path: "/api/fluctlights/fl-1/wardrobe"},
		{name: "fluctlight wardrobe add items", method: http.MethodPost, path: "/api/fluctlights/fl-1/wardrobe/items", body: `{"items":[]}`},
		{name: "fluctlight wardrobe update item", method: http.MethodPut, path: "/api/fluctlights/fl-1/wardrobe/items/item-1", body: `{"description":"test"}`},
		{name: "fluctlight wardrobe delete item", method: http.MethodDelete, path: "/api/fluctlights/fl-1/wardrobe/items/item-1"},
		{name: "fluctlight wake-up", method: http.MethodPost, path: "/api/fluctlights/fl-1/wake-up", body: `{}`},
		{name: "developing self", method: http.MethodGet, path: "/api/fluctlights/fl-1/developing-self"},
		{name: "developing self rollback", method: http.MethodPost, path: "/api/fluctlights/fl-1/developing-self/claim-1/rollback", body: `{"expectedRevision":1,"reason":"test"}`},
		{name: "developing self forget", method: http.MethodPost, path: "/api/fluctlights/fl-1/developing-self/claim-1/forget", body: `{"expectedRevision":1,"reason":"test"}`},
		{name: "fluctlight status", method: http.MethodPut, path: "/api/fluctlights/fl-1/status", body: `{"status":"active","expectedRevision":0,"reason":"test"}`},
		{name: "fluctlight retire", method: http.MethodPost, path: "/api/fluctlights/fl-1/retire", body: `{"expectedRevision":0,"reason":"test"}`},
		{name: "foundation revision", method: http.MethodPost, path: "/api/fluctlights/fl-1/foundation-revisions", body: `{"changes":{"name":"new"},"expectedRevision":0,"reason":"test","idempotencyKey":"foundation-proposal-1"}`},
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
		{name: "conversation turn cancel", method: http.MethodPost, path: "/api/conversations/conversation-1/turn/turn-1/cancel"},
		{name: "diagnostics", method: http.MethodGet, path: "/api/diagnostics"},
		{name: "lifecycle diagnostics", method: http.MethodGet, path: "/api/diagnostics/lifecycle"},
		{name: "diagnostics clear", method: http.MethodDelete, path: "/api/diagnostics"},
		{name: "diagnostic model runs", method: http.MethodGet, path: "/api/diagnostics/model-runs"},
		{name: "diagnostic agent runs", method: http.MethodGet, path: "/api/diagnostics/agent-runs"},
		{name: "diagnostic media prompts", method: http.MethodGet, path: "/api/diagnostics/media-prompts"},
		{name: "diagnostic media prompt retry", method: http.MethodPost, path: "/api/diagnostics/media-prompts/media-1/retry"},
		{name: "diagnostics export", method: http.MethodGet, path: "/api/diagnostics/export"},
		{name: "media", method: http.MethodGet, path: "/api/media/asset-1"},
		{name: "fluctlight media", method: http.MethodGet, path: "/api/fluctlights/fl-1/media"},
		{name: "fluctlight sidebar", method: http.MethodGet, path: "/api/fluctlights/fl-1/sidebar"},
		{name: "fluctlight nearby", method: http.MethodGet, path: "/api/fluctlights/fl-1/nearby"},
		{name: "diary list", method: http.MethodGet, path: "/api/fluctlights/fl-1/diary"},
		{name: "diary create", method: http.MethodPost, path: "/api/fluctlights/fl-1/diary", body: `{"idempotencyKey":"diary-1","body":"今天"}`},
		{name: "diary update", method: http.MethodPut, path: "/api/fluctlights/fl-1/diary/entry-1", body: `{"idempotencyKey":"diary-2","body":"今天","expectedRevision":1,"reason":"修订"}`},

		{name: "world save", method: http.MethodPut, path: "/api/fluctlights/fl-1/world", body: `{"name":"World","reason":"test","idempotencyKey":"world","expectedRevision":0}`},
		{name: "world catalog detail", method: http.MethodGet, path: "/api/fluctlights/fl-1/world/catalog/world-1"},
		{name: "world catalog", method: http.MethodGet, path: "/api/fluctlights/fl-1/world/catalog"},
		{name: "world catalog create", method: http.MethodPost, path: "/api/fluctlights/fl-1/world/catalog", body: `{"name":"World","reason":"test","idempotencyKey":"catalog"}`},
		{name: "world catalog update", method: http.MethodPut, path: "/api/fluctlights/fl-1/world/catalog/world-1", body: `{"name":"World","reason":"test","idempotencyKey":"catalog-edit","expectedRevision":1}`},
		{name: "world catalog delete", method: http.MethodDelete, path: "/api/fluctlights/fl-1/world/catalog/world-1", body: `{"reason":"test","idempotencyKey":"catalog-delete","expectedRevision":1}`},
		{name: "world select", method: http.MethodPut, path: "/api/fluctlights/fl-1/world/active", body: `{"worldId":"world-1","reason":"test","idempotencyKey":"select","expectedRevision":1}`},
		{name: "place create", method: http.MethodPost, path: "/api/fluctlights/fl-1/world/places", body: `{"name":"Place","reason":"test","idempotencyKey":"place","expectedRevision":0}`},
		{name: "place update", method: http.MethodPut, path: "/api/fluctlights/fl-1/world/places/place-1", body: `{"name":"Place","reason":"test","idempotencyKey":"place-edit","expectedRevision":1}`},
		{name: "place delete", method: http.MethodDelete, path: "/api/fluctlights/fl-1/world/places/place-1", body: `{"reason":"test","idempotencyKey":"place-delete","expectedRevision":1}`},
		{name: "NPCs", method: http.MethodGet, path: "/api/fluctlights/fl-1/npcs"},
		{name: "NPC detail", method: http.MethodGet, path: "/api/fluctlights/fl-1/npcs/npc-1"},
		{name: "NPC update", method: http.MethodPut, path: "/api/fluctlights/fl-1/npcs/npc-1", body: `{"name":"NPC","reason":"test","idempotencyKey":"npc-edit","expectedRevision":1}`},
		{name: "NPC delete", method: http.MethodDelete, path: "/api/fluctlights/fl-1/npcs/npc-1", body: `{"reason":"test","idempotencyKey":"npc-delete","expectedRevision":1}`},
		{name: "NPC encounter", method: http.MethodPost, path: "/api/fluctlights/fl-1/npcs/npc-1/encounters", body: `{"confirmed":true,"summary":"Meeting","reason":"test","idempotencyKey":"encounter","startAt":"2026-10-15T00:00:00Z","endAt":"2026-10-15T00:01:00Z","placeId":"place-1","evidenceRefs":["owner_confirmation"],"expectedLifeContextRevision":"revision"}`},
		{name: "resource publish", method: http.MethodPost, path: "/api/studio/resources/resource-1/publish", body: `{"reason":"test","idempotencyKey":"publish","expectedRevision":1}`},
		{name: "world", method: http.MethodGet, path: "/api/fluctlights/fl-1/world"},
		{name: "world templates", method: http.MethodGet, path: "/api/fluctlights/fl-1/world/templates"},
		{name: "world template update", method: http.MethodPut, path: "/api/fluctlights/fl-1/world/templates/template-1", body: `{"idempotencyKey":"tpl-1","enabled":true,"expectedRevision":0,"reason":"偏好"}`},
		{name: "world place", method: http.MethodGet, path: "/api/fluctlights/fl-1/world/places/place-1"},
		{name: "autonomy policy read", method: http.MethodGet, path: "/api/fluctlights/fl-1/autonomy-policy"},
		{name: "autonomy policy update", method: http.MethodPut, path: "/api/fluctlights/fl-1/autonomy-policy", body: `{"idempotencyKey":"auto-1","expectedRevision":0,"reason":"角色覆盖","policySource":"override","mode":"paused"}`},
		{name: "desires", method: http.MethodGet, path: "/api/fluctlights/fl-1/desires"},
		{name: "fluctlight owner requests", method: http.MethodGet, path: "/api/fluctlights/fl-1/owner-requests"},
		{name: "promise", method: http.MethodPost, path: "/api/fluctlights/fl-1/promises", body: `{"idempotencyKey":"promise-1","title":"一起吃饭","startsAt":"2026-10-20T10:00:00Z","dueAt":"2026-10-20T11:00:00Z","reason":"Owner约定"}`},
		{name: "moment suggestion", method: http.MethodPost, path: "/api/fluctlights/fl-1/moment-suggestions", body: `{"idempotencyKey":"suggest-1","text":"今天的光"}`},
		{name: "persona switch", method: http.MethodPost, path: "/api/fluctlights/fl-1/persona-switch", body: `{"idempotencyKey":"persona-1","targetPersonaId":"persona-1","expectedRevision":0,"expectedRuntimeRevision":0,"reason":"切换","confirm":true}`},
		{name: "visual identity", method: http.MethodPut, path: "/api/fluctlights/fl-1/visual-identity", body: `{"idempotencyKey":"visual-1","expectedRevision":0,"reason":"头像","mode":"canonical"}`},
		{name: "wardrobe wear", method: http.MethodPost, path: "/api/fluctlights/fl-1/wardrobe/wear", body: `{"idempotencyKey":"wear-1","itemId":"item-1","slot":"outer"}`},
		{name: "wardrobe use", method: http.MethodPost, path: "/api/fluctlights/fl-1/wardrobe/items/item-1/use", body: `{"idempotencyKey":"use-1","active":true}`},
		{name: "wardrobe photo", method: http.MethodPost, path: "/api/fluctlights/fl-1/wardrobe/items/item-1/photos", body: `{"idempotencyKey":"photo-1","prompt":"只绘制窗边的这件物品","reason":"Owner图片请求","expectedItemRevision":1}`},
		{name: "schedule item create", method: http.MethodPost, path: "/api/fluctlights/fl-1/schedules/schedule-1/items", body: `{"idempotencyKey":"item-1","activity":"阅读","startAt":"2026-01-01T10:00:00Z","endAt":"2026-01-01T11:00:00Z","expectedRevision":1,"expectedLifeContextRevision":"life_ctx_token","evidenceRefs":["owner:test"],"reason":"建议"}`},
		{name: "schedule item update", method: http.MethodPut, path: "/api/fluctlights/fl-1/schedules/schedule-1/items/item-1", body: `{"idempotencyKey":"item-2","activity":"阅读","startAt":"2026-01-01T10:00:00Z","endAt":"2026-01-01T11:00:00Z","expectedRevision":1,"expectedLifeContextRevision":"life_ctx_token","evidenceRefs":["owner:test"],"reason":"改期"}`},
		{name: "schedule item delete", method: http.MethodDelete, path: "/api/fluctlights/fl-1/schedules/schedule-1/items/item-1", body: `{"idempotencyKey":"item-3","expectedRevision":1,"expectedLifeContextRevision":"life_ctx_token","reason":"取消"}`},
		{name: "reflection", method: http.MethodGet, path: "/api/fluctlights/fl-1/reflections/reflection-1"},
		{name: "reflection catalog", method: http.MethodGet, path: "/api/fluctlights/fl-1/reflections"},
		{name: "reflection feedback history", method: http.MethodGet, path: "/api/fluctlights/fl-1/reflections/reflection-1/feedback"},
		{name: "reflection feedback", method: http.MethodPost, path: "/api/fluctlights/fl-1/reflections/reflection-1/feedback", body: `{"idempotencyKey":"reflect-1","text":"这个判断还不准确"}`},
		{name: "notifications", method: http.MethodGet, path: "/api/notifications"},
		{name: "notification unread count", method: http.MethodGet, path: "/api/notifications/unread-count", statuses: []int{http.StatusNotFound}},
		{name: "notifications read all", method: http.MethodPost, path: "/api/notifications/read-all", body: `{"idempotencyKey":"read-all-1","before":"2026-10-16T00:00:00Z"}`},
		{name: "notification read", method: http.MethodPost, path: "/api/notifications/notice-1/read", body: `{"idempotencyKey":"read-1"}`, statuses: []int{http.StatusNotFound}},
		{name: "pending", method: http.MethodGet, path: "/api/pending"},
		{name: "pending count", method: http.MethodGet, path: "/api/pending/count"},
		{name: "studio resources", method: http.MethodGet, path: "/api/studio/resources"},
		{name: "studio resource create", method: http.MethodPost, path: "/api/studio/resources", body: `{"idempotencyKey":"res-1","kind":"place","name":"窗边"}`},
		{name: "studio resource read", method: http.MethodGet, path: "/api/studio/resources/resource-1"},
		{name: "studio resource update", method: http.MethodPut, path: "/api/studio/resources/resource-1", body: `{"idempotencyKey":"res-2","name":"窗边","expectedVersion":"1"}`},
		{name: "studio apply preview", method: http.MethodPost, path: "/api/studio/resources/resource-1/apply-preview", body: `{"fluctlightId":"fl-1"}`},
		{name: "studio apply", method: http.MethodPost, path: "/api/studio/resources/resource-1/apply", body: `{"idempotencyKey":"apply-1","fluctlightId":"fl-1","resourceVersion":"1","reason":"记录"}`},
		{name: "studio previews", method: http.MethodGet, path: "/api/studio/previews"},
		{name: "studio preview create", method: http.MethodPost, path: "/api/studio/previews", body: `{"idempotencyKey":"preview-1","text":"试一试"}`},
		{name: "studio pack import", method: http.MethodPost, path: "/api/studio/packs", body: browserPackFixture(true)},
		{name: "studio pack validate", method: http.MethodPost, path: "/api/studio/packs/validate", body: browserPackFixture(false)},
		{name: "studio pack export", method: http.MethodGet, path: "/api/studio/packs/export"},
		{name: "backups", method: http.MethodGet, path: "/api/ops/backups"},
		{name: "backup create", method: http.MethodPost, path: "/api/ops/backups", body: `{"idempotencyKey":"backup-1","reason":"Owner备份"}`},
		{name: "backup restore", method: http.MethodPost, path: "/api/ops/backups/backup-1/restore", body: `{"idempotencyKey":"backup-2","expectedRevision":1,"targetName":"fluctlight_restore_abcdefgh","confirm":true,"reason":"恢复"}`},
		{name: "harm intercepts", method: http.MethodGet, path: "/api/diagnostics/harm-intercepts"},
		{name: "diagnostic task", method: http.MethodGet, path: "/api/diagnostics/tasks/task-1"},
		{name: "agents", method: http.MethodGet, path: "/api/settings/agents"},
		{name: "memory suggestion", method: http.MethodPost, path: "/api/memories/memory-1/suggestions", body: `{"idempotencyKey":"memory-1","text":"告诉她","expectedRevision":0,"reason":"候选"}`},
		{name: "owner request", method: http.MethodGet, path: "/api/owner-requests/request-1"},
		{name: "owner moment", method: http.MethodPost, path: "/api/moments", body: `{"idempotencyKey":"moment-1","text":"我的动态"}`},
		{name: "media upload", method: http.MethodPost, path: "/api/media", statuses: []int{http.StatusCreated}},
		{name: "actor group delete", method: http.MethodDelete, path: "/api/actor-groups/group-1", body: `{"idempotencyKey":"group-1","expectedRevision":0,"reason":"解散"}`},
	}, browserV3RouteCases()...)
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
			handler := newBrowserTestHandler(&matrixTransportBackend{fakeBackend: backend})
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
			body := route.body
			if route.path == "/api/media" && route.method == http.MethodPost {
				body, headers["Content-Type"] = multipartUploadFixture(t)
			}
			response := invokeBrowser(handler, route.method, "https://api.test"+route.path, body, headers, cookies)
			allowed := route.statuses
			if len(allowed) == 0 {
				allowed = []int{http.StatusOK, http.StatusNoContent, http.StatusPartialContent}
			}
			for _, status := range allowed {
				if response.Code == status {
					return
				}
			}
			t.Fatalf("route returned %d: %s", response.Code, response.Body.String())
		})
	}
}
