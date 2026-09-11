package core

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func currentLifeForTest(t *testing.T, ctx context.Context, app *App, fluctlightID string, at time.Time) map[string]any {
	t.Helper()
	_, life, err := app.readLifeContextSnapshotAt(ctx, fluctlightID, at)
	if err != nil {
		t.Fatal(err)
	}
	return life
}

func fullDaySchedulePayloadForTest(at time.Time, idempotencyKey, expectedLifeRevision string) map[string]any {
	location, _ := time.LoadLocation("Asia/Shanghai")
	local := at.In(location)
	dayStart := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
	return map[string]any{
		"local_date": dayStart.Format("2006-01-02"), "timezone": "Asia/Shanghai",
		"expected_revision": 0, "expected_life_context_revision": expectedLifeRevision,
		"idempotency_key": idempotencyKey, "evidence_refs": []any{"owner-test-fact"},
		"items": []any{map[string]any{
			"start_at": dayStart.Format(time.RFC3339), "end_at": dayStart.AddDate(0, 0, 1).Format(time.RFC3339),
			"activity": "阅读", "scene": "书房", "item_type": "planned", "status": "planned",
			"priority": 0.5, "flexibility": 0.5, "interruption_cost": 0.5,
		}},
		"reschedule_policy": map[string]any{},
	}
}

func TestOwnerLifeEventCommandsUseFrozenCASAndCanonicalReplay(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID := "owner-event-authority", "fluctlight-event-authority"
	seedLifeContextFluctlight(t, ctx, repository, ownerID, fluctlightID)
	app := &App{DB: repository}
	now := time.Now().UTC()
	initialLife := currentLifeForTest(t, ctx, app, fluctlightID, now)
	createPayload := map[string]any{
		"kind": "owner_event", "start_at": now.Add(-time.Minute).Format(time.RFC3339), "end_at": now.Add(time.Hour).Format(time.RFC3339),
		"scene": "客厅", "activity": "讨论", "location": "家", "evidence_refs": []any{"owner-event-fact"},
		"expected_life_context_revision": initialLife["context_revision"], "idempotency_key": "owner-event-create",
	}
	created, err := app.CreateLifeEvent(ctx, ownerID, fluctlightID, cloneMap(createPayload))
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := app.CreateLifeEvent(ctx, ownerID, fluctlightID, cloneMap(createPayload))
	if err != nil || !boolValueForTest(replayed["replayed"]) || stringValue(replayed["id"]) != stringValue(created["id"]) || intValue(replayed["revision"]) != 1 {
		t.Fatalf("event replay=%#v created=%#v err=%v", replayed, created, err)
	}
	conflictPayload := cloneMap(createPayload)
	conflictPayload["activity"] = "不同活动"
	if _, err := app.CreateLifeEvent(ctx, ownerID, fluctlightID, conflictPayload); err == nil || err.Error() != "life_event_idempotency_conflict" {
		t.Fatalf("event conflicting replay err=%v", err)
	}
	stalePayload := cloneMap(createPayload)
	stalePayload["idempotency_key"] = "owner-event-stale"
	if _, err := app.CreateLifeEvent(ctx, ownerID, fluctlightID, stalePayload); !errors.Is(err, ErrLifeContextStale) {
		t.Fatalf("stale event create err=%v", err)
	}

	currentLife := currentLifeForTest(t, ctx, app, fluctlightID, time.Now().UTC())
	cancelPayload := map[string]any{
		"expected_event_revision": 1, "expected_life_context_revision": currentLife["context_revision"],
		"idempotency_key": "owner-event-cancel",
	}
	cancelled, err := app.CancelLifeEvent(ctx, ownerID, fluctlightID, stringValue(created["id"]), cloneMap(cancelPayload))
	if err != nil || stringValue(cancelled["status"]) != "cancelled" || intValue(cancelled["revision"]) != 2 {
		t.Fatalf("event cancel=%#v err=%v", cancelled, err)
	}
	cancelReplay, err := app.CancelLifeEvent(ctx, ownerID, fluctlightID, stringValue(created["id"]), cloneMap(cancelPayload))
	if err != nil || !boolValueForTest(cancelReplay["replayed"]) || intValue(cancelReplay["revision"]) != 2 {
		t.Fatalf("event cancel replay=%#v err=%v", cancelReplay, err)
	}
	cancelConflict := cloneMap(cancelPayload)
	cancelConflict["expected_event_revision"] = 2
	if _, err := app.CancelLifeEvent(ctx, ownerID, fluctlightID, stringValue(created["id"]), cancelConflict); err == nil || err.Error() != "life_event_cancel_idempotency_conflict" {
		t.Fatalf("event cancel conflicting replay err=%v", err)
	}
	var eventCount, factCount, outboxCount, commandCount int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.life_events WHERE fluctlight_id=$1`, fluctlightID).Scan(&eventCount); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.cognition_inbox WHERE fluctlight_id=$1 AND event_type IN ('life.event.created','life.event.cancelled')`, fluctlightID).Scan(&factCount); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.platform_outbox_events WHERE fluctlight_id=$1 AND kind IN ('life.event.created','life.event.cancelled')`, fluctlightID).Scan(&outboxCount); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.life_context_commands WHERE fluctlight_id=$1`, fluctlightID).Scan(&commandCount); err != nil {
		t.Fatal(err)
	}
	if eventCount != 1 || factCount != 2 || outboxCount != 2 || commandCount != 1 {
		t.Fatalf("owner event replay duplicated effects: events=%d facts=%d outbox=%d commands=%d", eventCount, factCount, outboxCount, commandCount)
	}
}

