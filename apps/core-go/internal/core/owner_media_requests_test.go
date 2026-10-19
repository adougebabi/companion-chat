package core

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

type ownerMediaControlledPreflight struct {
	imageGenerateCapability
	seen *bool
}

func (c ownerMediaControlledPreflight) Preflight(_ context.Context, resolved CapabilityContext) error {
	*c.seen = resolved.Identity.FluctlightID == "actual-role"
	return errors.New("controlled missing media configuration")
}
func TestOwnerMediaWrapperForwardsTheRuntimePreflightContract(t *testing.T) {
	seen := false
	wrapper := ownerMediaCapability{ownerLifeCapability{Capability: ownerMediaControlledPreflight{seen: &seen}}}
	var preflight CapabilityPreflighter = wrapper
	err := preflight.Preflight(context.Background(), CapabilityContext{Identity: ContextIdentity{FluctlightID: "actual-role"}})
	if err == nil || !seen {
		t.Fatal("wrapper lost media Provider configuration preflight", err, seen)
	}
}

func TestOwnerMediaAcceptedReceiptIsAnImageIntentNotACompletedLifeMutation(t *testing.T) {
	result := CapabilityResult{CallID: "queued-call", CapabilityName: "media.image.generate", Status: "accepted", Output: map[string]any{"media_intent_id": "actual-intent", "status": "pending"}}
	effect, refs := ownerLifeEffects("item_photo", []CapabilityResult{result})
	if !effect || len(refs) < 2 {
		t.Fatal("durable async receipt discarded", effect, refs)
	}
	if effect, _ := ownerLifeEffects("wardrobe_wear", []CapabilityResult{result}); effect {
		t.Fatal("image intent became a clothing effect")
	}
	prose := result
	prose.Output = map[string]any{"text": "generated"}
	if effect, _ := ownerLifeEffects("media_generate", []CapabilityResult{prose}); effect {
		t.Fatal("prose became an image intent")
	}
	if authorizeOwnerMediaTargetTx(context.Background(), nil, MediaPublicationTarget{Kind: "owner_media_request", Ref: "untrusted"}) == nil {
		t.Fatal("unbound media target authorized")
	}
}

// Native cognition is a controlled Provider transport. Ready rows below are
// explicit settlement fixtures, not renderer or real object-storage evidence.
func TestOwnerMediaNativeRequestFreezesTargetRecoversAcceptedIntentAndPublishesItemReference(t *testing.T) {
	for _, scenario := range []string{"accepted", "final_failure", "declined", "wrong_intent"} {
		t.Run(scenario, func(t *testing.T) {
			ctx, repo, app, owner, fid, _ := setupDirectPublicationToolTest(t, "owner-photo-"+scenario)
			if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'controlled','test')`, owner); err != nil {
				t.Fatal(err)
			}
			var item string
			var revision int
			if err := repo.Pool().QueryRow(ctx, `SELECT id,revision FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 LIMIT 1`, fid).Scan(&item, &revision); err != nil {
				t.Fatal(err)
			}
			seedCognitiveProviderRole(t, ctx, repo, "owner-photo-endpoint-"+scenario)
			calls := 0
			render := ""
			app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
				calls++
				if scenario == "declined" {
					return fakeProviderResult{Structured: map[string]any{"decision": "rejected", "reason": "现在不想画"}}
				}
				if payloadHasToolResult(payload) {
					if scenario == "final_failure" {
						return fakeProviderResult{Err: errors.New("controlled final response failure")}
					}
					return fakeProviderResult{Structured: map[string]any{"decision": "accepted", "reason": "图片任务已排队，尚未完成"}}
				}
				intent := render
				if scenario == "wrong_intent" {
					intent = "unrelated image"
				}
				return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("owner-photo-tool", "media.image.generate", map[string]any{"intent": intent, "media_plan": mediaTestPlan("object", "none", "no", "unknown")})}}
			})}
			path := "/internal/fluctlights/" + fid + "/wardrobe/items/" + item + "/photos"
			envelope, err := app.DispatchOwnerSurface(ctx, "POST", path, nil, owner, map[string]any{"prompt": "只画这件物品，不出现人或人体", "expected_item_revision": revision, "reason": "仅提出图片请求", "idempotency_key": "owner-photo"})
			if err != nil {
				t.Fatal(err)
			}
			request := mapValue(envelope["request"])
			render = stringValue(mapValue(request["payload"])["_render_intent"])
			if calls != 0 || render == "" || mapValue(mapValue(request["payload"])["_item_snapshot"])["id"] != item {
				t.Fatal("API ran model or lost actual target", request, calls)
			}
			if _, err := app.ProcessCognitionInbox(ctx, stringValue(request["source_fact_id"])); err != nil {
				t.Fatal(err)
			}
			view, err := app.DispatchOwnerSurface(ctx, "GET", "/internal/fluctlights/"+fid+"/media-generations/"+stringValue(request["id"]), nil, owner, nil)
			if err != nil {
				t.Fatal(err)
			}
			accepted := scenario == "accepted" || scenario == "final_failure"
			if !accepted {
				if view["media"] != nil || mapValue(view["request"])["status"] == "accepted" {
					t.Fatal("unaccepted request generated a task", view)
				}
				return
			}
			if mapValue(view["request"])["status"] != "accepted" || view["asset"] != nil || mapValue(view["media"])["status"] != "pending" {
				t.Fatal("intent receipt claimed a ready image or was lost", view)
			}
			mid := stringValue(mapValue(view["media"])["id"])
			var count int
			if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.media_intents WHERE owner_request_id=$1`, request["id"]).Scan(&count); err != nil || count != 1 {
				t.Fatal("request lacks exactly one intent", count, err)
			}
			priorCalls := calls
			if _, err := app.ProcessCognitionInbox(ctx, stringValue(request["source_fact_id"])); err != nil || calls != priorCalls {
				t.Fatal("accepted intent recovery restarted Agent", calls, err)
			}
			intent, err := app.readMediaIntent(ctx, mid)
			if err != nil || intent.OwnerRequestID != request["id"] || intent.OwnerItemID != item {
				t.Fatal("actual intent target not persisted", intent, err)
			}
			asset := "asset_" + mid
			if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.media_assets(id,owner_fluctlight_id,version,kind,mime_type,byte_size,sha256,bucket,object_key,provider_request_id,workflow_id,status) VALUES($1,$2,'1','image','image/png',68,'controlled-ready-digest','controlled','not-read',$3,$4,'ready')`, asset, fid, intent.ProviderRequestID, intent.WorkflowID); err != nil {
				t.Fatal(err)
			}
			if err := app.publishMediaAsset(ctx, intent, asset); err != nil {
				t.Fatal(err)
			}
			if err := app.markMediaIntentCompleted(ctx, mid, asset); err != nil {
				t.Fatal(err)
			}
			actual, err := app.OwnerInventoryItem(ctx, owner, fid, item)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, value := range arrayValue(actual["media"]) {
				if mapValue(value)["asset_id"] == asset {
					found = true
				}
			}
			if !found {
				t.Fatal("actual item consumer did not receive ready image reference", actual)
			}
			if intValue(actual["revision"]) != revision {
				t.Fatal("image changed inventory facts", actual)
			}
			var messages int
			if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE author_actor_id=$1`, fid).Scan(&messages); err != nil || messages != 0 {
				t.Fatal("standalone image fabricated chat publication", messages, err)
			}
		})
	}
}
