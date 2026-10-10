package core

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"sync"
	"testing"
	"time"
)

type memoryLogicalLeases struct {
	mu      sync.Mutex
	tokens  map[string]string
	expired map[string]bool
}

func (s *memoryLogicalLeases) TryAcquire(_ context.Context, owner, token, _ string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tokens[owner] != "" && !s.expired[owner] {
		return false, nil
	}
	s.tokens[owner] = token
	s.expired[owner] = false
	return true, nil
}
func (s *memoryLogicalLeases) Renew(_ context.Context, owner, token string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.tokens[owner] == token && !s.expired[owner], nil
}
func (s *memoryLogicalLeases) Release(_ context.Context, owner, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tokens[owner] == token {
		delete(s.tokens, owner)
	}
	return nil
}
func (s *memoryLogicalLeases) Fence(_ context.Context, _ pgx.Tx, owner, token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.tokens[owner] != token || s.expired[owner] {
		return errLogicalAgentLeaseLost
	}
	return nil
}
func newMemoryLogicalLeases() *memoryLogicalLeases {
	return &memoryLogicalLeases{tokens: map[string]string{}, expired: map[string]bool{}}
}
func TestLogicalAgentRunSerializesSnapshotAcrossModelToolRounds(t *testing.T) {
	for _, firstKind := range []string{"wakeup", "cognition", "goal_evaluation"} {
		t.Run(firstKind, func(t *testing.T) {
			store := newMemoryLogicalLeases()
			ctx, release, err := enterLogicalAgentRun(context.Background(), store, "actor", firstKind)
			if err != nil {
				t.Fatal(err)
			}
			nested, nestedRelease, err := enterLogicalAgentRun(ctx, store, "actor", "nested_agent")
			if err != nil {
				t.Fatal(err)
			}
			nestedRelease()
			if err := fenceLogicalAgentRun(nested, nil); err != nil {
				t.Fatal("nested release erased parent ownership", err)
			}
			admitted := make(chan int, 1)
			var generation int
			go func() {
				_, done, err := enterLogicalAgentRun(context.Background(), store, "actor", "goal_evaluation")
				if err != nil {
					admitted <- -1
					return
				}
				defer done()
				admitted <- generation
			}()
			select {
			case <-admitted:
				t.Fatal("second logical snapshot started between model rounds")
			case <-time.After(30 * time.Millisecond):
			}
			generation = 1 // first loop's actual Tool commit, before its final settlement
			release()
			select {
			case observed := <-admitted:
				if observed != 1 {
					t.Fatal("evaluation froze stale pre-Tool facts", observed)
				}
			case <-time.After(time.Second):
				t.Fatal("waiting run did not proceed")
			}
		})
	}
}
func TestLogicalAgentRunDifferentOwnersAndCancellation(t *testing.T) {
	store := newMemoryLogicalLeases()
	_, release, err := enterLogicalAgentRun(context.Background(), store, "a", "wakeup")
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	_, otherRelease, err := enterLogicalAgentRun(context.Background(), store, "b", "goal_evaluation")
	if err != nil {
		t.Fatal(err)
	}
	otherRelease()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, _, err = enterLogicalAgentRun(ctx, store, "a", "goal_evaluation")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("cancelled waiter acquired or consumed a run", err)
	}
}
func TestLogicalAgentExpiredOwnerCannotCommitOrReleaseSuccessor(t *testing.T) {
	store := newMemoryLogicalLeases()
	old, releaseOld, err := enterLogicalAgentRun(context.Background(), store, "a", "wakeup")
	if err != nil {
		t.Fatal(err)
	}
	store.mu.Lock()
	store.expired["a"] = true
	store.mu.Unlock()
	current, releaseCurrent, err := enterLogicalAgentRun(context.Background(), store, "a", "cognition")
	if err != nil {
		t.Fatal(err)
	}
	defer releaseCurrent()
	if !errors.Is(fenceLogicalAgentRun(old, nil), errLogicalAgentLeaseLost) {
		t.Fatal("stale owner was allowed to commit")
	}
	releaseOld()
	if err := fenceLogicalAgentRun(current, nil); err != nil {
		t.Fatal("old cleanup erased successor", err)
	}
}