func TestOwnerPresenceReplacementExpiryAndReplayNeverResurrects(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID := "owner-presence-authority", "fluctlight-presence-authority"
	seedLifeContextFluctlight(t, ctx, repository, ownerID, fluctlightID)
	app := &App{DB: repository}
	now := time.Now().UTC()
	initialLife := currentLifeForTest(t, ctx, app, fluctlightID, now)
	firstPayload := map[string]any{
		"current_task": "长任务", "user_presence": "online", "expires_at": now.Add(4 * time.Hour).Format(time.RFC3339),
		"expected_life_context_revision": initialLife["context_revision"], "idempotency_key": "owner-presence-a",
	}
	first, err := app.SetPresence(ctx, ownerID, fluctlightID, cloneMap(firstPayload))
	if err != nil {
		t.Fatal(err)
	}
	firstReplay, err := app.SetPresence(ctx, ownerID, fluctlightID, cloneMap(firstPayload))
	if err != nil || !boolValueForTest(firstReplay["replayed"]) || stringValue(firstReplay["id"]) != stringValue(first["id"]) {
		t.Fatalf("Presence replay=%#v err=%v", firstReplay, err)
	}
	conflict := cloneMap(firstPayload)
	conflict["current_task"] = "冲突任务"
	if _, err := app.SetPresence(ctx, ownerID, fluctlightID, conflict); err == nil || err.Error() != "presence_idempotency_conflict" {
		t.Fatalf("Presence conflicting replay err=%v", err)
	}
	currentLife := currentLifeForTest(t, ctx, app, fluctlightID, time.Now().UTC())
	secondExpiry := time.Now().UTC().Add(time.Hour)
	second, err := app.SetPresence(ctx, ownerID, fluctlightID, map[string]any{
		"current_task": "短任务", "user_presence": "busy", "expires_at": secondExpiry.Format(time.RFC3339),
		"expected_life_context_revision": currentLife["context_revision"], "idempotency_key": "owner-presence-b",
	})
	if err != nil {
		t.Fatal(err)
	}
	var firstStatus, firstSupersededBy, secondStatus string
	if err := repository.Pool().QueryRow(ctx, `SELECT status,COALESCE(superseded_by_overlay_id,'') FROM public.life_presence_overlays WHERE id=$1`, first["id"]).Scan(&firstStatus, &firstSupersededBy); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT status FROM public.life_presence_overlays WHERE id=$1`, second["id"]).Scan(&secondStatus); err != nil {
		t.Fatal(err)
	}
	_, afterShortExpiry, err := resolveLifeContextSnapshotWith(ctx, repository.Pool(), fluctlightID, secondExpiry.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if firstStatus != "superseded" || firstSupersededBy != stringValue(second["id"]) || secondStatus != "active" || len(mapValue(afterShortExpiry["presence"])) != 0 {
		t.Fatalf("Presence replacement resurrected prior overlay: first=%s superseded_by=%s second=%s context=%#v", firstStatus, firstSupersededBy, secondStatus, afterShortExpiry)
	}
	var overlayCount, factCount, outboxCount int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.life_presence_overlays WHERE fluctlight_id=$1`, fluctlightID).Scan(&overlayCount); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.cognition_inbox WHERE fluctlight_id=$1 AND event_type='life.presence.updated'`, fluctlightID).Scan(&factCount); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.platform_outbox_events WHERE fluctlight_id=$1 AND kind='life.presence.updated'`, fluctlightID).Scan(&outboxCount); err != nil {
		t.Fatal(err)
	}
	if overlayCount != 2 || factCount != 2 || outboxCount != 2 {
		t.Fatalf("Presence replay duplicated effects: overlays=%d facts=%d outbox=%d", overlayCount, factCount, outboxCount)
	}
}

