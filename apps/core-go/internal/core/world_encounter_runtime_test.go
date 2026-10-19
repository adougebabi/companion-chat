package core

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestNPCNativeConsumerAndIndependentResponseUseSeparateInputs(t *testing.T) {
	for _, scenario := range []string{"accept", "decline", "cancel_during_response", "provider_failure"} {
		t.Run(scenario, func(t *testing.T) {
			f, npc, _, now := seedNPCEncounterRuntime(t, true)
			if _, err := f.app.RepairWorldNPCPresence(f.ctx, 20); err != nil {
				t.Fatal(err)
			}
			page, err := f.app.WorldNPCRecords(f.ctx, f.ownerID, f.fluctlightID, npc, "opportunities", url.Values{})
			if err != nil || len(arrayValue(page["items"])) != 1 {
				t.Fatal(page, err)
			}
			p := mapValue(arrayValue(page["items"])[0])
			consumeNPCOpportunityPredecessors(t, f, stringValue(p["source_fact_id"]))
			*now = now.Add(20 * time.Second)
			nativeCalls, responseCalls := 0, 0
			router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
				name := stringValue(mapValue(mapValue(payload["response_format"])["json_schema"])["name"])
				if name == "npc_response_v1" {
					responseCalls++
					if !strings.Contains(jsonString(payload), "PRIVATE_NPC_BIO_NOT_AUTOMATIC_KNOWLEDGE") {
						t.Error("NPC resolver did not consume its actual definition")
					}
					if scenario == "provider_failure" {
						return fakeProviderResult{Err: errors.New("controlled NPC response outage")}
					}
					if scenario == "cancel_during_response" {
						_, life, err := f.app.readLifeContextSnapshotAt(f.ctx, f.fluctlightID, *now)
						if err != nil {
							t.Fatal(err)
						}
						if _, err := f.app.CancelLifeEvent(f.ctx, f.ownerID, f.fluctlightID, stringValue(p["source_event_id"]), map[string]any{"expected_event_revision": p["source_event_revision"], "expected_life_context_revision": life["context_revision"], "idempotency_key": "cancel-while-model-running"}); err != nil {
							t.Fatal(err)
						}
					}
					decision := "accept"
					if scenario == "decline" {
						decision = "decline"
					}
					return fakeProviderResult{Structured: map[string]any{"decision": decision, "reason": "Current public situational response"}}
				}
				nativeCalls++
				if strings.Contains(jsonString(payload), "PRIVATE_NPC_BIO_NOT_AUTOMATIC_KNOWLEDGE") {
					t.Error("NPC private definition leaked into character cognition")
				}
				if payloadHasToolResult(payload) {
					return npcNoEffectNativeFinal()
				}
				return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("npc-character-approach", npcEncounterCapabilityName, map[string]any{"opportunity_id": p["id"], "expected_revision": p["revision"], "decision": "contact", "reason": "愿意打个招呼，允许对方拒绝"})}}
			})
			f.app.Provider.HTTP = &http.Client{Transport: router}
			result, err := f.app.ProcessCognitionInbox(f.ctx, stringValue(p["source_fact_id"]))
			if err != nil {
				t.Fatal(result, err)
			}
			stored, err := f.app.WorldNPCOpportunity(f.ctx, f.ownerID, f.fluctlightID, npc, stringValue(p["id"]))
			if err != nil {
				t.Fatal(err)
			}
			want := map[string]string{"accept": "contacted", "decline": "declined", "cancel_during_response": "expired", "provider_failure": "failed"}[scenario]
			if stored["status"] != want || nativeCalls < 1 || responseCalls < 1 || (scenario != "provider_failure" && responseCalls != 1) {
				t.Fatal("independent production/settlement broken", stored, nativeCalls, responseCalls)
			}
			calls := nativeCalls + responseCalls
			if _, err := f.app.ProcessCognitionInbox(f.ctx, stringValue(p["source_fact_id"])); err != nil || nativeCalls+responseCalls != calls {
				t.Fatal("terminal retry restarted models", err)
			}
		})
	}
}

