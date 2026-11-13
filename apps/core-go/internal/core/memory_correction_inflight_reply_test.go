package core

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// The correction commits while the formal Provider request holds the old
// projection. The Provider is controlled, publication and Memory authority
// use real isolated PostgreSQL transactions.
func TestOwnerMemoryCorrectionFencesInflightDirectAndWriterReplies(t *testing.T) {
	for _, mode := range []string{"direct", "fact_only"} {
		for _, operation := range []string{"revise", "forget"} {
			t.Run(mode+"/"+operation, func(t *testing.T) {
				var correct func() error
				mainCalls, writerCalls, corrections := 0, 0, 0
				router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
					writer := payload["model"] == "inflight-memory-writer"
					if writer {
						writerCalls++
					} else {
						mainCalls++
						if mainCalls > 1 {
							return fakeProviderResult{Status: 400}
						}
					}
					if mode == "direct" || writer {
						if corrections != 0 {
							return fakeProviderResult{Status: 400}
						}
						if !strings.Contains(jsonString(payload["messages"]), "M3_OLD_TICKET_MEMORY") {
							t.Error("request did not consume the old sourced memory")
							return fakeProviderResult{Status: 500}
						}
						if err := correct(); err != nil {
							t.Errorf("owner correction failed while request was in flight: %v", err)
							return fakeProviderResult{Status: 500}
						}
						corrections++
					}
					if writer {
						return fakeProviderResult{Text: "旧判断：票已经买好了。"}
					}
					args := map[string]any{}
					if mode == "direct" {
						args = map[string]any{"text": "旧判断：票已经买好了。", "decision": map[string]any{"action_type": "reply", "response_intent": "回应当前问题", "influences": []any{}}}
					}
					return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall(fmt.Sprintf("inflight-memory-%d", mainCalls), conversationReplyCapabilityName, args)}}
				})
				fixture := "memory-inflight-" + mode + "-" + operation
				ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, fixture, router)
				if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_direct_conversations(owner_actor_id,fluctlight_actor_id,conversation_id) VALUES($1,$2,$3)`, owner, fid, conversation); err != nil {
					t.Fatal(err)
				}
				persistConversationReplyMode(t, ctx, repo, mode)
				if mode == "fact_only" {
					role := agentModelBindingKey(FormalAgentConversationReply, "private_reply")
					if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,timeout_seconds) VALUES($1,$2,'inflight-memory-writer',30)`, role, "mixed-endpoint-"+fixture); err != nil {
						t.Fatal(err)
					}
				}
				accepted, err := app.AcceptTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": fid, "text": "买票情况是什么？", "turn_id": fixture, "idempotency_key": fixture, "attachment_refs": []any{}})
				if err != nil {
					t.Fatal(err)
				}
				command := PreparedMemoryMutation{SchemaVersion: memoryLifecycleSchemaVersion, Operation: MemoryCreate, OwnerFluctlightID: fid, OwnerActorID: owner, ActorID: fid, ActorRefs: []string{owner}, EventRefs: []string{}, EvidenceRefs: []string{"message:" + stringValue(accepted.UserMessage["id"])}, Visibility: "private", OccurredAt: time.Now().UTC(), SourceFactID: accepted.InboxID, SemanticReason: "受控旧记忆来源", IdempotencyKey: fixture + "-memory", Semantic: &MemorySemanticInput{Type: "autobiographical", Content: "M3_OLD_TICKET_MEMORY 用户曾自述已经买票", Confidence: 0.9, Importance: 0.9}}
				command.RequestDigest = memoryCommandDigest(command)
				created := applyMemoryLifecycleTestCommand(t, ctx, app, command)
				correct = func() error {
					correctionCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
					defer cancel()
					revision := created.Revision
					if operation == "revise" {
						_, err := app.ReviseMemory(correctionCtx, owner, created.MemoryID, "买票只是玩笑，是否购票未知", &revision, []any{"owner-confirmation"})
						return err
					}
					_, err := app.ForgetMemory(correctionCtx, owner, created.MemoryID, &revision, []any{"owner-withdrawal"})
					return err
				}
				runCtx, cancelRun := context.WithTimeout(ctx, 30*time.Second)
				defer cancelRun()
				_, runErr := app.ProcessCognitionInbox(runCtx, accepted.InboxID)
				var messages, successfulReceipts int
				var inboxStatus string
				if err := repo.Pool().QueryRow(ctx, `SELECT (SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant'),(SELECT count(*) FROM public.tool_executions WHERE fluctlight_id=$2 AND capability_name='conversation.reply' AND result->>'status'='completed'),(SELECT status FROM public.cognition_inbox WHERE id=$3)`, conversation, fid, accepted.InboxID).Scan(&messages, &successfulReceipts, &inboxStatus); err != nil {
					t.Fatal(err)
				}
				if runErr == nil || corrections != 1 || messages != 0 || successfulReceipts != 0 || inboxStatus == "processed" {
					t.Fatalf("stale memory reply escaped publication fence: err=%v corrections=%d messages=%d receipts=%d inbox=%s main=%d writer=%d", runErr, corrections, messages, successfulReceipts, inboxStatus, mainCalls, writerCalls)
				}
				var staleReceipts int
				if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.tool_executions WHERE fluctlight_id=$1 AND capability_name='conversation.reply' AND result->>'status'='handled' AND result->'output'->>'warning_code'='reply_context_stale'`, fid).Scan(&staleReceipts); err != nil {
					t.Fatal(err)
				}
				if mode == "fact_only" && staleReceipts == 0 {
					t.Fatal("Writer failure did not retain a stale-context rejection receipt")
				}
				if mode == "direct" && !strings.Contains(runErr.Error(), "current_facts_stale") {
					t.Fatalf("direct reply did not fail its facts-generation fence: %v", runErr)
				}
				t.Logf("old reply fenced after committed %s: main=%d writer=%d inbox=%s error=%v", operation, mainCalls, writerCalls, inboxStatus, runErr)
			})
		}
	}
}
