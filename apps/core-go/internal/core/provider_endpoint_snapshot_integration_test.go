package core

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type endpointReadTraceKey struct{}

// Rotate both values after the endpoint query has returned its row, before
// the caller can issue another SQL statement for the credential.
type endpointReadRotationTracer struct {
	once   sync.Once
	rotate func() error
	err    error
}

func TestProviderEndpointSnapshotPreservesUnauthenticatedAndPreflight(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'fixture','fixture') ON CONFLICT DO NOTHING`, f.ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status) VALUES('endpoint-no-auth','openai_compatible','http://no-auth.invalid','missing-secret','ready')`); err != nil {
		t.Fatal(err)
	}
	called := false
	provider := &ProviderClient{DB: f.repository, HTTP: &http.Client{Transport: projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		called = request.URL.Host == "no-auth.invalid" && request.Header.Get("Authorization") == ""
		return embeddingHTTPResponse(request, http.StatusOK, `{"data":[{"id":"fixture"}]}`), nil
	})}}
	app := &App{DB: f.repository, Provider: provider}
	if _, err := app.ProviderModels(f.ctx, f.ownerID, "endpoint-no-auth"); err != nil || !called {
		t.Fatalf("unauthenticated model directory failed: called=%t err=%v", called, err)
	}
	assignment, err := provider.embeddingAssignmentByID(f.ctx, "endpoint-no-auth", "embedding-fixture")
	if err != nil || assignment.Secret != "" || assignment.EndpointRevision < 1 || assignment.SecretPurpose != "missing-secret" {
		t.Fatalf("unauthenticated embedding snapshot unavailable: endpoint_revision=%d err=%v", assignment.EndpointRevision, err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.provider_endpoints SET capability_status='failed' WHERE id='endpoint-no-auth'`); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.embeddingAssignmentByID(f.ctx, "endpoint-no-auth", "embedding-fixture"); err == nil || err.Error() != "embedding_assignment_preflight_failed" {
		t.Fatalf("failed embedding preflight was not preserved: %v", err)
	}
	if _, err := provider.embeddingAssignmentByID(f.ctx, "endpoint-not-found", "embedding-fixture"); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("missing endpoint did not return no rows: %v", err)
	}
}

func (t *endpointReadRotationTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	sql := strings.TrimSpace(data.SQL)
	match := (strings.HasPrefix(sql, "SELECT base_url,secret_purpose") || strings.HasPrefix(sql, "SELECT e.base_url,COALESCE(e.secret_purpose")) && len(data.Args) > 0 && data.Args[0] == "endpoint-atomic-test"
	return context.WithValue(ctx, endpointReadTraceKey{}, match)
}

func (t *endpointReadRotationTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	if match, _ := ctx.Value(endpointReadTraceKey{}).(bool); match && data.Err == nil {
		t.once.Do(func() { t.err = t.rotate() })
	}
}

func TestProviderEndpointAndCredentialReadAsOneSnapshot(t *testing.T) {
	for _, operation := range []string{"models", "embedding"} {
		t.Run(operation, func(t *testing.T) {
			f := seedWardrobeToolFixture(t)
			if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'fixture','fixture') ON CONFLICT DO NOTHING`, f.ownerID); err != nil {
				t.Fatal(err)
			}
			const oldSecret, newSecret = "endpoint-old-secret", "endpoint-new-secret"
			key := []byte("0123456789abcdef0123456789abcdef")
			oldValue, err := encryptSecret(key, "endpoint-test-secret", oldSecret)
			if err != nil {
				t.Fatal(err)
			}
			newValue, err := encryptSecret(key, "endpoint-test-secret", newSecret)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.provider_endpoints(id,kind,base_url,secret_purpose,capability_status) VALUES('endpoint-atomic-test','openai_compatible','http://old.invalid','endpoint-test-secret','ready')`); err != nil {
				t.Fatal(err)
			}
			if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.setting_secrets(purpose,ciphertext,nonce) VALUES('endpoint-test-secret',$1,$2)`, oldValue.ciphertext, oldValue.nonce); err != nil {
				t.Fatal(err)
			}
			var endpointRevision, credentialRevision int64
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT (SELECT revision FROM public.configuration_revisions WHERE resource_key='endpoint:endpoint-atomic-test'),(SELECT revision FROM public.configuration_revisions WHERE resource_key='secret:endpoint-test-secret')`).Scan(&endpointRevision, &credentialRevision); err != nil {
				t.Fatal(err)
			}
			tracer := &endpointReadRotationTracer{rotate: func() error {
				return withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
					if _, err := tx.Exec(f.ctx, `UPDATE public.provider_endpoints SET base_url='http://new.invalid' WHERE id='endpoint-atomic-test'`); err != nil {
						return err
					}
					_, err := tx.Exec(f.ctx, `UPDATE public.setting_secrets SET ciphertext=$1,nonce=$2 WHERE purpose='endpoint-test-secret'`, newValue.ciphertext, newValue.nonce)
					return err
				})
			}}
			config := f.repository.Pool().Config()
			config.ConnConfig.Tracer = tracer
			pool, err := pgxpool.NewWithConfig(f.ctx, config)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(pool.Close)
			repo := &PostgresRepository{pool: pool}
			provider := &ProviderClient{DB: repo, SettingsKey: key}
			if operation == "embedding" {
				assignment, err := provider.embeddingAssignmentByID(f.ctx, "endpoint-atomic-test", "embedding-fixture")
				if err != nil || assignment.BaseURL != "http://old.invalid" || assignment.Secret != oldSecret || assignment.EndpointRevision != endpointRevision || assignment.CredentialRevision != credentialRevision {
					t.Fatalf("embedding paired a destination with the wrong credential: %v", err)
				}
				provider.HTTP = &http.Client{Transport: projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
					if request.URL.Host != "old.invalid" || request.Header.Get("Authorization") != "Bearer "+oldSecret {
						t.Fatal("embedding sent the wrong credential to its endpoint")
					}
					return embeddingHTTPResponse(request, http.StatusOK, `{"data":[{"embedding":[0.1,0.2],"index":0}],"model":"embedding-fixture"}`), nil
				})}
				if _, _, err := provider.embedWithAssignment(f.ctx, "controlled test", assignment); err != nil {
					t.Fatal(err)
				}
				var metrics []byte
				if err := f.repository.Pool().QueryRow(f.ctx, `SELECT metrics FROM public.diagnostic_model_runs WHERE role='embedding' AND endpoint_id='endpoint-atomic-test' ORDER BY queued_at DESC LIMIT 1`).Scan(&metrics); err != nil {
					t.Fatal(err)
				}
				configuration := mapValue(decodeObject(metrics)["model_configuration"])
				if intValue(configuration["endpoint_revision"]) != int(endpointRevision) || intValue(configuration["credential_revision"]) != int(credentialRevision) || strings.Contains(string(metrics), oldSecret) || strings.Contains(string(metrics), newSecret) {
					t.Fatal("embedding diagnostics did not preserve safe configuration versions")
				}
			} else {
				var correct bool
				provider.HTTP = &http.Client{Transport: projectHealthRoundTripFunc(func(request *http.Request) (*http.Response, error) {
					correct = request.URL.Host == "old.invalid" && request.Header.Get("Authorization") == "Bearer "+oldSecret
					return embeddingHTTPResponse(request, http.StatusOK, `{"data":[{"id":"fixture"}]}`), nil
				})}
				app := &App{DB: repo, Provider: provider}
				if _, err := app.ProviderModels(f.ctx, f.ownerID, "endpoint-atomic-test"); err != nil || !correct {
					t.Fatalf("model directory paired a destination with the wrong credential: %v", err)
				}
			}
			if tracer.err != nil {
				t.Fatalf("controlled endpoint rotation failed: %v", tracer.err)
			}
			var base string
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT base_url FROM public.provider_endpoints WHERE id='endpoint-atomic-test'`).Scan(&base); err != nil || base != "http://new.invalid" {
				t.Fatalf("test did not perform the rotation: %v", err)
			}
		})
	}
}
