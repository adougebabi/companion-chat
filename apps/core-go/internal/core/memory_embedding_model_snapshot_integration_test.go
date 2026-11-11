package core

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

type embeddingSnapshotFixture struct {
	ctx      context.Context
	repo     *PostgresRepository
	app      *App
	memoryID string
	intentID string
	command  PreparedMemoryMutation
}

func seedUnconfiguredEmbeddingSnapshot(t *testing.T) embeddingSnapshotFixture {
	t.Helper()
	ctx, repo := isolatedCoreTestRepository(t)
	const owner, fid = "snapshot-embedding-owner", "snapshot-embedding-fluctlight"
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active'),($2,'fluctlight','active')`, owner, fid); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.fluctlights(id,created_by_actor_id,initialization_mode,status,core_persona,identity,personality,behavioral_policy,life_profile,provenance) VALUES($1,$2,'blank_slate','active','{}','{}','{}','{}','{}','{}')`, fid, owner); err != nil {
		t.Fatal(err)
	}
	app := &App{DB: repo, Provider: &ProviderClient{DB: repo}}
	semantic := &MemorySemanticInput{Type: "semantic", Content: "embedding snapshot fixture", Confidence: 0.9, Importance: 0.8}
	command := memoryLifecycleTestCommand(MemoryCreate, "embedding-snapshot-fixture", semantic, nil, nil)
	command.OwnerFluctlightID, command.OwnerActorID, command.ActorID = fid, owner, owner
	command.RequestDigest = memoryCommandDigest(command)
	created := applyMemoryLifecycleTestCommand(t, ctx, app, command)
	return embeddingSnapshotFixture{ctx: ctx, repo: repo, app: app, memoryID: created.MemoryID, intentID: "memory_embedding_intent:" + created.MemoryID + ":0", command: command}
}

