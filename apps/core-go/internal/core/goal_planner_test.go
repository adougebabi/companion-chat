package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fluctlight/local-ai-companion/apps/core-go/internal/capability"
	"github.com/jackc/pgx/v5"
)

func plannerOwnerGoal(t *testing.T, f independentToolE2EFixture, label string, candidate bool) string {
	t.Helper()
	outcome, motivation := label, "新的可完成需求 "+label
	result, err := f.app.ApplyOwnerGoalCommand(f.ctx, f.ownerID, f.fluctlightID, "", GoalOwnerCommand{Operation: "create", IdempotencyKey: "seed-" + label, Reason: "Owner explicit goal", DesiredOutcome: &outcome, Motivation: &motivation, SuccessCriteria: []string{"真实观察 " + label}, CandidateOnly: candidate})
	if err != nil {
		t.Fatal(err)
	}
	return stringValue(result["goal_id"])
}
func plannerSnapshot(t *testing.T, f independentToolE2EFixture) map[string]any {
	t.Helper()
	s, e := f.app.GoalSetSnapshot(f.ctx, f.ownerID, f.fluctlightID, "*")
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func plannerCommand(t *testing.T, f independentToolE2EFixture, key string) GoalSetCommand {
	s := plannerSnapshot(t, f)
	return GoalSetCommand{ExpectedVersion: int64(intValue(s["revision"])), ExpectedFactsRevision: stringValue(s["facts_revision"]), IdempotencyKey: key, Reason: "Owner reviewed the final set"}
}
func TestGoalPlannerCapacityConcurrentAllEntrances(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	for i := 0; i < 4; i++ {
		plannerOwnerGoal(t, f, fmt.Sprint("old-", i), false)
	}
	paused := plannerOwnerGoal(t, f, "candidate", true)
	var wg sync.WaitGroup
	var mu sync.Mutex
	success := 0
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			outcome, motivation := fmt.Sprint("new-", i), "Owner new need"
			command := GoalOwnerCommand{Operation: "create", IdempotencyKey: fmt.Sprint("parallel-", i), Reason: "capacity competition", DesiredOutcome: &outcome, Motivation: &motivation, SuccessCriteria: []string{"real result"}}
			id := ""
			if i == 0 {
				command.Operation = "resume"
				command.ExpectedRevision = 1
				id = paused
			}
			_, err := f.app.ApplyOwnerGoalCommand(f.ctx, f.ownerID, f.fluctlightID, id, command)
			if err == nil {
				mu.Lock()
				success++
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	s := plannerSnapshot(t, f)
	if intValue(s["active_count"]) != 5 || success != 1 {
		t.Fatalf("capacity count=%v successes=%d", s["active_count"], success)
	}
	// Separate connection and direct SQL are guarded too.
	_, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.fluctlight_goals(id,fluctlight_id,desired_outcome,description,status) VALUES($1,$2,'sixth','sixth','active')`, "sixth-"+f.suffix, f.fluctlightID)
	if err == nil || !strings.Contains(err.Error(), "goal_capacity_exceeded") {
		t.Fatalf("SQL guard %v", err)
	}
}
func TestGoalPlannerAtomicDependenciesOrderAndReplay(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	a := plannerOwnerGoal(t, f, "A", false)
	b := plannerOwnerGoal(t, f, "B", false)
	c := plannerCommand(t, f, "manual-order")
	c.Order = []string{b, a}
	c.OrderingMode = "manual"
	c.Dependencies = map[string][]string{b: {a}}
	result, err := f.app.ApplyGoalSetCommand(f.ctx, f.ownerID, f.fluctlightID, c)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := f.app.ApplyGoalSetCommand(f.ctx, f.ownerID, f.fluctlightID, c)
	if err != nil || jsonString(result) != jsonString(replay) {
		t.Fatal("lost response replay changed", err)
	}
	cycle := plannerCommand(t, f, "cycle")
	cycle.Dependencies = map[string][]string{a: {b}}
	if _, err := f.app.ApplyGoalSetCommand(f.ctx, f.ownerID, f.fluctlightID, cycle); err == nil {
		t.Fatal("cycle accepted")
	}
	var count int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.goal_dependencies WHERE fluctlight_id=$1`, f.fluctlightID).Scan(&count); err != nil || count != 1 {
		t.Fatal("partial dependency committed", count, err)
	}
	// Admission gates an old linked intention at the actual execution seam.
	err = withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error { return requireGoalDependenciesTx(f.ctx, tx, f.fluctlightID, b) })
	if err == nil {
		t.Fatal("uncompleted prerequisite permitted action")
	}
	bad := plannerCommand(t, f, "rollback")
	bad.Changes = []GoalSetChange{{ID: a, Operation: "pause", ExpectedRevision: 1}, {ID: "new", Operation: "create", Activate: true, Outcome: "new", Criteria: []string{"real"}, Motivation: "new need"}}
	bad.Dependencies = map[string][]string{"new": {"foreign"}}
	if _, err := f.app.ApplyGoalSetCommand(f.ctx, f.ownerID, f.fluctlightID, bad); err == nil {
		t.Fatal("invalid batch accepted")
	}
	s := plannerSnapshot(t, f)
	if intValue(s["active_count"]) != 2 {
		t.Fatal("partial lifecycle committed")
	}
	// Owner order survives new activation and App reconstruction.
	n := plannerOwnerGoal(t, f, "new-after-manual", false)
	fresh := *f.app
	fresh.Capabilities = nil
	fresh.Runtime = nil
	s, err = fresh.GoalSetSnapshot(f.ctx, f.ownerID, f.fluctlightID, "*")
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for _, g := range arrayValue(s["goals"]) {
		ids = append(ids, stringValue(mapValue(g)["id"]))
	}
	if strings.Join(ids, ",") != strings.Join([]string{b, a, n}, ",") {
		t.Fatalf("manual order %v", ids)
	}
}
func TestGoalPlannerPolicyAndActorVersionFences(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	id := plannerOwnerGoal(t, f, "target", false)
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlight_goals SET target_actor_id=$2 WHERE id=$1`, id, f.ownerID); err != nil {
		t.Fatal(err)
	}
	actor, err := f.app.ActorContext(f.ctx, f.ownerID, f.fluctlightID, f.ownerID)
	if err != nil {
		t.Fatal(err)
	}
	body := map[string]any{"background": map[string]any{"location": "国外", "timezone": "Europe/Paris", "interests": "摄影"}, "expected_context_version": actor["context_version"], "idempotency_key": "actor-edit", "operation": "change", "reason": "Owner correct context"}
	updated, err := f.app.UpdateActorContext(f.ctx, f.ownerID, f.fluctlightID, f.ownerID, body)
	if err != nil {
		t.Fatal(err)
	}
	if updated["context_version"] == actor["context_version"] {
		t.Fatal("Actor version unchanged")
	}
	body["idempotency_key"] = "stale-edit"
	if _, err := f.app.UpdateActorContext(f.ctx, f.ownerID, f.fluctlightID, f.ownerID, body); err == nil {
		t.Fatal("stale edit accepted")
	}
	if _, err := f.app.ActorContext(f.ctx, f.foreignOwnerID, f.fluctlightID, f.ownerID); err == nil {
		t.Fatal("foreign Owner admitted")
	}
	err = withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error { return requireGoalDependenciesTx(f.ctx, tx, f.fluctlightID, id) })
	if err == nil {
		t.Fatal("stale background action permitted")
	}
	c := plannerCommand(t, f, "disabled")
	off := false
	c.AutoPlanningEnabled = &off
	if _, err := f.app.ApplyGoalSetCommand(f.ctx, f.ownerID, f.fluctlightID, c); err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.RequestGoalPlanning(f.ctx, f.ownerID, f.fluctlightID, "manual-disabled"); err != nil {
		t.Fatal(err)
	}
	var run string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT id FROM public.goal_planning_runs WHERE fluctlight_id=$1 ORDER BY created_at LIMIT 1`, f.fluctlightID).Scan(&run); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_planning_runs SET status='processing',claim_revision=2,mode='apply',claimed_at=now() WHERE id=$1`, run); err != nil {
		t.Fatal(err)
	}
	c = plannerCommand(t, f, "late-plan")
	c.Changes = []GoalSetChange{{ID: "late", Operation: "create", Activate: true, Outcome: "late", Criteria: []string{"real"}, Motivation: "new"}}
	err = withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		_, err := f.app.applyGoalSetTx(f.ctx, tx, f.ownerID, f.fluctlightID, "", run+"@2", c)
		return err
	})
	if err == nil {
		t.Fatal("closed switch permitted late plan")
	}
	err = withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		_, err := f.app.applyGoalSetTx(f.ctx, tx, f.ownerID, f.fluctlightID, "", run+"@1", c)
		return err
	})
	if err == nil {
		t.Fatal("stale lease admitted")
	}
}
func TestGoalPlannerNativeLoopConsumesQueryCommitAndReread(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	plannerOwnerGoal(t, f, "existing", false)
	seedCognitiveProviderRole(t, f.ctx, f.repository, "planner-model-"+f.suffix)
	if _, err := f.app.RequestGoalPlanning(f.ctx, f.ownerID, f.fluctlightID, "native-loop"); err != nil {
		t.Fatal(err)
	}
	var runID string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT id FROM public.goal_planning_runs WHERE fluctlight_id=$1 LIMIT 1`, f.fluctlightID).Scan(&runID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_planning_runs SET available_at=now()-interval '1 second' WHERE id=$1`, runID); err != nil {
		t.Fatal(err)
	}
	calls := 0
	var toolSnapshot map[string]any
	var committedID string
	call := func(name string, args any) fakeProviderResult {
		return fakeProviderResult{ToolCalls: []map[string]any{{"id": fmt.Sprint("planner-call-", calls), "type": "function", "function": map[string]any{"name": name, "arguments": jsonString(args)}}}}
	}
	router := newFakeProviderRouter().on("goal_planner_v1", func(payload map[string]any) fakeProviderResult {
		calls++
		messages := arrayValue(payload["messages"])
		for _, m := range messages {
			v := mapValue(m)
			if stringValue(v["role"]) == "tool" {
				raw := stringValue(v["content"])
				var result map[string]any
				_ = json.Unmarshal([]byte(raw), &result)
				output := mapValue(result["output"])
				if len(output) == 0 {
					output = mapValue(mapValue(result["result"])["output"])
				}
				if output["facts_revision"] != nil {
					toolSnapshot = output
				}
				for _, g := range arrayValue(output["applied"]) {
					committedID = stringValue(mapValue(g)["goal_id"])
				}
			}
		}
		switch calls {
		case 1:
			return call(goalPlannerQuery, map[string]any{"section": "snapshot"})
		case 2:
			if toolSnapshot == nil {
				t.Fatalf("model did not receive actual snapshot result: %s", jsonString(messages))
			}
			return call(goalPlannerQuery, map[string]any{"section": "history", "query": "摄影"})
		case 3:
			return call(goalPlannerCommit, map[string]any{"expected_version": toolSnapshot["revision"], "expected_facts_revision": toolSnapshot["facts_revision"], "idempotency_key": runID + "@1", "reason": "Stable interest and available window; new photography theme differs from history", "changes": []any{map[string]any{"id": "photo", "operation": "create", "activate": true, "desired_outcome": "向对方分享街景摄影想法", "success_criteria": []string{"实际发送一段街景摄影想法"}, "motivation": "摄影兴趣与可用时间", "source": map[string]any{"history_distinction": "新主题", "source_refs": []string{"persona:default"}}}}})
		case 4:
			if committedID == "" {
				t.Fatalf("model did not receive committed id: %s", jsonString(messages))
			}
			return call(goalPlannerQuery, map[string]any{"section": "snapshot"})
		default:
			return fakeProviderResult{Structured: map[string]any{"decision": "applied", "reason": "正式提交并回读 " + committedID, "review_condition": "新的实际结果", "suggestions": []any{}}}
		}
	})
	router.on("conversation_turn_response", func(_ map[string]any) fakeProviderResult {
		return fakeProviderResult{Structured: map[string]any{"action_type": "reply", "response_intent": "分享一个可行的摄影想法", "visible_text": "我想尝试以窗边光影为主题拍摄街景，这只是拍摄想法，还没有拍出作品。", "influences": []any{}, "goal_event_candidates": []any{map[string]any{"goal_ref": "goal:ctx_" + stableDigest(committedID), "reason": "实际分享摄影想法"}}}}
	})
	router.on("goal_evaluation_v1", func(_ map[string]any) fakeProviderResult {
		snapshot := readProcessingGoalSnapshot(t, f)
		var proof string
		for _, source := range snapshot.Sources {
			if source.Kind == "message" && source.SubjectActorID == f.fluctlightID && strings.Contains(stringValue(source.Data["text"]), "窗边光影") {
				proof = source.Ref
			}
		}
		evaluations := []GoalEvaluationCandidate{}
		for _, entry := range snapshot.Goals {
			candidate := GoalEvaluationCandidate{GoalID: entry.GoalID, ExpectedRevision: entry.Goal.Revision, CriteriaVersion: entry.Goal.CriteriaVersion, Impact: "no_change", Judgments: []GoalCriterionJudgment{{CriterionID: entry.Goal.CriterionIDs[0], Verdict: "unknown", Kind: "communication", Subject: "actor_self", Discourse: "uncertain", EvidenceRefs: []string{}, Reason: "没有这个目标的真实结果"}}}
			if entry.GoalID == committedID {
				if proof == "" {
					t.Fatal("actual published message missing")
				}
				candidate.Impact = "completed"
				candidate.Judgments = []GoalCriterionJudgment{{CriterionID: entry.Goal.CriterionIDs[0], Verdict: "satisfied", Kind: "communication", Subject: "actor_self", Discourse: "assertion", EvidenceRefs: []string{proof}, Reason: "真实发送的想法满足表达标准，不证明作品已产生"}}
			}
			evaluations = append(evaluations, candidate)
		}
		return fakeProviderResult{Structured: goalEvaluationProviderFixture(snapshot, GoalEvaluationTaskOutput{Evaluations: evaluations, Plans: []GoalPlanCandidate{}})}
	})
	f.app.Provider.HTTP = &http.Client{Transport: router}
	result, err := f.app.ProcessGoalPlanningIntent(f.ctx, runID)
	if err != nil || result["decision"] != "applied" {
		t.Fatalf("native planner result=%s err=%v calls=%d", jsonString(result), err, calls)
	}
	if committedID == "" || intValue(mapValue(result["snapshot"])["active_count"]) != 2 || calls != 5 {
		t.Fatalf("no real commit/reread: %v", result)
	}
	replay, err := f.app.ProcessGoalPlanningIntent(f.ctx, runID)
	if err != nil || replay["decision"] != "applied" || calls != 5 {
		t.Fatal("restart replay called model", err, calls)
	}
	turn, err := f.app.HandleTurn(f.ctx, f.ownerID, f.conversationID, map[string]any{"fluctlight_id": f.fluctlightID, "text": "你有什么摄影想法？", "idempotency_key": "planner-new-goal-expression"})
	if err != nil || stringValue(turn.Assistant["id"]) == "" {
		t.Fatal("new goal did not reach production conversation", err)
	}
	evaluationID := latestPendingGoalRequest(t, f)
	if _, err := f.app.ProcessGoalEvaluationIntent(f.ctx, evaluationID); err != nil {
		t.Fatal(err)
	}
	var status string
	var resolutions int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status,(SELECT count(*) FROM public.goal_resolutions WHERE goal_id=$1) FROM public.fluctlight_goals WHERE id=$1`, committedID).Scan(&status, &resolutions); err != nil || status != "completed" || resolutions != 1 {
		t.Fatal("new goal did not complete original evidence chain", status, resolutions, err)
	}
	t.Logf("NEW_GOAL_CLOSURE goal=%s status=%s resolutions=%d original_evidence_chain=true", committedID, status, resolutions)
	t.Logf("SCRIPT_PROVIDER_EVIDENCE run=%s goal=%s calls=%d tools=%v results=%v active=2", runID, committedID, calls, result["tool_calls"], result["tool_results"])
}

