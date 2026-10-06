package core

import (
	"context"

	"github.com/cloudwego/eino/schema"
)

// Compact only the outbound copy, never Eino's transcript or durable Tool audit.
// Current input, Runtime, system instructions and every ToolCall/ToolResult stay
// intact. Plain reasoning and old complete conversation turns are optional.
func (m *queuedToolCallingChatModel) compactPhysicalInput(ctx context.Context, input []*schema.Message) ([]*schema.Message, error) {
	policy, err := promptBudgetPolicyForAssignment(m.assignment)
	if err != nil {
		return nil, err
	}
	estimate := func(messages []*schema.Message) int {
		raw := make([]map[string]any, 0, len(messages))
		for _, message := range messages {
			if message != nil {
				raw = append(raw, einoBudgetMessage(message))
			}
		}
		return estimatePromptWireInput(raw, RenderCapabilityTools(m.definitions), m.responseFormat)
	}
	before := estimate(input)
	if before <= policy.MaxInputTokens {
		return input, nil
	}
	result := append([]*schema.Message(nil), input...)
	reasoningRemoved, historyRemoved := 0, 0
	for index, message := range result {
		if message == nil || message.Role != schema.Assistant || message.ReasoningContent == "" {
			continue
		}
		copyMessage := *message
		copyMessage.ReasoningContent = ""
		result[index] = &copyMessage
		reasoningRemoved++
		if estimate(result) <= policy.MaxInputTokens {
			break
		}
	}
	// Only a formal Runtime marker identifies the initial historical portion.
	// Never guess whether a current-run Tool transcript is expendable.
	runtimeFound, lastUser := false, -1
	for index, message := range result {
		if isRuntimeContextMessage(message) {
			runtimeFound = true
		} else if message != nil && message.Role == schema.User {
			lastUser = index
		}
	}
	if runtimeFound && lastUser >= 0 {
		for start := 0; start < lastUser && estimate(result) > policy.MaxInputTokens; {
			message := result[start]
			if message == nil || message.Role != schema.User || isRuntimeContextMessage(message) {
				start++
				continue
			}
			end := start + 1
			for end < lastUser && result[end] != nil && result[end].Role == schema.Assistant && len(result[end].ToolCalls) == 0 {
				end++
			}
			// Do not split a turn followed by a tool message or a ToolCall.
			if end == start+1 || (end < len(result) && result[end] != nil &&
				(result[end].Role == schema.Tool || len(result[end].ToolCalls) > 0)) {
				start = end
				continue
			}
			historyRemoved += end - start
			result = append(result[:start:start], result[end:]...)
			lastUser -= end - start
		}
	}
	if m.provider != nil && m.provider.DB != nil && (reasoningRemoved > 0 || historyRemoved > 0) {
		m.provider.runtimeSupport().RecordDiagnosticEvent(ctx, "adk.model.input_compacted", "info", "", "", m.correlationID, map[string]any{
			"before_estimated_tokens": before, "after_estimated_tokens": estimate(result),
			"reasoning_messages_removed": reasoningRemoved, "historical_messages_removed": historyRemoved,
			"max_input_tokens": policy.MaxInputTokens,
		})
	}
	return result, nil
}
