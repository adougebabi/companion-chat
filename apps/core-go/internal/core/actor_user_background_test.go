package core

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestActorUserInitializationJSONPreservesOnlyQualifiedHumanBackground(t *testing.T) {
	good := map[string]any{"core_persona": map[string]any{"identity": map[string]any{"name": "摇光"}}, "actor_user": map[string]any{"background": map[string]any{"name": "Vinson", "location_scope": "abroad", "timezone": nil, "meeting_confirmed": false}}}
	prepared, err := prepareInitializationResponse(good)
	if err != nil {
		t.Fatal(err)
	}
	if jsonString(prepared["actor_user"]) != jsonString(good["actor_user"]) || strings.Contains(jsonString(prepared["core_persona"]), "Vinson") {
		t.Fatalf("wrong subject/field loss %#v", prepared)
	}
	for _, value := range []any{nil, map[string]any{"background": map[string]any{"timezone": "Local"}}, map[string]any{"background": map[string]any{"timezone": "Nowhere/Zone"}}, map[string]any{"background": map[string]any{"meeting_confirmed": "false"}}, map[string]any{"background": map[string]any{"actor_id": "foreign"}}} {
		bad := cloneMap(good)
		bad["actor_user"] = value
		if _, err := prepareInitializationResponse(bad); err == nil {
			t.Fatalf("invalid user settings accepted %#v", value)
		}
	}
	legacy := cloneMap(good)
	delete(legacy, "actor_user")
	if prepared, err := prepareInitializationResponse(legacy); err != nil || prepared["actor_user"] != nil {
		t.Fatalf("legacy changed %v %#v", err, prepared)
	}
}

