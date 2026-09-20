package model

import (
	"context"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestProviderQueueHonorsPriorityAndFIFO(t *testing.T) {
	queue := NewProviderQueue(1)
	defer queue.Close()
	// Occupy the only slot so all following tasks are observed in the heap.
	started := make(chan struct{})
	release := make(chan struct{})
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- queue.Submit(context.Background(), 1, func(context.Context) error {
			close(started)
			<-release
			return nil
		}, nil)
	}()
	<-started
	var mu sync.Mutex
	order := make([]string, 0, 3)
	var wg sync.WaitGroup
	for _, task := range []struct {
		name     string
		priority int
	}{
		{name: "low", priority: 1},
		{name: "high", priority: 3},
		{name: "same", priority: 3},
	} {
		task := task
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = queue.Submit(context.Background(), task.priority, func(context.Context) error {
				mu.Lock()
				order = append(order, task.name)
				mu.Unlock()
				return nil
			}, nil)
		}()
		// The first task is blocked on the occupied slot; this short yield lets
		// the submitter enqueue before the next priority is added.
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(20 * time.Millisecond)
	close(release)
	if err := <-firstDone; err != nil {
		t.Fatalf("first task failed: %v", err)
	}
	wg.Wait()
	if want := []string{"high", "same", "low"}; !reflect.DeepEqual(order, want) {
		t.Fatalf("execution order = %v, want %v", order, want)
	}
}

func TestProviderQueueAgesBoundedWaitTaskAheadOfFreshPriority(t *testing.T) {
	now := time.Now().UTC()
	heap := TaskHeap{
		&QueueTask{Priority: 100, Sequence: 2, EnqueuedAt: now},
		&QueueTask{Priority: 70, Sequence: 1, EnqueuedAt: now.Add(-MaximumWait - time.Second)},
	}
	for index := range heap {
		heap[index].Index = index
	}
	selected := PopQueueTask(&heap, now)
	if selected == nil || selected.Priority != 70 {
		t.Fatalf("aged lifecycle task was starved by fresh priority: %#v", selected)
	}
}

func TestProviderQueueCancellationReleasesPendingTask(t *testing.T) {
	queue := NewProviderQueue(1)
	defer queue.Close()
	started := make(chan struct{})
	release := make(chan struct{})
	go func() {
		_ = queue.Submit(context.Background(), 1, func(context.Context) error {
			close(started)
			<-release
			return nil
		}, nil)
	}()
	<-started
	ctx, cancel := context.WithCancel(context.Background())
	states := make(chan string, 1)
	executed := make(chan struct{}, 1)
	go func() {
		_ = queue.Submit(ctx, 1, func(context.Context) error { executed <- struct{}{}; return nil }, func(status string, _ error) { states <- status })
	}()
	cancel()
	select {
	case <-executed:
		t.Fatal("canceled task was executed")
	case status := <-states:
		if status != RunCancelled {
			t.Fatalf("canceled task reported status %q", status)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("cancellation did not complete")
	}
	close(release)
}