func (f embeddingSnapshotFixture) configure(t *testing.T) {
	t.Helper()
	if _, err := f.repo.Pool().Exec(f.ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status) VALUES('embedding-snapshot-endpoint','openai_compatible','http://embedding-a.invalid','embedding-snapshot-secret','ready')`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repo.Pool().Exec(f.ctx, `INSERT INTO public.model_roles(role,provider_endpoint_id,model_id,required_capabilities,timeout_seconds) VALUES('embedding','embedding-snapshot-endpoint','embedding-a','embedding',30)`); err != nil {
		t.Fatal(err)
	}
}

func TestMemoryEmbeddingUnconfiguredIntentSelectsOnce(t *testing.T) {
	f := seedUnconfiguredEmbeddingSnapshot(t)
	var selected bool
	var contract string
	if err := f.repo.Pool().QueryRow(f.ctx, `SELECT w.embedding_snapshot_contract,s.selected FROM public.platform_workflow_intents w JOIN public.memory_embedding_model_snapshots s ON s.intent_id=w.intent_id WHERE w.intent_id=$1`, f.intentID).Scan(&contract, &selected); err != nil || contract != memoryEmbeddingModelSnapshotVersion || selected {
		t.Fatalf("unconfigured Memory did not keep an unselected snapshot: contract=%q selected=%t err=%v", contract, selected, err)
	}
	f.configure(t)
	first, err := f.app.resolveMemoryEmbeddingAssignment(f.ctx, f.intentID, "", "")
	if err != nil || first.BaseURL != "http://embedding-a.invalid" || first.ModelID != "embedding-a" {
		t.Fatalf("first available assignment failed: model=%q err=%v", first.ModelID, err)
	}
	var saved []byte
	if err := f.repo.Pool().QueryRow(f.ctx, `SELECT snapshot FROM public.memory_embedding_model_snapshots WHERE intent_id=$1`, f.intentID).Scan(&saved); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repo.Pool().Exec(f.ctx, `UPDATE public.memory_embedding_model_snapshots SET snapshot=jsonb_set(snapshot,'{assignment}',(snapshot->'assignment')-'SecretPurpose') WHERE intent_id=$1`, f.intentID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.resolveMemoryEmbeddingAssignment(f.ctx, f.intentID, "", ""); !errors.Is(err, errMemoryEmbeddingModelSnapshotInvalid) {
		t.Fatalf("snapshot without a credential reference was accepted: %v", err)
	}
	if _, err := f.repo.Pool().Exec(f.ctx, `UPDATE public.memory_embedding_model_snapshots SET snapshot=$2 WHERE intent_id=$1`, f.intentID, saved); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repo.Pool().Exec(f.ctx, `UPDATE public.memory_embedding_model_snapshots SET snapshot=jsonb_set(snapshot,'{assignment,SecretPurpose}','""') WHERE intent_id=$1`, f.intentID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.resolveMemoryEmbeddingAssignment(f.ctx, f.intentID, "", ""); !errors.Is(err, errMemoryEmbeddingModelSnapshotInvalid) {
		t.Fatalf("snapshot with a corrupted credential reference was accepted: %v", err)
	}
	if _, err := f.repo.Pool().Exec(f.ctx, `UPDATE public.memory_embedding_model_snapshots SET snapshot=$2 WHERE intent_id=$1`, f.intentID, saved); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repo.Pool().Exec(f.ctx, `UPDATE public.memory_embedding_model_snapshots SET snapshot=jsonb_set(snapshot,'{assignment}',(snapshot->'assignment')-'EndpointRevision') WHERE intent_id=$1`, f.intentID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.resolveMemoryEmbeddingAssignment(f.ctx, f.intentID, "", ""); !errors.Is(err, errMemoryEmbeddingModelSnapshotInvalid) {
		t.Fatalf("snapshot without an endpoint revision was accepted: %v", err)
	}
	if _, err := f.repo.Pool().Exec(f.ctx, `UPDATE public.memory_embedding_model_snapshots SET snapshot=$2 WHERE intent_id=$1`, f.intentID, saved); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repo.Pool().Exec(f.ctx, `UPDATE public.provider_endpoints SET base_url='http://embedding-b.invalid' WHERE id='embedding-snapshot-endpoint'`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repo.Pool().Exec(f.ctx, `UPDATE public.model_roles SET model_id='embedding-b' WHERE role='embedding'`); err != nil {
		t.Fatal(err)
	}
	restarted := &App{DB: f.repo, Provider: &ProviderClient{DB: f.repo}}
	restored, err := restarted.resolveMemoryEmbeddingAssignment(f.ctx, f.intentID, "", "")
	if err != nil || restored.BaseURL != first.BaseURL || restored.ModelID != first.ModelID || restored.Timeout != first.Timeout {
		t.Fatalf("retry changed an unconfigured intent's first binding: model=%q err=%v", restored.ModelID, err)
	}
	var requestDigest string
	if err := f.repo.Pool().QueryRow(f.ctx, `SELECT payload->>'request_digest' FROM public.platform_workflow_intents WHERE intent_id=$1`, f.intentID).Scan(&requestDigest); err != nil {
		t.Fatal(err)
	}
	row := memoryAuthorityRow{ID: f.memoryID, OwnerFluctlightID: f.command.OwnerFluctlightID, Revision: 0, RequestDigest: requestDigest}
	if err := withTransaction(f.ctx, f.repo.Pool(), func(tx pgx.Tx) error { return f.app.enqueueMemoryEmbeddingTx(f.ctx, tx, row, f.command) }); err != nil {
		t.Fatalf("duplicate enqueue changed a previously selected binding: %v", err)
	}
	if _, err := f.repo.Pool().Exec(f.ctx, `UPDATE public.memory_embedding_model_snapshots SET snapshot='{}' WHERE intent_id=$1`, f.intentID); err != nil {
		t.Fatal(err)
	}
	if _, err := restarted.resolveMemoryEmbeddingAssignment(f.ctx, f.intentID, "", ""); !errors.Is(err, errMemoryEmbeddingModelSnapshotInvalid) {
		t.Fatalf("cleared selected snapshot silently chose a new model: %v", err)
	}
	if _, err := f.repo.Pool().Exec(f.ctx, `UPDATE public.memory_embedding_model_snapshots SET snapshot=$2 WHERE intent_id=$1`, f.intentID, saved); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repo.Pool().Exec(f.ctx, `DELETE FROM public.memory_embedding_model_snapshots WHERE intent_id=$1`, f.intentID); err != nil {
		t.Fatal(err)
	}
	if _, err := restarted.resolveMemoryEmbeddingAssignment(f.ctx, f.intentID, "", ""); !errors.Is(err, errMemoryEmbeddingModelSnapshotInvalid) {
		t.Fatalf("missing snapshot silently chose a new model: %v", err)
	}
}

func TestMemoryEmbeddingSelectionSurvivesCredentialFailure(t *testing.T) {
	f := seedUnconfiguredEmbeddingSnapshot(t)
	f.configure(t)
	validKey := []byte("0123456789abcdef0123456789abcdef")
	wrongKey := []byte("abcdef0123456789abcdef0123456789")
	badSecret, err := encryptSecret(wrongKey, "embedding-snapshot-secret", "fixture-secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.repo.Pool().Exec(f.ctx, `INSERT INTO public.setting_secrets(purpose,ciphertext,nonce) VALUES('embedding-snapshot-secret',$1,$2)`, badSecret.ciphertext, badSecret.nonce); err != nil {
		t.Fatal(err)
	}
	f.app.Provider.SettingsKey = validKey
	if _, err := f.app.ProcessMemoryEmbeddingIntentAt(f.ctx, f.intentID, f.memoryID, 0, "", ""); err == nil {
		t.Fatal("invalid credential unexpectedly succeeded")
	}
	var raw []byte
	var selected bool
	if err := f.repo.Pool().QueryRow(f.ctx, `SELECT snapshot,selected FROM public.memory_embedding_model_snapshots WHERE intent_id=$1`, f.intentID).Scan(&raw, &selected); err != nil || !selected {
		t.Fatalf("first binding was not saved before credential failure: selected=%t err=%v", selected, err)
	}
	if _, err := f.repo.Pool().Exec(f.ctx, `UPDATE public.model_roles SET model_id='embedding-b' WHERE role='embedding'`); err != nil {
		t.Fatal(err)
	}
	validSecret, err := encryptSecret(validKey, "embedding-snapshot-secret", "fixture-secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.repo.Pool().Exec(f.ctx, `UPDATE public.setting_secrets SET ciphertext=$1,nonce=$2 WHERE purpose='embedding-snapshot-secret'`, validSecret.ciphertext, validSecret.nonce); err != nil {
		t.Fatal(err)
	}
	var usedModel string
	f.app.Provider.HTTP = &http.Client{Transport: projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		var payload map[string]any
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			return nil, err
		}
		usedModel = stringValue(payload["model"])
		return embeddingHTTPResponse(request, http.StatusOK, `{"data":[{"embedding":[0.1]}]}`), nil
	})}
	result, err := f.app.ProcessMemoryEmbeddingIntentAt(f.ctx, f.intentID, f.memoryID, 0, "", "")
	if err != nil || result["status"] != "ready" || usedModel != "embedding-a" {
		t.Fatalf("credential retry changed the first selected model: model=%q result=%v err=%v", usedModel, result, err)
	}
}

func TestMemoryEmbeddingEmptyPurposeDoesNotAdoptNewCredential(t *testing.T) {
	f := seedUnconfiguredEmbeddingSnapshot(t)
	f.configure(t)
	if _, err := f.repo.Pool().Exec(f.ctx, `UPDATE public.provider_endpoints SET secret_purpose='' WHERE id='embedding-snapshot-endpoint'`); err != nil {
		t.Fatal(err)
	}
	first, err := f.app.resolveMemoryEmbeddingAssignment(f.ctx, f.intentID, "", "")
	if err != nil || first.SecretPurpose != "" || first.Secret != "" {
		t.Fatalf("empty-purpose binding failed: %v", err)
	}
	if _, err := f.repo.Pool().Exec(f.ctx, `UPDATE public.provider_endpoints SET secret_purpose='embedding-snapshot-secret' WHERE id='embedding-snapshot-endpoint'`); err != nil {
		t.Fatal(err)
	}
	restored, err := f.app.resolveMemoryEmbeddingAssignment(f.ctx, f.intentID, "", "")
	if err != nil || restored.SecretPurpose != "" || restored.Secret != "" {
		t.Fatalf("unauthenticated snapshot adopted a later credential: %v", err)
	}
}

func TestLegacyMemoryEmbeddingReportsOwnerAndKeepsExistingBinding(t *testing.T) {
	f := seedUnconfiguredEmbeddingSnapshot(t)
	f.configure(t)
	if _, err := f.app.resolveMemoryEmbeddingAssignment(f.ctx, f.intentID, "", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repo.Pool().Exec(f.ctx, `DELETE FROM public.memory_embedding_model_snapshots WHERE intent_id=$1`, f.intentID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repo.Pool().Exec(f.ctx, `UPDATE public.platform_workflow_intents SET embedding_snapshot_contract='' WHERE intent_id=$1`, f.intentID); err != nil {
		t.Fatal(err)
	}
	assignment, err := f.app.resolveMemoryEmbeddingAssignment(f.ctx, f.intentID, "", "")
	if err != nil || assignment.EndpointID != "embedding-snapshot-endpoint" || assignment.ModelID != "embedding-a" {
		t.Fatalf("legacy embedding binding changed: model=%q err=%v", assignment.ModelID, err)
	}
	var count int
	if err := f.repo.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.diagnostic_events WHERE event_type='memory_embedding.model_binding.legacy_unfrozen' AND correlation_id=$1 AND fluctlight_id=$2`, f.intentID, f.command.OwnerFluctlightID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("legacy warning was not associated with its owner: count=%d err=%v", count, err)
	}
}

