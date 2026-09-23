package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// The built-ins are direct Capability implementations. Provider codec values
// are converted to CapabilityInvocation before reaching this file; results
// leave the file as CapabilityResult without a second executor facade.

type imageCapabilityService interface {
	preflightImageCapability(context.Context) error
	preflightImageCapabilityTx(context.Context, pgx.Tx) error
	createMediaIntentTargetTx(context.Context, pgx.Tx, string, map[string]any, string, string, string, string, string, string) error
	requireLifeContextRevisionTx(context.Context, pgx.Tx, string, string, time.Time) (map[string]any, error)
}
type visualIdentityCapabilityService interface {
	EnsureVisualIdentityInitializationWithPersona(context.Context, string, string, string, map[string]any) (string, error)
	EnsureVisualIdentityInitializationWithPersonaTx(context.Context, pgx.Tx, string, string, string, map[string]any) (string, error)
}
type sceneCapabilityService interface {
	prepareSceneCapability(context.Context, CapabilityInvocation, CapabilityContext) (CapabilityInvocation, error)
	applySceneCapability(context.Context, CapabilityInvocation, CapabilityContext) (CapabilityResult, error)
	applySceneCapabilityTx(context.Context, pgx.Tx, CapabilityInvocation, CapabilityContext) (CapabilityResult, error)
}
type presenceCapabilityService interface {
	preparePresenceCapability(context.Context, CapabilityInvocation, CapabilityContext) (CapabilityInvocation, error)
	applyPresenceCapability(context.Context, CapabilityInvocation, CapabilityContext) (CapabilityResult, error)
	applyPresenceCapabilityTx(context.Context, pgx.Tx, CapabilityInvocation, CapabilityContext) (CapabilityResult, error)
}
type memoryCapabilityService interface {
	prepareMemoryCapability(context.Context, CapabilityInvocation, CapabilityContext) (CapabilityInvocation, error)
	applyMemoryCapability(context.Context, CapabilityInvocation, CapabilityContext) (CapabilityResult, error)
	applyMemoryCapabilityTx(context.Context, pgx.Tx, CapabilityInvocation, CapabilityContext) (CapabilityResult, error)
}
type activeMemoryCapabilityService interface {
	prepareActiveMemoryCapability(context.Context, CapabilityInvocation, CapabilityContext) (CapabilityInvocation, error)
	applyActiveMemoryCapability(context.Context, CapabilityInvocation, CapabilityContext) (CapabilityResult, error)
	applyActiveMemoryCapabilityTx(context.Context, pgx.Tx, CapabilityInvocation, CapabilityContext) (CapabilityResult, error)
}
type affectCapabilityService interface {
	applyAffectEvent(context.Context, string, string, normalizedAffectEvent, map[string]any) (map[string]any, error)
	applyAffectEventTx(context.Context, pgx.Tx, string, string, normalizedAffectEvent, map[string]any) (map[string]any, error)
}

type conversationReplyCapability struct{ publication *ToolPublicationService }
type momentPublishCapability struct{ publication *ToolPublicationService }
type imageGenerateCapability struct {
	service     imageCapabilityService
	publication *ToolPublicationService
}
type visualIdentityInitializeCapability struct {
	service visualIdentityCapabilityService
}
type sceneEventCapability struct{ service sceneCapabilityService }
type presenceEventCapability struct{ service presenceCapabilityService }
type scheduleReplanCapability struct {
	planner SchedulePlanner
	apply   func(context.Context, CapabilityInvocation) (CapabilityResult, error)
	applyTx func(context.Context, pgx.Tx, CapabilityInvocation, CapabilityContext) (CapabilityResult, error)
}
type memoryEventCapability struct{ service memoryCapabilityService }
type activeMemoryEventCapability struct{ service activeMemoryCapabilityService }
type memoryRecallCapability struct{ service MemoryRecallService }
type personaDetailCapability struct{ service personaDetailService }
type affectEventCapability struct{ service affectCapabilityService }
type relationshipLookupCapability struct{ service *relationshipLookupService }
type capabilityRequestCapability struct{ service *capabilityRequestService }

func builtinCapabilities(app *App) []Capability {
	var schedule scheduleReplanCapability
	var memory memoryCapabilityService
	var activeMemory activeMemoryCapabilityService
	var affect affectCapabilityService
	if app != nil {
		affect = app
		schedule.planner = app.SchedulePlanner
		if app.Schedule != nil {
			schedule.apply = app.Schedule.ApplyScheduleReplanCapability
			schedule.applyTx = app.Schedule.ApplyScheduleReplanCapabilityTx
		} else {
			schedule.apply = app.applyScheduleReplanCapability
			schedule.applyTx = app.applyScheduleReplanCapabilityTx
		}
		if app.Memory != nil {
			memory = app.Memory
			activeMemory = app.Memory
		} else {
			memory = app
			activeMemory = app
		}
	}
	var image imageCapabilityService
	var visualIdentity visualIdentityCapabilityService
	var lifeScene sceneCapabilityService
	var lifePresence presenceCapabilityService
	if app != nil {
		if app.Media != nil {
			image = app.Media
			visualIdentity = app.Media
		} else {
			image = app
			visualIdentity = app
		}
		if app.LifeContext != nil {
			lifeScene = app.LifeContext
			lifePresence = app.LifeContext
		} else {
			lifeScene = app
			lifePresence = app
		}
	}
	publication := NewToolPublicationService(app)
	personaActions := newPersonaActionService(app)
	capabilities := []Capability{
		conversationReplyCapability{publication: publication}, momentPublishCapability{publication: publication}, imageGenerateCapability{service: image, publication: publication},
		visualIdentityInitializeCapability{service: visualIdentity}, sceneEventCapability{service: lifeScene}, schedule,
		presenceEventCapability{service: lifePresence}, memoryEventCapability{service: memory}, activeMemoryEventCapability{service: activeMemory}, affectEventCapability{service: affect},
		memoryRecallCapability{service: newMemoryRecallService(app)},
		personaDetailCapability{service: newPersonaDetailService(app)},
		relationshipLookupCapability{service: &relationshipLookupService{app: app}},
		capabilityRequestCapability{service: &capabilityRequestService{app: app}},
		personaActionCapability{name: personaTakeoverCapabilityName, service: personaActions},
		personaActionCapability{name: personaSwitchCapabilityName, service: personaActions},
	}
	return append(capabilities, visualIdentityAgentCapabilities(app)...)
}

