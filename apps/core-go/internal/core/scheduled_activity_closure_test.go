package core

import (
	"context"
	"net/http"
	"testing"
	"time"
)

type scheduledPlannerFunc func(context.Context, SchedulePlanInput) (map[string]any, error)

func (fn scheduledPlannerFunc) Plan(ctx context.Context, input SchedulePlanInput) (map[string]any, error) {
	return fn(ctx, input)
}

func TestIntentionScheduleCommitsLinkedFutureDyeWithoutChangingCurrentBody(t *testing.T) {
	fixture := seedWardrobeToolFixture(t)
	fixture.seedAcceptedSchedule(t)
	location, _ := time.LoadLocation("Asia/Shanghai")
	now := time.Now().In(location)
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	start := now.Add(5 * time.Minute).Truncate(time.Minute)
	end := start.Add(15 * time.Minute)
	dayEnd := dayStart.AddDate(0, 0, 1)
	if !end.Before(dayEnd) {
		t.Skip("not enough time remains in the local day for a future appointment")
	}
	item := func(from, to time.Time, activity, scene string, selected bool) map[string]any {
		value := map[string]any{"start_at": from.Format(time.RFC3339Nano), "end_at": to.Format(time.RFC3339Nano), "activity": activity, "scene": scene,
			"item_type": "planned", "status": "planned", "priority": 0.5, "flexibility": 0.5, "interruption_cost": 0.5}
		if selected {
			value["planned_action_slot"] = true
		}
		return value
	}
	fixture.app.SchedulePlanner = scheduledPlannerFunc(func(_ context.Context, input SchedulePlanInput) (map[string]any, error) {
		boundary, err := parseScheduleTime(stringValue(input.Schedule["completed_before"]))
		if err != nil {
			return nil, err
		}
		return map[string]any{"reschedule_policy": map[string]any{}, "items": []any{
			item(dayStart, boundary, "阅读", "书房", false), item(boundary, start, "阅读", "书房", false),
			item(start, end, "去染发", "理发店", true), item(end, dayEnd, "休息", "家", false),
		}}, nil
	})
	fixture.app.Capabilities = nil
	fixture.app.Runtime = nil
	request := fixture.request(scheduleActivityCapabilityName, "plan-hair-dye", map[string]any{
		"goal": "把头发染成粉色", "action": "安排去染发", "expected_outcome": "完成后头发为粉色", "reason": "用户提出染发",
		"action_plan": map[string]any{"kind": "hair_dye", "duration_minutes": 15, "desired_hair_color": "粉色"},
	})
	response, err := fixture.app.ExecuteTool(fixture.ctx, request)
	if err != nil || response.Result.Status != "completed" {
		t.Fatalf("schedule dye plan receipt=%#v err=%v", response, err)
	}
	output := mapValue(response.Result.Output)
	intentionID := stringValue(output["intention_id"])
	var intentionStatus, triggerType, scheduledItemID string
	var dueAt time.Time
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT status,trigger->>'type',(trigger->>'due_at')::timestamptz FROM public.fluctlight_intentions WHERE id=$1`, intentionID).Scan(&intentionStatus, &triggerType, &dueAt); err != nil {
		t.Fatal(err)
	}
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT id FROM public.life_schedule_items WHERE intention_id=$1`, intentionID).Scan(&scheduledItemID); err != nil {
		t.Fatal(err)
	}
	appearance, _, _, err := fixture.app.readEffectiveLifeSnapshot(fixture.ctx, fixture.fluctlightID, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	color := stringValue(mapValue(mapValue(appearance["body_fields"])["hair_color"])["value"])
	if intentionStatus != "qualified" || triggerType != "time" || !dueAt.Equal(start.UTC()) || scheduledItemID != stringValue(output["schedule_item_id"]) || color != "black" {
		t.Fatalf("planned dye state intention=%s/%s due=%s item=%q color=%q", intentionStatus, triggerType, dueAt, scheduledItemID, color)
	}
	again := fixture.request(scheduleActivityCapabilityName, "plan-hair-dye-again", map[string]any{
		"goal": "把头发染成粉色", "action": "安排去染发", "expected_outcome": "完成后头发为粉色", "reason": "用户提出染发",
		"action_plan": map[string]any{"kind": "hair_dye", "duration_minutes": 15, "desired_hair_color": "粉色"},
	})
	reused, err := fixture.app.ExecuteTool(fixture.ctx, again)
	if err != nil || stringValue(mapValue(reused.Result.Output)["intention_id"]) != intentionID || stringValue(mapValue(reused.Result.Output)["schedule_item_id"]) != scheduledItemID {
		t.Fatalf("repeat plan duplicated appointment: receipt=%#v err=%v", reused, err)
	}
	if result, err := fixture.app.ProcessIntentionTrigger(fixture.ctx, intentionID); err != nil || stringValue(result["status"]) != "pending" {
		t.Fatalf("future action started early: result=%#v err=%v", result, err)
	}
	plannedSchedule, currentLife, err := fixture.app.readLifeContextSnapshotAt(fixture.ctx, fixture.fluctlightID, time.Now().UTC())
	if err != nil || stringValue(currentLife["activity"]) != "阅读" || len(arrayValue(plannedSchedule["items"])) == 0 {
		t.Fatalf("future dye changed current schedule context: life=%#v err=%v", currentLife, err)
	}
	if _, err := fixture.app.AcceptSchedule(fixture.ctx, fixture.ownerID, fixture.fluctlightID, map[string]any{
		"local_date": dayStart.Format("2006-01-02"), "timezone": "Asia/Shanghai", "expected_revision": 2,
		"expected_life_context_revision": currentLife["context_revision"], "idempotency_key": "drop-planned-dye-" + fixture.suffix,
		"evidence_refs": []any{"owner:replan"}, "reschedule_policy": map[string]any{},
		"items": []any{item(dayStart, dayEnd, "阅读", "书房", false)},
	}); err == nil {
		t.Fatal("schedule replacement silently dropped a qualified future intention")
	}
	movedStart, movedEnd := start.Add(time.Minute), end.Add(time.Minute)
	if !movedEnd.Before(dayEnd) {
		t.Skip("not enough local-day room to replan appointment")
	}
	var linkedPlan map[string]any
	for _, raw := range arrayValue(plannedSchedule["items"]) {
		candidate := mapValue(raw)
		if stringValue(candidate["intention_id"]) == intentionID {
			linkedPlan = mapValue(candidate["action_plan"])
		}
	}
	if len(linkedPlan) == 0 {
		t.Fatal("accepted schedule lost action plan")
	}
	movedItem := item(movedStart, movedEnd, "去染发", "理发店", false)
	movedItem["intention_id"], movedItem["action_plan"] = intentionID, linkedPlan
	moved, err := fixture.app.AcceptSchedule(fixture.ctx, fixture.ownerID, fixture.fluctlightID, map[string]any{
		"local_date": dayStart.Format("2006-01-02"), "timezone": "Asia/Shanghai", "expected_revision": 2,
		"expected_life_context_revision": currentLife["context_revision"], "idempotency_key": "move-planned-dye-" + fixture.suffix,
		"evidence_refs": []any{"owner:move-dye"}, "reschedule_policy": map[string]any{},
		"items": []any{item(dayStart, movedStart, "阅读", "书房", false), movedItem, item(movedEnd, dayEnd, "休息", "家", false)},
	})
	if err != nil {
		t.Fatalf("moving linked appointment failed: %v", err)
	}
	oldItemID := scheduledItemID
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT id FROM public.life_schedule_items WHERE schedule_id=$1 AND intention_id=$2`, stringValue(moved["id"]), intentionID).Scan(&scheduledItemID); err != nil {
		t.Fatal(err)
	}
	if scheduledItemID == oldItemID {
		t.Fatal("new immutable schedule reused the old item identity")
	}
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT (trigger->>'due_at')::timestamptz FROM public.fluctlight_intentions WHERE id=$1`, intentionID).Scan(&dueAt); err != nil || !dueAt.Equal(movedStart.UTC()) {
		t.Fatalf("replanned trigger due=%s err=%v", dueAt, err)
	}
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `UPDATE public.life_schedule_items SET start_at=now()-interval '1 minute',end_at=now()+interval '15 minutes' WHERE id=$1`, oldItemID); err != nil {
		t.Fatal(err)
	}
	if stale, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(lifeActivityStartCapabilityName, "old-schedule-dye", map[string]any{
		"kind": "hair_dye", "intention_id": intentionID, "schedule_item_id": oldItemID, "duration_minutes": 15, "desired_hair_color": "粉色", "reason": "旧日程触发",
	})); err == nil {
		t.Fatalf("superseded schedule started an activity: %#v", stale)
	}
	forcedStart := time.Now().UTC().Add(-time.Minute)
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `UPDATE public.life_schedule_items SET end_at=$3 WHERE schedule_id=$1 AND end_at=$2 AND intention_id IS NULL`, stringValue(moved["id"]), movedStart, forcedStart); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `UPDATE public.life_schedule_items SET start_at=$2,end_at=now()+interval '20 minutes' WHERE id=$1`, scheduledItemID, forcedStart); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `UPDATE public.fluctlight_intentions SET trigger=jsonb_set(trigger,'{due_at}',to_jsonb($2::text),true),preferred_time=$3 WHERE id=$1`, intentionID, forcedStart.Format(time.RFC3339Nano), forcedStart); err != nil {
		t.Fatal(err)
	}
	started, err := fixture.app.ProcessIntentionTrigger(fixture.ctx, intentionID)
	if err != nil || stringValue(started["status"]) != "activity_started" || stringValue(started["activity_id"]) == "" {
		t.Fatalf("scheduled intention did not start activity: result=%#v err=%v", started, err)
	}
	activityID := stringValue(started["activity_id"])
	_, currentLife, err = fixture.app.readLifeContextSnapshotAt(fixture.ctx, fixture.fluctlightID, time.Now().UTC())
	if err != nil || stringValue(currentLife["source"]) != "event" || stringValue(currentLife["event_kind"]) != "life_activity" || stringValue(currentLife["schedule_item_id"]) != scheduledItemID || stringValue(currentLife["activity"]) != "去染发" || stringValue(currentLife["scene"]) != "理发店" {
		t.Fatalf("started dye conflicts with current schedule context: life=%#v err=%v", currentLife, err)
	}
	if replay, err := fixture.app.ProcessIntentionTrigger(fixture.ctx, intentionID); err != nil || stringValue(replay["activity_id"]) != activityID {
		t.Fatalf("due replay duplicated or lost activity: result=%#v err=%v", replay, err)
	}
	appearance, _, _, err = fixture.app.readEffectiveLifeSnapshot(fixture.ctx, fixture.fluctlightID, time.Now().UTC())
	if err != nil || stringValue(mapValue(mapValue(appearance["body_fields"])["hair_color"])["value"]) != "black" {
		t.Fatalf("activity start changed current color: appearance=%#v err=%v", appearance, err)
	}
	forceVirtualActivityDue(t, fixture, activityID)
	setupVirtualActivityTestProvider(t, fixture, map[string]any{"status": "completed", "reason": "不匹配的染发结果", "hair_color": "蓝色"})
	if invalid, err := fixture.app.ResolveScheduledLifeActivity(fixture.ctx, activityID); err == nil {
		t.Fatalf("mismatched dye result was accepted: %#v", invalid)
	}
	appearance, _, _, err = fixture.app.readEffectiveLifeSnapshot(fixture.ctx, fixture.fluctlightID, time.Now().UTC())
	if err != nil || stringValue(mapValue(mapValue(appearance["body_fields"])["hair_color"])["value"]) != "black" {
		t.Fatalf("invalid result changed hair color: appearance=%#v err=%v", appearance, err)
	}
	fixture.app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().on("virtual_activity_result", func(_ map[string]any) fakeProviderResult {
		return fakeProviderResult{Structured: map[string]any{"status": "completed", "reason": "虚拟染发完成", "hair_color": "粉色"}}
	})}
	resolved, err := fixture.app.ResolveScheduledLifeActivity(fixture.ctx, activityID)
	if err != nil || stringValue(resolved["status"]) != "completed" {
		t.Fatalf("scheduled dye did not resolve: result=%#v err=%v", resolved, err)
	}
	appearance, _, _, err = fixture.app.readEffectiveLifeSnapshot(fixture.ctx, fixture.fluctlightID, time.Now().UTC())
	if err != nil || stringValue(mapValue(mapValue(appearance["body_fields"])["hair_color"])["value"]) != "粉色" || stringValue(mapValue(mapValue(appearance["body_fields"])["hair_length"])["value"]) != "long" {
		t.Fatalf("completed dye changed wrong body fields: appearance=%#v err=%v", appearance, err)
	}
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT status FROM public.fluctlight_intentions WHERE id=$1`, intentionID).Scan(&intentionStatus); err != nil || intentionStatus != "completed" {
		t.Fatalf("completed dye left intention=%q err=%v", intentionStatus, err)
	}
	runGoalAssessmentFixture(t, fixture, "semantic", true, nil)
	var goalStatus string
	var goalProgress []byte
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT status,progress FROM public.fluctlight_goals WHERE id=$1`, stringValue(output["goal_id"])).Scan(&goalStatus, &goalProgress); err != nil || goalStatus != "completed" || numberOrZero(jsonNumber(goalProgress)) != 1 {
		t.Fatalf("completed dye left goal=%q progress=%s err=%v", goalStatus, goalProgress, err)
	}
}

