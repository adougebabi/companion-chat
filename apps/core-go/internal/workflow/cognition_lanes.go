package workflow

import "github.com/fluctlight/local-ai-companion/apps/core-go/internal/core"

// Only new starts move queues. Existing Workflow/Activity commands and stable
// IDs are unchanged, and both queues retain the cognition registration.
func cognitionDispatchTaskQueue(eventType string) string {
	if eventType == core.OwnerLifeRequestFactType {
		return LifecycleQueue
	}
	return InteractionQueue
}
