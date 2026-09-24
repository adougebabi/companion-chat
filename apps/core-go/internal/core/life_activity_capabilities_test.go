package core

import (
	"github.com/jackc/pgx/v5"
	"net/http"
	"strings"
	"testing"
	"time"
)

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
		"category": "boots", "slot": "shoes", "description": "合适的黑色短靴", "reason": "实际安排一次虚拟购物",
	}))
	if err != nil || started.Result.Status != "accepted" {
		t.Fatalf("shopping was not durably accepted: receipt=%#v err=%v", started, err)
	}
	activityID := stringValue(mapValue(started.Result.Output)["activity_id"])
	beforeDue, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(lifeActivityAdvanceCapabilityName, "advance-boots-too-early", map[string]any{"activity_id": activityID}))
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
			actionID := "due-action-" + fixture.suffix
			outcomes, err := buildActionOutcomes(actionID, fixture.fluctlightID, "due-fact-"+fixture.suffix, "no_op", []CapabilityResult{started.Result}, map[string]any{
				"status": "pending", "goal_refs": []string{goal.Ref}, "intention_refs": []string{intention.Ref},
				"context_references": map[string]ContextReference{goal.Ref: goal, intention.Ref: intention},
			}, fixture.app.capabilityRegistry())
			if err != nil || len(outcomes) != 2 || outcomes[1].ExternalRef != activityID {
				t.Fatalf("pending outcomes=%#v err=%v", outcomes, err)
			}
			if err := withTransaction(fixture.ctx, fixture.repository.Pool(), func(tx pgx.Tx) error {
				return persistActionOutcomesTx(fixture.ctx, tx, outcomes)
			}); err != nil {
				t.Fatal(err)
			}
			var attempts int
			if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.fluctlight_intention_attempts WHERE attempt_id=$1`, attemptID).Scan(&attempts); err != nil || attempts != 0 {
				t.Fatalf("start prematurely settled attempt: count=%d err=%v", attempts, err)
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
