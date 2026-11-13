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

func TestVisualSubmissionRecoveryRequiresReviewAndOriginalRun(t *testing.T) {
	body := map[string]any{"stage": "candidate", "expectedAttempt": float64(1), "expectedVisualRevision": float64(0), "expectedMediaRevision": float64(0), "workflowId": "original", "sourceRunId": "original-run", "expectedWorkflowRevision": float64(1), "submissionReviewed": true, "reason": "Owner verified original image", "idempotencyKey": "reviewed"}
	if !validVisualSubmissionRecovery(body) {
		t.Fatal("valid recovery rejected")
	}
	mapped := surfaceCommandBody("/api/fluctlights/A/visual-sessions/S/resume-reviewed", body)
	if mapped["submission_reviewed"] != true || mapped["source_run_id"] != "original-run" || mapped["expected_workflow_revision"] != float64(1) {
		t.Fatal(mapped)
	}
	for _, key := range []string{"expectedAttempt", "expectedVisualRevision", "expectedMediaRevision", "workflowId", "sourceRunId", "expectedWorkflowRevision", "submissionReviewed"} {
		value := body[key]
		delete(body, key)
		if validVisualSubmissionRecovery(body) {
			t.Fatal("missing recovery fence admitted", key)
		}
		body[key] = value
	}
	body["submissionReviewed"] = false
	if validVisualSubmissionRecovery(body) {
		t.Fatal("unreviewed result admitted")
	}
}
