package core

import (
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestOwnerGoalCommandsAuditCASReplayAndHistory(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	desired, motivation := "获得实际画笔", "绘画需要"
	create := GoalOwnerCommand{Operation: "create", IdempotencyKey: "owner-create", Reason: "明确目标", DesiredOutcome: &desired, Motivation: &motivation, SuccessCriteria: []string{"实际持有画笔"}}
	first, err := f.app.ApplyOwnerGoalCommand(f.ctx, f.ownerID, f.fluctlightID, "", create)
	if err != nil {
		t.Fatal(err)
	}
	id := stringValue(first["goal_id"])
	replay, err := f.app.ApplyOwnerGoalCommand(f.ctx, f.ownerID, f.fluctlightID, "", create)
	if err != nil || jsonString(first) != jsonString(replay) {
		t.Fatalf("replay %#v %v", replay, err)
	}
	changed := create
	changed.Reason = "不同请求"
	if _, err := f.app.ApplyOwnerGoalCommand(f.ctx, f.ownerID, f.fluctlightID, "", changed); !errors.Is(err, ErrConflict) {
		t.Fatalf("changed replay %v", err)
	}
	command := GoalOwnerCommand{Operation: "pause", ExpectedRevision: 1, IdempotencyKey: "owner-pause", Reason: "暂缓执行"}
	paused, err := f.app.ApplyOwnerGoalCommand(f.ctx, f.ownerID, f.fluctlightID, id, command)
	if err != nil || paused["status"] != GoalPaused {
		t.Fatalf("pause %#v %v", paused, err)
	}
	stale := command
	stale.IdempotencyKey = "owner-stale"
	if _, err := f.app.ApplyOwnerGoalCommand(f.ctx, f.ownerID, f.fluctlightID, id, stale); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale %v", err)
	}
	if _, err := f.app.ApplyOwnerGoalCommand(f.ctx, "foreign-owner", f.fluctlightID, id, command); err == nil {
		t.Fatal("foreign owner mutated goal")
	}
	command = GoalOwnerCommand{Operation: "resume", ExpectedRevision: 2, IdempotencyKey: "owner-resume", Reason: "恢复推进"}
	if _, err := f.app.ApplyOwnerGoalCommand(f.ctx, f.ownerID, f.fluctlightID, id, command); err != nil {
		t.Fatal(err)
	}
	command = GoalOwnerCommand{Operation: "reassess", ExpectedRevision: 3, IdempotencyKey: "owner-evaluate", Reason: "检查实际证据"}
	assessed, err := f.app.ApplyOwnerGoalCommand(f.ctx, f.ownerID, f.fluctlightID, id, command)
	if err != nil || stringValue(assessed["evaluation_request_id"]) == "" {
		t.Fatalf("request %#v %v", assessed, err)
	}
	command = GoalOwnerCommand{Operation: "cancel", ExpectedRevision: 3, IdempotencyKey: "owner-cancel", Reason: "不再需要"}
	if _, err := f.app.ApplyOwnerGoalCommand(f.ctx, f.ownerID, f.fluctlightID, id, command); err != nil {
		t.Fatal(err)
	}
	current, err := f.app.ListGoals(f.ctx, f.ownerID, f.fluctlightID, false, 20, "")
	if err != nil || len(current.Items) != 0 {
		t.Fatalf("current %#v %v", current, err)
	}
	history, err := f.app.ListGoals(f.ctx, f.ownerID, f.fluctlightID, true, 20, "")
	if err != nil || len(history.Items) != 1 || history.Items[0]["status"] != "cancelled" {
		t.Fatalf("history %#v %v", history, err)
	}
	page, err := f.app.GoalHistory(f.ctx, f.ownerID, f.fluctlightID, id, 2, "")
	if err != nil || len(page.Items) != 2 || page.NextCursor == "" {
		t.Fatalf("revisions %#v %v", page, err)
	}
	next, err := f.app.GoalHistory(f.ctx, f.ownerID, f.fluctlightID, id, 2, page.NextCursor)
	if err != nil || len(next.Items) != 2 {
		t.Fatalf("revision continuation %#v %v", next, err)
	}
	for _, p := range []GoalPage{page, next} {
		for _, record := range p.Items {
			if record["actor_id"] != f.ownerID || record["source"] != "owner" {
				t.Fatalf("audit source %#v", record)
			}
		}
	}
	detail, err := f.app.GoalDetail(f.ctx, f.ownerID, f.fluctlightID, id)
	if err != nil || detail["status"] != "cancelled" || mapValue(detail["execution"])["stage"] != "cancelled" {
		t.Fatalf("detail %#v %v", detail, err)
	}
}

