package core

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/ai/decision"
	"github.com/jackc/pgx/v5"
)

type kevSettingsReader struct{ app *App }

func decodeKevConfig(raw []byte) (decision.Config, error) {
	c := decision.DefaultConfig()
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(&c); err != nil {
		return c, errors.New("kev_config_invalid")
	}
	return c, c.Validate()
}
func (r kevSettingsReader) Read(ctx context.Context) (decision.Config, int64, string, error) {
	c := decision.DefaultConfig()
	if r.app == nil || r.app.DB == nil || r.app.DB.Pool() == nil {
		return c, 0, "", nil
	}
	var raw string
	var version int64
	err := r.app.DB.Pool().QueryRow(ctx, `SELECT value_json,revision FROM public.runtime_settings WHERE key='kev'`).Scan(&raw, &version)
	if errors.Is(err, pgx.ErrNoRows) {
		return c, 0, "", nil
	}
	if err != nil {
		return c, 0, "", err
	}
	c, err = decodeKevConfig([]byte(raw))
	if err != nil {
		return c, version, "", err
	}
	// Disabled configuration never decrypts a credential or touches the service.
	if !c.Enabled {
		return c, version, "", nil
	}
	p := ProviderClient{DB: r.app.DB, SettingsKey: r.app.SettingsKey}
	secret, err := p.secret(ctx, "kev:systemone")
	return c, version, secret, err
}

// kevStore has a bounded, fsync'd append journal. Reimport never executes a
// business action. A database or journal error propagates to the adoption gate.
type kevStore struct {
	app       *App
	directory string
	mu        sync.Mutex
}

const kevSpoolLimit = 32 * 1024 * 1024

