package core

import (
	"strings"
	"testing"
)

func TestDirectPhotoFollowupLoadsImageBeforeReply(t *testing.T) {
	const queued = "已经提交拍摄任务，等有结果再发给你。"
	mainCalls, discoveryCalls := 0, 0
	router := newFakeProviderRouter().
		on(directImageDiscoverySchema, func(payload map[string]any) fakeProviderResult {
			discoveryCalls++
			if len(arrayValue(payload["tools"])) != 0 {
				t.Error("discovery assessment should not execute tools")
			}
			neededTool := "none"
			if discoveryCalls == 1 {
				neededTool = "media.image.generate"
			}
			return fakeProviderResult{Structured: map[string]any{"needed_tool": neededTool}}
		}).
		on(directMediaDeliveryCheckSchema, func(map[string]any) fakeProviderResult {
			return fakeProviderResult{Structured: map[string]any{"claims": []any{}}}
		}).
		otherwise(func(payload map[string]any) fakeProviderResult {
			mainCalls++
			invoke := func(id, name string, args map[string]any) fakeProviderResult {
				return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall(id, name, args)}}
			}
			reply := func(id, text string) fakeProviderResult {
				return invoke(id, conversationReplyCapabilityName, map[string]any{
					"text": text, "decision": map[string]any{"action_type": "reply", "response_intent": "回应", "influences": []any{}},
				})
			}
			switch mainCalls {
			case 1:
				return reply("photo-promise", "等我拍好自拍再发给你。")
			case 2:
				tools := arrayValue(payload["tools"])
				if len(tools) != 3 || mapValue(mapValue(tools[0])["function"])["name"] != "capability_catalog" ||
					r6PayloadHasTool(payload, "media.image.generate") {
					t.Fatal("direct first request must offer the same three baseline tools with the catalog first")
				}
				return reply("photo-premature", "正在准备。")
			case 3:
				if len(arrayValue(payload["tools"])) != 1 || !r6PayloadHasTool(payload, capabilityCatalogName) ||
					!strings.Contains(nativePersonaToolMessages(payload), "catalog_discovery_pending") {
					t.Fatal("the continuation did not request the model-selected catalog lookup")
				}
				return invoke("photo-disclose", capabilityCatalogName, map[string]any{"names": []any{"media.image.generate"}})
			case 4:
				if !r6PayloadHasTool(payload, "media.image.generate") || !strings.Contains(nativePersonaToolMessages(payload), "media.image.generate") {
					t.Fatal("named catalog request did not disclose the native image tool")
				}
				plan := mediaTestPlan("person", "present", "yes", "full")
				plan["capture_method"], plan["viewpoint"], plan["framing"], plan["camera_angle"] = "handheld_selfie", "subject_facing", "full_body", "eye_level"
				return invoke("photo-execute", "media.image.generate", map[string]any{"intent": "拍一张当前自拍", "media_plan": plan})
			case 5:
				if !strings.Contains(nativePersonaToolMessages(payload), "accepted") {
					t.Error("model did not receive an actual accepted image intent receipt")
				}
				return reply("photo-truthful-reply", queued)
			case 6:
				if len(arrayValue(payload["tools"])) != 3 || r6PayloadHasTool(payload, "media.image.generate") {
					t.Error("the optional image tool leaked into an ordinary reply")
				}
				return reply("ordinary-reply", "晚安。")
			default:
				t.Errorf("unexpected continuation %d", mainCalls)
				return fakeProviderResult{Status: 500}
			}
		})
	ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "direct-photo-progress", router)
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_visual_identities(id,fluctlight_id,status,identity_snapshot) VALUES('fixture-photo-identity',$1,'active','{"identity":{"name":"Fixture identity"}}')`, fid); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.runtime_settings(key,value_json) VALUES('media.comfyui',$1)`, jsonString(map[string]any{
		"baseUrl": "http://comfy.invalid", "workflow": map[string]any{"prompt": "{{prompt}}"},
	})); err != nil {
		t.Fatal(err)
	}
	persistConversationReplyMode(t, ctx, repo, "direct")
	if _, err := app.HandleTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": fid, "text": "能发张自拍吗？", "idempotency_key": "photo-prior", "attachment_refs": []any{}}); err != nil {
		t.Fatalf("set up prior turn: %v", err)
	}
	followup := map[string]any{"fluctlight_id": fid, "text": "还没发哦", "idempotency_key": "photo-followup", "attachment_refs": []any{}}
	result, err := app.HandleTurn(ctx, owner, conversation, followup)
	var intents int
	queryErr := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.media_intents WHERE conversation_id=$1 AND owner_fluctlight_id=$2 AND kind='image'`, conversation, fid).Scan(&intents)
	if err != nil || queryErr != nil || intents != 1 || stringValue(result.Assistant["text"]) != queued || mainCalls != 5 {
		t.Fatalf("capture follow-up did not complete: intents=%d calls=%d reply=%q turn=%v query=%v", intents, mainCalls, stringValue(result.Assistant["text"]), err, queryErr)
	}
	replayed, err := app.HandleTurn(ctx, owner, conversation, followup)
	if err != nil || replayed.Assistant["id"] != result.Assistant["id"] || mainCalls != 5 {
		t.Fatalf("terminal replay created new work: calls=%d err=%v", mainCalls, err)
	}
	ordinary, err := app.HandleTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": fid, "text": "晚安", "idempotency_key": "ordinary", "attachment_refs": []any{}})
	if err != nil || stringValue(ordinary.Assistant["text"]) != "晚安。" || mainCalls != 6 || discoveryCalls != 2 {
		t.Fatalf("ordinary turn did not reply directly: main=%d discovery=%d err=%v", mainCalls, discoveryCalls, err)
	}
}
