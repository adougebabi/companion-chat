package core

import (
	"errors"
	"net/http"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestOwnerLifeRequestWearNativeConsumptionAndRecovery(t *testing.T) {
	for _, scenario := range []string{"success", "reject", "provider_failure", "final_failure", "stale", "target_mismatch"} {
		t.Run(scenario, func(t *testing.T) {
			f := seedWardrobeToolFixture(t)
			boots := wardrobeItemID(t, f, "黑色短靴")
			revision := wardrobeRevision(t, f)
			calls := 0
			router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
				calls++
				for _, raw := range arrayValue(payload["tools"]) {
					name := stringValue(mapValue(mapValue(raw)["function"])["name"])
					if name == "conversation.reply" || name == "moment.publish" || name == "persona.switch" {
						t.Errorf("unrelated capability installed: %s", name)
					}
				}
				if scenario == "reject" {
					return fakeProviderResult{Structured: map[string]any{"decision": "rejected", "reason": "今天不想换靴子"}}
				}
				if scenario == "provider_failure" || (scenario == "final_failure" && payloadHasToolResult(payload)) {
					return fakeProviderResult{Err: errors.New("controlled private provider failure")}
				}
				if payloadHasToolResult(payload) {
					return fakeProviderResult{Structured: map[string]any{"decision": "accepted", "reason": "已按请求穿上靴子"}}
				}
				target := boots
				if scenario == "target_mismatch" {
					target = wardrobeItemID(t, f, "银项链")
				}
				return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("owner-wear", wardrobeWearCapabilityName, map[string]any{"mode": "partial", "item_ids": []string{target}})}}
			})
			f.app.Provider.HTTP = &http.Client{Transport: router}
			seedCognitiveProviderRole(t, f.ctx, f.repository, "owner-life-endpoint-"+f.suffix)
			body := map[string]any{"item_id": boots, "slot": "shoes", "expected_revision": revision, "reason": "请穿上这双靴子", "idempotency_key": "owner-wear"}
			path := "/internal/fluctlights/" + f.fluctlightID + "/wardrobe/wear"
			admitted, err := f.app.DispatchOwnerSurface(f.ctx, "POST", path, nil, f.ownerID, body)
			if err != nil {
				t.Fatal(err)
			}
			request := mapValue(admitted["request"])
			id, inbox := stringValue(request["id"]), stringValue(request["source_fact_id"])
			if id == "" || inbox == "" || request["status"] != "pending" {
				t.Fatalf("no durable request: %#v", admitted)
			}
			if calls != 0 {
				t.Fatal("API called Provider")
			}
			var intents int
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.platform_workflow_intents WHERE intent_id=$1 AND intent_type='cognition.processing'`, "cognition_intent:"+inbox).Scan(&intents); err != nil || intents != 1 {
				t.Fatal("no cognition intent", intents, err)
			}
			replay, err := f.app.DispatchOwnerSurface(f.ctx, "POST", path, nil, f.ownerID, body)
			if err != nil || mapValue(replay["request"])["id"] != id {
				t.Fatal("admission replay", replay, err)
			}
			if scenario == "stale" {
				if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlight_wardrobe_states SET revision=revision+1 WHERE fluctlight_id=$1`, f.fluctlightID); err != nil {
					t.Fatal(err)
				}
			}
			result, err := f.app.ProcessCognitionInbox(f.ctx, inbox)
			if err != nil {
				t.Fatalf("worker: %#v %v", result, err)
			}
			read, err := f.app.DispatchOwnerSurface(f.ctx, "GET", "/internal/owner-requests/"+id, nil, f.ownerID, nil)
			if err != nil {
				t.Fatal(err)
			}
			status := stringValue(read["status"])
			if scenario == "success" || scenario == "final_failure" {
				if status != "completed" || mapValue(read["result"])["mutation_committed"] != true {
					t.Fatalf("committed effect lost: %#v; calls=%d", read, calls)
				}
				if wardrobeRevision(t, f) != revision+1 {
					t.Fatal("effect count")
				}
			} else {
				if status == "completed" {
					t.Fatalf("false completion: %#v", read)
				}
				want := revision
				if scenario == "stale" {
					want++
				}
				if wardrobeRevision(t, f) != want {
					t.Fatal("failed request wrote effect")
				}
			}
			if scenario == "success" && calls < 2 {
				t.Fatal("native receipt was not consumed")
			}
			count := calls
			if _, err := f.app.ProcessCognitionInbox(f.ctx, inbox); err != nil {
				t.Fatal(err)
			}
			if calls != count {
				t.Fatal("terminal replay restarted model")
			}
			if _, err := f.app.DispatchOwnerSurface(f.ctx, "GET", "/internal/owner-requests/"+id, nil, f.foreignOwnerID, nil); err == nil {
				t.Fatal("cross-owner request exposed")
			}
		})
	}
}

