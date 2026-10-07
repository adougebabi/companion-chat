package core

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestVirtualActivityModelInputOmitsStoredIDsAndRevisions(t *testing.T) {
	input := VirtualActivityResultTaskInput{
		Kind: "haircut", Request: map[string]any{"activity_id": "activity-private", "desired_hair_length": "短发", "description": "剪短头发", "revision": 3},
		CurrentAppearance: map[string]any{"body_revision": 4, "hair": "长发"},
		CurrentLife:       map[string]any{"scene": "理发店", "context_revision": "life-private"},
		RecentOutcomes:    []map[string]any{{"id": "outcome-private", "status": "completed", "observed": map[string]any{"hair_length": "长发"}}},
	}
	encoded := jsonString(virtualActivityModelInput(input))
	for _, forbidden := range []string{"activity-private", "body_revision", "life-private", "outcome-private", "\"revision\""} {
		if strings.Contains(encoded, forbidden) {
			t.Fatalf("virtual activity input retained %q: %s", forbidden, encoded)
		}
	}
	for _, necessary := range []string{"短发", "剪短头发", "长发", "理发店", "completed"} {
		if !strings.Contains(encoded, necessary) {
			t.Fatalf("virtual activity input lost %q: %s", necessary, encoded)
		}
	}
}

func TestLifeActivityAdvanceSchemaAllowsUniqueTargetInference(t *testing.T) {
	advance := lifeActivityAdvanceDefinition()
	if err := advance.Validate(); err != nil {
		t.Fatal(err)
	}
	withoutID := CapabilityInvocation{CallID: "advance-schema", CapabilityName: advance.Name, Arguments: jsonBytes(map[string]any{}), Metadata: InvocationMetadata{Source: "direct"}}
	if err := withoutID.Validate(advance); err != nil {
		t.Fatalf("unique active activity should allow an empty model argument object: %v", err)
	}
}

func setupVirtualActivityTestProvider(t *testing.T, fixture independentToolE2EFixture, result map[string]any) *fakeProviderRouter {
	t.Helper()
	router := newFakeProviderRouter().on("virtual_activity_result", func(_ map[string]any) fakeProviderResult {
		return fakeProviderResult{Structured: result}
	})
	fixture.app.Provider.HTTP = &http.Client{Transport: router}
	seedCognitiveProviderRole(t, fixture.ctx, fixture.repository, "activity-endpoint-"+fixture.suffix)
	return router
}

func forceVirtualActivityDue(t *testing.T, fixture independentToolE2EFixture, activityID string) {
	t.Helper()
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `UPDATE public.fluctlight_life_activity_runs SET started_at=now()-interval '20 minutes',not_before=now()-interval '1 minute' WHERE id=$1 AND fluctlight_id=$2`, activityID, fixture.fluctlightID); err != nil {
		t.Fatal(err)
	}
}

func TestVirtualShoppingCanEndWithoutAcquiringAnItem(t *testing.T) {
	fixture := seedWardrobeToolFixture(t)
	intentionID := createQualifiedBootIntention(t, fixture, "window-shopping")
	started, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(lifeActivityStartCapabilityName, "window-shopping", map[string]any{
		"kind": "virtual_shopping", "intention_id": intentionID, "duration_minutes": 15, "category": "boots", "slot": "shoes", "description": "黑色短靴",
		"scene": "商场", "activity": "逛服装店", "location": "商场二层", "reason": "看看衣服和鞋子",
	}))
	if err != nil || started.Result.Status != "accepted" {
		t.Fatalf("start shopping: receipt=%#v err=%v", started, err)
	}
	activityID := stringValue(mapValue(started.Result.Output)["activity_id"])
	eventID := stringValue(mapValue(started.Result.Output)["event_id"])
	_, life, err := fixture.app.readLifeContextSnapshotAt(fixture.ctx, fixture.fluctlightID, time.Now().UTC())
	if err != nil || stringValue(life["event_id"]) != eventID || stringValue(life["scene"]) != "商场" || stringValue(life["activity"]) != "逛服装店" {
		t.Fatalf("activity and scene did not start together: life=%#v err=%v", life, err)
	}
	forceVirtualActivityDue(t, fixture, activityID)
	router := setupVirtualActivityTestProvider(t, fixture, map[string]any{"status": "completed", "reason": "逛完后决定暂时不买"})
	resolved, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(lifeActivityAdvanceCapabilityName, "window-shopping-end", map[string]any{"activity_id": activityID}))
	if err != nil || resolved.Result.Status != "completed" || router.requestCount("virtual_activity_result") != 1 || stringValue(mapValue(resolved.Result.Output)["item_id"]) != "" {
		t.Fatalf("shopping without acquisition should simply end: receipt=%#v err=%v", resolved, err)
	}
	var items int
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND source_kind='purchase_result'`, fixture.fluctlightID).Scan(&items); err != nil || items != 0 {
		t.Fatalf("unconfirmed acquisition changed wardrobe: items=%d err=%v", items, err)
	}
	var intentionStatus string
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT status FROM public.fluctlight_intentions WHERE id=$1`, intentionID).Scan(&intentionStatus); err != nil || intentionStatus == string(IntentionCompleted) {
		t.Fatalf("shopping without acquisition completed purchase intention: status=%s err=%v", intentionStatus, err)
	}
	_, life, err = fixture.app.readLifeContextSnapshotAt(fixture.ctx, fixture.fluctlightID, time.Now().UTC())
	if err != nil || stringValue(life["event_id"]) == eventID {
		t.Fatalf("resolved activity still current: life=%#v err=%v", life, err)
	}
}

