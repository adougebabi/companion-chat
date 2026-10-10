package core

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestRecommendationEvaluationReachesProviderAfterLargeInspectionBacklog(t *testing.T) {
	f := seedWardrobeToolFixture(t)
	goalID := createDialogueGoalForClosure(t, f, []string{"根据实际科幻阅读偏好推荐具体作品并说明理由"})
	for i := 0; i < 40; i++ {
		action := fmt.Sprintf("inspection-backlog-%d-%s", i, f.suffix)
		results := []CapabilityResult{{CallID: "inspect", CapabilityName: "goal.inspect", Status: "completed", Output: map[string]any{"goal": map[string]any{"id": goalID, "reviews": strings.Repeat("full inspection history", 240)}}}}
		outcomes, err := buildActionOutcomes(action, f.fluctlightID, "fixture", "capability", results, map[string]any{"status": "completed"}, f.app.capabilityRegistry(), f.app.now())
		if err != nil {
			t.Fatal(err)
		}
		if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error { return persistActionOutcomesTx(f.ctx, tx, outcomes) }); err != nil {
			t.Fatal(err)
		}
	}
	// These genuine domain query results remain semantically available, but
	// their combined size requires more than one offered source batch.
	for i := 0; i < 16; i++ {
		action := fmt.Sprintf("domain-backlog-%d-%s", i, f.suffix)
		results := []CapabilityResult{{CallID: "domain-query", CapabilityName: wardrobeInspectCapabilityName, Status: "completed", Output: map[string]any{"notes": strings.Repeat("domain information ", 160)}}}
		outcomes, err := buildActionOutcomes(action, f.fluctlightID, "fixture", "capability", results, map[string]any{"status": "completed"}, f.app.capabilityRegistry(), f.app.now())
		if err != nil {
			t.Fatal(err)
		}
		if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error { return persistActionOutcomesTx(f.ctx, tx, outcomes) }); err != nil {
			t.Fatal(err)
		}
	}
	desired, motivation := "await a separate real result", "keep unfinished goal active"
	waiting, err := f.app.ApplyOwnerGoalCommand(f.ctx, f.ownerID, f.fluctlightID, "", GoalOwnerCommand{Operation: "create", IdempotencyKey: "waiting-remainder-goal", Reason: "source remainder", DesiredOutcome: &desired, Motivation: &motivation, SuccessCriteria: []string{"another real result"}})
	if err != nil {
		t.Fatal(err)
	}
	waitingID := stringValue(waiting["goal_id"])
	var omittedID, admittedID int64
	messageID := "novel-recommendation-" + f.suffix
	if err := withTransaction(f.ctx, f.repository.Pool(), func(tx pgx.Tx) error {
		for _, m := range []struct {
			id, author, kind, text string
			seq                    int
		}{{"novel-preference-" + f.suffix, f.ownerID, "user", "平时喜欢看科幻类型的，有啥推荐", 50}, {messageID, f.fluctlightID, "assistant", "我推荐特德·姜的《你一生的故事》，它以语言与时间为科幻设定，适合你的科幻阅读偏好。", 51}} {
			if _, err := tx.Exec(f.ctx, `INSERT INTO public.conversation_messages(id,conversation_id,sequence,author_actor_id,kind,text,attachment_refs,idempotency_key) VALUES($1,$2,$3,$4,$5,$6,'[]',$1)`, m.id, f.conversationID, m.seq, m.author, m.kind, m.text); err != nil {
				return err
			}
			if err := f.app.recordGoalMessageTx(f.ctx, tx, f.fluctlightID, m.id, "default"); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	seedCognitiveProviderRole(t, f.ctx, f.repository, "recommendation-input-"+f.suffix)
	calls := 0
	f.app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().onGoalEvaluation(func(payload map[string]any) fakeProviderResult {
		calls++
		snapshot := readProcessingGoalSnapshot(t, f)
		input, err := renderedGoalEvaluationProviderInput(snapshot)
		if err != nil {
			t.Fatal(err)
		}
		if EstimatePromptTokens(input) > goalEvaluationSourceInputBudget {
			t.Fatal("provider offered an oversized current input")
		}
		entry := goalEvaluationGoal{}
		for _, candidate := range snapshot.Goals {
			if candidate.GoalID == goalID {
				entry = candidate
			}
		}
		wire := jsonString(payload)
		for _, key := range []string{"expected_revision", "criteria_version", "criterion_ids", "conversation_id", "source_version"} {
			if strings.Contains(wire, "\""+key+"\"") {
				t.Fatalf("raw metadata key escaped HTTP payload: %s", key)
			}
		}
		for _, forbidden := range []string{entry.GoalID, entry.Goal.EntityID, entry.Goal.ProfileID} {
			if forbidden != "" && strings.Contains(wire, forbidden) {
				t.Fatalf("raw Goal authority escaped actual HTTP payload: %s", forbidden)
			}
		}
		for _, criterionID := range entry.Goal.CriterionIDs {
			if strings.Contains(wire, criterionID) {
				t.Fatalf("raw criterion escaped actual HTTP payload: %s", criterionID)
			}
		}
		binding, bindErr := newGoalEvaluationWireBinding(snapshot)
		if bindErr != nil {
			t.Fatal(bindErr)
		}
		for _, source := range admittedGoalEvaluationSources(snapshot) {
			forbiddenValues := []string{source.ID, source.ConversationID, snapshot.FluctlightID, snapshot.OwnerActorID}
			// Short integers/profile labels also occur in legitimate content and
			// local references. Assert their metadata keys separately.
			if len(source.Version) > 8 {
				forbiddenValues = append(forbiddenValues, source.Version)
			}
			if len(source.ProfileID) > 8 {
				forbiddenValues = append(forbiddenValues, source.ProfileID)
			}
			if binding.sourceRefs[source.Ref] != source.Ref {
				forbiddenValues = append(forbiddenValues, source.Ref)
			}
			for _, forbidden := range forbiddenValues {
				if forbidden != "" && strings.Contains(wire, forbidden) {
					at := strings.Index(wire, forbidden)
					t.Fatalf("raw source authority escaped actual HTTP payload: %s near %s", forbidden, wire[max(0, at-65):min(len(wire), at+100)])
				}
			}
		}
		messages := arrayValue(payload["messages"])
		lastContent := stringValue(mapValue(messages[len(messages)-1])["content"])
		_, stable, current, splitErr := goalEvaluationWireInput(snapshot)
		if splitErr != nil {
			t.Fatal(splitErr)
		}
		if stringValue(mapValue(messages[1])["content"]) != stringValue(stableTaskContextMessage(stable)["content"]) || lastContent != jsonString(current) {
			expected := jsonString(current)
			at := 0
			for at < min(len(expected), len(lastContent)) && expected[at] == lastContent[at] {
				at++
			}
			actualStable := stringValue(mapValue(messages[1])["content"])
			expectedStable := stringValue(stableTaskContextMessage(stable)["content"])
			sat := 0
			for sat < min(len(actualStable), len(expectedStable)) && actualStable[sat] == expectedStable[sat] {
				sat++
			}
			t.Fatalf("split mismatch: current_equal=%v stable diff=%d actual=%q expected=%q currentdiff=%d", lastContent == expected, sat, actualStable[max(0, sat-20):min(len(actualStable), sat+100)], expectedStable[max(0, sat-20):min(len(expectedStable), sat+100)], at)
		}
		if calls == 1 {
			for _, source := range snapshot.Sources {
				offered := false
				for _, id := range snapshot.ProviderSourceIDs {
					if id == source.EventID {
						offered = true
					}
				}
				if !offered && omittedID == 0 {
					omittedID = source.EventID
				}
				if offered && source.ID == messageID {
					admittedID = source.EventID
				}
			}
		}
		proof := ""
		for _, source := range admittedGoalEvaluationSources(snapshot) {
			if source.ID == messageID {
				proof = source.Ref
			}
		}
		if calls == 1 && proof == "" {
			t.Fatal("recent recommendation starved behind old inspection receipts")
		}
		candidates := []GoalEvaluationCandidate{}
		for _, target := range snapshot.Goals {
			candidate := GoalEvaluationCandidate{GoalID: target.GoalID, ExpectedRevision: target.Goal.Revision, CriteriaVersion: target.Goal.CriteriaVersion, Impact: "needs_evidence", WaitCondition: "await separate result", Judgments: []GoalCriterionJudgment{}}
			if target.GoalID == goalID {
				candidate.Impact = "completed"
				candidate.WaitCondition = ""
				candidate.Judgments = []GoalCriterionJudgment{{CriterionID: entry.Goal.CriterionIDs[0], Verdict: "satisfied", Kind: "communication", Subject: "actor_self", Discourse: "assertion", EvidenceRefs: []string{proof}, Reason: "actual preference-based recommendation"}}
			}
			candidates = append(candidates, candidate)
		}
		return fakeProviderResult{Structured: goalEvaluationProviderFixture(snapshot, GoalEvaluationTaskOutput{Evaluations: candidates, Plans: []GoalPlanCandidate{}})}
	})}
	request := latestPendingGoalRequest(t, f)
	if _, err := f.app.ProcessGoalEvaluationIntent(f.ctx, request); err != nil {
		t.Fatal(err)
	}
	if omittedID == 0 || admittedID == 0 {
		t.Fatalf("test did not exercise within-snapshot admission: omitted=%d admitted=%d", omittedID, admittedID)
	}
	var omittedProcessed, admittedProcessed bool
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT processed_at IS NOT NULL FROM public.goal_source_events WHERE id=$1`, omittedID).Scan(&omittedProcessed); err != nil {
		t.Fatal(err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT processed_at IS NOT NULL FROM public.goal_source_events WHERE id=$1`, admittedID).Scan(&admittedProcessed); err != nil {
		t.Fatal(err)
	}
	if omittedProcessed || !admittedProcessed {
		t.Fatalf("unoffered proof was consumed: omitted=%v offered=%v", omittedProcessed, admittedProcessed)
	}
	var remainder string
	var pendingCount int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*),min(id) FROM public.goal_evaluation_requests WHERE fluctlight_id=$1 AND status='pending' AND reason='assessment_source_remainder' AND goal_ids ? $2`, f.fluctlightID, waitingID).Scan(&pendingCount, &remainder); err != nil || pendingCount != 1 {
		t.Fatalf("source remainder not queued once: %d %v", pendingCount, err)
	}
	if _, err := f.app.ProcessGoalEvaluationIntent(f.ctx, remainder); err != nil {
		t.Fatal(err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT processed_at IS NOT NULL FROM public.goal_source_events WHERE id=$1`, omittedID).Scan(&omittedProcessed); err != nil || !omittedProcessed {
		t.Fatalf("admissible remainder did not advance: %v %v", omittedProcessed, err)
	}
	var state string
	var resolutions, attempts, unprocessed int
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT status FROM public.fluctlight_goals WHERE id=$1`, goalID).Scan(&state); err != nil {
		t.Fatal(err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.goal_resolutions WHERE goal_id=$1`, goalID).Scan(&resolutions); err != nil {
		t.Fatal(err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.fluctlight_intention_attempts WHERE fluctlight_id=$1`, f.fluctlightID).Scan(&attempts); err != nil {
		t.Fatal(err)
	}
	if err := f.repository.Pool().QueryRow(f.ctx, `SELECT count(*) FROM public.goal_source_events WHERE fluctlight_id=$1 AND processed_at IS NULL`, f.fluctlightID).Scan(&unprocessed); err != nil {
		t.Fatal(err)
	}
	if state != "completed" || resolutions != 1 || attempts != 0 || calls != 2 || unprocessed == 0 {
		t.Fatalf("recommendation closure/source preservation failed: %s resolutions=%d attempts=%d calls=%d unprocessed=%d", state, resolutions, attempts, calls, unprocessed)
	}
}