var (
	_ Capability                   = conversationReplyCapability{}
	_ Capability                   = momentPublishCapability{}
	_ Capability                   = imageGenerateCapability{}
	_ Capability                   = visualIdentityInitializeCapability{}
	_ Capability                   = sceneEventCapability{}
	_ Capability                   = presenceEventCapability{}
	_ Capability                   = scheduleReplanCapability{}
	_ Capability                   = memoryEventCapability{}
	_ Capability                   = activeMemoryEventCapability{}
	_ Capability                   = memoryRecallCapability{}
	_ Capability                   = personaDetailCapability{}
	_ Capability                   = affectEventCapability{}
	_ Capability                   = relationshipLookupCapability{}
	_ Capability                   = capabilityRequestCapability{}
	_ TransactionalCapability      = memoryEventCapability{}
	_ TransactionalCapability      = activeMemoryEventCapability{}
	_ TransactionalCapability      = affectEventCapability{}
	_ TransactionalCapability      = capabilityRequestCapability{}
	_ TransactionalCapability      = sceneEventCapability{}
	_ TransactionalCapability      = presenceEventCapability{}
	_ TransactionalCapability      = scheduleReplanCapability{}
	_ TransactionalCapability      = visualIdentityInitializeCapability{}
	_ CapabilityCandidateValidator = relationshipLookupCapability{}
	_ Capability                   = personaActionCapability{}
	_ TransactionalCapability      = personaActionCapability{}
	_ DirectToolCapability         = conversationReplyCapability{}
	_ DirectToolCapability         = momentPublishCapability{}
	_ DirectToolCapability         = imageGenerateCapability{}
)

func executeToolRequired(invocation CapabilityInvocation) (CapabilityResult, error) {
	err := newCapabilityError("execute_tool_required", false, ErrCapabilityExecution)
	return failedCapabilityResult(invocation, "execute_tool_required", false), err
}

func (c conversationReplyCapability) Definition() CapabilityDefinition {
	return conversationReplyCapabilityDefinition()
}
func (c conversationReplyCapability) RequiredContext() []ContextSlot {
	return []ContextSlot{SlotCurrentLife}
}
func (c conversationReplyCapability) Execute(_ context.Context, invocation CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	return executeToolRequired(invocation)
}

func (c conversationReplyCapability) ExecuteDirectTx(ctx context.Context, tx pgx.Tx, invocation CapabilityInvocation, _ CapabilityContext, target DirectToolTarget) (CapabilityResult, error) {
	if c.publication == nil {
		return failedCapabilityResultDetail(invocation, "conversation_publication_unavailable", true, "conversation publication is unavailable"), errors.New("conversation publication unavailable")
	}
	var args map[string]any
	if err := json.Unmarshal(invocation.Arguments, &args); err != nil {
		return failedCapabilityResult(invocation, "reply_arguments_invalid", false), newCapabilityError("reply_arguments_invalid", false, err)
	}
	text := strings.TrimSpace(stringValue(args["text"]))
	resource, err := c.publication.PublishConversationReplyTx(ctx, tx, ConversationReplyPublication{
		SuppressRecentDuplicate: target.AuthorizationPolicy == "autonomy",
		AuthorizationActorID:    target.AuthorizationActorID,
		FluctlightID:            target.FluctlightID,
		ConversationID:          target.ConversationID,
		OperationID:             capabilityOperationID(invocation),
		CorrelationID:           invocation.Metadata.CorrelationID,
		Text:                    text,
	})
	if err != nil {
		code, retryable := publicationCapabilityError(err, "reply_publication_failed")
		return failedCapabilityResultDetail(invocation, code, retryable, err.Error()), newCapabilityError(code, retryable, err)
	}
	output := map[string]any{"text": text, "target_kind": "conversation_message", "target_ref": resource.ID, "replayed": resource.Replayed}
	if resource.DuplicateSuppressed {
		output["delivery_status"] = "duplicate_suppressed"
	}
	return CapabilityResult{
		CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "completed",
		Output:            output,
		ProviderRequestID: invocation.ProviderRequestID, CorrelationID: "reply:" + resource.ID,
	}, nil
}
func (c momentPublishCapability) Definition() CapabilityDefinition {
	return momentPublishCapabilityDefinition()
}
func (c momentPublishCapability) RequiredContext() []ContextSlot { return nil }
func (c momentPublishCapability) Execute(_ context.Context, invocation CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	return executeToolRequired(invocation)
}