func seedNPCEncounterRuntime(t *testing.T, confirmed bool) (independentToolE2EFixture, string, string, *time.Time) {
	t.Helper()
	f := seedWardrobeToolFixture(t)
	// Controlled virtual noon avoids midnight/quiet-hour dependence. This is
	// fixture time, not a claim that a real model or wall-clock journey ran.
	now := time.Date(2026, 10, 19, 6, 0, 0, 0, time.UTC)
	f.app.Clock = func() time.Time { return now }
	base := "/internal/fluctlights/" + f.fluctlightID
	call := func(method, path string, body map[string]any) map[string]any {
		t.Helper()
		if body != nil {
			body["reason"] = "NPC runtime fixture"
			body["idempotency_key"] = randomID("npc_runtime_test_")
		}
		result, err := f.app.DispatchWorldSurface(f.ctx, method, path, nil, f.ownerID, body)
		if err != nil {
			t.Fatal(path, err)
		}
		return result
	}
	call("PUT", base+"/world", map[string]any{"name": "NPC-RUNTIME-TEST", "expected_revision": 0})
	place := call("POST", base+"/world/places", map[string]any{"name": "NPC runtime room", "description": "Explicit scoped setting"})
	resource, err := f.app.DispatchOwnerSurface(f.ctx, "POST", "/internal/studio/resources", nil, f.ownerID, map[string]any{"kind": "npc", "name": "Runtime keeper", "body": map[string]any{"role": "keeper", "background": "PRIVATE_NPC_BIO_NOT_AUTOMATIC_KNOWLEDGE"}, "reason": "test NPC definition", "idempotency_key": "npc-runtime-definition"})
	if err != nil {
		t.Fatal(err)
	}
	rid := stringValue(resource["id"])
	published := call("POST", "/internal/studio/resources/"+rid+"/publish", map[string]any{"expected_revision": resource["revision"]})
	fl, err := f.app.DB.GetFluctlight(f.ctx, f.fluctlightID, f.ownerID)
	if err != nil {
		t.Fatal(err)
	}
	applied := call("POST", "/internal/studio/resources/"+rid+"/apply", map[string]any{"fluctlight_id": f.fluctlightID, "resource_version": published["published_version"], "expected_instance_revision": fl.CurrentRevision, "expected_runtime_revision": 0, "home_place_id": place["id"], "choices": []any{}})
	if applied["applied"] != true {
		t.Fatal(applied)
	}
	if confirmed {
		_, life, err := f.app.readLifeContextSnapshotAt(f.ctx, f.fluctlightID, now)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.app.CreateLifeEvent(f.ctx, f.ownerID, f.fluctlightID, map[string]any{"kind": "visit", "start_at": now.Add(-time.Minute).Format(time.RFC3339Nano), "end_at": now.Add(time.Hour).Format(time.RFC3339Nano), "scene": "visit", "activity": "Explicit test arrival", "location": "NPC runtime room", "place_id": place["id"], "evidence_refs": []any{f.conversationID}, "expected_life_context_revision": life["context_revision"], "idempotency_key": "npc-runtime-arrival"}); err != nil {
			t.Fatal(err)
		}
	}
	return f, stringValue(applied["id"]), stringValue(place["id"]), &now
}

