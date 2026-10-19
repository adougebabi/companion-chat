package core

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestPromiseProjectionUsesCommitmentProofNotIntentionExecution(t *testing.T) {
	at := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name, state, intention string
		end                    time.Time
		cancelled              bool
		want                   string
	}{
		{"accepted is not fulfilled", "active", "candidate", at.Add(time.Hour), false, "accepted"},
		{"executed is not fulfilled", "active", "completed", at.Add(time.Hour), false, "accepted"},
		{"clock elapsed", "expired", "expired", at.Add(-time.Hour), false, "missed"},
		{"late valid evidence may fulfill", "completed", "completed", at.Add(-time.Hour), false, "fulfilled"},
		{"cancel keeps history", "cancelled", "cancelled", at.Add(time.Hour), true, "cancelled"},
	} {
		t.Run(test.name, func(t *testing.T) {
			p := map[string]any{"request": map[string]any{"status": "accepted"}, "commitment_id": "commitment-real", "commitment": map[string]any{"status": test.state}, "intention": map[string]any{"status": test.intention}, "window_end": test.end.Format(time.RFC3339Nano)}
			if test.cancelled {
				p["cancelled_at"] = at.Format(time.RFC3339Nano)
			}
			got := projectPromiseLifecycle(p, at)
			if got["lifecycle"] != test.want {
				t.Fatal(got)
			}
		})
	}
}

