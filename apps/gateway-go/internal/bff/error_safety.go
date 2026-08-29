package bff

import (
	"strings"
	"unicode/utf8"
)

const (
	maxPublicErrorDepth   = 4
	maxPublicErrorEntries = 32
	maxPublicErrorString  = 1024
)

var sensitiveErrorKeys = map[string]struct{}{
	"apikey":          {},
	"authorization":   {},
	"credential":      {},
	"credentials":     {},
	"password":        {},
	"rawprompt":       {},
	"rawresponse":     {},
	"reasoning":       {},
	"hiddenreasoning": {},
	"secret":          {},
	"secrets":         {},
	"sessiontoken":    {},
	"stack":           {},
	"token":           {},
	"traceback":       {},
}

// sanitizePublicErrorDetails keeps useful typed validation data while
// preventing internal credentials, prompts, provider responses, and
// unbounded payloads from crossing the browser boundary.
func sanitizePublicErrorDetails(details map[string]any) map[string]any {
	value, ok := sanitizePublicErrorValue(details, 0)
	if !ok {
		return nil
	}
	result, ok := value.(map[string]any)
	if !ok || len(result) == 0 {
		return nil
	}
	return result
}

func sanitizePublicErrorValue(value any, depth int) (any, bool) {
	if depth > maxPublicErrorDepth {
		return nil, false
	}
	switch current := value.(type) {
	case nil, bool, float64, float32, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return current, true
	case string:
		return truncatePublicErrorString(current), true
	case []any:
		result := make([]any, 0, minInt(len(current), maxPublicErrorEntries))
		for _, child := range current {
			if len(result) == maxPublicErrorEntries {
				break
			}
			if sanitized, ok := sanitizePublicErrorValue(child, depth+1); ok {
				result = append(result, sanitized)
			}
		}
		return result, true
	case map[string]any:
		result := make(map[string]any, minInt(len(current), maxPublicErrorEntries))
		for key, child := range current {
			if len(result) == maxPublicErrorEntries {
				break
			}
			if _, sensitive := sensitiveErrorKeys[normalizeErrorKey(key)]; sensitive {
				continue
			}
			if sanitized, ok := sanitizePublicErrorValue(child, depth+1); ok {
				result[key] = sanitized
			}
		}
		return result, true
	default:
		return nil, false
	}
}

func normalizeErrorKey(value string) string {
	return strings.ToLower(strings.NewReplacer("-", "", "_", "").Replace(value))
}

func truncatePublicErrorString(value string) string {
	if utf8.RuneCountInString(value) <= maxPublicErrorString {
		return value
	}
	runes := []rune(value)
	return string(runes[:maxPublicErrorString])
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}
