package browser

import (
	"net/http"
	"strings"
	"time"
)

func validLifeSurfaceBody(method, path string, body map[string]any) bool {
	parts := splitPath(path)
	if len(parts) < 6 || parts[0] != "api" || parts[1] != "fluctlights" || parts[3] != "schedules" || parts[5] != "items" {
		return true
	}
	if !validateInteger(body["expectedRevision"], 1) || !validateLifeContextCommandIdentity(body) || !validateString(body["reason"], 1, 500) {
		return false
	}
	if method == http.MethodDelete {
		return true
	}
	if !validateString(body["activity"], 1, 512) || !validateEvidence(body["evidenceRefs"]) {
		return false
	}
	start, e1 := time.Parse(time.RFC3339Nano, stringValue(body["startAt"]))
	end, e2 := time.Parse(time.RFC3339Nano, stringValue(body["endAt"]))
	if e1 != nil || e2 != nil || !end.After(start) {
		return false
	}
	for _, field := range []string{"scene", "location"} {
		if raw, ok := body[field]; ok && !validateString(raw, 0, 512) {
			return false
		}
	}
	if raw, ok := body["placeId"]; ok {
		if !validateString(raw, 0, 128) || strings.TrimSpace(stringValue(raw)) != raw {
			return false
		}
	}
	return true
}
