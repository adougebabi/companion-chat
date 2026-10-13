package core

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Opaque action references are issued by this reply, never by a model. Asset
// completion and review are separate facts; ready bytes are not visual evidence.
type replyActionState struct {
	Ref       string `json:"action_result_ref"`
	Kind      string `json:"kind"`
	Intent    string `json:"intent"`
	Status    string `json:"status"`
	ErrorCode string `json:"error_code,omitempty"`
	AssetID   string `json:"-"`
}

func replyActionRef(draft string, index int) string {
	return "reply_action_" + stableDigest(jsonString([]any{draft, index}))
}

func pendingReplyDraft(results []CapabilityResult) string {
	for i := len(results) - 1; i >= 0; i-- {
		result := results[i]
		if result.CapabilityName == conversationReplyCapabilityName && result.Status == "accepted" {
			output := mapValue(result.Output)
			if state := stringValue(output["reply_status"]); state == "needs_action" || state == "waiting" {
				return stringValue(output["draft_ref"])
			}
		}
	}
	return ""
}

func (a *App) readReplyActions(ctx context.Context, draft string) ([]replyActionState, error) {
	rows, err := a.DB.Pool().Query(ctx, `SELECT r.ref,r.kind,r.intent,r.status,COALESCE(r.error_code,''),COALESCE(m.status,''),COALESCE(a.id,'') FROM public.conversation_reply_actions r LEFT JOIN public.media_intents m ON m.id=r.media_intent_id LEFT JOIN public.media_assets a ON a.id='asset_'||m.id AND a.status='ready' WHERE r.draft_id=$1 ORDER BY r.request_index`, draft)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	states := []replyActionState{}
	for rows.Next() {
		var state replyActionState
		var mediaStatus string
		if err := rows.Scan(&state.Ref, &state.Kind, &state.Intent, &state.Status, &state.ErrorCode, &mediaStatus, &state.AssetID); err != nil {
			return nil, err
		}
		if state.Status == "bound" {
			switch mediaStatus {
			case "completed":
				if state.AssetID != "" {
					state.Status = "ready"
				} else {
					state.Status = "failed"
					state.ErrorCode = "media_asset_missing"
				}
			case "failed", "cancelled":
				state.Status = "failed"
				state.ErrorCode = "media_generation_failed"
			default:
				state.Status = "pending"
			}
		}
		states = append(states, state)
	}
	return states, rows.Err()
}

func replyActionsPending(states []replyActionState) bool {
	for _, state := range states {
		if state.Status == "requested" || state.Status == "pending" {
			return true
		}
	}
	return false
}

func validateReplyActionRefs(refs []string, states []replyActionState) error {
	allowed := map[string]bool{}
	for _, state := range states {
		allowed[state.Ref] = true
	}
	for _, ref := range refs {
		if !allowed[ref] {
			return newCapabilityError("reply_action_scope_invalid", false, ErrUnauthorized)
		}
	}
	return nil
}

