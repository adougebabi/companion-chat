package core

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"time"
)

func replyErrorCode(err error) string {
	if errors.Is(err, errReplyQualityRepair) {
		return "reply_quality_repair_failed"
	}
	var toolFailure *agentRunFailure
	if errors.As(err, &toolFailure) && toolFailure.stage == "tool" {
		return safeAgentFailureCode(toolFailure.code, "tool_execution_failed")
	}
	var gateway *providerHTTPResponseError
	if errors.As(err, &gateway) && (gateway.StatusCode == 524 || gateway.StatusCode == 504) {
		return "reply_gateway_timeout"
	}
	if code, _ := capabilityErrorInfo(err, "", false); code != "" {
		return code
	}
	if err != nil {
		switch err.Error() {
		case "reply_generation_incomplete", "reply_secret_leak_rejected", "reply_media_request_invalid", "reply_media_placeholder_invalid", "reply_envelope_invalid", "reply_draft_invalid", "reply_expression_version_invalid", "reply_expression_snapshot_mismatch", "reply_snapshot_credential_source_unknown":
			return err.Error()
		}
	}
	if errors.Is(err, context.Canceled) {
		return "reply_cancelled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "reply_timeout"
	}
	if errors.Is(err, ErrPromptRequiredBudgetExceeded) {
		return "reply_context_budget_exceeded"
	}
	var response *providerHTTPResponseError
	if errors.As(err, &response) {
		message := strings.ToLower(response.Detail)
		if strings.Contains(message, "out of memory") || strings.Contains(message, "allocation failed") || strings.Contains(message, "oom") {
			return "reply_model_oom"
		}
		if strings.Contains(message, "insufficient_user_quota") || strings.Contains(message, "insufficient_quota") {
			return "reply_model_http_error"
		}
		switch response.StatusCode {
		case 413:
			return "reply_input_too_large"
		case 429:
			return "reply_model_busy"
		case 500, 502, 503, 504:
			return "reply_model_unavailable"
		default:
			if strings.Contains(message, "get_channel_failed") && response.StatusCode >= 500 {
				return "reply_model_unavailable"
			}
			return "reply_model_http_error"
		}
	}
	return "reply_generation_failed"
}

