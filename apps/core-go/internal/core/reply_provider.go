package core

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
)

type ReplyMediaRequest struct {
	Kind   string `json:"kind"`
	Intent string `json:"intent"`
}
type ReplyDraft struct {
	AttachmentRefs []string            `json:"attachment_refs,omitempty"`
	Text           string              `json:"text"`
	MediaRequests  []ReplyMediaRequest `json:"media_requests,omitempty"`
	Mode           string              `json:"mode"`
	ModelRef       string              `json:"model_ref"`
	ConfigVersion  int64               `json:"config_version"`
	LifeRevision   string              `json:"life_revision,omitempty"`
	FactsRevision  string              `json:"facts_revision,omitempty"`
}

func parseReplyDraft(text string) (ReplyDraft, error) {
	text = strings.TrimSpace(text)
	draft := ReplyDraft{Text: text}
	if strings.HasPrefix(text, "{") {
		var shape map[string]json.RawMessage
		if json.Unmarshal([]byte(text), &shape) == nil && (shape["text"] != nil || shape["media_requests"] != nil) {
			var envelope struct {
				Text          string              `json:"text"`
				MediaRequests []ReplyMediaRequest `json:"media_requests"`
			}
			if _, err := decodeReplyVisionSetting([]byte(text), envelope); err != nil {
				return draft, errors.New("reply_envelope_invalid")
			}
			if err := json.Unmarshal([]byte(text), &envelope); err != nil {
				return draft, errors.New("reply_envelope_invalid")
			}
			draft.Text = strings.TrimSpace(envelope.Text)
			draft.MediaRequests = envelope.MediaRequests
		} else if strings.Contains(text, `"media_requests"`) || strings.Contains(text, `"text"`) {
			return draft, errors.New("reply_envelope_invalid")
		}
	}
	if draft.Text == "" || len([]rune(draft.Text)) > 32000 || len(draft.MediaRequests) > 2 {
		return draft, errors.New("reply_draft_invalid")
	}
	draft.Text = normalizeReplyText(draft.Text)
	if draft.Text == "" {
		return draft, errors.New("reply_draft_invalid")
	}
	for _, request := range draft.MediaRequests {
		if (request.Kind != "image" && request.Kind != "sticker") || strings.TrimSpace(request.Intent) == "" || len([]rune(request.Intent)) > 2048 {
			return draft, errors.New("reply_media_request_invalid")
		}
	}
	return draft, nil
}