func (c factOnlyReplyCapability) acceptMediaTx(ctx context.Context, tx pgx.Tx, inv CapabilityInvocation, target DirectToolTarget, draft ReplyDraft, ref string) (CapabilityResult, error) {
	if err := requireConversationPublicationOwnershipTx(ctx, tx, target.AuthorizationActorID, target.FluctlightID, target.ConversationID); err != nil {
		return CapabilityResult{}, err
	}
	if _, _, err := resolveConversationPublicationSourceTx(ctx, tx, target.FluctlightID, target.ConversationID, canonicalConversationPublicationCorrelation(inv.Metadata.CorrelationID)); err != nil {
		return CapabilityResult{}, err
	}
	if draft.FactsRevision != "" {
		if err := c.service.app.requireCurrentFactsRevisionTx(ctx, tx, target.FluctlightID, draft.FactsRevision); err != nil {
			return CapabilityResult{}, err
		}
	}
	requests := []any{}
	for index, request := range draft.MediaRequests {
		action := replyActionRef(ref, index)
		state := "requested"
		errorCode := ""
		if request.Kind == "sticker" {
			state = "unavailable"
			errorCode = "sticker_capability_unavailable"
		}
		if _, err := tx.Exec(ctx, `INSERT INTO public.conversation_reply_actions(ref,draft_id,request_index,kind,intent,status,error_code) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(draft_id,request_index) DO NOTHING`, action, ref, index, request.Kind, request.Intent, state, nullableString(errorCode)); err != nil {
			return CapabilityResult{}, err
		}
		var actualStatus string
		if err := tx.QueryRow(ctx, `SELECT status FROM public.conversation_reply_actions WHERE ref=$1`, action).Scan(&actualStatus); err != nil {
			return CapabilityResult{}, err
		}
		state = actualStatus
		requests = append(requests, map[string]any{"kind": request.Kind, "intent": request.Intent, "media_request_ref": action, "action_result_ref": action, "status": state, "error_code": errorCode})
	}
	execution := replyExecutionScope(c.run, inv)
	var inboundSequence int
	if err := tx.QueryRow(ctx, `SELECT COALESCE(max(sequence),0) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='user'`, target.ConversationID).Scan(&inboundSequence); err != nil {
		return CapabilityResult{}, err
	}
	execution["inbound_sequence"] = inboundSequence
	execution["cancellation_marker"] = providerCancellationMarker(ctx)
	if _, err := tx.Exec(ctx, `UPDATE public.conversation_reply_drafts SET resume_attempts=GREATEST(resume_attempts,1),status=CASE WHEN status IN ('prepared','needs_action') THEN 'needs_action' ELSE status END,execution_scope=$2,updated_at=now() WHERE id=$1 AND status IN ('prepared','needs_action','waiting_media')`, ref, jsonBytes(execution)); err != nil {
		return CapabilityResult{}, err
	}
	replyStatus := "waiting"
	for _, raw := range requests {
		if stringValue(mapValue(raw)["status"]) == "requested" || stringValue(mapValue(raw)["status"]) == "unavailable" || stringValue(mapValue(raw)["status"]) == "failed" {
			replyStatus = "needs_action"
		}
	}
	return CapabilityResult{CallID: inv.CallID, CapabilityName: inv.CapabilityName, Status: "accepted", ProviderRequestID: inv.ProviderRequestID, Output: map[string]any{"reply_status": replyStatus, "draft_ref": ref, "media_requests": requests, "reply_mode": draft.Mode, "reply_model_ref": draft.ModelRef, "config_version": draft.ConfigVersion}}, nil
}

func replyExecutionScope(run *privateReplyRun, inv CapabilityInvocation) map[string]any {
	scope := run.Scope
	scope.WorkingProfileID = inv.Metadata.WorkingProfileID
	scope.CorrelationID = canonicalConversationPublicationCorrelation(inv.Metadata.CorrelationID)
	scope.EvidenceID = inv.SourceFactID

	scope.Arguments = nil
	scope.FrozenContextSnapshot = nil
	args := decodeObject(inv.Arguments)
	return map[string]any{"scope": scope, "current_input": run.CurrentInput, "topic_key": args["topic_key"], "purpose": args["purpose"]}
}

// The wrapper delegates capture/Kev/planner to the real image capability. It
// only fixes the requested intent, operation identity and completion owner.
type replyImageCapability struct {
	imageGenerateCapability
	app *App
	run *privateReplyRun
}

func (c replyImageCapability) Definition() CapabilityDefinition {
	d := c.imageGenerateCapability.Definition()
	d.InputSchema = cloneMap(d.InputSchema)
	props := cloneMap(mapValue(d.InputSchema["properties"]))
	props["draft_ref"] = stringSchema()
	props["media_request_ref"] = stringSchema()
	d.InputSchema["properties"] = props
	d.Version = "reply_media.v1"
	d.Description += " For a reply media request, supply its exact intent, draft_ref and media_request_ref; it is generated once and attached by the reply owner."
	return d
}