func (c momentPublishCapability) ExecuteDirectTx(ctx context.Context, tx pgx.Tx, invocation CapabilityInvocation, _ CapabilityContext, target DirectToolTarget) (CapabilityResult, error) {
	if c.publication == nil {
		return failedCapabilityResultDetail(invocation, "moment_publication_unavailable", true, "Moment publication is unavailable"), errors.New("moment publication unavailable")
	}
	var args map[string]any
	if err := json.Unmarshal(invocation.Arguments, &args); err != nil {
		return failedCapabilityResult(invocation, "moment_arguments_invalid", false), newCapabilityError("moment_arguments_invalid", false, err)
	}
	text := strings.TrimSpace(stringValue(args["text"]))
	resource, err := c.publication.PublishMomentTx(ctx, tx, MomentPublication{
		FluctlightID: target.FluctlightID, OperationID: capabilityOperationID(invocation),
		CorrelationID: invocation.Metadata.CorrelationID, Text: text,
	})
	if err != nil {
		code, retryable := publicationCapabilityError(err, "moment_publication_failed")
		return failedCapabilityResultDetail(invocation, code, retryable, err.Error()), newCapabilityError(code, retryable, err)
	}
	return CapabilityResult{
		CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "completed",
		Output:            map[string]any{"text": text, "target_kind": "moment", "target_ref": resource.ID, "replayed": resource.Replayed},
		ProviderRequestID: invocation.ProviderRequestID, CorrelationID: "moment:" + resource.ID,
	}, nil
}
func (c imageGenerateCapability) Definition() CapabilityDefinition {
	d := imageCapabilityDefinition()
	d.InputSchema = map[string]any{"type": "object", "additionalProperties": false, "required": []any{"intent"}, "properties": map[string]any{"intent": map[string]any{"type": "string", "minLength": 1, "maxLength": 4000}}}
	d.RequiredContext = []ContextSlot{SlotVisualIdentity, SlotCurrentLife, SlotAppearance, SlotCurrentState}
	return d
}
func (c imageGenerateCapability) RequiredContext() []ContextSlot {
	return c.Definition().RequiredContext
}
func (c imageGenerateCapability) Prepare(_ context.Context, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityInvocation, error) {
	if err := requireCapabilityContext(resolved, SlotVisualIdentity, SlotCurrentLife, SlotAppearance, SlotCurrentState); err != nil {
		return invocation, err
	}
	var args map[string]any
	if err := json.Unmarshal(invocation.Arguments, &args); err != nil {
		return invocation, err
	}
	intent := strings.TrimSpace(stringValue(args["intent"]))
	if intent == "" {
		return invocation, errors.New("media intent is required")
	}
	if rawConcept, found, err := capabilityPreparedData(invocation, "media_concept"); err != nil {
		return invocation, err
	} else if found {
		if err := validatePreparedMediaConcept(mapValue(rawConcept), intent); err != nil {
			return invocation, err
		}
		return invocation, nil
	}
	prepared := map[string]any{"intent": intent, "context_binding": map[string]any{}}
	binding := mapValue(prepared["context_binding"])
	if resolved.Visual != nil {
		binding["visual_identity"] = resolved.Visual.Data
	}
	if resolved.Life != nil {
		binding["current_life"] = resolved.Life.Data
	}
	if resolved.Outfit != nil {
		binding["appearance"] = resolved.Outfit.Data
	}
	if resolved.State != nil {
		binding["current_state"] = resolved.State.Data
	}
	prepared["context_binding"] = binding
	return withCapabilityPreparedData(invocation, "media_concept", prepared)
}

func validatePreparedMediaConcept(concept map[string]any, intent string) error {
	if strings.TrimSpace(stringValue(concept["intent"])) == "" || strings.TrimSpace(stringValue(concept["intent"])) != strings.TrimSpace(intent) {
		return errors.New("prepared media intent does not match provider arguments")
	}
	binding := mapValue(concept["context_binding"])
	for _, key := range []string{"visual_identity", "current_life", "appearance", "current_state"} {
		if len(mapValue(binding[key])) == 0 {
			return fmt.Errorf("prepared media context %q is missing", key)
		}
	}
	return nil
}
func (c imageGenerateCapability) Execute(_ context.Context, invocation CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	return executeToolRequired(invocation)
}