func TestActorUserBackgroundOwnerEditCASReplayAndChatCorrectionShareAuthority(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	at := f.app.now()
	f.app.Clock = func() time.Time { return at }
	old := recordActorFact(t, f, "user-settings-old", "assert", "location_scope", "same_building", "")
	memory, err := f.app.ExecuteTool(f.ctx, f.request("memory_event", "user-settings-memory", map[string]any{"content": "用户在同楼", "type": "semantic", "confidence": 0.9, "importance": 0.8, "actor_fact_ids": []any{old}}))
	if err != nil {
		t.Fatal(err)
	}
	revision, err := f.app.readCurrentFactsRevision(f.ctx, f.fluctlightID)
	if err != nil {
		t.Fatal(err)
	}
	body := map[string]any{"background": map[string]any{"name": "Vinson", "location_scope": "abroad", "timezone": nil, "meeting_confirmed": false}, "operation": "correct", "reason": "补充用户设定并纠正同楼误判", "expected_current_facts_revision": revision, "idempotency_key": "user-settings-1"}
	result, err := f.app.UpdateActorUserBackground(f.ctx, f.ownerID, f.fluctlightID, body)
	if err != nil {
		t.Fatal(err)
	}
	if mapValue(result["background"])["location_scope"] != "abroad" || mapValue(result["background"])["timezone"] != nil {
		t.Fatal(result)
	}
	replay, err := f.app.UpdateActorUserBackground(f.ctx, f.ownerID, f.fluctlightID, body)
	if err != nil || replay["replayed"] != true {
		t.Fatalf("replay %#v %v", replay, err)
	}
	changed := cloneMap(body)
	changed["reason"] = "不同请求内容"
	if _, err := f.app.UpdateActorUserBackground(f.ctx, f.ownerID, f.fluctlightID, changed); !errors.Is(err, ErrConflict) {
		t.Fatalf("changed replay allowed %v", err)
	}
	stale := cloneMap(body)
	stale["idempotency_key"] = "user-settings-stale"
	if _, err := f.app.UpdateActorUserBackground(f.ctx, f.ownerID, f.fluctlightID, stale); !errors.Is(err, ErrCurrentFactsStale) {
		t.Fatalf("stale save allowed %v", err)
	}
	if _, err := f.app.UpdateActorUserBackground(f.ctx, f.foreignOwnerID, f.fluctlightID, body); err == nil {
		t.Fatal("foreign owner edited user settings")
	}
	var status string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT provenance_status FROM public.memories WHERE id=$1`, mapValue(memory.Result.Output)["memory_id"]).Scan(&status); err != nil || status != "invalid" {
		t.Fatalf("dependent memory %s %v", status, err)
	}
	at = at.Add(time.Minute)
	source := actorFactSourceMessage(t, f, "settings-return", f.ownerID, "我今天已经回国")
	recordActorFact(t, f, "settings-return", "change", "location_scope", "domestic", source)
	current, err := f.app.readActorUserBackground(f.ctx, f.repository.Pool(), f.ownerID, f.fluctlightID, at)
	if err != nil || mapValue(current["background"])["location_scope"] != "domestic" {
		t.Fatalf("chat correction separate authority %#v %v", current, err)
	}
	detail, err := f.app.FluctlightDetail(f.ctx, f.ownerID, f.fluctlightID)
	if err != nil || mapValue(mapValue(detail["actor_user"])["background"])["name"] != "Vinson" {
		t.Fatalf("detail lost owner settings %#v %v", detail, err)
	}
}

func TestActorUserBackgroundActivationWritesFactsAtomicallyAndReplays(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'hash','revision-1') ON CONFLICT DO NOTHING`, f.ownerID); err != nil {
		t.Fatal(err)
	}
	seedCognitiveProviderRole(t, f.ctx, f.repository, "actor-user-init-provider")
	router := newFakeProviderRouter().on("persona_compilation_response", func(map[string]any) fakeProviderResult {
		return fakeProviderResult{Structured: map[string]any{"portrait_text": "摇光温暖且独立。"}}
	})
	app := newTestApp(t, f.repository, router)
	foundation := map[string]any{"core_persona": map[string]any{"identity": map[string]any{"name": "摇光"}}, "actor_user": map[string]any{"background": map[string]any{"name": "Vinson", "occupation": "工程师", "location_scope": "abroad", "timezone": nil}}}
	analysis, err := app.ImportInitializationJSON(f.ctx, f.ownerID, foundation)
	if err != nil {
		t.Fatal(err)
	}
	source := stringValue(analysis["analysis_id"])
	delete(analysis, "analysis_id")
	delete(analysis, "correlation_id")
	id := "actor-user-created-" + f.suffix
	created, err := app.CreateFluctlight(f.ctx, f.ownerID, id, "摇光", "llm_defined", source, analysis, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(jsonString(created.CorePersona), "Vinson") {
		t.Fatal("human background entered self Foundation")
	}
	if _, err := app.CreateFluctlight(f.ctx, f.ownerID, id, "摇光", "llm_defined", source, analysis, nil, nil); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.actor_facts WHERE owner_fluctlight_id=$1 AND subject_actor_id=$2`, id, f.ownerID).Scan(&count); err != nil || count != 4 {
		t.Fatalf("facts after replay %d %v", count, err)
	}
	projection, err := app.BuildContextProjection(f.ctx, f.ownerID, id, "", "", "继续聊聊")
	if err != nil || !strings.Contains(jsonString(compactActorBackground(projection)), "Vinson") {
		t.Fatalf("new instance background absent %v", err)
	}
	drift := cloneMap(analysis)
	drift["actor_user"] = map[string]any{"background": map[string]any{"name": "different"}}
	if _, err := app.CreateFluctlight(f.ctx, f.ownerID, id, "摇光", "llm_defined", source, drift, nil, nil); !errors.Is(err, ErrConflict) {
		t.Fatalf("activation payload drift %v", err)
	}
}

func TestActorUserBackgroundBatchFailureRollsBackFactsAndGeneration(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	old := recordActorFact(t, f, "batch-old-location", "assert", "location_scope", "same_building", "")
	revision, err := f.app.readCurrentFactsRevision(f.ctx, f.fluctlightID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `CREATE FUNCTION public.reject_actor_user_command_test() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'forced command ledger failure'; END $$; CREATE TRIGGER reject_actor_user_command_test BEFORE INSERT ON public.actor_user_background_commands FOR EACH ROW EXECUTE FUNCTION public.reject_actor_user_command_test()`); err != nil {
		t.Fatal(err)
	}
	body := map[string]any{"background": map[string]any{"name": "New User", "location_scope": "abroad"}, "operation": "correct", "reason": "atomic owner update", "idempotency_key": "batch-failure", "expected_current_facts_revision": revision}
	if _, err := f.app.UpdateActorUserBackground(f.ctx, f.ownerID, f.fluctlightID, body); err == nil {
		t.Fatal("ledger failure ignored")
	}
	var status string
	var count int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status FROM public.actor_facts WHERE id=$1`, old).Scan(&status); err != nil || status != "active" {
		t.Fatalf("old fact altered %s %v", status, err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.actor_facts WHERE owner_fluctlight_id=$1`, f.fluctlightID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("partial batch persisted %d %v", count, err)
	}
	current, err := f.app.readCurrentFactsRevision(f.ctx, f.fluctlightID)
	if err != nil || current != revision {
		t.Fatalf("generation advanced %q→%q %v", revision, current, err)
	}
}
