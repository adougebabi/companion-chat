package core

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"sync"
	"testing"
)

func TestPostgresOwnerPersonaSwitchAssessmentExecutionAndNoChangeReceipt(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	const owner = "owner_persona_test"
	const fid = "persona_test"
	persona := map[string]any{"personality_system": map[string]any{"mode": "multiple", "active_profile_id": "a", "profiles": []any{map[string]any{"id": "a", "name": "A"}, map[string]any{"id": "b", "name": "B"}}, "switching": map[string]any{"cooldown_seconds": 60, "rules": []any{map[string]any{"id": "owner-b", "kind": "persistent_deterministic", "condition": "Owner explicitly selects B", "source_profile_id": "a", "target_profile_id": "b"}, map[string]any{"id": "owner-a", "kind": "persistent_deterministic", "condition": "Owner explicitly selects A", "target_profile_id": "a"}}}}}
	for _, statement := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active'),($2,'fluctlight','active')`, []any{owner, fid}},
		{`INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES($1,$2,'blank_slate','active',$3,'{}','{}','{}','{}','{}')`, []any{fid, owner, jsonBytes(persona)}},
		{`INSERT INTO public.fluctlight_affect_profiles(fluctlight_id) VALUES($1)`, []any{fid}},
		{`INSERT INTO public.fluctlight_personality_runtime(fluctlight_id,active_profile_id,revision) VALUES($1,'a',0)`, []any{fid}},
		{`INSERT INTO public.fluctlight_appearance_states(fluctlight_id,revision,state_json) VALUES($1,7,'{"hair_color":{"status":"known","value":"blue"}}')`, []any{fid}},
		{`INSERT INTO public.fluctlight_wardrobe_states(fluctlight_id,revision,wearing_state) VALUES($1,8,'known')`, []any{fid}},
		{`INSERT INTO public.fluctlight_wardrobe_items(id,fluctlight_id,category,slot,description,ownership,availability,source_kind,source_ref,source_item_key) VALUES('shirt',$1,'top','top','current shirt','owned','available','accepted_event','test-event','shirt')`, []any{fid}},
		{`INSERT INTO public.fluctlight_worn_items(fluctlight_id,slot,item_id) VALUES($1,'top','shirt')`, []any{fid}},
	} {
		if _, err := repository.Pool().Exec(ctx, statement.sql, statement.args...); err != nil {
			t.Fatal(err)
		}
	}
	app := &App{DB: repository}
	seedLegacyTestWorkingPersonas(t, app)
	request := func(key, target string, foundation, runtime int) map[string]any {
		return map[string]any{"idempotency_key": key, "target_persona_id": target, "expected_revision": foundation, "expected_runtime_revision": runtime, "reason": "Owner controlled switch", "confirm": true}
	}
	send := func(actor string, body map[string]any) (map[string]any, error) {
		return app.DispatchOwnerSurface(ctx, "POST", "/internal/fluctlights/"+fid+"/persona-switch", url.Values{}, actor, body)
	}
	assertCode := func(body map[string]any, actor, code string) {
		t.Helper()
		_, err := send(actor, body)
		var surface *SurfaceError
		if !errors.As(err, &surface) || surface.Code != code {
			t.Fatalf("want %s got %v", code, err)
		}
	}
	assertCode(request("foreign", "b", 0, 0), "foreign", "fluctlight_not_found")
	assertCode(request("missing", "unknown", 0, 0), owner, "personality_target_profile_not_found")
	assertCode(request("stale-foundation", "b", 99, 0), owner, "revision_conflict")
	assertCode(request("stale-runtime", "b", 0, 99), owner, "revision_conflict")
	legacy := request("legacy-pending", "b", 0, 0)
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.owner_surface_idempotency(owner_actor_id,idempotency_key,digest,response) VALUES($1,$2,$3,'{"status":"pending","_status":202}')`, owner, "legacy-pending", surfaceDigest("POST", "/internal/fluctlights/"+fid+"/persona-switch", legacy)); err != nil {
		t.Fatal(err)
	}
	assertCode(legacy, owner, "persona_switch_legacy_pending")
	// Parallel identical no-change clicks must replay one terminal result.
	var wg sync.WaitGroup
	results := make([]map[string]any, 2)
	errs := make([]error, 2)
	for i := range results {
		wg.Add(1)
		go func(i int) { defer wg.Done(); results[i], errs[i] = send(owner, request("switch-1", "a", 0, 0)) }(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil || results[i]["status"] != "completed" || intValue(results[i]["runtime_revision"]) != 0 {
			t.Fatalf("switch %d result=%v err=%v", i, results[i], err)
		}
	}
	if results[0]["request_id"] != results[1]["request_id"] || results[0]["audit_id"] != results[1]["audit_id"] {
		t.Fatal("concurrent replay changed receipt identity")
	}
	assertCode(request("switch-1", "b", 0, 1), owner, "idempotency_key_reused")
	app = &App{DB: repository} // New App instance: all authority and receipts come from PG.
	replay, err := send(owner, request("switch-1", "a", 0, 0))
	if err != nil || replay["_replay"] != true {
		t.Fatalf("restart replay=%v err=%v", replay, err)
	}
	stored, err := app.DispatchOwnerSurface(ctx, "GET", "/internal/owner-requests/"+stringValue(replay["request_id"]), url.Values{}, owner, nil)
	if err != nil || stored["status"] != "completed" {
		t.Fatalf("terminal read=%v err=%v", stored, err)
	}
	// The actual production Worker consumer calls the existing formal tool-free
	// assessment over a persisted Owner source, then uses the persona writer.
	seedCognitiveProviderRole(t, ctx, repository, "owner-persona-provider")
	router := newFakeProviderRouter().on(persistentSwitchAssessmentSchemaName, func(payload map[string]any) fakeProviderResult {
		if len(arrayValue(payload["tools"])) != 0 {
			t.Error("assessment exposed tools")
		}
		return fakeProviderResult{Structured: map[string]any{"personality_decision": map[string]any{"decision": "switch", "from_profile_id": "a", "target_profile_id": "b", "trigger_id": "switch:owner-b", "reason": "private-model-assessment-reason"}}}
	})
	app = newTestApp(t, repository, router)
	var admitted [2]map[string]any
	var admissionErrors [2]error
	for i := range admitted {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			admitted[i], admissionErrors[i] = send(owner, request("switch-b", "b", 0, 0))
		}(i)
	}
	wg.Wait()
	for i := range admitted {
		if admissionErrors[i] != nil || admitted[i]["status"] != "pending" {
			t.Fatalf("concurrent admission=%v err=%v", admitted[i], admissionErrors[i])
		}
	}
	if admitted[0]["request_id"] != admitted[1]["request_id"] {
		t.Fatal("concurrent admission duplicated request")
	}
	accepted, err := admitted[0], admissionErrors[0]
	if err != nil || accepted["status"] != "pending" || accepted["status_url"] == nil {
		t.Fatalf("admission=%v err=%v", accepted, err)
	}
	requestID := stringValue(accepted["request_id"])
	pending, err := app.DispatchOwnerSurface(ctx, "GET", "/internal/owner-requests/"+requestID, nil, owner, nil)
	if err != nil || pending["status"] != "pending" {
		t.Fatalf("pending=%v err=%v", pending, err)
	}
	terminal, err := app.ProcessOwnerPersonaSwitchIntent(ctx, requestID)
	if err != nil || terminal["status"] != "completed" {
		t.Fatalf("execution=%v err=%v", terminal, err)
	}
	app = newTestApp(t, repository, router)
	terminal, err = app.ProcessOwnerPersonaSwitchIntent(ctx, requestID)
	if err != nil || terminal["status"] != "completed" || router.requestCount(persistentSwitchAssessmentSchemaName) != 1 {
		t.Fatalf("Worker replay=%v err=%v", terminal, err)
	}
	stored, err = app.DispatchOwnerSurface(ctx, "GET", "/internal/owner-requests/"+requestID, nil, owner, nil)
	if err != nil || stored["status"] != "completed" || mapValue(stored["result"])["active_profile_id"] != "b" {
		t.Fatalf("durable terminal=%v err=%v", stored, err)
	}
	if failed, err := app.FailOwnerPersonaSwitchIntent(ctx, requestID); err != nil || failed["status"] != "completed" {
		t.Fatalf("late workflow failure changed completed request: %v %v", failed, err)
	}
	runtime, err := app.readPersonalityRuntime(ctx, fid, "a")
	if err != nil || runtime["active_profile_id"] != "b" || intValue(runtime["revision"]) != 1 {
		t.Fatalf("runtime=%v err=%v", runtime, err)
	}
	history, more, err := app.readPersonaSwitchHistory(ctx, fid)
	if err != nil || more || len(history) != 1 || history[0]["active_profile_id"] != "b" {
		t.Fatalf("history=%v more=%v err=%v", history, more, err)
	}
	if strings.Contains(jsonString(history), "private-model-assessment-reason") || strings.Contains(jsonString(runtime), "private-model-assessment-reason") {
		t.Fatal("private assessment reason leaked to public runtime/history")
	}
	projection, err := app.readOwnerPersonaProjection(ctx, fid, persona, 0, runtime)
	if err != nil || projection["status"] != "ready" || projection["working_persona"] == nil {
		t.Fatalf("projection=%v err=%v", projection, err)
	}
	var bodyRevision, wardrobeRevision int
	var bodyRaw []byte
	var worn string
	if err := repository.Pool().QueryRow(ctx, `SELECT a.revision,a.state_json,w.revision,i.item_id FROM public.fluctlight_appearance_states a JOIN public.fluctlight_wardrobe_states w USING(fluctlight_id) JOIN public.fluctlight_worn_items i USING(fluctlight_id) WHERE a.fluctlight_id=$1`, fid).Scan(&bodyRevision, &bodyRaw, &wardrobeRevision, &worn); err != nil || bodyRevision != 7 || wardrobeRevision != 8 || worn != "shirt" || stringValue(mapValue(decodeObject(bodyRaw)["hair_color"])["value"]) != "blue" {
		t.Fatalf("effective life changed body=%d wardrobe=%d worn=%s err=%v", bodyRevision, wardrobeRevision, worn, err)
	}

	assertCode(request("cooldown", "a", 0, 1), owner, "personality_switch_cooldown")
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.fluctlights SET status='retired' WHERE id=$1`, fid); err != nil {
		t.Fatal(err)
	}
	assertCode(request("retired", "a", 0, 1), owner, "fluctlight_retired")
	t.Log("real isolated PostgreSQL: Owner request → controlled Provider assessment → persistent switch, dual CAS, concurrent no-change receipt/replay, terminal read, Worker actual switch/history, reconstructed App, current body/worn preservation passed; controlled Provider transport, not live model")
}

func TestOwnerPersonaSwitchWithoutConnectedRuntimeIsNotPending(t *testing.T) {
	surface := NewMemoryOwnerSurface(SurfaceHooks{})
	_, err := surface.Dispatch(context.Background(), "POST", "/internal/fluctlights/fl-1/persona-switch", nil, "owner", map[string]any{"idempotency_key": "test", "target_persona_id": "b", "confirm": true})
	var failure *SurfaceError
	if !errors.As(err, &failure) || failure.Code != "persona_switch_unavailable" {
		t.Fatalf("unconnected switch returned %v", err)
	}
}
