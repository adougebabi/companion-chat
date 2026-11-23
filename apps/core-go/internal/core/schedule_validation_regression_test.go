package core

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func regressionScheduleItem(start, end string) map[string]any {
	return map[string]any{"start_at": start, "end_at": end, "activity": "自由活动", "scene": "未安排", "item_type": "free", "status": "planned", "priority": 0.0, "flexibility": 1.0, "interruption_cost": 0.0}
}

func TestInitialScheduleNormalizesMechanicalFormattingAndSmallBoundaries(t *testing.T) {
	for _, boundary := range []string{"14:02", "13:58", "14:00"} {
		t.Run(boundary, func(t *testing.T) {
			result, err := normalizeScheduleResponse(map[string]any{"items": []any{
				regressionScheduleItem("00:00", "2026-11-2014:00:0"),
				regressionScheduleItem(boundary, "24:00"),
			}}, "2026-11-20", "Asia/Shanghai")
			if err != nil {
				t.Fatal(err)
			}
			items := arrayValue(result["items"])
			if mapValue(items[1])["start_at"] != mapValue(items[0])["end_at"] {
				t.Fatal("boundaries remain disconnected", items)
			}
			if mapValue(items[0])["end_at"] != "2026-11-20T14:00:00+08:00" {
				t.Fatal("incorrect timestamp normalization", items)
			}
		})
	}
}

func TestInitialScheduleValidationIdentifiesOriginalItemFieldAndExpectation(t *testing.T) {
	for _, tc := range []struct{ name, start, end, field, expect string }{
		{"letter", "2026-11-20 l18:00", "24:00", "start_at", "RFC3339"},
		{"end", "14:00", "bad", "end_at", "RFC3339"},
		{"gap", "14:10", "24:00", "start_at", "14:00"},
		{"overlap", "13:50", "24:00", "start_at", "14:00"},
		{"coverage", "14:00", "23:00", "end_at", "2026-11-21"},
		{"reversed", "14:00", "13:00", "end_at", "after start_at"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := normalizeScheduleResponse(map[string]any{"items": []any{regressionScheduleItem("00:00", "14:00"), regressionScheduleItem(tc.start, tc.end)}}, "2026-11-20", "Asia/Shanghai")
			if err == nil {
				t.Fatal("invalid schedule accepted")
			}
			for _, part := range []string{"items[2]." + tc.field, tc.expect} {
				if !strings.Contains(err.Error(), part) {
					t.Fatalf("missing %q in %v", part, err)
				}
			}
		})
	}
}

func TestInitialScheduleRejectsLargeOrDestructiveBoundaryRepair(t *testing.T) {
	_, err := normalizeScheduleResponse(map[string]any{"items": []any{regressionScheduleItem("00:00", "14:04"), regressionScheduleItem("14:00", "14:03"), regressionScheduleItem("14:03", "24:00")}}, "2026-11-20", "Asia/Shanghai")
	if err == nil {
		t.Fatal("alignment erased an interval")
	}
}

func TestInitialScheduleLocalSecondsPreserveDSTValidation(t *testing.T) {
	loc, _ := time.LoadLocation("America/New_York")
	day := time.Date(2026, 3, 8, 0, 0, 0, 0, loc)
	if _, err := parseScheduleTimeInLocation("2026-03-0802:30:0", day, loc); err == nil {
		t.Fatal("nonexistent local time accepted")
	}
}

func TestInitialSchedulePlaceFeedbackIdentifiesItemAndExpectedReference(t *testing.T) {
	for _, tc := range []struct {
		name        string
		fields      map[string]any
		code, field string
	}{
		{"required", map[string]any{"location": "琴房"}, "schedule_place_reference_required", "place_ref"},
		{"conflict", map[string]any{"place_ref": "one", "place_id": "two"}, "schedule_place_reference_conflict", "place_ref/place_id"},
		{"transit", map[string]any{"location_kind": "transit", "place_ref": "one"}, "schedule_transit_place_invalid", "place_ref/place_id"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			item := regressionScheduleItem("00:00", "24:00")
			for key, value := range tc.fields {
				item[key] = value
			}
			_, err := normalizeInitialScheduleResponse(map[string]any{"items": []any{item}}, "2026-11-20", "Asia/Shanghai", &scheduleWorldPlaceContext{WorldID: "world"})
			if err == nil {
				t.Fatal("invalid place reference accepted")
			}
			for _, part := range []string{"items[1]." + tc.field, tc.code, "expected"} {
				if !strings.Contains(err.Error(), part) {
					t.Fatalf("missing %q in %v", part, err)
				}
			}
		})
	}
}

func TestDefaultInitialScheduleIsMarkedUnplannedAndCoversDSTDay(t *testing.T) {
	for _, date := range []string{"2026-03-08", "2026-11-01"} {
		payload, err := defaultInitialSchedule(date, "America/New_York", fmt.Errorf("items[2].end_at: invalid"))
		if err != nil {
			t.Fatal(err)
		}
		policy := mapValue(payload["reschedule_policy"])
		if payload["generated_from"] != "default_initial_schedule" || policy["default_initial_schedule"] != true || policy["replaceable"] != true || !strings.Contains(stringValue(policy["validation_error"]), "items[2]") {
			t.Fatal("default is unmarked", payload)
		}
		items := arrayValue(payload["items"])
		if len(items) != 1 {
			t.Fatal("default invented routine", items)
		}
		item := mapValue(items[0])
		if item["item_type"] != "free" || item["location_kind"] != "unknown" || stringValue(item["location"]) != "" || stringValue(item["place_ref"]) != "" {
			t.Fatal("default invented location or sleep", item)
		}
		start, _ := time.Parse(time.RFC3339, stringValue(item["start_at"]))
		end, _ := time.Parse(time.RFC3339, stringValue(item["end_at"]))
		want := 23 * time.Hour
		if date == "2026-11-01" {
			want = 25 * time.Hour
		}
		if end.Sub(start) != want {
			t.Fatal("incorrect DST day coverage", item)
		}
	}
}
