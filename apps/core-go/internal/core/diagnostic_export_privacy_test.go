package core

import (
	"strings"
	"testing"
)

func TestDiagnosticExportUsesPerExportAnonymousIdentity(t *testing.T) {
	first, err := newDiagnosticExportSanitizer()
	if err != nil {
		t.Fatal(err)
	}
	second, err := newDiagnosticExportSanitizer()
	if err != nil {
		t.Fatal(err)
	}
	if first.alias("same-private-id") != first.alias("same-private-id") || first.alias("same-private-id") == second.alias("same-private-id") || strings.Contains(first.alias("same-private-id"), "same-private-id") {
		t.Fatal("export identity is not stable within the export and unlinkable across exports")
	}
	filters := diagnosticExportFilters(map[string]any{"q": "private", "limit": 10, "private-key": "private"})
	if len(filters) != 2 || filters["q"] != "[REDACTED]" || filters["limit"] != "[REDACTED]" {
		t.Fatalf("export filters exposed values or unknown keys: %#v", filters)
	}
	attempt := first.data(map[string]any{"payload": map[string]any{"status": "opened", "physical_attempt_id": "private-attempt", "duration_ms": 739184250611}})
	if attempt["status"] != "opened" || attempt["physical_attempt_id"] != first.alias("private-attempt") || attempt["duration_ms"] != nil {
		t.Fatalf("physical attempt export lost safe state or retained arbitrary numeric data: %#v", attempt)
	}
}