func TestPromiseNativePlanningRemainsDurableUntilExplicitBinding(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	calls := 0
	title := "完成一幅约好的小画"
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		calls++
		if payloadHasToolResult(payload) {
			return fakeProviderResult{Structured: map[string]any{"decision": "accepted", "reason": "先请求规划，再决定执行安排"}}
		}
		return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("promise-create", intentionDecideCapabilityName, map[string]any{"operation": "create", "goal": "完成约好的绘画项目", "action": title, "expected_outcome": "实际完成并分享约好的小画", "reason": "接受讨论这个具体提议"})}}
	})
	f.app.Provider.HTTP = &http.Client{Transport: router}
	seedCognitiveProviderRole(t, f.ctx, f.repository, "promise-endpoint-"+f.suffix)
	due := f.app.now().Add(24 * time.Hour).UTC().Format(time.RFC3339Nano)
	base := "/internal/fluctlights/" + f.fluctlightID + "/promises"
	body := map[string]any{"title": title, "due_at": due, "reason": "提出未来约定", "idempotency_key": "promise-proposal"}
	admitted, err := f.app.DispatchOwnerSurface(f.ctx, "POST", base, nil, f.ownerID, body)
	if err != nil {
		t.Fatal(err)
	}
	request := mapValue(admitted["request"])
	id, inbox := stringValue(request["id"]), stringValue(request["source_fact_id"])
	if calls != 0 {
		t.Fatal("API invoked model")
	}
	repeated, err := f.app.DispatchOwnerSurface(f.ctx, "POST", base, nil, f.ownerID, body)
	if err != nil || mapValue(repeated["request"])["id"] != id {
		t.Fatal("proposal replay", repeated, err)
	}
	result, err := f.app.ProcessCognitionInbox(f.ctx, inbox)
	if err != nil || result["status"] != "awaiting_goal" {
		var diagnostic []byte
		if e := f.repository.Pool().QueryRow(f.ctx, `SELECT payload FROM public.diagnostic_events WHERE fluctlight_id=$1 AND event_type='owner.life.request.assessment_failed' ORDER BY created_at DESC LIMIT 1`, f.fluctlightID).Scan(&diagnostic); e != nil {
			t.Fatal("missing task diagnostic", e)
		}
		t.Fatal("provisional planning falsely accepted or failed", result, err, "task diagnostic", string(diagnostic))
	}
	p, err := f.app.OwnerPromise(f.ctx, f.ownerID, f.fluctlightID, id)
	if err != nil || p["lifecycle"] != "awaiting_goal" || p["intention_id"] != nil || stringValue(p["planning_source_key"]) == "" {
		t.Fatal(p, err)
	}
	beforeCalls := calls
	if _, err := f.app.ProcessCognitionInbox(f.ctx, inbox); err != nil || calls != beforeCalls {
		t.Fatal("replay restarted assessment", err, calls)
	}
	// Direct Owner set governance must never impersonate the character accepting.
	c := plannerCommand(t, f, "owner-cannot-bind")
	c.PromiseBindings = []GoalPromiseBinding{{RequestID: id, GoalID: "anything", ExpectedRevision: intValue(p["revision"])}}
	if _, err := f.app.ApplyGoalSetCommand(f.ctx, f.ownerID, f.fluctlightID, c); err == nil {
		t.Fatal("Owner bypassed explicit Planner acceptance")
	}
	// The dedicated read is independently scoped and survives App reconstruction.
	fresh := *f.app
	page, err := fresh.OwnerPromises(f.ctx, f.ownerID, f.fluctlightID, url.Values{})
	if err != nil || len(arrayValue(page["items"])) != 1 {
		t.Fatal(page, err)
	}
	if _, err := fresh.OwnerPromise(f.ctx, f.foreignOwnerID, f.fluctlightID, id); err == nil {
		t.Fatal("cross Owner promise read")
	}
	// The real private Planner Tool binds its local Goal in the same transaction.
	run := plannerRecoveryRun(t, f)
	snapshot := plannerSnapshot(t, f)
	args := map[string]any{"expected_version": snapshot["revision"], "expected_facts_revision": snapshot["facts_revision"], "idempotency_key": "bind-promise", "reason": "Use the prior chosen action under this concrete Goal", "changes": []any{map[string]any{"id": "new-promise-goal", "operation": "create", "activate": true, "desired_outcome": "完成约好的绘画项目", "success_criteria": []string{"实际完成并分享约好的小画"}, "motivation": "已确认的具体提议"}}, "promise_bindings": []any{map[string]any{"request_id": id, "goal_id": "new-promise-goal", "expected_revision": p["revision"]}}}
	command := plannerRecoveryRequest(t, f, run, "bind-promise", args)
	receipt, err := f.app.ExecuteTool(f.ctx, command)
	if err != nil || receipt.Result.Status != "completed" {
		t.Fatal(receipt, err)
	}
	p, err = f.app.OwnerPromise(f.ctx, f.ownerID, f.fluctlightID, id)
	if err != nil || p["lifecycle"] != "accepted" || stringValue(p["commitment_id"]) == "" || stringValue(p["intention_id"]) == "" {
		t.Fatal("Planner did not close original proposal", p, err)
	}
	replay, err := f.app.ExecuteTool(f.ctx, command)
	if err != nil || !replay.Replayed || jsonString(replay.Result.Output) != jsonString(receipt.Result.Output) {
		t.Fatal("binding receipt replay", replay, err)
	}
	if _, err := f.app.ProcessGoalPlanningIntent(f.ctx, strings.Split(run, "@")[0]); err != nil || calls != beforeCalls {
		t.Fatal("committed Planner recovery called Provider", err, calls)
	}
	// Withdrawal after binding cannot be revived by the old native fact.
	cancelled, err := f.app.DispatchOwnerSurface(f.ctx, "POST", base+"/"+id+"/cancel", nil, f.ownerID, map[string]any{"expected_revision": p["revision"], "reason": "撤回参与，保留先前讨论", "idempotency_key": "cancel-pending-promise"})
	if err != nil {
		t.Fatal(err, cancelled)
	}
	p, err = f.app.OwnerPromise(f.ctx, f.ownerID, f.fluctlightID, id)
	if err != nil || p["lifecycle"] != "cancelled" {
		t.Fatal(p, err)
	}
	if _, err := f.app.ProcessCognitionInbox(f.ctx, inbox); err != nil || calls != beforeCalls {
		t.Fatal("cancelled replay called model", err)
	}
	history, err := f.app.OwnerPromiseHistory(f.ctx, f.ownerID, f.fluctlightID, id, url.Values{})
	if err != nil || len(arrayValue(history["items"])) != 4 {
		t.Fatal(history, err)
	}
}

