// Package decision implements the bounded System One protocol. It owns no
// domain rules, database transactions, Agent loops or fallback model calls.
package decision

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"sync"
	"time"
)

var Points = []string{"runtime.wakeup", "runtime.reflection", "goal.completion_check", "goal.replenish_plan", "tools.select", "persona.switch", "context.select"}

type Config struct {
	Enabled         bool            `json:"enabled"`
	Points          map[string]bool `json:"points"`
	Endpoint        string          `json:"endpoint"`
	TimeoutMS       int             `json:"timeout_ms"`
	BudgetMS        int             `json:"budget_ms"`
	Concurrency     int             `json:"concurrency"`
	BatchSize       int             `json:"batch_size"`
	ResponseBytes   int             `json:"response_bytes"`
	RequestBytes    int             `json:"request_bytes"`
	Strategy        string          `json:"strategy"`
	MinProbability  float64         `json:"min_probability"`
	MinMargin       float64         `json:"min_margin"`
	MaxDeferrals    int             `json:"max_deferrals"`
	DeferralSeconds int             `json:"deferral_seconds"`
	RetentionDays   int             `json:"retention_days"`
	Model           string          `json:"model"`
	ModelRevision   string          `json:"model_revision"`
}

func DefaultConfig() Config {
	points := make(map[string]bool, len(Points))
	for _, point := range Points {
		points[point] = true
	}
	return Config{Points: points, Endpoint: "http://127.0.0.1:8010/v1/systemone", TimeoutMS: 1500, BudgetMS: 3000, Concurrency: 1, BatchSize: 8, ResponseBytes: 1048576, RequestBytes: 262144, Strategy: "choice_argmax", MaxDeferrals: 3, DeferralSeconds: 60, RetentionDays: 7, Model: "Jakevin/kev-4b-ternary-mlx", ModelRevision: "unknown"}
}

func (c Config) Allows(point string) bool { return c.Enabled && c.Points[point] }
func (c Config) Validate() error {
	u, err := url.Parse(c.Endpoint)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("kev_endpoint_invalid")
	}
	if c.TimeoutMS < 50 || c.TimeoutMS > 60000 || c.BudgetMS < c.TimeoutMS || c.BudgetMS > 120000 || c.Concurrency < 1 || c.Concurrency > 16 || c.BatchSize < 1 || c.BatchSize > 8 || c.ResponseBytes < 1024 || c.ResponseBytes > 4194304 || c.RequestBytes < 1024 || c.RequestBytes > 1048576 || c.MaxDeferrals < 1 || c.MaxDeferrals > 100 || c.DeferralSeconds < 1 || c.DeferralSeconds > 86400 || c.RetentionDays < 1 || c.RetentionDays > 90 {
		return errors.New("kev_bounds_invalid")
	}
	if c.Strategy != "choice_argmax" && c.Strategy != "choice_guarded" {
		return errors.New("kev_strategy_invalid")
	}
	if !unit(c.MinProbability) || !unit(c.MinMargin) {
		return errors.New("kev_threshold_invalid")
	}
	if len(c.Points) != len(Points) {
		return errors.New("kev_points_invalid")
	}
	for _, p := range Points {
		if _, ok := c.Points[p]; !ok {
			return errors.New("kev_points_invalid")
		}
	}
	if len(c.Model) > 256 || len(c.ModelRevision) > 256 {
		return errors.New("kev_model_invalid")
	}
	return nil
}

type Question struct {
	Type         string            `json:"type"`
	Instructions string            `json:"instructions"`
	Criteria     map[string]string `json:"criteria"`
}

func Choice(instructions string) Question {
	return Question{"choice", instructions, map[string]string{"yes": "The answer is yes", "no": "The answer is no", "unclear": "The available information does not determine the answer"}}
}

type Request struct {
	State     string              `json:"state"`
	Questions map[string]Question `json:"questions"`
}
type Candidate struct {
	ID       string
	Question Question
}
type Scope struct {
	ActorSelf     string `json:"actor_self"`
	ActorUser     string `json:"actor_user,omitempty"`
	Agent         string `json:"agent,omitempty"`
	RunID         string `json:"run_id,omitempty"`
	EventID       string `json:"event_id,omitempty"`
	CorrelationID string `json:"correlation_id,omitempty"`
	StateVersion  string `json:"state_version,omitempty"`
}

type stageDeadlineKey struct{}
type originalRouteKey struct{}

func WithOriginal(ctx context.Context) context.Context {
	return context.WithValue(ctx, originalRouteKey{}, true)
}

