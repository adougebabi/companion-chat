package migrations

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresMemoryProvenanceUpgradeRecoversOnlyRealSourcesAndReruns(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	seedMemoryProvenancePreviousHead(t, ctx, pool)
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatalf("0036 to 0037: %v", err)
	}
	var firstGeneration int64
	if err := pool.QueryRow(ctx, `SELECT generation FROM public.fluctlight_context_generations WHERE fluctlight_id='provenance-fluctlight'`).Scan(&firstGeneration); err != nil {
		t.Fatal(err)
	}
	var firstResidentGeneration int64
	if err := pool.QueryRow(ctx, `SELECT generation FROM public.resident_memory_snapshots WHERE fluctlight_id='provenance-fluctlight'`).Scan(&firstResidentGeneration); err != nil {
		t.Fatal(err)
	}
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatalf("0037 rerun: %v", err)
	}
	var secondGeneration int64
	if err := pool.QueryRow(ctx, `SELECT generation FROM public.fluctlight_context_generations WHERE fluctlight_id='provenance-fluctlight'`).Scan(&secondGeneration); err != nil || firstGeneration != secondGeneration {
		t.Fatalf("idempotent migration changed context generation: before=%d after=%d err=%v", firstGeneration, secondGeneration, err)
	}
	var secondResidentGeneration int64
	if err := pool.QueryRow(ctx, `SELECT generation FROM public.resident_memory_snapshots WHERE fluctlight_id='provenance-fluctlight'`).Scan(&secondResidentGeneration); err != nil || secondResidentGeneration != firstResidentGeneration {
		t.Fatalf("idempotent migration republished Resident: before=%d after=%d err=%v", firstResidentGeneration, secondResidentGeneration, err)
	}
	var head, knownStatus, knownKind, knownID, knownMemoryStatus, unknownStatus, unknownMemoryStatus string
	if err := pool.QueryRow(ctx, `SELECT version_num FROM public.alembic_version`).Scan(&head); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT l.status,l.source_kind,l.source_id,m.provenance_status FROM public.memory_source_links l JOIN public.memories m ON m.id=l.memory_id WHERE l.memory_id='memory-known'`).Scan(&knownStatus, &knownKind, &knownID, &knownMemoryStatus); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT l.status,m.provenance_status FROM public.memory_source_links l JOIN public.memories m ON m.id=l.memory_id WHERE l.memory_id='memory-unknown'`).Scan(&unknownStatus, &unknownMemoryStatus); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM public.memory_source_links`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if head != Head || knownStatus != "valid" || knownKind != "fact" || knownID != "provenance-fact" || knownMemoryStatus != "verified" || unknownStatus != "unknown" || unknownMemoryStatus != "legacy_unknown" || count != 2 {
		t.Fatalf("migration head=%s known=%s/%s/%s/%s unknown=%s/%s links=%d", head, knownStatus, knownKind, knownID, knownMemoryStatus, unknownStatus, unknownMemoryStatus, count)
	}
}

func seedMemoryProvenancePreviousHead(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	for _, statement := range []string{
		`INSERT INTO public.actors(id,actor_type,status) VALUES('provenance-owner','human','active'),('provenance-fluctlight','fluctlight','active')`,
		`INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES('provenance-fluctlight','provenance-owner','blank_slate','active','{}','{}','{}','{}','{}','{}')`,
		`INSERT INTO public.cognition_inbox_heads(fluctlight_id,next_sequence,last_processed_sequence) VALUES('provenance-fluctlight',2,1)`,
		`INSERT INTO public.cognition_inbox(id,fluctlight_id,sequence,event_type,payload,causation_id,correlation_id,idempotency_key,occurred_at,status) VALUES('provenance-fact','provenance-fluctlight',1,'conversation.user','{"text":"已确认"}','provenance-fact','provenance-fact','provenance-fact',now(),'processed')`,
		`INSERT INTO public.memories(id,owner_fluctlight_id,type,content,actor_refs,event_refs,evidence_refs,personality_perspectives,confidence,importance,emotional_significance,visibility,status,revision,canonical_key,request_digest) VALUES('memory-known','provenance-fluctlight','semantic','已确认','[]','[]','["sequence:1"]','[]',0.8,0.8,0,'private','active',0,'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa','bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb')`,
		`INSERT INTO public.memory_revisions(id,memory_id,revision,base_revision,content,personality_perspectives,status,actor_id,evidence_refs,idempotency_key) VALUES('memory-known-revision','memory-known',0,0,'已确认','[]','active','provenance-fluctlight','["sequence:1"]','memory-known-revision')`,
		`INSERT INTO public.memories(id,owner_fluctlight_id,type,content,actor_refs,event_refs,evidence_refs,personality_perspectives,confidence,importance,emotional_significance,visibility,status,revision,canonical_key,request_digest) VALUES('memory-unknown','provenance-fluctlight','semantic','来源不明','[]','[]','["unknown-old-ref"]','[]',0.8,0.8,0,'private','active',0,'cccccccccccccccccccccccccccccccc','dddddddddddddddddddddddddddddddd')`,
		`INSERT INTO public.memory_revisions(id,memory_id,revision,base_revision,content,personality_perspectives,status,actor_id,evidence_refs,idempotency_key) VALUES('memory-unknown-revision','memory-unknown',0,0,'来源不明','[]','active','provenance-fluctlight','["unknown-old-ref"]','memory-unknown-revision')`,
		`DROP TABLE public.memory_source_links`,
		`ALTER TABLE public.memories DROP COLUMN provenance_status`,
		`UPDATE public.alembic_version SET version_num='0036_effective_life'`,
	} {
		if _, err := pool.Exec(ctx, statement); err != nil {
			t.Fatalf("seed previous head: %v\n%s", err, statement)
		}
	}
}
