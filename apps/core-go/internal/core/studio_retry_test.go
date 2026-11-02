package core

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestStudioAutomaticRetryOnlyRetriesTransientFailures(t *testing.T) {
	for _, reason := range []string{"adk_run: context deadline exceeded", "studio_request_timeout", "activity Heartbeat timeout", "provider http 503", "provider http 429", "connection reset by peer"} {
		if !studioFailureRetryable(reason) {
			t.Errorf("transient failure %q was not retryable", reason)
		}
	}
	for _, reason := range []string{"request_cancelled", "provider http 403", "insufficient_quota timeout", "studio_snapshot_invalid", "media_not_configured", "invalid JSON"} {
		if studioFailureRetryable(reason) {
			t.Errorf("permanent failure %q was retried", reason)
		}
	}
	if studioAutomaticMaximumAttempts != 3 || studioAutomaticRetryDelay(0) != 15*time.Second || studioAutomaticRetryDelay(1) != time.Minute {
		t.Fatal("Studio retries must be bounded with increasing durable backoff")
	}
}

func TestStudioTimeoutCreatesNewAttemptsWithBackoffThenExhausts(t *testing.T) {
	ctx, repo, app, owner, _, _ := setupDirectPublicationToolTest(t, "studio-automatic-retry")
	resource, err := app.DispatchOwnerSurface(ctx, "POST", "/internal/studio/resources", nil, owner, map[string]any{"kind": "place", "name": "Retry definition", "body": map[string]any{"description": "original"}, "reason": "test", "idempotency_key": "retry-resource"})
	if err != nil {
		t.Fatal(err)
	}
	job, err := app.DispatchStudioJobSurface(ctx, "POST", "/internal/studio/jobs", nil, owner, map[string]any{"resource_id": resource["id"], "resource_revision": resource["revision"], "kind": "assistant", "name": "Retry definition", "body": map[string]any{"description": "original"}, "instruction": "extend", "reason": "test", "idempotency_key": "retry-job"})
	if err != nil {
		t.Fatal(err)
	}
	id := stringValue(job["id"])
	seedCognitiveProviderRole(t, ctx, repo, "studio-automatic-retry")
	calls := 0
	app.Provider.HTTP = &http.Client{Transport: newFakeProviderRouter().otherwise(func(map[string]any) fakeProviderResult {
		calls++
		return fakeProviderResult{Err: context.DeadlineExceeded}
	})}
	for attempt := 1; attempt <= 3; attempt++ {
		intent := fmt.Sprintf("studio_intent:%s_attempt_%d", id, attempt)
		if _, err := app.ProcessStudioGeneration(ctx, intent); err == nil {
			t.Fatal("scripted model timeout succeeded")
		}
		outcome, err := app.ReconcileStudioWorkflowFailure(ctx, intent, "adk_run: context deadline exceeded")
		if err != nil {
			t.Fatal(err)
		}
		actual, err := app.StudioJob(ctx, owner, id)
		if err != nil {
			t.Fatal(err)
		}
		if attempt < 3 {
			if outcome.Status != "retry" || actual["status"] != "queued" || intValue(actual["latest_attempt"]) != attempt+1 {
				t.Fatal(outcome, actual)
			}
			var status string
			var delay float64
			var retries int
			if err := repo.Pool().QueryRow(ctx, `SELECT status,extract(epoch FROM next_attempt_at-now()),(payload->>'automatic_retry_count')::integer FROM public.platform_workflow_intents WHERE intent_id=$1`, outcome.NextIntentID).Scan(&status, &delay, &retries); err != nil {
				t.Fatal(err)
			}
			if status != "retry" || retries != attempt || delay < outcome.Delay.Seconds()-3 || delay > outcome.Delay.Seconds() {
				t.Fatal(status, delay, retries, outcome)
			}
			// Reobserving an old failure cannot consume another attempt or move
			// the successor deadline; a stale execution cannot call the model.
			duplicate, err := app.ReconcileStudioWorkflowFailure(ctx, intent, "timeout")
			if err != nil || duplicate.NextIntentID != "" {
				t.Fatal(duplicate, err)
			}
			if _, err := app.ProcessStudioGeneration(ctx, intent); err == nil || !strings.Contains(err.Error(), "explicit_retry") {
				t.Fatal("old failed attempt reopened", err)
			}
		} else {
			if outcome.Status != "dead_letter" || actual["status"] != "failed" || intValue(actual["latest_attempt"]) != 3 {
				t.Fatal(outcome, actual)
			}
			var status string
			if err := repo.Pool().QueryRow(ctx, `SELECT status FROM public.platform_workflow_intents WHERE intent_id=$1`, intent).Scan(&status); err != nil || status != "dead_letter" {
				t.Fatal(status, err)
			}
			// Explicit recovery remains available and starts a fresh bounded series.
			manual, err := app.DispatchStudioJobSurface(ctx, "POST", "/internal/studio/jobs/"+id+"/retry", nil, owner, map[string]any{"expected_revision": actual["revision"], "reason": "retry after fixing provider", "idempotency_key": "manual-after-exhaustion"})
			if err != nil || intValue(manual["latest_attempt"]) != 4 {
				t.Fatal(manual, err)
			}
		}
	}
	if calls != 3 {
		t.Fatal("failed domain attempt was replayed or retry was not bounded", calls)
	}
}

