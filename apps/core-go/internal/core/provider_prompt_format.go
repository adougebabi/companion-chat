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
// untouched so response-format contracts remain explicit. Nested maps stay
// YAML, while homogeneous object arrays use a compact TOON table because the
// current cognition context repeats the same field names dozens of times.
func formatProviderMessages(messages []map[string]any) []map[string]any {
	return formatProviderMessagesForRole(messages, "")
}

func formatProviderMessagesForRole(messages []map[string]any, role string) []map[string]any {
	formatted := make([]map[string]any, 0, len(messages))
	useTOON := role != "media_prompt"
	for _, message := range messages {
		copyMessage := make(map[string]any, len(message))
		for key, value := range message {
			copyMessage[key] = value
		}
		if content, ok := message["content"].(string); ok && stringValue(message["role"]) != "system" {
			copyMessage["content"] = formatProviderPromptContentWithMode(content, useTOON)
		}
		formatted = append(formatted, copyMessage)
	}
	return formatted
}

func formatProviderPromptContent(content string) string {
	return formatProviderPromptContentWithMode(content, true)
}

func formatProviderPromptContentWithMode(content string, useTOON bool) string {
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
	formatted := renderProviderYAMLWithMode(value, useTOON)
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
	return renderProviderYAMLWithMode(value, true)
}

func renderProviderYAMLWithMode(value any, useTOON bool) string {
	var builder strings.Builder
	renderProviderYAMLValueWithMode(&builder, value, 0, useTOON)
	return strings.TrimRight(builder.String(), "\n")
}

func renderProviderYAMLValue(builder *strings.Builder, value any, indent int) {
	renderProviderYAMLValueWithMode(builder, value, indent, true)
}

func renderProviderYAMLValueWithMode(builder *strings.Builder, value any, indent int, useTOON bool) {
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
			if useTOON {
				if rows, ok := typed[key].([]any); ok {
					if fields, render := providerTOONFields(rows); render {
						builder.WriteString("[")
						builder.WriteString(strconv.Itoa(len(rows)))
						builder.WriteString("]{")
						builder.WriteString(strings.Join(fields, ","))
						builder.WriteString("}:\n")
						for _, row := range rows {
							object := row.(map[string]any)
							writeProviderIndent(builder, indent+2)
							for index, field := range fields {
								if index > 0 {
									builder.WriteString("|")
								}
								builder.WriteString(formatProviderTOONCell(object[field]))
							}
							builder.WriteString("\n")
						}
						continue
					}
				}
			}
			builder.WriteString(":")
			writeProviderYAMLChildWithMode(builder, typed[key], indent, useTOON)
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
	writeProviderYAMLChildWithMode(builder, value, indent, true)
}

func writeProviderYAMLChildWithMode(builder *strings.Builder, value any, indent int, useTOON bool) {
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
	renderProviderYAMLValueWithMode(builder, value, indent+2, useTOON)
}

func providerTOONFields(rows []any) ([]string, bool) {
	if len(rows) < 2 {
		return nil, false
	}
	first, ok := rows[0].(map[string]any)
	if !ok || len(first) == 0 {
		return nil, false
	}
	fields := providerYAMLMapKeys(first)
	for _, raw := range rows {
		object, ok := raw.(map[string]any)
		if !ok || len(object) != len(fields) {
			return nil, false
		}
		for _, field := range fields {
			if _, exists := object[field]; !exists || !isProviderTOONCell(object[field]) {
				return nil, false
			}
		}
	}
	return fields, true
}

func isProviderTOONCell(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		return false
	case []any:
		for _, item := range typed {
			if !isProviderYAMLScalar(item) {
				return false
			}
		}
	}
	return true
}

func formatProviderTOONCell(value any) string {
	if list, ok := value.([]any); ok {
		cells := make([]string, 0, len(list))
		for _, item := range list {
			cells = append(cells, formatProviderTOONCell(item))
		}
		return "[" + strings.Join(cells, ",") + "]"
	}
	if text, ok := value.(string); ok {
		if text == "" || strings.TrimSpace(text) != text || strings.ContainsAny(text, "|\r\n") || text == "null" || text == "true" || text == "false" || looksLikeProviderYAMLNumber(text) {
			return quoteProviderYAML(text)
		}
		return text
	}
	return formatProviderYAMLScalar(value)
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
