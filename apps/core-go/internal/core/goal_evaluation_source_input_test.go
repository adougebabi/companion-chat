package core

import (
	"strings"
	"testing"
	"time"
)

func TestGoalEvaluationInputDoesNotEmbedInspectionSnapshotsOrFactAudit(t *testing.T) {
	large := strings.Repeat("audit-payload-", 4000)
	fact := map[string]any{"attribute": "reading_preference", "value_json": "科幻", "epistemic_kind": "explicit", "valid_from": "2026-10-08T05:34:21Z", "source_fingerprint": large, "operation_id": large}
	message := "科幻的话，我推荐特德·姜的《你一生的故事》，它以语言与时间的设定展开，适合你喜欢的科幻类型。"
	input := goalEvaluationSnapshot{FluctlightID: "self", OwnerActorID: "owner", Goals: []goalEvaluationGoal{{GoalID: "novel-goal", Goal: GoalAuthority{EntityID: "novel-goal", FluctlightID: "self", Revision: 1, CriteriaVersion: 1, CriterionIDs: []string{"criterion"}, SuccessCriteria: []string{"根据实际阅读偏好推荐一本小说并说明理由"}, Status: GoalActive}}}, Sources: []GoalSource{
		{Ref: "source:1", EventID: 1, Kind: "message", Valid: true, CanSupportSuccess: true, Data: map[string]any{"text": message, "message_kind": "assistant"}},
		{Ref: "source:2", EventID: 2, Kind: "outcome", Valid: true, Data: map[string]any{"capability": "goal.inspect", "status": "completed", "observed": map[string]any{"goal": map[string]any{"id": "novel-goal", "execution": map[string]any{"history": large}, "stages": large, "reviews": large}}}},
		{Ref: "source:3", EventID: 3, Kind: "actor_fact", Valid: true, CanSupportSuccess: true, Data: map[string]any{"fact": fact}},
		{Ref: "source:4", EventID: 4, Kind: "outcome", Valid: true, Data: map[string]any{"capability": "actor.inspect", "status": "completed", "observed": map[string]any{"actor_id": "inspected-human", "history": false, "facts": []any{map[string]any{"actor_id": "inspected-human", "attribute": "favorite_setting", "value": "外星语言", "effective_at": "2026-10-08T05:00:00Z", "epistemic_kind": "explicit", "source_ref": large}}}}},
		{Ref: "source:5", EventID: 5, Kind: "outcome", Valid: true, CanSupportSuccess: true, Data: map[string]any{"capability": "wardrobe.inspect", "status": "completed", "observed": map[string]any{"items": []any{map[string]any{"id": "boots", "description": "黑色短靴", "ownership": "owned"}}}}},
		{Ref: "source:6", EventID: 6, Kind: "outcome", Valid: true, Data: map[string]any{"capability": "actor.fact.record", "status": "completed", "observed": map[string]any{"actor_id": "recorded-human", "attribute": "favorite_author", "value": "特德·姜", "effective_at": "2026-10-08T06:00:00Z", "epistemic_kind": "explicit", "status": "active"}}},
	}}
	for index := range input.Sources {
		input.Sources[index].FluctlightID = "self"
	}
	wire := jsonString(compactGoalEvaluationInput(input))
	rendered, err := renderedGoalEvaluationProviderInput(input)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(wire, large) {
		t.Fatal("inspection snapshot or fact audit escaped into current input")
	}
	for _, required := range []string{message, "科幻", "外星语言", "criterion:1.1", "e1", "e3", "object:", "黑色短靴", "actor_target:", "特德·姜", "2026-10-08T05:00:00Z", "2026-10-08T06:00:00Z"} {
		if !strings.Contains(wire, required) {
			t.Fatalf("compaction lost actual evidence: %s", required)
		}
	}
	for _, forbidden := range []string{"novel-goal", "boots", "inspected-human", "recorded-human"} {
		if strings.Contains(wire, forbidden) {
			t.Fatalf("raw identifier escaped semantic input: %s", forbidden)
		}
	}
	if EstimatePromptTokens(rendered) > defaultCurrentInputTokensCap {
		t.Fatalf("current input still exceeds original cap: %d", EstimatePromptTokens(rendered))
	}
	if fact["source_fingerprint"] != large {
		t.Fatal("provider compaction mutated durable source")
	}
}

func TestGoalEvaluationSourceBudgetRetainsPriorAndRecommendationWithoutConsumingLargeSource(t *testing.T) {
	input := goalEvaluationSnapshot{Goals: []goalEvaluationGoal{{Goal: GoalAuthority{EntityID: "novel", CriterionIDs: []string{"criterion"}, SuccessCriteria: []string{"recommend a novel"}}, CurrentJudgments: []GoalCriterionJudgment{{EvidenceRefs: []string{"source:1"}}}}}, Sources: []GoalSource{
		{EventID: 1, Ref: "source:1", Kind: "message", Data: map[string]any{"text": "我喜欢科幻"}},
		{EventID: 2, Ref: "source:2", Kind: "outcome", Data: map[string]any{"capability": "domain.query", "observed": map[string]any{"large": strings.Repeat("domain evidence", 15000)}}},
		{EventID: 3, Ref: "source:3", Kind: "message", Data: map[string]any{"text": "推荐《你一生的故事》，它以语言与时间为科幻设定。"}},
	}}
	selected, err := admitGoalEvaluationSourceInput(input)
	if err != nil {
		t.Fatal(err)
	}
	wire := jsonString(compactGoalEvaluationInput(selected))
	rendered, err := renderedGoalEvaluationProviderInput(selected)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(wire, "我喜欢科幻") || !strings.Contains(wire, "推荐《你一生的故事》") || strings.Contains(wire, "domain evidence") {
		t.Fatalf("source priority/admission failed: %v", selected.ProviderSourceIDs)
	}
	if len(selected.Sources) != 3 || EstimatePromptTokens(rendered) > goalEvaluationSourceInputBudget {
		t.Fatal("durable snapshot lost a source or offered input exceeds bound")
	}
}

func TestGoalEvaluationSourceAdmissionUsesRecordedTimeBeforeEventID(t *testing.T) {
	at := time.Now().UTC()
	text := strings.Repeat("long message ", 700)
	input := goalEvaluationSnapshot{Sources: []GoalSource{
		{EventID: 100, Ref: "source:100", Kind: "message", RecordedAt: at.Add(-time.Hour), Data: map[string]any{"text": text + " old"}},
		{EventID: 1, Ref: "source:1", Kind: "message", RecordedAt: at, Data: map[string]any{"text": text + " new"}},
	}}
	selected, err := admitGoalEvaluationSourceInput(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(selected.ProviderSourceIDs) != 1 || selected.ProviderSourceIDs[0] != 1 {
		t.Fatalf("arrival ID displaced newer evidence: %v", selected.ProviderSourceIDs)
	}
}
