package core

import "testing"

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

func TestNormalizeScheduleResponseSplitsAmbiguousActivityAndScene(t *testing.T) {
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
	if len(items) != 4 {
		t.Fatalf("normalized items = %d, want 2 original items plus 2 split items", len(items))
	}
	if got := stringValue(mapValue(items[1])["activity"]); got != "大学课程" {
		t.Fatalf("first split activity = %q", got)
	}
	if got := stringValue(mapValue(items[1])["scene"]); got != "教室" {
		t.Fatalf("first split scene = %q", got)
	}
	if got := stringValue(mapValue(items[2])["activity"]); got != "自习" {
		t.Fatalf("second split activity = %q", got)
	}
	if got := stringValue(mapValue(items[2])["scene"]); got != "图书馆" {
		t.Fatalf("second split scene = %q", got)
	}
	if got := stringValue(mapValue(items[3])["activity"]); got != "看电影" || stringValue(mapValue(items[3])["scene"]) != "影院" {
		t.Fatalf("continuous movie item changed: %#v", items[3])
	}
}
