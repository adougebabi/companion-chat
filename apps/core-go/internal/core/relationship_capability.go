package core

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

func relationshipLookupCapabilityManifest() CapabilityManifest {
	return CapabilityManifest{
		Name: "relationship.lookup", Version: "v1",
		Description: "Read one direct relationship from the current Fluctlight to an authorized Actor.",
		Parameters: map[string]any{
			"type": "object", "additionalProperties": false,
			"required":   []any{"target_actor_id"},
			"properties": map[string]any{"target_actor_id": map[string]any{"type": "string", "minLength": 1, "maxLength": 128}},
		},
		OutputSchema:    map[string]any{"type": "object"},
		SideEffectClass: "read_only", ConcurrencyClass: "parallel", SupportsRetry: true,
	}
}

type relationshipLookupCapabilityExecutor struct{ app *App }

func (executor *relationshipLookupCapabilityExecutor) Manifest() CapabilityManifest {
	return relationshipLookupCapabilityManifest()
}

func (executor *relationshipLookupCapabilityExecutor) Execute(ctx context.Context, fluctlightID, conversationID, sourceFactID string, call ToolCallV1) (ToolResultV1, error) {
	var args map[string]any
	if err := json.Unmarshal(call.Arguments, &args); err != nil {
		return failedToolResult(call, "relationship_lookup_arguments_invalid", false, err.Error()), err
	}
	target := strings.TrimSpace(stringValue(args["target_actor_id"]))
	if target == "" {
		return failedToolResult(call, "relationship_lookup_target_required", false, "target actor is required"), errors.New("relationship lookup target required")
	}
	var humanActorID string
	if err := executor.app.DB.Pool().QueryRow(ctx, `SELECT created_by_actor_id FROM public.fluctlights WHERE id=$1`, fluctlightID).Scan(&humanActorID); err != nil {
		return failedToolResult(call, "relationship_lookup_owner_failed", true, err.Error()), err
	}
	target = resolveInitializationActorRef(target, humanActorID, fluctlightID)
	if conversationID != "" {
		var participant bool
		if err := executor.app.DB.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.conversation_participants WHERE conversation_id=$1 AND actor_id=$2 AND status='active')`, conversationID, target).Scan(&participant); err != nil {
			return failedToolResult(call, "relationship_lookup_scope_failed", true, err.Error()), err
		}
		if !participant {
			return failedToolResult(call, "relationship_lookup_target_forbidden", false, "target actor is not an active conversation participant"), ErrUnauthorized
		}
	}
	var actorType, trend string
	var role, metrics, summary, emotional, provenance []byte
	var revision int
	if err := executor.app.DB.Pool().QueryRow(ctx, `SELECT COALESCE(a.actor_type,'unknown'),r.role,r.metrics,r.trend,r.summary,r.emotional_association,r.provenance,r.revision FROM public.relationships r LEFT JOIN public.actors a ON a.id=r.target_actor_id WHERE r.owner_fluctlight_id=$1 AND r.target_actor_id=$2`, fluctlightID, target).Scan(&actorType, &role, &metrics, &trend, &summary, &emotional, &provenance, &revision); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return failedToolResult(call, "relationship_lookup_not_found", false, "relationship is not established"), ErrNotFound
		}
		return failedToolResult(call, "relationship_lookup_failed", true, err.Error()), err
	}
	result := map[string]any{"target_actor_id": target, "target_actor_type": actorType, "role": decodeObject(role), "metrics": decodeObject(metrics), "trend": trend, "summary": summary, "emotional_association": decodeObject(emotional), "provenance": decodeObject(provenance), "revision": revision}
	return ToolResultV1{ToolCallID: call.ID, Name: call.Name, Status: "completed", Output: result, ProviderRequestID: call.ProviderRequestID, CorrelationID: "relationship:" + stableDigest(fluctlightID+":"+target), SchemaVersion: ToolResultSchemaVersion}, nil
}