func (c imageGenerateCapability) ExecuteDirectTx(ctx context.Context, tx pgx.Tx, invocation CapabilityInvocation, resolved CapabilityContext, target DirectToolTarget) (CapabilityResult, error) {
	if c.service == nil || c.publication == nil {
		return failedCapabilityResultDetail(invocation, "media_capability_unavailable", true, "media capability is unavailable"), errors.New("media capability unavailable")
	}
	if err := requireCapabilityContext(resolved, SlotVisualIdentity, SlotCurrentLife, SlotAppearance, SlotCurrentState); err != nil {
		return failedCapabilityResult(invocation, "context_resolve_failed", true), err
	}
	preparedConcept, found, err := capabilityPreparedData(invocation, "media_concept")
	if err != nil || !found || len(mapValue(preparedConcept)) == 0 {
		if err == nil {
			err = errors.New("image invocation was not prepared")
		}
		return failedCapabilityResultDetail(invocation, "media_prepare_required", false, err.Error()), newCapabilityError("media_prepare_required", false, err)
	}
	concept := mapValue(preparedConcept)
	contextBinding := mapValue(concept["context_binding"])
	expectedLifeContextRevision := stringValue(mapValue(contextBinding["current_life"])["context_revision"])
	frozenLifeContextRevision := stringValue(resolved.Life.Data["context_revision"])
	if frozenLifeContextRevision == "" || expectedLifeContextRevision != frozenLifeContextRevision {
		err := newCapabilityError("media_prepared_context_mismatch", false, ErrConflict)
		return failedCapabilityResult(invocation, "media_prepared_context_mismatch", false), err
	}
	if _, err := c.service.requireLifeContextRevisionTx(ctx, tx, invocation.Metadata.FluctlightID, frozenLifeContextRevision, time.Now().UTC()); err != nil {
		if errors.Is(err, ErrLifeContextStale) {
			return failedCapabilityResult(invocation, "media_context_stale", false), newCapabilityError("media_context_stale", false, err)
		}
		return failedCapabilityResult(invocation, "media_intent_failed", true), err
	}
	conversationID, messageID, momentID, err := c.publication.ValidateMediaTargetTx(ctx, tx, MediaPublicationTarget{
		AuthorizationActorID: target.AuthorizationActorID, FluctlightID: target.FluctlightID,
		ConversationID: target.ConversationID, Kind: target.Kind, Ref: target.Ref,
	})
	if err != nil {
		code, retryable := publicationCapabilityError(err, "media_target_invalid")
		return failedCapabilityResultDetail(invocation, code, retryable, err.Error()), newCapabilityError(code, retryable, err)
	}
	intentID, workflowID, providerRequestID := mediaInvocationIdentity(invocation)
	replayed, intentStatus, err := c.publication.CreateMediaIntentTx(ctx, tx, c.service, invocation.Metadata.FluctlightID, concept, intentID, workflowID, providerRequestID, conversationID, messageID, momentID)
	if err != nil {
		code, retryable := publicationCapabilityError(err, "media_intent_failed")
		return failedCapabilityResultDetail(invocation, code, retryable, err.Error()), newCapabilityError(code, retryable, err)
	}
	status := "accepted"
	switch intentStatus {
	case "pending", "running":
	case "completed":
		status = "completed"
	case "failed":
		err := newCapabilityError("media_task_failed", true, errors.New("media task previously failed"))
		return failedCapabilityResultDetail(invocation, "media_task_failed", true, err.Error()), err
	default:
		err := newCapabilityError("media_intent_status_invalid", false, fmt.Errorf("unsupported media intent status %q", intentStatus))
		return failedCapabilityResultDetail(invocation, "media_intent_status_invalid", false, err.Error()), err
	}
	return CapabilityResult{
		CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: status,
		Output: map[string]any{
			"media_intent_id": intentID, "task_id": workflowID, "status": intentStatus,
			"target_kind": target.Kind, "target_ref": target.Ref, "replayed": replayed,
		},
		ProviderRequestID: providerRequestID, CorrelationID: "image:" + intentID,
	}, nil
}
func (c imageGenerateCapability) Preflight(ctx context.Context, _ CapabilityContext) error {
	if c.service == nil {
		return errors.New("media capability unavailable")
	}
	return c.service.preflightImageCapability(ctx)
}
func (c imageGenerateCapability) PreflightTx(ctx context.Context, tx pgx.Tx) error {
	if c.service == nil {
		return errors.New("media capability unavailable")
	}
	return c.service.preflightImageCapabilityTx(ctx, tx)
}
func (c visualIdentityInitializeCapability) Definition() CapabilityDefinition {
	return visualIdentityInitializeCapabilityDefinition()
}
func (c visualIdentityInitializeCapability) RequiredContext() []ContextSlot {
	return []ContextSlot{SlotCorePersona, SlotVisualIdentity}
}
func (c visualIdentityInitializeCapability) Execute(ctx context.Context, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityResult, error) {
	if c.service == nil {
		return failedCapabilityResult(invocation, "visual_identity_unavailable", true), errors.New("visual identity unavailable")
	}
	if resolved.Visual == nil || resolved.Persona == nil {
		return failedCapabilityResultDetail(invocation, "context_resolve_failed", true, "visual identity context missing"), errors.New("visual identity context missing")
	}
	if status := stringValue(resolved.Visual.Data["status"]); status == "active" {
		return CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "completed", Output: map[string]any{"session_id": stringValue(resolved.Visual.Data["active_session_id"]), "status": "already_active"}, ProviderRequestID: invocation.ProviderRequestID, CorrelationID: "visual_identity:" + invocation.CallID}, nil
	}
	sessionID, err := c.service.EnsureVisualIdentityInitializationWithPersona(ctx, invocation.Metadata.FluctlightID, visualIdentityTriggerType(invocation), invocation.SourceFactID, resolved.Persona.Data)
	if err != nil {
		return failedCapabilityResult(invocation, "visual_identity_initialization_failed", true), err
	}
	return CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "accepted", Output: map[string]any{"session_id": sessionID, "status": "queued"}, ProviderRequestID: invocation.ProviderRequestID, CorrelationID: "visual_identity:" + sessionID}, nil
}
func (c visualIdentityInitializeCapability) ExecuteTx(ctx context.Context, tx pgx.Tx, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityResult, error) {
	if c.service == nil {
		return failedCapabilityResult(invocation, "visual_identity_unavailable", true), errors.New("visual identity unavailable")
	}
	if resolved.Visual == nil || resolved.Persona == nil {
		return failedCapabilityResultDetail(invocation, "context_resolve_failed", true, "visual identity context missing"), errors.New("visual identity context missing")
	}
	if status := stringValue(resolved.Visual.Data["status"]); status == "active" {
		return CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "completed", Output: map[string]any{"session_id": stringValue(resolved.Visual.Data["active_session_id"]), "status": "already_active"}, ProviderRequestID: invocation.ProviderRequestID, CorrelationID: "visual_identity:" + invocation.CallID}, nil
	}
	sessionID, err := c.service.EnsureVisualIdentityInitializationWithPersonaTx(ctx, tx, invocation.Metadata.FluctlightID, visualIdentityTriggerType(invocation), invocation.SourceFactID, resolved.Persona.Data)
	if err != nil {
		return failedCapabilityResult(invocation, "visual_identity_initialization_failed", true), err
	}
	return CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "accepted", Output: map[string]any{"session_id": sessionID, "status": "queued"}, ProviderRequestID: invocation.ProviderRequestID, CorrelationID: "visual_identity:" + sessionID}, nil
}

