package core

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/ai/decision"
)

func TestWorldEventAttemptReclaimFencesLateModelAndConsumesHour(t *testing.T) {
	f, _, _, now := seedNPCEncounterRuntime(t, false)
	world, err := worldIDWith(f.ctx, f.repository.Pool(), f.fluctlightID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.EnsureWorldEventGenerationIntents(f.ctx, 100); err != nil {
		t.Fatal(err)
	}
	intent := "world_event_generation:" + world
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.platform_workflow_intents SET status='started',attempt_count=1,started_at=now() WHERE intent_id=$1`, intent); err != nil {
		t.Fatal(err)
	}
	seedCognitiveProviderRole(t, f.ctx, f.repository, "world-reclaim")
	entered, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	f.app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().on("world_event_generation_v1", func(map[string]any) fakeProviderResult {
		if calls.Add(1) == 1 {
			close(entered)
			<-release
		}
		return fakeProviderResult{Structured: map[string]any{"decision": "no_event", "reason": "本小时平静"}}
	})}
	done := make(chan error, 1)
	go func() { _, err := f.app.GenerateWorldEventCycle(f.ctx, world); done <- err }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		close(release)
		t.Fatal("generation never started")
	}
	var payload []byte
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT payload FROM public.platform_workflow_intents WHERE intent_id=$1`, intent).Scan(&payload); err != nil {
		close(release)
		t.Fatal(err)
	}
	reclaimed, reclaimErr := f.app.ReclaimWorldEventGenerationAttempt(f.ctx, intent, 1, payload)
	close(release)
	if err := <-done; err == nil {
		t.Fatal("late model result was accepted after reclaim")
	}
	if reclaimErr != nil || !reclaimed {
		t.Fatal(reclaimed, reclaimErr)
	}
	var status, cycle, code string
	var attempts int
	var next time.Time
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status,attempt_count,next_attempt_at,payload->>'cycle_status' FROM public.platform_workflow_intents WHERE intent_id=$1`, intent).Scan(&status, &attempts, &next, &cycle); err != nil || status != "retry" || attempts != 1 || cycle != "failed" || time.Until(next) < 50*time.Second || time.Until(next) > 65*time.Second {
		t.Fatal("reclaim lost backoff/attempt/cycle", status, attempts, next, cycle, err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status,error_code FROM public.world_event_generations WHERE world_id=$1`, world).Scan(&status, &code); err != nil || status != "failed" || code != "world_event_generation_timeout" {
		t.Fatal("late completion overwrote timeout", status, code, err)
	}
	if again, err := f.app.ReclaimWorldEventGenerationAttempt(f.ctx, intent, 1, payload); err != nil || again {
		t.Fatal("repeated reclaim moved retry deadline", again, err)
	}
	if result, err := f.app.GenerateWorldEventCycle(f.ctx, world); err != nil || result["status"] != "failed" || calls.Load() != 1 {
		t.Fatal("consumed hour regenerated", result, calls.Load(), err)
	}
	*now = now.Add(time.Hour)
	if result, err := f.app.GenerateWorldEventCycle(f.ctx, world); err != nil || result["status"] != "no_event" || calls.Load() != 2 {
		t.Fatal("reclaim prevented next hourly cycle", result, calls.Load(), err)
	}
}

