package core

import (
	"context"
	"strings"

	"github.com/cloudwego/eino/schema"
)

func factOnlyRequestPhase(ctx context.Context) string {
	if repair, _ := ctx.Value(finalRepairContextKey{}).(bool); repair {
		return "final_repair"
	}
	if bridge, ok := adkCapabilityContext(ctx); ok && bridge.Trace != nil {
		_, results := bridge.Trace.Snapshot()
		if _, committed := committedConversationReplyResult(results); committed {
			return "settlement"
		}
	}
	return "cognition"
}

func factOnlyPhaseInput(ctx context.Context, input []*schema.Message) []*schema.Message {
	phase := factOnlyRequestPhase(ctx)
	if phase == "cognition" {
		return input
	}
	protocol := factOnlySettlementProtocol
	if phase == "final_repair" {
		protocol = factOnlyFinalRepairProtocol
	}
	result := append([]*schema.Message(nil), input...)
	for index, message := range input {
		if message == nil || message.Role != schema.System || !strings.Contains(message.Content, factOnlyCognitionProtocol) {
			continue
		}
		copyMessage := *message
		copyMessage.Content = strings.Replace(message.Content, factOnlyCognitionProtocol, protocol, 1)
		copyMessage.Content = strings.ReplaceAll(copyMessage.Content, factOnlyConversationPolicy, "")
		copyMessage.Content = strings.ReplaceAll(copyMessage.Content, factOnlyExecutionBoundary, "")
		result[index] = &copyMessage
	}
	return result
}
