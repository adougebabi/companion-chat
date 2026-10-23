package prompt

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
)

// FormatMessages copies the transport envelope, preserving user prose (including
// JSON), native Tool results, and multimodal content. Business views are encoded
// explicitly with RenderJSON before they become messages; transport never guesses
// whether a user's JSON is an internal task packet.
func FormatMessages(messages []map[string]any) []map[string]any {
	return FormatMessagesForRole(messages, "")
}

func FormatMessagesForRole(messages []map[string]any, _ string) []map[string]any {
	result := make([]map[string]any, len(messages))
	for i, message := range messages {
		copied := make(map[string]any, len(message))
		for key, value := range message {
			copied[key] = value
		}
		result[i] = copied
	}
	return result
}

func FormatPromptContent(content string) string { return content }

// RenderJSON is the single encoder for model-facing structured business data.
// Map keys are deterministic; strings, false/null, numbers and nested schemas
// retain their JSON semantics. No shape-dependent syntax or size policy exists.
func RenderJSON(value any) string {
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		panic("invalid prompt JSON: " + err.Error())
	}
	return strings.TrimSuffix(out.String(), "\n")
}

func DecodeJSONPayload(content string) (string, any, bool) {
	if strings.TrimSpace(content) == "" {
		return "", nil, false
	}
	start := 0
	if content[0] != '{' && content[0] != '[' {
		objectStart, arrayStart := strings.IndexByte(content, '{'), strings.IndexByte(content, '[')
		start = -1
		if objectStart >= 0 {
			start = objectStart
		}
		if arrayStart >= 0 && (start < 0 || arrayStart < start) {
			start = arrayStart
		}
		if start < 0 {
			return "", nil, false
		}
	}
	payload := content[start:]
	decoder := json.NewDecoder(strings.NewReader(payload))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return "", nil, false
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return "", nil, false
	}
	return strings.TrimSpace(content[:start]), value, true
}
