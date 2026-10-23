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

func TestQuietHoursWeekdaysUseTheStartDayAcrossMidnight(t *testing.T) {
	quiet := jsonBytes(map[string]any{"start": "22:00", "end": "08:00", "timezone": "Asia/Shanghai", "days": []any{float64(1)}})
	for _, tc := range []struct {
		at   string
		want bool
	}{
		{"2026-09-07T23:00:00+08:00", true},
		{"2026-09-08T07:00:00+08:00", true},
		{"2026-09-07T07:00:00+08:00", false},
		{"2026-09-08T23:00:00+08:00", false},
		{"2026-09-08T08:00:00+08:00", false},
	} {
		at, _ := time.Parse(time.RFC3339, tc.at)
		if got := quietHoursActive(quiet, at); got != tc.want {
			t.Fatalf("%s: %v != %v", tc.at, got, tc.want)
		}
	}
}

func TestAutonomyQuietWeekdaysValidateWithoutChangingLegacyDailyPolicy(t *testing.T) {
	value := defaultAutonomyConfiguration()
	base := map[string]any{"start": "22:00", "end": "08:00", "timezone": "Asia/Shanghai"}
	value["quiet_hours"] = base
	if err := validAutonomyConfiguration(value); err != nil {
		t.Fatal(err)
	}
	for _, days := range []any{[]any{float64(1), float64(5)}, []any{}, []any{float64(7)}, []any{float64(1), float64(1)}, []any{"1"}} {
		quiet := cloneMap(base)
		quiet["days"] = days
		value["quiet_hours"] = quiet
		err := validAutonomyConfiguration(value)
		valid := len(arrayValue(days)) == 2 && intValue(arrayValue(days)[1]) == 5
		if valid != (err == nil) {
			t.Fatal(days, err)
		}
	}
}

func TestAutonomySnapshotIncludesPolicyRevisionAndDecision(t *testing.T) {
	snapshot := autonomySnapshot("active", "moment", []any{"moment"}, "3", jsonBytes(map[string]any{"start": "22:00", "end": "08:00"}), nil, 2, 7)
	if stringValue(snapshot["mode"]) != "active" || stringValue(snapshot["action_type"]) != "moment" || intValue(snapshot["revision"]) != 7 || intValue(snapshot["concurrency_limit"]) != 2 {
		t.Fatalf("snapshot = %#v", snapshot)
	}
}
