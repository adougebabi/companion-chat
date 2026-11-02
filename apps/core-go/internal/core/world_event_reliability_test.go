package core

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

func TestWorldHourLockDeduplicatesConcurrentCallsRestartAndMultipleMembers(t *testing.T) {
	f, _, _, now := seedNPCEncounterRuntime(t, false)
	world, err := worldIDWith(f.ctx, f.repository.Pool(), f.fluctlightID)
	if err != nil {
		t.Fatal(err)
	}
	seedSharedWorldMember(t, f, "another-member", f.ownerID)
	if _, err := f.app.DispatchWorldSurface(f.ctx, "PUT", "/internal/fluctlights/another-member/world/active", nil, f.ownerID, map[string]any{"world_id": world, "expected_revision": 0, "reason": "shared clock", "idempotency_key": "clock-member"}); err != nil {
		t.Fatal(err)
	}
	count, err := f.app.EnsureWorldEventGenerationIntents(f.ctx, 100)
	if err != nil || count != 1 {
		t.Fatal("clock is per world, not per member", count, err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.platform_workflow_intents SET status='started' WHERE intent_type='world.events.generate'`); err != nil {
		t.Fatal(err)
	}
	seedCognitiveProviderRole(t, f.ctx, f.repository, "world-dedup")
	var calls atomic.Int32
	entered, release := make(chan struct{}), make(chan struct{})
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
		t.Fatal("generation did not start")
	}
	second, err := f.app.GenerateWorldEventCycle(f.ctx, world)
	if err != nil || second["status"] != "already_running" {
		t.Fatal(second, err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if again, err := f.app.GenerateWorldEventCycle(f.ctx, world); err != nil || again["status"] != "no_event" || calls.Load() != 1 {
		t.Fatal("repeated hourly work after restart", again, calls.Load(), err)
	}
	var stages int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.world_event_generations WHERE world_id=$1 AND slot_at=$2`, world, now.UTC().Truncate(time.Hour)).Scan(&stages); err != nil || stages != 1 {
		t.Fatal(stages, err)
	}
	// The raw intent still represents a durable clock. Its read-only task view
	// reports waiting and exposes the terminal cycle instead of a stuck activity.
	sql, err := ownerDiagnosticSource("workflow-intents")
	if err != nil {
		t.Fatal(err)
	}
	var raw []byte
	if err := f.repository.Pool().QueryRow(f.ctx, "SELECT data FROM ("+sql+") v WHERE key=$2", f.ownerID, "world_event_generation:"+world).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	view := decodeObject(raw)
	if view["status"] != "waiting" || view["clock_status"] != "started" || view["last_cycle_status"] != "no_event" || view["next_cycle_at"] == nil {
		t.Fatal("clock looked like a stuck task", view)
	}
	// A different world owns a distinct clock and generation, even at the same hour.
	another, err := f.app.DispatchWorldSurface(f.ctx, "POST", "/internal/fluctlights/"+f.fluctlightID+"/world/catalog", nil, f.ownerID, map[string]any{"name": "另一个世界", "reason": "independent clock", "idempotency_key": "another-world"})
	if err != nil {
		t.Fatal(err)
	}
	if result, err := f.app.GenerateWorldEventCycle(f.ctx, stringValue(another["id"])); err != nil || result["status"] != "no_event" || calls.Load() != 2 {
		t.Fatal("distinct worlds incorrectly deduplicated", result, calls.Load(), err)
	}
}

func TestWorldGenerationDeadlinePersistsTerminalFailureAndReleasesHourLock(t *testing.T) {
	f, _, _, _ := seedNPCEncounterRuntime(t, false)
	world, err := worldIDWith(f.ctx, f.repository.Pool(), f.fluctlightID)
	if err != nil {
		t.Fatal(err)
	}
	seedCognitiveProviderRole(t, f.ctx, f.repository, "world-timeout")
	f.app.Provider.HTTP = &http.Client{Transport: projectHealthRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		<-req.Context().Done()
		return nil, req.Context().Err()
	})}
	ctx, cancel := context.WithTimeout(f.ctx, 500*time.Millisecond)
	defer cancel()
	if _, err := f.app.GenerateWorldEventCycle(ctx, world); err == nil {
		t.Fatal("world generation timeout was swallowed")
	}
	var status, code string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status,error_code FROM public.world_event_generations WHERE world_id=$1`, world).Scan(&status, &code); err != nil || status != "failed" || code != "world_event_generation_timeout" {
		t.Fatal("hour never reached terminal timeout", status, code, err)
	}
	again, err := f.app.GenerateWorldEventCycle(f.ctx, world)
	if err != nil || again["status"] != "failed" {
		t.Fatal("lock leaked or failed slot regenerated", again, err)
	}
}

func TestIncompleteWorldsSkipEventsWithoutModelOrKev(t *testing.T) {
	for _, missing := range []string{"places", "npcs"} {
		t.Run(missing, func(t *testing.T) {
			f, _, _, _ := seedNPCEncounterRuntime(t, false)
			world, err := worldIDWith(f.ctx, f.repository.Pool(), f.fluctlightID)
			if err != nil {
				t.Fatal(err)
			}
			if missing == "npcs" {
				_, err = f.repository.Pool().Exec(f.ctx, `UPDATE public.world_npcs SET status='archived' WHERE world_id=$1`, world)
			} else {
				_, err = f.repository.Pool().Exec(f.ctx, `UPDATE public.world_places SET status='archived' WHERE world_id=$1`, world)
			}
			if err != nil {
				t.Fatal(err)
			}
			f.app.Provider.HTTP = &http.Client{Transport: projectHealthRoundTripFunc(func(*http.Request) (*http.Response, error) {
				t.Error("incomplete world reached model")
				return nil, errors.New("unexpected model")
			})}
			result, err := f.app.GenerateWorldEventCycle(f.ctx, world)
			if err != nil || result["status"] != "skipped" || result["reason_code"] != "world_base_objects_missing" {
				t.Fatal(result, err)
			}
			f.requireNoModelRuns(t)
		})
	}
}

func TestAbandonedWorldGenerationTimesOutAndClockTaskShowsTerminalCycle(t *testing.T) {
	f, _, _, now := seedNPCEncounterRuntime(t, false)
	world, err := worldIDWith(f.ctx, f.repository.Pool(), f.fluctlightID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.owner_accounts(human_actor_id,credential_hash,credential_revision) VALUES($1,'test-only','world-test') ON CONFLICT DO NOTHING`, f.ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.EnsureWorldEventGenerationIntents(f.ctx, 100); err != nil {
		t.Fatal(err)
	}
	intent := "world_event_generation:" + world
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.platform_workflow_intents SET status='started',started_at=now()-interval '1 hour',payload=payload||jsonb_build_object('cycle_status','generating','cycle_started_at',$2::text,'cycle_next_at',$3::text) WHERE intent_id=$1`, intent, now.UTC().Format(time.RFC3339Nano), now.UTC().Truncate(time.Hour).Add(time.Hour).Format(time.RFC3339Nano)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.world_event_generations(id,world_id,slot_at,status,model_correlation_id,updated_at) VALUES('abandoned-world-hour',$1,$2,'generating','abandoned',now()-interval '11 minutes')`, world, now.UTC().Truncate(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.EnsureWorldEventGenerationIntents(f.ctx, 100); err != nil {
		t.Fatal(err)
	}
	var status, code string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status,error_code FROM public.world_event_generations WHERE id='abandoned-world-hour'`).Scan(&status, &code); err != nil || status != "failed" || code != "world_event_generation_timeout" {
		t.Fatal(status, code, err)
	}
	task, err := f.app.OwnerTask(f.ctx, f.ownerID, intent)
	if err != nil || task["status"] != "waiting" || mapValue(task["source"])["last_cycle_status"] != "failed" || task["next_attempt_at"] == nil {
		t.Fatal("abandoned cycle still looked started", task, err)
	}
	f.requireNoModelRuns(t)
}

func TestWorldHourUniqueSlotFencesNoncanonicalGenerationID(t *testing.T) {
	f, _, _, now := seedNPCEncounterRuntime(t, false)
	world, err := worldIDWith(f.ctx, f.repository.Pool(), f.fluctlightID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.world_event_generations(id,world_id,slot_at,status,model_correlation_id) VALUES('existing-other-generation-id',$1,$2,'no_event','prior-attempt')`, world, now.UTC().Truncate(time.Hour)); err != nil {
		t.Fatal(err)
	}
	result, err := f.app.GenerateWorldEventCycle(f.ctx, world)
	if err != nil || result["status"] != "already_generated" {
		t.Fatal("world/hour uniqueness did not fence new attempt", result, err)
	}
	f.requireNoModelRuns(t)
}