func TestMemoryEmbeddingRejectsRotatedCredentialForChangedEndpoint(t *testing.T) {
	f := seedUnconfiguredEmbeddingSnapshot(t)
	f.configure(t)
	key := []byte("0123456789abcdef0123456789abcdef")
	oldSecret, err := encryptSecret(key, "embedding-snapshot-secret", "old-embedding-secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.repo.Pool().Exec(f.ctx, `INSERT INTO public.setting_secrets(purpose,ciphertext,nonce) VALUES('embedding-snapshot-secret',$1,$2)`, oldSecret.ciphertext, oldSecret.nonce); err != nil {
		t.Fatal(err)
	}
	calls := 0
	f.app.Provider.SettingsKey = key
	f.app.Provider.HTTP = &http.Client{Transport: projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls++
		return embeddingHTTPResponse(request, http.StatusOK, `{"data":[{"embedding":[0.1]}]}`), nil
	})}
	if _, err := f.app.resolveMemoryEmbeddingAssignment(f.ctx, f.intentID, "", ""); err != nil {
		t.Fatal(err)
	}
	rotated, err := encryptSecret(key, "embedding-snapshot-secret", "new-embedding-secret")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.repo.Pool().Exec(f.ctx, `UPDATE public.provider_endpoints SET base_url='http://embedding-b.invalid' WHERE id='embedding-snapshot-endpoint'`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repo.Pool().Exec(f.ctx, `UPDATE public.setting_secrets SET ciphertext=$1,nonce=$2 WHERE purpose='embedding-snapshot-secret'`, rotated.ciphertext, rotated.nonce); err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.ProcessMemoryEmbeddingIntentAt(f.ctx, f.intentID, f.memoryID, 0, "", ""); err == nil || !strings.Contains(err.Error(), "provider_snapshot_credential_destination_changed") || calls != 0 {
		t.Fatalf("changed endpoint accepted a rotated credential: calls=%d err=%v", calls, err)
	}
	var count int
	var status string
	if err := f.repo.Pool().QueryRow(f.ctx, `SELECT (SELECT count(*) FROM public.memory_embeddings WHERE memory_id=$1),status FROM public.memories WHERE id=$1`, f.memoryID).Scan(&count, &status); err != nil || count != 0 || status != "active" {
		t.Fatalf("failed restore changed Memory or created an embedding: count=%d status=%q err=%v", count, status, err)
	}
}

