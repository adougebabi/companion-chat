package core

import (
	"strings"
	"testing"
	"time"
)

func TestInitialScheduleRepairReceivesRejectedCandidateAndExactError(t *testing.T) {
	calls := 0
	bad := map[string]any{"items": []any{generatedBoundaryItem(480), generatedBoundaryItem(480), generatedBoundaryItem(1440)}, "reschedule_policy": map[string]any{}}
	router := newFakeProviderRouter().on("schedule_response", func(payload map[string]any) fakeProviderResult {
		calls++
		if calls == 1 {
			return fakeProviderResult{Structured: bad}
		}
		wire := jsonString(payload["messages"])
		for _, part := range []string{"repair_feedback", "previous_response", "validation_error", "expected an integer greater than 480", "480"} {
			if !strings.Contains(wire, part) {
				t.Errorf("repair physical request missing %q", part)
			}
		}
		return fakeProviderResult{Structured: map[string]any{"items": []any{generatedBoundaryItem(1440)}, "reschedule_policy": map[string]any{}}}
	})
	ctx, repo, app := personaPreparationFixture(t, router)
	if _, err := app.ProcessPersonaPreparationIntent(ctx, latestPreparationID(t, ctx, repo)); err != nil {
		t.Fatal(err)
	}
	if _, err := app.EnsureCurrentDaySchedule(ctx, "prep-role"); err != nil {
		t.Fatal(err)
	}
	var defaults, accepted int
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FILTER(WHERE generated_from='default_initial_schedule'),count(*) FILTER(WHERE status='accepted') FROM public.life_schedules WHERE fluctlight_id='prep-role'`).Scan(&defaults, &accepted); err != nil || defaults != 0 || accepted != 1 || calls != 2 {
		t.Fatal(defaults, accepted, calls, err)
	}
}

func TestScheduleGenerationBoundariesExpandWithoutGuessing(t *testing.T) {
	item := func(end any) map[string]any {
		v := regressionScheduleItem("", "")
		delete(v, "start_at")
		delete(v, "end_at")
		v["end_minute"] = end
		return v
	}
	result, err := normalizeScheduleResponse(map[string]any{"items": []any{item(480), item(540), item(1440)}}, "2026-11-24", "Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	items := arrayValue(result["items"])
	if mapValue(items[0])["start_at"] != "2026-11-24T00:00:00+08:00" || mapValue(items[1])["start_at"] != mapValue(items[0])["end_at"] || mapValue(items[2])["end_at"] != "2026-11-25T00:00:00+08:00" {
		t.Fatal(items)
	}
	for _, bad := range []any{0, 480, 1441, 540.5, "rest"} {
		if _, err := normalizeScheduleResponse(map[string]any{"items": []any{item(480), item(bad)}}, "2026-11-24", "Asia/Shanghai"); err == nil {
			t.Fatal("accepted invalid boundary", bad)
		}
	}
	schema := scheduleGenerationResponseSchema()
	entry := mapValue(mapValue(mapValue(schema["properties"])["items"])["items"])
	for _, key := range []string{"end_minute", "activity", "scene", "item_type", "priority", "flexibility", "interruption_cost"} {
		if !containsSchemaRequired(entry, key) {
			t.Fatal("missing generated item field", key)
		}
	}
	if mapValue(entry["properties"])["start_at"] != nil || mapValue(entry["properties"])["end_at"] != nil {
		t.Fatal("model still repeats boundaries")
	}
	if !containsSchemaRequired(mapValue(mapValue(mapValue(scheduleResponseSchema()["properties"])["items"])["items"]), "start_at") {
		t.Fatal("generation changed the canonical timestamp schema")
	}
	if _, err := normalizeScheduleResponse(map[string]any{"items": []any{item(480), regressionScheduleItem("08:00", "24:00")}}, "2026-11-24", "Asia/Shanghai"); err == nil {
		t.Fatal("mixed minute/timestamp contracts accepted")
	}
	for _, tc := range []struct {
		day   string
		hours time.Duration
	}{{"2026-03-08", 23 * time.Hour}, {"2026-11-01", 25 * time.Hour}} {
		result, err := normalizeScheduleResponse(map[string]any{"items": []any{item(480), item(1440)}}, tc.day, "America/New_York")
		if err != nil {
			t.Fatal(err)
		}
		items := arrayValue(result["items"])
		start, _ := time.Parse(time.RFC3339, stringValue(mapValue(items[0])["start_at"]))
		end, _ := time.Parse(time.RFC3339, stringValue(mapValue(items[1])["end_at"]))
		if end.Sub(start) != tc.hours || mapValue(items[1])["start_at"] != mapValue(items[0])["end_at"] {
			t.Fatal("DST boundary changed coverage", tc.day, items)
		}
	}
}

func generatedBoundaryItem(end any) map[string]any {
	item := regressionScheduleItem("", "")
	delete(item, "start_at")
	delete(item, "end_at")
	item["end_minute"] = end
	delete(item, "status")
	return item
}
