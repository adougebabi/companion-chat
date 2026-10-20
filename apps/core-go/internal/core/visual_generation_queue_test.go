package core

import (
	"sync"
	"testing"
)

func visualGenerationPath(fluctlightID string) string {
	return "/internal/fluctlights/" + fluctlightID + "/visual-identity/generate"
}

func visualGenerationBody(key string) map[string]any {
	return map[string]any{
		"expected_visual_revision": 0,
		"expected_brief_revision":  0,
		"reason":                   "queue visual generation",
		"idempotency_key":          key,
	}
}

func dispatchVisualGeneration(t *testing.T, fixture visualIdentityToolFixture, key string) map[string]any {
	t.Helper()
	result, err := fixture.app.DispatchOwnerSurface(fixture.ctx, "POST", visualGenerationPath(fixture.fluctlightID), nil, fixture.ownerID, visualGenerationBody(key))
	if err != nil {
		t.Fatal(err)
	}
	if intValue(result["_status"]) != 202 {
		t.Fatalf("generation status = %#v, want 202; body=%#v", result["_status"], result)
	}
	request := mapValue(result["request"])
	if stringValue(request["id"]) == "" || stringValue(request["status"]) != stringValue(result["status"]) {
		t.Fatalf("generation request = %#v", result)
	}
	if status := stringValue(result["status"]); status != "queued" && status != "pending" {
		t.Fatalf("generation status %q", status)
	}
	return result
}

func visualRequestState(t *testing.T, fixture visualIdentityToolFixture, requestID string) (status, sessionID, sessionStatus string, position int64) {
	t.Helper()
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT r.status,COALESCE(r.session_id,''),COALESCE(s.status,''),r.queue_position FROM public.fluctlight_visual_generation_requests r LEFT JOIN public.fluctlight_visual_identity_sessions s ON s.id=r.session_id WHERE r.id=$1`, requestID).Scan(&status, &sessionID, &sessionStatus, &position); err != nil {
		t.Fatal(err)
	}
	return status, sessionID, sessionStatus, position
}

func visualInitializeIntentCount(t *testing.T, fixture visualIdentityToolFixture, sessionID string) int {
	t.Helper()
	var count int
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.platform_workflow_intents WHERE intent_type='visual_identity.initialize' AND payload->>'session_id'=$1`, sessionID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func TestVisualGenerationConcurrentRequestsEnqueue(t *testing.T) {
	fixture := newVisualIdentityToolFixture(t)
	var fixtureStatus string
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT status FROM public.fluctlight_visual_identity_sessions WHERE id=$1`, fixture.sessionID).Scan(&fixtureStatus); err != nil || fixtureStatus != "queued" {
		t.Fatalf("fixture session status = %s err=%v", fixtureStatus, err)
	}
	results := make([]map[string]any, 2)
	errs := make([]error, 2)
	var group sync.WaitGroup
	for index := 0; index < 2; index++ {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			key := "concurrent-a"
			if index == 1 {
				key = "concurrent-b"
			}
			results[index], errs[index] = fixture.app.DispatchOwnerSurface(fixture.ctx, "POST", visualGenerationPath(fixture.fluctlightID), nil, fixture.ownerID, visualGenerationBody(key))
		}(index)
	}
	group.Wait()
	ids := map[string]struct{}{}
	sessions := map[string]struct{}{}
	for index, result := range results {
		if errs[index] != nil {
			t.Fatal(errs[index])
		}
		if intValue(result["_status"]) != 202 || stringValue(result["status"]) != "pending" {
			t.Fatalf("concurrent generation = %#v", result)
		}
		request := mapValue(result["request"])
		if stringValue(request["status"]) != "pending" || stringValue(request["id"]) == "" || stringValue(request["session_id"]) == "" || stringValue(request["session_id"]) != stringValue(result["session_id"]) {
			t.Fatalf("concurrent request = %#v", result)
		}
		ids[stringValue(request["id"])] = struct{}{}
		sessions[stringValue(result["session_id"])] = struct{}{}
		if visualInitializeIntentCount(t, fixture, stringValue(result["session_id"])) != 0 {
			t.Fatalf("queued-behind request started an intent: %#v", result)
		}
	}
	if len(ids) != 2 || len(sessions) != 2 {
		t.Fatalf("concurrent requests were not distinct: %#v", results)
	}
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT status FROM public.fluctlight_visual_identity_sessions WHERE id=$1`, fixture.sessionID).Scan(&fixtureStatus); err != nil || fixtureStatus != "queued" {
		t.Fatalf("in-progress session changed to %s err=%v", fixtureStatus, err)
	}
	var stored int
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.fluctlight_visual_generation_requests WHERE fluctlight_id=$1`, fixture.fluctlightID).Scan(&stored); err != nil || stored != 2 {
		t.Fatalf("stored requests = %d err=%v", stored, err)
	}
}

func TestVisualGenerationQueueRunsInOrderAndWaitsWhilePaused(t *testing.T) {
	fixture := newVisualIdentityToolFixture(t)
	first := dispatchVisualGeneration(t, fixture, "order-a")
	second := dispatchVisualGeneration(t, fixture, "order-b")
	firstID := stringValue(mapValue(first["request"])["id"])
	secondID := stringValue(mapValue(second["request"])["id"])
	firstStatus, _, _, firstPosition := visualRequestState(t, fixture, firstID)
	secondStatus, _, _, secondPosition := visualRequestState(t, fixture, secondID)
	if firstStatus != "pending" || secondStatus != "pending" || firstPosition >= secondPosition {
		t.Fatalf("enqueue order %s@%d then %s@%d", firstStatus, firstPosition, secondStatus, secondPosition)
	}
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `UPDATE public.fluctlight_visual_identity_sessions SET status='completed' WHERE id=$1`, fixture.sessionID); err != nil {
		t.Fatal(err)
	}
	if err := fixture.app.ReleaseVisualGenerationQueue(fixture.ctx, fixture.fluctlightID); err != nil {
		t.Fatal(err)
	}
	status, sessionID, sessionStatus, _ := visualRequestState(t, fixture, firstID)
	if status != "queued" || sessionStatus != "queued" || visualInitializeIntentCount(t, fixture, sessionID) != 1 {
		t.Fatalf("first request did not start: status=%s session=%s intents=%d", status, sessionStatus, visualInitializeIntentCount(t, fixture, sessionID))
	}
	status, sessionID, sessionStatus, _ = visualRequestState(t, fixture, secondID)
	if status != "pending" || sessionStatus != "pending" || visualInitializeIntentCount(t, fixture, sessionID) != 0 {
		t.Fatalf("second request started early: status=%s session=%s", status, sessionStatus)
	}
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `UPDATE public.fluctlight_visual_identity_sessions SET status='completed' WHERE id=$1 AND fluctlight_id=$2`, stringValue(mapValue(first["request"])["session_id"]), fixture.fluctlightID); err != nil {
		t.Fatal(err)
	}
	if err := fixture.app.ReleaseVisualGenerationQueue(fixture.ctx, fixture.fluctlightID); err != nil {
		t.Fatal(err)
	}
	status, sessionID, sessionStatus, _ = visualRequestState(t, fixture, secondID)
	if status != "queued" || sessionStatus != "queued" || visualInitializeIntentCount(t, fixture, sessionID) != 1 {
		t.Fatalf("second request did not start after the first: status=%s session=%s intents=%d", status, sessionStatus, visualInitializeIntentCount(t, fixture, sessionID))
	}
	third := dispatchVisualGeneration(t, fixture, "order-c")
	thirdID := stringValue(mapValue(third["request"])["id"])
	if stringValue(third["status"]) != "pending" {
		t.Fatalf("request behind a running session = %#v", third)
	}
	revision := 0
	if _, err := fixture.app.SetFluctlightStatus(fixture.ctx, fixture.ownerID, fixture.fluctlightID, "paused", "stop visual queue", &revision); err != nil {
		t.Fatal(err)
	}
	var runningStatus string
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT status FROM public.fluctlight_visual_identity_sessions WHERE id=$1`, stringValue(mapValue(second["request"])["session_id"])).Scan(&runningStatus); err != nil || runningStatus != "cancelled" {
		t.Fatalf("paused in-flight session = %s err=%v", runningStatus, err)
	}
	if err := fixture.app.ReleaseVisualGenerationQueue(fixture.ctx, fixture.fluctlightID); err != nil {
		t.Fatal(err)
	}
	status, sessionID, sessionStatus, _ = visualRequestState(t, fixture, thirdID)
	if status != "pending" || sessionStatus != "pending" || visualInitializeIntentCount(t, fixture, sessionID) != 0 {
		t.Fatalf("paused queue started: status=%s session=%s intents=%d", status, sessionStatus, visualInitializeIntentCount(t, fixture, sessionID))
	}
	if _, err := fixture.app.SetFluctlightStatus(fixture.ctx, fixture.ownerID, fixture.fluctlightID, "active", "resume visual queue", &revision); err != nil {
		t.Fatal(err)
	}
	status, sessionID, sessionStatus, _ = visualRequestState(t, fixture, thirdID)
	if status != "queued" || sessionStatus != "queued" || visualInitializeIntentCount(t, fixture, sessionID) != 1 {
		t.Fatalf("unpause did not start the waiting request: status=%s session=%s intents=%d", status, sessionStatus, visualInitializeIntentCount(t, fixture, sessionID))
	}
}

