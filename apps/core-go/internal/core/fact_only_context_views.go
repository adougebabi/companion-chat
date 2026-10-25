package core

import (
	"sort"
	"strings"
	"time"
)

// These are call-local read models. They never change the source projection or
// decide whether a Goal, relationship or inferred statement is true.
type factOnlyContextView struct {
	Facts     map[string]any
	Selection map[string]any
}

// Copy the complete value; prompt size never permits erasing a required field.
func factOnlyCopyValue(value any) any { return cloneMap(map[string]any{"value": value})["value"] }

func factOnlyFields(value map[string]any, keys ...string) map[string]any {
	result := map[string]any{}
	for _, key := range keys {
		if raw, present := value[key]; present {
			result[key] = factOnlyCopyValue(raw)
		}
	}
	return result
}

func factOnlyActor(value map[string]any) map[string]any {
	result := factOnlyFields(value, "type", "role", "label")
	if alias := stringValue(value["ref"]); alias == "actor_self" || alias == "actor_user" || alias == "actor_b" {
		result["actor"] = alias
	}
	name := strings.TrimSpace(firstString(value["display_name"], stringValue(value["name"])))
	if name != "" && name != stringValue(value["ref"]) && name != "actor_user" && name != "actor_self" {
		result["display_name"] = name
	}
	return result
}

func factOnlyContextFacts(projection ContextProjection, cognition bool) factOnlyContextView {
	profile := stringValue(projection.PersonalityRuntime["active_profile_id"])
	goals := filterActiveProfileRows(projection.Goals, profile)
	if target := stringValue(projection.CurrentSpeaker["actor_id"]); target != "" {
		goals, _ = filterAgencyForTarget(goals, nil, target)
	}
	facts := map[string]any{
		"self_actor":      factOnlyActor(projection.SelfActor),
		"current_speaker": factOnlyActor(projection.CurrentSpeaker),
		"time_view":       projectionTimeView(projection),
		"current_state":   factOnlyCurrentState(projection, cognition),
		"goals":           factOnlyGoals(goals, projection, cognition),
	}
	if background := compactActorBackground(projection); len(background) > 0 {
		if cognition {
			facts["actor_background"] = background
		} else {
			facts["actor_background"] = replySemanticValue(background)
		}
	}
	if len(projection.CommunicationState) > 0 {
		facts["communication_state"] = factOnlyFields(projection.CommunicationState, "channel", "remote", "meeting_confirmed", "relationship_distance")
		if projection.CommunicationState["channel_source"] == "validated_private_conversation" {
			facts["communication_state"] = privateChatCommunicationView(projection)
		}
	}
	var relationships []map[string]any
	for _, row := range selectActiveProfileRelationships(projection.Relationships, profile) {
		if target := stringValue(row["target_actor_id"]); target != "" && target != stringValue(projection.CurrentSpeaker["actor_id"]) {
			continue
		}
		item := factOnlyFields(row, "summary", "trend", "emotional_association")
		item["role"] = factOnlyFields(mapValue(row["role"]), "label", "category", "secondary", "preferred_addressing", "self_reference")
		item["target_actor"] = factOnlyActor(actorRefForID(projection.Actors, stringValue(row["target_actor_id"])))
		item["source_kind"] = "relationship_record"
		item["certainty"] = "记录状态；历史摘要和后台待核验均不自动改写现实关系。"
		if cognition {
			factOnlyAddRef(item, row, projection.ReferenceIndex, ContextReferenceRelationship)
		}
		relationships = append(relationships, item)
	}
	if len(relationships) > 0 {
		facts["relationships"] = relationships
	}
	// The current accepted item is a constraint/plan, not proof an activity ran.
	if schedule := factOnlySchedule(projection, cognition); len(schedule) > 0 {
		facts["schedule"] = schedule
	}
	if cognition {
		if len(projection.GoalPolicy) > 0 {
			facts["goal_policy"] = factOnlyFields(projection.GoalPolicy, "active_count", "max_active_goals", "auto_planning_enabled")
		}
		if visual := compactVisualIdentityForSurface(projection.VisualIdentity); len(visual) > 0 {
			facts["visual_identity"] = visual
		}
		if outcomes := compactRecentActionOutcomesForSurface(projection.RecentOutcomes, ProviderContextSurfaceConversationMain); len(outcomes) > 0 {
			facts["recent_outcomes"] = outcomes
		}
	}
	visibleActive := 0
	for _, goal := range goals {
		if firstString(goal["status"], stringValue(goal["state"])) == "active" {
			visibleActive++
		}
	}
	return factOnlyContextView{Facts: facts, Selection: map[string]any{
		"goal_source_count": len(projection.Goals), "goal_visible_count": len(goals), "goal_visible_active_count": visibleActive,
		"global_active_count": projection.GoalPolicy["active_count"], "goal_filter": "existing_profile_and_recipient_scope",
		"as_of": projection.AsOf, "source_fact_id": projection.SourceFactID, "facts_revision": projection.CurrentFactsRevision,
		"life_revision": projection.LifeContextRevision, "working_profile_id": profile,
	}}
}

