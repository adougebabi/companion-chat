package decision

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type testSettings struct {
	mu      sync.Mutex
	config  Config
	version int64
}

func (s *testSettings) Read(context.Context) (Config, int64, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.config, s.version, "test-secret", nil
}

type testStore struct {
	mu      sync.Mutex
	records []Record
	fail    bool
}

type cancellingSettings struct {
	cancel context.CancelFunc
	config Config
}

func (s cancellingSettings) Read(context.Context) (Config, int64, string, error) {
	s.cancel()
	return s.config, 1, "", context.Canceled
}
func TestKevCancellationDuringSettingsReadIsNotFallback(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	c := DefaultConfig()
	c.Enabled = true
	s := &Service{Settings: cancellingSettings{cancel, c}}
	_, err := s.Decide(ctx, "tools.select", Scope{}, "", []Candidate{{"a", Choice("question")}})
	if !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled settings read became original", err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	store := &testStore{}
	s.Settings = cancellingSettings{cancel, c}
	s.Store = store
	r := Result{Record: Record{Outcome: Yes, Point: "tools.select", ConfigVersion: 1}, service: s}
	_, err = r.Admit(ctx)
	if !errors.Is(err, context.Canceled) || r.Record.ApplicationStatus != "cancelled" || r.Record.Reason != "request_cancelled" {
		t.Fatal(r.Record, err)
	}
}

