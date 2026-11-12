package core

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5"
)

type momentWriterRunKey struct{}
type momentPublicationScopeKey struct{}

// Moment generation shares the configured persona Writer but owns a separate
// operation namespace. Committed replay is owned by the existing Tool ledger.
// The small run cache avoids repeating preparation in the same live run; it
// makes no claim of a durable private checkpoint before publication.
type factOnlyMomentService struct {
	app        *App
	run        *privateReplyRun
	mu         sync.Mutex
	candidates map[string]momentWriterCandidate
}
type momentWriterCandidate struct {
	draft      ReplyDraft
	argsDigest string
	profileID  string
	err        error
}
type factOnlyMomentCapability struct{ service *factOnlyMomentService }

func (a *App) withMomentWriterRun(ctx context.Context, scope ToolExecutionRequest) (context.Context, error) {
	if prior, _ := ctx.Value(momentWriterRunKey{}).(*factOnlyMomentService); prior != nil {
		if prior.run.Scope.FluctlightID != scope.FluctlightID || prior.run.Scope.AuthorizationActorID != scope.AuthorizationActorID || prior.run.Scope.ConversationID != scope.ConversationID {
			return ctx, ErrUnauthorized
		}
		return ctx, nil
	}
	if a == nil || a.DB == nil || a.DB.Pool() == nil {
		return ctx, nil
	}
	registry := a.capabilityRegistryForContext(ctx)
	definition, found := registry.Definition("moment.publish")
	if !found || scope.Surface != "" && !definition.SupportsSurface(scope.Surface) {
		return ctx, nil
	}
	run := &privateReplyRun{Scope: scope}
	if shared := replyRunFromContext(ctx); shared != nil {
		run.Settings, run.ConfigVersion = shared.Settings, shared.ConfigVersion
		run.CurrentInput = shared.CurrentInput
	} else {
		var err error
		run.Settings, run.ConfigVersion, err = a.conversationReplySettings(ctx)
		if err != nil {
			return ctx, err
		}
	}
	if run.Settings.Mode == "direct" {
		// Direct private chat does not change the independent Moment task.
		run.Settings.Mode = "fact_only"
	}
	if run.Settings.Mode != "fact_only" {
		return ctx, nil
	}
	a.freezePersonaWriterAssignment(ctx, run, FormalAgentConversationReply, "moment")
	if shared := replyRunFromContext(ctx); shared != nil && shareMomentWriterAssignment(run.Assignment, run.AssignmentError, shared.Assignment, shared.AssignmentError) {
		// Reuse the frozen value only when both tasks selected the same source.
		// A private/proactive variant override is not Moment's default.
		run.Assignment, run.AssignmentError = shared.Assignment, shared.AssignmentError
	}
	if scope.TargetKind == "owner_moment_request" {
		var err error
		run.CurrentInput, err = a.ownerMomentWriterInput(ctx, scope)
		if err != nil {
			return ctx, err
		}
	}
	service := &factOnlyMomentService{app: a, run: run, candidates: map[string]momentWriterCandidate{}}
	view, err := capabilityRegistryView(registry, factOnlyMomentCapability{service: service})
	if err != nil {
		return ctx, err
	}
	return withCapabilityRunRegistry(context.WithValue(ctx, momentWriterRunKey{}, service), view), nil
}

func shareMomentWriterAssignment(moment providerAssignment, momentErr error, shared providerAssignment, sharedErr error) bool {
	return momentErr == nil && sharedErr == nil && moment.SourceBindingRole != "" && moment.SourceBindingRole == shared.SourceBindingRole
}

func (c factOnlyMomentCapability) Definition() CapabilityDefinition {
	d := momentPublishCapabilityDefinition()
	d.Version = "fact_only.v1"
	d.Description = "Create and publish a NEW feed Moment using the configured independent persona reply model and authoritative current context. To read a previous post use moment.inspect. evidence_refs is optional; use only offered actionable refs, never historical source_refs. Do not provide text, a draft, tone or social strategy. The committed Writer text returns in this result."
	d.InputSchema = objectSchema(map[string]any{"evidence_refs": map[string]any{"type": "array", "items": stringSchema()}}, nil, false)
	d.ModelResultOmitFields = []string{"target_ref", "replayed"}
	d.SupportsCancel = true
	return d
}
func (factOnlyMomentCapability) RequiredContext() []ContextSlot { return nil }
func (factOnlyMomentCapability) Execute(_ context.Context, inv CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	return executeToolRequired(inv)
}
func (c factOnlyMomentCapability) Prepare(ctx context.Context, inv CapabilityInvocation, _ CapabilityContext) (CapabilityInvocation, error) {
	if c.service == nil {
		return inv, newCapabilityError("moment_writer_unavailable", false, ErrUnauthorized)
	}
	draft, err := c.service.prepare(ctx, inv)
	if err != nil {
		return inv, err
	}
	prepared, err := withCapabilityPreparedData(inv, "fact_only_moment", draft)
	if err != nil {
		return inv, err
	}
	return c.service.prepareUnattachedMediaClaims(ctx, prepared, draft)
}

