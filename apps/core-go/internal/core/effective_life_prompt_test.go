package core

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestFormalWakeUpFinalProviderRequestUsesCurrentSharedBodyAndWearing(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID := "effective-wakeup-owner", "effective-wakeup-fluctlight"
	seedLifeContextFluctlight(t, ctx, repository, ownerID, fluctlightID)
	foundation := map[string]any{"personality_system": map[string]any{"active_profile_id": "default"}, "life_profile": map[string]any{
		"appearance": map[string]any{"physical_features": map[string]any{"hair_length": "long"},
			"wardrobe_items": []any{map[string]any{"category": "shirt", "slot": "top", "description": "白衬衫", "ownership": "unknown", "available": true, "currently_worn": true}}},
	}}
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error { return initializeEffectiveLifeTx(ctx, tx, fluctlightID, foundation) }); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.fluctlight_appearance_states SET revision=1,state_json='{"hair_length":{"status":"known","value":"short"}}',source_kind='activity_result',source_ref='completed-haircut' WHERE fluctlight_id=$1`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	baseApp := &App{DB: repository}
	initialLife := currentLifeForTest(t, ctx, baseApp, fluctlightID, time.Now().UTC())
	if _, err := baseApp.AcceptSchedule(ctx, ownerID, fluctlightID, fullDaySchedulePayloadForTest(time.Now().UTC(), "effective-wakeup-schedule", stringValue(initialLife["context_revision"]))); err != nil {
		t.Fatal(err)
	}
	seedCognitiveProviderRole(t, ctx, repository, "effective-wakeup-endpoint")
	var finalWire string
	router := newFakeProviderRouter().on("wake_up_response", func(payload map[string]any) fakeProviderResult {
		finalWire = jsonString(payload)
		if os.Getenv("YAOGUANG_CAPTURE_WIRE") == "1" {
			t.Logf("WIRE_WAKE_SHORT=%s", finalWire)
		}
		return fakeProviderResult{Structured: map[string]any{"action_type": "no_op", "response_intent": "当前无须行动", "evidence_refs": []any{}, "influences": []any{}}}
	})
	app := newTestApp(t, repository, router)
	result, err := app.ProcessWakeUp(ctx, fluctlightID, 1)
	if err != nil {
		t.Fatalf("formal WakeUp did not finish: result=%#v err=%v", result, err)
	}
	if !strings.Contains(finalWire, "value: short") || strings.Contains(finalWire, "value: long") || !strings.Contains(finalWire, "白衬衫") {
		t.Fatalf("final WakeUp Provider request used an old body or omitted actual wearing: %s", finalWire)
	}
}

func TestVisualIdentityInitializationUsesCurrentBodyAfterHaircut(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID := "visual-current-owner", "visual-current-fluctlight"
	seedLifeContextFluctlight(t, ctx, repository, ownerID, fluctlightID)
	foundation := map[string]any{
		"identity": map[string]any{"name": "摇光", "gender": "female", "body_type": "C cup from old card", "visible_text": "long hair from initial card"},
		"life_profile": map[string]any{"daily_outfit_preferences": "初始白衬衫", "physical_traits": map[string]any{"chest_cup": "C"}, "appearance": map[string]any{
			"physical_features": map[string]any{"hair_length": "long"},
			"chest_cup":         "C",
			"wardrobe_items":    []any{map[string]any{"category": "shirt", "slot": "top", "description": "白衬衫", "ownership": "owned", "available": true, "currently_worn": true}},
		}},
		"extensions": map[string]any{"core_persona.life_profile.appearance.description": "long hair from old extension"},
	}
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error { return initializeEffectiveLifeTx(ctx, tx, fluctlightID, foundation) }); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Pool().Exec(ctx, `UPDATE public.fluctlight_appearance_states SET revision=1,state_json='{"hair_length":{"status":"known","value":"short"}}',source_kind='activity_result',source_ref='completed-haircut' WHERE fluctlight_id=$1`, fluctlightID); err != nil {
		t.Fatal(err)
	}
	app := &App{DB: repository}
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		_, err := app.ensureVisualIdentityInitializationTx(ctx, tx, fluctlightID, "wake_up", "test-wakeup", foundation)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	var raw, constraintsRaw []byte
	if err := repository.Pool().QueryRow(ctx, `SELECT identity_snapshot,renderer_constraints FROM public.fluctlight_visual_identities WHERE fluctlight_id=$1`, fluctlightID).Scan(&raw, &constraintsRaw); err != nil {
		t.Fatal(err)
	}
	snapshot := decodeObject(raw)
	appearance := mapValue(mapValue(snapshot["life_profile"])["appearance"])
	physical := mapValue(appearance["physical_features"])
	if stringValue(physical["hair_length"]) != "short" || !strings.Contains(jsonString(appearance), "白衬衫") || strings.Contains(jsonString(snapshot), "long hair") || strings.Contains(jsonString(snapshot), "初始白衬衫") || strings.Contains(jsonString(snapshot), "C cup from old card") || stringValue(decodeObject(constraintsRaw)["chest_cup"]) == "C" {
		t.Fatalf("visual initialization reused mutable Foundation text: %#v", snapshot)
	}
}

func TestVisualEffectiveLifeLockKeepsWearingAndRevisionTogether(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID := "visual-lock-owner", "visual-lock-fluctlight"
	seedLifeContextFluctlight(t, ctx, repository, ownerID, fluctlightID)
	foundation := map[string]any{"life_profile": map[string]any{"appearance": map[string]any{
		"wardrobe_items": []any{map[string]any{"category": "shirt", "slot": "top", "description": "白衬衫", "ownership": "owned", "available": true, "currently_worn": true}},
	}}}
	if err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error { return initializeEffectiveLifeTx(ctx, tx, fluctlightID, foundation) }); err != nil {
		t.Fatal(err)
	}
	reader, err := repository.Pool().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Rollback(ctx)
	if err := lockEffectiveLifeSnapshotTx(ctx, reader, fluctlightID); err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	finished := make(chan error, 1)
	go func() {
		close(started)
		finished <- withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
			var revision int
			if err := tx.QueryRow(ctx, `SELECT revision FROM public.fluctlight_wardrobe_states WHERE fluctlight_id=$1 FOR UPDATE`, fluctlightID).Scan(&revision); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `DELETE FROM public.fluctlight_worn_items WHERE fluctlight_id=$1`, fluctlightID); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, `UPDATE public.fluctlight_wardrobe_states SET revision=revision+1 WHERE fluctlight_id=$1`, fluctlightID)
			return err
		})
	}()
	<-started
	before, _, beforeWardrobe, err := readEffectiveLifeSnapshotWith(ctx, reader, fluctlightID, time.Now().UTC())
	if err != nil || beforeWardrobe != 0 || len(arrayValue(before["worn_items"])) != 1 {
		t.Fatalf("locked visual snapshot mixed versions: %#v revision=%d err=%v", before, beforeWardrobe, err)
	}
	if err := reader.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("wardrobe writer did not finish after visual snapshot released")
	}
	after, _, afterWardrobe, err := (&App{DB: repository}).readEffectiveLifeSnapshot(ctx, fluctlightID, time.Now().UTC())
	if err != nil || afterWardrobe != 1 || len(arrayValue(after["worn_items"])) != 0 {
		t.Fatalf("subsequent visual snapshot missed committed wear change: %#v revision=%d err=%v", after, afterWardrobe, err)
	}
}