func visualIdentityTriggerType(invocation CapabilityInvocation) string {
	if invocation.Metadata.Surface == CapabilitySurfaceWakeUp {
		return "wakeup"
	}
	return "initialization"
}

func (c sceneEventCapability) Definition() CapabilityDefinition {
	return sceneCapabilityDefinition()
}
func (c sceneEventCapability) RequiredContext() []ContextSlot {
	return []ContextSlot{SlotCurrentLife}
}
func (c sceneEventCapability) Prepare(ctx context.Context, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityInvocation, error) {
	if c.service == nil {
		return invocation, errors.New("scene capability unavailable")
	}
	return c.service.prepareSceneCapability(ctx, invocation, resolved)
}
func (c sceneEventCapability) Execute(ctx context.Context, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityResult, error) {
	if c.service == nil {
		return failedCapabilityResultDetail(invocation, "scene_capability_unavailable", true, "scene capability is unavailable"), errors.New("scene capability unavailable")
	}
	if err := requireCapabilityContext(resolved, SlotCurrentLife); err != nil {
		return failedCapabilityResult(invocation, "context_resolve_failed", true), err
	}
	return c.service.applySceneCapability(ctx, invocation, resolved)
}
func (c sceneEventCapability) ExecuteTx(ctx context.Context, tx pgx.Tx, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityResult, error) {
	if c.service == nil {
		return failedCapabilityResultDetail(invocation, "scene_capability_unavailable", true, "scene capability is unavailable"), errors.New("scene capability unavailable")
	}
	if err := requireCapabilityContext(resolved, SlotCurrentLife); err != nil {
		return failedCapabilityResult(invocation, "context_resolve_failed", true), err
	}
	return c.service.applySceneCapabilityTx(ctx, tx, invocation, resolved)
}

func (c presenceEventCapability) Definition() CapabilityDefinition {
	return presenceCapabilityDefinition()
}
func (c presenceEventCapability) RequiredContext() []ContextSlot {
	return []ContextSlot{SlotCurrentLife}
}
func (c presenceEventCapability) Prepare(ctx context.Context, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityInvocation, error) {
	if c.service == nil {
		return invocation, errors.New("presence capability unavailable")
	}
	return c.service.preparePresenceCapability(ctx, invocation, resolved)
}
func (c presenceEventCapability) Execute(ctx context.Context, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityResult, error) {
	if c.service == nil {
		return failedCapabilityResultDetail(invocation, "presence_capability_unavailable", true, "presence capability is unavailable"), errors.New("presence capability unavailable")
	}
	if err := requireCapabilityContext(resolved, SlotCurrentLife); err != nil {
		return failedCapabilityResult(invocation, "context_resolve_failed", true), err
	}
	return c.service.applyPresenceCapability(ctx, invocation, resolved)
}
func (c presenceEventCapability) ExecuteTx(ctx context.Context, tx pgx.Tx, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityResult, error) {
	if c.service == nil {
		return failedCapabilityResultDetail(invocation, "presence_capability_unavailable", true, "presence capability is unavailable"), errors.New("presence capability unavailable")
	}
	if err := requireCapabilityContext(resolved, SlotCurrentLife); err != nil {
		return failedCapabilityResult(invocation, "context_resolve_failed", true), err
	}
	return c.service.applyPresenceCapabilityTx(ctx, tx, invocation, resolved)
}

