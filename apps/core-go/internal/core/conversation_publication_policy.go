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
type conversationPublicationPhysicalRequestPolicy struct {
	directImageDiscoveryPrompt []*schema.Message
}

func (p conversationPublicationPhysicalRequestPolicy) DecidePhysicalModelRequest(ctx context.Context) (physicalModelRequestDecision, error) {
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
	imageReplyPending := directImageReplyPending(ctx, results)
	replyRequest := physicalModelRequestDecision{ToolChoice: schema.ToolChoiceForced, OmitResponseFormat: true,
		AllowedToolNames: []string{conversationReplyCapabilityName}}
	if imageReplyPending {
		replyRequest.ToolInputSchemas = map[string]map[string]any{
			conversationReplyCapabilityName: (directMainReplyCapability{replyOnly: true}).Definition().InputSchema,
		}
	}
	if run := replyRunFromContext(ctx); run != nil && run.Settings.Mode == "direct" {
		for _, result := range results {
			if result.CapabilityName == conversationReplyCapabilityName && result.Status == "handled" && stringValue(mapValue(result.Output)["warning_code"]) == "reply_expression_suggestion_stale" {
				// The model saw a frozen suggestion that is no longer valid.
				// Repeating this run cannot make its old context publishable.
				return physicalModelRequestDecision{}, newCapabilityError("reply_expression_suggestion_stale", false, ErrConflict)
			}
		}
	}
	if run := replyRunFromContext(ctx); run != nil && run.Settings.Mode == "direct" && directMainInvalidReplyAttempts(adkContext.Trace) >= 3 {
		return physicalModelRequestDecision{}, newCapabilityError("direct_reply_arguments_unresolved", false, errors.New("direct_reply_arguments_unresolved"))
	}
	if run := replyRunFromContext(ctx); run != nil && run.Settings.Mode == "direct" && directMediaDeliveryRejectedAttempts(adkContext.Trace) >= 3 {
		return physicalModelRequestDecision{}, newCapabilityError("direct_media_delivery_unresolved", false, errors.New("direct_media_delivery_unresolved"))
	}
	if run := replyRunFromContext(ctx); run != nil && run.Settings.Mode == "direct" && directMediaReplyRejectedBatches(adkContext.Trace) >= 3 {
		return physicalModelRequestDecision{}, newCapabilityError("direct_media_reply_stalled", false, errors.New("direct_media_reply_stalled"))
	}
	if conversationLoopNeedsReply(ctx) {
		if conversationClosedActionAttempts(adkContext.Trace, adkContext.Refresh) >= 2 || conversationNoProgressAttempts(ctx, 0, true) >= 5 {
			return physicalModelRequestDecision{}, newCapabilityError("tool_progress_stalled", false, errors.New("tool_progress_stalled"))
		}
		replyRequest.DisableThinking = true
		return replyRequest, nil
	}
	if queries := conversationLoadedQueryPhase(ctx); len(queries) > 0 {
		if conversationClosedActionAttempts(adkContext.Trace, adkContext.Refresh) >= 2 {
			return physicalModelRequestDecision{}, newCapabilityError("tool_progress_stalled", false, errors.New("tool_progress_stalled"))
		}
		return physicalModelRequestDecision{ToolChoice: schema.ToolChoiceForced, OmitResponseFormat: true, AllowedToolNames: queries}, nil
	}
	if repeatedDiscoveryLoadCount(adkContext.Trace) >= 3 || repeatedDiscoveryResultCount(adkContext.Trace) >= 3 {
		return physicalModelRequestDecision{}, newCapabilityError("tool_discovery_no_progress", false, errors.New("tool_discovery_no_progress"))
	}
	for _, result := range results {
		if result.CapabilityName == conversationReplyCapabilityName &&
			(result.ErrorCode == "reply_action_outcome_conflict" || result.ErrorCode == "reply_action_check_failed") {
			// The draft already exhausted its bounded fact correction. Asking the
			// parent again cannot retry this failed source or authorize publication.
			return physicalModelRequestDecision{}, newCapabilityError(result.ErrorCode, false, errors.New(result.ErrorCode))
		}
	}
	if correcting, _ := ctx.Value(conversationPublicationCorrectionKey{}).(bool); correcting {
		replyRequest.DisableThinking = true
		return replyRequest, nil
	}
	if imageReplyPending || directReplyRetryFromContext(ctx).intentID != "" {
		return replyRequest, nil
	}
	if len(results) > 0 && directImageCatalogPending(ctx, results) {
		return physicalModelRequestDecision{ToolChoice: schema.ToolChoiceForced, OmitResponseFormat: true,
			AllowedToolNames: []string{capabilityCatalogName}}, nil
	}
	if _, committed := committedMomentResult(adkContext.Trace); committed {
		return physicalModelRequestDecision{ToolChoice: schema.ToolChoiceForced, OmitResponseFormat: true, ExcludedToolNames: append([]string{"moment.publish"}, repeatedDiscoveryExclusions(adkContext.Trace)...)}, nil
	}
	return physicalModelRequestDecision{ToolChoice: schema.ToolChoiceForced, OmitResponseFormat: true, ExcludedToolNames: repeatedDiscoveryExclusions(adkContext.Trace), DisableThinking: len(p.directImageDiscoveryPrompt) != 0 && len(results) == 0}, nil
}