func TestCancelledScheduledIntentionCannotStartDyeOrChangeColor(t *testing.T) {
	fixture := seedWardrobeToolFixture(t)
	fixture.seedAcceptedSchedule(t)
	created, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(intentionDecideCapabilityName, "cancelled-dye-intention", map[string]any{
		"operation": "create", "goal": "把头发染成粉色", "action": "去染发", "expected_outcome": "发色变成粉色", "reason": "先作计划",
	}))
	if err != nil {
		t.Fatal(err)
	}
	intentionID := stringValue(mapValue(created.Result.Output)["intention_id"])
	location, _ := time.LoadLocation("Asia/Shanghai")
	now := time.Now().In(location)
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	start := now.Add(5 * time.Minute).Truncate(time.Minute)
	end := start.Add(15 * time.Minute)
	dayEnd := dayStart.AddDate(0, 0, 1)
	if !end.Before(dayEnd) {
		t.Skip("not enough time remains in the local day")
	}
	_, life, err := fixture.app.readLifeContextSnapshotAt(fixture.ctx, fixture.fluctlightID, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	item := func(from, to time.Time, activity, scene string) map[string]any {
		return map[string]any{
			"start_at": from.Format(time.RFC3339Nano), "end_at": to.Format(time.RFC3339Nano), "activity": activity, "scene": scene,
			"item_type": "planned", "status": "planned", "priority": 0.5, "flexibility": 0.5, "interruption_cost": 0.5,
		}
	}
	dyeItem := item(start, end, "去染发", "理发店")
	dyeItem["intention_id"] = intentionID
	dyeItem["action_plan"] = map[string]any{"capability": lifeActivityStartCapabilityName, "kind": "hair_dye", "duration_minutes": 15, "desired_hair_color": "粉色", "reason": "按日程染发"}
	_, err = fixture.app.AcceptSchedule(fixture.ctx, fixture.ownerID, fixture.fluctlightID, map[string]any{
		"local_date": dayStart.Format("2006-01-02"), "timezone": "Asia/Shanghai", "expected_revision": 1,
		"expected_life_context_revision": life["context_revision"], "idempotency_key": "cancelled-dye-schedule-" + fixture.suffix,
		"evidence_refs": []any{"owner:cancelled-dye"}, "reschedule_policy": map[string]any{},
		"items": []any{item(dayStart, start, "阅读", "书房"), dyeItem, item(end, dayEnd, "休息", "家")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(intentionDecideCapabilityName, "cancel-dye", map[string]any{
		"operation": "cancel", "intention_id": intentionID, "reason": "决定不染了",
	})); err != nil {
		t.Fatal(err)
	}
	var scheduleItemID string
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT id FROM public.life_schedule_items WHERE intention_id=$1`, intentionID).Scan(&scheduleItemID); err != nil {
		t.Fatal(err)
	}
	forcedStart := time.Now().UTC().Add(-time.Minute)
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `UPDATE public.life_schedule_items SET start_at=$2,end_at=now()+interval '20 minutes' WHERE id=$1`, scheduleItemID, forcedStart); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `UPDATE public.fluctlight_intentions SET trigger=jsonb_set(trigger,'{due_at}',to_jsonb($2::text),true) WHERE id=$1`, intentionID, forcedStart.Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	if result, err := fixture.app.ProcessIntentionTrigger(fixture.ctx, intentionID); err != nil || stringValue(result["status"]) != "cancelled" {
		t.Fatalf("cancelled trigger=%#v err=%v", result, err)
	}
	if _, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(lifeActivityStartCapabilityName, "stale-dye-start", map[string]any{
		"kind": "hair_dye", "intention_id": intentionID, "schedule_item_id": scheduleItemID, "duration_minutes": 15, "desired_hair_color": "粉色", "reason": "过期的预约",
	})); err == nil {
		t.Fatal("cancelled scheduled dye started")
	}
	var activities int
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.fluctlight_life_activity_runs WHERE intention_id=$1`, intentionID).Scan(&activities); err != nil || activities != 0 {
		t.Fatalf("cancelled intention activities=%d err=%v", activities, err)
	}
}

func TestIntentionScheduleCreatesCurrentDayWhenNoScheduleExists(t *testing.T) {
	fixture := seedWardrobeToolFixture(t)
	location, _ := time.LoadLocation("Asia/Shanghai")
	now := time.Now().In(location)
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	start := now.Add(5 * time.Minute).Truncate(time.Minute)
	end := start.Add(15 * time.Minute)
	dayEnd := dayStart.AddDate(0, 0, 1)
	if !end.Before(dayEnd) {
		t.Skip("not enough current-day time")
	}
	makeItem := func(from, to time.Time, activity, scene string, selected bool) map[string]any {
		item := map[string]any{"start_at": from.Format(time.RFC3339Nano), "end_at": to.Format(time.RFC3339Nano), "activity": activity, "scene": scene,
			"item_type": "planned", "status": "planned", "priority": 0.5, "flexibility": 0.5, "interruption_cost": 0.5}
		if selected {
			item["planned_action_slot"] = true
		}
		return item
	}
	fixture.app.SchedulePlanner = fakeSchedulePlanner{plan: map[string]any{"reschedule_policy": map[string]any{}, "items": []any{
		makeItem(dayStart, start, "阅读", "书房", false), makeItem(start, end, "去染发", "理发店", true), makeItem(end, dayEnd, "休息", "家", false),
	}}}
	fixture.app.Capabilities, fixture.app.Runtime = nil, nil
	receipt, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(scheduleActivityCapabilityName, "first-day-dye", map[string]any{
		"goal": "染成粉色头发", "action": "安排去染发", "expected_outcome": "发色变成粉色", "reason": "用户请求",
		"action_plan": map[string]any{"kind": "hair_dye", "duration_minutes": 15, "desired_hair_color": "粉色"},
	}))
	if err != nil || receipt.Result.Status != "completed" {
		t.Fatalf("first-day plan=%#v err=%v", receipt, err)
	}
	var versions, linked int
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.life_schedules WHERE fluctlight_id=$1 AND status='accepted'`, fixture.fluctlightID).Scan(&versions); err != nil {
		t.Fatal(err)
	}
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.life_schedule_items WHERE intention_id=$1`, stringValue(mapValue(receipt.Result.Output)["intention_id"])).Scan(&linked); err != nil {
		t.Fatal(err)
	}
	if versions != 1 || linked != 1 {
		t.Fatalf("first-day plan versions=%d linked=%d", versions, linked)
	}
}

