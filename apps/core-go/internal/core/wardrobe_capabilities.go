package core

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

const (
	wardrobeInspectCapabilityName = "wardrobe.inspect"
	wardrobeWearCapabilityName    = "wardrobe.wear"
)

type WardrobeService struct{ repository *PostgresRepository }

func newWardrobeService(app *App) *WardrobeService {
	if app == nil {
		return &WardrobeService{}
	}
	return &WardrobeService{repository: app.DB}
}

type wardrobeInspectCapability struct{ service *WardrobeService }
type wardrobeWearCapability struct{ service *WardrobeService }

func wardrobeInspectDefinition() CapabilityDefinition {
	return CapabilityDefinition{
		Name: wardrobeInspectCapabilityName, Version: "v1", Type: CapabilityTypeQuery,
		Description:   "Inspect recorded clothing items, saved outfits, or the actual current wearing state. A missing recorded item does not prove the whole wardrobe is complete.",
		Surfaces:      []CapabilitySurface{CapabilitySurfaceConversation, CapabilitySurfaceWakeUp, CapabilitySurfaceNativeCognition},
		FailurePolicy: FailurePolicyOptionalInternal,
		InputSchema: objectSchema(map[string]any{
			"operation": enumStringSchema("list", "detail", "wearing", "outfits", "outfit_detail"),
			"item_id":   map[string]any{"type": "string", "maxLength": 128},
			"outfit_id": map[string]any{"type": "string", "maxLength": 128},
			"category":  map[string]any{"type": "string", "maxLength": 64},
			"query":     map[string]any{"type": "string", "maxLength": 128},
			"cursor":    map[string]any{"type": "string", "maxLength": 128},
			"limit":     map[string]any{"type": "integer", "minimum": 1, "maximum": 30},
		}, []string{"operation"}, false),
		OutputSchema: openObjectSchema(), SideEffectClass: "read_only", SuccessBoundary: "query_result_available",
		ConcurrencyClass: "parallel", SupportsRetry: true,
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
	output, err := c.service.inspectWardrobe(ctx, invocation.Metadata.FluctlightID, invocation.Metadata.WorkingProfileID, args)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return failedCapabilityResultDetail(invocation, "wardrobe_item_not_found", false, "item is not in the recorded wardrobe"), err
		}
		return failedCapabilityResult(invocation, "wardrobe_query_failed", true), err
	}
	return CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "completed", Output: output,
		ProviderRequestID: invocation.ProviderRequestID, CorrelationID: "wardrobe-query:" + stableDigest(invocation.CallID)}, nil
}

