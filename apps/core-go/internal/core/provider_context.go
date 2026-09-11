package core

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"
)

var providerHashPattern = regexp.MustCompile(`\b(?:message|memory|inbox|wake_fact|fluctlight|conversation|claim|event|fact|turn|provider|assessment|decision)_[A-Za-z0-9]{16,64}\b`)

// compactCognitionContext is the Provider-facing projection of a full
// ContextProjection. The full projection remains the durable/replayable
// internal value; this DTO keeps the semantic three-layer context, bounded
// visual identity facts, and non-empty evidence collections needed by the
// current operation.
//
// In particular, capability definitions are intentionally absent here. Calls
// that can execute capabilities already send the authoritative native `tools`
// catalog separately in the Provider request. Sending the same schemas inside
// user content needlessly doubles prompt size and gives the model two copies
// of the contract to reconcile.
func compactCognitionContext(projection ContextProjection) map[string]any {
	activeProfileID := stringValue(mapValue(projection.PersonalityRuntime)["active_profile_id"])
	profileRelationships := selectActiveProfileRelationships(projection.Relationships, activeProfileID)
	profileGoals := filterActiveProfileRows(projection.Goals, activeProfileID)
	profileIntentions := filterActiveProfileRows(projection.Intentions, activeProfileID)
	result := map[string]any{
		"core_persona":  compactCorePersona(projection),
		"current_state": compactCurrentState(projection),
	}
	if self := compactActorRef(projection.SelfActor); len(self) > 0 {
		result["self_actor"] = self
	}
	if system := compactPersonalitySystem(projection.PersonalitySystem, projection.PersonalityRuntime); len(system) > 0 {
		result["personality_system"] = system
	}
	if effective := compactEffectivePersonaForProvider(projection.EffectivePersona); len(effective) > 0 {
		result["effective_persona"] = effective
	}
	if speaker := compactActorRef(projection.CurrentSpeaker); len(speaker) > 0 {
		result["current_speaker"] = speaker
	}
	if actors := compactActorRefs(projection.Actors); len(actors) > 0 {
		result["actors"] = actors
	}
	if len(projection.RecentMessages) > 0 {
		if recent := compactRecentMessagesForActors(projection.RecentMessages, projection.CurrentUserText, projection.Actors); len(recent) > 0 {
			result["recent_messages"] = recent
		}
	}
	if len(projection.DevelopingSelf) > 0 {
		result["developing_self"] = compactDevelopingSelf(projection.DevelopingSelf)
	}
	if len(projection.Memories) > 0 {
		result["memories"] = compactMemoriesForProfile(projection.Memories, stringValue(mapValue(projection.PersonalityRuntime)["active_profile_id"]))
	}
	if len(profileRelationships) > 0 {
		result["relationships"] = compactRelationships(profileRelationships, projection.Actors)
	}
	if len(projection.Hypotheses) > 0 {
		result["hypotheses"] = projection.Hypotheses
	}
	if len(projection.DriveSlots) > 0 {
		if drives := compactDriveSlotsForProvider(projection.DriveSlots); len(drives) > 0 {
			result["drive_slots"] = drives
		}
	}
	if len(projection.PreferenceSlots) > 0 {
		result["preference_slots"] = projection.PreferenceSlots
	}
	if len(projection.TriggerPreferences) > 0 {
		result["trigger_preferences"] = projection.TriggerPreferences
	}
	if len(projection.VisualIdentity) > 0 {
		if visualIdentity := compactVisualIdentity(projection.VisualIdentity); len(visualIdentity) > 0 {
			result["visual_identity"] = visualIdentity
		}
	}
	if schedule := compactScheduleForProvider(projection.Schedule); len(schedule) > 0 {
		result["schedule"] = schedule
	}
	if len(projection.Presence) > 0 {
		if presence := compactPresenceForProvider(projection.Presence); len(presence) > 0 {
			result["presence"] = presence
		}
	}
	if goals := compactProviderGoalsForActors(profileGoals, projection.Actors); len(goals) > 0 {
		result["goals"] = goals
	}
	if intentions := compactProviderIntentions(profileIntentions); len(intentions) > 0 {
		result["intentions"] = intentions
	}
	// Full Outcome authority remains Core-only. This is a physically separate,
	// field-level allowlist projection: no text, raw expected/observed payload,
	// entity ID, reference index, evidence body, provenance, or runtime identity
	// can cross the Provider boundary.
	if outcomes := compactRecentActionOutcomes(projection.RecentOutcomes); len(outcomes) > 0 {
		result["recent_outcomes"] = outcomes
	}
	cleaned := stripProviderContextMetadata(result).(map[string]any)
	// Current State is already explicitly allowlisted by its compactors. Restore
	// that safe shape after the generic metadata filter so authoritative life
	// source, opaque refs and expected revisions are not mistaken for storage
	// metadata and removed from the Provider decision context.
	cleaned["current_state"] = compactCurrentState(projection)
	if outcomes := compactRecentActionOutcomes(projection.RecentOutcomes); len(outcomes) > 0 {
		cleaned["recent_outcomes"] = outcomes
	}
	// `status` is storage metadata for most projections, but it is semantic
	// input for schedule.replan: the model must be able to carry the current
	// item's planned/completed state into a replacement schedule. Restore only
	// the already-sanitized schedule DTO; it contains no persistence IDs.
	if schedule := compactScheduleForProvider(projection.Schedule); len(schedule) > 0 {
		cleaned["schedule"] = schedule
	}
	return cleaned
}

