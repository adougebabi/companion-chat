package core

import (
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestGoalExecutionHandoffWakeWithoutUserMessageQualifiesAndAcquires(t *testing.T) {
	f := newIndependentToolE2EFixture(t, "autonomous-goal-handoff")
	var directErr error
	f.conversationID, directErr = f.app.EnsureDirectConversation(f.ctx, f.ownerID, f.fluctlightID)
	if directErr != nil {
		t.Fatal(directErr)
	}
	seedUnknownEffectiveLifeForTest(t, f.ctx, f.repository, f.fluctlightID)
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.fluctlight_personality_runtime(fluctlight_id,active_profile_id,revision) VALUES($1,'default',0) ON CONFLICT DO NOTHING`, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	f.seedAcceptedSchedule(t)
	goalID := "goal-handoff-" + f.suffix
	goal, record, err := CreateGoalAuthority(GoalAuthority{EntityID: goalID, SchemaVersion: goalAuthoritySchemaVersion, Ref: "goal:ctx_" + stableDigest(goalID), FluctlightID: f.fluctlightID, DesiredOutcome: "拥有黑色短靴", SuccessCriteria: []string{"实际获得黑色短靴"}, Motivation: "自己的生活需要", Scope: "general", Status: GoalActive, Revision: 1}, []string{"owner:goal"}, f.app.now())
	if err != nil {
		t.Fatal(err)
	}
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		if _, err := persistGoalAuthorityTx(f.ctx, tx, nil, goal, record, "handoff-goal-create"); err != nil {
			return err
		}
		next, err := applyGoalPlanTx(f.ctx, tx, goal, GoalPlanCandidate{GoalID: goalID, ExpectedRevision: goal.Revision, CriteriaVersion: 1, Reason: "安排自己的获准虚拟生活事务", NextStep: "安排一次虚拟购物"}, "handoff-plan", f.app.now())
		if err != nil {
			return err
		}
		return persistGoalExecutionHandoffTx(f.ctx, tx, &next, goal.ExecutionHint, f.app.now())
	}); err != nil {
		t.Fatal(err)
	}
	seedCognitiveProviderRole(t, f.ctx, f.repository, "handoff-provider-"+f.suffix)
	calls := 0
	router := newFakeProviderRouter().on("wake_up_response", func(payload map[string]any) fakeProviderResult {
		calls++
		if calls == 1 {
			if !strings.Contains(jsonString(payload), "goal_execution_obligations") {
				t.Fatal("wake did not receive durable obligation")
			}
			projection, err := f.app.BuildContextProjection(f.ctx, f.ownerID, f.fluctlightID, f.conversationID, "", "")
			if err != nil {
				t.Fatal(err)
			}
			goalRef := ""
			for ref, entry := range projection.ReferenceIndex.ByRef {
				if entry.Kind == ContextReferenceGoal && entry.EntityID == goalID {
					goalRef = ref
				}
			}
			if goalRef == "" {
				t.Fatal("Goal absent from current Wake projection")
			}
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("handoff-create", intentionDecideCapabilityName, map[string]any{"operation": "create", "goal_ref": goalRef, "action": "虚拟购物购买短靴", "expected_outcome": "短靴实际入柜", "reason": "目标已有获准下一步"})}}
		}
		if calls == 2 {
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("handoff-qualify", intentionDecideCapabilityName, map[string]any{"operation": "qualify", "reason": "当前可自主执行"})}}
		}
		return fakeProviderResult{Structured: map[string]any{"action_type": "capability", "response_intent": "意愿已确认资格，等待真实触发", "evidence_refs": []any{}, "influences": []any{}}}
	})
	f.app.Provider.HTTP = &http.Client{Transport: router}
	if _, err := f.app.EnsureWakeUpIntents(f.ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.ProcessWakeUp(f.ctx, f.fluctlightID, 1); err != nil {
		t.Fatal(err)
	}
	var intentionID, status string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT id,status FROM public.fluctlight_intentions WHERE fluctlight_id=$1 AND goal_id=$2`, f.fluctlightID, goalID).Scan(&intentionID, &status); err != nil || status != "qualified" {
		t.Fatalf("candidate never qualified: %s %v", status, err)
	}
	obligations, err := f.app.goalExecutionObligations(f.ctx, f.fluctlightID)
	if err != nil || len(obligations) != 0 {
		t.Fatalf("owned dispatch duplicated: %#v %v", obligations, err)
	}
	if _, err := f.app.ProcessWakeUp(f.ctx, f.fluctlightID, 1); err != nil || calls != 3 {
		t.Fatalf("wake replay restarted execution calls=%d err=%v", calls, err)
	}
	// A real lifecycle opportunity, without a conversation/user message, triggers
	// the qualified semantic Intention through the existing native dispatcher.
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		_, err := appendProcessedCognitionFactTx(f.ctx, tx, f.fluctlightID, "life.event.created", map[string]any{"summary": "可执行生活活动"}, "handoff-opportunity")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	due, err := f.app.ProcessIntentionTrigger(f.ctx, intentionID)
	if err != nil || due["status"] != "due" {
		t.Fatalf("qualified intention did not trigger: %#v %v", due, err)
	}
	intentionRef := stringValue(due["intention_ref"])
	goalRef := stringValue(due["goal_ref"])
	projection, err := f.app.BuildContextProjection(f.ctx, f.ownerID, f.fluctlightID, "", stringValue(due["inbox_id"]), "")
	if err != nil {
		t.Fatal(err)
	}
	for ref, entry := range projection.ReferenceIndex.ByRef {
		if entry.Kind == ContextReferenceIntention && entry.EntityID == intentionID {
			intentionRef = ref
		}
		if entry.Kind == ContextReferenceGoal && entry.EntityID == goalID {
			goalRef = ref
		}
	}
	nativeCalls := 0
	router.on("native_cognition_response", func(payload map[string]any) fakeProviderResult {
		nativeCalls++
		if nativeCalls == 1 {
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("handoff-start", lifeActivityStartCapabilityName, map[string]any{"kind": "virtual_shopping", "intention_id": intentionID, "duration_minutes": 15, "category": "boots", "slot": "shoes", "description": "黑色短靴", "reason": "已确认资格的目标活动"})}}
		}
		// Starting the activity refreshes Intention state and its reference. Use
		// only the actual current refs offered to this physical continuation.
		wire := jsonString(payload["messages"])
		if refs := regexp.MustCompile(`goal:ctx_[a-f0-9]{32}`).FindAllString(wire, -1); len(refs) > 0 {
			goalRef = refs[len(refs)-1]
		}
		if refs := regexp.MustCompile(`intention:ctx_[a-f0-9]{32}`).FindAllString(wire, -1); len(refs) > 0 {
			intentionRef = refs[len(refs)-1]
		}
		return fakeProviderResult{Structured: map[string]any{"attention": "活动执行中", "thought": "等待真实结果", "desire": "取得物品", "agency": "尚未完成目标", "appraisal": map[string]any{"relevance": 0.8, "goal_congruence": 0.8, "reward": 0.4, "loss": 0.0, "social_threat": 0.0, "controllability": 0.7, "responsibility": 0.6, "relationship_significance": 0.0, "expected_effect": 0.6, "evidence_refs": []any{}, "event_kind": "intention_due", "direction": "positive", "drive_signals": []any{}}, "influences": []any{map[string]any{"ref": goalRef, "role": "motivates", "confidence": 0.9, "note": "当前目标"}, map[string]any{"ref": intentionRef, "role": "grounds", "confidence": 0.9, "note": "到期意愿"}}}}
	})
	if _, err := f.app.ProcessCognitionInbox(f.ctx, stringValue(due["inbox_id"])); err != nil {
		t.Fatal(err)
	}
	var activityID string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT id FROM public.fluctlight_life_activity_runs WHERE intention_id=$1`, intentionID).Scan(&activityID); err != nil {
		t.Fatal(err)
	}
	forceVirtualActivityDue(t, f, activityID)
	router.on("virtual_activity_result", func(map[string]any) fakeProviderResult {
		return fakeProviderResult{Structured: map[string]any{"status": "completed", "reason": "实际获得", "acquired_item": map[string]any{"category": "boots", "slot": "shoes", "description": "黑色短靴"}}}
	})
	result, err := f.app.ExecuteTool(f.ctx, f.request(lifeActivityAdvanceCapabilityName, "handoff-resolve", map[string]any{"activity_id": activityID}))
	if err != nil || stringValue(mapValue(result.Result.Output)["item_id"]) == "" {
		var diagnostic []byte
		_ = f.repository.Pool().QueryRow(f.ctx, `SELECT payload FROM public.diagnostic_events WHERE fluctlight_id=$1 AND event_type='tool.prepare.failed' ORDER BY created_at DESC LIMIT 1`, f.fluctlightID).Scan(&diagnostic)
		t.Fatalf("no durable acquired item: %#v %v diagnostic=%s", result, err, diagnostic)
	}
	runGoalAssessmentFixture(t, f, "acquisition", true, nil)
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status FROM public.fluctlight_goals WHERE id=$1`, goalID).Scan(&status); err != nil || status != "completed" {
		t.Fatalf("goal not closed by actual acquisition: %s %v", status, err)
	}
	var userMessages int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.conversation_messages WHERE author_actor_id=$1 AND kind='user'`, f.ownerID).Scan(&userMessages); err != nil || userMessages != 0 {
		t.Fatalf("test depended on user prompting: %d %v", userMessages, err)
	}
}
