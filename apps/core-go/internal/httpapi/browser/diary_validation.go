package browser

import (
	"strings"
	"time"
)

func validDiarySurfaceBody(method, path string, body map[string]any) bool {
	if !strings.Contains(path, "/diary/") {
		return true
	}
	validRevision := func(required bool) bool {
		v, exists := body["expectedRevision"]
		if !exists {
			return !required
		}
		n, ok := v.(float64)
		return ok && n >= 0 && n <= 2147483647 && n == float64(int64(n))
	}
	if strings.HasSuffix(path, "/generate") {
		if !onlyKeys(body, "localDate", "timezone", "expectedRevision", "reason", "idempotencyKey") || !validateString(body["localDate"], 10, 10) || !validateString(body["reason"], 1, 1000) || !validateString(body["idempotencyKey"], 1, 256) || !validRevision(false) {
			return false
		}
		if _, err := time.Parse("2006-01-02", body["localDate"].(string)); err != nil {
			return false
		}
		if value, exists := body["timezone"]; exists {
			zone, ok := value.(string)
			if !ok || zone == "" {
				return false
			}
			if _, err := time.LoadLocation(zone); err != nil {
				return false
			}
		}
	}
	if strings.HasSuffix(path, "/annotation") {
		_, flag := body["favorite"].(bool)
		note, text := body["note"].(string)
		if !onlyKeys(body, "favorite", "note", "expectedRevision", "idempotencyKey") || !flag || !text || len([]rune(note)) > 5000 || !validRevision(true) || !validateString(body["idempotencyKey"], 1, 256) {
			return false
		}
	}
	if strings.HasSuffix(path, "/replies") && !onlyKeys(body, "text", "expectedRevision", "idempotencyKey") {
		return false
	}
	if strings.HasSuffix(path, "/replies") && (!validRevision(true) || !validateString(body["text"], 1, 2000) || !validateString(body["idempotencyKey"], 1, 256)) {
		return false
	}
	return true
}
