package core

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	aiprompt "github.com/fluctlight/local-ai-companion/apps/core-go/internal/ai/prompt"
)

// formatProviderMessages converts only complete JSON payloads in user/tool
// messages. System instructions and ordinary prose are intentionally left
// untouched so response-format contracts remain explicit.
func formatProviderMessages(messages []map[string]any) []map[string]any {
	return aiprompt.FormatMessages(messages)
}

func formatProviderMessagesForRole(messages []map[string]any, role string) []map[string]any {
	return aiprompt.FormatMessagesForRole(messages, role)
}

func formatProviderPromptContent(content string) string {
	return aiprompt.FormatPromptContent(content)
}

func formatProviderPromptContentWithMode(content string, useTOON bool) string {
	return aiprompt.FormatPromptContentWithMode(content, useTOON)
}

func decodeProviderJSONPayload(content string) (string, any, bool) {
	return aiprompt.DecodeJSONPayload(content)
}

func renderProviderYAML(value any) string {
	return aiprompt.RenderYAML(value)
}

func renderProviderYAMLWithMode(value any, useTOON bool) string {
	return aiprompt.RenderYAMLWithMode(value, useTOON)
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
				if rows, ok := toAnyRows(typed[key]); ok {
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
	case []map[string]any:
		converted := make([]any, len(typed))
		for i, row := range typed {
			converted[i] = row
		}
		renderProviderYAMLValueWithMode(builder, converted, indent, useTOON)
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
					writeProviderYAMLChildWithMode(builder, object[keys[0]], indent, useTOON)
					for _, key := range keys[1:] {
						writeProviderIndent(builder, indent+2)
						builder.WriteString(formatProviderYAMLKey(key))
						builder.WriteString(":")
						writeProviderYAMLChildWithMode(builder, object[key], indent+2, useTOON)
					}
				} else {
					builder.WriteString("\n")
					renderProviderYAMLValueWithMode(builder, object, indent+2, useTOON)
				}
				continue
			}
			if list, ok := item.([]any); ok && len(list) > 0 {
				builder.WriteString("\n")
				renderProviderYAMLValueWithMode(builder, list, indent+2, useTOON)
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
	if list, ok := value.([]map[string]any); ok && len(list) == 0 {
		builder.WriteString(" []\n")
		return
	}
	builder.WriteString("\n")
	renderProviderYAMLValueWithMode(builder, value, indent+2, useTOON)
}

func toAnyRows(value any) ([]any, bool) {
	if rows, ok := value.([]any); ok {
		return rows, true
	}
	if rows, ok := value.([]map[string]any); ok {
		converted := make([]any, len(rows))
		for i, row := range rows {
			converted[i] = row
		}
		return converted, true
	}
	return nil, false
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
		if text == "" || strings.TrimSpace(text) != text || strings.ContainsAny(text, ",|[]\r\n") || text == "null" || text == "true" || text == "false" || looksLikeProviderYAMLNumber(text) {
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