func factOnlyAddRef(item, source map[string]any, index ContextReferenceIndex, kind ContextReferenceKind) {
	if ref := providerSafeContextRef(source["ref"], index, kind); ref != "" {
		item["ref"] = ref
	}
}

func factOnlyGoals(goals []map[string]any, projection ContextProjection, cognition bool) []map[string]any {
	result := make([]map[string]any, 0, len(goals))
	for _, row := range goals {
		status := firstString(row["status"], stringValue(row["state"]))
		if status == "abandoned" || status == "cancelled" {
			continue
		}
		item := factOnlyFields(row, "description", "desired_outcome", "success_criteria", "motivation", "deadline_policy", "deadline", "scope", "progress")
		item["state"] = status
		if status == "completed" {
			item = factOnlyFields(item, "description", "desired_outcome", "state")
			item["continuity"] = "completed_record_not_active_goal"
		} else {
			execution := mapValue(row["execution"])
			conditions := factOnlyFields(mapValue(execution["dependency_snapshot"]), "context_review_required", "prerequisite_goals_completed")
			if review, ok := row["context_review_required"]; ok {
				conditions["context_review_required"] = review
			}
			for _, key := range []string{"last_attempt", "last_result"} {
				if raw := mapValue(execution[key]); len(raw) > 0 {
					conditions[key] = factOnlyFields(raw, "status", "capability", "error_code", "started_at", "settled_at", "occurred_at", "deadline")
				}
			}
			if len(conditions) > 0 {
				conditions["source_kind"] = "goal_execution_records"
				conditions["meaning"] = "后台准入/执行条件；不规定社交表达，待核验不等于现实否定。"
				item["operation_conditions"] = conditions
			}
		}
		if target := stringValue(row["target_actor_id"]); target != "" {
			item["target_actor"] = factOnlyActor(actorRefForID(projection.Actors, target))
		}
		if cognition {
			factOnlyAddRef(item, row, projection.ReferenceIndex, ContextReferenceGoal)
		}
		result = append(result, item)
	}
	return result
}

func factOnlyCurrentState(projection ContextProjection, cognition bool) map[string]any {
	base := compactCurrentState(projection)
	data := mapValue(base["data"])
	result := map[string]any{}
	life := mapValue(data["life_context"])
	if len(life) > 0 {
		item := compactLifeContextForSurface(life, projection.ReferenceIndex, ProviderContextSurfaceFactOnlyCognition)
		if !cognition {
			item = mapValue(replySemanticValue(item))
		}
		if factOnlyExpired(life, "expires_at", projection.AsOf) {
			// Expiry removes only this source's transient assertions.
			item = factOnlyFields(item, "timezone", "expires_at")
			item["status"] = "unknown"
		}
		presence := mapValue(life["presence"])
		if len(presence) == 0 {
			presence = projection.Presence
		}
		if len(presence) > 0 {
			present := factOnlyFields(presence, "current_task", "user_presence", "effective_at", "expires_at")
			if factOnlyExpired(presence, "expires_at", projection.AsOf) {
				present = factOnlyFields(present, "expires_at")
				present["status"] = "unknown"
			}
			item["presence"] = present
		}
		result["life_context"] = item
	}
	appearance := mapValue(data["appearance"])
	if len(appearance) == 0 {
		appearance = projection.EffectiveAppearance
	}
	if len(appearance) > 0 {
		item := compactEffectiveAppearanceForSurface(appearance)
		fields := map[string]any{}
		for key, raw := range mapValue(appearance["body_fields"]) {
			field := mapValue(raw)
			if len(field) > 0 {
				fields[key] = factOnlyFields(field, "status", "value")
			} else {
				fields[key] = raw
			}
		}
		if len(fields) > 0 {
			item["body_fields"] = fields
		}
		if !cognition {
			item = mapValue(replySemanticValue(item))
		}
		result["appearance"] = item
	}
	activities := arrayValue(data["active_activities"])
	if len(activities) == 0 {
		activities = arrayValue(projection.ActiveActivities)
	}
	var active []map[string]any
	for _, raw := range activities {
		row := mapValue(raw)
		if factOnlyExpired(row, "active_until", projection.AsOf) {
			continue
		}
		item := factOnlyFields(row, "id", "kind", "status", "started_at", "not_before", "active_until", "ready_for_resolution")
		if request := factOnlyFields(mapValue(row["request"]), "category", "slot", "description", "desired_hair_length", "desired_hair_color"); len(request) > 0 {
			item["request"] = request
		}
		active = append(active, item)
	}
	if len(active) > 0 {
		result["active_activities"] = active
	}
	if cognition {
		if inner := compactInnerStateForSurface(mapValue(data["inner_state"])); len(inner) > 0 {
			result["inner_state"] = inner
		}
	} else if mood := factOnlyFields(mapValue(mapValue(data["inner_state"])["mood"]), "label"); len(mood) > 0 {
		result["mood"] = mood
	}
	return map[string]any{"subject": "actor_self", "data": result}
}

