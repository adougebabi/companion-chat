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
	rows, err := a.modelRunsQuery(ctx, actorID, diagnosticPageLimit(limit), correlationID, runID, nil)
	for _, row := range rows {
		delete(row, "_cursor_time")
		delete(row, "_cursor_key")
	}
	return rows, err
}
func (a *App) modelRunsQuery(ctx context.Context, actorID string, limit int, correlationID, runID string, cursor *diagnosticPageCursor) ([]map[string]any, error) {
	if err := a.requireOwner(ctx, actorID); err != nil {
		return nil, err
	}
	if limit < 1 {
		limit = 100
	}
	if limit > 501 {
		limit = 501
	}
	query := `WITH candidates AS (
		SELECT *, COUNT(*) FILTER (WHERE COALESCE(metrics->>'model_call_id','') <> '') OVER (PARTITION BY COALESCE(NULLIF(metrics->>'run_id',''),correlation_id)) AS adk_physical_count
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
	if cursor != nil {
		args = append(args, cursor.Snapshot)
		where = append(where, fmt.Sprintf("pg_visible_in_snapshot(recorded_xid,$%d::pg_snapshot)", len(args)))
	}
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += `
	), runs AS (
		SELECT id,role,binding_role,scenario,priority,endpoint_id,model_id,prompt,response,status,error_code,correlation_id,created_at,queued_at,started_at,completed_at,metrics,
			COUNT(*) OVER (PARTITION BY COALESCE(NULLIF(metrics->>'run_id',''),correlation_id)) AS round_count,
			ROW_NUMBER() OVER (PARTITION BY COALESCE(NULLIF(metrics->>'run_id',''),correlation_id) ORDER BY queued_at ASC,id ASC) AS physical_sequence,
			COUNT(*) FILTER (WHERE status IN ('queued','running')) OVER (PARTITION BY binding_role) AS queue_pending_count,
			CASE WHEN status IN ('queued','running') THEN ROW_NUMBER() OVER (
				PARTITION BY binding_role
				ORDER BY CASE WHEN status IN ('queued','running') THEN 0 ELSE 1 END, priority DESC, queued_at ASC, id ASC
			) END AS queue_position
		FROM candidates
		WHERE NOT (adk_physical_count > 0 AND COALESCE(metrics->>'model_call_id','') = '' AND COALESCE(metrics->>'provider_attempt_id','') NOT IN ('','legacy'))`
	query += `
 )
 SELECT id,role,binding_role,scenario,priority,endpoint_id,model_id,prompt,response,status,error_code,correlation_id,created_at,queued_at,started_at,completed_at,metrics,round_count,physical_sequence,queue_pending_count,queue_position
 FROM runs WHERE true`
	query, args = appendDiagnosticCursorConditions(query, args, cursor, "created_at", "id")
	args = append(args, limit)
	query += fmt.Sprintf(" ORDER BY created_at DESC NULLS LAST,id DESC LIMIT $%d", len(args))
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
		var metrics []byte
		var roundCount, physicalSequence int64
		var queued time.Time
		var created *time.Time
		var started, completed *time.Time
		if err := rows.Scan(&id, &role, &bindingRole, &scenario, &priority, &endpoint, &model, &prompt, &response, &status, &code, &corr, &created, &queued, &started, &completed, &metrics, &roundCount, &physicalSequence, &queuePendingCount, &queuePosition); err != nil {
			return nil, err
		}
		metricValues := decodeObject(metrics)
		logicalID := stringValue(metricValues["run_id"])
		if logicalID == "" {
			logicalID = corr
		}
		var safePrompt, safeResponse any
		if len(prompt) > 0 {
			_ = json.Unmarshal(prompt, &safePrompt)
		}
		if len(response) > 0 {
			_ = json.Unmarshal(response, &safeResponse)
		}
		row := map[string]any{"id": id, "role": role, "binding_role": bindingRole, "scenario": scenario, "priority": priority, "endpoint_id": endpoint, "model_id": model, "prompt": redactDiagnostic(safePrompt), "response": redactDiagnostic(safeResponse), "status": status, "error_code": code, "correlation_id": corr, "created_at": "", "queued_at": formatInstant(queued), "ordering_key": "0|" + id, "_cursor_key": id, "logical_run_id": logicalID, "model_call_id": stringValue(metricValues["model_call_id"]), "round_count": roundCount, "stage": "unknown", "tool_summaries": []any{}}
		if created != nil {
			row["created_at"] = formatInstant(*created)
			row["ordering_key"] = created.UTC().Format("2006-01-02T15:04:05.000000000") + "|" + id
			row["_cursor_time"] = created.UTC().Format(time.RFC3339Nano)
		} else {
			row["_cursor_null"] = true
		}
		preflight := stringValue(mapValue(safeResponse)["stage"]) != ""
		if attemptID := stringValue(metricValues["provider_attempt_id"]); attemptID != "" && attemptID != "legacy" && stringValue(metricValues["model_call_id"]) == "" && !preflight {
			row["sequence"] = int(physicalSequence)
			row["stage"] = nonADKModelRunStage(status, safeResponse)
		} else if preflight {
			row["stage"] = status
		}
		if queuePendingCount != nil {
			row["queue_pending_count"] = *queuePendingCount
		}
		if queuePosition != nil {
			row["queue_position"] = *queuePosition
		}
		if started != nil {
			row["started_at"] = formatInstant(*started)
		}
		if completed != nil {
			row["completed_at"] = formatInstant(*completed)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	if err := a.decorateModelRunRounds(ctx, out); err != nil {
		return nil, err
	}
	return out, nil
}

func nonADKModelRunStage(status string, response any) string {
	switch status {
	case "failed", "cancelled", "timeout":
		return status
	case "queued", "running":
		return "pending"
	case "completed":
		if response != nil {
			return "final_response"
		}
	}
	return "unknown"
}

func (a *App) decorateModelRunRounds(ctx context.Context, runs []map[string]any) error {
	byCall := make(map[string]map[string]any, len(runs))
	callIDs := make([]string, 0, len(runs))
	for _, run := range runs {
		callID := stringValue(run["model_call_id"])
		if callID == "" {
			continue
		}
		byCall[callID] = run
		callIDs = append(callIDs, callID)
	}
	if len(callIDs) == 0 {
		return nil
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT event_type,payload FROM public.diagnostic_events WHERE event_type IN ('adk.model.input','adk.model.output','adk.tool.requested','adk.tool.dispatched','adk.tool.rejected','adk.tool.result') AND payload->>'model_call_id'=ANY($1::text[]) ORDER BY created_at,id LIMIT 4096`, callIDs)
	if err != nil {
		return err
	}
	defer rows.Close()
	toolsByCall := make(map[string]map[string]map[string]any, len(callIDs))
	toolOrder := make(map[string][]string, len(callIDs))
	for rows.Next() {
		var eventType string
		var raw []byte
		if err := rows.Scan(&eventType, &raw); err != nil {
			return err
		}
		payload := decodeObject(raw)
		callID := stringValue(payload["model_call_id"])
		run := byCall[callID]
		if run == nil {
			continue
		}
		if sequence := intValue(payload["sequence"]); sequence > 0 && (eventType == "adk.model.input" || eventType == "adk.model.output") {
			run["sequence"] = sequence
		}
		switch eventType {
		case "adk.model.output":
			calls := arrayValue(payload["tool_call_ids"])
			if status := stringValue(run["status"]); status == "failed" || status == "cancelled" || status == "timeout" {
				run["stage"] = status
			} else if len(calls) > 0 {
				run["stage"] = "tool_request"
			} else if stringValue(run["status"]) == "completed" && stringValue(payload["status"]) == "completed" {
				run["stage"] = "final_response"
			}
		case "adk.tool.requested", "adk.tool.dispatched", "adk.tool.rejected", "adk.tool.result":
			toolID := stringValue(payload["call_id"])
			if toolID == "" {
				continue
			}
			if toolsByCall[callID] == nil {
				toolsByCall[callID] = make(map[string]map[string]any)
			}
			if toolsByCall[callID][toolID] == nil {
				if len(toolOrder[callID]) >= 32 {
					continue
				}
				toolOrder[callID] = append(toolOrder[callID], toolID)
				toolsByCall[callID][toolID] = map[string]any{"call_id": toolID}
			}
			tool := toolsByCall[callID][toolID]
			tool["capability"] = stringValue(payload["capability"])
			tool["status"] = stringValue(payload["status"])
			if code := stringValue(payload["error_code"]); code != "" {
				tool["error_code"] = code
			}
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for callID, run := range byCall {
		if status := stringValue(run["status"]); status == "failed" || status == "cancelled" || status == "timeout" {
			run["stage"] = status
		} else if status == "queued" || status == "running" {
			run["stage"] = "pending"
		}
		summaries := make([]map[string]any, 0, len(toolOrder[callID]))
		for _, toolID := range toolOrder[callID] {
			summaries = append(summaries, toolsByCall[callID][toolID])
		}
		run["tool_summaries"] = summaries
	}
	return nil
}