func compactEffectivePersonaForProvider(value map[string]any) map[string]any {
	result := map[string]any{}
	for _, key := range []string{"profile_ref", "authority_revision", "personality", "behavioral_policy"} {
		if child, ok := value[key]; ok && child != nil {
			result[key] = child
		}
	}
	return result
}

func compactPersonalitySystem(system, runtime map[string]any) map[string]any {
	if len(system) == 0 && len(runtime) == 0 {
		return nil
	}
	result := cloneMap(system)
	if result == nil {
		result = map[string]any{}
	}
	delete(result, "extensions")
	if active := stringValue(mapValue(runtime)["active_profile_id"]); active != "" {
		result["active_profile_id"] = active
		for _, raw := range arrayValue(system["profiles"]) {
			profile := mapValue(raw)
			if stringValue(profile["id"]) == active {
				result["active_profile"] = profile
				break
			}
		}
	}
	if previous := stringValue(mapValue(runtime)["previous_profile_id"]); previous != "" {
		result["previous_profile_id"] = previous
	}
	if revision := intValue(mapValue(runtime)["revision"]); revision > 0 {
		result["runtime_revision"] = revision
	}
	return result
}

func compactActorRelationshipContext(projection ContextProjection) map[string]any {
	result := map[string]any{}
	if self := compactActorRef(projection.SelfActor); len(self) > 0 {
		result["self_actor"] = self
	}
	if speaker := compactActorRef(projection.CurrentSpeaker); len(speaker) > 0 {
		result["target_actor"] = speaker
		targetID := stringValue(projection.CurrentSpeaker["actor_id"])
		activeProfileID := stringValue(mapValue(projection.PersonalityRuntime)["active_profile_id"])
		var fallbackRelationship map[string]any
		for _, relationship := range projection.Relationships {
			if stringValue(relationship["target_actor_id"]) == targetID {
				if stringValue(relationship["profile_id"]) == activeProfileID && activeProfileID != "" {
					result["relationship"] = compactRelationship(relationship)
					break
				}
				if stringValue(relationship["profile_id"]) == "" {
					fallbackRelationship = relationship
				}
			}
		}
		if _, ok := result["relationship"]; !ok && len(fallbackRelationship) > 0 {
			result["relationship"] = compactRelationship(fallbackRelationship)
		}
		if _, ok := result["relationship"]; !ok {
			result["relationship"] = map[string]any{"role": map[string]any{"label": "unknown"}, "trend": "stable", "revision": 0, "provenance": map[string]any{"source": "unestablished"}}
		}
		profileGoals := filterActiveProfileRows(projection.Goals, activeProfileID)
		profileIntentions := filterActiveProfileRows(projection.Intentions, activeProfileID)
		if goals := compactProviderGoalsForActors(relationshipGoalsForTarget(profileGoals, targetID), projection.Actors); len(goals) > 0 {
			result["goals"] = goals
		}
		if intentions := compactProviderIntentions(relationshipIntentionsForTarget(profileIntentions, targetID)); len(intentions) > 0 {
			result["intentions"] = intentions
		}
	}
	return result
}

func compactRelationship(value map[string]any) map[string]any {
	result := map[string]any{}
	for _, key := range []string{"ref", "role", "metrics", "trend", "summary", "emotional_association"} {
		if raw, ok := value[key]; ok && raw != nil {
			result[key] = raw
		}
	}
	if revision, ok := value["revision"]; ok && revision != nil {
		result["expected_revision"] = revision
	}
	return result
}

func compactRelationships(values []map[string]any, actors []map[string]any) []map[string]any {
	result := make([]map[string]any, 0, len(values))
	for _, relationship := range values {
		item := compactRelationship(relationship)
		if actor := actorRefForID(actors, stringValue(relationship["target_actor_id"])); len(actor) > 0 {
			item["target_actor"] = compactActorRef(actor)
		}
		if len(item) > 0 {
			result = append(result, item)
		}
	}
	return result
}

func relationshipGoalsForTarget(goals []map[string]any, targetID string) []map[string]any {
	result := make([]map[string]any, 0)
	for _, goal := range goals {
		if stringValue(goal["scope"]) == "relationship" && stringValue(goal["target_actor_id"]) == targetID {
			result = append(result, goal)
		}
	}
	return result
}

func relationshipIntentionsForTarget(intentions []map[string]any, targetID string) []map[string]any {
	result := make([]map[string]any, 0)
	for _, intention := range intentions {
		if stringValue(intention["target_actor_id"]) == targetID {
			result = append(result, intention)
		}
	}
	return result
}

func withActorRelationshipSystemContext(messages []map[string]any, projection ContextProjection) []map[string]any {
	contextValue := compactActorRelationshipContext(projection)
	if len(contextValue) == 0 {
		return messages
	}
	system := map[string]any{"role": "system", "content": jsonString(map[string]any{"actor_relationship_context": contextValue})}
	return append([]map[string]any{system}, messages...)
}

func compactProviderGoals(goals []map[string]any) []map[string]any {
	return compactProviderGoalsForActors(goals, nil)
}

