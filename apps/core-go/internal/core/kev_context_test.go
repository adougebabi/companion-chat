package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/ai/decision"
)

type kevContextTestTransport func(*http.Request) (*http.Response, error)

func (f kevContextTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type kevContextTestStore struct {
	mu      sync.Mutex
	records []decision.Record
	delay   bool
}

func (s *kevContextTestStore) Save(ctx context.Context, r decision.Record) error {
	if s.delay {
		<-ctx.Done()
		return ctx.Err()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, r)
	return nil
}
func kevContextTestApp(transport http.RoundTripper, store *kevContextTestStore) *App {
	c := decision.DefaultConfig()
	c.Enabled = true
	c.TimeoutMS = 60000
	c.BudgetMS = 60000
	var ids atomic.Int64
	return &App{Kev: &decision.Service{Settings: &kevFixedSettings{c}, Store: store, HTTP: &http.Client{Transport: transport}, ID: func() string { return fmt.Sprint(ids.Add(1)) }}}
}
func kevContextTestInput() WorkingMemoryInput {
	return WorkingMemoryInput{
		RuntimeFacts:      []PromptFragment{{Kind: PromptFragmentRuntimeFact, Required: true, Content: map[string]any{"current": "must survive"}}},
		ActiveCandidates:  []PromptFragment{{Kind: PromptFragmentActiveMemory, Content: map[string]any{"memory": "active"}}},
		RetrievedMemories: []PromptFragment{{Kind: PromptFragmentRetrievedMemory, Content: "remembered fact"}},
		RecentMessages:    []PromptFragment{{Kind: PromptFragmentRecentMessage, Content: "complete native history"}},
	}
}
func TestKevContextSelectionGivesRequestOneMinute(t *testing.T) {
	var remaining time.Duration
	transport := kevContextTestTransport(func(r *http.Request) (*http.Response, error) {
		deadline, _ := r.Context().Deadline()
		remaining = time.Until(deadline)
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"answers":{"q_0001":{"type":"choice","choice":"yes","confidence":0.8,"probabilities":{"yes":0.8,"no":0.1,"unclear":0.1}}}}`))}, nil
	})
	app := kevContextTestApp(transport, &kevContextTestStore{})
	input := WorkingMemoryInput{ActiveCandidates: []PromptFragment{{Kind: PromptFragmentActiveMemory, Content: "memory"}}}
	selection := app.startKevContextSelection(context.Background(), ContextProjection{}, ProviderContextSurfaceConversationMain, "input", input)
	defer selection.cancel()
	if _, err := selection.wait(context.Background()); err != nil {
		t.Fatal(err)
	}
	if remaining < 59*time.Second || remaining > 60*time.Second {
		t.Fatalf("context selection cut the Kev request short: %s", remaining)
	}
}
func TestKevContextBudgetExpiryRestoresWholeInput(t *testing.T) {
	const testWait = 50 * time.Millisecond
	var calls atomic.Int32
	var deadline time.Duration
	transport := kevContextTestTransport(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		d, _ := r.Context().Deadline()
		deadline = time.Until(d)
		// Model the gateway's pending response; cancellation must arrive before its 30s 504.
		select {
		case <-r.Context().Done():
			return nil, r.Context().Err()
		case <-time.After(30 * time.Second):
			return nil, errors.New("gateway 504")
		}
	})
	store := &kevContextTestStore{}
	app := kevContextTestApp(transport, store)
	input := kevContextTestInput()
	started := time.Now()
	selection := app.startKevContextSelectionWithWait(context.Background(), ContextProjection{}, ProviderContextSurfaceConversationMain, "current input", input, testWait)
	defer selection.cancel()
	selected, err := selection.wait(context.Background())
	if err != nil || !reflect.DeepEqual(selected, input) {
		t.Fatalf("best effort did not preserve full context: %v %+v", err, selected)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("reply waited for KEV: %s", elapsed)
	}
	select {
	case <-selection.finished:
	case <-time.After(time.Second):
		t.Fatal("cancelled selection worker did not exit")
	}
	if deadline <= 0 || deadline > testWait {
		t.Fatalf("selection did not use the stage deadline: %s", deadline)
	}
	if calls.Load() != 1 {
		t.Fatalf("selection continued later sections after expiry: %d", calls.Load())
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.records) == 0 {
		t.Fatal("timeout audit missing")
	}
	for _, r := range store.records {
		if r.ApplicationStatus == "applied" {
			t.Fatal("late result claimed application", r)
		}
	}
}
func TestKevContextSlowAuditCannotBlockReply(t *testing.T) {
	transport := kevContextTestTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"answers":{"q_0001":{"type":"choice","choice":"no","confidence":0.8,"probabilities":{"yes":0.1,"no":0.8,"unclear":0.1}}}}`))}, nil
	})
	app := kevContextTestApp(transport, &kevContextTestStore{delay: true})
	input := kevContextTestInput()
	started := time.Now()
	selection := app.startKevContextSelectionWithWait(context.Background(), ContextProjection{}, ProviderContextSurfaceConversationMain, "input", input, 50*time.Millisecond)
	defer selection.cancel()
	selected, err := selection.wait(context.Background())
	if err != nil || !reflect.DeepEqual(selected, input) {
		t.Fatalf("slow audit changed context: %v", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("audit blocked reply: %s", elapsed)
	}
}
func TestKevContextPartialSelectionIsDiscardedAndParentCancellationPropagates(t *testing.T) {
	for _, cancelParent := range []bool{false, true} {
		t.Run(fmt.Sprint(cancelParent), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var calls atomic.Int32
			transport := kevContextTestTransport(func(r *http.Request) (*http.Response, error) {
				if calls.Add(1) > 1 {
					if cancelParent {
						cancel()
					}
					<-r.Context().Done()
					return nil, r.Context().Err()
				}
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"answers":{"q_0001":{"type":"choice","choice":"no","confidence":0.8,"probabilities":{"yes":0.1,"no":0.8,"unclear":0.1}}}}`))}, nil
			})
			store := &kevContextTestStore{}
			app := kevContextTestApp(transport, store)
			input := kevContextTestInput()
			selection := app.startKevContextSelectionWithWait(ctx, ContextProjection{}, ProviderContextSurfaceConversationMain, "input", input, 50*time.Millisecond)
			defer selection.cancel()
			selected, err := selection.wait(ctx)
			if cancelParent && !errors.Is(err, context.Canceled) {
				t.Fatal("parent cancellation became fallback", err)
			}
			if !cancelParent && err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(selected, input) {
				t.Fatal("partial selection leaked into reply")
			}
			select {
			case <-selection.finished:
			case <-time.After(time.Second):
				t.Fatal("worker did not settle")
			}
			store.mu.Lock()
			defer store.mu.Unlock()
			for _, r := range store.records {
				if r.ApplicationStatus == "applied" {
					t.Fatal("discarded partial result marked applied")
				}
			}
		})
	}
}
func TestKevContextAsyncSnapshotPreservesWireDataAndLiveInput(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	var captured decision.Request
	transport := kevContextTestTransport(func(r *http.Request) (*http.Response, error) {
		close(entered)
		<-release
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			return nil, err
		}
		return &http.Response{StatusCode: 504, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("gateway timeout"))}, nil
	})
	app := kevContextTestApp(transport, &kevContextTestStore{})
	input := WorkingMemoryInput{ActiveCandidates: []PromptFragment{{Kind: PromptFragmentActiveMemory, Content: map[string]any{"fact": "original fact"}}}}
	selection := app.startKevContextSelection(context.Background(), ContextProjection{}, ProviderContextSurfaceConversationMain, "CURRENT_INPUT_ONCE", input)
	defer selection.cancel()
	<-entered
	input.ActiveCandidates[0].Content.(map[string]any)["fact"] = "new live fact"
	close(release)
	selected, err := selection.wait(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(captured.State, "CURRENT_INPUT_ONCE") != 1 || !strings.Contains(captured.State, "original fact") {
		t.Fatalf("wire input changed: %s", captured.State)
	}
	if selected.ActiveCandidates[0].Content.(map[string]any)["fact"] != "original fact" || input.ActiveCandidates[0].Content.(map[string]any)["fact"] != "new live fact" {
		t.Fatal("late worker mutated live input")
	}
}
