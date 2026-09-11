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

func decisionInfluenceSchema() map[string]any {
	return objectSchema(map[string]any{
		"ref": map[string]any{
			"type": "string", "minLength": 1, "maxLength": maxContextReferenceRunes,
			"pattern": `^[a-z][a-z0-9_]{1,31}:ctx_[a-f0-9]{32}$`,
		},
		"role":       enumStringSchema("grounds", "motivates", "constrains", "conflicts", "satisfies"),
		"confidence": unitNumberSchema(),
		"note":       map[string]any{"type": "string", "minLength": 1, "maxLength": maxDecisionInfluenceNoteRunes},
	}, []string{"ref", "role", "confidence", "note"}, false)
}

func decisionInfluencesSchema() map[string]any {
	result := arraySchema(decisionInfluenceSchema())
	result["maxItems"] = maxDecisionInfluences
	return result
}

func driveSignalsSchema() map[string]any {
	item := objectSchema(map[string]any{
		"ref": map[string]any{
			"type": "string", "minLength": 1, "maxLength": maxContextReferenceRunes,
			"pattern": `^drive:ctx_[a-f0-9]{32}$`,
		},
		"direction":     enumStringSchema("increase", "decrease"),
		"strength":      unitNumberSchema(),
		"confidence":    unitNumberSchema(),
		"evidence_refs": arraySchema(map[string]any{"type": "string", "minLength": 1, "maxLength": maxContextReferenceRunes}),
	}, []string{"ref", "direction", "strength", "confidence", "evidence_refs"}, false)
	result := arraySchema(item)
	result["maxItems"] = 16
	return result
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
	properties["evidence_refs"] = arraySchema(map[string]any{
		"type": "string", "minLength": 1, "maxLength": maxContextReferenceRunes,
		"pattern": `^[a-z][a-z0-9_]{1,31}:ctx_[a-f0-9]{32}$`,
	})
	properties["event_kind"] = stringSchema()
	properties["direction"] = stringSchema()
	properties["drive_signals"] = driveSignalsSchema()
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
	// Claims are persisted by the cognition layer, so this is intentionally a
	// closed contract rather than an open provider extension point. The legacy
	// `claim` alias is normalized at the application boundary for providers that
	// still emit it, but new responses must use kind/content.
	return objectSchema(map[string]any{
		"kind":           enumStringSchema("confirmed_fact", "observed_fact", "supported_hypothesis", "uncertain_hypothesis", "unsupported_self_claim"),
		"content":        stringSchema(),
		"confidence":     unitNumberSchema(),
		"evidence_refs":  arraySchema(stringSchema()),
		"repetition_key": stringSchema(),
	}, []string{"kind", "content", "confidence", "evidence_refs"}, false)
}

func toolCallSchema() map[string]any {
	return openObjectSchema()
}

func selfEvaluationSchema() map[string]any {
	return objectSchema(map[string]any{
		"mode":         enumStringSchema("accepted", "uncertain", "omit", "deferred"),
		"reason_codes": arraySchema(stringSchema()),
		"confidence":   unitNumberSchema(),
		"note":         stringSchema(),
		"extensions":   openObjectSchema(),
	}, nil, false)
}

func responsePlanSchema() map[string]any {
	return objectSchema(map[string]any{
		"profile_id":       stringSchema(),
		"visible_text":     stringSchema(),
		"action_type":      enumStringSchema("reply"),
		"answer_mode":      stringSchema(),
		"response_outline": arraySchema(stringSchema()),
		"tone":             stringSchema(),
		"strategy":         stringSchema(),
		"length":           stringSchema(),
		"self_evaluation":  selfEvaluationSchema(),
		"core_alignment":   openObjectSchema(),
		"state_expression": openObjectSchema(),
		"claims":           arraySchema(claimSchema()),
		"extensions":       openObjectSchema(),
	}, nil, false)
}

func coreAlignmentSchema() map[string]any {
	return objectSchema(map[string]any{
		"aligned":    booleanSchema(),
		"note":       stringSchema(),
		"extensions": openObjectSchema(),
	}, nil, false)
}

