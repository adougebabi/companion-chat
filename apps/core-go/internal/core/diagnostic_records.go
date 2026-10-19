package core

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

var ownerDiagnosticKinds = []string{"events", "lifecycle", "model-runs", "agent-runs", "media-prompts", "kev", "workflow-intents", "audit", "harm", "media-recoveries", "turns", "links"}

type ownerDiagnosticFilter map[string]string

func parseOwnerDiagnosticFilter(q url.Values) (ownerDiagnosticFilter, error) {
	f := ownerDiagnosticFilter{}
	for _, key := range []string{"fluctlightId", "correlationId", "runId", "intentId", "workflowId", "surface", "status", "eventType", "severity", "role", "identity", "action", "from", "to", "q"} {
		value := strings.TrimSpace(q.Get(key))
		if len([]rune(value)) > 256 {
			return nil, ErrDiagnosticsFilterInvalid
		}
		if value != "" {
			f[key] = value
		}
	}
	for _, key := range []string{"from", "to"} {
		if f[key] != "" {
			if _, err := time.Parse(time.RFC3339Nano, f[key]); err != nil {
				return nil, ErrDiagnosticsFilterInvalid
			}
		}
	}
	if f["from"] != "" && f["to"] != "" {
		start, _ := time.Parse(time.RFC3339Nano, f["from"])
		end, _ := time.Parse(time.RFC3339Nano, f["to"])
		if end.Before(start) {
			return nil, ErrDiagnosticsFilterInvalid
		}
	}
	return f, nil
}

