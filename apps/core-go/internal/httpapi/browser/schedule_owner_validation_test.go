package browser

import (
	"net/url"
	"testing"
)

func TestOwnerScheduleBoundaryKeepsLinksAndTimezoneServerOwned(t *testing.T) {
	body := map[string]any{"localDate": "2026-10-22", "expectedRevision": float64(0), "expectedLifeContextRevision": "opaque-life", "idempotencyKey": "original", "reason": "安排缺失日程"}
	if !validScheduleOwnerCommand(body) {
		t.Fatal("valid request rejected")
	}
	for _, key := range []string{"timezone", "intentionId", "actionPlan", "items", "ownerId", "promise_request_id"} {
		copy := map[string]any{}
		for k, v := range body {
			copy[k] = v
		}
		copy[key] = "forged"
		if validScheduleOwnerCommand(copy) {
			t.Fatal("forged field", key)
		}
	}
	body["promiseId"] = "accepted-request"
	body["expectedPromiseRevision"] = float64(2)
	body["startAt"] = "2026-10-22T15:00:00+08:00"
	body["endAt"] = "2026-10-22T16:00:00+08:00"
	if !validScheduleOwnerCommand(body) {
		t.Fatal("valid promise request rejected")
	}
	body["startAt"] = "2026-10-22T15:00"
	if validScheduleOwnerCommand(body) {
		t.Fatal("offsetless time accepted")
	}
	for _, q := range []url.Values{{"from": {"2026-02-30"}}, {"from": {"2026-10-22", "2026-10-23"}}, {"timezone": {"UTC"}}} {
		if validScheduleQuery(q) {
			t.Fatal("invalid query", q)
		}
	}
}
