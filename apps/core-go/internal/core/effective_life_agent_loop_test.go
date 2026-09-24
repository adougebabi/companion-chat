package core

import (
	"github.com/jackc/pgx/v5"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// Controlled Provider protocol evidence: the real conversation Agent queries
// PostgreSQL inventory, consumes the native ToolResult, then persists a pending
// Intention. Behavioral reliability still requires separate live model samples.
func TestFormalMainWardrobeQueryResultCanLeadToPendingIntention(t *testing.T) {
	fixture := seedWardrobeToolFixture(t)
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `UPDATE public.fluctlight_wardrobe_items SET availability='lost',revision=revision+1 WHERE fluctlight_id=$1 AND category='boots'`, fixture.fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.repository.Pool().Exec(fixture.ctx, `UPDATE public.fluctlight_wardrobe_states SET revision=revision+1 WHERE fluctlight_id=$1`, fixture.fluctlightID); err != nil {
		t.Fatal(err)
	}
	seedCognitiveProviderRole(t, fixture.ctx, fixture.repository, "main-wardrobe-endpoint-"+fixture.suffix)
	calls := 0
	router := newFakeProviderRouter().on("conversation_turn_response", func(payload map[string]any) fakeProviderResult {
		calls++
		if os.Getenv("YAOGUANG_CAPTURE_WIRE") == "1" {
			t.Logf("WIRE_MAIN_%d=%s", calls, jsonString(payload))
		}
		switch calls {
		case 1:
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("inspect-boots", wardrobeInspectCapabilityName,
				map[string]any{"operation": "list", "category": "boots"})}}
		case 2:
			if !payloadHasToolResult(payload) || !strings.Contains(nativePersonaToolMessages(payload), "lost") {
				t.Fatalf("second model request did not consume actual unavailable item result: %#v", payload["messages"])
			}
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("form-boots-intention", intentionDecideCapabilityName,
				map[string]any{"operation": "create", "goal": "拥有可穿的短靴", "action": "安排虚拟购物购买短靴", "expected_outcome": "短靴实际入柜", "reason": "查询确认已记录的靴子不可用"})}}
		default:
			if !payloadHasToolResult(payload) || !strings.Contains(nativePersonaToolMessages(payload), "candidate") {
				t.Fatalf("final model request did not consume committed intention receipt: %#v", payload["messages"])
			}
			return nativePersonaFinal()
		}
	})
	fixture.app.Provider.HTTP = &http.Client{Transport: router}
	result, err := fixture.app.RunConversationCognitionAgent(fixture.ctx, ConversationCognitionAgentInput{
		AuthorizationActorID: fixture.ownerID, FluctlightID: fixture.fluctlightID, ConversationID: fixture.conversationID,
		RunID: "main-wardrobe-intention-" + fixture.suffix, CurrentInput: "我想看你穿靴子",
	})
	if err != nil || calls != 3 || result.Trace == nil {
		t.Fatalf("formal Main loop failed: calls=%d result=%#v err=%v", calls, result, err)
	}
	invocations, receipts := result.Trace.Snapshot()
	if len(invocations) != 2 || len(receipts) != 2 || invocations[0].CapabilityName != wardrobeInspectCapabilityName || invocations[1].CapabilityName != intentionDecideCapabilityName || receipts[1].Status != "completed" {
		t.Fatalf("formal Main Tool trace incomplete: calls=%#v results=%#v", invocations, receipts)
	}
	var pending, purchased, worn int
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.fluctlight_intentions WHERE fluctlight_id=$1 AND status='candidate'`, fixture.fluctlightID).Scan(&pending); err != nil {
		t.Fatal(err)
	}
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND source_kind='purchase_result'`, fixture.fluctlightID).Scan(&purchased); err != nil {
		t.Fatal(err)
	}
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.fluctlight_worn_items WHERE fluctlight_id=$1 AND slot='shoes'`, fixture.fluctlightID).Scan(&worn); err != nil {
		t.Fatal(err)
	}
	if pending != 1 || purchased != 0 || worn != 0 {
		t.Fatalf("query/intention path fabricated purchase or wearing: pending=%d purchased=%d worn=%d", pending, purchased, worn)
	}
}

// A WakeUp with no new user message can advance an elapsed activity through
// the same native Tool adapter and consume the committed result before no-op.
func TestFormalWakeUpAdvancesElapsedHaircutWithoutSendingMessage(t *testing.T) {
	fixture := seedWardrobeToolFixture(t)
	initialLife := currentLifeForTest(t, fixture.ctx, fixture.app, fixture.fluctlightID, time.Now().UTC())
	if _, err := fixture.app.AcceptSchedule(fixture.ctx, fixture.ownerID, fixture.fluctlightID,
		fullDaySchedulePayloadForTest(time.Now().UTC(), "activity-wakeup-schedule-"+fixture.suffix, stringValue(initialLife["context_revision"]))); err != nil {
		t.Fatal(err)
	}
	started, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(lifeActivityStartCapabilityName, "wake-haircut-start", map[string]any{
		"kind": "haircut", "duration_minutes": 15, "desired_hair_length": "short", "reason": "决定剪短头发",
	}))
	if err != nil {
		t.Fatal(err)
	}
	activityID := stringValue(mapValue(started.Result.Output)["activity_id"])
	forceVirtualActivityDue(t, fixture, activityID)
	seedCognitiveProviderRole(t, fixture.ctx, fixture.repository, "activity-wakeup-endpoint-"+fixture.suffix)
	wakeCalls := 0
	router := newFakeProviderRouter().on("virtual_activity_result", func(_ map[string]any) fakeProviderResult {
		return fakeProviderResult{Structured: map[string]any{"status": "completed", "reason": "虚拟理发完成", "hair_length": "short"}}
	}).on("wake_up_response", func(payload map[string]any) fakeProviderResult {
		wakeCalls++
		if os.Getenv("YAOGUANG_CAPTURE_WIRE") == "1" {
			t.Logf("WIRE_WAKE_%d=%s", wakeCalls, jsonString(payload))
		}
		if wakeCalls == 1 {
			if !strings.Contains(jsonString(payload), activityID) {
				t.Fatalf("WakeUp final request omitted the due activity: %#v", payload["messages"])
			}
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("wake-advance-haircut", lifeActivityAdvanceCapabilityName,
				map[string]any{"activity_id": activityID})}}
		}
		if !payloadHasToolResult(payload) || !strings.Contains(nativePersonaToolMessages(payload), "body_revision") {
			t.Fatalf("WakeUp continuation did not consume actual haircut result: %#v", payload["messages"])
		}
		return fakeProviderResult{Structured: map[string]any{"action_type": "no_op", "response_intent": "已完成生活活动，不主动打扰用户", "evidence_refs": []any{}, "influences": []any{}}}
	})
	fixture.app.Provider.HTTP = &http.Client{Transport: router}
	wake, err := fixture.app.ProcessWakeUp(fixture.ctx, fixture.fluctlightID, 1)
	if err != nil || wakeCalls != 2 || router.requestCount("virtual_activity_result") != 1 {
		t.Fatalf("formal WakeUp activity loop failed: wake=%#v err=%v wake_calls=%d result_calls=%d", wake, err, wakeCalls, router.requestCount("virtual_activity_result"))
	}
	appearance, _, _, err := fixture.app.readEffectiveLifeSnapshot(fixture.ctx, fixture.fluctlightID, time.Now().UTC())
	if err != nil || stringValue(mapValue(mapValue(appearance["body_fields"])["hair_length"])["value"]) != "short" {
		t.Fatalf("WakeUp Tool did not change shared body: %#v err=%v", appearance, err)
	}
	var visible int
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.conversation_messages WHERE author_actor_id=$1 AND kind='assistant'`, fixture.fluctlightID).Scan(&visible); err != nil || visible != 0 {
		t.Fatalf("internal life progress sent an unauthorized message: count=%d err=%v", visible, err)
	}
}

