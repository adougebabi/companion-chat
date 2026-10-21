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
	wardrobeInspectCapabilityName = "wardrobe.inspect"
	wardrobeWearCapabilityName    = "wardrobe.wear"
)

type WardrobeService struct {
	repository *PostgresRepository
	clock      func() time.Time
}

func newWardrobeService(app *App) *WardrobeService {
	if app == nil {
		return &WardrobeService{}
	}
	return &WardrobeService{repository: app.DB, clock: app.now}
}

type wardrobeInspectCapability struct{ service *WardrobeService }
type wardrobeWearCapability struct{ service *WardrobeService }

// WardrobeItems is the Owner-facing read projection used by the detail view.
// It shares the same bounded, cursor-based inventory query as wardrobe.inspect.
func (a *App) WardrobeItems(ctx context.Context, actorID, fluctlightID, cursor string) (map[string]any, error) {
	if _, err := a.DB.GetFluctlight(ctx, fluctlightID, actorID); err != nil {
		return nil, err
	}
	cursor = strings.TrimSpace(cursor)
	if len(cursor) > 128 {
		return nil, ErrInvalidArguments
	}
	return newWardrobeService(a).inspectWardrobe(ctx, fluctlightID, "", map[string]any{
		"operation": "list", "limit": 30, "cursor": cursor,
	})
}

func wardrobeInspectDefinition() CapabilityDefinition {
	return CapabilityDefinition{
		Name: wardrobeInspectCapabilityName, Version: "v1", Type: CapabilityTypeQuery,
		Description:   "Inspect recorded inventory, outfits or actual wearing. catalogue/catalogue_detail query the current world's obtainable definitions (not owned); catalogue_detail uses item_id as world_item_id. Use that ID plus exact item fields for virtual shopping. detail needs inventory item_id; outfit_detail needs outfit_id. A missing item does not prove inventory complete.",
		Surfaces:      []CapabilitySurface{CapabilitySurfaceConversation, CapabilitySurfaceWakeUp, CapabilitySurfaceNativeCognition},
		FailurePolicy: FailurePolicyOptionalInternal,
		InputSchema: objectSchema(map[string]any{
			"operation": withToolDefault(enumStringSchema("list", "detail", "wearing", "outfits", "outfit_detail", "catalogue", "catalogue_detail"), "list"),
			"item_id":   map[string]any{"type": "string", "minLength": 1, "maxLength": 128},
			"outfit_id": map[string]any{"type": "string", "minLength": 1, "maxLength": 128},
			"category":  map[string]any{"type": "string", "maxLength": 64},
			"query":     map[string]any{"type": "string", "maxLength": 128},
			"cursor":    map[string]any{"type": "string", "maxLength": 128},
			"limit":     map[string]any{"type": "integer", "minimum": 1, "maximum": 30},
		}, []string{"operation"}, false),
		OutputSchema: openObjectSchema(), SideEffectClass: "read_only", SuccessBoundary: "query_result_available",
		ConcurrencyClass: "parallel", SupportsRetry: true,
		ModelResultOmitFields: []string{"revision", "items.revision", "items.source_kind", "items.source_ref", "item.revision", "item.source_kind", "item.source_ref", "outfits.revision", "outfits.profile_id", "outfit.revision", "outfit.profile_id"},
	}
}

func (c wardrobeInspectCapability) Definition() CapabilityDefinition {
	return wardrobeInspectDefinition()
}
func (c wardrobeInspectCapability) RequiredContext() []ContextSlot { return nil }
func (c wardrobeInspectCapability) Execute(ctx context.Context, invocation CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	if c.service == nil || c.service.repository == nil {
		return failedCapabilityResult(invocation, "wardrobe_unavailable", true), errors.New("wardrobe unavailable")
	}
	args, err := capabilityExecutionArguments(invocation, wardrobeInspectDefinition())
	if err != nil {
		return failedCapabilityResult(invocation, "invalid_arguments", false), err
	}
	missingTarget := (stringValue(args["operation"]) == "detail" && stringValue(args["item_id"]) == "") || (stringValue(args["operation"]) == "outfit_detail" && stringValue(args["outfit_id"]) == "")
	if missingTarget {
		if stringValue(args["operation"]) == "outfit_detail" {
			args["operation"] = "outfits"
		} else {
			args["operation"] = "list"
		}
	}
	output, err := c.service.inspectWardrobe(ctx, invocation.Metadata.FluctlightID, invocation.Metadata.WorkingProfileID, args)
	if err != nil {
		if errors.Is(err, ErrInvalidArguments) {
			return failedCapabilityResultDetail(invocation, "invalid_arguments", false, "detail needs item_id; outfit_detail needs outfit_id"), err
		}
		if errors.Is(err, ErrNotFound) {
			return failedCapabilityResultDetail(invocation, "wardrobe_item_not_found", false, "item is not in the recorded wardrobe"), err
		}
		return failedCapabilityResult(invocation, "wardrobe_query_failed", true), err
	}
	if missingTarget {
		output["warning_code"] = "wardrobe_target_missing_listed"
	}
	return CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "completed", Output: output,
		ProviderRequestID: invocation.ProviderRequestID, CorrelationID: "wardrobe-query:" + stableDigest(invocation.CallID)}, nil
}

