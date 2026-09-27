package core

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestMessageTimeValidatesFrozenTimezoneOffsetAndLegacyAbsence(t *testing.T) {
	if legacy, err := parseMessageTime(map[string]any{}); err != nil || legacy.zone != nil {
		t.Fatalf("old client should retain unknown sender timezone: %#v %v", legacy, err)
	}
	valid := map[string]any{"sender_timezone": "America/New_York", "sender_utc_offset_minutes": -240, "sender_sent_at": "2026-07-01T12:00:00Z"}
	snapshot, err := parseMessageTime(valid)
	if err != nil || snapshot.zone == nil || *snapshot.offset != -240 || !snapshot.sentAt.Equal(time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC)) {
		t.Fatalf("valid summer snapshot rejected: %#v %v", snapshot, err)
	}
	if _, err := parseMessageTime(map[string]any{"sender_timezone": "America/New_York"}); err == nil {
		t.Fatal("partial snapshot accepted")
	}
	valid["sender_utc_offset_minutes"] = -300
	if _, err := parseMessageTime(valid); err == nil {
		t.Fatal("mismatched historical offset accepted")
	}
}

func TestPostgresConversationSenderTimeSurvivesReplayHistoryAndPersonaTimezoneChange(t *testing.T) {
	ctx, repository, app, ownerID, fluctlightID, conversationID := setupDirectPublicationToolTest(t, "sender-time")
	payload := map[string]any{"fluctlight_id": fluctlightID, "text": "我从纽约发消息", "idempotency_key": "sender-time-turn", "turn_id": "sender-time-turn", "attachment_refs": []any{},
		"sender_timezone": "America/New_York", "sender_utc_offset_minutes": -240, "sender_sent_at": "2026-07-01T12:00:00Z"}
	accepted, err := app.AcceptTurn(ctx, ownerID, conversationID, payload)
	if err != nil || accepted.UserMessage["sender_timezone"] == nil {
		t.Fatalf("accept sender time: message=%#v err=%v", accepted.UserMessage, err)
	}
	replayed, err := app.AcceptTurn(ctx, ownerID, conversationID, payload)
	if err != nil || replayed.UserMessage["sender_sent_at"] != accepted.UserMessage["sender_sent_at"] {
		t.Fatalf("replay changed sender time: message=%#v err=%v", replayed.UserMessage, err)
	}
	changed := cloneMap(payload)
	changed["sender_sent_at"] = "2026-07-01T12:01:00Z"
	if _, err := app.AcceptTurn(ctx, ownerID, conversationID, changed); !errors.Is(err, ErrConflict) {
		t.Fatalf("changed retry snapshot was accepted: %v", err)
	}
	first, err := app.ExecuteTool(ctx, ToolExecutionRequest{CapabilityName: "conversation.reply", OperationID: "sender-time-first", AuthorizationActorID: ownerID, FluctlightID: fluctlightID, ConversationID: conversationID, Arguments: json.RawMessage(`{"text":"上海时区回复"}`)})
	if err != nil || first.Result.Status != "completed" {
		t.Fatalf("first reply: %#v %v", first, err)
	}
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.fluctlights SET identity=jsonb_set(identity,'{timezone}','"UTC"'::jsonb) WHERE id=$1`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	second, err := app.ExecuteTool(ctx, ToolExecutionRequest{CapabilityName: "conversation.reply", OperationID: "sender-time-second", AuthorizationActorID: ownerID, FluctlightID: fluctlightID, ConversationID: conversationID, Arguments: json.RawMessage(`{"text":"UTC 时区回复"}`)})
	if err != nil || second.Result.Status != "completed" {
		t.Fatalf("second reply: %#v %v", second, err)
	}
	page, err := repository.History(ctx, conversationID, ownerID, nil, 20)
	if err != nil || len(page.Messages) != 3 {
		t.Fatalf("history: %#v %v", page, err)
	}
	if page.Messages[0].SenderTimezone == nil || *page.Messages[0].SenderTimezone != "America/New_York" || page.Messages[0].SenderUTCOffsetMinutes == nil || *page.Messages[0].SenderUTCOffsetMinutes != -240 {
		t.Fatalf("user sender provenance lost: %#v", page.Messages[0])
	}
	if page.Messages[1].SenderTimezone == nil || *page.Messages[1].SenderTimezone != "Asia/Shanghai" || page.Messages[2].SenderTimezone == nil || *page.Messages[2].SenderTimezone != "UTC" {
		t.Fatalf("assistant timezone snapshots drifted: %#v", page.Messages)
	}
}