func startDueShoppingThroughNative(t *testing.T, failFinal ...bool) (independentToolE2EFixture, string, map[string]any, string, *fakeProviderRouter) {
	t.Helper()
	fixture := seedWardrobeToolFixture(t)
	intentionID := createQualifiedBootIntention(t, fixture, "native-due")
	if err := withTransaction(fixture.ctx, fixture.repository.Pool(), func(tx pgx.Tx) error {
		_, err := appendProcessedCognitionFactTx(fixture.ctx, tx, fixture.fluctlightID, "life.event.created", map[string]any{"summary": "可以安排虚拟购物"}, "native-due-trigger-"+fixture.suffix)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	due, err := fixture.app.ProcessIntentionTrigger(fixture.ctx, intentionID)
	if err != nil || stringValue(due["status"]) != "due" {
		t.Fatalf("due=%#v err=%v", due, err)
	}
	goalRef, intentionRef := stringValue(due["goal_ref"]), stringValue(due["intention_ref"])
	projection, err := fixture.app.BuildContextProjection(fixture.ctx, fixture.ownerID, fixture.fluctlightID, "", stringValue(due["inbox_id"]), "")
	if err != nil {
		t.Fatal(err)
	}
	for ref, entry := range projection.ReferenceIndex.ByRef {
		if entry.Kind == ContextReferenceIntention && entry.EntityID == intentionID {
			intentionRef = ref
		}
	}
	seedCognitiveProviderRole(t, fixture.ctx, fixture.repository, "native-due-endpoint-"+fixture.suffix)
	modelCalls := 0
	router := newFakeProviderRouter().on("native_cognition_response", func(payload map[string]any) fakeProviderResult {
		modelCalls++
		if modelCalls == 1 {
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("due-start-shopping", lifeActivityStartCapabilityName, map[string]any{
				"kind": "virtual_shopping", "intention_id": intentionID, "duration_minutes": 15,
				"category": "boots", "slot": "shoes", "description": "黑色短靴", "reason": "已有到期意愿，安排一次购物",
			})}}
		}
		if !payloadHasToolResult(payload) || !strings.Contains(nativePersonaToolMessages(payload), "activity_id") {
			t.Fatalf("due native continuation lacked actual activity receipt: %#v", payload["messages"])
		}
		if len(failFinal) > 0 && failFinal[0] {
			return fakeProviderResult{Status: 500}
		}
		return fakeProviderResult{Structured: map[string]any{
			"attention": "购物活动已开始", "thought": "等待真实结果", "desire": "获得短靴", "agency": "暂不视为完成",
			"appraisal": map[string]any{"relevance": 0.8, "goal_congruence": 0.8, "reward": 0.4, "loss": 0.0, "social_threat": 0.0, "controllability": 0.7, "responsibility": 0.6, "relationship_significance": 0.0, "expected_effect": 0.6, "evidence_refs": []any{}, "event_kind": "intention_due", "direction": "positive", "drive_signals": []any{}},
			"influences": []any{
				map[string]any{"ref": goalRef, "role": "motivates", "confidence": 0.9, "note": "目标需要真实获得物品"},
				map[string]any{"ref": intentionRef, "role": "grounds", "confidence": 0.9, "note": "到期意愿已开始但尚未完成"},
			}}}
	})
	fixture.app.Provider.HTTP = &http.Client{Transport: router}
	runErr := fixture.app.ProcessNativeCognitionFact(fixture.ctx, stringValue(due["inbox_id"]))
	if len(failFinal) > 0 && failFinal[0] {
		if runErr == nil || modelCalls < 2 {
			t.Fatalf("expected failure after committed start: err=%v calls=%d", runErr, modelCalls)
		}
	} else if runErr != nil || modelCalls != 2 {
		t.Fatalf("native due run err=%v model_calls=%d", runErr, modelCalls)
	}
	var activityID, pending string
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT id FROM public.fluctlight_life_activity_runs WHERE intention_id=$1`, intentionID).Scan(&activityID); err != nil {
		t.Fatal(err)
	}
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT status FROM public.cognition_action_outcomes WHERE external_ref=$1`, activityID).Scan(&pending); err != nil || pending != "pending" {
		t.Fatalf("activity start settled too early: status=%q err=%v", pending, err)
	}
	return fixture, intentionID, due, activityID, router
}

