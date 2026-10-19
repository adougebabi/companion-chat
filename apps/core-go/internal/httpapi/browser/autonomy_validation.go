package browser

import "time"

func validAutonomyPolicyBody(body map[string]any) bool {
	if !onlyKeys(body, "policySource", "mode", "allowedActions", "budgetRemaining", "quietHours", "concurrencyLimit", "cooldownSeconds", "outreachPaused", "notificationsMuted", "confirmActionTypes", "expectedRevision", "reason", "idempotencyKey") {
		return false
	}
	if body["policySource"] != "inherit" && body["policySource"] != "override" {
		return false
	}
	if !validateString(body["reason"], 1, 1000) || !validateString(body["idempotencyKey"], 1, 256) {
		return false
	}
	for key, max := range map[string]float64{"expectedRevision": 2147483647, "concurrencyLimit": 32, "cooldownSeconds": 604800, "budgetRemaining": 1000000} {
		value, exists := body[key]
		if !exists {
			if key == "expectedRevision" {
				return false
			}
			continue
		}
		n, ok := value.(float64)
		if !ok || n < 0 || n > max || n != float64(int64(n)) || (key == "concurrencyLimit" && n == 0) {
			return false
		}
	}
	if value, exists := body["mode"]; exists && value != "active" && value != "paused" && value != "muted" {
		return false
	}
	for _, key := range []string{"outreachPaused", "notificationsMuted"} {
		if value, exists := body[key]; exists {
			if _, ok := value.(bool); !ok {
				return false
			}
		}
	}
	for _, key := range []string{"allowedActions", "confirmActionTypes"} {
		if value, exists := body[key]; exists {
			values, ok := value.([]any)
			if !ok || len(values) > 4 {
				return false
			}
			seen := map[string]bool{}
			for _, v := range values {
				name, ok := v.(string)
				if !ok || (name != "proactive_message" && name != "moment" && name != "capability" && name != "media_request") || seen[name] {
					return false
				}
				seen[name] = true
			}
		}
	}
	if value, exists := body["quietHours"]; exists {
		quiet, ok := value.(map[string]any)
		if !ok {
			return false
		}
		if len(quiet) > 0 {
			if len(quiet) != 3 {
				return false
			}
			for _, key := range []string{"start", "end"} {
				s, ok := quiet[key].(string)
				if !ok || len(s) != 5 {
					return false
				}
				if _, err := time.Parse("15:04", s); err != nil {
					return false
				}
			}
			zone, ok := quiet["timezone"].(string)
			if !ok || zone == "" {
				return false
			}
			if _, err := time.LoadLocation(zone); err != nil {
				return false
			}
		}
	}
	return true
}
