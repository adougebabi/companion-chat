package core

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// A model-generated completion claim is not evidence that a photo was sent.
// This fixture creates no media intent and makes no media ToolCall.
func TestDirectReplyDoesNotPublishUndeliveredPhotoClaim(t *testing.T) {
	checkerCalls, mainCalls := 0, 0
	router := newFakeProviderRouter().on(directMediaDeliveryCheckSchema, func(map[string]any) fakeProviderResult {
		checkerCalls++
		return fakeProviderResult{Structured: map[string]any{"claims": []any{map[string]any{"quote": "我已经把照片发给你了。", "delivery_ref": "", "subject_quote": ""}}}}
	}).otherwise(func(map[string]any) fakeProviderResult {
		mainCalls++
		decision := map[string]any{"action_type": "reply", "response_intent": "回复", "influences": []any{}}
		return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall(fmt.Sprintf("direct-media-claim-%d", mainCalls), conversationReplyCapabilityName, map[string]any{"text": "我已经把照片发给你了。", "decision": decision})}}
	})
	ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "direct-undelivered-photo-claim", router)
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
		t.Fatal(err)
	}
	persistConversationReplyMode(t, ctx, repo, "direct")
	_, turnErr := app.HandleTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": fid, "text": "照片呢？", "turn_id": "direct-media-claim", "idempotency_key": "direct-media-claim", "attachment_refs": []any{}})
	var claims, mediaCalls, mediaIntents int
	err := repo.Pool().QueryRow(ctx, `SELECT
		(SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant' AND text='我已经把照片发给你了。' AND attachment_refs='[]'::jsonb),
		(SELECT count(*) FROM public.tool_executions WHERE fluctlight_id=$2 AND capability_name='media.image.generate'),
		(SELECT count(*) FROM public.media_intents WHERE owner_fluctlight_id=$2)`, conversation, fid).Scan(&claims, &mediaCalls, &mediaIntents)
	if err != nil {
		t.Fatal(err)
	}
	if claims != 0 || mediaCalls != 0 || mediaIntents != 0 || checkerCalls != 3 || mainCalls != 3 || turnErr == nil || !strings.Contains(turnErr.Error(), "direct_media_delivery_unresolved") {
		t.Fatalf("unverified delivery claim or unrelated failure: claims=%d media_calls=%d media_intents=%d checker=%d main=%d turn_error=%v", claims, mediaCalls, mediaIntents, checkerCalls, mainCalls, turnErr)
	}
}

