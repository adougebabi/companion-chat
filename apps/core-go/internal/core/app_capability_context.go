package core

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// AppContextResolver is the initial Core resolver implementation. It loads
// only requested slots through existing owning services. It intentionally
// keeps authorization and revision checks in each capability executor as live
// guards; the resolved values are a bounded execution snapshot, not authority.
type AppContextResolver struct {
	app *App
}

func NewAppContextResolver(app *App) *AppContextResolver {
	return &AppContextResolver{app: app}
}

func (resolver *AppContextResolver) Resolve(ctx context.Context, request ContextRequest, slots []ContextSlot) (CapabilityContext, error) {
	if resolver == nil || resolver.app == nil {
		return CapabilityContext{}, fmt.Errorf("%w: app resolver unavailable", ErrContextResolve)
	}
	if err := ctx.Err(); err != nil {
		return CapabilityContext{}, fmt.Errorf("%w: %v", ErrContextResolve, err)
	}
	result := CapabilityContext{Identity: ContextIdentity{FluctlightID: request.FluctlightID, ConversationID: request.ConversationID, SourceFactID: request.SourceFactID, ActionID: request.ActionID}, extra: make(map[ContextSlot]any)}
	if len(slots) == 0 {
		return result, nil
	}
	if resolver.app.DB == nil {
		return CapabilityContext{}, fmt.Errorf("%w: app resolver database unavailable", ErrContextResolve)
	}
	ordered := make([]ContextSlot, 0, len(slots))
	seen := make(map[ContextSlot]struct{}, len(slots))
	for _, slot := range slots {
		if !knownContextSlot(slot) {
			return CapabilityContext{}, fmt.Errorf("%w: unknown slot %q", ErrContextResolve, slot)
		}
		if _, duplicate := seen[slot]; duplicate {
			continue
		}
		seen[slot] = struct{}{}
		ordered = append(ordered, slot)
	}
	needsOwner := false
	for _, slot := range ordered {
		switch slot {
		case SlotCorePersona, SlotAppearance, SlotRelationshipScope, SlotMemoryScope:
			needsOwner = true
		}
	}
	var ownerID string
	if needsOwner {
		if err := resolver.app.DB.Pool().QueryRow(ctx, `SELECT created_by_actor_id FROM public.fluctlights WHERE id=$1`, request.FluctlightID).Scan(&ownerID); err != nil {
			return CapabilityContext{}, fmt.Errorf("%w: owner: %v", ErrContextResolve, err)
		}
	}
	var cachedFluctlight *Fluctlight
	var cachedSchedule map[string]any
	var cachedLifeContext map[string]any
	var lifeContextLoaded bool
	contextAt := time.Now().UTC()
	loadFluctlight := func() (Fluctlight, error) {
		if cachedFluctlight != nil {
			return *cachedFluctlight, nil
		}
		fluctlight, err := resolver.app.DB.GetFluctlight(ctx, request.FluctlightID, ownerID)
		if err != nil {
			return Fluctlight{}, err
		}
		cachedFluctlight = &fluctlight
		return fluctlight, nil
	}
	loadSchedule := func() (map[string]any, error) {
		if lifeContextLoaded {
			return cachedSchedule, nil
		}
		schedule, life, err := resolver.app.readLifeContextSnapshotAt(ctx, request.FluctlightID, contextAt)
		if err != nil {
			return nil, err
		}
		cachedSchedule, cachedLifeContext, lifeContextLoaded = schedule, life, true
		return schedule, nil
	}
	loadLifeContext := func() (map[string]any, error) {
		if !lifeContextLoaded {
			if _, err := loadSchedule(); err != nil {
				return nil, err
			}
		}
		return cachedLifeContext, nil
	}
	for _, slot := range ordered {
		var value any
		var err error
		switch slot {
		case SlotCorePersona:
			fluctlight, loadErr := loadFluctlight()
			err = loadErr
			if err == nil {
				value = fluctlight.CorePersona
			}
		case SlotAppearance:
			fluctlight, loadErr := loadFluctlight()
			err = loadErr
			if err == nil {
				value = mapValue(fluctlight.Identity["appearance"])
			}
		case SlotSchedule:
			value, err = loadSchedule()
		case SlotCurrentLife:
			value, err = loadLifeContext()
		default:
			value, err = resolver.load(ctx, request, ownerID, slot)
		}
		if err != nil {
			return CapabilityContext{}, fmt.Errorf("%w: slot %q: %v", ErrContextResolve, slot, err)
		}
		if err := ctx.Err(); err != nil {
			return CapabilityContext{}, fmt.Errorf("%w: %v", ErrContextResolve, err)
		}
		if err := result.set(slot, value); err != nil {
			return CapabilityContext{}, err
		}
	}
	return result, nil
}

