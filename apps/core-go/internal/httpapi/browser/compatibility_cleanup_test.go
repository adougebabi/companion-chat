package browser

import (
	"os"
	"strings"
	"testing"
)

func TestBrowserBoundaryHasNoMigrationAliasesOrDualReadGuessing(t *testing.T) {
	for _, file := range []string{"routes.go", "dto.go"} {
		source, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		text := string(source)
		for _, forbidden := range []string{"NewServer(", "first(value, \"actorId\", \"actor_id\")", "first(value, \"session_token\", \"sessionToken\")", "first(row, \"binding_role\", \"bindingRole\")", "first(row, \"queue_pending_count\", \"queuePendingCount\")"} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("migration compatibility alias remains in %s: %q", file, forbidden)
			}
		}
	}
}