func TestNPCExecutionIsSeparateFromPredictionAndBrowsing(t *testing.T) {
	f, npc, _, now := seedNPCEncounterRuntime(t, false)
	for i := 0; i < 3; i++ {
		if _, err := f.app.WorldSnapshot(f.ctx, f.ownerID, f.fluctlightID); err != nil {
			t.Fatal(err)
		}
		if _, err := f.app.worldNearby(f.ctx, f.ownerID, f.fluctlightID); err != nil {
			t.Fatal(err)
		}
	}
	var presence, opportunity, relations int
	count := func() {
		t.Helper()
		if err := f.repository.Pool().QueryRow(f.ctx, `SELECT (SELECT count(*) FROM public.world_npc_presence_runs WHERE npc_id=$1),(SELECT count(*) FROM public.world_npc_opportunities WHERE npc_id=$1),(SELECT count(*) FROM public.relationships WHERE owner_fluctlight_id=$2)`, npc, f.fluctlightID).Scan(&presence, &opportunity, &relations); err != nil {
			t.Fatal(err)
		}
	}
	count()
	if presence != 0 || opportunity != 0 || relations != 0 {
		t.Fatal("GET produced state", presence, opportunity, relations)
	}
	if _, err := f.app.RepairWorldNPCPresence(f.ctx, 20); err != nil {
		t.Fatal(err)
	}
	count()
	if presence != 1 || opportunity != 0 || relations != 0 {
		t.Fatal("presence was treated as acquaintance", presence, opportunity, relations)
	}
	page, err := f.app.WorldNPCRecords(f.ctx, f.ownerID, f.fluctlightID, npc, "presences", url.Values{})
	if err != nil || len(arrayValue(page["items"])) != 1 {
		t.Fatal(page, err)
	}
	row := mapValue(arrayValue(page["items"])[0])
	start, err := time.Parse(time.RFC3339Nano, stringValue(row["start_at"]))
	if err != nil || !start.Equal(*now) {
		t.Fatal("offline history backfilled", row, err)
	}
	if _, err := f.app.RepairWorldNPCPresence(f.ctx, 20); err != nil {
		t.Fatal(err)
	}
	count()
	if presence != 1 {
		t.Fatal("presence replay duplicated", presence)
	}
	f.requireNoModelRuns(t)
}

