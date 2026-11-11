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
}
