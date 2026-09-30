package core

import (
	"strings"
	"testing"
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
