package core

func momentReplyAuthorityEvidence(kind ContextReferenceKind, row map[string]any, projection ContextProjection) map[string]any {
	switch kind {
	case ContextReferenceScene, ContextReferenceLifeContext:
		view := copyPromptFields(row, "source", "authority_status", "scene", "activity", "location", "current_time", "timezone", "effective_at", "expires_at")
		if presence := mapValue(row["presence"]); len(presence) > 0 {
			view["presence"] = copyPromptFields(presence, "current_task", "user_presence", "effective_at", "expires_at")
		}
		if world := mapValue(row["world_environment"]); len(world) > 0 {
			view["world_environment"] = momentWorldEvidence(world, projection)
		}
		return view
	case ContextReferenceSchedule:
		return copyPromptFields(row, "local_date", "timezone", "status", "completed_before", "reschedule_policy")
	case ContextReferenceScheduleItem:
		return copyPromptFields(row, "start_at", "end_at", "activity", "scene", "location", "item_type", "status", "action_status", "priority", "flexibility", "interruption_cost", "promise_schedule_valid", "promise_state", "local_start_time", "local_end_time", "local_time_range", "local_start_at", "local_end_at")
	case ContextReferencePresence:
		return copyPromptFields(row, "current_task", "user_presence", "effective_at", "expires_at")
	case ContextReferenceState:
		return mapValue(copyPromptData(compactInnerStateForSurface(row)))
	case ContextReferenceEvolutionOverlay:
		return copyPromptFields(row, "kind", "field_path", "semantic_direction", "confidence")
	default:
		return nil
	}
}

func momentReplyCurrentState(input ReplyContextInput) map[string]any {
	projection := input.Projection
	raw := mapValue(projection.CurrentState["data"])
	result := map[string]any{}
	inner := mapValue(raw["inner_state"])
	if len(inner) == 0 {
		inner = projection.InnerState
	}
	if len(inner) > 0 {
		result["inner_state"] = compactInnerStateForSurface(inner)
	}
	life := mapValue(raw["life_context"])
	if len(life) == 0 {
		life = projection.LifeContext
	}
	if len(life) > 0 {
		result["life_context"] = momentReplyAuthorityEvidence(ContextReferenceLifeContext, life, projection)
	}
	if !input.momentAppearanceSelected {
		appearance := input.momentAppearanceView
		if appearance == nil {
			appearance = mapValue(raw["appearance"])
			if len(appearance) == 0 {
				appearance = projection.EffectiveAppearance
			}
			appearance = replyAppearanceView(appearance)
		}
		if len(appearance) > 0 {
			result["appearance"] = appearance
		}
	}
	activities := raw["active_activities"]
	if len(arrayValue(activities)) == 0 {
		activities = projection.ActiveActivities
	}
	if values := compactActiveActivitiesForSurface(activities); len(values) > 0 {
		for _, value := range values {
			delete(value, "id")
		}
		result["active_activities"] = values
	}
	if len(result) == 0 {
		return nil
	}
	return map[string]any{"subject": "actor_self", "data": result}
}

func momentReplySchedule(schedule map[string]any) map[string]any {
	view := momentReplyAuthorityEvidence(ContextReferenceSchedule, schedule, ContextProjection{})
	base := compactScheduleForProvider(schedule)
	for _, key := range []string{"items", "upcoming_items"} {
		if raw, exists := base[key]; exists {
			items := []map[string]any{}
			for _, item := range arrayValue(raw) {
				items = append(items, momentReplyAuthorityEvidence(ContextReferenceScheduleItem, mapValue(item), ContextProjection{}))
			}
			view[key] = items
		}
	}
	if current := mapValue(base["current_item"]); len(current) > 0 {
		view["current_item"] = momentReplyAuthorityEvidence(ContextReferenceScheduleItem, current, ContextProjection{})
	}
	return view
}

func momentWorldEvidence(world map[string]any, projection ContextProjection) map[string]any {
	view := copyPromptFields(world, "name", "background", "rules", "description", "flavor", "source")
	if place := mapValue(world["current_place"]); len(place) > 0 {
		view["current_place"] = copyPromptFields(place, "name", "description", "atmosphere", "source", "authority_status")
	}
	for _, key := range []string{"known_npcs", "nearby_public_npcs", "npc_opportunities"} {
		if raw, exists := world[key]; exists {
			items := []map[string]any{}
			for _, item := range arrayValue(raw) {
				row := mapValue(item)
				value := copyPromptFields(row, "name", "relationship_summary", "source", "public_role", "known", "presence_source", "knowledge", "expires_at", "status")
				actorID := firstString(row["actor_id"], stringValue(row["npc_actor_id"]))
				if actorID != "" {
					value["associated_actor"] = momentReplyAssociatedActor(actorRefForID(projection.Actors, actorID))
				}
				items = append(items, value)
			}
			view[key] = items
		}
	}
	if raw, exists := world["active_world_effects"]; exists {
		events := []map[string]any{}
		for _, item := range arrayValue(raw) {
			row := mapValue(item)
			event := copyPromptFields(row, "source", "start_at", "end_at", "description")
			effects := []map[string]any{}
			for _, effect := range arrayValue(row["effects"]) {
				effects = append(effects, copyPromptFields(mapValue(effect), "kind", "scope", "value", "body_effect"))
			}
			event["effects"] = effects
			events = append(events, event)
		}
		view["active_world_effects"] = events
	}
	return view
}