func TestLifeActivityEventExpiryEndsRunWithoutAResultEffect(t *testing.T) {
	fixture := seedWardrobeToolFixture(t)
	started, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(lifeActivityStartCapabilityName, "expired-shopping", map[string]any{
		"kind": "virtual_shopping", "duration_minutes": 15, "category": "boots", "slot": "shoes", "description": "黑色短靴",
		"scene": "商场", "activity": "逛店", "reason": "临时出门",
	}))
	if err != nil || started.Result.Status != "accepted" {
		t.Fatalf("start activity: receipt=%#v err=%v", started, err)
	}
	activityID := stringValue(mapValue(started.Result.Output)["activity_id"])
	eventID := stringValue(mapValue(started.Result.Output)["event_id"])
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `UPDATE public.life_events SET start_at=now()-interval '20 minutes',end_at=now()-interval '1 minute',expires_at=now()-interval '1 minute' WHERE id=$1`, eventID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `UPDATE public.fluctlight_life_activity_runs SET started_at=now()-interval '20 minutes',not_before=now()-interval '1 minute',active_until=now()-interval '1 minute' WHERE id=$1`, activityID); err != nil {
		t.Fatal(err)
	}
	activities, err := fixture.app.readActiveLifeActivities(fixture.ctx, fixture.fluctlightID, time.Now().UTC())
	if err != nil || len(activities) != 0 {
		t.Fatalf("expired activity still projected as current: activities=%#v err=%v", activities, err)
	}
	var status string
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT status FROM public.fluctlight_life_activity_runs WHERE id=$1`, activityID).Scan(&status); err != nil || status != "cancelled" {
		t.Fatalf("expired run lacked durable terminal state: status=%q err=%v", status, err)
	}
	router := setupVirtualActivityTestProvider(t, fixture, map[string]any{"status": "completed", "reason": "迟到结果", "acquired_item": map[string]any{"category": "boots", "slot": "shoes", "description": "黑色短靴"}})
	_, _ = fixture.app.ExecuteTool(fixture.ctx, fixture.request(lifeActivityAdvanceCapabilityName, "late-shopping-result", map[string]any{"activity_id": activityID}))
	if router.requestCount("virtual_activity_result") != 0 {
		t.Fatal("late activity invoked result Provider")
	}
	var items int
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND source_kind='purchase_result'`, fixture.fluctlightID).Scan(&items); err != nil || items != 0 {
		t.Fatalf("late result changed wardrobe: items=%d err=%v", items, err)
	}
}

func TestExplicitLifeActivityExtensionMovesEventAndRunBoundaryTogether(t *testing.T) {
	fixture := seedWardrobeToolFixture(t)
	started, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(lifeActivityStartCapabilityName, "extend-activity", map[string]any{
		"kind": "virtual_shopping", "duration_minutes": 15, "category": "boots", "slot": "shoes", "description": "黑色短靴", "reason": "决定逛店",
	}))
	if err != nil || started.Result.Status != "accepted" {
		t.Fatalf("start activity: %#v %v", started, err)
	}
	activityID := stringValue(mapValue(started.Result.Output)["activity_id"])
	var oldUntil time.Time
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT active_until FROM public.fluctlight_life_activity_runs WHERE id=$1`, activityID).Scan(&oldUntil); err != nil {
		t.Fatal(err)
	}
	extended, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(lifeActivityAdvanceCapabilityName, "extend-activity-30", map[string]any{
		"activity_id": activityID, "extend_minutes": 30, "reason": "决定继续逛半小时",
	}))
	if err != nil || extended.Result.Status != "completed" || stringValue(mapValue(extended.Result.Output)["status"]) != "extended" {
		t.Fatalf("explicit extension failed: %#v %v", extended, err)
	}
	var eventUntil, runUntil time.Time
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT e.end_at,r.active_until FROM public.fluctlight_life_activity_runs r JOIN public.life_events e ON e.id=r.authority_event_id WHERE r.id=$1`, activityID).Scan(&eventUntil, &runUntil); err != nil {
		t.Fatal(err)
	}
	if !runUntil.After(oldUntil) || !runUntil.Equal(eventUntil) {
		t.Fatalf("Event and run extension diverged: old=%s event=%s run=%s", oldUntil, eventUntil, runUntil)
	}
}