func (s *kevStore) saveDB(ctx context.Context, r decision.Record) error {
	if s.app == nil || s.app.DB == nil || s.app.DB.Pool() == nil {
		return errors.New("kev_audit_database_unavailable")
	}
	_, err := s.app.DB.Pool().Exec(ctx, `INSERT INTO public.kev_decisions(id,request_id,question_id,decision_point,actor_self,actor_user,agent,run_id,candidate_id,started_at,completed_at,payload) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) ON CONFLICT(id) DO UPDATE SET payload=excluded.payload,completed_at=excluded.completed_at WHERE (excluded.payload->>'recorded_at')::timestamptz >= (kev_decisions.payload->>'recorded_at')::timestamptz`, r.ID, r.RequestID, r.QuestionID, r.Point, r.Scope.ActorSelf, nullableString(r.Scope.ActorUser), nullableString(r.Scope.Agent), nullableString(r.Scope.RunID), r.CandidateID, r.StartedAt, r.CompletedAt, jsonBytes(r))
	return err
}
func (s *kevStore) Save(ctx context.Context, r decision.Record) error {
	if err := s.saveDB(ctx, r); err == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.directory == "" {
		return errors.New("kev_audit_spool_unconfigured")
	}
	if err := os.MkdirAll(s.directory, 0700); err != nil {
		return err
	}
	release, err := s.lockJournal(ctx)
	if err != nil {
		return err
	}
	defer release()
	path := filepath.Join(s.directory, "decisions.jsonl")
	if err := s.repairJournalTail(path); err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	size := int64(0)
	if info != nil {
		size = info.Size()
	}
	if size+int64(len(data)) > kevSpoolLimit {
		slog.Warn("kev_audit_spool_full")
		return errors.New("kev_audit_spool_full")
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	if written, writeErr := f.Write(data); writeErr != nil {
		return writeErr
	} else if written != len(data) {
		return io.ErrShortWrite
	}
	if err = f.Sync(); err != nil {
		return err
	}
	// Persist creation of the directory entry as well as the file contents.
	d, err := os.Open(s.directory)
	if err != nil {
		return err
	}
	defer d.Close()
	if err = d.Sync(); err != nil {
		return err
	}
	slog.Warn("kev_audit_spooled", "decision_id", r.ID)
	return nil
}
func (s *kevStore) Import(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := os.Stat(s.directory); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	release, err := s.lockJournal(ctx)
	if err != nil {
		return err
	}
	defer release()
	path := filepath.Join(s.directory, "decisions.jsonl")
	if err := s.repairJournalTail(path); err != nil {
		return err
	}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	decoder := json.NewDecoder(f)
	for decoder.More() {
		var r decision.Record
		if err := decoder.Decode(&r); err != nil {
			return err
		}
		if err := s.saveDB(ctx, r); err != nil {
			return err
		}
	}
	// Truncate only after every append was durably imported. Reimport after a
	// crash is harmless because record identity and application state are stable.
	out, err := os.OpenFile(path, os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer out.Close()
	return out.Sync()
}

var kevServiceInitializationMu sync.Mutex

func (a *App) kevService() *decision.Service {
	if a == nil {
		return nil
	}
	kevServiceInitializationMu.Lock()
	defer kevServiceInitializationMu.Unlock()
	if a.Kev == nil {
		directory := os.Getenv("FLUCTLIGHT_KEV_SPOOL_DIR")
		if directory == "" {
			directory = "/var/lib/fluctlight/kev"
		}
		a.Kev = &decision.Service{Settings: kevSettingsReader{a}, Store: &kevStore{app: a, directory: directory}, HTTP: &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, ID: func() string { return randomID("kev_") }}
	}
	return a.Kev
}
func (a *App) kevScope(ctx context.Context, projection ContextProjection, agent string) decision.Scope {
	return decision.Scope{ActorSelf: projection.FluctlightID, ActorUser: projection.OwnerActorID, Agent: agent, RunID: providerCorrelation(ctx), EventID: projection.SourceFactID, CorrelationID: firstString(providerCorrelation(ctx), projection.SourceFactID), StateVersion: stableDigest(jsonString(map[string]any{"facts": projection.CurrentFactsRevision, "persona": projection.CorePersonaRevision, "profile": projection.PersonalityRuntime, "life": projection.LifeContextRevision}))}
}

func (a *App) decideKev(ctx context.Context, point string, scope decision.Scope, state any, id, instructions string) (decision.Result, decision.Outcome, error) {
	results, err := a.kevService().Decide(ctx, point, scope, jsonString(state), []decision.Candidate{{ID: id, Question: decision.Choice(instructions)}})
	if err != nil {
		return decision.Result{}, decision.Original, err
	}
	if len(results) == 0 {
		return decision.Result{}, decision.Original, nil
	}
	r := results[0]
	out, err := r.Admit(ctx)
	return r, out, err
}

// kevAutomaticGate retains the original execution budget and source identity.
// Its independent counter cannot mark an event as processed or a task complete.
func (a *App) kevAutomaticGate(ctx context.Context, point, owner, candidate string, state any) (bool, time.Time, error) {
	service := a.kevService()
	if !service.Enabled(ctx, point) {
		return true, time.Time{}, ctx.Err()
	}
	c, _, _, err := service.Settings.Read(ctx)
	if err != nil {
		if parentErr := ctx.Err(); parentErr != nil {
			return false, time.Time{}, parentErr
		}
		slog.Warn("kev_settings_unavailable", "decision_point", point)
		return true, time.Time{}, nil
	}
	var count int
	var until time.Time
	err = a.DB.Pool().QueryRow(ctx, `SELECT deferral_count,deferred_until FROM public.kev_deferrals WHERE decision_point=$1 AND actor_self=$2 AND candidate_id=$3`, point, owner, candidate).Scan(&count, &until)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		if parentErr := ctx.Err(); parentErr != nil {
			return false, time.Time{}, parentErr
		}
		slog.Warn("kev_deferral_read_failed", "decision_point", point)
		return true, time.Time{}, nil
	}
	if count >= c.MaxDeferrals {
		if err := service.RecordOriginal(ctx, point, decision.Scope{ActorSelf: owner, Agent: point, RunID: candidate}, candidate, "max_deferral_original"); err != nil {
			slog.Warn("kev_application_record_failed")
		}
		_, err = a.DB.Pool().Exec(ctx, `DELETE FROM public.kev_deferrals WHERE decision_point=$1 AND actor_self=$2 AND candidate_id=$3`, point, owner, candidate)
		return true, time.Time{}, err
	}
	if count > 0 && a.now().Before(until) {
		return false, until, nil
	}
	r, out, err := a.decideKev(ctx, point, decision.Scope{ActorSelf: owner, Agent: point, RunID: candidate, CorrelationID: providerCorrelation(ctx)}, state, candidate, "Should this eligible automatic process run now? Answer no only when it can safely be deferred; unclear means use its original process.")
	if err != nil {
		return false, time.Time{}, err
	}
	if out != decision.No {
		status := "applied"
		if out == decision.Original {
			status = "original_used"
		}
		if err := r.Finish(ctx, status, "", candidate); err != nil {
			slog.Warn("kev_application_record_failed", "decision_id", r.Record.ID)
		}
		return true, time.Time{}, nil
	}
	until = a.now().UTC().Add(time.Duration(c.DeferralSeconds) * time.Second)
	_, err = a.DB.Pool().Exec(ctx, `INSERT INTO public.kev_deferrals(decision_point,actor_self,candidate_id,deferral_count,deferred_until) VALUES($1,$2,$3,1,$4) ON CONFLICT(decision_point,actor_self,candidate_id) DO UPDATE SET deferral_count=kev_deferrals.deferral_count+1,deferred_until=excluded.deferred_until`, point, owner, candidate, until)
	if err != nil {
		if finishErr := r.Finish(ctx, "original_used", "deferral_persistence_failed", candidate); finishErr != nil {
			slog.Warn("kev_application_record_failed", "decision_id", r.Record.ID)
		}
		if parentErr := ctx.Err(); parentErr != nil {
			return false, time.Time{}, parentErr
		}
		slog.Warn("kev_deferral_persistence_failed", "decision_point", point)
		return true, time.Time{}, nil
	}
	_ = r.Finish(ctx, "applied", "deferred", candidate)
	return false, until, nil
}

func WithKevOriginal(ctx context.Context) context.Context { return decision.WithOriginal(ctx) }

func (a *App) selectKevContext(ctx context.Context, projection ContextProjection, surface ProviderContextSurface, currentInput string, input WorkingMemoryInput) (WorkingMemoryInput, error) {
	s := a.kevService()
	if !s.Enabled(ctx, "context.select") {
		return input, ctx.Err()
	}
	// Recent history is a contiguous protocol-owned sequence. Never select its
	// individual messages; only optional memory, summary and runtime fragments.
	sections := []*[]PromptFragment{&input.ActiveCandidates, &input.ResidentCandidates, &input.RetrievedMemories, &input.Summaries, &input.RuntimeFacts}
	for _, section := range sections {
		candidates := []decision.Candidate{}
		states := map[string]any{}
		for index, f := range *section {
			if f.Required {
				continue
			}
			id := fmt.Sprintf("%s:%d:%s", f.Kind, index, stableDigest(jsonString(f.SourceRefs)))
			candidates = append(candidates, decision.Candidate{ID: id, Question: decision.Choice("Is optional context " + id + " relevant to the current task? Keep unresolved commitments and necessary task evidence.")})
			states[id] = f.Content
		}
		results, err := s.DecideWithBatchState(ctx, "context.select", a.kevScope(ctx, projection, string(surface)), candidates, func(batch []decision.Candidate) string {
			batchStates := make(map[string]any, len(batch))
			for _, candidate := range batch {
				batchStates[candidate.ID] = states[candidate.ID]
			}
			return jsonString(map[string]any{"input": currentInput, "candidates": batchStates})
		})
		if err != nil {
			return input, err
		}
		selected := []PromptFragment{}
		n := 0
		for _, f := range *section {
			if f.Required {
				selected = append(selected, f)
				continue
			}
			r := results[n]
			n++
			out, err := r.Admit(ctx)
			if err != nil {
				return input, err
			}
			if out != decision.No {
				selected = append(selected, f)
			}
			status := "applied"
			if out == decision.Original {
				status = "original_used"
			}
			_ = r.Finish(ctx, status, "", "")
		}
		*section = selected
	}
	return input, nil
}

// KevDecisions is an Owner-only bounded JSON export/list. Filters never grant
// authority; the authenticated owner gate is applied before reading payloads.
func (a *App) KevDecisions(ctx context.Context, actor string, filter map[string]string) ([]map[string]any, error) {
	if err := a.requireOwner(ctx, actor); err != nil {
		return nil, err
	}
	limit := 100
	if filter["export"] == "true" {
		limit = 1000
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT payload FROM public.kev_decisions WHERE ($1='' OR actor_self=$1) AND ($2='' OR decision_point=$2) AND ($3='' OR agent=$3) AND ($4='' OR id=$4) AND ($5='' OR payload->>'policy_outcome'=$5) AND ($6='' OR payload->>'call_status'=$6) ORDER BY started_at DESC,id DESC LIMIT $7`, filter["actor_self"], filter["decision_point"], filter["agent"], filter["id"], filter["policy_outcome"], filter["call_status"], limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		out = append(out, decodeObject(raw))
	}
	return out, rows.Err()
}
func (a *App) MaintainKevDiagnostics(ctx context.Context) error {
	s := a.kevService()
	if store, ok := s.Store.(*kevStore); ok {
		if err := store.Import(ctx); err != nil && !errors.Is(err, os.ErrNotExist) {
			slog.Warn("kev_spool_import_failed")
		}
	}
	c, _, _, err := s.Settings.Read(ctx)
	if err != nil {
		return err
	}
	_, err = a.DB.Pool().Exec(ctx, `DELETE FROM public.kev_decisions WHERE id IN(SELECT id FROM public.kev_decisions WHERE started_at<now()-make_interval(days=>$1) ORDER BY started_at LIMIT 500)`, c.RetentionDays)
	return err
}

func sortedKevPointNames() []string {
	out := append([]string(nil), decision.Points...)
	sort.Strings(out)
	return out
}

func reflectionKevEvidence(evidence []map[string]any) any {
	out := make([]map[string]any, 0, len(evidence))
	for _, entry := range evidence {
		out = append(out, map[string]any{"event_type": entry["event_type"], "occurred_at": entry["occurred_at"], "appraisal": entry["appraisal"]})
	}
	return map[string]any{"new_events": out}
}

func (s *kevStore) lockJournal(ctx context.Context) (func(), error) {
	f, err := os.OpenFile(filepath.Join(s.directory, "journal.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	for {
		err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			break
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) {
			f.Close()
			return nil, err
		}
		timer := time.NewTimer(5 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			f.Close()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}

	return func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN); _ = f.Close() }, nil
}
func (s *kevStore) repairJournalTail(path string) error {
	f, err := os.OpenFile(path, os.O_RDWR, 0600)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if info.Size() == 0 {
		return nil
	}
	if info.Size() > kevSpoolLimit {
		return errors.New("kev_audit_spool_full")
	}
	last := []byte{0}
	if _, err := f.ReadAt(last, info.Size()-1); err != nil {
		return err
	}
	if last[0] == '\n' {
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(f, kevSpoolLimit+1))
	if err != nil {
		return err
	}
	boundary := bytes.LastIndexByte(data, '\n') + 1
	// An incomplete terminal record was never confirmed durable. Keep one
	// bounded quarantine copy for diagnostics; never adopt or execute it.
	quarantine, err := os.OpenFile(filepath.Join(s.directory, "incomplete-tail.json"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	_, err = quarantine.Write(data[boundary:])
	if err == nil {
		err = quarantine.Sync()
	}
	closeErr := quarantine.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err := f.Truncate(int64(boundary)); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	slog.Warn("kev_audit_incomplete_tail_quarantined")
	return nil
}
func (a *App) RunKevMaintenance(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			bounded, cancel := context.WithTimeout(ctx, 10*time.Second)
			if err := a.MaintainKevDiagnostics(bounded); err != nil && ctx.Err() == nil {
				slog.Warn("kev_maintenance_failed")
			}
			cancel()
		}
	}
}

func (a *App) kevWakeUpState(ctx context.Context, owner string, cycle int, life map[string]any) (map[string]any, error) {
	state := map[string]any{"trigger": "periodic_check", "cycle": cycle, "periodic_opportunity": true, "life": life, "last_run_at": nil, "elapsed_seconds": nil, "unprocessed_events": []map[string]any{}}
	var last *time.Time
	if err := a.DB.Pool().QueryRow(ctx, `SELECT max(occurred_at) FROM public.cognition_wakeups WHERE fluctlight_id=$1`, owner).Scan(&last); err != nil {
		return nil, err
	}
	if last != nil {
		state["last_run_at"] = last.UTC().Format(time.RFC3339Nano)
		state["elapsed_seconds"] = max(0, int(a.now().Sub(*last).Seconds()))
	}
	rows, err := a.DB.Pool().Query(ctx, `SELECT id,event_type,occurred_at,COALESCE(payload->>'summary','') FROM public.cognition_inbox WHERE fluctlight_id=$1 AND status='pending' ORDER BY sequence LIMIT 8`, owner)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := []map[string]any{}
	for rows.Next() {
		var id, kind, summary string
		var at time.Time
		if err := rows.Scan(&id, &kind, &at, &summary); err != nil {
			return nil, err
		}
		runes := []rune(summary)
		if len(runes) > 160 {
			summary = string(runes[:160])
		}
		events = append(events, map[string]any{"id": id, "event_type": kind, "occurred_at": at.UTC().Format(time.RFC3339Nano), "summary": summary})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	state["unprocessed_events"] = events
	return state, nil
}
