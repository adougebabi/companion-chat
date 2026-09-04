package core

import (
	"regexp"
	"strings"
	"time"
)

var providerHashPattern = regexp.MustCompile(`\b(?:message|memory|inbox|wake_fact|fluctlight|conversation|claim|event|fact|turn|provider|assessment|decision)_[A-Za-z0-9]{16,64}\b`)

// compactCognitionContext is the Provider-facing projection of a full
// ContextProjection. The full projection remains the durable/replayable
// internal value; this DTO keeps the semantic three-layer context, the stable
// visual identity snapshot, and non-empty evidence collections needed by the
// current operation.
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
	if projection.CurrentUserText != "" {
		result["current_user_text"] = projection.CurrentUserText
	}
	if len(projection.RecentMessages) > 0 {
		result["recent_messages"] = compactRecentMessages(projection.RecentMessages)
	}
	if len(projection.DevelopingSelf) > 0 {
		result["developing_self"] = compactDevelopingSelf(projection.DevelopingSelf)
	}
	if len(projection.Memories) > 0 {
		result["memories"] = compactMemories(projection.Memories)
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
	if len(projection.VisualIdentity) > 0 {
		result["visual_identity"] = projection.VisualIdentity
	}
	if len(projection.Presence) > 0 {
		result["presence"] = projection.Presence
	}
	return stripProviderMetadata(result).(map[string]any)
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
	if lifeContext := mapValue(data["life_context"]); len(lifeContext) > 0 {
		data["life_context"] = compactLifeContext(lifeContext)
	}
	result["data"] = data
	return result
}

func compactRecentMessages(messages []map[string]any) string {
	var result strings.Builder
	for _, message := range messages {
		kind := stringValue(message["kind"])
		text := strings.TrimSpace(stringValue(message["text"]))
		if kind == "" && text == "" {
			continue
		}
		if kind == "" {
			kind = "message"
		}
		stamp := compactMessageTime(stringValue(message["created_at"]))
		result.WriteString("[")
		result.WriteString(stamp)
		result.WriteString("] ")
		result.WriteString(kind)
		result.WriteString(": ")
		result.WriteString(text)
		result.WriteByte('\n')
	}
	return strings.TrimRight(result.String(), "\n")
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
		for _, key := range []string{"category", "claim", "value", "confidence", "evidence_refs", "provenance", "status", "expires_at"} {
			if value, ok := claim[key]; ok && value != nil && value != "" {
				if key == "evidence_refs" && len(arrayValue(value)) == 0 {
					continue
				}
				compact[key] = value
			}
		}
		if len(compact) > 0 {
			result = append(result, compact)
		}
	}
	return result
}

func compactMemories(memories []map[string]any) []map[string]any {
	result := make([]map[string]any, 0, len(memories))
	for _, memory := range memories {
		compact := make(map[string]any, 7)
		for _, key := range []string{"type", "content", "confidence", "importance", "emotional_significance", "created_at", "evidence_refs"} {
			if value, ok := memory[key]; ok && value != nil && value != "" {
				if key == "evidence_refs" && len(arrayValue(value)) == 0 {
					continue
				}
				compact[key] = value
			}
		}
		if len(compact) > 0 {
			result = append(result, compact)
		}
	}
	return result
}

func compactInnerState(inner map[string]any) map[string]any {
	result := make(map[string]any, 6)
	if pad := compactStateMap(inner["pad"], []string{"arousal", "pleasure", "dominance"}); len(pad) > 0 {
		result["pad"] = pad
	}
	if mood := compactStateMap(inner["mood"], []string{"label", "source", "intensity"}); len(mood) > 0 {
		result["mood"] = mood
	}
	if momentum := compactStateMap(inner["momentum"], []string{"value", "trend", "arousal_momentum", "dominance_momentum", "pleasure_momentum"}); len(momentum) > 0 {
		result["momentum"] = momentum
	}
	for _, key := range []string{"drives", "conflicts"} {
		if value, ok := inner[key]; ok && value != nil {
			result[key] = value
		}
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
	result := make(map[string]any, 5)
	for _, key := range []string{"source", "scene", "activity", "location", "instant"} {
		if value, ok := context[key]; ok && value != nil && value != "" {
			result[key] = value
		}
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
	if normalized == "id" || strings.HasSuffix(normalized, "id") {
		return true
	}
	switch normalized {
	case "provenance", "status", "schema", "schemaversion", "evidencerefs", "source", "sequence", "revision", "createdat", "updatedat", "lastupdatedat", "occurredat", "expiresat", "checkedat", "generatedat", "instant", "conversation", "conversationref", "fluctlight", "fluctlightid", "sourcefact":
		return true
	default:
		return false
	}
}