func stateExpressionSchema() map[string]any {
	return objectSchema(map[string]any{
		"body":       stringSchema(),
		"mood":       stringSchema(),
		"extensions": openObjectSchema(),
	}, nil, false)
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
		"action_type":                enumStringSchema("reply"),
		"response_intent":            stringSchema(),
		"visible_text":               stringSchema(),
		"response_plan":              responsePlanSchema(),
		"personality_decision":       personalityDecision,
		"output_preference_decision": outputPreferenceDecisionSchema(),
		"core_alignment":             coreAlignmentSchema(),
		"state_expression":           stateExpressionSchema(),
		"claims":                     arraySchema(claimSchema()),
		"appraisal":                  appraisalResponseSchema(),
		"attention":                  cognitiveStageSchema(),
		"thought":                    cognitiveStageSchema(),
		"desire":                     cognitiveStageSchema(),
		"agency":                     cognitiveStageSchema(),
		"self_evaluation":            selfEvaluationSchema(),
		"tool_calls":                 arraySchema(toolCallSchema()),
		"evidence_refs":              arraySchema(stringSchema()),
		"influences":                 decisionInfluencesSchema(),
	}
	// The direct-conversation product boundary has no successful no-op. Visible
	// text may arrive in this object or through conversation.reply, but the same
	// Main cognition must select reply and the application validates concrete
	// output before any effect is committed.
	return objectSchema(properties, []string{"action_type", "response_intent", "tool_calls", "influences"}, false)
}

func dailyReviewResponseSchema() map[string]any {
	return objectSchema(map[string]any{
		"action_type":                enumStringSchema("proactive_message", "moment", "no_op"),
		"response_intent":            stringSchema(),
		"tool_calls":                 arraySchema(toolCallSchema()),
		"output_preference_decision": outputPreferenceDecisionSchema(),
		"influences":                 decisionInfluencesSchema(),
	}, []string{"action_type", "response_intent", "tool_calls", "influences"}, false)
}

func wakeUpResponseSchema() map[string]any {
	return objectSchema(map[string]any{
		"action_type":                stringSchema(),
		"response_intent":            stringSchema(),
		"evidence_refs":              arraySchema(stringSchema()),
		"tool_calls":                 arraySchema(toolCallSchema()),
		"output_preference_decision": outputPreferenceDecisionSchema(),
		"influences":                 decisionInfluencesSchema(),
	}, []string{"action_type", "response_intent", "evidence_refs", "tool_calls", "influences"}, false)
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
		"appraisal":  appraisalResponseSchema(),
		"attention":  cognitiveStageSchema(),
		"thought":    cognitiveStageSchema(),
		"desire":     cognitiveStageSchema(),
		"agency":     cognitiveStageSchema(),
		"influences": decisionInfluencesSchema(),
	}, []string{"appraisal", "attention", "thought", "desire", "agency", "influences"}, false)
}

