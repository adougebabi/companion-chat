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
