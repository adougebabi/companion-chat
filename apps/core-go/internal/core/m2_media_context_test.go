package core

import (
	"fmt"
	"strings"
	"testing"
)

// The previous exchange supplies the missing photo reference. The current
// message and reply deliberately do not repeat the word "photo".
func TestDirectReplyDoesNotPublishUndeliveredPhotoClaimFromPreviousTurn(t *testing.T) {
	testDirectReplyUndeliveredPhotoClaimFromPreviousTurn(t, true)
}

func TestDirectReplyRejectsOmittedCrossTurnPhotoClaim(t *testing.T) {
	testDirectReplyUndeliveredPhotoClaimFromPreviousTurn(t, false)
}

func testDirectReplyUndeliveredPhotoClaimFromPreviousTurn(t *testing.T, extracted bool) {
	t.Helper()
	const priorReply = "照片还没准备好。"
	const claim = "已经贴到聊天里了，你往上翻就能看到。"
	mainCalls, claimChecks := 0, 0
	router := newFakeProviderRouter().on(directImageDiscoverySchema, func(map[string]any) fakeProviderResult {
		return fakeProviderResult{Structured: map[string]any{"needed_tool": "none"}}
	}).on(directMediaDeliveryCheckSchema, func(payload map[string]any) fakeProviderResult {
		var view map[string]any
		for _, raw := range arrayValue(payload["messages"]) {
			message := mapValue(raw)
			if message["role"] == "user" {
				view = decodeObject([]byte(stringValue(message["content"])))
			}
		}
		claims := []any{}
		if view["candidate"] == claim {
			claimChecks++
			if extracted {
				claims = append(claims, map[string]any{"quote": "已经贴到聊天里了", "delivery_ref": "", "subject_quote": ""})
			}
		}
		return fakeProviderResult{Structured: map[string]any{"claims": claims}}
	}).otherwise(func(map[string]any) fakeProviderResult {
		mainCalls++
		text := claim
		if mainCalls == 1 {
			text = priorReply
		}
		decision := map[string]any{"action_type": "reply", "response_intent": "回应", "influences": []any{}}
		return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall(fmt.Sprintf("context-reply-%d", mainCalls), conversationReplyCapabilityName, map[string]any{"text": text, "decision": decision})}}
	})
	ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "m2-media-context", router)
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
		t.Fatal(err)
	}
	persistConversationReplyMode(t, ctx, repo, "direct")
	if _, err := app.HandleTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": fid, "text": "能给我看看照片吗？", "turn_id": "m2-media-prior", "idempotency_key": "m2-media-prior", "attachment_refs": []any{}}); err != nil {
		t.Fatalf("previous conversation was not published: %v", err)
	}
	_, runErr := app.HandleTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": fid, "text": "弄好了没？", "turn_id": "m2-media-followup", "idempotency_key": "m2-media-followup", "attachment_refs": []any{}})
	var published, mediaIntents int
	if err := repo.Pool().QueryRow(ctx, `SELECT (SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant' AND text=$2), (SELECT count(*) FROM public.media_intents WHERE owner_fluctlight_id=$3)`, conversation, claim, fid).Scan(&published, &mediaIntents); err != nil {
		t.Fatal(err)
	}
	if runErr == nil || !strings.Contains(runErr.Error(), "direct_media_delivery_unresolved") || published != 0 || mediaIntents != 0 || claimChecks != 3 || mainCalls != 4 {
		t.Fatalf("unsupported cross-turn delivery was not rejected: published=%d media_intents=%d claim_checks=%d main_calls=%d err=%v", published, mediaIntents, claimChecks, mainCalls, runErr)
	}
}

