package workflow

import (
	"os"
	"strings"
	"testing"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"
)

func TestCognitionDispatchSeparatesOwnerRequestCapacityAndKeepsHistoryRegistrations(t *testing.T) {
	if got := cognitionDispatchTaskQueue(core.OwnerLifeRequestFactType); got != LifecycleQueue {
		t.Fatalf("Owner request queue=%s", got)
	}
	for _, event := range []string{"conversation.turn", "life.scene.updated", ""} {
		if got := cognitionDispatchTaskQueue(event); got != InteractionQueue {
			t.Fatalf("event %s queue=%s", event, got)
		}
	}
	raw, err := os.ReadFile("workflow.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	for _, queue := range []string{"LifecycleQueue", "InteractionQueue"} {
		start := strings.Index(source, "case "+queue+":")
		if start < 0 {
			t.Fatal("missing Worker", queue)
		}
		block := strings.SplitN(source[start:], "\n\t\tcase ", 2)[0]
		for _, registration := range []string{"w.RegisterWorkflow(CognitionProcessingWorkflow)", "w.RegisterActivity(ProcessCognitionActivity)"} {
			if !strings.Contains(block, registration) {
				t.Fatalf("%s missing history-compatible registration %s", queue, registration)
			}
		}
	}
}
