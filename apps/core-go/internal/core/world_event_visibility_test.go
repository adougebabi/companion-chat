package core

import (
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestWorldEventDiagnosticsFiltersReachOldClocksAndWorldOnlyGenerations(t *testing.T) {
	f, _, _, now := seedNPCEncounterRuntime(t, false)
	world, err := worldIDWith(f.ctx, f.repository.Pool(), f.fluctlightID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'test-only','world-test') ON CONFLICT DO NOTHING`, f.ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.EnsureWorldEventGenerationIntents(f.ctx, 100); err != nil {
		t.Fatal(err)
	}
	intent := "world_event_generation:" + world
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.platform_workflow_intents SET created_at=now()-interval '2 days' WHERE intent_id=$1`, intent); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 21; i++ {
		if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload) VALUES($1,$1,'test','test.recent','{}')`, fmt.Sprintf("recent-%d", i)); err != nil {
			t.Fatal(err)
		}
	}
	recent, err := f.app.OwnerDiagnosticRecords(f.ctx, f.ownerID, "workflow-intents", url.Values{"limit": {"20"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(recent["items"].([]map[string]any)) != 20 {
		t.Fatal(recent)
	}
	for _, row := range recent["items"].([]map[string]any) {
		if mapValue(row["data"])["intent_id"] == intent {
			t.Fatal("fixture must put the clock beyond recent tasks")
		}
	}
	clocks, err := f.app.OwnerDiagnosticRecords(f.ctx, f.ownerID, "workflow-intents", url.Values{"worldId": {world}, "intentType": {"world.events.generate"}, "limit": {"1"}})
	if err != nil {
		t.Fatal(err)
	}
	rows := clocks["items"].([]map[string]any)
	if len(rows) != 1 || mapValue(rows[0]["data"])["intent_id"] != intent || clocks["next_cursor"] != "" {
		t.Fatal(clocks)
	}
	if data := mapValue(rows[0]["data"]); data["world_id"] != world || stringValue(data["world_name"]) == "" {
		t.Fatal("world clock lost its visible world identity", data)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.world_event_generations(id,world_id,slot_at,status,proposal,model_correlation_id) VALUES('visible-no-event',$1,$2,'no_event','{"reason":"平静的一小时"}','world-visible'),('visible-admitted',$1,$3,'admitted','{"name":"晚间市集"}','world-visible')`, world, now.UTC().Truncate(time.Hour), now.UTC().Truncate(time.Hour).Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	first, err := f.app.OwnerDiagnosticRecords(f.ctx, f.ownerID, "world-event-generations", url.Values{"worldId": {world}, "limit": {"1"}})
	if err != nil {
		t.Fatal(err)
	}
	items := first["items"].([]map[string]any)
	if len(items) != 1 || items[0]["id"] != "visible-no-event" || first["next_cursor"] == "" {
		t.Fatal(first)
	}
	second, err := f.app.OwnerDiagnosticRecords(f.ctx, f.ownerID, "world-event-generations", url.Values{"worldId": {world}, "limit": {"1"}, "cursor": {stringValue(first["next_cursor"])}})
	if err != nil {
		t.Fatal(err)
	}
	items = second["items"].([]map[string]any)
	if len(items) != 1 || mapValue(items[0]["data"])["title"] != "晚间市集" || second["next_cursor"] != "" {
		t.Fatal(second)
	}
	for _, kind := range []string{"workflow-intents", "world-event-generations"} {
		page, err := f.app.OwnerDiagnosticRecords(f.ctx, f.ownerID, kind, url.Values{"worldId": {"another-world"}})
		if err != nil || len(page["items"].([]map[string]any)) != 0 {
			t.Fatal("world scope leaked", page, err)
		}
	}
	if _, err := f.app.OwnerDiagnosticRecords(f.ctx, f.ownerID, "world-event-generations", url.Values{"worldId": {"another-world"}, "limit": {"1"}, "cursor": {stringValue(first["next_cursor"])}}); err == nil {
		t.Fatal("cursor accepted a different world")
	}
	if _, err := f.app.OwnerDiagnosticRecords(f.ctx, "not-the-owner", "workflow-intents", nil); err == nil {
		t.Fatal("unauthorized diagnostics read")
	}
}

func TestWorldDiagnosticFiltersValidateLength(t *testing.T) {
	for _, key := range []string{"worldId", "intentType"} {
		if _, err := parseOwnerDiagnosticFilter(url.Values{key: {strings.Repeat("x", 257)}}); err == nil {
			t.Fatal("unbounded filter", key)
		}
	}
}