// WithStageBudget shares a deadline across tool and context selection without
// cancelling the parent business request when the preparation budget expires.
func WithStageBudget(ctx context.Context, budget time.Duration) context.Context {
	if _, ok := ctx.Value(stageDeadlineKey{}).(time.Time); ok {
		return ctx
	}
	return context.WithValue(ctx, stageDeadlineKey{}, time.Now().Add(budget))
}

type Answer struct {
	Type          string             `json:"type"`
	Choice        string             `json:"choice"`
	Confidence    *float64           `json:"confidence"`
	Probabilities map[string]float64 `json:"probabilities"`
}
type Outcome string

const (
	Yes      Outcome = "yes"
	No       Outcome = "no"
	Original Outcome = "use_original"
)

type Record struct {
	ID                    string          `json:"id"`
	RequestID             string          `json:"request_id"`
	QuestionID            string          `json:"question_id"`
	Point                 string          `json:"decision_point"`
	CandidateID           string          `json:"candidate_id"`
	Scope                 Scope           `json:"scope"`
	ConfigVersion         int64           `json:"config_version"`
	SpecVersion           string          `json:"spec_version"`
	Strategy              string          `json:"strategy"`
	Model                 string          `json:"configured_model"`
	ModelRevision         string          `json:"model_revision"`
	Request               Request         `json:"request"`
	RawResponse           string          `json:"raw_response"`
	ReturnedModel         *string         `json:"returned_model"`
	Usage                 json.RawMessage `json:"usage"`
	ServiceLatency        json.RawMessage `json:"service_latency"`
	RawAnswer             json.RawMessage `json:"raw_answer,omitempty"`
	Answer                *Answer         `json:"answer,omitempty"`
	SelectedProbability   *float64        `json:"selected_probability"`
	Margin                *float64        `json:"margin"`
	HTTPStatus            int             `json:"http_status"`
	ResponseBytes         int             `json:"response_bytes"`
	ResponseLimitExceeded bool            `json:"response_limit_exceeded"`
	CallStatus            string          `json:"call_status"`
	Outcome               Outcome         `json:"policy_outcome"`
	ApplicationStatus     string          `json:"application_status"`
	Reason                string          `json:"fallback_reason,omitempty"`
	RuleReason            string          `json:"rule_reason,omitempty"`
	ActionID              string          `json:"action_id,omitempty"`
	StartedAt             time.Time       `json:"started_at"`
	CompletedAt           time.Time       `json:"completed_at"`
	RecordedAt            time.Time       `json:"recorded_at"`
	DurationMS            int64           `json:"duration_ms"`
	Retries               int             `json:"retries"`
}
type Settings interface {
	Read(context.Context) (Config, int64, string, error)
}
type Store interface {
	Save(context.Context, Record) error
}
type Service struct {
	Settings Settings
	Store    Store
	HTTP     *http.Client
	ID       func() string
	mu       sync.Mutex
	active   int
}
type Result struct {
	Record      Record
	config      Config
	service     *Service
	auditFailed bool
}

func unit(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) && v >= 0 && v <= 1 }
func ParseAnswer(raw json.RawMessage, config Config) (*Answer, Outcome, float64, float64, error) {
	var a Answer
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, Original, 0, 0, errors.New("invalid_answer")
	}
	if a.Type != "choice" || (a.Choice != "yes" && a.Choice != "no" && a.Choice != "unclear") || a.Confidence == nil || !unit(*a.Confidence) || len(a.Probabilities) != 3 {
		return nil, Original, 0, 0, errors.New("invalid_answer")
	}
	sum := 0.0
	values := []float64{}
	for _, key := range []string{"yes", "no", "unclear"} {
		v, ok := a.Probabilities[key]
		if !ok || !unit(v) {
			return nil, Original, 0, 0, errors.New("invalid_probabilities")
		}
		sum += v
		values = append(values, v)
	}
	sort.Float64s(values)
	p := a.Probabilities[a.Choice]
	margin := values[2] - values[1]
	if math.Abs(sum-1) > 0.005 || values[2]-p > 0.0005 {
		return nil, Original, 0, 0, errors.New("inconsistent_choice")
	}
	out := Outcome(a.Choice)
	if a.Choice == "unclear" || (config.Strategy == "choice_guarded" && (p < config.MinProbability || margin < config.MinMargin)) {
		out = Original
	}
	return &a, out, p, margin, nil
}

