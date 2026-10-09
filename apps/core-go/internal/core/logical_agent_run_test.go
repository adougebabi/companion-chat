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