// Every source uses its actual primary identity. The optional raw payload is
// Owner diagnostics, never a product state or a reconstruction of missing facts.
func ownerDiagnosticSource(kind string) (string, error) {
	return ownerDiagnosticSourceSnapshot(kind, "")
}
func ownerDiagnosticSourceSnapshot(kind, visible string) (string, error) {
	modelMembership, agentMembership := "", ""
	if visible != "" {
		modelMembership = " WHERE pg_visible_in_snapshot(t.recorded_xid," + visible + "::pg_snapshot)"
		agentMembership = " AND pg_visible_in_snapshot(ar.recorded_xid," + visible + "::pg_snapshot)"
	}
	switch kind {
	case "events", "lifecycle":
		where := ""
		if kind == "lifecycle" {
			where = " AND t.event_type LIKE 'lifecycle.%'"
		}
		return `SELECT t.id AS key,t.created_at AS at,t.recorded_xid AS born,to_jsonb(t)-'recorded_xid' AS data FROM public.diagnostic_events t WHERE $1::text<>''` + where, nil
	case "model-runs":
		return `WITH candidates AS (SELECT t.*,COUNT(*)FILTER(WHERE COALESCE(metrics->>'model_call_id','')<>'') OVER(PARTITION BY COALESCE(NULLIF(metrics->>'run_id',''),correlation_id)) AS physical_count FROM public.diagnostic_model_runs t` + modelMembership + `), visible AS (SELECT *,COUNT(*) OVER(PARTITION BY COALESCE(NULLIF(metrics->>'run_id',''),correlation_id)) AS round_count,ROW_NUMBER() OVER(PARTITION BY COALESCE(NULLIF(metrics->>'run_id',''),correlation_id) ORDER BY queued_at,id) AS physical_sequence FROM candidates WHERE NOT(physical_count>0 AND COALESCE(metrics->>'model_call_id','')='' AND COALESCE(metrics->>'provider_attempt_id','') NOT IN ('','legacy'))) SELECT t.id AS key,t.created_at AS at,t.recorded_xid AS born,to_jsonb(t)-'recorded_xid'-'physical_count' AS data FROM visible t WHERE $1::text<>''`, nil
	case "agent-runs":
		return `SELECT jsonb_build_array('run',t.fluctlight_id,t.agent_id,t.run_id)::text AS key,t.started_at AS at,t.recorded_xid AS born,(to_jsonb(t)-'recorded_xid')||jsonb_build_object('source','agent_runs') AS data FROM public.agent_runs t WHERE $1::text<>'' UNION ALL SELECT jsonb_build_array('event',e.id)::text,e.created_at,e.recorded_xid,(to_jsonb(e)-'recorded_xid')||jsonb_build_object('source','termination_event','status','failed','run_id','event:'||e.id,'agent_id',e.payload->>'agent_id','failure_code',e.payload->>'failure_code','failure_stage',e.payload->>'failure_stage') FROM public.diagnostic_events e WHERE $1::text<>'' AND e.event_type='agent.run.termination' AND e.payload->>'status'='failed' AND NOT EXISTS(SELECT 1 FROM public.agent_runs ar WHERE ar.correlation_id=e.correlation_id AND ar.fluctlight_id=e.fluctlight_id AND ar.status='failed'` + agentMembership + ` AND (e.payload->>'agent_id' IS NULL OR ar.agent_id=e.payload->>'agent_id'))`, nil
	case "media-prompts":
		return `SELECT t.id AS key,t.created_at AS at,t.recorded_xid AS born,((to_jsonb(t)-'recorded_xid')||jsonb_build_object('fluctlight_id',t.owner_fluctlight_id,'correlation_id','media:'||t.id,'submission',(SELECT jsonb_build_object('event_id',e.id,'submitted_prompt',e.payload->'prompt','request_payload',e.payload->'request_payload','submitted_at',e.created_at) FROM public.diagnostic_events e WHERE e.event_type='media.comfyui.prompt_submitted' AND e.correlation_id='media:'||t.id ORDER BY e.created_at DESC,e.id DESC LIMIT 1),'execution',(SELECT to_jsonb(w) FROM public.platform_workflow_intents w WHERE w.workflow_id=t.workflow_id AND w.intent_type='media.generation'))) AS data FROM public.media_intents t WHERE $1::text<>''`, nil
	case "media-recoveries":
		return `SELECT t.id AS key,t.created_at AS at,t.recorded_xid AS born,(to_jsonb(t)-'recorded_xid')||jsonb_build_object('fluctlight_id',t.previous->>'owner_fluctlight_id','status',w.status,'attempt_count',w.attempt_count,'last_error',CASE WHEN w.last_error IS NULL THEN NULL ELSE 'media_workflow_failed' END) AS data FROM public.media_recovery_attempts t JOIN public.platform_workflow_intents w ON w.intent_id=t.intent_id WHERE t.owner_actor_id=$1`, nil
	case "workflow-intents":
		return `SELECT t.intent_id AS key,t.created_at AS at,t.recorded_xid AS born,to_jsonb(t)-'recorded_xid' AS data FROM public.platform_workflow_intents t WHERE $1::text<>''`, nil
	case "kev":
		return `SELECT t.id AS key,t.started_at AS at,t.recorded_xid AS born,(t.payload||jsonb_build_object('id',t.id,'fluctlight_id',t.actor_self,'run_id',t.run_id,'correlation_id',t.payload->'scope'->>'correlation_id')) AS data FROM public.kev_decisions t WHERE $1::text<>''`, nil
	case "audit":
		return `SELECT 'owner:'||t.id AS key,t.created_at AS at,t.recorded_xid AS born,(to_jsonb(t)-'recorded_xid')||jsonb_build_object('source','owner_surface_audits') AS data FROM public.owner_surface_audits t WHERE t.owner_actor_id=$1 UNION ALL SELECT 'workflow:'||t.id,t.created_at,t.recorded_xid,(to_jsonb(t)-'recorded_xid')||jsonb_build_object('source','workflow_management_audit') FROM public.platform_workflow_management_audit t WHERE t.actor_id=$1`, nil
	case "harm":
		return `SELECT t.id AS key,t.created_at AS at,t.recorded_xid AS born,to_jsonb(t)-'recorded_xid' AS data FROM public.permanent_harm_rejections t WHERE t.owner_actor_id=$1`, nil
	case "turns":
		return `SELECT t.id AS key,t.created_at AS at,t.recorded_xid AS born,to_jsonb(t)-'recorded_xid' AS data FROM public.diagnostic_turns t WHERE $1::text<>''`, nil
	case "links":
		return `SELECT t.id AS key,t.created_at AS at,t.recorded_xid AS born,to_jsonb(t)-'recorded_xid' AS data FROM public.diagnostic_workflow_links t WHERE $1::text<>''`, nil
	}
	return "", ErrNotFound
}

