package core

import (
	"errors"
	"net/url"
	"testing"
	"time"
)

func worldEventTestDefinition(effects ...any) map[string]any {
	return map[string]any{
		"schema_version": "world-event.v1", "description": "Explicit virtual-world test condition", "category": "environment", "intensity": "low", "duration_minutes": 5,
		"trigger":   map[string]any{"mode": "manual", "interval_minutes": 60, "cooldown_minutes": 10, "probability": 1},
		"condition": map[string]any{"day": "daily", "start": "00:00", "end": "00:00", "place_ids": []any{}, "require_presence": false}, "effects": effects,
	}
}
func applyTestWorldEvent(t *testing.T, f independentToolE2EFixture, d map[string]any) map[string]any {
	t.Helper()
	r, err := f.app.DispatchOwnerSurface(f.ctx, "POST", "/internal/studio/resources", nil, f.ownerID, map[string]any{"kind": "event_template", "name": "Event runtime test", "body": d, "reason": "explicit test definition", "idempotency_key": randomID("world_event_definition_")})
	if err != nil {
		t.Fatal(err)
	}
	published, err := f.app.publishWorldResource(f.ctx, f.ownerID, stringValue(r["id"]), map[string]any{"expected_revision": r["revision"], "reason": "test publication", "idempotency_key": randomID("world_event_publish_")})
	if err != nil {
		t.Fatal(err)
	}
	preview, err := f.app.DispatchWorldApply(f.ctx, f.ownerID, stringValue(r["id"]), map[string]any{"fluctlight_id": f.fluctlightID, "resource_version": published["published_version"]}, true)
	if err != nil {
		t.Fatal(err)
	}
	applied, err := f.app.DispatchWorldApply(f.ctx, f.ownerID, stringValue(r["id"]), map[string]any{"fluctlight_id": f.fluctlightID, "resource_version": published["published_version"], "expected_instance_revision": preview["instance_revision"], "expected_runtime_revision": preview["runtime_revision"], "choices": []any{}, "reason": "explicit runtime application", "idempotency_key": randomID("world_event_apply_")}, false)
	if err != nil || applied["applied"] != true {
		t.Fatal(applied, err)
	}
	result, err := readWorldTemplateWith(f.ctx, f.repository.Pool(), f.fluctlightID, stringValue(applied["id"]), false)
	if err != nil {
		t.Fatal(err)
	}
	return result
}
func eventRuntimeCall(t *testing.T, f independentToolE2EFixture, method, path string, b map[string]any) map[string]any {
	t.Helper()
	if b != nil {
		b["reason"] = "explicit test world command"
		if b["idempotency_key"] == nil {
			b["idempotency_key"] = randomID("world_event_command_")
		}
	}
	r, err := f.app.DispatchWorldSurface(f.ctx, method, "/internal/fluctlights/"+f.fluctlightID+"/world/templates"+path, nil, f.ownerID, b)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func TestWorldEventDefinitionClosedEffectsAndOvernightConditions(t *testing.T) {
	valid := worldEventTestDefinition(map[string]any{"kind": "weather", "scope": "world", "value": "virtual rain"})
	if err := validateWorldEventDefinition(valid); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"permanent_harm", "death", "money_transfer", "change_personality"} {
		d := worldEventTestDefinition(map[string]any{"kind": kind, "scope": "character", "value": "forbidden"})
		if err := validateWorldEventDefinition(d); err == nil {
			t.Fatal("unimplemented effect accepted", kind)
		}
	}
	duplicate := worldEventTestDefinition(map[string]any{"kind": "place_availability", "scope": "place", "place_id": "p", "value": "open"}, map[string]any{"kind": "place_availability", "scope": "place", "place_id": "p", "value": "closed"})
	if validateWorldEventDefinition(duplicate) == nil {
		t.Fatal("contradictory effects accepted")
	}
	c := map[string]any{"day": "weekday", "start": "22:00", "end": "02:00", "place_ids": []any{}}
	// Saturday 01:00 belongs to Friday's cross-midnight interval.
	if reason := worldEventConditionReason(c, map[string]any{}, time.Date(2026, 10, 24, 1, 0, 0, 0, time.UTC)); reason != "" {
		t.Fatal(reason)
	}
	if reason := worldEventConditionReason(c, map[string]any{}, time.Date(2026, 10, 25, 1, 0, 0, 0, time.UTC)); reason != "day_condition_not_met" {
		t.Fatal(reason)
	}
}
func TestWorldEventQueueExecutionReplayCooldownAndQueryPurity(t *testing.T) {
	f, _, _, now := seedNPCEncounterRuntime(t, true)
	d := worldEventTestDefinition(map[string]any{"kind": "weather", "scope": "world", "value": "virtual rain"})
	template := applyTestWorldEvent(t, f, d)
	id := stringValue(template["id"])
	if template["enabled"] != false {
		t.Fatal("application triggered/enabled the world", template)
	}
	_, before, err := f.app.readLifeContextSnapshotAt(f.ctx, f.fluctlightID, *now)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		eventRuntimeCall(t, f, "GET", "", nil)
		if _, err := f.app.worldLifeEvents(f.ctx, f.ownerID, f.fluctlightID, "", url.Values{}); err != nil {
			t.Fatal(err)
		}
	}
	var count int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.world_event_runs WHERE fluctlight_id=$1`, f.fluctlightID).Scan(&count); err != nil || count != 0 {
		t.Fatal("browsing created execution", count, err)
	}
	toggle := eventRuntimeCall(t, f, "PUT", "/"+id, map[string]any{"enabled": true, "expected_revision": template["revision"]})
	body := map[string]any{"expected_revision": toggle["revision"], "idempotency_key": randomID("stable_world_trigger_")}
	queued := eventRuntimeCall(t, f, "POST", "/"+id+"/runs", body)
	if queued["status"] != "queued" {
		t.Fatal(queued)
	}
	replay := eventRuntimeCall(t, f, "POST", "/"+id+"/runs", body)
	if replay["id"] != queued["id"] {
		t.Fatal("replay duplicated queued run")
	}
	if _, err := f.app.RepairWorldEvents(f.ctx, 20); err != nil {
		t.Fatal(err)
	}
	run, err := f.app.worldEventRun(f.ctx, f.ownerID, f.fluctlightID, id, stringValue(queued["id"]))
	if err != nil || run["status"] != "applied" || run["life_event_id"] == nil {
		t.Fatal(run, err)
	}
	_, after, err := f.app.readLifeContextSnapshotAt(f.ctx, f.fluctlightID, *now)
	if err != nil {
		t.Fatal(err)
	}
	if after["event_id"] != before["event_id"] || after["place_id"] != before["place_id"] || len(arrayValue(after["world_effects"])) != 1 || after["context_revision"] == before["context_revision"] {
		t.Fatal("ambient event moved character or was not consumed", before, after)
	}
	if _, err := f.app.RepairWorldEvents(f.ctx, 20); err != nil {
		t.Fatal(err)
	}
	next := eventRuntimeCall(t, f, "POST", "/"+id+"/runs", map[string]any{"expected_revision": toggle["revision"]})
	if _, err := f.app.RepairWorldEvents(f.ctx, 20); err != nil {
		t.Fatal(err)
	}
	filtered, err := f.app.worldEventRun(f.ctx, f.ownerID, f.fluctlightID, id, stringValue(next["id"]))
	if err != nil || filtered["status"] != "filtered" || mapValue(filtered["result"])["reason"] != "cooldown_or_active_effect" {
		t.Fatal(filtered, err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.life_events WHERE fluctlight_id=$1 AND kind='world_event'`, f.fluctlightID).Scan(&count); err != nil || count != 1 {
		t.Fatal("duplicate side effects", count, err)
	}
	event, err := f.app.worldLifeEvents(f.ctx, f.ownerID, f.fluctlightID, stringValue(run["life_event_id"]), nil)
	if err != nil || event["context_scope"] != "ambient" || len(arrayValue(event["impacts"])) != 1 {
		t.Fatal(event, err)
	}
}
func TestWorldEventMinorInjuryActuallyRecoversWithoutMovingCharacter(t *testing.T) {
	f, _, _, now := seedNPCEncounterRuntime(t, true)
	d := worldEventTestDefinition(map[string]any{"kind": "temporary_injury", "scope": "character", "value": "small temporary scratch", "body_effect": map[string]any{"body_part": "hand", "description": "temporary small scratch", "severity": "minor", "impact": "pain"}})
	mapValue(d["condition"])["require_presence"] = true
	template := applyTestWorldEvent(t, f, d)
	id := stringValue(template["id"])
	toggle := eventRuntimeCall(t, f, "PUT", "/"+id, map[string]any{"enabled": true, "expected_revision": template["revision"]})
	queued := eventRuntimeCall(t, f, "POST", "/"+id+"/runs", map[string]any{"expected_revision": toggle["revision"]})
	if _, err := f.app.RepairWorldEvents(f.ctx, 20); err != nil {
		t.Fatal(err)
	}
	run, err := f.app.worldEventRun(f.ctx, f.ownerID, f.fluctlightID, id, stringValue(queued["id"]))
	if err != nil || run["recovery_status"] != "pending" {
		t.Fatal(run, err)
	}
	var active bool
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT EXISTS(SELECT 1 FROM public.fluctlight_appearance_states s CROSS JOIN LATERAL jsonb_array_elements(s.state_json->'injuries'->'value') i WHERE s.fluctlight_id=$1 AND i->>'id'=$2 AND i->>'status'='active')`, f.fluctlightID, run["life_event_id"]).Scan(&active); err != nil || !active {
		t.Fatal("injury never applied", active, err)
	}
	*now = now.Add(6 * time.Minute)
	if _, err := f.app.RepairWorldEvents(f.ctx, 20); err != nil {
		t.Fatal(err)
	}
	recovered, err := f.app.worldEventRun(f.ctx, f.ownerID, f.fluctlightID, id, stringValue(run["id"]))
	if err != nil || recovered["recovery_status"] != "completed" || mapValue(recovered["recovery_result"])["id"] == nil {
		t.Fatal(recovered, err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT EXISTS(SELECT 1 FROM public.fluctlight_appearance_states s CROSS JOIN LATERAL jsonb_array_elements(s.state_json->'injuries'->'value') i WHERE s.fluctlight_id=$1 AND i->>'id'=$2 AND i->>'status'='recovered')`, f.fluctlightID, run["life_event_id"]).Scan(&active); err != nil || !active {
		t.Fatal("recovery has no physical result", active, err)
	}
}
func TestWorldEventVisitCreatesOpportunityAndCancelledSourceInvalidatesImmediately(t *testing.T) {
	f, npc, place, now := seedNPCEncounterRuntime(t, true)
	template := applyTestWorldEvent(t, f, worldEventTestDefinition(map[string]any{"kind": "npc_visit", "scope": "place", "place_id": place, "npc_id": npc, "value": "a configured visit"}))
	id := stringValue(template["id"])
	toggle := eventRuntimeCall(t, f, "PUT", "/"+id, map[string]any{"enabled": true, "expected_revision": template["revision"]})
	queued := eventRuntimeCall(t, f, "POST", "/"+id+"/runs", map[string]any{"expected_revision": toggle["revision"]})
	if _, err := f.app.RepairWorldEvents(f.ctx, 20); err != nil {
		t.Fatal(err)
	}
	detail, err := f.app.worldNPCDetail(f.ctx, f.fluctlightID, npc)
	if err != nil || mapValue(detail["current_presence"])["source_kind"] != "world_event_execution" {
		t.Fatal(detail, err)
	}
	page, err := f.app.WorldNPCRecords(f.ctx, f.ownerID, f.fluctlightID, npc, "opportunities", url.Values{})
	if err != nil || len(arrayValue(page["items"])) != 1 {
		t.Fatal(page, err)
	}
	opportunity := mapValue(arrayValue(page["items"])[0])
	var relationships int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.relationships WHERE owner_fluctlight_id=$1 AND target_actor_id=(SELECT actor_id FROM public.world_npcs WHERE id=$2)`, f.fluctlightID, npc).Scan(&relationships); err != nil || relationships != 0 {
		t.Fatal("visit became acquaintance", relationships, err)
	}
	run, err := f.app.worldEventRun(f.ctx, f.ownerID, f.fluctlightID, id, stringValue(queued["id"]))
	if err != nil {
		t.Fatal(err)
	}
	_, life, err := f.app.readLifeContextSnapshotAt(f.ctx, f.fluctlightID, *now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.CancelLifeEvent(f.ctx, f.ownerID, f.fluctlightID, stringValue(run["life_event_id"]), map[string]any{"expected_event_revision": 1, "expected_life_context_revision": life["context_revision"], "idempotency_key": randomID("cancel_visit_")}); err != nil {
		t.Fatal(err)
	}
	detail, err = f.app.worldNPCDetail(f.ctx, f.fluctlightID, npc)
	if err != nil || len(mapValue(detail["current_presence"])) != 0 {
		t.Fatal("cancelled visit remains current without another worker tick", detail, err)
	}
	tx, err := f.repository.Pool().Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(f.ctx)
	if _, err := f.app.validateNPCOpportunityTx(f.ctx, tx, opportunity, *now); !errors.Is(err, ErrConflict) {
		t.Fatal("cancelled visit is executable", err)
	}
}
func TestWorldEventAutomaticSelectionAndScheduledDisableKeepRealResults(t *testing.T) {
	f, _, place, now := seedNPCEncounterRuntime(t, true)
	d := worldEventTestDefinition(map[string]any{"kind": "place_availability", "scope": "place", "place_id": place, "value": "closed"})
	mapValue(d["trigger"])["mode"] = "automatic"
	mapValue(d["trigger"])["probability"] = 0
	template := applyTestWorldEvent(t, f, d)
	id := stringValue(template["id"])
	toggle := eventRuntimeCall(t, f, "PUT", "/"+id, map[string]any{"enabled": true, "expected_revision": template["revision"]})
	if _, err := f.app.RepairWorldEvents(f.ctx, 20); err != nil {
		t.Fatal(err)
	}
	page := eventRuntimeCall(t, f, "GET", "/"+id+"/runs", nil)
	if len(arrayValue(page["items"])) != 1 || mapValue(arrayValue(page["items"])[0])["status"] != "filtered" {
		t.Fatal("zero probability was not recorded as filtered", page)
	}
	// A controlled manual trigger still checks actual conditions and cooldown;
	// it explicitly bypasses only the automatic chance selection.
	manual := eventRuntimeCall(t, f, "POST", "/"+id+"/runs", map[string]any{"expected_revision": toggle["revision"]})
	if _, err := f.app.RepairWorldEvents(f.ctx, 20); err != nil {
		t.Fatal(err)
	}
	r, err := f.app.worldEventRun(f.ctx, f.ownerID, f.fluctlightID, id, stringValue(manual["id"]))
	if err != nil || r["status"] != "applied" {
		t.Fatal(r, err)
	}
	tx, err := f.repository.Pool().Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := requireWorldPlaceOpenTx(f.ctx, tx, f.fluctlightID, place, *now); !errors.Is(err, ErrConflict) {
		t.Fatal("closed place not consumed by activity gate", err)
	}
	tx.Rollback(f.ctx)
	scheduled := eventRuntimeCall(t, f, "POST", "/"+id+"/runs", map[string]any{"expected_revision": toggle["revision"], "scheduled_at": now.Add(2 * time.Hour).Format(time.RFC3339Nano)})
	eventRuntimeCall(t, f, "PUT", "/"+id, map[string]any{"enabled": false, "expected_revision": toggle["revision"]})
	*now = now.Add(2 * time.Hour)
	if _, err := f.app.RepairWorldEvents(f.ctx, 20); err != nil {
		t.Fatal(err)
	}
	r, err = f.app.worldEventRun(f.ctx, f.ownerID, f.fluctlightID, id, stringValue(scheduled["id"]))
	if err != nil || r["status"] != "filtered" || mapValue(r["result"])["reason"] != "template_disabled_or_changed" {
		t.Fatal("queued old template executed after disable", r, err)
	}
}
func TestNPCInterruptedRoutineResumesAsNewActualSegment(t *testing.T) {
	for _, cancel := range []bool{false, true} {
		t.Run(map[bool]string{false: "expiry", true: "source_cancel"}[cancel], func(t *testing.T) {
			f, npc, place, now := seedNPCEncounterRuntime(t, true)
			if _, err := f.app.RepairWorldNPCPresence(f.ctx, 20); err != nil {
				t.Fatal(err)
			}
			before, err := f.app.worldNPCDetail(f.ctx, f.fluctlightID, npc)
			if err != nil {
				t.Fatal(err)
			}
			old := mapValue(before["current_presence"])
			if old["status"] != "active" {
				t.Fatal(old)
			}
			*now = now.Add(time.Minute)
			d := worldEventTestDefinition(map[string]any{"kind": "npc_visit", "scope": "place", "place_id": place, "npc_id": npc, "value": "temporary visit"})
			template := applyTestWorldEvent(t, f, d)
			id := stringValue(template["id"])
			toggle := eventRuntimeCall(t, f, "PUT", "/"+id, map[string]any{"enabled": true, "expected_revision": template["revision"]})
			queued := eventRuntimeCall(t, f, "POST", "/"+id+"/runs", map[string]any{"expected_revision": toggle["revision"]})
			if _, err := f.app.RepairWorldEvents(f.ctx, 20); err != nil {
				t.Fatal(err)
			}
			if cancel {
				run, err := f.app.worldEventRun(f.ctx, f.ownerID, f.fluctlightID, id, stringValue(queued["id"]))
				if err != nil {
					t.Fatal(err)
				}
				_, life, err := f.app.readLifeContextSnapshotAt(f.ctx, f.fluctlightID, *now)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := f.app.CancelLifeEvent(f.ctx, f.ownerID, f.fluctlightID, stringValue(run["life_event_id"]), map[string]any{"expected_event_revision": 1, "expected_life_context_revision": life["context_revision"], "idempotency_key": randomID("cancel_routine_visit_")}); err != nil {
					t.Fatal(err)
				}
			} else {
				*now = now.Add(6 * time.Minute)
			}
			if _, err := f.app.RepairWorldNPCPresence(f.ctx, 20); err != nil {
				t.Fatal(err)
			}
			after, err := f.app.worldNPCDetail(f.ctx, f.fluctlightID, npc)
			resumed := mapValue(after["current_presence"])
			if err != nil || resumed["status"] != "active" || resumed["source_kind"] != "npc_routine_execution" || resumed["id"] == old["id"] {
				t.Fatal("routine did not resume with fresh actual source", after, err)
			}
			actual, err := time.Parse(time.RFC3339Nano, stringValue(resumed["start_at"]))
			if err != nil || !actual.Equal(*now) {
				t.Fatal("resumption backfilled visit time", resumed, err)
			}
		})
	}
}
func TestActivityOmittedPlaceRetainsCurrentStableIdentity(t *testing.T) {
	life := map[string]any{"place_id": "closed-shop", "location": "Shop", "scene": "shopping"}
	id, err := resolveUnscheduledActivityPlace(map[string]any{}, life)
	if err != nil || id != "closed-shop" {
		t.Fatal("omission dropped identity", id, err)
	}
	if _, err := resolveUnscheduledActivityPlace(map[string]any{"location": "different unbound shop"}, life); !errors.Is(err, ErrInvalidArguments) {
		t.Fatal("descriptive move bypassed stable-place requirement", err)
	}
}
