package core

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func seedMultiProfilePreparation(t *testing.T, ctx context.Context, repo *PostgresRepository) (string, string) {
	t.Helper()
	role := agentModelBindingKey(FormalAgentPersonaCompilation, "persona_compilation_response")
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,timeout_seconds) VALUES($1,'preparation-provider','first-model',30)`, role); err != nil {
		t.Fatal(err)
	}
	core := map[string]any{"identity": map[string]any{"name": "角色"}, "personality_system": map[string]any{"active_profile_id": "a", "profiles": []any{map[string]any{"id": "a", "personality": map[string]any{"style": "直接"}}, map[string]any{"id": "b", "personality": map[string]any{"style": "安静"}}}}}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.fluctlights SET current_revision=1,core_persona=$2 WHERE id=$1`, "prep-role", jsonBytes(core)); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlight_profile_habits(fluctlight_id,profile_id,revision,habits_json,source_kind) VALUES('prep-role','a',0,'[]','initialization'),('prep-role','b',0,'[]','initialization') ON CONFLICT DO NOTHING`); err != nil {
		t.Fatal(err)
	}
	var id string
	if err := withTransaction(ctx, repo.Pool(), func(tx pgx.Tx) error {
		var err error
		id, err = enqueuePersonaPreparationTx(ctx, tx, "prep-owner", "prep-role", 1, "test", "snapshot-profiles")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return id, role
}

func TestPersonaPreparationKeepsModelAfterWorkerRetry(t *testing.T) {
	var ctx context.Context
	var repo *PostgresRepository
	var models, hosts []string
	var temperatures, topPs []float64
	var authorized []bool
	role := agentModelBindingKey(FormalAgentPersonaCompilation, "persona_compilation_response")
	router := newFakeProviderRouter().on("persona_compilation_response", func(payload map[string]any) fakeProviderResult {
		models = append(models, stringValue(payload["model"]))
		temperature, temperatureOK := numberFloat(payload["temperature"])
		topP, topPOK := numberFloat(payload["top_p"])
		if !temperatureOK || !topPOK {
			t.Error("persona preparation request omitted configured sampling")
		}
		temperatures, topPs = append(temperatures, temperature), append(topPs, topP)
		if len(models) == 1 {
			if _, err := repo.Pool().Exec(ctx, `UPDATE public.model_roles SET provider_endpoint_id='preparation-next-endpoint',model_id='next-model',temperature=0.6,top_p=0.9 WHERE role=$1`, role); err != nil {
				t.Error(err)
			}
		}
		if len(models) == 2 {
			return fakeProviderResult{Status: 400}
		}
		return fakeProviderResult{Structured: map[string]any{"portrait_text": "保留来源中的人格特点。"}}
	})
	var app *App
	ctx, repo, app = personaPreparationFixture(t, router)
	id, _ := seedMultiProfilePreparation(t, ctx, repo)
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status) VALUES('preparation-next-endpoint','openai_compatible','http://next.invalid','next-secret','ready')`); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.model_roles SET temperature=0.2,top_p=0.7 WHERE role=$1`, role); err != nil {
		t.Fatal(err)
	}
	settingsKey := []byte("0123456789abcdef0123456789abcdef")
	const fixtureSecret = "test-persona-preparation-secret"
	encrypted, err := encryptSecret(settingsKey, "turn-secret", fixtureSecret)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.setting_secrets(purpose,ciphertext,nonce) VALUES('turn-secret',$1,$2)`, encrypted.ciphertext, encrypted.nonce); err != nil {
		t.Fatal(err)
	}
	transport := projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		hosts = append(hosts, request.URL.Host)
		authorized = append(authorized, request.Header.Get("Authorization") == "Bearer "+fixtureSecret)
		return router.RoundTrip(request)
	})
	app.Provider.SettingsKey, app.Provider.HTTP = settingsKey, &http.Client{Transport: transport}
	if _, err := app.ProcessPersonaPreparationIntent(ctx, id); err == nil || len(models) != 2 {
		t.Fatalf("expected the second profile to fail: models=%v err=%v", models, err)
	}
	var published int
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.fluctlight_working_personas WHERE fluctlight_id='prep-role' AND source_revision=1`).Scan(&published); err != nil || published != 0 {
		t.Fatalf("failed preparation published partial profiles: count=%d err=%v", published, err)
	}
	// Reconstruct the runtime without the legacy test fixture's automatic
	// persona seeding, which would otherwise replace the uncommitted profiles.
	restarted := &App{DB: repo, Provider: &ProviderClient{DB: repo, SettingsKey: settingsKey, HTTP: &http.Client{Transport: transport}}}
	if _, err := restarted.ProcessPersonaPreparationIntent(ctx, id); err != nil {
		t.Fatal(err)
	}
	if len(models) != 4 || models[0] != "first-model" || models[1] != "first-model" || models[2] != "first-model" || models[3] != "first-model" {
		t.Fatalf("same preparation changed models during retry: %v", models)
	}
	for index := range models {
		if hosts[index] != "turn.invalid" || temperatures[index] != 0.2 || topPs[index] != 0.7 || !authorized[index] {
			t.Fatalf("retry lost its endpoint, sampling or credential at request %d", index+1)
		}
	}
	var snapshot []byte
	if err := repo.Pool().QueryRow(ctx, `SELECT snapshot FROM public.persona_preparation_model_snapshots WHERE intent_id=$1`, id).Scan(&snapshot); err != nil || strings.Contains(string(snapshot), fixtureSecret) {
		t.Fatalf("durable preparation snapshot exposed a credential or could not be read: %v", err)
	}
	if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM public.fluctlight_working_personas WHERE fluctlight_id='prep-role' AND source_revision=1`).Scan(&published); err != nil || published != 2 {
		t.Fatalf("retried preparation did not publish both profiles: count=%d err=%v", published, err)
	}
}