func TestPromiseExistingGoalAcceptRescheduleAndCancelReuseAuthorities(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	goalID := plannerOwnerGoal(t, f, "共同画完小画", false)
	// A normal tactical Commitment already exists. A promise must not replace
	// it, and cancelling one promise must not cancel another participation.
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		if err := lockLifeContextTx(f.ctx, tx, f.fluctlightID); err != nil {
			return err
		}
		var revision int
		if err := tx.QueryRow(f.ctx, `SELECT revision FROM public.fluctlight_goals WHERE id=$1`, goalID).Scan(&revision); err != nil {
			return err
		}
		goal, err := loadGoalAuthorityTx(f.ctx, tx, f.fluctlightID, "goal:ctx_"+stableDigest(goalID), ContextReference{EntityID: goalID, Revision: revision})
		if err != nil {
			return err
		}
		return applyGoalCommitmentPlanTx(f.ctx, tx, goal, GoalCommitmentPlan{Operation: "create", ExpectedResult: "整理已有的画具", Criteria: []string{"画具已实际整理"}}, "existing-tactical-step", f.app.now())
	}); err != nil {
		t.Fatal(err)
	}
	title := "画好一张具体的小画"
	calls := 0
	router := newFakeProviderRouter().otherwise(func(payload map[string]any) fakeProviderResult {
		calls++
		if payloadHasToolResult(payload) {
			return fakeProviderResult{Structured: map[string]any{"decision": "accepted", "reason": "已记录具体安排，履行以后续证据为准"}}
		}
		return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("promise-create", intentionDecideCapabilityName, map[string]any{"operation": "create", "goal": "共同画完小画", "action": title, "expected_outcome": "实际完成并展示这张小画", "reason": "按提议建立具体执行意向"})}}
	})
	f.app.Provider.HTTP = &http.Client{Transport: router}
	seedCognitiveProviderRole(t, f.ctx, f.repository, "promise-accept-"+f.suffix)
	base := "/internal/fluctlights/" + f.fluctlightID + "/promises"
	end := f.app.now().Add(24 * time.Hour).UTC()
	admitted, err := f.app.DispatchOwnerSurface(f.ctx, "POST", base, nil, f.ownerID, map[string]any{"title": title, "due_at": end.Format(time.RFC3339Nano), "reason": "约定可观察的小画成果", "idempotency_key": "accept-promise"})
	if err != nil {
		t.Fatal(err)
	}
	request := mapValue(admitted["request"])
	id := stringValue(request["id"])
	if _, err := f.app.ProcessCognitionInbox(f.ctx, stringValue(request["source_fact_id"])); err != nil {
		t.Fatal(err)
	}
	p, err := f.app.OwnerPromise(f.ctx, f.ownerID, f.fluctlightID, id)
	if err != nil || p["lifecycle"] != "accepted" || p["goal_id"] != goalID || stringValue(p["commitment_id"]) == "" || stringValue(p["intention_id"]) == "" {
		t.Fatal(p, err)
	}
	if mapValue(p["intention"])["commitment_id"] != p["commitment_id"] {
		t.Fatal("Intention not governed by actual promise commitment", p)
	}
	title = "为同一项目准备另一张小画"
	second, err := f.app.DispatchOwnerSurface(f.ctx, "POST", base, nil, f.ownerID, map[string]any{"title": title, "due_at": end.Format(time.RFC3339Nano), "reason": "独立参与，保留第一条约定", "idempotency_key": "second-promise"})
	if err != nil {
		t.Fatal(err)
	}
	secondRequest := mapValue(second["request"])
	if _, err := f.app.ProcessCognitionInbox(f.ctx, stringValue(secondRequest["source_fact_id"])); err != nil {
		t.Fatal(err)
	}
	secondPromise, err := f.app.OwnerPromise(f.ctx, f.ownerID, f.fluctlightID, stringValue(secondRequest["id"]))
	if err != nil || secondPromise["lifecycle"] != "accepted" || secondPromise["commitment_id"] == p["commitment_id"] {
		t.Fatal("independent participation lost", secondPromise, err)
	}
	nextEnd := end.Add(12 * time.Hour).Format(time.RFC3339Nano)
	router.otherwise(func(payload map[string]any) fakeProviderResult {
		calls++
		if payloadHasToolResult(payload) {
			return fakeProviderResult{Structured: map[string]any{"decision": "accepted", "reason": "接受新期限"}}
		}
		return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("promise-reschedule", intentionDecideCapabilityName, map[string]any{"operation": "update", "intention_id": p["intention_id"], "reason": "接受提议的新期限"})}}
	})
	changed, err := f.app.DispatchOwnerSurface(f.ctx, "POST", base+"/"+id+"/reschedule", nil, f.ownerID, map[string]any{"expected_revision": p["revision"], "due_at": nextEnd, "reason": "申请更宽松的履行期限", "idempotency_key": "reschedule-promise"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.ProcessCognitionInbox(f.ctx, stringValue(mapValue(changed["request"])["source_fact_id"])); err != nil {
		t.Fatal(err)
	}
	next, err := f.app.OwnerPromise(f.ctx, f.ownerID, f.fluctlightID, id)
	if err != nil || next["commitment_id"] != p["commitment_id"] || next["intention_id"] != p["intention_id"] || intValue(next["revision"]) != intValue(p["revision"])+1 {
		t.Fatal(next, err)
	}
	stale := map[string]any{"expected_revision": p["revision"], "reason": "旧版本不能撤销新约定", "idempotency_key": "stale-cancel"}
	_, err = f.app.DispatchOwnerSurface(f.ctx, "POST", base+"/"+id+"/cancel", nil, f.ownerID, stale)
	var conflict *SurfaceError
	if !errors.As(err, &conflict) || conflict.Status != 409 {
		t.Fatal("stale promise accepted", err)
	}
	stale["expected_revision"], stale["idempotency_key"] = next["revision"], "real-cancel"
	if _, err := f.app.DispatchOwnerSurface(f.ctx, "POST", base+"/"+id+"/cancel", nil, f.ownerID, stale); err != nil {
		t.Fatal(err)
	}
	final, err := f.app.OwnerPromise(f.ctx, f.ownerID, f.fluctlightID, id)
	if err != nil || final["lifecycle"] != "cancelled" || mapValue(final["intention"])["status"] != "cancelled" || mapValue(final["commitment"])["status"] != "cancelled" {
		t.Fatal(final, err)
	}
	var status string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status FROM public.fluctlight_goals WHERE id=$1`, goalID).Scan(&status); err != nil || status != "active" {
		t.Fatal("promise cancellation destroyed independent Goal", status, err)
	}
	var remaining int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.goal_commitments WHERE goal_id=$1 AND status='active'`, goalID).Scan(&remaining); err != nil || remaining != 2 {
		t.Fatal("cancellation affected another promise or tactical step", remaining, err)
	}
}

func TestPromiseCancelBeforeAssessmentSettlesOldInboxWithoutModel(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	base := "/internal/fluctlights/" + f.fluctlightID + "/promises"
	accepted, err := f.app.DispatchOwnerSurface(f.ctx, "POST", base, nil, f.ownerID, map[string]any{"title": "撤回测试", "due_at": f.app.now().Add(time.Hour).Format(time.RFC3339Nano), "reason": "先提出", "idempotency_key": "propose-before-cancel"})
	if err != nil {
		t.Fatal(err)
	}
	request := mapValue(accepted["request"])
	id := stringValue(request["id"])
	if _, err := f.app.DispatchOwnerSurface(f.ctx, "POST", base+"/"+id+"/cancel", nil, f.ownerID, map[string]any{"expected_revision": 1, "reason": "判断前撤回", "idempotency_key": "withdraw-before-assessment"}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.ProcessCognitionInbox(context.Background(), stringValue(request["source_fact_id"])); err != nil {
		t.Fatal(err)
	}
	var state string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status FROM public.cognition_inbox WHERE id=$1`, request["source_fact_id"]).Scan(&state); err != nil || state != "processed" {
		t.Fatal("old claim was stranded", state, err)
	}
	f.requireNoModelRuns(t)
}