func seedFutureScheduledDye(t *testing.T) (independentToolE2EFixture, string, string, string, string) {
	t.Helper()
	fixture := seedWardrobeToolFixture(t)
	location, _ := time.LoadLocation("Asia/Shanghai")
	now := time.Now().In(location)
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	start := now.Add(5 * time.Minute).Truncate(time.Minute)
	end := start.Add(15 * time.Minute)
	dayEnd := dayStart.AddDate(0, 0, 1)
	if !end.Before(dayEnd) {
		t.Skip("not enough current-day time for appointment")
	}
	item := func(from, to time.Time, activity, scene string, selected bool) map[string]any {
		value := map[string]any{"start_at": from.Format(time.RFC3339Nano), "end_at": to.Format(time.RFC3339Nano), "activity": activity, "scene": scene,
			"item_type": "planned", "status": "planned", "priority": 0.5, "flexibility": 0.5, "interruption_cost": 0.5}
		if selected {
			value["planned_action_slot"] = true
		}
		return value
	}
	fixture.app.SchedulePlanner = fakeSchedulePlanner{plan: map[string]any{"reschedule_policy": map[string]any{}, "items": []any{
		item(dayStart, start, "阅读", "书房", false), item(start, end, "去染发", "理发店", true), item(end, dayEnd, "休息", "家", false),
	}}}
	fixture.app.Capabilities, fixture.app.Runtime = nil, nil
	receipt, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(scheduleActivityCapabilityName, "planned-dye-"+fixture.suffix, map[string]any{
		"goal": "染成粉色头发", "action": "安排去染发", "expected_outcome": "发色变成粉色", "reason": "用户请求",
		"action_plan": map[string]any{"kind": "hair_dye", "duration_minutes": 15, "desired_hair_color": "粉色"},
	}))
	if err != nil || receipt.Result.Status != "completed" {
		t.Fatalf("plan dye=%#v err=%v", receipt, err)
	}
	output := mapValue(receipt.Result.Output)
	return fixture, stringValue(output["goal_id"]), stringValue(output["intention_id"]), stringValue(output["schedule_id"]), stringValue(output["schedule_item_id"])
}