func compactProviderGoalsForActors(goals []map[string]any, actors []map[string]any) []map[string]any {
	result := make([]map[string]any, 0, len(goals))
	for _, goal := range goals {
		item := map[string]any{}
		for _, key := range []string{"ref", "description", "desired_outcome", "success_criteria", "motivation", "needs_reflection", "importance", "urgency", "progress", "scope", "target_actor_id", "deadline"} {
			if value, ok := goal[key]; ok && value != nil {
				item[key] = value
			}
		}
		if status := stringValue(goal["status"]); status != "" {
			item["state"] = status
		}
		if revision := intValue(goal["revision"]); revision >= 0 {
			item["expected_revision"] = revision
		}
		if target := stringValue(goal["target_actor_id"]); target != "" {
			delete(item, "target_actor_id")
			if actor := actorRefForID(actors, target); len(actor) > 0 {
				item["target_actor"] = compactActorRef(actor)
			} else {
				item["target_actor"] = map[string]any{"ref": "actor_unknown", "type": "unknown"}
			}
		}
		if len(item) > 0 {
			result = append(result, item)
		}
	}
	return result
}

func compactProviderIntentions(intentions []map[string]any) []map[string]any {
	result := make([]map[string]any, 0, len(intentions))
	for _, intention := range intentions {
		item := map[string]any{}
		for _, key := range []string{"ref", "goal_ref", "goal", "action", "action_intent", "expected_outcome", "capability_constraints", "confidence", "preferred_time", "expiration", "trigger"} {
			if value, ok := intention[key]; ok && value != nil && value != "" {
				if key == "expiration" {
					item["deadline"] = value
				} else {
					item[key] = value
				}
			}
		}
		if status := stringValue(intention["status"]); status != "" {
			item["state"] = status
		}
		if revision := intValue(intention["revision"]); revision >= 0 {
			item["expected_revision"] = revision
		}
		if len(item) > 0 {
			result = append(result, item)
		}
	}
	return result
}

func compactCorePersona(projection ContextProjection) map[string]any {
	result := cloneMap(projection.CorePersona)
	if result == nil {
		result = make(map[string]any, 2)
	}
	if stringValue(result["authority"]) == "" {
		result["authority"] = "hard_constraint"
	}
	data := mapValue(result["data"])
	if len(data) == 0 {
		data = make(map[string]any, 4)
	}
	// Older rows may have populated the parallel projection fields without
	// persisting a complete nested Core Persona envelope. Fill only missing
	// children once, rather than emitting those fields as duplicates.
	if _, ok := data["identity"]; !ok && len(projection.Identity) > 0 {
		data["identity"] = projection.Identity
	}
	if _, ok := data["personality"]; !ok && len(projection.Personality) > 0 {
		data["personality"] = projection.Personality
	}
	if _, ok := data["behavioral_policy"]; !ok && len(projection.BehavioralPolicy) > 0 {
		data["behavioral_policy"] = projection.BehavioralPolicy
	}
	if identity := mapValue(data["identity"]); len(identity) > 0 {
		delete(identity, "id")
	}
	delete(result, "id")
	delete(result, "schema_version")
	delete(data, "schema_version")
	result["data"] = data
	return result
}

// compactCurrentState removes the parallel top-level inner_state and
// life_context aliases while preserving the existing authority/data envelope.
// State semantics are not rewritten here; this function only changes the
// Provider input shape.
func compactCurrentState(projection ContextProjection) map[string]any {
	result := cloneMap(projection.CurrentState)
	if result == nil {
		result = make(map[string]any, 2)
	}
	if stringValue(result["authority"]) == "" {
		result["authority"] = "transient_state"
	}
	data := mapValue(result["data"])
	if len(data) == 0 {
		data = make(map[string]any, 2)
	}
	if _, ok := data["inner_state"]; !ok && len(projection.InnerState) > 0 {
		data["inner_state"] = projection.InnerState
	}
	if _, ok := data["life_context"]; !ok && len(projection.LifeContext) > 0 {
		data["life_context"] = projection.LifeContext
	}
	if inner := mapValue(data["inner_state"]); len(inner) > 0 {
		data["inner_state"] = compactInnerState(inner)
	}
	if profile := mapValue(data["affect_profile"]); len(profile) > 0 {
		data["affect_profile"] = compactAffectProfile(profile)
	}
	if lifeContext := mapValue(data["life_context"]); len(lifeContext) > 0 {
		data["life_context"] = compactLifeContext(lifeContext)
	}
	result["data"] = data
	return result
}

func compactRecentMessages(messages []map[string]any, currentUserText string) []map[string]any {
	return compactRecentMessagesForActors(messages, currentUserText, nil)
}

func compactActorRef(value map[string]any) map[string]any {
	if len(value) == 0 {
		return nil
	}
	result := map[string]any{}
	for _, key := range []string{"ref", "type", "role", "label", "display_name"} {
		if raw, ok := value[key]; ok && raw != nil && raw != "" {
			result[key] = raw
		}
	}
	return result
}

func compactActorRefs(values []map[string]any) []map[string]any {
	result := make([]map[string]any, 0, len(values))
	for _, value := range values {
		if compact := compactActorRef(value); len(compact) > 0 {
			result = append(result, compact)
		}
	}
	return result
}

