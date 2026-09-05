package core

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRecentExactAssistantMessageTxSuppressesOnlyRecentExactText(t *testing.T) {
	databaseURL := os.Getenv("GO_CORE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("GO_CORE_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	suffix := stableDigest(t.Name() + time.Now().UTC().String())
	conversationID := "test-proactive-dedupe-conversation-" + suffix
	oldConversationID := "test-proactive-dedupe-old-" + suffix
	actorID := "test-proactive-dedupe-actor-" + suffix
	recentMessageID := "test-proactive-dedupe-message-" + suffix
	oldMessageID := "test-proactive-dedupe-old-message-" + suffix
	text := "我已经起床了。"
	now := time.Now().UTC()

	_, err = pool.Exec(ctx, `
		INSERT INTO public.conversation_heads(conversation_id,next_sequence)
		VALUES($1,2),($2,2)`, conversationID, oldConversationID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO public.conversation_messages(
			id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key,created_at
		) VALUES($1,$2,1,$3,'assistant',$4,'[]',$5,$6),
		         ($7,$8,1,$3,'assistant',$4,'[]',$9,$10)`,
		recentMessageID, conversationID, actorID, text, "recent:"+suffix, now.Add(-time.Hour),
		oldMessageID, oldConversationID, "old:"+suffix, now.Add(-13*time.Hour))
	if err != nil {
		_, _ = pool.Exec(ctx, `DELETE FROM public.conversation_heads WHERE conversation_id IN ($1,$2)`, conversationID, oldConversationID)
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM public.conversation_messages WHERE id IN ($1,$2)`, recentMessageID, oldMessageID)
		_, _ = pool.Exec(ctx, `DELETE FROM public.conversation_heads WHERE conversation_id IN ($1,$2)`, conversationID, oldConversationID)
	})

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	gotID, found, err := recentExactAssistantMessageTx(ctx, tx, conversationID, actorID, text, proactiveMessageDuplicateWindow)
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if !found || gotID != recentMessageID {
		_ = tx.Rollback(ctx)
		t.Fatalf("recent duplicate = (%q,%t), want (%q,true)", gotID, found, recentMessageID)
	}
	if _, found, err := recentExactAssistantMessageTx(ctx, tx, conversationID, actorID, "另一条消息。", proactiveMessageDuplicateWindow); err != nil || found {
		_ = tx.Rollback(ctx)
		t.Fatalf("different text duplicate = (%t,%v), want (false,nil)", found, err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}

	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, found, err = recentExactAssistantMessageTx(ctx, tx, oldConversationID, actorID, text, proactiveMessageDuplicateWindow)
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if found {
		_ = tx.Rollback(ctx)
		t.Fatal("message outside duplicate window was suppressed")
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
}