func TestPersonaPreparationConcurrentSelectionUsesStoredWinner(t *testing.T) {
	ctx, repo, app := personaPreparationFixture(t, newFakeProviderRouter())
	id, _ := seedMultiProfilePreparation(t, ctx, repo)
	lockTx, err := repo.Pool().Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer lockTx.Rollback(ctx)
	var locked []byte
	if err := lockTx.QueryRow(ctx, `SELECT snapshot FROM public.persona_preparation_model_snapshots WHERE intent_id=$1 FOR UPDATE`, id).Scan(&locked); err != nil {
		t.Fatal(err)
	}
	type result struct {
		assignment providerAssignment
		err        error
	}
	start := make(chan struct{})
	results := make(chan result, 2)
	var workers sync.WaitGroup
	for _, model := range []string{"candidate-a", "candidate-b"} {
		workers.Add(1)
		go func(model string) {
			defer workers.Done()
			route := withAgentModelRoute(ctx, FormalAgentPersonaCompilation, personaPreparationModelVariant)
			route = withFrozenProviderAssignment(route, personaPreparationModelRole, providerAssignment{Role: "generic_llm", EndpointID: "preparation-provider", SecretPurpose: "turn-secret", BaseURL: "http://turn.invalid", ModelID: model, Timeout: 10 * time.Second, Secret: "private-test-secret"})
			<-start
			assignment, err := app.personaPreparationAssignment(route, id)
			results <- result{assignment: assignment, err: err}
		}(model)
	}
	close(start)
	// Hold the row until both workers have read the empty snapshot and are
	// blocked at the competing CAS updates; this exercises the loser path.
	var waiting int
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if err := repo.Pool().QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE 'UPDATE public.persona_preparation_model_snapshots%'`).Scan(&waiting); err != nil {
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
		t.Fatalf("workers did not both reach the competing snapshot updates: waiting=%d", waiting)
	}
	if err := lockTx.Commit(ctx); err != nil {
		workers.Wait()
		t.Fatal(err)
	}
	workers.Wait()
	close(results)
	var selected string
	for item := range results {
		if item.err != nil {
			t.Fatal(item.err)
		}
		if selected == "" {
			selected = item.assignment.ModelID
		} else if selected != item.assignment.ModelID {
			t.Fatalf("concurrent workers selected different models: %q and %q", selected, item.assignment.ModelID)
		}
	}
	var raw []byte
	if err := repo.Pool().QueryRow(ctx, `SELECT snapshot FROM public.persona_preparation_model_snapshots WHERE intent_id=$1`, id).Scan(&raw); err != nil || strings.Contains(string(raw), "private-test-secret") {
		t.Fatalf("snapshot exposed a credential or could not be read: %v", err)
	}
	var stored workflowModelSnapshot
	if err := json.Unmarshal(raw, &stored); err != nil || stored.Assignment.ModelID != selected {
		t.Fatalf("concurrent result did not match the durable winner: selected=%q stored=%q err=%v", selected, stored.Assignment.ModelID, err)
	}
}

func TestPersonaPreparationRejectsCorruptSnapshotWithoutModelCall(t *testing.T) {
	router := newFakeProviderRouter()
	ctx, repo, app := personaPreparationFixture(t, router)
	id, _ := seedMultiProfilePreparation(t, ctx, repo)
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.persona_preparation_model_snapshots SET snapshot='[]'::jsonb WHERE intent_id=$1`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := app.ProcessPersonaPreparationIntent(ctx, id); !errors.Is(err, errPersonaPreparationModelSnapshotInvalid) || router.totalRequests() != 0 {
		t.Fatalf("corrupt preparation snapshot reached the model: calls=%d err=%v", router.totalRequests(), err)
	}
}

