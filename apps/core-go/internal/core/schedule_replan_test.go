package core

import (
	"strings"
	"testing"
	"time"
)

func TestScheduleReplanManifestRequiresCompleteItemsAndCAS(t *testing.T) {
	manifest := scheduleReplanCapabilityManifest()
	for _, field := range []string{"local_date", "expected_revision", "completed_before", "items", "evidence_refs"} {
		if !containsSchemaRequired(manifest.Parameters, field) {
			t.Fatalf("schedule.replan missing required parameter %q: %#v", field, manifest.Parameters)
		}
	}
	item := mapValue(mapValue(manifest.Parameters["properties"])["items"])
	itemSchema := mapValue(item["items"])
	for _, field := range []string{"start_at", "end_at", "activity", "scene", "item_type", "status", "priority", "flexibility", "interruption_cost"} {
		if !containsSchemaRequired(itemSchema, field) {
			t.Fatalf("schedule.replan item missing required field %q: %#v", field, itemSchema)
		}
	}
	if _, ok := (&App{}).capabilityRegistry().Lookup("schedule.replan"); !ok {
		t.Fatal("schedule.replan is not registered in the runtime catalog")
	}
}

func TestValidateScheduleReplanItemsRejectsMissingSemanticFields(t *testing.T) {
	item := map[string]any{
		"start_at": "2026-09-08T00:00:00+08:00", "end_at": "2026-09-08T08:00:00+08:00",
		"activity": "睡眠", "scene": "卧室", "item_type": "planned", "status": "planned",
		"priority": 0.5, "flexibility": 0.5, "interruption_cost": 0.2,
	}
	if err := validateScheduleReplanItems([]any{item}); err != nil {
		t.Fatalf("valid item rejected: %v", err)
	}
	delete(item, "priority")
	if err := validateScheduleReplanItems([]any{item}); err == nil || !strings.Contains(err.Error(), `field "priority" is required`) {
		t.Fatalf("missing priority error = %v", err)
	}
}

func TestValidateCompletedScheduleHistoryPreservesCompletedItems(t *testing.T) {
	current := scheduleSnapshotForTest()
	boundary, _ := time.Parse(time.RFC3339, "2026-09-08T10:00:00+08:00")

	unchanged := scheduleItemsForTest(
		map[string]any{"start_at": "2026-09-08T00:00:00+08:00", "end_at": "2026-09-08T08:00:00+08:00", "activity": "睡眠", "scene": "卧室", "item_type": "planned", "status": "planned", "priority": 0.5, "flexibility": 0.5, "interruption_cost": 0.2},
		map[string]any{"start_at": "2026-09-08T08:00:00+08:00", "end_at": "2026-09-08T10:00:00+08:00", "activity": "早餐", "scene": "厨房", "item_type": "planned", "status": "planned", "priority": 0.4, "flexibility": 0.6, "interruption_cost": 0.2},
		map[string]any{"start_at": "2026-09-08T10:00:00+08:00", "end_at": "2026-09-08T14:00:00+08:00", "activity": "突发事件", "scene": "工作室", "item_type": "planned", "status": "planned", "priority": 0.9, "flexibility": 0.1, "interruption_cost": 0.8},
	)
	if err := validateCompletedScheduleHistory(current, map[string]any{"items": unchanged}, boundary); err != nil {
		t.Fatalf("unchanged completed history rejected: %v", err)
	}

	changed := scheduleItemsForTest(
		map[string]any{"start_at": "2026-09-08T00:00:00+08:00", "end_at": "2026-09-08T08:00:00+08:00", "activity": "睡眠", "scene": "卧室", "item_type": "planned", "status": "planned", "priority": 0.5, "flexibility": 0.5, "interruption_cost": 0.2},
		map[string]any{"start_at": "2026-09-08T08:00:00+08:00", "end_at": "2026-09-08T10:00:00+08:00", "activity": "被打断的早餐", "scene": "厨房", "item_type": "planned", "status": "planned", "priority": 0.4, "flexibility": 0.6, "interruption_cost": 0.2},
		map[string]any{"start_at": "2026-09-08T10:00:00+08:00", "end_at": "2026-09-08T14:00:00+08:00", "activity": "突发事件", "scene": "工作室", "item_type": "planned", "status": "planned", "priority": 0.9, "flexibility": 0.1, "interruption_cost": 0.8},
	)
	if err := validateCompletedScheduleHistory(current, map[string]any{"items": changed}, boundary); err == nil || !strings.Contains(err.Error(), "completed_history_changed") {
		t.Fatalf("changed completed history error = %v", err)
	}
}