func createQualifiedBootIntention(t *testing.T, fixture independentToolE2EFixture, label string) string {
	t.Helper()
	created, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(intentionDecideCapabilityName, label+"-create", map[string]any{
		"operation": "create", "goal": "拥有合适的短靴", "action": "虚拟购物购买短靴", "expected_outcome": "短靴实际入柜", "reason": "明确想穿短靴",
	}))
	if err != nil {
		t.Fatal(err)
	}
	id := stringValue(mapValue(created.Result.Output)["intention_id"])
	if _, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(intentionDecideCapabilityName, label+"-qualify", map[string]any{
		"operation": "qualify", "intention_id": id, "reason": "决定在合适时机购物",
	})); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestVirtualShoppingActivityRequiresElapsedResultAndReusesPurchasedItem(t *testing.T) {
	fixture := seedWardrobeToolFixture(t)
	intentionID := createQualifiedBootIntention(t, fixture, "boots-success")
	started, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(lifeActivityStartCapabilityName, "start-boots-shopping", map[string]any{
		"kind": "virtual_shopping", "intention_id": intentionID, "duration_minutes": 15,
		"category": "boots", "slot": "shoes", "description": "黑色短靴", "reason": "实际安排一次虚拟购物",
	}))
	if err != nil || started.Result.Status != "accepted" {
		t.Fatalf("shopping was not durably accepted: receipt=%#v err=%v", started, err)
	}
	activityID := stringValue(mapValue(started.Result.Output)["activity_id"])
	missing, missingErr := fixture.app.ExecuteTool(fixture.ctx, fixture.request(lifeActivityAdvanceCapabilityName, "advance-missing-activity", map[string]any{"activity_id": "missing-activity"}))
	if missingErr == nil || missing.Result.ErrorCode != "activity_not_found" || missing.Result.Retryable {
		t.Fatalf("unknown activity must be a correctable target error: receipt=%#v err=%v", missing, missingErr)
	}
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `UPDATE public.fluctlight_life_activity_runs SET profile_id='other-profile' WHERE id=$1`, activityID); err != nil {
		t.Fatal(err)
	}
	foreignProfile, foreignProfileErr := fixture.app.ExecuteTool(fixture.ctx, fixture.request(lifeActivityAdvanceCapabilityName, "advance-other-profile-activity", map[string]any{"activity_id": activityID}))
	if foreignProfileErr == nil || foreignProfile.Result.ErrorCode != "activity_not_found" || foreignProfile.Result.Retryable {
		t.Fatalf("another profile's activity must not resolve: receipt=%#v err=%v", foreignProfile, foreignProfileErr)
	}
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `UPDATE public.fluctlight_life_activity_runs SET profile_id='default' WHERE id=$1`, activityID); err != nil {
		t.Fatal(err)
	}
	beforeDue, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(lifeActivityAdvanceCapabilityName, "advance-boots-too-early", map[string]any{}))
	if err != nil || beforeDue.Result.Status != "accepted" || stringValue(mapValue(beforeDue.Result.Output)["status"]) != "in_progress" {
		t.Fatalf("activity completed before elapsed time: receipt=%#v err=%v", beforeDue, err)
	}
	var purchasedBefore int
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND source_kind='purchase_result'`, fixture.fluctlightID).Scan(&purchasedBefore); err != nil || purchasedBefore != 0 {
		t.Fatalf("planned purchase entered wardrobe: count=%d err=%v", purchasedBefore, err)
	}
	forceVirtualActivityDue(t, fixture, activityID)
	router := setupVirtualActivityTestProvider(t, fixture, map[string]any{
		"status": "completed", "reason": "在虚拟商店选到合适的一双", "acquired_item": map[string]any{"category": "boots", "slot": "shoes", "description": "黑色短靴"},
	})
	resolved, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(lifeActivityAdvanceCapabilityName, "advance-boots-complete", map[string]any{"activity_id": activityID}))
	if err != nil || resolved.Result.Status != "completed" || router.requestCount("virtual_activity_result") != 1 {
		t.Fatalf("virtual shopping did not resolve once: receipt=%#v err=%v calls=%d", resolved, err, router.requestCount("virtual_activity_result"))
	}
	itemID := stringValue(mapValue(resolved.Result.Output)["item_id"])
	if itemID == "" {
		t.Fatalf("completed purchase has no durable item: %#v", resolved.Result.Output)
	}
	var purchased, wearing int
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND id=$2 AND source_kind='purchase_result' AND ownership='owned'`, fixture.fluctlightID, itemID).Scan(&purchased); err != nil || purchased != 1 {
		t.Fatalf("purchase result did not create exactly one owned item: count=%d err=%v", purchased, err)
	}
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.fluctlight_worn_items WHERE fluctlight_id=$1 AND item_id=$2`, fixture.fluctlightID, itemID).Scan(&wearing); err != nil || wearing != 0 {
		t.Fatalf("purchase automatically wore the item: count=%d err=%v", wearing, err)
	}
	var intentionStatus string
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT status FROM public.fluctlight_intentions WHERE id=$1`, intentionID).Scan(&intentionStatus); err != nil || intentionStatus != "completed" {
		t.Fatalf("verified purchase did not complete linked intention: status=%s err=%v", intentionStatus, err)
	}
	wear, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(wardrobeWearCapabilityName, "wear-purchased-boots", map[string]any{"mode": "partial", "item_ids": []string{itemID}}))
	if err != nil || wear.Result.Status != "completed" {
		t.Fatalf("later independent wear did not reuse acquired item: receipt=%#v err=%v", wear, err)
	}
}

