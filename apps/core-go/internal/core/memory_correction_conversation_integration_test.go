package core

import (
	"strings"
	"testing"
	"time"
)

func TestMemoryCorrectionPreservesStatementsWithoutRevivingTicketClaim(t *testing.T) {
	ctx, repo, app, owner, fid, conversation := setupMixedMediaReplyTurnWithTransport(t, "ticket-correction", nil)
	accept := func(id, text string) TurnResult {
		t.Helper()
		result, err := app.AcceptTurn(ctx, owner, conversation, map[string]any{"fluctlight_id": fid, "text": text, "turn_id": id, "idempotency_key": id, "attachment_refs": []any{}})
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	first := accept("ticket-first", "I bought a ticket")
	firstID := stringValue(first.UserMessage["id"])
	command := PreparedMemoryMutation{SchemaVersion: memoryLifecycleSchemaVersion, Operation: MemoryCreate, OwnerFluctlightID: fid, OwnerActorID: owner, ActorID: fid,
		ConversationID: conversation, ActorRefs: []string{}, EventRefs: []string{}, EvidenceRefs: []string{"message:" + firstID}, Visibility: "private", OccurredAt: time.Now().UTC(), SourceFactID: first.InboxID,
		SemanticReason: "old inferred conclusion", IdempotencyKey: "ticket-old-memory", Semantic: &MemorySemanticInput{Type: "semantic", Content: "ticket was purchased", Confidence: 0.9, Importance: 0.8}}
	command.RequestDigest = memoryCommandDigest(command)
	old := applyMemoryLifecycleTestCommand(t, ctx, app, command)
	second := accept("ticket-correction", "the ticket statement was a joke")
	secondID := stringValue(second.UserMessage["id"])
	revision := old.Revision
	if _, err := app.ReviseMemory(ctx, owner, old.MemoryID, "ticket statement was a joke; purchase is unknown", &revision, []any{"message:" + secondID}); err != nil {
		t.Fatal(err)
	}
	receipt, err := app.ExecuteTool(ctx, ToolExecutionRequest{CapabilityName: "memory.recall", OperationID: "ticket-after-correction", AuthorizationActorID: owner, SubjectActorID: owner,
		FluctlightID: fid, ConversationID: conversation, Surface: CapabilitySurfaceConversation, Arguments: jsonBytes(map[string]any{"intent": "ticket"})})
	if err != nil || receipt.Result.Status != "completed" {
		t.Fatalf("recall after correction=%#v err=%v", receipt.Result, err)
	}
	statements := map[string]bool{}
	corrected := false
	for _, raw := range arrayValue(decodeObject(jsonBytes(receipt.Result.Output))["items"]) {
		item := mapValue(raw)
		content := stringValue(item["content"])
		switch item["source_kind"] {
		case "conversation_record":
			if item["speaker"] != "current_speaker" || item["validity"] != "historical" {
				t.Fatalf("original statement lost its source: %#v", item)
			}
			statements[content] = true
		case "long_term_memory", "episode_memory":
			if content == "ticket was purchased" {
				t.Fatalf("corrected purchase returned as a memory fact: %#v", item)
			}
			corrected = corrected || strings.Contains(content, "purchase is unknown")
		}
	}
	if !statements["I bought a ticket"] || !statements["the ticket statement was a joke"] || !corrected {
		t.Fatalf("history or corrected uncertainty was lost: statements=%#v corrected=%t", statements, corrected)
	}
	var originals int
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_messages WHERE id=ANY($1::text[]) AND kind='user'`, []string{firstID, secondID}).Scan(&originals); err != nil || originals != 2 {
		t.Fatalf("original conversation was rewritten: count=%d err=%v", originals, err)
	}
}
