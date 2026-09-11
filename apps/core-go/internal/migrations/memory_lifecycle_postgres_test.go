package migrations

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func applyAffectHeadMemoryFixture(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
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
	} {
		if _, err := pool.Exec(ctx, step.sql); err != nil {
			t.Fatalf("apply %s fixture: %v", step.name, err)
		}
	}
	// schemaSQL represents a fresh current database. Remove only the unpublished
	// 0029 Memory additions to model the released 0028 boundary relevant to this
	// migration, while retaining the real 0026/0027/0028 SQL effects above.
	if _, err := pool.Exec(ctx, `
DROP TABLE IF EXISTS public.memory_governance;
DROP TABLE IF EXISTS public.memory_lifecycle_repairs;
ALTER TABLE public.memory_embeddings DROP CONSTRAINT IF EXISTS memory_embeddings_memory_id_memory_revision_model_id_key;
ALTER TABLE public.memory_embeddings DROP COLUMN IF EXISTS provider_endpoint_id;
ALTER TABLE public.memory_revisions DROP CONSTRAINT IF EXISTS memory_revisions_memory_id_revision_key;
ALTER TABLE public.memory_revisions DROP COLUMN IF EXISTS operation;
ALTER TABLE public.memory_revisions DROP COLUMN IF EXISTS snapshot;
ALTER TABLE public.memory_revisions DROP COLUMN IF EXISTS source_window;
ALTER TABLE public.memory_revisions DROP COLUMN IF EXISTS proposal_id;
ALTER TABLE public.memory_revisions DROP COLUMN IF EXISTS candidate_index;
ALTER TABLE public.memory_revisions DROP COLUMN IF EXISTS semantic_reason;
ALTER TABLE public.memory_revisions DROP COLUMN IF EXISTS related_memory_ids;
ALTER TABLE public.memory_revisions DROP COLUMN IF EXISTS request_digest;
ALTER TABLE public.memory_revisions DROP COLUMN IF EXISTS reason_code;
ALTER TABLE public.memory_revisions DROP COLUMN IF EXISTS schema_version;
ALTER TABLE public.memories DROP COLUMN IF EXISTS canonical_key;
ALTER TABLE public.memories DROP COLUMN IF EXISTS request_digest;
ALTER TABLE public.memories DROP COLUMN IF EXISTS superseded_by_memory_id;
ALTER TABLE public.memories DROP COLUMN IF EXISTS supersedes_memory_id;
ALTER TABLE public.memories DROP COLUMN IF EXISTS updated_at;
ALTER TABLE public.memories DROP COLUMN IF EXISTS deprecated_at;
`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.alembic_version(version_num) VALUES($1)`, AffectCanonicalHead); err != nil {
		t.Fatal(err)
	}
}

func addAuditedMemoryRepairColumns(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(ctx, `
ALTER TABLE public.memories ADD COLUMN canonical_key varchar(128);
ALTER TABLE public.memories ADD COLUMN request_digest varchar(128);
CREATE TABLE public.memory_lifecycle_repairs (
  memory_id varchar(128) PRIMARY KEY,
  repair_version varchar(64) NOT NULL,
  source_snapshot jsonb NOT NULL,
  canonical_key varchar(128) NOT NULL,
  request_digest varchar(128) NOT NULL,
  repaired_by_actor_id varchar(128) NOT NULL,
  reason text NOT NULL,
  repaired_at timestamptz NOT NULL DEFAULT now()
);`); err != nil {
		t.Fatal(err)
	}
}

func migrationMemoryCanonicalKey(typeName, content string) string {
	digest := sha256.Sum256([]byte(strings.Join([]string{typeName, strings.TrimSpace(content), "", "private", "", ""}, "\x1f")))
	return hex.EncodeToString(digest[:])[:32]
}

func seedPreviousHeadMemory(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id, typeName, content, requestDigest string) {
	t.Helper()
	canonicalKey := migrationMemoryCanonicalKey(typeName, content)
	if _, err := pool.Exec(ctx, `INSERT INTO public.memories(id,owner_fluctlight_id,type,content,actor_refs,event_refs,evidence_refs,personality_perspectives,confidence,importance,emotional_significance,visibility,status,revision,canonical_key,request_digest) VALUES($1,'fl-memory-migration',$2,$3,'[]','[]','["sequence:1"]','[]',0.8,0.7,0.2,'private','active',0,$4,$5)`, id, typeName, content, canonicalKey, requestDigest); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.memory_revisions(id,memory_id,revision,base_revision,content,personality_perspectives,status,actor_id,evidence_refs,idempotency_key) VALUES($1,$2,0,0,$3,'[]','active','fl-memory-migration','["sequence:1"]',$4)`, "revision-"+id, id, content, "legacy-"+id); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.memory_lifecycle_repairs(memory_id,repair_version,source_snapshot,canonical_key,request_digest,repaired_by_actor_id,reason) SELECT id,'memory.lifecycle.repair.v1',jsonb_build_object('type',type,'content',btrim(content),'conversation_id',COALESCE(conversation_id,''),'visibility',visibility,'actor_refs','[]'::jsonb,'event_refs','[]'::jsonb),canonical_key,request_digest,'repair-operator','explicit test repair' FROM public.memories WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
}