func TestFailedVirtualShoppingDoesNotCreateItemOrCompleteIntention(t *testing.T) {
	fixture := seedWardrobeToolFixture(t)
	intentionID := createQualifiedBootIntention(t, fixture, "boots-failed")
	started, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(lifeActivityStartCapabilityName, "start-failed-shopping", map[string]any{
		"kind": "virtual_shopping", "intention_id": intentionID, "duration_minutes": 15,
		"category": "boots", "slot": "shoes", "description": "黑色短靴", "reason": "尝试虚拟购物",
	}))
	if err != nil {
		t.Fatal(err)
	}
	activityID := stringValue(mapValue(started.Result.Output)["activity_id"])
	forceVirtualActivityDue(t, fixture, activityID)
	setupVirtualActivityTestProvider(t, fixture, map[string]any{"status": "failed", "reason": "虚拟商店没有合适的尺码"})
	resolved, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(lifeActivityAdvanceCapabilityName, "advance-failed-shopping", map[string]any{"activity_id": activityID}))
	if err != nil || resolved.Result.Status != "rejected" || stringValue(mapValue(resolved.Result.Output)["status"]) != "failed" {
		t.Fatalf("failed shopping was misreported: receipt=%#v err=%v", resolved, err)
	}
	var purchased int
	var status string
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND source_kind='purchase_result'`, fixture.fluctlightID).Scan(&purchased); err != nil {
		t.Fatal(err)
	}
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT status FROM public.fluctlight_intentions WHERE id=$1`, intentionID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if purchased != 0 || status != "qualified" {
		t.Fatalf("failed purchase created property or completed desire: items=%d intention=%s", purchased, status)
	}
}