// Preflight belongs inside Prepare so a real unavailable result is persisted
// for the Writer to adjust, including independent direct callers.
func (c replyImageCapability) Preflight(context.Context, CapabilityContext) error { return nil }
func (c replyImageCapability) PreflightTx(context.Context, pgx.Tx) error          { return nil }
func (c replyImageCapability) Prepare(ctx context.Context, inv CapabilityInvocation, resolved CapabilityContext) (CapabilityInvocation, error) {
	args := decodeObject(inv.Arguments)
	draftRef := stringValue(args["draft_ref"])
	actionRef := stringValue(args["media_request_ref"])
	if draftRef == "" && actionRef == "" {
		c.run.mu.Lock()
		active := c.run.DraftRef != ""
		c.run.mu.Unlock()
		if active {
			return inv, newCapabilityError("reply_media_binding_required", false, ErrInvalidArguments)
		}
		return c.imageGenerateCapability.Prepare(ctx, inv, resolved)
	}
	expected := "reply_draft_" + stableDigest(privateReplyLogicalKey(ctx, inv))
	if draftRef != expected || actionRef == "" {
		return inv, newCapabilityError("reply_media_scope_invalid", false, ErrUnauthorized)
	}
	var intent, status string
	err := c.app.DB.Pool().QueryRow(ctx, `SELECT r.intent,r.status FROM public.conversation_reply_actions r JOIN public.conversation_reply_drafts d ON d.id=r.draft_id WHERE r.ref=$1 AND d.id=$2 AND d.fluctlight_id=$3 AND d.conversation_id=$4 AND d.authorization_actor_id=$5 AND d.status IN ('needs_action','waiting_media') AND d.expires_at>now()`, actionRef, draftRef, inv.Metadata.FluctlightID, inv.Metadata.ConversationID, inv.Metadata.AuthorizationActorID).Scan(&intent, &status)
	if err != nil {
		return inv, newCapabilityError("reply_media_scope_invalid", false, err)
	}
	if strings.TrimSpace(stringValue(args["intent"])) != intent {
		return inv, newCapabilityError("reply_media_intent_changed", false, ErrInvalidArguments)
	}
	if status == "failed" || status == "unavailable" {
		return inv, newCapabilityError("reply_media_action_terminal", false, ErrConflict)
	}
	// Identity is tied to a component, not the parent ToolCall sequence.
	inv.Metadata.OperationID = actionRef
	delete(args, "draft_ref")
	delete(args, "media_request_ref")
	inv.Arguments = jsonBytes(args)
	if status == "bound" {
		return withCapabilityPreparedData(inv, "reply_media_binding", map[string]any{"draft_ref": draftRef, "action_ref": actionRef, "replay": true})
	}
	prepared, err := c.imageGenerateCapability.Prepare(ctx, inv, resolved)
	if err == nil {
		err = c.imageGenerateCapability.Preflight(ctx, resolved)
	}
	if err != nil {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_, _ = c.app.DB.Pool().Exec(cleanup, `UPDATE public.conversation_reply_actions SET status='failed',error_code='media_prepare_failed' WHERE ref=$1 AND draft_id=$2 AND status='requested'`, actionRef, draftRef)
		return inv, err
	}
	return withCapabilityPreparedData(prepared, "reply_media_binding", map[string]any{"draft_ref": draftRef, "action_ref": actionRef})
}
func (c replyImageCapability) ExecuteDirectTx(ctx context.Context, tx pgx.Tx, inv CapabilityInvocation, resolved CapabilityContext, target DirectToolTarget) (CapabilityResult, error) {
	value, found, err := capabilityPreparedData(inv, "reply_media_binding")
	if err != nil {
		return CapabilityResult{}, err
	}
	if !found {
		return c.imageGenerateCapability.ExecuteDirectTx(ctx, tx, inv, resolved, target)
	}
	binding := mapValue(value)
	ref := stringValue(binding["action_ref"])
	draft := stringValue(binding["draft_ref"])
	if target.Kind != "conversation" || target.Ref != target.ConversationID {
		return CapabilityResult{}, ErrUnauthorized
	}
	if err := requireConversationPublicationOwnershipTx(ctx, tx, target.AuthorizationActorID, target.FluctlightID, target.ConversationID); err != nil {
		return CapabilityResult{}, err
	}
	if _, _, err := resolveConversationPublicationSourceTx(ctx, tx, target.FluctlightID, target.ConversationID, canonicalConversationPublicationCorrelation(inv.Metadata.CorrelationID)); err != nil {
		return CapabilityResult{}, err
	}
	var bound string
	if err := tx.QueryRow(ctx, `SELECT COALESCE(r.media_intent_id,'') FROM public.conversation_reply_actions r JOIN public.conversation_reply_drafts d ON d.id=r.draft_id WHERE r.ref=$1 AND d.id=$2 AND d.authorization_actor_id=$3 AND d.fluctlight_id=$4 AND d.conversation_id=$5 AND d.status IN ('needs_action','waiting_media') AND d.expires_at>now() FOR UPDATE OF r,d`, ref, draft, target.AuthorizationActorID, target.FluctlightID, target.ConversationID).Scan(&bound); err != nil {
		return CapabilityResult{}, err
	}
	if bound != "" {
		return CapabilityResult{CallID: inv.CallID, CapabilityName: inv.CapabilityName, Status: "accepted", ProviderRequestID: inv.ProviderRequestID, Output: map[string]any{"media_intent_id": bound, "target_kind": target.Kind, "target_ref": target.Ref, "status": "pending", "replayed": true}}, nil
	}
	result, err := c.imageGenerateCapability.ExecuteDirectTx(ctx, tx, inv, resolved, target)
	if err != nil {
		return result, err
	}
	intent := stringValue(mapValue(result.Output)["media_intent_id"])
	if _, err := tx.Exec(ctx, `UPDATE public.conversation_reply_actions SET media_intent_id=$2,status='bound' WHERE ref=$1 AND media_intent_id IS NULL`, ref, intent); err != nil {
		return result, err
	}
	if _, err := tx.Exec(ctx, `UPDATE public.conversation_reply_drafts SET status='waiting_media',updated_at=now() WHERE id=$1`, draft); err != nil {
		return result, err
	}
	return result, nil
}