func (c scheduleReplanCapability) Definition() CapabilityDefinition {
	d := scheduleReplanCapabilityDefinition()
	d.InputSchema = map[string]any{"type": "object", "additionalProperties": false, "required": []any{"intent"}, "properties": map[string]any{"intent": map[string]any{"type": "string", "minLength": 1, "maxLength": 4000}}}
	d.RequiredContext = []ContextSlot{SlotSchedule, SlotCurrentLife, SlotAgency}
	return d
}
func (c scheduleReplanCapability) RequiredContext() []ContextSlot {
	return c.Definition().RequiredContext
}
func (c scheduleReplanCapability) Prepare(ctx context.Context, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityInvocation, error) {
	if err := requireCapabilityContext(resolved, SlotSchedule, SlotCurrentLife, SlotAgency); err != nil {
		return invocation, err
	}
	var args map[string]any
	if err := json.Unmarshal(invocation.Arguments, &args); err != nil {
		return invocation, err
	}
	// A frozen/replayed invocation already contains the capability-local plan.
	// Do not call the planner a second time or produce a different replacement.
	if rawPlan, found, err := capabilityPreparedData(invocation, "schedule_plan"); err != nil {
		return invocation, err
	} else if found {
		if err := validatePreparedSchedulePlan(mapValue(rawPlan), args, resolved); err != nil {
			return invocation, newCapabilityError("schedule_replan_planner_failed", true, err)
		}
		return invocation, nil
	}
	planner := c.planner
	if planner == nil {
		return invocation, newCapabilityError("schedule_replan_planner_failed", true, errors.New("schedule planner is not configured"))
	}
	timezone := stringValue(resolved.Life.Data["timezone"])
	if timezone == "" {
		timezone = stringValue(resolved.Schedule.Data["timezone"])
	}
	planned, err := planner.Plan(ctx, SchedulePlanInput{Intent: stringValue(args["intent"]), Schedule: resolved.Schedule.Data, CurrentLife: resolved.Life.Data, Agency: resolved.Agency.Data, SourceFactID: invocation.SourceFactID, Timezone: timezone})
	if err != nil {
		return invocation, newCapabilityError("schedule_replan_planner_failed", true, err)
	}
	planned["local_date"] = firstString(planned["local_date"], stringValue(resolved.Schedule.Data["local_date"]))
	planned["timezone"] = firstString(planned["timezone"], stringValue(resolved.Schedule.Data["timezone"]))
	planned["expected_revision"] = firstInt(planned["expected_revision"], intValue(resolved.Schedule.Data["revision"]))
	planned["completed_before"] = firstString(planned["completed_before"], stringValue(resolved.Schedule.Data["completed_before"]))
	planned["expected_life_context_revision"] = stringValue(resolved.Life.Data["context_revision"])
	planned["intent"] = stringValue(args["intent"])
	planned["evidence_refs"] = []any{invocation.SourceFactID}
	planned["idempotency_key"] = "tool:" + capabilityOperationID(invocation)
	if err := validatePreparedSchedulePlan(planned, args, resolved); err != nil {
		code := "schedule_replan_planner_failed"
		if errors.Is(err, ErrConflict) {
			code = "schedule_replan_revision_conflict"
		}
		return invocation, newCapabilityError(code, true, err)
	}
	return withCapabilityPreparedData(invocation, "schedule_plan", planned)
}

func validatePreparedSchedulePlan(planned, arguments map[string]any, resolved CapabilityContext) error {
	if strings.TrimSpace(stringValue(planned["intent"])) == "" || strings.TrimSpace(stringValue(planned["intent"])) != strings.TrimSpace(stringValue(arguments["intent"])) {
		return errors.New("prepared schedule intent does not match provider arguments")
	}
	if len(arrayValue(planned["items"])) == 0 {
		return errors.New("schedule planner returned no items")
	}
	if strings.TrimSpace(stringValue(planned["expected_life_context_revision"])) == "" || stringValue(planned["expected_life_context_revision"]) != stringValue(resolved.Life.Data["context_revision"]) {
		return ErrConflict
	}
	for _, field := range []string{"local_date", "timezone", "completed_before", "expected_life_context_revision"} {
		if strings.TrimSpace(stringValue(planned[field])) == "" {
			return fmt.Errorf("schedule planner field %s missing", field)
		}
	}
	policy, policyOK := planned["reschedule_policy"].(map[string]any)
	if intValue(planned["expected_revision"]) <= 0 || !policyOK || policy == nil {
		return errors.New("schedule planner boundary fields missing")
	}
	if resolved.Schedule == nil {
		return fmt.Errorf("%w: schedule context is missing", ErrContextResolve)
	}
	if expected := intValue(resolved.Schedule.Data["revision"]); expected > 0 && intValue(planned["expected_revision"]) != expected {
		return ErrConflict
	}
	return validateScheduleReplanItems(arrayValue(planned["items"]))
}

func (c scheduleReplanCapability) Execute(ctx context.Context, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityResult, error) {
	preparedPlan, found, err := capabilityPreparedData(invocation, "schedule_plan")
	if err != nil {
		return failedCapabilityResult(invocation, "invalid_arguments", false), err
	}
	if !found || len(arrayValue(mapValue(preparedPlan)["items"])) == 0 {
		prepared, err := c.Prepare(ctx, invocation, resolved)
		if err != nil {
			code, retryable := capabilityErrorInfo(err, "schedule_replan_planner_failed", true)
			return failedCapabilityResultDetail(invocation, code, retryable, err.Error()), err
		}
		invocation = prepared
	}
	if c.apply == nil {
		return failedCapabilityResultDetail(invocation, "schedule_replan_persist_failed", true, "schedule service is unavailable"), errors.New("schedule service is unavailable")
	}
	return c.apply(ctx, invocation)
}

func (c scheduleReplanCapability) ExecuteTx(ctx context.Context, tx pgx.Tx, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityResult, error) {
	preparedPlan, found, err := capabilityPreparedData(invocation, "schedule_plan")
	if err != nil {
		return failedCapabilityResult(invocation, "invalid_arguments", false), err
	}
	if !found || len(arrayValue(mapValue(preparedPlan)["items"])) == 0 {
		return failedCapabilityResultDetail(invocation, "schedule_replan_planner_failed", true, "prepared schedule plan is required before transactional apply"), errors.New("prepared schedule plan is required")
	}
	if err := requireCapabilityContext(resolved, SlotSchedule, SlotCurrentLife, SlotAgency); err != nil {
		return failedCapabilityResult(invocation, "context_resolve_failed", true), err
	}
	if c.applyTx == nil {
		return failedCapabilityResultDetail(invocation, "schedule_replan_persist_failed", true, "schedule service is unavailable"), errors.New("schedule service is unavailable")
	}
	return c.applyTx(ctx, tx, invocation, resolved)
}

