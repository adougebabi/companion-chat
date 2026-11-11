package core

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestGoalEvaluationSnapshotFencesClaimsAndNewRequests(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	seedCognitiveProviderRole(t, f.ctx, f.repository, "goal-snapshot-endpoint")
	role := agentModelBindingKey(FormalAgentGoalEvaluation, goalEvaluationModelVariant)
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,timeout_seconds) VALUES($1,'goal-snapshot-endpoint','first-model',30)`, role); err != nil {
		t.Fatal(err)
	}
	settingsKey := []byte("0123456789abcdef0123456789abcdef")
	f.app.Provider.SettingsKey = settingsKey
	const fixtureSecret = "test-goal-snapshot-secret"
	encrypted, err := encryptSecret(settingsKey, "turn-secret", fixtureSecret)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.setting_secrets(purpose,ciphertext,nonce) VALUES('turn-secret',$1,$2)`, encrypted.ciphertext, encrypted.nonce); err != nil {
		t.Fatal(err)
	}
	insert := func(id string) {
		t.Helper()
		if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.goal_evaluation_requests(id,fluctlight_id,profile_id,reason,status,claim_revision) VALUES($1,$2,'default','test','processing',1)`, id, f.fluctlightID); err != nil {
			t.Fatal(err)
		}
	}
	insert("goal-snapshot-first")
	route := withAgentModelRoute(f.ctx, FormalAgentGoalEvaluation, goalEvaluationModelVariant)
	request := goalEvaluationSnapshot{RequestID: "goal-snapshot-first", FluctlightID: f.fluctlightID, ClaimRevision: 1}
	first, err := f.app.goalEvaluationAssignment(route, request)
	if err != nil || first.ModelID != "first-model" || first.Secret != fixtureSecret {
		t.Fatalf("first goal binding unavailable: model=%q err=%v", first.ModelID, err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.model_roles SET model_id='next-model' WHERE role=$1`, role); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_evaluation_requests SET claim_revision=2 WHERE id=$1`, request.RequestID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.goalEvaluationAssignment(route, request); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale claim selected a model: %v", err)
	}
	request.ClaimRevision = 2
	restored, err := f.app.goalEvaluationAssignment(route, request)
	if err != nil || restored.ModelID != "first-model" || restored.Secret != fixtureSecret {
		t.Fatalf("same request did not restore its first model: model=%q err=%v", restored.ModelID, err)
	}
	var raw []byte
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT model_provenance FROM public.goal_evaluation_requests WHERE id=$1`, request.RequestID).Scan(&raw); err != nil || strings.Contains(string(raw), fixtureSecret) {
		t.Fatalf("goal snapshot exposed a credential or could not be read: %v", err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_evaluation_requests SET status='succeeded' WHERE id=$1`, request.RequestID); err != nil {
		t.Fatal(err)
	}
	insert("goal-snapshot-new")
	newRequest := goalEvaluationSnapshot{RequestID: "goal-snapshot-new", FluctlightID: f.fluctlightID, ClaimRevision: 1}
	newModel, err := f.app.goalEvaluationAssignment(route, newRequest)
	if err != nil || newModel.ModelID != "next-model" {
		t.Fatalf("new goal request did not use current configuration: model=%q err=%v", newModel.ModelID, err)
	}
	accepted := map[string]any{"submissions": map[string]any{"goal.evaluation.submit:goal-1:": map[string]any{"digest": "accepted", "output": map[string]any{"status": "completed"}}}}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_evaluation_requests SET model_provenance='{"_goal_evaluation_model_snapshot_contract":"v1"}',result=$2 WHERE id=$1`, newRequest.RequestID, jsonBytes(accepted)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.goalEvaluationAssignment(route, newRequest); !errors.Is(err, errGoalEvaluationModelSnapshotInvalid) {
		t.Fatalf("a request with accepted submissions silently replaced its missing snapshot: %v", err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_evaluation_requests SET model_provenance='{"_goal_evaluation_model_snapshot_contract":"v1","_goal_evaluation_model_selected":true}',result='{}' WHERE id=$1`, newRequest.RequestID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.goalEvaluationAssignment(route, newRequest); !errors.Is(err, errGoalEvaluationModelSnapshotInvalid) {
		t.Fatalf("a previously selected but missing snapshot was replaced: %v", err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_evaluation_requests SET model_provenance='{"_goal_evaluation_model_snapshot_contract":"v1"}' WHERE id=$1`, newRequest.RequestID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.agent_runs(fluctlight_id,agent_id,run_id,input_digest,correlation_id,status) VALUES($1,$2,$3,'prior-attempt',$4,'failed')`, f.fluctlightID, FormalAgentGoalEvaluation, newRequest.RequestID+":claim:1", newRequest.RequestID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.goalEvaluationAssignment(route, newRequest); !errors.Is(err, errGoalEvaluationModelSnapshotInvalid) {
		t.Fatalf("a prior Agent attempt silently replaced its missing snapshot: %v", err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_evaluation_requests SET model_provenance='{"_goal_evaluation_model_snapshot_contract":"invalid"}' WHERE id=$1`, newRequest.RequestID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.goalEvaluationAssignment(route, newRequest); !errors.Is(err, errGoalEvaluationModelSnapshotInvalid) {
		t.Fatalf("corrupt goal snapshot silently selected a model: %v", err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_evaluation_requests SET model_provenance='{}' WHERE id=$1`, newRequest.RequestID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.goalEvaluationAssignment(route, newRequest); !errors.Is(err, errGoalEvaluationModelSnapshotInvalid) {
		t.Fatalf("a cleared new snapshot was mistaken for a legacy request: %v", err)
	}
}

func TestGoalEvaluationConcurrentSelectionUsesStoredWinner(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	seedCognitiveProviderRole(t, f.ctx, f.repository, "goal-snapshot-race-endpoint")
	f.app.Provider.SettingsKey = []byte("0123456789abcdef0123456789abcdef")
	const fixtureSecret = "private-test-secret"
	encrypted, err := encryptSecret(f.app.Provider.SettingsKey, "turn-secret", fixtureSecret)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.setting_secrets(purpose,ciphertext,nonce) VALUES('turn-secret',$1,$2)`, encrypted.ciphertext, encrypted.nonce); err != nil {
		t.Fatal(err)
	}
	candidate, err := f.app.Provider.assignment(withAgentModelRoute(f.ctx, FormalAgentGoalEvaluation, goalEvaluationModelVariant), goalEvaluationModelRole)
	if err != nil || candidate.Secret != fixtureSecret {
		t.Fatalf("race fixture could not load its credential: %v", err)
	}
	const requestID = "goal-snapshot-race"
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.goal_evaluation_requests(id,fluctlight_id,profile_id,reason,status,claim_revision) VALUES($1,$2,'default','test','processing',1)`, requestID, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	lockTx, err := f.repository.Pool().Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer lockTx.Rollback(f.ctx)
	var locked []byte
	if err := lockTx.QueryRow(f.ctx, `SELECT model_provenance FROM public.goal_evaluation_requests WHERE id=$1 FOR UPDATE`, requestID).Scan(&locked); err != nil {
		t.Fatal(err)
	}
	type result struct {
		assignment providerAssignment
		err        error
	}
	start, results := make(chan struct{}), make(chan result, 2)
	var workers sync.WaitGroup
	for _, model := range []string{"candidate-a", "candidate-b"} {
		workers.Add(1)
		go func(model string) {
			defer workers.Done()
			route := withAgentModelRoute(f.ctx, FormalAgentGoalEvaluation, goalEvaluationModelVariant)
			binding := candidate
			binding.ModelID = model
			route = withFrozenProviderAssignment(route, goalEvaluationModelRole, binding)
			<-start
			assignment, err := f.app.goalEvaluationAssignment(route, goalEvaluationSnapshot{RequestID: requestID, FluctlightID: f.fluctlightID, ClaimRevision: 1})
			results <- result{assignment: assignment, err: err}
		}(model)
	}
	close(start)
	var waiting int
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE 'UPDATE public.goal_evaluation_requests SET model_provenance%'`).Scan(&waiting); err != nil {
			_ = lockTx.Rollback(f.ctx)
			workers.Wait()
			t.Fatal(err)
		}
		if waiting == 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if waiting != 2 {
		_ = lockTx.Rollback(f.ctx)
		workers.Wait()
		t.Fatalf("workers did not reach competing updates: waiting=%d", waiting)
	}
	if err := lockTx.Commit(f.ctx); err != nil {
		workers.Wait()
		t.Fatal(err)
	}
	workers.Wait()
	close(results)
	var selected providerAssignment
	selectedSet := false
	for item := range results {
		if item.err != nil {
			t.Fatal(item.err)
		}
		if !selectedSet {
			selected, selectedSet = item.assignment, true
		} else if !reflect.DeepEqual(selected, item.assignment) {
			t.Fatal("workers returned different complete model bindings")
		}
	}
	var raw []byte
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT model_provenance FROM public.goal_evaluation_requests WHERE id=$1`, requestID).Scan(&raw); err != nil || strings.Contains(string(raw), "private-test-secret") {
		t.Fatalf("goal snapshot exposed a credential or could not be read: %v", err)
	}
	var provenance map[string]any
	if err := json.Unmarshal(raw, &provenance); err != nil {
		t.Fatal(err)
	}
	stored, found, err := readWorkflowModelSnapshot(provenance, goalEvaluationModelSnapshotsKey, goalEvaluationModelStage, FormalAgentGoalEvaluation, goalEvaluationModelVariant, goalEvaluationModelRole, errGoalEvaluationModelSnapshotInvalid)
	if err != nil || !found {
		t.Fatalf("durable winner not found: %v", err)
	}
	if err := f.app.restoreAssignmentCredential(f.ctx, &stored.Assignment); err != nil || !selectedSet || !reflect.DeepEqual(stored.Assignment, selected) || selected.Secret != fixtureSecret {
		t.Fatalf("returned binding did not match durable winner and its credential: %v", err)
	}
}

func TestLegacyGoalEvaluationReportsUnfrozenBinding(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	seedCognitiveProviderRole(t, f.ctx, f.repository, "legacy-goal-snapshot-endpoint")
	const requestID = "legacy-goal-snapshot"
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.goal_evaluation_requests(id,fluctlight_id,profile_id,reason,status,claim_revision,model_provenance,model_snapshot_required) VALUES($1,$2,'default','test','processing',1,'{}',false)`, requestID, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	route := withAgentModelRoute(f.ctx, FormalAgentGoalEvaluation, goalEvaluationModelVariant)
	assignment, err := f.app.goalEvaluationAssignment(route, goalEvaluationSnapshot{RequestID: requestID, FluctlightID: f.fluctlightID, ClaimRevision: 1})
	if err != nil || assignment.ModelID != "turn-model" {
		t.Fatalf("legacy request did not preserve its existing route: model=%q err=%v", assignment.ModelID, err)
	}
	var unchanged bool
	var diagnostics int
	err = f.repository.Pool().QueryRow(f.ctx, `SELECT (SELECT model_provenance='{}'::jsonb FROM public.goal_evaluation_requests WHERE id=$1),(SELECT count(*) FROM public.diagnostic_events WHERE correlation_id=$1 AND event_type='goal_evaluation.model_binding.legacy_unfrozen')`, requestID).Scan(&unchanged, &diagnostics)
	if err != nil || !unchanged || diagnostics != 1 {
		t.Fatalf("legacy binding was invented or not diagnosed: unchanged=%t diagnostics=%d err=%v", unchanged, diagnostics, err)
	}
}