var _ = time.Second

func TestGoalPlannerInitialImportCapacityAndRestart(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	goals := []any{}
	for i := 0; i < 8; i++ {
		goals = append(goals, map[string]any{"source_id": fmt.Sprint("source-", i), "description": fmt.Sprint("初始表达-", i), "success_criteria": []string{"真实表达"}, "motivation": "初始化明确愿望"})
	}
	importOnce := func() {
		t.Helper()
		if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
			return f.app.insertAgency(f.ctx, tx, f.fluctlightID, f.ownerID, goals, []any{}, map[string]struct{}{"default": {}})
		}); err != nil {
			t.Fatal(err)
		}
	}
	importOnce()
	s := plannerSnapshot(t, f)
	if intValue(s["active_count"]) != 5 || len(arrayValue(s["goals"])) != 8 {
		t.Fatal("initial input lost or exceeded capacity", s)
	}
	id := fmt.Sprintf("goal_initial_%s_0", f.fluctlightID)
	if _, err := f.app.ApplyOwnerGoalCommand(f.ctx, f.ownerID, f.fluctlightID, id, GoalOwnerCommand{Operation: "pause", ExpectedRevision: 1, IdempotencyKey: "pause-initial", Reason: "Owner does not want this now"}); err != nil {
		t.Fatal(err)
	}
	importOnce()
	goals[0] = map[string]any{"source_id": "source-0", "description": "换名的旧愿望", "success_criteria": []string{"real"}, "motivation": "修改来源不是新增授权"}
	importOnce()
	var status string
	var imports, total int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status FROM public.fluctlight_goals WHERE id=$1`, id).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT (SELECT count(*) FROM public.goal_initial_imports WHERE fluctlight_id=$1),(SELECT count(*) FROM public.fluctlight_goals WHERE fluctlight_id=$1)`, f.fluctlightID).Scan(&imports, &total); err != nil || status != "paused" || imports != 8 || total != 8 {
		t.Fatal("import revived or duplicated", status, imports, total, err)
	}
	t.Logf("INITIAL_IMPORT_EVIDENCE imports=%d total=%d active=4 candidate=3 Owner-paused preserved", imports, total)
}
func TestGoalPlannerToolsIndependentSuccessFailureAndPolicy(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	plannerOwnerGoal(t, f, "visible", false)
	for _, section := range []string{"snapshot", "persona", "life", "actor", "history", "triggers"} {
		t.Run(section, func(t *testing.T) {
			request := f.request(goalPlannerQuery, "query-"+section, map[string]any{"section": section})
			request.Surface = CapabilitySurfaceGoalPlanner
			r, err := f.app.ExecuteTool(f.ctx, request)
			if err != nil || r.Result.Status != "completed" {
				t.Fatalf("query %s: %v %v", section, r, err)
			}
		})
	}
	request := f.request(goalPlannerQuery, "foreign-query", map[string]any{"section": "snapshot"})
	request.Surface = CapabilitySurfaceGoalPlanner
	request.AuthorizationActorID = f.foreignOwnerID
	if _, err := f.app.ExecuteTool(f.ctx, request); err == nil {
		t.Fatal("foreign query accepted")
	}
	request = f.request(goalPlannerQuery, "hidden-actor", map[string]any{"section": "actor", "actor_id": f.foreignOwnerID})
	request.Surface = CapabilitySurfaceGoalPlanner
	if _, err := f.app.ExecuteTool(f.ctx, request); err == nil {
		t.Fatal("hidden Actor accepted")
	}
	s := plannerSnapshot(t, f)
	request = f.request(goalPlannerCommit, "no-run-target", map[string]any{"expected_version": s["revision"], "expected_facts_revision": s["facts_revision"], "idempotency_key": "direct", "reason": "no trusted run"})
	request.Surface = CapabilitySurfaceGoalPlanner
	if _, err := f.app.ExecuteTool(f.ctx, request); err == nil {
		t.Fatal("commit without trusted run accepted")
	}
	f.requireNoModelRuns(t)
}
func TestGoalPlannerConcurrentDependencyGraphAndStockMigration(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	a := plannerOwnerGoal(t, f, "A", false)
	b := plannerOwnerGoal(t, f, "B", false)
	base := plannerSnapshot(t, f)
	var wg sync.WaitGroup
	var mu sync.Mutex
	success := 0
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			g, p := a, b
			if i == 1 {
				g, p = b, a
			}
			_, err := f.app.ApplyGoalSetCommand(f.ctx, f.ownerID, f.fluctlightID, GoalSetCommand{ExpectedVersion: int64(intValue(base["revision"])), ExpectedFactsRevision: stringValue(base["facts_revision"]), IdempotencyKey: fmt.Sprint("cycle-race-", i), Reason: "parallel graph edit", Dependencies: map[string][]string{g: {p}}})
			if err == nil {
				mu.Lock()
				success++
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	if success != 1 {
		t.Fatal("concurrent graph version fence", success)
	}
	for i := 0; i < 6; i++ {
		plannerOwnerGoal(t, f, fmt.Sprint("stock-", i), true)
	}
	seedLegacyGoalActivityForTest(t, f)
	snapshot := plannerSnapshot(t, f)
	if snapshot["capacity_violation"] != true || intValue(snapshot["active_count"]) != 8 {
		t.Fatal("legacy stock silently removed", snapshot)
	}
	outcome, motivation := "new", "new"
	if _, err := f.app.ApplyOwnerGoalCommand(f.ctx, f.ownerID, f.fluctlightID, "", GoalOwnerCommand{Operation: "create", IdempotencyKey: "over-stock", Reason: "must block", DesiredOutcome: &outcome, Motivation: &motivation, SuccessCriteria: []string{"real"}}); err == nil {
		t.Fatal("excess allowed activation")
	}
	dry, err := f.app.MigrateGoalPlannerSources(f.ctx, f.ownerID, f.fluctlightID, "", 3, false)
	if err != nil || len(arrayValue(dry["items"])) != 3 || dry["capacity_violation"] != true {
		t.Fatal("migration dry-run", dry, err)
	}
	first, err := f.app.MigrateGoalPlannerSources(f.ctx, f.ownerID, f.fluctlightID, "", 3, true)
	if err != nil {
		t.Fatal(err)
	}
	again, err := f.app.MigrateGoalPlannerSources(f.ctx, f.ownerID, f.fluctlightID, "", 3, true)
	if err != nil || first["linked"] != again["linked"] {
		t.Fatal("migration not restartable", first, again, err)
	}
}
func TestGoalPlannerProviderFailureAndQuietDisabledRequests(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	plannerOwnerGoal(t, f, "old", false)
	seedCognitiveProviderRole(t, f.ctx, f.repository, "failure-"+f.suffix)
	if _, err := f.app.RequestGoalPlanning(f.ctx, f.ownerID, f.fluctlightID, "fail-provider"); err != nil {
		t.Fatal(err)
	}
	var run string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT id FROM public.goal_planning_runs WHERE fluctlight_id=$1`, f.fluctlightID).Scan(&run); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_planning_runs SET available_at=now() WHERE id=$1`, run); err != nil {
		t.Fatal(err)
	}
	f.app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().on("goal_planner_v1", func(_ map[string]any) fakeProviderResult { return fakeProviderResult{Status: 503} })}
	result, err := f.app.ProcessGoalPlanningIntent(f.ctx, run)
	if err != nil || result["decision"] != "failed" || result["status"] != "deferred" {
		t.Fatal("failure misclassified as no candidate", result, err)
	}
	deferred, err := f.app.ProcessGoalPlanningIntent(f.ctx, run)
	if err != nil || deferred["status"] != "deferred" {
		t.Fatal("retry backoff bypassed", deferred, err)
	}
	var active int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.fluctlight_goals WHERE fluctlight_id=$1 AND status='active'`, f.fluctlightID).Scan(&active); err != nil || active != 1 {
		t.Fatal("Provider failure changed set")
	}
}

func TestGoalPlannerCommitToolStandaloneReplayConflictAndProtection(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	id := plannerOwnerGoal(t, f, "protected", false)
	locked := true
	if _, err := f.app.ApplyOwnerGoalCommand(f.ctx, f.ownerID, f.fluctlightID, id, GoalOwnerCommand{Operation: "update", ExpectedRevision: 1, IdempotencyKey: "lock", Reason: "Owner preserve", OwnerProtected: &locked}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.RequestGoalPlanning(f.ctx, f.ownerID, f.fluctlightID, "standalone"); err != nil {
		t.Fatal(err)
	}
	var run string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT id FROM public.goal_planning_runs WHERE fluctlight_id=$1 LIMIT 1`, f.fluctlightID).Scan(&run); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_planning_runs SET status='processing',claim_revision=1,claimed_at=now(),mode='apply' WHERE id=$1`, run); err != nil {
		t.Fatal(err)
	}
	s := plannerSnapshot(t, f)
	args := map[string]any{"expected_version": s["revision"], "expected_facts_revision": s["facts_revision"], "idempotency_key": "standalone-commit", "reason": "new distinct need", "changes": []any{map[string]any{"id": "new", "operation": "create", "activate": true, "desired_outcome": "独立Tool目标", "success_criteria": []string{"真实结果"}, "motivation": "Owner测试明确新需求"}}}
	request := f.request(goalPlannerCommit, "standalone-commit", args)
	request.Surface = CapabilitySurfaceGoalPlanner
	request.TargetKind = "goal_planning_run"
	request.TargetRef = run + "@1"
	protectedRequest := request
	protectedRequest.OperationID = "protected-before-commit"
	protectedRequest.Arguments = jsonBytes(map[string]any{"expected_version": s["revision"], "expected_facts_revision": s["facts_revision"], "idempotency_key": "protected-before-commit", "reason": "model cannot override Owner retention", "changes": []any{map[string]any{"id": id, "operation": "pause", "expected_revision": 2}}})
	if _, err := f.app.ExecuteTool(f.ctx, protectedRequest); !errors.Is(err, ErrUnauthorized) {
		t.Fatal("protected goal branch not rejected by permission", err)
	}
	first, err := f.app.ExecuteTool(f.ctx, request)
	if err != nil || first.Result.Status != "completed" {
		t.Fatal("standalone commit", first, err)
	}
	replay, err := f.app.ExecuteTool(f.ctx, request)
	if err != nil || !replay.Replayed || jsonString(first.Result.Output) != jsonString(replay.Result.Output) {
		t.Fatal("standalone receipt replay", replay, err)
	}
	s = plannerSnapshot(t, f)
	args = map[string]any{"expected_version": s["revision"], "expected_facts_revision": s["facts_revision"], "idempotency_key": "protected-pause", "reason": "model wants capacity", "changes": []any{map[string]any{"id": id, "operation": "pause", "expected_revision": 2}}}
	request.OperationID = "protected-pause"
	request.Arguments = jsonBytes(args)
	failed, err := f.app.ExecuteTool(f.ctx, request)
	if err == nil || failed.Result.Status == "completed" {
		t.Fatal("Planner paused protected Owner goal", failed, err)
	}
	f.requireNoModelRuns(t)
}
func TestGoalPlannerNoViableCandidateIsQuietAndDistinctFromFailure(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	seedCognitiveProviderRole(t, f.ctx, f.repository, "empty-model-"+f.suffix)
	if _, err := f.app.RequestGoalPlanning(f.ctx, f.ownerID, f.fluctlightID, "no-direction"); err != nil {
		t.Fatal(err)
	}
	var run string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT id FROM public.goal_planning_runs WHERE fluctlight_id=$1`, f.fluctlightID).Scan(&run); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_planning_runs SET available_at=now() WHERE id=$1`, run); err != nil {
		t.Fatal(err)
	}
	calls := 0
	f.app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().on("goal_planner_v1", func(payload map[string]any) fakeProviderResult {
		calls++
		if calls <= 2 {
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall(fmt.Sprint("empty-", calls), goalPlannerQuery, map[string]any{"section": map[int]string{1: "snapshot", 2: "history"}[calls]})}}
		}
		if !payloadHasToolResult(payload) {
			t.Fatal("empty decision lacks real query result")
		}
		return fakeProviderResult{Structured: map[string]any{"decision": "no_viable_candidate", "reason": "当前输入没有可行的新方向，空位本身不构成动机", "review_condition": "收到真实新愿望或背景变化后复核", "suggestions": []any{}}}
	})}
	result, err := f.app.ProcessGoalPlanningIntent(f.ctx, run)
	if err != nil || result["decision"] != "no_viable_candidate" || intValue(mapValue(result["snapshot"])["active_count"]) != 0 {
		t.Fatal("empty decision invalid", result, err)
	}
	for i := 0; i < 3; i++ {
		if _, err := f.app.ProcessGoalPlanningIntent(f.ctx, run); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 3 {
		t.Fatal("empty snapshot caused hot model retry", calls)
	}
	t.Logf("EMPTY_SET_EVIDENCE run=%s model_calls=%d active=0 reason=%v review=%v", run, calls, result["reason"], result["review_condition"])
}

func TestGoalPlannerE2EAllGoalsFinishThenNewGoalFinishesOriginalChain(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	seedCognitiveProviderRole(t, f.ctx, f.repository, "all-finish-"+f.suffix)
	oldIDs := []string{}
	for i, label := range []string{"摄影偏好", "阅读兴趣", "尊重互动边界"} {
		outcome, motivation := "向对方表达"+label, "Owner授权的一次表达"
		r, err := f.app.ApplyOwnerGoalCommand(f.ctx, f.ownerID, f.fluctlightID, "", GoalOwnerCommand{Operation: "create", IdempotencyKey: fmt.Sprint("initial-expression-", i), Reason: "test initial finite goal", DesiredOutcome: &outcome, Motivation: &motivation, SuccessCriteria: []string{"实际明确表达" + label}})
		if err != nil {
			t.Fatal(err)
		}
		oldIDs = append(oldIDs, stringValue(r["goal_id"]))
	}
	plannerCalls, turns := 0, 0
	var snapshot map[string]any
	var newID string
	router := newFakeProviderRouter().on("conversation_turn_response", func(_ map[string]any) fakeProviderResult {
		turns++
		text := "我喜欢摄影构图和阅读科幻，也重视尊重双方的互动边界。"
		if turns > 1 {
			text = "我有一个街景摄影想法：用窗边光影作为主题。这是想法，还没有产生作品。"
		}
		goals, _, err := f.app.agencyProfile(f.ctx, f.fluctlightID)
		if err != nil {
			t.Fatal(err)
		}
		events := []any{}
		for _, g := range goals {
			if g["status"] == "active" {
				events = append(events, map[string]any{"goal_ref": "goal:ctx_" + stableDigest(stringValue(g["id"])), "reason": "实际发送表达，复核真实标准"})
			}
		}
		return fakeProviderResult{Structured: map[string]any{"action_type": "reply", "response_intent": "表达当前真实想法", "visible_text": text, "influences": []any{}, "goal_event_candidates": events}}
	})
	router.on("goal_evaluation_v1", func(_ map[string]any) fakeProviderResult {
		s := readProcessingGoalSnapshot(t, f)
		proof := ""
		for _, source := range s.Sources {
			if source.Kind == "message" && source.SubjectActorID == f.fluctlightID && source.Valid && source.CanSupportSuccess {
				proof = source.Ref
			}
		}
		if proof == "" {
			t.Fatal("actual sent message missing")
		}
		evaluations := []GoalEvaluationCandidate{}
		for _, entry := range s.Goals {
			evaluations = append(evaluations, GoalEvaluationCandidate{GoalID: entry.GoalID, ExpectedRevision: entry.Goal.Revision, CriteriaVersion: entry.Goal.CriteriaVersion, Impact: "completed", Judgments: []GoalCriterionJudgment{{CriterionID: entry.Goal.CriterionIDs[0], Verdict: "satisfied", Kind: "communication", Subject: "actor_self", Discourse: "assertion", EvidenceRefs: []string{proof}, Reason: "实际发送明确表达；不推断关系接受或作品产生"}}})
		}
		return fakeProviderResult{Structured: goalEvaluationProviderFixture(s, GoalEvaluationTaskOutput{Evaluations: evaluations, Plans: []GoalPlanCandidate{}})}
	})
	router.on("goal_planner_v1", func(payload map[string]any) fakeProviderResult {
		plannerCalls++
		for _, m := range arrayValue(payload["messages"]) {
			v := mapValue(m)
			if v["role"] != "tool" {
				continue
			}
			var envelope map[string]any
			_ = json.Unmarshal([]byte(stringValue(v["content"])), &envelope)
			output := mapValue(envelope["output"])
			if output["facts_revision"] != nil {
				snapshot = output
			}
			for _, item := range arrayValue(output["applied"]) {
				newID = stringValue(mapValue(item)["goal_id"])
			}
		}
		section := map[int]string{1: "snapshot", 2: "persona", 3: "history", 4: "actor", 5: "life", 7: "snapshot"}[plannerCalls]
		if section != "" {
			if plannerCalls == 2 && (snapshot == nil || intValue(snapshot["active_count"]) != 0) {
				t.Fatal("Planner did not consume empty active-set result", snapshot)
			}
			if plannerCalls == 7 && newID == "" {
				t.Fatal("Planner did not consume real committed id")
			}
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall(fmt.Sprint("all-finish-", plannerCalls), goalPlannerQuery, map[string]any{"section": section})}}
		}
		if plannerCalls == 6 {
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("all-finish-commit", goalPlannerCommit, map[string]any{"expected_version": snapshot["revision"], "expected_facts_revision": snapshot["facts_revision"], "idempotency_key": "all-finish-plan", "reason": "已表达的偏好保持终态；新的有限摄影主题交流", "changes": []any{map[string]any{"id": "theme", "operation": "create", "activate": true, "desired_outcome": "向对方分享一个街景摄影想法", "success_criteria": []string{"实际发送街景摄影想法"}, "motivation": "摄影兴趣与新的主题", "source": map[string]any{"derived_from_goal_id": oldIDs[0], "source_refs": []string{"goal:" + oldIDs[0]}, "history_distinction": "旧目标是表达偏好，新目标是交流有限创作主题"}}}})}}
		}
		return fakeProviderResult{Structured: map[string]any{"decision": "applied", "reason": "真实提交并回读新目标 " + newID, "review_condition": "新的实际结果或主题变化", "suggestions": []any{}}}
	})
	f.app.Provider.HTTP = &http.Client{Transport: router}
	completeViaConversation := func(key, text string) {
		t.Helper()
		turn, err := f.app.HandleTurn(f.ctx, f.ownerID, f.conversationID, map[string]any{"fluctlight_id": f.fluctlightID, "text": text, "idempotency_key": key})
		if err != nil || stringValue(turn.Assistant["id"]) == "" {
			t.Fatal(turn, err)
		}
		if _, err := f.app.ProcessGoalEvaluationIntent(f.ctx, latestPendingGoalRequest(t, f)); err != nil {
			t.Fatal(err)
		}
	}
	completeViaConversation("old-goals-finish", "请明确告诉我你的兴趣与沟通边界")
	var completed int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.fluctlight_goals WHERE fluctlight_id=$1 AND status='completed'`, f.fluctlightID).Scan(&completed); err != nil || completed != 3 {
		t.Fatal("initial set not actually finished", completed, err)
	}
	var run string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT id FROM public.goal_planning_runs WHERE fluctlight_id=$1 AND status='pending'`, f.fluctlightID).Scan(&run); err != nil {
		t.Fatal("completion did not persist merged planning", err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_planning_runs SET available_at=now() WHERE id=$1`, run); err != nil {
		t.Fatal(err)
	}
	result, err := f.app.ProcessGoalPlanningIntent(f.ctx, run)
	if err != nil || result["decision"] != "applied" || intValue(mapValue(result["snapshot"])["active_count"]) != 1 {
		t.Fatal("complete->plan path failed", result, err)
	}
	completeViaConversation("new-goal-finish", "你现在有什么有限的摄影想法？")
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.fluctlight_goals WHERE fluctlight_id=$1 AND status='completed'`, f.fluctlightID).Scan(&completed); err != nil || completed != 4 {
		t.Fatal("new goal did not use original closure chain", completed, err)
	}
	total, peak, chars := 0, 0, 0
	for _, payload := range router.payloads("goal_planner_v1") {
		tokens := EstimatePromptTokens(payload)
		total += tokens
		if tokens > peak {
			peak = tokens
		}
		chars += len([]rune(jsonString(payload)))
	}
	t.Logf("PLANNER_COST_EVIDENCE physical_calls=%d estimated_input_tokens=%d peak_estimated_input_tokens=%d total_wire_chars=%d real_provider_tokens=unavailable", plannerCalls, total, peak, chars)
	t.Logf("E2E01_EVIDENCE instance=%s old_goals=%v planner_run=%s new_goal=%s completed=4 planner_calls=%d merged_completion_events=true", f.fluctlightID, oldIDs, run, newID, plannerCalls)
}

func TestGoalPlannerConcurrentSameKeyReturnsSameDurableResult(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	plannerOwnerGoal(t, f, "old", false)
	c := plannerCommand(t, f, "same-key-concurrent")
	c.Changes = []GoalSetChange{{ID: "new", Operation: "create", Activate: true, Outcome: "并发同一需求", Criteria: []string{"真实结果"}, Motivation: "同一授权", Source: map[string]any{"source_id": "one"}}}
	var wg sync.WaitGroup
	results := make([]map[string]any, 2)
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = f.app.ApplyGoalSetCommand(f.ctx, f.ownerID, f.fluctlightID, c)
		}(i)
	}
	wg.Wait()
	if errs[0] != nil || errs[1] != nil || jsonString(results[0]) != jsonString(results[1]) {
		t.Fatal("same-key concurrent replay differs", errs, results)
	}
	if _, err := f.app.ApplyGoalSetCommand(f.ctx, f.ownerID, f.fluctlightID, c); err != nil {
		t.Fatal("command source map was mutated", err)
	}
}
func TestGoalPlannerSourceReviewVisibleVersionedAndRestartable(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	id := plannerOwnerGoal(t, f, "已有愿望", false)
	if _, err := f.app.ApplyOwnerGoalCommand(f.ctx, f.ownerID, f.fluctlightID, id, GoalOwnerCommand{Operation: "pause", ExpectedRevision: 1, IdempotencyKey: "hold-source", Reason: "Owner stopped"}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.goal_initial_source_reviews(fluctlight_id,source_id,raw_source) VALUES($1,'legacy-desire','"等待机会表达；长期重视关系"')`, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	c := plannerCommand(t, f, "link-legacy")
	c.SourceReviews = []GoalSourceReviewDecision{{SourceID: "legacy-desire", ExpectedRevision: 1, Operation: "link_goal", GoalID: id}}
	if _, err := f.app.ApplyGoalSetCommand(f.ctx, f.ownerID, f.fluctlightID, c); err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.ApplyGoalSetCommand(f.ctx, f.ownerID, f.fluctlightID, c); err != nil {
		t.Fatal(err)
	}
	var state, status string
	var imports int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT g.status,r.status,(SELECT count(*) FROM public.goal_initial_imports WHERE fluctlight_id=$1) FROM public.fluctlight_goals g JOIN public.goal_initial_source_reviews r ON r.fluctlight_id=g.fluctlight_id WHERE g.id=$2`, f.fluctlightID, id).Scan(&state, &status, &imports); err != nil || state != "paused" || status != "linked_existing" || imports != 1 {
		t.Fatal("source migration revived goal", state, status, imports, err)
	}
}
func TestGoalPlannerPureOrderCommitSurvivesLostFinalResponse(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	a := plannerOwnerGoal(t, f, "A", false)
	b := plannerOwnerGoal(t, f, "B", false)
	if _, err := f.app.RequestGoalPlanning(f.ctx, f.ownerID, f.fluctlightID, "pure-order"); err != nil {
		t.Fatal(err)
	}
	var run string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT id FROM public.goal_planning_runs WHERE fluctlight_id=$1 LIMIT 1`, f.fluctlightID).Scan(&run); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_planning_runs SET status='processing',claim_revision=1,claimed_at=now(),mode='apply',profile_id='default' WHERE id=$1`, run); err != nil {
		t.Fatal(err)
	}
	c := plannerCommand(t, f, "order-commit")
	c.Order = []string{b, a}
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		_, err := f.app.applyGoalSetTx(f.ctx, tx, f.ownerID, f.fluctlightID, "default", run+"@1", c)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_planning_runs SET claimed_at=now()-interval '6 minutes' WHERE id=$1`, run); err != nil {
		t.Fatal(err)
	}
	result, err := f.app.ProcessGoalPlanningIntent(f.ctx, run)
	if err != nil || result["decision"] != "applied" {
		t.Fatal("pure-order durable commit lost", result, err)
	}
	f.requireNoModelRuns(t)
}
func TestGoalPlannerOtherProfileEventsRetainedUntilSwitch(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	persona := map[string]any{"personality_system": map[string]any{"mode": "multiple", "active_profile_id": "A", "profiles": []any{map[string]any{"id": "A", "name": "A"}, map[string]any{"id": "B", "name": "B"}}}}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlights SET core_persona=$2 WHERE id=$1`, f.fluctlightID, jsonBytes(persona)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.fluctlight_personality_runtime(fluctlight_id,active_profile_id,revision) VALUES($1,'A',1) ON CONFLICT(fluctlight_id) DO UPDATE SET active_profile_id='A',revision=1`, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.fluctlight_profile_habits(fluctlight_id,profile_id,habits_json,revision) VALUES($1,'A','[]',0),($1,'B','[]',0) ON CONFLICT DO NOTHING`, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		return requestGoalPlanningTx(f.ctx, tx, f.fluctlightID, "B-private-event", "goal_resolved", map[string]any{"profile_id": "B", "private_hint": "B only"})
	}); err != nil {
		t.Fatal(err)
	}
	seedCognitiveProviderRole(t, f.ctx, f.repository, "profiles-"+f.suffix)
	calls := 0
	f.app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().on("goal_planner_v1", func(payload map[string]any) fakeProviderResult {
		calls++
		if strings.Contains(jsonString(payload), "B only") {
			t.Fatal("private B event leaked into A")
		}
		if calls <= 2 {
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall(fmt.Sprint("profile-", calls), goalPlannerQuery, map[string]any{"section": map[int]string{1: "snapshot", 2: "triggers"}[calls]})}}
		}
		return fakeProviderResult{Structured: map[string]any{"decision": "no_viable_candidate", "reason": "A没有新需求", "review_condition": "新事实", "suggestions": []any{}}}
	})}
	var run string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT id FROM public.goal_planning_runs WHERE fluctlight_id=$1 AND status='pending'`, f.fluctlightID).Scan(&run); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_planning_runs SET available_at=now() WHERE id=$1`, run); err != nil {
		t.Fatal(err)
	}
	result, err := f.app.ProcessGoalPlanningIntent(f.ctx, run)
	if err != nil || result["decision"] != "no_viable_candidate" {
		t.Fatal("A profile run", result, err)
	}
	var processed *string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT processed_run_id FROM public.goal_planning_events WHERE fluctlight_id=$1 AND source_key='B-private-event'`, f.fluctlightID).Scan(&processed); err != nil || processed != nil {
		t.Fatal("A swallowed B event", processed, err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlight_personality_runtime SET active_profile_id='B',revision=revision+1 WHERE fluctlight_id=$1`, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	var pending int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.goal_planning_runs WHERE fluctlight_id=$1 AND status='pending'`, f.fluctlightID).Scan(&pending); err != nil || pending != 1 {
		t.Fatal("profile switch failed to recover B", pending, err)
	}
}