// Normalize prose paragraph spacing, preserving fenced/inline code escapes.
func normalizeReplyText(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	var lines []string
	fenced, blank := false, false
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") || strings.HasPrefix(strings.TrimSpace(line), "~~~") {
			fenced = !fenced
			lines = append(lines, line)
			blank = false
			continue
		}
		if fenced {
			lines = append(lines, line)
			continue
		}
		segments := strings.Split(line, "`")
		for index := range segments {
			if index%2 == 0 {
				segments[index] = strings.ReplaceAll(segments[index], `\n`, "\n")
			}
		}
		line = strings.Join(segments, "`")
		for _, part := range strings.Split(line, "\n") {
			if strings.TrimSpace(part) == "" {
				if blank {
					continue
				}
				part = ""
				blank = true
			} else {
				blank = false
			}
			lines = append(lines, part)
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

type ConversationReplyTaskInput struct {
	Context       ReplyContextInput
	Assignment    providerAssignment
	ConfigVersion int64
	CorrelationID string
	ParentRunID   string
	DraftRef      string
	Concurrency   int
	Mode          string
}

// Clear parent model/tool policy while retaining authorization, logical lease,
// cancellation and the source lifecycle guard. No parent transcript is reused.
func isolatedReplyModelContext(ctx context.Context) context.Context {
	ctx = withoutPhysicalModelRequestPolicy(ctx)
	ctx = context.WithValue(ctx, kevRunSelectionKey{}, (*kevRunSelection)(nil))
	ctx = context.WithValue(ctx, kevPersonaKey{}, (*kevPersonaAdmission)(nil))
	ctx = context.WithValue(ctx, adkConversationContextKey{}, adkConversationContext{})
	ctx = context.WithValue(ctx, runtimeContextRefreshPlanKey{}, (func(context.Context) (modelContextRefreshContent, error))(nil))
	ctx = context.WithValue(ctx, finalRepairContextKey{}, false)
	ctx = withoutProviderQueueBypass(ctx)
	return WithPromptDiagnostics(ctx, map[string]any{})
}

func (a *App) RunConversationReplyTask(ctx context.Context, input ConversationReplyTaskInput) (ReplyDraft, error) {
	ctx = isolatedReplyModelContext(ctx)
	ctx = withFrozenProviderAssignment(ctx, conversationReplyModelRole, input.Assignment)
	ctx = WithProviderCorrelation(ctx, input.CorrelationID)
	ctx = WithProviderAttemptIdentity(ctx, "")
	ctx = withProviderCapacity(ctx, conversationReplyModelRole, max(1, input.Concurrency))
	imageTool := &replyImageRequestTool{}
	definitions := replyWriterDefinitions(input.Context)
	registry, err := NewCapabilityRegistry(imageTool)
	if err != nil {
		return ReplyDraft{}, err
	}
	ctx = withCapabilityRunRegistry(ctx, registry)
	prompt, err := buildFactOnlyReplyContext(input.Context, input.Assignment)
	if err != nil {
		return ReplyDraft{}, err
	}
	diagnostics := cloneMap(prompt.Diagnostics)
	for key, value := range map[string]any{"run_id": input.CorrelationID, "parent_run_id": input.ParentRunID,
		"draft_ref": input.DraftRef, "reply_mode": firstString(input.Mode, "fact_only"), "effective_binding_role": input.Assignment.Role, "config_version": input.ConfigVersion, "prompt_version": factOnlyReplyPromptVersion, "prompt_budget": prompt.Trace} {
		diagnostics[key] = value
	}
	ctx = WithPromptDiagnostics(ctx, diagnostics)
	result, err := a.RunFormalAgent(ctx, FormalAgentConversationReply, FormalAgentRunInput{Prompt: prompt, SchemaName: "conversation_reply_text", Definitions: definitions, Invoker: replyImageRequestInvoker{tool: imageTool}})
	if err != nil {
		return ReplyDraft{}, err
	}
	if result.Completion.FinishReason != "stop" {
		return ReplyDraft{}, errors.New("reply_generation_incomplete")
	}
	if input.Assignment.Secret != "" && strings.Contains(result.Completion.Text, input.Assignment.Secret) {
		return ReplyDraft{}, errors.New("reply_secret_leak_rejected")
	}
	draft, err := parseReplyDraft(result.Completion.Text)
	if err != nil {
		return draft, err
	}
	if len(draft.MediaRequests) > 0 && len(imageTool.requests) == 0 {
		return ReplyDraft{}, errors.New("reply_media_request_invalid")
	}
	// Native receipts own every fresh handoff; JSON cannot add requests. The
	// envelope decoder still reads stored drafts from the earlier protocol.
	draft.MediaRequests = append([]ReplyMediaRequest(nil), imageTool.requests...)
	if err := validateReplyMediaProse(draft, input.Context); err != nil {
		return ReplyDraft{}, err
	}
	a.recordDiagnosticEvent(withProviderDiagnosticSecrets(ctx, input.Assignment.Secret), "reply.draft.prepared", "info", input.Context.Projection.FluctlightID, input.DraftRef, input.CorrelationID, map[string]any{"parent_run_id": input.ParentRunID, "draft_ref": input.DraftRef, "stage": "private_draft", "visibility": "not_published", "text": draft.Text, "media_requests": draft.MediaRequests, "reply_mode": firstString(input.Mode, "fact_only"), "config_version": input.ConfigVersion})
	draft.Mode = firstString(input.Mode, "fact_only")
	draft.ModelRef = input.Assignment.ModelID
	draft.ConfigVersion = input.ConfigVersion
	return draft, nil
}

func validateReplyMediaProse(draft ReplyDraft, input ReplyContextInput) error {
	// Reject the concrete fabricated attachment markers seen in failed Writer
	// output. Never turn such prose into a synthetic image or a Tool receipt.
	if strings.Contains(draft.Text, "[path data for ") {
		return errors.New("reply_media_placeholder_invalid")
	}
	if strings.Contains(draft.Text, "（发送图片）") || strings.Contains(draft.Text, "(发送图片)") {
		for _, fact := range input.ToolFacts {
			if stringValue(fact["kind"]) == "media_action_result" && stringValue(mapValue(fact["value"])["status"]) == "ready" {
				return nil
			}
		}
		return errors.New("reply_media_placeholder_invalid")
	}
	return nil
}
