package core

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// Logical Agent records and pre-admission terminations share one SQL sort and
// one cursor. Physical model rounds remain a separate ordered detail stream.
func (a *App) AgentRunsFiltered(ctx context.Context, actorID string, limit int, correlationID string) ([]map[string]any, error) {
	page, err := a.AgentRunsPage(ctx, actorID, limit, correlationID, "")
	return page.Items, err
}
func (a *App) agentRunsQuery(ctx context.Context, limit int, correlationID string, cursor *diagnosticPageCursor) ([]map[string]any, error) {
	if !validLifecycleIdentity(strings.TrimSpace(correlationID), 128, false) {
		return nil, ErrDiagnosticsFilterInvalid
	}
	args := []any{correlationID}
	birthCondition, parentCondition := "", ""
	if cursor != nil {
		args = append(args, cursor.Snapshot)
		birthCondition = " AND pg_visible_in_snapshot(recorded_xid,$2::pg_snapshot)"
		parentCondition = " AND pg_visible_in_snapshot(ar.recorded_xid,$2::pg_snapshot)"
	}
	// Formal completion can precede a failed business settlement. Preserve that
	// termination alongside the completed formal run; an already failed run
	// carries its own failure and does not need a duplicate event identity.
	query := `WITH combined AS (
 SELECT fluctlight_id,agent_id,run_id,correlation_id,status,failure_stage,failure_code,error_detail,
 started_at AS recorded_at,finished_at,'agent_runs'::text AS source,
 concat_ws(':','run',fluctlight_id,agent_id,run_id) AS sort_key
 FROM public.agent_runs WHERE ($1='' OR correlation_id=$1)` + birthCondition + `
 UNION ALL
 SELECT COALESCE(e.fluctlight_id,''),COALESCE(e.payload->>'agent_id','unknown'),'event:'||e.id,e.correlation_id,'failed',
 COALESCE(NULLIF(e.payload->>'failure_stage',''),NULLIF(e.payload->>'stage',''),'unknown'),
 COALESCE(NULLIF(e.payload->>'failure_code',''),e.payload->>'reason','agent_run_failed'),COALESCE(e.payload->>'safe_cause',''),
 e.created_at,e.created_at,'termination_event','event:'||e.id
 FROM public.diagnostic_events e WHERE e.event_type='agent.run.termination' AND e.payload->>'status'='failed'
 AND ($1='' OR e.correlation_id=$1)` + strings.ReplaceAll(birthCondition, "recorded_xid", "e.recorded_xid") + `
 AND NOT EXISTS(SELECT 1 FROM public.agent_runs ar WHERE ar.correlation_id=e.correlation_id AND ar.fluctlight_id=e.fluctlight_id AND ar.status='failed'
 AND (e.payload->>'agent_id' IS NULL OR ar.agent_id=e.payload->>'agent_id')` + parentCondition + `)
 ) SELECT fluctlight_id,agent_id,run_id,correlation_id,status,failure_stage,failure_code,error_detail,recorded_at,finished_at,source,sort_key
 FROM combined WHERE true`
	query, args = appendDiagnosticCursorConditions(query, args, cursor, "recorded_at", "sort_key")
	args = append(args, limit)
	query += fmt.Sprintf(" ORDER BY recorded_at DESC NULLS LAST,sort_key DESC LIMIT $%d", len(args))
	rows, err := a.DB.Pool().Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]map[string]any, 0)
	for rows.Next() {
		var owner, agent, run, correlation, status, stage, code, detail, source, key string
		var started, finished *time.Time
		if err := rows.Scan(&owner, &agent, &run, &correlation, &status, &stage, &code, &detail, &started, &finished, &source, &key); err != nil {
			return nil, err
		}
		association := "unknown"
		if correlation != "" {
			association = "linked"
		}
		item := map[string]any{"fluctlight_id": owner, "agent_id": agent, "run_id": run, "correlation_id": correlation, "association_status": association, "status": status, "source": source, "_cursor_key": key, "started_at": "", "ordering_key": "0|" + key}
		if started != nil {
			item["started_at"] = formatInstant(*started)
			item["_cursor_time"] = started.UTC().Format(time.RFC3339Nano)
			item["ordering_key"] = started.UTC().Format("2006-01-02T15:04:05.000000000") + "|" + key
		} else {
			item["_cursor_null"] = true
		}
		if finished != nil {
			item["finished_at"] = formatInstant(*finished)
		}
		if status == "failed" {
			item["failure_stage"] = firstString(stage, "unknown")
			item["failure_code"] = safeAgentFailureCode(code, "agent_run_failed")
			if cause := boundedLifecycleCause(detail); cause != "" {
				item["safe_cause"] = cause
			}
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
