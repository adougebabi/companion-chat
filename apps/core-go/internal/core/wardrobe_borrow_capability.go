package core

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	wardrobeBorrowCapabilityName = "wardrobe.borrow"
	wardrobeReturnCapabilityName = "wardrobe.return"
)

// Borrowing uses the existing confirmed wardrobe Event and inventory authority.
// Registration deliberately does not wear the item or complete a purchase.
type wardrobeBorrowCapability struct {
	service   *WardrobeService
	returning bool
}

func wardrobeBorrowDefinition(returning bool) CapabilityDefinition {
	text := func(max int) map[string]any {
		return map[string]any{"type": "string", "minLength": 1, "maxLength": max}
	}
	name := wardrobeBorrowCapabilityName
	description := "Register clothing actually received on loan, including authorized shop try-on, as available borrowed inventory. Supply lender and concrete items; returns real item IDs for a separate wardrobe.wear call. A wish or browsing alone is not receipt. This does not buy or wear clothing. Inspect first and reuse IDs for items already registered."
	properties := map[string]any{
		"lender": text(256), "reason": text(512),
		"items": map[string]any{"type": "array", "minItems": 1, "maxItems": 8, "items": objectSchema(map[string]any{
			"category": text(64), "slot": text(64), "description": text(512),
		}, []string{"category", "slot", "description"}, false)},
	}
	required := []string{"lender", "reason", "items"}
	if returning {
		name = wardrobeReturnCapabilityName
		description = "Return recorded borrowed clothing by item IDs. Marks it unavailable and removes its current wearing links, retaining borrowed ownership and history. Restore your own clothes separately with wardrobe.wear. This never marks owned clothing lost or changes it to borrowed."
		properties = map[string]any{"reason": text(512), "item_ids": map[string]any{
			"type": "array", "minItems": 1, "maxItems": 8, "uniqueItems": true, "items": text(128),
		}}
		required = []string{"reason", "item_ids"}
	}
	return CapabilityDefinition{
		Name: name, Version: "v1", Type: CapabilityTypeAction, Description: description,
		Surfaces:        []CapabilitySurface{CapabilitySurfaceConversation, CapabilitySurfaceWakeUp, CapabilitySurfaceNativeCognition},
		RequiredContext: []ContextSlot{SlotCurrentLife}, FailurePolicy: FailurePolicyRequiredForVisibleClaim,
		InputSchema: objectSchema(properties, required, false), OutputSchema: openObjectSchema(),
		SideEffectClass: "native_projection", SuccessBoundary: "borrowed_inventory_committed", ConcurrencyClass: "exclusive", SupportsRetry: true,
	}
}

func (c wardrobeBorrowCapability) Definition() CapabilityDefinition {
	return wardrobeBorrowDefinition(c.returning)
}
func (c wardrobeBorrowCapability) RequiredContext() []ContextSlot {
	return []ContextSlot{SlotCurrentLife}
}
func (c wardrobeBorrowCapability) Execute(_ context.Context, i CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	return executeToolRequired(i)
}
func (c wardrobeBorrowCapability) Prepare(ctx context.Context, i CapabilityInvocation, _ CapabilityContext) (CapabilityInvocation, error) {
	if c.service == nil || c.service.repository == nil {
		return i, errors.New("wardrobe unavailable")
	}
	if _, err := capabilityExecutionArguments(i, c.Definition()); err != nil {
		return i, err
	}
	var revision int
	if err := c.service.repository.Pool().QueryRow(ctx, `SELECT revision FROM public.fluctlight_wardrobe_states WHERE fluctlight_id=$1`, i.Metadata.FluctlightID).Scan(&revision); err != nil {
		return i, err
	}
	return withCapabilityPreparedData(i, "expected_wardrobe_revision", revision)
}