func TestNPCIndependentToolOutcomesRecoverWithoutRepeatingResponseModel(t *testing.T) {
	for _, outcome := range []string{"accept", "decline", "pass", "cancelled_source"} {
		t.Run(outcome, func(t *testing.T) {
			f, npc, _, now := seedNPCEncounterRuntime(t, true)
			if _, err := f.app.RepairWorldNPCPresence(f.ctx, 20); err != nil {
				t.Fatal(err)
			}
			page, err := f.app.WorldNPCRecords(f.ctx, f.ownerID, f.fluctlightID, npc, "opportunities", url.Values{})
			if err != nil || len(arrayValue(page["items"])) != 1 {
				t.Fatal("no durable opportunity", page, err)
			}
			p := mapValue(arrayValue(page["items"])[0])
			*now = now.Add(15 * time.Second)
			if outcome == "cancelled_source" {
				_, life, err := f.app.readLifeContextSnapshotAt(f.ctx, f.fluctlightID, *now)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := f.app.CancelLifeEvent(f.ctx, f.ownerID, f.fluctlightID, stringValue(p["source_event_id"]), map[string]any{"expected_event_revision": p["source_event_revision"], "expected_life_context_revision": life["context_revision"], "idempotency_key": "cancel-arrival"}); err != nil {
					t.Fatal(err)
				}
			}
			decision, response := "contact", outcome
			if outcome == "pass" {
				decision, response = "pass", "not_applicable"
			}
			if outcome == "cancelled_source" {
				response = "accept"
			}
			responseCalls := 0
			router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
				responseCalls++
				return fakeProviderResult{Structured: map[string]any{"decision": response, "reason": "Controlled NPC response, without importing private biography"}}
			})
			f.app.Provider.HTTP = &http.Client{Transport: router}
			seedCognitiveProviderRole(t, f.ctx, f.repository, "npc-response-test-"+f.suffix)
			request := f.request(npcEncounterCapabilityName, "npc-encounter-"+outcome, map[string]any{"opportunity_id": p["id"], "expected_revision": p["revision"], "decision": decision, "reason": "Controlled independent virtual-world Tool outcome"})
			request.EvidenceID = stringValue(p["source_fact_id"])
			receipt, err := f.app.ExecuteTool(f.ctx, request)
			if outcome == "cancelled_source" {
				requireHandledToolNoEffect(t, receipt, err, "npc_opportunity_expired")
			} else if err != nil || receipt.Result.Status != "completed" {
				t.Fatal(receipt, err)
			}
			replay, err := f.app.ExecuteTool(f.ctx, request)
			if err != nil || !replay.Replayed {
				t.Fatal("replayed encounter missing", replay, err)
			}
			var encounters, relationships, memories int
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT (SELECT count(*) FROM public.world_encounters WHERE npc_id=$1),(SELECT count(*) FROM public.relationships WHERE owner_fluctlight_id=$2),(SELECT count(*) FROM public.memories WHERE owner_fluctlight_id=$2)`, npc, f.fluctlightID).Scan(&encounters, &relationships, &memories); err != nil {
				t.Fatal(err)
			}
			wantEnc, wantRel := 0, 0
			if outcome == "accept" {
				wantEnc, wantRel = 1, 1
			}
			if outcome == "decline" {
				wantEnc = 1
			}
			if encounters != wantEnc || relationships != wantRel || memories != wantEnc {
				t.Fatal("outcomes collapsed or duplicated", outcome, encounters, relationships, memories)
			}
			if wantEnc == 1 {
				var source string
				var refs []byte
				if err := f.repository.Pool().QueryRow(f.ctx, `SELECT source_kind,evidence_refs FROM public.world_encounters WHERE npc_id=$1`, npc).Scan(&source, &refs); err != nil || source != "virtual_world_encounter" || !strings.Contains(string(refs), stringValue(p["npc_presence_id"])) {
					t.Fatal("automatic source masqueraded as Owner confirmation", source, string(refs), err)
				}
				var leaked bool
				if err := f.repository.Pool().QueryRow(f.ctx, `SELECT EXISTS(SELECT 1 FROM public.memories WHERE owner_fluctlight_id=$1 AND (actor_refs ? $2 OR content LIKE '%PRIVATE_NPC_BIO_NOT_AUTOMATIC_KNOWLEDGE%'))`, f.fluctlightID, f.ownerID).Scan(&leaked); err != nil || leaked {
					t.Fatal("Owner co-presence or private biography fabricated", leaked, err)
				}
			}
			if wantEnc == 0 {
				f.requireNoModelRuns(t)
			}
			consumeNPCOpportunityPredecessors(t, f, stringValue(p["source_fact_id"]))
			if _, err := f.app.ProcessCognitionInbox(f.ctx, stringValue(p["source_fact_id"])); err != nil {
				t.Fatal(err)
			}
			if _, err := f.app.WorldNPCOpportunity(f.ctx, f.foreignOwnerID, f.fluctlightID, npc, stringValue(p["id"])); err == nil {
				t.Fatal("foreign scope read")
			}
			if responseCalls != wantEnc {
				t.Fatal("NPC response was repeated or missing", responseCalls, wantEnc)
			}
		})
	}
}

// The Owner arrival creates a real earlier native fact. Consume it through the
// normal ordered inbox before the later NPC opportunity, never by deleting or
// marking fixture rows processed. Keep this no-effect model separate from the
// NPC's private response provider.
func consumeNPCOpportunityPredecessors(t *testing.T, f independentToolE2EFixture, target string) {
	t.Helper()
	rows, err := f.repository.Pool().Query(f.ctx, `SELECT id FROM public.cognition_inbox WHERE fluctlight_id=$1 AND sequence<(SELECT sequence FROM public.cognition_inbox WHERE id=$2) AND status='pending' ORDER BY sequence`, f.fluctlightID, target)
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(ids) == 0 {
		return
	}
	previous := f.app.Provider.HTTP
	f.app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().on("native_cognition_response", func(map[string]any) fakeProviderResult {
		return npcNoEffectNativeFinal()
	})}
	defer func() { f.app.Provider.HTTP = previous }()
	var configured bool
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT EXISTS(SELECT 1 FROM public.model_roles WHERE role='cognitive_assessment')`).Scan(&configured); err != nil {
		t.Fatal(err)
	}
	if !configured {
		seedCognitiveProviderRole(t, f.ctx, f.repository, "npc-predecessor-"+f.suffix)
	}
	for _, id := range ids {
		if result, err := f.app.ProcessCognitionInbox(f.ctx, id); err != nil {
			t.Fatal("real predecessor consumption", id, result, err)
		}
	}
}

