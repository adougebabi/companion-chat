package browser

import "strings"

func validSelfCommandBody(path string, body map[string]any) bool {
	if !onlyKeys(body, "expectedRevision", "targetRevision", "reason", "idempotencyKey") || !validateString(body["reason"], 1, 1024) || !validateString(body["idempotencyKey"], 1, 256) {
		return false
	}
	expected, ok := body["expectedRevision"].(float64)
	if !ok || expected < 1 || expected > 2147483647 || expected != float64(int64(expected)) {
		return false
	}
	if strings.HasSuffix(path, "/restore") {
		target, ok := body["targetRevision"].(float64)
		return ok && target >= 1 && target <= 2147483647 && target == float64(int64(target))
	}
	_, target := body["targetRevision"]
	return !target
}
