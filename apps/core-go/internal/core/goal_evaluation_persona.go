package core

// Goal execution is a Core read model with known nested envelopes. Project
// those envelopes explicitly; values inside their business fields are opaque.
func goalExecutionPromptView(value any) map[string]any {
	source := mapValue(value)
	result := copyPromptFields(source, "stage", "next_step", "next_retry_at", "retry_reason", "due_at", "blocker", "wait_condition", "next_review_at", "ready_for_settlement")
	for key, fields := range map[string][]string{
		"last_attempt":       {"status", "started_at", "settled_at", "deadline", "error_code"},
		"last_result":        {"status", "reason", "occurred_at", "kind", "capability", "error_code"},
		"current_stage":      {"purpose", "strategy", "status"},
		"last_evidence":      {"kind", "status", "reason", "occurred_at"},
		"evaluation_request": {"status", "error_code", "available_at"},
	} {
		if item := copyPromptFields(mapValue(source[key]), fields...); len(item) > 0 {
			result[key] = item
		}
	}
	if rows := arrayValue(source["commitments"]); len(rows) > 0 {
		items := make([]any, 0, len(rows))
		for _, row := range rows {
			items = append(items, copyPromptFields(mapValue(row), "expected_result", "status", "blocker", "window_start", "window_end"))
		}
		result["commitments"] = items
	}
	if governance := mapValue(source["review_governance"]); len(governance) > 0 {
		item := copyPromptFields(governance, "missed_opportunity_count", "ineffective_attempt_count")
		if policy := copyPromptFields(mapValue(governance["policy"]), "missed_opportunity_threshold", "ineffective_attempt_threshold", "reset_after"); len(policy) > 0 {
			item["policy"] = policy
		}
		if len(item) > 0 {
			result["review_governance"] = item
		}
	}
	if assessment := mapValue(source["last_evaluation"]); len(assessment) > 0 {
		item := copyPromptFields(assessment, "impact", "occurred_at")
		if rows := arrayValue(assessment["judgments"]); len(rows) > 0 {
			judgments := make([]any, 0, len(rows))
			for _, row := range rows {
				judgments = append(judgments, copyPromptFields(mapValue(row), "kind", "verdict", "discourse", "subject", "reason"))
			}
			item["judgments"] = judgments
		}
		if len(item) > 0 {
			result["last_evaluation"] = item
		}
	}
	return result
}

// Only the compiled-persona and roster envelopes are Core bookkeeping.
// Portrait strings and values inside the selected persona are opaque data.
func goalEvaluationPersonaSemantics(value any) any {
	result, ok := copyPromptData(value).(map[string]any)
	if !ok {
		return copyPromptData(value)
	}
	for _, key := range []string{workingPersonaProfileIDKey, "source_revision", "source_hash", "source_hash_prefix", "overlay_revision", "rules_version"} {
		delete(result, key)
	}
	if body := mapValue(result[workingPersonaBodyKey]); body != nil {
		for _, key := range []string{"profile_id", "source_revision", "source_hash", "source_hash_prefix", "overlay_revision", "rules_version"} {
			delete(body, key)
		}
	}
	if system := mapValue(result["personality_system"]); system != nil {
		delete(system, "active_profile_id")
		switch profiles := system["profiles"].(type) {
		case []any:
			for _, profile := range profiles {
				deleteGoalPersonaRosterIDs(mapValue(profile))
			}
		case []map[string]any:
			for _, profile := range profiles {
				deleteGoalPersonaRosterIDs(profile)
			}
		}
	}
	return result
}

func deleteGoalPersonaRosterIDs(profile map[string]any) {
	delete(profile, "id")
	delete(profile, "profile_id")
}