func (resolver *AppContextResolver) load(ctx context.Context, request ContextRequest, ownerID string, slot ContextSlot) (any, error) {
	app := resolver.app
	switch slot {
	case SlotCorePersona:
		fluctlight, err := app.DB.GetFluctlight(ctx, request.FluctlightID, ownerID)
		if err != nil {
			return nil, err
		}
		return fluctlight.CorePersona, nil
	case SlotCurrentState:
		state, err := app.readInnerState(ctx, request.FluctlightID)
		if err != nil {
			return nil, err
		}
		policy, profile, err := app.readAffectProfile(ctx, request.FluctlightID)
		if err != nil {
			return nil, err
		}
		state = projectAffectStateAt(state, policy, time.Now().UTC())
		state["affect_profile"] = profile
		return state, nil
	case SlotSchedule:
		schedule, err := app.readSchedule(ctx, request.FluctlightID)
		return schedule, err
	case SlotCurrentLife:
		schedule, err := app.readSchedule(ctx, request.FluctlightID)
		if err != nil {
			return nil, err
		}
		return app.resolveContext(ctx, request.FluctlightID, schedule)
	case SlotVisualIdentity:
		return app.readVisualIdentityDetail(ctx, request.FluctlightID)
	case SlotAppearance:
		fluctlight, err := app.DB.GetFluctlight(ctx, request.FluctlightID, ownerID)
		if err != nil {
			return nil, err
		}
		return mapValue(fluctlight.Identity["appearance"]), nil
	case SlotRelationshipScope:
		relationships, err := app.readRelationships(ctx, request.FluctlightID, ownerID)
		if err != nil {
			return nil, err
		}
		authorized := make([]any, 0)
		seen := make(map[string]struct{})
		addAuthorized := func(actorID string) {
			if actorID = strings.TrimSpace(actorID); actorID == "" || actorID == request.FluctlightID {
				return
			}
			if _, exists := seen[actorID]; exists {
				return
			}
			seen[actorID] = struct{}{}
			authorized = append(authorized, actorID)
		}
		if request.ConversationID != "" {
			rows, queryErr := app.DB.Pool().Query(ctx, `SELECT actor_id FROM public.conversation_participants WHERE conversation_id=$1 AND status='active' ORDER BY actor_id`, request.ConversationID)
			if queryErr != nil {
				return nil, queryErr
			}
			for rows.Next() {
				var actorID string
				if scanErr := rows.Scan(&actorID); scanErr != nil {
					rows.Close()
					return nil, scanErr
				}
				addAuthorized(actorID)
			}
			if rowsErr := rows.Err(); rowsErr != nil {
				rows.Close()
				return nil, rowsErr
			}
			rows.Close()
		} else {
			addAuthorized(ownerID)
			for _, relationship := range relationships {
				addAuthorized(stringValue(relationship["target_actor_id"]))
			}
		}
		return map[string]any{"authorized_actor_ids": authorized, "relationships": relationships}, nil
	case SlotMemoryScope:
		viewers := append([]string(nil), request.ViewerActorIDs...)
		if len(viewers) == 0 {
			viewers = []string{ownerID}
		}
		mode := MemoryConversationGlobalOnly
		if strings.TrimSpace(request.ConversationID) != "" {
			mode = MemoryConversationExact
		}
		persona, err := app.DB.GetFluctlight(ctx, request.FluctlightID, ownerID)
		if err != nil {
			return nil, err
		}
		personalitySystem := mapValue(persona.CorePersona["personality_system"])
		personalityRuntime, err := app.readPersonalityRuntime(ctx, request.FluctlightID, stringValue(personalitySystem["active_profile_id"]))
		if err != nil {
			return nil, err
		}
		cues := []MemoryQueryCue{}
		if intent := strings.TrimSpace(request.SemanticIntent); intent != "" {
			cues = append(cues, MemoryQueryCue{Kind: "capability_intent", Text: intent})
		}
		plan, err := buildMemoryQueryPlan(MemoryForCapabilityPlanner, viewers, mode, request.ConversationID, nil, stringValue(personalityRuntime["active_profile_id"]), cues, 12, 2400)
		if err != nil {
			return nil, err
		}
		memoryResult, err := app.retrieveMemoryWithPlan(ctx, ownerID, request.FluctlightID, plan)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"owner_actor_id": ownerID, "viewer_actor_ids": viewers, "conversation_mode": mode,
			"active_profile_id": personalityRuntime["active_profile_id"], "memories": memoryResult.Items,
			"retrieval_trace": memoryResult.Trace,
		}, nil
	case SlotAgency:
		goals, intentions, err := app.agencyProfile(ctx, request.FluctlightID)
		return map[string]any{"goals": goals, "intentions": intentions}, err
	case SlotRecentOutcomes:
		outcomes, err := app.readRecentActionOutcomes(ctx, request.FluctlightID, 12)
		return map[string]any{"outcomes": outcomes}, err
	default:
		return map[string]any{}, nil
	}
}