func TestOwnerGoalEvidencePagesAreCompleteOrderedAndScoped(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	goalID := createDialogueGoalForClosure(t, f, []string{"actual result"})
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		for i := 0; i < 23; i++ {
			messageID := fmt.Sprintf("paged-message-%02d-%s", i, f.suffix)
			if _, err := tx.Exec(f.ctx, `INSERT INTO public.conversation_messages(id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key) VALUES($1,$2,$3,$4,'assistant','actual communication','[]',$1)`, messageID, f.conversationID, 50+i, f.fluctlightID); err != nil {
				return err
			}
			if err := f.app.recordGoalMessageTx(f.ctx, tx, f.fluctlightID, messageID, "default"); err != nil {
				return err
			}
			if _, err := tx.Exec(f.ctx, `UPDATE public.goal_source_events SET recorded_at='2026-10-07T10:00:00Z' WHERE source_id=$1`, messageID); err != nil {
				return err
			}
			if _, err := tx.Exec(f.ctx, `INSERT INTO public.goal_evidence_links(id,fluctlight_id,goal_id,source_event_id,status,reason) SELECT $1,$2,$3,id,'candidate','actual source association' FROM public.goal_source_events WHERE source_id=$4`, fmt.Sprintf("paged-evidence-%02d-%s", i, f.suffix), f.fluctlightID, goalID, messageID); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	detail, err := f.app.GoalDetail(f.ctx, f.ownerID, f.fluctlightID, goalID)
	if err != nil || len(arrayValue(detail["evidence"])) != 20 || stringValue(detail["evidence_next_cursor"]) == "" {
		t.Fatalf("bounded detail lacks continuation: %#v %v", detail, err)
	}
	cursor, firstCursor := "", ""
	ids := []string{}
	for {
		page, err := f.app.GoalEvidence(f.ctx, f.ownerID, f.fluctlightID, goalID, 7, cursor)
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range page.Items {
			if stringValue(mapValue(item["source"])["source_kind"]) != "message" {
				t.Fatalf("source detail missing: %#v", item)
			}
			ids = append(ids, stringValue(item["id"]))
		}
		if firstCursor == "" {
			firstCursor = page.NextCursor
		}
		if page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}
	if len(ids) != 23 {
		t.Fatalf("lost or duplicated evidence: %d", len(ids))
	}
	for i, id := range ids {
		if id != fmt.Sprintf("paged-evidence-%02d-%s", 22-i, f.suffix) {
			t.Fatalf("unstable tie order: %v", ids)
		}
	}
	if _, err := f.app.GoalHistory(f.ctx, f.ownerID, f.fluctlightID, goalID, 7, firstCursor); !errors.Is(err, ErrInvalidArguments) {
		t.Fatalf("evidence cursor accepted as revision cursor: %v", err)
	}
	if _, err := f.app.GoalEvidence(f.ctx, "foreign-owner", f.fluctlightID, goalID, 7, firstCursor); err == nil {
		t.Fatal("foreign owner read evidence")
	}
}

func TestOwnerEndedGoalHistoryPagesStableForAllStatuses(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	completed := createDialogueGoalForClosure(t, f, []string{"actual expression"})
	messageID := insertGoalBoundaryMessage(t, f)
	requestID := latestPendingGoalRequest(t, f)
	snapshot, _, err := f.app.claimGoalEvaluation(f.ctx, requestID)
	if err != nil {
		t.Fatal(err)
	}
	goal := snapshot.Goals[0].Goal
	sources := map[string]GoalSource{}
	proof := ""
	for _, source := range snapshot.Sources {
		sources[source.Ref] = source
		if source.ID == messageID {
			proof = source.Ref
		}
	}
	candidate := GoalEvaluationCandidate{GoalID: completed, ExpectedRevision: goal.Revision, CriteriaVersion: goal.CriteriaVersion, Impact: "completed", Judgments: []GoalCriterionJudgment{{CriterionID: goal.CriterionIDs[0], Verdict: "satisfied", Kind: "communication", Subject: "actor_self", Discourse: "assertion", EvidenceRefs: []string{proof}, Reason: "actual sent expression"}}}
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		_, err := f.app.commitGoalEvaluationTx(f.ctx, tx, goal, candidate, sources, requestID)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	states := map[string]string{completed: "completed"}
	for _, operation := range []string{"cancel", "abandon"} {
		desired, motivation := "history "+operation, "owner purpose"
		created, err := f.app.ApplyOwnerGoalCommand(f.ctx, f.ownerID, f.fluctlightID, "", GoalOwnerCommand{Operation: "create", IdempotencyKey: "history-create-" + operation, Reason: "history fixture", DesiredOutcome: &desired, Motivation: &motivation, SuccessCriteria: []string{"actual result"}})
		if err != nil {
			t.Fatal(err)
		}
		id := stringValue(created["goal_id"])
		if _, err := f.app.ApplyOwnerGoalCommand(f.ctx, f.ownerID, f.fluctlightID, id, GoalOwnerCommand{Operation: operation, ExpectedRevision: 1, IdempotencyKey: "history-end-" + operation, Reason: "explicit owner decision"}); err != nil {
			t.Fatal(err)
		}
		states[id] = map[string]string{"cancel": "cancelled", "abandon": "abandoned"}[operation]
	}
	// Equal timestamps exercise the ID tie breaker rather than clock spacing.
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.fluctlight_goals SET updated_at='2026-10-07T10:00:00Z' WHERE fluctlight_id=$1`, f.fluctlightID); err != nil {
		t.Fatal(err)
	}
	ids := []string{}
	for id := range states {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	slices.Reverse(ids)
	cursor := ""
	for i, expectedID := range ids {
		page, err := f.app.ListGoals(f.ctx, f.ownerID, f.fluctlightID, true, 1, cursor)
		if err != nil || len(page.Items) != 1 || stringValue(page.Items[0]["id"]) != expectedID || stringValue(page.Items[0]["status"]) != states[expectedID] {
			t.Fatalf("unstable history page=%#v expected=%s err=%v", page, expectedID, err)
		}
		detail, err := f.app.GoalDetail(f.ctx, f.ownerID, f.fluctlightID, expectedID)
		if err != nil || stringValue(detail["status"]) != states[expectedID] {
			t.Fatalf("ended detail unavailable: %#v %v", detail, err)
		}
		if i < len(ids)-1 && page.NextCursor == "" {
			t.Fatal("history cursor ended early")
		}
		if i == len(ids)-1 && page.NextCursor != "" {
			t.Fatal("history cursor continued beyond end")
		}
		if page.NextCursor != "" {
			if _, err := f.app.ListGoals(f.ctx, f.ownerID, f.fluctlightID, false, 1, page.NextCursor); !errors.Is(err, ErrInvalidArguments) {
				t.Fatalf("history cursor crossed collection: %v", err)
			}
		}
		cursor = page.NextCursor
	}
}

func TestOwnerGoalRejectsUnknownProfileAndForcedCompletion(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	desired, motivation := "真实目标", "明确动机"
	command := GoalOwnerCommand{Operation: "create", IdempotencyKey: "invalid-profile", Reason: "创建", ProfileID: "foreign-profile", DesiredOutcome: &desired, Motivation: &motivation, SuccessCriteria: []string{"实际完成"}}
	if _, err := f.app.ApplyOwnerGoalCommand(f.ctx, f.ownerID, f.fluctlightID, "", command); err == nil {
		t.Fatal("unknown profile accepted")
	}
	command.ProfileID = ""
	command.IdempotencyKey = "valid-goal"
	result, err := f.app.ApplyOwnerGoalCommand(f.ctx, f.ownerID, f.fluctlightID, "", command)
	if err != nil {
		t.Fatal(err)
	}
	command = GoalOwnerCommand{Operation: "complete", ExpectedRevision: 1, IdempotencyKey: "forced", Reason: "无证据强制结束"}
	if _, err := f.app.ApplyOwnerGoalCommand(f.ctx, f.ownerID, f.fluctlightID, stringValue(result["goal_id"]), command); err == nil {
		t.Fatal("forced completion accepted")
	}
}

func TestOwnerGoalReviewPolicyResetIsAuditedAndReplays(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	id := createDialogueGoalForClosure(t, f, []string{"actual result"})
	command := GoalOwnerCommand{Operation: "update", ExpectedRevision: 1, IdempotencyKey: "policy-reset", Reason: "重新审视策略，不删除历史", ReviewPolicy: &GoalReviewPolicy{MissedOpportunityThreshold: 2, IneffectiveAttemptThreshold: 4}, ResetReviewCounters: true}
	first, err := f.app.ApplyOwnerGoalCommand(f.ctx, f.ownerID, f.fluctlightID, id, command)
	if err != nil {
		t.Fatal(err)
	}
	again, err := f.app.ApplyOwnerGoalCommand(f.ctx, f.ownerID, f.fluctlightID, id, command)
	if err != nil || jsonString(first) != jsonString(again) {
		t.Fatalf("policy replay %#v %v", again, err)
	}
	detail, err := f.app.GoalDetail(f.ctx, f.ownerID, f.fluctlightID, id)
	if err != nil {
		t.Fatal(err)
	}
	policy := mapValue(detail["review_policy"])
	if intValue(policy["missed_opportunity_threshold"]) != 2 || intValue(policy["ineffective_attempt_threshold"]) != 4 || stringValue(policy["reset_after"]) == "" {
		t.Fatalf("policy not persisted %#v", policy)
	}
	history, err := f.app.GoalHistory(f.ctx, f.ownerID, f.fluctlightID, id, 20, "")
	if err != nil || len(history.Items) != 2 || history.Items[0]["source"] != "owner" {
		t.Fatalf("reset lacks audit %#v %v", history, err)
	}
	bad := command
	bad.IdempotencyKey = "bad-policy"
	bad.ExpectedRevision = 2
	bad.ReviewPolicy = &GoalReviewPolicy{MissedOpportunityThreshold: 0, IneffectiveAttemptThreshold: 4}
	bad.ResetReviewCounters = false
	if _, err := f.app.ApplyOwnerGoalCommand(f.ctx, f.ownerID, f.fluctlightID, id, bad); err == nil {
		t.Fatal("invalid threshold accepted")
	}
}