// Decide performs no candidate construction when disabled. Callers should
// likewise use Enabled before performing optional recall/semantic projection.
func (s *Service) Enabled(ctx context.Context, point string) bool {
	if original, _ := ctx.Value(originalRouteKey{}).(bool); original {
		return false
	}
	if s == nil || s.Settings == nil {
		return false
	}
	c, _, _, e := s.Settings.Read(ctx)
	return e == nil && c.Allows(point)
}
func (s *Service) Decide(ctx context.Context, point string, scope Scope, state string, candidates []Candidate) ([]Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if original, _ := ctx.Value(originalRouteKey{}).(bool); original {
		return originalResults(candidates, "forced_original"), nil
	}
	if s == nil || s.Settings == nil {
		return originalResults(candidates, "disabled"), nil
	}
	config, version, secret, err := s.Settings.Read(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err != nil || !config.Allows(point) {
		return originalResults(candidates, "disabled"), nil
	}
	if err = config.Validate(); err != nil {
		return originalResults(candidates, "config_invalid"), nil
	}
	budgetCtx, cancel := context.WithTimeout(ctx, time.Duration(config.BudgetMS)*time.Millisecond)
	defer cancel()
	if deadline, ok := ctx.Value(stageDeadlineKey{}).(time.Time); ok {
		shared, stop := context.WithDeadline(budgetCtx, deadline)
		defer stop()
		budgetCtx = shared
	}
	results := make([]Result, 0, len(candidates))
	for start := 0; start < len(candidates); start += config.BatchSize {
		end := min(start+config.BatchSize, len(candidates))
		batch := candidates[start:end]
		if e := ctx.Err(); e != nil {
			return results, e
		}
		rs, e := s.batch(budgetCtx, ctx, config, version, secret, point, scope, state, batch)
		results = append(results, rs...)
		if e != nil {
			return results, e
		}
	}
	return results, nil
}

func (s *Service) RecordOriginal(ctx context.Context, point string, scope Scope, candidate, reason string) error {
	if s == nil || s.Store == nil || s.ID == nil {
		return nil
	}
	c, version, _, err := s.Settings.Read(ctx)
	if err != nil {
		return err
	}
	at := time.Now().UTC()
	id := s.ID()
	return s.Store.Save(ctx, Record{ID: id, RequestID: id, QuestionID: "route", Point: point, CandidateID: candidate, Scope: scope, ConfigVersion: version, SpecVersion: "kev." + point + ".v1", Strategy: c.Strategy, Model: c.Model, ModelRevision: c.ModelRevision, Outcome: Original, CallStatus: "not_called", ApplicationStatus: "original_used", Reason: reason, StartedAt: at, CompletedAt: at, RecordedAt: at})
}
func originalResults(candidates []Candidate, reason string) []Result {
	out := make([]Result, len(candidates))
	for i, c := range candidates {
		out[i].Record = Record{CandidateID: c.ID, Outcome: Original, CallStatus: "not_called", ApplicationStatus: "original_used", Reason: reason}
	}
	return out
}
func (s *Service) acquire(ctx context.Context, limit int) error {
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()
	for {
		s.mu.Lock()
		if s.active < limit {
			s.active++
			s.mu.Unlock()
			return nil
		}
		s.mu.Unlock()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
func (s *Service) batch(budget, parent context.Context, c Config, version int64, secret, point string, scope Scope, state string, candidates []Candidate) ([]Result, error) {
	if s.ID == nil || s.Store == nil {
		return originalResults(candidates, "audit_unavailable"), nil
	}
	req := Request{State: state, Questions: map[string]Question{}}
	ids := make([]string, len(candidates))
	for i, v := range candidates {
		ids[i] = fmt.Sprintf("q_%04d", i+1)
		req.Questions[ids[i]] = v.Question
	}
	encoded, e := json.Marshal(req)
	if e != nil || len(encoded) > c.RequestBytes {
		return originalResults(candidates, "request_too_large"), nil
	}
	requestID := s.ID()
	start := time.Now().UTC()
	status := "ok"
	reason := ""
	httpStatus := 0
	responseBytes := 0
	var raw []byte
	if e = s.acquire(budget, c.Concurrency); e != nil {
		status = "timeout"
		reason = "budget_exhausted"
	} else {
		func() {
			defer func() { s.mu.Lock(); s.active--; s.mu.Unlock() }()
			child, stop := context.WithTimeout(budget, time.Duration(c.TimeoutMS)*time.Millisecond)
			defer stop()
			request, err := http.NewRequestWithContext(child, http.MethodPost, c.Endpoint, bytes.NewReader(encoded))
			if err != nil {
				status = "unavailable"
				reason = "request_invalid"
				return
			}
			request.Header.Set("Content-Type", "application/json")
			if secret != "" {
				request.Header.Set("Authorization", "Bearer "+secret)
			}
			client := s.HTTP
			if client == nil {
				client = &http.Client{}
			}
			response, err := client.Do(request)
			if err != nil {
				status = "unavailable"
				reason = "request_failed"
				if child.Err() != nil {
					status = "timeout"
					reason = "request_timeout"
				}
				return
			}
			defer response.Body.Close()
			httpStatus = response.StatusCode
			raw, err = io.ReadAll(io.LimitReader(response.Body, int64(c.ResponseBytes)+1))
			responseBytes = len(raw)
			if err != nil {
				status = "invalid_response"
				reason = "response_read_failed"
				return
			}
			if len(raw) > c.ResponseBytes {
				status = "invalid_response"
				reason = "response_too_large"
				raw = raw[:c.ResponseBytes]
				return
			}
			if httpStatus < 200 || httpStatus >= 300 {
				status = "unavailable"
				reason = "http_error"
			}
		}()
	}
	var envelope struct {
		Answers map[string]json.RawMessage `json:"answers"`
		Model   *string                    `json:"model"`
		Usage   json.RawMessage            `json:"usage"`
		Latency json.RawMessage            `json:"latency"`
	}
	if status == "ok" {
		if json.Unmarshal(raw, &envelope) != nil || envelope.Answers == nil {
			status = "invalid_response"
			reason = "invalid_envelope"
		}
		for id := range envelope.Answers {
			if _, ok := req.Questions[id]; !ok {
				status = "invalid_response"
				reason = "question_id_mismatch"
			}
		}
	}
	if parent.Err() != nil {
		status = "cancelled"
		reason = "request_cancelled"
	}
	end := time.Now().UTC()
	out := make([]Result, len(candidates))
	for i, candidate := range candidates {
		r := Record{ID: s.ID(), RequestID: requestID, QuestionID: ids[i], Point: point, CandidateID: candidate.ID, Scope: scope, ConfigVersion: version, SpecVersion: "kev." + point + ".v1", Strategy: c.Strategy, Model: c.Model, ModelRevision: c.ModelRevision, Request: req, RawResponse: string(raw), RawAnswer: envelope.Answers[ids[i]], HTTPStatus: httpStatus, ResponseBytes: len(raw), CallStatus: status, Outcome: Original, ApplicationStatus: "not_applied", Reason: reason, StartedAt: start, CompletedAt: end, RecordedAt: end, DurationMS: end.Sub(start).Milliseconds()}
		r.ReturnedModel = envelope.Model
		r.Usage = envelope.Usage
		r.ServiceLatency = envelope.Latency
		r.ResponseBytes = responseBytes
		r.ResponseLimitExceeded = reason == "response_too_large"
		if status == "ok" {
			a, outcome, p, margin, err := ParseAnswer(r.RawAnswer, c)
			if err != nil {
				r.CallStatus = "invalid_response"
				r.Reason = err.Error()
			} else {
				r.Answer = a
				r.Outcome = outcome
				r.SelectedProbability = &p
				r.Margin = &margin
				if outcome == Original {
					r.Reason = "abstained_or_guarded"
				} else {
					r.RuleReason = "model_selected_" + string(outcome)
				}
			}
		}
		persistCtx, stop := context.WithTimeout(context.WithoutCancel(parent), 2*time.Second)
		err := s.Store.Save(persistCtx, r)
		stop()
		if err != nil {
			r.Reason = "audit_unavailable"
		}
		out[i] = Result{Record: r, config: c, service: s, auditFailed: err != nil}
	}
	if parent.Err() != nil {
		return out, parent.Err()
	}
	return out, nil
}

// Admit must be called immediately before application, including results which
// waited for another candidate. The raw result is retained after revocation.
func (r *Result) Admit(ctx context.Context) (Outcome, error) {
	if e := ctx.Err(); e != nil {
		r.Finish(ctx, "cancelled", "request_cancelled", "")
		return Original, e
	}
	if r.service == nil || r.auditFailed || r.Record.Outcome == Original {
		return Original, nil
	}
	c, v, _, e := r.service.Settings.Read(ctx)
	if err := ctx.Err(); err != nil {
		_ = r.Finish(ctx, "cancelled", "request_cancelled", "")
		return Original, err
	}
	if e != nil || v != r.Record.ConfigVersion || !c.Allows(r.Record.Point) {
		reason := "config_changed"
		if e == nil && !c.Allows(r.Record.Point) {
			reason = "discarded_after_disable"
		}
		r.Finish(ctx, "original_used", reason, "")
		return Original, nil
	}
	return r.Record.Outcome, nil
}
func (r *Result) Finish(ctx context.Context, status, reason, actionID string) error {
	r.Record.ApplicationStatus = status
	r.Record.RecordedAt = time.Now().UTC()
	r.Record.ActionID = actionID
	if reason != "" {
		r.Record.Reason = reason
	}
	if r.service == nil {
		return nil
	}
	cancelCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	return r.service.Store.Save(cancelCtx, r.Record)
}
