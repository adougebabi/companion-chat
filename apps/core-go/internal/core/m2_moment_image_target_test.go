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

// The application uses a disposable PostgreSQL database and real native tool
// execution. Model, renderer, quality review and storage are controlled.
func TestDirectMomentImageTargetsPublishedMoment(t *testing.T) {
	var mainCalls, momentCalls, normalizations, reviews int
	var submitted, uploaded atomic.Int32
	var mediaAccepted, offeredDestination bool
	var momentReceipt string
	var concept map[string]any
	router := newFakeProviderRouter().
		on("media_prompt_normalized", func(map[string]any) fakeProviderResult {
			normalizations++
			return fakeProviderResult{Structured: mediaTestOutput(concept)}
		}).
		on("media_quality_acceptance_response", func(map[string]any) fakeProviderResult {
			reviews++
			return fakeProviderResult{Structured: map[string]any{"schema_version": 1, "verdict": "pass", "violations": []any{}, "retry_guidance": "", "observed_facts": map[string]any{"subject_matches": true, "appearance_matches": true, "scene_matches": true, "capture_matches": true, "framing_matches": true}}}
		}).
		on(directMediaDeliveryCheckSchema, func(map[string]any) fakeProviderResult {
			return fakeProviderResult{Structured: map[string]any{"claims": []any{}}}
		}).
		otherwise(func(payload map[string]any) fakeProviderResult {
			if payload["model"] == "m2-moment-model" {
				momentCalls++
				return fakeProviderResult{Text: "今天窗边的晚霞很好看。"}
			}
			mainCalls++
			invoke := func(id, name string, args map[string]any) fakeProviderResult {
				if !r6PayloadHasTool(payload, name) {
					t.Errorf("main request did not offer %s at step %d", name, mainCalls)
				}
				return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall(id, name, args)}}
			}
			switch mainCalls {
			case 1:
				return invoke("moment-create", "moment.publish", map[string]any{})
			case 2:
				for _, raw := range arrayValue(payload["messages"]) {
					message := mapValue(raw)
					if message["role"] == "tool" && message["tool_call_id"] == "moment-create" {
						momentReceipt = stringValue(message["content"])
					}
				}
				return invoke("moment-image-load", capabilityCatalogName, map[string]any{"names": []any{"media.image.generate"}})
			case 3:
				for _, raw := range arrayValue(payload["tools"]) {
					function := mapValue(mapValue(raw)["function"])
					if strings.ReplaceAll(stringValue(function["name"]), ".", "_") == "media_image_generate" {
						_, offeredDestination = mapValue(mapValue(function["parameters"])["properties"])["delivery_target"]
					}
				}
				return invoke("moment-image", "media.image.generate", map[string]any{"intent": "窗外晚霞", "media_plan": mediaTestPlan("landscape", "none", "no", "unknown"), "delivery_target": "current_moment"})
			case 4:
				for _, raw := range arrayValue(payload["messages"]) {
					message := mapValue(raw)
					if message["role"] == "tool" && message["tool_call_id"] == "moment-image" {
						mediaAccepted = strings.Contains(stringValue(message["content"]), "accepted")
					}
				}
				decision := map[string]any{"action_type": "reply", "response_intent": "回应", "influences": []any{}}
				return invoke("moment-reply", conversationReplyCapabilityName, map[string]any{"text": "好，我继续处理。", "decision": decision})
			default:
				t.Errorf("unexpected main request %d", mainCalls)
				return fakeProviderResult{Status: http.StatusBadRequest}
			}
		})
	ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "m2-moment-image", router)
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
		t.Fatal(err)
	}
	role := agentModelBindingKey(FormalAgentConversationReply, "moment")
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,timeout_seconds) VALUES($1,'mixed-endpoint-m2-moment-image','m2-moment-model',30)`, role); err != nil {
		t.Fatal(err)
	}
	enableControlledVision(t, ctx, repo)
	png := ownerMediaPNG(t)
	renderer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/prompt":
			submitted.Add(1)
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || !strings.Contains(jsonString(payload), "窗外晚霞") {
				t.Errorf("renderer lost the requested image intent: %#v err=%v", payload, err)
			}
			_, _ = io.WriteString(w, `{"prompt_id":"m2-moment-job"}`)
		case "/history/m2-moment-job":
			_, _ = io.WriteString(w, `{"m2-moment-job":{"outputs":{"save":{"images":[{"filename":"fixture.png","type":"output"}]}}}}`)
		case "/view":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(png)
		default:
			t.Errorf("unexpected renderer request %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer renderer.Close()
	rendererURL, err := url.Parse(renderer.URL)
	if err != nil {
		t.Fatal(err)
	}
	graph := map[string]any{"positive": map[string]any{"class_type": "CLIPTextEncode", "inputs": map[string]any{"text": "{{prompt}}"}}}
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
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		uploaded.Add(1)
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
	app.S3Bucket = "m2-moment-fixture"
	payload := map[string]any{"fluctlight_id": fid, "text": "发一条动态，再把窗外照片配给这条动态。", "turn_id": "m2-moment-image", "idempotency_key": "m2-moment-image", "attachment_refs": []any{}}
	turn, err := app.HandleTurn(ctx, owner, conversation, payload)
	if err != nil || mainCalls != 4 || momentCalls != 1 || !mediaAccepted || !offeredDestination || stringValue(turn.Assistant["text"]) != "好，我继续处理。" {
		t.Fatalf("Moment/image native loop: main=%d moment=%d accepted=%t offered=%t turn=%#v err=%v", mainCalls, momentCalls, mediaAccepted, offeredDestination, turn.Assistant, err)
	}
	var momentID, intentID, targetMoment, targetConversation, status string
	var raw []byte
	if err := repo.Pool().QueryRow(ctx, `SELECT id FROM public.moments WHERE owner_fluctlight_id=$1`, fid).Scan(&momentID); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(momentReceipt, "completed") || strings.Contains(momentReceipt, momentID) {
		t.Fatalf("model-facing Moment receipt lost completion or leaked its internal ID: %s", momentReceipt)
	}
	if err := repo.Pool().QueryRow(ctx, `SELECT id,COALESCE(moment_id,''),COALESCE(conversation_id,''),status,prompt FROM public.media_intents WHERE owner_fluctlight_id=$1`, fid).Scan(&intentID, &targetMoment, &targetConversation, &status, &raw); err != nil || targetMoment != momentID || targetConversation != "" || status != "pending" {
		t.Fatalf("media did not target the published Moment: moment=%q target=%q conversation=%q status=%q err=%v", momentID, targetMoment, targetConversation, status, err)
	}
	concept = decodeObject(raw)
	var assets []byte
	if err := repo.Pool().QueryRow(ctx, `SELECT media_asset_ids FROM public.moments WHERE id=$1`, momentID).Scan(&assets); err != nil || len(decodeArray(assets)) != 0 {
		t.Fatalf("pending image appeared on Moment: %s err=%v", assets, err)
	}
	if _, err := app.HandleTurn(ctx, owner, conversation, payload); err != nil || mainCalls != 4 || momentCalls != 1 {
		t.Fatalf("turn replay repeated work: main=%d moment=%d err=%v", mainCalls, momentCalls, err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		result, err := app.ProcessMediaIntent(ctx, intentID)
		if err != nil || result["status"] != "completed" {
			t.Fatalf("image process/replay %d result=%#v err=%v", attempt, result, err)
		}
	}
	var refs, messages int
	if err := repo.Pool().QueryRow(ctx, `SELECT media_asset_ids,(SELECT count(*) FROM public.media_references WHERE target_type='moment' AND target_id=$1 AND asset_id=$2),(SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$3 AND kind='media_reference') FROM public.moments WHERE id=$1`, momentID, "asset_"+intentID, conversation).Scan(&assets, &refs, &messages); err != nil || len(decodeArray(assets)) != 1 || !containsStringValue(decodeArray(assets), "asset_"+intentID) || refs != 1 || messages != 0 || submitted.Load() != 1 || uploaded.Load() != 1 || normalizations != 1 || reviews != 1 {
		t.Fatalf("Moment image delivery assets=%s refs=%d conversation_media=%d submit=%d upload=%d normalize=%d review=%d err=%v", assets, refs, messages, submitted.Load(), uploaded.Load(), normalizations, reviews, err)
	}
}

func TestDirectMomentImageTargetRequiresEarlierResult(t *testing.T) {
	for _, scenario := range []string{"no-moment", "same-batch", "invalid-target", "conversation-default"} {
		t.Run(scenario, func(t *testing.T) {
			mainCalls := 0
			var observed string
			router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
				if payload["model"] == "m2-boundary-moment" {
					return fakeProviderResult{Text: "记录窗边的晚霞。"}
				}
				mainCalls++
				image := func() map[string]any {
					args := map[string]any{"intent": "窗外晚霞", "media_plan": mediaTestPlan("landscape", "none", "no", "unknown")}
					if scenario == "invalid-target" {
						args["delivery_target"] = "unknown_target"
					} else if scenario != "conversation-default" {
						args["delivery_target"] = "current_moment"
					}
					return nativePersonaToolCall("boundary-image", "media.image.generate", args)
				}
				if mainCalls == 1 {
					return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("boundary-load", capabilityCatalogName, map[string]any{"names": []any{"media.image.generate"}})}}
				}
				if mainCalls == 2 {
					if scenario == "same-batch" {
						return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("boundary-moment", "moment.publish", map[string]any{}), image()}}
					}
					if scenario == "conversation-default" {
						return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("boundary-moment", "moment.publish", map[string]any{})}}
					}
					return fakeProviderResult{ToolCalls: []map[string]any{image()}}
				}
				if mainCalls == 3 && scenario == "conversation-default" {
					return fakeProviderResult{ToolCalls: []map[string]any{image()}}
				}
				for _, raw := range arrayValue(payload["messages"]) {
					message := mapValue(raw)
					if message["role"] == "tool" && message["tool_call_id"] == "boundary-image" {
						observed = stringValue(message["content"])
					}
				}
				decision := map[string]any{"action_type": "reply", "response_intent": "回应", "influences": []any{}}
				return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("boundary-reply", conversationReplyCapabilityName, map[string]any{"text": "好，我知道了。", "decision": decision})}}
			})
			ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "m2-boundary-"+scenario, router)
			if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
				t.Fatal(err)
			}
			role := agentModelBindingKey(FormalAgentConversationReply, "moment")
			if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,timeout_seconds) VALUES($1,$2,'m2-boundary-moment',30)`, role, "mixed-endpoint-m2-boundary-"+scenario); err != nil {
				t.Fatal(err)
			}
			if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.runtime_settings(key,value_json) VALUES('media.comfyui',$1)`, jsonString(map[string]any{"baseUrl": "http://renderer.invalid", "workflow": map[string]any{"prompt": "{{prompt}}"}})); err != nil {
				t.Fatal(err)
			}
			turn, err := app.HandleTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": fid, "text": "记录一下窗外的景色。", "turn_id": "m2-boundary-" + scenario, "idempotency_key": "m2-boundary-" + scenario, "attachment_refs": []any{}})
			if err != nil || stringValue(turn.Assistant["text"]) != "好，我知道了。" {
				t.Fatalf("turn failed: %#v err=%v", turn.Assistant, err)
			}
			var count int
			var target string
			if err := repo.Pool().QueryRow(ctx, `SELECT count(*),COALESCE(max(conversation_id),'') FROM public.media_intents WHERE owner_fluctlight_id=$1`, fid).Scan(&count, &target); err != nil {
				t.Fatal(err)
			}
			if scenario == "conversation-default" {
				if count != 1 || target != conversation || !strings.Contains(observed, "accepted") {
					t.Fatalf("ordinary image lost conversation target: count=%d target=%q result=%s", count, target, observed)
				}
			} else if scenario == "invalid-target" {
				if count != 0 || !strings.Contains(observed, "invalid_arguments") {
					t.Fatalf("invalid destination created an image: count=%d result=%s", count, observed)
				}
			} else if count != 0 || !strings.Contains(observed, "moment_media_target_unavailable") {
				t.Fatalf("%s allowed an unbound image: count=%d result=%s", scenario, count, observed)
			}
		})
	}
}

func TestIndependentMomentImageTargetChecksOwnershipAndPlugin(t *testing.T) {
	ctx, repo, app, owner, fid, conversation := setupDirectPublicationToolTest(t, "m2-moment-independent")
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'fixture','fixture')`, owner); err != nil {
		t.Fatal(err)
	}
	momentID := "m2-owned-moment"
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.moments(id,owner_fluctlight_id,author_actor_id,text,visibility,status,media_asset_ids) VALUES($1,$2,$2,'已有动态','participants','visible','[]')`, momentID, fid); err != nil {
		t.Fatal(err)
	}
	request := ToolExecutionRequest{CapabilityName: "media.image.generate", OperationID: "m2-independent-moment-image", AuthorizationActorID: owner, FluctlightID: fid, ConversationID: conversation, TargetKind: "moment", TargetRef: momentID, Arguments: jsonBytes(map[string]any{"intent": "窗外晚霞", "media_plan": mediaTestPlan("landscape", "none", "no", "unknown"), "delivery_target": "current_moment"})}
	receipt, err := app.ExecuteTool(ctx, request)
	if err != nil || receipt.Result.Status != "accepted" {
		t.Fatalf("independent Moment image admission: %#v err=%v", receipt, err)
	}
	replayed, err := app.ExecuteTool(ctx, request)
	if err != nil || !replayed.Replayed {
		t.Fatalf("independent Moment image replay: %#v err=%v", replayed, err)
	}
	request.OperationID = "m2-independent-wrong-target"
	request.TargetKind, request.TargetRef = "conversation", conversation
	wrong, err := app.ExecuteTool(ctx, request)
	if err == nil || wrong.Result.ErrorCode != "moment_media_target_required" {
		t.Fatalf("explicit Moment destination accepted a conversation target: %#v err=%v", wrong, err)
	}
	otherOwner, otherFID, otherConversation := "m2-other-owner", "m2-other-fid", "m2-other-conversation"
	seedTurnConversation(t, ctx, repo, otherOwner, otherFID, otherConversation)
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.moments(id,owner_fluctlight_id,author_actor_id,text,visibility,status,media_asset_ids) VALUES('m2-foreign-moment',$1,$1,'他人的动态','participants','visible','[]')`, otherFID); err != nil {
		t.Fatal(err)
	}
	request.OperationID = "m2-independent-foreign-target"
	request.TargetKind, request.TargetRef = "moment", "m2-foreign-moment"
	if foreign, err := app.ExecuteTool(ctx, request); err == nil {
		t.Fatalf("foreign Moment accepted: %#v", foreign)
	}
	if _, err := app.UpdateSettings(ctx, owner, map[string]any{"values": map[string]any{domainPluginsSettingKey: map[string]any{"image": false, "moment": true}}, "expected_versions": map[string]any{}, "idempotency_key": "m2-disable-image-moment"}); err != nil {
		t.Fatal(err)
	}
	request.OperationID = "m2-independent-disabled"
	request.TargetRef = momentID
	if disabled, err := app.ExecuteTool(ctx, request); err == nil || disabled.Result.ErrorCode != "domain_plugin_disabled" {
		t.Fatalf("disabled Image accepted a new Moment image: %#v err=%v", disabled, err)
	}
	var count int
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.media_intents WHERE owner_fluctlight_id=$1`, fid).Scan(&count); err != nil || count != 1 {
		t.Fatalf("unexpected independent intent count=%d err=%v", count, err)
	}
}

func TestMomentImageUncertainSubmissionPausesWithoutAttachment(t *testing.T) {
	ctx, repo, app, owner, fid, conversation := setupDirectPublicationToolTest(t, "m2-moment-uncertain")
	seedCognitiveProviderRole(t, ctx, repo, "m2-moment-uncertain-provider")
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'fixture','fixture')`, owner); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.moments(id,owner_fluctlight_id,author_actor_id,text,visibility,status,media_asset_ids) VALUES('m2-uncertain-moment',$1,$1,'文字动态','participants','visible','[]')`, fid); err != nil {
		t.Fatal(err)
	}
	var concept map[string]any
	var submits atomic.Int32
	router := newFakeProviderRouter().on("media_prompt_normalized", func(map[string]any) fakeProviderResult {
		return fakeProviderResult{Structured: mediaTestOutput(concept)}
	})
	renderer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/prompt" {
			t.Errorf("unexpected renderer path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		submits.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer renderer.Close()
	rendererURL, err := url.Parse(renderer.URL)
	if err != nil {
		t.Fatal(err)
	}
	app.Provider.HTTP = &http.Client{Transport: projectHealthRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host == rendererURL.Host {
			return http.DefaultTransport.RoundTrip(r)
		}
		return router.RoundTrip(r)
	})}
	graph := map[string]any{"positive": map[string]any{"class_type": "CLIPTextEncode", "inputs": map[string]any{"text": "{{prompt}}"}}}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.runtime_settings SET value_json=$1 WHERE key='media.comfyui'`, jsonString(map[string]any{"baseUrl": renderer.URL, "workflow": graph})); err != nil {
		t.Fatal(err)
	}
	request := ToolExecutionRequest{CapabilityName: "media.image.generate", OperationID: "m2-uncertain-moment-image", AuthorizationActorID: owner, FluctlightID: fid, ConversationID: conversation, TargetKind: "moment", TargetRef: "m2-uncertain-moment", Arguments: jsonBytes(map[string]any{"intent": "窗外晚霞", "media_plan": mediaTestPlan("landscape", "none", "no", "unknown"), "delivery_target": "current_moment"})}
	receipt, err := app.ExecuteTool(ctx, request)
	if err != nil || receipt.Result.Status != "accepted" {
		t.Fatalf("Moment image admission: %#v err=%v", receipt, err)
	}
	intentID := stringValue(mapValue(receipt.Result.Output)["media_intent_id"])
	var raw []byte
	if err := repo.Pool().QueryRow(ctx, `SELECT prompt FROM public.media_intents WHERE id=$1`, intentID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	concept = decodeObject(raw)
	for attempt := 0; attempt < 2; attempt++ {
		if _, err := app.ProcessMediaIntent(ctx, intentID); err == nil || !strings.Contains(err.Error(), "media_submission_ambiguous") {
			t.Fatalf("uncertain image attempt %d err=%v", attempt, err)
		}
	}
	var status string
	var assets []byte
	var references int
	if err := repo.Pool().QueryRow(ctx, `SELECT i.status,m.media_asset_ids,(SELECT count(*) FROM public.media_references WHERE target_type='moment' AND target_id=m.id) FROM public.media_intents i JOIN public.moments m ON m.id=i.moment_id WHERE i.id=$1`, intentID).Scan(&status, &assets, &references); err != nil || status != "submission_unknown" || len(decodeArray(assets)) != 0 || references != 0 || submits.Load() != 1 {
		t.Fatalf("uncertain Moment image status=%s assets=%s refs=%d submits=%d err=%v", status, assets, references, submits.Load(), err)
	}
}
