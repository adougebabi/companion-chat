package core

import (
	"strings"
	"testing"
	"time"
)

func TestTicketCorrectionKeepsBothUtterancesAndDoesNotConfirmPurchase(t *testing.T) {
	ctx, repo, app, owner, fid, conversation := durableReplyFixture(t, "ticket-correction")
	firstText := "ticket 我买了票。"
	secondText := "ticket 刚才是开玩笑，不能据此确认购票。"
	first, err := app.AcceptTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": fid, "text": firstText, "turn_id": "ticket-first", "idempotency_key": "ticket-first", "attachment_refs": []any{}})
	if err != nil {
		t.Fatal(err)
	}
	command := PreparedMemoryMutation{SchemaVersion: memoryLifecycleSchemaVersion, Operation: MemoryCreate, OwnerFluctlightID: fid, OwnerActorID: owner, ActorID: fid, ConversationID: conversation, ActorRefs: []string{owner}, EventRefs: []string{}, EvidenceRefs: []string{"message:" + stringValue(first.UserMessage["id"])}, Visibility: "private", OccurredAt: time.Now().UTC(), SourceFactID: first.InboxID, SemanticReason: "historical incorrect interpretation", IdempotencyKey: "ticket-old-memory", Semantic: &MemorySemanticInput{Type: "semantic", Content: "OLD_TICKET_ASSERTION 用户确已购票", Confidence: 0.8, Importance: 0.9}}
	command.RequestDigest = memoryCommandDigest(command)
	created := applyMemoryLifecycleTestCommand(t, ctx, app, command)
	second, err := app.AcceptTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": fid, "text": secondText, "turn_id": "ticket-second", "idempotency_key": "ticket-second", "attachment_refs": []any{}})
	if err != nil {
		t.Fatal(err)
	}
	revision := created.Revision
	if _, err := app.ReviseMemory(ctx, owner, created.MemoryID, "TICKET_PURCHASE_UNKNOWN 用户撤回了购票自述，购票事实未知", &revision, []any{"message:" + stringValue(second.UserMessage["id"])}); err != nil {
		t.Fatal(err)
	}
	receipt, err := app.ExecuteTool(ctx, ToolExecutionRequest{CapabilityName: "memory.recall", OperationID: "ticket-after-correction", AuthorizationActorID: owner, SubjectActorID: owner, FluctlightID: fid, ConversationID: conversation, Surface: CapabilitySurfaceConversation, Arguments: jsonBytes(map[string]any{"intent": "ticket"})})
	if err != nil || receipt.Result.Status != "completed" {
		t.Fatalf("formal recall failed: %#v err=%v", receipt, err)
	}
	seen := map[string]bool{}
	corrected := false
	for _, raw := range arrayValue(mapValue(receipt.Result.Output)["items"]) {
		item := mapValue(raw)
		content := stringValue(item["content"])
		if item["source_kind"] == "conversation_record" && (content == firstText || content == secondText) {
			if item["speaker"] != "current_speaker" || item["validity"] != "historical" || item["message_kind"] != "user" {
				t.Fatalf("actual utterance became an unattributed world fact: %#v", item)
			}
			seen[content] = true
		}
		if item["source_kind"] == "long_term_memory" {
			if strings.Contains(content, "OLD_TICKET_ASSERTION") {
				t.Fatal("corrected purchase assertion survived recall")
			}
			corrected = corrected || strings.Contains(content, "TICKET_PURCHASE_UNKNOWN")
		}
	}
	if len(seen) != 2 || !corrected {
		t.Fatalf("recall lost actual speech or purchase uncertainty: %#v", receipt.Result.Output)
	}
	var original string
	if err := repo.Pool().QueryRow(ctx, `SELECT text FROM public.conversation_messages WHERE id=$1`, stringValue(first.UserMessage["id"])).Scan(&original); err != nil || original != firstText {
		t.Fatalf("correction rewrote the first utterance: %q err=%v", original, err)
	}
}
