package browser

import "testing"

func TestManualVisualReviewAcceptsDecisionOnlyAndRequiresEvidenceForRegeneration(t *testing.T) {
	base := func(decision string) map[string]any {
		return map[string]any{"expectedAttempt": float64(1), "expectedVisualRevision": float64(0), "decision": decision, "idempotencyKey": "review-key"}
	}
	accepted := base("accepted")
	if !validManualVisualReview(accepted) {
		t.Fatal("manual acceptance should not require fabricated scores, observations, summary, or reason")
	}
	rejected := base("regenerate")
	if validManualVisualReview(rejected) {
		t.Fatal("regeneration without any actual correction evidence must be rejected")
	}
	rejected["feedback"] = "侧面视图缺失"
	if !validManualVisualReview(rejected) {
		t.Fatal("manual rejection with actionable feedback should be accepted")
	}
	legacy := base("accepted")
	legacy["identityMatch"], legacy["confidence"] = 0.9, 0.8
	legacy["observations"], legacy["missingSections"] = []any{"面部一致"}, []any{}
	legacy["summary"], legacy["feedback"], legacy["reason"] = "符合", "", "人工确认"
	if !validManualVisualReview(legacy) {
		t.Fatal("existing complete manual review requests must remain valid")
	}
	legacy["missingSections"] = []any{"侧面"}
	if validManualVisualReview(legacy) {
		t.Fatal("acceptance cannot claim a missing required section")
	}
	accepted["identityMatch"] = 1.1
	if validManualVisualReview(accepted) {
		t.Fatal("supplied scores still require validation")
	}
}
