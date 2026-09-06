package core

import (
	"testing"
	"time"
)

func TestQuietHoursActiveSupportsOvernightWindow(t *testing.T) {
	quiet := jsonBytes(map[string]any{"start": "22:00", "end": "08:00"})
	if !quietHoursActive(quiet, time.Date(2026, 9, 7, 23, 30, 0, 0, time.UTC)) {
		t.Fatal("expected overnight quiet hours to block late-night action")
	}
	if !quietHoursActive(quiet, time.Date(2026, 9, 7, 7, 30, 0, 0, time.UTC)) {
		t.Fatal("expected overnight quiet hours to block early-morning action")
	}
	if quietHoursActive(quiet, time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)) {
		t.Fatal("daytime action should not be blocked by overnight quiet hours")
	}
}

func TestAutonomySnapshotIncludesPolicyRevisionAndDecision(t *testing.T) {
	snapshot := autonomySnapshot("active", "moment", []any{"moment"}, "3", jsonBytes(map[string]any{"start": "22:00", "end": "08:00"}), nil, 2, 7)
	if stringValue(snapshot["mode"]) != "active" || stringValue(snapshot["action_type"]) != "moment" || intValue(snapshot["revision"]) != 7 || intValue(snapshot["concurrency_limit"]) != 2 {
		t.Fatalf("snapshot = %#v", snapshot)
	}
}
