package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/cloudwego/eino/schema"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

func queueTimeoutTestProvider(t *testing.T, useRedis bool) *ProviderClient {
	t.Helper()
	p := &ProviderClient{}
	t.Cleanup(func() {
		p.queueFor("generic_llm").close()
		p.queueFor("embedding").close()
	})
	if useRedis {
		server := miniredis.RunT(t)
		client := redis.NewClient(&redis.Options{Addr: server.Addr()})
		t.Cleanup(func() { _ = client.Close() })
		p.SetRedisClient(client, "queue-timeout-test")
	}
	return p
}

func occupyTimeoutTestSlot(t *testing.T, p *ProviderClient, useRedis bool) func() {
	t.Helper()
	if useRedis {
		release, enabled, err := p.acquireProviderRedisSlot(context.Background(), "generic_llm", 1, 1, "blocker")
		if err != nil || !enabled {
			t.Fatalf("occupy Redis slot: enabled=%t err=%v", enabled, err)
		}
		t.Cleanup(release)
		return release
	}
	started, release, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	go func() {
		done <- p.queueFor("generic_llm").submit(context.Background(), 1, func(context.Context) error {
			close(started)
			<-release
			return nil
		}, nil)
	}()
	if !waitForStreamSignal(started) {
		t.Fatal("blocker did not start")
	}
	var once sync.Once
	unblock := func() { once.Do(func() { close(release); <-done }) }
	t.Cleanup(unblock)
	return unblock
}

func invokeQueueTimeoutTest(p *ProviderClient, ctx context.Context, streaming bool, fn func(context.Context) error, diagnosticIDs ...string) error {
	diagnosticID := "test-run"
	if len(diagnosticIDs) > 0 {
		diagnosticID = diagnosticIDs[0]
	} else if p.DB != nil {
		diagnosticID = p.runtimeSupport().RecordQueuedModelRun(ctx, "generic_llm", "endpoint-test", "model-test", randomID("queue-timeout-test-"), "queue-timeout-test", 1, nil)
		if diagnosticID == "" {
			return errors.New("queued model run was not recorded")
		}
		defer func() {
			_, _ = p.DB.Pool().Exec(context.Background(), `DELETE FROM public.diagnostic_model_runs WHERE id=$1`, diagnosticID)
		}()
	}
	if !streaming {
		_, err := runProviderQueued(p, ctx, "generic_llm", "queue-timeout-test", 1, diagnosticID, func(ctx context.Context) (string, error) {
			return "ok", fn(ctx)
		})
		return err
	}
	reader, err := runProviderQueuedStream(p, ctx, "generic_llm", "queue-timeout-test", 1, diagnosticID, func(ctx context.Context) (*schema.StreamReader[*schema.Message], error) {
		if err := fn(ctx); err != nil {
			return nil, err
		}
		return schema.StreamReaderFromArray([]*schema.Message{{Role: schema.Assistant, Content: "ok"}}), nil
	})
	if err != nil {
		return err
	}
	defer reader.Close()
	for {
		_, err := reader.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func TestProviderExecutionTimeoutStartsAfterQueueAdmission(t *testing.T) {
	for _, useRedis := range []bool{false, true} {
		for _, streaming := range []bool{false, true} {
			t.Run(fmt.Sprintf("redis=%t/stream=%t", useRedis, streaming), func(t *testing.T) {
				p := queueTimeoutTestProvider(t, useRedis)
				release := occupyTimeoutTestSlot(t, p, useRedis)
				const executionTimeout = 200 * time.Millisecond
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				ctx = withProviderTimeout(ctx, executionTimeout)
				done := make(chan error, 1)
				go func() {
					done <- invokeQueueTimeoutTest(p, ctx, streaming, func(runCtx context.Context) error {
						deadline, ok := runCtx.Deadline()
						if !ok || time.Until(deadline) < executionTimeout*3/4 {
							return fmt.Errorf("execution did not receive a fresh deadline: %v", deadline)
						}
						select {
						case <-time.After(executionTimeout / 2):
							return nil
						case <-runCtx.Done():
							return runCtx.Err()
						}
					})
				}()
				select {
				case err := <-done:
					t.Fatalf("call returned before slot release: %v", err)
				case <-time.After(2 * executionTimeout):
				}
				release()
				if err := <-done; err != nil {
					t.Fatalf("queue wait consumed execution time: %v", err)
				}
			})
		}
	}
}

func TestProviderQueueTimeoutPreservesExecutionAndCallerCancellation(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		for _, callerDeadline := range []bool{false, true} {
			t.Run(fmt.Sprintf("stream=%t/caller-deadline=%t", streaming, callerDeadline), func(t *testing.T) {
				p := queueTimeoutTestProvider(t, false)
				ctx := context.Background()
				if callerDeadline {
					var cancel context.CancelFunc
					ctx, cancel = context.WithTimeout(ctx, 40*time.Millisecond)
					defer cancel()
					ctx = withProviderTimeout(ctx, time.Second)
				} else {
					ctx = withProviderTimeout(ctx, 40*time.Millisecond)
				}
				err := invokeQueueTimeoutTest(p, ctx, streaming, func(runCtx context.Context) error {
					<-runCtx.Done()
					return runCtx.Err()
				})
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("deadline error = %v", err)
				}
				if err := invokeQueueTimeoutTest(p, context.Background(), streaming, func(context.Context) error { return nil }); err != nil {
					t.Fatalf("expired execution retained slot: %v", err)
				}
			})
		}
	}
}

