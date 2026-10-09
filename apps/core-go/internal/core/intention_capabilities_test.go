package core

import (
	"fmt"
	"testing"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/capability"
)

func TestIntentionToolSchemasAllowUniqueTargetInference(t *testing.T) {
	for _, definition := range []CapabilityDefinition{intentionInspectDefinition(), intentionDecideDefinition()} {
		if err := definition.Validate(); err != nil {
			t.Fatalf("%s definition invalid: %v", definition.Name, err)
		}
	}
	cases := []struct {
		name       string
		definition CapabilityDefinition
		arguments  map[string]any
		valid      bool
	}{
		{"list needs no id", intentionInspectDefinition(), map[string]any{"operation": "list"}, true},
		{"list continuation cursor", intentionInspectDefinition(), map[string]any{"operation": "list", "cursor": 10, "include_closed": false}, true},
		{"list rejects negative cursor", intentionInspectDefinition(), map[string]any{"operation": "list", "cursor": -1}, false},
		{"detail may resolve unique target", intentionInspectDefinition(), map[string]any{"operation": "detail"}, true},
		{"detail with id", intentionInspectDefinition(), map[string]any{"operation": "detail", "intention_id": "intent-1"}, true},
		{"decision needs operation", intentionDecideDefinition(), map[string]any{"reason": "asked"}, false},
		{"create with semantics", intentionDecideDefinition(), map[string]any{"operation": "create", "reason": "asked", "goal": "get boots", "action": "shop", "expected_outcome": "boots owned"}, true},
		{"update may resolve unique target", intentionDecideDefinition(), map[string]any{"operation": "update", "reason": "changed", "action": "shop later"}, true},
		{"update accepts two patches", intentionDecideDefinition(), map[string]any{"operation": "update", "reason": "changed", "intention_id": "intent-1", "action": "shop later", "expected_outcome": "boots owned"}, true},
		{"pause with id", intentionDecideDefinition(), map[string]any{"operation": "pause", "reason": "wait", "intention_id": "intent-1"}, true},
	}
	for _, item := range cases {
		t.Run(item.name, func(t *testing.T) {
			err := capability.ValidateCapabilitySchemaValue(item.arguments, item.definition.InputSchema)
			if (err == nil) != item.valid {
				t.Fatalf("valid=%v err=%v arguments=%#v", item.valid, err, item.arguments)
			}
		})
	}
}

func TestIntentionInspectListCanReadEveryPageByCursor(t *testing.T) {
	fixture := seedWardrobeToolFixture(t)
	_ = plannerOwnerGoal(t, fixture, "pagination-goal", false)
	for index := 0; index < 12; index++ {
		created, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(intentionDecideCapabilityName, fmt.Sprintf("page-create-%02d", index), map[string]any{
			"operation": "create", "goal": "pagination-goal", "action": fmt.Sprintf("行动 %02d", index),
			"expected_outcome": fmt.Sprintf("结果 %02d", index), "reason": "分页验证",
		}))
		if err != nil || created.Result.Status != "completed" {
			t.Fatalf("create %d: result=%#v err=%v", index, created.Result, err)
		}
	}
	first, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(intentionInspectCapabilityName, "page-first", map[string]any{"operation": "list"}))
	if err != nil {
		t.Fatal(err)
	}
	firstOutput := mapValue(first.Result.Output)
	if len(arrayValue(firstOutput["intentions"])) != 10 || firstOutput["has_more"] != true || intValue(firstOutput["next_cursor"]) != 10 {
		t.Fatalf("first page = %#v", firstOutput)
	}
	second, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(intentionInspectCapabilityName, "page-second", map[string]any{"operation": "list", "cursor": firstOutput["next_cursor"]}))
	if err != nil {
		t.Fatal(err)
	}
	secondOutput := mapValue(second.Result.Output)
	if len(arrayValue(secondOutput["intentions"])) != 2 || secondOutput["has_more"] != false || intValue(secondOutput["next_cursor"]) != 12 {
		t.Fatalf("second page = %#v", secondOutput)
	}
	seen := map[string]struct{}{}
	for _, output := range []map[string]any{firstOutput, secondOutput} {
		for _, raw := range arrayValue(output["intentions"]) {
			id := stringValue(mapValue(raw)["id"])
			if id == "" {
				t.Fatalf("missing true intention ID: %#v", raw)
			}
			if _, duplicated := seen[id]; duplicated {
				t.Fatalf("duplicate intention across pages: %s", id)
			}
			seen[id] = struct{}{}
		}
	}
}

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
	qualified, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(intentionDecideCapabilityName, "qualify-boots", map[string]any{"operation": "qualify", "reason": "准备在合适时间购物"}))
	if err != nil || stringValue(mapValue(qualified.Result.Output)["status"]) != "qualified" {
		t.Fatalf("qualify intention: receipt=%#v err=%v", qualified, err)
	}
	updated, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(intentionDecideCapabilityName, "update-boots", map[string]any{"operation": "update", "action": "安排购买一双合适的短靴", "reason": "细化下一步行动"}))
	if err != nil || stringValue(mapValue(updated.Result.Output)["intention_id"]) != id {
		t.Fatalf("unique target update failed: receipt=%#v err=%v", updated, err)
	}
	inspected, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(intentionInspectCapabilityName, "inspect-boots", map[string]any{"operation": "detail"}))
	if err != nil {
		t.Fatal(err)
	}
	current := mapValue(mapValue(inspected.Result.Output)["intention"])
	if stringValue(current["status"]) != "qualified" {
		t.Fatalf("intention did not persist: %#v", current)
	}
	if stringValue(current["action"]) != "安排购买一双合适的短靴" {
		t.Fatalf("unique target update did not persist: %#v", current)
	}
	expiration, err := time.Parse(time.RFC3339Nano, stringValue(current["expiration"]))
	if err != nil || time.Until(expiration) < 24*time.Hour {
		t.Fatalf("uncompleted intention disappears after one day: expiration=%v err=%v", expiration, err)
	}
	var ownedBoots int
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND category='boots' AND source_kind='purchase_result'`, fixture.fluctlightID).Scan(&ownedBoots); err != nil || ownedBoots != 0 {
		t.Fatalf("plan created purchased boots: count=%d err=%v", ownedBoots, err)
	}
	second, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(intentionDecideCapabilityName, "create-second-intention", map[string]any{
		"operation": "create", "goal": "整理书架", "action": "先分类书籍", "expected_outcome": "书籍已归类", "reason": "需要整理空间",
	}))
	if err != nil || second.Result.Status != "completed" {
		t.Fatalf("create second intention: receipt=%#v err=%v", second, err)
	}
	ambiguous, ambiguousErr := fixture.app.ExecuteTool(fixture.ctx, fixture.request(intentionDecideCapabilityName, "pause-ambiguous-intention", map[string]any{"operation": "pause", "reason": "暂缓"}))
	if ambiguousErr == nil || ambiguous.Result.ErrorCode != "intention_selection_required" || ambiguous.Result.Retryable {
		t.Fatalf("ambiguous target must ask for selection: receipt=%#v err=%v", ambiguous, ambiguousErr)
	}
	if _, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(intentionDecideCapabilityName, "cancel-boots", map[string]any{"operation": "cancel", "intention_id": id, "reason": "决定暂不购买"})); err != nil {
		t.Fatal(err)
	}
	newDecision, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(intentionDecideCapabilityName, "create-boots-after-cancel", arguments))
	if err != nil || stringValue(mapValue(newDecision.Result.Output)["intention_id"]) == id {
		t.Fatalf("cancelled desire was permanently deduplicated: receipt=%#v err=%v", newDecision, err)
	}
}
