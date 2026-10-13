package core

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type factOnlyReplyCapability struct {
	service     *factOnlyReplyService
	run         *privateReplyRun
	publication *ToolPublicationService
}

func (c factOnlyReplyCapability) Definition() CapabilityDefinition {
	d := conversationReplyCapabilityDefinition()
	d.Version = "fact_only.v1"
	d.Description = "Generate the independent persona reply from authoritative context and publish it. Supply only offered facts/result references; never compose text or prescribe social strategy. Consume needs_action results through real media tools, then resume the same draft."
	properties := map[string]any{
		"evidence_refs": map[string]any{"type": "array", "maxItems": 24, "items": stringSchema()},
		"draft_ref":     stringSchema(), "action_result_refs": map[string]any{"type": "array", "maxItems": 8, "items": stringSchema()},
		"topic_key": map[string]any{"type": "string", "minLength": 1, "maxLength": 128},
		"purpose":   map[string]any{"type": "string", "minLength": 1, "maxLength": 256},
	}
	d.InputSchema = objectSchema(properties, nil, false)
	d.OutputSchema = objectSchema(map[string]any{"text": stringSchema(), "target_kind": stringSchema(), "target_ref": stringSchema(),
		"replayed": map[string]any{"type": "boolean"}, "delivery_status": stringSchema(), "reply_status": stringSchema(),
		"draft_ref": stringSchema(), "media_requests": map[string]any{"type": "array", "items": openObjectSchema()},
		"reply_mode": stringSchema(), "reply_model_ref": stringSchema(), "config_version": map[string]any{"type": "integer"}}, []string{"reply_status", "reply_mode"}, false)
	d.ModelResultOmitFields = []string{"target_ref", "replayed"}
	d.SupportsCancel = true
	return d
}
func (c factOnlyReplyCapability) RequiredContext() []ContextSlot {
	return []ContextSlot{SlotCurrentLife}
}
func (c factOnlyReplyCapability) Execute(_ context.Context, invocation CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	return executeToolRequired(invocation)
}

func (c factOnlyReplyCapability) Prepare(ctx context.Context, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityInvocation, error) {
	if c.service == nil || c.run == nil {
		return invocation, newCapabilityError("reply_context_unavailable", false, ErrUnauthorized)
	}
	draft, ref, err := c.service.prepare(ctx, invocation, c.run)
	if err != nil {
		return invocation, err
	}
	data := map[string]any{"draft": draft, "draft_ref": ref, "life_revision": stringValue(resolved.Life.Data["context_revision"])}
	if draft.LifeRevision != "" {
		data["life_revision"] = draft.LifeRevision
	}
	return withCapabilityPreparedData(invocation, "fact_only_reply", data)
}