func TestOwnerScheduleAcceptanceAndCancelUseMonotonicRevisionAndReplay(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID := "owner-schedule-authority", "fluctlight-schedule-authority"
	seedLifeContextFluctlight(t, ctx, repository, ownerID, fluctlightID)
	app := &App{DB: repository}
	now := time.Now().UTC()
	initialLife := currentLifeForTest(t, ctx, app, fluctlightID, now)
	payload := fullDaySchedulePayloadForTest(now, "owner-schedule-a", stringValue(initialLife["context_revision"]))
	first, err := app.AcceptSchedule(ctx, ownerID, fluctlightID, cloneMap(payload))
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := app.AcceptSchedule(ctx, ownerID, fluctlightID, cloneMap(payload))
	if err != nil || !boolValueForTest(replayed["replayed"]) || stringValue(replayed["id"]) != stringValue(first["id"]) {
		t.Fatalf("schedule replay=%#v err=%v", replayed, err)
	}
	conflict := cloneMap(payload)
	mapValue(arrayValue(conflict["items"])[0])["activity"] = "冲突活动"
	if _, err := app.AcceptSchedule(ctx, ownerID, fluctlightID, conflict); err == nil || err.Error() != "schedule_idempotency_conflict" {
		t.Fatalf("schedule conflicting replay err=%v", err)
	}
	currentLife := currentLifeForTest(t, ctx, app, fluctlightID, time.Now().UTC())
	cancelPayload := map[string]any{
		"expected_revision": 1, "expected_life_context_revision": currentLife["context_revision"],
		"idempotency_key": "owner-schedule-cancel",
	}
	cancelled, err := app.CancelScheduleExpected(ctx, ownerID, fluctlightID, stringValue(first["id"]), cloneMap(cancelPayload))
	if err != nil || intValue(cancelled["revision"]) != 2 || stringValue(cancelled["status"]) != "cancelled" {
		t.Fatalf("schedule cancel=%#v err=%v", cancelled, err)
	}
	cancelReplay, err := app.CancelScheduleExpected(ctx, ownerID, fluctlightID, stringValue(first["id"]), cloneMap(cancelPayload))
	if err != nil || !boolValueForTest(cancelReplay["replayed"]) || intValue(cancelReplay["revision"]) != 2 {
		t.Fatalf("schedule cancel replay=%#v err=%v", cancelReplay, err)
	}
	afterCancelLife := currentLifeForTest(t, ctx, app, fluctlightID, time.Now().UTC())
	secondPayload := fullDaySchedulePayloadForTest(now, "owner-schedule-b", stringValue(afterCancelLife["context_revision"]))
	second, err := app.AcceptSchedule(ctx, ownerID, fluctlightID, secondPayload)
	if err != nil {
		t.Fatal(err)
	}
	if intValue(second["revision"]) != 3 {
		t.Fatalf("schedule revision after cancel=%#v, want 3", second)
	}
	var acceptedCount, scheduleCount, commandCount int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FILTER (WHERE status='accepted'),count(*) FROM public.life_schedules WHERE fluctlight_id=$1`, fluctlightID).Scan(&acceptedCount, &scheduleCount); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.life_context_commands WHERE fluctlight_id=$1 AND command_type='schedule.cancel'`, fluctlightID).Scan(&commandCount); err != nil {
		t.Fatal(err)
	}
	if acceptedCount != 1 || scheduleCount != 2 || commandCount != 1 {
		t.Fatalf("schedule authority rows accepted=%d all=%d commands=%d", acceptedCount, scheduleCount, commandCount)
	}
}