func TestMemoryEmbeddingSnapshotFailureRollsBackMemory(t *testing.T) {
	f := seedUnconfiguredEmbeddingSnapshot(t)
	var originalIntents int
	if err := f.repo.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.platform_workflow_intents WHERE intent_type='memory.embedding'`).Scan(&originalIntents); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repo.Pool().Exec(f.ctx, `CREATE FUNCTION public.test_fail_embedding_snapshot() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected_embedding_snapshot_failure'; END $$; CREATE TRIGGER test_fail_embedding_snapshot BEFORE INSERT ON public.memory_embedding_model_snapshots FOR EACH ROW EXECUTE FUNCTION public.test_fail_embedding_snapshot()`); err != nil {
		t.Fatal(err)
	}
	semantic := &MemorySemanticInput{Type: "semantic", Content: "must roll back with snapshot", Confidence: 0.9, Importance: 0.8}
	command := memoryLifecycleTestCommand(MemoryCreate, "embedding-snapshot-rollback", semantic, nil, nil)
	command.OwnerFluctlightID, command.OwnerActorID, command.ActorID = f.command.OwnerFluctlightID, f.command.OwnerActorID, f.command.ActorID
	command.RequestDigest = memoryCommandDigest(command)
	err := withTransaction(f.ctx, f.repo.Pool(), func(tx pgx.Tx) error {
		_, err := f.app.applyMemoryCommandTx(f.ctx, tx, command)
		return err
	})
	if err == nil || !strings.Contains(err.Error(), "injected_embedding_snapshot_failure") {
		t.Fatalf("injected snapshot failure was not surfaced: %v", err)
	}
	var memories, intents int
	if err := f.repo.Pool().QueryRow(f.ctx, `SELECT (SELECT count(*) FROM public.memories WHERE content='must roll back with snapshot'),(SELECT count(*) FROM public.platform_workflow_intents WHERE intent_type='memory.embedding')`).Scan(&memories, &intents); err != nil || memories != 0 || intents != originalIntents {
		t.Fatalf("snapshot failure left partial writes: memories=%d intents=%d err=%v", memories, intents, err)
	}
}

