package browser

import (
	"net/http"
	"testing"
)

func TestScheduleItemTransportPreservesOpaqueLifeRevisionAndTime(t *testing.T) {
	body := map[string]any{"expectedRevision": float64(1), "expectedLifeContextRevision": "life_ctx_token", "idempotencyKey": "request", "reason": "test", "activity": "阅读", "startAt": "2026-10-17T15:00:00+08:00", "endAt": "2026-10-17T16:00:00+08:00", "evidenceRefs": []any{"owner:test"}, "placeId": "stable-place"}
	path := "/api/fluctlights/fl/schedules/sc/items"
	if !validLifeSurfaceBody(http.MethodPost, path, body) {
		t.Fatal("valid request rejected")
	}
	body["expectedLifeContextRevision"] = 1
	if validLifeSurfaceBody(http.MethodPost, path, body) {
		t.Fatal("numeric life revision accepted")
	}
	body["expectedLifeContextRevision"] = "life_ctx_token"
	body["startAt"] = "2026-10-17 15:00"
	if validLifeSurfaceBody(http.MethodPost, path, body) {
		t.Fatal("unzoned time accepted")
	}
	delete(body, "activity")
	delete(body, "startAt")
	delete(body, "endAt")
	if !validLifeSurfaceBody(http.MethodDelete, path+"/item", body) {
		t.Fatal("delete incorrectly requires create fields")
	}
}
