package core

// compactCognitionContext is the Provider-facing projection of a full
// ContextProjection. The full projection remains the durable/replayable
// internal value; this DTO keeps only the semantic three-layer context and
// non-empty evidence collections needed by the current operation.
//
// In particular, capability manifests are intentionally absent here. Calls
// that can execute capabilities already send the authoritative native `tools`
// catalog separately in the Provider request. Sending the same schemas inside
// user content needlessly doubles prompt size and gives the model two copies
// of the contract to reconcile.
func compactCognitionContext(projection ContextProjection) map[string]any {
	result := map[string]any{
		"schema_version": projection.SchemaVersion,
		"core_persona":   compactCorePersona(projection),
		"current_state":  compactCurrentState(projection),
	}
	if projection.FluctlightID != "" {
		result["fluctlight_id"] = projection.FluctlightID
	}
	if projection.ConversationID != "" {
		result["conversation_id"] = projection.ConversationID
	}
	if projection.SourceFactID != "" {
		result["source_fact_id"] = projection.SourceFactID
	}
	if projection.CurrentUserText != "" {
		result["current_user_text"] = projection.CurrentUserText
	}
	if len(projection.RecentMessages) > 0 {
		result["recent_messages"] = projection.RecentMessages
	}
	result["context_revision"] = projection.ContextRevision
	result["core_persona_revision"] = projection.CorePersonaRevision
	result["developing_self_revision"] = projection.DevelopingSelfRevision
	result["current_state_revision"] = projection.CurrentStateRevision
	if len(projection.DevelopingSelf) > 0 {
		result["developing_self"] = projection.DevelopingSelf
	}
	if len(projection.Memories) > 0 {
		result["memories"] = projection.Memories
	}
	if len(projection.Relationships) > 0 {
		result["relationships"] = projection.Relationships
	}
	if len(projection.Hypotheses) > 0 {
		result["hypotheses"] = projection.Hypotheses
	}
	if len(projection.DriveSlots) > 0 {
		result["drive_slots"] = projection.DriveSlots
	}
	if len(projection.PreferenceSlots) > 0 {
		result["preference_slots"] = projection.PreferenceSlots
	}
	if len(projection.TriggerPreferences) > 0 {
		result["trigger_preferences"] = projection.TriggerPreferences
	}
	if len(projection.Presence) > 0 {
		result["presence"] = projection.Presence
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
	result["data"] = data
	return result
}