func TestScheduleAcceptanceLifeContextCASIncludesEventPresenceAndItemBoundary(t *testing.T) {
	testCases := []struct {
		name   string
		mutate func(*testing.T, context.Context, *App, *PostgresRepository, string, string, map[string]any)
		stale  bool
	}{
		{name: "exact context", mutate: func(*testing.T, context.Context, *App, *PostgresRepository, string, string, map[string]any) {}, stale: false},
		{name: "Event changed", stale: true, mutate: func(t *testing.T, ctx context.Context, app *App, _ *PostgresRepository, ownerID, fluctlightID string, life map[string]any) {
			now := time.Now().UTC()
			if _, err := app.CreateLifeEvent(ctx, ownerID, fluctlightID, map[string]any{
				"kind": "owner_event", "start_at": now.Add(-time.Minute).Format(time.RFC3339), "end_at": now.Add(time.Hour).Format(time.RFC3339),
				"scene": "临时场景", "activity": "临时活动", "evidence_refs": []any{"schedule-cas-event"},
				"expected_life_context_revision": life["context_revision"], "idempotency_key": "schedule-cas-event",
			}); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "Presence changed", stale: true, mutate: func(t *testing.T, ctx context.Context, app *App, _ *PostgresRepository, ownerID, fluctlightID string, life map[string]any) {
			if _, err := app.SetPresence(ctx, ownerID, fluctlightID, map[string]any{
				"current_task": "临时任务", "expected_life_context_revision": life["context_revision"], "idempotency_key": "schedule-cas-presence",
			}); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			ctx, repository := isolatedCoreTestRepository(t)
			ownerID, fluctlightID := "owner-schedule-cas", "fluctlight-schedule-cas"
			seedLifeContextFluctlight(t, ctx, repository, ownerID, fluctlightID)
			app := &App{DB: repository}
			initialLife := currentLifeForTest(t, ctx, app, fluctlightID, time.Now().UTC())
			base := fullDaySchedulePayloadForTest(time.Now().UTC(), "schedule-cas-base", stringValue(initialLife["context_revision"]))
			if _, err := app.AcceptSchedule(ctx, ownerID, fluctlightID, base); err != nil {
				t.Fatal(err)
			}
			frozenLife := currentLifeForTest(t, ctx, app, fluctlightID, time.Now().UTC())
			testCase.mutate(t, ctx, app, repository, ownerID, fluctlightID, frozenLife)
			replacement := fullDaySchedulePayloadForTest(time.Now().UTC(), "schedule-cas-replacement", stringValue(frozenLife["context_revision"]))
			replacement["expected_revision"] = 1
			result, err := app.AcceptSchedule(ctx, ownerID, fluctlightID, replacement)
			if testCase.stale {
				if !errors.Is(err, ErrLifeContextStale) {
					t.Fatalf("stale Schedule result=%#v err=%v", result, err)
				}
				var scheduleCount int
				if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.life_schedules WHERE fluctlight_id=$1`, fluctlightID).Scan(&scheduleCount); err != nil || scheduleCount != 1 {
					t.Fatalf("stale Schedule mutated versions=%d err=%v", scheduleCount, err)
				}
				return
			}
			if err != nil || intValue(result["revision"]) != 2 {
				t.Fatalf("exact Schedule result=%#v err=%v", result, err)
			}
		})
	}

	t.Run("Schedule item boundary changed", func(t *testing.T) {
		ctx, repository := isolatedCoreTestRepository(t)
		ownerID, fluctlightID := "owner-schedule-boundary", "fluctlight-schedule-boundary"
		seedLifeContextFluctlight(t, ctx, repository, ownerID, fluctlightID)
		app := &App{DB: repository}
		now := time.Now().UTC()
		initialLife := currentLifeForTest(t, ctx, app, fluctlightID, now)
		base := fullDaySchedulePayloadForTest(now, "schedule-boundary-base", stringValue(initialLife["context_revision"]))
		location, _ := time.LoadLocation("Asia/Shanghai")
		local := now.In(location)
		dayStart := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location)
		boundary := time.Now().UTC().Add(750 * time.Millisecond)
		base["items"] = []any{
			map[string]any{"start_at": dayStart.Format(time.RFC3339Nano), "end_at": boundary.Format(time.RFC3339Nano), "activity": "边界前", "scene": "书房", "item_type": "planned", "status": "planned", "priority": 0.5, "flexibility": 0.5, "interruption_cost": 0.5},
			map[string]any{"start_at": boundary.Format(time.RFC3339Nano), "end_at": dayStart.AddDate(0, 0, 1).Format(time.RFC3339Nano), "activity": "边界后", "scene": "客厅", "item_type": "planned", "status": "planned", "priority": 0.5, "flexibility": 0.5, "interruption_cost": 0.5},
		}
		if _, err := app.AcceptSchedule(ctx, ownerID, fluctlightID, base); err != nil {
			t.Fatal(err)
		}
		frozenLife := currentLifeForTest(t, ctx, app, fluctlightID, time.Now().UTC())
		if stringValue(frozenLife["activity"]) != "边界前" {
			t.Fatalf("boundary fixture started in wrong item: %#v", frozenLife)
		}
		delay := time.Until(boundary.Add(75 * time.Millisecond))
		if delay > 0 {
			time.Sleep(delay)
		}
		replacement := cloneMap(base)
		replacement["expected_revision"] = 1
		replacement["expected_life_context_revision"] = frozenLife["context_revision"]
		replacement["idempotency_key"] = "schedule-boundary-replacement"
		if _, err := app.AcceptSchedule(ctx, ownerID, fluctlightID, replacement); !errors.Is(err, ErrLifeContextStale) {
			t.Fatalf("Schedule boundary change err=%v", err)
		}
		liveLife := currentLifeForTest(t, ctx, app, fluctlightID, time.Now().UTC())
		if stringValue(liveLife["activity"]) != "边界后" || stringValue(liveLife["context_revision"]) == stringValue(frozenLife["context_revision"]) {
			t.Fatalf("Schedule boundary did not change authority: before=%#v after=%#v", frozenLife, liveLife)
		}
	})
}

func TestCognitionAuthorityCASRejectsFoundationAndCurrentStateDrift(t *testing.T) {
	for _, testCase := range []struct {
		name    string
		mutate  string
		wantErr error
	}{
		{name: "Foundation", mutate: `UPDATE public.fluctlights SET current_revision=current_revision+1 WHERE id=$1`, wantErr: ErrFoundationRevisionStale},
		{name: "Current State", mutate: `UPDATE public.fluctlight_inner_states SET revision=revision+1 WHERE fluctlight_id=$1`, wantErr: ErrCurrentStateRevisionStale},
	} {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			ctx, repository := isolatedCoreTestRepository(t)
			ownerID, fluctlightID := "owner-authority-cas", "fluctlight-authority-cas"
			seedLifeContextFluctlight(t, ctx, repository, ownerID, fluctlightID)
			app := &App{DB: repository}
			projection := lifeProjectionForTest(t, ctx, app, ownerID, fluctlightID, "authority-cas-fact")
			if _, err := repository.Pool().Exec(ctx, testCase.mutate, fluctlightID); err != nil {
				t.Fatal(err)
			}
			tx, err := repository.Pool().Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			err = app.requireCognitionAuthorityRevisionsTx(ctx, tx, fluctlightID, projection.ContextRevision, projection.CurrentStateRevision, projection.LifeContextRevision, time.Now().UTC())
			_ = tx.Rollback(ctx)
			if !errors.Is(err, testCase.wantErr) {
				t.Fatalf("authority CAS err=%v want %v", err, testCase.wantErr)
			}
		})
	}
}

func TestFoundationTimezoneAcceptanceInvalidatesOldScheduleAuthority(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID := "owner-timezone-authority", "fluctlight-timezone-authority"
	seedLifeContextFluctlight(t, ctx, repository, ownerID, fluctlightID)
	app := &App{DB: repository}
	now := time.Now().UTC()
	initialLife := currentLifeForTest(t, ctx, app, fluctlightID, now)
	schedulePayload := fullDaySchedulePayloadForTest(now, "timezone-schedule-old", stringValue(initialLife["context_revision"]))
	accepted, err := app.AcceptSchedule(ctx, ownerID, fluctlightID, schedulePayload)
	if err != nil {
		t.Fatal(err)
	}
	proposed, err := app.ProposeFoundation(ctx, ownerID, fluctlightID, map[string]any{
		"changes": map[string]any{"timezone": "UTC"}, "expected_revision": 0, "reason": "move timezone",
		"evidence_refs": []any{"owner-timezone-fact"},
	})
	if err != nil {
		t.Fatal(err)
	}
	acceptedFoundation, err := app.SetFoundationDecisionExpected(ctx, ownerID, fluctlightID, stringValue(proposed["id"]), "accept", "accept timezone", intPointerForTest(0))
	if err != nil {
		t.Fatal(err)
	}
	replayedFoundation, err := app.SetFoundationDecisionExpected(ctx, ownerID, fluctlightID, stringValue(proposed["id"]), "accept", "accept timezone", intPointerForTest(0))
	if err != nil || !boolValueForTest(replayedFoundation["replayed"]) || stringValue(replayedFoundation["id"]) != stringValue(acceptedFoundation["id"]) {
		t.Fatalf("Foundation accept replay=%#v first=%#v err=%v", replayedFoundation, acceptedFoundation, err)
	}
	var scheduleStatus, timezone string
	if err := repository.Pool().QueryRow(ctx, `SELECT status FROM public.life_schedules WHERE id=$1`, accepted["id"]).Scan(&scheduleStatus); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT identity->>'timezone' FROM public.fluctlights WHERE id=$1`, fluctlightID).Scan(&timezone); err != nil {
		t.Fatal(err)
	}
	var factCount, outboxCount int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.cognition_inbox WHERE fluctlight_id=$1 AND event_type='life.timezone.changed'`, fluctlightID).Scan(&factCount); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.platform_outbox_events WHERE fluctlight_id=$1 AND kind='life.timezone.changed'`, fluctlightID).Scan(&outboxCount); err != nil {
		t.Fatal(err)
	}
	if scheduleStatus != "superseded" || timezone != "UTC" || factCount != 1 || outboxCount != 1 {
		t.Fatalf("timezone cutover schedule=%s timezone=%s facts=%d outbox=%d", scheduleStatus, timezone, factCount, outboxCount)
	}
}

func intPointerForTest(value int) *int { return &value }

func lifeCapabilityRuntimeForTest(t *testing.T, app *App) *CapabilityRuntime {
	t.Helper()
	app.ContextResolver = NewAppContextResolver(app)
	app.Capabilities = app.capabilityRegistry()
	runtime, err := NewCapabilityRuntime(app.Capabilities, app.ContextResolver)
	if err != nil {
		t.Fatal(err)
	}
	app.Runtime = runtime
	return runtime
}

func lifeProjectionForTest(t *testing.T, ctx context.Context, app *App, ownerID, fluctlightID, sourceFactID string) ContextProjection {
	t.Helper()
	projection, err := app.BuildContextProjectionFor(ctx, ContextProjectionRequest{
		AuthorizationActorID: ownerID, SpeakerActorID: ownerID, FluctlightID: fluctlightID,
		SourceFactID: sourceFactID, MemoryOperation: MemoryForNativeCognition, MemoryConversationMode: MemoryConversationGlobalOnly,
	})
	if err != nil {
		t.Fatal(err)
	}
	return projection
}

func prepareLifeCapabilityForTest(t *testing.T, ctx context.Context, app *App, runtime *CapabilityRuntime, projection ContextProjection, capabilityName, callID, actionID, arguments string) CapabilityInvocation {
	t.Helper()
	invocation := CapabilityInvocation{
		CallID: callID, CapabilityName: capabilityName, SchemaVersion: CapabilityInvocationSchemaVersion,
		Arguments: json.RawMessage(arguments), SourceFactID: projection.SourceFactID, ActionID: actionID,
		ProviderRequestID: "provider-" + callID, Metadata: InvocationMetadata{FluctlightID: projection.FluctlightID, Surface: CapabilitySurfaceNativeCognition},
	}
	definition, ok := app.Capabilities.Definition(capabilityName)
	if !ok {
		t.Fatalf("capability %s missing", capabilityName)
	}
	invocation.ContextSnapshot = capabilitySnapshotForProjection(projection, definition.RequiredContext, actionID)
	prepared, _, err := runtime.Prepare(ctx, invocation)
	if err != nil {
		t.Fatal(err)
	}
	return prepared
}

func executeLifeCapabilityForTest(ctx context.Context, repository *PostgresRepository, runtime *CapabilityRuntime, invocation CapabilityInvocation) (CapabilityResult, error) {
	var result CapabilityResult
	err := withTransaction(ctx, repository.Pool(), func(tx pgx.Tx) error {
		var executeErr error
		result, executeErr = runtime.ExecuteTransactional(ctx, tx, invocation)
		return executeErr
	})
	return result, err
}

func TestSceneEndAndStaleSwitchMutateOnlyFrozenEvent(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID := "owner-scene-target", "fluctlight-scene-target"
	seedLifeContextFluctlight(t, ctx, repository, ownerID, fluctlightID)
	app := &App{DB: repository}
	runtime := lifeCapabilityRuntimeForTest(t, app)
	now := time.Now().UTC()
	inferredEnd := now.Add(3 * time.Hour)
	confirmedEnd := now.Add(2 * time.Hour)
	if _, err := repository.Pool().Exec(ctx, `
		INSERT INTO public.life_events(id,fluctlight_id,kind,start_at,end_at,scene,activity,status,revision,evidence_refs,idempotency_key,request_digest,result)
		VALUES
		('scene-target-inferred',$1,'scene_inferred',$2,$3,'阳台','休息','inferred',1,'["fact-inferred"]','scene-target-inferred',$4,'{"event_id":"scene-target-inferred","status":"inferred","event_revision":1,"expected_context_revision":"life_ctx_before","resulting_context_revision":"life_ctx_inferred","replayed":false}'),
		('scene-target-confirmed',$1,'owner_event',$2,$5,'客厅','交谈','confirmed',1,'["fact-confirmed"]','scene-target-confirmed',$6,'{"event_id":"scene-target-confirmed","status":"confirmed","event_revision":1,"expected_context_revision":"life_ctx_before","resulting_context_revision":"life_ctx_confirmed","replayed":false}')`,
		fluctlightID, now.Add(-time.Minute), inferredEnd, stableDigest("scene-target-inferred"), confirmedEnd, stableDigest("scene-target-confirmed")); err != nil {
		t.Fatal(err)
	}
	projection := lifeProjectionForTest(t, ctx, app, ownerID, fluctlightID, "scene-target-end-fact")
	if stringValue(projection.LifeContext["event_id"]) != "scene-target-confirmed" {
		t.Fatalf("confirmed event was not authoritative: %#v", projection.LifeContext)
	}
	preparedEnd := prepareLifeCapabilityForTest(t, ctx, app, runtime, projection, "scene_event", "scene-target-end", "scene-target-end-action", `{"operation":"end","confidence":0.9}`)
	if result, err := executeLifeCapabilityForTest(ctx, repository, runtime, preparedEnd); err != nil || result.Status != "completed" {
		t.Fatalf("scene end result=%#v err=%v", result, err)
	}
	var confirmedRevision, inferredRevision int
	var storedConfirmedEnd, storedInferredEnd time.Time
	if err := repository.Pool().QueryRow(ctx, `SELECT revision,end_at FROM public.life_events WHERE id='scene-target-confirmed'`).Scan(&confirmedRevision, &storedConfirmedEnd); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT revision,end_at FROM public.life_events WHERE id='scene-target-inferred'`).Scan(&inferredRevision, &storedInferredEnd); err != nil {
		t.Fatal(err)
	}
	if confirmedRevision != 2 || !storedConfirmedEnd.Before(confirmedEnd) || inferredRevision != 1 || !storedInferredEnd.Equal(inferredEnd) {
		t.Fatalf("scene end touched wrong target: confirmed=(%d,%s) inferred=(%d,%s)", confirmedRevision, storedConfirmedEnd, inferredRevision, storedInferredEnd)
	}
	afterEnd := lifeProjectionForTest(t, ctx, app, ownerID, fluctlightID, "scene-target-switch-fact")
	if stringValue(afterEnd.LifeContext["event_id"]) != "scene-target-inferred" {
		t.Fatalf("inferred event did not become authoritative: %#v", afterEnd.LifeContext)
	}
	preparedSwitch := prepareLifeCapabilityForTest(t, ctx, app, runtime, afterEnd, "scene_event", "scene-target-stale-switch", "scene-target-stale-action", `{"operation":"switch","scene":"花园","activity":"散步","confidence":0.8}`)
	newConfirmed, err := app.CreateLifeEvent(ctx, ownerID, fluctlightID, map[string]any{
		"kind": "owner_event", "start_at": time.Now().UTC().Add(-time.Minute).Format(time.RFC3339), "end_at": time.Now().UTC().Add(time.Hour).Format(time.RFC3339),
		"scene": "厨房", "activity": "做饭", "evidence_refs": []any{"scene-target-new-fact"},
		"expected_life_context_revision": afterEnd.LifeContextRevision, "idempotency_key": "scene-target-new-confirmed",
	})
	if err != nil {
		t.Fatal(err)
	}
	staleResult, err := executeLifeCapabilityForTest(ctx, repository, runtime, preparedSwitch)
	if err == nil || staleResult.ErrorCode != "scene_context_stale" {
		t.Fatalf("stale switch result=%#v err=%v", staleResult, err)
	}
	var newRevision, oldRevision int
	var newEnd, oldEnd time.Time
	if err := repository.Pool().QueryRow(ctx, `SELECT revision,end_at FROM public.life_events WHERE id=$1`, newConfirmed["id"]).Scan(&newRevision, &newEnd); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT revision,end_at FROM public.life_events WHERE id='scene-target-inferred'`).Scan(&oldRevision, &oldEnd); err != nil {
		t.Fatal(err)
	}
	var staleReplacementCount int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.life_events WHERE fluctlight_id=$1 AND scene='花园'`, fluctlightID).Scan(&staleReplacementCount); err != nil {
		t.Fatal(err)
	}
	if newRevision != 1 || oldRevision != 1 || !oldEnd.Equal(inferredEnd) || staleReplacementCount != 0 || !newEnd.After(time.Now().UTC()) {
		t.Fatalf("stale switch mutated authority: new=(%d,%s) old=(%d,%s) replacements=%d", newRevision, newEnd, oldRevision, oldEnd, staleReplacementCount)
	}
}

func TestConcurrentSceneStartAndPresenceReplacementSerializeOnLifeContext(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID := "owner-life-concurrent", "fluctlight-life-concurrent"
	seedLifeContextFluctlight(t, ctx, repository, ownerID, fluctlightID)
	app := &App{DB: repository}
	runtime := lifeCapabilityRuntimeForTest(t, app)
	sceneProjection := lifeProjectionForTest(t, ctx, app, ownerID, fluctlightID, "scene-concurrent-fact")
	sceneInvocations := []CapabilityInvocation{
		prepareLifeCapabilityForTest(t, ctx, app, runtime, sceneProjection, "scene_event", "scene-concurrent-a", "scene-concurrent-action-a", `{"operation":"start","scene":"书房","activity":"阅读","confidence":0.9}`),
		prepareLifeCapabilityForTest(t, ctx, app, runtime, sceneProjection, "scene_event", "scene-concurrent-b", "scene-concurrent-action-b", `{"operation":"start","scene":"客厅","activity":"交谈","confidence":0.9}`),
	}
	assertConcurrentLifeMutation(t, ctx, repository, runtime, sceneInvocations, "scene_context_stale")
	var activeScenes int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.life_events WHERE fluctlight_id=$1 AND status='inferred' AND start_at<=now() AND end_at>now()`, fluctlightID).Scan(&activeScenes); err != nil || activeScenes != 1 {
		t.Fatalf("concurrent scene starts active=%d err=%v", activeScenes, err)
	}

	presenceProjection := lifeProjectionForTest(t, ctx, app, ownerID, fluctlightID, "presence-concurrent-fact")
	presenceInvocations := []CapabilityInvocation{
		prepareLifeCapabilityForTest(t, ctx, app, runtime, presenceProjection, "presence_event", "presence-concurrent-a", "presence-concurrent-action-a", `{"operation":"set","current_task":"任务A","confidence":0.9}`),
		prepareLifeCapabilityForTest(t, ctx, app, runtime, presenceProjection, "presence_event", "presence-concurrent-b", "presence-concurrent-action-b", `{"operation":"set","current_task":"任务B","confidence":0.9}`),
	}
	assertConcurrentLifeMutation(t, ctx, repository, runtime, presenceInvocations, "presence_context_stale")
	var activePresence int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.life_presence_overlays WHERE fluctlight_id=$1 AND status='active'`, fluctlightID).Scan(&activePresence); err != nil || activePresence != 1 {
		t.Fatalf("concurrent Presence replacements active=%d err=%v", activePresence, err)
	}
}

func assertConcurrentLifeMutation(t *testing.T, ctx context.Context, repository *PostgresRepository, runtime *CapabilityRuntime, invocations []CapabilityInvocation, staleCode string) {
	t.Helper()
	type outcome struct {
		result CapabilityResult
		err    error
	}
	start := make(chan struct{})
	outcomes := make(chan outcome, len(invocations))
	var group sync.WaitGroup
	for _, invocation := range invocations {
		invocation := invocation
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			result, err := executeLifeCapabilityForTest(ctx, repository, runtime, invocation)
			outcomes <- outcome{result: result, err: err}
		}()
	}
	close(start)
	group.Wait()
	close(outcomes)
	completed, stale := 0, 0
	for item := range outcomes {
		if item.err == nil && item.result.Status == "completed" {
			completed++
			continue
		}
		if item.err != nil && item.result.ErrorCode == staleCode {
			stale++
			continue
		}
		t.Fatalf("unexpected concurrent result=%#v err=%v", item.result, item.err)
	}
	if completed != 1 || stale != 1 {
		t.Fatalf("concurrent mutation completed=%d stale=%d", completed, stale)
	}
}

func TestSceneAndPresencePreparedReplayRejectsConflictingPayload(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID := "owner-life-replay", "fluctlight-life-replay"
	seedLifeContextFluctlight(t, ctx, repository, ownerID, fluctlightID)
	app := &App{DB: repository}
	runtime := lifeCapabilityRuntimeForTest(t, app)
	sceneProjection := lifeProjectionForTest(t, ctx, app, ownerID, fluctlightID, "life-replay-scene-fact")
	preparedScene := prepareLifeCapabilityForTest(t, ctx, app, runtime, sceneProjection, "scene_event", "life-replay-scene", "life-replay-scene-action", `{"operation":"start","scene":"书房","activity":"阅读","confidence":0.9}`)
	firstScene, err := executeLifeCapabilityForTest(ctx, repository, runtime, preparedScene)
	if err != nil {
		t.Fatal(err)
	}
	replayedScene, err := executeLifeCapabilityForTest(ctx, repository, runtime, preparedScene)
	if err != nil || !boolValueForTest(mapValue(replayedScene.Output)["replayed"]) || mapValue(replayedScene.Output)["event_id"] != mapValue(firstScene.Output)["event_id"] {
		t.Fatalf("scene replay=%#v first=%#v err=%v", replayedScene, firstScene, err)
	}
	sceneConflictPlan, err := scenePlanFromInvocation(preparedScene)
	if err != nil {
		t.Fatal(err)
	}
	sceneConflictPlan.Scene = "冲突场景"
	sceneConflictPlan.RequestDigest = sceneMutationDigest(sceneConflictPlan)
	conflictingScene, err := withCapabilityPreparedData(preparedScene, "scene_plan", sceneConflictPlan)
	if err != nil {
		t.Fatal(err)
	}
	conflictResult, err := executeLifeCapabilityForTest(ctx, repository, runtime, conflictingScene)
	if err == nil || conflictResult.ErrorCode != "scene_idempotency_conflict" {
		t.Fatalf("scene conflicting replay=%#v err=%v", conflictResult, err)
	}

	presenceProjection := lifeProjectionForTest(t, ctx, app, ownerID, fluctlightID, "life-replay-presence-fact")
	preparedPresence := prepareLifeCapabilityForTest(t, ctx, app, runtime, presenceProjection, "presence_event", "life-replay-presence", "life-replay-presence-action", `{"operation":"set","current_task":"一起阅读","confidence":0.9}`)
	firstPresence, err := executeLifeCapabilityForTest(ctx, repository, runtime, preparedPresence)
	if err != nil {
		t.Fatal(err)
	}
	replayedPresence, err := executeLifeCapabilityForTest(ctx, repository, runtime, preparedPresence)
	if err != nil || !boolValueForTest(mapValue(replayedPresence.Output)["replayed"]) || mapValue(replayedPresence.Output)["overlay_id"] != mapValue(firstPresence.Output)["overlay_id"] {
		t.Fatalf("Presence replay=%#v first=%#v err=%v", replayedPresence, firstPresence, err)
	}
	presenceConflictPlan, err := presencePlanFromInvocation(preparedPresence)
	if err != nil {
		t.Fatal(err)
	}
	presenceConflictPlan.CurrentTask = "冲突任务"
	presenceConflictPlan.RequestDigest = presenceMutationDigest(presenceConflictPlan)
	conflictingPresence, err := withCapabilityPreparedData(preparedPresence, "presence_plan", presenceConflictPlan)
	if err != nil {
		t.Fatal(err)
	}
	conflictResult, err = executeLifeCapabilityForTest(ctx, repository, runtime, conflictingPresence)
	if err == nil || conflictResult.ErrorCode != "presence_idempotency_conflict" {
		t.Fatalf("Presence conflicting replay=%#v err=%v", conflictResult, err)
	}
	var sceneRows, presenceRows, facts, outbox int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.life_events WHERE fluctlight_id=$1`, fluctlightID).Scan(&sceneRows); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.life_presence_overlays WHERE fluctlight_id=$1`, fluctlightID).Scan(&presenceRows); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.cognition_inbox WHERE fluctlight_id=$1 AND event_type IN ('life.scene.updated','life.presence.updated')`, fluctlightID).Scan(&facts); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.platform_outbox_events WHERE fluctlight_id=$1 AND kind IN ('life.scene.updated','life.presence.updated')`, fluctlightID).Scan(&outbox); err != nil {
		t.Fatal(err)
	}
	if sceneRows != 1 || presenceRows != 1 || facts != 2 || outbox != 2 {
		t.Fatalf("prepared replay duplicated effects scenes=%d Presence=%d facts=%d outbox=%d", sceneRows, presenceRows, facts, outbox)
	}
}

func TestExpiredSceneAndPresencePlansCannotCommitHistoricalEffects(t *testing.T) {
	ctx, repository := isolatedCoreTestRepository(t)
	ownerID, fluctlightID := "owner-life-expired-plan", "fluctlight-life-expired-plan"
	seedLifeContextFluctlight(t, ctx, repository, ownerID, fluctlightID)
	app := &App{DB: repository}
	runtime := lifeCapabilityRuntimeForTest(t, app)
	projection := lifeProjectionForTest(t, ctx, app, ownerID, fluctlightID, "expired-plan-fact")
	preparedScene := prepareLifeCapabilityForTest(t, ctx, app, runtime, projection, "scene_event", "expired-scene", "expired-scene-action", `{"operation":"start","scene":"过期场景","activity":"阅读","confidence":0.9}`)
	scenePlan, err := scenePlanFromInvocation(preparedScene)
	if err != nil {
		t.Fatal(err)
	}
	scenePlan.OccurredAt = time.Now().UTC().Add(-3 * time.Hour)
	scenePlan.EndsAt = scenePlan.OccurredAt.Add(sceneDefaultDuration)
	scenePlan.RequestDigest = sceneMutationDigest(scenePlan)
	preparedScene, err = withCapabilityPreparedData(preparedScene, "scene_plan", scenePlan)
	if err != nil {
		t.Fatal(err)
	}
	sceneResult, err := executeLifeCapabilityForTest(ctx, repository, runtime, preparedScene)
	if err == nil || sceneResult.ErrorCode != "scene_plan_expired" {
		t.Fatalf("expired Scene result=%#v err=%v", sceneResult, err)
	}

	preparedPresence := prepareLifeCapabilityForTest(t, ctx, app, runtime, projection, "presence_event", "expired-presence", "expired-presence-action", `{"operation":"set","current_task":"过期任务","confidence":0.9}`)
	presencePlan, err := presencePlanFromInvocation(preparedPresence)
	if err != nil {
		t.Fatal(err)
	}
	presencePlan.OccurredAt = time.Now().UTC().Add(-3 * time.Hour)
	expiredAt := presencePlan.OccurredAt.Add(presenceDefaultDuration)
	presencePlan.ExpiresAt = &expiredAt
	presencePlan.RequestDigest = presenceMutationDigest(presencePlan)
	preparedPresence, err = withCapabilityPreparedData(preparedPresence, "presence_plan", presencePlan)
	if err != nil {
		t.Fatal(err)
	}
	presenceResult, err := executeLifeCapabilityForTest(ctx, repository, runtime, preparedPresence)
	if err == nil || presenceResult.ErrorCode != "presence_plan_expired" {
		t.Fatalf("expired Presence result=%#v err=%v", presenceResult, err)
	}
	var eventCount, presenceCount int
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.life_events WHERE fluctlight_id=$1`, fluctlightID).Scan(&eventCount); err != nil {
		t.Fatal(err)
	}
	if err := repository.Pool().QueryRow(ctx, `SELECT count(*) FROM public.life_presence_overlays WHERE fluctlight_id=$1`, fluctlightID).Scan(&presenceCount); err != nil {
		t.Fatal(err)
	}
	if eventCount != 0 || presenceCount != 0 {
		t.Fatalf("expired plans committed effects events=%d Presence=%d", eventCount, presenceCount)
	}
}