func TestCancellingAcceptedScheduleClosesLinkedIntentionAndGoal(t *testing.T) {
	fixture, goalID, intentionID, scheduleID, _ := seedFutureScheduledDye(t)
	_, life, err := fixture.app.readLifeContextSnapshotAt(fixture.ctx, fixture.fluctlightID, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	payload := map[string]any{"expected_revision": 1, "expected_life_context_revision": life["context_revision"], "idempotency_key": "cancel-planned-dye-" + fixture.suffix}
	cancelled, err := fixture.app.CancelScheduleExpected(fixture.ctx, fixture.ownerID, fixture.fluctlightID, scheduleID, payload)
	if err != nil || stringValue(cancelled["status"]) != "cancelled" {
		t.Fatalf("cancel schedule=%#v err=%v", cancelled, err)
	}
	var intentionStatus, goalStatus string
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT status FROM public.fluctlight_intentions WHERE id=$1`, intentionID).Scan(&intentionStatus); err != nil {
		t.Fatal(err)
	}
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT status FROM public.fluctlight_goals WHERE id=$1`, goalID).Scan(&goalStatus); err != nil {
		t.Fatal(err)
	}
	if intentionStatus != "cancelled" || goalStatus != "cancelled" {
		t.Fatalf("cancelled schedule left open agency: intention=%s goal=%s", intentionStatus, goalStatus)
	}
	if replay, err := fixture.app.CancelScheduleExpected(fixture.ctx, fixture.ownerID, fixture.fluctlightID, scheduleID, payload); err != nil || replay["replayed"] != true {
		t.Fatalf("schedule cancellation replay=%#v err=%v", replay, err)
	}
	if due, err := fixture.app.ProcessIntentionTrigger(fixture.ctx, intentionID); err != nil || stringValue(due["status"]) != "cancelled" {
		t.Fatalf("cancelled schedule still triggered: due=%#v err=%v", due, err)
	}
}

