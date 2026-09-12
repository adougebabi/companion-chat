package migrations

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// These tests are intentionally authored in S02 but execute only in the S12
// isolated-PostgreSQL gate under the task's strict verification policy.

func applyEvolutionHeadPromptContextFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	for _, step := range []struct {
		name string
		sql  string
	}{
		{name: "schema", sql: schemaSQL},
		{name: "compatibility", sql: compatibilitySQL},
		{name: "capability_runtime", sql: capabilityRuntimeMigrationSQL},
		{name: "project_health", sql: projectHealthEvolutionMigrationSQL},
		{name: "affect", sql: affectCanonicalMigrationSQL},
		{name: "memory", sql: memoryLifecycleMigrationSQL},
		{name: "life_context", sql: lifeContextRevisionMigrationSQL},
		{name: "evolution", sql: evolutionAuthorityMigrationSQL},
	} {
		if _, err := pool.Exec(ctx, step.sql); err != nil {
			t.Fatalf("apply %s fixture: %v", step.name, err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.alembic_version(version_num) VALUES($1)`, EvolutionAuthorityHead); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresPromptContextMemoryMigrationUpgradesEvolutionHeadAndIsIdempotent(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	applyEvolutionHeadPromptContextFixture(t, ctx, pool)

	if _, err := pool.Exec(ctx, `
INSERT INTO public.conversations(id,created_by_actor_id,title) VALUES('prompt-history-conversation','owner','history');
INSERT INTO public.conversation_messages(id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key)
VALUES('prompt-history-message','prompt-history-conversation',1,'owner','user','flight tomorrow','[]','prompt-history-message');
INSERT INTO public.cognition_inbox(id,fluctlight_id,sequence,event_type,payload,causation_id,correlation_id,idempotency_key,occurred_at,status)
VALUES('prompt-history-fact','prompt-history-fluctlight',1,'conversation.turn','{"conversation_id":"prompt-history-conversation","text":"flight tomorrow"}','prompt-history-turn','turn:prompt-history-turn','prompt-history-fact',now(),'processed');
`); err != nil {
		t.Fatal(err)
	}

	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatalf("0032 rerun: %v", err)
	}

	var head string
	if err := pool.QueryRow(ctx, `SELECT version_num FROM public.alembic_version`).Scan(&head); err != nil || head != PromptContextMemoryHead {
		t.Fatalf("head=%q err=%v", head, err)
	}
	var columns, indexes, constraints int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.columns WHERE table_schema='public' AND table_name='conversation_messages' AND column_name IN ('turn_id','source_fact_id','correlation_id','search_document')`).Scan(&columns); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_indexes WHERE schemaname='public' AND indexname IN ('uq_conversation_messages_sequence','uq_cognition_inbox_sequence','uq_conversation_messages_turn_kind','ix_conversation_messages_source_fact','ix_conversation_messages_search_document')`).Scan(&indexes); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_constraint WHERE conname='fk_conversation_messages_source_fact' AND condeferrable AND condeferred`).Scan(&constraints); err != nil {
		t.Fatal(err)
	}
	if columns != 4 || indexes != 5 || constraints != 1 {
		t.Fatalf("0032 Raw History schema columns=%d indexes=%d constraints=%d", columns, indexes, constraints)
	}
	var searchable bool
	if err := pool.QueryRow(ctx, `SELECT search_document @@ plainto_tsquery('simple','flight') FROM public.conversation_messages WHERE id='prompt-history-message'`).Scan(&searchable); err != nil || !searchable {
		t.Fatalf("generated search document searchable=%t err=%v", searchable, err)
	}

	if _, err := pool.Exec(ctx, `
INSERT INTO public.cognition_inbox(id,fluctlight_id,sequence,event_type,payload,causation_id,correlation_id,idempotency_key,occurred_at,status)
VALUES('prompt-linked-fact','prompt-linked-fluctlight',1,'conversation.turn','{}','prompt-linked-turn','turn:prompt-linked-turn','prompt-linked-fact',now(),'processed');
INSERT INTO public.conversations(id,created_by_actor_id,title) VALUES('prompt-linked-conversation','owner','linked');
INSERT INTO public.conversation_messages(id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key,turn_id,source_fact_id,correlation_id)
VALUES('prompt-linked-message','prompt-linked-conversation',1,'owner','user','linked fact','[]','prompt-linked-message','prompt-linked-turn','prompt-linked-fact','turn:prompt-linked-turn');
`); err != nil {
		t.Fatalf("post-migration source linkage rejected: %v", err)
	}
}

func TestPostgresPromptContextMemoryMigrationRejectsDuplicateSequencesAndRollsBack(t *testing.T) {
	for _, testCase := range []struct {
		name string
		seed string
		want string
	}{
		{
			name: "conversation message",
			seed: `
INSERT INTO public.conversations(id,created_by_actor_id,title) VALUES('duplicate-message-conversation','owner','duplicate');
INSERT INTO public.conversation_messages(id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key)
VALUES('duplicate-message-a','duplicate-message-conversation',1,'owner','user','a','[]','duplicate-message-a'),
      ('duplicate-message-b','duplicate-message-conversation',1,'owner','assistant','b','[]','duplicate-message-b');`,
			want: "duplicate Conversation message sequence",
		},
		{
			name: "cognition fact",
			seed: `
INSERT INTO public.cognition_inbox(id,fluctlight_id,sequence,event_type,payload,causation_id,correlation_id,idempotency_key,occurred_at,status)
VALUES('duplicate-fact-a','duplicate-fact-fluctlight',1,'test.a','{}','a','a','duplicate-fact-a',now(),'processed'),
      ('duplicate-fact-b','duplicate-fact-fluctlight',1,'test.b','{}','b','b','duplicate-fact-b',now(),'processed');`,
			want: "duplicate Cognition fact sequence",
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			ctx, pool := isolatedMigrationPool(t)
			applyEvolutionHeadPromptContextFixture(t, ctx, pool)
			if _, err := pool.Exec(ctx, testCase.seed); err != nil {
				t.Fatal(err)
			}
			err := New(pool).Apply(ctx)
			if err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("duplicate sequence migration err=%v", err)
			}
			var head string
			if err := pool.QueryRow(ctx, `SELECT version_num FROM public.alembic_version`).Scan(&head); err != nil || head != EvolutionAuthorityHead {
				t.Fatalf("failed 0032 advanced ledger: head=%q err=%v", head, err)
			}
			var columns, indexes int
			if err := pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.columns WHERE table_schema='public' AND table_name='conversation_messages' AND column_name IN ('turn_id','source_fact_id','correlation_id','search_document')`).Scan(&columns); err != nil {
				t.Fatal(err)
			}
			if err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_indexes WHERE schemaname='public' AND indexname IN ('uq_conversation_messages_sequence','uq_cognition_inbox_sequence','uq_conversation_messages_turn_kind','ix_conversation_messages_source_fact','ix_conversation_messages_search_document')`).Scan(&indexes); err != nil {
				t.Fatal(err)
			}
			if columns != 0 || indexes != 0 {
				t.Fatalf("failed 0032 left columns=%d indexes=%d", columns, indexes)
			}
		})
	}
}
