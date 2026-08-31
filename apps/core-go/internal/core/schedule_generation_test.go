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
		t.Fatalf("normalized items = %d, want 2", got)
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
