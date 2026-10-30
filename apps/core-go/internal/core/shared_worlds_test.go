package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/ai/decision"
	"github.com/jackc/pgx/v5"
)

func seedSharedWorldMember(t *testing.T, f independentToolE2EFixture, id, owner string) {
	t.Helper()
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'fluctlight','active')`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) SELECT $1,$2,initialization_mode,'active',core_persona,identity,personality,behavioral_policy,life_profile,provenance FROM public.fluctlights WHERE id=$3`, id, owner, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.fluctlight_personality_runtime(fluctlight_id,active_profile_id,revision) VALUES($1,'default',0)`, id); err != nil {
		t.Fatal(err)
	}
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error { return initializeEffectiveLifeTx(f.ctx, tx, id, map[string]any{}) }); err != nil {
		t.Fatal(err)
	}
}

func TestSharedWorldSameNPCIndependentRelationshipsAndPresence(t *testing.T) {
	f, npc, place, now := seedNPCEncounterRuntime(t, true)
	world, err := worldIDWith(f.ctx, f.repository.Pool(), f.fluctlightID)
	if err != nil {
		t.Fatal(err)
	}
	other := "shared-member"
	seedSharedWorldMember(t, f, other, f.ownerID)
	if _, err := f.app.DispatchWorldSurface(f.ctx, "PUT", "/internal/fluctlights/"+other+"/world/active", nil, f.ownerID, map[string]any{"world_id": world, "expected_revision": 0, "reason": "join shared world", "idempotency_key": "shared-join"}); err != nil {
		t.Fatal(err)
	}
	first, err := f.app.WorldSnapshot(f.ctx, f.ownerID, f.fluctlightID)
	if err != nil {
		t.Fatal(err)
	}
	second, err := f.app.WorldSnapshot(f.ctx, f.ownerID, other)
	if err != nil {
		t.Fatal(err)
	}
	if mapValue(second["world"])["id"] != world || mapValue(arrayValue(second["npcs"])[0])["id"] != npc || mapValue(arrayValue(first["npcs"])[0])["actor_id"] != mapValue(arrayValue(second["npcs"])[0])["actor_id"] {
		t.Fatal("joining cloned world/NPC identity")
	}
	// Each member gets an independent opportunity from one executed NPC presence.
	life := currentLifeForTest(t, f.ctx, f.app, other, *now)
	if _, err := f.app.CreateLifeEvent(f.ctx, f.ownerID, other, map[string]any{"kind": "visit", "start_at": now.Add(-time.Minute).Format(time.RFC3339Nano), "end_at": now.Add(time.Hour).Format(time.RFC3339Nano), "scene": "reading", "activity": "reading", "location": "NPC runtime room", "place_id": place, "expected_life_context_revision": life["context_revision"], "evidence_refs": []any{"owner:" + f.ownerID}, "idempotency_key": "shared-arrival"}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.RepairWorldNPCPresence(f.ctx, 20); err != nil {
		t.Fatal(err)
	}
	var presences, opportunities int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT (SELECT count(*) FROM public.world_npc_presence_runs WHERE npc_id=$1),(SELECT count(DISTINCT fluctlight_id) FROM public.world_npc_opportunities WHERE npc_id=$1)`, npc).Scan(&presences, &opportunities); err != nil || presences != 1 || opportunities != 2 {
		t.Fatal("shared presence/member fanout", presences, opportunities, err)
	}
	page, err := f.app.WorldNPCRecords(f.ctx, f.ownerID, other, npc, "opportunities", url.Values{})
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range arrayValue(page["items"]) {
		if mapValue(raw)["fluctlight_id"] != other {
			t.Fatal("other member opportunity leaked")
		}
	}
	// World execution is independent of the character that created it.
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlights SET status='paused' WHERE id=ANY($1::varchar[])`, []string{f.fluctlightID, other}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `DELETE FROM public.fluctlight_world_bindings WHERE world_id=$1`, world); err != nil {
		t.Fatal(err)
	}
	if count, err := f.app.RepairWorldNPCPresence(f.ctx, 20); err != nil || count != 1 {
		t.Fatal("world NPC clock depended on creator/members", count, err)
	}
	foreign := "foreign-world-member"
	seedSharedWorldMember(t, f, foreign, f.foreignOwnerID)
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.fluctlight_world_bindings(fluctlight_id,world_id) VALUES($1,$2)`, foreign, world); err == nil {
		t.Fatal("database admitted cross-Owner world member")
	}
}

