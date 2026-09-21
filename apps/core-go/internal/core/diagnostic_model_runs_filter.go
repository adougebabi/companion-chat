package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// modelRunsFiltered is the internal run_id-aware variant used by the owner
// export. The ordinary ModelRuns API deliberately keeps its small projection;
// run_id matching is only an export correlation concern.
func (a *App) modelRunsFiltered(ctx context.Context, actorID string, limit int, correlationID, runID string) ([]map[string]any, error) {
	if err := a.requireOwner(ctx, actorID); err != nil {
		return nil, err
	}
	if limit < 1 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	query := `WITH runs AS (
		SELECT id,role,binding_role,scenario,priority,endpoint_id,model_id,prompt,response,status,error_code,correlation_id,created_at,queued_at,started_at,completed_at,
			COUNT(*) FILTER (WHERE status IN ('queued','running')) OVER (PARTITION BY binding_role) AS queue_pending_count,
			CASE WHEN status IN ('queued','running') THEN ROW_NUMBER() OVER (
				PARTITION BY binding_role
				ORDER BY CASE WHEN status IN ('queued','running') THEN 0 ELSE 1 END, priority DESC, queued_at ASC, id ASC
			) END AS queue_position
		FROM public.diagnostic_model_runs`
	args := []any{}
	where := make([]string, 0, 2)
	if strings.TrimSpace(correlationID) != "" {
		args = append(args, strings.TrimSpace(correlationID))
		where = append(where, fmt.Sprintf("correlation_id=$%d", len(args)))
	}
	if strings.TrimSpace(runID) != "" {
		args = append(args, strings.TrimSpace(runID))
		where = append(where, fmt.Sprintf("metrics->>'run_id'=$%d", len(args)))
	}
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	args = append(args, limit)
	query += fmt.Sprintf(`
	)
	SELECT id,role,binding_role,scenario,priority,endpoint_id,model_id,prompt,response,status,error_code,correlation_id,created_at,queued_at,started_at,completed_at,queue_pending_count,queue_position
	FROM runs
	ORDER BY CASE WHEN status IN ('queued','running') THEN 0 ELSE 1 END,
		CASE WHEN status IN ('queued','running') THEN priority END DESC NULLS LAST,
		CASE WHEN status IN ('queued','running') THEN queued_at END ASC NULLS LAST,
		CASE WHEN status NOT IN ('queued','running') THEN completed_at END DESC NULLS LAST,
		created_at DESC,id DESC
	LIMIT $%d`, len(args))
	rows, err := a.DB.Pool().Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]map[string]any, 0)
	for rows.Next() {
		var id, role, bindingRole, scenario, model, status, corr string
		var priority int
		var queuePendingCount, queuePosition *int64
		var endpoint, code *string
		var prompt, response []byte
		var created, queued time.Time
		var started, completed *time.Time
		if err := rows.Scan(&id, &role, &bindingRole, &scenario, &priority, &endpoint, &model, &prompt, &response, &status, &code, &corr, &created, &queued, &started, &completed, &queuePendingCount, &queuePosition); err != nil {
			return nil, err
		}
		row := map[string]any{"id": id, "role": role, "binding_role": bindingRole, "scenario": scenario, "priority": priority, "endpoint_id": endpoint, "model_id": model, "prompt": json.RawMessage(prompt), "response": json.RawMessage(response), "status": status, "error_code": code, "correlation_id": corr, "created_at": created.Format(time.RFC3339Nano), "queued_at": queued.Format(time.RFC3339Nano)}
		if queuePendingCount != nil {
			row["queue_pending_count"] = *queuePendingCount
		}
		if queuePosition != nil {
			row["queue_position"] = *queuePosition
		}
		if started != nil {
			row["started_at"] = started.Format(time.RFC3339Nano)
		}
		if completed != nil {
			row["completed_at"] = completed.Format(time.RFC3339Nano)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