func compactRecentMessagesForActors(messages []map[string]any, currentUserText string, actors []map[string]any) []map[string]any {
	currentUserText = strings.TrimSpace(currentUserText)
	skipIndex := -1
	if currentUserText != "" {
		// History is ordered oldest-to-newest. Skip only the newest matching
		// user message, so an earlier identical message remains evidence.
		for index := len(messages) - 1; index >= 0; index-- {
			if stringValue(messages[index]["kind"]) == "user" && strings.TrimSpace(stringValue(messages[index]["text"])) == currentUserText {
				skipIndex = index
				break
			}
		}
	}
	result := make([]map[string]any, 0, len(messages))
	for index, message := range messages {
		if index == skipIndex {
			continue
		}
		kind := stringValue(message["kind"])
		text := strings.TrimSpace(stringValue(message["text"]))
		if kind == "" && text == "" {
			continue
		}
		if kind == "" {
			kind = "message"
		}
		stamp := compactMessageTime(stringValue(message["created_at"]))
		item := map[string]any{"role": kind, "time": stamp, "content": text}
		if sender := actorRefForID(actors, stringValue(message["author_actor_id"])); len(sender) > 0 {
			if stringValue(sender["type"]) == "human" {
				item["sender"] = "actor_user"
			} else {
				item["sender"] = firstString(sender["display_name"], stringValue(sender["ref"]))
			}
			item["actor_type"] = sender["type"]
		} else if kind == "user" {
			item["sender"] = "actor_user"
		} else if kind == "assistant" {
			item["sender"] = "actor_self"
		}
		result = append(result, item)
	}
	return result
}

func actorRefForID(actors []map[string]any, actorID string) map[string]any {
	actorID = strings.TrimSpace(actorID)
	if actorID == "" {
		return nil
	}
	for _, actor := range actors {
		if stringValue(actor["actor_id"]) == actorID {
			return actor
		}
	}
	return nil
}

func compactMessageTime(value string) string {
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return parsed.UTC().Format("01-02 15:04:05")
	}
	if len(value) >= 8 && value[2] == '-' && value[5] == ' ' {
		return value[:8]
	}
	if len(value) >= 8 && value[2] == ':' {
		return value[:8]
	}
	return "--:--"
}

func compactDevelopingSelf(claims []map[string]any) []map[string]any {
	result := make([]map[string]any, 0, len(claims))
	for _, claim := range claims {
		compact := make(map[string]any, 6)
		for _, key := range []string{"ref", "category", "claim", "value", "confidence", "evidence_refs"} {
			if value, ok := claim[key]; ok && value != nil && value != "" {
				if key == "evidence_refs" && len(arrayValue(value)) == 0 {
					continue
				}
				compact[key] = value
			}
		}
		if source := stringValue(mapValue(claim["provenance"])["source"]); source != "" {
			compact["provenance_source"] = source
		}
		if len(compact) > 0 {
			result = append(result, compact)
		}
	}
	return result
}

func compactMemories(memories []map[string]any) []map[string]any {
	return compactMemoriesForProfile(memories, "")
}

func compactMemoriesForProfile(memories []map[string]any, activeProfileID string) []map[string]any {
	result := make([]map[string]any, 0, len(memories))
	for _, memory := range memories {
		compact := make(map[string]any, 7)
		for _, key := range []string{"ref", "type", "content", "confidence", "importance", "emotional_significance", "created_at"} {
			if value, ok := memory[key]; ok && value != nil && value != "" {
				compact[key] = value
			}
		}
		perspectives := arrayValue(memory["personality_perspectives"])
		if len(perspectives) == 0 {
			perspectives = arrayValue(memory["perspectives"])
		}
		if perspective := memoryPerspectiveForProfile(perspectives, activeProfileID); len(perspective) > 0 {
			compact["current_profile_perspective"] = perspective
		}
		if len(compact) > 0 {
			result = append(result, compact)
		}
	}
	return result
}

func memoryPerspectiveForProfile(values []any, activeProfileID string) map[string]any {
	if strings.TrimSpace(activeProfileID) == "" {
		return nil
	}
	for _, raw := range values {
		perspective := mapValue(raw)
		if stringValue(perspective["profile_id"]) == activeProfileID {
			result := make(map[string]any, 2)
			for _, key := range []string{"interpretation", "emotion"} {
				if value, ok := perspective[key]; ok && value != nil && value != "" {
					result[key] = value
				}
			}
			return result
		}
	}
	return nil
}

func compactInnerState(inner map[string]any) map[string]any {
	result := make(map[string]any, 6)
	if ref := stringValue(inner["ref"]); ref != "" {
		result["ref"] = ref
	}
	if revision := intValue(inner["revision"]); revision >= 0 {
		result["expected_revision"] = revision
	}
	if pad := compactStateMap(inner["pad"], []string{"arousal", "pleasure", "dominance"}); len(pad) > 0 {
		result["pad"] = pad
	}
	if mood := compactStateMap(inner["mood"], []string{"label", "source", "intensity"}); len(mood) > 0 {
		result["mood"] = mood
	}
	if momentum := compactStateMap(inner["momentum"], []string{"value", "trend", "arousal_momentum", "dominance_momentum", "pleasure_momentum"}); len(momentum) > 0 {
		result["momentum"] = momentum
	}
	if drives := compactCurrentDrives(inner["drives"]); len(drives) > 0 {
		result["drives"] = drives
	}
	if conflicts := compactDriveConflicts(inner["conflicts"]); len(conflicts) > 0 {
		result["conflicts"] = conflicts
	}
	if regulation := mapValue(inner["regulation"]); len(regulation) > 0 {
		compactRegulation := make(map[string]any, 2)
		for _, key := range []string{"stress", "stability"} {
			if value, ok := regulation[key]; ok && value != nil {
				compactRegulation[key] = value
			}
		}
		if len(compactRegulation) > 0 {
			result["regulation"] = compactRegulation
		}
	}
	return result
}