func TestPausedStartedScheduledDyeCannotCommitHairColor(t *testing.T) {
	fixture, _, intentionID, _, itemID := seedFutureScheduledDye(t)
	forcedStart := time.Now().UTC().Add(-time.Minute)
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `UPDATE public.life_schedule_items SET end_at=$2 WHERE schedule_id=(SELECT schedule_id FROM public.life_schedule_items WHERE id=$1) AND end_at=(SELECT start_at FROM public.life_schedule_items WHERE id=$1) AND intention_id IS NULL`, itemID, forcedStart); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `UPDATE public.life_schedule_items SET start_at=$2,end_at=now()+interval '20 minutes' WHERE id=$1`, itemID, forcedStart); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `UPDATE public.fluctlight_intentions SET trigger=jsonb_set(trigger,'{due_at}',to_jsonb($2::text),true),preferred_time=$3 WHERE id=$1`, intentionID, forcedStart.Format(time.RFC3339Nano), forcedStart); err != nil {
		t.Fatal(err)
	}
	started, err := fixture.app.ProcessIntentionTrigger(fixture.ctx, intentionID)
	if err != nil || stringValue(started["status"]) != "activity_started" {
		t.Fatalf("scheduled dye start=%#v err=%v", started, err)
	}
	activityID := stringValue(started["activity_id"])
	forceVirtualActivityDue(t, fixture, activityID)
	if _, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(intentionDecideCapabilityName, "pause-started-dye-"+fixture.suffix, map[string]any{
		"operation": "pause", "intention_id": intentionID, "reason": "暂停染发",
	})); err != nil {
		t.Fatal(err)
	}
	router := setupVirtualActivityTestProvider(t, fixture, map[string]any{"status": "completed", "reason": "虚拟染发完成", "hair_color": "粉色"})
	resolved, err := fixture.app.ResolveScheduledLifeActivity(fixture.ctx, activityID)
	if err != nil || stringValue(resolved["status"]) != "cancelled" {
		t.Fatalf("paused dye resolved=%#v err=%v", resolved, err)
	}
	if router.requestCount("virtual_activity_result") != 0 {
		t.Fatal("cancelled scheduled activity still requested a result")
	}
	appearance, _, _, err := fixture.app.readEffectiveLifeSnapshot(fixture.ctx, fixture.fluctlightID, time.Now().UTC())
	if err != nil || stringValue(mapValue(mapValue(appearance["body_fields"])["hair_color"])["value"]) != "black" {
		t.Fatalf("paused activity changed current color: appearance=%#v err=%v", appearance, err)
	}
	var runStatus string
	var events int
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT status FROM public.fluctlight_life_activity_runs WHERE id=$1`, activityID).Scan(&runStatus); err != nil {
		t.Fatal(err)
	}
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.life_events WHERE fluctlight_id=$1 AND kind='hair_dye'`, fixture.fluctlightID).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if runStatus != "cancelled" || events != 0 {
		t.Fatalf("paused activity effects run=%s events=%d", runStatus, events)
	}
	if _, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(intentionDecideCapabilityName, "resume-started-dye-"+fixture.suffix, map[string]any{
		"operation": "resume", "intention_id": intentionID, "reason": "重新考虑染发",
	})); err != nil {
		t.Fatal(err)
	}
	if due, err := fixture.app.ProcessIntentionTrigger(fixture.ctx, intentionID); err != nil || stringValue(due["status"]) != "stale_schedule" {
		t.Fatalf("resumed intention reused cancelled appointment: due=%#v err=%v", due, err)
	}
	var runs int
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.fluctlight_life_activity_runs WHERE intention_id=$1`, intentionID).Scan(&runs); err != nil || runs != 1 {
		t.Fatalf("resumed intention duplicated cancelled run: runs=%d err=%v", runs, err)
	}
}

func TestStartedScheduledDyeBlocksReplanAndStaleVersionCannotSettle(t *testing.T) {
	fixture, _, intentionID, scheduleID, itemID := seedFutureScheduledDye(t)
	forcedStart := time.Now().UTC().Add(-time.Minute)
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `UPDATE public.life_schedule_items SET end_at=$2 WHERE schedule_id=(SELECT schedule_id FROM public.life_schedule_items WHERE id=$1) AND end_at=(SELECT start_at FROM public.life_schedule_items WHERE id=$1) AND intention_id IS NULL`, itemID, forcedStart); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `UPDATE public.life_schedule_items SET start_at=$2,end_at=now()+interval '20 minutes' WHERE id=$1`, itemID, forcedStart); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `UPDATE public.fluctlight_intentions SET trigger=jsonb_set(trigger,'{due_at}',to_jsonb($2::text),true),preferred_time=$3 WHERE id=$1`, intentionID, forcedStart.Format(time.RFC3339Nano), forcedStart); err != nil {
		t.Fatal(err)
	}
	started, err := fixture.app.ProcessIntentionTrigger(fixture.ctx, intentionID)
	if err != nil || stringValue(started["status"]) != "activity_started" {
		t.Fatalf("scheduled dye start=%#v err=%v", started, err)
	}
	activityID := stringValue(started["activity_id"])
	_, life, err := fixture.app.readLifeContextSnapshotAt(fixture.ctx, fixture.fluctlightID, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	location, _ := time.LoadLocation("Asia/Shanghai")
	now := time.Now().In(location)
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, location)
	if _, err := fixture.app.AcceptSchedule(fixture.ctx, fixture.ownerID, fixture.fluctlightID, map[string]any{
		"local_date": dayStart.Format("2006-01-02"), "timezone": "Asia/Shanghai", "expected_revision": 1,
		"expected_life_context_revision": life["context_revision"], "idempotency_key": "replace-started-dye-" + fixture.suffix,
		"evidence_refs": []any{"owner:replan"}, "reschedule_policy": map[string]any{},
		"items": []any{map[string]any{"start_at": dayStart.Format(time.RFC3339Nano), "end_at": dayStart.AddDate(0, 0, 1).Format(time.RFC3339Nano), "activity": "阅读", "scene": "书房"}},
	}); err == nil || err.Error() != "schedule_active_activity_replan_blocked" {
		t.Fatalf("active dye was silently replanned away: %v", err)
	}
	forceVirtualActivityDue(t, fixture, activityID)
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `UPDATE public.life_schedules SET status='superseded' WHERE id=$1`, scheduleID); err != nil {
		t.Fatal(err)
	}
	router := setupVirtualActivityTestProvider(t, fixture, map[string]any{"status": "completed", "reason": "虚拟染发完成", "hair_color": "粉色"})
	resolved, err := fixture.app.ResolveScheduledLifeActivity(fixture.ctx, activityID)
	if err != nil || stringValue(resolved["status"]) != "cancelled" || router.requestCount("virtual_activity_result") != 0 {
		t.Fatalf("superseded dye settled: result=%#v err=%v provider_calls=%d", resolved, err, router.requestCount("virtual_activity_result"))
	}
	appearance, _, _, err := fixture.app.readEffectiveLifeSnapshot(fixture.ctx, fixture.fluctlightID, time.Now().UTC())
	if err != nil || stringValue(mapValue(mapValue(appearance["body_fields"])["hair_color"])["value"]) != "black" {
		t.Fatalf("stale schedule changed hair color: appearance=%#v err=%v", appearance, err)
	}
}

func TestCancellingScheduleAfterDeferredSlotClosesDye(t *testing.T) {
	fixture, goalID, intentionID, scheduleID, itemID := seedFutureScheduledDye(t)
	forcedStart := time.Now().UTC().Add(-time.Minute)
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `UPDATE public.life_schedule_items SET end_at=$2 WHERE schedule_id=(SELECT schedule_id FROM public.life_schedule_items WHERE id=$1) AND end_at=(SELECT start_at FROM public.life_schedule_items WHERE id=$1) AND intention_id IS NULL`, itemID, forcedStart); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `UPDATE public.life_schedule_items SET start_at=$2,end_at=now()+interval '20 minutes' WHERE id=$1`, itemID, forcedStart); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `UPDATE public.fluctlight_intentions SET trigger=jsonb_set(trigger,'{due_at}',to_jsonb($2::text),true),preferred_time=$3 WHERE id=$1`, intentionID, forcedStart.Format(time.RFC3339Nano), forcedStart); err != nil {
		t.Fatal(err)
	}
	started, err := fixture.app.ProcessIntentionTrigger(fixture.ctx, intentionID)
	if err != nil || stringValue(started["status"]) != "activity_started" {
		t.Fatalf("scheduled dye start=%#v err=%v", started, err)
	}
	activityID := stringValue(started["activity_id"])
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `UPDATE public.fluctlight_life_activity_runs SET status='deferred',not_before=now()+interval '30 minutes' WHERE id=$1`, activityID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `UPDATE public.life_schedule_items SET end_at=now()-interval '1 minute' WHERE id=$1`, itemID); err != nil {
		t.Fatal(err)
	}
	_, life, err := fixture.app.readLifeContextSnapshotAt(fixture.ctx, fixture.fluctlightID, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.app.CancelScheduleExpected(fixture.ctx, fixture.ownerID, fixture.fluctlightID, scheduleID, map[string]any{
		"expected_revision": 1, "expected_life_context_revision": life["context_revision"], "idempotency_key": "cancel-deferred-dye-" + fixture.suffix,
	}); err != nil {
		t.Fatal(err)
	}
	var intentionStatus, goalStatus string
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT status FROM public.fluctlight_intentions WHERE id=$1`, intentionID).Scan(&intentionStatus); err != nil {
		t.Fatal(err)
	}
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT status FROM public.fluctlight_goals WHERE id=$1`, goalID).Scan(&goalStatus); err != nil {
		t.Fatal(err)
	}
	if intentionStatus != "cancelled" || goalStatus != "cancelled" {
		t.Fatalf("deferred cancelled schedule left agency open: intention=%s goal=%s", intentionStatus, goalStatus)
	}
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `UPDATE public.fluctlight_life_activity_runs SET not_before=now()-interval '1 minute' WHERE id=$1`, activityID); err != nil {
		t.Fatal(err)
	}
	resolved, err := fixture.app.ResolveScheduledLifeActivity(fixture.ctx, activityID)
	if err != nil || stringValue(resolved["status"]) != "cancelled" {
		t.Fatalf("cancelled deferred dye resolved=%#v err=%v", resolved, err)
	}
}
