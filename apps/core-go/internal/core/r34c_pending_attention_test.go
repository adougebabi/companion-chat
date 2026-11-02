package core

import (
	"net/url"
	"testing"
	"time"
)

func TestR34CPendingAttentionSharesAllActionRequiredStates(t *testing.T) {
	f := v3OwnerFixture(t, "r34c-attention")
	states := []string{"failed", "dead_letter", "paused", "blocked", "error", "timed_out", "timeout", "deadletter", "running", "completed", "cancelled"}
	for _, status := range states {
		id := "r34c-" + status
		if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,intent_type,task_queue,status,payload,created_at) VALUES($1,$1,'visual.identity.prepare','fixture',$2,$3,$4)`, id, status, jsonBytes(map[string]any{"fluctlight_id": f.fluctlightID}), time.Now().UTC()); err != nil {
			t.Fatal(err)
		}
	}
	cursor, count, total := "", 0, 0
	for {
		page, err := f.app.OwnerPending(f.ctx, f.ownerID, url.Values{"kind": {"failed_task"}, "fluctlightId": {f.fluctlightID}, "limit": {"3"}, "cursor": {cursor}}, false)
		if err != nil {
			t.Fatal(err)
		}
		group := mapValue(arrayValue(page["groups"])[0])
		total = intValue(group["count"])
		for _, raw := range arrayValue(group["items"]) {
			item := mapValue(raw)
			if status := stringValue(item["status"]); status == "running" || status == "completed" || status == "cancelled" {
				t.Fatalf("non-actionable row: %#v", item)
			}
			count++
		}
		cursor = stringValue(group["next_cursor"])
		if cursor == "" {
			break
		}
	}
	if total != 8 || count != 8 {
		t.Fatalf("same source count/list: total=%d listed=%d", total, count)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.platform_workflow_intents SET status='completed' WHERE intent_id='r34c-failed'`); err != nil {
		t.Fatal(err)
	}
	page, err := f.app.OwnerPending(f.ctx, f.ownerID, url.Values{"kind": {"failed_task"}, "fluctlightId": {f.fluctlightID}}, false)
	if err != nil || intValue(page["count"]) != 7 {
		t.Fatalf("recovered task count: %#v %v", page, err)
	}
	foreign, err := f.app.OwnerPending(f.ctx, f.ownerID, url.Values{"kind": {"failed_task"}, "fluctlightId": {"unrelated-role"}}, false)
	if err == nil {
		t.Fatalf("unrelated role accepted: %#v", foreign)
	}
}

func TestR34CWorldTaskRetainsActualCycleStartForStalePresentation(t *testing.T) {
	f := v3OwnerFixture(t, "r34c-cycle-start")
	started := time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano)
	payload := map[string]any{"fluctlight_id": f.fluctlightID, "world_id": "clock-world", "cycle_status": "generating", "cycle_started_at": started}
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,intent_type,task_queue,status,payload,started_at) VALUES('r34c-clock','r34c-clock','world.events.generate','fixture','started',$1,now()-interval '1 day')`, jsonBytes(payload)); err != nil {
		t.Fatal(err)
	}
	task, err := f.app.OwnerTask(f.ctx, f.ownerID, "r34c-clock")
	if err != nil || mapValue(task["source"])["cycle_started_at"] != started {
		t.Fatalf("actual cycle start lost: %#v %v", task, err)
	}
}