func appendOwnerDiagnosticFilter(sql string, args []any, f ownerDiagnosticFilter) (string, []any) {
	expressions := map[string]string{
		"fluctlightId":  "COALESCE(data->>'fluctlight_id',data->'payload'->>'fluctlight_id',data->'metrics'->>'fluctlight_id')",
		"correlationId": "COALESCE(data->>'correlation_id',data->'payload'->>'correlation_id',data->'scope'->>'correlation_id')",
		"runId":         "COALESCE(data->>'run_id',data->'metrics'->>'run_id',data->'payload'->>'run_id')",
		"intentId":      "COALESCE(data->>'intent_id',data->'payload'->>'intent_id',data->'metrics'->>'intent_id')",
		"workflowId":    "COALESCE(data->>'workflow_id',data->'payload'->>'workflow_id',data->'metrics'->>'workflow_id')",
		"surface":       "COALESCE(data->>'surface',data->'payload'->>'surface',data->'metrics'->>'surface')",
		"status":        "COALESCE(data->>'status',data->'payload'->>'status',data->>'application_status')",
		"identity":      "data->>'identity'", "action": "data->>'action'",
		"eventType": "data->>'event_type'", "severity": "data->>'severity'", "role": "COALESCE(data->>'role',data->>'binding_role')",
	}
	for _, key := range []string{"fluctlightId", "correlationId", "runId", "intentId", "workflowId", "surface", "status", "eventType", "severity", "role", "identity", "action"} {
		if f[key] != "" {
			args = append(args, f[key])
			sql += fmt.Sprintf(" AND %s=$%d::text", expressions[key], len(args))
		}
	}
	for _, key := range []string{"from", "to"} {
		if f[key] != "" {
			op := ">="
			if key == "to" {
				op = "<="
			}
			args = append(args, f[key])
			sql += fmt.Sprintf(" AND at%s$%d::timestamptz", op, len(args))
		}
	}
	if f["q"] != "" {
		args = append(args, f["q"])
		sql += fmt.Sprintf(" AND strpos(lower(data::text),lower($%d::text))>0", len(args))
	}
	return sql, args
}
func diagnosticRecordRef(kind, key string) string {
	return base64.RawURLEncoding.EncodeToString(jsonBytes([]string{kind, key}))
}
func decodeDiagnosticRecordRef(kind, raw string) (string, error) {
	if len(raw) > 4096 {
		return "", ErrDiagnosticsFilterInvalid
	}
	b, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return "", ErrDiagnosticsFilterInvalid
	}
	var ref []string
	if json.Unmarshal(b, &ref) != nil || len(ref) != 2 || ref[0] != kind || len(ref[1]) > 2048 {
		return "", ErrDiagnosticsFilterInvalid
	}
	return ref[1], nil
}
func diagnosticRecord(kind, key string, at *time.Time, data map[string]any) map[string]any {
	ref := diagnosticRecordRef(kind, key)
	record := map[string]any{"id": key, "ref": ref, "kind": kind, "data": redactDiagnostic(data), "occurred_at": nil}
	if at != nil {
		record["occurred_at"] = at.UTC().Format(time.RFC3339Nano)
		record["_cursor_time"] = at.UTC().Format(time.RFC3339Nano)
	} else {
		record["_cursor_null"] = true
	}
	record["_cursor_key"] = key
	return record
}

func (a *App) OwnerDiagnosticRecords(ctx context.Context, owner, kind string, q url.Values) (map[string]any, error) {
	if err := a.requireOwner(ctx, owner); err != nil {
		return nil, err
	}
	if !containsString(ownerDiagnosticKinds, kind) {
		return nil, ErrNotFound
	}
	f, err := parseOwnerDiagnosticFilter(q)
	if err != nil {
		return nil, err
	}
	limit := 30
	if q.Get("limit") != "" {
		limit, err = strconv.Atoi(q.Get("limit"))
		if err != nil || limit < 1 || limit > 100 {
			return nil, ErrDiagnosticsFilterInvalid
		}
	}
	cursor, err := a.diagnosticCursor(ctx, owner, "records:"+kind, stableDigest(jsonString(f)), q.Get("cursor"))
	if err != nil {
		return nil, err
	}
	source, err := ownerDiagnosticSourceSnapshot(kind, "$2")
	if err != nil {
		return nil, err
	}
	sql, args := appendOwnerDiagnosticFilter("WITH source AS ("+source+") SELECT key,at,data FROM source WHERE pg_visible_in_snapshot(born,$2::pg_snapshot)", []any{owner, cursor.Snapshot}, f)
	sql, args = appendDiagnosticCursorConditions(sql, args, &cursor, "at", "key")
	args = append(args, limit+1)
	sql += fmt.Sprintf(" ORDER BY at DESC NULLS LAST,key DESC LIMIT $%d", len(args))
	rows, err := a.DB.Pool().Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	items := []map[string]any{}
	for rows.Next() {
		var key string
		var at *time.Time
		var raw []byte
		if err := rows.Scan(&key, &at, &raw); err != nil {
			rows.Close()
			return nil, err
		}
		items = append(items, diagnosticRecord(kind, key, at, decodeObject(raw)))
	}
	readErr := rows.Err()
	rows.Close()
	if readErr != nil {
		return nil, readErr
	}
	if kind == "model-runs" {
		if err := a.decorateOwnerDiagnosticModels(ctx, items); err != nil {
			return nil, err
		}
	}
	page := finishDiagnosticPage(items, limit, cursor)
	return map[string]any{"items": page.Items, "next_cursor": page.NextCursor, "snapshot": page.Snapshot, "as_of": a.now().UTC().Format(time.RFC3339Nano), "filters": f, "snapshot_semantics": "creation_membership; mutable states reflect each read", "coverage": map[bool]string{true: "Owner-authorized browser domain rejection records; not inferred global Tool totals", false: "actual owning records"}[kind == "harm"]}, nil
}

