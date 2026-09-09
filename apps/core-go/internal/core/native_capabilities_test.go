package core

import (
	"strings"
	"testing"
)

func TestNormalizeSceneOperationRequiresExplicitOperation(t *testing.T) {
	if _, err := normalizeSceneOperation(map[string]any{}); err == nil || !strings.Contains(err.Error(), "required") {
		t.Fatalf("missing operation error = %v", err)
	}
	if _, err := normalizeSceneOperation(map[string]any{"operation": "teleport"}); err == nil || !strings.Contains(err.Error(), "start, switch, or end") {
		t.Fatalf("invalid operation error = %v", err)
	}
	for _, operation := range []string{"start", "switch", "end"} {
		got, err := normalizeSceneOperation(map[string]any{"operation": operation})
		if err != nil || got != operation {
			t.Fatalf("operation %q normalized as %q with error %v", operation, got, err)
		}
	}
}