func TestProviderStreamExecutionTimeoutIncludesReading(t *testing.T) {
	p := queueTimeoutTestProvider(t, false)
	inner, writer := schema.Pipe[*schema.Message](1)
	var closeOnce sync.Once
	closeWriter := func() { closeOnce.Do(writer.Close) }
	defer closeWriter()
	reader, err := runProviderQueuedStream(p, withProviderTimeout(context.Background(), 40*time.Millisecond), "generic_llm", "read-timeout", 1, "", func(ctx context.Context) (*schema.StreamReader[*schema.Message], error) {
		// Model the transport producer exiting when its request is cancelled.
		go func() { <-ctx.Done(); closeWriter() }()
		return inner, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	if _, err := reader.Recv(); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("stream reading escaped execution deadline: %v", err)
	}
	if err := invokeQueueTimeoutTest(p, context.Background(), true, func(context.Context) error { return nil }); err != nil {
		t.Fatalf("timed out stream retained slot: %v", err)
	}
}

func TestProviderQueueWaitSettingsValidation(t *testing.T) {
	if got := normalizeProviderQueueSettings(nil)["queue_wait_timeout_seconds"]; got != 1800 {
		t.Fatalf("default wait timeout = %v", got)
	}
	for _, value := range []any{1, float64(300), 86400} {
		if got := normalizeProviderQueueSettings(map[string]any{"queue_wait_timeout_seconds": value}); got == nil {
			t.Fatalf("valid queue timeout rejected: %v", value)
		}
	}
	for _, value := range []any{0, -1, 86401, 1.5, "300", nil} {
		if got := normalizeProviderQueueSettings(map[string]any{"queue_wait_timeout_seconds": value}); got != nil {
			t.Fatalf("invalid queue timeout accepted: %v -> %v", value, got)
		}
	}
}

func TestPostgresProviderQueueWaitTimeoutConfiguration(t *testing.T) {
	databaseURL := os.Getenv("GO_CORE_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("GO_CORE_TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	repository, err := NewPostgresRepository(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()
	var previous string
	previousErr := repository.Pool().QueryRow(ctx, `SELECT value_json FROM public.runtime_settings WHERE key='llm.queue'`).Scan(&previous)
	if previousErr != nil && !errors.Is(previousErr, pgx.ErrNoRows) {
		t.Fatal(previousErr)
	}
	defer func() {
		if previousErr == nil {
			_, err = repository.Pool().Exec(ctx, `UPDATE public.runtime_settings SET value_json=$1 WHERE key='llm.queue'`, previous)
		} else {
			_, err = repository.Pool().Exec(ctx, `DELETE FROM public.runtime_settings WHERE key='llm.queue'`)
		}
		if err != nil {
			t.Errorf("restore queue settings: %v", err)
		}
	}()
	if _, err := repository.Pool().Exec(ctx, `INSERT INTO public.runtime_settings(key,value_json) VALUES('llm.queue','{"generated_concurrency":1,"embedding_concurrency":1,"queue_wait_timeout_seconds":1}') ON CONFLICT(key) DO UPDATE SET value_json=excluded.value_json`); err != nil {
		t.Fatal(err)
	}
	for _, useRedis := range []bool{false, true} {
		for _, streaming := range []bool{false, true} {
			t.Run(fmt.Sprintf("redis=%t/stream=%t", useRedis, streaming), func(t *testing.T) {
				p := queueTimeoutTestProvider(t, useRedis)
				p.DB = repository
				release := occupyTimeoutTestSlot(t, p, useRedis)
				diagnosticID := p.runtimeSupport().RecordQueuedModelRun(ctx, "generic_llm", "endpoint-test", "model-test", randomID("queue-timeout-test-"), "queue-timeout-test", 1, nil)
				if diagnosticID == "" {
					t.Fatal("queued model run was not recorded")
				}
				defer func() {
					_, _ = repository.Pool().Exec(ctx, `DELETE FROM public.diagnostic_model_runs WHERE id=$1`, diagnosticID)
				}()
				var executed atomic.Bool
				callCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
				defer cancel()
				err := invokeQueueTimeoutTest(p, withProviderTimeout(callCtx, 2*time.Second), streaming, func(context.Context) error {
					executed.Store(true)
					return nil
				}, diagnosticID)
				if !errors.Is(err, context.DeadlineExceeded) || callCtx.Err() != nil || executed.Load() {
					t.Fatalf("independent wait timeout: err=%v parent=%v executed=%t", err, callCtx.Err(), executed.Load())
				}
				var status, code string
				var neverStarted bool
				if err := repository.Pool().QueryRow(ctx, `SELECT status,COALESCE(error_code,''),started_at IS NULL FROM public.diagnostic_model_runs WHERE id=$1`, diagnosticID).Scan(&status, &code, &neverStarted); err != nil {
					t.Fatal(err)
				}
				if status != providerRunTimeout || code != "request_timeout" || !neverStarted {
					t.Fatalf("queue timeout diagnostic: status=%q code=%q neverStarted=%t", status, code, neverStarted)
				}
				release()
				if err := invokeQueueTimeoutTest(p, callCtx, streaming, func(context.Context) error { return nil }); err != nil {
					t.Fatalf("queue timeout blocked subsequent call: %v", err)
				}
			})
		}
	}
	for _, streaming := range []bool{false, true} {
		t.Run(fmt.Sprintf("execution-outlives-wait-budget/stream=%t", streaming), func(t *testing.T) {
			p := queueTimeoutTestProvider(t, false)
			p.DB = repository
			callCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()
			err := invokeQueueTimeoutTest(p, withProviderTimeout(callCtx, 2*time.Second), streaming, func(runCtx context.Context) error {
				select {
				case <-time.After(1200 * time.Millisecond):
					return nil
				case <-runCtx.Done():
					return runCtx.Err()
				}
			})
			if err != nil {
				t.Fatalf("wait budget cancelled an admitted execution: %v", err)
			}
		})
	}
}