func TestMemoryEmbeddingConcurrentSelectionUsesStoredWinner(t *testing.T) {
	f := seedUnconfiguredEmbeddingSnapshot(t)
	f.configure(t)
	key := []byte("0123456789abcdef0123456789abcdef")
	const fixtureSecret = "embedding-race-fixture-secret"
	encrypted, err := encryptSecret(key, "embedding-snapshot-secret", fixtureSecret)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.repo.Pool().Exec(f.ctx, `INSERT INTO public.setting_secrets(purpose,ciphertext,nonce) VALUES('embedding-snapshot-secret',$1,$2)`, encrypted.ciphertext, encrypted.nonce); err != nil {
		t.Fatal(err)
	}
	f.app.Provider.SettingsKey = key
	candidate, err := f.app.Provider.assignment(f.ctx, "embedding")
	if err != nil || candidate.Secret != fixtureSecret {
		t.Fatalf("race fixture could not load its credential: %v", err)
	}
	lockTx, err := f.repo.Pool().Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer lockTx.Rollback(f.ctx)
	var raw []byte
	if err := lockTx.QueryRow(f.ctx, `SELECT payload FROM public.platform_workflow_intents WHERE intent_id=$1 FOR UPDATE`, f.intentID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	type result struct {
		assignment providerAssignment
		err        error
	}
	start, results := make(chan struct{}), make(chan result, 2)
	var workers sync.WaitGroup
	for _, model := range []string{"embedding-candidate-a", "embedding-candidate-b"} {
		workers.Add(1)
		go func(model string) {
			defer workers.Done()
			binding := candidate
			binding.ModelID = model
			candidateCtx := withFrozenProviderAssignment(f.ctx, "embedding", binding)
			<-start
			assignment, err := f.app.resolveMemoryEmbeddingAssignment(candidateCtx, f.intentID, "", "")
			results <- result{assignment, err}
		}(model)
	}
	close(start)
	var waiting int
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if err := f.repo.Pool().QueryRow(f.ctx, `SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock' AND query LIKE 'SELECT payload,embedding_snapshot_contract FROM public.platform_workflow_intents%FOR UPDATE'`).Scan(&waiting); err != nil {
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
		t.Fatalf("selectors did not reach the intent lock: waiting=%d", waiting)
	}
	if err := lockTx.Commit(f.ctx); err != nil {
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
	if err := f.repo.Pool().QueryRow(f.ctx, `SELECT snapshot FROM public.memory_embedding_model_snapshots WHERE intent_id=$1`, f.intentID).Scan(&raw); err != nil || strings.Contains(string(raw), fixtureSecret) {
		t.Fatalf("snapshot exposed a credential or could not be read: %v", err)
	}
	var stored memoryEmbeddingModelSnapshot
	if err := json.Unmarshal(raw, &stored); err != nil {
		t.Fatal(err)
	}
	if err := f.app.restoreAssignmentCredential(f.ctx, &stored.Assignment); err != nil || !selected || !reflect.DeepEqual(winner, stored.Assignment) || winner.Secret != fixtureSecret {
		t.Fatalf("selectors did not use the persisted winner and credential: %v", err)
	}
	if _, err := f.repo.Pool().Exec(f.ctx, `UPDATE public.memory_embedding_model_snapshots SET snapshot=jsonb_set(snapshot,'{assignment,CredentialRevision}','0') WHERE intent_id=$1`, f.intentID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.resolveMemoryEmbeddingAssignment(f.ctx, f.intentID, "", ""); !errors.Is(err, errMemoryEmbeddingModelSnapshotInvalid) {
		t.Fatalf("snapshot with a corrupted credential revision was accepted: %v", err)
	}
}
