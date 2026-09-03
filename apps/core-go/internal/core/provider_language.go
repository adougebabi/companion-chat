package core

import (
	"encoding/json"
	"strings"
)

// withChineseOutputInstruction adds one transport-level language rule to all
// LLM roles except media_prompt. The media prompt role intentionally remains
// English because its output is consumed by the image-generation provider.
// Protocol keys, enum literals, identifiers and timestamps are never
// translated by this instruction.
func withChineseOutputInstruction(role string, messages []map[string]any) []map[string]any {
	if len(messages) == 0 {
		return messages
	}
	if role == "media_prompt" {
		// Media prompts intentionally skip the Chinese-language rule, but they
		// still cross the same Provider boundary and must not send multiple or
		// late system roles to a strict chat template.
		return prependSystemMessage(messages, nil)
	}
	instruction := map[string]any{
		"role":    "system",
		"content": "除媒体提示词生成外，所有自然语言字段必须使用中文，包括 visible_text、response_intent、attention、thought、desire、agency、activity、scene、description 以及 reflection/initialization 的自然语言内容。JSON key、枚举值、工具名、ID 和时间戳属于协议字段，必须保持原样，不要翻译。",
	}
	return prependSystemMessage(messages, instruction)
}

// prependSystemMessage normalizes the role shape expected by chat templates.
// mlx-serve accepts one system message at the beginning of the conversation;
// callers may independently contribute operation, context, and language
// rules, so those rules are merged in their existing order instead of being
// emitted as multiple system messages. Non-system history keeps its order.
func prependSystemMessage(messages []map[string]any, instruction map[string]any) []map[string]any {
	systemContents := make([]string, 0, len(messages)+1)
	if content := systemMessageContent(instruction); content != "" {
		systemContents = append(systemContents, content)
	}
	nonSystem := make([]map[string]any, 0, len(messages))
	for _, message := range messages {
		if stringValue(message["role"]) == "system" {
			if content := systemMessageContent(message); content != "" {
				systemContents = append(systemContents, content)
			}
			continue
		}
		nonSystem = append(nonSystem, message)
	}
	result := make([]map[string]any, 0, len(nonSystem)+1)
	if len(systemContents) > 0 {
		result = append(result, map[string]any{"role": "system", "content": strings.Join(systemContents, "\n\n")})
	}
	result = append(result, nonSystem...)
	return result
}

func systemMessageContent(message map[string]any) string {
	if message == nil {
		return ""
	}
	switch content := message["content"].(type) {
	case string:
		return strings.TrimSpace(content)
	case nil:
		return ""
	default:
		encoded, err := json.Marshal(content)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(encoded))
	}
}
