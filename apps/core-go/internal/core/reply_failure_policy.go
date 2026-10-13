package core

import (
	"context"
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
)

func replyErrorCode(err error) string {
	if code, _ := capabilityErrorInfo(err, "", false); code != "" {
		return code
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
		switch response.StatusCode {
		case 413:
			return "reply_input_too_large"
		case 429:
			return "reply_model_busy"
		case 502, 503, 504:
			return "reply_model_unavailable"
		default:
			return "reply_model_http_error"
		}
	}
	return "reply_generation_failed"
}

func replyTransportRetryable(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var response *providerHTTPResponseError
	if errors.As(err, &response) {
		return response.StatusCode == 502 || response.StatusCode == 503 || response.StatusCode == 504
	}
	var network net.Error
	return errors.Is(err, io.ErrUnexpectedEOF) || (errors.As(err, &network) && !network.Timeout())
}

func (a *App) runReplyWithFailurePolicy(ctx context.Context, input ConversationReplyTaskInput, run *privateReplyRun) (ReplyDraft, error) {
	var draft ReplyDraft
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
		for attempt := 0; attempt <= run.Settings.RetryLimit; attempt++ {
			if ctx.Err() != nil {
				return draft, ctx.Err()
			}
			input.CorrelationID = "reply:" + stableDigest(input.CorrelationID+"attempt:"+strconv.Itoa(attempt))
			draft, err = a.RunConversationReplyTask(ctx, input)
			if err == nil {
				return draft, nil
			}
			if err.Error() == "reply_envelope_invalid" || err.Error() == "reply_draft_invalid" || err.Error() == "reply_media_request_invalid" || err.Error() == "reply_media_placeholder_invalid" {
				input.Context.OutputCorrection = "[回复格式纠错]上一次输出格式无效或用文字冒充图片。需要配图时真实调用已提供的 reply.image.request；没有工具或结果时不声称发送图片，不输出 path/SVG 占位。然后只输出正常段落正文，最多一个段间空行；不要输出无效JSON或推理。"
				input.CorrelationID = "reply-correction:" + stableDigest(input.CorrelationID)
				draft, err = a.RunConversationReplyTask(ctx, input)
				break
			}
			if !replyTransportRetryable(err) {
				break
			}
		}
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
	if run.Settings.FailurePolicy != "legacy_fallback" || effectiveMode == "legacy_fallback" {
		return draft, newCapabilityError(replyErrorCode(err), false, errors.New(replyErrorCode(err)))
	}
	assignment, assignmentErr := a.Provider.assignment(isolatedReplyModelContext(ctx), "generic_llm")
	if assignmentErr != nil {
		return draft, newCapabilityError("reply_legacy_fallback_unavailable", false, errors.New("reply fallback unavailable"))
	}
	if assignment.EndpointID == input.Assignment.EndpointID && assignment.ModelID == input.Assignment.ModelID {
		return draft, newCapabilityError("reply_legacy_fallback_same_model", false, errors.New("fallback would repeat the same failed model"))
	}
	assignment.TokenBudget = min(assignment.TokenBudget, run.Settings.MaxTokens)
	assignment.MaxInputTokens = min(assignment.MaxInputTokens, run.Settings.MaxInputTokens)
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