func (a *App) OwnerDiagnosticRecord(ctx context.Context, owner, kind, ref string) (map[string]any, error) {
	if err := a.requireOwner(ctx, owner); err != nil {
		return nil, err
	}
	source, err := ownerDiagnosticSource(kind)
	if err != nil {
		return nil, err
	}
	key, err := decodeDiagnosticRecordRef(kind, ref)
	if err != nil {
		return nil, err
	}
	var at *time.Time
	var raw []byte
	err = a.DB.Pool().QueryRow(ctx, "WITH source AS ("+source+") SELECT at,data FROM source WHERE key=$2", owner, key).Scan(&at, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, &SurfaceError{Status: 404, Code: "diagnostic_record_unavailable", Message: "该诊断记录不存在或已被清理；业务来源和领域审计不因此删除"}
	}
	if err != nil {
		return nil, err
	}
	record := diagnosticRecord(kind, key, at, decodeObject(raw))
	delete(record, "_cursor_key")
	delete(record, "_cursor_time")
	delete(record, "_cursor_null")
	if kind == "model-runs" {
		if err := a.decorateOwnerDiagnosticModels(ctx, []map[string]any{record}); err != nil {
			return nil, err
		}
	}
	d := mapValue(record["data"])
	correlation := firstString(d["correlation_id"], stringValue(mapValue(d["scope"])["correlation_id"]))
	workflow := firstString(d["workflow_id"], stringValue(mapValue(d["metrics"])["workflow_id"]))
	related, err := readWorldObjects(ctx, a.DB.Pool(), `SELECT jsonb_build_object('id',l.id,'workflow_id',l.workflow_id,'intent_id',l.intent_id,'event_id',l.event_id,'correlation_id',l.correlation_id) FROM public.diagnostic_workflow_links l WHERE ($1::text<>'' AND l.correlation_id=$1) OR ($2::text<>'' AND l.workflow_id=$2) ORDER BY l.created_at DESC,l.id DESC LIMIT 51`, correlation, workflow)
	if err != nil {
		return nil, err
	}
	record["related_has_more"] = len(related) > 50
	if len(related) > 50 {
		related = related[:50]
	}
	record["related_workflows"] = related
	return record, nil
}

func matchDiagnosticOperations(method, path string) bool {
	p := surfaceParts(path)
	if len(p) < 1 || p[0] != "diagnostic-records" {
		return false
	}
	return method == "GET" && (len(p) == 2 || len(p) == 3) || method == "POST" && (len(p) == 2 && p[1] == "clear" || len(p) == 4 && p[1] == "media-prompts" && p[3] == "retry")
}
func (a *App) dispatchDiagnosticOperations(ctx context.Context, method, path string, q url.Values, owner string, body map[string]any) (map[string]any, error) {
	p := surfaceParts(path)
	if method == "POST" && len(p) == 4 {
		return a.RetryOwnerDiagnosticMedia(ctx, owner, p[2], body)
	}
	if method == "POST" {
		return a.ClearOwnerDiagnosticRecords(ctx, owner, body)
	}
	if len(p) == 3 {
		return a.OwnerDiagnosticRecord(ctx, owner, p[1], p[2])
	}
	return a.OwnerDiagnosticRecords(ctx, owner, p[1], q)
}

func (a *App) decorateOwnerDiagnosticModels(ctx context.Context, records []map[string]any) error {
	models := []map[string]any{}
	for _, r := range records {
		d := mapValue(r["data"])
		m := mapValue(d["metrics"])
		d["logical_run_id"] = firstString(stringValue(m["run_id"]), stringValue(d["correlation_id"]))
		d["parent_run_id"] = m["parent_run_id"]
		d["model_call_id"] = m["model_call_id"]
		d["stage"] = "unknown"
		status := stringValue(d["status"])
		if status == "failed" || status == "cancelled" || status == "timeout" {
			d["stage"] = status
		} else if status == "queued" || status == "running" {
			d["stage"] = "pending"
		} else if stringValue(mapValue(d["response"])["stage"]) != "" {
			d["stage"] = status
		} else if stringValue(m["model_call_id"]) == "" && stringValue(m["provider_attempt_id"]) != "" && m["provider_attempt_id"] != "legacy" {
			d["stage"] = nonADKModelRunStage(status, d["response"])
			d["sequence"] = d["physical_sequence"]
		}
		models = append(models, d)
	}
	if err := a.decorateModelRunRounds(ctx, models); err != nil {
		return err
	}
	a.decorateGoalEvaluationSettlement(ctx, models)
	return nil
}
