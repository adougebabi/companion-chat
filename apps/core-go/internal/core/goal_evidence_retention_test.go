package core

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestGoalEvidenceInterpretationUnitIsAtomicPrivateAndVersioned(t *testing.T) {
	source := GoalSource{Ref: "source:1", EventID: 1, Kind: "message", ID: "answer-secret", FluctlightID: "self", SubjectActorID: "human-secret", Version: "answer-version", Valid: true, CanSupportSuccess: true, Data: map[string]any{
		"text": "人物本身？应该是", "message_kind": "user",
		"surrounding_context": []any{map[string]any{"text": "你更喜欢人物本身，还是情节？", "author_actor_id": "self", "interpretation_only": true}},
		"context_authority":   []any{map[string]any{"id": "question-secret", "version": "question-version", "author": "self"}},
	}}
	snapshot := goalEvaluationSnapshot{FluctlightID: "self", OwnerActorID: "human-secret", Sources: []GoalSource{source}}
	selected, err := admitGoalEvaluationSourceInput(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := renderedGoalEvaluationProviderInput(selected)
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"人物本身？应该是", "你更喜欢人物本身，还是情节？", "interpretation_only"} {
		if !strings.Contains(rendered, text) {
			t.Fatalf("lost interpretation unit: %s", text)
		}
	}
	for _, private := range []string{"question-secret", "question-version", "human-secret", "context_authority"} {
		if strings.Contains(rendered, private) {
			t.Fatalf("private context escaped: %s", private)
		}
	}
	binding, err := newGoalEvaluationWireBinding(selected)
	if err != nil {
		t.Fatal(err)
	}
	if len(binding.sourcesByRef) != 1 {
		t.Fatal("neighbor became independently citable", binding.sourcesByRef)
	}
	original := goalAssessmentSourceFingerprint(source)
	source.Data = cloneMap(source.Data)
	source.Data["context_authority"] = []any{map[string]any{"id": "question-secret", "version": "edited-question", "author": "self"}}
	if original == goalAssessmentSourceFingerprint(source) {
		t.Fatal("changed surrounding question did not invalidate source/CAS")
	}
	source.Data["surrounding_context"] = []any{map[string]any{"text": strings.Repeat("context ", 30000)}}
	snapshot.Sources = []GoalSource{source, {Ref: "source:2", EventID: 2, Kind: "message", FluctlightID: "self", Data: map[string]any{"text": "another small source"}}}
	selected, err = admitGoalEvaluationSourceInput(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(selected.ProviderSourceIDs, int64(1)) || !slices.Contains(selected.ProviderSourceIDs, int64(2)) {
		t.Fatal("over-budget interpretation unit was partially admitted", selected.ProviderSourceIDs)
	}
}

func TestGoalHistoricalCoverageOrderSurvivesInputAdmission(t *testing.T) {
	at := time.Now()
	snapshot := goalEvaluationSnapshot{RecoverySourceIDs: []int64{1, 2}, Sources: []GoalSource{
		{Ref: "source:1", EventID: 1, Kind: "message", RecordedAt: at.Add(-time.Hour), Data: map[string]any{"text": strings.Repeat("old unseen evidence ", 450)}},
		{Ref: "source:2", EventID: 2, Kind: "message", RecordedAt: at, Data: map[string]any{"text": strings.Repeat("recent already offered ", 450)}},
	}}
	selected, err := admitGoalEvaluationSourceInput(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if len(selected.ProviderSourceIDs) != 1 || selected.ProviderSourceIDs[0] != 1 {
		t.Fatal("newest-first admission starved historical recovery", selected.ProviderSourceIDs)
	}
}

func TestGoalCandidateOmissionRetainsAndRotatesHistoricalEvidence(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	goalID := createDialogueGoalForClosure(t, f, []string{"根据实际偏好推荐具体小说并说明理由"})
	ids := []int64{}
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		for i := 0; i < 40; i++ {
			messageID := fmt.Sprintf("retained-%d-%s", i, f.suffix)
			text := fmt.Sprintf("source %d", i)
			if i == 0 {
				text = "你喜欢人物本身，还是情节？"
			}
			if i == 1 {
				text = "人物本身？应该是"
			}
			if i == 39 {
				text = "我推荐《灯塔》，因为你在意人物本身，它把人物内心放在中心。"
			}
			author, kind := f.fluctlightID, "assistant"
			if i == 1 {
				author, kind = f.ownerID, "user"
			}
			if _, err := tx.Exec(f.ctx, `INSERT INTO public.conversation_messages(id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key) VALUES($1,$2,$3,$4,$5,$6,'[]',$1)`, messageID, f.conversationID, 200+i, author, kind, text); err != nil {
				return err
			}
			if err := f.app.recordGoalMessageTx(f.ctx, tx, f.fluctlightID, messageID, "default"); err != nil {
				return err
			}
			var eventID int64
			if err := tx.QueryRow(f.ctx, `SELECT id FROM public.goal_source_events WHERE fluctlight_id=$1 AND source_id=$2`, f.fluctlightID, messageID).Scan(&eventID); err != nil {
				return err
			}
			ids = append(ids, eventID)
			status := "candidate"
			if i == 1 {
				status = "rejected"
			}
			if _, err := tx.Exec(f.ctx, `INSERT INTO public.goal_evidence_links(id,fluctlight_id,goal_id,source_event_id,status,reason) VALUES($1,$2,$3,$4,$5,'legacy association')`, fmt.Sprintf("link-%d-%s", i, f.suffix), f.fluctlightID, goalID, eventID, status); err != nil {
				return err
			}
			if _, err := tx.Exec(f.ctx, `UPDATE public.goal_source_events SET processed_at=now() WHERE id=$1`, eventID); err != nil {
				return err
			}
		}
		_, err := queueGoalEvaluationTx(f.ctx, tx, f.fluctlightID, "default", "owner_reassess", "retained-first", []string{goalID})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	request := latestPendingGoalRequest(t, f)
	first, prior, err := f.app.claimGoalEvaluation(f.ctx, request)
	if err != nil || prior != nil {
		t.Fatal(prior, err)
	}
	first, err = admitGoalEvaluationSourceInput(first)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.RecoverySourceIDs) != 32 {
		t.Fatalf("historical recovery not bounded: %v", first.RecoverySourceIDs)
	}
	entry := first.Goals[0]
	candidate := GoalEvaluationCandidate{GoalID: goalID, ExpectedRevision: entry.Goal.Revision, CriteriaVersion: entry.Goal.CriteriaVersion, Impact: "no_change", Judgments: []GoalCriterionJudgment{{CriterionID: entry.Goal.CriterionIDs[0], Verdict: "unknown", Kind: "information", Subject: "target_actor", Discourse: "assertion", Reason: "retain complementary evidence for later assessment"}}}
	sources := map[string]GoalSource{}
	for _, s := range admittedGoalEvaluationSources(first) {
		sources[s.Ref] = s
	}
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		if _, err := f.app.commitGoalEvaluationTx(f.ctx, tx, entry.Goal, candidate, sources, request); err != nil {
			return err
		}
		if _, err := tx.Exec(f.ctx, `UPDATE public.goal_evaluation_requests SET status='succeeded',snapshot=$2,result=$3 WHERE id=$1`, request, jsonBytes(first), jsonBytes(map[string]any{"evaluated_goals": []string{goalID}})); err != nil {
			return err
		}
		_, err := queueGoalEvaluationTx(f.ctx, tx, f.fluctlightID, "default", "owner_reassess", "retained-second", []string{goalID})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	var retained int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.goal_evidence_links WHERE goal_id=$1 AND status='candidate'`, goalID).Scan(&retained); err != nil || retained != 39 {
		t.Fatalf("omission discarded offered or unoffered candidate: %d %v", retained, err)
	}
	// A committed root remains covered even if a later sibling/model failure
	// makes the whole request fail. Rotation must still honor that actual offer.
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.goal_evaluation_requests SET status='failed' WHERE id=$1`, request); err != nil {
		t.Fatal(err)
	}
	second, prior, err := f.app.claimGoalEvaluation(f.ctx, latestPendingGoalRequest(t, f))
	if err != nil || prior != nil {
		t.Fatal(prior, err)
	}
	covered := append(slices.Clone(first.RecoverySourceIDs), second.RecoverySourceIDs...)
	for _, id := range ids {
		if !slices.Contains(covered, id) {
			t.Fatalf("historical source starved after bounded rotation: %d", id)
		}
	}
	var answer GoalSource
	for _, s := range second.Sources {
		if s.EventID == ids[1] {
			answer = s
		}
	}
	if !answer.Valid || !strings.Contains(jsonString(answer.Data["surrounding_context"]), "人物本身，还是情节") {
		t.Fatalf("processed/rejected answer did not recover actual question: %#v", answer)
	}
	// Actual preference plus later recommendation can satisfy the original
	// criterion without inventing another standard or turning prose into a Tool.
	preference, err := readGoalSourceWith(f.ctx, f.repository.Pool(), ids[1], false)
	if err != nil {
		t.Fatal(err)
	}
	recommendation, err := readGoalSourceWith(f.ctx, f.repository.Pool(), ids[39], false)
	if err != nil {
		t.Fatal(err)
	}
	current := second.Goals[0].Goal
	complete := GoalEvaluationCandidate{GoalID: goalID, ExpectedRevision: current.Revision, CriteriaVersion: current.CriteriaVersion, Impact: "completed", Judgments: []GoalCriterionJudgment{{CriterionID: current.CriterionIDs[0], Verdict: "satisfied", Kind: "communication", Subject: "actor_self", Discourse: "assertion", EvidenceRefs: []string{preference.Ref, recommendation.Ref}, Reason: "真实偏好与后续具体小说和理由共同满足原标准"}}}
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		settled, err := f.app.commitGoalEvaluationTx(f.ctx, tx, current, complete, map[string]GoalSource{preference.Ref: preference, recommendation.Ref: recommendation}, "retained-complete-"+f.suffix)
		if err != nil {
			return err
		}
		if settled.Status != GoalCompleted || settled.Progress != 1 || settled.CriteriaVersion != current.CriteriaVersion {
			t.Fatal("actual evidence did not close original criterion", settled)
		}
		candidate.ExpectedRevision = settled.Revision
		terminal, err := f.app.commitGoalEvaluationTx(f.ctx, tx, settled, candidate, nil, "retained-terminal-"+f.suffix)
		if err != nil {
			return err
		}
		if terminal.Status != GoalCompleted || terminal.Revision != settled.Revision {
			t.Fatal("historical recovery reopened/revised terminal root", terminal)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	// A real neighboring edit invalidates the entire frozen interpretation unit.
	if _, err := f.repository.Pool().Exec(f.ctx, `UPDATE public.conversation_messages SET text='changed actual question' WHERE id=$1`, fmt.Sprintf("retained-0-%s", f.suffix)); err != nil {
		t.Fatal(err)
	}
	live, err := readGoalSourceWith(f.ctx, f.repository.Pool(), ids[1], false)
	if err != nil {
		t.Fatal(err)
	}
	if goalAssessmentSourceFingerprint(live) == goalAssessmentSourceFingerprint(answer) {
		t.Fatal("real neighboring edit bypassed frozen evidence fence")
	}
}

func TestGoalFreshAndHistoricalUnitsShareBoundedAdmission(t *testing.T) {
	at := time.Now()
	snapshot := goalEvaluationSnapshot{UnprocessedSourceIDs: []int64{3, 2}, RecoverySourceIDs: []int64{1}, Sources: []GoalSource{
		{Ref: "source:3", EventID: 3, Kind: "message", RecordedAt: at, Data: map[string]any{"text": strings.Repeat("fresh message ", 600)}},
		{Ref: "source:2", EventID: 2, Kind: "message", RecordedAt: at.Add(-time.Minute), Data: map[string]any{"text": strings.Repeat("another new message ", 600)}},
		{Ref: "source:1", EventID: 1, Kind: "message", RecordedAt: at.Add(-time.Hour), Data: map[string]any{"text": "earlier actual preference"}},
	}}
	selected, err := admitGoalEvaluationSourceInput(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(selected.ProviderSourceIDs, int64(3)) || !slices.Contains(selected.ProviderSourceIDs, int64(1)) {
		t.Fatal("fresh backlog starved complementary historical evidence", selected.ProviderSourceIDs)
	}
}

func TestGoalLinkedBacklogDoesNotStarveUnlinkedRecoveryAdmission(t *testing.T) {
	linked := []int64{}
	snapshot := goalEvaluationSnapshot{}
	for i := int64(1); i <= 32; i++ {
		linked = append(linked, i)
		snapshot.Sources = append(snapshot.Sources, GoalSource{Ref: fmt.Sprintf("source:%d", i), EventID: i, Kind: "message", Data: map[string]any{"text": strings.Repeat("linked backlog ", 500)}})
	}
	snapshot.Sources = append(snapshot.Sources, GoalSource{Ref: "source:33", EventID: 33, Kind: "message", Data: map[string]any{"text": "old unlinked actual preference"}})
	snapshot.RecoverySourceIDs = interleaveGoalRecoveryLanes(linked, []int64{33})
	selected, err := admitGoalEvaluationSourceInput(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(selected.ProviderSourceIDs, int64(33)) {
		t.Fatal("linked recovery backlog starved unlinked evidence", selected.ProviderSourceIDs)
	}
}
