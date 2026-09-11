package core

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

func relationshipLookupCapabilityDefinition() CapabilityDefinition {
	return CapabilityDefinition{
		Name: "relationship.lookup", Version: "v1", Type: CapabilityTypeQuery,
		Description:     "Read one direct relationship from the current Fluctlight to an authorized Actor.",
		Surfaces:        []CapabilitySurface{CapabilitySurfaceConversation, CapabilitySurfaceWakeUp, CapabilitySurfaceAutonomy, CapabilitySurfaceNativeCognition},
		FailurePolicy:   FailurePolicyOptionalInternal,
		RequiredContext: []ContextSlot{SlotRelationshipScope},
		InputSchema: map[string]any{
			"type": "object", "additionalProperties": false,
			"required":   []any{"target_actor_id"},
			"properties": map[string]any{"target_actor_id": map[string]any{"type": "string", "minLength": 1, "maxLength": 128}},
		},
		OutputSchema:    map[string]any{"type": "object"},
		SideEffectClass: "read_only", SuccessBoundary: "query_result_available", ConcurrencyClass: "parallel", SupportsRetry: true,
	}
}

type relationshipLookupService struct{ app *App }

func (service *relationshipLookupService) execute(ctx context.Context, invocation CapabilityInvocation, resolved CapabilityContext) (CapabilityResult, error) {
	fluctlightID, conversationID := invocation.Metadata.FluctlightID, invocation.Metadata.ConversationID
	var args map[string]any
	if err := json.Unmarshal(invocation.Arguments, &args); err != nil {
		return failedCapabilityResultDetail(invocation, "relationship_lookup_arguments_invalid", false, err.Error()), err
	}
	target := strings.TrimSpace(stringValue(args["target_actor_id"]))
	if target == "" {
		return failedCapabilityResultDetail(invocation, "relationship_lookup_target_required", false, "target actor is required"), errors.New("relationship lookup target required")
	}
	var humanActorID string
	if service == nil || service.app == nil || service.app.DB == nil {
		return failedCapabilityResultDetail(invocation, "relationship_capability_unavailable", true, "relationship capability is unavailable"), errors.New("relationship capability unavailable")
	}
	if err := service.app.DB.Pool().QueryRow(ctx, `SELECT created_by_actor_id FROM public.fluctlights WHERE id=$1`, fluctlightID).Scan(&humanActorID); err != nil {
		return failedCapabilityResultDetail(invocation, "relationship_lookup_owner_failed", true, err.Error()), err
	}
	target = resolveInitializationActorRef(target, humanActorID, fluctlightID)
	if conversationID != "" {
		target = service.app.resolveConversationActorAlias(ctx, conversationID, humanActorID, fluctlightID, target)
	}
	// Authorization and relationship rows are separate resolver concerns. An
	// empty authorized set denies every target, including when no relationship
	// row exists yet; rows can never widen the frozen actor scope.
	if resolved.Relation == nil || !resolved.Relation.Allows(target) {
		return failedCapabilityResultDetail(invocation, "relationship_lookup_target_forbidden", false, "target is outside the resolved relationship scope"), ErrUnauthorized
	}
	if conversationID != "" {
		var participant bool
		if err := service.app.DB.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.conversation_participants WHERE conversation_id=$1 AND actor_id=$2 AND status='active')`, conversationID, target).Scan(&participant); err != nil {
			return failedCapabilityResultDetail(invocation, "relationship_lookup_scope_failed", true, err.Error()), err
		}
		if !participant {
			return failedCapabilityResultDetail(invocation, "relationship_lookup_target_forbidden", false, "target actor is not an active conversation participant"), ErrUnauthorized
		}
	}
	var actorType, trend string
	var activeProfileID string
	_ = service.app.DB.Pool().QueryRow(ctx, `SELECT COALESCE(active_profile_id,'default') FROM public.fluctlight_personality_runtime WHERE fluctlight_id=$1`, fluctlightID).Scan(&activeProfileID)
	var role, metrics, summary, emotional, provenance []byte
	var revision int
	var profileID *string
	if err := service.app.DB.Pool().QueryRow(ctx, `SELECT r.profile_id,COALESCE(a.actor_type,'unknown'),r.role,r.metrics,r.trend,r.summary,r.emotional_association,r.provenance,r.revision FROM public.relationships r LEFT JOIN public.actors a ON a.id=r.target_actor_id WHERE r.owner_fluctlight_id=$1 AND r.target_actor_id=$2 AND (r.profile_id=$3 OR r.profile_id IS NULL) ORDER BY CASE WHEN r.profile_id=$3 THEN 0 ELSE 1 END LIMIT 1`, fluctlightID, target, activeProfileID).Scan(&profileID, &actorType, &role, &metrics, &trend, &summary, &emotional, &provenance, &revision); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return failedCapabilityResultDetail(invocation, "relationship_lookup_not_found", false, "relationship is not established"), ErrNotFound
		}
		return failedCapabilityResultDetail(invocation, "relationship_lookup_failed", true, err.Error()), err
	}
	result := map[string]any{"target_actor_id": target, "target_actor_type": actorType, "role": decodeObject(role), "metrics": decodeObject(metrics), "trend": trend, "summary": summary, "emotional_association": decodeObject(emotional), "provenance": decodeObject(provenance), "revision": revision}
	if profileID != nil && strings.TrimSpace(*profileID) != "" {
		result["profile_id"] = *profileID
	}
	return CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "completed", Output: result, ProviderRequestID: invocation.ProviderRequestID, CorrelationID: "relationship:" + stableDigest(fluctlightID+":"+target)}, nil
}
