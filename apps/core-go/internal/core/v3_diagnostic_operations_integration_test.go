package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"strings"
	"testing"
	"time"
)

type v3FailingExportWriter struct {
	bytes.Buffer
	writes int
}

func (w *v3FailingExportWriter) Write(p []byte) (int, error) {
	w.writes++
	if w.writes > 1 {
		return 0, io.ErrClosedPipe
	}
	return w.Buffer.Write(p)
}

func TestV3DiagnosticsPostgresSnapshotExportScopedClearAndAuthority(t *testing.T) {
	f := v3OwnerFixture(t, "v3-diagnostic")
	at := time.Now().UTC().Add(-time.Hour).Truncate(time.Microsecond)
	for _, id := range []string{"event-1", "event-2", "event-3"} {
		if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.diagnostic_events(id,event_type,severity,correlation_id,payload,created_at) VALUES($1,'lifecycle.fixture','info','diagnostic-scope',$2,$3)`, id, jsonBytes(map[string]any{"text": "visible evidence", "api_key": "secret-fixture", "image_url": map[string]any{"url": "https://fixture/image"}}), at); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.diagnostic_workflow_links(id,correlation_id,workflow_id,created_at) VALUES('fixture-link','diagnostic-scope','fixture-workflow',$1)`, at); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.agent_runs(fluctlight_id,agent_id,run_id,input_digest,status) VALUES($1,'native_cognition','retained-agent','fixture-input','completed')`, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	query := url.Values{"correlationId": {"diagnostic-scope"}, "limit": {"1"}}
	first, err := f.app.OwnerDiagnosticRecords(f.ctx, f.ownerID, "events", query)
	if err != nil {
		t.Fatal(err)
	}
	items := first["items"].([]map[string]any)
	if len(items) != 1 || items[0]["id"] != "event-3" || strings.Contains(jsonString(items), "secret-fixture") || strings.Contains(jsonString(items), "https://fixture/image") || !strings.Contains(jsonString(items), "visible evidence") {
		t.Fatal("ordering or redaction failed", items)
	}
	ref := stringValue(items[0]["ref"])
	detail, err := f.app.OwnerDiagnosticRecord(f.ctx, f.ownerID, "events", ref)
	if err != nil || len(arrayValue(detail["related_workflows"])) != 1 {
		t.Fatal("source detail/workflow link missing", detail, err)
	}
	if _, err = f.repository.Pool().Exec(f.ctx, `INSERT INTO public.diagnostic_events(id,event_type,severity,correlation_id,payload,created_at) VALUES('late-backdated','lifecycle.fixture','info','diagnostic-scope','{}',$1),('other-scope','lifecycle.fixture','info','another-scope','{}',$1)`, at); err != nil {
		t.Fatal(err)
	}
	if _, err = f.repository.Pool().Exec(f.ctx, `UPDATE public.diagnostic_events SET payload='{"text":"latest mutable state"}' WHERE id='event-2'`); err != nil {
		t.Fatal(err)
	}
	nextQuery := url.Values{"correlationId": {"diagnostic-scope"}, "limit": {"1"}, "cursor": {stringValue(first["next_cursor"])}}
	second, err := f.app.OwnerDiagnosticRecords(f.ctx, f.ownerID, "events", nextQuery)
	if err != nil {
		t.Fatal(err)
	}
	secondItems := second["items"].([]map[string]any)
	if len(secondItems) != 1 || secondItems[0]["id"] != "event-2" || !strings.Contains(jsonString(secondItems), "latest mutable state") {
		t.Fatal("snapshot membership or mutable state broken", second)
	}
	thirdQuery := url.Values{"correlationId": {"diagnostic-scope"}, "limit": {"1"}, "cursor": {stringValue(second["next_cursor"])}}
	third, err := f.app.OwnerDiagnosticRecords(f.ctx, f.ownerID, "events", thirdQuery)
	if err != nil || third["items"].([]map[string]any)[0]["id"] != "event-1" || third["next_cursor"] != "" {
		t.Fatal("snapshot included backdated insertion or lost page", third, err)
	}
	nextQuery.Set("correlationId", "changed-filter")
	_, err = f.app.OwnerDiagnosticRecords(f.ctx, f.ownerID, "events", nextQuery)
	if !errors.Is(err, ErrDiagnosticsFilterInvalid) {
		t.Fatal("cursor reused across filters", err)
	}
	var stream bytes.Buffer
	if err = f.app.ExportOwnerDiagnostics(f.ctx, f.ownerID, url.Values{"dataset": {"events"}, "correlationId": {"diagnostic-scope"}}, &stream); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(stream.String()), "\n")
	records := make([]map[string]any, len(lines))
	for i, line := range lines {
		if err = json.Unmarshal([]byte(line), &records[i]); err != nil {
			t.Fatal("invalid JSONL", err)
		}
	}
	last := records[len(records)-1]
	if len(lines) != 6 || records[0]["type"] != "manifest" || last["type"] != "end" || last["complete"] != true || last["snapshot"] != records[0]["snapshot"] || intValue(mapValue(last["counts"])["events"]) != 4 || strings.Contains(stream.String(), "secret-fixture") || strings.Contains(stream.String(), "https://fixture/image") {
		t.Fatal("export missing complete consistent source", stream.String())
	}
	failed := &v3FailingExportWriter{}
	if err = f.app.ExportOwnerDiagnostics(f.ctx, f.ownerID, url.Values{"dataset": {"events"}}, failed); err == nil || strings.Contains(failed.String(), `"complete":true`) {
		t.Fatal("failed output claimed complete", failed.String(), err)
	}
	// Exercise every actual source SQL against the migrated database.
	for _, kind := range ownerDiagnosticKinds {
		if _, err = f.app.OwnerDiagnosticRecords(f.ctx, f.ownerID, kind, url.Values{"limit": {"1"}}); err != nil {
			t.Errorf("source %s cannot be read: %v", kind, err)
		}
	}
	clear := map[string]any{"datasets": []any{"events"}, "before": time.Now().UTC().Format(time.RFC3339Nano), "snapshot": first["snapshot"], "filters": map[string]any{"correlationId": "diagnostic-scope"}, "confirm": true, "reason": "Owner clears the reviewed snapshot", "idempotency_key": "clear-snapshot"}
	receipt, err := f.app.ClearOwnerDiagnosticRecords(f.ctx, f.ownerID, clear)
	if err != nil || receipt["cleared"] != int64(3) || receipt["domain_history_preserved"] != true {
		t.Fatal("scoped clear failed", receipt, err)
	}
	replay, err := f.app.ClearOwnerDiagnosticRecords(f.ctx, f.ownerID, clear)
	if err != nil || replay["id"] != receipt["id"] || replay["_replay"] != true {
		t.Fatal("cleanup replay duplicated", replay, err)
	}
	var remaining, agents, exports, clears int
	if err = f.repository.Pool().QueryRow(f.ctx, `SELECT (SELECT count(*) FROM public.diagnostic_events),(SELECT count(*) FROM public.agent_runs WHERE run_id='retained-agent'),(SELECT count(*) FROM public.owner_surface_audits WHERE action='diagnostic_export'),(SELECT count(*) FROM public.owner_surface_audits WHERE action='diagnostic_clear')`).Scan(&remaining, &agents, &exports, &clears); err != nil || remaining != 2 || agents != 1 || exports != 1 || clears != 1 {
		t.Fatal("cleanup lost late/unrelated/authority rows or duplicated audits", remaining, agents, exports, clears, err)
	}
	_, err = f.app.OwnerDiagnosticRecord(f.ctx, f.ownerID, "events", ref)
	requireV3SurfaceError(t, err, "diagnostic_record_unavailable")
	illegal := cloneMap(clear)
	illegal["idempotency_key"] = "illegal-clear"
	illegal["datasets"] = []any{"agent-runs"}
	_, err = f.app.ClearOwnerDiagnosticRecords(f.ctx, f.ownerID, illegal)
	if err == nil {
		t.Fatal("Agent authority was accepted as a cleanup target")
	}
	if _, err = f.app.OwnerDiagnosticRecords(f.ctx, f.foreignOwnerID, "events", nil); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("foreign Owner read diagnostics", err)
	}
}