// Matching claims to the contents of Owner-selected images still needs its
// own evidence contract and is not asserted here.
func (s *factOnlyMomentService) prepareUnattachedMediaClaims(ctx context.Context, inv CapabilityInvocation, draft ReplyDraft) (CapabilityInvocation, error) {
	surface := "moment_without_attachment"
	hasCurrentImage := false
	checkerInput := s.run.CurrentInput
	if s.run.Scope.TargetKind == "owner_moment_request" {
		var raw []byte
		err := s.app.DB.Pool().QueryRow(ctx, `SELECT payload FROM public.owner_surface_records WHERE id=$1 AND owner_actor_id=$2 AND fluctlight_id=$3 AND kind='owner_request' AND status='pending' AND payload->>'kind'='moment_suggestion' AND payload->>'source_fact_id'=$4`, s.run.Scope.TargetRef, inv.Metadata.AuthorizationActorID, inv.Metadata.FluctlightID, inv.SourceFactID).Scan(&raw)
		if err != nil {
			return inv, err
		}
		request := mapValue(decodeObject(raw)["payload"])
		// The checker needs the actual suggestion text, not Core asset IDs.
		checkerInput = stringValue(request["text"])
		selected := stringList(request["media_ids"])
		assets, hasImage, err := s.app.selectedMomentMedia(ctx, s.app.DB.Pool(), inv.Metadata.AuthorizationActorID, selected, false)
		if err != nil {
			return inv, err
		}
		inv, err = withCapabilityPreparedData(inv, "moment_owner_selected_media", selected)
		if err != nil {
			return inv, err
		}
		inv, err = withCapabilityPreparedData(inv, "moment_owner_selected_assets", assets)
		if err != nil {
			return inv, err
		}
		if hasImage {
			surface, hasCurrentImage = "moment_with_attachment", true
		}
		for _, asset := range assets {
			kind := stringValue(asset["kind"])
			if kind == "audio" || kind == "video" {
				surface += ":" + kind
			}
		}
	}
	text, err := momentPublicationText(draft.Text, hasCurrentImage)
	if err != nil || !momentMediaMayNeedHistory(text, checkerInput) {
		return inv, err
	}
	memoryMode := MemoryConversationExact
	if inv.Metadata.ConversationID == "" {
		memoryMode = MemoryConversationGlobalOnly
	}
	subject := firstString(inv.Metadata.SubjectActorID, inv.Metadata.AuthorizationActorID)
	projection, err := s.app.BuildContextProjectionFor(ctx, ContextProjectionRequest{AuthorizationActorID: inv.Metadata.AuthorizationActorID, SpeakerActorID: subject, SourceFactID: inv.SourceFactID, FluctlightID: inv.Metadata.FluctlightID, ConversationID: inv.Metadata.ConversationID, WorkingProfileID: inv.Metadata.WorkingProfileID, CurrentUserText: checkerInput, MemoryOperation: MemoryForConversation, MemoryConversationMode: memoryMode})
	if err != nil {
		return inv, err
	}
	if !directMediaDeliveryCheckNeeded(text, checkerInput, projection.RecentMessages) && !momentMediaCheckNeeded(text, checkerInput, projection.RecentMessages) {
		return inv, nil
	}
	var deliveries []directMediaDelivery
	if inv.Metadata.ConversationID != "" {
		deliveries, err = directMediaDeliveries(ctx, s.app.DB.Pool(), inv.Metadata.AuthorizationActorID, subject, inv.Metadata.FluctlightID, inv.Metadata.ConversationID, false)
		if err != nil {
			return inv, newCapabilityError("direct_media_delivery_check_failed", true, err)
		}
	}
	momentDeliveries, err := momentMediaDeliveriesForIntents(ctx, s.app.DB.Pool(), inv.Metadata.AuthorizationActorID, inv.Metadata.FluctlightID, nil, false)
	if err != nil {
		return inv, newCapabilityError("direct_media_delivery_check_failed", true, err)
	}
	deliveries = append(deliveries, momentDeliveries...)
	refs, err := s.app.checkDirectMediaDeliveryClaims(ctx, projection, checkerInput, text, deliveries, nil, surface)
	if err != nil {
		return inv, err
	}
	privateRefs, privateIntents, momentRefs, momentIntents := []string{}, []string{}, []string{}, []string{}
	for _, ref := range refs {
		delivery, err := directMediaDeliveryByRef(deliveries, ref)
		if err != nil {
			return inv, newCapabilityError("direct_media_delivery_unverified", false, err)
		}
		if delivery.momentID != "" {
			momentRefs, momentIntents = append(momentRefs, ref), append(momentIntents, delivery.intentID)
		} else {
			privateRefs, privateIntents = append(privateRefs, ref), append(privateIntents, delivery.intentID)
		}
	}
	return withCapabilityPreparedData(inv, "moment_unattached_media_check", map[string]any{"refs": privateRefs, "intents": privateIntents, "moment_refs": momentRefs, "moment_intents": momentIntents})
}
func (s *factOnlyMomentService) prepare(ctx context.Context, inv CapabilityInvocation) (ReplyDraft, error) {
	if err := ctx.Err(); err != nil {
		return ReplyDraft{}, err
	}
	if inv.Metadata.FluctlightID != s.run.Scope.FluctlightID || inv.Metadata.AuthorizationActorID != s.run.Scope.AuthorizationActorID || inv.Metadata.ConversationID != s.run.Scope.ConversationID {
		return ReplyDraft{}, ErrUnauthorized
	}
	if err := validateNativeArguments(inv.Arguments, (factOnlyMomentCapability{}).Definition()); err != nil {
		return ReplyDraft{}, err
	}
	var args struct {
		EvidenceRefs []string `json:"evidence_refs"`
	}
	if err := json.Unmarshal(inv.Arguments, &args); err != nil {
		return ReplyDraft{}, err
	}
	key := "moment_draft_" + stableDigest(strings.Join([]string{inv.Metadata.FluctlightID, inv.Metadata.AuthorizationActorID, capabilityOperationID(inv)}, "\x1f"))
	digest := momentWriterRequestDigest(inv)
	s.mu.Lock()
	defer s.mu.Unlock()
	if candidate, found := s.candidates[key]; found {
		if candidate.argsDigest != digest {
			return ReplyDraft{}, ErrConflict
		}
		if candidate.profileID != inv.Metadata.WorkingProfileID {
			return ReplyDraft{}, newCapabilityError("moment_profile_changed", false, ErrLifeContextStale)
		}
	}
	reader := newFactOnlyReplyService(s.app)
	projection, facts, includedEntries, err := reader.resolveReplyEvidence(ctx, inv, s.run, args.EvidenceRefs)
	if err != nil {
		return ReplyDraft{}, err
	}
	if candidate, found := s.candidates[key]; found {
		return candidate.draft, candidate.err
	}
	input, err := reader.writerTaskInput(ctx, inv, s.run, key, projection, facts, replySceneMoment)
	if err != nil {
		return ReplyDraft{}, err
	}
	input.Context.momentSelectedRelationshipRefs = make(map[string]struct{})
	for _, entry := range includedEntries {
		if entry.Kind == ContextReferenceRelationship && entry.Ref != "" {
			input.Context.momentSelectedRelationshipRefs[entry.Ref] = struct{}{}
		}
	}
	input.CorrelationID = "moment-writer:" + stableDigest(key+inv.ProviderRequestID)
	draft, err := s.app.runReplyWithFailurePolicy(ctx, input, s.run)
	if err == nil && len(draft.MediaRequests) > 0 {
		err = errors.New("moment_writer_media_request_invalid")
	}
	draft.LifeRevision, draft.FactsRevision = projection.LifeContextRevision, projection.CurrentFactsRevision
	if s.candidates == nil {
		s.candidates = map[string]momentWriterCandidate{}
	}
	s.candidates[key] = momentWriterCandidate{draft: draft, argsDigest: digest, profileID: inv.Metadata.WorkingProfileID, err: err}
	return draft, err
}
func (c factOnlyMomentCapability) ExecuteDirectTx(ctx context.Context, tx pgx.Tx, inv CapabilityInvocation, _ CapabilityContext, target DirectToolTarget) (CapabilityResult, error) {
	if err := ctx.Err(); err != nil {
		return failedCapabilityResult(inv, "moment_cancelled", false), err
	}
	value, found, err := capabilityPreparedData(inv, "fact_only_moment")
	if err != nil || !found {
		return failedCapabilityResult(inv, "moment_prepared_missing", false), errors.New("moment_prepared_missing")
	}
	var draft ReplyDraft
	if err := json.Unmarshal(jsonBytes(value), &draft); err != nil {
		return failedCapabilityResult(inv, "moment_prepared_invalid", false), err
	}
	if err := c.service.app.requireCurrentFactsRevisionTx(ctx, tx, target.FluctlightID, draft.FactsRevision); err != nil {
		return failedCapabilityResult(inv, "moment_context_stale", false), err
	}
	if _, err := c.service.app.requireLifeContextRevisionTx(ctx, tx, target.FluctlightID, draft.LifeRevision, c.service.app.now().UTC()); err != nil {
		return failedCapabilityResult(inv, "moment_context_stale", false), err
	}
	if err := requireMomentHistoricalMediaTx(ctx, tx, inv, target); err != nil {
		code, retryable := capabilityErrorInfo(err, "moment_media_delivery_stale", false)
		return failedCapabilityResult(inv, code, retryable), err
	}
	resource, err := c.service.app.publishInvocationMomentTx(ctx, tx, inv, target, draft.Text)
	if err != nil {
		code, retryable := publicationCapabilityError(err, "moment_publication_failed")
		return failedCapabilityResult(inv, code, retryable), newCapabilityError(code, retryable, err)
	}
	if err := c.service.app.requireMomentSelectedMediaTx(ctx, tx, inv, target, resource.ID); err != nil {
		code, retryable := capabilityErrorInfo(err, "moment_media_check_failed", true)
		return failedCapabilityResult(inv, code, retryable), err
	}
	return CapabilityResult{CallID: inv.CallID, CapabilityName: inv.CapabilityName, Status: "completed", ProviderRequestID: inv.ProviderRequestID, CorrelationID: "moment:" + resource.ID,
		Output: map[string]any{"text": resource.Text, "target_kind": "moment", "target_ref": resource.ID, "replayed": resource.Replayed}}, nil
}