func TestGoalPlannerCanResumeSystemPausedButNeverOwnerPaused(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	id := plannerOwnerGoal(t, f, "system-held", false)
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		g, err := loadGoalAuthorityTx(f.ctx, tx, f.fluctlightID, "goal:ctx_"+stableDigest(id), ContextReference{EntityID: id, Revision: 1})
		if err != nil {
			return err
		}
		next, record, err := ApplyGoalCommand(&g, GoalCommand{Operation: GoalPause, ExpectedRevision: 1, Reason: "System waiting, not Owner stopped", EvidenceRefs: []string{"system:review"}, OccurredAt: f.app.now()})
		if err != nil {
			return err
		}
		_, err = persistGoalAuthorityTx(f.ctx, tx, &g, next, record, "system-pause")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	var run string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT id FROM public.goal_planning_runs WHERE fluctlight_id=$1 LIMIT 1`, f.fluctlightID).Scan(&run); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_planning_runs SET status='processing',claim_revision=1,claimed_at=now(),mode='apply',profile_id='default' WHERE id=$1`, run); err != nil {
		t.Fatal(err)
	}
	c := plannerCommand(t, f, "legal-resume")
	c.Changes = []GoalSetChange{{ID: id, Operation: "resume", ExpectedRevision: 2}}
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		_, err := f.app.applyGoalSetTx(f.ctx, tx, f.ownerID, f.fluctlightID, "default", run+"@1", c)
		return err
	}); err != nil {
		t.Fatal("system-paused legal resume rejected", err)
	}
}

