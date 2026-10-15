package core

import "context"

// This is the exact snapshot placed in the initial Planner request by Core,
// not a new read at commit time. Later consumed query receipts take precedence.
type goalPlannerInitialSnapshotKey struct{}
type goalPlannerSnapshotFence struct {
	Version int
	Facts   string
}

func withGoalPlannerInitialSnapshot(ctx context.Context, snapshot map[string]any) context.Context {
	return context.WithValue(ctx, goalPlannerInitialSnapshotKey{}, goalPlannerSnapshotFence{
		Version: intValue(snapshot["revision"]), Facts: stringValue(snapshot["facts_revision"]),
	})
}

// context_review_required is state metadata, never a prerequisite Goal. Do not
// infer completed review from its presence. An all-metadata entry is omitted,
// preserving existing dependencies rather than turning it into a delete request.
// Work on a copy so command identity and callers' input remain unchanged.
func normalizeGoalPlannerDependencies(c GoalSetCommand) (GoalSetCommand, []string) {
	deps := make(map[string][]string, len(c.Dependencies))
	changed := false
	for goal, parents := range c.Dependencies {
		if goal == "context_review_required" {
			changed = true
			continue
		}
		clean := make([]string, 0, len(parents))
		omitted := false
		for _, parent := range parents {
			if parent == "context_review_required" {
				changed, omitted = true, true
				continue
			}
			clean = append(clean, parent)
		}
		if !omitted || len(clean) > 0 {
			deps[goal] = clean
		}
	}
	if !changed {
		return c, nil
	}
	c.Dependencies = deps
	return c, []string{"planner_context_review_flag_ignored"}
}

// Safe, fixed contract guidance: never echo raw values or provisional outputs
// from a rolled-back transaction into a handled receipt.
func goalPlannerCommitFeedback(code string) map[string]any {
	section, detail := "", ""
	switch code {
	case "planner_snapshot_required", "goal_set_version_required", "goal_set_facts_revision_required":
		section, detail = "snapshot", "Use revision and facts_revision from a consumed snapshot. Only missing technical fields can be bound automatically."
	case "goal_set_reason_required":
		detail = "Provide the actual planning or review reason. Review-only commits use reviewed_goal_ids and do not require changes or source."
	case "goal_set_idempotency_key_invalid":
		detail = "Use a nonempty stable idempotency key up to 256 bytes, or omit it for Core binding."
	case "goal_set_request_limits_exceeded":
		detail = "Keep reason within 2000 bytes, changes within 20 entries, and order/dependencies within 40 entries."
	case "goal_set_change_id_invalid":
		detail = "Every create change needs a distinct local id; existing changes use the actual Goal id."
	case "goal_set_change_operation_invalid":
		detail = "Planner changes support create, pause or resume. To retain a valid existing Goal and clear context review, use reviewed_goal_ids without a change."
	case "goal_set_dependency_limit_exceeded":
		detail = "Each dependency list accepts up to 20 real prerequisite Goal ids. context_review_required is a state flag; review uses reviewed_goal_ids."
	case "goal_set_order_incomplete", "goal_set_order_duplicate":
		section, detail = "snapshot", "Order must list every visible active Goal exactly once. Omit order to preserve existing order for a review-only commit."
	case "invalid_arguments", "goal_plan_rejected":
		detail = "Use the published commit schema. A review-only commit needs reason and reviewed_goal_ids; dependencies map actual Goal ids to prerequisite Goal ids."
	}
	if detail == "" {
		return nil
	}
	out := map[string]any{"detail": detail}
	if section != "" {
		out["query_section"] = section
	}
	return out
}

func goalPlannerToolWarnings(results []CapabilityResult) []any {
	warnings := []any{}
	for _, result := range results {
		if result.CapabilityName != goalPlannerCommit {
			continue
		}
		out := mapValue(result.Output)
		if code := stringValue(out["warning_code"]); code != "" {
			warnings = append(warnings, map[string]any{"call_id": result.CallID, "warning_code": code, "disposition": out["disposition"], "effect_applied": false, "mutation_committed": false})
		}
		for _, code := range arrayValue(out["warnings"]) {
			if code == "planner_context_review_flag_ignored" {
				warnings = append(warnings, map[string]any{"call_id": result.CallID, "warning_code": code, "commit_succeeded": out["commit_succeeded"]})
			}
		}
	}
	return warnings
}

func reconcileGoalPlannerHandledSummary(result map[string]any, commits int, results []CapabilityResult) bool {
	decision := stringValue(result["decision"])
	if commits != 0 || (decision != "blocked_by_policy" && decision != "failed") {
		return false
	}
	for _, receipt := range results {
		switch receipt.Status {
		case "failed", "rejected", "cancelled", "timeout":
			return false
		}
	}
	for i := len(results) - 1; i >= 0; i-- {
		receipt := results[i]
		if receipt.CapabilityName != goalPlannerCommit {
			continue
		}
		out := mapValue(receipt.Output)
		code := stringValue(out["warning_code"])
		if receipt.Status != "handled" || code == "" || out["effect_applied"] != false || out["mutation_committed"] != false {
			return false
		}
		result["decision"] = "awaiting_facts"
		result["reason"] = "Planner 提交尚未生效：" + code + "；本次回执为可恢复的输入或状态条件"
		result["review_condition"] = firstString(goalPlannerCommitFeedback(code)["detail"], "按 tool_warnings 中的 warning_code 和 disposition 处理，再使用实际快照提交")
		return true
	}
	return false
}
