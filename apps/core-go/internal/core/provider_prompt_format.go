package core

import aiprompt "github.com/fluctlight/local-ai-companion/apps/core-go/internal/ai/prompt"

func formatProviderMessages(messages []map[string]any) []map[string]any {
	return aiprompt.FormatMessages(messages)
}
func formatProviderMessagesForRole(messages []map[string]any, role string) []map[string]any {
	return aiprompt.FormatMessagesForRole(messages, role)
}
func formatProviderPromptContent(content string) string { return content }
func decodeProviderJSONPayload(content string) (string, any, bool) {
	return aiprompt.DecodeJSONPayload(content)
}
func renderProviderJSON(value any) string { return aiprompt.RenderJSON(value) }
