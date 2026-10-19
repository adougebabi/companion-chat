package browser

import (
	"net/http"
	"strings"
	"unicode/utf8"
)

// Validate world transport shapes here; ownership, references and semantics stay in Core.
func validWorldSurfaceBody(method, path string, body map[string]any) bool {
	p := splitPath(path)
	if len(p) == 5 && p[0] == "api" && p[1] == "studio" && p[2] == "resources" {
		return validWorldStudioBody(p[4], body)
	}
	if len(p) < 4 || p[0] != "api" || p[1] != "fluctlights" {
		return true
	}
	domain := p[3]
	if domain != "world" && domain != "npcs" {
		return true
	}
	if domain == "world" && len(p) == 6 && p[4] == "items" {
		return method == http.MethodPut && validWorldItemConfig(body)
	}
	if domain == "world" && len(p) == 7 && p[4] == "places" && p[6] == "defaults" {
		if !onlyKeys(body, "worldId", "expectedRevision", "expectedDefaultRevision", "operation", "source", "reason", "idempotencyKey") || !validateInteger(body["expectedRevision"], 1) || !validateInteger(body["expectedDefaultRevision"], 0) {
			return false
		}
		for key, max := range map[string]int{"worldId": 128, "reason": 1000, "idempotencyKey": 256} {
			if !validateString(body[key], 1, max) || strings.TrimSpace(stringValue(body[key])) == "" {
				return false
			}
		}
		operation, source := stringValue(body["operation"]), stringValue(body["source"])
		return operation == "restore" && source == "recorded" || operation == "capture" && (source == "current_saved" || source == "current_definition")
	}
	if len(p) > 4 && p[4] == "templates" {
		allowed := map[string]bool{"expectedRevision": true, "reason": true, "idempotencyKey": true}
		if method == http.MethodPut {
			allowed["enabled"] = true
		} else {
			allowed["scheduledAt"] = true
			allowed["expectedRunRevision"] = true
		}
		for k := range body {
			if !allowed[k] {
				return false
			}
		}
		if !worldRevision(body["expectedRevision"]) {
			return false
		}
		for k, max := range map[string]int{"reason": 1000, "idempotencyKey": 256} {
			s, ok := body[k].(string)
			if !ok || strings.TrimSpace(s) == "" || utf8.RuneCountInString(s) > max {
				return false
			}
		}
		if method == http.MethodPut {
			_, ok := body["enabled"].(bool)
			return ok
		}
		if v, ok := body["expectedRunRevision"]; ok && !worldRevision(v) {
			return false
		}
		if v, ok := body["scheduledAt"]; ok {
			s, ok := v.(string)
			if !ok || len(s) > 128 {
				return false
			}
		}
		return true
	}
	allowed := map[string]bool{"reason": true, "idempotencyKey": true, "expectedRevision": true}
	fields := []string{}
	if domain == "world" {
		if len(p) > 4 && p[4] == "active" {
			fields = []string{"worldId"}
		} else if len(p) > 4 && p[4] == "places" {
			fields = []string{"worldId", "name", "description", "parentPlaceId", "status", "alias", "group", "floor", "scene", "atmosphere", "furniture", "layout", "connections", "connectionPlaceIds", "roomItems"}
		} else {
			fields = []string{"name", "background", "rules", "description", "flavor", "itemPool"}
		}
	} else if len(p) > 5 && p[5] == "encounters" {
		fields = []string{"confirmed", "summary", "startAt", "endAt", "placeId", "evidenceRefs", "expectedLifeContextRevision"}
	} else {
		fields = []string{"name", "role", "background", "homePlaceId", "routine", "status", "enabled", "positionMode"}
	}
	for _, key := range fields {
		allowed[key] = true
	}
	for key := range body {
		if !allowed[key] {
			return false
		}
	}
	text := func(key string, max int, required bool) bool {
		v, ok := body[key]
		if !ok {
			return !required
		}
		s, ok := v.(string)
		return ok && utf8.RuneCountInString(s) <= max && (!required || strings.TrimSpace(s) != "")
	}
	if !text("reason", 1000, true) || !text("idempotencyKey", 256, true) {
		return false
	}
	if revision, ok := body["expectedRevision"]; ok {
		if !worldRevision(revision) {
			return false
		}
	} else if !(domain == "npcs" && len(p) > 5 && p[5] == "encounters") && method != http.MethodPost {
		return false
	}
	for key, limit := range map[string]int{"name": 256, "role": 512, "background": 16000, "description": 8000, "rules": 8000, "flavor": 1000, "alias": 256, "group": 256, "floor": 128, "scene": 1000, "atmosphere": 2000, "layout": 8000, "summary": 2000, "startAt": 128, "endAt": 128, "positionMode": 32, "worldId": 128, "status": 16} {
		if !text(key, limit, false) {
			return false
		}
	}
	for _, key := range []string{"homePlaceId", "parentPlaceId"} {
		if value, ok := body[key]; ok && value != nil {
			if !text(key, 128, false) {
				return false
			}
		}
	}
	if len(p) > 4 && p[4] == "active" && !text("worldId", 128, true) {
		return false
	}
	if domain == "npcs" && len(p) > 5 && p[5] == "encounters" {
		if body["confirmed"] != true || !text("summary", 2000, true) || !text("startAt", 128, true) || !text("endAt", 128, true) || !text("placeId", 128, true) || !text("expectedLifeContextRevision", 256, true) {
			return false
		}
		refs, ok := body["evidenceRefs"].([]any)
		if !ok || len(refs) == 0 || len(refs) > 32 {
			return false
		}
		for _, ref := range refs {
			v, ok := ref.(string)
			if !ok || v == "" || len(v) > 256 {
				return false
			}
		}
	}
	if value, ok := body["positionMode"]; ok && value != "routine" && value != "fixed" && value != "unknown" {
		return false
	}
	if value, ok := body["status"]; ok && value != "active" && value != "archived" {
		return false
	}

	if method != http.MethodDelete && !(len(p) > 4 && p[4] == "active") && !(domain == "npcs" && len(p) > 5) {
		if !text("name", 256, true) {
			return false
		}
	}
	for _, key := range []string{"enabled", "confirmed"} {
		if value, ok := body[key]; ok {
			if _, valid := value.(bool); !valid {
				return false
			}
		}
	}
	for _, key := range []string{"itemPool", "furniture", "connections", "roomItems"} {
		if value, ok := body[key]; ok {
			switch v := value.(type) {
			case string:
				if utf8.RuneCountInString(v) > 16000 {
					return false
				}
			case []any:
				if len(v) > 200 {
					return false
				}
			default:
				return false
			}
		}
	}
	for _, key := range []string{"connectionPlaceIds", "routine", "evidenceRefs"} {
		if value, ok := body[key]; ok {
			if arr, valid := value.([]any); !valid || len(arr) > 200 {
				return false
			}
		}
	}
	return true
}