func TestFormalDueNativeAgentStartsActivityAndResultSettlesAttempt(t *testing.T) {
	fixture, intentionID, due, activityID, router := startDueShoppingThroughNative(t)
	forceVirtualActivityDue(t, fixture, activityID)
	router.on("virtual_activity_result", func(_ map[string]any) fakeProviderResult {
		return fakeProviderResult{Structured: map[string]any{"status": "completed", "reason": "实际买到了", "acquired_item": map[string]any{"category": "boots", "slot": "shoes", "description": "黑色短靴"}}}
	})
	resolved, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(lifeActivityAdvanceCapabilityName, "native-due-resolve", map[string]any{"activity_id": activityID}))
	if err != nil || resolved.Result.Status != "completed" {
		t.Fatalf("resolve=%#v err=%v", resolved, err)
	}
	var status, attemptStatus string
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT i.status,a.status FROM public.fluctlight_intentions i JOIN public.fluctlight_intention_attempts a ON a.attempt_id=i.current_attempt_id WHERE i.id=$1 AND a.attempt_id=$2`, intentionID, stringValue(due["attempt_id"])).Scan(&status, &attemptStatus); err != nil || status != "completed" || attemptStatus != "succeeded" {
		t.Fatalf("native due attempt not verified: status=%q attempt=%q err=%v", status, attemptStatus, err)
	}
}

// A decision revoked after start cannot be silently resurrected by an
// asynchronous result, though the completed activity remains a real event.
func TestFormalDueActivityResultPreservesPausedOrCancelledIntention(t *testing.T) {
	for _, operation := range []string{"pause", "cancel"} {
		t.Run(operation, func(t *testing.T) {
			fixture, intentionID, due, activityID, router := startDueShoppingThroughNative(t)
			changed, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(intentionDecideCapabilityName, "stop-due-"+operation, map[string]any{
				"operation": operation, "intention_id": intentionID, "reason": "改变当前意愿",
			}))
			if err != nil || changed.Result.Status != "completed" {
				t.Fatalf("intention %s: receipt=%#v err=%v", operation, changed, err)
			}
			forceVirtualActivityDue(t, fixture, activityID)
			router.on("virtual_activity_result", func(_ map[string]any) fakeProviderResult {
				return fakeProviderResult{Structured: map[string]any{"status": "completed", "reason": "购物活动结束", "acquired_item": map[string]any{"category": "boots", "slot": "shoes", "description": "黑色短靴"}}}
			})
			resolved, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(lifeActivityAdvanceCapabilityName, "resolve-after-"+operation, map[string]any{"activity_id": activityID}))
			if err != nil || resolved.Result.Status != "completed" {
				t.Fatalf("result lost after %s: receipt=%#v err=%v", operation, resolved, err)
			}
			var status string
			var items, attempts int
			if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT status FROM public.fluctlight_intentions WHERE id=$1`, intentionID).Scan(&status); err != nil {
				t.Fatal(err)
			}
			if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND source_kind='purchase_result'`, fixture.fluctlightID).Scan(&items); err != nil {
				t.Fatal(err)
			}
			if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.fluctlight_intention_attempts WHERE attempt_id=$1`, stringValue(due["attempt_id"])).Scan(&attempts); err != nil {
				t.Fatal(err)
			}
			expected := "paused"
			if operation == "cancel" {
				expected = "cancelled"
			}
			if status != expected || items != 1 || attempts != 0 {
				t.Fatalf("result after %s: intention=%q items=%d attempts=%d", operation, status, items, attempts)
			}
		})
	}
}

