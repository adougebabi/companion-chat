package core

import (
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestEmbeddingCacheChangesDoNotInvalidateFacts(t *testing.T) {
	ctx, repo := isolatedCoreTestRepository(t)
	owner, actor := "embedding-facts-owner", "embedding-facts-actor"
	seedLifeContextFluctlight(t, ctx, repo, owner, actor)
	app := &App{DB: repo}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.memories(id,owner_fluctlight_id,type,content,actor_refs,event_refs,evidence_refs,confidence,importance,emotional_significance,visibility,status,revision,canonical_key,request_digest) VALUES('cache-memory',$1,'semantic','原始记忆事实','[]','[]','[]',0.9,0.5,0,'private','active',0,'cache-memory','aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa')`, actor); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.memory_revisions(id,memory_id,revision,base_revision,content,status,actor_id,evidence_refs,source_window,idempotency_key) VALUES('cache-memory-revision','cache-memory',0,0,'原始记忆事实','active',$1,'[]','cache-memory','cache-memory-revision')`, owner); err != nil {
		t.Fatal(err)
	}
	before, err := app.readCurrentFactsRevision(ctx, actor)
	if err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{
		`INSERT INTO public.memory_embeddings(id,memory_id,memory_revision,model_id,dimensions,embedding,status) VALUES('cache-vector','cache-memory',0,'cache-model',2,'[]','pending')`,
		`UPDATE public.memory_embeddings SET embedding='[1,0]',embedding_vector='[1,0]'::vector,status='ready' WHERE id='cache-vector'`,
		`UPDATE public.memory_embeddings SET status='failed',error_code='provider_request_failed' WHERE id='cache-vector'`,
		`UPDATE public.memory_embeddings SET status='pending',error_code=NULL WHERE id='cache-vector'`,
		`DELETE FROM public.memory_embeddings WHERE id='cache-vector'`,
	} {
		if _, err := repo.Pool().Exec(ctx, query); err != nil {
			t.Fatal(err)
		}
		if after, err := app.readCurrentFactsRevision(ctx, actor); err != nil || after != before {
			t.Fatal("derived embedding cache changed fact authority", before, after, err)
		}
	}
	for _, query := range []string{
		`UPDATE public.memories SET content='修订后的记忆事实',revision=1 WHERE id='cache-memory'`,
		`INSERT INTO public.memory_source_links(memory_id,memory_revision,source_ref,source_kind,source_id,status) VALUES('cache-memory',0,'owner:cache-memory','owner_confirmation','embedding-facts-owner','valid')`,
		`UPDATE public.memory_source_links SET status='invalid' WHERE memory_id='cache-memory'`,
	} {
		if _, err := repo.Pool().Exec(ctx, query); err != nil {
			t.Fatal(err)
		}
		after, err := app.readCurrentFactsRevision(ctx, actor)
		if err != nil || after == before {
			t.Fatal("real memory/provenance change no longer advances authority", before, after, err)
		}
		before = after
	}
}

func TestIdenticalWorkingPersonaWriteDoesNotInvalidateFacts(t *testing.T) {
	ctx, repo := isolatedCoreTestRepository(t)
	owner, actor := "persona-facts-owner", "persona-facts-actor"
	seedLifeContextFluctlight(t, ctx, repo, owner, actor)
	app := &App{DB: repo}
	compiled := CompiledWorkingPersona{ProfileID: "cache-test", SourceHash: "same-source", RulesVersion: personaCompilationRulesVersion, BudgetRunes: 512, PortraitText: "有效人格文本"}
	write := func() error {
		return withTransaction(ctx, repo.Pool(), func(tx pgx.Tx) error {
			return insertCompiledWorkingPersonasTx(ctx, tx, actor, []CompiledWorkingPersona{compiled})
		})
	}
	if err := write(); err != nil {
		t.Fatal(err)
	}
	before, _ := app.readCurrentFactsRevision(ctx, actor)
	if err := write(); err != nil {
		t.Fatal(err)
	}
	if after, err := app.readCurrentFactsRevision(ctx, actor); err != nil || after != before {
		t.Fatal("identical compilation invalidated current facts", before, after, err)
	}
	compiled.PortraitText = "确实发生变化的人格文本"
	if err := write(); err != nil {
		t.Fatal(err)
	}
	if after, err := app.readCurrentFactsRevision(ctx, actor); err != nil || after == before {
		t.Fatal("real portrait change failed to advance facts", before, after, err)
	}
}
