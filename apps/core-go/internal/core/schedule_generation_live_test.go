package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Opt-in replay keeps private source facts outside Git. It uses the production
// generation/repair/validation path with the actual selected model, not a fake
// response. The separate lifecycle test verifies acceptance in PostgreSQL.
func TestLiveInitialScheduleFromPrivateCapture(t *testing.T) {
	base, model := liveProviderConfig(t)
	path := os.Getenv("FLUCTLIGHT_LIVE_SCHEDULE_CAPTURE")
	if path == "" {
		t.Skip("private schedule capture was not provided")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var capture struct {
		Prompt []map[string]any `json:"prompt"`
	}
	if err := json.Unmarshal(raw, &capture); err != nil {
		t.Fatal(err)
	}
	var data map[string]any
	for _, message := range capture.Prompt {
		if message["role"] == "user" {
			if err := json.Unmarshal([]byte(stringValue(message["content"])), &data); err != nil {
				t.Fatal(err)
			}
		}
	}
	if data == nil {
		t.Fatal("capture has no typed schedule input")
	}
	world := mapValue(data["world_places"])
	places := &scheduleWorldPlaceContext{Timezone: stringValue(world["timezone"]), Name: stringValue(world["name"]), Background: stringValue(world["background"]), Rules: stringValue(world["rules"])}
	if world["configured"] == true {
		places.WorldID = "private-capture-world"
	}
	for _, value := range arrayValue(world["places"]) {
		places.Places = append(places.Places, mapValue(value))
	}
	rows := func(value any) []map[string]any {
		var result []map[string]any
		for _, raw := range arrayValue(value) {
			result = append(result, mapValue(raw))
		}
		return result
	}
	input := ScheduleGenerationTaskInput{LocalDate: stringValue(data["local_date"]), Timezone: stringValue(data["timezone"]), Identity: mapValue(data["identity"]), LifeProfile: mapValue(data["life_profile"]), CurrentState: mapValue(data["current_state"]), CurrentLife: mapValue(data["current_life"]), WorldPlaces: places, Goals: rows(data["goals"]), Intentions: rows(data["intentions"]), RecentOutcomes: rows(data["recent_outcomes"])}
	ctx, repo := isolatedCoreTestRepository(t)
	seedCognitiveProviderRole(t, ctx, repo, "live-schedule-capture")
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.provider_endpoints SET base_url=$1 WHERE id='live-schedule-capture'`, base); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.model_roles SET model_id=$1,timeout_seconds=180 WHERE provider_endpoint_id='live-schedule-capture'`, model); err != nil {
		t.Fatal(err)
	}
	dir := os.Getenv("FLUCTLIGHT_LIVE_EVIDENCE_DIR")
	if dir == "" {
		t.Fatal("private evidence directory is required")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"fresh", "repair"} {
		t.Run(mode, func(t *testing.T) {
			wire := &r7LiveWire{path: filepath.Join(dir, "schedule-"+mode+"-wire.json")}
			app := newTestApp(t, repo, wire)
			current := input
			if mode == "repair" {
				var full map[string]any
				if err := json.Unmarshal(raw, &full); err != nil {
					t.Fatal(err)
				}
				previous := decodeObject([]byte(stringValue(mapValue(full["response"])["content"])))
				_, validationErr := normalizeInitialScheduleResponse(previous, input.LocalDate, input.Timezone, places)
				if validationErr == nil {
					t.Fatal("recorded failed response unexpectedly passed")
				}
				current.RepairFeedback = &ScheduleGenerationRepairFeedback{PreviousResponse: previous, ValidationError: validationErr.Error()}
			}
			liveCtx, cancel := context.WithTimeout(context.Background(), 7*time.Minute)
			defer cancel()
			result, err := app.generateScheduleCandidate(liveCtx, current)
			if err != nil {
				t.Fatal(err)
			}
			if mapValue(result["reschedule_policy"])["default_initial_schedule"] == true {
				t.Fatalf("real model still failed schedule validation: %v", mapValue(result["reschedule_policy"])["validation_error"])
			}
			if len(arrayValue(result["items"])) == 0 {
				t.Fatal("empty schedule")
			}
			for _, row := range wire.rows {
				request := mapValue(row["request"])
				if request["max_tokens"] != nil || request["max_completion_tokens"] != nil {
					t.Fatal("unexpected output cap")
				}
			}
			if err := os.WriteFile(filepath.Join(dir, "schedule-"+mode+"-accepted.json"), jsonBytes(result), 0600); err != nil {
				t.Fatal(err)
			}
			t.Logf("validated %d intervals after %d physical requests", len(arrayValue(result["items"])), len(wire.rows))
		})
	}
}

