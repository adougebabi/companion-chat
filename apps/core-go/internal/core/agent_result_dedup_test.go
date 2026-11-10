package core

import (
	"reflect"
	"testing"
)

func TestCommittedReplyResultsDeduplicateMessageEventsWithoutDiscardingReceipts(t *testing.T) {
	receipt := func(callID, status, kind, ref string, replayed bool) CapabilityResult {
		return CapabilityResult{CallID: callID, CapabilityName: conversationReplyCapabilityName, Status: status, Output: map[string]any{"target_kind": kind, "target_ref": ref, "replayed": replayed}}
	}
	results := []CapabilityResult{
		receipt("first", "completed", "conversation_message", "message-a", false),
		receipt("replay", "completed", "conversation_message", " message-a ", true),
		receipt("second", "completed", "conversation_message", "message-b", false),
		receipt("failed", "failed", "conversation_message", "failed", false),
		receipt("waiting", "accepted", "conversation_message", "waiting", false),
		receipt("wrong-kind", "completed", "moment", "foreign", false),
		receipt("empty", "completed", "conversation_message", " ", false),
	}
	replies := committedConversationReplyResults(results)
	if len(replies) != 2 || replies[0].CallID != "first" || replies[1].CallID != "second" {
		t.Fatalf("unexpected committed messages: %#v", replies)
	}
	if len(results) != 7 || results[1].CallID != "replay" || mapValue(results[1].Output)["replayed"] != true {
		t.Fatalf("original receipts were changed: %#v", results)
	}
	var chunks []string
	var events []string
	messages := []map[string]any{}
	for _, reply := range replies {
		id := stringValue(mapValue(reply.Output)["target_ref"])
		messages = append(messages, map[string]any{"id": id, "text": id + " text"})
	}
	err := emitCommittedAssistantMessages(turnCallbacks{
		onChunk: func(chunk string) error { chunks = append(chunks, chunk); return nil },
		onActionResult: func(event map[string]any) error {
			events = append(events, stringValue(mapValue(event["message"])["id"]))
			return nil
		},
	}, messages, "turn:test")
	if err != nil || !reflect.DeepEqual(chunks, []string{"message-a text", "message-b text"}) || !reflect.DeepEqual(events, []string{"message-a", "message-b"}) {
		t.Fatalf("duplicate or missing events: chunks=%v events=%v err=%v", chunks, events, err)
	}
}

func TestReplayAgentResultDeduplicatesHistoricalMessageIDs(t *testing.T) {
	for name, test := range map[string]struct {
		result map[string]any
		want   []string
	}{
		"historical duplicates": {map[string]any{"message_ids": []any{"message-a", " message-a ", "message-b", "", "message-b"}, "message_id": "old-single"}, []string{"message-a", "message-b"}},
		"legacy single":         {map[string]any{"message_id": "message-c"}, []string{"message-c"}},
		"empty IDs fall back":   {map[string]any{"message_ids": []any{"", " "}, "message_id": "message-d"}, []string{"message-d"}},
	} {
		t.Run(name, func(t *testing.T) {
			if got := replayAgentResultMessageIDs(test.result); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("IDs = %v, want %v", got, test.want)
			}
		})
	}
}

func TestWakeUpReplyDeduplicationPreservesActualDelivery(t *testing.T) {
	suppressed := CapabilityResult{CallID: "suppressed", CapabilityName: conversationReplyCapabilityName, Status: "completed", Output: map[string]any{"target_kind": "conversation_message", "target_ref": "message-a", "delivery_status": "duplicate_suppressed"}}
	actual := CapabilityResult{CallID: "actual", CapabilityName: conversationReplyCapabilityName, Status: "completed", Output: map[string]any{"target_kind": "conversation_message", "target_ref": "message-a", "delivery_status": "visible_output_committed"}}
	replay := CapabilityResult{CallID: "replay", CapabilityName: conversationReplyCapabilityName, Status: "completed", Output: map[string]any{"target_kind": "conversation_message", "target_ref": "message-a", "delivery_status": "visible_output_committed"}}
	got := wakeUpPublishedReplyResults([]CapabilityResult{suppressed, actual, replay})
	if len(got) != 1 || got[0].CallID != "actual" {
		t.Fatalf("suppression hid delivery or replay duplicated it: %#v", got)
	}
}

func TestReplayAgentTurnDeduplicatesPersistedMessageIDs(t *testing.T) {
	ctx, repository, app, owner, fid, conversation := durableReplyFixture(t, "reply-replay-dedup")
	accepted, err := app.AcceptTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": fid, "text": "请回答", "turn_id": "reply-dedup-turn", "idempotency_key": "reply-dedup-user"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.conversation_messages(id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key,turn_id) VALUES('reply-dedup-message',$1,2,$2,'assistant','已经回答','[]','reply-dedup-assistant',$3)`, conversation, fid, accepted.TurnID); err != nil {
		t.Fatal(err)
	}
	prior := map[string]any{"message_ids": []any{"reply-dedup-message", "reply-dedup-message"}, "message_id": "reply-dedup-message", "capability_results": []any{map[string]any{"call_id": "first"}, map[string]any{"call_id": "replay"}}}
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.cognition_inbox SET status='processed',payload=jsonb_set(payload,'{agent_result}',$2::jsonb,true) WHERE id=$1`, accepted.InboxID, jsonBytes(prior)); err != nil {
		t.Fatal(err)
	}
	turn, messages, err := app.replayCommittedAgentTurn(ctx, accepted.UserMessage, accepted.InboxID, fid, conversation, accepted.TurnID)
	if err != nil || turn == nil || len(messages) != 1 || stringValue(messages[0]["id"]) != "reply-dedup-message" || stringValue(turn.Assistant["id"]) != "reply-dedup-message" {
		t.Fatalf("replay = %#v messages=%#v err=%v", turn, messages, err)
	}
	var saved []byte
	if err := repository.Pool().QueryRow(ctx, `SELECT payload->'agent_result' FROM public.cognition_inbox WHERE id=$1`, accepted.InboxID).Scan(&saved); err != nil {
		t.Fatal(err)
	}
	if len(arrayValue(decodeObject(saved)["capability_results"])) != 2 {
		t.Fatal("replay rewrote the original tool receipts")
	}
}
