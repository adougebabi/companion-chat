package core

import (
	"fmt"
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
				return fakeProviderResult{}
			case 6, 7, 8:
				receipts := nativePersonaToolMessages(payload)
				if !strings.Contains(receipts, "accepted") {
					t.Error("model did not receive an actual accepted image intent receipt")
				}
				if mainCalls == 6 && (!strings.Contains(jsonString(payload["messages"]), "private_reply_not_published") ||
					len(arrayValue(payload["tools"])) != 1 || !r6PayloadHasTool(payload, conversationReplyCapabilityName)) {
					t.Error("empty stop did not lead to a reply-only correction")
				}
				if mainCalls > 6 && !strings.Contains(receipts, "internal_intent") && !strings.Contains(receipts, "state_expression") {
					t.Error("model did not receive specific feedback about the optional field")
				}
				return invoke(fmt.Sprintf("photo-truthful-reply-%d", mainCalls), conversationReplyCapabilityName, map[string]any{
					"text": queued,
					"decision": map[string]any{
						"action_type": "reply", "response_intent": "确认任务已提交", "influences": []any{},
						"internal_intent":  strings.Repeat("等待", 121),
						"state_expression": []any{"无效结构"},
						"core_alignment":   map[string]any{"aligned": true, "note": "保留"},
					},
				})
			case 9:
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
	if err != nil || queryErr != nil || intents != 1 || stringValue(result.Assistant["text"]) != queued || mainCalls != 8 {
		t.Fatalf("capture follow-up did not complete: intents=%d calls=%d reply=%q turn=%v query=%v", intents, mainCalls, stringValue(result.Assistant["text"]), err, queryErr)
	}
	var recorded []byte
	if err := repo.Pool().QueryRow(ctx, `SELECT invocation->'arguments'->'decision' FROM public.tool_executions WHERE invocation->>'call_id'='photo-truthful-reply-8'`).Scan(&recorded); err != nil {
		t.Fatal(err)
	}
	decision := decodeObject(recorded)
	if decision["internal_intent"] != nil || decision["state_expression"] != nil ||
		mapValue(decision["core_alignment"])["note"] != "保留" || decision["response_intent"] != "确认任务已提交" {
		t.Fatalf("recovery did not preserve the valid decision fields: %s", recorded)
	}
	replayed, err := app.HandleTurn(ctx, owner, conversation, followup)
	if err != nil || replayed.Assistant["id"] != result.Assistant["id"] || mainCalls != 8 {
		t.Fatalf("terminal replay created new work: calls=%d err=%v", mainCalls, err)
	}
	ordinary, err := app.HandleTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": fid, "text": "晚安", "idempotency_key": "ordinary", "attachment_refs": []any{}})
	if err != nil || stringValue(ordinary.Assistant["text"]) != "晚安。" || mainCalls != 9 || discoveryCalls != 2 {
		t.Fatalf("ordinary turn did not reply directly: main=%d discovery=%d err=%v", mainCalls, discoveryCalls, err)
	}
}