func npcNoEffectNativeFinal() fakeProviderResult {
	return fakeProviderResult{Structured: map[string]any{
		"attention": "已读取实际事件", "thought": "按实际结果结算", "desire": "暂不增加操作", "agency": "保持当前状态",
		"appraisal":  map[string]any{"relevance": 0.5, "goal_congruence": 0.5, "reward": 0.0, "loss": 0.0, "social_threat": 0.0, "controllability": 0.5, "responsibility": 0.5, "relationship_significance": 0.0, "expected_effect": 0.0, "evidence_refs": []any{}, "event_kind": "life_event", "direction": "mixed", "drive_signals": []any{}},
		"influences": []any{},
	}}
}

func TestNPCRoutineWindowOvernightAndHomeBoundary(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	npc := map[string]any{"home_place_id": "home", "position_mode": "routine", "routine": []any{map[string]any{"day": "weekday", "start": "22:00", "end": "06:00", "place_id": "shop", "activity": "Night shift"}}}
	at := time.Date(2026, 10, 17, 2, 0, 0, 0, loc)
	plan := npcPlannedPosition(npc, at)
	end, err := time.Parse(time.RFC3339Nano, stringValue(plan["window_end"]))
	if err != nil || plan["source"] != "schedule_prediction" || plan["place_id"] != "shop" || !end.Equal(time.Date(2026, 10, 17, 6, 0, 0, 0, loc)) {
		t.Fatal(plan, err)
	}
	at = time.Date(2026, 10, 19, 21, 0, 0, 0, loc)
	plan = npcPlannedPosition(npc, at)
	end, err = time.Parse(time.RFC3339Nano, stringValue(plan["window_end"]))
	if err != nil || plan["place_id"] != "home" || !end.Equal(at.Add(time.Hour)) {
		t.Fatal("home plan overran next execution boundary", plan, err)
	}
}

