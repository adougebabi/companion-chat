package core

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"strings"
	"time"
)

const itemUseCapabilityName = "item.use"

type itemUseCapability struct{ service *WardrobeService }

func itemUseDefinition() CapabilityDefinition {
	return CapabilityDefinition{Name: itemUseCapabilityName, Version: "v1", Type: CapabilityTypeAction, Description: "Start or stop using an available owned ordinary object. This does not acquire it or change wearing.", Surfaces: []CapabilitySurface{CapabilitySurfaceConversation, CapabilitySurfaceWakeUp, CapabilitySurfaceNativeCognition}, FailurePolicy: FailurePolicyRequiredForVisibleClaim, InputSchema: objectSchema(map[string]any{"operation": enumStringSchema("start", "stop"), "item_id": stringSchema(), "activity": map[string]any{"type": "string", "maxLength": 512}}, []string{"operation", "item_id"}, false), OutputSchema: openObjectSchema(), SideEffectClass: "native_projection", SuccessBoundary: "item_use_committed", ConcurrencyClass: "exclusive", SupportsRetry: true}
}
func (c itemUseCapability) Definition() CapabilityDefinition { return itemUseDefinition() }
func (c itemUseCapability) RequiredContext() []ContextSlot   { return nil }
func (c itemUseCapability) Execute(_ context.Context, i CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	return executeToolRequired(i)
}
func (c itemUseCapability) ExecuteTx(ctx context.Context, tx pgx.Tx, i CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	if c.service == nil || c.service.repository == nil {
		return failedCapabilityResult(i, "item_service_unavailable", true), errors.New("item service unavailable")
	}
	args, err := capabilityExecutionArguments(i, itemUseDefinition())
	if err != nil {
		return failedCapabilityResult(i, "invalid_arguments", false), err
	}
	at := time.Now().UTC()
	if c.service.clock != nil {
		at = c.service.clock().UTC()
	}
	owner, itemID, op := i.Metadata.FluctlightID, stringValue(args["item_id"]), stringValue(args["operation"])
	if err := requireAwakeLifeTx(ctx, tx, owner, at); err != nil {
		return failedCapabilityResult(i, "life_state_sleeping", false), err
	}
	var kind, availability, ownership string
	var itemRevision int
	var verified bool
	if err := tx.QueryRow(ctx, `SELECT item_kind,availability,ownership,revision,`+inventorySourceVerifiedSQL+` FROM public.fluctlight_wardrobe_items i WHERE fluctlight_id=$1 AND id=$2 FOR SHARE`, owner, itemID).Scan(&kind, &availability, &ownership, &itemRevision, &verified); err != nil {
		return failedCapabilityResult(i, "item_not_found", false), err
	}
	if !verified || kind != "object" || availability != "available" || (ownership != "owned" && ownership != "borrowed") {
		return failedCapabilityResult(i, "item_not_usable", false), ErrConflict
	}
	activity := strings.TrimSpace(stringValue(args["activity"]))
	if op == "start" && (activity == "" || len([]rune(activity)) > 512) {
		return failedCapabilityResult(i, "item_use_activity_required", false), ErrInvalidArguments
	}
	eventID := "item_use_" + stableDigest(owner+"\x1f"+capabilityOperationID(i))
	if op == "start" {
		if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_item_uses(fluctlight_id,item_id,activity,started_at,source_event_id) VALUES($1,$2,$3,$4,$5) ON CONFLICT(fluctlight_id,item_id) DO UPDATE SET revision=fluctlight_item_uses.revision+1,activity=EXCLUDED.activity,started_at=EXCLUDED.started_at,source_event_id=EXCLUDED.source_event_id`, owner, itemID, activity, at, eventID); err != nil {
			return failedCapabilityResult(i, "item_use_write_failed", true), err
		}
	} else {
		if _, err := tx.Exec(ctx, `DELETE FROM public.fluctlight_item_uses WHERE fluctlight_id=$1 AND item_id=$2`, owner, itemID); err != nil {
			return failedCapabilityResult(i, "item_use_write_failed", true), err
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_item_use_events(id,fluctlight_id,item_id,operation,activity,occurred_at,source_ref,item_revision) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, eventID, owner, itemID, op, activity, at, i.SourceFactID, itemRevision); err != nil {
		return failedCapabilityResult(i, "item_use_event_failed", true), err
	}
	if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_wardrobe_states SET revision=revision+1,updated_at=now() WHERE fluctlight_id=$1`, owner); err != nil {
		return failedCapabilityResult(i, "item_revision_failed", true), err
	}
	if err := appendOutboxTx(ctx, tx, "item.use.changed", "item", itemID, owner, i.SourceFactID, i.Metadata.CorrelationID, "item-use:"+eventID, map[string]any{"event_id": eventID, "item_id": itemID, "operation": op}); err != nil {
		return failedCapabilityResult(i, "item_use_outbox_failed", true), err
	}
	return CapabilityResult{CallID: i.CallID, CapabilityName: i.CapabilityName, Status: "completed", Output: map[string]any{"event_id": eventID, "item_id": itemID, "operation": op, "occurred_at": formatInstant(at)}}, nil
}
func readUsedItemsWith(ctx context.Context, q lifeContextQuerier, owner string) ([]map[string]any, error) {
	rows, err := q.Query(ctx, `SELECT i.id,i.category,i.description,u.activity,u.source_event_id,u.started_at,`+inventorySourceVerifiedSQL+` FROM public.fluctlight_item_uses u JOIN public.fluctlight_wardrobe_items i ON i.fluctlight_id=u.fluctlight_id AND i.id=u.item_id WHERE u.fluctlight_id=$1 AND i.item_kind='object' AND i.availability='available' AND i.ownership IN ('owned','borrowed') ORDER BY i.id LIMIT 16`, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]map[string]any, 0)
	for rows.Next() {
		var id, category, description, activity, eventID string
		var started time.Time
		var verified bool
		if err := rows.Scan(&id, &category, &description, &activity, &eventID, &started, &verified); err != nil {
			return nil, err
		}
		result = append(result, map[string]any{"id": id, "category": category, "description": description, "activity": activity, "event_id": eventID, "started_at": formatInstant(started), "source_verified": verified})
	}
	return result, rows.Err()
}
