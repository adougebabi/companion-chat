package core

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

var errLifecycleSupersededByCognition = errors.New("superseded_by_cognition")

type lifecycleIntent struct {
	intentID, workflowID, intentType, status string
	payload                                  []byte
}

type lifecycleIntentContextKey struct{}

// WithLifecycleIntentID binds the durable intent to an Activity's Core
// context.  Redis is only an acceleration/cancellation hint; the final
// settlement guard can therefore also consult PostgreSQL when Redis is down.
func WithLifecycleIntentID(ctx context.Context, intentID string) context.Context {
	return context.WithValue(ctx, lifecycleIntentContextKey{}, strings.TrimSpace(intentID))
}

func lifecycleIntentID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	value, _ := ctx.Value(lifecycleIntentContextKey{}).(string)
	return strings.TrimSpace(value)
}

func (a *App) lifecycleIntentSuperseded(ctx context.Context) bool {
	intentID := lifecycleIntentID(ctx)
	if a == nil || a.DB == nil || a.DB.Pool() == nil || intentID == "" {
		return false
	}
	var status, lastError string
	if err := a.DB.Pool().QueryRow(ctx, `SELECT status,COALESCE(last_error,'') FROM public.platform_workflow_intents WHERE intent_id=$1`, intentID).Scan(&status, &lastError); err != nil {
		return false
	}
	return strings.TrimSpace(status) == "superseded" && strings.TrimSpace(lastError) == "superseded_by_cognition"
}

func (a *App) lifecycleCancellationRequested(ctx context.Context, marker string) bool {
	if a.ProviderCancellationRequested(ctx, marker) {
		return true
	}
	return a.lifecycleIntentSuperseded(ctx)
}

func lifecycleIntentSupersededTx(ctx context.Context, tx pgx.Tx) (bool, error) {
	intentID := lifecycleIntentID(ctx)
	if intentID == "" || tx == nil {
		return false, nil
	}
	var status, lastError string
	err := tx.QueryRow(ctx, `SELECT status,COALESCE(last_error,'') FROM public.platform_workflow_intents WHERE intent_id=$1`, intentID).Scan(&status, &lastError)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(status) == "superseded" && strings.TrimSpace(lastError) == "superseded_by_cognition", nil
}