func TestDirectMediaContextKeepsOrdinaryChatAndMatchingDelivery(t *testing.T) {
	for _, tc := range []struct {
		name, input, reply, subject, quote string
		delivered, reject                  bool
		wantChecks                         int
	}{
		{name: "ordinary", input: "今天工作怎么样？", reply: "我刚下班，很放松。"},
		{name: "matching_delivery", input: "窗外日落弄好了没？", reply: "已经贴到聊天里了，你往上翻就能看到。", subject: "窗外日落", delivered: true, wantChecks: 1},
		{name: "matching_full_quote", input: "窗外日落弄好了没？", reply: "已经贴到聊天里了，你往上翻就能看到。", subject: "窗外日落", quote: "已经贴到聊天里了，你往上翻就能看到。", delivered: true, wantChecks: 1},
		{name: "additional_unsupported_claim", input: "窗外日落弄好了没？", reply: "已经贴到聊天里了，另一张也已经发给你了。", subject: "窗外日落", quote: "已经贴到聊天里了，另一张也已经发给你了。", delivered: true, reject: true, wantChecks: 3},
		{name: "unrelated_delivery", input: "海边落日弄好了没？", reply: "已经贴到聊天里了，你往上翻就能看到。", subject: "窗外日落", delivered: true, reject: true, wantChecks: 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mainCalls, finalChecks := 0, 0
			router := newFakeProviderRouter().on(directImageDiscoverySchema, func(map[string]any) fakeProviderResult {
				return fakeProviderResult{Structured: map[string]any{"needed_tool": "none"}}
			}).on(directMediaDeliveryCheckSchema, func(payload map[string]any) fakeProviderResult {
				var view map[string]any
				for _, raw := range arrayValue(payload["messages"]) {
					message := mapValue(raw)
					if message["role"] == "user" {
						view = decodeObject([]byte(stringValue(message["content"])))
					}
				}
				claims := []any{}
				if view["candidate"] == tc.reply {
					finalChecks++
					if tc.delivered {
						deliveries := arrayValue(view["committed_deliveries"])
						if len(deliveries) != 1 {
							t.Errorf("matching delivery missing: %#v", deliveries)
						} else {
							quote := "已经贴到聊天里了"
							if tc.quote != "" {
								quote = tc.quote
							}
							claims = append(claims, map[string]any{"quote": quote, "delivery_ref": mapValue(deliveries[0])["ref"], "subject_quote": tc.subject})
						}
					}
				}
				return fakeProviderResult{Structured: map[string]any{"claims": claims}}
			}).otherwise(func(map[string]any) fakeProviderResult {
				mainCalls++
				text := tc.reply
				if mainCalls == 1 {
					text = "照片还没准备好。"
				}
				decision := map[string]any{"action_type": "reply", "response_intent": "回应", "influences": []any{}}
				return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall(fmt.Sprintf("context-%s-%d", tc.name, mainCalls), conversationReplyCapabilityName, map[string]any{"text": text, "decision": decision})}}
			})
			ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "m2-context-"+tc.name, router)
			if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
				t.Fatal(err)
			}
			persistConversationReplyMode(t, ctx, repo, "direct")
			if _, err := app.HandleTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": fid, "text": "想看看窗外日落的照片。", "turn_id": "context-prior-" + tc.name, "idempotency_key": "context-prior-" + tc.name, "attachment_refs": []any{}}); err != nil {
				t.Fatal(err)
			}
			if tc.delivered {
				seedDirectMediaEvidence(t, ctx, repo, app, fid, conversation, "context-delivered-"+tc.name, "completed", tc.subject, true)
			}
			result, err := app.HandleTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": fid, "text": tc.input, "turn_id": "context-final-" + tc.name, "idempotency_key": "context-final-" + tc.name, "attachment_refs": []any{}})
			if tc.reject {
				var published int
				if queryErr := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant' AND text=$2`, conversation, tc.reply).Scan(&published); queryErr != nil {
					t.Fatal(queryErr)
				}
				if err == nil || !strings.Contains(err.Error(), "direct_media_delivery_unresolved") || mainCalls != 4 || finalChecks != tc.wantChecks || published != 0 {
					t.Fatalf("unrelated delivery supported the claim: published=%d main_calls=%d checks=%d result=%#v err=%v", published, mainCalls, finalChecks, result, err)
				}
				return
			}
			if err != nil || stringValue(result.Assistant["text"]) != tc.reply || mainCalls != 2 || finalChecks != tc.wantChecks {
				t.Fatalf("valid reply did not complete: main_calls=%d checks=%d result=%#v err=%v", mainCalls, finalChecks, result, err)
			}
		})
	}
}

func TestDirectMediaContextPrefilterIgnoresOlderUnrelatedTopic(t *testing.T) {
	history := []map[string]any{
		{"kind": "user", "text": "刚才那张照片怎么样？"},
		{"kind": "assistant", "text": "挺好看的。"},
		{"kind": "user", "text": "今天工作怎么样？"},
		{"kind": "assistant", "text": "今天已经下班了。"},
		{"kind": "user", "text": "弄好了没？"},
	}
	if directMediaDeliveryCheckNeeded("已经贴到聊天里了。", "弄好了没？", history) {
		t.Fatal("an older unrelated media topic triggered a new media check")
	}
}
