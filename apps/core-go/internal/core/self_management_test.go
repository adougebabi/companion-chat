package core

import (
	"fmt"
	"net/url"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestOwnerSelfFullPagingReviewForgetRestoreAndScopedReplay(t *testing.T) {
	ctx, repo, app, owner, fid, _ := setupDirectPublicationToolTest(t, "self-management")
	seeds := []any{}
	for i := 0; i < 205; i++ {
		seeds = append(seeds, map[string]any{"category": "preference", "claim": fmt.Sprintf("自我认识分页样本 %03d", i), "value": map[string]any{"choice": i}, "confidence": .8, "provenance": map[string]any{"source": "owner_defined"}, "status": "active"})
	}
	if err := withTransaction(ctx, repo.Pool(), func(tx pgx.Tx) error { return app.insertDevelopingSelfSeeds(ctx, tx, fid, seeds) }); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	cursor := ""
	target := ""
	for {
		page, err := app.OwnerSelfClaims(ctx, owner, fid, url.Values{"limit": {"37"}, "cursor": {cursor}, "status": {"all"}})
		if err != nil {
			t.Fatal(err)
		}
		for _, raw := range arrayValue(page["items"]) {
			row := mapValue(raw)
			id := stringValue(row["id"])
			if seen[id] || row["fluctlight_id"] != fid {
				t.Fatalf("duplicate/foreign claim %#v", row)
			}
			seen[id] = true
			target = id
		}
		cursor = stringValue(page["next_cursor"])
		if cursor == "" {
			break
		}
	}
	if len(seen) != 205 {
		t.Fatalf("truncated management read %d", len(seen))
	}
	command := func(operation, key string, expected, targetRevision int) (map[string]any, error) {
		body := map[string]any{"expected_revision": expected, "reason": "Owner governance integration", "idempotency_key": key}
		if operation == "restore" {
			body["target_revision"] = targetRevision
		}
		return app.DispatchOwnerSurface(ctx, "POST", "/internal/fluctlights/"+fid+"/self-claims/"+target+"/"+operation, nil, owner, body)
	}
	forgotten, err := command("forget", "forget-stable", 1, 0)
	if err != nil || intValue(forgotten["revision"]) != 2 {
		t.Fatalf("forget %#v %v", forgotten, err)
	}
	replay, err := command("forget", "forget-stable", 1, 0)
	if err != nil || intValue(replay["revision"]) != 2 {
		t.Fatalf("unknown-result replay %#v %v", replay, err)
	}
	archived, err := app.OwnerSelfClaims(ctx, owner, fid, url.Values{"status": {"forgotten"}})
	if err != nil || len(arrayValue(archived["items"])) != 1 {
		t.Fatalf("forgotten entity hidden %#v %v", archived, err)
	}
	for _, step := range []struct {
		key      string
		expected int
	}{{"restore-once", 2}, {"restore-again", 3}} {
		result, err := command("restore", step.key, step.expected, 1)
		if err != nil || intValue(result["revision"]) != step.expected+1 {
			t.Fatalf("restore %#v %v", result, err)
		}
	}
	legacy, err := app.RollbackDevelopingSelfClaim(ctx, owner, target, 4, "legacy replay keeps full snapshot")
	if err != nil || intValue(legacy["revision"]) != 5 {
		t.Fatalf("legacy rollback %#v %v", legacy, err)
	}
	legacy, err = app.RollbackDevelopingSelfClaim(ctx, owner, target, 4, "legacy replay keeps full snapshot")
	if err != nil || intValue(legacy["revision"]) != 5 || legacy["replayed"] != true {
		t.Fatalf("legacy replay %#v %v", legacy, err)
	}
	before, err := app.OwnerSelfClaim(ctx, owner, fid, target)
	if err != nil {
		t.Fatal(err)
	}
	review, err := command("review", "review-once", 5, 0)
	if err != nil || intValue(review["revision"]) != 5 {
		t.Fatalf("review changed claim version %#v %v", review, err)
	}
	after, err := app.OwnerSelfClaim(ctx, owner, fid, target)
	if err != nil || after["confidence"] != before["confidence"] || jsonString(after["value"]) != jsonString(before["value"]) || mapValue(after["owner_review"])["disposition"] != "acknowledged" {
		t.Fatalf("review changed truth/value %#v %v", after, err)
	}
	history, err := app.OwnerSelfHistory(ctx, owner, fid, target, "revisions", url.Values{})
	if err != nil || len(arrayValue(history["items"])) != 5 {
		t.Fatalf("lost revisions %#v %v", history, err)
	}
	var correctBefore bool
	if err := repo.Pool().QueryRow(ctx, `SELECT before_value->>'status'='active' AND after_value->>'status'='forgotten' AND candidate->>'reason'='Owner governance integration' FROM public.fluctlight_developing_self_revisions WHERE claim_id=$1 AND revision=2`, target).Scan(&correctBefore); err != nil || !correctBefore {
		t.Fatalf("forget before/after %v %v", correctBefore, err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES('self-other-owned','fluctlight','active')`); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) SELECT 'self-other-owned',created_by_actor_id,initialization_mode,'active',core_persona,identity,personality,behavioral_policy,life_profile,provenance FROM public.fluctlights WHERE id=$1`, fid); err != nil {
		t.Fatal(err)
	}
	if _, err := app.OwnerSelfClaim(ctx, owner, "self-other-owned", target); err == nil {
		t.Fatal("claim from another owned character accepted")
	}
	if _, err := app.DispatchOwnerSurface(ctx, "POST", "/internal/fluctlights/self-other-owned/self-claims/"+target+"/forget", nil, owner, map[string]any{"expected_revision": 5, "reason": "wrong path", "idempotency_key": "wrong-scope"}); err == nil {
		t.Fatal("path scope ignored")
	}
}