func replyTransportRetryable(err error) bool {
	if errors.Is(err, errReplyQualityRepair) {
		return false
	}
	if code := replyErrorCode(err); strings.HasPrefix(code, "reply_style_") || code == "reply_model_oom" {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	return providerUpstreamRetryable(err) || replyNetworkRetryable(err)
}

func replyNetworkRetryable(err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	// An HTTP status is classified above. url.Error also implements net.Error,
	// so a 4xx/5xx body must not be retried again as a dropped socket.
	var response *providerHTTPResponseError
	if errors.As(err, &response) {
		return false
	}
	var network net.Error
	return errors.Is(err, io.ErrUnexpectedEOF) || (errors.As(err, &network) && !network.Timeout())
}

// An explicitly configured legacy fallback may address model availability,
// but it must never ask another model to reconsider a character's decision or
// turn a business, authorization, tool, or output-consistency failure into a
// successful reply. Unknown failures are not evidence of an unavailable model.
func replyFailureAllowsLegacyFallback(err error) bool {
	if err == nil || errors.Is(err, errReplyQualityRepair) || errors.Is(err, context.Canceled) || errors.Is(err, ErrUnauthorized) || errors.Is(err, ErrInvalidArguments) || errors.Is(err, ErrConflict) {
		return false
	}
	var toolFailure *agentRunFailure
	if errors.As(err, &toolFailure) && toolFailure.stage == "tool" {
		return false
	}
	var capabilityFailure *CapabilityError
	if errors.As(err, &capabilityFailure) {
		return false
	}
	switch err.Error() {
	case "reply_model_unconfigured", "reply_model_unavailable":
		return true
	}
	var response *providerHTTPResponseError
	if errors.As(err, &response) {
		return providerHTTPUpstreamRetryable(response)
	}
	// The caller has already checked the overall request context. A timeout of
	// this model's own physical request may use the explicitly configured
	// fallback; an expired or cancelled overall reply still stops above it.
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var network net.Error
	if errors.As(err, &network) && network.Timeout() {
		return true
	}
	return replyNetworkRetryable(err)
}

// providerHTTPUpstreamRetryable is the shared HTTP classifier for one physical
// model call. 4xx and quota/OOM bodies are terminal. 500 joins 502/503/504, and
// a gateway get_channel_failed on any 5xx is retryable.
func providerHTTPUpstreamRetryable(response *providerHTTPResponseError) bool {
	if response == nil {
		return false
	}
	detail := strings.ToLower(response.Detail)
	if strings.Contains(detail, "out of memory") || strings.Contains(detail, "allocation failed") || strings.Contains(detail, "oom") {
		return false
	}
	if strings.Contains(detail, "insufficient_user_quota") || strings.Contains(detail, "insufficient_quota") {
		return false
	}
	if response.StatusCode >= 400 && response.StatusCode < 500 {
		return false
	}
	if strings.Contains(detail, "get_channel_failed") && response.StatusCode >= 500 {
		return true
	}
	return response.StatusCode == 500 || response.StatusCode == 502 || response.StatusCode == 503 || response.StatusCode == 504
}

func (a *App) runReplyWithFailurePolicy(ctx context.Context, input ConversationReplyTaskInput, run *privateReplyRun) (ReplyDraft, error) {
	if run.Settings.TimeoutSeconds > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(run.Settings.TimeoutSeconds)*time.Second)
		defer cancel()
	}
	var draft ReplyDraft
	var outputCorrectionReason error
	run.mu.Lock()
	effectiveMode := run.EffectiveMode
	if run.EffectiveAssignment != nil {
		input.Assignment = *run.EffectiveAssignment
		input.Mode = effectiveMode
	}
	run.mu.Unlock()
	err := run.AssignmentError
	if effectiveMode != "" {
		err = nil
	}
	if err == nil {
		if ctx.Err() != nil {
			return draft, ctx.Err()
		}
		// Transport retries happen inside the physical model call and keep its
		// idempotency key. Repeating this task would replay tools.
		input.CorrelationID = "reply:" + stableDigest(input.CorrelationID+"attempt:0")
		draft, err = a.RunConversationReplyTask(ctx, input)
		if err != nil && (err.Error() == "reply_envelope_invalid" || err.Error() == "reply_draft_invalid" || err.Error() == "reply_media_request_invalid" || err.Error() == "reply_media_placeholder_invalid") {
			outputCorrectionReason = err
			input.Context.OutputCorrection = "[回复格式纠错]上一次输出格式无效或用文字冒充图片。需要配图时真实调用已提供的 reply.image.request；没有工具或结果时不声称发送图片，不输出 path/SVG 占位。然后只输出正常段落正文，最多一个段间空行；不要输出无效JSON或推理。"
			input.CorrelationID = "reply-correction:" + stableDigest(input.CorrelationID)
			draft, err = a.RunConversationReplyTask(ctx, input)
		}
	}
	if errors.Is(err, errReplyQualityRepair) {
		return draft, err
	}
	if err == nil {
		return draft, nil
	}
	if ctx.Err() != nil {
		return draft, ctx.Err()
	}
	if errors.Is(err, context.Canceled) {
		return draft, context.Canceled
	}
	if outputCorrectionReason != nil {
		// A network failure while correcting an invalid output does not turn
		// the original consistency failure into permission to choose a new model.
		code := replyErrorCode(outputCorrectionReason)
		return draft, newCapabilityError(code, false, errors.New("reply output correction failed"))
	}
	if strings.HasPrefix(replyErrorCode(err), "reply_style_") {
		return draft, err
	}
	if run.Settings.FailurePolicy != "legacy_fallback" || effectiveMode == "legacy_fallback" || !replyFailureAllowsLegacyFallback(err) {
		return draft, newCapabilityError(replyErrorCode(err), false, errors.New(replyErrorCode(err)))
	}
	assignment, assignmentErr := a.Provider.assignment(isolatedReplyModelContext(ctx), "generic_llm")
	if assignmentErr != nil {
		return draft, newCapabilityError("reply_legacy_fallback_unavailable", false, errors.New("reply fallback unavailable"))
	}
	if assignment.EndpointID == input.Assignment.EndpointID && assignment.ModelID == input.Assignment.ModelID {
		return draft, newCapabilityError("reply_legacy_fallback_same_model", false, errors.New("fallback would repeat the same failed model"))
	}
	input.Assignment = assignment
	input.Mode = "legacy_fallback"
	input.CorrelationID = "reply-fallback:" + stableDigest(input.CorrelationID)
	draft, err = a.RunConversationReplyTask(ctx, input)
	if err == nil {
		run.mu.Lock()
		run.EffectiveAssignment = &assignment
		run.EffectiveMode = "legacy_fallback"
		run.mu.Unlock()
	}
	return draft, err
}