func TestSharedWorldSchedulePlacesCommitAndReplay(t *testing.T) {
	f, _, _, now := seedNPCEncounterRuntime(t, false)
	life := currentLifeForTest(t, f.ctx, f.app, f.fluctlightID, *now)
	input := fullDaySchedulePayloadForTest(*now, "generated-place", stringValue(life["context_revision"]))
	for _, raw := range arrayValue(input["items"]) {
		item := mapValue(raw)
		item["place_ref"] = "home"
		item["location"] = "明确的角色住处"
	}
	input["new_places"] = []any{map[string]any{"key": "home", "name": "明确的角色住处", "description": "已声明的住处"}}
	places, err := f.app.readScheduleWorldPlaces(f.ctx, f.fluctlightID)
	if err != nil {
		t.Fatal(err)
	}
	places.bind(input)
	before := jsonString(input)
	first, err := f.app.AcceptSchedule(f.ctx, f.ownerID, f.fluctlightID, input)
	if err != nil {
		t.Fatal(err)
	}
	again, err := f.app.AcceptSchedule(f.ctx, f.ownerID, f.fluctlightID, input)
	if err != nil || again["id"] != first["id"] || again["replayed"] != true {
		t.Fatal("place acceptance replay", again, err)
	}
	if jsonString(input) != before {
		t.Fatal("acceptance rewrote original model proposal")
	}
	var valid bool
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*)=1 FROM public.world_places p WHERE p.world_id=$1 AND p.name='明确的角色住处' AND EXISTS(SELECT 1 FROM public.life_schedule_items i WHERE i.schedule_id=$2 AND i.place_id=p.id)`, places.WorldID, first["id"]).Scan(&valid); err != nil || !valid {
		t.Fatal("text location did not close into stable world place", valid, err)
	}
}

func TestWorldEventGenerationRequiresKevAndRunsOncePerWorldHour(t *testing.T) {
	for _, choice := range []string{"yes", "no", "unclear", "disabled", "no_event"} {
		t.Run(choice, func(t *testing.T) {
			f, _, _, now := seedNPCEncounterRuntime(t, false)
			world, err := worldIDWith(f.ctx, f.repository.Pool(), f.fluctlightID)
			if err != nil {
				t.Fatal(err)
			}
			// Keep the test world independent of any member.
			if _, err := f.repository.Pool().Exec(f.ctx, `DELETE FROM public.fluctlight_world_bindings WHERE world_id=$1`, world); err != nil {
				t.Fatal(err)
			}
			seedCognitiveProviderRole(t, f.ctx, f.repository, "world-event-test")
			models, kevCalls := 0, 0
			f.app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().on("world_event_generation_v1", func(payload map[string]any) fakeProviderResult {
				models++
				text := jsonString(payload["messages"])
				if strings.Contains(text, f.fluctlightID) || !strings.Contains(text, "NPC runtime room") {
					t.Error("world task leaked member identity or lost places")
				}
				if choice == "no_event" {
					return fakeProviderResult{Structured: map[string]any{"decision": "no_event", "reason": "安静的一小时"}}
				}
				return fakeProviderResult{Structured: map[string]any{"decision": "propose", "name": "小雨", "reason": "符合世界情境", "event": worldEventTestDefinition(map[string]any{"kind": "weather", "scope": "world", "value": "温和的小雨"})}}
			})}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				kevCalls++
				var req decision.Request
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Error(err)
				}
				answers := map[string]any{}
				for id := range req.Questions {
					c := choice
					if c == "disabled" || c == "no_event" {
						c = "yes"
					}
					prob := map[string]float64{"yes": 0.05, "no": 0.05, "unclear": 0.05}
					prob[c] = 0.9
					answers[id] = map[string]any{"type": "choice", "choice": c, "confidence": 0.9, "probabilities": prob}
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"answers": answers})
			}))
			defer server.Close()
			config := decision.DefaultConfig()
			config.Enabled = choice != "disabled"
			config.Endpoint = server.URL
			if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.runtime_settings(key,value_json,revision) VALUES('kev',$1,1) ON CONFLICT(key) DO UPDATE SET value_json=excluded.value_json,revision=1`, jsonString(config)); err != nil {
				t.Fatal(err)
			}
			f.app.Kev = &decision.Service{Settings: &kevFixedSettings{config}, Store: &kevStore{app: f.app, directory: t.TempDir()}, HTTP: server.Client(), ID: func() string { return randomID("world_kev_") }}
			result, err := f.app.GenerateWorldEventCycle(f.ctx, world)
			if err != nil {
				t.Fatal(result, err)
			}
			if _, err := f.app.GenerateWorldEventCycle(f.ctx, world); err != nil {
				t.Fatal(err)
			}
			if models != 1 {
				t.Fatal("same world/hour repeated generation", models)
			}
			var templates, runs int
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT (SELECT count(*) FROM public.world_event_templates WHERE generation_id IS NOT NULL),(SELECT count(*) FROM public.world_event_runs)`).Scan(&templates, &runs); err != nil {
				t.Fatal(err)
			}
			if choice == "yes" {
				if templates != 1 || runs != 1 || result["status"] != "approved" {
					t.Fatal("explicit Kev approval not adopted", result, templates, runs)
				}
				if _, err := f.app.RepairWorldEvents(f.ctx, 20); err != nil {
					t.Fatal(err)
				}
				var applied bool
				if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status='applied' AND fluctlight_id IS NULL AND life_event_id IS NULL FROM public.world_event_runs WHERE id=$1`, result["run_id"]).Scan(&applied); err != nil || !applied {
					t.Fatal("independent world event required fake actor", applied, err)
				}
				*now = now.Add(time.Hour)
				if _, err := f.app.GenerateWorldEventCycle(f.ctx, world); err != nil {
					t.Fatal(err)
				}
				if models != 2 {
					t.Fatal("new hour did not generate", models)
				}
			} else if templates != 0 || runs != 0 {
				t.Fatal("event entered executable store without explicit Kev Yes", choice, result, templates, runs)
			}
			if (choice == "disabled" || choice == "no_event") && kevCalls != 0 {
				t.Fatal("disabled/no-event made a Kev HTTP call", kevCalls)
			}
		})
	}
}

func TestWorldEventInputKeepsParticipantsDistinct(t *testing.T) {
	view := npcEncounterPromptView(map[string]any{"actual_prior_interactions": []any{map[string]any{"summary": "与甲交谈", "participant_name": "甲", "is_current_character": false}, map[string]any{"summary": "与乙交谈", "participant_name": "乙", "is_current_character": true}}})
	history := arrayValue(view["actual_prior_interactions"])
	if len(history) != 2 || mapValue(history[0])["is_current_character"] != false || mapValue(history[1])["participant_name"] != "乙" {
		t.Fatal("shared NPC history lost interaction subject", view)
	}
	identity := scheduleGenerationIdentity(map[string]any{"occupation": "书店合伙人", "residence": "老城区公寓"})
	if identity["residence"] != "老城区公寓" {
		t.Fatal("schedule identity discarded declared residence")
	}
	if _, exists := scheduleGenerationIdentity(map[string]any{"occupation": "书店合伙人"})["residence"]; exists {
		t.Fatal("occupation manufactured a home")
	}
}
