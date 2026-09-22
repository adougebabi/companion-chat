package core

import (
	"context"
	"time"
)

// ProviderRuntimeSupport is the narrow persistence/diagnostic seam used by a
// physical model call. It deliberately does not expose App, Repository,
// CapabilityRuntime or domain state; the composition root supplies it when a
// ProviderClient is created.
type ProviderRuntimeSupport interface {
	RecordModelRun(context.Context, string, string, string, string, any, any, string, string)
	PersistModelRunLifecycle(context.Context, string, string, string, string, string, int, any, any, string, string) (string, error)
	RecordQueuedModelRun(context.Context, string, string, string, string, string, int, any) string
	UpdateModelRunState(context.Context, string, string, error)
	UpdateModelRunResponse(context.Context, string, any)
	UpdateModelRunPromptMetrics(context.Context, string, map[string]any, time.Duration)
	RecordDiagnosticEvent(context.Context, string, string, string, string, string, any)
	RecordLifecycleDiagnostic(context.Context, LifecycleDiagnostic) (string, error)
	RecordLifecycleDiagnosticBestEffort(context.Context, LifecycleDiagnostic)
}

type providerRuntimeSupport struct{ DB ProviderDatabase }

func newProviderRuntimeSupport(db ProviderDatabase) ProviderRuntimeSupport {
	return providerRuntimeSupport{DB: db}
}
