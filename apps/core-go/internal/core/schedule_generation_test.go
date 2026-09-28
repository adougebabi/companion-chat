package core

import (
	"strings"
	"testing"
	"time"
)

func TestScheduleGenerationModelInputKeepsSemanticsWithoutStorageIDs(t *testing.T) {
	input := ScheduleGenerationTaskInput{
		LocalDate: "2026-09-28", Timezone: "Asia/Shanghai",
		LifeProfile:    map[string]any{"profile_id": "profile-private", "status": "active"},
		Goals:          []map[string]any{{"id": "goal_initial_fluctlight_1_1", "description": "练习钢琴", "desired_outcome": "练习钢琴", "evidence_refs": []any{"foundation:fluctlight_1"}, "revision": 1, "importance": 0.8}},
		Intentions:     []map[string]any{{"id": "intention_initial_fluctlight_1_1", "goal_id": "goal_initial_fluctlight_1_1", "action": "练习半小时", "action_intent": "练习半小时", "evidence_refs": []any{"foundation:fluctlight_1"}, "preferred_time": "18:00"}},
		RecentOutcomes: []map[string]any{{"id": "outcome-1", "request_digest": "secret-digest", "capability_name": "life.activity.start", "status": "completed", "observed": map[string]any{"resulting_state_ref": "state:ctx_secret", "activity": "练习"}}},
		CurrentLife:    map[string]any{"scene": "琴房", "context_revision": "life_ctx_123", "schedule_ref": "schedule:ctx_123"},
	}
	encoded := jsonString(scheduleGenerationModelInput(input))
	for _, forbidden := range []string{"goal_initial_", "intention_initial_", "foundation:fluctlight_", "outcome-1", "secret-digest", "life_ctx_", "schedule:ctx_", "profile-private", "\"description\"", "\"action\""} {
		if strings.Contains(encoded, forbidden) {
			t.Fatalf("schedule input contains %q: %s", forbidden, encoded)
		}
	}
	for _, necessary := range []string{"练习钢琴", "练习半小时", "琴房", "18:00", "completed"} {
		if !strings.Contains(encoded, necessary) {
			t.Fatalf("schedule input lost %q: %s", necessary, encoded)
		}
	}
}

