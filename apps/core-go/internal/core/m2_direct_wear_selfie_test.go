package core

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// The database and formal application paths are real. Model, renderer, quality
// judgement and object storage are controlled; this is not a live image test.
func TestDefaultDirectWearsBeforeSelfieAndDeliversOnce(t *testing.T) {
	const boots = "M2 受控黑色短靴"
	const unworn = "M2 未穿红色长裤"
	const pendingReply = "我在拍啦，稍等一下。"
	var concept map[string]any
	var mainCalls, normalizations, qualityChecks int
	var submissions, uploads atomic.Int32
	var wearResultSeen, mediaResultSeen bool
	var checkWornBeforeMedia func()
	toolResult := func(payload map[string]any, id string) string {
		for _, raw := range arrayValue(payload["messages"]) {
			message := mapValue(raw)
			if message["role"] == "tool" && message["tool_call_id"] == id {
				return stringValue(message["content"])
			}
		}
		return ""
	}
	router := newFakeProviderRouter().
		on("media_prompt_normalized", func(payload map[string]any) fakeProviderResult {
			normalizations++
			if wire := jsonString(payload["messages"]); !strings.Contains(wire, boots) || strings.Contains(wire, unworn) {
				t.Errorf("normalizer did not receive the actual worn items: %s", wire)
			}
			output := mediaTestOutput(concept)
			output["prompt"] = "穿着" + boots + "拍一张全身自拍。"
			return fakeProviderResult{Structured: output}
		}).
		on("media_quality_acceptance_response", func(map[string]any) fakeProviderResult {
			qualityChecks++
			return fakeProviderResult{Structured: map[string]any{"schema_version": 1, "verdict": "pass", "violations": []any{}, "retry_guidance": "", "observed_facts": map[string]any{"subject_matches": true, "appearance_matches": true, "scene_matches": true, "capture_matches": true, "framing_matches": true}}}
		}).
		on(directMediaDeliveryCheckSchema, func(map[string]any) fakeProviderResult {
			return fakeProviderResult{Structured: map[string]any{"claims": []any{}}}
		}).
		otherwise(func(payload map[string]any) fakeProviderResult {
			mainCalls++
			invoke := func(id, name string, args map[string]any) fakeProviderResult {
				if !r6PayloadHasTool(payload, name) {
					t.Errorf("main request did not offer %s at step %d", name, mainCalls)
				}
				return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall(id, name, args)}}
			}
			switch mainCalls {
			case 1:
				return invoke("m2-load", capabilityCatalogName, map[string]any{"names": []any{wardrobeInspectCapabilityName, wardrobeWearCapabilityName, "media.image.generate"}})
			case 2:
				return invoke("r6-inspect", wardrobeInspectCapabilityName, map[string]any{"operation": "list"})
			case 3:
				id := r6InspectedItem(payload, boots)
				if id == "" {
					t.Error("real inspection receipt did not offer the requested item")
					return fakeProviderResult{Status: http.StatusBadRequest}
				}
				return invoke("m2-wear", wardrobeWearCapabilityName, map[string]any{"mode": "partial", "item_ids": []any{id}})
			case 4:
				wearResultSeen = strings.Contains(toolResult(payload, "m2-wear"), "completed")
				if !wearResultSeen {
					t.Errorf("main model did not receive the real wear receipt: %s", toolResult(payload, "m2-wear"))
				}
				checkWornBeforeMedia()
				plan := mediaTestPlan("person", "present", "yes", "full")
				plan["capture_method"], plan["viewpoint"], plan["framing"], plan["camera_angle"] = "handheld_selfie", "subject_facing", "full_body", "eye_level"
				return invoke("m2-selfie", "media.image.generate", map[string]any{"intent": "拍现在穿着的全身自拍", "media_plan": plan})
			case 5:
				mediaResultSeen = strings.Contains(toolResult(payload, "m2-selfie"), "accepted")
				if !mediaResultSeen {
					t.Errorf("main model did not receive the accepted media receipt: %s", toolResult(payload, "m2-selfie"))
				}
				decision := map[string]any{"action_type": "reply", "response_intent": "回应", "influences": []any{}}
				return invoke("m2-reply", conversationReplyCapabilityName, map[string]any{"text": pendingReply, "decision": decision})
			default:
				t.Errorf("unexpected main request %d", mainCalls)
				return fakeProviderResult{Status: http.StatusBadRequest}
			}
		})

	ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "m2-wear-selfie", router)
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_visual_identities(id,fluctlight_id,status,identity_snapshot) VALUES($1,$2,'active','{"identity":{"name":"Fixture identity"}}')`, "m2-wear-visual", fid); err != nil {
		t.Fatal(err)
	}
	enableControlledVision(t, ctx, repo)
	addItem := func(description, slot string) string {
		t.Helper()
		var revision int
		if err := repo.Pool().QueryRow(ctx, `SELECT revision FROM public.fluctlight_wardrobe_states WHERE fluctlight_id=$1`, fid).Scan(&revision); err != nil {
			t.Fatal(err)
		}
		result, err := app.DispatchOwnerSurface(ctx, "POST", "/internal/fluctlights/"+fid+"/inventory/items", nil, owner, map[string]any{"expected_revision": revision, "reason": "受控测试物品", "idempotency_key": "m2-item-" + slot, "item": map[string]any{"item_kind": "wearable", "category": "clothing", "slot": slot, "clothing_type": slot, "description": description, "seasons": []any{}, "ownership": "owned", "availability": "available"}})
		if err != nil {
			t.Fatal(err)
		}
		return stringValue(mapValue(result["item"])["id"])
	}
	bootsID := addItem(boots, "shoes")
	addItem(unworn, "lower_body")
	checkWornBeforeMedia = func() {
		worn, err := readCurrentWornItems(ctx, repo.Pool(), fid)
		if err != nil || !strings.Contains(jsonString(worn), bootsID) || strings.Contains(jsonString(worn), unworn) {
			t.Errorf("real wear was not persisted before the selfie request: %#v err=%v", worn, err)
		}
	}
	graph := map[string]any{"positive": map[string]any{"class_type": "CLIPTextEncode", "inputs": map[string]any{"text": "{{prompt}}"}}, "sampler": map[string]any{"class_type": "KSampler", "inputs": map[string]any{"positive": []any{"positive", 0}}}}
	image := ownerMediaPNG(t)
	renderer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/prompt":
			submissions.Add(1)
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatal(err)
			}
			if wire := jsonString(payload); !strings.Contains(wire, boots) || strings.Contains(wire, unworn) {
				t.Errorf("renderer did not receive actual worn items: %s", wire)
			}
			_, _ = io.WriteString(w, `{"prompt_id":"m2-controlled-selfie"}`)
		case "/history/m2-controlled-selfie":
			_, _ = io.WriteString(w, `{"m2-controlled-selfie":{"outputs":{"save":{"images":[{"filename":"fixture.png","type":"output"}]}}}}`)
		case "/view":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(image)
		default:
			t.Errorf("unexpected renderer path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer renderer.Close()
	rendererURL, err := url.Parse(renderer.URL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.runtime_settings(key,value_json) VALUES('media.comfyui',$1)`, jsonString(map[string]any{"baseUrl": renderer.URL, "workflow": graph})); err != nil {
		t.Fatal(err)
	}
	app.Provider.HTTP = &http.Client{Transport: projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host == rendererURL.Host {
			return http.DefaultTransport.RoundTrip(request)
		}
		return router.RoundTrip(request)
	})}
	storage := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("unexpected controlled storage method %s", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		uploads.Add(1)
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("ETag", `"controlled-etag"`)
	}))
	defer storage.Close()
	storageURL, err := url.Parse(storage.URL)
	if err != nil {
		t.Fatal(err)
	}
	app.Storage, err = minio.New(storageURL.Host, &minio.Options{Secure: false, Creds: credentials.NewStaticV4("controlled", "controlled", ""), Region: "us-east-1", BucketLookup: minio.BucketLookupPath})
	if err != nil {
		t.Fatal(err)
	}
	app.S3Bucket = "m2-selfie-fixture"
	payload := map[string]any{"fluctlight_id": fid, "text": "换上那双短靴，给我看一张现在的全身自拍。", "turn_id": "m2-wear-selfie", "idempotency_key": "m2-wear-selfie", "attachment_refs": []any{}}
	turn, err := app.HandleTurn(ctx, owner, conversation, payload)
	if err != nil || stringValue(turn.Assistant["text"]) != pendingReply || mainCalls != 5 || !wearResultSeen || !mediaResultSeen {
		t.Fatalf("direct wear/selfie failed: calls=%d wear=%t media=%t turn=%#v err=%v", mainCalls, wearResultSeen, mediaResultSeen, turn.Assistant, err)
	}
	worn, err := readCurrentWornItems(ctx, repo.Pool(), fid)
	if err != nil || !strings.Contains(jsonString(worn), bootsID) || strings.Contains(jsonString(worn), unworn) {
		t.Fatalf("persisted worn items wrong: %#v err=%v", worn, err)
	}
	var intentID, status string
	var raw []byte
	var attachments int
	if err := repo.Pool().QueryRow(ctx, `SELECT id,status,prompt FROM public.media_intents WHERE owner_fluctlight_id=$1`, fid).Scan(&intentID, &status, &raw); err != nil || status != "pending" {
		t.Fatalf("selfie intent status=%q err=%v", status, err)
	}
	concept = decodeObject(raw)
	if wire := jsonString(concept); !strings.Contains(wire, boots) || strings.Contains(wire, unworn) {
		t.Fatalf("selfie did not freeze the real worn items: %s", wire)
	}
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='media_reference'`, conversation).Scan(&attachments); err != nil || attachments != 0 {
		t.Fatalf("media was delivered before rendering: count=%d err=%v", attachments, err)
	}
	if _, err := app.HandleTurn(ctx, owner, conversation, payload); err != nil || mainCalls != 5 {
		t.Fatalf("turn replay repeated main request: calls=%d err=%v", mainCalls, err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		result, err := app.ProcessMediaIntent(ctx, intentID)
		if err != nil || result["status"] != "completed" {
			t.Fatalf("process/replay %d result=%#v err=%v", attempt, result, err)
		}
	}
	var references, assets int
	if err := repo.Pool().QueryRow(ctx, `SELECT (SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND idempotency_key=$2 AND attachment_refs ? $3), (SELECT count(*) FROM public.media_references WHERE asset_id=$3 AND target_type='conversation' AND target_id=$1), (SELECT count(*) FROM public.media_assets WHERE id=$3 AND status='ready')`, conversation, "media:"+intentID+":conversation-result", "asset_"+intentID).Scan(&attachments, &references, &assets); err != nil || attachments != 1 || references != 1 || assets != 1 || submissions.Load() != 1 || uploads.Load() != 1 || normalizations != 1 || qualityChecks != 1 {
		t.Fatalf("delivery effects attachments=%d references=%d assets=%d submit=%d upload=%d normalize=%d quality=%d err=%v", attachments, references, assets, submissions.Load(), uploads.Load(), normalizations, qualityChecks, err)
	}
}