func (c wardrobeBorrowCapability) ExecuteTx(ctx context.Context, tx pgx.Tx, i CapabilityInvocation, current CapabilityContext) (CapabilityResult, error) {
	args, err := capabilityExecutionArguments(i, c.Definition())
	if err != nil {
		return failedCapabilityResult(i, "invalid_arguments", false), err
	}
	expected, found, err := capabilityPreparedData(i, "expected_wardrobe_revision")
	if err != nil || !found {
		return failedCapabilityResult(i, "borrowing_plan_missing", false), ErrInvalidArguments
	}
	fluctlightID := i.Metadata.FluctlightID
	at := time.Now().UTC()
	if c.service.clock != nil {
		at = c.service.clock().UTC()
	}
	if err := requireAwakeLifeTx(ctx, tx, fluctlightID, at); err != nil {
		return failedCapabilityResult(i, "life_state_sleeping", false), err
	}
	var revision int
	if err := tx.QueryRow(ctx, `SELECT revision FROM public.fluctlight_wardrobe_states WHERE fluctlight_id=$1 FOR UPDATE`, fluctlightID).Scan(&revision); err != nil {
		return failedCapabilityResult(i, "wardrobe_state_unavailable", true), err
	}
	if revision != intValue(expected) {
		return failedCapabilityResult(i, "wardrobe_revision_conflict", true), ErrConflict
	}
	if strings.TrimSpace(stringValue(args["reason"])) == "" || strings.TrimSpace(i.SourceFactID) == "" {
		return failedCapabilityResult(i, "borrowing_source_required", false), ErrInvalidArguments
	}
	kind := "wardrobe_gain"
	members := arrayValue(args["items"])
	if c.returning {
		kind, members = "wardrobe_unavailable", arrayValue(args["item_ids"])
	} else if strings.TrimSpace(stringValue(args["lender"])) == "" {
		return failedCapabilityResult(i, "borrowing_lender_required", false), ErrInvalidArguments
	}
	if len(members) < 1 || len(members) > 8 {
		return failedCapabilityResult(i, "borrowing_items_invalid", false), ErrInvalidArguments
	}
	items := make([]any, 0, len(members))
	seen := map[string]bool{}
	for index, member := range members {
		var effect map[string]any
		if c.returning {
			itemID := strings.TrimSpace(stringValue(member))
			if seen[itemID] {
				return failedCapabilityResult(i, "borrowed_item_duplicate", false), ErrInvalidArguments
			}
			seen[itemID] = true
			var ownership, availability, itemKind string
			if err := tx.QueryRow(ctx, `SELECT ownership,availability,item_kind FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND id=$2 FOR UPDATE`, fluctlightID, itemID).Scan(&ownership, &availability, &itemKind); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return failedCapabilityResult(i, "wardrobe_item_not_found", false), ErrNotFound
				}
				return failedCapabilityResult(i, "wardrobe_item_read_failed", true), err
			}
			if ownership != "borrowed" || availability != "available" || itemKind != "wearable" {
				return failedCapabilityResultDetail(i, "borrowed_item_not_returnable", false, "Only available borrowed items can be returned; inspect current inventory."), ErrConflict
			}
			effect = map[string]any{"item_id": itemID}
		} else {
			item := mapValue(member)
			effect, err = validateWardrobeEventEffect(kind, map[string]any{
				"category": item["category"], "slot": item["slot"], "description": item["description"], "ownership": "borrowed",
			})
			if err != nil {
				return failedCapabilityResult(i, "borrowed_item_invalid", false), err
			}
			key := stableDigest(jsonString(effect))
			if seen[key] {
				return failedCapabilityResult(i, "borrowed_item_duplicate", false), ErrInvalidArguments
			}
			seen[key] = true
		}
		eventID := "wardrobe_loan_" + stableDigest(fluctlightID+"\x1f"+i.CapabilityName+"\x1f"+capabilityOperationID(i)+fmt.Sprintf("\x1f%d", index))
		result := map[string]any{"wardrobe_effect": effect, "reason": args["reason"], "lender": args["lender"], "source_fact_id": i.SourceFactID}
		if current.Life != nil {
			result["loan_context"] = current.Life.Data
		}
		// This is a completed inventory fact, never a new active scene Event.
		if _, err := tx.Exec(ctx, `INSERT INTO public.life_events(id,fluctlight_id,kind,start_at,end_at,activity,status,revision,evidence_refs,idempotency_key,request_digest,result,expires_at) VALUES($1,$2,$3,$4 - interval '1 microsecond',$4,$5,'confirmed',1,$6,$7,$8,$9,$4)`, eventID, fluctlightID, kind, at, args["reason"], jsonBytes([]any{i.SourceFactID}), "loan:"+eventID, stableDigest(jsonString(result)), jsonBytes(result)); err != nil {
			return failedCapabilityResult(i, "borrowing_event_failed", true), err
		}
		output, err := applyWardrobeEffectFromConfirmedEventTx(ctx, tx, fluctlightID, eventID, kind, effect)
		if err != nil {
			return failedCapabilityResult(i, "borrowing_effect_failed", true), err
		}
		output["ownership"] = "borrowed"
		if !c.returning {
			output["availability"] = "available"
			output["category"], output["slot"], output["description"] = effect["category"], effect["slot"], effect["description"]
		}
		items = append(items, output)
	}
	return CapabilityResult{CallID: i.CallID, CapabilityName: i.CapabilityName, Status: "completed", Output: map[string]any{"items": items}, ProviderRequestID: i.ProviderRequestID, CorrelationID: "wardrobe-loan:" + stableDigest(capabilityOperationID(i))}, nil
}
