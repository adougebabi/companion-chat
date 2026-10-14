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
	AttachmentRefs    []string                `json:"attachment_refs,omitempty"`
	Text              string                  `json:"text"`
	MediaRequests     []ReplyMediaRequest     `json:"media_requests,omitempty"`
	Mode              string                  `json:"mode"`
	ModelRef          string                  `json:"model_ref"`
	ConfigVersion     int64                   `json:"config_version"`
	LifeRevision      string                  `json:"life_revision,omitempty"`
	FactsRevision     string                  `json:"facts_revision,omitempty"`
	ExpressionVersion string                  `json:"expression_version,omitempty"`
	Expression        ReplyExpressionContract `json:"expression_contract,omitempty"`
	Style             ReplyStyleTrace         `json:"style,omitempty"`
	Candidates        []ReplyStyleTrace       `json:"candidates,omitempty"`
	RawCandidate      string                  `json:"raw_candidate,omitempty"`
	FinishReason      string                  `json:"finish_reason,omitempty"`
	ProviderUsage     map[string]any          `json:"provider_usage,omitempty"`
	CommittedText     string                  `json:"committed_text,omitempty"`
	CommittedMetadata map[string]any          `json:"committed_metadata,omitempty"`
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
	Context          ReplyContextInput
	Assignment       providerAssignment
	ConfigVersion    int64
	CorrelationID    string
	ParentRunID      string
	DraftRef         string
	Concurrency      int
	Mode             string
	ClaimStyleRepair func(context.Context, ReplyStyleTrace) error
	RecordStyleTrace func(context.Context, ReplyStyleTrace)
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
	if a.DB == nil || a.DB.Pool() == nil {
		input.Context.CapabilityFacts = cloneMap(input.Context.CapabilityFacts)
		delete(input.Context.CapabilityFacts, "persona_detail_registered")
	}
	definitions := replyWriterDefinitions(input.Context)
	capabilities := []Capability{imageTool}
	if input.Context.CapabilityFacts["persona_detail_registered"] == true {
		capabilities = append(capabilities, personaDetailCapability{service: newPersonaDetailService(a)})
	}
	registry, err := NewCapabilityRegistry(capabilities...)
	if err != nil {
		return ReplyDraft{}, err
	}
	ctx = withCapabilityRunRegistry(ctx, registry)
	prompt, err := buildFactOnlyReplyContext(input.Context, input.Assignment)
	if err != nil {
		return ReplyDraft{}, err
	}
	diagnostics := cloneMap(prompt.Diagnostics)
	contract, err := replyExpressionContract(input.Context.ExpressionVersion)
	if err != nil {
		return ReplyDraft{}, err
	}
	for key, value := range map[string]any{"run_id": input.CorrelationID, "parent_run_id": input.ParentRunID,
		"draft_ref": input.DraftRef, "reply_mode": firstString(input.Mode, "fact_only"), "output_scene": firstString(input.Context.Scene, "private_reply"), "effective_binding_role": input.Assignment.Role, "config_version": input.ConfigVersion, "prompt_version": contract.Version, "style_repair": input.Context.StyleRepair, "prompt_budget": prompt.Trace} {
		diagnostics[key] = value
	}
	ctx = WithPromptDiagnostics(ctx, diagnostics)
	invoker := &replyWriterInvoker{image: replyImageRequestInvoker{tool: imageTool}, app: a, input: input}
	result, err := a.RunFormalAgent(ctx, FormalAgentConversationReply, FormalAgentRunInput{Prompt: prompt, SchemaName: "conversation_reply_text", Definitions: definitions, Invoker: invoker})
	if err != nil {
		return ReplyDraft{}, err
	}
	draft, err := decodeReplyCompletion(result.Completion, input.Context, imageTool.requests, input.Assignment.Secret)
	if err != nil {
		return draft, err
	}
	draft.ExpressionVersion, draft.Expression = contract.Version, contract
	if !input.Context.StyleRepair && input.Context.Scene != replySceneMoment && contract.Version == factOnlyReplyPromptVersion {
		draft, err = a.validateAndRepairReplyStyle(ctx, input, draft, result.Completion.Text)
		if err != nil {
			return draft, err
		}
	}
	a.recordDiagnosticEvent(withProviderDiagnosticSecrets(ctx, input.Assignment.Secret), "reply.draft.prepared", "info", input.Context.Projection.FluctlightID, input.DraftRef, input.CorrelationID, map[string]any{"parent_run_id": input.ParentRunID, "draft_ref": input.DraftRef, "stage": "private_draft", "visibility": "not_published", "text": draft.Text, "media_requests": draft.MediaRequests, "style": draft.Style, "prompt_version": contract.Version, "reply_mode": firstString(input.Mode, "fact_only"), "config_version": input.ConfigVersion})
	draft.Mode = firstString(input.Mode, "fact_only")
	draft.ModelRef = input.Assignment.ModelID
	draft.ConfigVersion = input.ConfigVersion
	return draft, nil
}

func decodeReplyCompletion(completion ProviderCompletion, input ReplyContextInput, requests []ReplyMediaRequest, secret string) (ReplyDraft, error) {
	if secret != "" && strings.Contains(completion.Text, secret) {
		return ReplyDraft{}, errors.New("reply_secret_leak_rejected")
	}
	draft := ReplyDraft{RawCandidate: completion.Text, FinishReason: completion.FinishReason, ProviderUsage: cloneMap(completion.Usage)}
	if completion.FinishReason != "stop" {
		return draft, errors.New("reply_generation_incomplete")
	}
	parsed, err := parseReplyDraft(completion.Text)
	parsed.RawCandidate, parsed.FinishReason, parsed.ProviderUsage = draft.RawCandidate, draft.FinishReason, draft.ProviderUsage
	if err != nil {
		return parsed, err
	}
	if len(parsed.MediaRequests) > 0 && len(requests) == 0 {
		return parsed, errors.New("reply_media_request_invalid")
	}
	parsed.MediaRequests = append([]ReplyMediaRequest(nil), requests...)
	if err := validateReplyMediaProse(parsed, input); err != nil {
		return parsed, err
	}
	return parsed, nil
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
