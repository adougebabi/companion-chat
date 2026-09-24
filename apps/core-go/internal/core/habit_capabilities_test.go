package core

import (
	"strings"
	"testing"
)

func TestHabitDecisionUpdatesPortraitWithoutChangingWearingOrInventory(t *testing.T) {
	fixture := seedWardrobeToolFixture(t)
	initialWearing, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(wardrobeInspectCapabilityName, "habit-initial-wearing", map[string]any{"operation": "wearing"}))
	if err != nil {
		t.Fatal(err)
	}
	beforeWorn := len(arrayValue(mapValue(initialWearing.Result.Output)["items"]))
	appendReceipt, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(habitDecideCapabilityName, "habit-append", map[string]any{
		"operation": "append", "text": "近期日常更喜欢深色上衣", "reason": "明确决定调整自己的日常搭配偏好",
	}))
	if err != nil || appendReceipt.Result.Status != "completed" {
		t.Fatalf("habit decision did not commit: receipt=%#v err=%v", appendReceipt, err)
	}
	current, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(habitInspectCapabilityName, "habit-inspect", map[string]any{}))
	if err != nil || !strings.Contains(jsonString(current.Result.Output), "深色上衣") || intValue(mapValue(current.Result.Output)["revision"]) != 1 {
		t.Fatalf("effective habit query missed new decision: receipt=%#v err=%v", current, err)
	}
	var compiled []byte
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT compiled_json FROM public.fluctlight_working_personas WHERE fluctlight_id=$1 AND profile_id='default'`, fixture.fluctlightID).Scan(&compiled); err != nil || !strings.Contains(string(compiled), "深色上衣") {
		t.Fatalf("working persona did not incorporate the effective habit: %s err=%v", compiled, err)
	}
	afterWearing, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(wardrobeInspectCapabilityName, "habit-after-wearing", map[string]any{"operation": "wearing"}))
	if err != nil || len(arrayValue(mapValue(afterWearing.Result.Output)["items"])) != beforeWorn {
		t.Fatalf("habit decision changed current clothing: receipt=%#v err=%v", afterWearing, err)
	}
	var items int
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1`, fixture.fluctlightID).Scan(&items); err != nil || items != 4 {
		t.Fatalf("habit decision changed inventory: items=%d err=%v", items, err)
	}
	replaceReceipt, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(habitDecideCapabilityName, "habit-replace", map[string]any{
		"operation": "replace", "index": 0, "text": "近期日常更喜欢浅色上衣", "reason": "明确改变前一项日常搭配偏好",
	}))
	if err != nil || intValue(mapValue(replaceReceipt.Result.Output)["revision"]) != 2 {
		t.Fatalf("habit replacement did not commit: receipt=%#v err=%v", replaceReceipt, err)
	}
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT compiled_json FROM public.fluctlight_working_personas WHERE fluctlight_id=$1 AND profile_id='default'`, fixture.fluctlightID).Scan(&compiled); err != nil || !strings.Contains(string(compiled), "浅色上衣") || strings.Contains(string(compiled), "深色上衣") {
		t.Fatalf("old habit remained current in portrait: %s err=%v", compiled, err)
	}
}
