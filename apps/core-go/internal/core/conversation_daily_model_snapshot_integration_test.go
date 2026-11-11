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

func TestDailyMemoryRetryRestoresFrozenModel(t *testing.T) {
	ctx, repo, app, intentID, _ := seedDailyMemorySource(t)
	key := []byte("0123456789abcdef0123456789abcdef")
	const fixtureSecret = "daily-fixture-secret"
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
		if payload["temperature"] != 0.2 || payload["top_p"] != 0.7 || request.URL.Host != "daily.invalid" || request.Header.Get("Authorization") != "Bearer "+fixtureSecret {
			t.Fatal("daily retry did not preserve its endpoint, sampling and credential")
		}
		if len(models) == 1 {
			if _, err := repo.Pool().Exec(ctx, `UPDATE public.model_roles SET model_id='next-model',temperature=0.8,top_p=0.9 WHERE role='reflection'`); err != nil {
				t.Fatal(err)
			}
			return embeddingHTTPResponse(request, http.StatusBadRequest, `{"error":"controlled failure"}`), nil
		}
		response := map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": jsonString(map[string]any{"schema_version": conversationDailyMemorySchemaVersion, "episode": "一段记忆"})}}}}
		return embeddingHTTPResponse(request, http.StatusOK, string(jsonBytes(response))), nil
	})
	app.Provider = &ProviderClient{DB: repo, SettingsKey: key, HTTP: &http.Client{Transport: transport}}
	if _, err := app.ProcessConversationDailyMemoryIntentAt(ctx, intentID, 0); err == nil {
		t.Fatal("first provider failure was not returned")
	}
	var saved []byte
	if err := repo.Pool().QueryRow(ctx, `SELECT snapshot FROM public.conversation_daily_model_snapshots WHERE intent_id=$1 AND source_generation=0`, intentID).Scan(&saved); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.conversation_daily_model_snapshots SET snapshot='{}' WHERE intent_id=$1 AND source_generation=0`, intentID); err != nil {
		t.Fatal(err)
	}
	if _, err := app.ProcessConversationDailyMemoryIntentAt(ctx, intentID, 0); !errors.Is(err, errConversationDailyModelSnapshotInvalid) || len(models) != 1 {
		t.Fatalf("cleared snapshot caused a second selection: requests=%d err=%v", len(models), err)
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.conversation_daily_model_snapshots SET snapshot=$2 WHERE intent_id=$1 AND source_generation=0`, intentID, saved); err != nil {
		t.Fatal(err)
	}
	restarted := &App{DB: repo, Provider: &ProviderClient{DB: repo, SettingsKey: key, HTTP: &http.Client{Transport: transport}}}
	result, err := restarted.ProcessConversationDailyMemoryIntentAt(ctx, intentID, 0)
	if err != nil || result["status"] != "completed" || len(models) != 2 || models[0] != "fixture" || models[1] != "fixture" {
		t.Fatalf("same generation changed model after retry: models=%v result=%v err=%v", models, result, err)
	}
	var raw []byte
	if err := repo.Pool().QueryRow(ctx, `SELECT snapshot FROM public.conversation_daily_model_snapshots WHERE intent_id=$1 AND source_generation=0`, intentID).Scan(&raw); err != nil || strings.Contains(string(raw), fixtureSecret) {
		t.Fatalf("daily model snapshot exposed a credential or could not be read: %v", err)
	}
}

func TestDailyMemoryConcurrentSelectionUsesStoredWinner(t *testing.T) {
	ctx, repo, app, intentID, _ := seedDailyMemorySource(t)
	key := []byte("0123456789abcdef0123456789abcdef")
	const fixtureSecret = "daily-race-fixture-secret"
	encrypted, err := encryptSecret(key, "fixture", fixtureSecret)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.setting_secrets(purpose,ciphertext,nonce) VALUES('fixture',$1,$2)`, encrypted.ciphertext, encrypted.nonce); err != nil {
		t.Fatal(err)
	}
	app.Provider = &ProviderClient{DB: repo, SettingsKey: key}
	route := withAgentModelRoute(ctx, FormalAgentConversationSummary, conversationDailyModelVariant)
	candidate, err := app.Provider.assignment(route, conversationDailyModelRole)
	if err != nil || candidate.Secret != fixtureSecret {
		t.Fatalf("race fixture could not load its credential: %v", err)
	}
	lockTx, err := repo.Pool().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer lockTx.Rollback(ctx)
	var raw []byte
	if err := lockTx.QueryRow(ctx, `SELECT snapshot FROM public.conversation_daily_model_snapshots WHERE intent_id=$1 AND source_generation=0 FOR UPDATE`, intentID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	type result struct {
		assignment providerAssignment
		err        error
	}
	start, results := make(chan struct{}), make(chan result, 2)
	var workers sync.WaitGroup
	for _, model := range []string{"daily-candidate-a", "daily-candidate-b"} {
		workers.Add(1)
		go func(model string) {
			defer workers.Done()
			binding := candidate
			binding.ModelID = model
			candidateCtx := withFrozenProviderAssignment(route, conversationDailyModelRole, binding)
			<-start
			selected, err := app.conversationDailyAssignment(candidateCtx, intentID, 0)
			results <- result{selected, err}
		}(model)
	}
	close(start)
	var waiting int
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE 'UPDATE public.conversation_daily_model_snapshots s SET snapshot%'`).Scan(&waiting); err != nil {
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
		if item.err != nil {
			t.Fatal(item.err)
		}
		if !selected {
			winner, selected = item.assignment, true
		} else if !reflect.DeepEqual(winner, item.assignment) {
			t.Fatal("concurrent selectors returned different complete bindings")
		}
	}
	if err := repo.Pool().QueryRow(ctx, `SELECT snapshot FROM public.conversation_daily_model_snapshots WHERE intent_id=$1 AND source_generation=0`, intentID).Scan(&raw); err != nil || strings.Contains(string(raw), fixtureSecret) {
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

func TestDailyMemoryMissingSnapshotIsNotRecreated(t *testing.T) {
	ctx, repo, app, intentID, enqueue := seedDailyMemorySource(t)
	app.Provider = &ProviderClient{DB: repo}
	if _, err := repo.Pool().Exec(ctx, `DELETE FROM public.conversation_daily_model_snapshots WHERE intent_id=$1 AND source_generation=0`, intentID); err != nil {
		t.Fatal(err)
	}
	enqueue()
	route := withAgentModelRoute(ctx, FormalAgentConversationSummary, conversationDailyModelVariant)
	if _, err := app.conversationDailyAssignment(route, intentID, 0); !errors.Is(err, errConversationDailyModelSnapshotInvalid) {
		t.Fatalf("a missing new snapshot was recreated or treated as legacy: %v", err)
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.platform_workflow_intents SET payload=payload-$2::text WHERE intent_id=$1`, intentID, conversationDailyModelContractKey); err != nil {
		t.Fatal(err)
	}
	assignment, err := app.conversationDailyAssignment(route, intentID, 0)
	if err != nil || assignment.ModelID != "fixture" {
		t.Fatalf("legacy daily request did not keep its old route: model=%q err=%v", assignment.ModelID, err)
	}
	var diagnostics int
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.diagnostic_events WHERE correlation_id=$1 AND event_type='conversation_daily_memory.model_binding.legacy_unfrozen'`, intentID).Scan(&diagnostics); err != nil || diagnostics != 1 {
		t.Fatalf("legacy unfrozen binding was not diagnosed: count=%d err=%v", diagnostics, err)
	}
}
