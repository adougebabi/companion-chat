package core

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"time"

	"github.com/jackc/pgx/v5"
)

// Export is a bounded-memory stream from one actual PostgreSQL repeatable-read
// snapshot. The terminal line is emitted only after all selected sources and
// commit succeed. A truncated HTTP download cannot be mistaken for completion.
func (a *App) ExportOwnerDiagnostics(ctx context.Context, owner string, q url.Values, w io.Writer) (resultErr error) {
	if err := a.requireOwner(ctx, owner); err != nil {
		return err
	}
	f, err := parseOwnerDiagnosticFilter(q)
	if err != nil {
		return err
	}
	kinds := []string{"events", "model-runs", "agent-runs", "media-prompts", "kev", "workflow-intents", "audit", "harm", "turns", "links", "media-recoveries"}
	if selected := q.Get("dataset"); selected != "" && selected != "all" {
		if !containsString(append(append([]string{}, kinds...), "lifecycle"), selected) {
			return ErrDiagnosticsFilterInvalid
		}
		kinds = []string{selected}
	}
	tx, err := a.DB.Pool().BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return err
	}
	defer tx.Rollback(context.WithoutCancel(ctx))
	var snapshot string
	if err := tx.QueryRow(ctx, `SELECT pg_current_snapshot()::text`).Scan(&snapshot); err != nil {
		return err
	}
	encoder := json.NewEncoder(w)
	started := false
	defer func() {
		if resultErr != nil && started {
			_ = encoder.Encode(map[string]any{"type": "error", "complete": false, "code": "diagnostic_export_incomplete", "message": "诊断导出未完成，请重新导出；没有声明部分文件完整"})
		}
	}()
	if err := encoder.Encode(map[string]any{"type": "manifest", "format": "fluctlight.diagnostics.jsonl.v1", "snapshot": snapshot, "consistency": "repeatable_read", "filters": f, "datasets": kinds, "exported_at": a.now().UTC().Format(time.RFC3339Nano), "record_policy": "Owner diagnostic views; image/credential redaction retained; Agent authority and media intents are not cleanup targets"}); err != nil {
		return err
	}
	started = true
	counts := map[string]int64{}
	for _, kind := range kinds {
		source, err := ownerDiagnosticSource(kind)
		if err != nil {
			return err
		}
		sql, args := appendOwnerDiagnosticFilter("WITH source AS ("+source+") SELECT key,at,data FROM source WHERE true", []any{owner}, f)
		sql += " ORDER BY at DESC NULLS LAST,key DESC"
		rows, err := tx.Query(ctx, sql, args...)
		if err != nil {
			return err
		}
		counts[kind] = 0
		for rows.Next() {
			var key string
			var at *time.Time
			var raw []byte
			if err := rows.Scan(&key, &at, &raw); err != nil {
				rows.Close()
				return err
			}
			record := diagnosticRecord(kind, key, at, decodeObject(raw))
			delete(record, "_cursor_time")
			delete(record, "_cursor_key")
			delete(record, "_cursor_null")
			if err := encoder.Encode(map[string]any{"type": "record", "dataset": kind, "record": record}); err != nil {
				rows.Close()
				return err
			}
			counts[kind]++
		}
		readErr := rows.Err()
		rows.Close()
		if readErr != nil {
			return readErr
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	auditID := randomID("audit_diagnostic_export_")
	if _, err := a.DB.Pool().Exec(ctx, `INSERT INTO public.owner_surface_audits(id,owner_actor_id,fluctlight_id,identity,actor,action,reason,record_id) VALUES($1,$2,'','owner_edit','owner','diagnostic_export','诊断记录已读取并完成服务端快照；客户端文件完成情况由完整性回执检查',$3)`, auditID, owner, "diagnostic_export_"+stableDigest(snapshot+":"+jsonString(counts))); err != nil {
		return err
	}
	return encoder.Encode(map[string]any{"type": "end", "complete": true, "counts": counts, "snapshot": snapshot})
}

func (a *App) ClearOwnerDiagnosticRecords(ctx context.Context, owner string, body map[string]any) (map[string]any, error) {
	if err := a.requireOwner(ctx, owner); err != nil {
		return nil, err
	}
	if !eventKeys(body, "datasets", "before", "snapshot", "filters", "confirm", "reason", "idempotency_key") || body["confirm"] != true {
		return nil, invalid("明确的诊断清理确认、截止时间与读取快照是必需的")
	}
	before, err := time.Parse(time.RFC3339Nano, stringValue(body["before"]))
	if err != nil || before.After(a.now().Add(time.Minute)) {
		return nil, ErrDiagnosticsFilterInvalid
	}
	snapshot := stringValue(body["snapshot"])
	if len(snapshot) > 4096 || !diagnosticSnapshotPattern.MatchString(snapshot) {
		return nil, ErrDiagnosticsFilterInvalid
	}
	datasets := decisionServiceRefValues(body["datasets"])
	if len(datasets) == 0 || len(datasets) > 4 {
		return nil, invalid("请明确选择诊断清理来源")
	}
	allowed := map[string]string{"events": "diagnostic_events", "model-runs": "diagnostic_model_runs", "turns": "diagnostic_turns", "links": "diagnostic_workflow_links"}
	filters := url.Values{}
	for k, v := range mapValue(body["filters"]) {
		value, ok := v.(string)
		if !ok {
			return nil, ErrDiagnosticsFilterInvalid
		}
		filters.Set(k, value)
	}
	f, err := parseOwnerDiagnosticFilter(filters)
	if err != nil {
		return nil, err
	}
	ctx = context.WithValue(ctx, ownerCommandAuditActionKey{}, "diagnostic_clear")
	return a.worldCommand(ctx, owner, "", "diagnostic-clear", body, func(ctx context.Context, tx pgx.Tx) (map[string]any, error) {
		var canonical string
		if err := tx.QueryRow(ctx, `SELECT $1::pg_snapshot::text`, snapshot).Scan(&canonical); err != nil {
			return nil, ErrDiagnosticsFilterInvalid
		}
		counts := map[string]int64{}
		var total int64
		for _, kind := range datasets {
			table, ok := allowed[kind]
			if !ok {
				return nil, invalid("清理仅允许诊断事件、模型、回合和工作流诊断链接；领域、Agent、审计和媒体来源保留")
			}
			if _, seen := counts[kind]; seen {
				return nil, invalid("重复的诊断清理来源")
			}
			source, err := ownerDiagnosticSource(kind)
			if kind == "model-runs" {
				source = `SELECT t.id AS key,t.created_at AS at,t.recorded_xid AS born,to_jsonb(t)-'recorded_xid' AS data FROM public.diagnostic_model_runs t WHERE $1::text<>''`
			}
			if err != nil {
				return nil, err
			}
			sql, args := appendOwnerDiagnosticFilter("WITH source AS ("+source+") SELECT key FROM source WHERE true", []any{owner}, f)
			args = append(args, before, canonical)
			sql += fmt.Sprintf(" AND at<=$%d AND pg_visible_in_snapshot(born,$%d::pg_snapshot)", len(args)-1, len(args))
			command, err := tx.Exec(ctx, "DELETE FROM public."+table+" WHERE id IN ("+sql+")", args...)
			if err != nil {
				return nil, err
			}
			counts[kind] = command.RowsAffected()
			total += command.RowsAffected()
		}
		return map[string]any{"id": randomID("diagnostic_clear_"), "cleared": total, "counts": counts, "before": before.UTC().Format(time.RFC3339Nano), "snapshot": canonical, "domain_history_preserved": true}, nil
	})
}
