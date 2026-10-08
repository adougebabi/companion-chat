package core

import (
	"context"
	"github.com/jackc/pgx/v5"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestScheduledAcquisitionGoalClockIndependentWearAndFinalWorkflow(t *testing.T) {
	for _, object := range []bool{false, true} {
		name := "boots"
		if object {
			name = "brush"
		}
		t.Run(name, func(t *testing.T) {
			f := newIndependentToolE2EFixture(t, "goal-chain")
			seedControlledCurrentCapture(t, f.ctx, f.repository, f.fluctlightID)
			if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.fluctlight_personality_runtime(fluctlight_id,active_profile_id,revision) VALUES($1,'default',0)`, f.fluctlightID); err != nil {
				t.Fatal(err)
			}
			at := time.Date(2026, 10, 5, 2, 0, 0, 0, time.UTC)
			f.app.Clock = func() time.Time { return at }
			life := currentLifeForTest(t, f.ctx, f.app, f.fluctlightID, at)
			baseline := fullDaySchedulePayloadForTest(at, "chain-baseline", stringValue(life["context_revision"]))
			if _, err := f.app.AcceptSchedule(f.ctx, f.ownerID, f.fluctlightID, baseline); err != nil {
				t.Fatal(err)
			}
			member := map[string]any{"category": "boots", "slot": "shoes", "description": "黑色短靴"}
			if object {
				member = map[string]any{"item_kind": "object", "category": "art_supply", "description": "细头画笔"}
			}
			query, err := f.app.ExecuteTool(f.ctx, f.request(wardrobeInspectCapabilityName, "chain-query", map[string]any{"operation": "list", "category": member["category"]}))
			if err != nil || len(arrayValue(mapValue(query.Result.Output)["items"])) != 0 {
				t.Fatalf("missing query %#v %v", query, err)
			}
			args := map[string]any{"operation": "create", "goal": "获得" + stringValue(member["description"]), "action": "去商场购买" + stringValue(member["description"]), "expected_outcome": "实际持有" + stringValue(member["description"]), "reason": "缺少所需物品"}
			created, err := f.app.ExecuteTool(f.ctx, f.request(intentionDecideCapabilityName, "chain-goal", args))
			if err != nil {
				t.Fatal(err)
			}
			goalID := stringValue(mapValue(created.Result.Output)["goal_id"])
			intentionID := stringValue(mapValue(created.Result.Output)["intention_id"])
			start := at.Add(time.Hour)
			end := start.Add(30 * time.Minute)
			dayStart, _ := parseScheduleTime(stringValue(mapValue(arrayValue(baseline["items"])[0])["start_at"]))
			dayEnd := dayStart.AddDate(0, 0, 1)
			item := func(from, to time.Time, activity, scene string, selected bool) map[string]any {
				v := map[string]any{"start_at": formatInstant(from), "end_at": formatInstant(to), "activity": activity, "scene": scene, "item_type": "planned", "status": "planned", "priority": 0.5, "flexibility": 0.5, "interruption_cost": 0.5}
				if selected {
					v["planned_action_slot"] = true
				}
				return v
			}
			f.app.SchedulePlanner = scheduledPlannerFunc(func(_ context.Context, input SchedulePlanInput) (map[string]any, error) {
				boundary, err := parseScheduleTime(stringValue(input.Schedule["completed_before"]))
				if err != nil {
					return nil, err
				}
				return map[string]any{"reschedule_policy": map[string]any{}, "items": []any{item(dayStart, boundary, "阅读", "书房", false), item(boundary, start, "阅读", "书房", false), item(start, end, "购买所需物品", "商场", true), item(end, dayEnd, "阅读", "书房", false)}}, nil
			})
			f.app.Capabilities = nil
			f.app.Runtime = nil
			plan := cloneMap(member)
			plan["kind"] = "virtual_shopping"
			plan["duration_minutes"] = 15
			scheduleArgs := cloneMap(args)
			delete(scheduleArgs, "operation")
			scheduleArgs["action_plan"] = plan
			scheduled, err := f.app.ExecuteTool(f.ctx, f.request(scheduleActivityCapabilityName, "chain-schedule", scheduleArgs))
			if err != nil {
				t.Fatal(err)
			}
			if stringValue(mapValue(scheduled.Result.Output)["intention_id"]) != intentionID {
				t.Fatal("schedule duplicated goal/intention", scheduled)
			}
			if early, err := f.app.ProcessIntentionTrigger(f.ctx, intentionID); err != nil || early["status"] != "pending" {
				t.Fatalf("early %#v %v", early, err)
			}
			setupVirtualActivityTestProvider(t, f, map[string]any{"status": "completed", "reason": "实际获取成功", "acquired_item": member})
			at = start
			trigger, err := f.app.ProcessIntentionTrigger(f.ctx, intentionID)
			if err != nil || trigger["status"] != "activity_started" {
				t.Fatalf("due %#v %v", trigger, err)
			}
			activityID := stringValue(trigger["activity_id"])
			before, _, _, err := f.app.readEffectiveLifeSnapshot(f.ctx, f.fluctlightID, at)
			if err != nil {
				t.Fatal(err)
			}
			pending, err := f.app.ExecuteTool(f.ctx, f.request(lifeActivityAdvanceCapabilityName, "chain-early-result", map[string]any{"activity_id": activityID}))
			if err != nil || pending.Result.Status != "accepted" {
				t.Fatalf("early result %#v %v", pending, err)
			}
			at = start.Add(15 * time.Minute)
			request := f.request(lifeActivityAdvanceCapabilityName, "chain-acquire", map[string]any{"activity_id": activityID})
			result, err := f.app.ExecuteTool(f.ctx, request)
			if err != nil {
				t.Fatal(err)
			}
			itemID := stringValue(mapValue(result.Result.Output)["item_id"])
			if itemID == "" {
				t.Fatal(result)
			}
			if replay, err := f.app.ExecuteTool(f.ctx, request); err != nil || !replay.Replayed {
				t.Fatalf("acquisition replay %#v %v", replay, err)
			}
			after, _, _, err := f.app.readEffectiveLifeSnapshot(f.ctx, f.fluctlightID, at)
			if err != nil {
				t.Fatal(err)
			}
			if jsonString(before["worn_items"]) != jsonString(after["worn_items"]) || len(arrayValue(after["used_items"])) != 0 {
				t.Fatal("acquisition changed actual wear/use")
			}
			runGoalAssessmentFixture(t, f, "acquisition", true, nil)
			logGoalClosureEvidence(t, f, goalID, "actual_acquisition_separate_from_wearing")
			var goalStatus string
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status FROM public.fluctlight_goals WHERE id=$1`, goalID).Scan(&goalStatus); err != nil || goalStatus != "completed" {
				t.Fatalf("goal %s %v", goalStatus, err)
			}
			useName := wardrobeWearCapabilityName
			useArgs := map[string]any{"mode": "partial", "item_ids": []any{itemID}}
			if object {
				useName = itemUseCapabilityName
				useArgs = map[string]any{"operation": "start", "item_id": itemID, "activity": "画画"}
			}
			used, err := f.app.ExecuteTool(f.ctx, f.request(useName, "chain-independent-use", useArgs))
			if err != nil {
				t.Fatal(err)
			}
			projection, err := f.app.BuildContextProjection(f.ctx, f.ownerID, f.fluctlightID, f.conversationID, "chain-capture", "拍现在的照片")
			if err != nil {
				t.Fatal(err)
			}
			concept := map[string]any{"capture": map[string]any{"mode": "mirror_selfie", "camera": "rear", "framing": "upper body", "device_visibility": "visible"}, "context_binding": map[string]any{"appearance": projection.EffectiveAppearance, "current_life": projection.LifeContext}}
			style := captureStyleForTest()
			if object {
				style["pose"] = "holding_used_item"
			}
			prompt, err := renderCurrentCapturePrompt(concept, style)
			if err != nil {
				t.Fatal(err)
			}
			workflow := map[string]any{"positive": map[string]any{"class_type": "CLIPTextEncode", "inputs": map[string]any{"text": "{{prompt}}"}}, "sampler": map[string]any{"class_type": "KSampler", "inputs": map[string]any{"positive": []any{"positive", 0}}}}
			if err := validateCurrentCaptureWorkflow(workflow); err != nil {
				t.Fatal(err)
			}
			final, err := replaceMediaPlaceholders(workflow, prompt, map[string]any{})
			if err != nil || !strings.Contains(jsonString(final), stringValue(member["description"])) {
				t.Fatalf("final workflow %#v %v", final, err)
			}
			t.Logf("CHAIN_EVIDENCE=%s", jsonString(map[string]any{"goal_id": goalID, "intention_id": intentionID, "schedule": scheduled.Result.Output, "activity_id": activityID, "acquisition": result.Result.Output, "use": used.Result.Output, "capture_snapshot": projection.EffectiveAppearance, "final_workflow": final}))
		})
	}
}