func compactDriveSlotsForProvider(rows []map[string]any) []map[string]any {
	result := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		item := map[string]any{}
		for _, key := range []string{"ref", "key", "label", "description", "value_schema", "confidence"} {
			if value, present := row[key]; present && value != nil && value != "" {
				item[key] = value
			}
		}
		if value := mapValue(row["value"]); stringValue(row["value_schema"]) == "pressure" && len(value) > 0 {
			item["value"] = compactStateMap(value, []string{"pressure", "salience", "direction"})
		}
		if len(item) > 0 {
			result = append(result, item)
		}
	}
	return result
}

func compactCurrentDrives(value any) []map[string]any {
	result := make([]map[string]any, 0)
	for _, raw := range arrayValue(value) {
		drive := mapValue(raw)
		item := map[string]any{}
		for _, key := range []string{"ref", "key", "label", "description", "pressure", "salience", "direction", "confidence"} {
			if field, present := drive[key]; present && field != nil && field != "" {
				item[key] = field
			}
		}
		if source := stringValue(drive["source"]); source == "built_in" || source == "typed_slot" {
			item["authority"] = source
		}
		if stringValue(item["ref"]) != "" && stringValue(item["key"]) != "" {
			result = append(result, item)
		}
	}
	return result
}

func compactDriveConflicts(value any) []map[string]any {
	result := make([]map[string]any, 0)
	for _, raw := range arrayValue(value) {
		conflict := mapValue(raw)
		item := compactStateMap(conflict, []string{"key", "pressure", "source"})
		if stringValue(item["key"]) != "" {
			result = append(result, item)
		}
	}
	return result
}

func compactStateMap(value any, keys []string) map[string]any {
	source := mapValue(value)
	result := make(map[string]any, len(keys))
	for _, key := range keys {
		if item, ok := source[key]; ok && item != nil && item != "" {
			result[key] = item
		}
	}
	return result
}

func compactLifeContext(context map[string]any) map[string]any {
	result := make(map[string]any, 7)
	for _, key := range []string{"ref", "source", "authority_status", "context_revision", "event_ref", "schedule_ref", "schedule_item_ref", "presence_ref", "scene", "activity", "location", "current_time", "timezone", "effective_at", "expires_at"} {
		if value, ok := context[key]; ok && value != nil && value != "" {
			result[key] = value
		}
	}
	if presence := compactPresenceForProvider(mapValue(context["presence"])); len(presence) > 0 {
		result["presence"] = presence
	}
	if schedule := compactScheduleForProvider(mapValue(context["schedule"])); len(schedule) > 0 {
		result["schedule"] = schedule
	}
	return result
}

func compactPresenceForProvider(presence map[string]any) map[string]any {
	result := make(map[string]any, 6)
	for _, key := range []string{"ref", "current_task", "user_presence", "effective_at", "expires_at"} {
		if value, ok := presence[key]; ok && value != nil && value != "" {
			result[key] = value
		}
	}
	if revision := intValue(presence["revision"]); revision > 0 {
		result["expected_revision"] = revision
	}
	return result
}

// compactScheduleForProvider keeps the semantic plan needed for an LLM-owned
// replan while removing storage IDs and coordination metadata. The revision is
// intentionally exposed as expected_revision because it is the CAS input of
// schedule.replan, not a provider-facing storage detail.
func compactScheduleForProvider(value map[string]any) map[string]any {
	if len(value) == 0 {
		return nil
	}
	result := map[string]any{}
	for _, key := range []string{"ref", "local_date", "timezone", "completed_before", "reschedule_policy"} {
		if raw, ok := value[key]; ok && raw != nil && raw != "" {
			result[key] = raw
		}
	}
	revision := intValue(value["revision"])
	if revision == 0 {
		revision = intValue(value["expected_revision"])
	}
	if revision >= 0 {
		result["expected_revision"] = revision
	}
	items := make([]map[string]any, 0)
	for _, raw := range arrayValue(value["items"]) {
		item := mapValue(raw)
		if len(item) == 0 {
			continue
		}
		compact := map[string]any{}
		for _, key := range []string{"ref", "start_at", "end_at", "activity", "scene", "location", "item_type", "status", "priority", "flexibility", "interruption_cost"} {
			if child, ok := item[key]; ok && child != nil && child != "" {
				compact[key] = child
			}
		}
		if len(compact) > 0 {
			items = append(items, compact)
		}
	}
	if len(items) > 0 {
		result["items"] = items
	}
	// Make the temporal decision boundary explicit. The full item list remains
	// available for schedule.replan, while these projections tell the model
	// which item is in progress and which future items are candidates for
	// invalidation after a sudden scene/event change. This is semantic context,
	// not a Go-side decision to replan.
	boundary := time.Now().UTC()
	if raw := stringValue(result["completed_before"]); raw != "" {
		if parsed, err := time.Parse(time.RFC3339Nano, raw); err == nil {
			boundary = parsed
		}
	}
	upcoming := make([]map[string]any, 0)
	for _, item := range items {
		start, startErr := time.Parse(time.RFC3339Nano, stringValue(item["start_at"]))
		end, endErr := time.Parse(time.RFC3339Nano, stringValue(item["end_at"]))
		if startErr != nil || endErr != nil {
			continue
		}
		if !boundary.Before(start) && boundary.Before(end) {
			result["current_item"] = item
			continue
		}
		if !start.Before(boundary) {
			upcoming = append(upcoming, item)
		}
	}
	if len(upcoming) > 0 {
		result["upcoming_items"] = upcoming
	}
	return result
}

