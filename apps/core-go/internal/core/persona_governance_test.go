package core

import (
	"fmt"
	"github.com/jackc/pgx/v5"
	"net/url"
	"sync"
	"testing"
)

// Real PostgreSQL commands only. These cases must not call a model/worker.
func TestOwnerFoundationProposalConcurrentReplayAndSnapshot(t *testing.T) {
	ctx, repo, app, owner, fid, _ := setupDirectPublicationToolTest(t, "foundation-governance")
	current, err := app.DB.GetFluctlight(ctx, fid, owner)
	if err != nil {
		t.Fatal(err)
	}
	body := map[string]any{"changes": map[string]any{"identity": map[string]any{"name": "FOUNDATION-MANAGEMENT-TEST"}}, "expected_revision": current.CurrentRevision, "reason": "verify immutable baseline and retry", "idempotency_key": "same-command"}
	results := make(chan map[string]any, 6)
	failures := make(chan error, 6)
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := app.ProposeFoundation(ctx, owner, fid, body)
			results <- result
			failures <- err
		}()
	}
	wg.Wait()
	close(results)
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	id := ""
	for result := range results {
		if id == "" {
			id = stringValue(result["id"])
		}
		if result["id"] != id {
			t.Fatal("duplicate proposal", result)
		}
	}
	row, err := app.OwnerGovernanceObject(ctx, owner, fid, "foundation", id)
	if err != nil || intValue(mapValue(row["base_snapshot"])["revision"]) != current.CurrentRevision {
		t.Fatalf("snapshot %#v %v", row, err)
	}
	changed := cloneMap(body)
	changed["reason"] = "different reason"
	if _, err := app.ProposeFoundation(ctx, owner, fid, changed); err == nil {
		t.Fatal("key accepted a different command")
	}
	after, err := app.DB.GetFluctlight(ctx, fid, owner)
	if err != nil || after.CurrentRevision != current.CurrentRevision || jsonString(after.Identity) != jsonString(current.Identity) {
		t.Fatal("proposal changed effective identity", err)
	}
	// Multiple rejected proposals can share the same proposed revision. Their
	// history is paged by object identity, not ambiguously selected by revision.
	for i := 0; i < 23; i++ {
		next := cloneMap(body)
		next["idempotency_key"] = fmt.Sprint("page-", i)
		if _, err := app.ProposeFoundation(ctx, owner, fid, next); err != nil {
			t.Fatal(err)
		}
	}
	seen := map[string]bool{}
	cursor := ""
	for {
		page, err := app.OwnerGovernanceCatalog(ctx, owner, fid, "foundation", url.Values{"limit": {"7"}, "cursor": {cursor}})
		if err != nil {
			t.Fatal(err)
		}
		for _, raw := range arrayValue(page["items"]) {
			row := mapValue(raw)
			rid := stringValue(row["id"])
			if seen[rid] {
				t.Fatal("duplicate page")
			}
			seen[rid] = true
		}
		cursor = stringValue(page["next_cursor"])
		if cursor == "" {
			break
		}
	}
	if len(seen) != 24 {
		t.Fatalf("truncated proposals %d", len(seen))
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.fluctlights SET current_revision=current_revision+1 WHERE id=$1`, fid); err != nil {
		t.Fatal(err)
	}
	if _, err := app.SetFoundationDecisionExpected(ctx, owner, fid, id, "accept", "stale proposal", nil); err == nil {
		t.Fatal("stale proposal accepted")
	}
}

func TestOwnerCapabilityReviewVersionHistoryReplayAndRegisteredVersion(t *testing.T) {
	ctx, repo, app, owner, fid, _ := setupDirectPublicationToolTest(t, "capability-governance")
	if err := app.persistCapabilityRequest(ctx, "capability-management-test", fid, "explicit-management-fixture", "capability.request", "能力申请测试", "管理闭环", "管理回归测试", map[string]any{}, "internal", "normal", []any{"fixture:owner-review"}, "review-fixture"); err != nil {
		t.Fatal(err)
	}
	path := "/internal/fluctlights/" + fid + "/capability-requests/capability-management-test/review"
	command := func(status, key string, revision int, version string) (map[string]any, error) {
		body := map[string]any{"status": status, "note": "review test", "expected_revision": revision, "idempotency_key": key}
		if version != "" {
			body["capability_version"] = version
		}
		return app.DispatchOwnerSurface(ctx, "POST", path, nil, owner, body)
	}
	results := make(chan error, 6)
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := command("accepted", "same-review", 1, "")
			if err == nil && intValue(r["revision"]) != 2 {
				err = fmt.Errorf("unexpected receipt %#v", r)
			}
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err := command("rejected", "stale-review", 1, ""); err == nil {
		t.Fatal("stale review accepted")
	}
	if _, err := command("fulfilled", "wrong-version", 2, "unregistered-version"); err == nil {
		t.Fatal("unregistered version fulfilled")
	}
	r, err := command("fulfilled", "registered-version", 2, "v1")
	if err != nil || r["status"] != "fulfilled" || intValue(r["revision"]) != 3 {
		t.Fatalf("fulfillment %#v %v", r, err)
	}
	if _, err := command("rejected", "terminal-rewrite", 3, ""); err == nil {
		t.Fatal("terminal state rewritten")
	}
	history, err := app.OwnerGovernanceHistory(ctx, owner, fid, "capability", "capability-management-test", url.Values{})
	if err != nil || len(arrayValue(history["items"])) != 2 {
		t.Fatalf("history %#v %v", history, err)
	}
	var events int
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.platform_outbox_events WHERE kind='capability.request.reviewed' AND aggregate_id='capability-management-test'`).Scan(&events); err != nil || events != 2 {
		t.Fatalf("duplicate/absent events %d %v", events, err)
	}
}

