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

func enumStringSchema(values ...string) map[string]any {
	enumValues := make([]any, len(values))
	for index, value := range values {
		enumValues[index] = value
	}
	return map[string]any{"type": "string", "enum": enumValues}
}

func unitNumberSchema() map[string]any {
	return map[string]any{"type": "number", "minimum": 0, "maximum": 1}
}

func arraySchema(items map[string]any) map[string]any {
	return map[string]any{"type": "array", "items": items}
}

func openObjectSchema() map[string]any {
	return map[string]any{"type": "object", "additionalProperties": true}
}

// MLX strict-json-schema rejects an unconstrained `{}` schema by hanging while
// compiling the response grammar. Persona values are intentionally rendered
// as open JSON objects for the first slice, which preserves arbitrary nested
// fields without relying on an unsupported empty schema.
func anyJSONSchema() map[string]any { return openObjectSchema() }

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
		"action_type":      stringSchema(),
		"response_intent":  stringSchema(),
		"visible_text":     stringSchema(),
		"response_plan":    openObjectSchema(),
		"core_alignment":   openObjectSchema(),
		"state_expression": openObjectSchema(),
		"claims":           arraySchema(claimSchema()),
		"appraisal":        appraisalResponseSchema(),
		"attention":        cognitiveStageSchema(),
		"thought":          cognitiveStageSchema(),
		"desire":           cognitiveStageSchema(),
		"agency":           cognitiveStageSchema(),
		"self_evaluation":  openObjectSchema(),
		"tool_calls":       arraySchema(toolCallSchema()),
		"evidence_refs":    arraySchema(stringSchema()),
	}
	return objectSchema(properties, []string{"action_type", "response_intent", "visible_text", "response_plan", "claims", "appraisal", "attention", "thought", "desire", "agency", "self_evaluation", "tool_calls", "evidence_refs"}, false)
}

func dailyReviewResponseSchema() map[string]any {
	return objectSchema(map[string]any{
		"action_type":     enumStringSchema("proactive_message", "moment", "no_op"),
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
	developingSelfCandidate := objectSchema(map[string]any{
		"category":      enumStringSchema("preference", "habit", "sensitivity", "emotion_pattern", "self_perception", "capability", "interest"),
		"claim":         stringSchema(),
		"value":         anyJSONSchema(),
		"confidence":    unitNumberSchema(),
		"evidence_refs": arraySchema(stringSchema()),
		"provenance":    openObjectSchema(),
	}, []string{"category", "claim", "value", "confidence", "evidence_refs", "provenance"}, false)
	return objectSchema(map[string]any{
		"memory_candidates":          candidates,
		"relationship_candidates":    candidates,
		"developing_self_candidates": arraySchema(developingSelfCandidate),
		"drive_candidates":           candidates,
		"preference_candidates":      candidates,
		"trigger_candidates":         candidates,
	}, []string{"memory_candidates", "relationship_candidates", "developing_self_candidates", "drive_candidates", "preference_candidates", "trigger_candidates"}, false)
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
	claim := objectSchema(map[string]any{
		"category":      enumStringSchema("preference", "habit", "sensitivity", "emotion_pattern", "self_perception", "capability", "interest"),
		"claim":         stringSchema(),
		"value":         anyJSONSchema(),
		"confidence":    unitNumberSchema(),
		"evidence_refs": arraySchema(stringSchema()),
		"provenance":    openObjectSchema(),
		"status":        enumStringSchema("active", "uncertain"),
	}, []string{"category", "claim", "value", "confidence", "evidence_refs", "provenance"}, false)
	corePersona := objectSchema(map[string]any{
		"schema_version":    integerSchema(),
		"identity":          openObjectSchema(),
		"personality":       openObjectSchema(),
		"behavioral_policy": openObjectSchema(),
		"life_profile":      openObjectSchema(),
	}, []string{"identity", "personality", "behavioral_policy", "life_profile"}, false)
	developingSelf := objectSchema(map[string]any{"claims": arraySchema(claim)}, []string{"claims"}, false)
	return objectSchema(map[string]any{
		"core_persona":       corePersona,
		"developing_self":    developingSelf,
		"initial_goals":      arraySchema(goal),
		"initial_intentions": arraySchema(intention),
	}, []string{"core_persona", "developing_self", "initial_goals", "initial_intentions"}, false)
}

func visualIdentityVisionResponseSchema() map[string]any {
	return objectSchema(map[string]any{
		"summary":        stringSchema(),
		"observations":   openObjectSchema(),
		"identity_match": unitNumberSchema(),
		"confidence":     unitNumberSchema(),
	}, []string{"summary", "observations", "identity_match", "confidence"}, false)
}

func visualIdentityPatchResponseSchema() map[string]any {
	return objectSchema(map[string]any{
		"decision":             enumStringSchema("accepted", "regenerate"),
		"summary":              stringSchema(),
		"seed_prompt":          stringSchema(),
		"prompt_patch":         openObjectSchema(),
		"renderer_constraints": openObjectSchema(),
		"feedback":             stringSchema(),
		"confidence":           unitNumberSchema(),
	}, []string{"decision", "summary", "prompt_patch", "renderer_constraints", "feedback", "confidence"}, false)
}

func visualIdentitySeedResponseSchema() map[string]any {
	return objectSchema(map[string]any{
		"decision":             enumStringSchema("accepted", "regenerate"),
		"summary":              stringSchema(),
		"seed_prompt":          stringSchema(),
		"prompt_patch":         openObjectSchema(),
		"renderer_constraints": openObjectSchema(),
		"feedback":             stringSchema(),
		"confidence":           unitNumberSchema(),
	}, []string{"seed_prompt"}, false)
}
