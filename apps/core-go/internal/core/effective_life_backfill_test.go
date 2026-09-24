package core

import (
	"strings"
	"testing"
)

func TestEffectiveLifeBackfillPreviewsAndPreservesLaterFacts(t *testing.T) {
	fixture := newIndependentToolE2EFixture(t, "effective-life-backfill")
	foundation := map[string]any{
		"identity":           map[string]any{"name": "摇光"},
		"personality_system": map[string]any{"active_profile_id": "default"},
		"life_profile": map[string]any{"appearance": map[string]any{
			"description":              "最初留长发",
			"physical_features":        map[string]any{"hair_length": "long"},
			"daily_outfit_preferences": []any{"喜欢靴子"},
			"wardrobe_items":           []any{map[string]any{"category": "shirt", "slot": "top", "description": "初始白衬衫", "ownership": "unknown", "available": true, "currently_worn": true}},
		}, "life_habits": []any{"习惯散步"}},
	}
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `UPDATE public.fluctlights SET core_persona=$2,life_profile=$3 WHERE id=$1`, fixture.fluctlightID, jsonBytes(foundation), jsonBytes(mapValue(foundation["life_profile"]))); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `DELETE FROM public.fluctlight_profile_habits WHERE fluctlight_id=$1`, fixture.fluctlightID); err != nil {
		t.Fatal(err)
	}
	preview, err := fixture.app.BackfillEffectiveLife(fixture.ctx, fixture.ownerID, []string{fixture.fluctlightID}, false, false)
	if err != nil || len(preview) != 1 || preview[0].Status != "would_initialize" || len(preview[0].Missing) != 3 {
		t.Fatalf("preview=%#v err=%v", preview, err)
	}
	var stateRows int
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.fluctlight_wardrobe_states WHERE fluctlight_id=$1`, fixture.fluctlightID).Scan(&stateRows); err != nil || stateRows != 0 {
		t.Fatalf("preview wrote domain state: count=%d err=%v", stateRows, err)
	}
	if err := fixture.app.VerifyEffectiveLifeReady(fixture.ctx); err == nil || !strings.Contains(err.Error(), "effective_life_not_ready") {
		t.Fatalf("startup accepted unmigrated effective life: %v", err)
	}
	applied, err := fixture.app.BackfillEffectiveLife(fixture.ctx, fixture.ownerID, []string{fixture.fluctlightID}, false, true)
	if err != nil || len(applied) != 1 || applied[0].Status != "initialized" {
		t.Fatalf("apply=%#v err=%v", applied, err)
	}
	if err := fixture.app.VerifyEffectiveLifeReady(fixture.ctx); err != nil {
		t.Fatalf("migrated instance not ready: %v", err)
	}
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `UPDATE public.fluctlight_appearance_states SET revision=1,state_json='{"hair_length":{"status":"known","value":"short"}}' WHERE fluctlight_id=$1`, fixture.fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `UPDATE public.fluctlight_wardrobe_items SET availability='lost',revision=revision+1 WHERE fluctlight_id=$1 AND category='shirt'`, fixture.fluctlightID); err != nil {
		t.Fatal(err)
	}
	replayed, err := fixture.app.BackfillEffectiveLife(fixture.ctx, fixture.ownerID, []string{fixture.fluctlightID}, false, true)
	if err != nil || replayed[0].Status != "skipped" {
		t.Fatalf("backfill rerun=%#v err=%v", replayed, err)
	}
	var body []byte
	var availability string
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT state_json FROM public.fluctlight_appearance_states WHERE fluctlight_id=$1`, fixture.fluctlightID).Scan(&body); err != nil {
		t.Fatal(err)
	}
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT availability FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND category='shirt'`, fixture.fluctlightID).Scan(&availability); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "short") || strings.Contains(string(body), "long") || availability != "lost" {
		t.Fatalf("initial source restored a superseded current fact: body=%s availability=%s", body, availability)
	}
}