func assertMemoryMigrationRollback(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	var head string
	if err := pool.QueryRow(ctx, `SELECT version_num FROM public.alembic_version`).Scan(&head); err != nil || head != AffectCanonicalHead {
		t.Fatalf("failed 0029 advanced ledger: head=%q err=%v", head, err)
	}
	var governanceTable int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.tables WHERE table_schema='public' AND table_name='memory_governance'`).Scan(&governanceTable); err != nil {
		t.Fatal(err)
	}
	if governanceTable != 0 {
		t.Fatal("failed 0029 left memory_governance behind")
	}
}

func TestMemoryLifecycleMigrationUpgradesAffectHeadAndIsIdempotent(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	applyAffectHeadMemoryFixture(t, ctx, pool)
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	var head string
	if err := pool.QueryRow(ctx, `SELECT version_num FROM public.alembic_version`).Scan(&head); err != nil || head != Head {
		t.Fatalf("head=%q err=%v", head, err)
	}
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatalf("0029 rerun failed: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO public.memories(id,owner_fluctlight_id,type,content,actor_refs,event_refs,evidence_refs,personality_perspectives,confidence,importance,emotional_significance,visibility,status,revision,canonical_key,request_digest) VALUES('invalid-memory','fl','semantic','invalid','[]','[]','["sequence:1"]','[]',0.8,0.7,0.2,'private','active',0,'not-a-digest','also-not-a-digest')`); err == nil {
		t.Fatal("post-cutover Memory constraint accepted noncanonical digests")
	}
}

func TestMemoryLifecycleMigrationPreservesExplicitlyRepairedMemoryAt0029And0030RejectsIt(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	applyAffectHeadMemoryFixture(t, ctx, pool)
	addAuditedMemoryRepairColumns(t, ctx, pool)
	seedPreviousHeadMemory(t, ctx, pool, "memory-repaired", "semantic", "repaired content", strings.Repeat("b", 32))
	if _, err := pool.Exec(ctx, memoryLifecycleMigrationSQL); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE public.alembic_version SET version_num=$1`, MemoryLifecycleHead); err != nil {
		t.Fatal(err)
	}
	var content, canonicalKey, requestDigest string
	var revisionCount int
	if err := pool.QueryRow(ctx, `SELECT content,canonical_key,request_digest FROM public.memories WHERE id='memory-repaired'`).Scan(&content, &canonicalKey, &requestDigest); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM public.memory_revisions WHERE memory_id='memory-repaired'`).Scan(&revisionCount); err != nil {
		t.Fatal(err)
	}
	if content != "repaired content" || canonicalKey != migrationMemoryCanonicalKey("semantic", "repaired content") || requestDigest != strings.Repeat("b", 32) || revisionCount != 1 {
		t.Fatalf("repaired Memory changed content=%q key=%q digest=%q revisions=%d", content, canonicalKey, requestDigest, revisionCount)
	}
	if err := New(pool).Apply(ctx); err == nil || !strings.Contains(err.Error(), "clean-start cutover") {
		t.Fatalf("0030 accepted explicitly repaired 0029 business data: %v", err)
	}
	var head string
	if err := pool.QueryRow(ctx, `SELECT version_num FROM public.alembic_version`).Scan(&head); err != nil || head != MemoryLifecycleHead {
		t.Fatalf("failed 0030 changed repaired 0029 ledger: head=%q err=%v", head, err)
	}
}

