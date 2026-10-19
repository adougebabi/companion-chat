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
		run.Settings, run.ConfigVersion, run.Assignment, run.AssignmentError = shared.Settings, shared.ConfigVersion, shared.Assignment, shared.AssignmentError
		run.CurrentInput = shared.CurrentInput
	} else {
		var err error
		run.Settings, run.ConfigVersion, err = a.conversationReplySettings(ctx)
		if err != nil {
			return ctx, err
		}
		if run.Settings.Mode == "fact_only" {
			a.freezePersonaWriterAssignment(ctx, run)
		}
	}
	if run.Settings.Mode != "fact_only" {
		return ctx, nil
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

func (c factOnlyMomentCapability) Definition() CapabilityDefinition {
	d := momentPublishCapabilityDefinition()
	d.Version = "fact_only.v1"
	d.Description = "Generate and publish one feed Moment using the configured independent persona reply model and authoritative current context. evidence_refs is optional; use only offered actionable refs, never historical source_refs. Do not provide text, a draft, tone or social strategy. The committed Writer text returns in this result."
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
	return withCapabilityPreparedData(inv, "fact_only_moment", draft)
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
	projection, facts, err := reader.resolveReplyEvidence(ctx, inv, s.run, args.EvidenceRefs)
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
	resource, err := c.service.app.publishInvocationMomentTx(ctx, tx, inv, target, draft.Text)
	if err != nil {
		code, retryable := publicationCapabilityError(err, "moment_publication_failed")
		return failedCapabilityResult(inv, code, retryable), newCapabilityError(code, retryable, err)
	}
	return CapabilityResult{CallID: inv.CallID, CapabilityName: inv.CapabilityName, Status: "completed", ProviderRequestID: inv.ProviderRequestID, CorrelationID: "moment:" + resource.ID,
		Output: map[string]any{"text": resource.Text, "target_kind": "moment", "target_ref": resource.ID, "replayed": resource.Replayed}}, nil
}

func momentWriterRequestDigest(inv CapabilityInvocation) string {
	return stableDigest(jsonString(map[string]any{"arguments": decodeObject(inv.Arguments), "authorization_actor_id": inv.Metadata.AuthorizationActorID, "subject_actor_id": inv.Metadata.SubjectActorID, "source_fact_id": inv.SourceFactID, "conversation_id": inv.Metadata.ConversationID, "output_binding": inv.Metadata.OutputBinding, "surface": inv.Metadata.Surface, "working_profile_id": inv.Metadata.WorkingProfileID}))
}
