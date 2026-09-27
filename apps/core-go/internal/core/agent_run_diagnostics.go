package core

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

// AgentRunsFiltered is an Owner-only view of logical Agent runs. Physical
// model calls remain separate: a completed Provider call can precede a failed
// Tool or Agent run.
func (a *App) AgentRunsFiltered(ctx context.Context, actorID string, limit int, correlationID string) ([]map[string]any, error) {
	if err := a.requireOwner(ctx, actorID); err != nil {
		return nil, err
	}
	if !validLifecycleIdentity(strings.TrimSpace(correlationID), 128, false) {
		return nil, ErrDiagnosticsFilterInvalid
	}
	if limit < 1 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	query := `SELECT fluctlight_id,agent_id,run_id,correlation_id,status,failure_stage,failure_code,error_detail,started_at,finished_at FROM public.agent_runs`
	args := []any{}
	if correlationID = strings.TrimSpace(correlationID); correlationID != "" {
		args = append(args, correlationID)
		query += fmt.Sprintf(" WHERE correlation_id=$%d", len(args))
	}
	args = append(args, limit)
	query += fmt.Sprintf(" ORDER BY started_at DESC,run_id DESC LIMIT $%d", len(args))
	rows, err := a.DB.Pool().Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]map[string]any, 0)
	for rows.Next() {
		var fluctlightID, agentID, runID, correlation, status, stage, code, detail string
		var started time.Time
		var finished *time.Time
		if err := rows.Scan(&fluctlightID, &agentID, &runID, &correlation, &status, &stage, &code, &detail, &started, &finished); err != nil {
			return nil, err
		}
		associationStatus := "unknown"
		if correlation != "" {
			associationStatus = "linked"
		}
		item := map[string]any{
			"fluctlight_id":      fluctlightID,
			"agent_id":           agentID,
			"run_id":             runID,
			"correlation_id":     correlation,
			"association_status": associationStatus,
			"status":             status,
			"source":             "agent_runs",
			"started_at":         started.Format(time.RFC3339Nano),
		}
		if finished != nil {
			item["finished_at"] = finished.Format(time.RFC3339Nano)
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
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()

	// A conversation can fail before formal admission (for example while
	// assembling the prompt). Its durable inbox termination has no agent_runs
	// row, but the bounded diagnostic event still explains the failure.
	eventQuery := `SELECT e.id,e.fluctlight_id,e.correlation_id,e.payload,e.created_at FROM public.diagnostic_events AS e WHERE e.event_type='agent.run.termination' AND e.payload->>'status'='failed' AND NOT EXISTS (
		SELECT 1 FROM public.agent_runs AS ar
		WHERE ar.correlation_id=e.correlation_id AND ar.fluctlight_id=e.fluctlight_id AND ar.status='failed'
		  AND (e.payload->>'agent_id' IS NULL OR ar.agent_id=e.payload->>'agent_id')
	)`
	eventArgs := []any{}
	if correlationID != "" {
		eventArgs = append(eventArgs, correlationID)
		eventQuery += fmt.Sprintf(" AND e.correlation_id=$%d", len(eventArgs))
	}
	eventArgs = append(eventArgs, limit)
	eventQuery += fmt.Sprintf(" ORDER BY e.created_at DESC,e.id DESC LIMIT $%d", len(eventArgs))
	events, err := a.DB.Pool().Query(ctx, eventQuery, eventArgs...)
	if err != nil {
		return nil, err
	}
	defer events.Close()
	for events.Next() {
		var eventID, correlation string
		var fluctlightID *string
		var payload []byte
		var created time.Time
		if err := events.Scan(&eventID, &fluctlightID, &correlation, &payload, &created); err != nil {
			return nil, err
		}
		fields := decodeObject(payload)
		ownerFluctlightID := ""
		if fluctlightID != nil {
			ownerFluctlightID = *fluctlightID
		}
		stage := stringValue(fields["stage"])
		if stage == "agent_run" || stage == "" {
			stage = firstString(stringValue(fields["failure_stage"]), "unknown")
		}
		associationStatus := "unknown"
		if correlation != "" {
			associationStatus = "linked"
		}
		failureCode := stringValue(fields["reason"])
		if failureCode == "agent_run_failed" || failureCode == "" {
			failureCode = firstString(stringValue(fields["failure_code"]), failureCode)
		}
		item := map[string]any{
			"fluctlight_id":      ownerFluctlightID,
			"agent_id":           firstString(stringValue(fields["agent_id"]), "unknown"),
			"run_id":             "event:" + eventID,
			"correlation_id":     correlation,
			"association_status": associationStatus,
			"status":             "failed",
			"source":             "termination_event",
			"started_at":         created.Format(time.RFC3339Nano),
			"finished_at":        created.Format(time.RFC3339Nano),
			"failure_stage":      stage,
			"failure_code":       safeAgentFailureCode(failureCode, "agent_run_failed"),
		}
		if cause := boundedLifecycleCause(stringValue(fields["safe_cause"])); cause != "" {
			item["safe_cause"] = cause
		}
		result = append(result, item)
	}
	if err := events.Err(); err != nil {
		return nil, err
	}
	sort.SliceStable(result, func(i, j int) bool {
		return stringValue(result[i]["started_at"]) > stringValue(result[j]["started_at"])
	})
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}