func TestValidateCompletedScheduleHistoryAllowsCurrentItemToBeTruncated(t *testing.T) {
	current := map[string]any{
		"items": scheduleItemsForTest(
			map[string]any{"start_at": "2026-09-08T00:00:00+08:00", "end_at": "2026-09-08T08:00:00+08:00", "activity": "睡眠", "scene": "卧室", "item_type": "planned", "status": "planned", "priority": 0.5, "flexibility": 0.5, "interruption_cost": 0.2},
			map[string]any{"start_at": "2026-09-08T08:00:00+08:00", "end_at": "2026-09-08T12:00:00+08:00", "activity": "工作", "scene": "工作室", "item_type": "planned", "status": "planned", "priority": 0.7, "flexibility": 0.4, "interruption_cost": 0.6},
		),
	}
	boundary, _ := time.Parse(time.RFC3339, "2026-09-08T10:00:00+08:00")
	proposal := map[string]any{"items": scheduleItemsForTest(
		map[string]any{"start_at": "2026-09-08T00:00:00+08:00", "end_at": "2026-09-08T08:00:00+08:00", "activity": "睡眠", "scene": "卧室", "item_type": "planned", "status": "planned", "priority": 0.5, "flexibility": 0.5, "interruption_cost": 0.2},
		map[string]any{"start_at": "2026-09-08T08:00:00+08:00", "end_at": "2026-09-08T10:00:00+08:00", "activity": "工作", "scene": "工作室", "item_type": "planned", "status": "planned", "priority": 0.7, "flexibility": 0.4, "interruption_cost": 0.6},
		map[string]any{"start_at": "2026-09-08T10:00:00+08:00", "end_at": "2026-09-08T12:00:00+08:00", "activity": "突发事件", "scene": "医院", "item_type": "planned", "status": "planned", "priority": 1.0, "flexibility": 0.0, "interruption_cost": 1.0},
	)}
	if err := validateCompletedScheduleHistory(current, proposal, boundary); err != nil {
		t.Fatalf("truncated current item rejected: %v", err)
	}
}

func TestValidateScheduleReplanBoundaryRejectsStaleOrFutureBoundary(t *testing.T) {
	now := time.Date(2026, 9, 9, 12, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	if err := validateScheduleReplanBoundary(now.Add(-5*time.Minute), now); err != nil {
		t.Fatalf("boundary at the staleness tolerance should be accepted: %v", err)
	}
	if err := validateScheduleReplanBoundary(now.Add(-5*time.Minute-time.Second), now); err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("stale boundary error = %v", err)
	}
	if err := validateScheduleReplanBoundary(now.Add(30*time.Second), now); err != nil {
		t.Fatalf("boundary at the future tolerance should be accepted: %v", err)
	}
	if err := validateScheduleReplanBoundary(now.Add(30*time.Second+time.Second), now); err == nil || !strings.Contains(err.Error(), "future") {
		t.Fatalf("future boundary error = %v", err)
	}
}

func scheduleSnapshotForTest() map[string]any {
	return map[string]any{
		"local_date": "2026-09-08", "timezone": "Asia/Shanghai", "revision": 4,
		"items": scheduleItemsForTest(
			map[string]any{"start_at": "2026-09-08T00:00:00+08:00", "end_at": "2026-09-08T08:00:00+08:00", "activity": "睡眠", "scene": "卧室", "item_type": "planned", "status": "planned", "priority": "0.5", "flexibility": "0.5", "interruption_cost": "0.2"},
			map[string]any{"start_at": "2026-09-08T08:00:00+08:00", "end_at": "2026-09-08T10:00:00+08:00", "activity": "早餐", "scene": "厨房", "item_type": "planned", "status": "planned", "priority": "0.4", "flexibility": "0.6", "interruption_cost": "0.2"},
		),
	}
}

func scheduleItemsForTest(items ...map[string]any) []any {
	result := make([]any, len(items))
	for index, item := range items {
		result[index] = item
	}
	return result
}
