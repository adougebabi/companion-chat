package core

import (
	"errors"
	"fmt"
	"testing"
)

func TestManagedMemoryCompletePagingGovernanceAndSourceHistory(t *testing.T) {
	ctx, repo := isolatedCoreTestRepository(t)
	owner, fid := "memory-life-owner", "memory-life-fluctlight"
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active'),($2,'fluctlight','active'),('memory-foreign-owner','human','active')`, owner, fid); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES($1,$2,'blank_slate','active','{}','{}','{}','{}','{}','{}')`, fid, owner); err != nil {
		t.Fatal(err)
	}
	app := &App{DB: repo}
	service := NewMemoryService(repo.Pool(), app)
	ids := map[string]bool{}
	for i := 0; i < 105; i++ {
		command := memoryLifecycleTestCommand(MemoryCreate, fmt.Sprintf("manage-%03d", i), &MemorySemanticInput{Type: "semantic", Content: fmt.Sprintf("分页记忆 %03d 与百分号%%和下划线_", i), Confidence: .8, Importance: .4}, nil, nil)
		command.ActorID = owner
		command.RequestDigest = memoryCommandDigest(command)
		result := applyMemoryLifecycleTestCommand(t, ctx, app, command)
		ids[result.MemoryID] = false
	}
	cursor := ""
	pages := 0
	var target map[string]any
	for {
		page, err := service.Browse(ctx, owner, fid, MemoryBrowseQuery{Limit: 17, Cursor: cursor})
		if err != nil {
			t.Fatal(err)
		}
		pages++
		for _, row := range page.Items {
			id := stringValue(row["id"])
			seen, ok := ids[id]
			if !ok || seen {
				t.Fatalf("duplicate/foreign memory %s", id)
			}
			ids[id] = true
			target = row
		}
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	if pages != 7 {
		t.Fatalf("pages=%d", pages)
	}
	for id, seen := range ids {
		if !seen {
			t.Fatalf("missing %s", id)
		}
	}
	first, err := service.Browse(ctx, owner, fid, MemoryBrowseQuery{Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Browse(ctx, owner, fid, MemoryBrowseQuery{Limit: 2, Cursor: first.NextCursor, Query: "other"}); !errors.Is(err, ErrMemoryQueryInvalid) {
		t.Fatalf("cross-filter cursor %v", err)
	}
	literal, err := service.Browse(ctx, owner, fid, MemoryBrowseQuery{Query: "号%", Limit: 100})
	if err != nil || len(literal.Items) != 100 || literal.NextCursor == "" {
		t.Fatalf("literal substring %d %v", len(literal.Items), err)
	}
	if _, err := service.Browse(ctx, "memory-foreign-owner", fid, MemoryBrowseQuery{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("foreign list %v", err)
	}
	id := stringValue(target["id"])
	if _, err := service.ReadManaged(ctx, "memory-foreign-owner", id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("foreign detail %v", err)
	}
	base := intValue(target["revision"])
	updated, err := app.ReviseMemory(ctx, owner, id, "修订后不再使用旧事实", &base, []any{"owner:分页管理校验"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.ReviseMemory(ctx, owner, id, "并发旧稿", &base, []any{"owner:并发校验"}); err == nil {
		t.Fatal("stale write accepted")
	}
	current, err := service.ReadManaged(ctx, owner, id)
	if err != nil || current["content"] != "修订后不再使用旧事实" || intValue(current["revision"]) != intValue(updated["revision"]) {
		t.Fatalf("readback %#v %v", current, err)
	}
	if _, exists := current["canonical_key"]; exists {
		t.Fatal("internal digest leaked")
	}
	history, err := service.Revisions(ctx, owner, id, "", 1)
	if err != nil || len(history.Items) != 1 || history.NextCursor == "" {
		t.Fatalf("history %#v %v", history, err)
	}
	older, err := service.Revisions(ctx, owner, id, history.NextCursor, 1)
	if err != nil || len(older.Items) != 1 || intValue(older.Items[0]["revision"]) != base {
		t.Fatalf("older history %#v %v", older, err)
	}
	newSources, err := service.Sources(ctx, owner, id, nil, "", 20)
	if err != nil || len(newSources.Items) != 1 || newSources.Items[0]["source_kind"] != "owner_confirmation" || newSources.Items[0]["live"] != true {
		t.Fatalf("new sources %#v %v", newSources, err)
	}
	oldSources, err := service.Sources(ctx, owner, id, &base, "", 20)
	if err != nil || len(oldSources.Items) != 1 || oldSources.Items[0]["source_ref"] == newSources.Items[0]["source_ref"] {
		t.Fatalf("source lineage %#v %v", oldSources, err)
	}
	rev := intValue(current["revision"])
	if _, err = app.ForgetMemory(ctx, owner, id, &rev, []any{"owner:不再召回"}); err != nil {
		t.Fatal(err)
	}
	forgotten, err := service.Browse(ctx, owner, fid, MemoryBrowseQuery{Status: "forgotten"})
	if err != nil || len(forgotten.Items) != 1 {
		t.Fatalf("forgotten browse %#v %v", forgotten, err)
	}
	rev = intValue(forgotten.Items[0]["revision"])
	if _, err = app.RollbackMemory(ctx, owner, id, base, rev, []any{"owner:补偿恢复"}); err != nil {
		t.Fatal(err)
	}
	restored, err := service.ReadManaged(ctx, owner, id)
	if err != nil || restored["status"] != "active" || restored["content"] != target["content"] || intValue(restored["revision"]) <= rev {
		t.Fatalf("restore %#v %v", restored, err)
	}
}

func TestManagedMemoryQueryRejectsInvalidLimitsAndCursor(t *testing.T) {
	for _, n := range []int{-1, 101} {
		if _, err := memoryPageLimit(n); err == nil {
			t.Fatalf("accepted %d", n)
		}
	}
	if _, err := decodeMemoryBrowseCursor("not-json", "scope"); err == nil {
		t.Fatal("bad cursor accepted")
	}
	if _, err := decodeMemoryBrowseCursor(encodeMemoryBrowseCursor(memoryBrowseCursor{Scope: "other"}), "scope"); err == nil {
		t.Fatal("scope mismatch accepted")
	}
}
