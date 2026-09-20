package prompt

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// FormatMessages converts only complete JSON payloads in user/tool
// messages. System instructions and ordinary prose are intentionally left
// untouched so response-format contracts remain explicit.
func FormatMessages(messages []map[string]any) []map[string]any {
	return FormatMessagesForRole(messages, "")
}

func FormatMessagesForRole(messages []map[string]any, role string) []map[string]any {
	formatted := make([]map[string]any, 0, len(messages))
	useTOON := role != "media_prompt"
	for _, message := range messages {
		copyMessage := make(map[string]any, len(message))
		for key, value := range message {
			copyMessage[key] = value
		}
		roleStr, _ := message["role"].(string)
		if content, ok := message["content"].(string); ok && roleStr != "system" {
			copyMessage["content"] = FormatPromptContentWithMode(content, useTOON)
		}
		formatted = append(formatted, copyMessage)
	}
	return formatted
}

func FormatPromptContent(content string) string {
	return FormatPromptContentWithMode(content, true)
}

func FormatPromptContentWithMode(content string, useTOON bool) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return content
	}
	prefix, value, ok := DecodeJSONPayload(trimmed)
	if !ok {
		return content
	}
	if _, ok := value.(map[string]any); !ok {
		if _, ok := value.([]any); !ok {
			return content
		}
	}
	formatted := RenderYAMLWithMode(value, useTOON)
	if prefix != "" {
		return strings.TrimSpace(prefix) + "\n\n" + formatted
	}
	return formatted
}

func DecodeJSONPayload(content string) (string, any, bool) {
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

func RenderYAML(value any) string {
	return RenderYAMLWithMode(value, true)
}

func RenderYAMLWithMode(value any, useTOON bool) string {
	var builder strings.Builder
	RenderYAMLValueWithMode(&builder, value, 0, useTOON)
	return strings.TrimRight(builder.String(), "\n")
}

func RenderYAMLValueWithMode(builder *strings.Builder, value any, indent int, useTOON bool) {
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
			WriteIndent(builder, indent)
			builder.WriteString(FormatYAMLKey(key))
			if useTOON {
				if rows, ok := typed[key].([]any); ok {
					if fields, render := TOONFields(rows); render {
						builder.WriteString("[")
						builder.WriteString(strconv.Itoa(len(rows)))
						builder.WriteString("]{")
						builder.WriteString(strings.Join(fields, ","))
						builder.WriteString("}:\n")
						for _, row := range rows {
							object := row.(map[string]any)
							WriteIndent(builder, indent+2)
							for index, field := range fields {
								if index > 0 {
									builder.WriteString("|")
								}
								builder.WriteString(FormatTOONCell(object[field]))
							}
							builder.WriteString("\n")
						}
						continue
					}
				}
			}
			builder.WriteString(":")
			WriteYAMLChildWithMode(builder, typed[key], indent, useTOON)
		}
	case []any:
		if len(typed) == 0 {
			builder.WriteString("[]\n")
			return
		}
		for _, item := range typed {
			WriteIndent(builder, indent)
			builder.WriteString("-")
			if text, ok := item.(string); ok && strings.ContainsAny(text, "\r\n") {
				builder.WriteString(" |\n")
				WriteYAMLBlock(builder, text, indent+2)
				continue
			}
			if object, ok := item.(map[string]any); ok && len(object) > 0 {
				keys := YAMLMapKeys(object)
				if isYAMLScalar(object[keys[0]]) {
					builder.WriteString(" ")
					builder.WriteString(FormatYAMLKey(keys[0]))
					builder.WriteString(":")
					WriteYAMLChildWithMode(builder, object[keys[0]], indent, useTOON)
					for _, key := range keys[1:] {
						WriteIndent(builder, indent+2)
						builder.WriteString(FormatYAMLKey(key))
						builder.WriteString(":")
						WriteYAMLChildWithMode(builder, object[key], indent+2, useTOON)
					}
				} else {
					builder.WriteString("\n")
					RenderYAMLValueWithMode(builder, object, indent+2, useTOON)
				}
				continue
			}
			if list, ok := item.([]any); ok && len(list) > 0 {
				builder.WriteString("\n")
				RenderYAMLValueWithMode(builder, list, indent+2, useTOON)
				continue
			}
			builder.WriteString(" ")
			builder.WriteString(FormatYAMLScalar(item))
			builder.WriteString("\n")
		}
	default:
		WriteIndent(builder, indent)
		builder.WriteString(FormatYAMLScalar(value))
		builder.WriteString("\n")
	}
}

