package core

import (
	"errors"
	"testing"
)

func TestRuntimeSummaryLegacyAndMissingSnapshot(t *testing.T) {
	ctx, repo, _, fid, _ := seedConversationSummaryAuthority(t)
	seedCognitiveProviderRole(t, ctx, repo, "runtime-summary-legacy-provider")
	const intentID = "runtime-summary-legacy"
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload,status) VALUES($1,$1,'lifecycle','conversation.summary',jsonb_build_object('fluctlight_id',$2::text),'pending')`, intentID, fid); err != nil {
		t.Fatal(err)
	}
	app := &App{DB: repo, Provider: &ProviderClient{DB: repo}}
	route := withAgentModelRoute(ctx, FormalAgentConversationSummary, runtimeSummaryModelVariant)
	assignment, frozen, err := app.runtimeSummaryAssignment(route, intentID)
	if err != nil || frozen || assignment.ModelID != "turn-model" {
		t.Fatalf("legacy summary did not keep its existing route: model=%q frozen=%t err=%v", assignment.ModelID, frozen, err)
	}
	var diagnostics int
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.diagnostic_events WHERE event_type='runtime_summary.model_binding.legacy_unfrozen' AND correlation_id=$1`, intentID).Scan(&diagnostics); err != nil || diagnostics != 1 {
		t.Fatalf("legacy summary was not diagnosed: count=%d err=%v", diagnostics, err)
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET payload=payload||jsonb_build_object($2::text,'v1') WHERE intent_id=$1`, intentID, runtimeSummaryModelContractKey); err != nil {
		t.Fatal(err)
	}
	if _, _, err := app.runtimeSummaryAssignment(route, intentID); !errors.Is(err, errRuntimeSummaryModelSnapshotInvalid) {
		t.Fatalf("missing new summary snapshot silently selected a model: %v", err)
	}
}