func TestVisualGenerationIdempotentReplayDoesNotEnqueueTwice(t *testing.T) {
	fixture := newVisualIdentityToolFixture(t)
	key := "replay-visual"
	first := dispatchVisualGeneration(t, fixture, key)
	requestID := stringValue(mapValue(first["request"])["id"])
	replay, err := fixture.app.DispatchOwnerSurface(fixture.ctx, "POST", visualGenerationPath(fixture.fluctlightID), nil, fixture.ownerID, visualGenerationBody(key))
	if err != nil {
		t.Fatal(err)
	}
	if replay["_replay"] != true || stringValue(mapValue(replay["request"])["id"]) != requestID || stringValue(replay["status"]) != stringValue(first["status"]) {
		t.Fatalf("surface replay = %#v want request %s", replay, requestID)
	}
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `DELETE FROM public.owner_surface_idempotency WHERE owner_actor_id=$1 AND idempotency_key=$2`, fixture.ownerID, key); err != nil {
		t.Fatal(err)
	}
	again := dispatchVisualGeneration(t, fixture, key)
	if again["_replay"] == true || stringValue(mapValue(again["request"])["id"]) != requestID {
		t.Fatalf("domain replay = %#v want the original request %s", again, requestID)
	}
	var requests, sessions int
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.fluctlight_visual_generation_requests WHERE fluctlight_id=$1 AND idempotency_key=$2`, fixture.fluctlightID, key).Scan(&requests); err != nil || requests != 1 {
		t.Fatalf("request rows = %d err=%v", requests, err)
	}
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.fluctlight_visual_identity_sessions WHERE id=$1`, stringValue(first["session_id"])).Scan(&sessions); err != nil || sessions != 1 {
		t.Fatalf("session rows = %d err=%v", sessions, err)
	}
}