func (c memoryEventCapability) Definition() CapabilityDefinition {
	return memoryCapabilityDefinition()
}
func (c memoryEventCapability) RequiredContext() []ContextSlot {
	return []ContextSlot{SlotCorePersona, SlotMemoryScope}
}
func (c memoryEventCapability) Prepare(ctx context.Context, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityInvocation, error) {
	if c.service == nil {
		return invocation, errors.New("memory capability unavailable")
	}
	return c.service.prepareMemoryCapability(ctx, invocation, resolved)
}
func (c memoryEventCapability) Execute(ctx context.Context, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityResult, error) {
	if c.service == nil {
		return failedCapabilityResultDetail(invocation, "memory_capability_unavailable", true, "memory capability is unavailable"), errors.New("memory capability unavailable")
	}
	if err := requireCapabilityContext(resolved, SlotCorePersona, SlotMemoryScope); err != nil {
		return failedCapabilityResult(invocation, "context_resolve_failed", true), err
	}
	return c.service.applyMemoryCapability(ctx, invocation, resolved)
}

func (c memoryEventCapability) ExecuteTx(ctx context.Context, tx pgx.Tx, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityResult, error) {
	if c.service == nil {
		return failedCapabilityResultDetail(invocation, "memory_capability_unavailable", true, "memory capability is unavailable"), errors.New("memory capability unavailable")
	}
	if err := requireCapabilityContext(resolved, SlotCorePersona, SlotMemoryScope); err != nil {
		return failedCapabilityResult(invocation, "context_resolve_failed", true), err
	}
	return c.service.applyMemoryCapabilityTx(ctx, tx, invocation, resolved)
}

func (c activeMemoryEventCapability) Definition() CapabilityDefinition {
	return activeMemoryEventCapabilityDefinition()
}
func (c activeMemoryEventCapability) RequiredContext() []ContextSlot {
	return []ContextSlot{SlotMemoryScope, SlotCurrentLife}
}
func (c activeMemoryEventCapability) Prepare(ctx context.Context, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityInvocation, error) {
	if c.service == nil {
		return invocation, errors.New("active memory capability unavailable")
	}
	return c.service.prepareActiveMemoryCapability(ctx, invocation, resolved)
}
func (c activeMemoryEventCapability) Execute(ctx context.Context, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityResult, error) {
	if c.service == nil {
		return failedCapabilityResultDetail(invocation, "active_memory_capability_unavailable", true, "active memory capability is unavailable"), errors.New("active memory capability unavailable")
	}
	if err := requireCapabilityContext(resolved, SlotMemoryScope, SlotCurrentLife); err != nil {
		return failedCapabilityResult(invocation, "context_resolve_failed", true), err
	}
	return c.service.applyActiveMemoryCapability(ctx, invocation, resolved)
}
func (c activeMemoryEventCapability) ExecuteTx(ctx context.Context, tx pgx.Tx, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityResult, error) {
	if c.service == nil {
		return failedCapabilityResultDetail(invocation, "active_memory_capability_unavailable", true, "active memory capability is unavailable"), errors.New("active memory capability unavailable")
	}
	if err := requireCapabilityContext(resolved, SlotMemoryScope, SlotCurrentLife); err != nil {
		return failedCapabilityResult(invocation, "context_resolve_failed", true), err
	}
	return c.service.applyActiveMemoryCapabilityTx(ctx, tx, invocation, resolved)
}

func (c memoryRecallCapability) Definition() CapabilityDefinition {
	return memoryRecallCapabilityDefinition()
}
func (c memoryRecallCapability) RequiredContext() []ContextSlot {
	return []ContextSlot{SlotMemoryScope}
}
func (c memoryRecallCapability) Execute(ctx context.Context, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityResult, error) {
	if c.service == nil {
		return failedCapabilityResultDetail(invocation, "memory_recall_unavailable", true, "memory recall is unavailable"), errors.New("memory recall unavailable")
	}
	if err := requireCapabilityContext(resolved, SlotMemoryScope); err != nil {
		return failedCapabilityResult(invocation, "context_resolve_failed", true), err
	}
	args, err := capabilityExecutionArguments(invocation, memoryRecallCapabilityDefinition())
	if err != nil {
		return failedCapabilityResult(invocation, "invalid_arguments", false), err
	}
	viewers := decisionServiceRefValues(resolved.Memory.Data["viewer_actor_ids"])
	request := MemoryRecallRequest{
		AuthorizationActorID: stringValue(resolved.Memory.Data["owner_actor_id"]), FluctlightID: invocation.Metadata.FluctlightID,
		ConversationID: invocation.Metadata.ConversationID, ViewerActorIDs: viewers,
		ConversationMode: MemoryConversationScopeMode(stringValue(resolved.Memory.Data["conversation_mode"])),
		ActiveProfileID:  stringValue(resolved.Memory.Data["active_profile_id"]), Intent: stringValue(args["intent"]),
	}
	items, truncated, err := c.service.Recall(ctx, request)
	if err != nil {
		return failedCapabilityResultDetail(invocation, "memory_recall_failed", true, err.Error()), err
	}
	output := map[string]any{"items": items, "count": len(items), "truncated": truncated}
	return CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "completed", Output: output, ProviderRequestID: invocation.ProviderRequestID, CorrelationID: "memory-recall:" + stableDigest(invocation.CallID)}, nil
}

func (c affectEventCapability) Definition() CapabilityDefinition {
	return affectEventCapabilityDefinition()
}
func (c affectEventCapability) RequiredContext() []ContextSlot {
	return []ContextSlot{SlotCurrentState}
}
func (c affectEventCapability) Execute(ctx context.Context, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityResult, error) {
	if err := requireCapabilityContext(resolved, SlotCurrentState); err != nil {
		return failedCapabilityResult(invocation, "context_resolve_failed", true), err
	}
	return (&affectEventService{service: c.service}).execute(ctx, invocation, resolved)
}