// A committed start survives a final Provider failure and is settled by the
// later activity result, without replaying the model or duplicating the run.
func TestFormalDueStartSurvivesAgentFailureAndSettlesOnce(t *testing.T) {
	fixture, intentionID, due, activityID, router := startDueShoppingThroughNative(t, true)
	var failed string
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT status FROM public.cognition_inbox WHERE id=$1`, stringValue(due["inbox_id"])).Scan(&failed); err != nil || failed != "failed" {
		t.Fatalf("agent failure was not recorded: %q %v", failed, err)
	}
	forceVirtualActivityDue(t, fixture, activityID)
	router.on("virtual_activity_result", func(_ map[string]any) fakeProviderResult {
		return fakeProviderResult{Structured: map[string]any{"status": "completed", "reason": "购物活动实际完成", "acquired_item": map[string]any{"category": "boots", "slot": "shoes", "description": "黑色短靴"}}}
	})
	for index := 0; index < 2; index++ {
		receipt, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(lifeActivityAdvanceCapabilityName, "recover-due-result", map[string]any{"activity_id": activityID}))
		if err != nil || receipt.Result.Status != "completed" {
			t.Fatalf("settle/replay %d: receipt=%#v err=%v", index, receipt, err)
		}
	}
	var runs, attempts, outcomes int
	var status string
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.fluctlight_life_activity_runs WHERE intention_id=$1`, intentionID).Scan(&runs); err != nil {
		t.Fatal(err)
	}
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.fluctlight_intention_attempts WHERE attempt_id=$1`, stringValue(due["attempt_id"])).Scan(&attempts); err != nil {
		t.Fatal(err)
	}
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.cognition_action_outcomes WHERE external_ref=$1`, activityID).Scan(&outcomes); err != nil {
		t.Fatal(err)
	}
	if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT status FROM public.fluctlight_intentions WHERE id=$1`, intentionID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if runs != 1 || attempts != 1 || outcomes != 1 || status != "completed" {
		t.Fatalf("recovery not idempotent: runs=%d attempts=%d outcomes=%d intention=%q", runs, attempts, outcomes, status)
	}
}

func TestFormalDueActivityResultCannotSettleSupersededAttempt(t *testing.T) {
	for _, changes := range [][]string{{"update"}, {"pause", "resume"}} {
		t.Run(strings.Join(changes, "_"), func(t *testing.T) {
			fixture, intentionID, due, activityID, router := startDueShoppingThroughNative(t)
			for _, change := range changes {
				args := map[string]any{"operation": change, "intention_id": intentionID, "reason": "新的意愿修订"}
				if change == "update" {
					args["expected_outcome"] = "希望改为另一种靴子"
				}
				receipt, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(intentionDecideCapabilityName, "supersede-"+change, args))
				if err != nil || receipt.Result.Status != "completed" {
					t.Fatalf("%s: receipt=%#v err=%v", change, receipt, err)
				}
			}
			forceVirtualActivityDue(t, fixture, activityID)
			router.on("virtual_activity_result", func(_ map[string]any) fakeProviderResult {
				return fakeProviderResult{Structured: map[string]any{"status": "completed", "reason": "原购物活动结束", "acquired_item": map[string]any{"category": "boots", "slot": "shoes", "description": "黑色短靴"}}}
			})
			receipt, err := fixture.app.ExecuteTool(fixture.ctx, fixture.request(lifeActivityAdvanceCapabilityName, "resolve-superseded", map[string]any{"activity_id": activityID}))
			if err != nil || receipt.Result.Status != "completed" {
				t.Fatalf("independent result failed: receipt=%#v err=%v", receipt, err)
			}
			var status string
			var items, attempts int
			if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT status FROM public.fluctlight_intentions WHERE id=$1`, intentionID).Scan(&status); err != nil {
				t.Fatal(err)
			}
			if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.fluctlight_wardrobe_items WHERE fluctlight_id=$1 AND source_kind='purchase_result'`, fixture.fluctlightID).Scan(&items); err != nil {
				t.Fatal(err)
			}
			if err := fixture.repository.Pool().QueryRow(fixture.ctx, `SELECT count(*) FROM public.fluctlight_intention_attempts WHERE attempt_id=$1`, stringValue(due["attempt_id"])).Scan(&attempts); err != nil {
				t.Fatal(err)
			}
			want := "in_progress"
			if len(changes) == 2 {
				want = "qualified"
			}
			if status != want || items != 1 || attempts != 0 {
				t.Fatalf("superseded result: status=%q want=%q items=%d attempts=%d", status, want, items, attempts)
			}
		})
	}
}
