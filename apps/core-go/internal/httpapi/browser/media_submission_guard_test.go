package browser

import "testing"

func TestMediaRecoveryReviewAcknowledgementMapsToCore(t *testing.T) {
	body := surfaceCommandBody("/api/diagnostic-records/media-prompts/fixture/retry", map[string]any{"expectedRevision": 3, "mode": "regenerate", "reason": "已人工核对", "idempotencyKey": "reviewed-retry", "submissionReviewed": true})
	if body["submission_reviewed"] != true || body["expected_revision"] != 3 || body["mode"] != "regenerate" {
		t.Fatalf("review acknowledgement did not reach the Core command: %#v", body)
	}
}
