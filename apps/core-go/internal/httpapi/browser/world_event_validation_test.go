package browser

import (
	"net/http"
	"testing"
)

func TestWorldEventTransportDoesNotAcceptRuntimeEffectsOrAlternateActor(t *testing.T) {
	path := "/api/fluctlights/fl/world/templates/template/runs"
	body := map[string]any{"expectedRevision": float64(2), "reason": "explicit trigger", "idempotencyKey": "event-key", "scheduledAt": "2026-10-20T09:00:00+08:00"}
	if !validWorldSurfaceBody(http.MethodPost, path, body) {
		t.Fatal("valid event trigger rejected")
	}
	for _, key := range []string{"effects", "definition", "actorId", "bodyEffect", "permanentHarm"} {
		body[key] = true
		if validWorldSurfaceBody(http.MethodPost, path, body) {
			t.Fatal("unexpected runtime authority accepted", key)
		}
		delete(body, key)
	}
	body["expectedRevision"] = "2"
	if validWorldSurfaceBody(http.MethodPost, path, body) {
		t.Fatal("string CAS accepted")
	}
}
