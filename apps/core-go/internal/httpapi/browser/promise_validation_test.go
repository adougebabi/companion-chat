package browser

import "testing"

func TestPromiseWindowValidationKeepsExplicitOffsetsAndScope(t *testing.T) {
	base := "/api/fluctlights/fid/promises"
	good := map[string]any{"title": "具体约定", "dueAt": "2026-10-20T15:00:00+08:00", "startsAt": "2026-10-20T14:00:00+08:00", "reason": "测试", "idempotencyKey": "stable"}
	if !validPromiseSurfaceBody(base, good) {
		t.Fatal("valid offset window rejected")
	}
	for _, due := range []string{"2026-10-20T15:00:00", "2026-10-20T13:00:00+08:00", "nonsense"} {
		body := map[string]any{}
		for key, value := range good {
			body[key] = value
		}
		body["dueAt"] = due
		if validPromiseSurfaceBody(base, body) {
			t.Fatal("invalid window accepted", due)
		}
	}
	good["ownerActorId"] = "other-owner"
	if validPromiseSurfaceBody(base, good) {
		t.Fatal("caller-supplied authority accepted")
	}
	if !validPromiseSurfaceBody(base+"/id/cancel", map[string]any{"expectedRevision": float64(1), "reason": "撤回", "idempotencyKey": "cancel"}) {
		t.Fatal("cancel rejected")
	}
}