func TestGoalPlannerInitialSourceLinksPausedOrCancelledEquivalent(t *testing.T) {
	for _, operation := range []string{"pause", "cancel"} {
		t.Run(operation, func(t *testing.T) {
			f := seedWardrobeToolFixture(t)
			id := plannerOwnerGoal(t, f, "已有愿望", false)
			if _, err := f.app.ApplyOwnerGoalCommand(f.ctx, f.ownerID, f.fluctlightID, id, GoalOwnerCommand{Operation: operation, ExpectedRevision: 1, IdempotencyKey: "stop", Reason: "Owner治理"}); err != nil {
				t.Fatal(err)
			}
			goals := []any{map[string]any{"source_id": "new-import-source", "description": "已有愿望", "success_criteria": []string{"真实结果"}, "motivation": "初始来源"}}
			if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
				return f.app.insertAgency(f.ctx, tx, f.fluctlightID, f.ownerID, goals, []any{}, map[string]struct{}{"default": {}})
			}); err != nil {
				t.Fatal(err)
			}
			var total int
			var linked string
			if err := f.repository.Pool().QueryRow(f.ctx, `SELECT (SELECT count(*) FROM public.fluctlight_goals WHERE fluctlight_id=$1),goal_id FROM public.goal_initial_imports WHERE fluctlight_id=$1 AND source_id='new-import-source'`, f.fluctlightID).Scan(&total, &linked); err != nil || total != 1 || linked != id {
				t.Fatal("import bypassed historical state", total, linked, err)
			}
		})
	}
}