func (c affectEventCapability) ExecuteTx(ctx context.Context, tx pgx.Tx, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityResult, error) {
	if err := requireCapabilityContext(resolved, SlotCurrentState); err != nil {
		return failedCapabilityResult(invocation, "context_resolve_failed", true), err
	}
	return (&affectEventService{service: c.service}).executeTx(ctx, tx, invocation, resolved)
}

func (c relationshipLookupCapability) Definition() CapabilityDefinition {
	return relationshipLookupCapabilityDefinition()
}
func (c relationshipLookupCapability) RequiredContext() []ContextSlot {
	return []ContextSlot{SlotRelationshipScope}
}

// ValidateCandidate authorizes relationship.lookup against the frozen
// relationship scope before the Judge. It deliberately has no access to the
// service or App: resolving actor aliases and checking the allowlist must use
// only the snapshot that Core already captured for this candidate. The actual
// relationship row lookup remains winner-only Execute work.
func (c relationshipLookupCapability) ValidateCandidate(_ context.Context, invocation CapabilityInvocation, resolved CapabilityContext) error {
	if resolved.Relation == nil {
		return fmt.Errorf("%w: relationship scope is missing", ErrContextResolve)
	}
	var args map[string]any
	if err := json.Unmarshal(invocation.Arguments, &args); err != nil || args == nil {
		return fmt.Errorf("%w: relationship lookup arguments must be an object", ErrInvalidArguments)
	}
	target := strings.TrimSpace(stringValue(args["target_actor_id"]))
	if target == "" {
		return fmt.Errorf("%w: relationship lookup target is required", ErrInvalidArguments)
	}
	// Alias resolution is intentionally a lookup in a frozen, explicit map. An
	// unknown alias is left untouched and will fail the allowlist check; there is
	// no DB fallback on the Judge path.
	canonical, _, targetErr := resolveRelationshipTargetFromScope(resolved.Relation, target)
	if targetErr != nil {
		return fmt.Errorf("%w: %v", ErrUnauthorized, targetErr)
	}
	target = canonical
	if !resolved.Relation.Allows(target) {
		return fmt.Errorf("%w: relationship lookup target %q is outside the frozen scope", ErrUnauthorized, target)
	}
	return nil
}

func (c relationshipLookupCapability) Execute(ctx context.Context, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityResult, error) {
	if c.service == nil {
		return failedCapabilityResultDetail(invocation, "relationship_capability_unavailable", true, "relationship capability is unavailable"), errors.New("relationship capability unavailable")
	}
	if err := requireCapabilityContext(resolved, SlotRelationshipScope); err != nil {
		return failedCapabilityResult(invocation, "context_resolve_failed", true), err
	}
	return c.service.execute(ctx, invocation, resolved)
}

func (c capabilityRequestCapability) Definition() CapabilityDefinition {
	return capabilityRequestDefinition()
}
func (c capabilityRequestCapability) RequiredContext() []ContextSlot { return nil }
func (c capabilityRequestCapability) Execute(ctx context.Context, invocation CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	if c.service == nil {
		return failedCapabilityResultDetail(invocation, "capability_request_unavailable", true, "capability request service is unavailable"), errors.New("capability request service unavailable")
	}
	return c.service.execute(ctx, invocation)
}
func (c capabilityRequestCapability) ExecuteTx(ctx context.Context, tx pgx.Tx, invocation CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	if c.service == nil {
		return failedCapabilityResultDetail(invocation, "capability_request_unavailable", true, "capability request service is unavailable"), errors.New("capability request service unavailable")
	}
	return c.service.executeWith(ctx, tx, invocation)
}

func (a *App) preflightImageCapability(ctx context.Context) error {
	config, err := a.runtimeValue(ctx, "media.comfyui")
	if err != nil {
		return err
	}
	_, _, err = comfyConfig(config)
	return err
}

func (a *App) preflightImageCapabilityTx(ctx context.Context, tx pgx.Tx) error {
	var raw string
	if err := tx.QueryRow(ctx, `SELECT value_json FROM public.runtime_settings WHERE key='media.comfyui'`).Scan(&raw); err != nil {
		return err
	}
	var config map[string]any
	if err := json.Unmarshal([]byte(raw), &config); err != nil {
		return err
	}
	_, _, err := comfyConfig(config)
	return err
}

func requireCapabilityContext(resolved CapabilityContext, slots ...ContextSlot) error {
	for _, slot := range slots {
		if !resolved.Has(slot) {
			return fmt.Errorf("%w: required context slot %q missing", ErrContextResolve, slot)
		}
	}
	return nil
}

func mediaInvocationIdentity(invocation CapabilityInvocation) (string, string, string) {
	base := invocation.Metadata.FluctlightID + ":" + invocation.CapabilityName + ":" + capabilityOperationID(invocation)
	return "media_intent_" + stableDigest(base), "media_workflow_" + stableDigest(base), "media_request_" + stableDigest(base)
}

func publicationCapabilityError(err error, fallback string) (string, bool) {
	switch {
	case errors.Is(err, ErrUnauthorized):
		return "tool_target_unauthorized", false
	case errors.Is(err, ErrNotFound):
		return "tool_target_not_found", false
	case errors.Is(err, ErrInvalidArguments):
		return "invalid_arguments", false
	case errors.Is(err, ErrConflict):
		return "operation_id_conflict", false
	default:
		return fallback, true
	}
}

func firstInt(value any, fallback int) int {
	if n := intValue(value); n != 0 {
		return n
	}
	return fallback
}
