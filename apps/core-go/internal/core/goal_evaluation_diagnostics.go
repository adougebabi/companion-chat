package core

import (
	"context"
	"log/slog"
)

// Model transport status and semantic settlement are independent. This read
// enrichment must never overwrite the first terminal physical model status.
func (a *App) decorateGoalEvaluationSettlement(ctx context.Context, runs []map[string]any) {
	ids := []string{}
	for _, run := range runs {
		if stringValue(run["scenario"]) == "goal_evaluation" {
			ids = append(ids, firstString(run["logical_run_id"], stringValue(run["correlation_id"])))
		}
	}
	ids = sortedUniqueStrings(ids)
	if len(ids) == 0 {
		return
	}
	outcomes := map[string]map[string]any{}
	rows, err := a.DB.Pool().Query(ctx, `SELECT id,status,COALESCE(error_code,''),result FROM public.goal_evaluation_requests WHERE id=ANY($1::text[])`, ids)
	if err == nil {
		for rows.Next() {
			var id, status, code string
			var result []byte
			if scanErr := rows.Scan(&id, &status, &code, &result); scanErr != nil {
				err = scanErr
				break
			}
			outcomes[id] = map[string]any{"request_id": id, "status": status, "error_code": code, "result": decodeObject(result)}
		}
		if err == nil {
			err = rows.Err()
		}
		rows.Close()
	}
	if err != nil {
		slog.Warn("goal_evaluation_diagnostic_lookup_failed")
		for _, id := range ids {
			outcomes[id] = map[string]any{"request_id": id, "status": "unavailable", "error_code": "settlement_status_unavailable"}
		}
	}
	applyGoalEvaluationSettlement(runs, outcomes)
}
func applyGoalEvaluationSettlement(runs []map[string]any, outcomes map[string]map[string]any) {
	for _, run := range runs {
		if stringValue(run["scenario"]) != "goal_evaluation" {
			continue
		}
		id := firstString(run["logical_run_id"], stringValue(run["correlation_id"]))
		if outcome, ok := outcomes[id]; ok {
			run["goal_evaluation"] = outcome
		} else {
			run["goal_evaluation"] = map[string]any{"request_id": id, "status": "unknown", "error_code": "settlement_status_unknown"}
		}
	}
}