func (c factOnlyReplyCapability) ExecuteDirectTx(ctx context.Context, tx pgx.Tx, invocation CapabilityInvocation, resolved CapabilityContext, target DirectToolTarget) (CapabilityResult, error) {
	if err := ctx.Err(); err != nil {
		return failedCapabilityResult(invocation, "reply_cancelled", false), err
	}
	value, found, err := capabilityPreparedData(invocation, "fact_only_reply")
	if err != nil || !found {
		return failedCapabilityResult(invocation, "reply_prepared_missing", false), errors.New("reply_prepared_missing")
	}
	data := mapValue(value)
	var existingMessage string
	readErr := tx.QueryRow(ctx, `SELECT COALESCE(result_message_id,'') FROM public.conversation_reply_drafts WHERE id=$1 AND fluctlight_id=$2 AND conversation_id=$3 AND status='published' FOR UPDATE`, stringValue(data["draft_ref"]), target.FluctlightID, target.ConversationID).Scan(&existingMessage)
	if readErr == nil && existingMessage != "" {
		if err := requireConversationPublicationOwnershipTx(ctx, tx, target.AuthorizationActorID, target.FluctlightID, target.ConversationID); err != nil {
			return failedCapabilityResult(invocation, "reply_replay_unauthorized", false), err
		}
		var text string
		if err := tx.QueryRow(ctx, `SELECT text FROM public.conversation_messages WHERE id=$1 AND conversation_id=$2 AND author_actor_id=$3`, existingMessage, target.ConversationID, target.FluctlightID).Scan(&text); err != nil {
			return failedCapabilityResult(invocation, "reply_replay_message_missing", false), err
		}
		var saved ReplyDraft
		_ = json.Unmarshal(jsonBytes(data["draft"]), &saved)
		return CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "completed", ProviderRequestID: invocation.ProviderRequestID,
			Output: map[string]any{"text": text, "target_kind": "conversation_message", "target_ref": existingMessage, "replayed": true, "delivery_status": "duplicate_suppressed", "reply_status": "committed", "reply_mode": saved.Mode, "reply_model_ref": saved.ModelRef, "config_version": saved.ConfigVersion}}, nil
	}
	if readErr != nil && !errors.Is(readErr, pgx.ErrNoRows) {
		return failedCapabilityResult(invocation, "reply_replay_read_failed", true), readErr
	}
	var draft ReplyDraft
	if err := json.Unmarshal(jsonBytes(data["draft"]), &draft); err != nil {
		return failedCapabilityResult(invocation, "reply_prepared_invalid", false), err
	}
	if len(draft.MediaRequests) > 0 {
		return c.acceptMediaTx(ctx, tx, invocation, target, draft, stringValue(data["draft_ref"]))
	}
	if draft.FactsRevision != "" {
		if err := c.service.app.requireCurrentFactsRevisionTx(ctx, tx, target.FluctlightID, draft.FactsRevision); err != nil {
			return failedCapabilityResult(invocation, "reply_context_stale", false), newCapabilityError("reply_context_stale", false, err)
		}
	}
	args := decodeObject(invocation.Arguments)
	if resumed, _ := ctx.Value(replyContinuationKey{}).(string); resumed != "" {
		var scope []byte
		if err := tx.QueryRow(ctx, `SELECT execution_scope FROM public.conversation_reply_drafts WHERE id=$1`, resumed).Scan(&scope); err != nil {
			return CapabilityResult{}, err
		}
		for _, key := range []string{"topic_key", "purpose"} {
			args[key] = decodeObject(scope)[key]
		}
		if c.run.Scope.Surface == CapabilitySurfaceWakeUp {
			if err := authorizeWakeReplyContinuationTx(ctx, tx, resumed); err != nil {
				return CapabilityResult{}, err
			}
		}
	}
	resource, err := c.publication.PublishConversationReplyTx(ctx, tx, ConversationReplyPublication{
		WorkingProfileID: invocation.Metadata.WorkingProfileID, SuppressRecentDuplicate: target.AuthorizationPolicy == "autonomy",
		TopicKey: stringValue(args["topic_key"]), Purpose: stringValue(args["purpose"]), AuthorizationActorID: target.AuthorizationActorID,
		FluctlightID: target.FluctlightID, ConversationID: target.ConversationID, OperationID: privateReplyLogicalKey(ctx, invocation),
		CorrelationID: invocation.Metadata.CorrelationID, Text: draft.Text, ExpectedLifeContextRevision: stringValue(data["life_revision"]),
		AttachmentRefs: draft.AttachmentRefs,
		ReplyMetadata:  map[string]any{"reply_mode": draft.Mode, "reply_model_ref": draft.ModelRef, "config_version": draft.ConfigVersion, "prompt_version": factOnlyReplyPromptVersion}})
	if err != nil {
		code, retryable := publicationCapabilityError(err, "reply_publication_failed")
		return failedCapabilityResultDetail(invocation, code, retryable, "reply publication did not commit"), newCapabilityError(code, retryable, err)
	}
	if _, err := tx.Exec(ctx, `UPDATE public.conversation_reply_drafts SET status='published',result_message_id=$2,updated_at=now() WHERE id=$1 AND fluctlight_id=$3 AND conversation_id=$4`, stringValue(data["draft_ref"]), resource.ID, target.FluctlightID, target.ConversationID); err != nil {
		return failedCapabilityResult(invocation, "reply_draft_commit_failed", true), err
	}
	if resumed, _ := ctx.Value(replyContinuationKey{}).(string); resumed != "" {
		if err := completeReplyContinuationTx(ctx, tx, resumed); err != nil {
			return failedCapabilityResult(invocation, "reply_continuation_settle_failed", true), err
		}
	}
	delivery := "visible_output_committed"
	if resource.DuplicateSuppressed {
		delivery = "duplicate_suppressed"
	}
	return CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "completed",
		Output: map[string]any{"text": draft.Text, "target_kind": "conversation_message", "target_ref": resource.ID, "replayed": resource.Replayed,
			"delivery_status": delivery, "reply_status": "committed", "reply_mode": draft.Mode, "reply_model_ref": draft.ModelRef, "config_version": draft.ConfigVersion},
		ProviderRequestID: invocation.ProviderRequestID, CorrelationID: "conversation:" + target.ConversationID}, nil
}