// compactVisualIdentity keeps only the durable visual facts that a cognition
// model can use. Workflow timeline stages, asset references, revision/status
// bookkeeping, and renderer adapter metadata belong to Core/media workers and
// must not be copied into every wake-up or chat prompt.
func compactVisualIdentity(value map[string]any) map[string]any {
	if len(value) == 0 {
		return nil
	}
	result := make(map[string]any, 2)
	status := stringValue(value["status"])
	if missing, ok := value["missing"].(bool); ok {
		result["missing"] = missing
	}
	switch status {
	case "active":
		result["available"] = true
	case "missing":
		result["available"] = false
		result["missing"] = true
	case "renderer_config_pending":
		result["available"] = false
		result["renderer_config_pending"] = true
	case "", "queued", "running", "awaiting_review":
		result["available"] = false
	default:
		result["available"] = false
	}
	if constraints := compactRendererConstraints(mapValue(value["renderer_constraints"])); len(constraints) > 0 {
		result["renderer_constraints"] = constraints
	}
	return result
}

func compactVisualIdentityForMedia(value map[string]any) map[string]any {
	result := compactVisualIdentity(value)
	if result == nil {
		result = make(map[string]any, 2)
	}
	if reference := firstString(stringValue(value["character_sheet_asset_id"]), stringValue(value["canonical_asset_id"])); reference != "" {
		// This is retained in the durable media concept for Core's ComfyUI
		// reference-image lookup. Provider-facing media input strips it again.
		result["reference_asset_id"] = reference
	}
	return result
}

func compactVisualIdentityForMediaProvider(value map[string]any) map[string]any {
	result := compactVisualIdentity(value)
	if result == nil {
		result = make(map[string]any, 2)
	}
	if snapshot := mapValue(value["identity_snapshot"]); len(snapshot) > 0 {
		compactSnapshot := make(map[string]any, 2)
		if identity := stripProviderMetadata(mapValue(snapshot["identity"])); len(mapValue(identity)) > 0 {
			compactSnapshot["identity"] = identity
		}
		if lifeProfile := stripProviderMetadata(mapValue(snapshot["life_profile"])); len(mapValue(lifeProfile)) > 0 {
			compactSnapshot["life_profile"] = lifeProfile
		}
		if len(compactSnapshot) > 0 {
			result["identity_snapshot"] = compactSnapshot
		}
	}
	return result
}

func compactRendererConstraints(value map[string]any) map[string]any {
	result := make(map[string]any, 3)
	for _, key := range []string{"chest_cup", "chest_lora_weight", "chest_lora_applicable"} {
		if item, ok := value[key]; ok && item != nil && item != "" {
			result[key] = item
		}
	}
	return result
}

// compactResponsePlanForProvider is the small semantic hand-off consumed by
// action realization. The full plan remains frozen for replay, but IDs,
// schema/revision fields, capability calls, and internal candidate buckets do
// not help a model write the visible reply.
func compactResponsePlanForProvider(plan map[string]any) map[string]any {
	result := make(map[string]any, 8)
	for _, key := range []string{"answer_mode", "action_type", "response_intent", "tone", "profile_id"} {
		if value, ok := plan[key]; ok && value != nil && value != "" {
			result[key] = value
		}
	}
	if decision := stripProviderMetadata(mapValue(plan["personality_decision"])); len(mapValue(decision)) > 0 {
		result["personality_decision"] = decision
	}
	if decision := stripProviderMetadata(mapValue(plan["output_preference_decision"])); len(mapValue(decision)) > 0 {
		result["output_preference_decision"] = decision
	}
	for _, key := range []string{"approved_claims", "uncertain_claims"} {
		claims := compactResponseClaims(arrayValue(plan[key]))
		if len(claims) > 0 {
			result[key] = claims
		}
	}
	if outline := stripProviderMetadata(plan["response_outline"]); len(arrayValue(outline)) > 0 {
		result["response_outline"] = outline
	}
	if evaluation := compactResponseEvaluation(mapValue(plan["self_evaluation"])); len(evaluation) > 0 {
		result["self_evaluation"] = evaluation
	}
	for _, key := range []string{"core_alignment", "state_expression"} {
		if value := stripProviderMetadata(mapValue(plan[key])); len(mapValue(value)) > 0 {
			result[key] = value
		}
	}
	return result
}

func compactResponseClaims(claims []any) []map[string]any {
	result := make([]map[string]any, 0, len(claims))
	for _, raw := range claims {
		claim := mapValue(raw)
		item := make(map[string]any, 3)
		for _, key := range []string{"kind", "content", "confidence"} {
			if value, ok := claim[key]; ok && value != nil && value != "" {
				item[key] = value
			}
		}
		if len(item) > 0 {
			result = append(result, item)
		}
	}
	return result
}

