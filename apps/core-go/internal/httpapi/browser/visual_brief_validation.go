package browser

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
	if !onlyKeys(body, "expectedAttempt", "expectedVisualRevision", "decision", "identityMatch", "confidence", "observations", "missingSections", "summary", "feedback", "reason", "idempotencyKey") || !validateInteger(body["expectedAttempt"], 1) || !validateInteger(body["expectedVisualRevision"], 0) || !validateString(body["summary"], 1, 1000) || !validateString(body["feedback"], 0, 2000) || !validateString(body["reason"], 1, 1024) || !validateString(body["idempotencyKey"], 1, 256) {
		return false
	}
	for _, key := range []string{"identityMatch", "confidence"} {
		value, ok := body[key].(float64)
		if !ok || value < 0 || value > 1 {
			return false
		}
	}
	if !isStringArray(body["observations"]) || len(array(body["observations"])) < 1 || len(array(body["observations"])) > 24 || !isStringArray(body["missingSections"]) || len(array(body["missingSections"])) > 16 {
		return false
	}
	for _, value := range array(body["observations"]) {
		if !validateString(value, 1, 500) {
			return false
		}
	}
	for _, value := range array(body["missingSections"]) {
		if !validateString(value, 1, 200) {
			return false
		}
	}
	decision := stringValue(body["decision"])
	return (decision == "accepted" && len(array(body["missingSections"])) == 0) || decision == "regenerate"
}