// CancelLifecycleForCognition preempts the lifecycle jobs that can overlap a
// newly-triggered cognition. Pending lifecycle work is superseded; running
// work receives both a Provider cancellation marker and a Temporal cancel
// request when the current process owns a workflow runtime. Cognition remains
// the durable priority and its completion re-arms the two debounce clocks.
func (a *App) CancelLifecycleForCognition(ctx context.Context, fluctlightID, requestID string) error {
	if a == nil || a.DB == nil || a.DB.Pool() == nil || strings.TrimSpace(fluctlightID) == "" {
		return nil
	}
	// Read a best-effort snapshot before taking the database lock.  This lets us
	// put the Provider cancellation marker in Redis before a running activity
	// can cross its final commit check.  The transaction below re-reads under
	// FOR UPDATE and also catches an intent inserted during this short window.
	intents := make([]lifecycleIntent, 0)
	rows, err := a.DB.Pool().Query(ctx, `
		SELECT intent_id,workflow_id,intent_type,status,payload
		FROM public.platform_workflow_intents
		WHERE payload->>'fluctlight_id'=$1
		  AND intent_type IN ('wake_up.current','reflection.run')
		  AND status IN ('pending','retry','started','running','cancel_requested')`, fluctlightID)
	if err != nil {
		return err
	}
	for rows.Next() {
		var item lifecycleIntent
		if scanErr := rows.Scan(&item.intentID, &item.workflowID, &item.intentType, &item.status, &item.payload); scanErr != nil {
			rows.Close()
			return scanErr
		}
		intents = append(intents, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	var firstErr error
	preemptedMarkers := make(map[string]struct{}, len(intents))
	for _, item := range intents {
		marker := lifecycleIntentCancellationMarker(fluctlightID, item)
		preemptedMarkers[marker] = struct{}{}
		if cancelErr := a.cancelLifecycleExecution(ctx, item, marker, requestID); cancelErr != nil && firstErr == nil {
			firstErr = cancelErr
		}
	}
	lockedIntents := make([]lifecycleIntent, 0, len(intents))
	err = withTransaction(ctx, a.DB.Pool(), func(tx pgx.Tx) error {
		if _, lockErr := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('fluctlight_lifecycle:' || $1))`, fluctlightID); lockErr != nil {
			return lockErr
		}
		rows, queryErr := tx.Query(ctx, `
			SELECT intent_id,workflow_id,intent_type,status,payload
			FROM public.platform_workflow_intents
			WHERE payload->>'fluctlight_id'=$1
			  AND intent_type IN ('wake_up.current','reflection.run')
			  AND status IN ('pending','retry','started','running','cancel_requested')
			FOR UPDATE`, fluctlightID)
		if queryErr != nil {
			return queryErr
		}
		for rows.Next() {
			var item lifecycleIntent
			if scanErr := rows.Scan(&item.intentID, &item.workflowID, &item.intentType, &item.status, &item.payload); scanErr != nil {
				rows.Close()
				return scanErr
			}
			lockedIntents = append(lockedIntents, item)
		}
		if rowsErr := rows.Err(); rowsErr != nil {
			rows.Close()
			return rowsErr
		}
		rows.Close()
		for _, item := range lockedIntents {
			if _, updateErr := tx.Exec(ctx, `
				UPDATE public.platform_workflow_intents
				SET status='superseded',completed_at=now(),started_at=NULL,last_error='superseded_by_cognition'
				WHERE intent_id=$1 AND status IN ('pending','retry','started','running','cancel_requested')`, item.intentID); updateErr != nil {
				return updateErr
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	// Intents created after the first snapshot were not marker-cancelled yet.
	// Their status is already superseded, so this follow-up only closes the
	// Provider/Temporal cancellation side of the contract and cannot execute a
	// business effect.
	for _, item := range lockedIntents {
		marker := lifecycleIntentCancellationMarker(fluctlightID, item)
		if _, already := preemptedMarkers[marker]; already {
			continue
		}
		if cancelErr := a.cancelLifecycleExecution(ctx, item, marker, requestID); cancelErr != nil && firstErr == nil {
			firstErr = cancelErr
		}
	}
	return firstErr
}

func lifecycleIntentCancellationMarker(fluctlightID string, item lifecycleIntent) string {
	if item.intentType == "wake_up.current" {
		payload := decodeObject(item.payload)
		return WakeUpProviderCancellationMarker(fluctlightID, intValue(payload["cycle"]))
	}
	return ReflectionProviderCancellationMarker(item.intentID)
}

func (a *App) cancelLifecycleExecution(ctx context.Context, item lifecycleIntent, marker, requestID string) error {
	var firstErr error
	if err := a.RequestProviderCancellation(ctx, marker); err != nil {
		firstErr = fmt.Errorf("request %s provider cancellation: %w", item.intentType, err)
	}
	if item.status == "started" || item.status == "running" || item.status == "cancel_requested" {
		workflowID := normalizedLifecycleWorkflowID(item.workflowID)
		if a.Workflows != nil && workflowID != "" {
			if cancelErr := a.Workflows.Cancel(ctx, workflowID, "", requestID); cancelErr != nil && !errors.Is(cancelErr, pgx.ErrNoRows) && firstErr == nil {
				firstErr = fmt.Errorf("cancel %s workflow: %w", item.intentType, cancelErr)
			}
		}
	}
	return firstErr
}

func normalizedLifecycleWorkflowID(workflowID string) string {
	workflowID = strings.TrimSpace(workflowID)
	if workflowID == "" || strings.HasPrefix(workflowID, "go:") {
		return workflowID
	}
	return "go:" + workflowID
}