func TestDirectMediaClaimChecksSpecificCommittedDeliveryAndAllowsOrdinaryReplies(t *testing.T) {
	cases := []struct {
		name, input, reply, deliveredIntent, subject, quote string
		claim, publish                                      bool
	}{
		{name: "ordinary", input: "今天怎么样？", reply: "今天的风很舒服。", publish: true},
		{name: "discuss_photo", input: "这张照片怎么样？", reply: "照片里的天空很漂亮。", publish: true},
		{name: "user_sent_photo", input: "这张照片怎么样？", reply: "你发给我的照片很好看。", publish: true},
		{name: "user_sent_photo_variant", input: "这张照片怎么样？", reply: "你发来的照片真美。", publish: true},
		{name: "ordinary_occurrence", input: "你觉得这张照片怎么样？", reply: "这张照片让我想起去年发生的事。", publish: true},
		{name: "ordinary_tradition", input: "照片里是什么？", reply: "照片里的传统建筑很美。", publish: true},
		{name: "ordinary_question", input: "这张照片怎么样？", reply: "这张照片的感觉给你怎样的印象？", publish: true},
		{name: "ordinary_photo_feeling", input: "这张照片怎么样？", reply: "这张照片给你一种温暖的感觉。", publish: true},
		{name: "delivery_question", input: "照片呢？", reply: "照片发给你了没？", publish: true},
		{name: "deny", input: "照片发了吗？", reply: "我还没发照片，等会儿再发给你。", publish: true},
		{name: "cannot_send", input: "照片发了吗？", reply: "照片发不出去。", publish: true},
		{name: "extractor_misreads_denial", input: "照片发了吗？", reply: "我还没发照片，等会儿再发给你。", quote: "我还没发照片", claim: true, publish: true},
		{name: "deny_with_completion_words", input: "图片发了吗？", reply: "图片还没有成功发送完成。", publish: true},
		{name: "future", input: "能发张照片吗？", reply: "等天气好了，我再发给你一张照片。", publish: true},
		{name: "tonight", input: "能发张照片吗？", reply: "我今晚发你照片。", publish: true},
		{name: "conditional_future", input: "图片发了吗？", reply: "等图片发送完成后，我会告诉你。", publish: true},
		{name: "extractor_misreads_future", input: "图片发了吗？", reply: "等图片发送完成后，我会告诉你。", quote: "图片发送完成", claim: true, publish: true},
		{name: "history_match", input: "窗外日落的照片发过吗？", reply: "我之前已经把窗外日落的照片发给你了。", deliveredIntent: "窗外日落", subject: "窗外日落", claim: true, publish: true},
		{name: "single_character_history_subject", input: "猫的照片发过吗？", reply: "猫的照片已经发给你了。", deliveredIntent: "猫", subject: "猫", claim: true, publish: true},
		{name: "unrelated_history", input: "海边落日的照片发过吗？", reply: "我已经把海边落日的照片发给你了。", deliveredIntent: "窗外的猫", subject: "窗外的猫", claim: true},
		{name: "generic_subject_attack", input: "月亮照片发了吗？", reply: "我已经把月亮照片发给你了。", deliveredIntent: "我家的猫", subject: "我", claim: true},
		{name: "generic_quote_attack", input: "月亮照片发了吗？", reply: "我已经把月亮照片发给你了。", deliveredIntent: "我家的猫", subject: "我", quote: "我", claim: true},
		{name: "shared_location_attack", input: "窗外的白猫照片发了吗？", reply: "窗外的白猫照片已经发给你了。", deliveredIntent: "窗外的黑猫", subject: "窗外", claim: true},
		{name: "multi_subject_attack", input: "猫咪和狗狗的照片都发了吗？", reply: "狗狗的照片已经发给你了。", deliveredIntent: "猫咪", subject: "猫咪", claim: true},
		{name: "short_quote_cannot_borrow_other_subject", input: "猫咪和狗狗的照片都发了吗？", reply: "狗狗的照片已经发给你了。", deliveredIntent: "猫咪", subject: "猫咪", quote: "发给你了", claim: true},
		{name: "no_delivery", input: "照片发了吗？", reply: "我已经把照片发给你了。", claim: true},
		{name: "extractor_identifies_posted_to_chat", input: "照片发了吗？", reply: "照片我刚贴到聊天里了，你往上翻就能看到。", claim: true},
		{name: "implicit_photo_in_current_input", input: "照片发了吗？", reply: "已经放到聊天里了，你往上翻就能看到。", quote: "已经放到聊天里了", claim: true},
		{name: "negated_surprise", input: "照片发了吗？", reply: "我没想到照片已经发给你了。", claim: true},
		{name: "extractor_omitted_obvious_claim", input: "照片发了吗？", reply: "我已经把照片发给你了。"},
		{name: "delivered_word", input: "照片呢？", reply: "照片已经交付了。", claim: true},
		{name: "delivered_synonym", input: "照片呢？", reply: "照片已投递到你的聊天里了。", claim: true},
		{name: "attached_english", input: "Where is the photo?", reply: "The photo is attached.", claim: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mainCalls, checkerCalls := 0, 0
			router := newFakeProviderRouter().on(directMediaDeliveryCheckSchema, func(payload map[string]any) fakeProviderResult {
				checkerCalls++
				var view map[string]any
				for _, raw := range arrayValue(payload["messages"]) {
					message := mapValue(raw)
					if message["role"] == "user" {
						view = decodeObject([]byte(stringValue(message["content"])))
					}
				}
				if view["candidate"] != tc.reply || len(arrayValue(payload["tools"])) != 0 {
					t.Errorf("checker lost candidate or gained tools")
				}
				claims := []any{}
				if tc.claim {
					ref := ""
					if deliveries := arrayValue(view["committed_deliveries"]); len(deliveries) > 0 {
						ref = stringValue(mapValue(deliveries[0])["ref"])
					}
					quote := tc.reply
					if tc.quote != "" {
						quote = tc.quote
					}
					claims = append(claims, map[string]any{"quote": quote, "delivery_ref": ref, "subject_quote": tc.subject})
				}
				return fakeProviderResult{Structured: map[string]any{"claims": claims}}
			}).otherwise(func(map[string]any) fakeProviderResult {
				mainCalls++
				decision := map[string]any{"action_type": "reply", "response_intent": "回复", "influences": []any{}}
				return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall(fmt.Sprintf("reply-%d", mainCalls), conversationReplyCapabilityName, map[string]any{"text": tc.reply, "decision": decision})}}
			})
			ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "direct-claim-"+tc.name, router)
			if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
				t.Fatal(err)
			}
			persistConversationReplyMode(t, ctx, repo, "direct")
			if tc.deliveredIntent != "" {
				seedDirectMediaEvidence(t, ctx, repo, app, fid, conversation, "prior-"+tc.name, "completed", tc.deliveredIntent, true)
			}
			result, turnErr := app.HandleTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": fid, "text": tc.input, "turn_id": "claim-" + tc.name, "idempotency_key": "claim-" + tc.name, "attachment_refs": []any{}})
			var messages int
			if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant' AND text=$2`, conversation, tc.reply).Scan(&messages); err != nil {
				t.Fatal(err)
			}
			if tc.publish {
				if turnErr != nil || messages != 1 || stringValue(result.Assistant["text"]) != tc.reply {
					t.Fatalf("valid reply was blocked: messages=%d result=%#v error=%v", messages, result, turnErr)
				}
			} else if turnErr == nil || !strings.Contains(turnErr.Error(), "direct_media_delivery_unresolved") || messages != 0 || mainCalls != 3 || checkerCalls != 3 {
				t.Fatalf("unsupported delivery was published or failed for an unrelated reason: messages=%d main=%d checker=%d error=%v", messages, mainCalls, checkerCalls, turnErr)
			}
			if tc.name == "ordinary" && (mainCalls != 1 || checkerCalls != 0) {
				t.Fatalf("ordinary chat gained a fixed checker: main=%d checker=%d", mainCalls, checkerCalls)
			}
		})
	}
}