// The business service owns generation and the durable candidate cache; the
// generic registry/ADK/transaction runtime never dispatches on reply modes.
type factOnlyReplyService struct{ app *App }

func newFactOnlyReplyService(app *App) *factOnlyReplyService { return &factOnlyReplyService{app: app} }

func (s *factOnlyReplyService) prepare(ctx context.Context, invocation CapabilityInvocation, run *privateReplyRun) (ReplyDraft, string, error) {
	logicalKey := privateReplyLogicalKey(ctx, invocation)
	ref := "reply_draft_" + stableDigest(logicalKey)
	var args struct {
		EvidenceRefs     []string `json:"evidence_refs"`
		DraftRef         string   `json:"draft_ref"`
		ActionResultRefs []string `json:"action_result_refs"`
	}
	if err := json.Unmarshal(invocation.Arguments, &args); err != nil {
		return ReplyDraft{}, "", err
	}
	if args.DraftRef != "" && args.DraftRef != ref {
		return ReplyDraft{}, "", newCapabilityError("reply_draft_scope_invalid", false, ErrUnauthorized)
	}
	var payload, bindingBytes []byte
	var status, profile string
	var lastError string
	err := s.app.DB.Pool().QueryRow(ctx, `SELECT payload,status,profile_id,binding_snapshot,COALESCE(last_error,'') FROM public.conversation_reply_drafts WHERE id=$1 AND logical_key=$2 AND fluctlight_id=$3 AND conversation_id=$4 AND authorization_actor_id=$5`, ref, logicalKey, invocation.Metadata.FluctlightID, invocation.Metadata.ConversationID, invocation.Metadata.AuthorizationActorID).Scan(&payload, &status, &profile, &bindingBytes, &lastError)
	if err == nil && (status == "prepared" || status == "published" || status == "needs_action" || status == "waiting_media" || status == "ready") {
		var stored ReplyDraft
		if profile != invocation.Metadata.WorkingProfileID && status != "published" {
			return stored, ref, newCapabilityError("reply_profile_changed", false, ErrLifeContextStale)
		}
		run.mu.Lock()
		run.DraftRef = ref
		run.mu.Unlock()
		if err := json.Unmarshal(payload, &stored); err != nil {
			return stored, ref, err
		}
		if status != "published" && status != "needs_action" && status != "waiting_media" && stored.FactsRevision != "" {
			live, readErr := s.app.readCurrentFactsRevision(ctx, invocation.Metadata.FluctlightID)
			if readErr != nil {
				return stored, ref, readErr
			}
			if live != stored.FactsRevision {
				_, updateErr := s.app.DB.Pool().Exec(ctx, `UPDATE public.conversation_reply_drafts SET status='failed',last_error='reply_context_stale',generation_owner=NULL,generation_until=NULL,updated_at=now() WHERE id=$1 AND status='prepared'`, ref)
				if updateErr != nil {
					return stored, ref, updateErr
				}
				return stored, ref, newCapabilityError("reply_context_stale", false, ErrCurrentFactsStale)
			}
		}
		if status == "needs_action" || status == "waiting_media" {
			stored, err = s.resumeMedia(ctx, invocation, run, stored, ref, args.ActionResultRefs)
		}
		return stored, ref, err
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return ReplyDraft{}, ref, err
	}
	if status == "cancelled" {
		return ReplyDraft{}, ref, newCapabilityError("reply_cancelled", false, context.Canceled)
	}
	currentRunID := firstString(run.Scope.RunID, run.Scope.OperationID)
	if status == "failed" && lastError != "reply_context_stale" && stringValue(decodeObject(bindingBytes)["run_id"]) == currentRunID {
		return ReplyDraft{}, ref, newCapabilityError("reply_generation_already_failed", false, errors.New("reply generation already failed in this run"))
	}
	if run.AssignmentError != nil && run.Settings.FailurePolicy != "legacy_fallback" {
		return ReplyDraft{}, ref, newCapabilityError(run.AssignmentError.Error(), false, run.AssignmentError)
	}
	owner := randomID("reply_generation_")
	safeAssignment := run.Assignment
	safeAssignment.Secret = ""
	binding := map[string]any{"assignment": safeAssignment, "endpoint_id": run.Assignment.EndpointID, "model_id": run.Assignment.ModelID, "binding_role": run.Assignment.Role, "run_id": currentRunID}
	_, err = s.app.DB.Pool().Exec(ctx, `INSERT INTO public.conversation_reply_drafts(id,logical_key,fluctlight_id,conversation_id,authorization_actor_id,subject_actor_id,source_fact_id,profile_id,status,config_snapshot,config_version,binding_snapshot,generation_owner,generation_until) VALUES($1,$2,$3,$4,$5,$6,$7,$8,'generating',$9,$10,$11,$12,now()+make_interval(secs=>$13)) ON CONFLICT(logical_key) DO NOTHING`, ref, logicalKey, invocation.Metadata.FluctlightID, invocation.Metadata.ConversationID, invocation.Metadata.AuthorizationActorID, firstString(invocation.Metadata.SubjectActorID, invocation.Metadata.AuthorizationActorID), invocation.SourceFactID, invocation.Metadata.WorkingProfileID, jsonBytes(run.Settings), run.ConfigVersion, jsonBytes(binding), owner, run.Settings.TimeoutSeconds)
	if err != nil {
		return ReplyDraft{}, ref, err
	}
	var currentOwner string
	err = s.app.DB.Pool().QueryRow(ctx, `UPDATE public.conversation_reply_drafts SET generation_owner=$2,generation_until=now()+make_interval(secs=>$3),status='generating',profile_id=$4,binding_snapshot=$5,updated_at=now() WHERE id=$1 AND status IN ('generating','failed') AND (generation_owner=$2 OR generation_until<=now() OR generation_owner IS NULL) RETURNING generation_owner`, ref, owner, run.Settings.TimeoutSeconds, invocation.Metadata.WorkingProfileID, jsonBytes(binding)).Scan(&currentOwner)
	if errors.Is(err, pgx.ErrNoRows) {
		return ReplyDraft{}, ref, newCapabilityError("reply_generation_in_progress", true, ErrConflict)
	}
	if err != nil {
		return ReplyDraft{}, ref, err
	}
	draft, generationErr := s.generate(ctx, invocation, run, ref, args.EvidenceRefs)
	if generationErr != nil {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cleanupCancel()
		_, _ = s.app.DB.Pool().Exec(cleanupCtx, `UPDATE public.conversation_reply_drafts SET status='failed',last_error=$3,generation_owner=NULL,generation_until=NULL,updated_at=now() WHERE id=$1 AND generation_owner=$2`, ref, owner, "reply_generation_failed")
		return draft, ref, generationErr
	}
	if err := ctx.Err(); err != nil {
		return ReplyDraft{}, ref, err
	}
	run.mu.Lock()
	if run.EffectiveAssignment != nil {
		actual := *run.EffectiveAssignment
		actual.Secret = ""
		binding["assignment"] = actual
		binding["reply_mode"] = run.EffectiveMode
		binding["endpoint_id"] = actual.EndpointID
		binding["model_id"] = actual.ModelID
		binding["binding_role"] = actual.Role
	}
	run.mu.Unlock()
	tag, err := s.app.DB.Pool().Exec(ctx, `UPDATE public.conversation_reply_drafts SET binding_snapshot=$4,payload=$3,status='prepared',generation_owner=NULL,generation_until=NULL,updated_at=now() WHERE id=$1 AND generation_owner=$2 AND generation_until>now() AND status='generating'`, ref, owner, jsonBytes(draft), jsonBytes(binding))
	if err != nil {
		return draft, ref, err
	}
	if tag.RowsAffected() != 1 {
		return draft, ref, newCapabilityError("reply_generation_lease_lost", false, ErrConflict)
	}
	run.mu.Lock()
	run.DraftRef = ref
	run.mu.Unlock()
	return draft, ref, nil
}

