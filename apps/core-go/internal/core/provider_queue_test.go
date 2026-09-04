package core

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestProviderQueueHonorsPriorityAndFIFO(t *testing.T) {
	queue := newProviderQueue(1)
	defer queue.close()
	// Occupy the only slot so all following tasks are observed in the heap.
	started := make(chan struct{})
	release := make(chan struct{})
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- queue.submit(context.Background(), 1, func(context.Context) error {
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
			_ = queue.submit(context.Background(), task.priority, func(context.Context) error {
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

func TestProviderQueueCancellationReleasesPendingTask(t *testing.T) {
	queue := newProviderQueue(1)
	defer queue.close()
	started := make(chan struct{})
	release := make(chan struct{})
	go func() {
		_ = queue.submit(context.Background(), 1, func(context.Context) error {
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
		_ = queue.submit(ctx, 1, func(context.Context) error { executed <- struct{}{}; return nil }, func(status string, _ error) { states <- status })
	}()
	cancel()
	select {
	case state := <-states:
		if state != providerRunCancelled {
			t.Fatalf("cancel status = %q", state)
		}
	case <-time.After(time.Second):
		t.Fatal("pending cancellation was not observed")
	}
	select {
	case <-executed:
		t.Fatal("cancelled task executed")
	default:
	}
	close(release)
}

func TestProviderQueueHonorsConfiguredConcurrency(t *testing.T) {
	queue := newProviderQueue(2)
	defer queue.close()
	var mu sync.Mutex
	running, peak := 0, 0
	start := make(chan struct{})
	var wg sync.WaitGroup
	for index := 0; index < 4; index++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = queue.submit(context.Background(), 1, func(context.Context) error {
				mu.Lock()
				running++
				if running > peak {
					peak = running
				}
				mu.Unlock()
				<-start
				mu.Lock()
				running--
				mu.Unlock()
				return nil
			}, nil)
		}()
	}
	deadline := time.After(time.Second)
	for {
		mu.Lock()
		currentPeak := peak
		mu.Unlock()
		if currentPeak == 2 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("queue did not start two concurrent tasks; peak=%d", currentPeak)
		default:
			time.Sleep(time.Millisecond)
		}
	}
	close(start)
	wg.Wait()
	mu.Lock()
	defer mu.Unlock()
	if peak > 2 {
		t.Fatalf("peak concurrency = %d, want <= 2", peak)
	}
}

func TestProviderRunStatusForError(t *testing.T) {
	for _, test := range []struct {
		err  error
		want string
	}{
		{err: nil, want: providerRunCompleted},
		{err: context.Canceled, want: providerRunCancelled},
		{err: context.DeadlineExceeded, want: providerRunTimeout},
		{err: errProviderPaused, want: providerRunCancelled},
		{err: errProviderInactive, want: providerRunCancelled},
		{err: errors.New("boom"), want: providerRunFailed},
	} {
		if got := providerRunStatusForError(test.err); got != test.want {
			t.Errorf("providerRunStatusForError(%v) = %q, want %q", test.err, got, test.want)
		}
	}
}

func TestProviderSuppressionErrorCodesAreBounded(t *testing.T) {
	if got := providerRunErrorCode(errProviderPaused); got != "fluctlight_paused" {
		t.Fatalf("paused error code = %q", got)
	}
	if got := providerRunErrorCode(errProviderInactive); got != "fluctlight_inactive" {
		t.Fatalf("inactive error code = %q", got)
	}
}

func TestProviderSuppressionStatusDistinguishesPausedAndInactive(t *testing.T) {
	if status, ok := providerSuppressionStatus(errProviderPaused); !ok || status != "paused" {
		t.Fatalf("paused suppression = %q, %t", status, ok)
	}
	if status, ok := providerSuppressionStatus(errProviderInactive); !ok || status != "inactive" {
		t.Fatalf("inactive suppression = %q, %t", status, ok)
	}
	if status, ok := providerSuppressionStatus(errors.New("provider failed")); ok || status != "" {
		t.Fatalf("ordinary error suppression = %q, %t", status, ok)
	}
}

func TestProviderScenarioAndPriorityMapping(t *testing.T) {
	tests := []struct {
		role, schema, wantScenario string
		wantPriority               int
	}{
		{role: "action_realization", wantScenario: "reply", wantPriority: 100},
		{role: "cognitive_assessment", wantScenario: "cognitive_assessment", wantPriority: 90},
		{role: "cognitive_assessment", schema: "daily_review_response", wantScenario: "daily_review", wantPriority: 90},
		{role: "cognitive_assessment", schema: "wake_up_response", wantScenario: "wake_up", wantPriority: 70},
		{role: "reflection", wantScenario: "reflection", wantPriority: 70},
		{role: "initialization", wantScenario: "initialization", wantPriority: 60},
		{role: "media_prompt", wantScenario: "media_prompt", wantPriority: 80},
		{role: "media_prompt", schema: "media_quality_acceptance_response", wantScenario: "media_quality_acceptance", wantPriority: 80},
		{role: "embedding", wantScenario: "embedding", wantPriority: 50},
	}
	for _, testCase := range tests {
		if got := providerScenario(context.Background(), testCase.role, testCase.schema); got != testCase.wantScenario {
			t.Errorf("providerScenario(%q,%q) = %q, want %q", testCase.role, testCase.schema, got, testCase.wantScenario)
		}
		if got := providerPriority(testCase.wantScenario); got != testCase.wantPriority {
			t.Errorf("providerPriority(%q) = %d, want %d", testCase.wantScenario, got, testCase.wantPriority)
		}
	}
}

func TestProviderBindingRoleOnlyExposesTwoTargets(t *testing.T) {
	for _, role := range []string{"generic_llm", "embedding"} {
		if got := providerBindingRole(role); got != role {
			t.Fatalf("providerBindingRole(%q) = %q", role, got)
		}
	}
	for _, role := range []string{"action_realization", "cognitive_assessment", "reflection", "media_prompt", "initialization"} {
		if got := providerBindingRole(role); got != "generic_llm" {
			t.Fatalf("providerBindingRole(%q) = %q, want generic_llm", role, got)
		}
	}
}