func TestWorldEventAttemptReclaimFencesLateKevAdmission(t *testing.T) {
	for _, choice := range []string{"yes", "no", "unclear"} {
		t.Run(choice, func(t *testing.T) {
			f, _, _, _ := seedNPCEncounterRuntime(t, false)
			world, err := worldIDWith(f.ctx, f.repository.Pool(), f.fluctlightID)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := f.app.EnsureWorldEventGenerationIntents(f.ctx, 100); err != nil {
				t.Fatal(err)
			}
			intent := "world_event_generation:" + world
			if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.platform_workflow_intents SET status='started',attempt_count=1 WHERE intent_id=$1`, intent); err != nil {
				t.Fatal(err)
			}
			seedCognitiveProviderRole(t, f.ctx, f.repository, "world-late-kev")
			f.app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().on("world_event_generation_v1", func(map[string]any) fakeProviderResult {
				return fakeProviderResult{Structured: map[string]any{"decision": "propose", "name": "小雨", "reason": "符合世界情境", "event": worldEventTestDefinition(map[string]any{"kind": "weather", "scope": "world", "value": "温和的小雨"})}}
			})}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var payload []byte
				if err := f.repository.Pool().QueryRow(f.ctx, `SELECT payload FROM public.platform_workflow_intents WHERE intent_id=$1`, intent).Scan(&payload); err != nil {
					t.Error(err)
					w.WriteHeader(500)
					return
				}
				if reclaimed, err := f.app.ReclaimWorldEventGenerationAttempt(f.ctx, intent, 1, payload); err != nil || !reclaimed {
					t.Error("reclaim failed before late Kev result", reclaimed, err)
				}
				var req decision.Request
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Error(err)
				}
				answers := map[string]any{}
				for id := range req.Questions {
					prob := map[string]float64{"yes": 0.05, "no": 0.05, "unclear": 0.05}
					prob[choice] = 0.9
					answers[id] = map[string]any{"type": "choice", "choice": choice, "confidence": 0.9, "probabilities": prob}
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{"answers": answers})
			}))
			defer server.Close()
			config := decision.DefaultConfig()
			config.Enabled, config.Endpoint = true, server.URL
			if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.runtime_settings(key,value_json,revision) VALUES('kev',$1,1) ON CONFLICT(key) DO UPDATE SET value_json=excluded.value_json,revision=1`, jsonString(config)); err != nil {
				t.Fatal(err)
			}
			f.app.Kev = &decision.Service{Settings: &kevFixedSettings{config}, Store: &kevStore{app: f.app, directory: t.TempDir()}, HTTP: server.Client(), ID: func() string { return randomID("reclaimed_kev_") }}
			if _, err := f.app.GenerateWorldEventCycle(f.ctx, world); err == nil {
				t.Fatal("late Kev result accepted after reclaim")
			}
			var templates, runs int
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT (SELECT count(*) FROM public.world_event_templates WHERE generation_id IS NOT NULL),(SELECT count(*) FROM public.world_event_runs)`).Scan(&templates, &runs); err != nil || templates != 0 || runs != 0 {
				t.Fatal("late Kev result created executable effects", templates, runs, err)
			}
			var status, code string
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status,error_code FROM public.world_event_generations WHERE world_id=$1`, world).Scan(&status, &code); err != nil || status != "failed" || code != "world_event_generation_timeout" {
				t.Fatal("late Kev result overwrote terminal timeout", status, code, err)
			}
		})
	}
}

func TestWorldEventAttemptReclaimPreservesNewerAttemptAndCancellation(t *testing.T) {
	ctx, repo := isolatedCoreTestRepository(t)
	app := &App{DB: repo}
	const payload = `{"world_id":"reclaim-world"}`
	for _, state := range []string{"cancel_requested", "cancelled", "completed", "retry", "started"} {
		t.Run(state, func(t *testing.T) {
			if _, err := repo.Pool().Exec(ctx, `INSERT INTO public.platform_workflow_intents(intent_id,workflow_id,task_queue,intent_type,payload,status,attempt_count) VALUES($1,$1,'world-events','world.events.generate',$2,$3,2) ON CONFLICT(intent_id) DO UPDATE SET status=$3,attempt_count=2,payload=$2`, "reclaim-test", payload, state); err != nil {
				t.Fatal(err)
			}
			if reclaimed, err := app.ReclaimWorldEventGenerationAttempt(ctx, "reclaim-test", 1, []byte(payload)); err != nil || reclaimed {
				t.Fatal("stale attempt changed newer/cancelled ledger", reclaimed, err)
			}
			if reclaimed, err := app.ReclaimWorldEventGenerationAttempt(ctx, "reclaim-test", 2, []byte(`{"world_id":"older-cycle"}`)); err != nil || reclaimed {
				t.Fatal("old payload changed current cycle", reclaimed, err)
			}
			var got string
			if err := repo.Pool().QueryRow(ctx, `SELECT status FROM public.platform_workflow_intents WHERE intent_id='reclaim-test'`).Scan(&got); err != nil || got != state {
				t.Fatal(got, state, err)
			}
		})
	}
}