func worldRevision(value any) bool {
	v, ok := value.(float64)
	return ok && v >= 0 && v <= 2147483647 && v == float64(int64(v))
}

func validWorldStudioBody(operation string, body map[string]any) bool {
	if operation != "publish" && operation != "apply" && operation != "apply-preview" {
		return true
	}
	allowed := map[string]bool{"reason": true, "idempotencyKey": true, "expectedRevision": true}
	if operation != "publish" {
		for _, key := range []string{"fluctlightId", "resourceVersion", "expectedInstanceRevision", "expectedRuntimeRevision", "homePlaceId", "parentPlaceId", "routine", "choices"} {
			allowed[key] = true
		}
	}
	for key, value := range body {
		if !allowed[key] {
			return false
		}
		switch key {
		case "expectedRevision", "expectedInstanceRevision", "expectedRuntimeRevision":
			if !worldRevision(value) {
				return false
			}
		case "choices":
			choices, ok := value.([]any)
			if !ok || len(choices) > 64 {
				return false
			}
			for _, raw := range choices {
				choice, ok := raw.(map[string]any)
				if !ok || len(choice) != 2 {
					return false
				}
				field, ok := choice["field"].(string)
				if !ok || field == "" || len(field) > 128 {
					return false
				}
				if choice["decision"] != "keep" && choice["decision"] != "override" {
					return false
				}
			}
		case "routine":
			items, ok := value.([]any)
			if !ok || len(items) > 42 {
				return false
			}
		default:
			if value == nil && (key == "homePlaceId" || key == "parentPlaceId") {
				continue
			}
			v, ok := value.(string)
			if !ok || len([]rune(v)) > 1000 {
				return false
			}
		}
	}
	if operation == "publish" {
		_, ok := body["expectedRevision"]
		key, k := body["idempotencyKey"].(string)
		reason, r := body["reason"].(string)
		return ok && k && r && key != "" && reason != ""
	}
	return true
}
