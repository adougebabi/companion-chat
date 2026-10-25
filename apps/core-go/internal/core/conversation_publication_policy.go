package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"
	capabilitycontract "github.com/fluctlight/local-ai-companion/apps/core-go/internal/capability"
)

// conversationPublicationPhysicalRequestPolicy keeps private conversation
// publication inside the native Eino loop. Until a real private reply is
// committed, normal physical requests retain the selected Tool catalog and
// cannot terminate through final JSON grammar. Media may accompany a reply but cannot
// replace the private conversation message.
type conversationPublicationPhysicalRequestPolicy struct{}

func (conversationPublicationPhysicalRequestPolicy) DecidePhysicalModelRequest(ctx context.Context) (physicalModelRequestDecision, error) {
	if err := ctx.Err(); err != nil {
		return physicalModelRequestDecision{}, err
	}
	adkContext, ok := adkCapabilityContext(ctx)
	if !ok || adkContext.Trace == nil {
		return physicalModelRequestDecision{}, errors.New("conversation_publication_trace_missing")
	}
	if err := unchangedReplyMediaResult(adkContext.Trace); err != nil {
		return physicalModelRequestDecision{}, err
	}
	if conversationHasAuthoritativeOutput(adkContext.Trace) {
		return physicalModelRequestDecision{ToolChoice: schema.ToolChoiceAllowed}, nil
	}
	_, results := adkContext.Trace.Snapshot()
	for _, result := range results {
		if result.CapabilityName == conversationReplyCapabilityName &&
			(result.ErrorCode == "reply_action_outcome_conflict" || result.ErrorCode == "reply_action_check_failed") {
			// The draft already exhausted its bounded fact correction. Asking the
			// parent again cannot retry this failed source or authorize publication.
			return physicalModelRequestDecision{}, newCapabilityError(result.ErrorCode, false, errors.New(result.ErrorCode))
		}
	}
	if correcting, _ := ctx.Value(conversationPublicationCorrectionKey{}).(bool); correcting {
		return physicalModelRequestDecision{ToolChoice: schema.ToolChoiceForced, OmitResponseFormat: true,
			AllowedToolNames: []string{conversationReplyCapabilityName}, DisableThinking: true}, nil
	}
	if _, committed := committedMomentResult(adkContext.Trace); committed {
		return physicalModelRequestDecision{ToolChoice: schema.ToolChoiceForced, OmitResponseFormat: true, ExcludedToolNames: []string{"moment.publish"}}, nil
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
		if conversationPublicationPolicyEnabled(ctx) && ctx.Err() == nil {
			if bridge, ok := adkCapabilityContext(ctx); ok && bridge.Trace != nil {
				_, results := bridge.Trace.Snapshot()
				for index := len(results) - 1; index >= 0; index-- {
					result := results[index]
					if result.CapabilityName != conversationReplyCapabilityName {
						continue
					}
					output := mapValue(result.Output)
					if result.Status == "accepted" && output["reply_status"] == "waiting" && stringValue(output["draft_ref"]) != "" {
						return schema.AssistantMessage(`{"action_type":"reply","response_intent":"reply awaits accepted media completion","influences":[]}`, nil), true
					}
					break
				}
			}
		}
		return nil, false
	}
	return visibleReplyTerminalMessage(input), true
}

var errReplyMediaNoProgress = errors.New("reply_media_no_progress")

func unchangedReplyMediaResult(trace *ADKCapabilityTrace) error {
	if trace == nil {
		return nil
	}
	_, results := trace.Snapshot()
	latest := ""
	for index := len(results) - 1; index >= 0; index-- {
		result := results[index]
		if result.CapabilityName != conversationReplyCapabilityName {
			continue
		}
		output := mapValue(result.Output)
		if result.Status != "accepted" || output["reply_status"] != "needs_action" || stringValue(output["draft_ref"]) == "" {
			return nil
		}
		fingerprint := jsonString(map[string]any{"draft_ref": output["draft_ref"], "media_requests": output["media_requests"]})
		if latest != "" {
			if latest == fingerprint {
				return errReplyMediaNoProgress
			}
			return nil
		}
		latest = fingerprint
	}
	return nil
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
	if !correcting && conversationPublicationPolicyEnabled(ctx) {
		if bridge, ok := adkCapabilityContext(ctx); ok {
			if _, committed := committedMomentResult(bridge.Trace); committed && message != nil {
				for _, call := range message.ToolCalls {
					if capabilitycontract.CapabilityNamesMatch(call.Function.Name, "moment.publish") {
						return fmt.Errorf("%w: conversation_publication_already_committed", errADKFinalContractInvalid)
					}
				}
			}
		}
	}
	if !correcting {
		return nil
	}
	if message == nil || len(message.ToolCalls) != 1 || !capabilitycontract.CapabilityNamesMatch(message.ToolCalls[0].Function.Name, conversationReplyCapabilityName) {
		return fmt.Errorf("%w: conversation_native_output_missing", errADKFinalContractInvalid)
	}
	return nil
}