func TestDirectMediaCompletionGuardDoesNotTreatDenialAsConfirmation(t *testing.T) {
	for _, tc := range []struct {
		text string
		want bool
	}{
		{"我已经把照片发给你了。", true},
		{"我还没发照片，等会儿发给你。", false},
		{"我没发过照片。", false},
		{"我没有发给你。", false},
		{"我没拍新照片，但之前已经发给你了。", true},
		{"图片还没有成功发送完成。", false},
		{"等图片发送完成后，我会告诉你。", false},
		{"照片已经交付了。", true},
		{"我把照片交付完了，你看到了吗？", true},
		{"我没拍新照片但我已经发给你了。", true},
		{"我没想到照片已经发给你了。", true},
		{"照片已经发了吗？", false},
		{"照片发给你了没？", false},
		{"我已经发给你了你看到了没？", true},
		{"我已经发给你了你看到了吗？", true},
		{"I haven't sent the picture.", false},
		{"I already sent you the picture.", true},
		{"The photo is attached.", true},
	} {
		if got := directObviousMediaCompletion(tc.text); got != tc.want {
			t.Errorf("directObviousMediaCompletion(%q)=%t want=%t", tc.text, got, tc.want)
		}
	}
	if directMediaSubjectMatches("照片", "照片发给你了", "照片呢", "照片：窗外的猫") || !directMediaSubjectMatches("窗外的猫", "窗外的猫发给你了", "照片呢", "窗外的猫") {
		t.Fatal("generic or specific historical subject was misclassified")
	}
	if directMediaSubjectMatches("窗外的猫", "海边落日发给你了", "海边落日呢", "窗外的猫") {
		t.Fatal("an unrelated historic subject supported the current claim")
	}
	if directMediaSubjectMatches("我", "我已经把月亮照片发给你了", "月亮照片发了吗", "我家的猫") {
		t.Fatal("a pronoun linked an unrelated historical task")
	}
	if directMediaSubjectMatches("发给", "我发给你月亮的照片", "月亮照片呢", "发给用户的猫照片") || directMediaSubjectMatches("猫", "白猫照片发给你了", "白猫照片呢", "黑猫照片") {
		t.Fatal("generic action or one-character overlap linked different media tasks")
	}
	if directMediaSubjectMatches("窗外", "窗外的白猫照片已经发给你了", "窗外的白猫呢", "窗外的黑猫") || directMediaSubjectMatches("猫咪", "狗狗的照片已经发给你了", "猫咪和狗狗的照片都发了吗", "猫咪") {
		t.Fatal("a shared location or another subject linked a different historical task")
	}
	current := directMediaDelivery{intentID: "current", Intent: "窗外日落"}
	if !directSameCurrentMediaIntent(current, "窗外日落", "这张照片已经发给你了。", map[string]bool{"current": true}) || !directSameCurrentMediaIntent(current, "给我看看窗外日落吧", "这张照片已经发给你了。", map[string]bool{"current": true}) || directSameCurrentMediaIntent(current, "猫咪照片", "这张照片已经发给你了。", map[string]bool{"current": true}) || directSameCurrentMediaIntent(current, "窗外日落", "猫照片已经发给你了。", map[string]bool{"current": true}) {
		t.Fatal("current task reference accepted an explicitly different subject")
	}
	if !directMediaDeliveryCheckNeeded("我已经发给你了。", "那张照片呢？") || directMediaDeliveryCheckNeeded("今天的风很舒服。", "今天怎么样？") {
		t.Fatal("candidate prefilter lost its intended scope")
	}
	if directMediaDeliveryCheckNeeded("我现在不想拍，改天吧。", "能发张照片吗？") || !directMediaDeliveryCheckNeeded("已经放到聊天里了。", "照片发了吗？") {
		t.Fatal("media context misclassified a plain refusal or implicit completion")
	}
	if !directMediaDeliveryCheckNeeded("我已经发给你了。", "怎么样了？", []map[string]any{{"kind": "user", "text": "之前那张照片呢？"}}) {
		t.Fatal("implicit media claim in a follow-up lost its historical context")
	}
	if !directMediaDeliveryCheckNeeded("发给你啦。", "怎么样了？", []map[string]any{{"kind": "user", "text": "之前那张照片呢？"}}) {
		t.Fatal("implicit media action in a follow-up lost its historical context")
	}
	if directMediaDeliveryCheckNeeded("这个图论问题可以发散讨论。", "说说算法") {
		t.Fatal("ordinary discussion was mistaken for an image delivery")
	}
	if !directClearlyNoDeliveryClaim("照片已经发了吗？") || directClearlyNoDeliveryClaim("我把照片交付完了，你看到了吗？") {
		t.Fatal("a question obscured whether a different clause asserted delivery")
	}
}

