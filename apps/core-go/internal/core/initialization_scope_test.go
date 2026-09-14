package core

import (
	"testing"

	"github.com/jackc/pgx/v5"
)

// TestInitializationRelationshipRoleKeepsDeclaredStringLabel pins the R11 fix:
// a card that states the role as prose ("朋友") must keep that label instead of
// falling back to "unknown" because no type/relationship_type/intimacy key was
// declared. The assertion is on the exact field path, not on object keywords.
func TestInitializationRelationshipRoleKeepsDeclaredStringLabel(t *testing.T) {
	result := map[string]any{
		"initial_relationships": []any{
			map[string]any{"target_actor_id": "actor_user", "role": "朋友"},
			map[string]any{"target_actor_id": "actor_user", "role": "同事", "type": "colleague"},
			map[string]any{"target_actor_id": "actor_user"},
		},
	}
	normalizeInitializationAliases(result)

	relationships := arrayValue(result["initial_relationships"])
	if len(relationships) != 3 {
		t.Fatalf("relationships were dropped: %#v", relationships)
	}
	if label := stringValue(mapValue(mapValue(relationships[0])["role"])["label"]); label != "朋友" {
		t.Fatalf("a declared string role was discarded: role.label = %q", label)
	}
	// A structured role object still wins over the sibling shorthand key.
	if label := stringValue(mapValue(mapValue(relationships[1])["role"])["label"]); label != "同事" {
		t.Fatalf("declared role object was not preserved: role.label = %q", label)
	}
	// Nothing declared at all still yields the explicit unknown fallback.
	if label := stringValue(mapValue(mapValue(relationships[2])["role"])["label"]); label != "unknown" {
		t.Fatalf("missing role should stay explicitly unknown, got %q", label)
	}
}

// TestInitializationScopeDefaultsToSharedRow pins the second R11 fix: an item
// that declares no profile_id is a SHARED row (SQL NULL), not a row bound to the
// initial profile. Without this, a shared goal/relationship disappears from every
// profile once the instance switches or a takeover happens.
func TestInitializationScopeDefaultsToSharedRow(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	app := &App{DB: repository}
	ownerID, fluctlightID := "scope-owner", "scope-fluctlight"
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active'),($2,'fluctlight','active')`, ownerID, fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES($1,$2,'blank_slate','active','{}','{"timezone":"Asia/Shanghai"}','{}','{}','{}','{}')`, fluctlightID, ownerID); err != nil {
		t.Fatal(err)
	}
	profileIDs := map[string]struct{}{"spark": {}, "twilight": {}}

	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		if err := app.insertAgency(ctx, tx, fluctlightID, ownerID,
			[]any{
				map[string]any{"description": "共享目标"},
				map[string]any{"description": "暮光私有目标", "profile_id": "twilight"},
			},
			[]any{
				map[string]any{"action": "共享意图", "goal_index": 0},
				map[string]any{"action": "暮光私有意图", "goal_index": 1, "profile_id": "twilight"},
			},
			profileIDs); err != nil {
			return err
		}
		return app.insertRelationshipSeeds(ctx, tx, fluctlightID, ownerID, map[string]any{
			"initial_relationships": []any{
				map[string]any{"target_actor_id": "actor_user", "role": map[string]any{"label": "朋友"}},
				map[string]any{"target_actor_id": "actor_user", "role": map[string]any{"label": "搭档"}, "profile_id": "twilight"},
			},
		}, profileIDs)
	}); err != nil {
		t.Fatalf("initialization with a shared item was rejected: %v", err)
	}

	// Exact field-level checks: profile_id NULL for shared, the declared id for scoped.
	for _, check := range []struct {
		table       string
		id          string
		wantProfile string // "" means the shared scope (SQL NULL)
	}{
		{"public.fluctlight_goals", "goal_initial_" + fluctlightID + "_0", ""},
		{"public.fluctlight_goals", "goal_initial_" + fluctlightID + "_1", "twilight"},
		{"public.fluctlight_intentions", "intention_initial_" + fluctlightID + "_0", ""},
		{"public.fluctlight_intentions", "intention_initial_" + fluctlightID + "_1", "twilight"},
	} {
		var profileID *string
		if err := repository.Pool().QueryRow(ctx, `SELECT profile_id FROM `+check.table+` WHERE id=$1`, check.id).Scan(&profileID); err != nil {
			t.Fatalf("read %s %s: %v", check.table, check.id, err)
		}
		if check.wantProfile == "" {
			if profileID != nil {
				t.Fatalf("%s %s must use the shared scope (NULL), got %q", check.table, check.id, *profileID)
			}
			continue
		}
		if profileID == nil || *profileID != check.wantProfile {
			t.Fatalf("%s %s lost its declared profile scope: want %q got %v", check.table, check.id, check.wantProfile, profileID)
		}
	}

	rows, err := repository.Pool().Query(ctx, `SELECT profile_id FROM public.relationships WHERE owner_fluctlight_id=$1 ORDER BY id`, fluctlightID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var scopes []*string
	for rows.Next() {
		var profileID *string
		if err := rows.Scan(&profileID); err != nil {
			t.Fatal(err)
		}
		scopes = append(scopes, profileID)
	}
	if len(scopes) != 2 {
		t.Fatalf("expected two seeded relationships, got %d", len(scopes))
	}
	sharedCount := 0
	for _, scope := range scopes {
		if scope == nil {
			sharedCount++
		}
	}
	if sharedCount != 1 {
		t.Fatalf("exactly one relationship must be shared, got %d of %d", sharedCount, len(scopes))
	}
}

// TestInitializationRejectsUndeclaredProfileScope keeps the membership guard:
// an explicit profile_id that the persona never declares is still an error, so
// defaulting to shared did not disable scope validation.
func TestInitializationRejectsUndeclaredProfileScope(t *testing.T) {
	if _, err := initializationScopeProfileID(map[string]any{"profile_id": "ghost"}, "initial_relationship_profile_invalid", map[string]struct{}{"spark": {}}); err == nil {
		t.Fatal("an undeclared profile scope must still be rejected")
	}
	if profileID, err := initializationScopeProfileID(map[string]any{}, "unused", map[string]struct{}{"spark": {}}); err != nil || profileID != "" {
		t.Fatalf("absent profile_id must resolve to the shared scope, got %q / %v", profileID, err)
	}
}
