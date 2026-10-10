package migrations

import (
	"strings"
	"testing"
)

func TestFactGenerationProvenanceSchemaContract(t *testing.T) {
	for _, fragment := range []string{
		"fluctlight_context_generation_journal",
		"PRIMARY KEY(fluctlight_id,generation)",
		"transaction_id bigint NOT NULL",
		"txid_current()",
		"generation<=v_generation-512",
		"bump_fluctlight_context_generation_with_source",
		"'unknown','unknown'",
		"TG_TABLE_NAME='fluctlight_intentions'",
		"to_jsonb(OLD)-'trigger_cursor_sequence'-'updated_at'",
		"bump_context_conversation_message_trigger",
		"bump_context_fact_trigger",
		"bump_context_schedule_item_trigger",
	} {
		if !strings.Contains(factGenerationProvenanceSchemaSQL, fragment) {
			t.Fatalf("fact generation provenance schema missing %q", fragment)
		}
	}
	if strings.Contains(factGenerationProvenanceSchemaSQL, "payload jsonb") || strings.Contains(factGenerationProvenanceSchemaSQL, "password") {
		t.Fatal("generation journal must not persist business payloads or credentials")
	}
}

func TestFactGenerationProvenanceUpgradeAndHeadRerun(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		CREATE TRIGGER context_generation_bump AFTER INSERT OR UPDATE OR DELETE ON public.memory_embeddings
		 FOR EACH ROW EXECUTE FUNCTION public.bump_context_memory_child_trigger();
		UPDATE public.alembic_version SET version_num=$1`, FactGenerationProvenancePreviousHead); err != nil {
		t.Fatal(err)
	}
	for _, phase := range []string{"0058 upgrade", "0059 head rerun"} {
		if err := New(pool).Apply(ctx); err != nil {
			t.Fatalf("%s: %v", phase, err)
		}
		var head string
		var journal, embeddingTrigger bool
		if err := pool.QueryRow(ctx, `SELECT version_num FROM public.alembic_version`).Scan(&head); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(ctx, `SELECT to_regclass('public.fluctlight_context_generation_journal') IS NOT NULL,
		 EXISTS(SELECT 1 FROM pg_trigger WHERE tgrelid='public.memory_embeddings'::regclass AND tgname='context_generation_bump')`).Scan(&journal, &embeddingTrigger); err != nil {
			t.Fatal(err)
		}
		if head != Head || !journal || embeddingTrigger {
			t.Fatalf("%s left head/journal/embedding trigger invalid: head=%q journal=%v embedding=%v", phase, head, journal, embeddingTrigger)
		}
	}
}

func TestFactGenerationProvenanceCursorSourcesRetentionAndRollback(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO public.actors(id,actor_type,status) VALUES
		 ('facts-journal-owner','human','active'),('facts-journal-fl','fluctlight','active'),('facts-journal-other','fluctlight','active');
		INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES
		 ('facts-journal-fl','facts-journal-owner','blank_slate','active','{}','{}','{}','{}','{}','{}'),
		 ('facts-journal-other','facts-journal-owner','blank_slate','active','{}','{}','{}','{}','{}','{}');
		INSERT INTO public.fluctlight_goals(id,fluctlight_id,profile_id,source,scope,description,desired_outcome,success_criteria,motivation,needs_reflection,importance,urgency,progress,status,evidence_refs,revision,idempotency_key,request_digest)
		 VALUES('facts-journal-goal','facts-journal-fl','default','reflection','general','finish','finish','["done"]','motivation',false,0.8,0.6,0,'active','["fact"]',1,'facts-journal-goal','aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa');
		INSERT INTO public.fluctlight_intentions(id,fluctlight_id,profile_id,goal_id,action,action_intent,expected_outcome,capability_constraints,trigger,confidence,expiration,evidence_refs,permission_snapshot,budget_snapshot,status,revision,idempotency_key,request_digest)
		 VALUES('facts-journal-intention','facts-journal-fl','default','facts-journal-goal','act','act','result','["schedule.replan"]','{"type":"event","event_type":"life.event.created"}',0.9,now()+interval '1 day','["fact"]','{}','{}','qualified',1,'facts-journal-intention','bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb')`); err != nil {
		t.Fatal(err)
	}
	var before int64
	if err := pool.QueryRow(ctx, `SELECT generation FROM public.fluctlight_context_generations WHERE fluctlight_id='facts-journal-fl'`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE public.fluctlight_intentions SET trigger_cursor_sequence=trigger_cursor_sequence+1 WHERE id='facts-journal-intention'`); err != nil {
		t.Fatal(err)
	}
	var afterCursor int64
	if err := pool.QueryRow(ctx, `SELECT generation FROM public.fluctlight_context_generations WHERE fluctlight_id='facts-journal-fl'`).Scan(&afterCursor); err != nil || afterCursor != before {
		t.Fatalf("cursor-only update advanced facts: before=%d after=%d err=%v", before, afterCursor, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE public.fluctlight_intentions SET status='paused' WHERE id='facts-journal-intention'`); err != nil {
		t.Fatal(err)
	}
	var afterStatus int64
	var tableName, operation, entityID string
	if err := pool.QueryRow(ctx, `SELECT generation FROM public.fluctlight_context_generations WHERE fluctlight_id='facts-journal-fl'`).Scan(&afterStatus); err != nil || afterStatus != before+1 {
		t.Fatalf("semantic Intention update did not advance once: before=%d after=%d err=%v", before, afterStatus, err)
	}
	if err := pool.QueryRow(ctx, `SELECT source_table,source_operation,COALESCE(entity_id,'') FROM public.fluctlight_context_generation_journal WHERE fluctlight_id='facts-journal-fl' AND generation=$1`, afterStatus).Scan(&tableName, &operation, &entityID); err != nil || tableName != "fluctlight_intentions" || operation != "update" || entityID != "facts-journal-intention" {
		t.Fatalf("Intention provenance=(%q,%q,%q): %v", tableName, operation, entityID, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE public.fluctlight_intentions SET trigger='{"type":"semantic","event_type":"life.event.changed"}' WHERE id='facts-journal-intention'`); err != nil {
		t.Fatal(err)
	}
	var afterTrigger int64
	if err := pool.QueryRow(ctx, `SELECT generation FROM public.fluctlight_context_generations WHERE fluctlight_id='facts-journal-fl'`).Scan(&afterTrigger); err != nil || afterTrigger != afterStatus+1 {
		t.Fatalf("semantic Intention trigger update did not advance once: before=%d after=%d err=%v", afterStatus, afterTrigger, err)
	}

	if _, err := pool.Exec(ctx, `SELECT public.bump_fluctlight_context_generation('facts-journal-other')`); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT source_table,source_operation FROM public.fluctlight_context_generation_journal WHERE fluctlight_id='facts-journal-other' ORDER BY generation DESC LIMIT 1`).Scan(&tableName, &operation); err != nil || tableName != "unknown" || operation != "unknown" {
		t.Fatalf("legacy one-argument provenance=(%q,%q): %v", tableName, operation, err)
	}
	var otherRows int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM public.fluctlight_context_generation_journal WHERE fluctlight_id='facts-journal-fl' AND source_table='unknown'`).Scan(&otherRows); err != nil || otherRows != 0 {
		t.Fatalf("journal leaked across Fluctlight scope: count=%d err=%v", otherRows, err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `SELECT public.bump_fluctlight_context_generation_with_source('facts-journal-fl','rollback_probe','update','probe')`); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	var afterRollback int64
	var rollbackRows int
	if err := pool.QueryRow(ctx, `SELECT generation FROM public.fluctlight_context_generations WHERE fluctlight_id='facts-journal-fl'`).Scan(&afterRollback); err != nil || afterRollback != afterTrigger {
		t.Fatalf("rolled-back generation persisted: before=%d after=%d err=%v", afterTrigger, afterRollback, err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM public.fluctlight_context_generation_journal WHERE fluctlight_id='facts-journal-fl' AND source_table='rollback_probe'`).Scan(&rollbackRows); err != nil || rollbackRows != 0 {
		t.Fatalf("rolled-back journal row persisted: count=%d err=%v", rollbackRows, err)
	}

	if _, err := pool.Exec(ctx, `SELECT public.bump_fluctlight_context_generation_with_source('facts-journal-fl','retention_probe','update',g::text) FROM generate_series(1,513) AS g`); err != nil {
		t.Fatal(err)
	}
	var retained int
	var minimum, maximum int64
	if err := pool.QueryRow(ctx, `SELECT count(*),min(generation),max(generation) FROM public.fluctlight_context_generation_journal WHERE fluctlight_id='facts-journal-fl'`).Scan(&retained, &minimum, &maximum); err != nil || retained != 512 || maximum-minimum != 511 {
		t.Fatalf("journal retention count/range=%d/%d..%d err=%v", retained, minimum, maximum, err)
	}
}
