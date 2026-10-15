package core

// These closed domain errors describe an unresolved request or stale facts.
// Infrastructure/transport errors are deliberately absent. Every write caller
// rolls back before emitting the resulting technical receipt.
var toolSemanticRecovery = map[string]string{
	"memory_target_invalid": "needs_input", "memory_correction_ref_invalid": "needs_input",
	"memory_correction_ref_ambiguous": "needs_input", "memory_correction_ref_not_found": "no_results",
	"memory_correction_ref_stale": "awaiting_state", "memory_command_owner_stale": "awaiting_state",
	"memory_target_revision_conflict": "awaiting_state",
	"active_memory_target_invalid":    "needs_input", "active_memory_context_ref_unknown": "needs_input",
	"active_memory_context_ref_snapshot_invalid": "awaiting_state", "active_memory_target_conversation_invalid": "needs_input",
	"active_memory_owner_stale": "awaiting_state", "active_memory_source_version_stale": "awaiting_state",
	"active_memory_target_revision_conflict": "awaiting_state", "active_memory_scope_invalid": "awaiting_state",
	"active_memory_expire_provider_forbidden": "no_change", "active_memory_confidence_invalid": "needs_input",
	"active_memory_importance_invalid": "needs_input", "active_memory_time_invalid": "needs_input",
	"active_memory_kind_invalid": "needs_input", "active_memory_semantic_invalid": "needs_input",
	"active_memory_time_precision_invalid": "needs_input", "active_memory_timezone_invalid": "needs_input",
	"active_memory_time_expression_invalid": "needs_input", "active_memory_unknown_time_must_be_unresolved": "needs_input",
	"active_memory_time_expression_required": "needs_input", "active_memory_time_range_invalid": "needs_input",
	"active_memory_time_horizon_invalid": "needs_input", "active_memory_time_offset_invalid": "needs_input",
	"activity_selection_required": "needs_input", "activity_already_resolved": "no_change",
	"activity_result_invalid": "awaiting_evidence", "virtual_activity_result_invalid": "awaiting_evidence",
	"scheduled_activity_plan_invalid": "awaiting_state", "schedule_action_invalid": "needs_input",
	"schedule_plan_invalid": "awaiting_state", "schedule_edit_plan_invalid": "awaiting_state",
	"schedule_edit_link_invalid":      "awaiting_state",
	"scene_context_revision_required": "awaiting_state", "scene_runtime_evidence_invalid": "awaiting_state",
	"presence_actor_required": "awaiting_state", "presence_runtime_evidence_invalid": "awaiting_state",
	"presence_context_revision_required": "awaiting_state",
	"affect_arguments_invalid":           "needs_input", "affect_event_required": "needs_input", "affect_type_invalid": "needs_input",
	"affect_confidence_invalid": "needs_input", "affect_evidence_refs_invalid": "awaiting_state", "affect_idempotency_key_invalid": "awaiting_state",
	"capability_request_key_invalid": "needs_input", "capability_request_title_invalid": "needs_input",
	"capability_request_description_invalid": "needs_input", "capability_request_rationale_invalid": "needs_input",
	"capability_request_priority_invalid": "needs_input", "capability_request_side_effect_invalid": "needs_input",
}

// Inspect exact typed codes/plain sentinel leaves, including Go's multi-wrap
// error tree. Never infer a business condition from arbitrary error prose.
func toolSemanticError(err error) (string, string) {
	if err == nil {
		return "", ""
	}
	if many, ok := err.(interface{ Unwrap() []error }); ok {
		for _, child := range many.Unwrap() {
			if code, disposition := toolSemanticError(child); disposition != "" {
				return code, disposition
			}
		}
	}
	if one, ok := err.(interface{ Unwrap() error }); ok {
		if code, disposition := toolSemanticError(one.Unwrap()); disposition != "" {
			return code, disposition
		}
	}
	code, _ := capabilityErrorInfo(err, "", false)
	if disposition := toolSemanticRecovery[code]; disposition != "" {
		return code, disposition
	}
	if disposition := toolSemanticRecovery[err.Error()]; disposition != "" {
		return err.Error(), disposition
	}
	return "", ""
}
