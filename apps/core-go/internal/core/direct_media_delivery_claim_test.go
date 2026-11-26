package core

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestCurrentImageReplyChecksDeliveryAssessment(t *testing.T) {
	for _, test := range []struct {
		name, candidate, assessment, quote string
		pass                               bool
	}{
		{"generation pending", "想拍的照片请求我已经提交给系统啦，正在排队！等弄好了，第一时间就分享给你。", "generation_pending", "想拍的照片请求我已经提交给系统啦", true},
		{"future delivery across clauses", "我这就去准备。等我弄好啦，第一时间就分享给你。", "future_delivery", "我这就去准备。", true},
		{"false completion overrides assessment", "新照片还在排队，旧照我已经发给你了。", "generation_pending", "新照片还在排队", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			router := newFakeProviderRouter().on(directMediaDeliveryCheckSchema, func(payload map[string]any) fakeProviderResult {
				calls++
				format := mapValue(payload["response_format"])
				schema := mapValue(mapValue(format["json_schema"])["schema"])
				if len(mapValue(mapValue(schema["properties"])["assessment"])) == 0 {
					t.Error("model did not receive the typed assessment")
				}
				return fakeProviderResult{Structured: map[string]any{
					"assessment": test.assessment,
					"claims":     []any{map[string]any{"quote": test.quote, "delivery_ref": "", "subject_quote": ""}},
				}}
			})
			app := &App{Provider: &ProviderClient{HTTP: &http.Client{Transport: router}}}
			ctx := withFrozenProviderAssignment(context.Background(), "cognitive_assessment", providerAssignment{
				Role: "cognitive_assessment", BaseURL: "http://fixture.invalid", ModelID: "assessment-fixture", Timeout: time.Second,
			})
			refs, err := app.checkDirectMediaDeliveryClaims(ctx, ContextProjection{}, "照片呢？", test.candidate, nil, map[string]bool{"current-photo": true})
			if (err == nil) != test.pass || len(refs) != 0 || calls != 1 {
				t.Fatalf("unexpected assessment: references=%v calls=%d err=%v", refs, calls, err)
			}
		})
	}
}

// Photo wording in a direct reply is the character's words, not a delivery
// receipt. It is published once, without a delivery check or a model retry.
func TestDirectReplyPublishesPhotoWordingWithoutDeliveryCheck(t *testing.T) {
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
	if claims != 1 || mediaCalls != 0 || mediaIntents != 0 || checkerCalls != 0 || mainCalls != 1 || turnErr != nil {
		t.Fatalf("photo wording was withheld or re-asked: claims=%d media_calls=%d media_intents=%d checker=%d main=%d turn_error=%v", claims, mediaCalls, mediaIntents, checkerCalls, mainCalls, turnErr)
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
