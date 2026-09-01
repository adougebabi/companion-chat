package core

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestReleaseCognitionClaimRequeuesOwnedClaim(t *testing.T) {
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

	inboxID := "test-cognition-claim-" + stableDigest(time.Now().UTC().String())
	fluctlightID := "test-fluctlight-" + stableDigest(inboxID)
	conversationID := "test-conversation-" + stableDigest(inboxID)
	turnID := "test-turn-" + stableDigest(inboxID)
	idempotencyKey := "test-idempotency-" + stableDigest(inboxID)
	claimOwner := "go-cognition:test-owner"

	_, err = pool.Exec(ctx, `
		INSERT INTO public.cognition_inbox(
			id,fluctlight_id,sequence,event_type,payload,causation_id,correlation_id,
			idempotency_key,occurred_at,status,claimed_by,claimed_at
		) VALUES($1,$2,1,'conversation.turn',$3,$4,$5,$6,now(),'claimed',$7,now())`,
		inboxID,
		fluctlightID,
		jsonBytes(map[string]any{"conversation_id": conversationID, "turn_id": turnID, "text": "test"}),
		turnID,
		"turn:"+turnID,
		idempotencyKey,
		claimOwner,
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM public.cognition_inbox WHERE id=$1`, inboxID)
	})

	app := &App{DB: &PostgresRepository{pool: pool}}
	if err := app.releaseCognitionClaim(ctx, inboxID, claimOwner); err != nil {
		t.Fatal(err)
	}

	var status string
	var claimedBy *string
	var claimedAt *time.Time
	if err := pool.QueryRow(ctx, `SELECT status,claimed_by,claimed_at FROM public.cognition_inbox WHERE id=$1`, inboxID).Scan(&status, &claimedBy, &claimedAt); err != nil {
		t.Fatal(err)
	}
	if status != "pending" || claimedBy != nil || claimedAt != nil {
		t.Fatalf("released claim = status %q, claimed_by %v, claimed_at %v; want pending and empty lease", status, claimedBy, claimedAt)
	}
}

func TestEnqueueTurnFactClaimedReplaysCommittedAssistant(t *testing.T) {
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

	inboxID := "test-cognition-claim-assistant-" + stableDigest(time.Now().UTC().String())
	fluctlightID := "test-fluctlight-" + stableDigest(inboxID)
	conversationID := "test-conversation-" + stableDigest(inboxID)
	turnID := "test-turn-" + stableDigest(inboxID)
	idempotencyKey := "test-idempotency-" + stableDigest(inboxID)
	actorID := "test-owner-" + stableDigest(inboxID)
	claimOwner := "go-stream:test-owner"
	assistantID := "test-assistant-" + stableDigest(inboxID)

	_, err = pool.Exec(ctx, `
		INSERT INTO public.cognition_inbox(
			id,fluctlight_id,sequence,event_type,payload,causation_id,correlation_id,
			idempotency_key,occurred_at,status,claimed_by,claimed_at
		) VALUES($1,$2,1,'conversation.turn',$3,$4,$5,$6,now(),'claimed',$7,now())`,
		inboxID,
		fluctlightID,
		jsonBytes(map[string]any{"actor_id": actorID, "conversation_id": conversationID, "turn_id": turnID, "text": "test"}),
		"turn:"+turnID,
		idempotencyKey,
		claimOwner,
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `
		INSERT INTO public.conversation_messages(
			id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key
		) VALUES($1,$2,1,$3,'assistant','already committed','[]',$4)`,
		assistantID,
		conversationID,
		fluctlightID,
		"assistant:"+turnID,
	)
	if err != nil {
		_, _ = pool.Exec(ctx, `DELETE FROM public.cognition_inbox WHERE id=$1`, inboxID)
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM public.conversation_messages WHERE id=$1`, assistantID)
		_, _ = pool.Exec(ctx, `DELETE FROM public.cognition_inbox WHERE id=$1`, inboxID)
	})

	app := &App{DB: &PostgresRepository{pool: pool}}
	replayedInboxID, _, err := app.enqueueTurnFactClaimed(ctx, actorID, fluctlightID, conversationID, turnID, idempotencyKey, "test", []any{})
	if err != nil {
		t.Fatal(err)
	}
	if replayedInboxID != inboxID {
		t.Fatalf("replayed inbox = %q, want %q", replayedInboxID, inboxID)
	}

	var status string
	var processedAt *time.Time
	var claimedBy *string
	if err := pool.QueryRow(ctx, `SELECT status,processed_at,claimed_by FROM public.cognition_inbox WHERE id=$1`, inboxID).Scan(&status, &processedAt, &claimedBy); err != nil {
		t.Fatal(err)
	}
	if status != "processed" || processedAt == nil || claimedBy != nil {
		t.Fatalf("settled claim = status %q, processed_at %v, claimed_by %v; want processed with timestamp and empty lease", status, processedAt, claimedBy)
	}
}
