package core

// withChineseOutputInstruction adds one transport-level language rule to all
// LLM roles except media_prompt. The media prompt role intentionally remains
// English because its output is consumed by the image-generation provider.
// Protocol keys, enum literals, identifiers and timestamps are never
// translated by this instruction.
func withChineseOutputInstruction(role string, messages []map[string]any) []map[string]any {
	if role == "media_prompt" || len(messages) == 0 {
		return messages
	}
	instruction := map[string]any{
		"role":    "system",
		"content": "除媒体提示词生成外，所有自然语言字段必须使用中文，包括 visible_text、response_intent、attention、thought、desire、agency、activity、scene、description 以及 reflection/initialization 的自然语言内容。JSON key、枚举值、工具名、ID 和时间戳属于协议字段，必须保持原样，不要翻译。",
	}
	result := make([]map[string]any, 0, len(messages)+1)
	result = append(result, messages[0], instruction)
	result = append(result, messages[1:]...)
	return result
}
