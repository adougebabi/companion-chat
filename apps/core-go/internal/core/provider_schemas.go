package core

// The Provider boundary uses operation-specific JSON Schemas. A single model
// role can serve several domain operations (for example cognitive_assessment
// also generates a daily schedule), so role-only schemas are too permissive
// and allow the model to echo unrelated context.

func objectSchema(properties map[string]any, required []string, additionalProperties bool) map[string]any {
	result := map[string]any{"type": "object", "properties": properties, "additionalProperties": additionalProperties}
	if len(required) > 0 {
		values := make([]any, len(required))
		for index, value := range required {
			values[index] = value
		}
		result["required"] = values
	}
	return result
}

func stringSchema() map[string]any  { return map[string]any{"type": "string"} }
func numberSchema() map[string]any  { return map[string]any{"type": "number"} }
func integerSchema() map[string]any { return map[string]any{"type": "integer", "minimum": 0} }

func unitNumberSchema() map[string]any {
	return map[string]any{"type": "number", "minimum": 0, "maximum": 1}
}

func arraySchema(items map[string]any) map[string]any {
	return map[string]any{"type": "array", "items": items}
}

func openObjectSchema() map[string]any {
	return map[string]any{"type": "object", "additionalProperties": true}
}

func appraisalResponseSchema() map[string]any {
	properties := map[string]any{}
	for _, field := range []string{"relevance", "goal_congruence", "reward", "loss", "social_threat", "controllability", "responsibility", "relationship_significance", "expected_effect"} {
		properties[field] = numberSchema()
	}
	properties["evidence_refs"] = arraySchema(stringSchema())
	properties["event_kind"] = stringSchema()
	properties["direction"] = stringSchema()
	return objectSchema(properties, []string{"relevance", "goal_congruence", "reward", "loss", "social_threat", "controllability", "responsibility", "relationship_significance", "expected_effect", "evidence_refs", "event_kind", "direction"}, false)
}

func cognitiveStageSchema() map[string]any {
	return map[string]any{"anyOf": []any{stringSchema(), openObjectSchema()}}
}

func claimSchema() map[string]any {
	return openObjectSchema()
}

func toolCallSchema() map[string]any {
	return openObjectSchema()
}

func cognitiveTurnResponseSchema() map[string]any {
	properties := map[string]any{
		"action_type":     stringSchema(),
		"response_intent": stringSchema(),
		"visible_text":    stringSchema(),
		"response_plan":   openObjectSchema(),
		"claims":          arraySchema(claimSchema()),
		"appraisal":       appraisalResponseSchema(),
		"attention":       cognitiveStageSchema(),
		"thought":         cognitiveStageSchema(),
		"desire":          cognitiveStageSchema(),
		"agency":          cognitiveStageSchema(),
		"self_evaluation": openObjectSchema(),
		"tool_calls":      arraySchema(toolCallSchema()),
		"evidence_refs":   arraySchema(stringSchema()),
	}
	return objectSchema(properties, []string{"action_type", "response_intent", "visible_text", "response_plan", "claims", "appraisal", "attention", "thought", "desire", "agency", "self_evaluation", "tool_calls", "evidence_refs"}, false)
}

func dailyReviewResponseSchema() map[string]any {
	return objectSchema(map[string]any{
		"action_type":     stringSchema(),
		"response_intent": stringSchema(),
		"tool_calls":      arraySchema(toolCallSchema()),
	}, []string{"action_type", "response_intent", "tool_calls"}, false)
}

func wakeUpResponseSchema() map[string]any {
	return objectSchema(map[string]any{
		"attention":       cognitiveStageSchema(),
		"thought":         cognitiveStageSchema(),
		"desire":          cognitiveStageSchema(),
		"agency":          cognitiveStageSchema(),
		"appraisal":       appraisalResponseSchema(),
		"action_type":     stringSchema(),
		"response_intent": stringSchema(),
		"evidence_refs":   arraySchema(stringSchema()),
		"tool_calls":      arraySchema(toolCallSchema()),
	}, []string{"attention", "thought", "desire", "agency", "appraisal", "action_type", "response_intent", "evidence_refs", "tool_calls"}, false)
}

func nativeCognitionResponseSchema() map[string]any {
	return objectSchema(map[string]any{
		"appraisal": appraisalResponseSchema(),
		"attention": cognitiveStageSchema(),
		"thought":   cognitiveStageSchema(),
		"desire":    cognitiveStageSchema(),
		"agency":    cognitiveStageSchema(),
	}, []string{"appraisal", "attention", "thought", "desire", "agency"}, false)
}

func scheduleResponseSchema() map[string]any {
	item := objectSchema(map[string]any{
		"start_at":          stringSchema(),
		"end_at":            stringSchema(),
		"activity":          stringSchema(),
		"scene":             stringSchema(),
		"item_type":         stringSchema(),
		"status":            stringSchema(),
		"priority":          unitNumberSchema(),
		"flexibility":       unitNumberSchema(),
		"interruption_cost": unitNumberSchema(),
	}, []string{"start_at", "end_at", "activity", "scene", "item_type", "status", "priority", "flexibility", "interruption_cost"}, false)
	return objectSchema(map[string]any{
		"items":             arraySchema(item),
		"reschedule_policy": openObjectSchema(),
	}, []string{"items", "reschedule_policy"}, false)
}

func reflectionResponseSchema() map[string]any {
	candidates := arraySchema(openObjectSchema())
	return objectSchema(map[string]any{
		"memory_candidates":       candidates,
		"relationship_candidates": candidates,
		"self_model_candidates":   candidates,
		"personality_candidates":  candidates,
		"drive_candidates":        candidates,
		"preference_candidates":   candidates,
		"trigger_candidates":      candidates,
	}, []string{"memory_candidates", "relationship_candidates", "self_model_candidates", "personality_candidates", "drive_candidates", "preference_candidates", "trigger_candidates"}, false)
}

func initializationResponseSchema() map[string]any {
	goal := objectSchema(map[string]any{
		"description": stringSchema(),
		"importance":  unitNumberSchema(),
		"urgency":     unitNumberSchema(),
	}, []string{"description", "importance", "urgency"}, false)
	intention := objectSchema(map[string]any{
		"action":     stringSchema(),
		"goal_index": integerSchema(),
		"confidence": unitNumberSchema(),
	}, []string{"action", "goal_index", "confidence"}, false)
	foundation := objectSchema(map[string]any{
		"identity":           openObjectSchema(),
		"personality":        openObjectSchema(),
		"behavioral_policy":  openObjectSchema(),
		"life_profile":       openObjectSchema(),
		"initial_goals":      arraySchema(goal),
		"initial_intentions": arraySchema(intention),
		"provenance":         openObjectSchema(),
	}, []string{"identity", "personality", "behavioral_policy", "life_profile", "initial_goals", "initial_intentions", "provenance"}, false)
	// Accept both the canonical {foundation:{...}} envelope and the existing
	// flat-provider compatibility shape without allowing unrelated root keys.
	return map[string]any{
		"anyOf": []any{
			objectSchema(map[string]any{"foundation": foundation}, []string{"foundation"}, false),
			objectSchema(map[string]any{
				"identity":           openObjectSchema(),
				"personality":        openObjectSchema(),
				"behavioral_policy":  openObjectSchema(),
				"life_profile":       openObjectSchema(),
				"initial_goals":      arraySchema(goal),
				"initial_intentions": arraySchema(intention),
				"provenance":         openObjectSchema(),
			}, []string{"identity", "personality", "behavioral_policy", "life_profile", "initial_goals", "initial_intentions", "provenance"}, false),
		},
	}
}