func requireMomentHistoricalMediaTx(ctx context.Context, tx pgx.Tx, inv CapabilityInvocation, target DirectToolTarget) error {
	value, found, err := capabilityPreparedData(inv, "moment_unattached_media_check")
	if err != nil || !found {
		return err
	}
	proof := mapValue(value)
	if err := requireDirectMediaDeliveriesTx(ctx, tx, target.AuthorizationActorID, firstString(inv.Metadata.SubjectActorID, target.AuthorizationActorID), target.FluctlightID, target.ConversationID, arrayValue(proof["refs"]), arrayValue(proof["intents"])); err != nil {
		return directMediaDeliveryTxError(err)
	}
	if err := requireMomentMediaDeliveriesTx(ctx, tx, target.AuthorizationActorID, target.FluctlightID, arrayValue(proof["moment_refs"]), arrayValue(proof["moment_intents"])); err != nil {
		return directMediaDeliveryTxError(err)
	}
	return nil
}

func (a *App) requireMomentSelectedMediaTx(ctx context.Context, tx pgx.Tx, inv CapabilityInvocation, target DirectToolTarget, momentID string) error {
	selected, found, err := capabilityPreparedData(inv, "moment_owner_selected_media")
	if err != nil || !found {
		return err
	}
	if target.Kind != "owner_moment_request" {
		return newCapabilityError("moment_media_delivery_stale", false, ErrConflict)
	}
	var actual []byte
	if err := tx.QueryRow(ctx, `SELECT media_asset_ids FROM public.moments WHERE id=$1 AND owner_fluctlight_id=$2`, momentID, target.FluctlightID).Scan(&actual); err != nil {
		return err
	}
	if jsonString(decodeArray(actual)) != jsonString(arrayValue(selected)) {
		return newCapabilityError("moment_media_delivery_stale", false, ErrConflict)
	}
	assets, found, err := capabilityPreparedData(inv, "moment_owner_selected_assets")
	if err != nil || !found {
		return err
	}
	actualAssets, _, err := a.selectedMomentMedia(ctx, tx, target.AuthorizationActorID, arrayValue(selected), true)
	if err != nil {
		return directMediaDeliveryTxError(err)
	}
	if jsonString(actualAssets) != jsonString(arrayValue(assets)) {
		return newCapabilityError("moment_media_delivery_stale", false, ErrConflict)
	}
	return nil
}

func momentWriterRequestDigest(inv CapabilityInvocation) string {
	return stableDigest(jsonString(map[string]any{"arguments": decodeObject(inv.Arguments), "authorization_actor_id": inv.Metadata.AuthorizationActorID, "subject_actor_id": inv.Metadata.SubjectActorID, "source_fact_id": inv.SourceFactID, "conversation_id": inv.Metadata.ConversationID, "output_binding": inv.Metadata.OutputBinding, "surface": inv.Metadata.Surface, "working_profile_id": inv.Metadata.WorkingProfileID}))
}
