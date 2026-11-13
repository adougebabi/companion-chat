package core

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func waitForMemoryLockWaiter(t *testing.T, ctx context.Context, repo *PostgresRepository, blocker uint32) uint32 {
	t.Helper()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var pid uint32
		err := repo.Pool().QueryRow(ctx, `SELECT pid FROM pg_stat_activity WHERE datname=current_database() AND pid<>pg_backend_pid() AND $1::integer=ANY(pg_blocking_pids(pid)) ORDER BY pid LIMIT 1`, blocker).Scan(&pid)
		if err == nil {
			return pid
		}
		if err != pgx.ErrNoRows {
			t.Fatal(err)
		}
		select {
		case <-ctx.Done():
			t.Fatalf("no observed waiter for backend %d: %v", blocker, ctx.Err())
		case <-ticker.C:
		}
	}
}

func TestOwnerMemoryCorrectionAndSegmentReplacementDoNotDeadlock(t *testing.T) {
	base, repo, app, intentID, _ := seedConversationSegmentSource(t)
	ctx, cancel := context.WithTimeout(base, 15*time.Second)
	defer cancel()
	app.Provider = &ProviderClient{DB: repo, HTTP: &http.Client{Transport: newFakeProviderRouter().otherwise(func(map[string]any) fakeProviderResult {
		return fakeProviderResult{Structured: map[string]any{"schema_version": conversationSegmentSchemaVersion, "summary": "旧摘要结论", "ending_state": "稍后继续", "open_threads": []any{}, "core_events": []any{}}}
	})}}
	first, err := app.ProcessConversationSegmentIntent(ctx, intentID)
	if err != nil || first["status"] != "active" {
		t.Fatalf("initial formal segment: %#v err=%v", first, err)
	}
	owner, fid, conversation := "summary-owner", "summary-fluctlight", "summary-conversation"
	command := PreparedMemoryMutation{SchemaVersion: memoryLifecycleSchemaVersion, Operation: MemoryCreate, OwnerFluctlightID: fid, OwnerActorID: owner, ActorID: fid, ConversationID: conversation, ActorRefs: []string{}, EventRefs: []string{}, EvidenceRefs: []string{"message:summary-message-1"}, Visibility: "private", OccurredAt: time.Now().UTC(), SourceFactID: "summary-message-1", SemanticReason: "original sourced conclusion", IdempotencyKey: "lock-order-memory", Semantic: &MemorySemanticInput{Type: "semantic", Content: "旧结论", Confidence: 0.9, Importance: 0.9}}
	command.RequestDigest = memoryCommandDigest(command)
	created := applyMemoryLifecycleTestCommand(t, ctx, app, command)
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET status='pending' WHERE intent_id=$1`, intentID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `CREATE FUNCTION public.test_hold_segment_replacement() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
		 IF OLD.status='active' AND NEW.status='superseded' THEN PERFORM pg_advisory_xact_lock(938701); END IF;
		 RETURN NEW; END $$;
		CREATE TRIGGER test_hold_segment_replacement BEFORE UPDATE ON public.conversation_summaries FOR EACH ROW EXECUTE FUNCTION public.test_hold_segment_replacement()`); err != nil {
		t.Fatal(err)
	}
	gate, err := repo.Pool().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer gate.Rollback(context.Background())
	if _, err := gate.Exec(ctx, `SELECT pg_advisory_xact_lock(938701)`); err != nil {
		t.Fatal(err)
	}
	segmentDone := make(chan error, 1)
	go func() { _, err := app.ProcessConversationSegmentIntent(ctx, intentID); segmentDone <- err }()
	segmentPID := waitForMemoryLockWaiter(t, ctx, repo, gate.Conn().PgConn().PID())
	ownerDone := make(chan error, 1)
	go func() {
		revision := created.Revision
		_, err := app.ReviseMemory(ctx, owner, created.MemoryID, "Owner 更正结论", &revision, []any{"message:summary-message-1", "owner:lock-order-correction"})
		ownerDone <- err
	}()
	ownerPID := waitForMemoryLockWaiter(t, ctx, repo, segmentPID)
	if err := gate.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	segmentErr, ownerErr := <-segmentDone, <-ownerDone
	if segmentErr != nil || ownerErr != nil {
		t.Fatalf("formal segment/Owner lock cycle: segment_pid=%d owner_pid=%d segment=%v owner=%v", segmentPID, ownerPID, segmentErr, ownerErr)
	}
	var content string
	if err := repo.Pool().QueryRow(ctx, `SELECT content FROM public.memories WHERE id=$1`, created.MemoryID).Scan(&content); err != nil || content != "Owner 更正结论" {
		t.Fatalf("correction lost: %q err=%v", content, err)
	}
	visible, err := app.retrieveConversationSummaries(ctx, ConversationSummaryQuery{AuthorizationActorID: owner, FluctlightID: fid, ConversationID: conversation, Limit: 4, MaxRunes: 4000})
	if err != nil || len(visible.Items) != 0 {
		t.Fatalf("replacement resurrected corrected summary: %#v err=%v", visible.Items, err)
	}
}

func TestOwnerMemoryCorrectionAndPublicationGenerationDoNotDeadlock(t *testing.T) {
	base, repo, app, owner, fid, conversation := durableReplyFixture(t, "memory-journal-lock")
	ctx, cancel := context.WithTimeout(base, 15*time.Second)
	defer cancel()
	created := createLegacyUnknownMemoryForTest(t, ctx, repo, app, owner, fid, "memory-journal-lock", "可被 Owner 管理的旧记录")
	facts, err := app.readCurrentFactsRevision(ctx, fid)
	if err != nil {
		t.Fatal(err)
	}
	publication, err := repo.Pool().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer publication.Rollback(context.Background())
	if err := app.requireCurrentFactsRevisionTx(ctx, publication, fid, facts); err != nil {
		t.Fatal(err)
	}
	ownerDone := make(chan error, 1)
	go func() {
		revision := created.Revision
		_, err := app.ReviseMemory(ctx, owner, created.MemoryID, "Owner 已确认的新结论", &revision, []any{"owner:journal-lock-correction"})
		ownerDone <- err
	}()
	ownerPID := waitForMemoryLockWaiter(t, ctx, repo, publication.Conn().PgConn().PID())
	resource, publicationErr := NewToolPublicationService(app).PublishConversationReplyTx(ctx, publication, ConversationReplyPublication{AuthorizationActorID: owner, FluctlightID: fid, ConversationID: conversation, OperationID: "memory-journal-lock-reply", Text: "正常交流仍然可用。"})
	if publicationErr == nil {
		publicationErr = publication.Commit(ctx)
	} else {
		_ = publication.Rollback(ctx)
	}
	ownerErr := <-ownerDone
	if publicationErr != nil || ownerErr != nil || resource.ID == "" {
		t.Fatalf("formal publication/Owner generation cycle: owner_pid=%d publication=%v owner=%v resource=%#v", ownerPID, publicationErr, ownerErr, resource)
	}
	var messages int
	var content string
	if err := repo.Pool().QueryRow(ctx, `SELECT (SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1 AND kind='assistant'),(SELECT content FROM public.memories WHERE id=$2)`, conversation, created.MemoryID).Scan(&messages, &content); err != nil || messages != 1 || content != "Owner 已确认的新结论" {
		t.Fatalf("concurrent operations lost or duplicated: messages=%d content=%q err=%v", messages, content, err)
	}
}