func TestNPCOpportunityConsumesActualActivityAndRejectsCancelledAuthority(t *testing.T) {
	f, npc, place, now := seedNPCEncounterRuntime(t, false)
	started, err := f.app.ExecuteTool(f.ctx, f.request(lifeActivityStartCapabilityName, "actual-presence-activity", map[string]any{"kind": "haircut", "desired_hair_length": "short", "duration_minutes": 30, "place_id": place, "scene": "hair_salon", "activity": "实际进行理发", "location": "NPC runtime room", "reason": "Explicit actual virtual activity, outcome still pending"}))
	if err != nil || started.Result.Status != "accepted" {
		t.Fatal(started, err)
	}
	if _, err := f.app.RepairWorldNPCPresence(f.ctx, 20); err != nil {
		t.Fatal(err)
	}
	page, err := f.app.WorldNPCRecords(f.ctx, f.ownerID, f.fluctlightID, npc, "opportunities", url.Values{})
	if err != nil || len(arrayValue(page["items"])) != 1 {
		t.Fatal(page, err)
	}
	p := mapValue(arrayValue(page["items"])[0])
	if p["character_presence_source"] != "active_activity" || p["source_activity_id"] != mapValue(started.Result.Output)["activity_id"] {
		t.Fatal("actual activity not the presence authority", p)
	}
	*now = now.Add(time.Minute)
	_, life, err := f.app.readLifeContextSnapshotAt(f.ctx, f.fluctlightID, *now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.CancelLifeEvent(f.ctx, f.ownerID, f.fluctlightID, stringValue(p["source_event_id"]), map[string]any{"expected_event_revision": p["source_event_revision"], "expected_life_context_revision": life["context_revision"], "idempotency_key": "end-activity-presence"}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.RepairNPCOpportunities(f.ctx, 20); err != nil {
		t.Fatal(err)
	}
	read, err := f.app.WorldNPCOpportunity(f.ctx, f.ownerID, f.fluctlightID, npc, stringValue(p["id"]))
	if err != nil || read["status"] != "expired" {
		t.Fatal("cancelled activity still enabled encounter", read, err)
	}
	f.requireNoModelRuns(t)
}

func TestNPCTickFailureBackoffDoesNotStarveAnotherCharacter(t *testing.T) {
	f, badNPC, _, now := seedNPCEncounterRuntime(t, true)
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'test-only-hash','npc-test') ON CONFLICT DO NOTHING`, f.ownerID); err != nil {
		t.Fatal(err)
	}
	other, err := f.app.CreateFluctlight(f.ctx, f.ownerID, randomID("npc_runtime_other_"), "Healthy runtime fixture", "blank_slate", "", nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Prepare a second initial active fixture. Activation/model behavior is not
	// the subject of this recovery test; no completed business result is seeded.
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlights SET status='active' WHERE id=$1`, other.ID); err != nil {
		t.Fatal(err)
	}
	call := func(path string, body map[string]any) map[string]any {
		t.Helper()
		body["reason"] = "NPC failure isolation fixture"
		body["idempotency_key"] = randomID("npc_recovery_")
		method := "POST"
		if strings.HasSuffix(path, "/world") {
			method = "PUT"
		}
		result, err := f.app.DispatchWorldSurface(f.ctx, method, path, nil, f.ownerID, body)
		if err != nil {
			t.Fatal(path, err)
		}
		return result
	}
	base := "/internal/fluctlights/" + other.ID
	world := call(base+"/world", map[string]any{"name": "Healthy world", "expected_revision": 0})
	place := call(base+"/world/places", map[string]any{"name": "Healthy place"})
	var resource, version string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT source_resource_id,source_version FROM public.world_npcs WHERE id=$1`, badNPC).Scan(&resource, &version); err != nil {
		t.Fatal(err)
	}
	current, err := f.app.DB.GetFluctlight(f.ctx, other.ID, f.ownerID)
	if err != nil {
		t.Fatal(err)
	}
	applied := call("/internal/studio/resources/"+resource+"/apply", map[string]any{"fluctlight_id": other.ID, "resource_version": version, "expected_instance_revision": current.CurrentRevision, "expected_runtime_revision": 0, "home_place_id": place["id"], "choices": []any{}})
	healthyNPC := stringValue(applied["id"])
	var badWorld string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT world_id FROM public.world_npcs WHERE id=$1`, badNPC).Scan(&badWorld); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.world_npc_runtime_clocks(npc_id,world_id,last_tick_at) VALUES($1,$2,$5),($3,$4,$6)`, badNPC, badWorld, healthyNPC, world["id"], now.Add(-2*time.Hour), now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	// Fault injection in the disposable fixture, not a recovery strategy: this
	// target cannot resolve its current profile while producing an opportunity.
	if _, err := f.repository.Pool().Exec(f.ctx, `DELETE FROM public.fluctlight_personality_runtime WHERE fluctlight_id=$1`, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	count, err := f.app.RepairWorldNPCPresence(f.ctx, 20)
	if err == nil || count != 1 {
		t.Fatal("failure hidden or healthy target starved", count, err)
	}
	var healthy, failures int
	var retry *time.Time
	var code *string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT (SELECT count(*) FROM public.world_npc_presence_runs WHERE npc_id=$1),failure_count,next_attempt_at,error_code FROM public.world_npc_runtime_clocks WHERE npc_id=$2`, healthyNPC, badNPC).Scan(&healthy, &failures, &retry, &code); err != nil || healthy != 1 || failures != 1 || retry == nil || !retry.After(*now) || code == nil {
		t.Fatal("failure backoff not durable", healthy, failures, retry, code, err)
	}
	if count, err := f.app.RepairWorldNPCPresence(f.ctx, 1); err != nil || count != 1 {
		t.Fatal("broken target monopolized next batch", count, err)
	}
	f.requireNoModelRuns(t)
}