func (s *testStore) Save(_ context.Context, r Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fail {
		return errors.New("disk_unavailable")
	}
	s.records = append(s.records, r)
	return nil
}
func validAnswer(choice string) map[string]any {
	p := map[string]float64{"yes": 0.2, "no": 0.2, "unclear": 0.2}
	p[choice] = 0.6
	return map[string]any{"type": "choice", "choice": choice, "confidence": 0.4, "probabilities": p}
}
func testService(t *testing.T, handler http.HandlerFunc) (*Service, *testSettings, *testStore) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	c := DefaultConfig()
	c.Enabled = true
	c.Endpoint = server.URL
	c.TimeoutMS = 100
	c.BudgetMS = 300
	settings := &testSettings{config: c, version: 1}
	store := &testStore{}
	var ids atomic.Uint64
	return &Service{Settings: settings, Store: store, HTTP: server.Client(), ID: func() string { return fmt.Sprint(ids.Add(1)) }}, settings, store
}
func answerHandler(choice string, calls *atomic.Int32) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var request Request
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			w.WriteHeader(400)
			return
		}
		answers := map[string]any{}
		for id := range request.Questions {
			answers[id] = validAnswer(choice)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"answers": answers, "model": "test", "metadata": map[string]any{"unknown": true}})
	}
}
func TestKevSevenPointsChoicesAndDisabled(t *testing.T) {
	for _, point := range Points {
		for _, choice := range []string{"yes", "no", "unclear"} {
			t.Run(point+"/"+choice, func(t *testing.T) {
				var calls atomic.Int32
				s, _, store := testService(t, answerHandler(choice, &calls))
				rs, err := s.Decide(context.Background(), point, Scope{ActorSelf: "actor"}, "bounded-state", []Candidate{{"item", Choice("question")}})
				if err != nil {
					t.Fatal(err)
				}
				out, err := rs[0].Admit(context.Background())
				want := Outcome(choice)
				if choice == "unclear" {
					want = Original
				}
				if err != nil || out != want || calls.Load() != 1 || len(store.records) != 1 || !strings.Contains(store.records[0].RawResponse, "metadata") {
					t.Fatalf("out=%s err=%v calls=%d record=%+v", out, err, calls.Load(), store.records)
				}
			})
		}
		t.Run(point+"/disabled", func(t *testing.T) {
			var calls atomic.Int32
			s, cfg, _ := testService(t, answerHandler("yes", &calls))
			cfg.config.Points[point] = false
			rs, err := s.Decide(context.Background(), point, Scope{}, "", []Candidate{{"item", Choice("question")}})
			if err != nil || rs[0].Record.Outcome != Original || calls.Load() != 0 {
				t.Fatalf("disabled called HTTP: %v", err)
			}
		})
	}
}
func TestKevGlobalDisableAndParentCancellationNeverCall(t *testing.T) {
	var calls atomic.Int32
	s, cfg, _ := testService(t, answerHandler("yes", &calls))
	cfg.config.Enabled = false
	_, err := s.Decide(context.Background(), "tools.select", Scope{}, "", []Candidate{{"item", Choice("question")}})
	if err != nil || calls.Load() != 0 {
		t.Fatal(err)
	}
	cfg.config.Enabled = true
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = s.Decide(ctx, "tools.select", Scope{}, "", []Candidate{{"item", Choice("question")}})
	if !errors.Is(err, context.Canceled) || calls.Load() != 0 {
		t.Fatal("parent cancellation started a call")
	}
}
func TestKevRawChoiceSurvivesDisableBeforeApplication(t *testing.T) {
	var calls atomic.Int32
	s, cfg, store := testService(t, answerHandler("yes", &calls))
	rs, err := s.Decide(context.Background(), "tools.select", Scope{}, "", []Candidate{{"item", Choice("question")}})
	if err != nil {
		t.Fatal(err)
	}
	cfg.config.Enabled = false
	cfg.version++
	out, err := rs[0].Admit(context.Background())
	if err != nil || out != Original {
		t.Fatal(out, err)
	}
	last := store.records[len(store.records)-1]
	if last.Answer.Choice != "yes" || last.Outcome != Yes || last.Reason != "discarded_after_disable" || last.ApplicationStatus != "original_used" {
		t.Fatalf("raw/model outcome lost: %+v", last)
	}
}
func TestKevPersistenceFailureCannotAdopt(t *testing.T) {
	var calls atomic.Int32
	s, _, store := testService(t, answerHandler("yes", &calls))
	store.fail = true
	rs, err := s.Decide(context.Background(), "context.select", Scope{}, "", []Candidate{{"item", Choice("question")}})
	if err != nil {
		t.Fatal(err)
	}
	out, err := rs[0].Admit(context.Background())
	if err != nil || out != Original || rs[0].Record.Reason != "audit_unavailable" {
		t.Fatal(out, err)
	}
}
func TestKevPartialQuestionAndMalformedResponses(t *testing.T) {
	for _, body := range []string{`{}`, `null`, `{"answers":[]}`, `{"answers":{"q_0001":{"type":"choice","choice":"maybe"}}}`, `{"answers":{"q_0001":{"type":"choice","choice":"yes","confidence":0.7,"probabilities":{"yes":-1,"no":1,"unclear":1}}}}`} {
		t.Run(body, func(t *testing.T) {
			s, _, _ := testService(t, func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, body) })
			rs, err := s.Decide(context.Background(), "context.select", Scope{}, "", []Candidate{{"item", Choice("question")}})
			if err != nil || rs[0].Record.Outcome != Original || rs[0].Record.CallStatus != "invalid_response" {
				t.Fatal(rs, err)
			}
		})
	}
	s, _, _ := testService(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"answers": map[string]any{"q_0001": validAnswer("no")}})
	})
	rs, err := s.Decide(context.Background(), "tools.select", Scope{}, "", []Candidate{{"a", Choice("question")}, {"b", Choice("question")}})
	if err != nil || rs[0].Record.Outcome != No || rs[1].Record.Outcome != Original || rs[0].Record.RequestID != rs[1].Record.RequestID {
		t.Fatal(rs, err)
	}
}
func TestKevChoiceArgmaxAndGuardedAreDistinct(t *testing.T) {
	raw := json.RawMessage(`{"type":"choice","choice":"yes","confidence":0.1,"probabilities":{"yes":0.4,"no":0.35,"unclear":0.25}}`)
	c := DefaultConfig()
	a, out, p, _, err := ParseAnswer(raw, c)
	if err != nil || out != Yes || p != 0.4 || *a.Confidence != 0.1 {
		t.Fatal(out, err)
	}
	c.Strategy = "choice_guarded"
	c.MinProbability = 0.8
	_, out, _, _, err = ParseAnswer(raw, c)
	if err != nil || out != Original {
		t.Fatal(out, err)
	}
}
func TestKevTimeoutAndSharedStageBudget(t *testing.T) {
	s, cfg, _ := testService(t, func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(200 * time.Millisecond):
		}
		fmt.Fprint(w, `{}`)
	})
	cfg.config.TimeoutMS = 50
	cfg.config.BudgetMS = 100
	ctx := WithStageBudget(context.Background(), 20*time.Millisecond)
	rs, err := s.Decide(ctx, "tools.select", Scope{}, "", []Candidate{{"a", Choice("question")}})
	if err != nil || rs[0].Record.Outcome != Original || rs[0].Record.CallStatus != "timeout" {
		t.Fatal(rs, err)
	}
	if ctx.Err() != nil {
		t.Fatal("stage budget cancelled business request")
	}
}
func TestKevRequestAndResponseLimits(t *testing.T) {
	s, cfg, _ := testService(t, func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, strings.Repeat("x", 2000)) })
	cfg.config.ResponseBytes = 1024
	rs, err := s.Decide(context.Background(), "tools.select", Scope{}, "", []Candidate{{"a", Choice("question")}})
	if err != nil || rs[0].Record.Reason != "response_too_large" {
		t.Fatal(rs, err)
	}
	cfg.config.RequestBytes = 1024
	rs, err = s.Decide(context.Background(), "tools.select", Scope{}, strings.Repeat("x", 2000), []Candidate{{"a", Choice("question")}})
	if err != nil || rs[0].Record.Outcome != Original || rs[0].Record.Reason != "request_too_large" {
		t.Fatal(rs, err)
	}
}
