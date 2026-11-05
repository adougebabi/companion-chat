package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/ai/decision"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type wakeGateRow struct {
	count int
	until time.Time
	err   error
}

func (r wakeGateRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	*dest[0].(*int), *dest[1].(*time.Time) = r.count, r.until
	return nil
}

type wakeGateStore struct {
	row       wakeGateRow
	writeErr  error
	writes    int
	lastQuery string
	lastArgs  []any
}

type wakeGateUnavailableSettings struct{}

func (wakeGateUnavailableSettings) Read(context.Context) (decision.Config, int64, string, error) {
	return decision.Config{}, 0, "", errors.New("controlled Kev settings failure")
}

func TestManualWakeUpIgnoresUnavailableKevSettings(t *testing.T) {
	service := &decision.Service{Settings: wakeGateUnavailableSettings{}}
	if enabled, err := wakeUpKevEnabled(context.Background(), service, true); enabled || err != nil {
		t.Fatalf("manual wake should bypass Kev settings: enabled=%v err=%v", enabled, err)
	}
	if enabled, err := wakeUpKevEnabled(context.Background(), service, false); enabled || err == nil {
		t.Fatalf("automatic wake must report unavailable Kev settings: enabled=%v err=%v", enabled, err)
	}
}

func (s *wakeGateStore) QueryRow(context.Context, string, ...any) pgx.Row { return s.row }
func (s *wakeGateStore) Exec(_ context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	s.writes++
	s.lastQuery, s.lastArgs = query, args
	return pgconn.NewCommandTag("INSERT 0 1"), s.writeErr
}

func TestKevAutomaticWakeGateWaitsForAnAdmissibleDecision(t *testing.T) {
	for _, tc := range []struct {
		name, point, choice     string
		count                   int
		until                   time.Time
		disabled, manual, fail  bool
		writeFails, wantErr     bool
		wantAllowed, wantCalled bool
	}{
		{name: "yes", point: "runtime.wakeup", choice: "yes", wantAllowed: true, wantCalled: true},
		{name: "no", point: "runtime.wakeup", choice: "no", wantCalled: true},
		{name: "still asks after previous limit", point: "runtime.wakeup", choice: "no", count: 3, until: time.Now().Add(-time.Minute), wantCalled: true},
		{name: "unclear", point: "runtime.wakeup", choice: "unclear", wantCalled: true},
		{name: "service unavailable", point: "runtime.wakeup", fail: true, wantCalled: true},
		{name: "deferral persistence failed", point: "runtime.wakeup", choice: "no", writeFails: true, wantErr: true, wantCalled: true},
		{name: "not yet due", point: "runtime.wakeup", count: 1, until: time.Now().Add(time.Minute)},
		{name: "disabled", point: "runtime.wakeup", disabled: true, wantAllowed: true},
		{name: "manual override", point: "runtime.wakeup", manual: true, wantAllowed: true},
		{name: "other gate keeps its limit", point: "runtime.reflection", count: 3, until: time.Now().Add(-time.Minute), wantAllowed: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := decision.DefaultConfig()
			config.Enabled = !tc.disabled
			calls, ids := 0, 0
			audit := &kevMemoryStore{}
			transport := kevContextTestTransport(func(request *http.Request) (*http.Response, error) {
				calls++
				var packet decision.Request
				if err := json.NewDecoder(request.Body).Decode(&packet); err != nil {
					t.Fatal(err)
				}
				if tc.point == "runtime.wakeup" && !strings.Contains(packet.Questions["q_0001"].Instructions, "unclear means wait") {
					t.Errorf("wake question did not preserve the decision boundary: %+v", packet.Questions)
				}
				if tc.fail {
					return nil, errors.New("controlled Kev failure")
				}
				probabilities := map[string]float64{"yes": 0.1, "no": 0.1, "unclear": 0.1}
				probabilities[tc.choice] = 0.8
				response := jsonString(map[string]any{"answers": map[string]any{"q_0001": map[string]any{"type": "choice", "choice": tc.choice, "confidence": 0.8, "probabilities": probabilities}}})
				return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(response))}, nil
			})
			app := &App{Kev: &decision.Service{Settings: &kevFixedSettings{config}, Store: audit, HTTP: &http.Client{Transport: transport}, ID: func() string { ids++; return fmt.Sprint(ids) }}}
			row := wakeGateRow{count: tc.count, until: tc.until}
			if tc.count == 0 {
				row.err = pgx.ErrNoRows
			}
			store := &wakeGateStore{row: row}
			if tc.writeFails {
				store.writeErr = errors.New("controlled deferral write failure")
			}
			ctx := context.Background()
			if tc.manual {
				ctx = decision.WithOriginal(ctx)
			}
			allowed, until, err := app.kevAutomaticGateWithStore(ctx, tc.point, "character", "cycle", map[string]any{"cycle": 0}, store)
			if allowed != tc.wantAllowed || (err != nil) != tc.wantErr || (calls != 0) != tc.wantCalled {
				t.Fatalf("allowed=%v until=%v calls=%d err=%v", allowed, until, calls, err)
			}
			if !allowed && err == nil && until.IsZero() {
				t.Fatal("deferred wake has no retry time")
			}
			if tc.name == "still asks after previous limit" && (store.writes != 1 || len(store.lastArgs) != 5 || store.lastArgs[4] != config.MaxDeferrals) {
				t.Fatalf("wake deferral count is not bounded: %+v", store)
			}
			if (tc.name == "unclear" || tc.name == "service unavailable") && (len(audit.records) == 0 || audit.records[len(audit.records)-1].ApplicationStatus != "not_applied") {
				t.Fatalf("unavailable decision should not be recorded as an applied yes: %+v", audit.records)
			}
		})
	}
}
