package core

import "testing"

func goalFormalAdapterCases() []formalToolAdapterCase {
	goalID := func(t *testing.T, f *formalToolAdapterFixture) string {
		t.Helper()
		var id string
		if err := f.repository.Pool().QueryRow(f.ctx, `SELECT id FROM public.fluctlight_goals WHERE fluctlight_id=$1 AND desired_outcome='adapter actual result'`, f.fluctlightID).Scan(&id); err != nil {
			t.Fatal(err)
		}
		return id
	}
	return []formalToolAdapterCase{
		{name: "goal.decide", surface: CapabilitySurfaceConversation, wantStatus: "completed", request: func(_ *testing.T, f *formalToolAdapterFixture, _ string) ToolExecutionRequest {
			return f.request("goal.decide", "goal-adapter-create", map[string]any{"operation": "create", "desired_outcome": "adapter actual result", "success_criteria": []string{"actual evidence"}, "motivation": "current need", "reason": "focused goal"})
		}, verify: func(t *testing.T, f *formalToolAdapterFixture, r ToolExecutionReceipt) {
			requireFormalAdapterSQLCountArgs(t, f, `SELECT count(*) FROM public.fluctlight_goal_revisions WHERE goal_id=$1 AND source='tool'`, 1, mapValue(r.Result.Output)["goal_id"])
		}},
		{name: "goal.inspect", surface: CapabilitySurfaceConversation, wantStatus: "completed", request: func(t *testing.T, f *formalToolAdapterFixture, _ string) ToolExecutionRequest {
			return f.request("goal.inspect", "goal-adapter-inspect", map[string]any{"operation": "detail", "goal_id": goalID(t, f)})
		}, verify: func(t *testing.T, _ *formalToolAdapterFixture, r ToolExecutionReceipt) {
			if mapValue(mapValue(r.Result.Output)["goal"])["desired_outcome"] != "adapter actual result" {
				t.Fatal("query did not reread actual Goal")
			}
		}},
		{name: "goal.evaluate", surface: CapabilitySurfaceConversation, wantStatus: "completed", request: func(t *testing.T, f *formalToolAdapterFixture, _ string) ToolExecutionRequest {
			return f.request("goal.evaluate", "goal-adapter-evaluate", map[string]any{"goal_id": goalID(t, f), "expected_revision": 1, "reason": "check actual evidence"})
		}, verify: func(t *testing.T, f *formalToolAdapterFixture, r ToolExecutionReceipt) {
			requireFormalAdapterSQLCountArgs(t, f, `SELECT count(*) FROM public.goal_evaluation_requests WHERE id=$1 AND status='pending'`, 1, mapValue(r.Result.Output)["evaluation_request_id"])
		}},
		{name: "goal.review", surface: CapabilitySurfaceConversation, wantStatus: "completed", request: func(_ *testing.T, f *formalToolAdapterFixture, _ string) ToolExecutionRequest {
			return f.request("goal.review", "goal-adapter-review", map[string]any{"reason": "review current local day"})
		}, verify: func(t *testing.T, f *formalToolAdapterFixture, _ ToolExecutionReceipt) {
			requireFormalAdapterSQLCountArgs(t, f, `SELECT count(*) FROM public.goal_reviews WHERE goal_id=$1 AND status='pending'`, 1, goalID(t, f))
		}},
		{name: "wardrobe.borrow", surface: CapabilitySurfaceConversation, wantStatus: "completed", request: func(_ *testing.T, f *formalToolAdapterFixture, _ string) ToolExecutionRequest {
			return f.request("wardrobe.borrow", "adapter-borrow", map[string]any{"lender": "authorized shop", "reason": "actual loan received", "items": []any{map[string]any{"category": "shirt", "slot": "top", "description": "adapter borrowed shirt"}}})
		}, verify: func(t *testing.T, f *formalToolAdapterFixture, _ ToolExecutionReceipt) {
			requireFormalAdapterSQLCountArgs(t, f, `SELECT count(*) FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND description='adapter borrowed shirt' AND ownership='borrowed' AND availability='available'`, 1, f.fluctlightID)
		}},
		{name: "wardrobe.return", surface: CapabilitySurfaceConversation, wantStatus: "completed", request: func(t *testing.T, f *formalToolAdapterFixture, _ string) ToolExecutionRequest {
			var id string
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT id FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND description='adapter borrowed shirt'`, f.fluctlightID).Scan(&id); err != nil {
				t.Fatal(err)
			}
			return f.request("wardrobe.return", "adapter-return", map[string]any{"reason": "actual return", "item_ids": []string{id}})
		}, verify: func(t *testing.T, f *formalToolAdapterFixture, _ ToolExecutionReceipt) {
			requireFormalAdapterSQLCountArgs(t, f, `SELECT count(*) FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND description='adapter borrowed shirt' AND availability='unavailable'`, 1, f.fluctlightID)
		}},
	}
}

func TestIndependentGoalToolsPersistAndDoNotCompleteFromCommands(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	request := f.request("goal.decide", "independent-goal-create", map[string]any{"operation": "create", "desired_outcome": "真实表达", "success_criteria": []string{"确已表达"}, "motivation": "原始动机", "reason": "形成明确目标"})
	receipt, err := f.app.ExecuteTool(f.ctx, request)
	if err != nil || receipt.Result.Status != "completed" {
		t.Fatalf("create %#v %v", receipt, err)
	}
	id := stringValue(mapValue(receipt.Result.Output)["goal_id"])
	if replay, err := f.app.ExecuteTool(f.ctx, request); err != nil || !replay.Replayed {
		t.Fatalf("replay %#v %v", replay, err)
	}
	for _, name := range []string{"goal.inspect", "goal.evaluate", "goal.review"} {
		args := map[string]any{"reason": "独立检查", "goal_id": id, "expected_revision": 1}
		if name == "goal.inspect" {
			args = map[string]any{"operation": "detail", "goal_id": id}
		}
		if name == "goal.review" {
			args = map[string]any{"reason": "独立检查"}
		}
		if result, err := f.app.ExecuteTool(f.ctx, f.request(name, "independent-"+name, args)); err != nil || result.Result.Status != "completed" {
			t.Fatalf("%s %#v %v", name, result, err)
		}
	}
	if _, err := f.app.ExecuteTool(f.ctx, f.request("goal.decide", "force-complete", map[string]any{"operation": "complete", "goal_id": id, "expected_revision": 1, "reason": "没有事实"})); err == nil {
		t.Fatal("forced completion accepted")
	}
	if _, err := f.app.ExecuteTool(f.ctx, f.request("goal.evaluate", "stale-assessment", map[string]any{"goal_id": id, "expected_revision": 0, "reason": "陈旧请求"})); err == nil {
		t.Fatal("stale assessment accepted")
	}
	var state string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status FROM public.fluctlight_goals WHERE id=$1`, id).Scan(&state); err != nil || state != "active" {
		t.Fatalf("command/queue completed Goal %s %v", state, err)
	}
}

func TestGoalCatalogWireCost(t *testing.T) {
	registry := mustCapabilityRegistry(builtinCapabilities(&App{})...)
	total := 0
	for _, definition := range registry.Catalog(CapabilitySurfaceConversation) {
		cost := EstimatePromptTokens(RenderCapabilityTools([]CapabilityDefinition{definition}))
		total += cost
		t.Logf("TOOL_WIRE_COST name=%s estimated=%d description_runes=%d", definition.Name, cost, len([]rune(definition.Description)))
	}
	t.Logf("TOOL_WIRE_COST total=%d", total)
}
