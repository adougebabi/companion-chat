package core

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// Controlled model transport plus a task-owned disposable PostgreSQL database.
// A skipped case is not persistent publication or real-model acceptance.
func TestFactOnlyCommittedReplyMalformedFinalSettlement(t *testing.T) {
	for _, repairValid := range []bool{true, false} {
		t.Run(fmt.Sprint(repairValid), func(t *testing.T) {
			writerCalls, cognitionCalls, repairCalls := 0, 0, 0
			router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
				if payload["model"] == "controlled-settlement-writer" {
					writerCalls++
					return fakeProviderResult{Text: "这是一条真实写入测试数据库的独立回复。"}
				}
				if len(arrayValue(payload["tools"])) == 0 && providerWireSchemaName(payload) == "conversation_turn_response" {
					repairCalls++
					if !payloadHasToolResult(payload) || payload["tool_choice"] != nil || payload["enable_thinking"] != false {
						t.Error("repair lost paired actual Tool result or regained business tools")
					}
					if repairValid {
						return fakeProviderResult{Structured: map[string]any{"action_type": "reply", "response_intent": "settled actual committed reply", "influences": []any{}}}
					}
					return fakeProviderResult{Text: `{"action_type":`}
				}
				cognitionCalls++
				if payloadHasToolResult(payload) {
					return fakeProviderResult{Text: `{"action_type":`}
				}
				return fakeProviderResult{ToolCalls: []map[string]any{{"call_id": "settlement-reply", "capability_name": conversationReplyCapabilityName, "arguments": map[string]any{}}}}
			})
			ctx, repository, app, owner, self, conversation := setupMixedMediaReplyTurnWithTransport(t, "fact-only-settlement-"+fmt.Sprint(repairValid), router)
			text := "合成测试：发布后修复最终认知，不要再发一次。"
			turn, err := app.AcceptTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": self, "text": text, "turn_id": "settlement-turn", "idempotency_key": "settlement-source", "attachment_refs": []any{}})
			if err != nil {
				t.Fatal(err)
			}
			settings := defaultConversationReplySettings()
			settings.Mode = "fact_only"
			settings.RetryLimit = 0
			run := &privateReplyRun{Settings: settings, ConfigVersion: 1, CurrentInput: text,
				Scope:      ToolExecutionRequest{AuthorizationActorID: owner, SubjectActorID: owner, FluctlightID: self, ConversationID: conversation, EvidenceID: turn.InboxID, RunID: "settlement-run", Surface: CapabilitySurfaceConversation},
				Assignment: providerAssignment{Role: conversationReplyModelRole, BaseURL: "http://controlled.invalid", ModelID: "controlled-settlement-writer", Timeout: time.Second, TokenBudget: 1024}}
			ctx = context.WithValue(ctx, privateReplyRunKey{}, run)
			registry, err := app.privateReplyRegistry(run)
			if err != nil {
				t.Fatal(err)
			}
			ctx = withCapabilityRunRegistry(ctx, registry)
			if _, err := app.ProcessCognitionInbox(ctx, turn.InboxID); err != nil {
				t.Fatal(err)
			}
			var count int
			var publishedText, draftStatus, inboxStatus, errorCode string
			if err := repository.Pool().QueryRow(ctx, `SELECT count(*),COALESCE(min(text),'') FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant'`, conversation).Scan(&count, &publishedText); err != nil {
				t.Fatal(err)
			}
			if err := repository.Pool().QueryRow(ctx, `SELECT status FROM public.conversation_reply_drafts WHERE source_fact_id=$1`, turn.InboxID).Scan(&draftStatus); err != nil {
				t.Fatal(err)
			}
			if err := repository.Pool().QueryRow(ctx, `SELECT status,COALESCE(error_code,'') FROM public.cognition_inbox WHERE id=$1`, turn.InboxID).Scan(&inboxStatus, &errorCode); err != nil {
				t.Fatal(err)
			}
			if count != 1 || publishedText != "这是一条真实写入测试数据库的独立回复。" || draftStatus != "published" {
				t.Fatalf("publication changed after final validation: %d/%q/%s", count, publishedText, draftStatus)
			}
			if inboxStatus != "processed" || errorCode != "" {
				t.Fatalf("committed reply did not end the turn: %s/%s", inboxStatus, errorCode)
			}
			if writerCalls != 1 || cognitionCalls != 1 || repairCalls != 0 {
				t.Fatalf("unexpected physical calls writer=%d cognition=%d repair=%d", writerCalls, cognitionCalls, repairCalls)
			}
			if _, err := app.ProcessCognitionInbox(ctx, turn.InboxID); err != nil {
				t.Fatal(err)
			}
			if writerCalls != 1 || cognitionCalls != 1 || repairCalls != 0 {
				t.Fatal("terminal replay regenerated or resent")
			}
			var controls int
			if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND (text LIKE 'Validation:%' OR text LIKE '%Correct the final response contract%')`, conversation).Scan(&controls); err != nil || controls != 0 {
				t.Fatalf("repair instructions leaked into chat: %d/%v", controls, err)
			}
		})
	}
}

func TestFactOnlyHistoryBackfillUsesExactStoredSource(t *testing.T) {
	ctx, _, app, owner, self, conversation := durableReplyFixture(t, "history-backfill")
	turn, err := app.AcceptTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": self, "text": "ORIGINAL_STORED_TRIGGER", "turn_id": "backfill-turn", "idempotency_key": "backfill-user", "attachment_refs": []any{}})
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := app.ExecuteTool(ctx, ToolExecutionRequest{AgentID: FormalAgentConversationCognition, RunID: "backfill-run", CapabilityName: conversationReplyCapabilityName, OperationID: "backfill-reply", AuthorizationActorID: owner, FluctlightID: self, ConversationID: conversation, CorrelationID: "turn:" + turn.TurnID, EvidenceID: turn.InboxID, Surface: CapabilitySurfaceConversation, Arguments: jsonBytes(map[string]any{"text": "REAL_STORED_ASSISTANT"})})
	if err != nil {
		t.Fatal(err)
	}
	id := stringValue(mapValue(receipt.Result.Output)["target_ref"])
	projection := ContextProjection{FluctlightID: self, ConversationID: conversation, RecentMessages: []map[string]any{{"id": id, "kind": "assistant", "text": "REAL_STORED_ASSISTANT"}}}
	foreign := projection
	foreign.ConversationID = "foreign-conversation"
	if err := app.backfillFactOnlyHistory(ctx, &foreign); err != nil || len(foreign.RecentMessages) != 1 {
		t.Fatalf("cross-conversation backfill: %#v/%v", foreign.RecentMessages, err)
	}
	if err := app.backfillFactOnlyHistory(ctx, &projection); err != nil {
		t.Fatal(err)
	}
	if len(projection.RecentMessages) != 2 || projection.RecentMessages[0]["kind"] != "user" || projection.RecentMessages[0]["text"] != "ORIGINAL_STORED_TRIGGER" || projection.RecentMessages[1]["id"] != id {
		t.Fatal("real trigger not restored", projection.RecentMessages)
	}
}
