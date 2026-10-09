package core

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/ai/decision"
)

func (a *App) KevDecisionsPage(ctx context.Context, actor string, filter map[string]string) (DiagnosticRunPage, error) {
	if err := a.requireOwner(ctx, actor); err != nil {
		return DiagnosticRunPage{}, err
	}
	identity := map[string]string{}
	for _, key := range []string{"actor_self", "actor_user", "agent", "decision_point", "id", "request_id", "policy_outcome", "call_status", "application_status", "correlation_id", "run_id", "from", "to"} {
		v := strings.TrimSpace(filter[key])
		if len(v) > 256 {
			return DiagnosticRunPage{}, ErrDiagnosticsFilterInvalid
		}
		identity[key] = v
	}
	for _, key := range []string{"from", "to"} {
		if identity[key] != "" {
			if _, err := time.Parse(time.RFC3339Nano, identity[key]); err != nil {
				return DiagnosticRunPage{}, ErrDiagnosticsFilterInvalid
			}
		}
	}
	cursor, err := a.diagnosticCursor(ctx, actor, "kev", stableDigest(jsonString(identity)), filter["cursor"])
	if err != nil {
		return DiagnosticRunPage{}, err
	}
	args := []any{}
	query := `SELECT payload,started_at,id FROM public.kev_decisions WHERE true`
	for _, pair := range []struct{ key, column string }{{"actor_self", "actor_self"}, {"actor_user", "actor_user"}, {"agent", "agent"}, {"decision_point", "decision_point"}, {"id", "id"}, {"request_id", "request_id"}, {"policy_outcome", "payload->>'policy_outcome'"}, {"call_status", "payload->>'call_status'"}, {"application_status", "payload->>'application_status'"}, {"correlation_id", "payload->'scope'->>'correlation_id'"}, {"run_id", "run_id"}} {
		if value := identity[pair.key]; value != "" {
			args = append(args, value)
			query += fmt.Sprintf(" AND %s=$%d::text", pair.column, len(args))
		}
	}
	for _, key := range []string{"from", "to"} {
		if identity[key] != "" {
			args = append(args, identity[key])
			op := ">="
			if key == "to" {
				op = "<="
			}
			query += fmt.Sprintf(" AND started_at%s$%d::timestamptz", op, len(args))
		}
	}
	// Snapshot bounds late insertions without ordering by asynchronous writes.
	args = append(args, cursor.Snapshot)
	query += fmt.Sprintf(" AND pg_visible_in_snapshot(xmin::text::xid8,$%d::pg_snapshot)", len(args))
	query, args = appendDiagnosticCursorConditions(query, args, &cursor, "started_at", "id")
	limit := 100
	if filter["limit"] != "" {
		var parsed int
		if _, err := fmt.Sscan(filter["limit"], &parsed); err != nil || parsed < 1 || parsed > 500 {
			return DiagnosticRunPage{}, ErrDiagnosticsFilterInvalid
		}
		limit = parsed
	}
	args = append(args, limit+1)
	query += fmt.Sprintf(" ORDER BY started_at DESC,id DESC LIMIT $%d", len(args))
	rows, err := a.DB.Pool().Query(ctx, query, args...)
	if err != nil {
		return DiagnosticRunPage{}, err
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var raw []byte
		var at time.Time
		var id string
		if err := rows.Scan(&raw, &at, &id); err != nil {
			return DiagnosticRunPage{}, err
		}
		item := decodeObject(raw)
		item["_cursor_time"] = at.UTC().Format(time.RFC3339Nano)
		item["_cursor_key"] = id
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return DiagnosticRunPage{}, err
	}
	return finishDiagnosticPage(items, limit, cursor), nil
}

type explicitKevSettings struct {
	config  decision.Config
	version int64
	secret  string
}

func (s explicitKevSettings) Read(context.Context) (decision.Config, int64, string, error) {
	return s.config, s.version, s.secret, nil
}
func (a *App) TestKevConnection(ctx context.Context, actor string) (map[string]any, error) {
	if err := a.requireOwner(ctx, actor); err != nil {
		return nil, err
	}
	c, v, _, err := a.kevService().Settings.Read(ctx)
	if err != nil {
		return nil, err
	}
	p := ProviderClient{DB: a.DB, SettingsKey: a.SettingsKey}
	secret, err := p.secret(ctx, "kev:systemone")
	if err != nil {
		return nil, err
	}
	c.Enabled = true
	c.Points["tools.select"] = true
	service := &decision.Service{Settings: explicitKevSettings{c, v, secret}, Store: a.kevService().Store, HTTP: a.kevService().HTTP, ID: func() string { return randomID("kev_test_") }}
	results, err := service.Decide(ctx, "tools.select", decision.Scope{ActorUser: actor, Agent: "admin_connection_test"}, "This is an explicit administrator protocol test without any business action.", []decision.Candidate{{ID: "protocol_test", Question: decision.Choice("Is this a protocol test?")}})
	if err != nil {
		return nil, err
	}
	if len(results) != 1 {
		return nil, ErrInvalidArguments
	}
	r := &results[0]
	r.Record.RuleReason = "connection_test"
	_ = r.Finish(ctx, "not_applied", "", "")
	return map[string]any{"call_status": r.Record.CallStatus, "policy_outcome": r.Record.Outcome, "decision_id": r.Record.ID, "config_version": v, "error_code": r.Record.Reason}, nil
}
