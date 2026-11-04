package browser

import "strings"

func validVisualBrief(body map[string]any) bool {
	if !onlyKeys(body, "appearance", "clothing", "constraints", "references", "expectedRevision", "reason", "idempotencyKey") || !validateRevisionReason(body) || !validateString(body["idempotencyKey"], 1, 256) {
		return false
	}
	for _, key := range []string{"appearance", "clothing", "constraints", "references"} {
		if !validateString(body[key], 0, 1000) {
			return false
		}
	}
	return true
}
func validVisualGeneration(body map[string]any) bool {
	if value, present := body["supersedeSessionId"]; present && !validateString(value, 1, 128) {
		return false
	}
	return onlyKeys(body, "expectedBriefRevision", "expectedVisualRevision", "reason", "idempotencyKey", "supersedeSessionId") && validateInteger(body["expectedBriefRevision"], 0) && validateInteger(body["expectedVisualRevision"], 0) && validateString(body["reason"], 1, 1024) && validateString(body["idempotencyKey"], 1, 256)
}

func validManualVisualReview(body map[string]any) bool {
	if !onlyKeys(body, "expectedAttempt", "expectedVisualRevision", "decision", "identityMatch", "confidence", "observations", "missingSections", "summary", "feedback", "reason", "idempotencyKey") || !validateInteger(body["expectedAttempt"], 1) || !validateInteger(body["expectedVisualRevision"], 0) || !validateString(body["idempotencyKey"], 1, 256) {
		return false
	}
	for _, key := range []string{"identityMatch", "confidence"} {
		if value, present := body[key]; present && !validateNumber(value, 0, 1) {
			return false
		}
	}
	for key, max := range map[string]int{"summary": 1000, "feedback": 2000, "reason": 1024} {
		if value, present := body[key]; present && !validateString(value, 0, max) {
			return false
		}
	}
	for key, limits := range map[string][2]int{"observations": {24, 500}, "missingSections": {16, 200}} {
		value, present := body[key]
		if !present {
			continue
		}
		if !isStringArray(value) || len(array(value)) > limits[0] {
			return false
		}
		for _, item := range array(value) {
			if !validateString(item, 1, limits[1]) || strings.TrimSpace(stringValue(item)) == "" {
				return false
			}
		}
	}
	decision := stringValue(body["decision"])
	if decision == "accepted" {
		return len(array(body["missingSections"])) == 0
	}
	return decision == "regenerate" && (strings.TrimSpace(stringValue(body["feedback"])) != "" || strings.TrimSpace(stringValue(body["summary"])) != "" || len(array(body["observations"])) > 0 || len(array(body["missingSections"])) > 0)
}
