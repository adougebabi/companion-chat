package migrations

import (
	"fmt"
	"strings"
	"testing"
)

func TestMediaKevUpgradeDoesNotRewriteReleasedSQLOrSettings(t *testing.T) {
	if strings.Contains(kevDecisionsSchemaSQL, "media.subject_route") {
		t.Fatal("released 0056 SQL rewritten")
	}
	for _, word := range []string{"UPDATE public.runtime_settings", "DELETE FROM public.kev_decisions", "DELETE FROM public.kev_deferrals"} {
		if strings.Contains(mediaKevSchemaSQL, word) {
			t.Fatalf("destructive media migration: %s", word)
		}
	}
	assertCurrentAdditiveChain(t)
	if WorldCatalogPreviousHead != "0064_world_runtime" || WorldRuntimePreviousHead != "0063_goal_agent_cadence" || GoalAgentCadencePreviousHead != "0062_reply_media" || ReplyMediaPreviousHead != "0061_fact_only_reply" || FactOnlyReplyPreviousHead != "0060_media_kev" || MediaKevPreviousHead != "0059_fact_generation_provenance" {
		t.Fatal("upgrade chain invalid")
	}
}
func TestMediaKevUpgradeRerunPreservesAuditAndAcceptsOnlyKnownPoints(t *testing.T) {
	ctx, pool := isolatedMigrationPool(t)
	if err := New(pool).Apply(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `ALTER TABLE public.kev_decisions DROP CONSTRAINT kev_decisions_decision_point_check; ALTER TABLE public.kev_decisions ADD CONSTRAINT kev_decisions_decision_point_check CHECK(decision_point IN ('runtime.wakeup','runtime.reflection','goal.completion_check','goal.replenish_plan','tools.select','persona.switch','context.select'))`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE public.alembic_version SET version_num=$1`, MediaKevPreviousHead); err != nil {
		t.Fatal(err)
	}
	insert := `INSERT INTO public.kev_decisions(id,request_id,question_id,decision_point,actor_self,candidate_id,started_at,completed_at,payload) VALUES($1,$1,'q_0001',$2,'actor','candidate',now(),now(),'{}')`
	if _, err := pool.Exec(ctx, insert, "old-audit", "tools.select"); err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		if err := New(pool).Apply(ctx); err != nil {
			t.Fatal(err)
		}
		for _, point := range []string{"media.subject_route", "media.capture_plan", "media.view_plan"} {
			if _, err := pool.Exec(ctx, insert, fmt.Sprintf("%d-%s", attempt, point), point); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := pool.Exec(ctx, insert, fmt.Sprintf("unknown-%d", attempt), "media.unregistered"); err == nil {
			t.Fatal("unknown point accepted")
		}
		var head string
		var oldCount int
		if err := pool.QueryRow(ctx, `SELECT version_num FROM public.alembic_version`).Scan(&head); err != nil {
			t.Fatal(err)
		}
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM public.kev_decisions WHERE id='old-audit'`).Scan(&oldCount); err != nil {
			t.Fatal(err)
		}
		if head != Head || oldCount != 1 {
			t.Fatal("head/audit not retained")
		}
	}
}