func TestDirectMediaClaimRetryBoundSurvivesInterleavedToolResults(t *testing.T) {
	trace := &ADKCapabilityTrace{}
	for i := 0; i < 3; i++ {
		trace.AppendResult(CapabilityResult{CapabilityName: conversationReplyCapabilityName, Status: "handled", Output: map[string]any{"warning_code": "direct_media_delivery_unverified"}})
		trace.AppendResult(CapabilityResult{CapabilityName: "memory.recall", Status: "completed"})
	}
	if got := directMediaDeliveryRejectedAttempts(trace); got != 3 {
		t.Fatalf("interleaved tools reset the reply rejection bound: got=%d", got)
	}
}

func TestDirectCurrentMediaIntentRequiresMatchingCallAndScope(t *testing.T) {
	trace := &ADKCapabilityTrace{}
	reply := CapabilityInvocation{SourceFactID: "current-fact", Metadata: InvocationMetadata{AuthorizationActorID: "owner", FluctlightID: "self", ConversationID: "conversation"}}
	valid := CapabilityInvocation{CallID: "valid", CapabilityName: "media.image.generate", SourceFactID: reply.SourceFactID, Metadata: reply.Metadata}
	foreign := valid
	foreign.CallID = "foreign"
	foreign.Metadata.ConversationID = "other-conversation"
	trace.AppendInvocation(valid)
	trace.AppendInvocation(foreign)
	trace.AppendResult(CapabilityResult{CallID: "valid", CapabilityName: "media.image.generate", Status: "accepted", Output: map[string]any{"media_intent_id": "actual-intent"}})
	trace.AppendResult(CapabilityResult{CallID: "foreign", CapabilityName: "media.image.generate", Status: "completed", Output: map[string]any{"media_intent_id": "foreign-intent"}})
	trace.AppendResult(CapabilityResult{CallID: "fabricated", CapabilityName: "media.image.generate", Status: "completed", Output: map[string]any{"media_intent_id": "no-call"}})
	got := directCurrentMediaIntents(trace, reply)
	if len(got) != 1 || !got["actual-intent"] {
		t.Fatalf("current media association accepted a foreign or unpaired receipt: %#v", got)
	}
}

