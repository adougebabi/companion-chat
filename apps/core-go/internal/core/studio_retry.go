package core

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const studioAutomaticMaximumAttempts = 3

type StudioWorkflowRetryOutcome struct {
	Status       string
	ReasonCode   string
	NextIntentID string
	Delay        time.Duration
}

func studioAutomaticRetryDelay(retries int) time.Duration {
	if retries == 0 {
		return 15 * time.Second
	}
	return time.Minute
}

func studioFailureRetryable(reason string) bool {
	text := strings.ToLower(reason)
	for _, terminal := range []string{"cancelled", "canceled", "insufficient_quota", "insufficient_user_quota", "provider http 401", "provider http 403", "not_configured", "snapshot_invalid", "invalid_argument"} {
		if strings.Contains(text, terminal) {
			return false
		}
	}
	for _, transient := range []string{"timeout", "timed out", "timed_out", "deadline exceeded", "deadline_exceeded", "provider http 429", "provider http 5", "connection reset", "connection refused", "unexpected eof", "temporarily unavailable", "get_channel_failed"} {
		if strings.Contains(text, transient) {
			return true
		}
	}
	return false
}

// ReconcileStudioWorkflowFailure retries a terminal execution with a new domain
// attempt and a durable dispatch deadline. Each explicit Owner retry starts a
// fresh bounded automatic series; replaying an old terminal observation cannot
// enqueue another successor or overwrite a completed/discarded/latest attempt.
func (a *App) ReconcileStudioWorkflowFailure(ctx context.Context, intentID, failure string) (out StudioWorkflowRetryOutcome, err error) {
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		var owner, id, aid string
		if err := tx.QueryRow(ctx, `SELECT j.owner_actor_id,j.id,a.id FROM public.platform_workflow_intents i JOIN public.studio_job_attempts a ON a.id=i.payload->>'studio_attempt_id' JOIN public.studio_jobs j ON j.id=a.job_id WHERE i.intent_id=$1 AND i.intent_type='studio.generate'`, intentID).Scan(&owner, &id, &aid); err != nil {
			return err
		}
		job, err := readStudioJobWith(ctx, tx, owner, id, true)
		if err != nil {
			return err
		}
		var attempt int
		var code string
		if err := tx.QueryRow(ctx, `SELECT attempt,COALESCE(error_code,'') FROM public.studio_job_attempts WHERE id=$1 FOR UPDATE`, aid).Scan(&attempt, &code); err != nil {
			return err
		}
		var payload []byte
		var intentStatus string
		if err := tx.QueryRow(ctx, `SELECT payload,status FROM public.platform_workflow_intents WHERE intent_id=$1 FOR UPDATE`, intentID).Scan(&payload, &intentStatus); err != nil {
			return err
		}
		if intentStatus == "dead_letter" {
			out.Status = "dead_letter"
			out.ReasonCode = "studio_workflow_already_settled"
			return nil
		}
		if attempt != intValue(job["latest_attempt"]) || job["status"] == "completed" || job["status"] == "discarded" {
			out.Status = "failed"
			out.ReasonCode = "studio_attempt_superseded"
			_, err := tx.Exec(ctx, `UPDATE public.platform_workflow_intents SET status='failed',completed_at=COALESCE(completed_at,now()),last_error=COALESCE(NULLIF(last_error,''),$2) WHERE intent_id=$1 AND status IN ('pending','retry','started','failed')`, intentID, boundedOperationalCause(failure))
			return err
		}
		if job["kind"] == "image_preview" {
			var delegated bool
			if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.media_intents WHERE studio_job_id=$1 AND studio_owner_actor_id=$2 AND media_scope='studio' AND status IN ('pending','running','completed'))`, id, owner).Scan(&delegated); err != nil {
				return err
			}
			if delegated {
				// A committed media consumer owns the running product job. A lost
				// planner receipt must not replace its workflow or fail its attempt.
				out.Status, out.ReasonCode = "failed", "studio_media_already_delegated"
				_, err := tx.Exec(ctx, `UPDATE public.platform_workflow_intents SET status='failed',completed_at=COALESCE(completed_at,now()),last_error=$2 WHERE intent_id=$1`, intentID, boundedOperationalCause(failure))
				return err
			}
		}
		retries := intValue(decodeObject(payload)["automatic_retry_count"])
		if retries < 0 {
			retries = studioAutomaticMaximumAttempts
		}
		retry := intentStatus != "cancel_requested" && intentStatus != "cancelled" && studioFailureRetryable(failure+" "+code) && retries < studioAutomaticMaximumAttempts-1
		out.Status = "dead_letter"
		out.ReasonCode = "studio_workflow_terminal_failure"
		if studioFailureRetryable(failure+" "+code) && retries >= studioAutomaticMaximumAttempts-1 {
			out.ReasonCode = "studio_retry_exhausted"
		}
		if retry {
			out.Status = "retry"
			out.ReasonCode = "studio_retry_scheduled"
		}
		code = safeAgentFailureCode(code, "studio_workflow_failed")
		if _, err := tx.Exec(ctx, `UPDATE public.studio_job_attempts SET status='failed',error_code=$2,finished_at=COALESCE(finished_at,now()) WHERE id=$1 AND status IN ('queued','running')`, aid, code); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE public.studio_jobs SET status='failed',error_code=$2,revision=revision+1,updated_at=now() WHERE id=$1 AND status IN ('queued','running')`, id, code); err != nil {
			return err
		}
		settledStatus := "dead_letter"
		if retry {
			settledStatus = "failed"
		}
		if _, err := tx.Exec(ctx, `UPDATE public.platform_workflow_intents SET status=$2,completed_at=COALESCE(completed_at,now()),last_error=$3 WHERE intent_id=$1`, intentID, settledStatus, boundedOperationalCause(failure)); err != nil {
			return err
		}
		if !retry {
			return nil
		}
		if err := retryStudioAttemptTx(ctx, tx, owner, job); err != nil {
			return err
		}
		out.NextIntentID = fmt.Sprintf("studio_intent:%s_attempt_%d", id, attempt+1)
		out.Delay = studioAutomaticRetryDelay(retries)
		_, err = tx.Exec(ctx, `UPDATE public.platform_workflow_intents SET status='retry',next_attempt_at=now()+($2 * interval '1 second'),payload=payload||jsonb_build_object('automatic_retry_count',$3::integer) WHERE intent_id=$1`, out.NextIntentID, int64(out.Delay/time.Second), retries+1)
		return err
	})
	return out, err
}