func compactResponseEvaluation(value map[string]any) map[string]any {
	result := make(map[string]any, 2)
	for _, key := range []string{"mode", "confidence"} {
		if item, ok := value[key]; ok && item != nil && item != "" {
			result[key] = item
		}
	}
	return result
}

func compactCapabilityResultsForProvider(results []CapabilityResult) []map[string]any {
	result := make([]map[string]any, 0, len(results))
	for _, capability := range results {
		item := make(map[string]any, 4)
		for _, key := range []string{"name", "status", "error_code"} {
			value := map[string]any{"name": capability.CapabilityName, "status": capability.Status, "error_code": capability.ErrorCode}[key]
			if value != nil && value != "" {
				item[key] = value
			}
		}
		if capability.Output != nil {
			item["output"] = stripProviderMetadata(capability.Output)
		}
		if len(item) > 0 {
			result = append(result, item)
		}
	}
	return result
}

// compactReflectionEvidencePayload keeps the original observation payloads
// available to Reflection while projecting action-result facts exclusively
// through the typed ActionOutcome allowlist. A visible assistant realization
// remains durable for replay, but it is never a learning fact sent back to the
// model.
func compactReflectionEvidencePayload(eventType string, value any) any {
	decoded := decodeProviderJSONValue(value)
	if eventType != "autonomy.result" {
		return stripProviderMetadata(decoded)
	}
	payload := mapValue(decoded)
	result := make(map[string]any, 2)
	outcomeValues := payload["outcomes"]
	if outcomeValues == nil && payload["outcome"] != nil {
		outcomeValues = []any{payload["outcome"]}
	}
	var outcomes []map[string]any
	if encoded := jsonBytes(outcomeValues); len(encoded) > 0 {
		_ = json.Unmarshal(encoded, &outcomes)
	}
	if compact := compactRecentActionOutcomes(outcomes); len(compact) > 0 {
		result["outcomes"] = compact
	}
	if influences := compactReflectionInfluences(payload["influences"]); len(influences) > 0 {
		result["influences"] = influences
	}
	return result
}

func compactReflectionInfluences(value any) []map[string]any {
	var values []map[string]any
	if encoded := jsonBytes(value); len(encoded) > 0 {
		_ = json.Unmarshal(encoded, &values)
	}
	result := make([]map[string]any, 0, len(values))
	for _, influence := range values {
		item := make(map[string]any, 4)
		for _, key := range []string{"ref", "role", "confidence", "note"} {
			if child, ok := influence[key]; ok && child != nil && child != "" {
				item[key] = child
			}
		}
		if len(item) > 0 {
			result = append(result, item)
		}
	}
	return result
}

// decodeProviderJSONValue normalizes values read from JSONB columns before
// applying provider redaction. pgx may scan JSONB into json.RawMessage (or
// []byte); treating those values as opaque bytes makes mapValue return an
// empty object and silently drops the evidence from Reflection prompts.
func decodeProviderJSONValue(value any) any {
	switch typed := value.(type) {
	case json.RawMessage:
		return decodeJSONValue([]byte(typed))
	case []byte:
		return decodeJSONValue(typed)
	default:
		return value
	}
}

func isEmptyReflectionProviderValue(value any) bool {
	switch typed := value.(type) {
	case nil:
		return true
	case map[string]any:
		return len(typed) == 0
	case []any:
		return len(typed) == 0
	case []map[string]any:
		return len(typed) == 0
	case string:
		return strings.TrimSpace(typed) == ""
	default:
		return false
	}
}

func compactProviderFact(raw []byte) any {
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return map[string]any{}
	}
	return stripProviderMetadata(value)
}

// compactMediaConceptForProvider separates the durable media concept used by
// Core/ComfyUI from the smaller semantic payload sent to a media prompt or
// quality model. Reference asset IDs stay in the persisted concept so Core
// can upload them, but workflow/timeline and transport metadata are omitted
// from the LLM-facing copy.
func compactMediaConceptForProvider(raw string) string {
	var value map[string]any
	if err := json.Unmarshal([]byte(raw), &value); err != nil || len(value) == 0 {
		return raw
	}
	result := cloneMap(value)
	if binding := mapValue(result["context_binding"]); len(binding) > 0 {
		compactBinding := make(map[string]any, 4)
		life := mapValue(binding["current_life"])
		if len(life) == 0 {
			life = mapValue(binding["life_context"])
		}
		if lifeContext := compactLifeContext(life); len(lifeContext) > 0 {
			compactBinding["current_life"] = lifeContext
		}
		if visualIdentity := compactVisualIdentity(mapValue(binding["visual_identity"])); len(visualIdentity) > 0 {
			compactBinding["visual_identity"] = visualIdentity
		}
		if appearance := mapValue(binding["appearance"]); len(appearance) > 0 {
			compactBinding["appearance"] = appearance
		}
		state := mapValue(binding["current_state"])
		if len(state) == 0 {
			state = mapValue(binding["inner_state"])
		}
		if innerState := compactInnerState(state); len(innerState) > 0 {
			compactBinding["current_state"] = innerState
		}
		result["context_binding"] = compactBinding
	}
	if visualIdentity := compactVisualIdentityForMediaProvider(mapValue(result["visual_identity"])); len(visualIdentity) > 0 {
		result["visual_identity"] = visualIdentity
	}
	if constraints := compactRendererConstraints(mapValue(result["renderer_constraints"])); len(constraints) > 0 {
		result["renderer_constraints"] = constraints
	}
	cleaned, ok := stripProviderMetadata(result).(map[string]any)
	if !ok {
		return raw
	}
	return jsonString(filterMediaProviderConcept(cleaned))
}