func TestBackgroundLogicalEntriesWaitForActiveConversationLease(t *testing.T) {
	entries := []struct {
		name string
		run  func(*App, context.Context, string, func(context.Context) (map[string]any, error)) (map[string]any, error)
	}{
		{name: "conversation_summary", run: func(app *App, ctx context.Context, owner string, body func(context.Context) (map[string]any, error)) (map[string]any, error) {
			return app.runConversationSummaryIntentWork(ctx, owner, body)
		}},
		{name: "conversation_daily_memory", run: func(app *App, ctx context.Context, owner string, body func(context.Context) (map[string]any, error)) (map[string]any, error) {
			return app.runConversationDailyMemoryIntentWork(ctx, owner, body)
		}},
	}
	for _, entry := range entries {
		t.Run(entry.name, func(t *testing.T) {
			store := newMemoryLogicalLeases()
			app := &App{logicalAgentLeases: store}
			_, releaseChat, err := enterLogicalAgentRun(context.Background(), store, "actor", "cognition")
			if err != nil {
				t.Fatal(err)
			}
			events := make(chan string, 3)
			errs := make(chan error, 1)
			go func() {
				_, err := entry.run(app, context.Background(), "actor", func(ctx context.Context) (map[string]any, error) {
					events <- "snapshot"
					if err := ctx.Err(); err != nil {
						return nil, context.Cause(ctx)
					}
					events <- "provider"
					events <- "commit"
					return map[string]any{"status": "completed"}, nil
				})
				errs <- err
			}()
			select {
			case event := <-events:
				t.Fatalf("background entry reached %s while chat held the logical lease", event)
			case err := <-errs:
				t.Fatalf("background entry returned before chat release: %v", err)
			case <-time.After(30 * time.Millisecond):
			}
			releaseChat()
			for _, expected := range []string{"snapshot", "provider", "commit"} {
				select {
				case actual := <-events:
					if actual != expected {
						t.Fatalf("event=%s want %s", actual, expected)
					}
				case <-time.After(time.Second):
					t.Fatalf("background entry did not reach %s after lease release", expected)
				}
			}
			if err := <-errs; err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestBackgroundLogicalEntriesReenterAndKeepOwnersIndependent(t *testing.T) {
	store := newMemoryLogicalLeases()
	app := &App{logicalAgentLeases: store}
	ownerCtx, releaseOwner, err := enterLogicalAgentRun(context.Background(), store, "actor-a", "cognition")
	if err != nil {
		t.Fatal(err)
	}
	defer releaseOwner()
	called := false
	if _, err := app.runConversationSummaryIntentWork(ownerCtx, "actor-a", func(context.Context) (map[string]any, error) {
		called = true
		return map[string]any{"status": "completed"}, nil
	}); err != nil || !called {
		t.Fatalf("same-owner background work did not reenter: called=%v err=%v", called, err)
	}
	if err := fenceLogicalAgentRun(ownerCtx, nil); err != nil {
		t.Fatal("nested background release erased the parent lease", err)
	}
	otherDone := make(chan error, 1)
	go func() {
		_, err := app.runConversationDailyMemoryIntentWork(context.Background(), "actor-b", func(context.Context) (map[string]any, error) {
			return map[string]any{"status": "completed"}, nil
		})
		otherDone <- err
	}()
	select {
	case err := <-otherDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("different owner was blocked by another Fluctlight's lease")
	}
}

func TestBackgroundLogicalEntryCancellationPreservesCause(t *testing.T) {
	store := newMemoryLogicalLeases()
	app := &App{logicalAgentLeases: store}
	_, release, err := enterLogicalAgentRun(context.Background(), store, "actor", "cognition")
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	cause := errors.New("summary_workflow_superseded")
	ctx, cancel := context.WithCancelCause(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := app.runConversationSummaryIntentWork(ctx, "actor", func(context.Context) (map[string]any, error) {
			return nil, errors.New("background body must not run while the lease is held")
		})
		done <- err
	}()
	time.Sleep(20 * time.Millisecond)
	cancel(cause)
	select {
	case err := <-done:
		if !errors.Is(err, cause) {
			t.Fatalf("cancellation cause=%v want %v", err, cause)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled background lease waiter did not stop")
	}

	activeCtx, cancelActive := context.WithCancelCause(context.Background())
	started := make(chan struct{})
	activeDone := make(chan error, 1)
	go func() {
		_, err := app.runConversationDailyMemoryIntentWork(activeCtx, "other-actor", func(runCtx context.Context) (map[string]any, error) {
			close(started)
			<-runCtx.Done()
			return nil, runCtx.Err()
		})
		activeDone <- err
	}()
	<-started
	cancelActive(cause)
	select {
	case err := <-activeDone:
		if !errors.Is(err, cause) {
			t.Fatalf("active work cancellation cause=%v want %v", err, cause)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled active background work did not stop")
	}
}

func TestBackgroundLogicalEntryPassesLeaseFenceToCommit(t *testing.T) {
	store := newMemoryLogicalLeases()
	app := &App{logicalAgentLeases: store}
	_, err := app.runConversationDailyMemoryIntentWork(context.Background(), "actor", func(ctx context.Context) (map[string]any, error) {
		store.mu.Lock()
		store.expired["actor"] = true
		store.mu.Unlock()
		return nil, fenceLogicalAgentRun(ctx, nil)
	})
	if !errors.Is(err, errLogicalAgentLeaseLost) {
		t.Fatalf("expired background lease reached commit fence: %v", err)
	}
}

func TestPostgresLogicalAgentRunCoordinatesIndependentApps(t *testing.T) {
	f := newIndependentToolE2EFixture(t, "logical-run")
	first, releaseFirst, err := f.app.enterLogicalRun(f.ctx, f.fluctlightID, "wakeup")
	if err != nil {
		t.Fatal(err)
	}
	defer releaseFirst()
	secondApp := &App{DB: f.repository}
	ready := make(chan string, 1)
	errs := make(chan error, 1)
	go func() {
		ctx, release, err := secondApp.enterLogicalRun(f.ctx, f.fluctlightID, "goal_evaluation")
		if err != nil {
			errs <- err
			return
		}
		defer release()
		revision, err := readCurrentFactsRevisionWith(ctx, f.repository.Pool(), f.fluctlightID)
		if err != nil {
			errs <- err
			return
		}
		ready <- revision
	}()
	select {
	case <-ready:
		t.Fatal("second App evaluated during first logical run")
	case err := <-errs:
		t.Fatal(err)
	case <-time.After(50 * time.Millisecond):
	}
	err = withTransaction(first, f.repository.Pool(), func(tx pgx.Tx) error {
		_, err := tx.Exec(first, `UPDATE public.fluctlight_context_generations SET generation=generation+1 WHERE fluctlight_id=$1`, f.fluctlightID)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	expected, err := readCurrentFactsRevisionWith(first, f.repository.Pool(), f.fluctlightID)
	if err != nil {
		t.Fatal(err)
	}
	releaseFirst()
	select {
	case actual := <-ready:
		if actual != expected {
			t.Fatalf("stale snapshot: %s want %s", actual, expected)
		}
	case err := <-errs:
		t.Fatal(err)
	case <-time.After(3 * time.Second):
		t.Fatal("waiter did not resume")
	}
}