func TestScheduleGenerationFinalProviderRequestOmitsStorageIDs(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	seedCognitiveProviderRole(t, ctx, repository, "schedule-generation-wire")
	var modelText string
	router := newFakeProviderRouter().on("schedule_response", func(payload map[string]any) fakeProviderResult {
		for _, raw := range arrayValue(payload["messages"]) {
			message := mapValue(raw)
			if stringValue(message["role"]) == "user" {
				modelText += stringValue(message["content"])
			}
		}
		return fakeProviderResult{Structured: map[string]any{"items": []any{map[string]any{
			"start_at": "2026-09-28T00:00:00+08:00", "end_at": "2026-09-29T00:00:00+08:00", "activity": "练习钢琴", "scene": "琴房", "location": "家", "item_type": "planned", "status": "planned", "priority": 0.5, "flexibility": 0.5, "interruption_cost": 0.5,
		}}, "reschedule_policy": map[string]any{}}}
	})
	app := newTestApp(t, repository, router)
	_, err := app.RunScheduleGenerationTask(ctx, ScheduleGenerationTaskInput{
		LocalDate: "2026-09-28", Timezone: "Asia/Shanghai",
		Goals:      []map[string]any{{"id": "goal_initial_fluctlight_private_1", "desired_outcome": "练习钢琴", "description": "练习钢琴", "evidence_refs": []any{"foundation:fluctlight_private"}}},
		Intentions: []map[string]any{{"id": "intention_initial_private", "action": "练习半小时", "action_intent": "练习半小时"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"goal_initial_", "intention_initial_", "foundation:fluctlight_", "\"description\"", "\"action\""} {
		if strings.Contains(modelText, forbidden) {
			t.Fatalf("final schedule Provider text retained %q: %s", forbidden, modelText)
		}
	}
	if !strings.Contains(modelText, "练习钢琴") || !strings.Contains(modelText, "练习半小时") || strings.HasPrefix(modelText, "{") {
		t.Fatalf("final schedule Provider text lost readable semantics: %s", modelText)
	}
}

func TestParseScheduleTimeAcceptsISOEndOfDay(t *testing.T) {
	location, err := time.LoadLocation("UTC")
	if err != nil {
		t.Fatal(err)
	}
	day := time.Date(2026, time.September, 2, 0, 0, 0, 0, location)
	parsed, err := parseScheduleTimeInLocation("2026-09-02T24:00:00Z", day, location)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, time.September, 3, 0, 0, 0, 0, location)
	if !parsed.Equal(want) {
		t.Fatalf("parsed end-of-day timestamp = %s, want %s", parsed, want)
	}
}

func TestNormalizeScheduleResponseRequiresContiguousLocalDay(t *testing.T) {
	result, err := normalizeScheduleResponse(map[string]any{
		"items": []any{
			map[string]any{"start_at": "2026-08-31T00:00:00+08:00", "end_at": "2026-08-31T12:00:00+08:00", "activity": "睡眠", "scene": "卧室"},
			map[string]any{"start_at": "2026-08-31T12:00:00+08:00", "end_at": "2026-09-01T00:00:00+08:00", "activity": "拍摄与自由活动", "scene": "上海街头"},
		},
	}, "2026-08-31", "Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	if got := len(arrayValue(result["items"])); got != 2 {
		t.Fatalf("normalized items = %d, want 2 continuous items", got)
	}

	_, err = normalizeScheduleResponse(map[string]any{
		"items": []any{
			map[string]any{"start_at": "2026-08-31T00:00:00+08:00", "end_at": "2026-08-31T11:00:00+08:00", "activity": "睡眠", "scene": "卧室"},
			map[string]any{"start_at": "2026-08-31T12:00:00+08:00", "end_at": "2026-09-01T00:00:00+08:00", "activity": "拍摄", "scene": "街头"},
		},
	}, "2026-08-31", "Asia/Shanghai")
	if err == nil {
		t.Fatal("expected a gap in schedule items to be rejected")
	}
}

func TestNormalizeScheduleResponsePreservesProviderSemanticLabelsWithoutInference(t *testing.T) {
	result, err := normalizeScheduleResponse(map[string]any{
		"items": []any{
			map[string]any{"start_at": "2026-08-31T00:00:00+08:00", "end_at": "2026-08-31T13:30:00+08:00", "activity": "睡眠", "scene": "卧室"},
			map[string]any{"start_at": "2026-08-31T13:30:00+08:00", "end_at": "2026-08-31T16:00:00+08:00", "activity": "大学课程/自习", "scene": "教室/图书馆"},
			map[string]any{"start_at": "2026-08-31T16:00:00+08:00", "end_at": "2026-09-01T00:00:00+08:00", "activity": "看电影", "scene": "影院"},
		},
	}, "2026-08-31", "Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	items := arrayValue(result["items"])
	if len(items) != 3 {
		t.Fatalf("normalized items = %d, want the three Provider-declared intervals", len(items))
	}
	if got := stringValue(mapValue(items[1])["activity"]); got != "大学课程/自习" {
		t.Fatalf("Provider activity label was reinterpreted: %q", got)
	}
	if got := stringValue(mapValue(items[1])["scene"]); got != "教室/图书馆" {
		t.Fatalf("Provider scene label was reinterpreted: %q", got)
	}
	if got := stringValue(mapValue(items[2])["activity"]); got != "看电影" || stringValue(mapValue(items[2])["scene"]) != "影院" {
		t.Fatalf("continuous movie item changed: %#v", items[2])
	}
}
