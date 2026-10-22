package browser

import (
	"net/url"
	"time"
)

func validScheduleQuery(q url.Values) bool {
	for key, values := range q {
		if key != "from" && key != "to" {
			return false
		}
		if len(values) != 1 {
			return false
		}
		if _, err := time.Parse("2006-01-02", values[0]); err != nil {
			return false
		}
	}
	return true
}
func validScheduleOwnerCommand(body map[string]any) bool {
	if !onlyKeys(body, "localDate", "expectedRevision", "expectedLifeContextRevision", "idempotencyKey", "reason", "promiseId", "expectedPromiseRevision", "startAt", "endAt") {
		return false
	}
	if !validateString(body["localDate"], 10, 10) || !validateString(body["reason"], 1, 500) || !validateString(body["idempotencyKey"], 1, 256) || !validateString(body["expectedLifeContextRevision"], 1, 128) {
		return false
	}
	if _, err := time.Parse("2006-01-02", body["localDate"].(string)); err != nil {
		return false
	}
	if !validateInteger(body["expectedRevision"], 0) {
		return false
	}
	if promise, exists := body["promiseId"]; exists {
		if !validateString(promise, 1, 128) {
			return false
		}
		if !validateInteger(body["expectedPromiseRevision"], 1) {
			return false
		}
		startRaw, _ := body["startAt"].(string)
		endRaw, _ := body["endAt"].(string)
		start, e1 := time.Parse(time.RFC3339Nano, startRaw)
		end, e2 := time.Parse(time.RFC3339Nano, endRaw)
		return e1 == nil && e2 == nil && end.After(start)
	}
	_, revision := body["expectedPromiseRevision"]
	_, start := body["startAt"]
	_, end := body["endAt"]
	return !revision && !start && !end
}
