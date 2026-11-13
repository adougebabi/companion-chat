package core

import (
	"bytes"
	"errors"
	"net/url"
	"testing"
)

func TestDiagnosticDebugReadDeniedByDefaultAcrossRawEntrypoints(t *testing.T) {
	f := v3OwnerFixture(t, "diagnostic-debug-access")
	for _, kind := range ownerDiagnosticKinds {
		t.Run("records/"+kind, func(t *testing.T) {
			_, err := f.app.OwnerDiagnosticRecords(f.ctx, f.ownerID, kind, nil)
			requireV3SurfaceError(t, err, "diagnostic_debug_read_disabled")
		})
	}
	checks := map[string]func() error{
		"kev":                 func() error { _, err := f.app.KevDecisions(f.ctx, f.ownerID, nil); return err },
		"kev-page":            func() error { _, err := f.app.KevDecisionsPage(f.ctx, f.ownerID, nil); return err },
		"workflow-status":     func() error { _, err := f.app.WorkflowStatus(f.ctx, f.ownerID, "missing"); return err },
		"workflow-inspection": func() error { _, err := f.app.OwnerWorkflowInspection(f.ctx, f.ownerID, "missing"); return err },

		"detail": func() error { _, err := f.app.OwnerDiagnosticRecord(f.ctx, f.ownerID, "events", "invalid"); return err },
		"events": func() error { _, err := f.app.Diagnostics(f.ctx, f.ownerID, 10); return err },
		"lifecycle": func() error {
			_, err := f.app.LifecycleDiagnostics(f.ctx, f.ownerID, LifecycleDiagnosticsFilter{})
			return err
		},
		"models":           func() error { _, err := f.app.ModelRuns(f.ctx, f.ownerID, 10); return err },
		"model-page":       func() error { _, err := f.app.ModelRunsPage(f.ctx, f.ownerID, 10, "", ""); return err },
		"media":            func() error { _, err := f.app.MediaPromptsFiltered(f.ctx, f.ownerID, 10); return err },
		"workflow-history": func() error { _, err := f.app.OwnerWorkflowHistoryPage(f.ctx, f.ownerID, "missing", nil); return err },
	}
	for name, read := range checks {
		t.Run(name, func(t *testing.T) { requireV3SurfaceError(t, read(), "diagnostic_debug_read_disabled") })
	}
	for _, query := range []url.Values{{"q": {"private text"}}, {"qAny": {`["private text"]`}}, {"q_any": {`["private text"]`}}} {
		var output bytes.Buffer
		err := f.app.ExportOwnerDiagnostics(f.ctx, f.ownerID, query, &output)
		requireV3SurfaceError(t, err, "diagnostic_debug_read_disabled")
		if output.Len() != 0 {
			t.Fatal("denied search wrote export bytes")
		}
		filters := map[string]any{}
		for key := range query {
			filters[key] = query.Get(key)
		}
		_, err = f.app.ClearOwnerDiagnosticRecords(f.ctx, f.ownerID, map[string]any{
			"confirm": true, "before": "2020-01-01T00:00:00Z", "snapshot": "1:2:",
			"datasets": []any{"events"}, "filters": filters,
		})
		requireV3SurfaceError(t, err, "diagnostic_debug_read_disabled")
	}
	// Metadata-only exports and operational run summaries remain usable.
	var output bytes.Buffer
	if err := f.app.ExportOwnerDiagnostics(f.ctx, f.ownerID, url.Values{"dataset": {"events"}}, &output); err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.AgentRunsPage(f.ctx, f.ownerID, 10, "", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.OwnerDiagnosticRecords(f.ctx, f.foreignOwnerID, "events", nil); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("foreign owner", err)
	}
}

func TestDiagnosticDebugReadExplicitGrantAndRevocation(t *testing.T) {
	f := v3OwnerFixture(t, "diagnostic-debug-grant")
	f.app.DiagnosticDebugReadEnabled = true
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.diagnostic_events(id,event_type,severity,correlation_id,payload) VALUES('debug-private-event','debug.fixture','info','debug-fixture','{"text":"SYNTHETIC_PRIVATE_HISTORY","api_key":"fixture-secret"}'),('debug-private-event-2','debug.fixture','info','debug-fixture','{"text":"SYNTHETIC_PRIVATE_HISTORY"}')`); err != nil {
		t.Fatal(err)
	}
	for _, kind := range ownerDiagnosticKinds {
		if _, err := f.app.OwnerDiagnosticRecords(f.ctx, f.ownerID, kind, nil); err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
		if _, err := f.app.OwnerDiagnosticRecords(f.ctx, f.foreignOwnerID, kind, nil); !errors.Is(err, ErrUnauthorized) {
			t.Fatalf("grant bypassed Owner for %s: %v", kind, err)
		}
	}
	page, err := f.app.OwnerDiagnosticRecords(f.ctx, f.ownerID, "events", url.Values{"q": {"SYNTHETIC_PRIVATE_HISTORY"}, "limit": {"1"}})
	if err != nil {
		t.Fatal(err)
	}
	items := page["items"].([]map[string]any)
	if len(items) != 1 || stringValue(page["next_cursor"]) == "" {
		t.Fatal("authorized search lost stored history", page)
	}
	ref := stringValue(items[0]["ref"])
	detail, err := f.app.OwnerDiagnosticRecord(f.ctx, f.ownerID, "events", ref)
	if err != nil || !bytes.Contains([]byte(jsonString(detail)), []byte("SYNTHETIC_PRIVATE_HISTORY")) || bytes.Contains([]byte(jsonString(detail)), []byte("fixture-secret")) {
		t.Fatal("authorized detail/redaction", detail, err)
	}
	for _, query := range []url.Values{{"q": {"SYNTHETIC_PRIVATE_HISTORY"}}, {"qAny": {`["SYNTHETIC_PRIVATE_HISTORY"]`}}, {"q_any": {`["SYNTHETIC_PRIVATE_HISTORY"]`}}} {
		query.Set("dataset", "events")
		var output bytes.Buffer
		if err := f.app.ExportOwnerDiagnostics(f.ctx, f.ownerID, query, &output); err != nil || !bytes.Contains(output.Bytes(), []byte(`"complete":true`)) || bytes.Contains(output.Bytes(), []byte("SYNTHETIC_PRIVATE_HISTORY")) {
			t.Fatal("authorized privacy-safe search export", err, output.String())
		}
	}
	// An already obtained reference and cursor do not preserve a revoked grant.
	f.app.DiagnosticDebugReadEnabled = false
	_, err = f.app.OwnerDiagnosticRecord(f.ctx, f.ownerID, "events", ref)
	requireV3SurfaceError(t, err, "diagnostic_debug_read_disabled")
	_, err = f.app.OwnerDiagnosticRecords(f.ctx, f.ownerID, "events", url.Values{"cursor": {stringValue(page["next_cursor"])}})
	requireV3SurfaceError(t, err, "diagnostic_debug_read_disabled")
	var stored string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT payload->>'text' FROM public.diagnostic_events WHERE id='debug-private-event'`).Scan(&stored); err != nil || stored != "SYNTHETIC_PRIVATE_HISTORY" {
		t.Fatal("read authorization mutated retained evidence", stored, err)
	}
}
