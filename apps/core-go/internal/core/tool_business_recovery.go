package core

import (
	"context"
	"errors"
	"log/slog"
	"net"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/capability"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// These are domain preconditions, never infrastructure failure text. Callers
// must be read-only, in Prepare, or roll back the business savepoint first.
func recoverToolBusinessResult(inv CapabilityInvocation, result CapabilityResult, err error) (CapabilityResult, bool) {
	var databaseError *pgconn.PgError
	var networkError net.Error
	if errors.As(err, &databaseError) || errors.As(err, &networkError) || errors.Is(err, pgx.ErrTxClosed) || errors.Is(err, pgx.ErrTxCommitRollback) {
		return result, false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, ErrUnauthorized) {
		return result, false
	}
	if result.Status == "completed" || result.Status == "accepted" || result.Status == "handled" {
		return result, false
	}
	code := result.ErrorCode
	if code == "" {
		code, _ = capabilityErrorInfo(err, "", false)
	}
	disposition := ""
	switch {
	case errors.Is(err, ErrInvalidArguments), errors.Is(err, capability.ErrInvalidArguments):
		disposition = "needs_input"
	case errors.Is(err, ErrNotFound):
		disposition = "no_results"
	case errors.Is(err, pgx.ErrNoRows):
		disposition = "no_results"
		if result.Retryable {
			disposition = "unavailable"
		}
	case errors.Is(err, ErrConflict), errors.Is(err, ErrLifeContextStale), errors.Is(err, ErrFoundationRevisionStale), errors.Is(err, ErrCurrentFactsStale), errors.Is(err, ErrCurrentStateRevisionStale):
		disposition = "awaiting_state"
	}
	typedCode, _ := capabilityErrorInfo(err, "", false)
	if value := toolRecoveryCodes[code]; value != "" && (err == nil || err.Error() == code || typedCode == code) {
		disposition = value
	}
	if recoveredCode, recoveredDisposition := toolSemanticError(err); recoveredDisposition != "" {
		code, disposition = recoveredCode, recoveredDisposition
	}
	if err == nil && toolSemanticRecovery[code] != "" {
		disposition = toolSemanticRecovery[code]
	}
	if disposition == "" {
		return result, false
	}
	if code == "" {
		code = "tool_precondition_unresolved"
	}
	// Do not carry a provisional success payload out of a rolled-back operation.
	staleReply := inv.CapabilityName == conversationReplyCapabilityName && (errors.Is(err, ErrLifeContextStale) || errors.Is(err, ErrCurrentFactsStale) || errors.Is(err, ErrFoundationRevisionStale) || errors.Is(err, ErrCurrentStateRevisionStale))
	output := map[string]any{"retry_same_arguments": false, "refresh_context": disposition == "awaiting_state", "refresh_reply": staleReply}
	if staleReply {
		output["recovery_anchor"] = "stale_reply"
	}
	if code == "reply_control_value_invalid" {
		output["detail"] = "Control text was ignored; no message published. Use response_intent for the final silent decision."
	}
	return handledCapabilityResult(inv, code, disposition, output), true
}

var toolRecoveryCodes = map[string]string{
	"reply_control_value_invalid": "no_change",
	"life_state_sleeping":         "awaiting_state", "goal_terminal": "no_change",
	"goal_submission_parent_terminal": "no_change", "goal_submission_parent_not_active": "no_change",
	"goal_evaluation_wire_goal_ref_invalid": "needs_input", "goal_evaluation_wire_object_ref_invalid": "needs_input",
	"goal_evaluation_wire_criterion_ref_invalid": "needs_input", "goal_evaluation_wire_output_invalid": "needs_input",
	"goal_evaluation_unknown_requires_evidence": "awaiting_evidence", "goal_assessment_next_step_missing": "awaiting_evidence",
	"goal_review_adjust_plan_required": "awaiting_state", "goal_review_decision_required": "awaiting_evidence",
	"habit_already_present": "no_change", "habit_unchanged": "no_change",
	"relationship_lookup_target_required": "needs_input", "relationship_lookup_not_found": "no_results",
	"persona_detail_source_changed": "awaiting_state", "persona_detail_source_not_found": "no_results",
	"scene_event_not_found": "no_results", "scene_event_not_active": "no_change",
	"schedule_item_selection_required": "needs_input", "schedule_item_not_found": "no_results",
	"schedule_cursor_invalid": "no_results", "wardrobe_no_change_requested": "no_change",
	"item_use_activity_required": "needs_input", "presence_ttl_invalid": "needs_input",
}

func recordToolRecovery(ctx context.Context, inv CapabilityInvocation, result CapabilityResult) {
	if result.Status != "handled" {
		return
	}
	out := mapValue(result.Output)
	slog.WarnContext(ctx, "tool.call.handled", "capability", inv.CapabilityName, "call_id", inv.CallID,
		"warning_code", out["warning_code"], "disposition", out["disposition"], "effect_applied", false)
}
