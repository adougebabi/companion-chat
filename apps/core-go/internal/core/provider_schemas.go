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
func booleanSchema() map[string]any { return map[string]any{"type": "boolean"} }

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

func nullableStringSchema() map[string]any {
	return map[string]any{"anyOf": []any{stringSchema(), map[string]any{"type": "null"}}}
}

func nullableNumberSchema() map[string]any {
	return map[string]any{"anyOf": []any{numberSchema(), map[string]any{"type": "null"}}}
}

func jsonValueSchema() map[string]any {
	return map[string]any{"anyOf": []any{stringSchema(), numberSchema(), booleanSchema(), map[string]any{"type": "null"}, openObjectSchema(), arraySchema(openObjectSchema())}}
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

func outputPreferenceDecisionSchema() map[string]any {
	return objectSchema(map[string]any{
		"matched":    booleanSchema(),
		"channel":    enumStringSchema("text", "image", "voice", "moment", "none"),
		"profile_id": stringSchema(),
		"trigger_id": stringSchema(),
		"reason":     stringSchema(),
		"confidence": unitNumberSchema(),
	}, []string{"matched", "channel", "profile_id", "trigger_id", "reason", "confidence"}, false)
}

func claimSchema() map[string]any {
	return openObjectSchema()
}

func toolCallSchema() map[string]any {
	return openObjectSchema()
}

func cognitiveTurnResponseSchema() map[string]any {
	personalityDecision := objectSchema(map[string]any{
		"decision":          enumStringSchema("keep", "switch"),
		"from_profile_id":   stringSchema(),
		"target_profile_id": stringSchema(),
		"trigger_id":        stringSchema(),
		"reason":            stringSchema(),
		"confidence":        unitNumberSchema(),
		"evidence_refs":     arraySchema(stringSchema()),
	}, []string{"decision", "from_profile_id", "target_profile_id", "trigger_id", "reason", "confidence", "evidence_refs"}, false)
	properties := map[string]any{
		"action_type":                stringSchema(),
		"response_intent":            stringSchema(),
		"visible_text":               stringSchema(),
		"response_plan":              openObjectSchema(),
		"personality_decision":       personalityDecision,
		"output_preference_decision": outputPreferenceDecisionSchema(),
		"core_alignment":             openObjectSchema(),
		"state_expression":           openObjectSchema(),
		"claims":                     arraySchema(claimSchema()),
		"appraisal":                  appraisalResponseSchema(),
		"attention":                  cognitiveStageSchema(),
		"thought":                    cognitiveStageSchema(),
		"desire":                     cognitiveStageSchema(),
		"agency":                     cognitiveStageSchema(),
		"self_evaluation":            openObjectSchema(),
		"tool_calls":                 arraySchema(toolCallSchema()),
		"evidence_refs":              arraySchema(stringSchema()),
	}
	return objectSchema(properties, []string{"action_type", "response_intent", "visible_text", "response_plan", "personality_decision", "output_preference_decision", "claims", "appraisal", "attention", "thought", "desire", "agency", "self_evaluation", "tool_calls", "evidence_refs"}, false)
}

func dailyReviewResponseSchema() map[string]any {
	return objectSchema(map[string]any{
		"action_type":                enumStringSchema("proactive_message", "moment", "no_op"),
		"response_intent":            stringSchema(),
		"tool_calls":                 arraySchema(toolCallSchema()),
		"output_preference_decision": outputPreferenceDecisionSchema(),
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

func mediaQualityAcceptanceResponseSchema() map[string]any {
	violation := objectSchema(map[string]any{
		"code":     stringSchema(),
		"severity": enumStringSchema("hard"),
		"detail":   stringSchema(),
	}, []string{"code", "severity", "detail"}, false)
	observedFacts := objectSchema(map[string]any{
		"subject_matches":    booleanSchema(),
		"appearance_matches": booleanSchema(),
		"scene_matches":      booleanSchema(),
		"capture_matches":    booleanSchema(),
		"framing_matches":    booleanSchema(),
	}, []string{"subject_matches", "appearance_matches", "scene_matches", "capture_matches", "framing_matches"}, false)
	return objectSchema(map[string]any{
		"schema_version": integerSchema(),
		"verdict":        enumStringSchema("pass", "retry", "reject"),
		"violations":     arraySchema(violation),
		"observed_facts": observedFacts,
		"retry_guidance": stringSchema(),
	}, []string{"schema_version", "verdict", "violations", "observed_facts", "retry_guidance"}, false)
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
		"goal_candidates":            candidates,
		"intention_candidates":       candidates,
		"developing_self_candidates": arraySchema(developingSelfCandidate),
		"drive_candidates":           candidates,
		"preference_candidates":      candidates,
		"trigger_candidates":         candidates,
	}, []string{"memory_candidates", "relationship_candidates", "goal_candidates", "intention_candidates", "developing_self_candidates", "drive_candidates", "preference_candidates", "trigger_candidates"}, false)
}

func initializationResponseSchema() map[string]any {
	identity := objectSchema(map[string]any{
		"name":        stringSchema(),
		"age":         nullableNumberSchema(),
		"gender":      nullableStringSchema(),
		"occupation":  nullableStringSchema(),
		"residence":   nullableStringSchema(),
		"timezone":    nullableStringSchema(),
		"birthday":    nullableStringSchema(),
		"background":  nullableStringSchema(),
		"biography":   nullableStringSchema(),
		"core_values": arraySchema(jsonValueSchema()),
		"worldview":   nullableStringSchema(),
		"notes":       nullableStringSchema(),
	}, []string{"name", "age", "gender", "occupation", "residence", "timezone", "birthday", "background", "biography", "core_values", "worldview", "notes"}, false)
	personality := objectSchema(map[string]any{
		"openness":          unitNumberSchema(),
		"conscientiousness": unitNumberSchema(),
		"extraversion":      unitNumberSchema(),
		"agreeableness":     unitNumberSchema(),
		"neuroticism":       unitNumberSchema(),
		"curiosity":         unitNumberSchema(),
		"independence":      unitNumberSchema(),
		"patience":          unitNumberSchema(),
		"empathy":           unitNumberSchema(),
		"assertiveness":     unitNumberSchema(),
		"humor":             unitNumberSchema(),
		"sociability":       unitNumberSchema(),
		"risk_tolerance":    unitNumberSchema(),
		"update_policy":     openObjectSchema(),
	}, []string{"openness", "conscientiousness", "extraversion", "agreeableness", "neuroticism", "curiosity", "independence", "patience", "empathy", "assertiveness", "humor", "sociability", "risk_tolerance", "update_policy"}, false)
	behavioralPolicy := objectSchema(map[string]any{
		"response_style":       stringSchema(),
		"message_length":       stringSchema(),
		"emoji_frequency":      unitNumberSchema(),
		"punctuation_style":    stringSchema(),
		"humor_style":          stringSchema(),
		"sarcasm_tendency":     unitNumberSchema(),
		"directness":           unitNumberSchema(),
		"initiative":           unitNumberSchema(),
		"topic_initiation":     unitNumberSchema(),
		"silence_tolerance":    unitNumberSchema(),
		"response_delay":       numberSchema(),
		"emotional_expression": unitNumberSchema(),
		"conflict_style":       stringSchema(),
		"refusal_style":        stringSchema(),
		"intimacy_expression":  stringSchema(),
	}, []string{"response_style", "message_length", "emoji_frequency", "punctuation_style", "humor_style", "sarcasm_tendency", "directness", "initiative", "topic_initiation", "silence_tolerance", "response_delay", "emotional_expression", "conflict_style", "refusal_style", "intimacy_expression"}, false)
	appearance := objectSchema(map[string]any{
		"chest_cup": enumStringSchema("A", "B", "C", "D"),
	}, nil, true)
	lifeProfile := objectSchema(map[string]any{
		"appearance":            appearance,
		"social_background":     openObjectSchema(),
		"preferences":           openObjectSchema(),
		"life_habits":           arraySchema(jsonValueSchema()),
		"recurring_commitments": arraySchema(jsonValueSchema()),
		"relationship_seeds":    arraySchema(openObjectSchema()),
		"character_constraints": arraySchema(jsonValueSchema()),
	}, []string{"appearance", "social_background", "preferences", "life_habits", "recurring_commitments", "relationship_seeds", "character_constraints"}, false)
	goal := objectSchema(map[string]any{
		"description":     stringSchema(),
		"profile_id":      stringSchema(),
		"importance":      unitNumberSchema(),
		"urgency":         unitNumberSchema(),
		"scope":           enumStringSchema("general", "relationship"),
		"target_actor_id": stringSchema(),
	}, []string{"description", "importance", "urgency"}, false)
	intention := objectSchema(map[string]any{
		"action":     stringSchema(),
		"profile_id": stringSchema(),
		"goal_index": integerSchema(),
		"confidence": unitNumberSchema(),
	}, []string{"action", "goal_index", "confidence"}, false)
	relationship := objectSchema(map[string]any{
		"target_actor_id":       stringSchema(),
		"profile_id":            stringSchema(),
		"role":                  openObjectSchema(),
		"metrics":               openObjectSchema(),
		"trend":                 enumStringSchema("improving", "stable", "declining"),
		"summary":               stringSchema(),
		"emotional_association": openObjectSchema(),
		"evidence_refs":         arraySchema(stringSchema()),
	}, []string{"target_actor_id", "role"}, false)
	personalityProfile := objectSchema(map[string]any{
		"id":                     stringSchema(),
		"name":                   stringSchema(),
		"identity":               openObjectSchema(),
		"personality":            openObjectSchema(),
		"behavioral_policy":      openObjectSchema(),
		"emotional_state":        openObjectSchema(),
		"voice":                  openObjectSchema(),
		"body_language":          openObjectSchema(),
		"behavior_state_machine": openObjectSchema(),
		"behavior_loops":         jsonValueSchema(),
		"scenario_behavior":      jsonValueSchema(),
		"secrets":                jsonValueSchema(),
		"intimacy_progression":   jsonValueSchema(),
		"output_preferences":     jsonValueSchema(),
		"fears":                  arraySchema(jsonValueSchema()),
		"desires":                arraySchema(jsonValueSchema()),
		"extensions":             openObjectSchema(),
	}, []string{"id"}, false)
	personalitySystem := objectSchema(map[string]any{
		"mode":                   enumStringSchema("single", "multiple"),
		"profiles":               arraySchema(personalityProfile),
		"active_profile_id":      stringSchema(),
		"switching":              openObjectSchema(),
		"influence":              openObjectSchema(),
		"conflict_resolution":    openObjectSchema(),
		"integration":            openObjectSchema(),
		"behavior_state_machine": openObjectSchema(),
		"extensions":             openObjectSchema(),
	}, []string{"mode", "profiles", "active_profile_id", "switching", "influence", "conflict_resolution", "integration", "behavior_state_machine", "extensions"}, false)
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
		"schema_version":     integerSchema(),
		"identity":           identity,
		"personality":        personality,
		"behavioral_policy":  behavioralPolicy,
		"life_profile":       lifeProfile,
		"personality_system": personalitySystem,
	}, []string{"identity", "personality", "behavioral_policy", "life_profile", "personality_system"}, false)
	developingSelf := objectSchema(map[string]any{"claims": arraySchema(claim)}, []string{"claims"}, false)
	return objectSchema(map[string]any{
		"schema_version":        integerSchema(),
		"core_persona":          corePersona,
		"developing_self":       developingSelf,
		"initial_goals":         arraySchema(goal),
		"initial_intentions":    arraySchema(intention),
		"initial_relationships": arraySchema(relationship),
		"extensions":            openObjectSchema(),
	}, []string{"schema_version", "core_persona", "developing_self", "initial_relationships", "initial_goals", "initial_intentions", "extensions"}, false)
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
