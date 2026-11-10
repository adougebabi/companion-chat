package browser

import (
	"net/http"
	"strings"
)

func validExpressionLearningBody(method, path string, body map[string]any) bool {
	if method == http.MethodPut && strings.HasSuffix(path, "/expression-learning/settings") {
		_, learningOK := body["learningEnabled"].(bool)
		_, useOK := body["useEnabled"].(bool)
		return onlyKeys(body, "learningEnabled", "useEnabled", "expectedRevision", "idempotencyKey") && learningOK && useOK && validateInteger(body["expectedRevision"], 0) && validateString(body["idempotencyKey"], 1, 256)
	}
	if method == http.MethodPost && strings.HasSuffix(path, "/expression-learning/extractions") {
		return onlyKeys(body, "sourceMessageId", "idempotencyKey") && validateString(body["sourceMessageId"], 1, 128) && validateString(body["idempotencyKey"], 1, 256)
	}
	if method == http.MethodPost && strings.Contains(path, "/expression-learning/candidates/") {
		if !onlyKeys(body, "expectedRevision", "reason", "triggerText", "idempotencyKey") || !validateInteger(body["expectedRevision"], 1) || !validateString(body["reason"], 1, 1024) || !validateString(body["idempotencyKey"], 1, 256) {
			return false
		}
		if strings.HasSuffix(path, "/enable") {
			return validateString(body["triggerText"], 1, 256)
		}
		if body["triggerText"] != nil && !validateString(body["triggerText"], 0, 256) {
			return false
		}
		return strings.HasSuffix(path, "/disable") || strings.HasSuffix(path, "/revoke")
	}
	return false
}