func TestReflectionCanonicalSelfOutcomeRetainsActorDependencies(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	factID := recordActorFact(t, f, "self-source", "assert", "location_scope", "remote", "")
	proposal := ReflectionProposalV2{DevelopingSelfCandidates: []ReflectionDevelopingSelfCandidateV2{{Category: " preference ", Claim: " 我喜欢阅读 ", Confidence: .9, EvidenceRefs: []string{"evidence-a", "evidence-b"}}}}
	plan := ReflectionEvolutionPlan{ProposalID: "self-canonical", SourceWindow: "canonical-window", Candidates: []EvolutionCandidatePlan{{Domain: EvolutionDevelopingSelf, Index: 0, Disposition: EvolutionAccepted}}}
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		return f.app.applyReflectionDevelopingSelfV2Tx(f.ctx, tx, f.fluctlightID, proposal, &plan, EvolutionContext{}, []map[string]any{{"fact_id": factID}})
	}); err != nil {
		t.Fatal(err)
	}
	var id string
	var revision int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT id,revision FROM public.fluctlight_developing_self_claims WHERE fluctlight_id=$1 AND category='preference' AND claim='我喜欢阅读'`, f.fluctlightID).Scan(&id, &revision); err != nil {
		t.Fatal(err)
	}
	var deps int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.actor_fact_artifacts WHERE artifact_kind='developing_self' AND artifact_id=$1 AND artifact_revision=$2 AND fact_id=$3`, id, revision, factID).Scan(&deps); err != nil || deps != 1 {
		t.Fatal("lost normalized dependency", deps, err)
	}
	if _, err := f.app.legacyOwnerSelfCommand(f.ctx, f.ownerID, id, "forget", revision, "explicit forget"); err != nil {
		t.Fatal(err)
	}
	plan.Candidates[0].Disposition = EvolutionAccepted
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		return f.app.applyReflectionDevelopingSelfV2Tx(f.ctx, tx, f.fluctlightID, proposal, &plan, EvolutionContext{}, []map[string]any{{"fact_id": factID}})
	}); err != nil {
		t.Fatal(err)
	}
	if plan.Candidates[0].Disposition != EvolutionRejected || plan.Candidates[0].ReasonCode != "self_claim_inactive" {
		t.Fatalf("false acceptance %#v", plan.Candidates[0])
	}
}
