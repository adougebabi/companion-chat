package core

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

type DiagnosticRunPage struct {
	Items      []map[string]any `json:"items"`
	NextCursor string           `json:"next_cursor"`
	Snapshot   string           `json:"snapshot"`
}
type diagnosticPageCursor struct {
	Kind     string `json:"kind"`
	Filter   string `json:"filter"`
	Snapshot string `json:"snapshot"`
	Time     string `json:"time,omitempty"`
	Key      string `json:"key,omitempty"`
	Null     bool   `json:"null,omitempty"`
}

var diagnosticSnapshotPattern = regexp.MustCompile(`^[0-9]+:[0-9]+:([0-9]+(,[0-9]+)*)?$`)

func diagnosticPageLimit(limit int) int {
	if limit < 1 {
		return 100
	}
	if limit > 500 {
		return 500
	}
	return limit
}
func (a *App) diagnosticCursor(ctx context.Context, actor, kind, correlation, raw string) (diagnosticPageCursor, error) {
	cursor := diagnosticPageCursor{Kind: kind, Filter: stableDigest(actor + "\x1f" + kind + "\x1f" + correlation)}
	if len(raw) > 8192 || len(correlation) > 128 {
		return cursor, ErrDiagnosticsFilterInvalid
	}
	if raw != "" {
		data, err := base64.RawURLEncoding.DecodeString(raw)
		if err != nil {
			return cursor, ErrDiagnosticsFilterInvalid
		}
		var decoded diagnosticPageCursor
		if err := json.Unmarshal(data, &decoded); err != nil || decoded.Kind != cursor.Kind || decoded.Filter != cursor.Filter || !diagnosticSnapshotPattern.MatchString(decoded.Snapshot) || len(decoded.Snapshot) > 4096 || len(decoded.Key) > 512 {
			return cursor, ErrDiagnosticsFilterInvalid
		}
		if decoded.Key == "" || (decoded.Null && decoded.Time != "") || (!decoded.Null && decoded.Time == "") {
			return cursor, ErrDiagnosticsFilterInvalid
		}
		var canonical string
		if err := a.DB.Pool().QueryRow(ctx, `SELECT $1::pg_snapshot::text`, decoded.Snapshot).Scan(&canonical); err != nil {
			return cursor, ErrDiagnosticsFilterInvalid
		}
		if decoded.Time != "" {
			if _, err := time.Parse(time.RFC3339Nano, decoded.Time); err != nil || decoded.Key == "" {
				return cursor, ErrDiagnosticsFilterInvalid
			}
		}
		return decoded, nil
	}
	err := a.DB.Pool().QueryRow(ctx, `SELECT pg_current_snapshot()::text`).Scan(&cursor.Snapshot)
	return cursor, err
}
func finishDiagnosticPage(rows []map[string]any, limit int, cursor diagnosticPageCursor) DiagnosticRunPage {
	page := DiagnosticRunPage{Items: rows, Snapshot: cursor.Snapshot}
	if len(rows) > limit {
		page.Items = rows[:limit]
		last := page.Items[len(page.Items)-1]
		cursor.Time = stringValue(last["_cursor_time"])
		cursor.Key = stringValue(last["_cursor_key"])
		cursor.Null = last["_cursor_null"] == true
		data, _ := json.Marshal(cursor)
		page.NextCursor = base64.RawURLEncoding.EncodeToString(data)
	}
	for _, row := range page.Items {
		delete(row, "_cursor_time")
		delete(row, "_cursor_key")
		delete(row, "_cursor_null")
	}
	return page
}
func (a *App) ModelRunsPage(ctx context.Context, actor string, limit int, correlation, rawCursor string) (DiagnosticRunPage, error) {
	if err := a.requireOwner(ctx, actor); err != nil {
		return DiagnosticRunPage{}, err
	}
	correlation = strings.TrimSpace(correlation)
	cursor, err := a.diagnosticCursor(ctx, actor, "model", correlation, rawCursor)
	if err != nil {
		return DiagnosticRunPage{}, err
	}
	limit = diagnosticPageLimit(limit)
	rows, err := a.modelRunsQuery(ctx, actor, limit+1, correlation, "", &cursor)
	if err != nil {
		return DiagnosticRunPage{}, err
	}
	return finishDiagnosticPage(rows, limit, cursor), nil
}
func (a *App) AgentRunsPage(ctx context.Context, actor string, limit int, correlation, rawCursor string) (DiagnosticRunPage, error) {
	if err := a.requireOwner(ctx, actor); err != nil {
		return DiagnosticRunPage{}, err
	}
	correlation = strings.TrimSpace(correlation)
	cursor, err := a.diagnosticCursor(ctx, actor, "agent", correlation, rawCursor)
	if err != nil {
		return DiagnosticRunPage{}, err
	}
	limit = diagnosticPageLimit(limit)
	rows, err := a.agentRunsQuery(ctx, limit+1, correlation, &cursor)
	if err != nil {
		return DiagnosticRunPage{}, err
	}
	return finishDiagnosticPage(rows, limit, cursor), nil
}
func appendDiagnosticCursorConditions(query string, args []any, cursor *diagnosticPageCursor, timeColumn, keyColumn string) (string, []any) {
	if cursor != nil && cursor.Key != "" {
		if cursor.Null {
			args = append(args, cursor.Key)
			query += fmt.Sprintf(" AND %s IS NULL AND %s<$%d::text", timeColumn, keyColumn, len(args))
		} else {
			args = append(args, cursor.Time, cursor.Key)
			query += fmt.Sprintf(" AND ((%s,%s)<($%d::timestamptz,$%d::text) OR %s IS NULL)", timeColumn, keyColumn, len(args)-1, len(args), timeColumn)
		}
	}
	return query, args
}