func (s *WardrobeService) inspectWardrobe(ctx context.Context, fluctlightID, workingProfileID string, args map[string]any) (map[string]any, error) {
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
		rows, err := query.Query(ctx, `SELECT id,category,slot,description,ownership,availability,source_kind,revision FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND id>$2 AND ($3='' OR category=$3) AND ($4='' OR position(lower($4) in lower(description))>0) ORDER BY id LIMIT $5`, fluctlightID, cursor, category, phrase, limit+1)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		items := make([]map[string]any, 0, limit)
		for rows.Next() {
			var id, itemCategory, slot, description, ownership, availability, sourceKind string
			var itemRevision int
			if err := rows.Scan(&id, &itemCategory, &slot, &description, &ownership, &availability, &sourceKind, &itemRevision); err != nil {
				return nil, err
			}
			items = append(items, map[string]any{"id": id, "category": itemCategory, "slot": slot, "description": description,
				"ownership": ownership, "availability": availability, "source_kind": sourceKind, "revision": itemRevision})
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
		rows, err := query.Query(ctx, `SELECT id,profile_id,name,revision FROM public.fluctlight_wardrobe_outfits WHERE fluctlight_id=$1 AND (profile_id='' OR profile_id=$2) ORDER BY id LIMIT 31`, fluctlightID, profileID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		outfits := make([]map[string]any, 0)
		for rows.Next() {
			var id, profileID, name string
			var outfitRevision int
			if err := rows.Scan(&id, &profileID, &name, &outfitRevision); err != nil {
				return nil, err
			}
			outfits = append(outfits, map[string]any{"id": id, "profile_id": profileID, "name": name, "revision": outfitRevision})
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
		output["has_more"] = len(outfits) > 30
		if len(outfits) > 30 {
			outfits = outfits[:30]
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
		var outfitRevision int
		err = query.QueryRow(ctx, `SELECT profile_id,name,revision FROM public.fluctlight_wardrobe_outfits WHERE fluctlight_id=$1 AND id=$2 AND (profile_id='' OR profile_id=$3)`, fluctlightID, outfitID, profileID).Scan(&ownerProfile, &name, &outfitRevision)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		if err != nil {
			return nil, err
		}
		rows, err := query.Query(ctx, `SELECT r.slot,i.id,i.description,i.availability FROM public.fluctlight_wardrobe_outfit_items r JOIN public.fluctlight_wardrobe_items i ON i.fluctlight_id=r.fluctlight_id AND i.id=r.item_id WHERE r.fluctlight_id=$1 AND r.outfit_id=$2 ORDER BY r.slot`, fluctlightID, outfitID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		items := make([]map[string]any, 0)
		for rows.Next() {
			var slot, id, description, availability string
			if err := rows.Scan(&slot, &id, &description, &availability); err != nil {
				return nil, err
			}
			items = append(items, map[string]any{"slot": slot, "id": id, "description": description, "availability": availability})
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
		output["outfit"] = map[string]any{"id": outfitID, "profile_id": ownerProfile, "name": name, "revision": outfitRevision, "items": items}
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
	rows, err := query.Query(ctx, `SELECT w.slot,i.id,i.category,i.description,i.ownership,i.availability FROM public.fluctlight_worn_items w JOIN public.fluctlight_wardrobe_items i ON i.fluctlight_id=w.fluctlight_id AND i.id=w.item_id WHERE w.fluctlight_id=$1 ORDER BY w.slot`, fluctlightID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]map[string]any, 0)
	for rows.Next() {
		var slot, id, category, description, ownership, availability string
		if err := rows.Scan(&slot, &id, &category, &description, &ownership, &availability); err != nil {
			return nil, err
		}
		items = append(items, map[string]any{"slot": slot, "id": id, "category": category, "description": description,
			"ownership": ownership, "availability": availability})
	}
	return items, rows.Err()
}

func wardrobeWearDefinition() CapabilityDefinition {
	return CapabilityDefinition{
		Name: wardrobeWearCapabilityName, Version: "v1", Type: CapabilityTypeAction,
		Description:   "Actually change what you are wearing using available recorded item IDs. Full replaces the entire outfit; partial changes only selected or removed slots. This never buys or creates clothing.",
		Surfaces:      []CapabilitySurface{CapabilitySurfaceConversation, CapabilitySurfaceWakeUp, CapabilitySurfaceNativeCognition},
		FailurePolicy: FailurePolicyRequiredForVisibleClaim,
		InputSchema: objectSchema(map[string]any{
			"mode":         enumStringSchema("full", "partial"),
			"item_ids":     map[string]any{"type": "array", "maxItems": 16, "items": map[string]any{"type": "string", "minLength": 1, "maxLength": 128}},
			"remove_slots": map[string]any{"type": "array", "maxItems": 16, "items": map[string]any{"type": "string", "minLength": 1, "maxLength": 64}},
		}, []string{"mode", "item_ids"}, false),
		OutputSchema: openObjectSchema(), SideEffectClass: "native_projection", SuccessBoundary: "wearing_state_committed",
		ConcurrencyClass: "exclusive", SupportsRetry: true,
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
	var revision int
	if err := tx.QueryRow(ctx, `SELECT revision FROM public.fluctlight_wardrobe_states WHERE fluctlight_id=$1 FOR UPDATE`, fluctlightID).Scan(&revision); err != nil {
		return failedCapabilityResult(invocation, "wardrobe_state_unavailable", true), err
	}
	if revision != intValue(prepared.Data["expected_wardrobe_revision"]) {
		return failedCapabilityResult(invocation, "wardrobe_revision_conflict", true), ErrConflict
	}
	mode := stringValue(args["mode"])
	selected := map[string]string{}
	for _, rawID := range arrayValue(args["item_ids"]) {
		id := strings.TrimSpace(stringValue(rawID))
		var slot, availability string
		err := tx.QueryRow(ctx, `SELECT slot,availability FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND id=$2 FOR SHARE`, fluctlightID, id).Scan(&slot, &availability)
		if errors.Is(err, pgx.ErrNoRows) {
			return failedCapabilityResultDetail(invocation, "wardrobe_item_not_found", false, id), ErrNotFound
		}
		if err != nil {
			return failedCapabilityResult(invocation, "wardrobe_item_read_failed", true), err
		}
		if availability != "available" {
			return failedCapabilityResultDetail(invocation, "wardrobe_item_unavailable", false, id), ErrConflict
		}
		if _, duplicate := selected[slot]; duplicate {
			return failedCapabilityResult(invocation, "wardrobe_slot_duplicate", false), ErrInvalidArguments
		}
		selected[slot] = id
	}
	remove := map[string]struct{}{}
	for _, rawSlot := range arrayValue(args["remove_slots"]) {
		slot := strings.TrimSpace(stringValue(rawSlot))
		if slot == "" {
			return failedCapabilityResult(invocation, "wardrobe_slot_invalid", false), ErrInvalidArguments
		}
		if _, duplicate := selected[slot]; duplicate {
			return failedCapabilityResult(invocation, "wardrobe_slot_conflict", false), ErrInvalidArguments
		}
		remove[slot] = struct{}{}
	}
	if mode == "partial" && len(selected) == 0 && len(remove) == 0 {
		return failedCapabilityResult(invocation, "wardrobe_no_change_requested", false), ErrInvalidArguments
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
		if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_worn_items(fluctlight_id,slot,item_id) VALUES($1,$2,$3) ON CONFLICT(fluctlight_id,slot) DO UPDATE SET item_id=EXCLUDED.item_id,changed_at=now()`, fluctlightID, slot, id); err != nil {
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
	if err := appendOutboxTx(ctx, tx, "wardrobe.wearing.changed", "fluctlight", fluctlightID, fluctlightID, invocation.SourceFactID,
		"wardrobe:"+fluctlightID, "wardrobe-wear:"+fluctlightID+":"+stableDigest(capabilityOperationID(invocation)), map[string]any{"revision": newRevision, "item_ids": args["item_ids"]}); err != nil {
		return failedCapabilityResult(invocation, "wearing_event_failed", true), err
	}
	return CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "completed",
		Output:            map[string]any{"revision": newRevision, "items": worn, "mode": mode},
		ProviderRequestID: invocation.ProviderRequestID, CorrelationID: fmt.Sprintf("wardrobe:%s:%d", fluctlightID, newRevision)}, nil
}

const wardrobeOutfitSaveCapabilityName = "wardrobe.outfit.save"

type wardrobeOutfitSaveCapability struct{ service *WardrobeService }

func wardrobeOutfitSaveDefinition() CapabilityDefinition {
	return CapabilityDefinition{
		Name: wardrobeOutfitSaveCapabilityName, Version: "v1", Type: CapabilityTypeAction,
		Description:   "Save a named combination of existing wardrobe item IDs for the current speaking profile. This does not change current clothing or create inventory.",
		Surfaces:      []CapabilitySurface{CapabilitySurfaceConversation, CapabilitySurfaceWakeUp},
		FailurePolicy: FailurePolicyOptionalInternal,
		InputSchema: objectSchema(map[string]any{
			"outfit_id": map[string]any{"type": "string", "maxLength": 128},
			"name":      map[string]any{"type": "string", "minLength": 1, "maxLength": 128},
			"item_ids": map[string]any{"type": "array", "minItems": 1, "maxItems": 16,
				"items": map[string]any{"type": "string", "minLength": 1, "maxLength": 128}},
		}, []string{"name", "item_ids"}, false),
		OutputSchema: openObjectSchema(), SideEffectClass: "native_projection", SuccessBoundary: "wardrobe_outfit_saved",
		ConcurrencyClass: "exclusive", SupportsRetry: true,
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
	var wardrobeRevision int
	if err := tx.QueryRow(ctx, `SELECT revision FROM public.fluctlight_wardrobe_states WHERE fluctlight_id=$1 FOR UPDATE`, fluctlightID).Scan(&wardrobeRevision); err != nil {
		return failedCapabilityResult(invocation, "wardrobe_state_unavailable", true), err
	}
	if wardrobeRevision != intValue(payload.Data["expected_wardrobe_revision"]) {
		return failedCapabilityResult(invocation, "wardrobe_revision_conflict", true), ErrConflict
	}
	outfitID := strings.TrimSpace(stringValue(args["outfit_id"]))
	name := strings.TrimSpace(stringValue(args["name"]))
	if outfitID == "" {
		outfitID = "outfit_" + stableDigest(fluctlightID+"\x1f"+profileID+"\x1f"+capabilityOperationID(invocation))
	}
	items := make(map[string]string)
	for _, rawID := range arrayValue(args["item_ids"]) {
		id := strings.TrimSpace(stringValue(rawID))
		var slot string
		err := tx.QueryRow(ctx, `SELECT slot FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND id=$2 FOR SHARE`, fluctlightID, id).Scan(&slot)
		if errors.Is(err, pgx.ErrNoRows) {
			return failedCapabilityResultDetail(invocation, "wardrobe_item_not_found", false, id), ErrNotFound
		}
		if err != nil {
			return failedCapabilityResult(invocation, "wardrobe_item_read_failed", true), err
		}
		if _, duplicate := items[slot]; duplicate {
			return failedCapabilityResult(invocation, "outfit_slot_duplicate", false), ErrInvalidArguments
		}
		items[slot] = id
	}
	previousRevision := intValue(payload.Data["expected_outfit_revision"])
	if previousRevision == 0 {
		if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_wardrobe_outfits(id,fluctlight_id,profile_id,name,revision) VALUES($1,$2,$3,$4,1)`, outfitID, fluctlightID, profileID, name); err != nil {
			return failedCapabilityResult(invocation, "outfit_insert_failed", true), err
		}
	} else {
		var storedRevision int
		var storedProfile string
		err := tx.QueryRow(ctx, `SELECT profile_id,revision FROM public.fluctlight_wardrobe_outfits WHERE fluctlight_id=$1 AND id=$2 FOR UPDATE`, fluctlightID, outfitID).Scan(&storedProfile, &storedRevision)
		if err != nil {
			return failedCapabilityResult(invocation, "outfit_read_failed", true), err
		}
		if storedProfile != profileID || storedRevision != previousRevision {
			return failedCapabilityResult(invocation, "outfit_revision_conflict", true), ErrConflict
		}
		if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_wardrobe_outfits SET name=$3,revision=revision+1,updated_at=now() WHERE fluctlight_id=$1 AND id=$2`, fluctlightID, outfitID, name); err != nil {
			return failedCapabilityResult(invocation, "outfit_update_failed", true), err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM public.fluctlight_wardrobe_outfit_items WHERE fluctlight_id=$1 AND outfit_id=$2`, fluctlightID, outfitID); err != nil {
			return failedCapabilityResult(invocation, "outfit_update_failed", true), err
		}
	}
	for slot, id := range items {
		if _, err := tx.Exec(ctx, `INSERT INTO public.fluctlight_wardrobe_outfit_items(fluctlight_id,outfit_id,slot,item_id) VALUES($1,$2,$3,$4)`, fluctlightID, outfitID, slot, id); err != nil {
			return failedCapabilityResult(invocation, "outfit_item_insert_failed", true), err
		}
	}
	newWardrobeRevision := wardrobeRevision + 1
	if _, err := tx.Exec(ctx, `UPDATE public.fluctlight_wardrobe_states SET revision=$2,updated_at=now() WHERE fluctlight_id=$1`, fluctlightID, newWardrobeRevision); err != nil {
		return failedCapabilityResult(invocation, "outfit_state_failed", true), err
	}
	if err := appendOutboxTx(ctx, tx, "wardrobe.outfit.saved", "wardrobe_outfit", outfitID, fluctlightID, invocation.SourceFactID,
		"wardrobe-outfit:"+outfitID, "wardrobe-outfit-save:"+outfitID+":"+fmt.Sprint(previousRevision+1), map[string]any{"outfit_id": outfitID, "revision": previousRevision + 1}); err != nil {
		return failedCapabilityResult(invocation, "outfit_event_failed", true), err
	}
	return CapabilityResult{CallID: invocation.CallID, CapabilityName: invocation.CapabilityName, Status: "completed",
		Output:            map[string]any{"outfit_id": outfitID, "revision": previousRevision + 1, "wardrobe_revision": newWardrobeRevision, "item_count": len(items)},
		ProviderRequestID: invocation.ProviderRequestID, CorrelationID: "wardrobe-outfit:" + outfitID}, nil
}
