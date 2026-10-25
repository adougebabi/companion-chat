package core

import (
	"net/http"
	"testing"
	"time"
)

func TestR9ChatProposalUsesOriginalMessageThenActualAcceptanceAndCalendar(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	title := "明天晚上七点一起在西湖边散步，你记一下。"
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.conversation_messages(id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key) VALUES('r9-invite',$1,1,$2,'user',$3,'[]','r9-invite')`, f.conversationID, f.ownerID, title); err != nil {
		t.Fatal(err)
	}
	zone, err := readLifeContextTimezoneWith(f.ctx, f.repository.Pool(), f.fluctlightID)
	if err != nil {
		t.Fatal(err)
	}
	loc, _ := time.LoadLocation(zone)
	tomorrow := f.app.now().In(loc).AddDate(0, 0, 1)
	start := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 19, 0, 0, 0, loc)
	end := start.Add(time.Hour)
	args := map[string]any{"starts_at": start.Format(time.RFC3339Nano), "due_at": end.Format(time.RFC3339Nano), "reason": "保留原话，提出明确时间的约定"}
	request := f.request(chatPromiseCapabilityName, "r9-chat-proposal", args)
	request.Surface = CapabilitySurfaceConversation
	request.EvidenceID = "r9-invite"
	admitted, err := f.app.ExecuteTool(f.ctx, request)
	if err != nil || admitted.Result.Status != "accepted" || mapValue(admitted.Result.Output)["accepted_by_character"] != false || mapValue(admitted.Result.Output)["scheduled"] != false {
		t.Fatal(admitted, err)
	}
	id := stringValue(mapValue(admitted.Result.Output)["promise_request_id"])
	p, err := f.app.OwnerPromise(f.ctx, f.ownerID, f.fluctlightID, id)
	if err != nil || p["title"] != title || p["accepted_at"] != nil {
		t.Fatal("proposal invented acceptance or rewrote owner", p, err)
	}
	repeat := f.request(chatPromiseCapabilityName, "r9-chat-proposal-again", map[string]any{"starts_at": args["starts_at"], "due_at": args["due_at"], "reason": "相同提议重新核对"})
	repeat.Surface = CapabilitySurfaceConversation
	repeat.EvidenceID = request.EvidenceID
	again, err := f.app.ExecuteTool(f.ctx, repeat)
	if err != nil || mapValue(again.Result.Output)["promise_request_id"] != id {
		t.Fatal("duplicate proposal", again, err)
	}
	bad := request
	bad.OperationID = "r9-missing-message"
	bad.EvidenceID = "unrelated-message"
	notAdmitted, err := f.app.ExecuteTool(f.ctx, bad)
	if err != nil || notAdmitted.Result.Status != "handled" || mapValue(notAdmitted.Result.Output)["warning_code"] != "promise_source_required" {
		t.Fatal("unbound source accepted", notAdmitted, err)
	}
	if count, err := f.app.EnsureAcceptedChatPromiseSchedules(f.ctx, 50); err != nil || count != 0 {
		t.Fatal("unaccepted proposal scheduled", count, err)
	}
	var inbox string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT payload->>'source_fact_id' FROM public.owner_surface_records WHERE id=$1`, id).Scan(&inbox); err != nil {
		t.Fatal(err)
	}
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		if payloadHasToolResult(payload) {
			return fakeProviderResult{Structured: map[string]any{"decision": "accepted", "reason": "先请求规划后接受明确提议"}}
		}
		return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("r9-promise-intention", intentionDecideCapabilityName, map[string]any{"operation": "create", "goal": "履行已讨论的散步约定", "action": title, "expected_outcome": "按约定进行散步并有实际结果", "reason": "愿意接受并安排"})}}
	})
	f.app.Provider.HTTP = &http.Client{Transport: router}
	seedCognitiveProviderRole(t, f.ctx, f.repository, "r9-promise-provider")
	result, err := f.app.ProcessCognitionInbox(f.ctx, inbox)
	if err != nil || result["status"] != "awaiting_goal" {
		t.Fatal(result, err)
	}
	p, err = f.app.OwnerPromise(f.ctx, f.ownerID, f.fluctlightID, id)
	if err != nil {
		t.Fatal(err)
	}
	run := plannerRecoveryRun(t, f)
	snapshot := plannerSnapshot(t, f)
	command := plannerRecoveryRequest(t, f, run, "r9-bind", map[string]any{"expected_version": snapshot["revision"], "expected_facts_revision": snapshot["facts_revision"], "idempotency_key": "r9-bind", "reason": "绑定真实目标和约定", "changes": []any{map[string]any{"id": "r9-meet-goal", "operation": "create", "activate": true, "desired_outcome": "履行已讨论的散步约定", "success_criteria": []string{"按约定进行散步并有实际结果"}, "motivation": "真实聊天提议"}}, "promise_bindings": []any{map[string]any{"request_id": id, "goal_id": "r9-meet-goal", "expected_revision": p["revision"]}}})
	accepted, err := f.app.ExecuteTool(f.ctx, command)
	if err != nil || accepted.Result.Status != "completed" {
		t.Fatal(accepted, err)
	}
	p, err = f.app.OwnerPromise(f.ctx, f.ownerID, f.fluctlightID, id)
	if err != nil || p["lifecycle"] != "accepted" {
		t.Fatal(p, err)
	}
	// A worker returning after the beginning of a window must leave the old
	// promise untouched and continue sweeping, rather than erroring each tick.
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.owner_promise_links SET window_start=now()-interval '5 minutes',window_end=now()+interval '30 minutes' WHERE request_id=$1`, id); err != nil {
		t.Fatal(err)
	}
	if count, err := f.app.EnsureAcceptedChatPromiseSchedules(f.ctx, 50); err != nil || count != 0 {
		t.Fatal("stale appointment blocks scanner", count, err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.owner_promise_links SET window_start=$2,window_end=$3 WHERE request_id=$1`, id, start, end); err != nil {
		t.Fatal(err)
	}
	if count, err := f.app.EnsureAcceptedChatPromiseSchedules(f.ctx, 50); err != nil || count != 1 {
		t.Fatal("accepted appointment never reached calendar request", count, err)
	}
	if count, err := f.app.EnsureAcceptedChatPromiseSchedules(f.ctx, 50); err != nil || count != 0 {
		t.Fatal("calendar request duplicated", count, err)
	}
	var scheduleRequest string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT id FROM public.owner_schedule_requests WHERE fluctlight_id=$1 AND payload->>'promise_id'=$2`, f.fluctlightID, id).Scan(&scheduleRequest); err != nil {
		t.Fatal(err)
	}
	day, _ := scheduleDay(start.In(loc).Format("2006-01-02"), zone)
	item := func(from, to time.Time, activity string) map[string]any {
		return map[string]any{"start_at": from.Format(time.RFC3339Nano), "end_at": to.Format(time.RFC3339Nano), "activity": activity, "scene": "湖边", "item_type": "planned", "status": "planned", "priority": 0.5, "flexibility": 0.5, "interruption_cost": 0.5}
	}
	appointment := item(start, end, title)
	appointment["promise_request_id"] = id
	proposal := map[string]any{"local_date": day.Format("2006-01-02"), "timezone": zone, "reschedule_policy": map[string]any{}, "items": []any{item(day, start, "日常生活"), appointment, item(end, day.AddDate(0, 0, 1), "休息")}}
	// This is a durable validated-proposal recovery fixture, not live model
	// quality evidence. Core must still validate/apply all real associations.
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.owner_schedule_requests SET proposal=$2 WHERE id=$1`, scheduleRequest, jsonBytes(proposal)); err != nil {
		t.Fatal(err)
	}
	scheduled, err := f.app.ProcessOwnerScheduleRequest(f.ctx, scheduleRequest)
	if err != nil || scheduled["disposition"] != "scheduled" {
		t.Fatal(scheduled, err)
	}
	var promiseID string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT promise_request_id FROM public.life_schedule_items WHERE id=$1`, scheduled["schedule_item_id"]).Scan(&promiseID); err != nil || promiseID != id {
		t.Fatal("calendar did not retain actual promise", promiseID, err)
	}
	p, err = f.app.OwnerPromise(f.ctx, f.ownerID, f.fluctlightID, id)
	if err != nil || p["lifecycle"] != "accepted" {
		t.Fatal("scheduling fabricated fulfillment", p, err)
	}
}
