package core

import (
	aitask "github.com/fluctlight/local-ai-companion/apps/core-go/internal/ai/task"
)

// withChineseOutputInstruction adds one transport-level language rule to all
// LLM roles except media_prompt.
func withChineseOutputInstruction(role string, messages []map[string]any) []map[string]any {
	return aitask.WithChineseOutputInstruction(role, messages)
}

// prependSystemMessage normalizes the role shape expected by chat templates.
func prependSystemMessage(messages []map[string]any, instruction map[string]any) []map[string]any {
	return aitask.PrependSystemMessage(messages, instruction)
}

func systemMessageContent(message map[string]any) string {
	return aitask.SystemMessageContent(message)
}