func TestMemoryLifecycleMigrationFailsClosedWithoutRewritingHistory(t *testing.T) {
	t.Run("pre-lifecycle Memory", func(t *testing.T) {
		ctx, pool := isolatedMigrationPool(t)
		applyAffectHeadMemoryFixture(t, ctx, pool)
		if _, err := pool.Exec(ctx, `INSERT INTO public.memories(id,owner_fluctlight_id,type,content,actor_refs,event_refs,evidence_refs,personality_perspectives,confidence,importance,emotional_significance,visibility,status,revision) VALUES('memory-unrepaired','fl','semantic','legacy','[]','[]','["sequence:1"]','[]',0.8,0.7,0.2,'private','active',0)`); err != nil {
			t.Fatal(err)
		}
		if err := New(pool).Apply(ctx); err == nil {
			t.Fatal("unrepaired Memory crossed 0029")
		}
		assertMemoryMigrationRollback(t, ctx, pool)
		var lifecycleColumns int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM information_schema.columns WHERE table_schema='public' AND table_name='memories' AND column_name='canonical_key'`).Scan(&lifecycleColumns); err != nil || lifecycleColumns != 0 {
			t.Fatalf("failed migration left canonical_key count=%d err=%v", lifecycleColumns, err)
		}
	})

	t.Run("duplicate active canonical key", func(t *testing.T) {
		ctx, pool := isolatedMigrationPool(t)
		applyAffectHeadMemoryFixture(t, ctx, pool)
		addAuditedMemoryRepairColumns(t, ctx, pool)
		seedPreviousHeadMemory(t, ctx, pool, "memory-duplicate-a", "semantic", "duplicate content", strings.Repeat("b", 32))
		seedPreviousHeadMemory(t, ctx, pool, "memory-duplicate-b", "semantic", "duplicate content", strings.Repeat("c", 32))
		if err := New(pool).Apply(ctx); err == nil {
			t.Fatal("duplicate active canonical key crossed 0029")
		}
		assertMemoryMigrationRollback(t, ctx, pool)
	})

	t.Run("well-shaped but unverifiable repair", func(t *testing.T) {
		ctx, pool := isolatedMigrationPool(t)
		applyAffectHeadMemoryFixture(t, ctx, pool)
		addAuditedMemoryRepairColumns(t, ctx, pool)
		seedPreviousHeadMemory(t, ctx, pool, "memory-wrong-repair", "semantic", "wrong repair content", strings.Repeat("b", 32))
		if _, err := pool.Exec(ctx, `UPDATE public.memory_lifecycle_repairs SET source_snapshot=jsonb_set(source_snapshot,'{content}','"different"') WHERE memory_id='memory-wrong-repair'`); err != nil {
			t.Fatal(err)
		}
		if err := New(pool).Apply(ctx); err == nil {
			t.Fatal("unverifiable audited repair crossed 0029")
		}
		assertMemoryMigrationRollback(t, ctx, pool)
	})

	t.Run("well-shaped but incorrect canonical key", func(t *testing.T) {
		ctx, pool := isolatedMigrationPool(t)
		applyAffectHeadMemoryFixture(t, ctx, pool)
		addAuditedMemoryRepairColumns(t, ctx, pool)
		seedPreviousHeadMemory(t, ctx, pool, "memory-wrong-key", "semantic", "canonical source content", strings.Repeat("b", 32))
		if _, err := pool.Exec(ctx, `UPDATE public.memories SET canonical_key=$1 WHERE id='memory-wrong-key'`, strings.Repeat("a", 32)); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `UPDATE public.memory_lifecycle_repairs SET canonical_key=$1 WHERE memory_id='memory-wrong-key'`, strings.Repeat("a", 32)); err != nil {
			t.Fatal(err)
		}
		if err := New(pool).Apply(ctx); err == nil {
			t.Fatal("incorrect but well-shaped canonical key crossed 0029")
		}
		assertMemoryMigrationRollback(t, ctx, pool)
	})

	t.Run("duplicate embedding tuple", func(t *testing.T) {
		ctx, pool := isolatedMigrationPool(t)
		applyAffectHeadMemoryFixture(t, ctx, pool)
		addAuditedMemoryRepairColumns(t, ctx, pool)
		seedPreviousHeadMemory(t, ctx, pool, "memory-embedding-duplicate", "semantic", "embedding duplicate content", strings.Repeat("b", 32))
		if _, err := pool.Exec(ctx, `INSERT INTO public.memory_embeddings(id,memory_id,memory_revision,model_id,dimensions,embedding,status) VALUES('embedding-a','memory-embedding-duplicate',0,'model-a',0,'[]','stale'),('embedding-b','memory-embedding-duplicate',0,'model-a',0,'[]','stale')`); err != nil {
			t.Fatal(err)
		}
		if err := New(pool).Apply(ctx); err == nil {
			t.Fatal("duplicate embedding tuple crossed 0029")
		}
		assertMemoryMigrationRollback(t, ctx, pool)
	})

	t.Run("working Memory type", func(t *testing.T) {
		ctx, pool := isolatedMigrationPool(t)
		applyAffectHeadMemoryFixture(t, ctx, pool)
		addAuditedMemoryRepairColumns(t, ctx, pool)
		seedPreviousHeadMemory(t, ctx, pool, "memory-working", "working", "working content", strings.Repeat("b", 32))
		if err := New(pool).Apply(ctx); err == nil {
			t.Fatal("durable working Memory crossed 0029")
		}
		assertMemoryMigrationRollback(t, ctx, pool)
	})

	t.Run("malformed active embedding intent", func(t *testing.T) {
		ctx, pool := isolatedMigrationPool(t)
		applyAffectHeadMemoryFixture(t, ctx, pool)
		addAuditedMemoryRepairColumns(t, ctx, pool)
		seedPreviousHeadMemory(t, ctx, pool, "memory-intent", "semantic", "intent content", strings.Repeat("b", 32))
		if _, err := pool.Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload,status) VALUES('memory-intent-invalid','memory-intent-invalid','lifecycle','memory.embedding','{"memory_id":"memory-intent","revision":0}','pending')`); err != nil {
			t.Fatal(err)
		}
		if err := New(pool).Apply(ctx); err == nil {
			t.Fatal("malformed active embedding intent crossed 0029")
		}
		assertMemoryMigrationRollback(t, ctx, pool)
	})
}
