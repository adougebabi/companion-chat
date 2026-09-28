package core

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"
)

func TestLifecycleDiagnosticPayloadKeepsTextAndBoundsCollections(t *testing.T) {
	type imagePart struct {
		URL    string `json:"url"`
		Detail string `json:"detail"`
	}
	long := strings.Repeat("x", 900)
	many := make([]any, 80)
	for index := range many {
		many[index] = index
	}
	event := LifecycleDiagnostic{
		Surface:       "wake_up",
		Transition:    LifecycleTransitionFailed,
		Severity:      "error",
		FluctlightID:  "fl-1",
		CorrelationID: "wake_up:fl-1:cycle-4",
		IntentID:      "wake_up_intent:fl-1",
		WorkflowID:    "go:wake_up:fl-1",
		RunID:         "run-4",
		Stage:         "provider",
		Status:        "failed",
		ReasonCode:    "provider_request_failed",
		ErrorCode:     "request_timeout",
		SafeCause:     long,
		Retryable:     true,
		NextDueAt:     time.Date(2026, 9, 13, 2, 0, 0, 0, time.UTC),
		Metadata: map[string]any{
			"authorization": "Bearer secret",
			"raw_prompt":    "private character card",
			"binary":        []byte("private binary card"),
			"image_url":     "https://media.invalid/private-image?grant=secret",
			"image_object":  map[string]any{"image_url": map[string]any{"url": "https://media.invalid/nested?grant=secret", "detail": "high"}},
			"typed_image": struct {
				ImageURL imagePart `json:"image_url"`
			}{ImageURL: imagePart{URL: "https://media.invalid/typed?grant=secret", Detail: "high"}},
			"image_bytes":  []byte("raw-image-bytes-without-mime"),
			"image_base64": []byte("raw-base64-without-mime"),
			"image_note":   "saw data:image/png;base64,ZmFrZQ== today",
			"items":        many,
			"note":         long,
		},
	}
	payload, err := lifecycleDiagnosticPayload(event, time.Date(2026, 9, 13, 1, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	encoded := jsonString(payload)
	if !strings.Contains(encoded, "Bearer secret") || !strings.Contains(encoded, "private character card") || !strings.Contains(encoded, "private binary card") || strings.Contains(encoded, "[REDACTED]") || strings.Contains(encoded, "media.invalid") || strings.Contains(encoded, "ZmFrZQ==") || strings.Contains(encoded, "raw-image-bytes-without-mime") || strings.Contains(encoded, "raw-base64-without-mime") {
		t.Fatalf("lifecycle diagnostic changed non-image text: %s", encoded)
	}
	if stringValue(payload["safe_cause"]) == long || len([]rune(stringValue(payload["safe_cause"]))) > 512 {
		t.Fatalf("safe cause was not bounded: %#v", payload["safe_cause"])
	}
	metadata := mapValue(payload["metadata"])
	if len(arrayValue(metadata["items"])) != 32 || len([]rune(stringValue(metadata["note"]))) > 512 {
		t.Fatalf("lifecycle metadata was not bounded: %#v", metadata)
	}
}

func TestLifecycleCauseKeepsFreeformText(t *testing.T) {
	for _, unsafe := range []string{
		"request failed: token=sk-live-private",
		"session=private-session",
		"credential: private",
		"https://user:password@example.invalid/api",
		"upstream key sk-live-private",
	} {
		if got := boundedLifecycleCause(unsafe); got != unsafe {
			t.Fatalf("cause %q was changed to %q", unsafe, got)
		}
	}
	if got := boundedLifecycleCause("tool execution capability_prepare_failed: invalid arguments"); !strings.Contains(got, "invalid arguments") {
		t.Fatalf("safe cause lost useful failure detail: %q", got)
	}
}

func TestLifecycleDiagnosticTransitionIdentityDeduplicatesRepeats(t *testing.T) {
	base := LifecycleDiagnostic{
		Surface: "wake_up", Transition: LifecycleTransitionRetryScheduled,
		CorrelationID: "wake_up:fl-1:cycle-4", IntentID: "wake_up_intent:fl-1",
		WorkflowID: "go:wake_up:fl-1", RunID: "run-4", Stage: "workflow",
		Status: "retry", ReasonCode: "workflow_terminal_failure", Attempt: 4,
	}
	repeat := base
	repeat.Metadata = map[string]any{"transport_detail": "changed but same transition"}
	if first, second := lifecycleDiagnosticID(base), lifecycleDiagnosticID(repeat); first == "" || first != second {
		t.Fatalf("repeat transition IDs differ: %q %q", first, second)
	}
	nextAttempt := base
	nextAttempt.Attempt++
	if lifecycleDiagnosticID(base) == lifecycleDiagnosticID(nextAttempt) {
		t.Fatal("different lifecycle attempts collapsed into one transition identity")
	}
}

func TestLifecycleDiagnosticWriterRejectsUnavailableStore(t *testing.T) {
	_, err := (&App{}).RecordLifecycleDiagnostic(context.Background(), LifecycleDiagnostic{
		Surface: "wake_up", Transition: LifecycleTransitionScheduled,
		CorrelationID: "wake_up:fl-1:cycle-1",
	})
	if !errors.Is(err, ErrDiagnosticsUnavailable) {
		t.Fatalf("RecordLifecycleDiagnostic error = %v, want diagnostics unavailable", err)
	}
}

func TestLifecycleFailureDiagnosticPersistsAfterCallerCancellation(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	newProviderRuntimeSupport(repository).RecordLifecycleDiagnosticBestEffort(cancelled, LifecycleDiagnostic{
		Surface: "wake_up", Transition: LifecycleTransitionFailed,
		CorrelationID: "wake_up:cancelled-diagnostic:cycle:1", Stage: "activity",
		Status: "failed", ReasonCode: "agent_run_failed", SafeCause: "Tool execution failed",
	})
	var count int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.diagnostic_events WHERE correlation_id='wake_up:cancelled-diagnostic:cycle:1' AND event_type LIKE 'lifecycle.%'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("cancelled failure diagnostic count = %d: %v", count, err)
	}
}

func TestLifecycleDiagnosticWriterOwnsWorkflowLinkPersistence(t *testing.T) {
	source, err := os.ReadFile("lifecycle_diagnostics.go")
	if err != nil {
		t.Fatal(err)
	}
	text := string(source)
	for _, required := range []string{
		"diagnostic_events",
		"diagnostic_workflow_links",
		"occurrence_count",
		"first_seen_at",
		"last_seen_at",
		"RowsAffected",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("lifecycle diagnostic writer missing %q", required)
		}
	}
}
