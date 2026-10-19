package browser

import "time"

func validPromiseSurfaceBody(path string, body map[string]any) bool {
	parts := splitPath(path)
	if len(parts) < 4 || parts[0] != "api" || parts[1] != "fluctlights" || parts[3] != "promises" {
		return true
	}
	if !validateString(body["reason"], 1, 500) || !validateString(body["idempotencyKey"], 1, 256) {
		return false
	}
	if len(parts) == 4 {
		if !onlyKeys(body, "title", "dueAt", "startsAt", "reason", "idempotencyKey") || !validateString(body["title"], 1, 2000) {
			return false
		}
	} else {
		if len(parts) != 6 || !validateInteger(body["expectedRevision"], 1) {
			return false
		}
		if parts[5] == "cancel" || parts[5] == "retry" {
			return onlyKeys(body, "expectedRevision", "reason", "idempotencyKey")
		}
		if parts[5] != "reschedule" || !onlyKeys(body, "dueAt", "startsAt", "expectedRevision", "reason", "idempotencyKey") {
			return false
		}
	}
	end, err := time.Parse(time.RFC3339Nano, stringValue(body["dueAt"]))
	if err != nil {
		return false
	}
	if raw, present := body["startsAt"]; present {
		start, err := time.Parse(time.RFC3339Nano, stringValue(raw))
		if err != nil || !end.After(start) {
			return false
		}
	}
	return true
}
