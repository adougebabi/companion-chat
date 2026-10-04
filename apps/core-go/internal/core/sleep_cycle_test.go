package core

import (
	"net/http"
	"testing"
	"time"
)

func TestTwentySleepingChecksDoNotWakePublishShopOrCreateReflectionEvidence(t *testing.T) {
	fixture := seedWardrobeToolFixture(t)
	at := time.Now().UTC().Truncate(time.Millisecond)
	fixture.app.Clock = func() time.Time { return at }
	life := currentLifeForTest(t, fixture.ctx, fixture.app, fixture.fluctlightID, at)
	payload := fullDaySchedulePayloadForTest(at, "sleep-cycle-baseline", stringValue(life["context_revision"]))
	item := mapValue(arrayValue(payload["items"])[0])
	item["item_type"] = "sleep"
	item["activity"] = "睡眠"
	item["scene"] = "卧室"
	if _, err := fixture.app.AcceptSchedule(fixture.ctx, fixture.ownerID, fixture.fluctlightID, payload); err != nil {
		t.Fatal(err)
	}
	router := newFakeProviderRouter()
	fixture.app.Provider.HTTP = &http.Client{Transport: router}
	var factsBefore, reflectionBefore int
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT (SELECT count(*) FROM public.cognition_inbox WHERE fluctlight_id=$1),(SELECT count(*) FROM public.platform_workflow_intents WHERE intent_type='reflection.run' AND payload->>'fluctlight_id'=$1)`, fixture.fluctlightID).Scan(&factsBefore, &reflectionBefore); err != nil {
		t.Fatal(err)
	}
	for cycle := 1; cycle <= 20; cycle++ {
		wake, err := fixture.app.ProcessWakeUp(fixture.ctx, fixture.fluctlightID, cycle)
		if err != nil || wake["reason"] != "sleeping" || wake["status"] != "no_op" {
			t.Fatalf("cycle=%d result=%#v err=%v", cycle, wake, err)
		}
	}
	var factsAfter, reflectionAfter, visible, activities int
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT (SELECT count(*) FROM public.cognition_inbox WHERE fluctlight_id=$1),(SELECT count(*) FROM public.platform_workflow_intents WHERE intent_type='reflection.run' AND payload->>'fluctlight_id'=$1),(SELECT count(*) FROM public.conversation_messages WHERE author_actor_id=$1 AND kind='assistant'),(SELECT count(*) FROM public.fluctlight_life_activity_runs WHERE fluctlight_id=$1)`, fixture.fluctlightID).Scan(&factsAfter, &reflectionAfter, &visible, &activities); err != nil {
		t.Fatal(err)
	}
	if factsAfter != factsBefore || reflectionAfter != reflectionBefore || visible != 0 || activities != 0 {
		t.Fatalf("facts %d→%d reflection %d→%d visible=%d activities=%d", factsBefore, factsAfter, reflectionBefore, reflectionAfter, visible, activities)
	}
	denied, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(lifeActivityStartCapabilityName, "sleep-shopping", map[string]any{"kind": "virtual_shopping", "duration_minutes": 15, "category": "boots", "slot": "shoes", "description": "黑色短靴", "reason": "买靴子"}))
	if err == nil || denied.Result.ErrorCode != "life_state_sleeping" {
		t.Fatalf("independent shopping bypassed sleep: %#v %v", denied, err)
	}
	projection, err := fixture.app.BuildContextProjectionFor(fixture.ctx, ContextProjectionRequest{AuthorizationActorID: fixture.ownerID, TargetActorID: fixture.ownerID, TriggerSource: "periodic_check", FluctlightID: fixture.fluctlightID, ConversationID: fixture.conversationID, MemoryOperation: MemoryForWakeUp, MemoryConversationMode: MemoryConversationExact})
	if err != nil || len(projection.CurrentSpeaker) != 0 || projection.ReferenceIndex.SpeakerActorID != "" || projection.LifeContext["behavior_state"] != "sleep" {
		t.Fatalf("system event invented a user utterance: %#v %v", projection.CurrentSpeaker, err)
	}
}

func TestConfirmedWakeEventBecomesEffectiveOnlyWhenBusinessClockIsDue(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	at := time.Date(2026, 10, 6, 2, 0, 0, 0, time.UTC)
	f.app.Clock = func() time.Time { return at }
	life := currentLifeForTest(t, f.ctx, f.app, f.fluctlightID, at)
	if _, err := f.app.AcceptSchedule(f.ctx, f.ownerID, f.fluctlightID, fullDaySchedulePayloadForTest(at, "legal-wake-baseline", stringValue(life["context_revision"]))); err != nil {
		t.Fatal(err)
	}
	life = currentLifeForTest(t, f.ctx, f.app, f.fluctlightID, at)
	wakeAt := at.Add(10 * time.Minute)
	if _, err := f.app.CreateLifeEvent(f.ctx, f.ownerID, f.fluctlightID, map[string]any{"kind": "sleep", "start_at": formatInstant(at), "end_at": formatInstant(wakeAt), "scene": "卧室", "activity": "睡眠", "evidence_refs": []any{"owner:confirmed-sleep"}, "idempotency_key": "legal-sleep", "expected_life_context_revision": life["context_revision"]}); err != nil {
		t.Fatal(err)
	}
	life = currentLifeForTest(t, f.ctx, f.app, f.fluctlightID, at)
	wake, err := f.app.CreateLifeEvent(f.ctx, f.ownerID, f.fluctlightID, map[string]any{"kind": "wake", "start_at": formatInstant(wakeAt), "end_at": formatInstant(wakeAt.Add(time.Hour)), "scene": "卧室", "activity": "合法起床", "evidence_refs": []any{"owner:confirmed-wake"}, "idempotency_key": "legal-wake", "expected_life_context_revision": life["context_revision"]})
	if err != nil {
		t.Fatal(err)
	}
	router := newFakeProviderRouter().on("wake_up_response", func(map[string]any) fakeProviderResult {
		return fakeProviderResult{Structured: map[string]any{"action_type": "no_op", "response_intent": "", "evidence_refs": []any{}, "influences": []any{}}}
	})
	f.app.Provider.HTTP = &http.Client{Transport: router}
	seedCognitiveProviderRole(t, f.ctx, f.repository, "legal-wake-provider-"+f.suffix)
	if result, err := f.app.ProcessWakeUp(f.ctx, f.fluctlightID, 1); err != nil || result["reason"] != "sleeping" || router.totalRequests() != 0 {
		t.Fatalf("early wake %#v %v", result, err)
	}
	at = wakeAt
	if _, err := f.app.ProcessWakeUp(f.ctx, f.fluctlightID, 2); err != nil {
		t.Fatal(err)
	}
	life = currentLifeForTest(t, f.ctx, f.app, f.fluctlightID, at)
	if life["behavior_state"] != "awake" || life["event_id"] != wake["id"] || router.requestCount("wake_up_response") != 1 {
		t.Fatalf("legal wake did not resolve before policy %#v calls=%d", life, router.totalRequests())
	}
	var count int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.life_events WHERE fluctlight_id=$1 AND kind='wake'`, f.fluctlightID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("periodic check manufactured another wake %d %v", count, err)
	}
}
