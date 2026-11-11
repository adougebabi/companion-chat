package core

import (
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestConversationSegmentRetryRestoresFrozenModelAndNewGeneration(t *testing.T) {
	ctx, repo, app, intentID, enqueue := seedConversationSegmentSource(t)
	key := []byte("0123456789abcdef0123456789abcdef")
	const fixtureSecret = "segment-fixture-secret"
	encrypted, err := encryptSecret(key, "fixture", fixtureSecret)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.setting_secrets(purpose,ciphertext,nonce) VALUES('fixture',$1,$2)`, encrypted.ciphertext, encrypted.nonce); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.model_roles SET temperature=0.2,top_p=0.7 WHERE role='reflection'`); err != nil {
		t.Fatal(err)
	}
	var models []string
	transport := projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		var payload map[string]any
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		models = append(models, stringValue(payload["model"]))
		wantTemperature, wantTopP := 0.2, 0.7
		if len(models) == 3 {
			wantTemperature, wantTopP = 0.8, 0.9
		}
		if payload["temperature"] != wantTemperature || payload["top_p"] != wantTopP || request.URL.Host != "segment.invalid" || request.Header.Get("Authorization") != "Bearer "+fixtureSecret {
			t.Fatal("segment request did not preserve the intended endpoint, sampling and credential")
		}
		if len(models) == 1 {
			if _, err := repo.Pool().Exec(ctx, `UPDATE public.model_roles SET model_id='next-model',temperature=0.8,top_p=0.9 WHERE role='reflection'`); err != nil {
				t.Fatal(err)
			}
			return embeddingHTTPResponse(request, http.StatusBadRequest, `{"error":"controlled failure"}`), nil
		}
		value := map[string]any{"schema_version": conversationSegmentSchemaVersion, "summary": "一段对话", "ending_state": "稍后继续", "open_threads": []any{}, "core_events": []any{}}
		response := map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": jsonString(value)}}}}
		return embeddingHTTPResponse(request, http.StatusOK, string(jsonBytes(response))), nil
	})
	app.Provider = &ProviderClient{DB: repo, SettingsKey: key, HTTP: &http.Client{Transport: transport}}
	if _, err := app.ProcessConversationSegmentIntentAt(ctx, intentID, 0); err == nil {
		t.Fatal("first provider failure was not returned")
	}
	var saved []byte
	if err := repo.Pool().QueryRow(ctx, `SELECT snapshot FROM public.conversation_segment_model_snapshots WHERE intent_id=$1 AND source_generation=0`, intentID).Scan(&saved); err != nil || strings.Contains(string(saved), fixtureSecret) {
		t.Fatalf("segment snapshot exposed a credential or could not be read: %v", err)
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.conversation_segment_model_snapshots SET snapshot='{}' WHERE intent_id=$1 AND source_generation=0`, intentID); err != nil {
		t.Fatal(err)
	}
	if _, err := app.ProcessConversationSegmentIntentAt(ctx, intentID, 0); !errors.Is(err, errConversationSegmentModelSnapshotInvalid) || len(models) != 1 {
		t.Fatalf("cleared snapshot caused a second selection: requests=%d err=%v", len(models), err)
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.conversation_segment_model_snapshots SET snapshot=$2 WHERE intent_id=$1 AND source_generation=0`, intentID, saved); err != nil {
		t.Fatal(err)
	}
	restarted := &App{DB: repo, Provider: &ProviderClient{DB: repo, SettingsKey: key, HTTP: &http.Client{Transport: transport}}}
	result, err := restarted.ProcessConversationSegmentIntentAt(ctx, intentID, 0)
	if err != nil || result["status"] != "active" || len(models) != 2 || models[0] != "fixture" || models[1] != "fixture" {
		t.Fatalf("same generation changed model after retry: models=%v result=%v err=%v", models, result, err)
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.conversation_summaries SET status='invalidated' WHERE id=$1`, result["summary_id"]); err != nil {
		t.Fatal(err)
	}
	enqueue()
	result, err = restarted.ProcessConversationSegmentIntentAt(ctx, intentID, 1)
	if err != nil || result["status"] != "active" || len(models) != 3 || models[2] != "next-model" {
		t.Fatalf("new generation did not use current model: models=%v result=%v err=%v", models, result, err)
	}
}

func TestConversationSegmentConcurrentSelectionUsesStoredWinner(t *testing.T) {
	ctx, repo, app, intentID, _ := seedConversationSegmentSource(t)
	key := []byte("0123456789abcdef0123456789abcdef")
	const fixtureSecret = "segment-race-fixture-secret"
	encrypted, err := encryptSecret(key, "fixture", fixtureSecret)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.setting_secrets(purpose,ciphertext,nonce) VALUES('fixture',$1,$2)`, encrypted.ciphertext, encrypted.nonce); err != nil {
		t.Fatal(err)
	}
	app.Provider = &ProviderClient{DB: repo, SettingsKey: key}
	route := withAgentModelRoute(ctx, FormalAgentConversationSummary, conversationSegmentModelVariant)
	candidate, err := app.Provider.assignment(route, conversationSegmentModelRole)
	if err != nil || candidate.Secret != fixtureSecret {
		t.Fatalf("race fixture could not load its credential: %v", err)
	}
	lockTx, err := repo.Pool().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer lockTx.Rollback(ctx)
	var raw []byte
	if err := lockTx.QueryRow(ctx, `SELECT snapshot FROM public.conversation_segment_model_snapshots WHERE intent_id=$1 AND source_generation=0 FOR UPDATE`, intentID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	type result struct {
		assignment providerAssignment
		frozen     bool
		err        error
	}
	start, results := make(chan struct{}), make(chan result, 2)
	var workers sync.WaitGroup
	for _, model := range []string{"segment-candidate-a", "segment-candidate-b"} {
		workers.Add(1)
		go func(model string) {
			defer workers.Done()
			binding := candidate
			binding.ModelID = model
			candidateCtx := withFrozenProviderAssignment(route, conversationSegmentModelRole, binding)
			<-start
			selected, frozen, err := app.conversationSegmentAssignment(candidateCtx, intentID, 0)
			results <- result{selected, frozen, err}
		}(model)
	}
	close(start)
	var waiting int
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE 'UPDATE public.conversation_segment_model_snapshots s SET snapshot%'`).Scan(&waiting); err != nil {
			_ = lockTx.Rollback(ctx)
			workers.Wait()
			t.Fatal(err)
		}
		if waiting == 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if waiting != 2 {
		_ = lockTx.Rollback(ctx)
		workers.Wait()
		t.Fatalf("selectors did not reach competing updates: waiting=%d", waiting)
	}
	if err := lockTx.Commit(ctx); err != nil {
		workers.Wait()
		t.Fatal(err)
	}
	workers.Wait()
	close(results)
	var winner providerAssignment
	selected := false
	for item := range results {
		if item.err != nil || !item.frozen {
			t.Fatalf("selection did not return a frozen binding: %v", item.err)
		}
		if !selected {
			winner, selected = item.assignment, true
		} else if !reflect.DeepEqual(winner, item.assignment) {
			t.Fatal("concurrent selectors returned different complete bindings")
		}
	}
	if err := repo.Pool().QueryRow(ctx, `SELECT snapshot FROM public.conversation_segment_model_snapshots WHERE intent_id=$1 AND source_generation=0`, intentID).Scan(&raw); err != nil || strings.Contains(string(raw), fixtureSecret) {
		t.Fatalf("snapshot exposed a credential or could not be read: %v", err)
	}
	var stored workflowModelSnapshot
	if err := json.Unmarshal(raw, &stored); err != nil {
		t.Fatal(err)
	}
	if err := app.restoreAssignmentCredential(ctx, &stored.Assignment); err != nil || !selected || !reflect.DeepEqual(winner, stored.Assignment) || winner.Secret != fixtureSecret {
		t.Fatalf("selectors did not use the persisted winner and credential: %v", err)
	}
}

func TestConversationSegmentMissingSnapshotIsNotRecreated(t *testing.T) {
	ctx, repo, app, intentID, enqueue := seedConversationSegmentSource(t)
	app.Provider = &ProviderClient{DB: repo}
	if _, err := repo.Pool().Exec(ctx, `DELETE FROM public.conversation_segment_model_snapshots WHERE intent_id=$1 AND source_generation=0`, intentID); err != nil {
		t.Fatal(err)
	}
	enqueue()
	route := withAgentModelRoute(ctx, FormalAgentConversationSummary, conversationSegmentModelVariant)
	if _, _, err := app.conversationSegmentAssignment(route, intentID, 0); !errors.Is(err, errConversationSegmentModelSnapshotInvalid) {
		t.Fatalf("a missing new snapshot was recreated or treated as legacy: %v", err)
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET payload=payload-$2::text WHERE intent_id=$1`, intentID, conversationSegmentModelContractKey); err != nil {
		t.Fatal(err)
	}
	assignment, frozen, err := app.conversationSegmentAssignment(route, intentID, 0)
	if err != nil || frozen || assignment.ModelID != "fixture" {
		t.Fatalf("legacy segment did not keep its old route: model=%q frozen=%t err=%v", assignment.ModelID, frozen, err)
	}
	var diagnostics int
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.diagnostic_events WHERE correlation_id=$1 AND event_type='conversation_segment.model_binding.legacy_unfrozen'`, intentID).Scan(&diagnostics); err != nil || diagnostics != 1 {
		t.Fatalf("legacy unfrozen binding was not diagnosed: count=%d err=%v", diagnostics, err)
	}
}

func TestLegacyConversationSegmentStillRejectsChangedModel(t *testing.T) {
	ctx, repo, app, intentID, _ := seedConversationSegmentSource(t)
	if _, err := repo.Pool().Exec(ctx, `DELETE FROM public.conversation_segment_model_snapshots WHERE intent_id=$1`, intentID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET payload=payload-$2::text WHERE intent_id=$1`, intentID, conversationSegmentModelContractKey); err != nil {
		t.Fatal(err)
	}
	app.Provider = &ProviderClient{DB: repo, HTTP: &http.Client{Transport: projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		if _, err := repo.Pool().Exec(ctx, `UPDATE public.model_roles SET model_id='next-model' WHERE role='reflection'`); err != nil {
			t.Fatal(err)
		}
		value := map[string]any{"schema_version": conversationSegmentSchemaVersion, "summary": "一段对话", "ending_state": "稍后继续", "open_threads": []any{}, "core_events": []any{}}
		response := map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": jsonString(value)}}}}
		return embeddingHTTPResponse(request, http.StatusOK, string(jsonBytes(response))), nil
	})}}
	if _, err := app.ProcessConversationSegmentIntentAt(ctx, intentID, 0); err == nil || !strings.Contains(err.Error(), "conversation_segment_provider_assignment_stale") {
		t.Fatalf("legacy segment accepted a changed model: %v", err)
	}
	var summaries int
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.conversation_summaries`).Scan(&summaries); err != nil || summaries != 0 {
		t.Fatalf("legacy segment published after model change: count=%d err=%v", summaries, err)
	}
}