func (s *factOnlyReplyService) generate(ctx context.Context, invocation CapabilityInvocation, run *privateReplyRun, ref string, evidenceRefs []string) (ReplyDraft, error) {
	projection, err := s.app.BuildContextProjectionFor(ctx, ContextProjectionRequest{AuthorizationActorID: invocation.Metadata.AuthorizationActorID,
		SpeakerActorID: firstString(invocation.Metadata.SubjectActorID, invocation.Metadata.AuthorizationActorID), SourceFactID: invocation.SourceFactID,
		FluctlightID: invocation.Metadata.FluctlightID, ConversationID: invocation.Metadata.ConversationID, WorkingProfileID: invocation.Metadata.WorkingProfileID,
		CurrentUserText: run.CurrentInput, MemoryOperation: MemoryForConversation, MemoryConversationMode: MemoryConversationExact})
	if err != nil {
		return ReplyDraft{}, err
	}
	var toolFacts []map[string]any
	run.mu.Lock()
	for _, state := range run.ActionFacts {
		toolFacts = append(toolFacts, map[string]any{"kind": "media_action_result", "value": map[string]any{"kind": state.Kind, "intent": state.Intent, "status": state.Status, "error_code": state.ErrorCode}, "observation": "仅证明工具实际状态；ready表示文件可用，不证明已视觉检验或已发送。不要新增媒体请求，只基于这些结果自主完成正文。"})
	}
	if run.ActionDraftText != "" {
		toolFacts = append(toolFacts, map[string]any{"kind": "private_candidate_text", "value": run.ActionDraftText, "authority": "未发送草稿，按本次真实媒体结果局部调整；不能作为状态证据"})
	}
	run.mu.Unlock()
	for _, ref := range evidenceRefs {
		entry, exists := replyEvidenceEntry(invocation, projection, ref)
		if !exists {
			return ReplyDraft{}, newCapabilityError("reply_evidence_not_authorized", false, ErrInvalidArguments)
		}
		fact := decodeObject(entry.Snapshot)
		// A matched piece of inferred text is not an authoritative restriction.
		if stringValue(fact["assertion_type"]) == "inference" || entry.Kind == ContextReferenceMemory || entry.Kind == ContextReferenceIntention || entry.Kind == ContextReferenceDevelopingSelf {
			continue
		}
		toolFacts = append(toolFacts, map[string]any{"kind": string(entry.Kind), "value": replySemanticValue(fact), "certainty": "仅按对应类型的权威状态/真实消息解释，引用本身不证明事件发生"})
	}
	compiled, err := s.app.loadCompiledWorkingPersona(ctx, projection)
	if err != nil {
		return ReplyDraft{}, err
	}
	persona := systemPersonaForProjectionWithWorking(projection, "conversation_reply_text", renderCompiledWorkingPersona(compiled))
	delete(persona, workingPersonaSwitchKey)
	delete(persona, "personality_system")
	current := firstString(run.CurrentInput, projection.CurrentUserText)
	if current == "" {
		current = "这是一次已授权的主动私聊，请依据当前人格、目标和真实历史自主决定表达。"
	}
	vision, _, err := s.app.visionUnderstandingSettings(ctx)
	if err != nil {
		return ReplyDraft{}, err
	}
	visionState := "disabled"
	if vision.Enabled {
		visionState = "configured_separately_no_observation_yet"
	}
	_, imageRegistered := s.app.capabilityRegistryForContext(ctx).Definition("media.image.generate")
	input := ConversationReplyTaskInput{Context: ReplyContextInput{Projection: projection, WorkingPersona: persona, CurrentInput: current, ToolFacts: toolFacts,
		CapabilityFacts: map[string]any{"image_generation_registered": imageRegistered, "media_requires_real_tool_result": true, "sticker_attachments": false, "vision_understanding": visionState}},
		DraftRef: ref, Assignment: run.Assignment, ConfigVersion: run.ConfigVersion, CorrelationID: "reply:" + stableDigest(ref+invocation.ProviderRequestID), ParentRunID: providerCorrelation(ctx), Concurrency: run.Settings.Concurrency}
	draft, err := s.app.runReplyWithFailurePolicy(ctx, input, run)
	if err != nil {
		return draft, err
	}
	draft.LifeRevision = projection.LifeContextRevision
	draft.FactsRevision = projection.CurrentFactsRevision
	if len(draft.MediaRequests) > 0 {
		return draft, nil
	}
	if strings.TrimSpace(draft.Text) == "" {
		return draft, errors.New("reply_text_missing")
	}
	return draft, nil
}

func replyEvidenceEntry(inv CapabilityInvocation, live ContextProjection, ref string) (ContextReference, bool) {
	current, ok := live.ReferenceIndex.ByRef[ref]
	if !ok {
		return ContextReference{}, false
	}
	if inv.Metadata.Source != "model_tool" {
		return current, true
	}
	var offered ContextReferenceIndex
	if err := json.Unmarshal(jsonBytes(inv.ContextSnapshot["context_reference_index"]), &offered); err != nil {
		return ContextReference{}, false
	}
	original, ok := offered.ByRef[ref]
	if !ok || offered.FluctlightID != inv.Metadata.FluctlightID || offered.ConversationID != inv.Metadata.ConversationID || original.Kind != current.Kind || original.EntityID != current.EntityID || original.Revision != current.Revision || jsonString(decodeObject(original.Snapshot)) != jsonString(decodeObject(current.Snapshot)) {
		return ContextReference{}, false
	}
	return original, true
}
