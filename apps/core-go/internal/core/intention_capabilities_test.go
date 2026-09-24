package core

import (
	"testing"
	"time"
)

func TestIntentionIndependentToolPersistsAcrossDaysAndDoesNotConflatePlanWithResult(t *testing.T) {
	fixture := seedWardrobeToolFixture(t)
	arguments := map[string]any{"operation": "create", "goal": "拥有合适的短靴", "action": "安排虚拟购物购买短靴", "expected_outcome": "短靴已实际入柜", "reason": "她明确想穿短靴且现有衣柜没有合适物品"}
	created, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(intentionDecideCapabilityName, "create-boots-intention", arguments))
	if err != nil || created.Result.Status != "completed" {
		t.Fatalf("create intention: receipt=%#v err=%v", created, err)
	}
	id := stringValue(mapValue(created.Result.Output)["intention_id"])
	if id == "" || stringValue(mapValue(created.Result.Output)["status"]) != "candidate" {
		t.Fatalf("created intention has no pending identity: %#v", created.Result.Output)
	}
	repeated, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(intentionDecideCapabilityName, "create-boots-again", arguments))
	if err != nil || stringValue(mapValue(repeated.Result.Output)["intention_id"]) != id || mapValue(repeated.Result.Output)["reused"] != true {
		t.Fatalf("same active intention duplicated: receipt=%#v err=%v", repeated, err)
	}
	var count int
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.fluctlight_intentions WHERE fluctlight_id=$1`, fixture.fluctlightID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("duplicate intention rows=%d err=%v", count, err)
	}
	qualified, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(intentionDecideCapabilityName, "qualify-boots", map[string]any{"operation": "qualify", "intention_id": id, "reason": "准备在合适时间购物"}))
	if err != nil || stringValue(mapValue(qualified.Result.Output)["status"]) != "qualified" {
		t.Fatalf("qualify intention: receipt=%#v err=%v", qualified, err)
	}
	inspected, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(intentionInspectCapabilityName, "inspect-boots", map[string]any{"operation": "detail", "intention_id": id}))
	if err != nil {
		t.Fatal(err)
	}
	current := mapValue(mapValue(inspected.Result.Output)["intention"])
	if stringValue(current["status"]) != "qualified" {
		t.Fatalf("intention did not persist: %#v", current)
	}
	expiration, err := time.Parse(time.RFC3339Nano, stringValue(current["expiration"]))
	if err != nil || time.Until(expiration) < 24*time.Hour {
		t.Fatalf("uncompleted intention disappears after one day: expiration=%v err=%v", expiration, err)
	}
	var ownedBoots int
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND category='boots' AND source_kind='purchase_result'`, fixture.fluctlightID).Scan(&ownedBoots); err != nil || ownedBoots != 0 {
		t.Fatalf("plan created purchased boots: count=%d err=%v", ownedBoots, err)
	}
	if _, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(intentionDecideCapabilityName, "cancel-boots", map[string]any{"operation": "cancel", "intention_id": id, "reason": "决定暂不购买"})); err != nil {
		t.Fatal(err)
	}
	newDecision, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(intentionDecideCapabilityName, "create-boots-after-cancel", arguments))
	if err != nil || stringValue(mapValue(newDecision.Result.Output)["intention_id"]) == id {
		t.Fatalf("cancelled desire was permanently deduplicated: receipt=%#v err=%v", newDecision, err)
	}
}