func scheduleResponseSchema() map[string]any {
	item := objectSchema(map[string]any{
		"start_at":          stringSchema(),
		"end_at":            stringSchema(),
		"activity":          stringSchema(),
		"scene":             stringSchema(),
		"location":          stringSchema(),
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

func reflectionProposalV2ProviderSchema() map[string]any {
	evidenceRefs := map[string]any{"type": "array", "minItems": 1, "maxItems": 64, "items": map[string]any{"type": "string", "minLength": 1, "maxLength": 256}}
	contextRef := map[string]any{"type": "string", "minLength": 1, "maxLength": maxContextReferenceRunes}
	memoryCandidate := objectSchema(map[string]any{
		"operation": enumStringSchema("create", "confirm", "revise", "merge", "supersede", "deprecate"), "target_ref": contextRef,
		"merge_refs": arraySchema(contextRef), "type": enumStringSchema("episodic", "semantic", "relationship", "autobiographical"),
		"content": stringSchema(), "confidence": unitNumberSchema(), "importance": unitNumberSchema(), "emotional_significance": unitNumberSchema(),
		"evidence_refs": evidenceRefs, "semantic_reason": stringSchema(),
	}, []string{"operation", "confidence", "importance", "emotional_significance", "evidence_refs", "semantic_reason"}, false)
	relationshipObservation := objectSchema(map[string]any{
		"target_ref": contextRef, "observation": stringSchema(), "direction": stringSchema(), "strength": unitNumberSchema(),
		"confidence": unitNumberSchema(), "evidence_refs": evidenceRefs, "semantic_reason": stringSchema(),
	}, []string{"target_ref", "observation", "direction", "strength", "confidence", "evidence_refs", "semantic_reason"}, false)
	goalCandidate := objectSchema(map[string]any{
		"operation": enumStringSchema("create", "update", "pause", "resume", "complete", "abandon", "cancel"), "target_ref": contextRef,
		"desired_outcome": stringSchema(), "success_criteria": arraySchema(stringSchema()), "motivation": stringSchema(), "direction": stringSchema(),
		"strength": unitNumberSchema(), "confidence": unitNumberSchema(), "outcome_refs": arraySchema(contextRef),
		"criterion_indexes": map[string]any{"type": "array", "maxItems": 16, "items": map[string]any{"type": "integer", "minimum": 0}}, "complete": map[string]any{"type": "boolean"},
		"evidence_refs": evidenceRefs, "semantic_reason": stringSchema(),
	}, []string{"operation", "strength", "confidence", "evidence_refs", "semantic_reason"}, false)
	typedTrigger := objectSchema(map[string]any{"type": enumStringSchema("time", "event", "semantic"), "due_at": stringSchema(), "event_type": stringSchema(), "event_ref": contextRef}, []string{"type"}, false)
	intentionCandidate := objectSchema(map[string]any{
		"operation": enumStringSchema("create", "update", "qualify", "pause", "resume", "complete", "expire", "cancel"), "target_ref": contextRef, "goal_ref": contextRef,
		"action_intent": stringSchema(), "expected_outcome": stringSchema(), "capability_constraints": arraySchema(stringSchema()), "typed_trigger": typedTrigger,
		"preferred_time": stringSchema(), "expiration": stringSchema(),
		"confidence": unitNumberSchema(), "evidence_refs": evidenceRefs, "semantic_reason": stringSchema(),
	}, []string{"operation", "goal_ref", "confidence", "evidence_refs", "semantic_reason"}, false)
	emotionalSummary := objectSchema(map[string]any{
		"dominant_patterns": arraySchema(stringSchema()), "triggers": arraySchema(stringSchema()), "recovery_patterns": arraySchema(stringSchema()),
		"conflicts": arraySchema(stringSchema()), "evidence_refs": arraySchema(stringSchema()),
	}, []string{"dominant_patterns", "triggers", "recovery_patterns", "conflicts", "evidence_refs"}, false)
	affectCandidate := objectSchema(map[string]any{
		"target": enumStringSchema("baseline.pleasure", "baseline.arousal", "baseline.dominance", "decay.pad_half_life_seconds", "decay.momentum_half_life_seconds", "decay.mood_half_life_seconds", "decay.drive_half_life_seconds", "regulation.strength"), "direction": enumStringSchema("increase", "decrease", "strengthen", "weaken", "toward", "away", "maintain"), "strength": unitNumberSchema(),
		"confidence": unitNumberSchema(), "evidence_refs": evidenceRefs, "semantic_reason": stringSchema(),
	}, []string{"target", "direction", "strength", "confidence", "evidence_refs", "semantic_reason"}, false)
	slotCandidate := objectSchema(map[string]any{
		"operation": stringSchema(), "target_ref": contextRef, "key": stringSchema(), "semantic_value": stringSchema(), "direction": stringSchema(),
		"strength": unitNumberSchema(), "confidence": unitNumberSchema(), "evidence_refs": evidenceRefs, "semantic_reason": stringSchema(),
	}, []string{"operation", "key", "semantic_value", "direction", "strength", "confidence", "evidence_refs", "semantic_reason"}, false)
	selfCandidate := objectSchema(map[string]any{
		"operation": stringSchema(), "target_ref": contextRef, "category": stringSchema(), "claim": stringSchema(), "confidence": unitNumberSchema(),
		"evidence_refs": evidenceRefs, "semantic_reason": stringSchema(),
	}, []string{"operation", "category", "claim", "confidence", "evidence_refs", "semantic_reason"}, false)
	overlayCandidate := objectSchema(map[string]any{
		"field_path": stringSchema(), "direction": stringSchema(), "semantic_value": stringSchema(), "strength": unitNumberSchema(),
		"confidence": unitNumberSchema(), "evidence_refs": evidenceRefs, "semantic_reason": stringSchema(),
	}, []string{"field_path", "direction", "strength", "confidence", "evidence_refs", "semantic_reason"}, false)
	return objectSchema(map[string]any{
		"schema_version": stringSchema(), "summary": stringSchema(), "memory_candidates": arraySchema(memoryCandidate),
		"relationship_observations": arraySchema(relationshipObservation), "goal_candidates": arraySchema(goalCandidate), "intention_candidates": arraySchema(intentionCandidate),
		"emotional_summary": emotionalSummary, "affect_recalibration_candidates": arraySchema(affectCandidate), "drive_candidates": arraySchema(slotCandidate),
		"preference_candidates": arraySchema(slotCandidate), "trigger_candidates": arraySchema(slotCandidate), "developing_self_candidates": arraySchema(selfCandidate),
		"personality_evolution_candidates": arraySchema(overlayCandidate), "behavior_policy_evolution_candidates": arraySchema(overlayCandidate),
	}, []string{"schema_version", "summary", "memory_candidates", "relationship_observations", "goal_candidates", "intention_candidates", "emotional_summary", "affect_recalibration_candidates", "drive_candidates", "preference_candidates", "trigger_candidates", "developing_self_candidates", "personality_evolution_candidates", "behavior_policy_evolution_candidates"}, false)
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
		"emotional_state":        personalityEmotionalStateSchema(),
		"voice":                  personalityVoiceSchema(),
		"body_language":          personalityBodyLanguageSchema(),
		"behavior_state_machine": personalityStateMachineSchema(),
		"behavior_loops":         personalityBehaviorLoopsSchema(),
		"scenario_behavior":      personalityScenarioBehaviorSchema(),
		"secrets":                personalitySecretsSchema(),
		"intimacy_progression":   personalityIntimacySchema(),
		"output_preferences":     personalityOutputPreferencesSchema(),
		"fears":                  arraySchema(jsonValueSchema()),
		"desires":                arraySchema(jsonValueSchema()),
		"extensions":             openObjectSchema(),
	}, personalityProfileFieldNames(), false)
	personalitySystem := objectSchema(map[string]any{
		"mode":                   enumStringSchema("single", "multiple"),
		"profiles":               arraySchema(personalityProfile),
		"active_profile_id":      stringSchema(),
		"switching":              personalitySwitchingSchema(),
		"influence":              personalityInfluenceSchema(),
		"conflict_resolution":    personalityConflictResolutionSchema(),
		"integration":            personalityIntegrationSchema(),
		"behavior_state_machine": personalityStateMachineSchema(),
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
	}, []string{"schema_version", "identity", "personality", "behavioral_policy", "life_profile", "personality_system"}, false)
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

// personalityProfileFieldNames is the complete known personality contract.
// Unknown future material belongs in extensions rather than becoming an
// accidental second protocol shape.
func personalityProfileFieldNames() []string {
	return []string{
		"id", "name", "identity", "personality", "behavioral_policy", "emotional_state",
		"voice", "body_language", "behavior_state_machine", "behavior_loops", "scenario_behavior",
		"secrets", "intimacy_progression", "output_preferences", "fears", "desires", "extensions",
	}
}

func personalityEmotionalStateSchema() map[string]any {
	return objectSchema(map[string]any{"baseline": jsonValueSchema(), "triggers": arraySchema(jsonValueSchema()), "expression": openObjectSchema(), "regulation": openObjectSchema(), "extensions": openObjectSchema()}, nil, false)
}

func personalityVoiceSchema() map[string]any {
	return objectSchema(map[string]any{"tone": stringSchema(), "pitch": stringSchema(), "speed": numberSchema(), "volume": numberSchema(), "timbre": stringSchema(), "speech_patterns": arraySchema(jsonValueSchema()), "sensory_features": arraySchema(jsonValueSchema()), "extensions": openObjectSchema()}, nil, false)
}

func personalityBodyLanguageSchema() map[string]any {
	return objectSchema(map[string]any{"posture": stringSchema(), "gestures": arraySchema(jsonValueSchema()), "movement_style": stringSchema(), "gaze": stringSchema(), "proximity": stringSchema(), "extensions": openObjectSchema()}, nil, false)
}

func personalityStateMachineSchema() map[string]any {
	return objectSchema(map[string]any{"initial_state": stringSchema(), "states": arraySchema(jsonValueSchema()), "transitions": arraySchema(jsonValueSchema()), "extensions": openObjectSchema()}, nil, false)
}

func personalityBehaviorLoopsSchema() map[string]any {
	return personalityStructuredValueSchema(map[string]any{"loops": arraySchema(jsonValueSchema()), "interruptions": arraySchema(jsonValueSchema()), "extensions": openObjectSchema()})
}

func personalityScenarioBehaviorSchema() map[string]any {
	return objectSchema(map[string]any{"default": jsonValueSchema(), "mappings": openObjectSchema(), "overrides": openObjectSchema(), "extensions": openObjectSchema()}, nil, false)
}

func personalitySecretsSchema() map[string]any {
	return personalityStructuredValueSchema(map[string]any{"items": arraySchema(jsonValueSchema()), "disclosures": arraySchema(jsonValueSchema()), "information_asymmetry": openObjectSchema(), "extensions": openObjectSchema()})
}

func personalityIntimacySchema() map[string]any {
	return personalityStructuredValueSchema(map[string]any{"stage": stringSchema(), "milestones": arraySchema(jsonValueSchema()), "next_steps": arraySchema(jsonValueSchema()), "boundaries": arraySchema(jsonValueSchema()), "extensions": openObjectSchema()})
}

func personalityOutputPreferencesSchema() map[string]any {
	return personalityStructuredValueSchema(map[string]any{"channels": arraySchema(stringSchema()), "image": openObjectSchema(), "voice": openObjectSchema(), "moment": openObjectSchema(), "triggers": arraySchema(jsonValueSchema()), "frequency": jsonValueSchema(), "extensions": openObjectSchema()})
}

func personalitySwitchingSchema() map[string]any {
	rule := objectSchema(map[string]any{"id": stringSchema(), "condition": stringSchema(), "priority": numberSchema(), "target_profile_id": stringSchema(), "cooldown_seconds": numberSchema(), "evidence_refs": arraySchema(stringSchema()), "extensions": openObjectSchema()}, []string{"id", "condition", "target_profile_id"}, false)
	return objectSchema(map[string]any{"rules": arraySchema(rule), "cooldown_seconds": numberSchema(), "default_profile_id": stringSchema(), "extensions": openObjectSchema()}, nil, false)
}

func personalityInfluenceSchema() map[string]any {
	edge := objectSchema(map[string]any{"from_profile_id": stringSchema(), "to_profile_id": stringSchema(), "strength": unitNumberSchema(), "direction": stringSchema(), "condition": stringSchema(), "extensions": openObjectSchema()}, []string{"from_profile_id", "to_profile_id", "strength", "direction"}, false)
	return objectSchema(map[string]any{"edges": arraySchema(edge), "extensions": openObjectSchema()}, nil, false)
}

func personalityConflictResolutionSchema() map[string]any {
	return objectSchema(map[string]any{"strategy": stringSchema(), "priority": arraySchema(stringSchema()), "dominant_profile_id": stringSchema(), "tie_breaker": stringSchema(), "extensions": openObjectSchema()}, nil, false)
}

func personalityIntegrationSchema() map[string]any {
	return objectSchema(map[string]any{"fusion_progress": unitNumberSchema(), "stage": stringSchema(), "shared_memory_policy": stringSchema(), "milestones": arraySchema(jsonValueSchema()), "extensions": openObjectSchema()}, nil, false)
}

func personalityStructuredValueSchema(properties map[string]any) map[string]any {
	return map[string]any{"anyOf": []any{objectSchema(properties, nil, false), arraySchema(openObjectSchema())}}
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