func (s *WardrobeService) inspectWardrobe(ctx context.Context, fluctlightID, workingProfileID string, args map[string]any) (map[string]any, error) {
	if operation := stringValue(args["operation"]); operation == "catalogue" || operation == "catalogue_detail" {
		return s.inspectWorldItems(ctx, fluctlightID, args)
	}
	query := s.repository.Pool()
	operation := stringValue(args["operation"])
	var revision int
	var inventoryComplete bool
	var wearingState string
	if err := query.QueryRow(ctx, `SELECT revision,inventory_complete,wearing_state FROM public.fluctlight_wardrobe_states WHERE fluctlight_id=$1`, fluctlightID).Scan(&revision, &inventoryComplete, &wearingState); err != nil {
		return nil, err
	}
	output := map[string]any{"operation": operation, "revision": revision, "inventory_complete": inventoryComplete}
	switch operation {
	case "list":
		limit := intValue(args["limit"])
		if limit == 0 {
			limit = 12
		}
		cursor := strings.TrimSpace(stringValue(args["cursor"]))
		category := strings.TrimSpace(stringValue(args["category"]))
		phrase := strings.TrimSpace(stringValue(args["query"]))
		rows, err := query.Query(ctx, `SELECT id,category,slot,description,ownership,availability,source_kind,revision,item_kind,COALESCE(clothing_type,''),seasons FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND archived_at IS NULL AND id>$2 AND ($3='' OR category=$3) AND ($4='' OR position(lower($4) in lower(description))>0) ORDER BY id LIMIT $5`, fluctlightID, cursor, category, phrase, limit+1)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		items := make([]map[string]any, 0, limit)
		for rows.Next() {
			var id, itemCategory, slot, description, ownership, availability, sourceKind, itemKind string
			var itemRevision int
			var clothing string
			var seasons []string
			if err := rows.Scan(&id, &itemCategory, &slot, &description, &ownership, &availability, &sourceKind, &itemRevision, &itemKind, &clothing, &seasons); err != nil {
				return nil, err
			}
			items = append(items, map[string]any{"id": id, "category": itemCategory, "slot": slot, "description": description,
				"ownership": ownership, "availability": availability, "source_kind": sourceKind, "revision": itemRevision, "item_kind": itemKind, "clothing_type": clothing, "seasons": seasons})
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
		hasMore := len(items) > limit
		if hasMore {
			items = items[:limit]
		}
		nextCursor := ""
		if hasMore {
			nextCursor = stringValue(items[len(items)-1]["id"])
		}
		output["items"] = items
		output["has_more"] = hasMore
		output["next_cursor"] = nextCursor
		output["searched_scope"] = "recorded_items"
		output["can_conclude_absent"] = inventoryComplete && !hasMore && len(items) == 0 && cursor == ""
	case "detail":
		id := strings.TrimSpace(stringValue(args["item_id"]))
		if id == "" {
			return nil, ErrInvalidArguments
		}
		var category, slot, description, ownership, availability, sourceKind, sourceRef string
		var itemRevision int
		err := query.QueryRow(ctx, `SELECT category,slot,description,ownership,availability,source_kind,source_ref,revision FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND id=$2`, fluctlightID, id).Scan(&category, &slot, &description, &ownership, &availability, &sourceKind, &sourceRef, &itemRevision)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		if err != nil {
			return nil, err
		}
		output["item"] = map[string]any{"id": id, "category": category, "slot": slot, "description": description,
			"ownership": ownership, "availability": availability, "source_kind": sourceKind, "source_ref": sourceRef, "revision": itemRevision}
	case "wearing":
		worn, err := readCurrentWornItems(ctx, query, fluctlightID)
		if err != nil {
			return nil, err
		}
		output["items"] = worn
		output["wearing_state"] = wearingState
		output["scope"] = "current_actual_wearing"
	case "outfits":
		profileID, err := s.effectiveProfileID(ctx, query, fluctlightID, workingProfileID)
		if err != nil {
			return nil, err
		}
		rows, err := query.Query(ctx, `SELECT id,profile_id,name,revision,tags FROM public.fluctlight_wardrobe_outfits WHERE fluctlight_id=$1 AND (profile_id='' OR profile_id=$2) AND id>$3 ORDER BY id LIMIT 31`, fluctlightID, profileID, stringValue(args["cursor"]))
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		outfits := make([]map[string]any, 0)
		for rows.Next() {
			var id, profileID, name string
			var outfitRevision int
			var tags []string
			if err := rows.Scan(&id, &profileID, &name, &outfitRevision, &tags); err != nil {
				return nil, err
			}
			outfits = append(outfits, map[string]any{"id": id, "profile_id": profileID, "name": name, "revision": outfitRevision, "tags": tags})
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
		output["has_more"] = len(outfits) > 30
		if len(outfits) > 30 {
			outfits = outfits[:30]
			output["next_cursor"] = outfits[len(outfits)-1]["id"]
		}
		output["outfits"] = outfits
	case "outfit_detail":
		profileID, err := s.effectiveProfileID(ctx, query, fluctlightID, workingProfileID)
		if err != nil {
			return nil, err
		}
		outfitID := strings.TrimSpace(stringValue(args["outfit_id"]))
		if outfitID == "" {
			return nil, ErrInvalidArguments
		}
		var ownerProfile, name string
		var tags []string
		var outfitRevision int
		err = query.QueryRow(ctx, `SELECT profile_id,name,revision,tags FROM public.fluctlight_wardrobe_outfits WHERE fluctlight_id=$1 AND id=$2 AND (profile_id='' OR profile_id=$3)`, fluctlightID, outfitID, profileID).Scan(&ownerProfile, &name, &outfitRevision, &tags)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		if err != nil {
			return nil, err
		}
		rows, err := query.Query(ctx, `SELECT r.slot,i.id,i.description,i.availability,i.slot,i.item_kind,i.archived_at,COALESCE(i.clothing_type,''),i.seasons,i.ownership,`+inventorySourceVerifiedSQL+` FROM public.fluctlight_wardrobe_outfit_items r JOIN public.fluctlight_wardrobe_items i ON i.fluctlight_id=r.fluctlight_id AND i.id=r.item_id WHERE r.fluctlight_id=$1 AND r.outfit_id=$2 ORDER BY r.slot`, fluctlightID, outfitID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		items := make([]map[string]any, 0)
		for rows.Next() {
			var slot, id, description, availability, currentSlot, itemKind string
			var archived *time.Time
			var clothing, ownership string
			var seasons []string
			var verified bool
			if err := rows.Scan(&slot, &id, &description, &availability, &currentSlot, &itemKind, &archived, &clothing, &seasons, &ownership, &verified); err != nil {
				return nil, err
			}
			items = append(items, map[string]any{"slot": slot, "current_slot": currentSlot, "definition_slot_matches": slot == currentSlot, "item_kind": itemKind, "archived_at": archived, "id": id, "description": description, "availability": availability, "clothing_type": clothing, "seasons": seasons, "ownership": ownership, "source_verified": verified})
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
		output["outfit"] = map[string]any{"id": outfitID, "profile_id": ownerProfile, "name": name, "revision": outfitRevision, "items": items, "tags": tags}
	}
	return output, nil
}

func (s *WardrobeService) effectiveProfileID(ctx context.Context, query DBTX, fluctlightID, requested string) (string, error) {
	if requested = strings.TrimSpace(requested); requested != "" {
		return requested, nil
	}
	var active string
	if err := query.QueryRow(ctx, `SELECT active_profile_id FROM public.fluctlight_personality_runtime WHERE fluctlight_id=$1`, fluctlightID).Scan(&active); err != nil {
		return "", err
	}
	return active, nil
}

func readCurrentWornItems(ctx context.Context, query DBTX, fluctlightID string) ([]map[string]any, error) {
	rows, err := query.Query(ctx, `SELECT w.slot,i.id,i.category,i.description,i.ownership,i.availability,`+inventorySourceVerifiedSQL+` FROM public.fluctlight_worn_items w JOIN public.fluctlight_wardrobe_items i ON i.fluctlight_id=w.fluctlight_id AND i.id=w.item_id WHERE w.fluctlight_id=$1 ORDER BY w.slot`, fluctlightID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]map[string]any, 0)
	for rows.Next() {
		var slot, id, category, description, ownership, availability string
		var verified bool
		if err := rows.Scan(&slot, &id, &category, &description, &ownership, &availability, &verified); err != nil {
			return nil, err
		}
		items = append(items, map[string]any{"slot": slot, "id": id, "category": category, "description": description,
			"ownership": ownership, "availability": availability, "source_verified": verified})
	}
	return items, rows.Err()
}

func wardrobeWearDefinition() CapabilityDefinition {
	return CapabilityDefinition{
		Name: wardrobeWearCapabilityName, Version: "v1", Type: CapabilityTypeAction,
		Description:   "Wear available IDs. Full: include retained items. Partial replaces selected slots. remove_slots only undresses unreplaced slots. No acquisition.",
		Surfaces:      []CapabilitySurface{CapabilitySurfaceConversation, CapabilitySurfaceWakeUp, CapabilitySurfaceNativeCognition},
		FailurePolicy: FailurePolicyRequiredForVisibleClaim,
		InputSchema: objectSchema(map[string]any{
			"mode":         enumStringSchema("full", "partial"),
			"item_ids":     map[string]any{"type": "array", "maxItems": 16, "items": map[string]any{"type": "string", "minLength": 1, "maxLength": 128}},
			"remove_slots": map[string]any{"type": "array", "maxItems": 16, "items": map[string]any{"type": "string", "minLength": 1, "maxLength": 64}},
		}, []string{"mode", "item_ids"}, false),
		OutputSchema: openObjectSchema(), SideEffectClass: "native_projection", SuccessBoundary: "wearing_state_committed",
		ConcurrencyClass: "exclusive", SupportsRetry: true,
		ModelResultOmitFields: []string{"revision", "items"},
	}
}

func (c wardrobeWearCapability) Definition() CapabilityDefinition { return wardrobeWearDefinition() }
func (c wardrobeWearCapability) RequiredContext() []ContextSlot   { return nil }
func (c wardrobeWearCapability) Execute(_ context.Context, invocation CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	return executeToolRequired(invocation)
}
func (c wardrobeWearCapability) Prepare(ctx context.Context, invocation CapabilityInvocation, _ CapabilityContext) (CapabilityInvocation, error) {
	if c.service == nil || c.service.repository == nil {
		return invocation, errors.New("wardrobe unavailable")
	}
	if _, err := capabilityExecutionArguments(invocation, wardrobeWearDefinition()); err != nil {
		return invocation, err
	}
	var revision int
	if err := c.service.repository.Pool().QueryRow(ctx, `SELECT revision FROM public.fluctlight_wardrobe_states WHERE fluctlight_id=$1`, invocation.Metadata.FluctlightID).Scan(&revision); err != nil {
		return invocation, err
	}
	if fence, ok := ctx.Value(wardrobeSelectionFenceKey{}).(wardrobeSelectionFence); ok {
		revision = fence.Revision
	}
	return withCapabilityPreparedData(invocation, "expected_wardrobe_revision", revision)
}
func (c wardrobeWearCapability) ExecuteTx(ctx context.Context, tx pgx.Tx, invocation CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	args, err := capabilityExecutionArguments(invocation, wardrobeWearDefinition())
	if err != nil {
		return failedCapabilityResult(invocation, "invalid_arguments", false), err
	}
	prepared, err := decodeCapabilityPreparedPayload(invocation.PreparedPayload)
	if err != nil {
		return failedCapabilityResult(invocation, "wearing_plan_invalid", false), err
	}
	fluctlightID := invocation.Metadata.FluctlightID
	at := time.Now().UTC()
	if c.service.clock != nil {
		at = c.service.clock().UTC()
	}
	if _, ok := ctx.Value(wardrobeSelectionFenceKey{}).(wardrobeSelectionFence); ok {
		if err := lockLifeContextTx(ctx, tx, fluctlightID); err != nil {
			return failedCapabilityResult(invocation, "life_context_unavailable", true), err
		}
	}
	if err := requireAwakeLifeTx(ctx, tx, fluctlightID, at); err != nil {
		return failedCapabilityResult(invocation, "life_state_sleeping", false), err
	}
	var revision int
	var wearingState string
	if err := tx.QueryRow(ctx, `SELECT revision,wearing_state FROM public.fluctlight_wardrobe_states WHERE fluctlight_id=$1 FOR UPDATE`, fluctlightID).Scan(&revision, &wearingState); err != nil {
		return failedCapabilityResult(invocation, "wardrobe_state_unavailable", true), err
	}
	if fence, ok := ctx.Value(wardrobeSelectionFenceKey{}).(wardrobeSelectionFence); ok {
		var profile, status string
		if err := tx.QueryRow(ctx, `SELECT r.active_profile_id,f.status FROM public.fluctlight_personality_runtime r JOIN public.fluctlights f ON f.id=r.fluctlight_id WHERE r.fluctlight_id=$1 FOR SHARE OF f`, fluctlightID).Scan(&profile, &status); err != nil {
			return failedCapabilityResult(invocation, "persona_runtime_unavailable", true), err
		}
		_, life, err := resolveLifeContextSnapshotWith(ctx, tx, fluctlightID, at)
		if err != nil {
			return failedCapabilityResult(invocation, "life_context_unavailable", true), err
		}
		if status != "active" || fence.LifeRevision == "" || fence.ProfileID != profile || fence.Revision != revision || fence.LifeRevision != stringValue(life["context_revision"]) {
			return handledCapabilityResult(invocation, "wardrobe_selection_stale", "awaiting_state", map[string]any{"revision": revision}), nil
		}
	}
	if revision != intValue(prepared.Data["expected_wardrobe_revision"]) {
		return failedCapabilityResult(invocation, "wardrobe_revision_conflict", true), ErrConflict
	}
	mode := stringValue(args["mode"])
	selected := map[string]string{}
	for _, rawID := range arrayValue(args["item_ids"]) {
		id := strings.TrimSpace(stringValue(rawID))
		var slot, availability, itemKind, ownership string
		var verified, alreadyWorn bool
		err := tx.QueryRow(ctx, `SELECT slot,availability,item_kind,ownership,`+inventorySourceVerifiedSQL+`,EXISTS(SELECT 1 FROM public.fluctlight_worn_items w WHERE w.fluctlight_id=i.fluctlight_id AND w.item_id=i.id) FROM public.fluctlight_wardrobe_items i WHERE fluctlight_id=$1 AND id=$2 FOR SHARE`, fluctlightID, id).Scan(&slot, &availability, &itemKind, &ownership, &verified, &alreadyWorn)
		if errors.Is(err, pgx.ErrNoRows) {
			return failedCapabilityResultDetail(invocation, "wardrobe_item_not_found", false, id), ErrNotFound
		}
		if err != nil {
			return failedCapabilityResult(invocation, "wardrobe_item_read_failed", true), err
		}
		if itemKind != "wearable" {
			return failedCapabilityResult(invocation, "item_not_wearable", false), ErrInvalidArguments
		}
		if !verified || (ownership != "owned" && ownership != "borrowed" && !alreadyWorn) {
			return failedCapabilityResult(invocation, "wardrobe_item_source_unverified", false), ErrConflict
		}
		if availability != "available" {
			return failedCapabilityResultDetail(invocation, "wardrobe_item_unavailable", false, id), ErrConflict
		}
		if prior, duplicate := selected[wardrobeSlotKey(slot, id)]; duplicate && prior != id {
			return failedCapabilityResult(invocation, "wardrobe_slot_duplicate", false), ErrInvalidArguments
		}
		selected[wardrobeSlotKey(slot, id)] = id
	}
	remove := map[string]struct{}{}
	ignoredRemovals := 0
	for _, rawSlot := range arrayValue(args["remove_slots"]) {
		slot := strings.TrimSpace(stringValue(rawSlot))
		if slot == "" {
			return failedCapabilityResult(invocation, "wardrobe_slot_invalid", false), ErrInvalidArguments
		}
		if _, duplicate := selected[slot]; duplicate {
			ignoredRemovals++
			continue
		}
		remove[slot] = struct{}{}
	}
	if mode == "partial" && len(selected) == 0 && len(remove) == 0 {
		return handledCapabilityResult(invocation, "wardrobe_no_change_requested", "no_change", map[string]any{"revision": revision}), nil
	}
	current, err := readCurrentWornItems(ctx, tx, fluctlightID)
	if err != nil {
		return failedCapabilityResult(invocation, "wearing_read_failed", true), err
	}
	before := map[string]string{}
	for _, item := range current {
		before[wardrobeSlotKey(stringValue(item["slot"]), stringValue(item["id"]))] = stringValue(item["id"])
	}
	after := map[string]string{}
	if mode != "full" {
		for slot, id := range before {
			after[slot] = id
		}
	}
	for slot := range remove {
		for key := range after {
			if wardrobeStoredSlot(key) == slot {
				delete(after, key)
			}
		}
	}
	for slot, id := range selected {
		after[slot] = id
	}
	if wearingState == "known" && jsonString(before) == jsonString(after) {
		return handledCapabilityResult(invocation, "wearing_unchanged", "no_change", map[string]any{"revision": revision, "items": current}), nil
	}
	if mode == "full" {
		if _, err := tx.Exec(ctx, `DELETE FROM public.fluctlight_worn_items WHERE fluctlight_id=$1`, fluctlightID); err != nil {
			return failedCapabilityResult(invocation, "wearing_update_failed", true), err
		}
	} else {
		for slot := range remove {
			if _, err := tx.Exec(ctx, `DELETE FROM public.fluctlight_worn_items WHERE fluctlight_id=$1 AND slot=$2`, fluctlightID, slot); err != nil {
				return failedCapabilityResult(invocation, "wearing_update_failed", true), err
			}
		}
	}
	for slot, id := range selected {
		if err := putWornItemTx(ctx, tx, fluctlightID, wardrobeStoredSlot(slot), id); err != nil {
			return failedCapabilityResult(invocation, "wearing_update_failed", true), err
		}
	}
	newRevision := revision + 1
	if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_wardrobe_states SET revision=$2,wearing_state='known',updated_at=now() WHERE fluctlight_id=$1`, fluctlightID, newRevision); err != nil {
		return failedCapabilityResult(invocation, "wearing_update_failed", true), err
	}
	worn, err := readCurrentWornItems(ctx, tx, fluctlightID)
	if err != nil {
		return failedCapabilityResult(invocation, "wearing_read_failed", true), err
	}
	if err := recordWearingTransitionTx(ctx, tx, fluctlightID, "tool_action", invocation.SourceFactID, newRevision, current, at); err != nil {
		return failedCapabilityResult(invocation, "wearing_history_failed", true), err
	}
	if err := appendOutboxTx(ctx, tx, "wardrobe.wearing.changed", "fluctlight", fluctlightID, fluctlightID, invocation.SourceFactID,
		"wardrobe:"+fluctlightID, "wardrobe-wear:"+fluctlightID+":"+stableDigest(capabilityOperationID(invocation)), map[string]any{"revision": newRevision, "item_ids": args["item_ids"]}); err != nil {
		return failedCapabilityResult(invocation, "wearing_event_failed", true), err
	}
	return CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "completed",
		Output:            map[string]any{"revision": newRevision, "items": worn, "mode": mode, "ignored_redundant_removals": ignoredRemovals},
		ProviderRequestID: invocation.ProviderRequestID, CorrelationID: fmt.Sprintf("wardrobe:%s:%d", fluctlightID, newRevision)}, nil
}

const wardrobeOutfitSaveCapabilityName = "wardrobe.outfit.save"

type wardrobeOutfitSaveCapability struct{ service *WardrobeService }

func wardrobeOutfitSaveDefinition() CapabilityDefinition {
	return CapabilityDefinition{
		Name: wardrobeOutfitSaveCapabilityName, Version: "v1", Type: CapabilityTypeAction,
		Description:   "Save existing item IDs as a speaking-profile outfit; no wearing or acquisition.",
		Surfaces:      []CapabilitySurface{CapabilitySurfaceConversation, CapabilitySurfaceWakeUp},
		FailurePolicy: FailurePolicyOptionalInternal,
		InputSchema: objectSchema(map[string]any{
			"tags":      map[string]any{"type": "array", "maxItems": 32, "items": map[string]any{"type": "string", "minLength": 1, "maxLength": 40}},
			"outfit_id": map[string]any{"type": "string", "maxLength": 128},
			"name":      map[string]any{"type": "string", "minLength": 1, "maxLength": 128},
			"item_ids": map[string]any{"type": "array", "minItems": 1, "maxItems": 16,
				"items": map[string]any{"type": "string", "minLength": 1, "maxLength": 128}},
		}, []string{"name", "item_ids"}, false),
		OutputSchema: openObjectSchema(), SideEffectClass: "native_projection", SuccessBoundary: "wardrobe_outfit_saved",
		ConcurrencyClass: "exclusive", SupportsRetry: true,
		ModelResultOmitFields: []string{"revision", "wardrobe_revision"},
	}
}

func (c wardrobeOutfitSaveCapability) Definition() CapabilityDefinition {
	return wardrobeOutfitSaveDefinition()
}
func (c wardrobeOutfitSaveCapability) RequiredContext() []ContextSlot { return nil }
func (c wardrobeOutfitSaveCapability) Execute(_ context.Context, invocation CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	return executeToolRequired(invocation)
}
func (c wardrobeOutfitSaveCapability) Prepare(ctx context.Context, invocation CapabilityInvocation, _ CapabilityContext) (CapabilityInvocation, error) {
	if c.service == nil || c.service.repository == nil {
		return invocation, errors.New("wardrobe unavailable")
	}
	args, err := capabilityExecutionArguments(invocation, wardrobeOutfitSaveDefinition())
	if err != nil {
		return invocation, err
	}
	var wardrobeRevision int
	if err := c.service.repository.Pool().QueryRow(ctx, `SELECT revision FROM public.fluctlight_wardrobe_states WHERE fluctlight_id=$1`, invocation.Metadata.FluctlightID).Scan(&wardrobeRevision); err != nil {
		return invocation, err
	}
	profileID, err := c.service.effectiveProfileID(ctx, c.service.repository.Pool(), invocation.Metadata.FluctlightID, invocation.Metadata.WorkingProfileID)
	if err != nil {
		return invocation, err
	}
	outfitRevision := 0
	if outfitID := strings.TrimSpace(stringValue(args["outfit_id"])); outfitID != "" {
		err := c.service.repository.Pool().QueryRow(ctx, `SELECT revision FROM public.fluctlight_wardrobe_outfits WHERE fluctlight_id=$1 AND id=$2`, invocation.Metadata.FluctlightID, outfitID).Scan(&outfitRevision)
		if errors.Is(err, pgx.ErrNoRows) {
			return invocation, ErrNotFound
		}
		if err != nil {
			return invocation, err
		}
	}
	invocation, err = withCapabilityPreparedData(invocation, "expected_wardrobe_revision", wardrobeRevision)
	if err != nil {
		return invocation, err
	}
	invocation, err = withCapabilityPreparedData(invocation, "expected_profile_id", profileID)
	if err != nil {
		return invocation, err
	}
	return withCapabilityPreparedData(invocation, "expected_outfit_revision", outfitRevision)
}
func (c wardrobeOutfitSaveCapability) ExecuteTx(ctx context.Context, tx pgx.Tx, invocation CapabilityInvocation, _ CapabilityContext) (CapabilityResult, error) {
	args, err := capabilityExecutionArguments(invocation, wardrobeOutfitSaveDefinition())
	if err != nil {
		return failedCapabilityResult(invocation, "invalid_arguments", false), err
	}
	payload, err := decodeCapabilityPreparedPayload(invocation.PreparedPayload)
	if err != nil {
		return failedCapabilityResult(invocation, "outfit_plan_invalid", false), err
	}
	fluctlightID := invocation.Metadata.FluctlightID
	profileID, err := c.service.effectiveProfileID(ctx, tx, fluctlightID, invocation.Metadata.WorkingProfileID)
	if err != nil {
		return failedCapabilityResult(invocation, "outfit_profile_unavailable", true), err
	}
	if profileID != stringValue(payload.Data["expected_profile_id"]) {
		return failedCapabilityResult(invocation, "outfit_profile_changed", true), ErrConflict
	}
	tags, err := normalizedOutfitTags(args["tags"])
	if err != nil {
		return failedCapabilityResult(invocation, "outfit_tags_invalid", false), err
	}
	_, tagsSet := args["tags"]
	output, err := c.service.saveOutfitTx(ctx, tx, wardrobeOutfitWrite{FluctlightID: fluctlightID, ProfileID: profileID, OutfitID: stringValue(args["outfit_id"]), Name: stringValue(args["name"]), Tags: tags, TagsSet: tagsSet, OperationID: capabilityOperationID(invocation), SourceRef: invocation.SourceFactID, ItemIDs: decisionServiceRefValues(args["item_ids"]), ExpectedWardrobeRevision: intValue(payload.Data["expected_wardrobe_revision"]), ExpectedOutfitRevision: intValue(payload.Data["expected_outfit_revision"])})
	if err != nil {
		code, retry := capabilityErrorInfo(err, "outfit_save_failed", true)
		return failedCapabilityResult(invocation, code, retry), err
	}
	return CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "completed", Output: output, ProviderRequestID: invocation.ProviderRequestID, CorrelationID: "wardrobe-outfit:" + stringValue(output["outfit_id"])}, nil
}

// AddWardrobeItems adds one or more items to the fluctlight's wardrobe from owner governance.
func (a *App) AddWardrobeItems(ctx context.Context, actorID, fluctlightID string, payload map[string]any) (map[string]any, error) {
	if _, err := a.DB.GetFluctlight(ctx, fluctlightID, actorID); err != nil {
		return nil, err
	}
	var rawItems []any
	if items, ok := payload["items"].([]any); ok && len(items) > 0 {
		rawItems = items
	} else if len(payload) > 0 {
		rawItems = []any{payload}
	}
	if len(rawItems) == 0 {
		return nil, errors.New("no_wardrobe_items_provided")
	}

	type itemToAdd struct {
		clothingType string
		seasons      []string
		seasonsSet   bool
		id           string
		itemKind     string
		category     string
		slot         string
		description  string
		ownership    string
		availability string
		worn         bool
	}

	toAdd := make([]itemToAdd, 0, len(rawItems))
	for i, raw := range rawItems {
		m := mapValue(raw)
		itemKind := firstString(m["item_kind"], "wearable")
		category := strings.TrimSpace(stringValue(m["category"]))
		slot := strings.TrimSpace(stringValue(m["slot"]))
		description := strings.TrimSpace(stringValue(m["description"]))
		ownership := strings.TrimSpace(stringValue(m["ownership"]))
		if ownership == "" {
			ownership = "owned"
		}
		availability := strings.TrimSpace(stringValue(m["availability"]))
		if availability == "" {
			availability = "available"
		}
		if (itemKind != "wearable" && itemKind != "object") || category == "" || description == "" || (itemKind == "wearable" && slot == "") || (itemKind == "object" && slot != "") {
			return nil, fmt.Errorf("item %d: category and description are required; wearable items need a slot and ordinary objects must have no slot", i)
		}
		if ownership != "owned" && ownership != "borrowed" && ownership != "unknown" {
			return nil, fmt.Errorf("item %d: invalid ownership %q", i, ownership)
		}
		if availability != "available" && availability != "unavailable" && availability != "lost" {
			return nil, fmt.Errorf("item %d: invalid availability %q", i, availability)
		}
		id := strings.TrimSpace(stringValue(m["id"]))
		if id == "" {
			id = "wardrobe_" + randomID("item_")
		}
		worn, _ := m["worn"].(bool)
		if itemKind == "object" && worn {
			return nil, fmt.Errorf("item %d: ordinary objects cannot be worn", i)
		}
		metadata := cloneMap(m)
		metadata["item_kind"] = itemKind
		metadata["slot"] = slot
		clothing, seasons, metaErr := wardrobeMetadata(metadata)
		if metaErr != nil {
			return nil, metaErr
		}
		_, seasonsSet := m["seasons"]
		toAdd = append(toAdd, itemToAdd{
			id: id, clothingType: clothing, seasons: seasons, seasonsSet: seasonsSet,
			itemKind:     itemKind,
			category:     category,
			slot:         slot,
			description:  description,
			ownership:    ownership,
			availability: availability,
			worn:         worn,
		})
	}

	var newRevision int
	createdItems := make([]map[string]any, 0, len(toAdd))
	err := a.withOwnerInventoryTransaction(ctx, actorID, fluctlightID, func(tx pgx.Tx) error {
		var wardrobeRevision int
		if err := tx.QueryRow(ctx, `SELECT revision FROM public.fluctlight_wardrobe_states WHERE fluctlight_id=$1 FOR UPDATE`, fluctlightID).Scan(&wardrobeRevision); err != nil {
			return err
		}
		for _, it := range toAdd {
			var previousSlot, previousKind string
			previousErr := tx.QueryRow(ctx, `SELECT slot,item_kind FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND id=$2 FOR UPDATE`, fluctlightID, it.id).Scan(&previousSlot, &previousKind)
			if previousErr != nil && !errors.Is(previousErr, pgx.ErrNoRows) {
				return previousErr
			}
			if previousErr == nil && (previousSlot != it.slot || previousKind != it.itemKind) {
				if _, err := tx.Exec(ctx, `DELETE FROM public.fluctlight_worn_items WHERE fluctlight_id=$1 AND item_id=$2`, fluctlightID, it.id); err != nil {
					return err
				}
			}
			sourceRef := "owner:" + actorID
			command, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_wardrobe_items(id,fluctlight_id,category,slot,description,ownership,availability,source_kind,source_ref,source_item_key,item_kind,clothing_type,seasons)
				VALUES($1,$2,$3,$4,$5,$6,$7,'accepted_event',$8,$9,$10,$11,$12)
				ON CONFLICT(id) DO UPDATE SET clothing_type=EXCLUDED.clothing_type,seasons=CASE WHEN $13 THEN EXCLUDED.seasons ELSE public.fluctlight_wardrobe_items.seasons END,item_kind=EXCLUDED.item_kind,category=EXCLUDED.category,slot=EXCLUDED.slot,description=EXCLUDED.description,ownership=EXCLUDED.ownership,availability=EXCLUDED.availability,revision=public.fluctlight_wardrobe_items.revision+1,updated_at=now() WHERE public.fluctlight_wardrobe_items.fluctlight_id=EXCLUDED.fluctlight_id AND public.fluctlight_wardrobe_items.archived_at IS NULL`,
				it.id, fluctlightID, it.category, it.slot, it.description, it.ownership, it.availability, sourceRef, it.id, it.itemKind, nilIfEmpty(it.clothingType), it.seasons, it.seasonsSet)
			if err != nil {
				return err
			}
			if command.RowsAffected() != 1 {
				return ErrUnauthorized
			}
			if it.itemKind == "object" || it.availability != "available" {
				if _, err := tx.Exec(ctx, `DELETE FROM public.fluctlight_worn_items WHERE fluctlight_id=$1 AND item_id=$2`, fluctlightID, it.id); err != nil {
					return err
				}
			}
			if it.itemKind != "object" || it.availability != "available" || (it.ownership != "owned" && it.ownership != "borrowed") {
				if err := clearUnavailableItemUseTx(ctx, tx, fluctlightID, it.id, "owner:"+actorID, a.now()); err != nil {
					return err
				}
			}
			if it.worn && it.availability == "available" {
				if err := putWornItemTx(ctx, tx, fluctlightID, it.slot, it.id); err != nil {
					return err
				}
			}
			createdItems = append(createdItems, map[string]any{
				"id":           it.id,
				"item_kind":    it.itemKind,
				"category":     it.category,
				"slot":         it.slot,
				"description":  it.description,
				"ownership":    it.ownership,
				"availability": it.availability,
			})
		}
		newRevision = wardrobeRevision + 1
		if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_wardrobe_states SET revision=$2,updated_at=now() WHERE fluctlight_id=$1`, fluctlightID, newRevision); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"items":             createdItems,
		"wardrobe_revision": newRevision,
	}, nil
}

// UpdateWardrobeItem updates an existing wardrobe item.
func (a *App) UpdateWardrobeItem(ctx context.Context, actorID, fluctlightID, itemID string, payload map[string]any) (map[string]any, error) {
	if _, err := a.DB.GetFluctlight(ctx, fluctlightID, actorID); err != nil {
		return nil, err
	}
	itemID = strings.TrimSpace(itemID)
	if itemID == "" {
		return nil, ErrInvalidArguments
	}
	var newRevision int
	var updatedItem map[string]any
	err := a.withOwnerInventoryTransaction(ctx, actorID, fluctlightID, func(tx pgx.Tx) error {
		var category, slot, description, ownership, availability, itemKind string
		var itemRevision int
		var clothing string
		var seasons []string
		if err := tx.QueryRow(ctx, `SELECT category,slot,description,ownership,availability,item_kind,revision,COALESCE(clothing_type,''),seasons FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND id=$2 AND archived_at IS NULL FOR UPDATE`, fluctlightID, itemID).Scan(&category, &slot, &description, &ownership, &availability, &itemKind, &itemRevision, &clothing, &seasons); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		beforeSlot := slot
		if val := strings.TrimSpace(stringValue(payload["category"])); val != "" {
			category = val
		}
		if val, exists := payload["item_kind"]; exists {
			itemKind = stringValue(val)
		}
		if val, exists := payload["slot"]; exists {
			slot = strings.TrimSpace(stringValue(val))
		}
		if val := strings.TrimSpace(stringValue(payload["description"])); val != "" {
			description = val
		}
		if val := strings.TrimSpace(stringValue(payload["ownership"])); val != "" {
			if val != "owned" && val != "borrowed" && val != "unknown" {
				return errors.New("invalid_ownership")
			}
			ownership = val
		}
		if val := strings.TrimSpace(stringValue(payload["availability"])); val != "" {
			if val != "available" && val != "unavailable" && val != "lost" {
				return errors.New("invalid_availability")
			}
			availability = val
		}
		if (itemKind != "wearable" && itemKind != "object") || (itemKind == "object" && slot != "") || (itemKind == "wearable" && slot == "") {
			return ErrInvalidArguments
		}
		if itemKind != "object" || availability != "available" || (ownership != "owned" && ownership != "borrowed") {
			if err := clearUnavailableItemUseTx(ctx, tx, fluctlightID, itemID, "owner:"+actorID, a.now()); err != nil {
				return err
			}
		}
		if availability != "available" || itemKind != "wearable" || beforeSlot != slot {
			if _, err := tx.Exec(ctx, `DELETE FROM public.fluctlight_worn_items WHERE fluctlight_id=$1 AND item_id=$2`, fluctlightID, itemID); err != nil {
				return err
			}
		}

		meta := cloneMap(payload)
		meta["item_kind"] = itemKind
		meta["slot"] = slot
		if _, present := meta["seasons"]; !present {
			meta["seasons"] = seasons
		}
		var metaErr error
		clothing, seasons, metaErr = wardrobeMetadata(meta)
		if metaErr != nil {
			return metaErr
		}
		if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_wardrobe_items SET category=$3,slot=$4,description=$5,ownership=$6,availability=$7,item_kind=$8,clothing_type=$9,seasons=$10,revision=revision+1,updated_at=now() WHERE fluctlight_id=$1 AND id=$2`, fluctlightID, itemID, category, slot, description, ownership, availability, itemKind, nilIfEmpty(clothing), seasons); err != nil {
			return err
		}
		var wardrobeRevision int
		if err := tx.QueryRow(ctx, `SELECT revision FROM public.fluctlight_wardrobe_states WHERE fluctlight_id=$1 FOR UPDATE`, fluctlightID).Scan(&wardrobeRevision); err != nil {
			return err
		}
		newRevision = wardrobeRevision + 1
		if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_wardrobe_states SET revision=$2,updated_at=now() WHERE fluctlight_id=$1`, fluctlightID, newRevision); err != nil {
			return err
		}
		updatedItem = map[string]any{
			"id":           itemID,
			"category":     category,
			"slot":         slot,
			"description":  description,
			"ownership":    ownership,
			"availability": availability,
			"revision":     itemRevision + 1,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"item":              updatedItem,
		"wardrobe_revision": newRevision,
	}, nil
}

// DeleteWardrobeItem removes an item from the wardrobe.
func (a *App) DeleteWardrobeItem(ctx context.Context, actorID, fluctlightID, itemID string) (map[string]any, error) {
	if _, err := a.DB.GetFluctlight(ctx, fluctlightID, actorID); err != nil {
		return nil, err
	}
	itemID = strings.TrimSpace(itemID)
	if itemID == "" {
		return nil, ErrInvalidArguments
	}
	var newRevision int
	err := a.withOwnerInventoryTransaction(ctx, actorID, fluctlightID, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM public.fluctlight_worn_items WHERE fluctlight_id=$1 AND item_id=$2`, fluctlightID, itemID); err != nil {
			return err
		}
		if err := clearUnavailableItemUseTx(ctx, tx, fluctlightID, itemID, "owner:"+actorID, a.now()); err != nil {
			return err
		}
		cmd, err := tx.Exec(ctx, `UPDATE public.fluctlight_wardrobe_items SET archived_at=now(),availability='unavailable',revision=revision+1,updated_at=now() WHERE fluctlight_id=$1 AND id=$2 AND archived_at IS NULL`, fluctlightID, itemID)
		if err != nil {
			return err
		}
		if cmd.RowsAffected() == 0 {
			return ErrNotFound
		}
		var wardrobeRevision int
		if err := tx.QueryRow(ctx, `SELECT revision FROM public.fluctlight_wardrobe_states WHERE fluctlight_id=$1 FOR UPDATE`, fluctlightID).Scan(&wardrobeRevision); err != nil {
			return err
		}
		newRevision = wardrobeRevision + 1
		if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_wardrobe_states SET revision=$2,updated_at=now() WHERE fluctlight_id=$1`, fluctlightID, newRevision); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"deleted": true, "archived": true,
		"id":                itemID,
		"wardrobe_revision": newRevision,
	}, nil
}
