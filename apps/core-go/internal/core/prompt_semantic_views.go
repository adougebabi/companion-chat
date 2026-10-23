package core

import "encoding/json"

// promptScheduleView is a presentation-only copy. Each plan item appears once;
// current/upcoming pointers use zero-based indexes into items. Core retains the
// original IDs, revisions and complete plan for authorization and mutations.
func promptScheduleView(value map[string]any) map[string]any {
	result := copyPromptData(value).(map[string]any)
	items := arrayValue(result["items"])
	if len(items) == 0 {
		return result
	}
	current := mapValue(result["current_item"])
	upcoming := arrayValue(result["upcoming_items"])
	currentIndex := -1
	upcomingIndexes := []any{}
	for i, raw := range items {
		item := mapValue(raw)
		if jsonString(item) == jsonString(current) {
			currentIndex = i
		}
		for _, next := range upcoming {
			if jsonString(item) == jsonString(next) {
				upcomingIndexes = append(upcomingIndexes, i)
				break
			}
		}
		for _, key := range []string{"local_start_at", "local_end_at", "local_start_time", "local_end_time", "local_time_range"} {
			delete(item, key)
		}
	}
	if currentIndex >= 0 {
		result["current_item_index"] = currentIndex
		delete(result, "current_item")
	}
	if len(upcomingIndexes) == len(upcoming) && len(upcoming) > 0 {
		result["upcoming_item_indexes"] = upcomingIndexes
		delete(result, "upcoming_items")
	}
	return result
}

// Studio definition IDs/versions inside body and allowed_references are output
// vocabulary. Only the job's own persistence envelope is omitted.
func studioPromptView(snapshot map[string]any) map[string]any {
	return compactStateMap(snapshot, []string{"resource_kind", "name", "body", "instruction", "purpose", "allowed_references", "reference_asset"})
}
func npcEncounterPromptView(definition map[string]any) map[string]any {
	result := compactStateMap(definition, []string{"name", "role", "background", "world_rules", "public_character", "public_place", "current_npc_activity"})
	history := []any{}
	for _, raw := range arrayValue(definition["actual_prior_interactions"]) {
		history = append(history, compactStateMap(mapValue(raw), []string{"summary", "occurred_at", "source_kind"}))
	}
	if len(history) > 0 {
		result["actual_prior_interactions"] = history
	}
	return result
}
func npcMomentPromptView(snapshot map[string]any) map[string]any {
	result := compactStateMap(snapshot, []string{"name", "role", "background", "world_rules", "known_encounter_summary", "semantics"})
	result["moment"] = compactStateMap(snapshot["moment"], []string{"text"})
	return result
}

// Requested entity/asset IDs are necessary exact Tool targets. CAS, request
// identity and authorization stay in the original frozen request held by Core.
func ownerLifePromptView(kind string, request map[string]any) map[string]any {
	fields := []string{"reason"}
	switch kind {
	case "wardrobe_wear":
		fields = append(fields, "item_id", "slot")
	case "item_use":
		fields = append(fields, "item_id", "active")
	case "inventory_borrow":
		fields = append(fields, "lender", "items")
	case "inventory_return":
		fields = append(fields, "item_ids")
	case "inventory_remove":
		fields = append(fields, "slots")
	case "inventory_outfit":
		fields = append(fields, "item_ids", "outfit_id")
	case "schedule_item_suggestion":
		fields = append(fields, "operation", "item_id", "start_at", "end_at", "activity", "scene", "location", "place_id", "timezone")
	case "promise":
		fields = append(fields, "title", "starts_at", "due_at")
	case "promise_reschedule":
		fields = append(fields, "intention_id", "starts_at", "due_at")
	case "moment_suggestion":
		fields = append(fields, "text", "media_ids")
	case "media_generate", "item_photo":
		fields = append(fields, "prompt", "_render_intent", "item_id", "place_id", "reference_asset")
	}
	result := compactStateMap(request, fields)
	if item := mapValue(request["_item_snapshot"]); len(item) > 0 {
		result["_item_snapshot"] = compactStateMap(item, []string{"item_kind", "category", "slot", "description", "ownership", "availability"})
	}
	if place := mapValue(request["_place_snapshot"]); len(place) > 0 {
		result["_place_snapshot"] = compactStateMap(place, []string{"name", "description"})
	}
	return result
}

// Preserve JSON numbers and opaque business values without a marshal/unmarshal
// round trip through float64. Only mutable JSON containers are copied.
func copyPromptData(value any) any {
	switch v := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(v))
		for key, child := range v {
			result[key] = copyPromptData(child)
		}
		return result
	case []any:
		result := make([]any, len(v))
		for i, child := range v {
			result[i] = copyPromptData(child)
		}
		return result
	case []map[string]any:
		result := make([]map[string]any, len(v))
		for i, child := range v {
			result[i] = copyPromptData(child).(map[string]any)
		}
		return result
	case json.RawMessage:
		return append(json.RawMessage(nil), v...)
	case []string:
		return append([]string(nil), v...)
	default:
		return value
	}
}

// Only named data fields are opaque. Core event payload envelopes still use
// their declared projections; user JSON/schema properties are never entities.
func promptOpaqueDataField(key string, value any) bool {
	switch key {
	case "value", "value_json", "properties", "$defs", "definitions", "parameters", "input_schema", "output_schema", "document", "body":
		return true
	case "schema":
		_, ok := value.(map[string]any)
		return ok
	default:
		return false
	}
}

func providerPersistenceIDKey(normalized string) bool {
	switch normalized {
	case "id", "actorid", "ownerid", "owneractorid", "authoractorid", "targetactorid", "subjectactorid", "fluctlightid", "ownerfluctlightid", "conversationid", "messageid", "memoryid", "activememoryid", "sourcefactid", "sourceeventid", "factid", "eventid", "goalid", "intentionid", "presenceid", "scheduleid", "scheduleitemid", "itemid", "assetid", "outfitid", "worldid", "placeid", "npcid", "runid", "requestid", "operationid", "workflowid", "correlationid", "providerrequestid", "toolcallid", "callid", "turnid", "assessmentid", "decisionid":
		return true
	default:
		return false
	}
}

// The opportunity identity/revision are the exact world.npc.encounter selector.
// Source event/presence/profile IDs are already frozen and checked by Core.
func npcOpportunityPromptView(opportunity map[string]any) map[string]any {
	result := compactStateMap(opportunity, []string{"id", "revision", "place_name", "character_presence_source", "created_at", "expires_at"})
	result["npc"] = compactStateMap(opportunity["npc"], []string{"public_role", "known"})
	return result
}
