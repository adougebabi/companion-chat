package core

import (
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestSharedInitialPortraitDoesNotSelectAnotherProfile(t *testing.T) {
	core := map[string]any{"identity": map[string]any{"name": "shared"}, "personality_system": map[string]any{"active_profile_id": "default", "profiles": []any{map[string]any{"id": "day", "voice": map[string]any{"tone": "DAY_ONLY"}}, map[string]any{"id": "night", "voice": map[string]any{"tone": "NIGHT_ONLY"}}}}}
	source, err := personaCompilationSource(PersonaCompilationInput{CorePersona: core, ProfileID: "default"})
	if err != nil || strings.Contains(jsonString(source), "DAY_ONLY") || strings.Contains(jsonString(source), "NIGHT_ONLY") {
		t.Fatal(source, err)
	}
	if _, err := personaCompilationSource(PersonaCompilationInput{CorePersona: core, ProfileID: "undeclared"}); err == nil {
		t.Fatal("undeclared profile admitted")
	}
}

func TestSharedInitialProfileHasPersistedHabitsAndBackfillPreservesExisting(t *testing.T) {
	f := newIndependentToolE2EFixture(t, "shared-initial-profile")
	fl, err := f.repository.GetFluctlight(f.ctx, f.fluctlightID, f.ownerID)
	if err != nil {
		t.Fatal(err)
	}
	foundation := cloneMap(fl.CorePersona)
	foundation["personality_system"] = map[string]any{"mode": "multiple", "active_profile_id": "default", "profiles": []any{map[string]any{"id": "day"}, map[string]any{"id": "night"}}}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlights SET core_persona=$2 WHERE id=$1`, f.fluctlightID, jsonBytes(foundation)); err != nil {
		t.Fatal(err)
	}
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error { return initializeEffectiveLifeTx(f.ctx, tx, f.fluctlightID, foundation) }); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"default", "day", "night"} {
		if _, _, err := readProfileHabits(f.ctx, f.repository.Pool(), f.fluctlightID, id); err != nil {
			t.Fatalf("runtime %s: %v", id, err)
		}
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlight_profile_habits SET revision=3,habits_json='["current day habit"]' WHERE fluctlight_id=$1 AND profile_id='day'`, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `DELETE FROM public.fluctlight_profile_habits WHERE fluctlight_id=$1 AND profile_id='default'`, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	preview, err := f.app.BackfillEffectiveLife(f.ctx, f.ownerID, []string{f.fluctlightID}, false, false)
	if err != nil || len(preview) != 1 || len(preview[0].Missing) != 1 || preview[0].Missing[0] != "habits:default" {
		t.Fatal(preview, err)
	}
	if _, err := f.app.BackfillEffectiveLife(f.ctx, f.ownerID, []string{f.fluctlightID}, false, true); err != nil {
		t.Fatal(err)
	}
	habits, rev, err := readProfileHabits(f.ctx, f.repository.Pool(), f.fluctlightID, "day")
	if err != nil || rev != 3 || jsonString(habits) != `["current day habit"]` {
		t.Fatal(habits, rev, err)
	}
	if _, _, err := readProfileHabits(f.ctx, f.repository.Pool(), f.fluctlightID, "default"); err != nil {
		t.Fatal(err)
	}
}