func TestDirectCurrentMediaClaimUsesCommittedToolAndDelivery(t *testing.T) {
	var ctx context.Context
	var repo *PostgresRepository
	var app *App
	var owner, fid, conversation string
	mainCalls, checkerCalls := 0, 0
	const input = "给我看看窗外日落吧"
	const intent = "窗外日落"
	const reply = "这张照片已经发给你了。"
	router := newFakeProviderRouter().on(directMediaDeliveryCheckSchema, func(payload map[string]any) fakeProviderResult {
		checkerCalls++
		var view map[string]any
		for _, raw := range arrayValue(payload["messages"]) {
			message := mapValue(raw)
			if message["role"] == "user" {
				view = decodeObject([]byte(stringValue(message["content"])))
			}
		}
		deliveries := arrayValue(view["committed_deliveries"])
		if len(deliveries) != 1 || mapValue(deliveries[0])["current_request"] != true {
			t.Errorf("actual current Tool result was not bound to its delivery: %#v", deliveries)
			return fakeProviderResult{Structured: map[string]any{"claims": []any{}}}
		}
		return fakeProviderResult{Structured: map[string]any{"claims": []any{map[string]any{"quote": reply, "delivery_ref": mapValue(deliveries[0])["ref"], "subject_quote": ""}}}}
	}).otherwise(func(payload map[string]any) fakeProviderResult {
		mainCalls++
		switch mainCalls {
		case 1:
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("load-image", capabilityCatalogName, map[string]any{"names": []any{"media.image.generate"}})}}
		case 2:
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("create-image", "media.image.generate", map[string]any{"intent": intent, "media_plan": mediaTestPlan("landscape", "none", "no", "unknown")})}}
		case 3:
			if !strings.Contains(nativePersonaToolMessages(payload), "accepted") {
				t.Error("the main model did not receive the actual accepted media ToolResult")
			}
			decision := map[string]any{"action_type": "reply", "response_intent": "回复", "influences": []any{}}
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("delivered-reply", conversationReplyCapabilityName, map[string]any{"text": reply, "decision": decision})}}
		default:
			t.Errorf("unexpected main-model continuation %d", mainCalls)
			return fakeProviderResult{Status: 500}
		}
	})
	ctx, repo, app, owner, fid, conversation = setupMixedMediaReplyTurnWithTransport(t, "direct-current-delivery", router)
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
		t.Fatal(err)
	}
	persistConversationReplyMode(t, ctx, repo, "direct")
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.runtime_settings(key,value_json) VALUES('media.comfyui',$1)`, jsonString(map[string]any{"baseUrl": "http://comfy.invalid", "workflow": map[string]any{"prompt": "{{prompt}}"}})); err != nil {
		t.Fatal(err)
	}
	// An isolated database fixture commits an already-delivered asset with the
	// Tool intent, before the next model request is assembled. No renderer runs.
	if _, err := repo.Pool().Exec(ctx, `CREATE FUNCTION public.m1_test_commit_delivery() RETURNS trigger LANGUAGE plpgsql AS $$
	DECLARE seq integer; asset text := 'asset_' || NEW.id; msg text := 'fixture_message_' || NEW.id;
	BEGIN
	 INSERT INTO public.media_assets(id,owner_fluctlight_id,version,kind,mime_type,byte_size,sha256,bucket,object_key,provider_request_id,workflow_id,status)
	 VALUES(asset,NEW.owner_fluctlight_id,'v1','image','image/png',1,'fixture','fixture','fixture',NEW.provider_request_id,NEW.workflow_id,'ready');
	 UPDATE public.conversation_heads SET next_sequence=next_sequence+1 WHERE conversation_id=NEW.conversation_id RETURNING next_sequence-1 INTO seq;
	 INSERT INTO public.conversation_messages(id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key)
	 VALUES(msg,NEW.conversation_id,seq,NEW.owner_fluctlight_id,'media_reference','图片已生成。',jsonb_build_array(asset),'media:'||NEW.id||':conversation-result');
	 INSERT INTO public.media_references(id,asset_id,owner_fluctlight_id,target_type,target_id) VALUES('fixture_ref_'||NEW.id,asset,NEW.owner_fluctlight_id,'conversation',NEW.conversation_id);
	 RETURN NEW;
	END $$; CREATE TRIGGER m1_test_commit_delivery AFTER INSERT ON public.media_intents FOR EACH ROW EXECUTE FUNCTION public.m1_test_commit_delivery()`); err != nil {
		t.Fatal(err)
	}
	result, err := app.HandleTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": fid, "text": input, "turn_id": "direct-current-delivery", "idempotency_key": "direct-current-delivery", "attachment_refs": []any{}})
	var replyCount, attachmentCount int
	queryErr := repo.Pool().QueryRow(ctx, `SELECT (SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant' AND text=$2), (SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='media_reference' AND jsonb_array_length(attachment_refs)=1)`, conversation, reply).Scan(&replyCount, &attachmentCount)
	if err != nil || queryErr != nil || replyCount != 1 || attachmentCount != 1 || mainCalls != 3 || checkerCalls != 1 || stringValue(result.Assistant["text"]) != reply {
		t.Fatalf("current committed delivery reply failed: replies=%d attachments=%d main=%d checker=%d query=%v turn=%v", replyCount, attachmentCount, mainCalls, checkerCalls, queryErr, err)
	}
}

