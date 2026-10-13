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

var Points = []string{"runtime.wakeup", "runtime.reflection", "goal.completion_check", "goal.replenish_plan", "tools.select", "persona.switch", "context.select", "media.subject_route", "media.capture_plan", "media.view_plan"}

type Config struct {
	MediaEnabled    bool            `json:"media_enabled"`
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
	return Config{MediaEnabled: true, Points: points, Endpoint: "http://127.0.0.1:8010/v1/systemone", TimeoutMS: 1500, BudgetMS: 3000, Concurrency: 1, BatchSize: 8, ResponseBytes: 1048576, RequestBytes: 262144, Strategy: "choice_argmax", MaxDeferrals: 3, DeferralSeconds: 60, RetentionDays: 7, Model: "Jakevin/kev-4b-ternary-mlx", ModelRevision: "unknown"}
}

func (c Config) Allows(point string) bool {
	if len(point) > 6 && point[:6] == "media." && !c.MediaEnabled {
		return false
	}
	enabled, present := c.Points[point]
	if !present && (point == "media.subject_route" || point == "media.capture_plan" || point == "media.view_plan") {
		enabled = true
	}
	return c.Enabled && enabled
}
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
	if len(c.Points) < 7 || len(c.Points) > len(Points) {
		return errors.New("kev_points_invalid")
	}
	for _, p := range Points[:7] {
		if _, ok := c.Points[p]; !ok {
			return errors.New("kev_points_invalid")
		}
	}
	for p := range c.Points {
		known := false
		for _, valid := range Points {
			if p == valid {
				known = true
			}
		}
		if !known {
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
	Point    string // Optional per-question point for one mixed media batch.
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

// WithFreshStageBudget starts a new explicit selection stage, e.g. a later
// directory request after prompt assembly. The parent's deadline still applies.
func WithFreshStageBudget(ctx context.Context, budget time.Duration) context.Context {
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
	return ParseChoiceAnswer(raw, config, Choice("").Criteria)
}

// ParseChoiceAnswer validates against the exact question options, preserving the binary gates.
func ParseChoiceAnswer(raw json.RawMessage, config Config, criteria map[string]string) (*Answer, Outcome, float64, float64, error) {
	var a Answer
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, Original, 0, 0, errors.New("invalid_answer")
	}
	if a.Type != "choice" || (criteria[a.Choice] == "") || a.Confidence == nil || !unit(*a.Confidence) || len(criteria) < 2 || len(a.Probabilities) != len(criteria) {
		return nil, Original, 0, 0, errors.New("invalid_answer")
	}
	sum := 0.0
	values := []float64{}
	for key := range criteria {
		v, ok := a.Probabilities[key]
		if !ok || !unit(v) {
			return nil, Original, 0, 0, errors.New("invalid_probabilities")
		}
		sum += v
		values = append(values, v)
	}
	sort.Float64s(values)
	p := a.Probabilities[a.Choice]
	margin := values[len(values)-1] - values[len(values)-2]
	if math.Abs(sum-1) > 0.005 || values[len(values)-1]-p > 0.0005 {
		return nil, Original, 0, 0, errors.New("inconsistent_choice")
	}
	out := Outcome(a.Choice)
	if a.Choice == "unclear" || a.Choice == "unknown" || (config.Strategy == "choice_guarded" && (p < config.MinProbability || margin < config.MinMargin)) {
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
	return s.decide(ctx, point, scope, candidates, func([]Candidate) string { return state })
}

// DecideWithBatchState lets a typed caller project only the state owned by the
// candidates in each physical request. Decide remains the generic protocol
// entry and sends its state unchanged for every batch.
func (s *Service) DecideWithBatchState(ctx context.Context, point string, scope Scope, candidates []Candidate, stateForBatch func([]Candidate) string) ([]Result, error) {
	if stateForBatch == nil {
		return nil, errors.New("kev_batch_state_builder_missing")
	}
	return s.decide(ctx, point, scope, candidates, stateForBatch)
}

func (s *Service) decide(ctx context.Context, point string, scope Scope, candidates []Candidate, stateForBatch func([]Candidate) string) ([]Result, error) {
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
	batchPoint := point
	if !config.Allows(batchPoint) {
		for _, candidate := range candidates {
			if candidate.Point != "" && config.Allows(candidate.Point) {
				batchPoint = candidate.Point
				break
			}
		}
	}
	if err != nil || !config.Allows(batchPoint) {
		return originalResults(candidates, "disabled"), nil
	}
	if err = config.Validate(); err != nil {
		return originalResults(candidates, "config_invalid"), nil
	}
	serviceDeadline := time.Now().Add(time.Duration(config.BudgetMS) * time.Millisecond)
	budgetDeadline := serviceDeadline
	budgetReason := "budget_exhausted"
	if deadline, ok := ctx.Value(stageDeadlineKey{}).(time.Time); ok && !deadline.After(budgetDeadline) {
		budgetDeadline = deadline
		budgetReason = "stage_budget_exhausted"
	}
	budgetCtx, cancel := context.WithDeadline(ctx, budgetDeadline)
	defer cancel()
	results := originalResults(candidates, "disabled")
	eligible := []Candidate{}
	indices := []int{}
	for index, candidate := range candidates {
		if candidate.Point != "" && !config.Allows(candidate.Point) {
			continue
		}
		eligible = append(eligible, candidate)
		indices = append(indices, index)
	}
	for start := 0; start < len(eligible); start += config.BatchSize {
		end := min(start+config.BatchSize, len(eligible))
		batch := eligible[start:end]
		if e := ctx.Err(); e != nil {
			return results, e
		}
		rs, e := s.batch(budgetCtx, ctx, config, version, secret, batchPoint, scope, stateForBatch(batch), batch, budgetReason)
		for offset, result := range rs {
			results[indices[start+offset]] = result
		}
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
func (s *Service) batch(budget, parent context.Context, c Config, version int64, secret, point string, scope Scope, state string, candidates []Candidate, budgetReason string) ([]Result, error) {
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
		reason = budgetReason
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
				if budget.Err() != nil {
					status = "timeout"
					reason = budgetReason
				} else if child.Err() != nil {
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
				if budget.Err() != nil {
					status, reason = "timeout", budgetReason
				} else if child.Err() != nil {
					status, reason = "timeout", "request_timeout"
				}
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
		if candidate.Point != "" {
			r.Point = candidate.Point
			r.SpecVersion = "kev." + candidate.Point + ".v1"
		}
		r.ReturnedModel = envelope.Model
		r.Usage = envelope.Usage
		r.ServiceLatency = envelope.Latency
		r.ResponseBytes = responseBytes
		r.ResponseLimitExceeded = reason == "response_too_large"
		if status == "ok" {
			a, outcome, p, margin, err := ParseChoiceAnswer(r.RawAnswer, c, candidate.Question.Criteria)
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
