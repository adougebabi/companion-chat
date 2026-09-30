package core

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestConversationReplyPublishesAtMostOncePerTurnAcrossOperations(t *testing.T) {
	ctx, repository, app, ownerID, fluctlightID, conversationID := durableReplyFixture(t, "reply-once")
	turn, err := app.AcceptTurn(ctx, ownerID, conversationID, map[string]any{
		"fluctlight_id": fluctlightID, "text": "回复一次", "turn_id": "reply-once-turn", "idempotency_key": "reply-once-user", "attachment_refs": []any{},
	})
	if err != nil {
		t.Fatal(err)
	}
	request := ToolExecutionRequest{
		AgentID: FormalAgentConversationCognition, RunID: "reply-once-run", CapabilityName: conversationReplyCapabilityName,
		AuthorizationActorID: ownerID, FluctlightID: fluctlightID, ConversationID: conversationID,
		CorrelationID: "turn:" + turn.TurnID, EvidenceID: turn.InboxID, Surface: CapabilitySurfaceConversation,
	}
	request.OperationID = "reply-once-first"
	request.Arguments = json.RawMessage(`{"text":"第一条"}`)
	first, err := app.ExecuteTool(ctx, request)
	if err != nil || first.Result.Status != "completed" {
		t.Fatalf("first reply: receipt=%#v err=%v", first, err)
	}
	request.OperationID = "reply-once-same-text-other-operation"
	second, err := app.ExecuteTool(ctx, request)
	if err != nil || second.Result.Status != "completed" || !boolValue(mapValue(second.Result.Output)["replayed"]) {
		t.Fatalf("same-turn same-text replay: receipt=%#v err=%v", second, err)
	}
	request.OperationID = "reply-once-different-text"
	request.Arguments = json.RawMessage(`{"text":"第二条"}`)
	third, err := app.ExecuteTool(ctx, request)
	if !errors.Is(err, ErrReplyAlreadyPublished) || third.Result.ErrorCode != "reply_already_published" {
		t.Fatalf("second distinct reply should be a typed conflict: receipt=%#v err=%v", third, err)
	}
	var count int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND turn_id=$2 AND kind='assistant'`, conversationID, turn.TurnID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("assistant messages for turn = %d, err=%v", count, err)
	}
}
