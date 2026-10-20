package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"
)

// conversationPublicationPhysicalRequestPolicy keeps private conversation
// publication inside the native Eino loop. Until a real private reply is
// committed, normal physical requests retain the selected Tool catalog and
// cannot terminate through final JSON grammar. Media may accompany a reply but cannot
// replace the private conversation message.
type conversationPublicationPhysicalRequestPolicy struct{}

func (conversationPublicationPhysicalRequestPolicy) DecidePhysicalModelRequest(ctx context.Context) (physicalModelRequestDecision, error) {
	adkContext, ok := adkCapabilityContext(ctx)
	if !ok || adkContext.Trace == nil {
		return physicalModelRequestDecision{}, errors.New("conversation_publication_trace_missing")
	}
	if conversationHasAuthoritativeOutput(adkContext.Trace) {
		return physicalModelRequestDecision{ToolChoice: schema.ToolChoiceAllowed}, nil
	}
	if correcting, _ := ctx.Value(conversationPublicationCorrectionKey{}).(bool); correcting {
		return physicalModelRequestDecision{ToolChoice: schema.ToolChoiceForced, OmitResponseFormat: true,
			AllowedToolNames: []string{conversationReplyCapabilityName}, DisableThinking: true}, nil
	}
	return physicalModelRequestDecision{ToolChoice: schema.ToolChoiceForced, OmitResponseFormat: true}, nil
}

func conversationHasAuthoritativeOutput(trace *ADKCapabilityTrace) bool {
	if trace == nil {
		return false
	}
	_, results := trace.Snapshot()
	_, committed := committedConversationReplyResult(results)
	if committed {
		return true
	}
	for _, result := range results {
		if result.CapabilityName == conversationReplyCapabilityName && result.Status == "accepted" && stringValue(mapValue(result.Output)["reply_status"]) == "waiting" && stringValue(mapValue(result.Output)["draft_ref"]) != "" {
			return true
		}
	}
	return false
}

func conversationPublicationPolicyEnabled(ctx context.Context) bool {
	policy, ok := physicalModelRequestPolicyFromContext(ctx)
	if !ok {
		return false
	}
	_, ok = policy.(conversationPublicationPhysicalRequestPolicy)
	return ok
}

// This marker affects one physical request inside the same native Runner.
type conversationPublicationCorrectionKey struct{}

func conversationPublicationProposal(ctx context.Context) bool {
	if !conversationPublicationPolicyEnabled(ctx) {
		return false
	}
	bridge, ok := adkCapabilityContext(ctx)
	return ok && !conversationHasAuthoritativeOutput(bridge.Trace)
}

func conversationPublicationOmission(message *schema.Message) bool {
	if message == nil || len(message.ToolCalls) != 0 || strings.TrimSpace(message.Content) == "" {
		return false
	}
	finish := ""
	if message.ResponseMeta != nil {
		finish = strings.TrimSpace(message.ResponseMeta.FinishReason)
	}
	return finish == "" || finish == "stop" || finish == "unknown"
}

// A committed visible reply ends the conversation turn. The next physical
// model request is replaced locally so Eino can stop without another provider call.
func visibleReplyEndsTurn(ctx context.Context, input []*schema.Message) (*schema.Message, bool) {
	if !visibleReplyCommitted(ctx) {
		return nil, false
	}
	return visibleReplyTerminalMessage(input), true
}

func visibleReplyCommitted(ctx context.Context) bool {
	if !conversationPublicationPolicyEnabled(ctx) {
		return false
	}
	bridge, ok := adkCapabilityContext(ctx)
	if !ok || bridge.Trace == nil {
		return false
	}
	_, results := bridge.Trace.Snapshot()
	_, committed := committedConversationReplyResult(results)
	return committed
}

func visibleReplyTerminalMessage(input []*schema.Message) *schema.Message {
	content := `{"action_type":"reply","response_intent":"visible reply committed","influences":[]}`
	for index := len(input) - 1; index >= 0; index-- {
		message := input[index]
		if message == nil || message.Role != schema.Assistant {
			continue
		}
		trimmed := strings.TrimSpace(message.Content)
		if trimmed != "" && json.Valid([]byte(trimmed)) {
			parsed := map[string]any{}
			if json.Unmarshal([]byte(trimmed), &parsed) == nil && parsed["action_type"] != nil && parsed["response_intent"] != nil && parsed["influences"] != nil {
				content = trimmed
			}
		}
		break
	}
	message := schema.AssistantMessage(content, nil)
	if message.ResponseMeta == nil {
		message.ResponseMeta = &schema.ResponseMeta{}
	}
	message.ResponseMeta.FinishReason = "stop"
	return message
}

func assistantMessageStream(message *schema.Message) *schema.StreamReader[*schema.Message] {
	reader, writer := schema.Pipe[*schema.Message](1)
	writer.Send(message, nil)
	writer.Close()
	return reader
}

func validateConversationPublicationCorrection(ctx context.Context, message *schema.Message) error {
	correcting, _ := ctx.Value(conversationPublicationCorrectionKey{}).(bool)
	if !correcting {
		return nil
	}
	if message == nil || len(message.ToolCalls) != 1 || message.ToolCalls[0].Function.Name != conversationReplyCapabilityName {
		return fmt.Errorf("%w: conversation_native_output_missing", errADKFinalContractInvalid)
	}
	return nil
}