func TestGoalPlannerDependencyBlocksOldDueAndReleasesAfterRealCompletion(t *testing.T) {
	f, intentionID, _ := seedNativeDueForGoalExecution(t)
	var downstream string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT goal_id FROM public.fluctlight_intentions WHERE id=$1`, intentionID).Scan(&downstream); err != nil {
		t.Fatal(err)
	}
	outcome, motivation := "实际发送前置表达", "先确认有限的行动前提"
	created, err := f.app.ApplyOwnerGoalCommand(f.ctx, f.ownerID, f.fluctlightID, "", GoalOwnerCommand{Operation: "create", IdempotencyKey: "prerequisite", Reason: "明确前置", DesiredOutcome: &outcome, Motivation: &motivation, SuccessCriteria: []string{"实际发送一段明确表达"}})
	if err != nil {
		t.Fatal(err)
	}
	parent := stringValue(created["goal_id"])
	c := plannerCommand(t, f, "dependency")
	c.Dependencies = map[string][]string{downstream: {parent}}
	if _, err := f.app.ApplyGoalSetCommand(f.ctx, f.ownerID, f.fluctlightID, c); err != nil {
		t.Fatal(err)
	}
	blocked, err := f.app.ProcessIntentionTrigger(f.ctx, intentionID)
	if err != nil || blocked["execution_blocked"] != true || blocked["reason_code"] != "goal_dependency_unsatisfied" {
		t.Fatal("old Worker bypassed dependency", blocked, err)
	}
	request := f.request(lifeActivityStartCapabilityName, "dependency-start", map[string]any{"kind": "virtual_shopping", "intention_id": intentionID, "duration_minutes": 15, "category": "boots", "slot": "shoes", "description": "黑色短靴", "reason": "正式已授权虚拟获取"})
	if _, err := f.app.ExecuteTool(f.ctx, request); err == nil {
		t.Fatal("direct start bypassed dependency")
	}
	receipt, err := f.app.ExecuteTool(f.ctx, f.request(conversationReplyCapabilityName, "prerequisite-message", map[string]any{"text": "我明确表达了当前想法，这是实际发送的前置表达。"}))
	if err != nil || receipt.Result.Status != "completed" {
		t.Fatal(receipt, err)
	}
	seedCognitiveProviderRole(t, f.ctx, f.repository, "dependency-eval-"+f.suffix)
	f.app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().on("goal_evaluation_v1", func(_ map[string]any) fakeProviderResult {
		s := readProcessingGoalSnapshot(t, f)
		proof := ""
		for _, source := range s.Sources {
			if source.Kind == "message" && source.SubjectActorID == f.fluctlightID {
				proof = source.Ref
			}
		}
		if proof == "" {
			t.Fatal("actual prerequisite message not offered")
		}
		evaluations := []GoalEvaluationCandidate{}
		for _, entry := range s.Goals {
			v := GoalEvaluationCandidate{GoalID: entry.GoalID, ExpectedRevision: entry.Goal.Revision, CriteriaVersion: entry.Goal.CriteriaVersion, Impact: "needs_evidence", Judgments: []GoalCriterionJudgment{{CriterionID: entry.Goal.CriterionIDs[0], Verdict: "unknown", Kind: "acquisition", Subject: "domain", Discourse: "uncertain", EvidenceRefs: []string{}, Reason: "未真实获取物品"}}}
			if entry.GoalID == parent {
				v.Impact = "completed"
				v.Judgments = []GoalCriterionJudgment{{CriterionID: entry.Goal.CriterionIDs[0], Verdict: "satisfied", Kind: "communication", Subject: "actor_self", Discourse: "assertion", EvidenceRefs: []string{proof}, Reason: "正式发送完成前置表达"}}
			}
			evaluations = append(evaluations, v)
		}
		return fakeProviderResult{Structured: goalEvaluationProviderFixture(s, GoalEvaluationTaskOutput{Evaluations: evaluations, Plans: []GoalPlanCandidate{}})}
	})}
	if _, err := f.app.ProcessGoalEvaluationIntent(f.ctx, latestPendingGoalRequest(t, f)); err != nil {
		t.Fatal(err)
	}
	request.OperationID = "dependency-start-after-result"
	started, err := f.app.ExecuteTool(f.ctx, request)
	if err != nil || stringValue(mapValue(started.Result.Output)["activity_id"]) == "" {
		t.Fatal("real prerequisite did not release original execution", started, err)
	}
	t.Logf("DEPENDENCY_EVIDENCE prerequisite=%s downstream=%s intention=%s old_due_blocked=true actual_parent_result=true new_activity=%v", parent, downstream, intentionID, mapValue(started.Result.Output)["activity_id"])
}

func TestGoalPlannerCommittedCrashRetainsLaterEventsAndRepairQueuesThem(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	a := plannerOwnerGoal(t, f, "A", false)
	if _, err := f.app.RequestGoalPlanning(f.ctx, f.ownerID, f.fluctlightID, "crash"); err != nil {
		t.Fatal(err)
	}
	var run string
	var watermark int64
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT id,watermark FROM public.goal_planning_runs WHERE fluctlight_id=$1 LIMIT 1`, f.fluctlightID).Scan(&run, &watermark); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_planning_runs SET status='processing',claim_revision=1,claimed_at=now(),mode='apply',profile_id='default' WHERE id=$1`, run); err != nil {
		t.Fatal(err)
	}
	c := plannerCommand(t, f, "crash-order")
	c.Order = []string{a}
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		_, err := f.app.applyGoalSetTx(f.ctx, tx, f.ownerID, f.fluctlightID, "default", run+"@1", c)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		return requestGoalPlanningTx(f.ctx, tx, f.fluctlightID, "during-committed-crash", "actor_context_changed", map[string]any{"profile_id": "default"})
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.ProcessGoalPlanningIntent(f.ctx, run); err != nil {
		t.Fatal(err)
	}
	var processed *string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT processed_run_id FROM public.goal_planning_events WHERE fluctlight_id=$1 AND source_key='during-committed-crash'`, f.fluctlightID).Scan(&processed); err != nil || processed != nil {
		t.Fatal("late event consumed without reading", processed, err)
	}
	if _, err := f.app.RepairGoalPlanning(f.ctx, 20); err != nil {
		t.Fatal(err)
	}
	var pending int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.goal_planning_runs WHERE fluctlight_id=$1 AND status='pending'`, f.fluctlightID).Scan(&pending); err != nil || pending != 1 {
		t.Fatal("committed recovery lost follow-up request", pending, err)
	}
	f.requireNoModelRuns(t)
}

func TestGoalPlannerRecoveryPolicyIsOperationalAndModelCannotExpandIt(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	plannerOwnerGoal(t, f, "old", false)
	c := plannerCommand(t, f, "recovery-settings")
	c.RecoveryPolicy = &GoalPlannerRecoveryPolicy{MergeWindowSeconds: 7, RetryBackoffSeconds: 55, MaxAttempts: 1, LeaseSeconds: 120, CandidateLimit: 1}
	s, err := f.app.ApplyGoalSetCommand(f.ctx, f.ownerID, f.fluctlightID, c)
	if err != nil || intValue(s["merge_window_seconds"]) != 7 || intValue(s["candidate_limit"]) != 1 {
		t.Fatal("recovery policy not stored", s, err)
	}
	plannerOwnerGoal(t, f, "one-candidate", true)
	outcome, motivation := "overflow", "Owner candidate"
	_, err = f.app.ApplyOwnerGoalCommand(f.ctx, f.ownerID, f.fluctlightID, "", GoalOwnerCommand{Operation: "create", CandidateOnly: true, IdempotencyKey: "overflow-candidate", Reason: "candidate quota", DesiredOutcome: &outcome, Motivation: &motivation, SuccessCriteria: []string{"real"}})
	if !errors.Is(err, ErrConflict) {
		t.Fatal("candidate capacity not domain conflict", err)
	}
	// A previously queued batch retains its original deadline. The setting applies to new batches.
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_planning_runs SET status='awaiting_profile' WHERE fluctlight_id=$1 AND status='pending'`, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.RequestGoalPlanning(f.ctx, f.ownerID, f.fluctlightID, "merge-window"); err != nil {
		t.Fatal(err)
	}
	var seconds float64
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT extract(epoch FROM available_at-created_at) FROM public.goal_planning_runs WHERE fluctlight_id=$1 AND status='pending' LIMIT 1`, f.fluctlightID).Scan(&seconds); err != nil || seconds < 6 || seconds > 8 {
		t.Fatal("merge window ignored", seconds, err)
	}
	for _, tool := range goalPlannerCapabilities(f.app) {
		if tool.Definition().Name == goalPlannerCommit {
			if err := capability.ValidateCapabilitySchemaValue(map[string]any{"expected_version": 1, "expected_facts_revision": "facts", "idempotency_key": "bad", "reason": "expand", "recovery_policy": map[string]any{"max_attempts": 100}}, tool.Definition().InputSchema); err == nil {
				t.Fatal("model policy expansion accepted")
			}
		}
	}
}
func TestGoalPlannerPolicyBlockRetainsEventsAndRecoveryWaitsForPermission(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	plannerOwnerGoal(t, f, "old", false)
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.autonomy_policies(fluctlight_id,mode,allowed_actions,budget_remaining,quiet_hours,concurrency_limit,revision) VALUES($1,'paused','["capability"]','100','{}',1,1)`, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		return requestGoalPlanningTx(f.ctx, tx, f.fluctlightID, "permission-wait", "goal_resolved", nil)
	}); err != nil {
		t.Fatal(err)
	}
	var run string
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT id FROM public.goal_planning_runs WHERE fluctlight_id=$1`, f.fluctlightID).Scan(&run); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_planning_runs SET available_at=now() WHERE id=$1`, run); err != nil {
		t.Fatal(err)
	}
	result, err := f.app.ProcessGoalPlanningIntent(f.ctx, run)
	if err != nil || result["decision"] != "blocked_by_policy" {
		t.Fatal("policy state", result, err)
	}
	if repaired, err := f.app.RepairGoalPlanning(f.ctx, 20); err != nil || repaired != 0 {
		t.Fatal("blocked policy generated hot work", repaired, err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.autonomy_policies SET mode='active',revision=revision+1 WHERE fluctlight_id=$1`, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	if repaired, err := f.app.RepairGoalPlanning(f.ctx, 20); err != nil || repaired == 0 {
		t.Fatal("permission restoration lost waiting event", repaired, err)
	}
	f.requireNoModelRuns(t)
}

func TestGoalPlannerQueryAndCommitDependencyFailureWithoutAgent(t *testing.T) {
	for _, tool := range goalPlannerCapabilities(&App{}) {
		inv := CapabilityInvocation{CallID: "dependency-failure", CapabilityName: tool.Definition().Name, Arguments: jsonBytes(map[string]any{"section": "snapshot"})}
		var result CapabilityResult
		var err error
		if tool.Definition().Type == CapabilityTypeQuery {
			result, err = tool.Execute(context.Background(), inv, CapabilityContext{})
		} else {
			result, err = tool.(DirectToolCapability).ExecuteDirectTx(context.Background(), nil, inv, CapabilityContext{}, DirectToolTarget{Kind: "goal_planning_run"})
		}
		if err == nil || result.Status != "failed" || !result.Retryable {
			t.Fatal("Tool dependency failure was not independently safe", result, err)
		}
	}
}

func TestGoalPlannerScheduledWishCannotCreateIntentionOrCallScheduleModel(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	seedCognitiveProviderRole(t, f.ctx, f.repository, "scheduled-wish-"+f.suffix)
	scheduleCalls := 0
	f.app.SchedulePlanner = scheduledPlannerFunc(func(context.Context, SchedulePlanInput) (map[string]any, error) {
		scheduleCalls++
		return nil, errors.New("schedule must wait for active goal")
	})
	f.app.Capabilities = nil
	f.app.Runtime = nil
	calls := 0
	f.app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().on("conversation_turn_response", func(payload map[string]any) fakeProviderResult {
		calls++
		if calls == 1 {
			return fakeProviderResult{ToolCalls: []map[string]any{nativePersonaToolCall("new-schedule-wish", scheduleActivityCapabilityName, map[string]any{"goal": "获得一副新手套", "action": "计划虚拟购物获取手套", "expected_outcome": "手套真实入库", "action_plan": map[string]any{"kind": "virtual_shopping", "duration_minutes": 15, "category": "accessory", "slot": "hands", "description": "保暖手套"}, "reason": "提出新的有限获取愿望"})}}
		}
		text := nativePersonaToolMessages(payload)
		if !strings.Contains(text, "planning_requested") || !strings.Contains(text, "false") {
			t.Fatal("Model did not consume the actual unscheduled result", text)
		}
		return nativePersonaFinal()
	})}
	result, err := f.app.RunConversationCognitionAgent(f.ctx, ConversationCognitionAgentInput{AuthorizationActorID: f.ownerID, FluctlightID: f.fluctlightID, ConversationID: f.conversationID, RunID: "scheduled-wish-" + f.suffix, CurrentInput: "我想看看新的手套"})
	if err != nil || calls != 2 || scheduleCalls != 0 {
		t.Fatal("new wish prematurely planned execution", calls, scheduleCalls, result, err)
	}
	var goals, intentions int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT (SELECT count(*) FROM public.fluctlight_goals WHERE fluctlight_id=$1),(SELECT count(*) FROM public.fluctlight_intentions WHERE fluctlight_id=$1)`, f.fluctlightID).Scan(&goals, &intentions); err != nil || goals != 0 || intentions != 0 {
		t.Fatal("scheduled wish bypassed Planner authority", goals, intentions, err)
	}
}