func TestOwnerLifeRequestRouteIdentityAndRollback(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	added, err := f.app.AddWardrobeItems(f.ctx, f.ownerID, f.fluctlightID, map[string]any{"item_kind": "object", "category": "art_supply", "description": "画笔", "ownership": "owned", "availability": "available"})
	if err != nil {
		t.Fatal(err)
	}
	brush := stringValue(mapValue(arrayValue(added["items"])[0])["id"])
	admitted, err := f.app.DispatchOwnerSurface(f.ctx, "POST", "/internal/fluctlights/"+f.fluctlightID+"/wardrobe/items/"+brush+"/use", nil, f.ownerID, map[string]any{"active": true, "reason": "用这支画笔绘画", "idempotency_key": "use-request"})
	if err != nil {
		t.Fatal(err)
	}
	request := mapValue(admitted["request"])
	payload := mapValue(request["payload"])
	if payload["item_id"] != brush || payload["expected_revision"] == nil {
		t.Fatal("URL identity or submitted CAS lost", request)
	}
	var before, after int
	f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.owner_surface_records WHERE kind='owner_request'`).Scan(&before)
	_, err = f.app.DispatchOwnerSurface(f.ctx, "POST", "/internal/fluctlights/"+f.fluctlightID+"/wardrobe/items/missing/use", nil, f.ownerID, map[string]any{"active": true, "reason": "missing", "idempotency_key": "missing-request"})
	if err == nil {
		t.Fatal("missing item accepted")
	}
	f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.owner_surface_records WHERE kind='owner_request'`).Scan(&after)
	if after != before {
		t.Fatal("rejected admission orphaned pending request")
	}
}

func TestOwnerScheduleRequestPlanRequiresExactRequestedFields(t *testing.T) {
	req := map[string]any{"operation": "create", "start_at": "2026-10-16T10:00:00+08:00", "end_at": "2026-10-16T11:00:00+08:00", "activity": "阅读", "location": "书房"}
	item := cloneMap(req)
	if err := validateOwnerScheduleRequestPlan(req, map[string]any{"items": []any{item}}); err != nil {
		t.Fatal(err)
	}
	item["activity"] = "购物"
	if err := validateOwnerScheduleRequestPlan(req, map[string]any{"items": []any{item}}); err == nil {
		t.Fatal("planner changed requested activity")
	}
}

func TestOwnerLifeCapabilityScopeDoesNotBroadenCatalog(t *testing.T) {
	app := &App{}
	registry, err := app.ownerLifeRegistry("owner", "fid", "wardrobe_wear", map[string]any{"item_id": "boots"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := registry.Lookup("wardrobe.wear"); !ok {
		t.Fatal("requested writer missing")
	}
	for _, name := range []string{"item.use", "life.activity.start", "persona.switch", "conversation.reply", "moment.publish"} {
		if _, ok := registry.Lookup(name); ok {
			t.Fatal("unrelated writer available", name)
		}
	}
}

// Savepoint rollback must retain neither a rejected target effect nor another
// request row. This test also proves a changed payload cannot reuse a command.
func TestOwnerLifeRequestAdmissionCAS(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	boots := wardrobeItemID(t, f, "黑色短靴")
	body := map[string]any{"item_id": boots, "slot": "shoes", "expected_revision": wardrobeRevision(t, f) + 1, "reason": "请穿靴子", "idempotency_key": "stale-admission"}
	_, err := f.app.DispatchOwnerSurface(f.ctx, "POST", "/internal/fluctlights/"+f.fluctlightID+"/wardrobe/wear", nil, f.ownerID, body)
	var conflict *SurfaceError
	if !errors.As(err, &conflict) || conflict.Code != "revision_conflict" {
		t.Fatal("stale admitted", err)
	}
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		var count int
		if err := tx.QueryRow(f.ctx, `SELECT count(*) FROM public.cognition_inbox WHERE event_type=$1`, ownerLifeRequestFactType).Scan(&count); err != nil {
			return err
		}
		if count != 0 {
			t.Fatal("stale request enqueued")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
