package core

import (
	"strings"
	"testing"
	"time"
)

// The event and previous delivery are persisted test fixtures. The Provider
// only records the real request and returns scripted silence, not a judgment
// about what a live personality would choose to do.
func TestWakeUpRequestReceivesNewEventAndUnansweredDelivery(t *testing.T) {
	ctx, repo := isolatedCoreTestRepository(t)
	owner, self := "wakeup-event-owner", "wakeup-event-self"
	seedLifeContextFluctlight(t, ctx, repo, owner, self)
	seedCognitiveProviderRole(t, ctx, repo, "wakeup-event-endpoint")
	persistConversationReplyMode(t, ctx, repo, "legacy")
	var requests []map[string]any
	publish := false
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		requests = append(requests, payload)
		if publish && !conversationPayloadHasToolResult(payload) {
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("event-share", conversationReplyCapabilityName, map[string]any{"text": "我刚看到一朵新花。", "topic_key": "new-flower", "purpose": "share-new-event"})}}
		}
		return fakeProviderResult{Structured: map[string]any{"action_type": "no_op", "response_intent": "fixture", "evidence_refs": []any{}, "influences": []any{}}}
	})
	app := newTestApp(t, repo, router)
	life := currentLifeForTest(t, ctx, app, self, time.Now().UTC())
	if _, err := app.AcceptSchedule(ctx, owner, self, fullDaySchedulePayloadForTest(time.Now().UTC(), "wakeup-event-schedule", stringValue(life["context_revision"]))); err != nil {
		t.Fatal(err)
	}
	conversation, err := app.EnsureDirectConversation(ctx, owner, self)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.conversation_messages(id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key)
		VALUES('previous-proactive',$1,1,$2,'assistant','上一条还未收到回应','[]','previous-proactive')`, conversation, self); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.conversation_heads SET next_sequence=2 WHERE conversation_id=$1`, conversation); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.conversation_proactive_deliveries(message_id,fluctlight_id,conversation_id,topic_key,purpose,inbound_sequence,occurred_at)
		VALUES('previous-proactive',$1,$2,'previous-topic','share',0,now())`, self, conversation); err != nil {
		t.Fatal(err)
	}
	if _, err := app.EnsureWakeUpIntents(ctx); err != nil {
		t.Fatal(err)
	}
	wake := func(cycle int, want string) {
		t.Helper()
		if _, err := repo.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET status='started',payload=jsonb_set(payload,'{cycle}',to_jsonb($2::integer)),started_at=now() WHERE intent_id=$1`, "wake_up_intent:"+self, cycle); err != nil {
			t.Fatal(err)
		}
		wakeCtx := WithWakeUpCycle(WithLifecycleIntentID(ctx, "wake_up_intent:"+self), cycle)
		result, err := app.ProcessWakeUp(wakeCtx, self, cycle)
		if err != nil || result["action_type"] != want {
			t.Fatalf("controlled WakeUp cycle %d result=%#v err=%v", cycle, result, err)
		}
	}
	wake(1, "no_op")
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.life_events(id,fluctlight_id,kind,start_at,end_at,scene,activity,location,status,revision,evidence_refs,idempotency_key,request_digest,result)
		VALUES('wake-new-event',$1,'owner_event',now()-interval '1 minute',now()+interval '15 minutes','独立测试新场景','发现一朵新花','花园','confirmed',1,'["fixture"]','wake-new-event',$2,$3)`, self, stableDigest("wake-new-event"), jsonBytes(map[string]any{"id": "wake-new-event", "status": "confirmed", "revision": 1, "expected_context_revision": "before", "resulting_context_revision": "after", "replayed": false})); err != nil {
		t.Fatal(err)
	}
	wake(2, "no_op")
	if len(requests) != 2 {
		t.Fatalf("physical WakeUp requests=%d", len(requests))
	}
	for index, payload := range requests {
		var contents []string
		for _, raw := range arrayValue(payload["messages"]) {
			contents = append(contents, stringValue(mapValue(raw)["content"]))
		}
		messages := strings.Join(contents, "\n")
		for _, required := range []string{`"awaiting_response":true`, `"has_new_inbound_message":false`, "previous-topic"} {
			if !strings.Contains(messages, required) {
				t.Errorf("cycle %d missing actual communication fact %q", index+1, required)
			}
		}
		if strings.Contains(messages, "独立测试新场景") != (index == 1) || strings.Contains(messages, "发现一朵新花") != (index == 1) {
			t.Errorf("cycle %d lost or invented the current event", index+1)
		}
	}
	var count int
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant'`, conversation).Scan(&count); err != nil || count != 1 {
		t.Fatalf("scripted silence published another message: count=%d err=%v", count, err)
	}
	publish = true
	wake(3, "proactive_message")
	var shared int
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_proactive_deliveries d JOIN public.conversation_messages m ON m.id=d.message_id WHERE d.conversation_id=$1 AND d.topic_key='new-flower' AND m.text='我刚看到一朵新花。'`, conversation).Scan(&shared); err != nil || shared != 1 {
		t.Fatalf("new event sharing was suppressed or duplicated: count=%d err=%v", shared, err)
	}
}