func TestGoalPlannerActorABEditsDoNotCrossContextOrGoalReview(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	a, b := "actor-A-"+f.suffix, "actor-B-"+f.suffix
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.actors(id,actor_type,status) VALUES($1,'human','active'),($2,'human','active')`, a, b); err != nil {
		t.Fatal(err)
	}
	if _, err := f.repository.Pool().Exec(f.ctx, `INSERT INTO public.relationships(id,owner_fluctlight_id,target_actor_id,role,metrics,revision) VALUES($1,$3,$1,'{"label":"configured contact"}','{}',1),($2,$3,$2,'{"label":"configured contact"}','{}',1)`, a, b, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	goals := map[string]string{}
	for _, target := range []string{a, b} {
		outcome, motivation := "与"+target+"交流一项明确想法", "有限交流"
		r, err := f.app.ApplyOwnerGoalCommand(f.ctx, f.ownerID, f.fluctlightID, "", GoalOwnerCommand{Operation: "create", TargetActorID: target, IdempotencyKey: target, Reason: "Owner明确不同对象", DesiredOutcome: &outcome, Motivation: &motivation, SuccessCriteria: []string{"实际对该对象表达"}})
		if err != nil {
			t.Fatal(err)
		}
		goals[target] = stringValue(r["goal_id"])
	}
	ac, err := f.app.ActorContext(f.ctx, f.ownerID, f.fluctlightID, a)
	if err != nil {
		t.Fatal(err)
	}
	bc, err := f.app.ActorContext(f.ctx, f.ownerID, f.fluctlightID, b)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.app.UpdateActorContext(f.ctx, f.ownerID, f.fluctlightID, a, map[string]any{"background": map[string]any{"location": "法国", "timezone": "Europe/Paris"}, "expected_context_version": ac["context_version"], "idempotency_key": "A-edit", "operation": "change", "reason": "Owner背景配置"}); err != nil {
		t.Fatal(err)
	}
	refreshedB, err := f.app.ActorContext(f.ctx, f.ownerID, f.fluctlightID, b)
	if err != nil || refreshedB["context_version"] != bc["context_version"] || mapValue(refreshedB["background"])["location"] != nil {
		t.Fatal("A facts crossed into B", refreshedB, err)
	}
	var aReview, bReview bool
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT (SELECT context_review_required FROM public.fluctlight_goals WHERE id=$1),(SELECT context_review_required FROM public.fluctlight_goals WHERE id=$2)`, goals[a], goals[b]).Scan(&aReview, &bReview); err != nil || !aReview || bReview {
		t.Fatal("review invalidation crossed Actors", aReview, bReview, err)
	}
	var interactions int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.conversation_messages WHERE conversation_id=$1`, f.conversationID).Scan(&interactions); err != nil || interactions != 0 {
		t.Fatal("config edit fabricated interaction", interactions, err)
	}
}
