package task

import (
	"strings"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/ai/prompt"
)

// WithChineseOutputInstruction adds one transport-level language rule to all
// LLM roles except media_prompt.
func WithChineseOutputInstruction(role string, messages []map[string]any) []map[string]any {
	if len(messages) == 0 {
		return messages
	}
	if role == "media_prompt" {
		return PrependSystemMessage(messages, nil)
	}
	instruction := map[string]any{
		"role":    "system",
		"content": prompt.ProviderLanguageRule,
	}
	return PrependSystemMessage(messages, instruction)
}

// PrependSystemMessage normalizes the role shape expected by chat templates.
func PrependSystemMessage(messages []map[string]any, instruction map[string]any) []map[string]any {
	systemContents := make([]string, 0, len(messages)+1)
	if content := SystemMessageContent(instruction); content != "" {
		systemContents = append(systemContents, content)
	}
	nonSystem := make([]map[string]any, 0, len(messages))
	for _, message := range messages {
		if StringValue(message["role"]) == "system" {
			if content := SystemMessageContent(message); content != "" {
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

func SystemMessageContent(message map[string]any) string {
	if message == nil {
		return ""
	}
	return strings.TrimSpace(StringValue(message["content"]))
}