func factOnlyExpired(value map[string]any, key, asOf string) bool {
	end, err := time.Parse(time.RFC3339Nano, stringValue(value[key]))
	if err != nil {
		return false
	}
	now, err := time.Parse(time.RFC3339Nano, asOf)
	return err == nil && !now.Before(end)
}

func factOnlyNotYetEffective(value map[string]any, asOf string) bool {
	start, err := time.Parse(time.RFC3339Nano, stringValue(value["valid_from"]))
	if err != nil {
		return false
	}
	now, err := time.Parse(time.RFC3339Nano, asOf)
	return err == nil && now.Before(start)
}

func factOnlySchedule(projection ContextProjection, cognition bool) map[string]any {
	result := factOnlyFields(projection.Schedule, "local_date", "timezone")
	now, err := time.Parse(time.RFC3339Nano, projection.AsOf)
	if err != nil {
		return result
	}
	rows := arrayValue(projection.Schedule["items"])
	if len(rows) == 0 {
		if current := mapValue(projection.Schedule["current_item"]); len(current) > 0 {
			rows = append(rows, current)
		}
		rows = append(rows, arrayValue(projection.Schedule["upcoming_items"])...)
	}
	var current, next map[string]any
	var nextStart time.Time
	for _, raw := range rows {
		row := mapValue(raw)
		switch stringValue(row["status"]) {
		case "cancelled", "invalidated", "skipped":
			continue
		}
		start, startErr := time.Parse(time.RFC3339Nano, stringValue(row["start_at"]))
		end, endErr := time.Parse(time.RFC3339Nano, stringValue(row["end_at"]))
		if startErr != nil || endErr != nil {
			continue
		}
		if !now.Before(start) && now.Before(end) {
			current = row
		}
		if !start.Before(now) && (next == nil || start.Before(nextStart)) {
			next, nextStart = row, start
		}
	}
	for key, row := range map[string]map[string]any{"current_item": current, "next_confirmed_item": next} {
		if row == nil {
			continue
		}
		item := mapValue(compactScheduleValueForSurface(row, projection.ReferenceIndex, ProviderContextSurfaceFactOnlyCognition))
		if !cognition {
			item = mapValue(replySemanticValue(item))
		}
		result[key] = item
	}
	if current != nil || next != nil {
		result["certainty"] = "accepted_schedule_not_execution_receipt"
	}
	return result
}

func factOnlyRuntimeFragments(view factOnlyContextView, projection ContextProjection, cognition bool) []PromptFragment {
	keys := make([]string, 0, len(view.Facts))
	for key := range view.Facts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	fragments := make([]PromptFragment, 0, len(keys))
	for _, key := range keys {
		value := view.Facts[key]
		if cognition {
			value = providerEncodeContextRefs(value, projection.ReferenceIndex)
		}
		fragments = append(fragments, PromptFragment{Kind: PromptFragmentRuntimeFact, Required: true, Priority: 100,
			Content: map[string]any{"kind": key, "value": value}, SourceRefs: []string{"authority:" + key}})
	}
	return fragments
}