func YAMLMapKeys(value map[string]any) []string {
	keys := make([]string, 0, len(value))
	for key := range value {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func WriteYAMLChildWithMode(builder *strings.Builder, value any, indent int, useTOON bool) {
	if text, ok := value.(string); ok && strings.ContainsAny(text, "\r\n") {
		builder.WriteString(" |\n")
		WriteYAMLBlock(builder, text, indent+2)
		return
	}
	if isYAMLScalar(value) {
		builder.WriteString(" ")
		builder.WriteString(FormatYAMLScalar(value))
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
	RenderYAMLValueWithMode(builder, value, indent+2, useTOON)
}

func TOONFields(rows []any) ([]string, bool) {
	if len(rows) < 2 {
		return nil, false
	}
	first, ok := rows[0].(map[string]any)
	if !ok || len(first) == 0 {
		return nil, false
	}
	fields := YAMLMapKeys(first)
	for _, raw := range rows {
		object, ok := raw.(map[string]any)
		if !ok || len(object) != len(fields) {
			return nil, false
		}
		for _, field := range fields {
			if _, exists := object[field]; !exists || !isTOONCell(object[field]) {
				return nil, false
			}
		}
	}
	return fields, true
}

func isTOONCell(value any) bool {
	switch typed := value.(type) {
	case map[string]any:
		return false
	case []any:
		for _, item := range typed {
			if !isYAMLScalar(item) {
				return false
			}
		}
	}
	return true
}

func FormatTOONCell(value any) string {
	if list, ok := value.([]any); ok {
		cells := make([]string, 0, len(list))
		for _, item := range list {
			cells = append(cells, FormatTOONCell(item))
		}
		return "[" + strings.Join(cells, ",") + "]"
	}
	if text, ok := value.(string); ok {
		if text == "" || strings.TrimSpace(text) != text || strings.ContainsAny(text, ",|[]\r\n") || text == "null" || text == "true" || text == "false" || looksLikeYAMLNumber(text) {
			return QuoteYAML(text)
		}
		return text
	}
	return FormatYAMLScalar(value)
}

func isYAMLScalar(value any) bool {
	switch value.(type) {
	case map[string]any, []any:
		return false
	default:
		return true
	}
}

func FormatYAMLKey(value string) string {
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
	return FormatYAMLString(value)
}

func FormatYAMLScalar(value any) string {
	if value == nil {
		return "null"
	}
	switch typed := value.(type) {
	case string:
		return FormatYAMLString(typed)
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
		return FormatYAMLString(string(encoded))
	}
}

func FormatYAMLString(value string) string {
	if value == "" {
		return "''"
	}
	if strings.Contains(value, "\n") || strings.Contains(value, "\r") {
		return QuoteYAML(value)
	}
	trimmed := strings.TrimSpace(value)
	if trimmed != value || strings.ContainsAny(value, ":#{}[],&*!|>'\"%@`\n\r\t") || value == "-" || value == "?" || value == "null" || value == "true" || value == "false" || looksLikeYAMLNumber(value) {
		return QuoteYAML(value)
	}
	return value
}

func QuoteYAML(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func looksLikeYAMLNumber(value string) bool {
	if value == "" {
		return false
	}
	_, err := strconv.ParseFloat(value, 64)
	return err == nil
}

func WriteYAMLBlock(builder interface{ WriteString(string) (int, error) }, value string, indent int) {
	for _, line := range strings.Split(strings.ReplaceAll(value, "\r\n", "\n"), "\n") {
		WriteIndent(builder, indent)
		_, _ = builder.WriteString(line + "\n")
	}
}

func WriteIndent(builder interface{ WriteString(string) (int, error) }, indent int) {
	if indent > 0 {
		_, _ = builder.WriteString(strings.Repeat(" ", indent))
	}
}