func TestInitializedSharedRelationshipGoalCanCreateAndPauseNextStepWithoutDuplicatingGoal(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		return f.app.insertAgency(f.ctx, tx, f.fluctlightID, f.ownerID, []any{map[string]any{"description": "建立双方愿意的亲密关系", "scope": "relationship", "success_criteria": []any{"双方明确同意"}}}, nil, map[string]struct{}{"default": {}})
	}); err != nil {
		t.Fatal(err)
	}
	projection, err := f.app.BuildContextProjection(f.ctx, f.ownerID, f.fluctlightID, f.conversationID, "relationship-source", "我想找一个伴侣")
	if err != nil {
		t.Fatal(err)
	}
	if len(projection.Goals) != 1 || stringValue(projection.Goals[0]["profile_id"]) != "" {
		t.Fatal("initial shared goal not projected", projection.Goals)
	}
	goalRef := stringValue(projection.Goals[0]["ref"])
	request := f.request(intentionDecideCapabilityName, "shared-next-step", map[string]any{"operation": "create", "goal_ref": goalRef, "action": "温和地询问彼此对关系的期待", "expected_outcome": "获得真实回应", "reason": "有明确的关系目标与用户信号"})
	seedCognitiveProviderRole(t, f.ctx, f.repository, "shared-goal-provider-"+f.suffix)
	calls := 0
	router := newFakeProviderRouter().on("shared_goal_action", func(map[string]any) fakeProviderResult {
		calls++
		if calls == 1 {
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("shared-goal-native", intentionDecideCapabilityName, decodeObject(request.Arguments))}}
		}
		return fakeProviderResult{Structured: map[string]any{"answer": "done"}}
	})
	f.app.Provider.HTTP = &http.Client{Transport: router}
	run, err := f.app.RunFormalAgent(f.ctx, FormalAgentConversationCognition, FormalAgentRunInput{Prompt: PromptAssemblyResult{Messages: []map[string]any{{"role": "system", "content": "Create the next step for the existing shared Goal, then return the supplied final DTO."}, {"role": "user", "content": "为现有关系目标建立可行下一步"}}, ResponseFormat: objectSchema(map[string]any{"answer": stringSchema()}, []string{"answer"}, false)}, Definitions: []CapabilityDefinition{intentionDecideDefinition()}, SchemaName: "shared_goal_action", Capability: &ADKCapabilityRequest{Projection: projection, AuthorizationActorID: f.ownerID, FluctlightID: f.fluctlightID, ConversationID: f.conversationID, SourceFactID: "relationship-source", OperationID: "shared-native-root", ActionID: "shared-native-action", Surface: CapabilitySurfaceConversation}})
	if err != nil {
		t.Fatal(err)
	}
	_, results := run.Trace.Snapshot()
	if len(results) != 1 || results[0].Status != "completed" || calls != 2 {
		t.Fatalf("shared goal native loop %#v calls=%d", results, calls)
	}
	created := ToolExecutionReceipt{Result: results[0]}
	id := stringValue(mapValue(created.Result.Output)["intention_id"])
	inspected, err := f.app.ExecuteTool(f.ctx, f.request(intentionInspectCapabilityName, "shared-inspect", map[string]any{"operation": "detail", "intention_id": id}))
	if err != nil || mapValue(mapValue(inspected.Result.Output)["intention"])["action"] != "温和地询问彼此对关系的期待" {
		t.Fatalf("shared next step hidden %#v %v", inspected, err)
	}
	if _, err := f.app.ExecuteTool(f.ctx, f.request(intentionDecideCapabilityName, "shared-qualify", map[string]any{"operation": "qualify", "intention_id": id, "reason": "正式对话能力可用，可在适当情境询问"})); err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.ExecuteTool(f.ctx, f.request(intentionDecideCapabilityName, "shared-wait", map[string]any{"operation": "pause", "intention_id": id, "reason": "用户未回复，合理等待"})); err != nil {
		t.Fatal(err)
	}
	goals, _, err := f.app.agencyProfile(f.ctx, f.fluctlightID)
	if err != nil || len(goals) != 1 || mapValue(goals[0]["execution"])["stage"] != "waiting" {
		t.Fatalf("waiting lost persistent next step %#v %v", goals, err)
	}
	var count int
	var goalProfile *string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.fluctlight_goals WHERE fluctlight_id=$1`, f.fluctlightID).Scan(&count); err != nil || count != 1 {
		t.Fatal("created second relationship goal", count, err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT profile_id FROM public.fluctlight_goals WHERE fluctlight_id=$1`, f.fluctlightID).Scan(&goalProfile); err != nil || goalProfile != nil {
		t.Fatal("shared scope converted to default profile", goalProfile, err)
	}
}
