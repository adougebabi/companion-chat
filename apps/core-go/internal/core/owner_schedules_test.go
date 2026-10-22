package core

import (
	"context"
	"github.com/jackc/pgx/v5"
	"net/http"
	"net/url"
	"testing"
	"time"
)

func TestOwnerScheduleDayUsesLocalCalendarAcrossDST(t *testing.T) {
	first, err := scheduleDay("2026-11-01", "America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	if first.AddDate(0, 0, 1).Sub(first) != 25*time.Hour {
		t.Fatal("local day was replaced with 24h duration")
	}
	for _, date := range []string{"2026-02-30", "2026-2-03", "2026-10-22T00:00:00Z"} {
		if _, err := scheduleDay(date, "Asia/Shanghai"); err == nil {
			t.Fatal("invalid date", date)
		}
	}
}

func ownerScheduleTestBody(t *testing.T, f independentToolE2EFixture, date, key string) map[string]any {
	t.Helper()
	_, life, err := f.app.readLifeContextSnapshotAt(f.ctx, f.fluctlightID, f.app.now())
	if err != nil {
		t.Fatal(err)
	}
	return map[string]any{"local_date": date, "expected_revision": 0, "expected_life_context_revision": life["context_revision"], "idempotency_key": key, "reason": "Owner请求为缺失日期规划"}
}

