package core

import (
	"testing"
	"time"
)

func TestProactiveSemanticTopicSuppressionUsesPurposeInboundAndBusinessWindow(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	at := time.Now().UTC().Add(24 * time.Hour)
	f.app.Clock = func() time.Time { return at }
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.runtime_settings(key,value_json) VALUES('product.autonomy','{"topic_suppression_seconds":300}')`); err != nil {
		t.Fatal(err)
	}
	send := func(op, text, purpose string) ToolExecutionReceipt {
		t.Helper()
		request := f.request(conversationReplyCapabilityName, op, map[string]any{"text": text, "topic_key": "daily-hydration", "purpose": purpose})
		request.AuthorizationPolicy = "autonomy"
		receipt, err := f.app.ExecuteTool(f.ctx, request)
		if err != nil {
			t.Fatal(err)
		}
		return receipt
	}
	first := send("topic-first", "记得喝水呀。", "gentle-reminder")
	second := send("topic-paraphrase", "杯子里的水喝了吗？", "gentle-reminder")
	if mapValue(second.Result.Output)["delivery_status"] != "duplicate_suppressed" || mapValue(second.Result.Output)["target_ref"] != mapValue(first.Result.Output)["target_ref"] {
		t.Fatal("paraphrase repeated unchanged topic", second)
	}
	third := send("topic-new-purpose", "今天想试试新茶吗？", "ask-preference")
	if mapValue(third.Result.Output)["delivery_status"] != nil {
		t.Fatal("new purpose blocked", third)
	}
	actorFactSourceMessage(t, f, "topic-inbound", f.ownerID, "我喝过了")
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.conversation_heads SET next_sequence=(SELECT max(sequence)+1 FROM public.conversation_messages WHERE conversation_id=$1) WHERE conversation_id=$1`, f.conversationID); err != nil {
		t.Fatal(err)
	}
	fourth := send("topic-response", "收到，稍后补充一点就好。", "gentle-reminder")
	if mapValue(fourth.Result.Output)["delivery_status"] != nil {
		t.Fatal("new actual inbound ignored", fourth)
	}
	at = at.Add(6 * time.Minute)
	fifth := send("topic-later", "休息一下，补充点水分吧。", "gentle-reminder")
	if mapValue(fifth.Result.Output)["delivery_status"] != nil {
		t.Fatal("business window did not expire", fifth)
	}
	state, err := readCommunicationStateWith(f.ctx, f.repository.Pool(), f.fluctlightID, f.conversationID)
	if err != nil || state["awaiting_response"] != true || len(arrayValue(state["recent_proactive"])) != 4 {
		t.Fatalf("bounded communication %#v %v", state, err)
	}
}
