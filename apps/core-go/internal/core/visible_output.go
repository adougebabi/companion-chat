package core

import (
	"encoding/json"
	"strings"
)

// normalizeVisibleReply removes transport/protocol wrappers that a text-only
// realization model may return despite the plain-text contract. Structured
// action objects are control payloads; only their user-facing content belongs
// in a conversation message.
func normalizeVisibleReply(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	var object map[string]any
	if json.Unmarshal([]byte(trimmed), &object) != nil || object == nil {
		return trimmed
	}
	if action := mapValue(object["action"]); len(action) > 0 {
		for _, key := range []string{"content", "text", "visible_text", "message"} {
			if text := strings.TrimSpace(stringValue(action[key])); text != "" {
				return text
			}
		}
		return ""
	}
	for _, key := range []string{"content", "text", "visible_text", "message"} {
		if text := strings.TrimSpace(stringValue(object[key])); text != "" {
			return text
		}
	}
	return trimmed
}

func visibleReplyIsStructured(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || (trimmed[0] != '{' && trimmed[0] != '[') {
		return false
	}
	var object map[string]any
	if json.Unmarshal([]byte(trimmed), &object) != nil || object == nil {
		return false
	}
	return len(mapValue(object["action"])) > 0
}

// newVisibleReplyStream buffers only JSON-looking output so a protocol object
// is never emitted as a visible token. Ordinary Chinese text continues to
// stream immediately; a structured action is emitted once, after its content
// has been extracted.
func newVisibleReplyStream(onChunk func(string) error) (func(string) error, func() bool) {
	var buffer strings.Builder
	emitted := false
	decided := false
	push := func(chunk string) error {
		if onChunk == nil {
			return nil
		}
		if decided {
			emitted = true
			return onChunk(chunk)
		}
		buffer.WriteString(chunk)
		candidate := strings.TrimSpace(buffer.String())
		if candidate == "" {
			return nil
		}
		if visibleReplyIsStructured(candidate) {
			decided = true
			emitted = true
			return onChunk(normalizeVisibleReply(candidate))
		}
		if candidate[0] == '{' || candidate[0] == '[' {
			// Hold a bounded JSON-looking prefix until the complete object arrives;
			// malformed/ordinary prose is flushed rather than held indefinitely.
			if len([]rune(candidate)) <= 4096 {
				return nil
			}
		}
		decided = true
		text := buffer.String()
		buffer.Reset()
		emitted = true
		return onChunk(text)
	}
	return push, func() bool { return emitted }
}
