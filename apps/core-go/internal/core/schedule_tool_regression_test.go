package core

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestSchedulePlannerOnlyOffersExistingIntentionLinks(t *testing.T) {
	input := SchedulePlanInput{Schedule: map[string]any{"items": []any{
		map[string]any{"id": "item-a", "intention_id": "intention-a"},
		map[string]any{"id": "item-b"},
	}}, TargetEdit: map[string]any{"item_id": "item-b"}}
	schema := schedulePlannerOutputSchemaForInput(input)
	properties := mapValue(mapValue(mapValue(mapValue(schema["properties"])["items"])["items"])["properties"])
	links := arrayValue(mapValue(properties["intention_id"])["enum"])
	selections := arrayValue(mapValue(properties["source_item_id"])["enum"])
	if !containsStringValue(links, "intention-a") || containsStringValue(links, "intention-invented") || !containsStringValue(selections, "item-b") {
		t.Fatalf("planner selection enums: links=%#v source items=%#v", links, selections)
	}
}

func TestSchedulePlannerSeparatesUnknownLinkFromMissingStoredPlan(t *testing.T) {
	current := map[string]any{"items": []any{map[string]any{"intention_id": "known", "action_plan": map[string]any{}}}}
	unknown := map[string]any{"items": []any{map[string]any{"intention_id": "invented"}}}
	if err := preserveScheduledActionPlans(unknown, current); err == nil || err.Error() != "schedule_replan_intention_link_unknown" {
		t.Fatalf("unknown planner link should be diagnosed: %v", err)
	}
	known := map[string]any{"items": []any{map[string]any{"intention_id": "known"}}}
	if err := preserveScheduledActionPlans(known, current); err == nil || err.Error() != "schedule_replan_action_plan_missing" {
		t.Fatalf("missing stored plan should be diagnosed separately: %v", err)
	}
}

func TestScheduleInspectionOmitsExecutableActionPlan(t *testing.T) {
	item := scheduleInspectionItem(map[string]any{"id": "item-a", "activity": "染发", "intention_id": "intention-a", "action_plan": map[string]any{"desired_hair_color": "pink"}})
	if stringValue(item["id"]) != "item-a" || stringValue(item["intention_id"]) != "intention-a" || item["action_plan"] != nil {
		t.Fatalf("inspection selection or plan projection invalid: %#v", item)
	}
}

func TestScheduleEditRequiresConcreteMoveAndRevisionChanges(t *testing.T) {
	if err := validateScheduleEditArguments(map[string]any{"operation": "move", "changes": map[string]any{"start_at": "invalid"}}); err == nil || !strings.Contains(err.Error(), "schedule_edit_time_invalid") {
		t.Fatalf("move without valid interval accepted: %v", err)
	}
	if err := validateScheduleEditArguments(map[string]any{"operation": "revise", "changes": map[string]any{}}); err == nil || !strings.Contains(err.Error(), "schedule_edit_changes_required") {
		t.Fatalf("empty revision accepted: %v", err)
	}
}

func TestScheduleEditCanReviseInterruptibleCurrentRemainderPreservingHistory(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	at := time.Date(2026, 10, 6, 2, 0, 0, 0, time.UTC)
	f.app.Clock = fixedClock(at)
	life := currentLifeForTest(t, f.ctx, f.app, f.fluctlightID, at)
	baseline := fullDaySchedulePayloadForTest(at, "current-edit-baseline", stringValue(life["context_revision"]))
	if _, err := f.app.AcceptSchedule(f.ctx, f.ownerID, f.fluctlightID, baseline); err != nil {
		t.Fatal(err)
	}
	schedule, err := f.app.currentAcceptedSchedule(f.ctx, f.fluctlightID)
	if err != nil {
		t.Fatal(err)
	}
	target := mapValue(arrayValue(schedule["items"])[0])
	id := stringValue(target["id"])
	f.app.SchedulePlanner = scheduledPlannerFunc(func(_ context.Context, input SchedulePlanInput) (map[string]any, error) {
		history := cloneMap(mapValue(arrayValue(input.Schedule["items"])[0]))
		delete(history, "id")
		history["end_at"] = input.Schedule["completed_before"]
		remainder := cloneMap(target)
		delete(remainder, "id")
		remainder["start_at"] = input.Schedule["completed_before"]
		remainder["activity"] = "画画"
		remainder["source_item_id"] = id
		return map[string]any{"reschedule_policy": map[string]any{}, "items": []any{history, remainder}}, nil
	})
	f.app.Capabilities = nil
	f.app.Runtime = nil
	receipt, err := f.app.ExecuteTool(f.ctx, f.request(scheduleEditCapabilityName, "current-edit", map[string]any{"operation": "revise", "item_id": id, "expected_revision": 1, "intent": "改排当前剩余时间为画画", "reason": "当前阅读可以中断", "changes": map[string]any{"activity": "画画"}}))
	if err != nil {
		t.Fatal(err)
	}
	current, life, err := f.app.readLifeContextSnapshotAt(f.ctx, f.fluctlightID, at)
	if err != nil {
		t.Fatal(err)
	}
	items := arrayValue(current["items"])
	if intValue(current["revision"]) != 2 || len(items) != 2 || mapValue(items[0])["activity"] != "阅读" || life["activity"] != "画画" || receipt.Result.Status != "completed" {
		t.Fatalf("current edit lost history or effectiveness %#v %#v", current, life)
	}
}
