package core

import (
	"encoding/json"
	"fmt"
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
	if len(projection.RecentMessages) > 0 {
		if recent := compactRecentMessages(projection.RecentMessages, projection.CurrentUserText); recent != "" {
			result["recent_messages"] = recent
		}
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
		if visualIdentity := compactVisualIdentity(projection.VisualIdentity); len(visualIdentity) > 0 {
			result["visual_identity"] = visualIdentity
		}
	}
	if len(projection.Presence) > 0 {
		result["presence"] = projection.Presence
	}
	if goals := compactProviderGoals(projection.Goals); len(goals) > 0 {
		result["goals"] = goals
	}
	if intentions := compactProviderIntentions(projection.Intentions); len(intentions) > 0 {
		result["intentions"] = intentions
	}
	return stripProviderMetadata(result).(map[string]any)
}

func compactProviderGoals(goals []map[string]any) []map[string]any {
	result := make([]map[string]any, 0, len(goals))
	for _, goal := range goals {
		item := map[string]any{}
		for _, key := range []string{"description", "importance", "urgency", "progress"} {
			if value, ok := goal[key]; ok && value != nil {
				item[key] = value
			}
		}
		if status := stringValue(goal["status"]); status != "" {
			item["state"] = status
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
		for _, key := range []string{"goal", "action", "confidence", "preferred_time", "expiration"} {
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

func compactRecentMessages(messages []map[string]any, currentUserText string) string {
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
	var result strings.Builder
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
	result := make(map[string]any, 6)
	for _, key := range []string{"source", "scene", "activity", "location", "current_time", "timezone"} {
		if value, ok := context[key]; ok && value != nil && value != "" {
			result[key] = value
		}
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
	for _, key := range []string{"answer_mode", "action_type", "response_intent", "tone"} {
		if value, ok := plan[key]; ok && value != nil && value != "" {
			result[key] = value
		}
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

func compactToolResultsForProvider(results []ToolResultV1) []map[string]any {
	result := make([]map[string]any, 0, len(results))
	for _, tool := range results {
		item := make(map[string]any, 4)
		for _, key := range []string{"name", "status", "error_code"} {
			value := map[string]any{"name": tool.Name, "status": tool.Status, "error_code": tool.ErrorCode}[key]
			if value != nil && value != "" {
				item[key] = value
			}
		}
		if tool.Output != nil {
			item["output"] = stripProviderMetadata(tool.Output)
		}
		if len(item) > 0 {
			result = append(result, item)
		}
	}
	return result
}

func compactReflectionEvidence(evidence []map[string]any) []map[string]any {
	result := make([]map[string]any, 0, len(evidence))
	for _, item := range evidence {
		compact := make(map[string]any, 2)
		if eventType := stringValue(item["event_type"]); eventType != "" {
			compact["event_type"] = eventType
		}
		if sequence, ok := item["sequence"]; ok && sequence != nil {
			// Keep a short stable reference so reflection candidates can cite the
			// evidence without exposing the database fact ID or hash.
			compact["evidence_ref"] = "sequence:" + fmt.Sprint(sequence)
		}
		if payload := stripProviderMetadata(item["payload"]); len(mapValue(payload)) > 0 {
			compact["payload"] = payload
		}
		if len(compact) > 0 {
			result = append(result, compact)
		}
	}
	return result
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
		if lifeContext := compactLifeContext(mapValue(binding["life_context"])); len(lifeContext) > 0 {
			compactBinding["life_context"] = lifeContext
		}
		if visualIdentity := compactVisualIdentity(mapValue(binding["visual_identity"])); len(visualIdentity) > 0 {
			compactBinding["visual_identity"] = visualIdentity
		}
		if appearance := mapValue(binding["appearance"]); len(appearance) > 0 {
			compactBinding["appearance"] = appearance
		}
		if innerState := compactInnerState(mapValue(binding["inner_state"])); len(innerState) > 0 {
			compactBinding["inner_state"] = innerState
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
	return jsonString(cleaned)
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