func TestVirtualHaircutUpdatesSharedBodyAndLeavesHistoricalFoundation(t *testing.T) {
	fixture := seedWardrobeToolFixture(t)
	started, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(lifeActivityStartCapabilityName, "start-haircut", map[string]any{
		"kind": "haircut", "duration_minutes": 15, "desired_hair_length": "short", "reason": "决定外出剪短头发",
	}))
	if err != nil || started.Result.Status != "accepted" {
		t.Fatalf("haircut did not start: receipt=%#v err=%v", started, err)
	}
	activityID := stringValue(mapValue(started.Result.Output)["activity_id"])
	forceVirtualActivityDue(t, fixture, activityID)
	setupVirtualActivityTestProvider(t, fixture, map[string]any{"status": "completed", "reason": "虚拟理发完成", "hair_length": "short"})
	resolved, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(lifeActivityAdvanceCapabilityName, "advance-haircut", map[string]any{"activity_id": activityID}))
	if err != nil || resolved.Result.Status != "completed" {
		t.Fatalf("haircut result not applied: receipt=%#v err=%v", resolved, err)
	}
	appearance, _, _, err := fixture.app.readEffectiveLifeSnapshot(fixture.ctx, fixture.fluctlightID, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	hair := mapValue(mapValue(appearance["body_fields"])["hair_length"])
	if stringValue(hair["value"]) != "short" || stringValue(mapValue(mapValue(appearance["body_fields"])["hair_style"])["status"]) != "cleared" {
		t.Fatalf("current body did not replace long hair or clear stale temporary style: %#v", appearance)
	}
	var previous []byte
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT state_json FROM public.fluctlight_appearance_revisions WHERE fluctlight_id=$1 AND revision=0`, fixture.fluctlightID).Scan(&previous); err != nil || !strings.Contains(string(previous), "long") {
		t.Fatalf("historical long hair was lost: state=%s err=%v", previous, err)
	}
}

func TestDueIntentionActivityResultSettlesFrozenAttempt(t *testing.T) {
	for _, terminal := range []string{"completed", "failed"} {
		t.Run(terminal, func(t *testing.T) {
			fixture := seedWardrobeToolFixture(t)
			intentionID := createQualifiedBootIntention(t, fixture, "due-"+terminal)
			attemptID := "attempt-" + fixture.suffix
			if _, err := fixture.repository.Pool().Exec(fixture.ctx, `UPDATE public.fluctlight_intentions SET status='due',revision=revision+1,current_attempt_id=$2 WHERE id=$1`, intentionID, attemptID); err != nil {
				t.Fatal(err)
			}
			projection, err := fixture.app.BuildContextProjection(fixture.ctx, fixture.ownerID, fixture.fluctlightID, fixture.conversationID, "", "")
			if err != nil {
				t.Fatal(err)
			}
			var goal, intention ContextReference
			for _, entry := range projection.ReferenceIndex.ByRef {
				if entry.Kind == ContextReferenceIntention && entry.EntityID == intentionID {
					intention = entry
				}
			}
			if intention.Ref == "" {
				t.Fatal("due intention missing from frozen projection")
			}
			goal = projection.ReferenceIndex.ByRef[stringValue(decodeObject(intention.Snapshot)["goal_ref"])]
			if goal.Ref == "" {
				t.Fatalf("goal missing for due intention: %#v", intention)
			}
			started, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(lifeActivityStartCapabilityName, "due-start-"+terminal, map[string]any{
				"kind": "virtual_shopping", "intention_id": intentionID, "duration_minutes": 15,
				"category": "boots", "slot": "shoes", "description": "黑色短靴", "reason": "处理到期购买意愿",
			}))
			if err != nil || started.Result.Status != "accepted" {
				t.Fatalf("start=%#v err=%v", started, err)
			}
			activityID := stringValue(mapValue(started.Result.Output)["activity_id"])
			var attempts int
			var startedAttemptStatus string
			if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*),min(status) FROM public.fluctlight_intention_attempts WHERE attempt_id=$1`, attemptID).Scan(&attempts, &startedAttemptStatus); err != nil || attempts != 1 || startedAttemptStatus != "waiting" {
				t.Fatalf("start must retain a nonterminal durable attempt: %d %s %v", attempts, startedAttemptStatus, err)
			}

			forceVirtualActivityDue(t, fixture, activityID)
			result := map[string]any{"status": terminal, "reason": "虚拟商店返回实际结果"}
			if terminal == "completed" {
				result["acquired_item"] = map[string]any{"category": "boots", "slot": "shoes", "description": "黑色短靴"}
			}
			setupVirtualActivityTestProvider(t, fixture, result)
			resolved, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(lifeActivityAdvanceCapabilityName, "due-resolve-"+terminal, map[string]any{"activity_id": activityID}))
			if err != nil {
				t.Fatalf("resolve=%#v err=%v", resolved, err)
			}
			var status, attemptStatus, outcomeStatus string
			if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT i.status,a.status,o.status FROM public.fluctlight_intentions i JOIN public.fluctlight_intention_attempts a ON a.attempt_id=i.current_attempt_id JOIN public.cognition_action_outcomes o ON o.id=a.outcome_id WHERE i.id=$1`, intentionID).Scan(&status, &attemptStatus, &outcomeStatus); err != nil {
				t.Fatal(err)
			}
			wantStatus, wantAttempt := "completed", "succeeded"
			if terminal == "failed" {
				wantStatus, wantAttempt = "qualified", "failed"
			}
			if status != wantStatus || attemptStatus != wantAttempt || outcomeStatus != terminal {
				t.Fatalf("result not joined to due attempt: intention=%s attempt=%s outcome=%s", status, attemptStatus, outcomeStatus)
			}
		})
	}
}
