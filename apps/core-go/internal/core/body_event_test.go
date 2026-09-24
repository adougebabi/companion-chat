package core

import (
	"strings"
	"testing"
	"time"
)

func TestConfirmedBodyEventPersistsInjuryAcrossLifeContextAndRequiresRecovery(t *testing.T) {
	fixture := seedWardrobeToolFixture(t)
	now := time.Now().UTC()
	life := currentLifeForTest(t, fixture.ctx, fixture.app, fixture.fluctlightID, now)
	injuryPayload := map[string]any{
		"kind": "body_injury", "start_at": now.Add(-time.Minute).Format(time.RFC3339Nano), "end_at": now.Add(time.Minute).Format(time.RFC3339Nano),
		"activity": "脚踝扭伤", "evidence_refs": []any{"owner:accepted-virtual-event"}, "idempotency_key": "injury-" + fixture.suffix,
		"expected_life_context_revision": life["context_revision"],
		"body_effect":                    map[string]any{"body_part": "ankle", "description": "脚踝扭伤", "severity": "moderate", "impact": "mobility_limited"},
	}
	created, err := fixture.app.CreateLifeEvent(fixture.ctx, fixture.ownerID, fixture.fluctlightID, injuryPayload)
	if err != nil {
		t.Fatalf("confirmed body event rejected: %v", err)
	}
	injuryID := stringValue(created["id"])
	if injuryID == "" || intValue(created["body_revision"]) != 1 {
		t.Fatalf("body event lacked state result: %#v", created)
	}
	appearance, _, _, err := fixture.app.readEffectiveLifeSnapshot(fixture.ctx, fixture.fluctlightID, now.Add(24*time.Hour))
	if err != nil || !strings.Contains(jsonString(appearance), "mobility_limited") {
		t.Fatalf("injury disappeared across the day: %#v err=%v", appearance, err)
	}
	if _, err := fixture.app.CancelLifeEvent(fixture.ctx, fixture.ownerID, fixture.fluctlightID, injuryID, map[string]any{
		"expected_event_revision": 1, "expected_life_context_revision": created["resulting_context_revision"], "idempotency_key": "cancel-injury-" + fixture.suffix,
	}); err == nil || !strings.Contains(err.Error(), "state_event_requires_followup") {
		t.Fatalf("body event was silently cancelled after affecting body: %v", err)
	}
	life = currentLifeForTest(t, fixture.ctx, fixture.app, fixture.fluctlightID, time.Now().UTC())
	recoveryPayload := map[string]any{
		"kind": "body_recovery", "start_at": now.Add(-time.Second).Format(time.RFC3339Nano), "end_at": now.Add(time.Minute).Format(time.RFC3339Nano),
		"activity": "恢复脚踝活动", "evidence_refs": []any{"owner:accepted-recovery-event"}, "idempotency_key": "recovery-" + fixture.suffix,
		"expected_life_context_revision": life["context_revision"], "body_effect": map[string]any{"injury_event_id": injuryID},
	}
	recovered, err := fixture.app.CreateLifeEvent(fixture.ctx, fixture.ownerID, fixture.fluctlightID, recoveryPayload)
	if err != nil || intValue(recovered["body_revision"]) != 2 {
		t.Fatalf("body recovery not applied: event=%#v err=%v", recovered, err)
	}
	appearance, _, _, err = fixture.app.readEffectiveLifeSnapshot(fixture.ctx, fixture.fluctlightID, now.Add(48*time.Hour))
	if err != nil || !strings.Contains(jsonString(appearance), "recovered") {
		t.Fatalf("recovery did not update durable body state: %#v err=%v", appearance, err)
	}
}