var mediaProviderConceptKeys = map[string]struct{}{
	"purpose": {}, "stage": {}, "intent": {}, "render_intent": {},
	"scene": {}, "activity": {}, "location": {}, "mood": {},
	"action": {}, "pose": {}, "expression": {}, "appearance": {}, "wardrobe": {},
	"lighting": {}, "style": {}, "color": {}, "palette": {},
	"camera": {}, "framing": {}, "composition": {}, "angle": {},
	"capture": {}, "capture_intent": {}, "capture_relationship": {},
	"device": {}, "device_visibility": {}, "mirror": {}, "photographer": {},
	"human_subjects": {}, "humanSubjects": {}, "non_human_objects": {}, "nonHumanObjects": {},
	"subjects": {}, "people": {}, "objects": {}, "props": {},
	"exclusions": {}, "negative_prompt": {}, "constraints": {},
	"prompt": {}, "subject_count": {}, "views": {},
	"context_binding": {}, "renderer_constraints": {}, "visual_identity": {},
	"context_override": {},
}

func filterMediaProviderConcept(value map[string]any) map[string]any {
	result := make(map[string]any, len(value))
	for key, child := range value {
		if _, allowed := mediaProviderConceptKeys[key]; !allowed {
			continue
		}
		if key == "context_binding" {
			result[key] = cloneMap(mapValue(child))
			continue
		}
		if key == "context_override" {
			override := mapValue(child)
			if explicit, ok := override["explicit"].(bool); ok && explicit {
				result[key] = map[string]any{"explicit": true}
			}
			continue
		}
		result[key] = child
	}
	return result
}

// stripProviderMetadata removes persistence/coordination fields from the
// provider-facing projection without touching the authoritative Core value.
// IDs and evidence hashes are useful for replay and writes, but they add no
// semantic signal for the model and make prompts much harder to read.
func stripProviderMetadata(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, child := range typed {
			if providerMetadataKey(key) {
				continue
			}
			result[key] = stripProviderMetadata(child)
		}
		return result
	case []map[string]any:
		result := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			cleaned := stripProviderMetadata(item).(map[string]any)
			if len(cleaned) > 0 {
				result = append(result, cleaned)
			}
		}
		return result
	case []any:
		result := make([]any, 0, len(typed))
		for _, child := range typed {
			cleaned := stripProviderMetadata(child)
			if object, ok := cleaned.(map[string]any); ok && len(object) == 0 {
				continue
			}
			result = append(result, cleaned)
		}
		return result
	default:
		if text, ok := value.(string); ok {
			return strings.TrimSpace(providerHashPattern.ReplaceAllString(text, ""))
		}
		return value
	}
}

func providerMetadataKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "-", ""), "_", ""))
	// Personality profile identifiers are semantic protocol values: the model
	// must be able to name the dominant profile and request a valid switch.
	switch normalized {
	case "profileid", "activeprofileid", "fromprofileid", "targetprofileid", "currentprofileid":
		return false
	}
	if normalized == "id" || strings.HasSuffix(normalized, "id") {
		return true
	}
	switch normalized {
	case "provenance", "status", "schema", "schemaversion", "evidencerefs", "source", "sequence", "revision", "createdat", "updatedat", "lastupdatedat", "occurredat", "expiresat", "checkedat", "generatedat", "instant", "conversation", "conversationref", "fluctlight", "fluctlightid", "sourcefact", "idempotencykey", "nativecognitiondepth", "cycleguard":
		return true
	default:
		return false
	}
}

// stripProviderContextMetadata removes storage/coordination metadata while
// preserving semantic evidence references and memory creation time. Unlike
// stripProviderMetadata, it never rewrites arbitrary natural-language strings
// that merely resemble an internal ID.
func stripProviderContextMetadata(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, child := range typed {
			if providerContextMetadataKey(key) {
				continue
			}
			result[key] = stripProviderContextMetadata(child)
		}
		return result
	case []map[string]any:
		result := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			cleaned, _ := stripProviderContextMetadata(item).(map[string]any)
			if len(cleaned) > 0 {
				result = append(result, cleaned)
			}
		}
		return result
	case []any:
		result := make([]any, 0, len(typed))
		for _, child := range typed {
			cleaned := stripProviderContextMetadata(child)
			if object, ok := cleaned.(map[string]any); ok && len(object) == 0 {
				continue
			}
			result = append(result, cleaned)
		}
		return result
	default:
		return value
	}
}

func providerContextMetadataKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "-", ""), "_", ""))
	switch normalized {
	case "profileid", "activeprofileid", "fromprofileid", "targetprofileid", "currentprofileid":
		return false
	}
	if normalized == "id" || strings.HasSuffix(normalized, "id") {
		return true
	}
	switch normalized {
	case "schema", "schemaversion", "revision", "updatedat", "lastupdatedat", "occurredat", "expiresat", "checkedat", "generatedat", "instant", "conversation", "conversationref", "fluctlight", "fluctlightid", "sourcefact", "visibility", "foreignkey", "persistence", "transport", "status", "source", "provenance", "idempotencykey", "nativecognitiondepth", "cycleguard":
		return true
	default:
		return false
	}
}