func TestDirectMediaDeliveryDuringReplyPreservesEffectForNextTurn(t *testing.T) {
	var ctx context.Context
	var repo *PostgresRepository
	var app *App
	var owner, fid, conversation string
	mainCalls, checkerCalls := 0, 0
	const intentText = "窗外日落"
	const firstReply = "这张照片已经发给你了。"
	const nextReply = "窗外日落的照片已经发给你了。"
	router := newFakeProviderRouter().on(directMediaDeliveryCheckSchema, func(payload map[string]any) fakeProviderResult {
		checkerCalls++
		var view map[string]any
		for _, raw := range arrayValue(payload["messages"]) {
			message := mapValue(raw)
			if message["role"] == "user" {
				view = decodeObject([]byte(stringValue(message["content"])))
			}
		}
		deliveries := arrayValue(view["committed_deliveries"])
		if len(deliveries) != 1 {
			t.Errorf("committed effect was lost: %#v", deliveries)
			return fakeProviderResult{Structured: map[string]any{"claims": []any{}}}
		}
		subject := ""
		if checkerCalls == 2 {
			subject = intentText
		}
		return fakeProviderResult{Structured: map[string]any{"claims": []any{map[string]any{"quote": view["candidate"], "delivery_ref": mapValue(deliveries[0])["ref"], "subject_quote": subject}}}}
	}).otherwise(func(payload map[string]any) fakeProviderResult {
		mainCalls++
		switch mainCalls {
		case 1:
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("race-load-image", capabilityCatalogName, map[string]any{"names": []any{"media.image.generate"}})}}
		case 2:
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("race-create-image", "media.image.generate", map[string]any{"intent": intentText, "media_plan": mediaTestPlan("landscape", "none", "no", "unknown")})}}
		case 3:
			if !strings.Contains(nativePersonaToolMessages(payload), "accepted") {
				t.Error("the main model did not receive the accepted media ToolResult")
			}
			var intentID string
			if err := repo.Pool().QueryRow(ctx, `SELECT id FROM public.media_intents WHERE owner_fluctlight_id=$1`, fid).Scan(&intentID); err != nil {
				t.Fatal(err)
			}
			if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.media_assets(id,owner_fluctlight_id,version,kind,mime_type,byte_size,sha256,bucket,object_key,provider_request_id,workflow_id,status) SELECT 'asset_'||id,owner_fluctlight_id,'v1','image','image/png',1,'fixture','fixture','fixture',provider_request_id,workflow_id,'ready' FROM public.media_intents WHERE id=$1`, intentID); err != nil {
				t.Fatal(err)
			}
			intent, err := app.readMediaIntent(ctx, intentID)
			if err != nil {
				t.Fatal(err)
			}
			if err := app.publishMediaAsset(ctx, intent, "asset_"+intentID); err != nil {
				t.Fatal(err)
			}
			decision := map[string]any{"action_type": "reply", "response_intent": "回复", "influences": []any{}}
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("race-first-reply", conversationReplyCapabilityName, map[string]any{"text": firstReply, "decision": decision})}}
		case 4:
			decision := map[string]any{"action_type": "reply", "response_intent": "回复", "influences": []any{}}
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("race-next-reply", conversationReplyCapabilityName, map[string]any{"text": nextReply, "decision": decision})}}
		default:
			t.Errorf("unexpected main-model continuation %d", mainCalls)
			return fakeProviderResult{Status: 500}
		}
	})
	ctx, repo, app, owner, fid, conversation = setupMixedMediaReplyTurnWithTransport(t, "direct-delivery-race", router)
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
		t.Fatal(err)
	}
	persistConversationReplyMode(t, ctx, repo, "direct")
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.runtime_settings(key,value_json) VALUES('media.comfyui',$1)`, jsonString(map[string]any{"baseUrl": "http://comfy.invalid", "workflow": map[string]any{"prompt": "{{prompt}}"}})); err != nil {
		t.Fatal(err)
	}
	_, err := app.HandleTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": fid, "text": intentText, "turn_id": "direct-delivery-race-1", "idempotency_key": "direct-delivery-race-1", "attachment_refs": []any{}})
	if err == nil || !strings.Contains(err.Error(), "current_facts_stale") {
		t.Fatalf("late delivery should fence the stale candidate: %v", err)
	}
	var attachments, firstReplies int
	if err := repo.Pool().QueryRow(ctx, `SELECT (SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='media_reference' AND jsonb_array_length(attachment_refs)=1), (SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant')`, conversation).Scan(&attachments, &firstReplies); err != nil || attachments != 1 || firstReplies != 0 {
		t.Fatalf("late delivery effect lost or stale reply published: attachments=%d replies=%d err=%v", attachments, firstReplies, err)
	}
	result, err := app.HandleTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": fid, "text": "窗外日落的照片发过吗？", "turn_id": "direct-delivery-race-2", "idempotency_key": "direct-delivery-race-2", "attachment_refs": []any{}})
	if err != nil || stringValue(result.Assistant["text"]) != nextReply || mainCalls != 4 || checkerCalls != 2 {
		t.Fatalf("next turn could not use the committed history: result=%#v main=%d checker=%d err=%v", result, mainCalls, checkerCalls, err)
	}
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='media_reference'`, conversation).Scan(&attachments); err != nil || attachments != 1 {
		t.Fatalf("next turn duplicated the delivered asset: count=%d err=%v", attachments, err)
	}
}
