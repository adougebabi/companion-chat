package core

import (
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestDefaultNPCRoutinesAreValidAndAvoidHome(t *testing.T) {
	places := []string{"grocer", "food", "square", "meeting"}
	for index := range seedNPCNames {
		routine := defaultNPCRoutine(places, index)
		if len(routine) == 0 {
			t.Fatal("seed NPC has no routine")
		}
		for i, raw := range routine {
			slot := mapValue(raw)
			start, ok1 := parseNPCClock(stringValue(slot["start"]))
			end, ok2 := parseNPCClock(stringValue(slot["end"]))
			if !ok1 || !ok2 || end <= start || slot["day"] != "daily" || stringValue(slot["activity"]) == "" {
				t.Fatalf("invalid slot %v", slot)
			}
			for j := 0; j < i; j++ {
				other := mapValue(routine[j])
				os, _ := parseNPCClock(stringValue(other["start"]))
				oe, _ := parseNPCClock(stringValue(other["end"]))
				if start < oe && os < end {
					t.Fatalf("slots overlap: %v %v", other, slot)
				}
			}
		}
	}
	if len(defaultNPCRoutine(nil, 0)) != 0 {
		t.Fatal("routine invented a place")
	}
}

func TestArrivalPresenceAndWorldEffectsScope(t *testing.T) {
	presence, err := npcCharacterPresenceWith(nil, nil, "f", map[string]any{"source": "event", "authority_status": "inferred", "event_kind": scheduleArrivalEventKind, "place_id": "p", "behavior_state": "awake"}, time.Now())
	if err != nil || presence["kind"] != "confirmed_event" {
		t.Fatalf("schedule arrival must count as presence: %v %v", presence, err)
	}
	if p, _ := npcCharacterPresenceWith(nil, nil, "f", map[string]any{"source": "schedule", "authority_status": "accepted", "place_id": "p"}, time.Now()); p != nil {
		t.Fatal("a bare schedule plan is not presence; the arrival event is")
	}
	definition := map[string]any{"effects": []any{map[string]any{"scope": "world", "kind": "weather"}, map[string]any{"scope": "place", "place_id": "p1", "kind": "atmosphere"}}}
	if got := worldEventMemberEffects(definition, map[string]any{}); len(got) != 1 {
		t.Fatalf("member elsewhere should get only the world effect: %v", got)
	}
	if got := worldEventMemberEffects(definition, map[string]any{"place_id": "p1"}); len(got) != 2 {
		t.Fatalf("member at the place should get both: %v", got)
	}
}

// The whole loop against PostgreSQL: a schedule item at the grocer becomes a
// place arrival, the default resident's routine puts them there, an encounter
// opportunity appears, a world event reaches the member and leaves a memory,
// and editing the schedule moves the arrival.
func TestWorldLoopScheduleArrivalMeetsDefaultResident(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	now := time.Date(2026, 10, 19, 6, 0, 0, 0, time.UTC) // 14:00 Asia/Shanghai
	f.app.Clock = func() time.Time { return now }
	pool := f.repository.Pool()
	if _, err := f.app.DispatchWorldSurface(f.ctx, "PUT", "/internal/fluctlights/"+f.fluctlightID+"/world", nil, f.ownerID, map[string]any{"name": "日常世界", "creation_preset": "daily", "expected_revision": 0, "reason": "world loop test", "idempotency_key": "world-loop"}); err != nil {
		t.Fatal(err)
	}
	var world, grocer, food string
	if err := pool.QueryRow(f.ctx, `SELECT world_id FROM public.fluctlight_world_bindings WHERE fluctlight_id=$1`, f.fluctlightID).Scan(&world); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(f.ctx, `SELECT (SELECT id FROM public.world_places WHERE world_id=$1 AND framework_key='["shops","grocer"]'),(SELECT id FROM public.world_places WHERE world_id=$1 AND framework_key='["shops","food"]')`, world).Scan(&grocer, &food); err != nil {
		t.Fatal(err)
	}
	var mode string
	var home *string
	var routine []byte
	if err := pool.QueryRow(f.ctx, `SELECT position_mode,home_place_id,routine FROM public.world_npcs WHERE world_id=$1 AND name='周予安'`, world).Scan(&mode, &home, &routine); err != nil {
		t.Fatal(err)
	}
	if mode != "routine" || home != nil || !strings.Contains(string(routine), grocer) {
		t.Fatalf("default resident not placed: mode=%s home=%v routine=%s", mode, home, routine)
	}

	// An accepted schedule item at the grocer, 13:00-16:00 local.
	if _, err := pool.Exec(f.ctx, `DELETE FROM public.life_schedule_items WHERE schedule_id IN (SELECT id FROM public.life_schedules WHERE fluctlight_id=$1)`, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(f.ctx, `DELETE FROM public.life_schedules WHERE fluctlight_id=$1`, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(f.ctx, `INSERT INTO public.life_schedules(id,fluctlight_id,local_date,timezone,status,generated_from,evidence_refs,revision,reschedule_policy,idempotency_key,request_digest,result) VALUES('loop-schedule',$1,'2026-10-19','Asia/Shanghai','accepted','test','["fact"]',1,'{}','loop-schedule',$2,'{"id":"loop-schedule","status":"accepted","revision":1,"expected_context_revision":"life_ctx_before","resulting_context_revision":"life_ctx_after","replayed":false}')`, f.fluctlightID, stableDigest("loop-schedule")); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(f.ctx, `INSERT INTO public.life_schedule_items(id,schedule_id,start_at,end_at,activity,scene,location,item_type,status,priority,flexibility,interruption_cost,place_id) VALUES('loop-item','loop-schedule',$1,$2,'去杂货店买东西','杂货店','杂货店','planned','planned','0.5','0.5','0.5',$3)`, now.Add(-time.Hour), now.Add(2*time.Hour), grocer); err != nil {
		t.Fatal(err)
	}
	_, life, err := f.app.readLifeContextSnapshotAt(f.ctx, f.fluctlightID, now)
	if err != nil || life["source"] != "schedule" || life["place_id"] != grocer {
		t.Fatalf("schedule did not place the character: %v %v", life, err)
	}
	if count, err := f.app.EnsureScheduledArrivals(f.ctx, 50); err != nil || count != 1 {
		t.Fatalf("arrival = %d err=%v", count, err)
	}
	if count, err := f.app.EnsureScheduledArrivals(f.ctx, 50); err != nil || count != 0 {
		t.Fatalf("arrival repeated = %d err=%v", count, err)
	}
	_, life, err = f.app.readLifeContextSnapshotAt(f.ctx, f.fluctlightID, now)
	if err != nil || life["event_kind"] != scheduleArrivalEventKind || life["place_id"] != grocer || life["activity"] != "去杂货店买东西" {
		t.Fatalf("arrival does not mirror the schedule: %v %v", life, err)
	}

	if _, err := f.app.RepairWorldNPCPresence(f.ctx, 20); err != nil {
		t.Fatal(err)
	}
	var opportunities int
	if err := pool.QueryRow(f.ctx, `SELECT count(*) FROM public.world_npc_opportunities o JOIN public.world_npcs n ON n.id=o.npc_id WHERE o.fluctlight_id=$1 AND n.name='周予安' AND o.place_id=$2`, f.fluctlightID, grocer).Scan(&opportunities); err != nil || opportunities != 1 {
		t.Fatalf("no encounter opportunity with the resident at the grocer: %d %v", opportunities, err)
	}

	var enabled int
	if err := pool.QueryRow(f.ctx, `SELECT count(*) FROM public.world_event_templates WHERE world_id=$1 AND id LIKE 'world_template_starter_%' AND enabled AND definition->'trigger'->>'mode'='automatic'`, world).Scan(&enabled); err != nil || enabled != 2 {
		t.Fatalf("starter events not automatic: %d %v", enabled, err)
	}

	// A world-wide event reaches the member and leaves a memory.
	run := map[string]any{"id": "loop-run", "world_id": world}
	definition := map[string]any{"description": "天空落下短暂的小雨。", "effects": []any{map[string]any{"kind": "weather", "scope": "world", "value": "小雨"}}}
	if err := withTransaction(f.ctx, pool, func(tx pgx.Tx) error { return f.app.notifyWorldEventMembersTx(f.ctx, tx, run, definition, now) }); err != nil {
		t.Fatal(err)
	}
	var memories, facts int
	if err := pool.QueryRow(f.ctx, `SELECT (SELECT count(*) FROM public.memories WHERE owner_fluctlight_id=$1 AND content LIKE '%小雨%' AND content LIKE '%杂货店%'),(SELECT count(*) FROM public.cognition_inbox WHERE fluctlight_id=$1 AND event_type='life.world.event')`, f.fluctlightID).Scan(&memories, &facts); err != nil || memories != 1 || facts != 1 {
		t.Fatalf("world event memory=%d facts=%d err=%v", memories, facts, err)
	}

	// Editing the schedule (new revision, new place) moves the arrival.
	if _, err := pool.Exec(f.ctx, `UPDATE public.life_schedule_items SET place_id=$1,activity='去小吃店吃点东西',location='小吃店' WHERE id='loop-item'`, food); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(f.ctx, `UPDATE public.life_schedules SET revision=2 WHERE id='loop-schedule'`); err != nil {
		t.Fatal(err)
	}
	if count, err := f.app.EnsureScheduledArrivals(f.ctx, 50); err != nil || count != 1 {
		t.Fatalf("re-arrival = %d err=%v", count, err)
	}
	_, life, err = f.app.readLifeContextSnapshotAt(f.ctx, f.fluctlightID, now)
	if err != nil || life["event_kind"] != scheduleArrivalEventKind || life["place_id"] != food {
		t.Fatalf("edited schedule did not move the character: %v %v", life, err)
	}
}

func TestWorldDefaultsBackfillUntouchedSeedsOnly(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	pool := f.repository.Pool()
	if _, err := f.app.DispatchWorldSurface(f.ctx, "PUT", "/internal/fluctlights/"+f.fluctlightID+"/world", nil, f.ownerID, map[string]any{"name": "旧世界", "creation_preset": "daily", "expected_revision": 0, "reason": "backfill test", "idempotency_key": "world-backfill"}); err != nil {
		t.Fatal(err)
	}
	var world string
	if err := pool.QueryRow(f.ctx, `SELECT world_id FROM public.fluctlight_world_bindings WHERE fluctlight_id=$1`, f.fluctlightID).Scan(&world); err != nil {
		t.Fatal(err)
	}
	// Recreate the old state: unknown residents, manual disabled starters,
	// except one resident the Owner already edited.
	if _, err := pool.Exec(f.ctx, `UPDATE public.world_npcs SET position_mode='unknown',routine='[]',revision=1 WHERE world_id=$1`, world); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(f.ctx, `UPDATE public.world_npcs SET background='主人改过的背景' WHERE world_id=$1 AND name='林晚晴'`, world); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(f.ctx, `UPDATE public.world_event_templates SET enabled=false,revision=1,definition=jsonb_set(definition,'{trigger,mode}','"manual"') WHERE world_id=$1 AND id LIKE 'world_template_starter_%'`, world); err != nil {
		t.Fatal(err)
	}
	if count, err := f.app.EnsureWorldDefaults(f.ctx, 20); err != nil || count != 3 {
		t.Fatalf("backfill = %d err=%v (want 1 resident + 2 starters)", count, err)
	}
	if count, err := f.app.EnsureWorldDefaults(f.ctx, 20); err != nil || count != 0 {
		t.Fatalf("backfill repeated = %d err=%v", count, err)
	}
	var edited string
	if err := pool.QueryRow(f.ctx, `SELECT position_mode FROM public.world_npcs WHERE world_id=$1 AND name='林晚晴'`, world).Scan(&edited); err != nil || edited != "unknown" {
		t.Fatalf("backfill touched an Owner-edited resident: %s %v", edited, err)
	}
}
