package core

import (
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func claimEvidencePlan(t *testing.T, source, key, content string, confidence float64, projection ContextProjection) map[string]any {
	t.Helper()
	plan, err := normalizeResponsePlan(map[string]any{"action_type": "reply", "claims": []any{map[string]any{"kind": ClaimObservedFact, "content": content, "repetition_key": key, "confidence": confidence, "evidence_refs": []any{"current_message"}}}}, source, projection)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func TestClaimsWithSharedKeyAndEvidenceKeepDifferentFacts(t *testing.T) {
	projection := ContextProjection{Hypotheses: []map[string]any{{"id": "prior-claim", "source_fact_id": "same-source", "repetition_key": "shared-key", "content": "用户喜欢咖啡", "evidence_refs": []any{"same-source"}}}}
	plan := claimEvidencePlan(t, "same-source", "shared-key", "用户喜欢茶", 0.8, projection)
	if len(arrayValue(plan["approved_claims"])) != 1 || len(arrayValue(plan["omitted_claims"])) != 0 {
		t.Fatalf("different fact was omitted only because alias/source matched: %#v", plan)
	}
}

func TestClaimEvidenceVersionsPostgres(t *testing.T) {
	ctx, repo := isolatedCoreTestRepository(t)
	seedLifeContextFluctlight(t, ctx, repo, "claim-history-owner", "claim-history-role")
	app := &App{DB: repo}
	at := time.Now().UTC().Truncate(time.Microsecond)
	persist := func(source string, plan map[string]any, when time.Time) error {
		return withTransaction(ctx, repo.Pool(), func(tx pgx.Tx) error { return persistClaimsTx(ctx, tx, "claim-history-role", source, plan, when) })
	}
	current := func(t *testing.T, key, content string) map[string]any {
		t.Helper()
		var raw []byte
		if err := repo.Pool().QueryRow(ctx, `SELECT to_jsonb(c) FROM public.cognition_claims c WHERE fluctlight_id='claim-history-role' AND content=$1`, content).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		return decodeObject(raw)
	}
	audit := func(t *testing.T, id string) []map[string]any {
		t.Helper()
		rows, err := repo.Pool().Query(ctx, `SELECT jsonb_build_object('version',version,'source_fact_id',source_fact_id,'snapshot',snapshot,'applied',applied,'origin',origin) FROM public.cognition_claim_evaluations WHERE claim_id=$1 ORDER BY version`, id)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			var raw []byte
			if err := rows.Scan(&raw); err != nil {
				t.Fatal(err)
			}
			out = append(out, decodeObject(raw))
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		return out
	}
	t.Run("new_evidence_updates_current_projection_and_keeps_prior_snapshot", func(t *testing.T) {
		first := claimEvidencePlan(t, "evidence-old", "interest", "用户喜欢旅行", 0.2, ContextProjection{})
		if err := persist("evidence-old", first, at); err != nil {
			t.Fatal(err)
		}
		old := current(t, "interest", "用户喜欢旅行")
		hypotheses, err := app.readActiveHypotheses(ctx, "claim-history-role")
		if err != nil {
			t.Fatal(err)
		}
		next := claimEvidencePlan(t, "evidence-new", "interest", "用户喜欢旅行", 0.8, ContextProjection{Hypotheses: hypotheses})
		if len(arrayValue(next["approved_claims"])) != 1 {
			t.Fatal("new evidence was not approved")
		}
		if err := persist("evidence-new", next, at.Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
		got := current(t, "interest", "用户喜欢旅行")
		if got["id"] != old["id"] || got["status"] != "active" || got["source_fact_id"] != "evidence-new" || !reflect.DeepEqual(got["evidence_refs"], []any{"evidence-new"}) || got["confidence"] != 0.8 {
			t.Fatalf("approved new evidence disappeared: old=%#v current=%#v", old, got)
		}
		versions := audit(t, stringValue(got["id"]))
		if len(versions) != 2 || !reflect.DeepEqual(mapValue(versions[0]["snapshot"])["evidence_refs"], []any{"evidence-old"}) || !reflect.DeepEqual(mapValue(versions[1]["snapshot"])["evidence_refs"], []any{"evidence-new"}) {
			t.Fatalf("old/new provenance was lost: %#v", versions)
		}
		hypotheses, err = app.readActiveHypotheses(ctx, "claim-history-role")
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, h := range hypotheses {
			if h["id"] == got["id"] {
				found = true
				if !reflect.DeepEqual(h["evidence_refs"], []any{"evidence-new"}) {
					t.Fatalf("projection has stale evidence %#v", h)
				}
			}
		}
		if !found {
			t.Fatal("updated hypothesis absent")
		}
	})
	t.Run("same_assessment_replay_and_concurrent_replay_do_not_renew_expiry", func(t *testing.T) {
		plan := claimEvidencePlan(t, "replay-source", "replay-key", "用户可能喜欢安静", 0.2, ContextProjection{})
		if err := persist("replay-source", plan, at); err != nil {
			t.Fatal(err)
		}
		before := current(t, "replay-key", "用户可能喜欢安静")
		var wg sync.WaitGroup
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := persist("replay-source", plan, at.Add(24*time.Hour)); err != nil {
					t.Error(err)
				}
			}()
		}
		wg.Wait()
		after := current(t, "replay-key", "用户可能喜欢安静")
		if !reflect.DeepEqual(before, after) || len(audit(t, stringValue(after["id"]))) != 1 {
			t.Fatalf("replay changed state/expiry or duplicated assessment: before=%#v after=%#v", before, after)
		}
	})
	t.Run("same_source_and_alias_do_not_merge_distinct_facts", func(t *testing.T) {
		for _, content := range []string{"用户喜欢咖啡", "用户喜欢茶"} {
			plan := claimEvidencePlan(t, "shared-source", "shared-alias", content, 0.8, ContextProjection{})
			if err := persist("shared-source", plan, at); err != nil {
				t.Fatal(err)
			}
		}
		one, two := current(t, "shared-alias", "用户喜欢咖啡"), current(t, "shared-alias", "用户喜欢茶")
		if one["id"] == two["id"] {
			t.Fatal("distinct facts merged")
		}
		hypotheses, err := app.readActiveHypotheses(ctx, "claim-history-role")
		if err != nil {
			t.Fatal(err)
		}
		for _, content := range []string{"用户喜欢咖啡", "用户喜欢茶"} {
			plan := claimEvidencePlan(t, "shared-source", "shared-alias", content, 0.8, ContextProjection{Hypotheses: hypotheses})
			if len(arrayValue(plan["omitted_claims"])) != 1 {
				t.Fatalf("same fact replay no longer recognized: %#v", plan)
			}
		}
		if len(audit(t, stringValue(one["id"]))) != 1 || len(audit(t, stringValue(two["id"]))) != 1 {
			t.Fatal("facts did not get separate provenance")
		}
	})
	t.Run("old_assessment_replay_and_late_evidence_cannot_replace_newer_current", func(t *testing.T) {
		old := claimEvidencePlan(t, "old-current-source", "current-order", "用户喜欢散步", 0.2, ContextProjection{})
		newer := claimEvidencePlan(t, "new-current-source", "current-order", "用户喜欢散步", 0.8, ContextProjection{})
		if err := persist("old-current-source", old, at); err != nil {
			t.Fatal(err)
		}
		if err := persist("new-current-source", newer, at.Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
		before := current(t, "current-order", "用户喜欢散步")
		if err := persist("old-current-source", old, at.Add(2*time.Hour)); err != nil {
			t.Fatal(err)
		}
		late := claimEvidencePlan(t, "late-source", "current-order", "用户喜欢散步", 0.1, ContextProjection{})
		if err := persist("late-source", late, at.Add(-time.Hour)); err != nil {
			t.Fatal(err)
		}
		if after := current(t, "current-order", "用户喜欢散步"); !reflect.DeepEqual(before, after) {
			t.Fatalf("old evaluation rewrote current: %#v", after)
		}
		versions := audit(t, stringValue(before["id"]))
		if len(versions) != 3 || versions[2]["applied"] != false {
			t.Fatalf("late evaluation not retained as unapplied: %#v", versions)
		}
	})
	t.Run("omitted_repeat_does_not_reject_an_active_claim", func(t *testing.T) {
		plan := claimEvidencePlan(t, "omitted-source", "omitted-key", "用户喜欢阅读", 0.8, ContextProjection{})
		if err := persist("omitted-source", plan, at); err != nil {
			t.Fatal(err)
		}
		before := current(t, "omitted-key", "用户喜欢阅读")
		hypotheses, err := app.readActiveHypotheses(ctx, "claim-history-role")
		if err != nil {
			t.Fatal(err)
		}
		repeated := claimEvidencePlan(t, "omitted-source", "omitted-key", "用户喜欢阅读", 0.8, ContextProjection{Hypotheses: hypotheses})
		if len(arrayValue(repeated["omitted_claims"])) != 1 {
			t.Fatal("fixture did not reach omitted repeat")
		}
		if err := persist("omitted-source", repeated, at.Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
		if after := current(t, "omitted-key", "用户喜欢阅读"); !reflect.DeepEqual(before, after) {
			t.Fatalf("omission overwrote current %#v", after)
		}
	})
	t.Run("pre_upgrade_identity_and_evidence_remain_a_honest_legacy_snapshot", func(t *testing.T) {
		if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.cognition_claims(id,fluctlight_id,source_fact_id,claim_type,content,evidence_refs,confidence,repetition_key,status,expires_at) VALUES('preserved-legacy-id','claim-history-role','legacy-source','observed_fact','升级前既有判断','["legacy-source"]',0.2,'legacy-alias','uncertain',$1)`, at.Add(7*24*time.Hour)); err != nil {
			t.Fatal(err)
		}
		plan := claimEvidencePlan(t, "legacy-new-source", "legacy-alias", "升级前既有判断", 0.8, ContextProjection{})
		if err := persist("legacy-new-source", plan, at.Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
		got := current(t, "legacy-alias", "升级前既有判断")
		if got["id"] != "preserved-legacy-id" || got["source_fact_id"] != "legacy-new-source" {
			t.Fatalf("legacy identity/current wrong: %#v", got)
		}
		versions := audit(t, "preserved-legacy-id")
		if len(versions) != 2 || versions[0]["origin"] != "legacy_current" || mapValue(versions[0]["snapshot"])["source_fact_id"] != "legacy-source" || versions[1]["origin"] != "evaluated" {
			t.Fatalf("legacy provenance %#v", versions)
		}
		var unknown bool
		if err := repo.Pool().QueryRow(ctx, `SELECT business_at IS NULL FROM public.cognition_claim_evaluations WHERE claim_id='preserved-legacy-id' AND origin='legacy_current'`).Scan(&unknown); err != nil || !unknown {
			t.Fatalf("legacy time fabricated: %v", err)
		}
	})
	t.Run("expired_or_rejected_claims_need_fresh_evidence_to_reactivate", func(t *testing.T) {
		for _, state := range []string{"expired", "rejected"} {
			key, content, source := "terminal-"+state, "终态证据测试 "+state, "source-"+state
			first := claimEvidencePlan(t, source, key, content, 0.2, ContextProjection{})
			firstAt := at.Add(-8 * 24 * time.Hour)
			if state == "rejected" {
				first = map[string]any{"omitted_claims": []any{map[string]any{"kind": ClaimUnsupportedSelf, "content": content, "confidence": 0.9, "evidence_refs": []any{}, "repetition_key": key, "reason_code": "unsupported_self_claim"}}}
				firstAt = at
			}
			if err := persist(source, first, firstAt); err != nil {
				t.Fatal(err)
			}
			before := current(t, key, content)
			hypotheses, err := app.readActiveHypotheses(ctx, "claim-history-role")
			if err != nil {
				t.Fatal(err)
			}
			for _, h := range hypotheses {
				if h["id"] == before["id"] {
					t.Fatalf("terminal claim returned %#v", h)
				}
			}
			if err := persist(source, first, at.Add(time.Hour)); err != nil {
				t.Fatal(err)
			}
			if after := current(t, key, content); !reflect.DeepEqual(before, after) {
				t.Fatalf("terminal replay revived %#v", after)
			}
			if state == "expired" {
				index := ContextReferenceIndex{ByRef: map[string]ContextReference{}}
				ref, err := index.add(ContextReferenceOutcome, source, 0, map[string]any{"summary": "same old evidence"})
				if err != nil {
					t.Fatal(err)
				}
				stale, err := normalizeResponsePlan(map[string]any{"action_type": "reply", "claims": []any{map[string]any{"kind": ClaimObservedFact, "content": content, "confidence": 0.9, "repetition_key": key, "evidence_refs": []any{ref}}}}, "new-attempt-with-old-evidence", ContextProjection{ReferenceIndex: index})
				if err != nil {
					t.Fatal(err)
				}
				if err := persist("new-attempt-with-old-evidence", stale, at.Add(2*time.Hour)); err != nil {
					t.Fatal(err)
				}
				if after := current(t, key, content); !reflect.DeepEqual(before, after) {
					t.Fatalf("old support renewed a terminal claim %#v", after)
				}
			}
			fresh := claimEvidencePlan(t, "fresh-"+source, key, content, 0.8, ContextProjection{})
			if err := persist("fresh-"+source, fresh, at.Add(3*time.Hour)); err != nil {
				t.Fatal(err)
			}
			if got := current(t, key, content); got["status"] != "active" || got["source_fact_id"] != "fresh-"+source {
				t.Fatalf("valid fresh evidence still blocked %#v", got)
			}
		}
	})
	t.Run("two_distinct_facts_in_one_plan_keep_identity_when_replayed_in_reverse_order", func(t *testing.T) {
		makePlan := func(contents []string) map[string]any {
			raw := []any{}
			for _, content := range contents {
				raw = append(raw, map[string]any{"kind": ClaimObservedFact, "content": content, "repetition_key": "same-plan-alias", "confidence": 0.8, "evidence_refs": []any{"current_message"}})
			}
			plan, err := normalizeResponsePlan(map[string]any{"action_type": "reply", "claims": raw}, "same-plan-source", ContextProjection{})
			if err != nil {
				t.Fatal(err)
			}
			return plan
		}
		if err := persist("same-plan-source", makePlan([]string{"同批不同事实一", "同批不同事实二"}), at); err != nil {
			t.Fatal(err)
		}
		first, second := current(t, "same-plan-alias", "同批不同事实一"), current(t, "same-plan-alias", "同批不同事实二")
		if err := persist("same-plan-source", makePlan([]string{"同批不同事实二", "同批不同事实一"}), at.Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(first, current(t, "same-plan-alias", "同批不同事实一")) || !reflect.DeepEqual(second, current(t, "same-plan-alias", "同批不同事实二")) {
			t.Fatal("reordered replay changed identities")
		}
		if len(audit(t, stringValue(first["id"]))) != 1 || len(audit(t, stringValue(second["id"]))) != 1 {
			t.Fatal("reordered replay duplicated evaluations")
		}
	})
	t.Run("historical_support_rotation_or_subset_cannot_renew_a_claim", func(t *testing.T) {
		for _, mode := range []string{"rotate", "subset"} {
			key, content, aSource, bSource := "history-"+mode, "历史支持轮换 "+mode, "history-A-"+mode, "history-B-"+mode
			first := claimEvidencePlan(t, aSource, key, content, 0.2, ContextProjection{})
			if err := persist(aSource, first, at.Add(-10*24*time.Hour)); err != nil {
				t.Fatal(err)
			}
			second := claimEvidencePlan(t, bSource, key, content, 0.2, ContextProjection{})
			index := ContextReferenceIndex{ByRef: map[string]ContextReference{}}
			ref, err := index.add(ContextReferenceOutcome, aSource, 0, map[string]any{"summary": "previously used support"})
			if err != nil {
				t.Fatal(err)
			}
			if mode == "subset" {
				second, err = normalizeResponsePlan(map[string]any{"action_type": "reply", "claims": []any{map[string]any{"kind": ClaimObservedFact, "content": content, "confidence": 0.2, "repetition_key": key, "evidence_refs": []any{ref, "current_message"}}}}, bSource, ContextProjection{ReferenceIndex: index})
				if err != nil {
					t.Fatal(err)
				}
			}
			if err := persist(bSource, second, at.Add(-9*24*time.Hour)); err != nil {
				t.Fatal(err)
			}
			before := current(t, key, content)
			recycled, err := normalizeResponsePlan(map[string]any{"action_type": "reply", "claims": []any{map[string]any{"kind": ClaimObservedFact, "content": content, "confidence": 0.9, "repetition_key": key, "evidence_refs": []any{ref}}}}, "reused-old-support-"+mode, ContextProjection{ReferenceIndex: index})
			if err != nil {
				t.Fatal(err)
			}
			if len(arrayValue(recycled["approved_claims"])) != 1 {
				t.Fatal("fixture must reach an approved claim with visible old support")
			}
			if err := persist("reused-old-support-"+mode, recycled, at); err != nil {
				t.Fatal(err)
			}
			if after := current(t, key, content); !reflect.DeepEqual(before, after) {
				t.Fatalf("old %s support renewed/revived current: before=%#v after=%#v", mode, before, after)
			}
			versions := audit(t, stringValue(before["id"]))
			if len(versions) != 3 || versions[2]["applied"] != false {
				t.Fatalf("reused support not retained as unapplied %#v", versions)
			}
			freshSource := "genuinely-new-support-" + mode
			fresh := claimEvidencePlan(t, freshSource, key, content, 0.8, ContextProjection{})
			if err := persist(freshSource, fresh, at.Add(time.Hour)); err != nil {
				t.Fatal(err)
			}
			if got := current(t, key, content); got["source_fact_id"] != freshSource || got["status"] != "active" || !reflect.DeepEqual(got["evidence_refs"], []any{freshSource}) {
				t.Fatalf("new support blocked or old support unioned into current: %#v", got)
			}
		}
	})
	t.Run("rollback_removes_both_current_and_evaluation", func(t *testing.T) {
		err := withTransaction(ctx, repo.Pool(), func(tx pgx.Tx) error {
			plan := claimEvidencePlan(t, "rollback-source", "rollback-key", "回滚测试事实", 0.8, ContextProjection{})
			if err := persistClaimsTx(ctx, tx, "claim-history-role", "rollback-source", plan, at); err != nil {
				return err
			}
			return errors.New("injected final settlement failure")
		})
		if err == nil {
			t.Fatal("expected failure")
		}
		var count int
		if err := repo.Pool().QueryRow(ctx, `SELECT (SELECT count(*) FROM public.cognition_claims WHERE fluctlight_id='claim-history-role' AND content='回滚测试事实')+(SELECT count(*) FROM public.cognition_claim_evaluations WHERE source_fact_id='rollback-source')`).Scan(&count); err != nil || count != 0 {
			t.Fatalf("partial settlement survived: count=%d err=%v", count, err)
		}
	})
}