func TestOwnerScheduleRequestPersistsOriginalAndNeverOverwrites(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	zone, err := readLifeContextTimezoneWith(f.ctx, f.repository.Pool(), f.fluctlightID)
	if err != nil {
		t.Fatal(err)
	}
	location, _ := time.LoadLocation(zone)
	day, err := scheduleDay(f.app.now().In(location).AddDate(0, 0, 1).Format("2006-01-02"), zone)
	if err != nil {
		t.Fatal(err)
	}
	date := day.Format("2006-01-02")
	base := "/internal/fluctlights/" + f.fluctlightID + "/schedules"
	body := ownerScheduleTestBody(t, f, date, "owner-schedule-original")
	requested, err := f.app.DispatchOwnerSurface(f.ctx, "POST", base+"/requests", nil, f.ownerID, body)
	if err != nil || requested["status"] != "queued" {
		t.Fatal(requested, err)
	}
	id := stringValue(requested["request_id"])
	replay, err := f.app.DispatchOwnerSurface(f.ctx, "POST", base+"/requests", nil, f.ownerID, body)
	if err != nil || replay["request_id"] != id {
		t.Fatal("replay", replay, err)
	}
	changed := cloneMap(body)
	changed["reason"] = "changed same key"
	if _, err = f.app.DispatchOwnerSurface(f.ctx, "POST", base+"/requests", nil, f.ownerID, changed); err == nil {
		t.Fatal("changed body replayed")
	}
	if _, err = f.app.OwnerScheduleRequest(f.ctx, f.foreignOwnerID, f.fluctlightID, id); err == nil {
		t.Fatal("cross owner read")
	}
	// A persisted, validated Provider proposal is the recovery checkpoint. This
	// test exercises real SQL/authority/acceptance, not live model quality.
	proposal := map[string]any{"local_date": date, "timezone": zone, "reschedule_policy": map[string]any{}, "items": []any{map[string]any{"start_at": day.Format(time.RFC3339), "end_at": day.AddDate(0, 0, 1).Format(time.RFC3339), "activity": "自主安排的一天", "scene": "住所", "item_type": "planned", "status": "planned", "priority": 0.5, "flexibility": 0.5, "interruption_cost": 0.5}}}
	if _, err = f.repository.Pool().Exec(f.ctx, `UPDATE public.owner_schedule_requests SET proposal=$2 WHERE id=$1`, id, jsonBytes(proposal)); err != nil {
		t.Fatal(err)
	}
	completed, err := f.app.ProcessOwnerScheduleRequest(f.ctx, id)
	if err != nil || completed["status"] != "completed" {
		t.Fatal("apply persisted proposal", completed, err)
	}
	again, err := f.app.ProcessOwnerScheduleRequest(f.ctx, id)
	if err != nil || jsonString(again) != jsonString(completed) {
		t.Fatal("recovery duplicated result", again, err)
	}
	page, err := f.app.OwnerSchedules(f.ctx, f.ownerID, f.fluctlightID, url.Values{"from": {date}, "to": {day.AddDate(0, 0, 1).Format("2006-01-02")}})
	if err != nil {
		t.Fatal(err)
	}
	days := arrayValue(page["days"])
	if len(days) != 2 || mapValue(days[1])["schedule"] != nil {
		t.Fatal("missing day must remain absent", page)
	}
	schedule := mapValue(mapValue(days[0])["schedule"])
	if schedule["id"] != completed["schedule_id"] || schedule["current_item_id"] != nil || schedule["completed_before"] != nil {
		t.Fatal("calendar invented current context", schedule)
	}
	protect := ownerScheduleTestBody(t, f, date, "explicit-protected-day")
	protect["expected_revision"] = schedule["revision"]
	protected, err := f.app.DispatchOwnerSurface(f.ctx, "POST", base+"/requests", nil, f.ownerID, protect)
	if err != nil || protected["status"] != "completed" || mapValue(protected["result"])["disposition"] != "preserved" {
		t.Fatal("accepted day overwritten", protected, err)
	}
	var count int
	if err = f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.life_schedules WHERE fluctlight_id=$1 AND local_date=$2`, f.fluctlightID, date).Scan(&count); err != nil || count != 1 {
		t.Fatal("duplicate schedule", count, err)
	}
}

func TestOwnerScheduleChangedScopeDoesNotApplyStoredProposal(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	zone, err := readLifeContextTimezoneWith(f.ctx, f.repository.Pool(), f.fluctlightID)
	if err != nil {
		t.Fatal(err)
	}
	location, _ := time.LoadLocation(zone)
	date := f.app.now().In(location).AddDate(0, 0, 1).Format("2006-01-02")
	body := ownerScheduleTestBody(t, f, date, "scope-fenced")
	requested, err := f.app.DispatchOwnerSurface(f.ctx, "POST", "/internal/fluctlights/"+f.fluctlightID+"/schedules/requests", nil, f.ownerID, body)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlights SET current_revision=current_revision+1 WHERE id=$1`, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err = f.app.ProcessOwnerScheduleRequest(f.ctx, stringValue(requested["request_id"])); err == nil {
		t.Fatal("stale scope applied")
	}
	read, err := f.app.OwnerScheduleRequest(f.ctx, f.ownerID, f.fluctlightID, stringValue(requested["request_id"]))
	if err != nil || read["status"] != "conflict" || read["error_code"] != "schedule_scope_changed" {
		t.Fatal(read, err)
	}
	var count int
	if err = f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.life_schedules WHERE fluctlight_id=$1`, f.fluctlightID).Scan(&count); err != nil || count != 0 {
		t.Fatal("scope conflict committed", count, err)
	}
}

func TestOwnerScheduleAcceptedPromiseBindsActualAppointmentWithoutExecution(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	goalID := plannerOwnerGoal(t, f, "共同画完小画", false)
	title := "画好约定的小画"
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		if payloadHasToolResult(payload) {
			return fakeProviderResult{Structured: map[string]any{"decision": "accepted", "reason": "接受已有目标下的约定"}}
		}
		return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("promise-create", intentionDecideCapabilityName, map[string]any{"operation": "create", "goal": "共同画完小画", "action": title, "expected_outcome": "实际画好并分享小画", "reason": "接受具体约定"})}}
	})
	f.app.Provider.HTTP = &http.Client{Transport: router}
	seedCognitiveProviderRole(t, f.ctx, f.repository, "promise-schedule-"+f.suffix)
	admitted, err := f.app.DispatchOwnerSurface(f.ctx, "POST", "/internal/fluctlights/"+f.fluctlightID+"/promises", nil, f.ownerID, map[string]any{"title": title, "due_at": f.app.now().Add(72 * time.Hour).Format(time.RFC3339Nano), "reason": "约定真实结果", "idempotency_key": "promise-for-schedule"})
	if err != nil {
		t.Fatal(err)
	}
	root := mapValue(admitted["request"])
	if _, err = f.app.ProcessCognitionInbox(f.ctx, stringValue(root["source_fact_id"])); err != nil {
		t.Fatal(err)
	}
	promise, err := f.app.OwnerPromise(f.ctx, f.ownerID, f.fluctlightID, stringValue(root["id"]))
	if err != nil || promise["lifecycle"] != "accepted" || promise["goal_id"] != goalID {
		t.Fatal("promise not actually accepted", promise, err)
	}
	location, _ := time.LoadLocation("Asia/Shanghai")
	day, _ := scheduleDay(f.app.now().In(location).AddDate(0, 0, 1).Format("2006-01-02"), "Asia/Shanghai")
	date := day.Format("2006-01-02")
	life := currentLifeForTest(t, f.ctx, f.app, f.fluctlightID, f.app.now())
	accepted, err := f.app.AcceptSchedule(f.ctx, f.ownerID, f.fluctlightID, fullDaySchedulePayloadForTest(day, "future-baseline", stringValue(life["context_revision"])))
	if err != nil {
		t.Fatal(err)
	}
	start, end := day.Add(time.Hour), day.Add(2*time.Hour)
	item := func(from, to time.Time, activity string) map[string]any {
		return map[string]any{"start_at": from.Format(time.RFC3339), "end_at": to.Format(time.RFC3339), "activity": activity, "scene": "书房", "item_type": "planned", "status": "planned", "priority": 0.5, "flexibility": 0.5, "interruption_cost": 0.5}
	}
	appointment := item(start, end, title)
	appointment["planned_action_slot"] = true
	calls := 0
	f.app.Schedule.SetPlanner(fakeSchedulePlanner{calls: &calls, plan: map[string]any{"reschedule_policy": map[string]any{}, "items": []any{item(day, start, "阅读"), appointment, item(end, day.AddDate(0, 0, 1), "阅读")}}})
	body := ownerScheduleTestBody(t, f, date, "schedule-accepted-promise")
	body["expected_revision"] = accepted["revision"]
	body["promise_id"] = promise["request_id"]
	body["expected_promise_revision"] = promise["revision"]
	body["start_at"] = start.Format(time.RFC3339)
	body["end_at"] = end.Format(time.RFC3339)
	path := "/internal/fluctlights/" + f.fluctlightID + "/schedules/requests"
	requested, err := f.app.DispatchOwnerSurface(f.ctx, "POST", path, nil, f.ownerID, body)
	if err != nil || requested["status"] != "queued" || calls != 0 {
		t.Fatal("API performed planning or failed", requested, err, calls)
	}
	result, err := f.app.ProcessOwnerScheduleRequest(f.ctx, stringValue(requested["request_id"]))
	if err != nil || result["disposition"] != "scheduled" || calls != 1 || result["schedule_item_id"] == nil {
		t.Fatal("appointment result", result, err, calls)
	}
	var intentionID *string
	var plan []byte
	var promiseID string
	if err = f.repository.Pool().QueryRow(f.ctx, `SELECT intention_id,action_plan,promise_request_id FROM public.life_schedule_items WHERE id=$1`, result["schedule_item_id"]).Scan(&intentionID, &plan, &promiseID); err != nil || intentionID != nil || len(plan) != 0 || promiseID != promise["request_id"] {
		t.Fatal("appointment invented executable action", intentionID, string(plan), promiseID, err)
	}
	after, err := f.app.OwnerPromise(f.ctx, f.ownerID, f.fluctlightID, stringValue(root["id"]))
	if err != nil || after["lifecycle"] != "accepted" || mapValue(after["intention"])["status"] == "completed" {
		t.Fatal("schedule claimed fulfillment", after, err)
	}
	// A fresh request discovers the real association and invokes no planner.
	body["idempotency_key"] = "discover-existing-appointment"
	body["expected_revision"] = result["revision"]
	existing, err := f.app.DispatchOwnerSurface(f.ctx, "POST", path, nil, f.ownerID, body)
	if err != nil || existing["status"] != "completed" || mapValue(existing["result"])["schedule_item_id"] != result["schedule_item_id"] || calls != 1 {
		t.Fatal("duplicate appointment", existing, err, calls)
	}
	// A genuine accepted reschedule moves the window to another local day.
	// The old accepted schedule survives as history but cannot block D2.
	nextDay := day.AddDate(0, 0, 1)
	router.otherwise(func(payload map[string]any) fakeProviderResult {
		if payloadHasToolResult(payload) {
			return fakeProviderResult{Structured: map[string]any{"decision": "accepted", "reason": "接受新的约定窗口"}}
		}
		return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("promise-reschedule", intentionDecideCapabilityName, map[string]any{"operation": "update", "intention_id": after["intention_id"], "reason": "接受改到新的日期"})}}
	})
	shifted, err := f.app.DispatchOwnerSurface(f.ctx, "POST", "/internal/fluctlights/"+f.fluctlightID+"/promises/"+stringValue(root["id"])+"/reschedule", nil, f.ownerID, map[string]any{"expected_revision": after["revision"], "starts_at": nextDay.Format(time.RFC3339), "due_at": nextDay.Add(4 * time.Hour).Format(time.RFC3339), "reason": "改到下一天", "idempotency_key": "shift-window"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.app.ProcessCognitionInbox(f.ctx, stringValue(mapValue(shifted["request"])["source_fact_id"])); err != nil {
		t.Fatal(err)
	}
	after, err = f.app.OwnerPromise(f.ctx, f.ownerID, f.fluctlightID, stringValue(root["id"]))
	if err != nil {
		t.Fatal(err)
	}
	life = currentLifeForTest(t, f.ctx, f.app, f.fluctlightID, f.app.now())
	nextBase, err := f.app.AcceptSchedule(f.ctx, f.ownerID, f.fluctlightID, fullDaySchedulePayloadForTest(nextDay, "next-day-baseline", stringValue(life["context_revision"])))
	if err != nil {
		t.Fatal(err)
	}
	nextStart, nextEnd := nextDay.Add(time.Hour), nextDay.Add(2*time.Hour)
	nextItem := item(nextStart, nextEnd, title)
	nextItem["planned_action_slot"] = true
	f.app.Schedule.SetPlanner(fakeSchedulePlanner{calls: &calls, plan: map[string]any{"reschedule_policy": map[string]any{}, "items": []any{item(nextDay, nextStart, "阅读"), nextItem, item(nextEnd, nextDay.AddDate(0, 0, 1), "阅读")}}})
	body = ownerScheduleTestBody(t, f, nextDay.Format("2006-01-02"), "rescheduled-appointment")
	body["expected_revision"] = nextBase["revision"]
	body["promise_id"] = after["request_id"]
	body["expected_promise_revision"] = after["revision"]
	body["start_at"] = nextStart.Format(time.RFC3339)
	body["end_at"] = nextEnd.Format(time.RFC3339)
	nextRequest, err := f.app.DispatchOwnerSurface(f.ctx, "POST", path, nil, f.ownerID, body)
	if err != nil {
		t.Fatal(err)
	}
	nextResult, err := f.app.ProcessOwnerScheduleRequest(f.ctx, stringValue(nextRequest["request_id"]))
	if err != nil || nextResult["disposition"] != "scheduled" {
		t.Fatal("obsolete D1 relation blocked D2", nextResult, err)
	}
	oldDay, err := readScheduleAtWith(f.ctx, f.repository.Pool(), f.fluctlightID, "Asia/Shanghai", start.Add(time.Minute))
	if err != nil || scheduleItemAt(oldDay, start.Add(time.Minute)) != nil {
		t.Fatal("old window still effective", oldDay, err)
	}
	// Cancellation preserves the schedule version as history, but invalidates its
	// appointment as current-life authority rather than fabricating a new routine.
	if _, err = f.app.DispatchOwnerSurface(f.ctx, "POST", "/internal/fluctlights/"+f.fluctlightID+"/promises/"+stringValue(root["id"])+"/cancel", nil, f.ownerID, map[string]any{"expected_revision": after["revision"], "idempotency_key": "cancel-scheduled-promise", "reason": "撤回我的参与"}); err != nil {
		t.Fatal(err)
	}
	read, err := readScheduleAtWith(f.ctx, f.repository.Pool(), f.fluctlightID, "Asia/Shanghai", start.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range arrayValue(read["items"]) {
		row := mapValue(raw)
		if row["promise_request_id"] == promiseID && row["promise_schedule_valid"] != false {
			t.Fatal("cancelled appointment still current", row)
		}
	}
	if current := scheduleItemAt(read, start.Add(time.Minute)); current != nil {
		t.Fatal("cancelled appointment invented current life", current)
	}
}

func TestOwnerScheduleOriginalRequestReplaysAfterMidnightAndTimezoneChange(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	at := f.app.now()
	f.app.Clock = func() time.Time { return at }
	location, _ := time.LoadLocation("Asia/Shanghai")
	date := at.In(location).Format("2006-01-02")
	body := ownerScheduleTestBody(t, f, date, "midnight-original")
	var original map[string]any
	run := func() (map[string]any, error) {
		var result map[string]any
		err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
			var err error
			result, err = f.app.RequestOwnerSchedule(context.WithValue(f.ctx, surfaceDBTxKey{}, tx), f.ownerID, f.fluctlightID, body)
			return err
		})
		return result, err
	}
	var err error
	original, err = run()
	if err != nil {
		t.Fatal(err)
	}
	at = at.AddDate(0, 0, 2)
	if _, err = f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlights SET identity=identity||'{"timezone":"America/New_York"}'::jsonb,current_revision=current_revision+1 WHERE id=$1`, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	replay, err := run()
	if err != nil || replay["request_id"] != original["request_id"] || replay["timezone"] != original["timezone"] {
		t.Fatal("replay was reinterpreted as a new day", replay, err)
	}
}
