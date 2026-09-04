package core

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// formatProviderMessages converts only complete JSON payloads in user/tool
// messages. System instructions and ordinary prose are intentionally left
// untouched so response-format contracts remain explicit. YAML is selected
// over Markdown/TOON because cognition payloads are deeply nested and contain
// heterogeneous arrays; YAML removes JSON punctuation without asking every
// configured model to learn a second tabular protocol.
func formatProviderMessages(messages []map[string]any) []map[string]any {
	formatted := make([]map[string]any, 0, len(messages))
	for _, message := range messages {
		copyMessage := make(map[string]any, len(message))
		for key, value := range message {
			copyMessage[key] = value
		}
		if content, ok := message["content"].(string); ok && stringValue(message["role"]) != "system" {
			copyMessage["content"] = formatProviderPromptContent(content)
		}
		formatted = append(formatted, copyMessage)
	}
	return formatted
}

func formatProviderPromptContent(content string) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return content
	}
	prefix, value, ok := decodeProviderJSONPayload(trimmed)
	if !ok {
		return content
	}
	if _, ok := value.(map[string]any); !ok {
		if _, ok := value.([]any); !ok {
			return content
		}
	}
	formatted := renderProviderYAML(value)
	if prefix != "" {
		return strings.TrimSpace(prefix) + "\n\n" + formatted
	}
	return formatted
}

func decodeProviderJSONPayload(content string) (string, any, bool) {
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

func renderProviderYAML(value any) string {
	var builder strings.Builder
	renderProviderYAMLValue(&builder, value, 0)
	return strings.TrimRight(builder.String(), "\n")
}

func renderProviderYAMLValue(builder *strings.Builder, value any, indent int) {
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		if len(keys) == 0 {
			builder.WriteString("{}\n")
			return
		}
		for _, key := range keys {
			writeProviderIndent(builder, indent)
			builder.WriteString(formatProviderYAMLKey(key))
			builder.WriteString(":")
			writeProviderYAMLChild(builder, typed[key], indent)
		}
	case []any:
		if len(typed) == 0 {
			builder.WriteString("[]\n")
			return
		}
		for _, item := range typed {
			writeProviderIndent(builder, indent)
			builder.WriteString("-")
			if text, ok := item.(string); ok && strings.ContainsAny(text, "\r\n") {
				builder.WriteString(" |\n")
				writeProviderYAMLBlock(builder, text, indent+2)
				continue
			}
			if object, ok := item.(map[string]any); ok && len(object) > 0 {
				keys := providerYAMLMapKeys(object)
				if isProviderYAMLScalar(object[keys[0]]) {
					builder.WriteString(" ")
					builder.WriteString(formatProviderYAMLKey(keys[0]))
					builder.WriteString(":")
					writeProviderYAMLChild(builder, object[keys[0]], indent)
					for _, key := range keys[1:] {
						writeProviderIndent(builder, indent+2)
						builder.WriteString(formatProviderYAMLKey(key))
						builder.WriteString(":")
						writeProviderYAMLChild(builder, object[key], indent+2)
					}
				} else {
					builder.WriteString("\n")
					renderProviderYAMLValue(builder, object, indent+2)
				}
				continue
			}
			if list, ok := item.([]any); ok && len(list) > 0 {
				builder.WriteString("\n")
				renderProviderYAMLValue(builder, list, indent+2)
				continue
			}
			builder.WriteString(" ")
			builder.WriteString(formatProviderYAMLScalar(item))
			builder.WriteString("\n")
		}
	default:
		writeProviderIndent(builder, indent)
		builder.WriteString(formatProviderYAMLScalar(value))
		builder.WriteString("\n")
	}
}

func providerYAMLMapKeys(value map[string]any) []string {
	keys := make([]string, 0, len(value))
	for key := range value {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func writeProviderYAMLChild(builder *strings.Builder, value any, indent int) {
	if text, ok := value.(string); ok && strings.ContainsAny(text, "\r\n") {
		builder.WriteString(" |\n")
		writeProviderYAMLBlock(builder, text, indent+2)
		return
	}
	if isProviderYAMLScalar(value) {
		builder.WriteString(" ")
		builder.WriteString(formatProviderYAMLScalar(value))
		builder.WriteString("\n")
		return
	}
	if object, ok := value.(map[string]any); ok && len(object) == 0 {
		builder.WriteString(" {}\n")
		return
	}
	if list, ok := value.([]any); ok && len(list) == 0 {
		builder.WriteString(" []\n")
		return
	}
	builder.WriteString("\n")
	renderProviderYAMLValue(builder, value, indent+2)
}

func isProviderYAMLScalar(value any) bool {
	switch value.(type) {
	case map[string]any, []any:
		return false
	default:
		return true
	}
}

func formatProviderYAMLKey(value string) string {
	if value != "" && utf8.ValidString(value) {
		safe := true
		for index, char := range value {
			if !(char == '_' || char == '-' || char == '.' || char == ':' || char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || index > 0 && char >= '0' && char <= '9') {
				safe = false
				break
			}
		}
		if safe {
			return value
		}
	}
	return formatProviderYAMLString(value)
}

func formatProviderYAMLScalar(value any) string {
	if value == nil {
		return "null"
	}
	switch typed := value.(type) {
	case string:
		return formatProviderYAMLString(typed)
	case json.Number:
		return typed.String()
	case float64:
		return strconv.FormatFloat(typed, 'g', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(typed), 'g', -1, 32)
	case int:
		return strconv.Itoa(typed)
	case int8, int16, int32, int64:
		return fmt.Sprint(typed)
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprint(typed)
	case bool:
		return strconv.FormatBool(typed)
	default:
		encoded, _ := json.Marshal(typed)
		return formatProviderYAMLString(string(encoded))
	}
}

func formatProviderYAMLString(value string) string {
	if value == "" {
		return "''"
	}
	if strings.Contains(value, "\n") || strings.Contains(value, "\r") {
		// Scalar block strings are handled by the dedicated helper below. Keep
		// this quoted fallback for keys and unusual nested values.
		return quoteProviderYAML(value)
	}
	trimmed := strings.TrimSpace(value)
	if trimmed != value || strings.ContainsAny(value, ":#{}[],&*!|>'\"%@`\n\r\t") || value == "-" || value == "?" || value == "null" || value == "true" || value == "false" || looksLikeProviderYAMLNumber(value) {
		return quoteProviderYAML(value)
	}
	return value
}

func quoteProviderYAML(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func looksLikeProviderYAMLNumber(value string) bool {
	if value == "" {
		return false
	}
	_, err := strconv.ParseFloat(value, 64)
	return err == nil
}

// writeProviderYAMLBlock keeps multiline prompt values readable without JSON
// escape sequences while preserving their exact line boundaries.
func writeProviderYAMLBlock(builder interface{ WriteString(string) (int, error) }, value string, indent int) {
	for _, line := range strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n") {
		writeProviderIndent(builder, indent)
		_, _ = builder.WriteString(line + "\n")
	}
}

func writeProviderIndent(builder interface{ WriteString(string) (int, error) }, indent int) {
	if indent > 0 {
		_, _ = builder.WriteString(strings.Repeat(" ", indent))
	}
}