// Resume only terminal real components. The original generated prose stays
// private; one result-aware Writer adjustment is cached before publication.
func (s *factOnlyReplyService) resumeMedia(ctx context.Context, inv CapabilityInvocation, run *privateReplyRun, stored ReplyDraft, ref string, refs []string) (ReplyDraft, error) {
	if bridge, ok := adkCapabilityContext(ctx); ok && bridge.Trace != nil {
		invocations, results := bridge.Trace.Snapshot()
		failed := map[string]bool{}
		for _, result := range results {
			if result.CapabilityName == "media.image.generate" && (result.Status == "failed" || result.Status == "rejected") {
				failed[result.CallID] = true
			}
		}
		for _, action := range invocations {
			args := decodeObject(action.Arguments)
			if failed[action.CallID] && stringValue(args["draft_ref"]) == ref {
				if _, err := s.app.DB.Pool().Exec(ctx, `UPDATE public.conversation_reply_actions SET status='failed',error_code='media_action_failed' WHERE ref=$1 AND draft_id=$2 AND status='requested'`, stringValue(args["media_request_ref"]), ref); err != nil {
					return stored, err
				}
			}
		}
	}
	states, err := s.app.readReplyActions(ctx, ref)
	if err != nil {
		return stored, err
	}
	if err := validateReplyActionRefs(refs, states); err != nil {
		return stored, err
	}
	if len(states) == 0 || replyActionsPending(states) {
		return stored, nil
	}
	if run.Settings.MaxActionHandoffs <= 1 {
		return stored, newCapabilityError("reply_media_handoffs_exhausted", false, ErrConflict)
	}
	owner := randomID("reply_adjust_")
	tag, err := s.app.DB.Pool().Exec(ctx, `UPDATE public.conversation_reply_drafts SET generation_owner=$2,generation_until=now()+make_interval(secs=>$3) WHERE id=$1 AND status IN ('needs_action','waiting_media') AND resume_attempts<$4 AND (generation_owner IS NULL OR generation_until<=now()) AND expires_at>now()`, ref, owner, run.Settings.TimeoutSeconds, run.Settings.MaxActionHandoffs)
	if err != nil {
		return stored, err
	}
	if tag.RowsAffected() != 1 {
		return stored, newCapabilityError("reply_adjustment_in_progress", true, ErrConflict)
	}
	run.mu.Lock()
	run.ActionFacts = states
	run.ActionDraftText = stored.Text
	run.mu.Unlock()
	adjusted, err := s.generate(ctx, inv, run, ref, nil)
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err == nil && len(adjusted.MediaRequests) > 0 {
		err = errors.New("reply_media_handoffs_exhausted")
	}
	if err != nil {
		_, _ = s.app.DB.Pool().Exec(cleanup, `UPDATE public.conversation_reply_drafts SET generation_owner=NULL,generation_until=NULL,resume_attempts=resume_attempts+1,last_error='reply_adjustment_failed' WHERE id=$1 AND generation_owner=$2`, ref, owner)
		return stored, err
	}
	for _, state := range states {
		if state.AssetID != "" {
			adjusted.AttachmentRefs = append(adjusted.AttachmentRefs, state.AssetID)
		}
	}
	tag, err = s.app.DB.Pool().Exec(ctx, `UPDATE public.conversation_reply_drafts SET payload=$3,status='ready',resume_attempts=resume_attempts+1,generation_owner=NULL,generation_until=NULL,updated_at=now() WHERE id=$1 AND generation_owner=$2 AND generation_until>now()`, ref, owner, jsonBytes(adjusted))
	if err == nil && tag.RowsAffected() != 1 {
		err = errors.New("reply_adjustment_lease_lost")
	}
	return adjusted, err
}