func TestStudioAutomaticRetryReusesValidatedStageAndPreservesDiscard(t *testing.T) {
	ctx, repo, app, owner, _, _ := setupDirectPublicationToolTest(t, "studio-retry-stage")
	resource, err := app.DispatchOwnerSurface(ctx, "POST", "/internal/studio/resources", nil, owner, map[string]any{"kind": "place", "name": "Stage", "body": map[string]any{"description": "original"}, "reason": "test", "idempotency_key": "stage-resource"})
	if err != nil {
		t.Fatal(err)
	}
	job, err := app.DispatchStudioJobSurface(ctx, "POST", "/internal/studio/jobs", nil, owner, map[string]any{"resource_id": resource["id"], "resource_revision": resource["revision"], "kind": "text_preview", "name": "Stage", "body": map[string]any{"description": "original"}, "instruction": "preview", "reason": "test", "idempotency_key": "stage-job"})
	if err != nil {
		t.Fatal(err)
	}
	id := stringValue(job["id"])
	intent := "studio_intent:" + id + "_attempt_1"
	output := map[string]any{"text": "A validated fictional stage", "label": "虚构创作预览·不进入生活"}
	if _, err := repo.Pool().Exec(ctx, `UPDATE public.studio_job_attempts SET status='running',output=$2,provenance='{"stage":"validated"}' WHERE job_id=$1`, id, jsonBytes(output)); err != nil {
		t.Fatal(err)
	}
	outcome, err := app.ReconcileStudioWorkflowFailure(ctx, intent, "activity heartbeat timeout")
	if err != nil || outcome.Status != "retry" {
		t.Fatal(outcome, err)
	}
	app.Provider = nil
	if _, err := app.ProcessStudioGeneration(ctx, outcome.NextIntentID); err != nil {
		t.Fatal("durable validated output required a second model call", err)
	}
	completed, err := app.StudioJob(ctx, owner, id)
	if err != nil || completed["status"] != "completed" || mapValue(completed["result"])["text"] != output["text"] {
		t.Fatal(completed, err)
	}
	if _, err := app.ReconcileStudioWorkflowFailure(ctx, intent, "timeout"); err != nil {
		t.Fatal(err)
	}
	discarded, err := app.DispatchStudioJobSurface(ctx, "POST", "/internal/studio/jobs/"+id+"/discard", nil, owner, map[string]any{"expected_revision": completed["revision"], "reason": "discard", "idempotency_key": "discard-stage"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.ReconcileStudioWorkflowFailure(ctx, outcome.NextIntentID, "timeout"); err != nil {
		t.Fatal(err)
	}
	current, err := app.StudioJob(ctx, owner, id)
	if err != nil || current["status"] != "discarded" || current["revision"] != discarded["revision"] {
		t.Fatal(current, err)
	}
}