func TestLegacyPersonaPreparationDoesNotInventSnapshotOnReplay(t *testing.T) {
	ctx, repo, app := personaPreparationFixture(t, newFakeProviderRouter())
	id, _ := seedMultiProfilePreparation(t, ctx, repo)
	if _, err := repo.Pool().Exec(ctx, `DELETE FROM public.persona_preparation_model_snapshots WHERE intent_id=$1`, id); err != nil {
		t.Fatal(err)
	}
	var replay string
	if err := withTransaction(ctx, repo.Pool(), func(tx pgx.Tx) error {
		var err error
		replay, err = enqueuePersonaPreparationTx(ctx, tx, "prep-owner", "prep-role", 1, "test", "snapshot-profiles")
		return err
	}); err != nil || replay != id {
		t.Fatalf("legacy intent replay changed identity: replay=%q err=%v", replay, err)
	}
	route := withAgentModelRoute(ctx, FormalAgentPersonaCompilation, personaPreparationModelVariant)
	assignment, err := app.personaPreparationAssignment(route, id)
	if err != nil || assignment.ModelID != "first-model" {
		t.Fatalf("legacy preparation did not preserve its existing path: model=%q err=%v", assignment.ModelID, err)
	}
	var snapshots, diagnostics int
	err = repo.Pool().QueryRow(ctx, `SELECT (SELECT count(*) FROM public.persona_preparation_model_snapshots WHERE intent_id=$1),(SELECT count(*) FROM public.diagnostic_events WHERE correlation_id=$2 AND event_type='persona_preparation.model_binding.legacy_unfrozen')`, id, "persona-preparation:"+id).Scan(&snapshots, &diagnostics)
	if err != nil || snapshots != 0 || diagnostics != 1 {
		t.Fatalf("legacy replay invented a snapshot or lost its diagnostic: snapshots=%d diagnostics=%d err=%v", snapshots, diagnostics, err)
	}
}

func TestPersonaPreparationKeepsModelAcrossProfiles(t *testing.T) {
	var ctx context.Context
	var repo *PostgresRepository
	var models []string
	role := agentModelBindingKey(FormalAgentPersonaCompilation, "persona_compilation_response")
	router := newFakeProviderRouter().on("persona_compilation_response", func(payload map[string]any) fakeProviderResult {
		models = append(models, stringValue(payload["model"]))
		if len(models) == 1 {
			if _, err := repo.Pool().Exec(ctx, `UPDATE public.model_roles SET model_id='next-model' WHERE role=$1`, role); err != nil {
				t.Error(err)
			}
		}
		return fakeProviderResult{Structured: map[string]any{"portrait_text": "表达直接，尊重边界。"}}
	})
	var app *App
	ctx, repo, app = personaPreparationFixture(t, router)
	id, _ := seedMultiProfilePreparation(t, ctx, repo)
	if _, err := app.ProcessPersonaPreparationIntent(ctx, id); err != nil {
		t.Fatal(err)
	}
	if len(models) != 2 || models[0] != "first-model" || models[1] != "first-model" {
		t.Fatalf("one preparation changed models between profiles: %v", models)
	}
}
