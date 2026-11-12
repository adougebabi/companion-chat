package core

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	mediaSubmissionAmbiguousCode = "media_submission_ambiguous"
	mediaSubmissionUnknownStatus = "submission_unknown"
)

// This durable fence is committed immediately before any renderer request.
// A process loss after it commits cannot be distinguished from a request that
// reached the renderer, so only an explicitly reviewed retry can clear it.
func (a *App) beginMediaRendererSubmission(ctx context.Context, intent mediaIntent) error {
	if a == nil || a.DB == nil || a.DB.Pool() == nil {
		return errors.New("media_submission_storage_unavailable")
	}
	command, err := a.DB.Pool().Exec(ctx, `UPDATE public.media_intents SET renderer_submission_started=true,status='submitting',revision=revision+1 WHERE id=$1 AND workflow_id=$2 AND status IN ('pending','running') AND COALESCE(provider_job_id,'')='' AND NOT renderer_submission_started`, intent.ID, intent.WorkflowID)
	if err != nil {
		return err
	}
	if command.RowsAffected() != 1 {
		return ErrConflict
	}
	return nil
}

// ComfyUI's structured validation response rejects the prompt before it is
// enqueued. Generic errors and transport failures do not establish that fact.
func isDefiniteComfyValidationRejection(status int, data []byte) bool {
	if status != 400 {
		return false
	}
	value := decodeObject(data)
	_, nodeErrors := value["node_errors"].(map[string]any)
	if !nodeErrors {
		return false
	}
	switch strings.TrimSpace(stringValue(mapValue(value["error"])["type"])) {
	case "prompt_outputs_failed_validation", "invalid_prompt_id":
		return true
	default:
		return false
	}
}

func (a *App) clearRejectedMediaSubmission(ctx context.Context, intent mediaIntent) error {
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	command, err := a.DB.Pool().Exec(cleanup, `UPDATE public.media_intents SET renderer_submission_started=false,status='pending',revision=revision+1 WHERE id=$1 AND workflow_id=$2 AND status='submitting' AND renderer_submission_started AND COALESCE(provider_job_id,'')=''`, intent.ID, intent.WorkflowID)
	if err != nil {
		return err
	}
	if command.RowsAffected() != 1 {
		return ErrConflict
	}
	return nil
}

// Persist uncertainty even when the caller was cancelled after submission.
// If storage is temporarily unavailable, the previously committed fence
// still prevents an automatic re-submission on the next process.
func (a *App) markMediaSubmissionUnknown(ctx context.Context, intent mediaIntent) error {
	cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	command, err := a.DB.Pool().Exec(cleanup, `UPDATE public.media_intents SET status=$3,revision=revision+1 WHERE id=$1 AND workflow_id=$2 AND status IN ('pending','running','submitting','failed','reviewed_ready') AND renderer_submission_started AND COALESCE(provider_job_id,'')=''`, intent.ID, intent.WorkflowID, mediaSubmissionUnknownStatus)
	if err == nil && command.RowsAffected() > 0 {
		// A missing ActionOutcome is a valid independent media flow. Failure to
		// update diagnostics cannot undo the durable no-resubmit fence.
		_ = a.markActionOutcomeUnknown(cleanup, intent.ID, mediaSubmissionAmbiguousCode)
		a.recordDiagnosticEvent(cleanup, "media.submission.ambiguous", "warn", intent.Owner, intent.ID, "media:"+intent.ID, map[string]any{"reason_code": mediaSubmissionAmbiguousCode})
	}
	return err
}

func (a *App) pauseUncertainMediaSubmission(ctx context.Context, intent mediaIntent, cause error) error {
	return newCapabilityError(mediaSubmissionAmbiguousCode, false, errors.Join(cause, a.markMediaSubmissionUnknown(ctx, intent)))
}

// A terminated workflow may not have run its Activity cleanup after a crash.
// Reconciliation must expose the uncertain submission without resubmitting it.
func (a *App) ReconcileUncertainMediaSubmission(ctx context.Context, workflowIntentID string) error {
	var id, workflowID string
	err := a.DB.Pool().QueryRow(ctx, `SELECT m.id,m.workflow_id FROM public.media_intents m JOIN public.platform_workflow_intents w ON m.id=w.payload->>'intent_id' AND m.workflow_id=w.workflow_id WHERE w.intent_id=$1 AND m.status IN ('pending','running','submitting','failed','reviewed_ready') AND m.renderer_submission_started AND COALESCE(m.provider_job_id,'')=''`, workflowIntentID).Scan(&id, &workflowID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	intent, err := a.readMediaIntent(ctx, id)
	if err != nil {
		return err
	}
	if intent.WorkflowID != workflowID {
		return nil
	}
	return a.markMediaSubmissionUnknown(ctx, intent)
}
