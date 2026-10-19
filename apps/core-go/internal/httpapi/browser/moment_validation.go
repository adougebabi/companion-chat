package browser

import "strings"

func validMomentFeedBody(path string, b map[string]any) bool {
	if !strings.HasPrefix(path, "/api/moment-feed/") {
		return true
	}
	if !validateInteger(b["expectedRevision"], 1) || !validateString(b["reason"], 1, 1000) || !validateString(b["idempotencyKey"], 1, 256) {
		return false
	}
	switch {
	case strings.HasSuffix(path, "/comments"):
		return onlyKeys(b, "expectedRevision", "reason", "idempotencyKey", "text") && validateString(b["text"], 1, 2000)
	case strings.HasSuffix(path, "/reaction"):
		return onlyKeys(b, "expectedRevision", "reason", "idempotencyKey", "kind") && (b["kind"] == "like" || b["kind"] == "care" || b["kind"] == "celebrate" || b["kind"] == "none")
	default:
		return onlyKeys(b, "expectedRevision", "reason", "idempotencyKey")
	}
}