// Count physical model responses, not individual rejected Tool results. Any
// intervening real result ends the consecutive no-progress sequence.
func directMediaReplyRejectedBatches(trace *ADKCapabilityTrace) int {
	if trace == nil {
		return 0
	}
	_, results := trace.Snapshot()
	sequences := map[uint64]bool{}
	for index := len(results) - 1; index >= 0; index-- {
		result := results[index]
		if result.Status == "handled" && stringValue(mapValue(result.Output)["warning_code"]) == "invalid_arguments" &&
			(result.CapabilityName == conversationReplyCapabilityName || result.CapabilityName == "media.image.generate") {
			continue
		}
		if result.Status != "rejected" || result.ErrorCode != "direct_media_reply_separate_turn_required" {
			break
		}
		if identity, ok := trace.ModelIdentity(result.CallID); ok && identity.ModelCallSequence != 0 {
			sequences[identity.ModelCallSequence] = true
		}
	}
	return len(sequences)
}

// Consecutive invalid direct replies did not execute any effect. Bound this
// no-progress repair without preventing unrelated valid native Tool calls.
func directMainInvalidReplyAttempts(trace *ADKCapabilityTrace) int {
	if trace == nil {
		return 0
	}
	_, results := trace.Snapshot()
	count := 0
	for index := len(results) - 1; index >= 0; index-- {
		result := results[index]
		invalidSchema := result.Status == "handled" && stringValue(mapValue(result.Output)["warning_code"]) == "invalid_arguments"
		invalidDecision := result.Status == "failed" && (result.ErrorCode == "direct_reply_decision_invalid" || result.ErrorCode == "direct_reply_decision_required")
		if result.CapabilityName != conversationReplyCapabilityName || (!invalidSchema && !invalidDecision) {
			break
		}
		count++
	}
	return count
}

// Media-claim retries are bounded for the whole run, including when unrelated
// Tool results are interleaved between rejected reply candidates.
func directMediaDeliveryRejectedAttempts(trace *ADKCapabilityTrace) int {
	if trace == nil {
		return 0
	}
	_, results := trace.Snapshot()
	count := 0
	for _, result := range results {
		if result.CapabilityName != conversationReplyCapabilityName {
			continue
		}
		code := result.ErrorCode
		if result.Status == "handled" {
			code = stringValue(mapValue(result.Output)["warning_code"])
		}
		if (result.Status == "handled" || result.Status == "failed") && (code == "direct_media_delivery_unverified" || code == "direct_media_delivery_stale") {
			count++
		}
	}
	return count
}

// Two identical successful named disclosures already supplied the schema.
// Cool down discovery for one physical request, without choosing a business
// action or forcing acceptance. Any actual query/action restores discovery.
func repeatedDiscoveryExclusions(trace *ADKCapabilityTrace) []string {
	if repeatedDiscoveryLoadCount(trace) >= 2 {
		return []string{capabilityCatalogName, capabilityDiscoverName}
	}
	return nil
}

func repeatedDiscoveryLoadCount(trace *ADKCapabilityTrace) int {
	_, results := trace.Snapshot()
	previous := ""
	count := 0
	for index := len(results) - 1; index >= 0; index-- {
		result := results[index]
		if result.Status != "completed" || (result.CapabilityName != capabilityCatalogName && result.CapabilityName != capabilityDiscoverName) {
			break
		}
		out := mapValue(result.Output)
		if len(arrayValue(out["loaded"])) == 0 || len(arrayValue(out["unavailable"])) != 0 {
			break
		}
		fingerprint := jsonString(out["loaded"])
		if previous != "" && previous != fingerprint {
			break
		}
		previous = fingerprint
		count++
	}
	return count
}

// Repeated unavailable-name or empty-directory results are also no progress.
// Compare the actual complete result, never infer intent from user language.
func repeatedDiscoveryResultCount(trace *ADKCapabilityTrace) int {
	_, results := trace.Snapshot()
	previous := ""
	count := 0
	for index := len(results) - 1; index >= 0; index-- {
		result := results[index]
		if result.CapabilityName != capabilityCatalogName && result.CapabilityName != capabilityDiscoverName {
			break
		}
		fingerprint := jsonString(map[string]any{"name": result.CapabilityName, "status": result.Status, "output": result.Output, "error_code": result.ErrorCode})
		if previous != "" && previous != fingerprint {
			break
		}
		previous = fingerprint
		count++
	}
	return count
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

func conversationPublicationOmission(ctx context.Context, message *schema.Message) bool {
	if message == nil || len(message.ToolCalls) != 0 {
		return false
	}
	finish := ""
	if message.ResponseMeta != nil {
		finish = strings.TrimSpace(message.ResponseMeta.FinishReason)
	}
	if strings.TrimSpace(message.Content) == "" {
		// An explicit stop after an accepted Direct image left the reply
		// unpublished. Reuse the existing reply-only correction once.
		run := replyRunFromContext(ctx)
		bridge, ok := adkCapabilityContext(ctx)
		if finish != "stop" || run == nil || run.Settings.Mode != "direct" || !ok || bridge.Trace == nil {
			return false
		}
		if directReplyRetryFromContext(ctx).intentID != "" {
			return true
		}
		_, results := bridge.Trace.Snapshot()
		if directImageCatalogPending(ctx, results) && conversationLoopNeedsReply(ctx) {
			return true
		}
		for _, result := range results {
			if result.CapabilityName == "media.image.generate" && result.Status == "accepted" {
				return true
			}
		}
		return false
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