// Reuse the exact real-model artifacts above; this checks the production
// acceptance transaction and read path without another model request.
func TestPrivateLiveScheduleArtifactsPersistAndReadBack(t *testing.T) {
	dir := os.Getenv("FLUCTLIGHT_LIVE_EVIDENCE_DIR")
	if dir == "" {
		t.Skip("private live artifacts were not provided")
	}
	for _, mode := range []string{"fresh", "repair"} {
		t.Run(mode, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(dir, "schedule-"+mode+"-accepted.json"))
			if err != nil {
				t.Fatal(err)
			}
			payload := decodeObject(raw)
			f := seedWardrobeToolFixture(t)
			world := stringValue(mapValue(payload["_schedule_world"])["world_id"])
			if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.life_worlds(id,owner_actor_id,timezone,name) VALUES($1,$2,$3,'private replay world')`, world, f.ownerID, payload["timezone"]); err != nil {
				t.Fatal(err)
			}
			if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.fluctlight_world_bindings(fluctlight_id,world_id) VALUES($1,$2) ON CONFLICT(fluctlight_id) DO UPDATE SET world_id=EXCLUDED.world_id,revision=1`, f.fluctlightID, world); err != nil {
				t.Fatal(err)
			}
			seen := map[string]bool{}
			for _, raw := range arrayValue(payload["items"]) {
				item := mapValue(raw)
				id := stringValue(item["place_id"])
				if id == "" || seen[id] {
					continue
				}
				seen[id] = true
				if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.world_places(id,world_id,name) VALUES($1,$2,$3)`, id, world, firstString(item["location"], "private replay place")); err != nil {
					t.Fatal(err)
				}
			}
			day, err := time.ParseInLocation("2006-01-02", stringValue(payload["local_date"]), time.UTC)
			if err != nil {
				t.Fatal(err)
			}
			f.app.Clock = fixedClock(day.Add(4 * time.Hour))
			_, life, err := f.app.readLifeContextSnapshotAt(f.ctx, f.fluctlightID, f.app.now())
			if err != nil {
				t.Fatal(err)
			}
			actualPlaces, err := f.app.readScheduleWorldPlaces(f.ctx, f.fluctlightID)
			if err != nil {
				t.Fatal(err)
			}
			actualPlaces.bind(payload)
			payload["expected_revision"] = 0
			payload["expected_life_context_revision"] = life["context_revision"]
			payload["evidence_refs"] = []any{"owner:" + f.ownerID}
			payload["idempotency_key"] = "private-live-" + mode
			if _, err := f.app.AcceptSchedule(f.ctx, f.ownerID, f.fluctlightID, payload); err != nil {
				t.Fatal("real generated schedule was not accepted", err)
			}
			accepted, _, err := f.app.readLifeContextSnapshotAt(f.ctx, f.fluctlightID, f.app.now())
			if err != nil || accepted == nil || len(arrayValue(accepted["items"])) != len(arrayValue(payload["items"])) || accepted["generated_from"] == "default_initial_schedule" {
				t.Fatal("accepted schedule read-back mismatch", err)
			}
			t.Logf("persisted and read back %d real-model intervals", len(arrayValue(accepted["items"])))
		})
	}
}
